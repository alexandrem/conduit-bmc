// Package terminal provides terminal-based streaming console access for
// BMC Serial-over-LAN (SOL) sessions.
//
// It implements a terminal handler that bridges Connect RPC bidirectional
// streaming with local terminal I/O, enabling direct serial console access
// from the command line.
//
// # ARCHITECTURE
//
// The terminal package uses buf Connect bidirectional streaming to
// communicate with the Gateway:
//
//	CLI Terminal (stdin/stdout) ↔ SOLTerminal ↔ Connect Stream ↔ Gateway ↔ Agent ↔ BMC SOL
//
// Key components:
//   - Terminal raw mode: Character-by-character input using golang.org/x/term
//   - Bidirectional streaming: Real-time data flow in both directions
//   - Connect RPC: Type-safe protobuf messages (ConsoleDataChunk)
//   - Clean exit: Ctrl+] followed by 'q' to gracefully close the session
//
// # USAGE
//
// Command-line usage:
//
//	# Web console (default, user-friendly)
//	bmc-cli server console <server-id>
//
//	# Terminal streaming (advanced, for automation)
//	bmc-cli server console <server-id> --terminal
//
// Programmatic usage:
//
//	// Create SOL session to get session ID
//	session, err := client.CreateSOLSession(ctx, serverID)
//	if err != nil {
//	    return err
//	}
//
//	// Open Connect bidirectional stream
//	stream, err := client.StreamConsoleData(ctx, serverID, session.ID)
//	if err != nil {
//	    return err
//	}
//
//	// Create terminal handler
//	terminal := terminal.NewSOLTerminal(stream, session.ID)
//	defer terminal.Close()
//
//	// Start streaming
//	if err := terminal.Start(ctx); err != nil {
//	    return err
//	}
//
// # TERMINAL CONTROL
//
// The handler puts the local terminal into raw mode for proper
// serial console interaction. Raw mode is automatically restored
// when the session ends.
//
// Exit sequences:
//   - Ctrl+] then 'q': Clean exit with goodbye message
//   - Ctrl+C: Interrupt signal with graceful cleanup
//
// # STREAMING PROTOCOL
//
// Uses Connect RPC bidirectional streaming with ConsoleDataChunk messages.
// Handshake flow:
//  1. CLI sends handshake chunk with IsHandshake=true
//  2. Gateway validates session and routes to the agent
//  3. Agent establishes BMC SOL connection
//  4. Bidirectional data streaming begins
//
// # ERROR HANDLING
//
// Handles stream connection failures, terminal mode errors,
// I/O read/write errors, context cancellations, and signals.
// Cleanup routines restore terminal state and close resources.
//
// # THREAD SAFETY
//
// SOLTerminal uses goroutines for concurrent read/write operations:
//   - streamToStdout: Reads from Connect stream → writes to stdout
//   - stdinToStream: Reads from stdin → writes to Connect stream
//
// A mutex protects shared state during exit sequence detection and cleanup.
//
// # COMPARISON WITH WEB CONSOLE
//
// Terminal Streaming (this package, --terminal flag):
//   - Connect RPC bidirectional streaming
//   - Direct stdin/stdout integration
//   - Raw terminal mode
//   - Ideal for automation, scripting, CLI workflows
//   - Advanced usage
//
// Web Console (default behavior):
//   - WebSocket protocol
//   - XTerm.js browser terminal emulator
//   - Web-based GUI with power controls
//   - Ideal for interactive browser-based management
//   - User-friendly, recommended for most users
package terminal
