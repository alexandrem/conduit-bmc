package sol

import "sync"

// circularBuffer implements a thread-safe circular buffer for session replay
type circularBuffer struct {
	data     []byte
	size     int
	writePos int
	count    int // Number of bytes currently in buffer
	mu       sync.RWMutex
}

// newCircularBuffer creates a new circular buffer with the specified size
func newCircularBuffer(size int) *circularBuffer {
	return &circularBuffer{
		data: make([]byte, size),
		size: size,
	}
}

// Write appends data to the circular buffer
// If the buffer is full, it will overwrite the oldest data
func (cb *circularBuffer) Write(data []byte) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	for _, b := range data {
		cb.data[cb.writePos] = b
		cb.writePos = (cb.writePos + 1) % cb.size

		if cb.count < cb.size {
			cb.count++
		}
	}
}

// Read returns all available data from the circular buffer
// This returns a snapshot of the buffer contents in chronological order
func (cb *circularBuffer) Read() []byte {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.count == 0 {
		return nil // Empty buffer
	}

	result := make([]byte, cb.count)

	if cb.count < cb.size {
		// Buffer not full yet, data is from 0 to writePos
		copy(result, cb.data[:cb.count])
	} else {
		// Buffer is full, data wraps around
		// Oldest data starts at writePos
		oldestPos := cb.writePos
		copy(result, cb.data[oldestPos:])
		copy(result[cb.size-oldestPos:], cb.data[:oldestPos])
	}

	return result
}

// Size returns the total capacity of the buffer
func (cb *circularBuffer) Size() int {
	return cb.size
}

// Available returns the number of bytes currently stored in the buffer
func (cb *circularBuffer) Available() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.count
}

// Clear resets the circular buffer to empty state
func (cb *circularBuffer) Clear() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.writePos = 0
	cb.count = 0
}
