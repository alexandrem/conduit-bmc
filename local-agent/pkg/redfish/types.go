package redfish

// PowerState represents the power state of a server
type PowerState string

const (
	PowerStateOn      PowerState = "On"
	PowerStateOff     PowerState = "Off"
	PowerStateUnknown PowerState = "Unknown"
)

// ServiceRoot represents the Redfish service root response
type ServiceRoot struct {
	ID             string `json:"Id"`
	Name           string `json:"Name"`
	RedfishVersion string `json:"RedfishVersion"`
	UUID           string `json:"UUID"`
	Systems        struct {
		ODataID string `json:"@odata.id"`
	} `json:"Systems"`
	Chassis struct {
		ODataID string `json:"@odata.id"`
	} `json:"Chassis"`
}

// ComputerSystem represents a Redfish computer system
type ComputerSystem struct {
	ID            string     `json:"Id"`
	Name          string     `json:"Name"`
	Manufacturer  string     `json:"Manufacturer"`
	Model         string     `json:"Model"`
	SerialNumber  string     `json:"SerialNumber"`
	SKU           string     `json:"SKU"`
	HostName      string     `json:"HostName"`
	BiosVersion   string     `json:"BiosVersion"`
	PowerState    PowerState `json:"PowerState"`
	LastResetTime string     `json:"LastResetTime"`
	Status        struct {
		State        string `json:"State"`
		Health       string `json:"Health"`
		HealthRollup string `json:"HealthRollup"`
	} `json:"Status"`
	Boot struct {
		BootSourceOverrideTarget  string   `json:"BootSourceOverrideTarget"`
		BootSourceOverrideEnabled string   `json:"BootSourceOverrideEnabled"`
		BootSourceOverrideMode    string   `json:"BootSourceOverrideMode"`
		BootOrder                 []string `json:"BootOrder"`
	} `json:"Boot"`
	BootProgress struct {
		LastState    string `json:"LastState"`
		OemLastState string `json:"OemLastState"`
	} `json:"BootProgress"`
	PostState string `json:"PostState"`
	Oem       struct {
		Dell struct {
			DellSystem struct {
				CPURollupStatus          string `json:"CPURollupStatus"`
				StorageRollupStatus      string `json:"StorageRollupStatus"`
				TempRollupStatus         string `json:"TempRollupStatus"`
				VoltRollupStatus         string `json:"VoltRollupStatus"`
				FanRollupStatus          string `json:"FanRollupStatus"`
				PSRollupStatus           string `json:"PSRollupStatus"`
				BatteryRollupStatus      string `json:"BatteryRollupStatus"`
				SystemHealthRollupStatus string `json:"SystemHealthRollupStatus"`
			} `json:"DellSystem"`
		} `json:"Dell"`
	} `json:"Oem"`
	Actions struct {
		ComputerSystemReset struct {
			Target                   string   `json:"target"`
			ResetTypeAllowableValues []string `json:"ResetType@Redfish.AllowableValues"`
		} `json:"#ComputerSystem.Reset"`
	} `json:"Actions"`
}

// BMCInfo represents information about a Redfish BMC
type BMCInfo struct {
	Vendor          string
	Model           string
	FirmwareVersion string
	RedfishVersion  string
	Features        []string
}

// Manager represents a Redfish Manager (BMC)
type Manager struct {
	ID              string     `json:"Id"`
	Name            string     `json:"Name"`
	ManagerType     string     `json:"ManagerType"`
	Model           string     `json:"Model"`
	FirmwareVersion string     `json:"FirmwareVersion"`
	Manufacturer    string     `json:"Manufacturer"`
	PowerState      PowerState `json:"PowerState"`
	Status          struct {
		State  string `json:"State"`
		Health string `json:"Health"`
	} `json:"Status"`
	NetworkProtocol struct {
		ODataID string `json:"@odata.id"`
	} `json:"NetworkProtocol"`
}

// NetworkProtocol represents Redfish network protocol information
type NetworkProtocol struct {
	ID          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	Status      struct {
		State  string `json:"State"`
		Health string `json:"Health"`
	} `json:"Status"`
	HTTP struct {
		ProtocolEnabled bool  `json:"ProtocolEnabled"`
		Port            int32 `json:"Port"`
	} `json:"HTTP"`
	HTTPS struct {
		ProtocolEnabled bool  `json:"ProtocolEnabled"`
		Port            int32 `json:"Port"`
	} `json:"HTTPS"`
	SSH struct {
		ProtocolEnabled bool  `json:"ProtocolEnabled"`
		Port            int32 `json:"Port"`
	} `json:"SSH"`
	IPMI struct {
		ProtocolEnabled bool  `json:"ProtocolEnabled"`
		Port            int32 `json:"Port"`
	} `json:"IPMI"`
}

// Session represents a Redfish session response
type Session struct {
	ODataID string `json:"@odata.id"`
	ID      string `json:"Id"`
}

// SessionCollection represents the sessions collection
type SessionCollection struct {
	Members []struct {
		ODataID string `json:"@odata.id"`
	} `json:"Members"`
}

// SerialConsoleInfo represents SerialConsole support info
type SerialConsoleInfo struct {
	Supported      bool
	Enabled        bool
	Vendor         VendorType
	FallbackToIPMI bool
	SerialPath     string // Specific path for vendor, e.g., /Managers/iDRAC.Embedded.1/SerialInterfaces/Serial.1
	// Add more fields as needed, e.g., ServiceEnabled, MaxConcurrentSessions
}
