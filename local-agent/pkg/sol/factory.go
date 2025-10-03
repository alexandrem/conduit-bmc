package sol

import (
	"fmt"

	"core/types"
)

// NewClient creates a Client based on the BMC type using unified implementation
func NewClient(bmcType types.SOLType) (Client, error) {
	var transport Transport

	switch bmcType {
	case types.SOLTypeIPMI:
		transport = NewIPMITransport()
	case types.SOLTypeRedfishSerial:
		transport = NewRedfishTransport()
	case TypeMock:
		transport = NewMockTransport()
	default:
		return nil, fmt.Errorf("unsupported SOL type: %s", bmcType)
	}

	return NewUnifiedClient(transport), nil
}

// NewClientWithTransport creates a Client with a specific transport
// This allows for custom transport implementations
func NewClientWithTransport(transport Transport) Client {
	return NewUnifiedClient(transport)
}

// GetSupportedSOLTypes returns all supported SOL types
func GetSupportedSOLTypes() []types.SOLType {
	return []types.SOLType{
		types.SOLTypeIPMI,
		types.SOLTypeRedfishSerial,
		TypeMock,
	}
}

// IsValidSOLType checks if the given SOL type is supported
func IsValidSOLType(solType types.SOLType) bool {
	for _, supported := range GetSupportedSOLTypes() {
		if supported == solType {
			return true
		}
	}
	return false
}
