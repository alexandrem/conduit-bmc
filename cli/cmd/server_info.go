package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"cli/pkg/client"
	"cli/pkg/output"
	"core/types"
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

// formatDiscoveryMethod converts discovery method enum to human-readable format
func formatDiscoveryMethod(method types.DiscoveryMethod) string {
	switch method {
	case types.DiscoveryMethodStaticConfig:
		return "Static Configuration"
	case types.DiscoveryMethodNetworkScan:
		return "Network Scan"
	case types.DiscoveryMethodAPIRegistration:
		return "API Registration"
	case types.DiscoveryMethodManual:
		return "Manual"
	default:
		return string(method)
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

		// Get output format
		format, err := output.GetFormatFromCmd(cmd)
		if err != nil {
			return err
		}

		formatter := output.New(format)

		// If JSON format, output raw data and return
		if formatter.IsJSON() {
			return formatter.Output(server)
		}

		// Check if verbose metadata display is requested
		showFullMetadata, _ := cmd.Flags().GetBool("metadata")

		// Text format - use improved formatting
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		// Server header with discovery info inline
		fmt.Fprintf(w, "Server ID:\t%s\n", server.ID)
		fmt.Fprintf(w, "Status:\t%s\n", server.Status)
		fmt.Fprintf(w, "Datacenter:\t%s\n", server.DatacenterID)

		// Show discovery summary inline
		if server.DiscoveryMetadata != nil {
			discoveryInfo := formatDiscoveryMethod(server.DiscoveryMetadata.DiscoveryMethod)
			if server.DiscoveryMetadata.DiscoverySource != "" {
				discoveryInfo += " (" + server.DiscoveryMetadata.DiscoverySource + ")"
			}
			fmt.Fprintf(w, "Discovered:\t%s", discoveryInfo)
			if !server.DiscoveryMetadata.DiscoveredAt.IsZero() {
				fmt.Fprintf(w, " at %s", server.DiscoveryMetadata.DiscoveredAt.Format("2006-01-02 15:04:05"))
			}
			fmt.Fprintf(w, "\n")
		}

		fmt.Fprintf(w, "Features:\t%v\n", server.Features)

		// Display BMC hardware information (merged with vendor)
		if server.ControlEndpoint != nil {
			fmt.Fprintf(w, "\nBMC Hardware:\n")

			// Show vendor info if available
			if server.DiscoveryMetadata != nil && server.DiscoveryMetadata.Vendor != nil {
				vendor := server.DiscoveryMetadata.Vendor.Manufacturer
				if server.DiscoveryMetadata.Vendor.Model != "" {
					vendor += " " + server.DiscoveryMetadata.Vendor.Model
				}
				fmt.Fprintf(w, "  Vendor:\t%s\n", vendor)
			}

			// Endpoint with protocol and auth method
			endpointDisplay := server.ControlEndpoint.Endpoint
			authMethod := "Basic Auth"
			if server.DiscoveryMetadata != nil && server.DiscoveryMetadata.Security != nil {
				authMethod = server.DiscoveryMetadata.Security.AuthMethod
			}
			fmt.Fprintf(w, "  Endpoint:\t%s (%s, %s)\n",
				endpointDisplay,
				formatBMCType(server.ControlEndpoint.Type),
				authMethod)

			fmt.Fprintf(w, "  Credentials:\t%s (configured)\n", server.ControlEndpoint.Username)
			fmt.Fprintf(w, "  Capabilities:\t%v\n", server.ControlEndpoint.Capabilities)
		}

		// Display operations available
		fmt.Fprintf(w, "\nOperations Available:\n")

		// Power operations
		hasPower := false
		for _, feature := range server.Features {
			if feature == "power" {
				hasPower = true
				break
			}
		}
		if hasPower && server.ControlEndpoint != nil {
			fmt.Fprintf(w, "  Power:\t✓ via %s\n", formatBMCType(server.ControlEndpoint.Type))
		}

		// Console operations
		if server.SOLEndpoint != nil {
			consoleType := formatSOLType(server.SOLEndpoint.Type)
			consoleInfo := fmt.Sprintf("✓ via %s (%s)", consoleType, server.SOLEndpoint.Endpoint)
			fmt.Fprintf(w, "  Console:\t%s\n", consoleInfo)
		}

		// VNC operations
		if server.VNCEndpoint != nil {
			vncType := formatVNCType(server.VNCEndpoint.Type)
			vncAuth := ""
			if server.DiscoveryMetadata != nil && server.DiscoveryMetadata.Security != nil &&
				server.DiscoveryMetadata.Security.VNCAuthType != "" {
				vncAuth = ", " + server.DiscoveryMetadata.Security.VNCAuthType + " auth"
			}
			vncInfo := fmt.Sprintf("✓ via %s (%s%s)", vncType, server.VNCEndpoint.Endpoint, vncAuth)
			fmt.Fprintf(w, "  VNC:\t%s\n", vncInfo)
		}

		// Sensors
		hasSensors := false
		for _, feature := range server.Features {
			if feature == "sensors" {
				hasSensors = true
				break
			}
		}
		if hasSensors && server.ControlEndpoint != nil {
			fmt.Fprintf(w, "  Sensors:\t✓ via %s\n", formatBMCType(server.ControlEndpoint.Type))
		}

		// Security section with warnings
		fmt.Fprintf(w, "\nSecurity:\n")
		if server.DiscoveryMetadata != nil && server.DiscoveryMetadata.Security != nil {
			security := server.DiscoveryMetadata.Security

			if security.TLSEnabled {
				verifyStatus := "enabled"
				if !security.TLSVerify {
					verifyStatus = "disabled (Warning: insecure)"
				}
				fmt.Fprintf(w, "  TLS:\tEnabled (Verify: %s)\n", verifyStatus)
			} else {
				fmt.Fprintf(w, "  TLS:\tDisabled (Warning: insecure connection)\n")
			}
		} else if server.ControlEndpoint != nil && server.ControlEndpoint.TLS != nil {
			if server.ControlEndpoint.TLS.Enabled {
				fmt.Fprintf(w, "  TLS:\tEnabled\n")
			} else {
				fmt.Fprintf(w, "  TLS:\tDisabled (Warning: insecure connection)\n")
			}
		} else {
			fmt.Fprintf(w, "  TLS:\tN/A\n")
		}

		// Display metadata if available
		if len(server.Metadata) > 0 {
			fmt.Fprintf(w, "\nMetadata:\n")
			for key, value := range server.Metadata {
				fmt.Fprintf(w, "  %s:\t%s\n", key, value)
			}
		}

		// Display full discovery metadata only if --metadata flag is set
		if showFullMetadata && server.DiscoveryMetadata != nil {
			fmt.Fprintf(w, "\nBMC Discovery Metadata:\n")
			fmt.Fprintf(w, "======================\n")

			// Discovery information
			fmt.Fprintf(w, "\nDiscovery Information:\n")
			if server.DiscoveryMetadata.DiscoveryMethod != "" {
				fmt.Fprintf(w, "  Method:\t%s\n", formatDiscoveryMethod(server.DiscoveryMetadata.DiscoveryMethod))
			}
			if server.DiscoveryMetadata.DiscoverySource != "" {
				fmt.Fprintf(w, "  Source:\t%s\n", server.DiscoveryMetadata.DiscoverySource)
			}
			if !server.DiscoveryMetadata.DiscoveredAt.IsZero() {
				fmt.Fprintf(w, "  Discovered:\t%s\n", server.DiscoveryMetadata.DiscoveredAt.Format("2006-01-02 15:04:05"))
			}
			if server.DiscoveryMetadata.ConfigSource != "" {
				fmt.Fprintf(w, "  Config File:\t%s\n", server.DiscoveryMetadata.ConfigSource)
			}

			// Vendor information
			if server.DiscoveryMetadata.Vendor != nil {
				fmt.Fprintf(w, "\nVendor Information:\n")
				if server.DiscoveryMetadata.Vendor.Manufacturer != "" {
					fmt.Fprintf(w, "  Manufacturer:\t%s\n", server.DiscoveryMetadata.Vendor.Manufacturer)
				}
				if server.DiscoveryMetadata.Vendor.Model != "" {
					fmt.Fprintf(w, "  Model:\t%s\n", server.DiscoveryMetadata.Vendor.Model)
				}
				if server.DiscoveryMetadata.Vendor.FirmwareVersion != "" {
					fmt.Fprintf(w, "  Firmware:\t%s\n", server.DiscoveryMetadata.Vendor.FirmwareVersion)
				}
				if server.DiscoveryMetadata.Vendor.BMCVersion != "" {
					fmt.Fprintf(w, "  BMC Version:\t%s\n", server.DiscoveryMetadata.Vendor.BMCVersion)
				}
			}

			// Protocol configuration
			if server.DiscoveryMetadata.Protocol != nil {
				fmt.Fprintf(w, "\nProtocol Configuration:\n")
				if server.DiscoveryMetadata.Protocol.PrimaryProtocol != "" {
					version := server.DiscoveryMetadata.Protocol.PrimaryVersion
					if version != "" {
						fmt.Fprintf(w, "  Primary:\t%s %s\n", server.DiscoveryMetadata.Protocol.PrimaryProtocol, version)
					} else {
						fmt.Fprintf(w, "  Primary:\t%s\n", server.DiscoveryMetadata.Protocol.PrimaryProtocol)
					}
				}
				if server.DiscoveryMetadata.Protocol.FallbackProtocol != "" {
					fmt.Fprintf(w, "  Fallback:\t%s\n", server.DiscoveryMetadata.Protocol.FallbackProtocol)
					if server.DiscoveryMetadata.Protocol.FallbackReason != "" {
						fmt.Fprintf(w, "  Fallback Reason:\t%s\n", server.DiscoveryMetadata.Protocol.FallbackReason)
					}
				}
				if server.DiscoveryMetadata.Protocol.ConsoleType != "" {
					fmt.Fprintf(w, "  Console Type:\t%s\n", server.DiscoveryMetadata.Protocol.ConsoleType)
				}
				if server.DiscoveryMetadata.Protocol.ConsolePath != "" {
					fmt.Fprintf(w, "  Console Path:\t%s\n", server.DiscoveryMetadata.Protocol.ConsolePath)
				}
				if server.DiscoveryMetadata.Protocol.VNCTransport != "" {
					fmt.Fprintf(w, "  VNC Transport:\t%s\n", server.DiscoveryMetadata.Protocol.VNCTransport)
				}
			}

			// Endpoint details
			if server.DiscoveryMetadata.Endpoints != nil {
				fmt.Fprintf(w, "\nEndpoint Details:\n")
				if server.DiscoveryMetadata.Endpoints.ControlEndpoint != "" {
					fmt.Fprintf(w, "  Control:\t%s", server.DiscoveryMetadata.Endpoints.ControlEndpoint)
					if server.DiscoveryMetadata.Endpoints.ControlScheme != "" {
						fmt.Fprintf(w, " (%s)", server.DiscoveryMetadata.Endpoints.ControlScheme)
					}
					fmt.Fprintf(w, "\n")
				}
				if server.DiscoveryMetadata.Endpoints.ConsoleEndpoint != "" {
					fmt.Fprintf(w, "  Console:\t%s\n", server.DiscoveryMetadata.Endpoints.ConsoleEndpoint)
				}
				if server.DiscoveryMetadata.Endpoints.VNCEndpoint != "" {
					fmt.Fprintf(w, "  VNC:\t%s\n", server.DiscoveryMetadata.Endpoints.VNCEndpoint)
				}
			}

			// Security configuration
			if server.DiscoveryMetadata.Security != nil {
				fmt.Fprintf(w, "\nSecurity Configuration:\n")
				fmt.Fprintf(w, "  TLS Enabled:\t%t\n", server.DiscoveryMetadata.Security.TLSEnabled)
				if server.DiscoveryMetadata.Security.TLSEnabled {
					fmt.Fprintf(w, "  TLS Verify:\t%t\n", server.DiscoveryMetadata.Security.TLSVerify)
				}
				if server.DiscoveryMetadata.Security.AuthMethod != "" {
					fmt.Fprintf(w, "  Auth Method:\t%s\n", server.DiscoveryMetadata.Security.AuthMethod)
				}
				if server.DiscoveryMetadata.Security.VNCAuthType != "" {
					fmt.Fprintf(w, "  VNC Auth:\t%s", server.DiscoveryMetadata.Security.VNCAuthType)
					if server.DiscoveryMetadata.Security.VNCPasswordLength > 0 {
						fmt.Fprintf(w, " (%d chars)", server.DiscoveryMetadata.Security.VNCPasswordLength)
					}
					fmt.Fprintf(w, "\n")
				}
			}

			// Network information
			if server.DiscoveryMetadata.Network != nil {
				fmt.Fprintf(w, "\nNetwork Information:\n")
				if server.DiscoveryMetadata.Network.IPAddress != "" {
					fmt.Fprintf(w, "  IP Address:\t%s\n", server.DiscoveryMetadata.Network.IPAddress)
				}
				if server.DiscoveryMetadata.Network.MACAddress != "" {
					fmt.Fprintf(w, "  MAC Address:\t%s\n", server.DiscoveryMetadata.Network.MACAddress)
				}
				fmt.Fprintf(w, "  Reachable:\t%t\n", server.DiscoveryMetadata.Network.Reachable)
				if server.DiscoveryMetadata.Network.LatencyMs > 0 {
					fmt.Fprintf(w, "  Latency:\t%dms\n", server.DiscoveryMetadata.Network.LatencyMs)
				}
			}

			// Capabilities
			if server.DiscoveryMetadata.Capabilities != nil {
				if len(server.DiscoveryMetadata.Capabilities.SupportedFeatures) > 0 {
					fmt.Fprintf(w, "\nCapabilities:\n")
					fmt.Fprintf(w, "  Supported:\t%v\n", server.DiscoveryMetadata.Capabilities.SupportedFeatures)
				}
				if len(server.DiscoveryMetadata.Capabilities.UnsupportedFeatures) > 0 {
					fmt.Fprintf(w, "  Unsupported:\t%v\n", server.DiscoveryMetadata.Capabilities.UnsupportedFeatures)
				}
				if len(server.DiscoveryMetadata.Capabilities.DiscoveryErrors) > 0 {
					fmt.Fprintf(w, "  Errors:\t%v\n", server.DiscoveryMetadata.Capabilities.DiscoveryErrors)
				}
				if len(server.DiscoveryMetadata.Capabilities.DiscoveryWarnings) > 0 {
					fmt.Fprintf(w, "  Warnings:\t%v\n", server.DiscoveryMetadata.Capabilities.DiscoveryWarnings)
				}
			}

			// Additional info
			if len(server.DiscoveryMetadata.AdditionalInfo) > 0 {
				fmt.Fprintf(w, "\nAdditional Information:\n")
				for key, value := range server.DiscoveryMetadata.AdditionalInfo {
					fmt.Fprintf(w, "  %s:\t%s\n", key, value)
				}
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

		// Get output format
		format, err := output.GetFormatFromCmd(cmd)
		if err != nil {
			return err
		}

		formatter := output.New(format)

		// If JSON format, output raw data and return
		if formatter.IsJSON() {
			return formatter.Output(servers)
		}

		// Text format - use existing tabwriter formatting
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

	// Add output format flag to commands
	output.AddFormatFlag(showCmd)
	output.AddFormatFlag(listCmd)

	// Add metadata flag to show full discovery metadata
	showCmd.Flags().Bool("metadata", false, "Show full discovery metadata details")
}
