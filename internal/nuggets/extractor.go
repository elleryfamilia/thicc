package nuggets

import (
	"log"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultExtractionThreshold is the default token count before triggering extraction
	DefaultExtractionThreshold = 4000
	// BytesPerToken is the approximate bytes per token ratio
	BytesPerToken = 4
	// ContextOverlap is how much of the previous chunk to include for context
	ContextOverlap = 500 // tokens
)

// Extractor manages chunked nugget extraction from streaming session output
type Extractor struct {
	summarizer Summarizer
	store      *Store
	threshold  int // tokens before extraction

	// State
	mu                  sync.Mutex
	buffer              strings.Builder
	tokenCount          int
	lastChunkEnd        string   // End of last chunk for context overlap
	incompleteNuggets   []Nugget // Nuggets from incomplete extractions
	extractedNuggets    []Nugget // All nuggets extracted so far
	extractionCount     int

	// Configuration
	projectRoot string
	client      string
	model       string
}

// ExtractorConfig holds configuration for the extractor
type ExtractorConfig struct {
	Summarizer  Summarizer
	Store       *Store
	Threshold   int    // Token threshold (default: 4000)
	ProjectRoot string // Project root for nugget storage
}

// NewExtractor creates a new extraction orchestrator
func NewExtractor(config ExtractorConfig) *Extractor {
	threshold := config.Threshold
	if threshold <= 0 {
		threshold = DefaultExtractionThreshold
	}

	e := &Extractor{
		summarizer:  config.Summarizer,
		store:       config.Store,
		threshold:   threshold,
		projectRoot: config.ProjectRoot,
	}

	if config.Summarizer != nil {
		e.client = config.Summarizer.Name()
	}

	return e
}

// ProcessChunk adds text to the buffer and triggers extraction if threshold reached
func (e *Extractor) ProcessChunk(text string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.buffer.WriteString(text)

	// Approximate token count (bytes / 4)
	e.tokenCount = e.buffer.Len() / BytesPerToken

	// Check if we should extract
	if e.tokenCount >= e.threshold {
		return e.extractLocked()
	}

	return nil
}

// extractLocked performs extraction (must hold mu)
func (e *Extractor) extractLocked() error {
	if e.summarizer == nil {
		// No summarizer configured, just clear buffer
		e.resetBufferLocked()
		return nil
	}

	text := e.buffer.String()
	if text == "" {
		return nil
	}

	// Include context from previous chunk if available
	fullText := text
	if e.lastChunkEnd != "" {
		fullText = e.lastChunkEnd + "\n...\n" + text
	}

	e.extractionCount++
	log.Printf("NUGGETS: Extraction #%d triggered (%d tokens)", e.extractionCount, e.tokenCount)

	// Run extraction (this can be slow, but we're holding the lock)
	// In a production system, you might want to do this async
	result, err := e.summarizer.Extract(fullText, "", e.extractedNuggets)
	if err != nil {
		log.Printf("NUGGETS: Extraction failed: %v", err)
		// Don't lose the buffer on error, but prevent infinite retries
		e.resetBufferLocked()
		return err
	}

	// Handle extracted nuggets
	if len(result.Nuggets) > 0 {
		log.Printf("NUGGETS: Found %d nuggets in chunk", len(result.Nuggets))
		for i := range result.Nuggets {
			result.Nuggets[i].Created = time.Now()
			result.Nuggets[i].Client = e.client
		}

		if result.Incomplete {
			// Save for potential merging with next chunk
			e.incompleteNuggets = append(e.incompleteNuggets, result.Nuggets...)
		} else {
			// Finalize nuggets
			e.extractedNuggets = append(e.extractedNuggets, result.Nuggets...)
			// Clear incomplete nuggets if we got a complete extraction
			e.incompleteNuggets = nil
		}
	}

	// Save context for next chunk
	e.saveContextLocked()

	// Reset buffer
	e.resetBufferLocked()

	return nil
}

// saveContextLocked saves the end of current buffer for context overlap
func (e *Extractor) saveContextLocked() {
	text := e.buffer.String()
	overlapBytes := ContextOverlap * BytesPerToken

	if len(text) > overlapBytes {
		e.lastChunkEnd = text[len(text)-overlapBytes:]
	} else {
		e.lastChunkEnd = text
	}
}

// resetBufferLocked resets the buffer (must hold mu)
func (e *Extractor) resetBufferLocked() {
	e.buffer.Reset()
	e.tokenCount = 0
}

// Finalize performs final extraction and saves all pending nuggets
func (e *Extractor) Finalize() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Extract any remaining content
	if e.buffer.Len() > 0 && e.summarizer != nil {
		if err := e.extractLocked(); err != nil {
			log.Printf("NUGGETS: Final extraction failed: %v", err)
		}
	}

	// Merge incomplete nuggets into extracted
	if len(e.incompleteNuggets) > 0 {
		e.extractedNuggets = append(e.extractedNuggets, e.incompleteNuggets...)
		e.incompleteNuggets = nil
	}

	// Deduplicate nuggets
	e.extractedNuggets = deduplicateNuggets(e.extractedNuggets)

	// Save all nuggets to pending
	if len(e.extractedNuggets) > 0 && e.store != nil {
		log.Printf("NUGGETS: Saving %d nuggets to pending", len(e.extractedNuggets))
		for _, nugget := range e.extractedNuggets {
			nuggetCopy := nugget
			if err := e.store.AddPendingNugget(&nuggetCopy); err != nil {
				log.Printf("NUGGETS: Failed to save nugget %s: %v", nugget.ID, err)
			}
		}
	}

	return nil
}

// GetExtractedNuggets returns all nuggets extracted so far
func (e *Extractor) GetExtractedNuggets() []Nugget {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make([]Nugget, len(e.extractedNuggets))
	copy(result, e.extractedNuggets)
	return result
}

// GetPendingCount returns the count of nuggets pending finalization
func (e *Extractor) GetPendingCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.extractedNuggets) + len(e.incompleteNuggets)
}

// deduplicateNuggets removes duplicate nuggets based on title similarity
func deduplicateNuggets(nuggets []Nugget) []Nugget {
	if len(nuggets) <= 1 {
		return nuggets
	}

	seen := make(map[string]bool)
	result := make([]Nugget, 0, len(nuggets))

	for _, n := range nuggets {
		// Create a key from normalized title
		key := normalizeTitle(n.Title)
		if !seen[key] {
			seen[key] = true
			result = append(result, n)
		}
	}

	return result
}

// normalizeTitle normalizes a title for deduplication comparison
func normalizeTitle(title string) string {
	// Lowercase and remove extra whitespace
	title = strings.ToLower(strings.TrimSpace(title))
	// Replace multiple spaces with single space
	for strings.Contains(title, "  ") {
		title = strings.ReplaceAll(title, "  ", " ")
	}
	return title
}

// TokenCount returns the current token count in the buffer
func (e *Extractor) TokenCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.tokenCount
}

// SetSummarizer sets the summarizer (can be called after creation)
func (e *Extractor) SetSummarizer(s Summarizer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.summarizer = s
	if s != nil {
		e.client = s.Name()
	}
}

// SetStore sets the nugget store (can be called after creation)
func (e *Extractor) SetStore(store *Store) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.store = store
}
