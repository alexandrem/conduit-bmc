package sol

import (
	"context"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// IPMISOLSession manages a Serial-over-LAN session using FreeIPMI's ipmiconsole subprocess
type IPMISOLSession struct {
	endpoint string
	username string
	password string

	// Subprocess management
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

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

	// Build ipmiconsole command
	// Use environment variables for credentials to avoid them appearing in process list
	s.cmd = exec.CommandContext(s.ctx, s.ipmiconsoleePath,
		"-h", host,
		"--serial-keepalive",
		"--dont-steal",
	)

	// Set credentials via environment variables (more secure than command-line args)
	s.cmd.Env = append(os.Environ(),
		fmt.Sprintf("IPMI_USERNAME=%s", s.username),
		fmt.Sprintf("IPMI_PASSWORD=%s", s.password),
	)

	// Setup pipes
	s.stdin, err = s.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	s.stdout, err = s.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	s.stderr, err = s.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ipmiconsole: %w", err)
	}

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

// handleOutput reads from ipmiconsole stdout/stderr and sends to outputChan
func (s *IPMISOLSession) handleOutput() {
	// Stdout reader
	go func() {
		buffer := make([]byte, s.bufferSize)
		for {
			if s.stdout == nil {
				return
			}

			n, err := s.stdout.Read(buffer)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buffer[:n])

				// Store in replay buffer
				if s.replayBuffer != nil {
					s.replayBuffer.Write(data)
				}

				s.recordRead(n)

				select {
				case s.outputChan <- data:
				case <-s.ctx.Done():
					return
				}
			}

			if err != nil {
				if err != io.EOF {
					log.Debug().Err(err).Msg("stdout read failed")
					s.errorChan <- fmt.Errorf("stdout read failed: %w", err)
				}
				return
			}
		}
	}()

	// Stderr reader
	go func() {
		buffer := make([]byte, s.bufferSize)
		for {
			if s.stderr == nil {
				return
			}

			n, err := s.stderr.Read(buffer)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buffer[:n])

				// Also send stderr to output (for error messages from ipmiconsole)
				log.Debug().Str("stderr", string(data)).Msg("ipmiconsole stderr")

				select {
				case s.outputChan <- data:
				case <-s.ctx.Done():
					return
				}
			}

			if err != nil {
				if err != io.EOF {
					log.Debug().Err(err).Msg("stderr read failed")
				}
				return
			}
		}
	}()
}

// Write sends data to the BMC console (client → BMC)
func (s *IPMISOLSession) Write(data []byte) error {
	select {
	case s.inputChan <- data:
		return nil
	case <-s.ctx.Done():
		return s.ctx.Err()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("write timeout")
	}
}

// Read receives data from the BMC console (BMC → client)
func (s *IPMISOLSession) Read() ([]byte, error) {
	select {
	case data := <-s.outputChan:
		return data, nil
	case err := <-s.errorChan:
		return nil, err
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
