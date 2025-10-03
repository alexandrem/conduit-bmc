package types

// Feature represents a high-level customer-facing service capability
type Feature string

const (
	// FeaturePower - Power management (on/off/cycle/reset/status)
	FeaturePower Feature = "power"

	// FeatureConsole - Serial console access (IPMI SOL or Redfish serial)
	FeatureConsole Feature = "console"

	// FeatureVNC - Graphical console access (KVM/VNC)
	FeatureVNC Feature = "vnc"

	// FeatureSensors - Hardware monitoring (temperature, fans, voltage, etc.)
	FeatureSensors Feature = "sensors"

	// FeatureMedia - Virtual media mounting (ISO/image mounting)
	FeatureMedia Feature = "media"

	// FeatureFirmware - Firmware update capability
	FeatureFirmware Feature = "firmware"
)

// AllFeatures returns all defined features
func AllFeatures() []Feature {
	return []Feature{
		FeaturePower,
		FeatureConsole,
		FeatureVNC,
		FeatureSensors,
		FeatureMedia,
		FeatureFirmware,
	}
}

// String returns the string representation of a Feature
func (f Feature) String() string {
	return string(f)
}

// IsValid checks if a feature string is valid
func (f Feature) IsValid() bool {
	switch f {
	case FeaturePower, FeatureConsole, FeatureVNC, FeatureSensors, FeatureMedia, FeatureFirmware:
		return true
	}
	return false
}

// FeaturesToStrings converts a slice of Features to strings
func FeaturesToStrings(features []Feature) []string {
	result := make([]string, len(features))
	for i, f := range features {
		result[i] = f.String()
	}
	return result
}

// StringsToFeatures converts a slice of strings to Features
func StringsToFeatures(strs []string) []Feature {
	result := make([]Feature, 0, len(strs))
	for _, s := range strs {
		f := Feature(s)
		if f.IsValid() {
			result = append(result, f)
		}
	}
	return result
}
