package redfish

import (
	"errors"
	"fmt"
)

// VendorNotSupportedError indicates an unsupported or unknown BMC vendor
type VendorNotSupportedError struct {
	Vendor VendorType
}

func (e *VendorNotSupportedError) Error() string {
	return fmt.Sprintf("vendor '%s' is not supported", e.Vendor)
}

// IsVendorNotSupportedError checks if an error is a VendorNotSupportedError
func IsVendorNotSupportedError(err error) bool {
	var e *VendorNotSupportedError
	return errors.As(err, &e)
}

// HTTPError represents an HTTP-related error with status code
type HTTPError struct {
	StatusCode int
	Status     string
	Operation  string // e.g., "get manager", "create session"
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s failed: HTTP %d: %s", e.Operation, e.StatusCode, e.Status)
}

// NewHTTPError creates a new HTTPError
func NewHTTPError(statusCode int, status, operation string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Status:     status,
		Operation:  operation,
	}
}
