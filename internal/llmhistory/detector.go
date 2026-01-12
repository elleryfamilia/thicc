package llmhistory

import (
	"bytes"
	"crypto/sha256"
	"sync"
	"time"
)

// OutputProcessor handles line-based deduplication of terminal output
type OutputProcessor struct {
	buffer     bytes.Buffer
	lineHashes map[[32]byte]time.Time // Hash -> last seen time
	hashMu     sync.Mutex
	dedupWindow time.Duration

	// Callback for deduplicated lines
	onLine func(line string)

	// Carriage return handling - track partial line being overwritten
	partialLine bytes.Buffer
}

// NewOutputProcessor creates a new output processor with line deduplication
func NewOutputProcessor(onLine func(line string)) *OutputProcessor {
	return &OutputProcessor{
		lineHashes:  make(map[[32]byte]time.Time),
		dedupWindow: 2 * time.Second,
		onLine:      onLine,
	}
}

// Process processes raw terminal output, deduplicates, and emits clean lines
func (p *OutputProcessor) Process(data []byte) {
	for _, b := range data {
		switch b {
		case '\n':
			// Complete line - emit if not duplicate
			line := p.partialLine.String()
			p.partialLine.Reset()

			if line != "" {
				p.emitIfNew(line)
			}

		case '\r':
			// Carriage return - discard partial line (being overwritten)
			p.partialLine.Reset()

		default:
			p.partialLine.WriteByte(b)
		}
	}
}

// emitIfNew emits the line if it hasn't been seen recently
func (p *OutputProcessor) emitIfNew(line string) {
	// Hash the line
	hash := sha256.Sum256([]byte(line))
	now := time.Now()

	p.hashMu.Lock()
	defer p.hashMu.Unlock()

	// Check if seen recently
	if lastSeen, exists := p.lineHashes[hash]; exists {
		if now.Sub(lastSeen) < p.dedupWindow {
			// Duplicate - skip
			return
		}
	}

	// Record this line
	p.lineHashes[hash] = now

	// Clean up old hashes periodically (every 100 unique lines)
	if len(p.lineHashes) > 100 {
		for h, t := range p.lineHashes {
			if now.Sub(t) > p.dedupWindow*2 {
				delete(p.lineHashes, h)
			}
		}
	}

	// Emit the line
	if p.onLine != nil {
		p.onLine(line)
	}
}

// Flush flushes any remaining partial line
func (p *OutputProcessor) Flush() {
	if p.partialLine.Len() > 0 {
		line := p.partialLine.String()
		p.partialLine.Reset()
		if line != "" {
			p.emitIfNew(line)
		}
	}
}
