package sol

import (
	"context"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/rs/zerolog/log"
)

// IPMISOLSession manages a Serial-over-LAN session using FreeIPMI's ipmiconsole subprocess
type IPMISOLSession struct {
	endpoint string
	username string
	password string

	// Subprocess management
	cmd     *exec.Cmd
	ptyFile *os.File // PTY master for ipmiconsole
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser

	// Streaming channels
	inputChan  chan []byte // Client → BMC
	outputChan chan []byte // BMC → Client
	errorChan  chan error

	// Lifecycle management
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	running bool

	// Configuration
	bufferSize       int
	retryDelay       time.Duration
	maxRetryDelay    time.Duration
	retryMultiplier  float64
	ipmiconsoleePath string

	// Session replay
	replayBuffer *circularBuffer

	// Metrics
	metrics SOLMetrics
}

// SOLMetrics tracks session statistics
type SOLMetrics struct {
	bytesRead     uint64
	bytesWritten  uint64
	reconnections uint64
	lastError     error
	lastErrorTime time.Time
	uptime        time.Time
	mu            sync.RWMutex
}

// backoffState tracks exponential backoff state
type backoffState struct {
	attempts int
}

// NewIPMISOLSession creates a new IPMI SOL session using ipmiconsole subprocess
func NewIPMISOLSession(ctx context.Context, endpoint, username, password string, replayBufferSize int) (*IPMISOLSession, error) {
	sessionCtx, cancel := context.WithCancel(ctx)

	session := &IPMISOLSession{
		endpoint:         endpoint,
		username:         username,
		password:         password,
		ctx:              sessionCtx,
		cancel:           cancel,
		inputChan:        make(chan []byte, 64),
		outputChan:       make(chan []byte, 64),
		errorChan:        make(chan error, 16),
		bufferSize:       1024,
		retryDelay:       2 * time.Second,
		maxRetryDelay:    60 * time.Second,
		retryMultiplier:  2.0,
		ipmiconsoleePath: "/usr/sbin/ipmiconsole",
		metrics: SOLMetrics{
			uptime: time.Now(),
		},
	}

	// Check for ipmiconsole in PATH if default doesn't exist
	if _, err := os.Stat(session.ipmiconsoleePath); os.IsNotExist(err) {
		if path, err := exec.LookPath("ipmiconsole"); err == nil {
			session.ipmiconsoleePath = path
		} else {
			return nil, fmt.Errorf("ipmiconsole not found: install freeipmi-tools package")
		}
	}

	// Initialize replay buffer if requested
	if replayBufferSize > 0 {
		session.replayBuffer = newCircularBuffer(replayBufferSize)
	}

	// Start the session in background
	go func() {
		if err := session.runWithBackoff(); err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("IPMI SOL session terminated")
		}
	}()

	return session, nil
}

// runWithBackoff manages the session lifecycle with exponential backoff
func (s *IPMISOLSession) runWithBackoff() error {
	backoff := &backoffState{attempts: 0}

	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		default:
		}

		// Start ipmiconsole subprocess
		err := s.startProcess()
		if err == nil {
			backoff.attempts = 0 // Reset on success

			// Start I/O handlers
			go s.handleInput()
			go s.handleOutput()

			// Wait for process to exit
			err = s.cmd.Wait()

			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}

		// Check if context was cancelled
		if s.ctx.Err() != nil {
			return s.ctx.Err()
		}

		// Calculate backoff delay
		delay := s.calculateBackoff(backoff)
		backoff.attempts++

		// Record reconnection
		s.recordReconnection(err)

		log.Warn().
			Err(err).
			Int("attempt", backoff.attempts).
			Dur("retry_in", delay).
			Msg("ipmiconsole failed, retrying")

		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-time.After(delay):
			continue
		}
	}
}

// calculateBackoff computes the exponential backoff delay
func (s *IPMISOLSession) calculateBackoff(backoff *backoffState) time.Duration {
	delay := time.Duration(float64(s.retryDelay) *
		math.Pow(s.retryMultiplier, float64(backoff.attempts)))

	if delay > s.maxRetryDelay {
		delay = s.maxRetryDelay
	}

	return delay
}

