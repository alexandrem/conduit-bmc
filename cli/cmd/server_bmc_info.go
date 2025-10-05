package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	gatewayv1 "gateway/gen/gateway/v1"

	"cli/pkg/client"
)

var (
	infoOutputFormat string
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

		// Handle JSON output format
		if infoOutputFormat == "json" {
			return outputJSON(serverID, bmcInfo)
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
		}

		w.Flush()

		return nil
	},
}

func outputJSON(serverID string, bmcInfo *gatewayv1.BMCInfo) error {
	// Create a JSON-friendly structure
	output := map[string]interface{}{
		"server_id": serverID,
		"bmc_type":  bmcInfo.BmcType,
	}

	// Add protocol-specific details
	switch details := bmcInfo.Details.(type) {
	case *gatewayv1.BMCInfo_IpmiInfo:
		ipmi := details.IpmiInfo
		output["ipmi_info"] = map[string]interface{}{
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
		output["redfish_info"] = map[string]interface{}{
			"manager_id":        redfish.ManagerId,
			"name":              redfish.Name,
			"model":             redfish.Model,
			"manufacturer":      redfish.Manufacturer,
			"firmware_version":  redfish.FirmwareVersion,
			"status":            redfish.Status,
			"power_state":       redfish.PowerState,
			"network_protocols": protocols,
		}
	}

	// Pretty print JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func init() {
	serverCmd.AddCommand(infoCmd)
	infoCmd.Flags().StringVar(&infoOutputFormat, "output", "text", "Output format (text or json)")
}
