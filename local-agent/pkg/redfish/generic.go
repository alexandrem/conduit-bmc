package redfish

import (
	"context"
	"fmt"
)

// GenericRedfish handles generic Redfish BMC discovery
type GenericRedfish struct {
	Client *Client
}

// GetVendorType returns the vendor type this handler is for
func (g *GenericRedfish) GetVendorType() VendorType {
	return VendorGeneric
}

// DiscoverSerialConsole discovers serial console support for generic Redfish BMCs.
// This is the VendorHandler interface implementation.
func (g *GenericRedfish) DiscoverSerialConsole(ctx context.Context, endpoint, username, password, token string) (*SerialConsoleInfo, error) {
	// Generic Redfish
	managersURL := BuildManagersURL(endpoint)
	var managersCollection struct {
		Members []struct {
			ODataID string `json:"@odata.id"`
		} `json:"Members"`
	}
	if err := g.Client.GetWithToken(ctx, managersURL, token, &managersCollection); err != nil {
		return nil, err
	}

	if len(managersCollection.Members) == 0 {
		return nil, fmt.Errorf("no managers found")
	}

	// Get first manager
	managerURL := BuildRedfishURL(endpoint, managersCollection.Members[0].ODataID)
	var genericManager struct {
		SerialConsole struct {
			ServiceEnabled        bool `json:"ServiceEnabled"`
			MaxConcurrentSessions int  `json:"MaxConcurrentSessions"`
		} `json:"SerialConsole"`
	}
	if err := g.Client.GetWithToken(ctx, managerURL, token, &genericManager); err != nil {
		return nil, err
	}

	info := &SerialConsoleInfo{
		Vendor:    VendorGeneric,
		Supported: genericManager.SerialConsole.MaxConcurrentSessions > 0,
		Enabled:   genericManager.SerialConsole.ServiceEnabled,
	}

	return info, nil
}
