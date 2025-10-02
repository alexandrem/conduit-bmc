package config

import (
	"strconv"
)

// parseIntValue parses a string as an integer
func parseIntValue(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

// parseUintValue parses a string as an unsigned integer
func parseUintValue(value string) (uint64, error) {
	return strconv.ParseUint(value, 10, 64)
}

// parseFloatValue parses a string as a float
func parseFloatValue(value string, bitSize int) (float64, error) {
	return strconv.ParseFloat(value, bitSize)
}