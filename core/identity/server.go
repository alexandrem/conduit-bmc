package identity

import (
	"fmt"
	"strings"
)

// GenerateServerIDFromBMCEndpoint creates a server ID from datacenter ID and BMC endpoint.
// This is used by the manager to create synthetic server IDs for BMC endpoints reported by gateways.
//
// The format is: bmc-{datacenter_id}-{sanitized_endpoint}
// where sanitized_endpoint has colons (:) and dots (.) replaced with hyphens (-).
// Slashes (/) are NOT replaced to maintain URL structure visibility.
//
// Examples:
//   - "http://localhost:9001" in "dc-local-dev" -> "bmc-dc-local-dev-http-//localhost-9001"
//   - "http://redfish-01:8000" in "dc-east-1" -> "bmc-dc-east-1-http-//redfish-01-8000"
//   - "192.168.1.100:623" in "dc-west-2" -> "bmc-dc-west-2-192-168-1-100-623"
func GenerateServerIDFromBMCEndpoint(datacenterID, bmcEndpoint string) string {
	// Sanitize endpoint: replace : and . with -, but NOT /
	sanitizedEndpoint := strings.ReplaceAll(
		strings.ReplaceAll(bmcEndpoint, ":", "-"),
		".", "-")

	return fmt.Sprintf("bmc-%s-%s", datacenterID, sanitizedEndpoint)
}

// SanitizeBMCEndpointForID sanitizes a BMC endpoint string for use in server IDs.
// Replaces : and . with -, but preserves / for URL structure visibility.
func SanitizeBMCEndpointForID(endpoint string) string {
	return strings.ReplaceAll(
		strings.ReplaceAll(endpoint, ":", "-"),
		".", "-")
}