// startProcess starts the ipmiconsole subprocess
func (s *IPMISOLSession) startProcess() error {
	// Parse endpoint for host/port
	host, _, err := net.SplitHostPort(s.endpoint)
	if err != nil {
		host = s.endpoint
	}

	log.Debug().
		Str("host", host).
		Str("username", s.username).
		Str("password", s.password).
		Msg("ipmiconsole starting")

	// Build ipmiconsole command
	// Note: -u and -p flags are required (ipmiconsole doesn't support env vars)
	// Note: Removed --dont-steal to allow taking over existing SOL sessions
	// Note: --lock-memory suppresses connection status messages to keep output clean
	s.cmd = exec.CommandContext(s.ctx, s.ipmiconsoleePath,
		"-h", host,
		"-u", s.username,
		"-p", s.password,
		"--serial-keepalive",
		"--lock-memory",
	)

	// Start the process with a PTY
	// ipmiconsole REQUIRES a PTY to work properly, otherwise it exits with
	// "tcgetattr: Inappropriate ioctl for device" and closes the connection
	s.ptyFile, err = pty.Start(s.cmd)
	if err != nil {
		return fmt.Errorf("failed to start ipmiconsole with PTY: %w", err)
	}

	// Use the PTY for both stdin and stdout
	// ipmiconsole communicates through the PTY, not separate pipes
	s.stdin = s.ptyFile
	s.stdout = s.ptyFile
	// We won't get stderr separately with PTY, but that's OK

	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	log.Info().
		Str("endpoint", s.endpoint).
		Str("username", s.username).
		Msg("Started ipmiconsole subprocess")

	return nil
}

// handleInput reads from inputChan and writes to ipmiconsole stdin
func (s *IPMISOLSession) handleInput() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case data := <-s.inputChan:
			if s.stdin == nil {
				continue
			}

			n, err := s.stdin.Write(data)
			if err != nil {
				log.Debug().Err(err).Msg("stdin write failed")
				s.errorChan <- fmt.Errorf("stdin write failed: %w", err)
				return
			}

			s.recordWrite(n)
		}
	}
}

// handleOutput reads from ipmiconsole PTY and sends to outputChan
func (s *IPMISOLSession) handleOutput() {
	// PTY reader (stdout and stderr are merged in PTY)
	buffer := make([]byte, s.bufferSize)
	for {
		if s.stdout == nil {
			return
		}

		n, err := s.stdout.Read(buffer)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buffer[:n])

			// Store in replay buffer (before filtering)
			if s.replayBuffer != nil {
				s.replayBuffer.Write(data)
			}

			s.recordRead(n)

			// Filter out ipmiconsole status messages before sending to client
			filtered := s.filterIPMIConsoleMessages(data)
			if len(filtered) > 0 {
				select {
				case s.outputChan <- filtered:
				case <-s.ctx.Done():
					return
				}
			}
		}

		if err != nil {
			if err != io.EOF {
				log.Debug().Err(err).Msg("PTY read failed")
				s.errorChan <- fmt.Errorf("PTY read failed: %w", err)
			}
			return
		}
	}
}

