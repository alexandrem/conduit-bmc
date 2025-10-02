package terminal

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"connectrpc.com/connect"
	gatewayv1 "gateway/gen/gateway/v1"
	"golang.org/x/term"
)

const (
	// exitSequence1 is the first byte of the exit sequence (Ctrl+]).
	// This follows the convention used by telnet and other terminal programs.
	exitSequence1 = 0x1D // Ctrl+]

	// exitSequence2 is the second byte of the exit sequence ('q').
	// After pressing Ctrl+], the user must press 'q' to exit.
	exitSequence2 = 'q'
)

// SOLTerminal handles terminal streaming for Serial-over-LAN console using Connect RPC.
//
// SOLTerminal provides a bridge between the local terminal (stdin/stdout) and a remote
// BMC serial console via Connect RPC bidirectional streaming. It manages terminal raw mode,
// bidirectional data flow, exit sequence detection, and graceful cleanup.
//
// The terminal operates in raw mode (character-by-character input) to properly support
// interactive console applications like text editors, system installers, and shell prompts.
//
// Thread safety: Multiple goroutines handle concurrent read/write operations. A mutex
// protects shared state during exit sequence detection and cleanup operations.
type SOLTerminal struct {
	// stream is the Connect RPC bidirectional stream for console data
	stream *connect.BidiStreamForClient[gatewayv1.ConsoleDataChunk, gatewayv1.ConsoleDataChunk]

	// sessionID uniquely identifies this console session
	sessionID string

	// stdin is the local terminal input (typically os.Stdin)
	stdin *os.File

	// stdout is the local terminal output (typically os.Stdout)
	stdout *os.File

	// oldState stores the original terminal state for restoration on exit
	oldState *term.State

	// mu protects concurrent access to shared state
	mu sync.Mutex

	// done signals that the session should terminate
	done chan struct{}

	// exitPressed tracks whether Ctrl+] was pressed (first byte of exit sequence)
	exitPressed bool
}

// NewSOLTerminal creates a new SOL terminal handler with Connect bidirectional streaming.
//
// The stream parameter should be an active Connect RPC bidirectional stream obtained from
// the gateway client. The sessionID must match the session created via CreateSOLSession.
//
// The terminal handler uses os.Stdin and os.Stdout by default. To use custom I/O streams,
// modify the returned SOLTerminal's stdin and stdout fields before calling Start.
//
// Example:
//
//	stream, err := client.StreamConsoleData(ctx, serverID, sessionID)
//	if err != nil {
//	    return err
//	}
//	terminal := terminal.NewSOLTerminal(stream, sessionID)
//	defer terminal.Close()
//	return terminal.Start(ctx)
func NewSOLTerminal(stream *connect.BidiStreamForClient[gatewayv1.ConsoleDataChunk, gatewayv1.ConsoleDataChunk], sessionID string) *SOLTerminal {
	return &SOLTerminal{
		stream:    stream,
		sessionID: sessionID,
		stdin:     os.Stdin,
		stdout:    os.Stdout,
		done:      make(chan struct{}),
	}
}

// Start begins the terminal streaming session.
//
// This method sets the terminal to raw mode, starts bidirectional streaming goroutines,
// and blocks until the session ends due to:
//   - User exit sequence (Ctrl+] then 'q')
//   - Interrupt signal (Ctrl+C)
//   - Context cancellation
//   - Stream error
//
// The terminal state is automatically restored before returning, even on errors.
//
// Start should only be called once per SOLTerminal instance. Calling Start multiple
// times on the same instance will result in undefined behavior.
//
// Returns an error if:
//   - Terminal cannot be set to raw mode (e.g., stdin is not a TTY)
//   - Stream encounters a fatal error
//   - Context is cancelled
func (t *SOLTerminal) Start(ctx context.Context) error {
	// Print initial message
	fmt.Println("Connected to server console. Press Ctrl+] then 'q' to exit.")
	fmt.Println("----------------------------------------")

	// Set terminal to raw mode
	if err := t.setRawMode(); err != nil {
		return fmt.Errorf("failed to set raw mode: %w", err)
	}
	defer t.restore()

	// Handle cleanup on interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// Start goroutines for bidirectional streaming
	errCh := make(chan error, 2)
	var wg sync.WaitGroup

	// Read from Connect stream and write to stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := t.streamToStdout(ctx); err != nil && err != io.EOF {
			errCh <- fmt.Errorf("stream read error: %w", err)
		}
	}()

	// Read from stdin and write to Connect stream
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := t.stdinToStream(ctx); err != nil && err != io.EOF {
			errCh <- fmt.Errorf("stdin read error: %w", err)
		}
	}()

	// Wait for completion or error
	select {
	case <-ctx.Done():
		t.Close()
		wg.Wait()
		return ctx.Err()
	case <-sigCh:
		fmt.Println("\nInterrupted. Closing console...")
		t.Close()
		wg.Wait()
		return nil
	case err := <-errCh:
		t.Close()
		wg.Wait()
		return err
	case <-t.done:
		wg.Wait()
		return nil
	}
}

