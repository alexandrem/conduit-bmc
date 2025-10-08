package redfish

import (
	"strings"
)

// BuildRedfishURL constructs a Redfish URL by combining an endpoint with a path.
// It ensures there's no double slash between endpoint and path.
func BuildRedfishURL(endpoint, path string) string {
	endpoint = strings.TrimSuffix(endpoint, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return endpoint + path
}

// BuildServiceRootURL returns the Redfish service root URL
func BuildServiceRootURL(endpoint string) string {
	return BuildRedfishURL(endpoint, "/redfish/v1/")
}

// BuildManagersURL returns the Managers collection URL
func BuildManagersURL(endpoint string) string {
	return BuildRedfishURL(endpoint, "/redfish/v1/Managers")
}

// BuildSessionsURL returns the Sessions collection URL
func BuildSessionsURL(endpoint string) string {
	return BuildRedfishURL(endpoint, "/redfish/v1/SessionService/Sessions")
}

// BuildSystemsURL returns the Systems collection URL
func BuildSystemsURL(endpoint string) string {
	return BuildRedfishURL(endpoint, "/redfish/v1/Systems")
}

// BuildChassisURL returns the Chassis collection URL
func BuildChassisURL(endpoint string) string {
	return BuildRedfishURL(endpoint, "/redfish/v1/Chassis")
}