// filterIPMIConsoleMessages removes ipmiconsole status messages from console output
// These messages are control information from ipmiconsole itself, not from the BMC
func (s *IPMISOLSession) filterIPMIConsoleMessages(data []byte) []byte {
	// Filter ipmiconsole control messages that appear during connection setup
	// These include status messages with carriage returns used for spinner animation

	// Pattern 1: Lines with "establishing link..." and spinner characters
	// This includes carriage return sequences used to update the spinner in place
	// Match: "\restablishing link...|  " or similar with \r at start
	establishingPattern := regexp.MustCompile(`\r?\s*establishing link\.\.\.[\|/\-\\ ]*\r?`)
	result := establishingPattern.ReplaceAll(data, []byte{})

	// Pattern 2: Carriage returns followed by "establishing" (catch animation frames)
	crEstablishingPattern := regexp.MustCompile(`\r+establishing link\.\.\.[\|/\-\\ ]*`)
	result = crEstablishingPattern.ReplaceAll(result, []byte{})

	// Pattern 3: Standalone "Initializing" messages
	initPattern := regexp.MustCompile(`(?m)^\r?Initializing\s*\r?$`)
	result = initPattern.ReplaceAll(result, []byte{})

	// Pattern 4: [SOL established] messages
	solEstablishedPattern := regexp.MustCompile(`(?m)^\r?\[SOL established\]\s*\r?$`)
	result = solEstablishedPattern.ReplaceAll(result, []byte{})

	// Pattern 5: Remove orphaned carriage returns that were part of status updates
	// Only remove multiple carriage returns or CR at start of data before real content
	// Don't remove all CRs as they might be legitimate (e.g., Windows line endings)
	orphanedCRPattern := regexp.MustCompile(`^\r+`)
	result = orphanedCRPattern.ReplaceAll(result, []byte{})

	// Pattern 6: Remove sequences like "\r   \r" (clear line patterns from spinner)
	clearLinePattern := regexp.MustCompile(`\r\s+\r`)
	result = clearLinePattern.ReplaceAll(result, []byte{})

	return result
}

// Write sends data to the BMC console (client → BMC)
func (s *IPMISOLSession) Write(ctx context.Context, data []byte) error {
	select {
	case s.inputChan <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ctx.Done():
		return s.ctx.Err()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("write timeout")
	}
}

// Read receives data from the BMC console (BMC → client)
func (s *IPMISOLSession) Read(ctx context.Context) ([]byte, error) {
	select {
	case data := <-s.outputChan:
		return data, nil
	case err := <-s.errorChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

// ReadChannel returns the output channel for direct access
func (s *IPMISOLSession) ReadChannel() <-chan []byte {
	return s.outputChan
}

// WriteChannel returns the input channel for direct access
func (s *IPMISOLSession) WriteChannel() chan<- []byte {
	return s.inputChan
}

// Close terminates the SOL session
func (s *IPMISOLSession) Close() error {
	s.cancel() // Trigger context cancellation

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil && s.cmd.Process != nil {
		// Graceful shutdown: send interrupt signal
		if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
			log.Debug().Err(err).Msg("Failed to send interrupt to ipmiconsole")
		}

		// Wait with timeout
		done := make(chan struct{})
		go func() {
			s.cmd.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Clean exit
			log.Debug().Msg("ipmiconsole exited cleanly")
		case <-time.After(5 * time.Second):
			// Force kill
			log.Warn().Msg("ipmiconsole did not exit, force killing")
			if err := s.cmd.Process.Kill(); err != nil {
				log.Error().Err(err).Msg("Failed to kill ipmiconsole")
			}
		}
	}

	// Close PTY
	if s.ptyFile != nil {
		s.ptyFile.Close()
	}

	// Close channels
	close(s.inputChan)
	close(s.outputChan)
	close(s.errorChan)

	return nil
}

// IsRunning returns true if the ipmiconsole subprocess is running
func (s *IPMISOLSession) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// GetReplayBuffer returns the replay buffer contents
func (s *IPMISOLSession) GetReplayBuffer() []byte {
	if s.replayBuffer == nil {
		return nil
	}
	return s.replayBuffer.Read()
}

// GetMetrics returns session metrics
func (s *IPMISOLSession) GetMetrics() SOLMetrics {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()
	return s.metrics
}

// Metrics recording methods

func (s *IPMISOLSession) recordRead(n int) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()
	s.metrics.bytesRead += uint64(n)
}

func (s *IPMISOLSession) recordWrite(n int) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()
	s.metrics.bytesWritten += uint64(n)
}

func (s *IPMISOLSession) recordReconnection(err error) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()
	s.metrics.reconnections++
	s.metrics.lastError = err
	s.metrics.lastErrorTime = time.Now()
}
