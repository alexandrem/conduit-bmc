package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"local-agent/pkg/sol"
)

var (
	// Connection flags
	host     string
	port     int
	username string
	password string
	timeout  time.Duration
	solType  string

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
	Use:   "sol-connect",
	Short: "SOL test client for BMC Serial-over-LAN connections",
	Long: `SOL Test Client - Swiss Army Knife for BMC Serial Console Testing

A specialized utility for testing Serial-over-LAN (SOL) connections to BMC (Baseboard
Management Controller) systems such as Dell iDRAC, HP iLO, and Supermicro IPMI.
Supports both IPMI SOL and Redfish serial console with comprehensive diagnostics.

Features:
  • IPMI SOL (using FreeIPMI's ipmiconsole)
  • Redfish Serial Console (WebSocket-based)
  • Connection diagnostics and testing
  • Debug logging for troubleshooting
  • Interactive or one-shot testing modes`,
	Example: `  # Test Dell iDRAC IPMI SOL:
  sol-connect --host 10.147.8.25 --port 623 --username admin --password secret --type ipmi --debug

  # Test Redfish serial console:
  sol-connect --host 10.147.8.25 --username admin --password secret --type redfish --debug

  # Test with custom timeout:
  sol-connect --host 192.168.1.100 --username admin --password secret --timeout 60s --verbose`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Validate required flags
		if host == "" {
			return fmt.Errorf("host is required")
		}
		if username == "" {
			return fmt.Errorf("username is required")
		}
		if password == "" {
			return fmt.Errorf("password is required")
		}
		if solType != "ipmi" && solType != "redfish" {
			return fmt.Errorf("sol-type must be either 'ipmi' or 'redfish'")
		}
		return nil
	},
	RunE:          runSOLConnect,
	SilenceUsage:  true,  // Don't show usage on errors from RunE
	SilenceErrors: false, // Still print errors, just not usage
}

func init() {
	// Connection flags
	rootCmd.Flags().StringVar(&host, "host", "", "BMC hostname or IP address (required)")
	rootCmd.Flags().IntVar(&port, "port", 623, "IPMI port (default: 623)")
	rootCmd.Flags().StringVar(&username, "username", "", "BMC username (required)")
	rootCmd.Flags().StringVar(&password, "password", "", "BMC password (required)")
	rootCmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Connection timeout")
	rootCmd.Flags().StringVar(&solType, "type", "ipmi", "SOL type: 'ipmi' or 'redfish'")

	// Logging flags
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging (info level)")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging (most detailed)")

	// Mark required flags
	_ = rootCmd.MarkFlagRequired("host")
	_ = rootCmd.MarkFlagRequired("username")
	_ = rootCmd.MarkFlagRequired("password")
}

func runSOLConnect(cmd *cobra.Command, args []string) error {
	// Setup logging
	setupLogging(verbose, debug)

	log.Info().
		Str("host", host).
		Int("port", port).
		Str("username", username).
		Str("type", solType).
		Dur("timeout", timeout).
		Msg("SOL Test Client starting")

	// Build endpoint based on type
	var endpoint string
	var client sol.Client
	var err error

	if solType == "ipmi" {
		endpoint = fmt.Sprintf("%s:%d", host, port)
		log.Info().
			Str("endpoint", endpoint).
			Msg("Creating IPMI SOL client...")
		client, err = sol.NewClient("ipmi")
	} else {
		endpoint = fmt.Sprintf("https://%s", host)
		log.Info().
			Str("endpoint", endpoint).
			Msg("Creating Redfish serial console client...")
		client, err = sol.NewClient("redfish_serial")
	}

	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to create SOL client")
		return fmt.Errorf("failed to create SOL client: %w", err)
	}

	// Create SOL config
	config := sol.DefaultSOLConfig()
	config.InsecureSkipVerify = true // For self-signed certs

	// Connect and create session
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	log.Info().Msg("Connecting to BMC...")
	session, err := client.CreateSession(ctx, endpoint, username, password, config)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to create SOL session")
		return fmt.Errorf("SOL connection failed: %w", err)
	}
	defer session.Close()

	log.Info().Msg("✅ SOL connection successful!")
	log.Info().Msg("✅ Authentication successful")

	// Test reading data with timeout
	log.Info().Msg("Testing data transfer (waiting for console output)...")
	readCtx, readCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readCancel()

	data, err := session.Read(readCtx)
	if err != nil {
		// This might timeout if there's no console output, which is OK
		log.Warn().
			Err(err).
			Msg("No console data received (this is normal if server is idle)")
	} else {
		dataStr := string(data)

		// Check for connection close messages
		if detectConnectionClose(dataStr) {
			log.Error().
				Int("bytes_received", len(data)).
				Str("message", dataStr).
				Msg("❌ BMC is closing the connection!")

			fmt.Println("\n" + repeat("=", 60))
			fmt.Println("SOL Connection Error")
			fmt.Println(repeat("=", 60))
			fmt.Printf("Status:         ❌ FAILED - Connection Closed by BMC\n")
			fmt.Printf("Message:        %s\n", dataStr)
			fmt.Println(repeat("-", 60))
			fmt.Println("Possible causes:")
			fmt.Println("  1. SOL is disabled in BMC settings")
			fmt.Println("  2. User lacks Serial Console privileges")
			fmt.Println("  3. Another session is active (check BMC web interface)")
			fmt.Println("  4. BMC requires specific configuration")
			fmt.Println("  5. PTY is required but not available (IPMI only)")
			fmt.Println(repeat("=", 60))

			return fmt.Errorf("BMC closed connection: %s", dataStr)
		}

		log.Info().
			Int("bytes_received", len(data)).
			Str("data_preview", string(data[:min(50, len(data))])).
			Msg("✅ Data transfer working - received console data")
	}

	// Test writing data
	log.Info().Msg("Testing console input (sending newline)...")
	testData := []byte("\r\n")
	if err := session.Write(ctx, testData); err != nil {
		log.Error().
			Err(err).
			Msg("Failed to write to console")
		return fmt.Errorf("write test failed: %w", err)
	}
	log.Info().Msg("✅ Console input working")

	// Try to read response
	time.Sleep(1 * time.Second) // Give it a moment to respond
	readCtx2, readCancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer readCancel2()

	data, err = session.Read(readCtx2)
	if err != nil {
		log.Debug().
			Err(err).
			Msg("No immediate response to newline (normal)")
	} else if len(data) > 0 {
		log.Info().
			Int("bytes_received", len(data)).
			Str("response", string(data)).
			Msg("Received console response")
	}

	// Summary
	fmt.Println("\n" + repeat("=", 60))
	fmt.Println("SOL Test Results")
	fmt.Println(repeat("=", 60))
	fmt.Printf("Host:           %s:%d\n", host, port)
	fmt.Printf("Type:           %s\n", solType)
	fmt.Printf("Username:       %s\n", username)
	fmt.Printf("Status:         ✅ SUCCESS\n")
	fmt.Println(repeat("=", 60))
	fmt.Println("\nConnection is working! You can now use this endpoint for console access.")

	log.Info().Msg("SOL test completed successfully")
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

// detectConnectionClose checks if the data contains a connection close message
func detectConnectionClose(data string) bool {
	// Common patterns that indicate the BMC is closing the connection
	closePatterns := []string{
		"[closing the connection]",
		"closing the connection",
		"Connection closed",
		"session closed",
		"SOL session closed",
		"Console session terminated",
		"Exiting",
	}

	// Check for exact matches or contains (case-insensitive)
	dataLower := strings.ToLower(data)
	for _, pattern := range closePatterns {
		if strings.Contains(dataLower, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}
