package redfish

import (
	"context"
)

// VendorHandler defines the interface for vendor-specific Redfish operations.
// Different BMC vendors (Dell iDRAC, HPE iLO, Supermicro, etc.) may have
// vendor-specific implementations or extensions to the Redfish standard.
type VendorHandler interface {
	// DiscoverSerialConsole checks if serial console is supported and how to access it.
	// Returns information about serial console availability and configuration.
	DiscoverSerialConsole(ctx context.Context, endpoint, username, password, token string) (*SerialConsoleInfo, error)

	// GetVendorType returns the vendor type this handler is for
	GetVendorType() VendorType
}

// NewVendorHandler creates the appropriate VendorHandler based on vendor type
func NewVendorHandler(vendorType VendorType, client *Client) VendorHandler {
	switch vendorType {
	case VendorIDRAC:
		return &IDRACRedfish{Client: client}
	case VendorGeneric:
		return &GenericRedfish{Client: client}
	default:
		// Default to generic handler for unknown vendors
		return &GenericRedfish{Client: client}
	}
}
