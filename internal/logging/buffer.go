package logging

import (
	"bufio"
	"bytes"
	"fmt"
	"sync"
	"time"
)

const DefaultBufferSize = 1000

// RingBuffer is a thread-safe circular buffer for log lines.
type RingBuffer struct {
	mu    sync.Mutex
	lines []string
	size  int
	pos   int
	count int
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(size int) *RingBuffer {
	if size <= 0 {
		size = DefaultBufferSize
	}
	return &RingBuffer{
		lines: make([]string, size),
		size:  size,
	}
}

// WriteString appends a line to the buffer.
func (rb *RingBuffer) WriteString(line string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.lines[rb.pos] = line
	rb.pos = (rb.pos + 1) % rb.size
	if rb.count < rb.size {
		rb.count++
	}
}

// Write implements io.Writer. It splits input on newlines and timestamps each line.
func (rb *RingBuffer) Write(p []byte) (int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(p))
	for scanner.Scan() {
		line := scanner.Text()
		ts := time.Now().Format("15:04:05")
		rb.WriteString(fmt.Sprintf("[%s] %s", ts, line))
	}
	return len(p), nil
}

// Lines returns the last n lines. If n <= 0 or n > count, returns all lines.
func (rb *RingBuffer) Lines(n int) []string {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if n <= 0 || n > rb.count {
		n = rb.count
	}
	if n == 0 {
		return nil
	}

	result := make([]string, n)
	start := (rb.pos - n + rb.size) % rb.size
	for i := 0; i < n; i++ {
		result[i] = rb.lines[(start+i)%rb.size]
	}
	return result
}

// All returns all lines in order.
func (rb *RingBuffer) All() []string {
	return rb.Lines(0)
}

// Len returns the number of lines currently in the buffer.
func (rb *RingBuffer) Len() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count
}
