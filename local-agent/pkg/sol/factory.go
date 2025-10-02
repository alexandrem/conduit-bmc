package sol

import (
	"fmt"
)

// NewClient creates a Client based on the BMC type using unified implementation
func NewClient(bmcType Type) (Client, error) {
	var transport Transport

	switch bmcType {
	case TypeIPMI:
		transport = NewIPMITransport()
	case TypeRedfishSerial:
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
func GetSupportedSOLTypes() []Type {
	return []Type{
		TypeIPMI,
		TypeRedfishSerial,
		TypeMock,
	}
}

// IsValidSOLType checks if the given SOL type is supported
func IsValidSOLType(solType Type) bool {
	for _, supported := range GetSupportedSOLTypes() {
		if supported == solType {
			return true
		}
	}
	return false
}
