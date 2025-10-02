package cmd

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"cli/pkg/client"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
}

var loginPassword string

var loginCmd = &cobra.Command{
	Use:   "login [email]",
	Short: "Authenticate with BMC Manager",
	Long: `Authenticate with the BMC Manager using email and password.
This will obtain delegated tokens for accessing Regional Gateways.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Use global configuration loaded by PersistentPreRunE
		cfg := GetConfig()

		// Get email
		var email string
		if len(args) > 0 {
			email = args[0]
		} else {
			fmt.Print("Email: ")
			fmt.Scanln(&email)
		}

		// Get password
		var password string
		if loginPassword != "" {
			password = loginPassword
		} else {
			fmt.Print("Password: ")
			passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}
			password = string(passwordBytes)
			fmt.Println() // New line after password input
		}

		// Create client and authenticate
		bmcClient := client.New(cfg)
		err := bmcClient.Authenticate(ctx, email, password)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// Save updated config with tokens
		err = cfg.Save()
		if err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println("Authentication successful! Tokens saved to config.")
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Use global configuration loaded by PersistentPreRunE
		cfg := GetConfig()

		if cfg.Auth.AccessToken == "" {
			fmt.Println("Not authenticated. Run 'bmc-cli auth login' to authenticate.")
			return nil
		}

		fmt.Printf("Authenticated as: %s\n", cfg.Auth.Email)
		fmt.Printf("Access token expires: %s\n", cfg.Auth.TokenExpiresAt.Format("2006-01-02 15:04:05"))

		// Check if token is expired or expires soon
		now := time.Now()
		if now.After(cfg.Auth.TokenExpiresAt) {
			fmt.Println("Status: ❌ Access token is expired")
		} else if time.Until(cfg.Auth.TokenExpiresAt) < 5*time.Minute {
			fmt.Printf("Status: ⚠️  Access token expires in %v\n", time.Until(cfg.Auth.TokenExpiresAt).Round(time.Second))
		} else {
			fmt.Printf("Status: ✅ Access token valid for %v\n", time.Until(cfg.Auth.TokenExpiresAt).Round(time.Second))
		}

		return nil
	},
}

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh access token",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Use global configuration loaded by PersistentPreRunE
		cfg := GetConfig()

		if cfg.Auth.RefreshToken == "" {
			return fmt.Errorf("no refresh token found. Please login again with 'bmc-cli auth login'")
		}

		// Access the manager client to refresh token
		// This is a simplified approach - in a real implementation,
		// you might want to expose this method on the main client
		fmt.Println("Refreshing access token...")

		// For now, suggest re-login
		fmt.Println("Token refresh not yet implemented. Please use 'bmc-cli auth login' to re-authenticate.")

		return nil
	},
}

func init() {
	loginCmd.Flags().StringVar(&loginPassword, "password", "", "Password for authentication (for non-interactive use)")
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(statusCmd)
	authCmd.AddCommand(refreshCmd)
	rootCmd.AddCommand(authCmd)
}