// streamToStdout reads from Connect stream and writes to stdout.
//
// This goroutine continuously receives ConsoleDataChunk messages from the Connect stream
// and writes the data to stdout. It handles:
//   - CloseStream signals from the server
//   - Stream errors and EOF
//   - Context cancellation
//   - Done channel signals
//
// This method runs in its own goroutine and returns when the stream ends.
func (t *SOLTerminal) streamToStdout(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.done:
			return io.EOF
		default:
			// Receive from Connect stream
			msg, err := t.stream.Receive()
			if err != nil {
				if err == io.EOF {
					return io.EOF
				}
				return fmt.Errorf("stream receive error: %w", err)
			}

			// Check for close signal
			if msg.CloseStream {
				return io.EOF
			}

			// Write console data to stdout
			if len(msg.Data) > 0 {
				if _, err := t.stdout.Write(msg.Data); err != nil {
					return fmt.Errorf("failed to write to stdout: %w", err)
				}
			}
		}
	}
}

// stdinToStream reads from stdin and writes to Connect stream.
//
// This goroutine continuously reads from stdin and sends ConsoleDataChunk messages
// to the Connect stream. It handles:
//   - Exit sequence detection (Ctrl+] then 'q')
//   - Context cancellation
//   - Done channel signals
//   - Stream send errors
//
// Each chunk of data read from stdin is sent as a separate ConsoleDataChunk with
// the session ID. The exit sequence check happens before sending data.
//
// This method runs in its own goroutine and returns when the user exits or the stream ends.
func (t *SOLTerminal) stdinToStream(ctx context.Context) error {
	buf := make([]byte, 1024)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.done:
			return io.EOF
		default:
			n, err := t.stdin.Read(buf)
			if err != nil {
				return err
			}

			if n > 0 {
				data := buf[:n]

				// Check for exit sequence
				if t.checkExitSequence(data) {
					fmt.Println("\n----------------------------------------")
					fmt.Println("Console closed by user.")
					close(t.done)
					return io.EOF
				}

				// Send data to Connect stream
				chunk := &gatewayv1.ConsoleDataChunk{
					SessionId: t.sessionID,
					Data:      data,
				}

				if err := t.stream.Send(chunk); err != nil {
					return fmt.Errorf("failed to send to stream: %w", err)
				}
			}
		}
	}
}

// checkExitSequence checks if the exit sequence was pressed.
//
// The exit sequence is Ctrl+] (0x1D) followed by 'q'. This method maintains state
// across calls to handle the sequence split across multiple read operations.
//
// Returns true if the complete exit sequence is detected, false otherwise.
//
// Thread safety: This method is protected by the SOLTerminal mutex.
func (t *SOLTerminal) checkExitSequence(data []byte) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, b := range data {
		if t.exitPressed {
			// Previous byte was Ctrl+], check for 'q'
			if b == exitSequence2 {
				return true
			}
			t.exitPressed = false
		} else if b == exitSequence1 {
			// Got Ctrl+], wait for next byte
			t.exitPressed = true
		}
	}

	return false
}

// setRawMode sets the terminal to raw mode for character-by-character input.
//
// Raw mode disables line buffering and echo, allowing the BMC console to receive
// each keystroke immediately. This is essential for interactive applications like
// text editors, system installers, and shell prompts.
//
// The original terminal state is saved in t.oldState for restoration on exit.
//
// Returns an error if stdin is not a terminal (e.g., input is piped).
func (t *SOLTerminal) setRawMode() error {
	// Check if stdin is a terminal
	if !term.IsTerminal(int(t.stdin.Fd())) {
		return fmt.Errorf("stdin is not a terminal")
	}

	// Get current terminal state
	state, err := term.MakeRaw(int(t.stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw mode: %w", err)
	}

	t.oldState = state
	return nil
}

// restore restores the terminal to its original state.
//
// This method is called automatically by Start's defer to ensure terminal state
// is always restored, even if an error occurs. It is safe to call multiple times.
func (t *SOLTerminal) restore() {
	if t.oldState != nil {
		_ = term.Restore(int(t.stdin.Fd()), t.oldState)
	}
}

// Close closes the Connect stream and cleans up resources.
//
// This method sends a CloseStream signal to the server, closes the done channel to
// signal goroutines to exit, and closes the Connect stream.
//
// Close is safe to call multiple times. Subsequent calls after the first are no-ops.
//
// Note: Close does NOT restore terminal state. Use defer with Start for proper cleanup:
//
//	terminal := terminal.NewSOLTerminal(stream, sessionID)
//	defer terminal.Close()
//	return terminal.Start(ctx)  // Start will restore terminal state
func (t *SOLTerminal) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	select {
	case <-t.done:
		// Already closed
	default:
		close(t.done)
	}

	if t.stream != nil {
		// Send close signal
		closeChunk := &gatewayv1.ConsoleDataChunk{
			SessionId:   t.sessionID,
			CloseStream: true,
		}
		_ = t.stream.Send(closeChunk)

		// Close the stream
		return t.stream.CloseRequest()
	}

	return nil
}
