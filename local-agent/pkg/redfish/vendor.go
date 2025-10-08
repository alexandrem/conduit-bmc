package redfish

// VendorType represents the BMC vendor
type VendorType string

const (
	VendorGeneric VendorType = "generic"
	VendorIDRAC   VendorType = "idrac"
)
