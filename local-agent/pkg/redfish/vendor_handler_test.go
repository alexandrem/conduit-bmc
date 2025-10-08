package redfish

import (
	"testing"
)

func TestDetectVendorFromManager(t *testing.T) {
	tests := []struct {
		name     string
		manager  *Manager
		expected VendorType
	}{
		{
			name: "iDRAC by ID",
			manager: &Manager{
				ID:           "iDRAC.Embedded.1",
				Manufacturer: "Dell Inc.",
			},
			expected: VendorIDRAC,
		},
		{
			name: "iDRAC by manufacturer",
			manager: &Manager{
				ID:           "BMC1",
				Manufacturer: "Dell Inc.",
			},
			expected: VendorIDRAC,
		},
		{
			name: "iDRAC by manufacturer lowercase",
			manager: &Manager{
				ID:           "BMC1",
				Manufacturer: "dell",
			},
			expected: VendorIDRAC,
		},
		{
			name: "Generic BMC",
			manager: &Manager{
				ID:           "BMC1",
				Manufacturer: "Supermicro",
			},
			expected: VendorGeneric,
		},
		{
			name: "Generic BMC - no manufacturer",
			manager: &Manager{
				ID:           "Manager1",
				Manufacturer: "",
			},
			expected: VendorGeneric,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectVendorFromManager(tt.manager)
			if result != tt.expected {
				t.Errorf("detectVendorFromManager() = %v; want %v", result, tt.expected)
			}
		})
	}
}

func TestNewVendorHandler(t *testing.T) {
	client := NewClient()

	tests := []struct {
		name       string
		vendorType VendorType
		wantType   VendorType
	}{
		{
			name:       "iDRAC handler",
			vendorType: VendorIDRAC,
			wantType:   VendorIDRAC,
		},
		{
			name:       "Generic handler",
			vendorType: VendorGeneric,
			wantType:   VendorGeneric,
		},
		{
			name:       "Unknown defaults to Generic",
			vendorType: VendorType("unknown"),
			wantType:   VendorGeneric,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewVendorHandler(tt.vendorType, client)
			if handler.GetVendorType() != tt.wantType {
				t.Errorf("NewVendorHandler(%v).GetVendorType() = %v; want %v",
					tt.vendorType, handler.GetVendorType(), tt.wantType)
			}
		})
	}
}

func TestVendorHandlerInterface(t *testing.T) {
	client := NewClient()

	// Verify IDRACRedfish implements VendorHandler
	var _ VendorHandler = &IDRACRedfish{Client: client}

	// Verify GenericRedfish implements VendorHandler
	var _ VendorHandler = &GenericRedfish{Client: client}
}
