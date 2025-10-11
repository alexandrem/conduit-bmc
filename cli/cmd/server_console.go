package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"cli/pkg/client"
	"cli/pkg/terminal"
)

var consoleCmd = &cobra.Command{
	Use:   "console <server-id>",
	Short: "Open server console (SOL)",
	Long: `Open a Serial Over LAN (SOL) console connection to the specified server.

By default, this opens a web-based console viewer in your browser.
Use --terminal flag for direct terminal streaming (advanced).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverID := args[0]
		terminalMode, _ := cmd.Flags().GetBool("terminal")
		rawMode, _ := cmd.Flags().GetBool("raw")

		client := client.New(GetConfig())
		ctx := context.Background()

		if terminalMode {
			// Terminal streaming mode - direct to CLI terminal
			return openSOLConsole(ctx, client, serverID, rawMode)
		} else {
			// Web console mode (default) - redirect to gateway
			return openWebConsole(ctx, client, serverID)
		}
	},
}

func openBrowser(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
	return err
}

var vncCmd = &cobra.Command{
	Use:   "vnc <server-id>",
	Short: "Open VNC console viewer",
	Long: `Open a web-based VNC console viewer for the specified server.

This creates a VNC session with the gateway and opens the VNC viewer
directly in your web browser for remote graphical console access.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverID := args[0]

		client := client.New(GetConfig())
		ctx := context.Background()

		fmt.Printf("Creating VNC session for server %s...\n", serverID)

		// Create VNC session
		session, err := client.CreateVNCSession(ctx, serverID)
		if err != nil {
			return fmt.Errorf("failed to create VNC session: %w", err)
		}

		fmt.Printf("VNC session created: %s\n", session.ID)
		fmt.Printf("Session expires: %s\n", session.ExpiresAt)
		fmt.Printf("Opening VNC viewer: %s\n", session.ViewerURL)

		// Open VNC viewer in browser
		if err := openBrowser(session.ViewerURL); err != nil {
			fmt.Printf("Failed to open browser automatically. Please navigate to: %s\n", session.ViewerURL)
		}

		fmt.Println("VNC session is ready!")
		return nil
	},
}

func openWebConsole(ctx context.Context, client *client.Client, serverID string) error {
	fmt.Printf("Creating web console session for server %s...\n", serverID)

	// Create SOL session for web console
	session, err := client.CreateSOLSession(ctx, serverID)
	if err != nil {
		return fmt.Errorf("failed to create web console session: %w", err)
	}

	fmt.Printf("Web console session created: %s\n", session.ID)
	fmt.Printf("Session expires: %s\n", session.ExpiresAt)
	fmt.Printf("Opening web console: %s\n", session.ConsoleURL)

	// Open web console in browser
	if err := openBrowser(session.ConsoleURL); err != nil {
		fmt.Printf("Failed to open browser automatically. Please navigate to: %s\n", session.ConsoleURL)
	}

	fmt.Println("Web console is ready!")
	return nil
}

func openSOLConsole(ctx context.Context, client *client.Client, serverID string, rawMode bool) error {
	fmt.Fprintf(os.Stderr, "Opening SOL console for server %s...\n", serverID)

	// Create SOL session
	session, err := client.CreateSOLSession(ctx, serverID)
	if err != nil {
		return fmt.Errorf("failed to create SOL session: %w", err)
	}

	fmt.Fprintf(os.Stderr, "SOL session created: %s\n", session.ID)
	fmt.Fprintf(os.Stderr, "Connecting to console...\n\n")

	// Open Connect bidirectional stream
	// Note: StreamConsoleData signature is (ctx, serverID, sessionID)
	stream, err := client.StreamConsoleData(ctx, serverID, session.ID)
	if err != nil {
		return fmt.Errorf("failed to open console stream: %w", err)
	}

	// Create terminal handler with Connect stream
	solTerminal := terminal.NewSOLTerminal(stream, session.ID)
	solTerminal.SetRawMode(rawMode)
	defer solTerminal.Close()

	// Start streaming
	if err := solTerminal.Start(ctx); err != nil {
		return fmt.Errorf("console session error: %w", err)
	}

	return nil
}

func init() {
	// Add --terminal flag to console command
	consoleCmd.Flags().Bool("terminal", false, "Use direct terminal streaming instead of web console (advanced)")
	// Add --raw flag for preserving terminal control sequences
	consoleCmd.Flags().Bool("raw", false, "Preserve terminal control sequences (allows overwriting lines). Default is append-only mode.")

	serverCmd.AddCommand(consoleCmd)
	serverCmd.AddCommand(vncCmd)
}
