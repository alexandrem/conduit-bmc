package sol

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// MockTransport implements Transport with a simulated shell session
// This provides a realistic demonstration of SOL functionality without requiring
// actual IPMI SOL support from hardware/VirtualBMC
type MockTransport struct {
	mu      sync.RWMutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	status  TransportStatus
	stopCh  chan struct{}
	readCh  chan []byte
	writeCh chan []byte
}

// NewMockTransport creates a new mock SOL transport with shell simulation
func NewMockTransport() *MockTransport {
	return &MockTransport{
		status:  TransportStatus{Connected: false, Protocol: "mock-sol", Message: "disconnected"},
		stopCh:  make(chan struct{}),
		readCh:  make(chan []byte, 1024),
		writeCh: make(chan []byte, 1024),
	}
}

// Connect establishes a mock SOL session by spawning a shell
func (t *MockTransport) Connect(ctx context.Context, endpoint, username, password string, config *Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status.Connected {
		return nil
	}

	// Spawn a shell process to simulate the serial console
	// Use sh for maximum compatibility
	t.cmd = exec.Command("/bin/sh")

	// Set up pipes for stdin/stdout
	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	t.stdin = stdin

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	t.stdout = stdout

	// Also capture stderr to stdout
	t.cmd.Stderr = t.cmd.Stdout

	// Start the shell
	if err := t.cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return fmt.Errorf("failed to start shell: %w", err)
	}

	// Send a welcome banner to simulate BMC console
	welcomeBanner := fmt.Sprintf(
		"\r\n"+
			"=====================================\r\n"+
			"  Serial-Over-LAN Console\r\n"+
			"  Endpoint: %s\r\n"+
			"  Session established\r\n"+
			"=====================================\r\n"+
			"\r\n",
		endpoint,
	)
	t.readCh <- []byte(welcomeBanner)

	// Start goroutines to handle I/O
	go t.handleShellOutput(ctx)
	go t.handleShellInput(ctx)

	t.status = TransportStatus{Connected: true, Protocol: "mock-sol", Message: "shell active"}
	return nil
}

// Read reads console output from the mock SOL transport
func (t *MockTransport) Read(ctx context.Context) ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.status.Connected {
		return nil, fmt.Errorf("transport not connected")
	}

	select {
	case data := <-t.readCh:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.stopCh:
		return nil, fmt.Errorf("transport stopped")
	}
}

// Write sends console input to the mock SOL transport
func (t *MockTransport) Write(ctx context.Context, data []byte) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.status.Connected {
		return fmt.Errorf("transport not connected")
	}

	select {
	case t.writeCh <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-t.stopCh:
		return fmt.Errorf("transport stopped")
	}
}

// Close terminates the mock SOL transport
func (t *MockTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.status.Connected {
		return nil
	}

	close(t.stopCh)

	// Close pipes
	if t.stdin != nil {
		t.stdin.Close()
	}
	if t.stdout != nil {
		t.stdout.Close()
	}

	// Kill the shell process
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
		t.cmd.Wait()
	}

	t.status = TransportStatus{Connected: false, Protocol: "mock-sol", Message: "disconnected"}
	return nil
}

// Status returns the current transport status
func (t *MockTransport) Status() TransportStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

// SupportsSOL checks if mock SOL is available (always true for mock)
func (t *MockTransport) SupportsSOL(ctx context.Context, endpoint, username, password string) (bool, error) {
	return true, nil
}

// handleShellOutput reads from the shell stdout and sends to readCh
func (t *MockTransport) handleShellOutput(ctx context.Context) {
	defer func() {
		close(t.readCh)
	}()

	reader := bufio.NewReader(t.stdout)
	buf := make([]byte, 4096)

	// Use a goroutine to read from stdout
	readCh := make(chan []byte)
	errCh := make(chan error, 1)

	go func() {
		for {
			n, err := reader.Read(buf)
			if err != nil {
				errCh <- err
				return
			}
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				readCh <- data
			}
		}
	}()

	for {
		select {
		case <-t.stopCh:
			return
		case <-ctx.Done():
			return
		case err := <-errCh:
			if err == io.EOF {
				// Shell exited
				return
			}
			// Other error, continue
			return
		case data := <-readCh:
			// Send data to read channel
			select {
			case t.readCh <- data:
			case <-t.stopCh:
				return
			case <-ctx.Done():
				return
			default:
				// Channel full, drop data
			}
		case <-time.After(100 * time.Millisecond):
			// Periodic check to allow context/stop checks
			continue
		}
	}
}

// handleShellInput reads from writeCh and sends to shell stdin
func (t *MockTransport) handleShellInput(ctx context.Context) {
	defer func() {
		close(t.writeCh)
	}()

	for {
		select {
		case <-t.stopCh:
			return
		case <-ctx.Done():
			return
		case data := <-t.writeCh:
			if len(data) > 0 {
				// Write to shell stdin
				_, err := t.stdin.Write(data)
				if err != nil {
					// Shell closed, exit
					return
				}
			}
		}
	}
}
