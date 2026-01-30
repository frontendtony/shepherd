package logging

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRingBuffer_WriteAndRead(t *testing.T) {
	rb := NewRingBuffer(5)

	rb.WriteString("line 1")
	rb.WriteString("line 2")
	rb.WriteString("line 3")

	lines := rb.All()
	assert.Equal(t, []string{"line 1", "line 2", "line 3"}, lines)
	assert.Equal(t, 3, rb.Len())
}

func TestRingBuffer_Overflow(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.WriteString("1")
	rb.WriteString("2")
	rb.WriteString("3")
	rb.WriteString("4")
	rb.WriteString("5")

	lines := rb.All()
	assert.Equal(t, []string{"3", "4", "5"}, lines)
	assert.Equal(t, 3, rb.Len())
}

func TestRingBuffer_LinesN(t *testing.T) {
	rb := NewRingBuffer(10)

	for i := 1; i <= 5; i++ {
		rb.WriteString(fmt.Sprintf("line %d", i))
	}

	last2 := rb.Lines(2)
	assert.Equal(t, []string{"line 4", "line 5"}, last2)

	last10 := rb.Lines(10)
	assert.Len(t, last10, 5)
}

func TestRingBuffer_Empty(t *testing.T) {
	rb := NewRingBuffer(5)
	assert.Nil(t, rb.All())
	assert.Nil(t, rb.Lines(3))
	assert.Equal(t, 0, rb.Len())
}

func TestRingBuffer_Write_IOWriter(t *testing.T) {
	rb := NewRingBuffer(10)

	input := []byte("hello world\nsecond line\n")
	n, err := rb.Write(input)
	assert.NoError(t, err)
	assert.Equal(t, len(input), n)

	lines := rb.All()
	assert.Len(t, lines, 2)
	assert.Contains(t, lines[0], "hello world")
	assert.Contains(t, lines[1], "second line")
	// Check timestamps are present.
	assert.Contains(t, lines[0], "[")
	assert.Contains(t, lines[0], "]")
}

func TestRingBuffer_ThreadSafety(t *testing.T) {
	rb := NewRingBuffer(100)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rb.WriteString(fmt.Sprintf("goroutine %d line %d", id, j))
			}
		}(i)
	}

	wg.Wait()
	lines := rb.All()
	assert.Equal(t, 100, len(lines))
}
