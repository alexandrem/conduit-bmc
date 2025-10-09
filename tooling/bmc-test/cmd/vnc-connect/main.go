package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"local-agent/pkg/vnc"
)

var (
	// Connection flags
	host        string
	port        int
	password    string
	tlsEnabled  bool
	tlsInsecure bool
	timeout     time.Duration

	// Logging flags
	verbose bool
	debug   bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "vnc-connect",
	Short: "VNC test client for BMC VNC connections",
	Long: `VNC Test Client - Swiss Army Knife for BMC VNC Testing

A specialized utility for testing VNC connections to BMC (Baseboard Management Controller)
systems such as Dell iDRAC, HP iLO, and Supermicro IPMI. Supports both standard VNC and
TLS-encrypted VNC connections with comprehensive diagnostics.

Features:
  • Standard VNC (RFB 3.3, 3.7, 3.8) protocol support
  • TLS/SSL encrypted connections for secure BMC access
  • VNC Authentication (password-based)
  • Connection diagnostics and testing
  • Debug logging for troubleshooting`,
	Example: `  # Test Dell iDRAC VNC with TLS:
  vnc-connect --host 10.147.8.25 --port 5901 --password secret --tls --debug

  # Test standard VNC without TLS:
  vnc-connect --host 192.168.1.100 --port 5900 --password secret

  # Test VNC without password (no authentication):
  vnc-connect --host 192.168.1.100 --port 5900

  # Test with verbose logging and custom timeout:
  vnc-connect --host 10.0.0.100 --port 5901 --timeout 60s --verbose`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Validate required flags
		if host == "" {
			return fmt.Errorf("host is required")
		}
		return nil
	},
	RunE:          runVNCConnect,
	SilenceUsage:  true,  // Don't show usage on errors from RunE
	SilenceErrors: false, // Still print errors, just not usage
}

func init() {
	// Connection flags
	rootCmd.Flags().StringVar(&host, "host", "", "VNC server hostname or IP address (required)")
	rootCmd.Flags().IntVar(&port, "port", 5901, "VNC server port")
	rootCmd.Flags().StringVar(&password, "password", "", "VNC password (omit for no authentication)")
	rootCmd.Flags().BoolVar(&tlsEnabled, "tls", false, "Enable TLS encryption")
	rootCmd.Flags().BoolVar(&tlsInsecure, "tls-insecure", true, "Skip TLS certificate verification (only with --tls)")
	rootCmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Connection timeout")

	// Logging flags
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging (info level)")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging (most detailed)")

	// Mark required flags
	_ = rootCmd.MarkFlagRequired("host")
}

func runVNCConnect(cmd *cobra.Command, args []string) error {
	// Setup logging
	setupLogging(verbose, debug)

	log.Info().
		Str("host", host).
		Int("port", port).
		Bool("tls", tlsEnabled).
		Bool("has_password", password != "").
		Dur("timeout", timeout).
		Msg("VNC Test Client starting")

	// Create VNC endpoint configuration
	endpoint := &vnc.Endpoint{
		Endpoint: fmt.Sprintf("%s:%d", host, port),
		Password: password,
	}

	// Add TLS configuration if enabled
	if tlsEnabled {
		endpoint.TLS = &vnc.TLSConfig{
			Enabled:            true,
			InsecureSkipVerify: tlsInsecure,
		}
		log.Info().
			Bool("insecure_skip_verify", tlsInsecure).
			Msg("TLS enabled for VNC connection")
	}

	// Create VNC transport
	log.Info().Msg("Creating VNC transport...")
	transport := vnc.NewNativeTransport(timeout)

	// Connect and authenticate
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	log.Info().Msg("Connecting to VNC server...")
	if err := vnc.ConnectTransport(ctx, transport, endpoint); err != nil {
		log.Error().
			Err(err).
			Msg("Failed to connect to VNC server")
		return fmt.Errorf("VNC connection failed: %w", err)
	}
	defer transport.Close()

	log.Info().Msg("✅ VNC connection successful!")
	log.Info().Msg("✅ RFB handshake completed")
	log.Info().Msg("✅ Authentication successful")

	// Step 5: Send FramebufferUpdateRequest to trigger server to send screen data
	//
	// CRITICAL: VNC servers are passive after authentication - they don't send
	// framebuffer data until the client explicitly requests it.
	//
	// FramebufferUpdateRequest message format (10 bytes):
	//   - Message type: 3 (1 byte)
	//   - Incremental: 0=full update, 1=incremental (1 byte)
	//   - X position: big-endian u16 (2 bytes)
	//   - Y position: big-endian u16 (2 bytes)
	//   - Width: big-endian u16 (2 bytes)
	//   - Height: big-endian u16 (2 bytes)
	//
	// Reference: RFC 6143 Section 7.5.3
	log.Info().Msg("Requesting framebuffer update...")

	fbUpdateReq := []byte{
		3,    // Message type: FramebufferUpdateRequest
		1,    // Incremental: 1 (only send changes, more efficient)
		0, 0, // X position: 0 (big-endian u16)
		0, 0, // Y position: 0 (big-endian u16)
		0xFF, 0xFF, // Width: 65535 (request entire screen width)
		0xFF, 0xFF, // Height: 65535 (request entire screen height)
	}

	if err := transport.Write(context.Background(), fbUpdateReq); err != nil {
		log.Error().
			Err(err).
			Msg("Failed to send FramebufferUpdateRequest")
		return fmt.Errorf("FramebufferUpdateRequest failed: %w", err)
	}

	log.Info().Msg("✅ FramebufferUpdateRequest sent")

	// Now read the server's response (should be a FramebufferUpdate message)
	log.Info().Msg("Testing data transfer (waiting for FramebufferUpdate)...")
	readCtx, readCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readCancel()

	data, err := transport.Read(readCtx)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Read test failed - server did not send FramebufferUpdate")
		return fmt.Errorf("failed to read FramebufferUpdate: %w", err)
	}

	log.Info().
		Int("bytes_received", len(data)).
		Str("data_hex", fmt.Sprintf("%x", data[:min(16, len(data))])).
		Msg("✅ Data transfer working - received FramebufferUpdate")

	// Summary
	fmt.Println("\n" + repeat("=", 60))
	fmt.Println("VNC Test Results")
	fmt.Println(repeat("=", 60))
	fmt.Printf("Host:           %s:%d\n", host, port)
	fmt.Printf("TLS:            %v\n", tlsEnabled)
	fmt.Printf("Authentication: %s\n", authStatus(password))
	fmt.Printf("Status:         ✅ SUCCESS\n")
	fmt.Println(repeat("=", 60))

	log.Info().Msg("VNC test completed successfully")
	return nil
}

func setupLogging(verbose, debug bool) {
	// Setup zerolog
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "3:04PM"})

	// Set log level
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else if verbose {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	}
}

func authStatus(password string) string {
	if password == "" {
		return "None"
	}
	return "VNC Authentication (password provided)"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// String repeat helper
type stringRepeat string

func (s stringRepeat) repeat(count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += string(s)
	}
	return result
}

func repeat(s string, count int) string {
	return stringRepeat(s).repeat(count)
}
