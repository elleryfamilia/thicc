package terminal

import (
	"strings"
	"sync"

	"github.com/hinshun/vt10x"
)

// ScrollbackLine stores a single line of terminal content with styling
type ScrollbackLine struct {
	Cells []vt10x.Glyph // One glyph per column (preserves colors/attributes)
}

// ToString converts the scrollback line to a plain text string
// Trailing spaces are trimmed for cleaner output
func (sl *ScrollbackLine) ToString() string {
	var result strings.Builder
	for _, cell := range sl.Cells {
		if cell.Char == 0 {
			result.WriteRune(' ')
		} else {
			result.WriteRune(cell.Char)
		}
	}
	return strings.TrimRight(result.String(), " ")
}

// ScrollbackBuffer is a circular buffer for terminal history
type ScrollbackBuffer struct {
	lines    []ScrollbackLine
	capacity int // Maximum lines (default 10000)
	start    int // Index of oldest line in circular buffer
	count    int // Current number of lines stored
	mu       sync.RWMutex
}

// NewScrollbackBuffer creates a buffer with given capacity
func NewScrollbackBuffer(capacity int) *ScrollbackBuffer {
	if capacity <= 0 {
		capacity = 10000
	}
	return &ScrollbackBuffer{
		lines:    make([]ScrollbackLine, capacity),
		capacity: capacity,
		start:    0,
		count:    0,
	}
}

// Push adds a line to the buffer (evicts oldest if full)
func (sb *ScrollbackBuffer) Push(line ScrollbackLine) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.count < sb.capacity {
		// Buffer not full - add at next position
		sb.lines[sb.count] = line
		sb.count++
	} else {
		// Buffer full - overwrite oldest and advance start
		sb.lines[sb.start] = line
		sb.start = (sb.start + 1) % sb.capacity
	}
}

// Get retrieves a line by index (0 = oldest visible, count-1 = newest)
// Returns nil if index is out of range
func (sb *ScrollbackBuffer) Get(index int) *ScrollbackLine {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	if index < 0 || index >= sb.count {
		return nil
	}

	// Map logical index to physical position in circular buffer
	physicalIndex := (sb.start + index) % sb.capacity
	return &sb.lines[physicalIndex]
}

// Count returns number of stored lines
func (sb *ScrollbackBuffer) Count() int {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.count
}

// Clear empties the buffer
func (sb *ScrollbackBuffer) Clear() {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.start = 0
	sb.count = 0
}

// Capacity returns the maximum number of lines the buffer can hold
func (sb *ScrollbackBuffer) Capacity() int {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.capacity
}
