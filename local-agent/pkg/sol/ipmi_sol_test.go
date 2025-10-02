package sol

import (
	"context"
	"testing"
	"time"
)

func TestCircularBuffer(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		writes   [][]byte
		expected string
	}{
		{
			name:     "write less than buffer size",
			size:     10,
			writes:   [][]byte{[]byte("hello")},
			expected: "hello",
		},
		{
			name:     "write exactly buffer size",
			size:     10,
			writes:   [][]byte{[]byte("0123456789")},
			expected: "0123456789",
		},
		{
			name:     "write more than buffer size (should wrap)",
			size:     10,
			writes:   [][]byte{[]byte("0123456789abcdef")},
			expected: "6789abcdef", // Last 10 characters (wraps around)
		},
		{
			name:     "multiple writes",
			size:     10,
			writes:   [][]byte{[]byte("abc"), []byte("def"), []byte("ghi")},
			expected: "abcdefghi",
		},
		{
			name:     "multiple writes with overflow",
			size:     5,
			writes:   [][]byte{[]byte("abc"), []byte("def"), []byte("ghi")},
			expected: "efghi", // Last 5 characters
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := newCircularBuffer(tt.size)

			for _, write := range tt.writes {
				buffer.Write(write)
			}

			result := buffer.Read()
			if string(result) != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, string(result))
			}
		})
	}
}

func TestCircularBuffer_Available(t *testing.T) {
	buffer := newCircularBuffer(10)

	// Empty buffer
	if buffer.Available() != 0 {
		t.Errorf("Expected 0 bytes available, got %d", buffer.Available())
	}

	// Partial write
	buffer.Write([]byte("hello"))
	if buffer.Available() != 5 {
		t.Errorf("Expected 5 bytes available, got %d", buffer.Available())
	}

	// Full buffer
	buffer.Write([]byte("world!"))
	if buffer.Available() != 10 {
		t.Errorf("Expected 10 bytes available (full), got %d", buffer.Available())
	}

	// Overflow (should still be 10)
	buffer.Write([]byte("test"))
	if buffer.Available() != 10 {
		t.Errorf("Expected 10 bytes available (full), got %d", buffer.Available())
	}
}

func TestCircularBuffer_Clear(t *testing.T) {
	buffer := newCircularBuffer(10)

	buffer.Write([]byte("test data"))
	if buffer.Available() != 9 {
		t.Errorf("Expected 9 bytes before clear, got %d", buffer.Available())
	}

	buffer.Clear()
	if buffer.Available() != 0 {
		t.Errorf("Expected 0 bytes after clear, got %d", buffer.Available())
	}

	result := buffer.Read()
	if result != nil {
		t.Errorf("Expected nil after clear, got %q", string(result))
	}
}

func TestIPMISOLSession_Lifecycle(t *testing.T) {
	// Skip if ipmiconsole is not available
	ctx := context.Background()
	session, err := NewIPMISOLSession(ctx, "localhost:623", "admin", "password", 0)
	if err != nil {
		t.Skipf("Skipping test: ipmiconsole not available: %v", err)
	}
	defer session.Close()

	// Test session is created
	if session == nil {
		t.Fatal("Expected session to be created")
	}

	// Test close
	err = session.Close()
	if err != nil {
		t.Errorf("Expected clean close, got error: %v", err)
	}

	// Should not be running after close
	if session.IsRunning() {
		t.Error("Expected session to not be running after close")
	}
}

func TestIPMISOLSession_WriteRead(t *testing.T) {
	// Skip if ipmiconsole is not available
	ctx := context.Background()
	session, err := NewIPMISOLSession(ctx, "localhost:623", "admin", "password", 0)
	if err != nil {
		t.Skipf("Skipping test: ipmiconsole not available: %v", err)
	}
	defer session.Close()

	// Test write
	testData := []byte("test command\n")
	err = session.Write(testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	// Test read (with timeout)
	readCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan bool)
	go func() {
		_, err := session.Read()
		if err != nil && err != context.Canceled {
			t.Logf("Read returned error (expected if no BMC): %v", err)
		}
		done <- true
	}()

	select {
	case <-done:
		// Read completed (expected with connection error)
	case <-readCtx.Done():
		t.Log("Read timed out (expected if no BMC)")
	}
}

func TestIPMITransport_Lifecycle(t *testing.T) {
	transport := NewIPMITransport()
	if transport == nil {
		t.Fatal("Expected transport to be created")
	}

	// Test status before connection
	status := transport.Status()
	if status.Connected {
		t.Error("Expected transport to not be connected initially")
	}
	if status.Protocol != "ipmi_sol" {
		t.Errorf("Expected protocol 'ipmi_sol', got %q", status.Protocol)
	}
}

func TestIPMITransport_ConnectClose(t *testing.T) {
	transport := NewIPMITransport()

	ctx := context.Background()
	config := DefaultSOLConfig()

	// Attempt to connect (will fail without real BMC, but tests the flow)
	err := transport.Connect(ctx, "localhost:623", "admin", "password", config)
	if err != nil {
		// Expected to fail without real BMC or ipmiconsole
		t.Logf("Connect failed (expected without BMC): %v", err)
	}

	// Test close
	err = transport.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Status should show disconnected
	status := transport.Status()
	if status.Connected {
		t.Error("Expected transport to be disconnected after close")
	}
}

func TestSOLMetrics_Recording(t *testing.T) {
	// Skip if ipmiconsole is not available
	ctx := context.Background()
	session, err := NewIPMISOLSession(ctx, "localhost:623", "admin", "password", 0)
	if err != nil {
		t.Skipf("Skipping test: ipmiconsole not available: %v", err)
	}
	defer session.Close()

	// Record some metrics
	session.recordWrite(100)
	session.recordRead(200)

	metrics := session.GetMetrics()
	if metrics.bytesWritten != 100 {
		t.Errorf("Expected 100 bytes written, got %d", metrics.bytesWritten)
	}
	if metrics.bytesRead != 200 {
		t.Errorf("Expected 200 bytes read, got %d", metrics.bytesRead)
	}
}

func TestIPMISOLSession_ReplayBuffer(t *testing.T) {
	// Skip if ipmiconsole is not available
	ctx := context.Background()
	bufferSize := 1024
	session, err := NewIPMISOLSession(ctx, "localhost:623", "admin", "password", bufferSize)
	if err != nil {
		t.Skipf("Skipping test: ipmiconsole not available: %v", err)
	}
	defer session.Close()

	// Test replay buffer exists
	buffer := session.GetReplayBuffer()
	if buffer == nil {
		// Empty is fine initially
		t.Log("Replay buffer is empty (expected)")
	}

	// Replay buffer should be initialized with correct size
	if session.replayBuffer == nil {
		t.Error("Expected replay buffer to be initialized")
	}
	if session.replayBuffer.Size() != bufferSize {
		t.Errorf("Expected replay buffer size %d, got %d", bufferSize, session.replayBuffer.Size())
	}
}
