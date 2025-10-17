package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	gatewayv1 "gateway/gen/gateway/v1"

	"cli/pkg/client"
	"cli/pkg/output"
)

var infoCmd = &cobra.Command{
	Use:   "info <server-id>",
	Short: "Show BMC hardware information for a server",
	Long:  "Display detailed BMC hardware information including firmware version, manufacturer, and capabilities",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverID := args[0]

		client := client.New(GetConfig())
		ctx := context.Background()

		bmcInfo, err := client.GetBMCInfo(ctx, serverID)
		if err != nil {
			return fmt.Errorf("failed to get BMC info: %w", err)
		}

		// Get output format
		format, err := output.GetFormatFromCmd(cmd)
		if err != nil {
			return err
		}

		formatter := output.New(format)

		// Handle JSON output format
		if formatter.IsJSON() {
			return outputInfoJSON(formatter, serverID, bmcInfo)
		}

		// Default text output
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		fmt.Fprintf(w, "Server ID:\t%s\n", serverID)
		fmt.Fprintf(w, "BMC Type:\t%s\n", bmcInfo.BmcType)
		fmt.Fprintln(w)

		// Display type-specific information
		switch details := bmcInfo.Details.(type) {
		case *gatewayv1.BMCInfo_IpmiInfo:
			ipmi := details.IpmiInfo
			fmt.Fprintf(w, "IPMI Information:\n")
			fmt.Fprintf(w, "  Device ID:\t%s\n", ipmi.DeviceId)
			fmt.Fprintf(w, "  Device Revision:\t%s\n", ipmi.DeviceRevision)
			fmt.Fprintf(w, "  Firmware Revision:\t%s\n", ipmi.FirmwareRevision)
			fmt.Fprintf(w, "  IPMI Version:\t%s\n", ipmi.IpmiVersion)
			fmt.Fprintf(w, "  Manufacturer ID:\t%s\n", ipmi.ManufacturerId)
			if ipmi.ManufacturerName != "" {
				fmt.Fprintf(w, "  Manufacturer Name:\t%s\n", ipmi.ManufacturerName)
			}
			fmt.Fprintf(w, "  Product ID:\t%s\n", ipmi.ProductId)
			fmt.Fprintf(w, "  Device Available:\t%t\n", ipmi.DeviceAvailable)
			fmt.Fprintf(w, "  Provides Device SDRs:\t%t\n", ipmi.ProvidesDeviceSdrs)
			if len(ipmi.AdditionalDeviceSupport) > 0 {
				fmt.Fprintf(w, "  Additional Device Support:\n")
				for _, support := range ipmi.AdditionalDeviceSupport {
					fmt.Fprintf(w, "    - %s\n", support)
				}
			}

		case *gatewayv1.BMCInfo_RedfishInfo:
			redfish := details.RedfishInfo
			fmt.Fprintf(w, "Redfish Information:\n")
			fmt.Fprintf(w, "  Manager ID:\t%s\n", redfish.ManagerId)
			fmt.Fprintf(w, "  Name:\t%s\n", redfish.Name)
			if redfish.Model != "" {
				fmt.Fprintf(w, "  Model:\t%s\n", redfish.Model)
			}
			if redfish.Manufacturer != "" {
				fmt.Fprintf(w, "  Manufacturer:\t%s\n", redfish.Manufacturer)
			}
			if redfish.FirmwareVersion != "" {
				fmt.Fprintf(w, "  Firmware Version:\t%s\n", redfish.FirmwareVersion)
			}
			fmt.Fprintf(w, "  Status:\t%s\n", redfish.Status)
			if redfish.PowerState != "" {
				fmt.Fprintf(w, "  Power State:\t%s\n", redfish.PowerState)
			}
			if len(redfish.NetworkProtocols) > 0 {
				fmt.Fprintf(w, "  Network Protocols:\n")
				for _, proto := range redfish.NetworkProtocols {
					fmt.Fprintf(w, "    - %s (port %d)\n", proto.Name, proto.Port)
				}
			}

			// Display System Status if available (RFD 020)
			if sys := redfish.SystemStatus; sys != nil {
				fmt.Fprintln(w)
				fmt.Fprintf(w, "System Status:\n")
				if sys.SystemId != "" {
					fmt.Fprintf(w, "  System ID:\t%s\n", sys.SystemId)
				}
				if sys.Hostname != "" {
					fmt.Fprintf(w, "  Hostname:\t%s\n", sys.Hostname)
				}
				if sys.SerialNumber != "" {
					fmt.Fprintf(w, "  Serial Number:\t%s\n", sys.SerialNumber)
				}
				if sys.Sku != "" {
					fmt.Fprintf(w, "  SKU:\t%s\n", sys.Sku)
				}
				if sys.BiosVersion != "" {
					fmt.Fprintf(w, "  BIOS Version:\t%s\n", sys.BiosVersion)
				}
				if sys.LastResetTime != "" {
					fmt.Fprintf(w, "  Last Reset:\t%s\n", sys.LastResetTime)
				}
				if sys.BootProgress != "" {
					bootIndicator := getBootProgressIndicator(sys.BootProgress)
					if sys.BootProgress == "OEM" && sys.BootProgressOem != "" {
						fmt.Fprintf(w, "  Boot Progress:\t%s - %s %s\n", sys.BootProgress, sys.BootProgressOem, bootIndicator)
					} else {
						fmt.Fprintf(w, "  Boot Progress:\t%s %s\n", sys.BootProgress, bootIndicator)
					}
				}
				if sys.PostState != "" {
					fmt.Fprintf(w, "  POST State:\t%s\n", sys.PostState)
				}
				if boot := sys.BootSource; boot != nil {
					bootStr := fmt.Sprintf("%s (%s, %s)", boot.Target, boot.Mode, boot.Enabled)
					fmt.Fprintf(w, "  Boot Source:\t%s\n", bootStr)
				}
				if len(sys.BootOrder) > 0 {
					fmt.Fprintf(w, "  Boot Order:\t%s", sys.BootOrder[0])
					for i := 1; i < len(sys.BootOrder) && i < 3; i++ {
						fmt.Fprintf(w, ", %s", sys.BootOrder[i])
					}
					if len(sys.BootOrder) > 3 {
						fmt.Fprintf(w, " (%d more)", len(sys.BootOrder)-3)
					}
					fmt.Fprintln(w)
				}
				if len(sys.OemHealth) > 0 {
					healthStr := formatHealthStatus(sys.OemHealth)
					fmt.Fprintf(w, "  Health Status:\t%s\n", healthStr)
				}
				// Add contextual hints
				if hint := getBootStatusHint(sys); hint != "" {
					fmt.Fprintln(w)
					fmt.Fprintf(w, "  Note: %s\n", hint)
				}
			}
		}

		w.Flush()

		return nil
	},
}

