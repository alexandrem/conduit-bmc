package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"cli/pkg/client"
)

// formatBMCType converts protobuf enum string to human-readable format
func formatBMCType(protoType string) string {
	switch protoType {
	case "BMC_IPMI":
		return "ipmi"
	case "BMC_REDFISH":
		return "redfish"
	default:
		// Handle lowercase already formatted values
		lower := strings.ToLower(protoType)
		if lower == "ipmi" || lower == "redfish" {
			return lower
		}
		return protoType
	}
}

// formatSOLType converts protobuf enum string to human-readable format
func formatSOLType(protoType string) string {
	switch protoType {
	case "SOL_IPMI":
		return "ipmi"
	case "SOL_REDFISH_SERIAL":
		return "redfish_serial"
	default:
		lower := strings.ToLower(protoType)
		if lower == "ipmi" || lower == "redfish_serial" {
			return lower
		}
		return protoType
	}
}

// formatVNCType converts protobuf enum string to human-readable format
func formatVNCType(protoType string) string {
	switch protoType {
	case "VNC_NATIVE":
		return "native"
	case "VNC_WEBSOCKET":
		return "websocket"
	default:
		lower := strings.ToLower(protoType)
		if lower == "native" || lower == "websocket" {
			return lower
		}
		return protoType
	}
}

var showCmd = &cobra.Command{
	Use:   "show <server-id>",
	Short: "Show server BMC information",
	Long:  "Display detailed information about a server's BMC interface and capabilities",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverID := args[0]

		client := client.New(GetConfig())
		ctx := context.Background()

		server, err := client.GetServer(ctx, serverID)
		if err != nil {
			return fmt.Errorf("failed to get server info: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Server ID:\t%s\n", server.ID)
		fmt.Fprintf(w, "Status:\t%s\n", server.Status)
		fmt.Fprintf(w, "Datacenter:\t%s\n", server.DatacenterID)
		fmt.Fprintf(w, "Features:\t%v\n", server.Features)

		// Display control endpoint information
		if server.ControlEndpoint != nil {
			fmt.Fprintf(w, "\nBMC Control API:\n")
			fmt.Fprintf(w, "  Type:\t%s\n", formatBMCType(server.ControlEndpoint.Type))
			fmt.Fprintf(w, "  Endpoint:\t%s\n", server.ControlEndpoint.Endpoint)
			fmt.Fprintf(w, "  Username:\t%s\n", server.ControlEndpoint.Username)
			fmt.Fprintf(w, "  Capabilities:\t%v\n", server.ControlEndpoint.Capabilities)
			if server.ControlEndpoint.TLS != nil {
				fmt.Fprintf(w, "  TLS Enabled:\t%t\n", server.ControlEndpoint.TLS.Enabled)
			}
		}

		// Display SOL endpoint information
		if server.SOLEndpoint != nil {
			fmt.Fprintf(w, "\nSerial Console (SOL):\n")
			fmt.Fprintf(w, "  Type:\t%s\n", formatSOLType(server.SOLEndpoint.Type))
			fmt.Fprintf(w, "  Endpoint:\t%s\n", server.SOLEndpoint.Endpoint)
			fmt.Fprintf(w, "  Username:\t%s\n", server.SOLEndpoint.Username)
			if server.SOLEndpoint.Config != nil {
				fmt.Fprintf(w, "  Baud Rate:\t%d\n", server.SOLEndpoint.Config.BaudRate)
				fmt.Fprintf(w, "  Timeout:\t%ds\n", server.SOLEndpoint.Config.TimeoutSeconds)
			}
		}

		// Display VNC endpoint information
		if server.VNCEndpoint != nil {
			fmt.Fprintf(w, "\nVNC Console:\n")
			fmt.Fprintf(w, "  Type:\t%s\n", formatVNCType(server.VNCEndpoint.Type))
			fmt.Fprintf(w, "  Endpoint:\t%s\n", server.VNCEndpoint.Endpoint)
			fmt.Fprintf(w, "  Username:\t%s\n", server.VNCEndpoint.Username)
			if server.VNCEndpoint.Config != nil {
				fmt.Fprintf(w, "  Protocol:\t%s\n", server.VNCEndpoint.Config.Protocol)
				if server.VNCEndpoint.Config.Path != "" {
					fmt.Fprintf(w, "  Path:\t%s\n", server.VNCEndpoint.Config.Path)
				}
				fmt.Fprintf(w, "  Display:\t%d\n", server.VNCEndpoint.Config.Display)
			}
		}

		// Display metadata if available
		if len(server.Metadata) > 0 {
			fmt.Fprintf(w, "\nMetadata:\n")
			for key, value := range server.Metadata {
				fmt.Fprintf(w, "  %s:\t%s\n", key, value)
			}
		}

		w.Flush()

		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all servers",
	Long:  "List all servers accessible to the current user",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := client.New(GetConfig())
		ctx := context.Background()

		servers, err := client.ListServers(ctx)
		if err != nil {
			return fmt.Errorf("failed to list servers: %w", err)
		}

		if len(servers) == 0 {
			fmt.Println("No servers found")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "SERVER ID\tBMC TYPE\tSTATUS\tDATACENTER\tCONSOLE\tVNC\n")
		for _, server := range servers {
			// Determine BMC type from control endpoint
			bmcType := "N/A"
			if server.ControlEndpoint != nil {
				bmcType = formatBMCType(server.ControlEndpoint.Type)
			}

			// Check console availability (SOL)
			consoleAvailable := "N/A"
			if server.SOLEndpoint != nil {
				consoleAvailable = "✓"
			}

			// Check VNC availability
			vncAvailable := "N/A"
			if server.VNCEndpoint != nil {
				vncAvailable = "✓"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				server.ID, bmcType, server.Status, server.DatacenterID, consoleAvailable, vncAvailable)
		}
		w.Flush()

		return nil
	},
}

func init() {
	serverCmd.AddCommand(showCmd)
	serverCmd.AddCommand(listCmd)
}
