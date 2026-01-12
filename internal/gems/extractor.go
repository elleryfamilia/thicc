package gems

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

// Extractor manages chunked gem extraction from streaming session output
type Extractor struct {
	summarizer Summarizer
	store      *Store
	threshold  int // tokens before extraction

	// State
	mu              sync.Mutex
	buffer          strings.Builder
	tokenCount      int
	lastChunkEnd    string // End of last chunk for context overlap
	incompleteGems  []Gem  // Gems from incomplete extractions
	extractedGems   []Gem  // All gems extracted so far
	extractionCount int

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
	ProjectRoot string // Project root for gem storage
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
	log.Printf("GEMS: Extraction #%d triggered (%d tokens)", e.extractionCount, e.tokenCount)

	// Run extraction (this can be slow, but we're holding the lock)
	// In a production system, you might want to do this async
	result, err := e.summarizer.Extract(fullText, "", e.extractedGems)
	if err != nil {
		log.Printf("GEMS: Extraction failed: %v", err)
		// Don't lose the buffer on error, but prevent infinite retries
		e.resetBufferLocked()
		return err
	}

	// Handle extracted gems
	if len(result.Gems) > 0 {
		log.Printf("GEMS: Found %d gems in chunk", len(result.Gems))
		for i := range result.Gems {
			result.Gems[i].Created = time.Now()
			result.Gems[i].Client = e.client
		}

		if result.Incomplete {
			// Save for potential merging with next chunk
			e.incompleteGems = append(e.incompleteGems, result.Gems...)
		} else {
			// Finalize gems
			e.extractedGems = append(e.extractedGems, result.Gems...)
			// Clear incomplete gems if we got a complete extraction
			e.incompleteGems = nil
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

// Finalize performs final extraction and saves all pending gems
func (e *Extractor) Finalize() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Extract any remaining content
	if e.buffer.Len() > 0 && e.summarizer != nil {
		if err := e.extractLocked(); err != nil {
			log.Printf("GEMS: Final extraction failed: %v", err)
		}
	}

	// Merge incomplete gems into extracted
	if len(e.incompleteGems) > 0 {
		e.extractedGems = append(e.extractedGems, e.incompleteGems...)
		e.incompleteGems = nil
	}

	// Deduplicate gems
	e.extractedGems = deduplicateGems(e.extractedGems)

	// Save all gems to pending
	if len(e.extractedGems) > 0 && e.store != nil {
		log.Printf("GEMS: Saving %d gems to pending", len(e.extractedGems))
		for _, gem := range e.extractedGems {
			gemCopy := gem
			if err := e.store.AddPendingGem(&gemCopy); err != nil {
				log.Printf("GEMS: Failed to save gem %s: %v", gem.ID, err)
			}
		}
	}

	return nil
}

// GetExtractedGems returns all gems extracted so far
func (e *Extractor) GetExtractedGems() []Gem {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make([]Gem, len(e.extractedGems))
	copy(result, e.extractedGems)
	return result
}

// GetPendingCount returns the count of gems pending finalization
func (e *Extractor) GetPendingCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.extractedGems) + len(e.incompleteGems)
}

// deduplicateGems removes duplicate gems based on title similarity
func deduplicateGems(gems []Gem) []Gem {
	if len(gems) <= 1 {
		return gems
	}

	seen := make(map[string]bool)
	result := make([]Gem, 0, len(gems))

	for _, g := range gems {
		// Create a key from normalized title
		key := normalizeTitle(g.Title)
		if !seen[key] {
			seen[key] = true
			result = append(result, g)
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

// SetStore sets the gem store (can be called after creation)
func (e *Extractor) SetStore(store *Store) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.store = store
}
