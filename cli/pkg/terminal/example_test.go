package terminal_test

import (
	"context"
	"fmt"
	"log"

	"cli/pkg/client"
	"cli/pkg/config"
	"cli/pkg/terminal"
)

// ExampleSOLTerminal demonstrates basic usage of the SOL terminal handler.
func ExampleSOLTerminal() {
	// Create client configuration
	cfg := &config.Config{
		Manager: config.ManagerConfig{
			Endpoint: "http://localhost:8080",
		},
		Auth: config.AuthConfig{
			APIKey: "test-api-key-123",
		},
	}

	// Initialize client
	client := client.New(cfg)
	ctx := context.Background()

	// Server ID to connect to
	serverID := "server-001"

	// Create SOL session
	session, err := client.CreateSOLSession(ctx, serverID)
	if err != nil {
		log.Fatalf("Failed to create SOL session: %v", err)
	}

	fmt.Printf("SOL session created: %s\n", session.ID)

	// Open Connect bidirectional stream
	stream, err := client.StreamConsoleData(ctx, serverID, session.ID)
	if err != nil {
		log.Fatalf("Failed to open console stream: %v", err)
	}

	// Create terminal handler
	term := terminal.NewSOLTerminal(stream, session.ID)
	defer term.Close()

	// Start streaming (blocks until exit)
	if err := term.Start(ctx); err != nil {
		log.Fatalf("Console session error: %v", err)
	}
}

// ExampleSOLTerminal_customStreams demonstrates using custom I/O streams.
func ExampleSOLTerminal_customStreams() {
	// This example shows how to use custom stdin/stdout (useful for testing)

	ctx := context.Background()
	serverID := "server-001"

	// Create client and session (omitted for brevity)
	var client *client.Client
	session, _ := client.CreateSOLSession(ctx, serverID)
	stream, _ := client.StreamConsoleData(ctx, serverID, session.ID)

	// Create terminal handler
	term := terminal.NewSOLTerminal(stream, session.ID)

	// Customize I/O streams before starting
	// term.stdin = customStdin   // Not exported - use default os.Stdin/Stdout
	// term.stdout = customStdout

	defer term.Close()

	// Start streaming
	_ = term.Start(ctx)
}

// ExampleSOLTerminal_contextCancellation demonstrates graceful shutdown via context.
func ExampleSOLTerminal_contextCancellation() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverID := "server-001"

	// Create client and session (omitted for brevity)
	var client *client.Client
	session, _ := client.CreateSOLSession(ctx, serverID)
	stream, _ := client.StreamConsoleData(ctx, serverID, session.ID)

	// Create terminal handler
	term := terminal.NewSOLTerminal(stream, session.ID)
	defer term.Close()

	// Simulate external cancellation after some work
	go func() {
		// In real code, this might be triggered by a signal or timeout
		// time.Sleep(30 * time.Second)
		// cancel()
	}()

	// Start streaming - will exit when context is cancelled
	err := term.Start(ctx)
	if err == context.Canceled {
		fmt.Println("Session cancelled by context")
	}
}
