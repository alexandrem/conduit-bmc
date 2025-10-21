package types

// BMCType represents the type of BMC interface available on a server
type BMCType string

const (
	BMCTypeNone    BMCType = ""
	BMCTypeIPMI    BMCType = "ipmi"
	BMCTypeRedfish BMCType = "redfish"
)

// String returns the string representation of BMCType
func (b BMCType) String() string {
	return string(b)
}

// SOLType represents the type of Serial-over-LAN endpoint
type SOLType string

const (
	SOLTypeNone          SOLType = ""
	SOLTypeIPMI          SOLType = "ipmi"
	SOLTypeRedfishSerial SOLType = "redfish_serial"
)

// String returns the string representation of SOLType
func (s SOLType) String() string {
	return string(s)
}

// VNCType represents the type of VNC transport
type VNCType string

const (
	VNCTypeNone      VNCType = ""
	VNCTypeNative    VNCType = "native"
	VNCTypeWebSocket VNCType = "websocket"
)

// String returns the string representation of VNCType
func (v VNCType) String() string {
	return string(v)
}

// InferBMCType infers the BMC type from an endpoint URL.
// Returns BMCTypeRedfish for HTTP/HTTPS endpoints, BMCTypeIPMI otherwise.
func InferBMCType(endpoint string) BMCType {
	if len(endpoint) >= 7 && (endpoint[:7] == "http://" || endpoint[:8] == "https://") {
		return BMCTypeRedfish
	}
	return BMCTypeIPMI
}

// InferSOLType infers the SOL type from an endpoint URL.
// Returns SOLTypeRedfishSerial for HTTP/HTTPS endpoints, SOLTypeIPMI otherwise.
func InferSOLType(endpoint string) SOLType {
	if len(endpoint) >= 7 && (endpoint[:7] == "http://" || endpoint[:8] == "https://") {
		return SOLTypeRedfishSerial
	}
	return SOLTypeIPMI
}

// InferVNCType infers the VNC type from an endpoint URL.
// Returns VNCTypeWebSocket for ws:// or wss:// endpoints, VNCTypeNative otherwise.
func InferVNCType(endpoint string) VNCType {
	if len(endpoint) >= 5 && (endpoint[:5] == "ws://" || endpoint[:6] == "wss://") {
		return VNCTypeWebSocket
	}
	return VNCTypeNative
}