func outputInfoJSON(formatter *output.Formatter, serverID string, bmcInfo *gatewayv1.BMCInfo) error {
	// Create a JSON-friendly structure
	data := map[string]interface{}{
		"server_id": serverID,
		"bmc_type":  bmcInfo.BmcType,
	}

	// Add protocol-specific details
	switch details := bmcInfo.Details.(type) {
	case *gatewayv1.BMCInfo_IpmiInfo:
		ipmi := details.IpmiInfo
		data["ipmi_info"] = map[string]interface{}{
			"device_id":                 ipmi.DeviceId,
			"device_revision":           ipmi.DeviceRevision,
			"firmware_revision":         ipmi.FirmwareRevision,
			"ipmi_version":              ipmi.IpmiVersion,
			"manufacturer_id":           ipmi.ManufacturerId,
			"manufacturer_name":         ipmi.ManufacturerName,
			"product_id":                ipmi.ProductId,
			"device_available":          ipmi.DeviceAvailable,
			"provides_device_sdrs":      ipmi.ProvidesDeviceSdrs,
			"additional_device_support": ipmi.AdditionalDeviceSupport,
		}

	case *gatewayv1.BMCInfo_RedfishInfo:
		redfish := details.RedfishInfo
		protocols := make([]map[string]interface{}, 0, len(redfish.NetworkProtocols))
		for _, proto := range redfish.NetworkProtocols {
			protocols = append(protocols, map[string]interface{}{
				"name":    proto.Name,
				"port":    proto.Port,
				"enabled": proto.Enabled,
			})
		}
		redfishData := map[string]interface{}{
			"manager_id":        redfish.ManagerId,
			"name":              redfish.Name,
			"model":             redfish.Model,
			"manufacturer":      redfish.Manufacturer,
			"firmware_version":  redfish.FirmwareVersion,
			"status":            redfish.Status,
			"power_state":       redfish.PowerState,
			"network_protocols": protocols,
		}

		// Add system status if available (RFD 020)
		if sys := redfish.SystemStatus; sys != nil {
			systemStatus := map[string]interface{}{
				"system_id":         sys.SystemId,
				"hostname":          sys.Hostname,
				"serial_number":     sys.SerialNumber,
				"sku":               sys.Sku,
				"bios_version":      sys.BiosVersion,
				"last_reset_time":   sys.LastResetTime,
				"boot_progress":     sys.BootProgress,
				"boot_progress_oem": sys.BootProgressOem,
				"post_state":        sys.PostState,
				"boot_order":        sys.BootOrder,
				"oem_health":        sys.OemHealth,
			}
			if boot := sys.BootSource; boot != nil {
				systemStatus["boot_source"] = map[string]interface{}{
					"target":  boot.Target,
					"enabled": boot.Enabled,
					"mode":    boot.Mode,
				}
			}
			redfishData["system_status"] = systemStatus
		}

		data["redfish_info"] = redfishData
	}

	return formatter.Output(data)
}

