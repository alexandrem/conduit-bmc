package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"cli/pkg/client"
)

var powerCmd = &cobra.Command{
	Use:   "power",
	Short: "Server power management commands",
	Long:  "Commands for controlling server power state through BMC",
}

var powerOnCmd = &cobra.Command{
	Use:   "on <server-id>",
	Short: "Power on a server",
	Long:  "Power on the specified server through its BMC interface",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverID := args[0]

		client := client.New(GetConfig())
		ctx := context.Background()

		fmt.Printf("Powering on server %s...\n", serverID)

		if err := client.PowerOn(ctx, serverID); err != nil {
			return fmt.Errorf("failed to power on server: %w", err)
		}

		fmt.Printf("Server %s powered on successfully\n", serverID)
		return nil
	},
}

var powerOffCmd = &cobra.Command{
	Use:   "off <server-id>",
	Short: "Power off a server",
	Long:  "Power off the specified server through its BMC interface",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverID := args[0]

		client := client.New(GetConfig())
		ctx := context.Background()

		fmt.Printf("Powering off server %s...\n", serverID)

		if err := client.PowerOff(ctx, serverID); err != nil {
			return fmt.Errorf("failed to power off server: %w", err)
		}

		fmt.Printf("Server %s powered off successfully\n", serverID)
		return nil
	},
}

var powerCycleCmd = &cobra.Command{
	Use:   "cycle <server-id>",
	Short: "Power cycle a server",
	Long:  "Power cycle the specified server (power off then on) through its BMC interface",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverID := args[0]

		client := client.New(GetConfig())
		ctx := context.Background()

		fmt.Printf("Power cycling server %s...\n", serverID)

		if err := client.PowerCycle(ctx, serverID); err != nil {
			return fmt.Errorf("failed to power cycle server: %w", err)
		}

		fmt.Printf("Server %s power cycled successfully\n", serverID)
		return nil
	},
}

var powerStatusCmd = &cobra.Command{
	Use:   "status <server-id>",
	Short: "Get server power status",
	Long:  "Get the current power status of the specified server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverID := args[0]

		client := client.New(GetConfig())
		ctx := context.Background()

		status, err := client.GetPowerStatus(ctx, serverID)
		if err != nil {
			return fmt.Errorf("failed to get power status: %w", err)
		}

		fmt.Printf("Server %s power status: %s\n", serverID, status)
		return nil
	},
}

var resetCmd = &cobra.Command{
	Use:   "reset <server-id>",
	Short: "Reset a server",
	Long:  "Perform a hard reset on the specified server through its BMC interface",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverID := args[0]

		client := client.New(GetConfig())
		ctx := context.Background()

		fmt.Printf("Resetting server %s...\n", serverID)

		if err := client.Reset(ctx, serverID); err != nil {
			return fmt.Errorf("failed to reset server: %w", err)
		}

		fmt.Printf("Server %s reset successfully\n", serverID)
		return nil
	},
}

func init() {
	serverCmd.AddCommand(powerCmd)
	serverCmd.AddCommand(resetCmd)

	powerCmd.AddCommand(powerOnCmd)
	powerCmd.AddCommand(powerOffCmd)
	powerCmd.AddCommand(powerCycleCmd)
	powerCmd.AddCommand(powerStatusCmd)
}
