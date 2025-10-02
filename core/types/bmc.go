package types

// BMCType represents the type of BMC interface available on a server
type BMCType string

const (
	BMCTypeIPMI    BMCType = "ipmi"
	BMCTypeRedfish BMCType = "redfish"
)
