package framework

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// TestE2EMain runs the full E2E test suite
func TestE2EMain(t *testing.T) {
	t.Log("Running E2E test suite...")
	suite.Run(t, new(E2ETestSuite))
}
