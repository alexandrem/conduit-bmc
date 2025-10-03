package types

// BMCType represents the type of BMC interface available on a server
type BMCType string

const (
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
	VNCTypeNative    VNCType = "native"
	VNCTypeWebSocket VNCType = "websocket"
)

// String returns the string representation of VNCType
func (v VNCType) String() string {
	return string(v)
}
