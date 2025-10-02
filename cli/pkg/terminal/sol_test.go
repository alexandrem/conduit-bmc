package terminal

import (
	"testing"
)

// TestCheckExitSequence verifies exit sequence detection logic.
//
// Tests various input scenarios including:
//   - Complete exit sequence (Ctrl+] then 'q')
//   - Incomplete sequences
//   - Normal text input
//   - Exit sequence split across multiple calls
func TestCheckExitSequence(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{
			name:     "exit sequence ctrl+] then q",
			input:    []byte{0x1D, 'q'},
			expected: true,
		},
		{
			name:     "ctrl+] but not q",
			input:    []byte{0x1D, 'x'},
			expected: false,
		},
		{
			name:     "normal text",
			input:    []byte("hello world"),
			expected: false,
		},
		{
			name:     "ctrl+] at end",
			input:    []byte("test\x1D"),
			expected: false,
		},
		{
			name:     "split across calls - ctrl+]",
			input:    []byte{0x1D},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terminal := &SOLTerminal{
				done: make(chan struct{}),
			}

			result := terminal.checkExitSequence(tt.input)
			if result != tt.expected {
				t.Errorf("checkExitSequence(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestCheckExitSequenceSplitAcrossCalls verifies state preservation across multiple calls.
//
// This test ensures that the exit sequence can be detected even when Ctrl+] and 'q'
// arrive in separate stdin read operations.
func TestCheckExitSequenceSplitAcrossCalls(t *testing.T) {
	terminal := &SOLTerminal{
		done: make(chan struct{}),
	}

	// First call with Ctrl+]
	result := terminal.checkExitSequence([]byte{0x1D})
	if result {
		t.Error("First call with Ctrl+] should not exit")
	}

	// Second call with 'q' should trigger exit
	result = terminal.checkExitSequence([]byte{'q'})
	if !result {
		t.Error("Second call with 'q' after Ctrl+] should exit")
	}
}