// getBootProgressIndicator returns a visual indicator for boot progress state
func getBootProgressIndicator(bootProgress string) string {
	switch bootProgress {
	case "OSRunning":
		return "\u2713" // ✓
	case "SetupEntered", "OEM":
		return "\u26A0" // ⚠
	case "None", "":
		return "\u2717" // ✗
	default:
		return ""
	}
}

// formatHealthStatus formats OEM health status map into a readable string
func formatHealthStatus(health map[string]string) string {
	if len(health) == 0 {
		return "N/A"
	}

	// Check system health rollup first
	if systemHealth, ok := health["system_health_rollup_status"]; ok && systemHealth != "" {
		result := systemHealth
		// Add key subsystem status
		details := []string{}
		if cpu, ok := health["cpu_rollup_status"]; ok && cpu != "" {
			details = append(details, fmt.Sprintf("CPU: %s", cpu))
		}
		if storage, ok := health["storage_rollup_status"]; ok && storage != "" {
			details = append(details, fmt.Sprintf("Storage: %s", storage))
		}
		if temp, ok := health["temp_rollup_status"]; ok && temp != "" {
			details = append(details, fmt.Sprintf("Temp: %s", temp))
		}
		if fans, ok := health["fan_rollup_status"]; ok && fans != "" {
			details = append(details, fmt.Sprintf("Fans: %s", fans))
		}
		if len(details) > 0 {
			result = fmt.Sprintf("%s (%s)", result, joinStrings(details, ", "))
		}
		return result
	}

	return "N/A"
}

// getBootStatusHint provides contextual hints based on boot status
func getBootStatusHint(sys *gatewayv1.SystemStatus) string {
	if sys.BootProgress == "SetupEntered" {
		return "Server is in BIOS Setup mode. Use VNC console for graphical access."
	}
	if sys.BootProgress == "OEM" && sys.BootProgressOem != "" {
		if sys.BootProgressOem == "No bootable devices." {
			return "No bootable devices detected. Check boot configuration or use VNC to access BIOS setup."
		}
	}
	if sys.BootProgress == "MemoryInitializationStarted" || sys.BootProgress == "PrimaryProcessorInitializationStarted" {
		return "Server in early POST - VGA output only. Use VNC for BIOS messages."
	}
	return ""
}

// joinStrings joins string slices (helper to avoid importing strings)
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}

func init() {
	serverCmd.AddCommand(infoCmd)
	output.AddFormatFlag(infoCmd)
}
