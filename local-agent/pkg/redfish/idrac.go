package redfish

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// IDRACRedfish handles iDRAC-specific Redfish operations.
type IDRACRedfish struct {
	Client *Client
}

// GetVendorType returns the vendor type this handler is for
func (i *IDRACRedfish) GetVendorType() VendorType {
	return VendorIDRAC
}

// DiscoverSerialConsole discovers serial console support for iDRAC BMCs.
// This is the VendorHandler interface implementation.
func (i *IDRACRedfish) DiscoverSerialConsole(ctx context.Context, endpoint, username, password, token string) (*SerialConsoleInfo, error) {
	// Get the manager to extract the manager ID
	manager, err := i.Client.getFirstManager(ctx, endpoint, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get manager: %w", err)
	}

	return i.discoverSerialConsoleIDRAC(ctx, endpoint, token, manager.ID)
}

// discoverSerialConsoleIDRAC is the internal implementation for iDRAC serial console discovery
func (i *IDRACRedfish) discoverSerialConsoleIDRAC(ctx context.Context, endpoint, token, managerID string) (*SerialConsoleInfo, error) {
	log.Debug().Str("managerID", managerID).Msg("Discovering iDRAC serial console support")

	// Construct the SerialInterfaces collection URL using the manager ID (e.g., iDRAC.Embedded.1)
	serialInterfacesURL := BuildRedfishURL(endpoint, "/redfish/v1/Managers/"+managerID+"/SerialInterfaces")

	var collection struct {
		Members []struct {
			ODataID string `json:"@odata.id"`
		} `json:"Members"`
	}

	if err := i.Client.GetWithToken(ctx, serialInterfacesURL, token, &collection); err != nil {
		return nil, fmt.Errorf("failed to get iDRAC serial interfaces collection: %w", err)
	}

	if len(collection.Members) == 0 {
		log.Warn().Msg("No serial interfaces found in iDRAC")
		return &SerialConsoleInfo{
			Vendor:         VendorIDRAC,
			Supported:      false,
			Enabled:        false,
			FallbackToIPMI: true,
		}, nil
	}

	// Get the first (and typically only) serial interface, e.g., Serial.1
	serialODataID := collection.Members[0].ODataID
	serialURL := BuildRedfishURL(endpoint, serialODataID)

	var serialInterface struct {
		InterfaceEnabled bool `json:"InterfaceEnabled"`
		// Note: iDRAC does not expose a direct Redfish SOL streaming action in standard SerialInterface.
		// OEM actions like SerialDataClear/SerialDataExport are present, but SOL is handled via IPMI.
		Actions struct {
			Oem map[string]interface{} `json:"Oem"`
		} `json:"Actions"`
	}

	if err := i.Client.GetWithToken(ctx, serialURL, token, &serialInterface); err != nil {
		return nil, fmt.Errorf("failed to get iDRAC serial interface details: %w", err)
	}

	// For iDRAC, no native Redfish SOL streaming; always fallback to IPMI if serial hardware is enabled.
	info := &SerialConsoleInfo{
		Vendor:         VendorIDRAC,
		Supported:      false,                            // No Redfish SOL support
		Enabled:        false,                            // No Redfish SOL enabled
		FallbackToIPMI: serialInterface.InterfaceEnabled, // Fallback if hardware serial is enabled
		SerialPath:     "",                               // No Redfish SOL path
	}

	// Optional: Check OEM actions for any potential SOL extension (unlikely based on docs).
	if solAction, exists := serialInterface.Actions.Oem["#DellSerialInterface.SOL"]; exists {
		if target, ok := solAction.(map[string]interface{})["target"].(string); ok && target != "" {
			info.SerialPath = BuildRedfishURL(endpoint, target)
			info.FallbackToIPMI = false
		}
	}

	log.Debug().
		Bool("supported", info.Supported).
		Bool("enabled", info.Enabled).
		Bool("fallbackToIPMI", info.FallbackToIPMI).
		Msg("iDRAC serial console discovery completed")

	return info, nil
}
