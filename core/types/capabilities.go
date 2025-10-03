package types

// Capability represents a low-level BMC protocol command or API capability
type Capability string

// IPMI Protocol Capabilities
const (
	// CapabilityIPMISEL - System Event Log (ipmitool sel)
	CapabilityIPMISEL Capability = "sel"

	// CapabilityIPMISDR - Sensor Data Repository (ipmitool sdr)
	CapabilityIPMISDR Capability = "sdr"

	// CapabilityIPMIFRU - Field Replaceable Unit data (ipmitool fru)
	CapabilityIPMIFRU Capability = "fru"

	// CapabilityIPMIChassis - Chassis control and status (ipmitool chassis)
	CapabilityIPMIChassis Capability = "chassis"

	// CapabilityIPMILAN - LAN configuration (ipmitool lan)
	CapabilityIPMILAN Capability = "lan"

	// CapabilityIPMIPEF - Platform Event Filtering
	CapabilityIPMIPEF Capability = "pef"

	// CapabilityIPMIUser - User management
	CapabilityIPMIUser Capability = "user"
)

// Redfish API Resource Capabilities
const (
	// CapabilityRedfishSystems - /redfish/v1/Systems endpoint
	CapabilityRedfishSystems Capability = "Systems"

	// CapabilityRedfishChassis - /redfish/v1/Chassis endpoint
	CapabilityRedfishChassis Capability = "Chassis"

	// CapabilityRedfishManagers - /redfish/v1/Managers endpoint
	CapabilityRedfishManagers Capability = "Managers"

	// CapabilityRedfishSessionService - Session management
	CapabilityRedfishSessionService Capability = "SessionService"

	// CapabilityRedfishEventService - Event subscription and streaming
	CapabilityRedfishEventService Capability = "EventService"

	// CapabilityRedfishUpdateService - Firmware update service
	CapabilityRedfishUpdateService Capability = "UpdateService"

	// CapabilityRedfishAccountService - User account management
	CapabilityRedfishAccountService Capability = "AccountService"
)

// IPMICapabilities returns all standard IPMI capabilities
func IPMICapabilities() []Capability {
	return []Capability{
		CapabilityIPMISEL,
		CapabilityIPMISDR,
		CapabilityIPMIFRU,
		CapabilityIPMIChassis,
		CapabilityIPMILAN,
	}
}

// RedfishCapabilities returns all standard Redfish capabilities
func RedfishCapabilities() []Capability {
	return []Capability{
		CapabilityRedfishSystems,
		CapabilityRedfishChassis,
		CapabilityRedfishManagers,
		CapabilityRedfishSessionService,
		CapabilityRedfishEventService,
	}
}

// String returns the string representation of a Capability
func (c Capability) String() string {
	return string(c)
}

// IsIPMI checks if this is an IPMI capability
func (c Capability) IsIPMI() bool {
	switch c {
	case CapabilityIPMISEL, CapabilityIPMISDR, CapabilityIPMIFRU,
		CapabilityIPMIChassis, CapabilityIPMILAN, CapabilityIPMIPEF, CapabilityIPMIUser:
		return true
	}
	return false
}

// IsRedfish checks if this is a Redfish capability
func (c Capability) IsRedfish() bool {
	switch c {
	case CapabilityRedfishSystems, CapabilityRedfishChassis, CapabilityRedfishManagers,
		CapabilityRedfishSessionService, CapabilityRedfishEventService,
		CapabilityRedfishUpdateService, CapabilityRedfishAccountService:
		return true
	}
	return false
}

// CapabilitiesToStrings converts a slice of Capabilities to strings
func CapabilitiesToStrings(caps []Capability) []string {
	result := make([]string, len(caps))
	for i, c := range caps {
		result[i] = c.String()
	}
	return result
}

// StringsToCapabilities converts a slice of strings to Capabilities
func StringsToCapabilities(strs []string) []Capability {
	result := make([]Capability, len(strs))
	for i, s := range strs {
		result[i] = Capability(s)
	}
	return result
}
