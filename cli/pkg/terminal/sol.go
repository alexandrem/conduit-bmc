package terminal

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/term"

	gatewayv1 "gateway/gen/gateway/v1"
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

	// rawMode controls whether to preserve terminal control sequences (true)
	// or convert them for append-only output (false, default)
	rawMode bool
}

// NewSOLTerminal creates a new SOL terminal handler with Connect bidirectional streaming.
//
// The stream parameter should be an active Connect RPC bidirectional stream obtained from
// the gateway client. The sessionID must match the session created via CreateSOLSession.
//
// The terminal handler uses os.Stdin and os.Stdout by default. To use custom I/O streams,
// modify the returned SOLTerminal's stdin and stdout fields before calling Start.
//
// By default, the terminal operates in append-only mode (rawMode=false), which converts
// carriage returns to newlines for cleaner output. Use SetRawMode(true) to preserve all
// terminal control sequences.
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
		rawMode:   false, // Default to append-only mode
	}
}

// SetRawMode configures whether to preserve terminal control sequences.
//
// When rawMode is true, all terminal control sequences (carriage returns, ANSI codes, etc.)
// are preserved, allowing the BMC console to overwrite lines and position the cursor.
//
// When rawMode is false (default), carriage returns are converted to newlines for append-only
// output, preventing lines from being overwritten.
//
// This method must be called before Start().
func (t *SOLTerminal) SetRawMode(raw bool) {
	t.rawMode = raw
}

// Start begins the terminal streaming session.
//
// This method sets the terminal to raw mode, starts bidirectional streaming goroutines,
// and blocks until the session ends due to:
//   - User exit sequence (Ctrl+] then 'q')
//   - Interrupt signal (Ctrl+C, detected as raw byte 0x03)
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
	// Clear the terminal screen before starting
	// This ensures any previous output doesn't interfere with console display
	clearScreen()

	// Print initial message to stderr (stdout is for console data only)
	fmt.Fprintln(os.Stderr, "Connected to server console. Press Ctrl+C or Ctrl+] then 'q' to exit.")
	fmt.Fprintln(os.Stderr, "----------------------------------------")

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
		fmt.Fprintln(os.Stderr, "\nInterrupted. Closing console...")
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
				// Apply CR to newline conversion if not in raw mode
				data := msg.Data
				if !t.rawMode {
					data = t.convertCRtoNewline(data)
				}

				if _, err := t.stdout.Write(data); err != nil {
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
//   - Ctrl+C interrupt signal (raw byte 0x03)
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
			// Set a short read deadline to make stdin reads interruptible
			// This allows the select statement to check for cancellation
			t.stdin.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

			n, err := t.stdin.Read(buf)
			if err != nil {
				// Check if it's a timeout error - this is expected and we should continue
				if os.IsTimeout(err) {
					continue
				}
				// Check for temporary errors
				if netErr, ok := err.(interface{ Temporary() bool }); ok && netErr.Temporary() {
					continue
				}
				// Real error occurred
				if err != io.EOF {
					return err
				}
				return io.EOF
			}

			if n > 0 {
				// Make a copy of the data since buf is reused
				data := make([]byte, n)
				copy(data, buf[:n])

				// Check for Ctrl+C (0x03) in raw mode
				for _, b := range data {
					if b == 0x03 {
						fmt.Fprintln(os.Stderr, "\n----------------------------------------")
						fmt.Fprintln(os.Stderr, "Console interrupted by user.")
						close(t.done)
						return io.EOF
					}
				}

				// Check for exit sequence
				if t.checkExitSequence(data) {
					fmt.Fprintln(os.Stderr, "\n----------------------------------------")
					fmt.Fprintln(os.Stderr, "Console closed by user.")
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

// convertCRtoNewline converts standalone carriage returns to newlines for append-only output.
//
// This method converts bare \r characters (not followed by \n) to \n to prevent lines from
// being overwritten in the terminal. Windows line endings (\r\n) are preserved as-is.
//
// This ensures append-only output where each line is added to the screen rather than
// overwriting previous content.
func (t *SOLTerminal) convertCRtoNewline(data []byte) []byte {
	result := make([]byte, 0, len(data))

	for i := 0; i < len(data); i++ {
		if data[i] == '\r' {
			// Check if this is part of \r\n (Windows line ending)
			if i+1 < len(data) && data[i+1] == '\n' {
				// Keep \r\n as is
				result = append(result, '\r')
			} else {
				// Convert standalone \r to \n
				result = append(result, '\n')
			}
		} else {
			result = append(result, data[i])
		}
	}

	return result
}

// clearScreen clears the terminal screen using ANSI escape sequences.
//
// This sends the standard ANSI clear screen escape sequence "\033[2J" followed by
// moving the cursor to home position "\033[H". This works on all modern terminals
// (Linux, macOS, Windows Terminal, etc.).
//
// The clear happens on stderr to avoid mixing with console data on stdout.
func clearScreen() {
	// ANSI escape sequences:
	// \033[2J - Clear entire screen
	// \033[H  - Move cursor to home position (0,0)
	fmt.Fprint(os.Stderr, "\033[2J\033[H")
}
