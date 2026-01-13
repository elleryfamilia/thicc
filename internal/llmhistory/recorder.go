package llmhistory

import (
	"bytes"
	"log"
	"sync"
	"time"

	"github.com/ellery/thicc/internal/nuggets"
	"github.com/google/uuid"
)

// Recorder manages session recording using scrollback-based capture
type Recorder struct {
	store     *Store
	session   *Session
	mu        sync.Mutex
	isRunning bool

	// Output buffer (receives clean lines from scrollback)
	outputBuffer bytes.Buffer
	outputMu     sync.Mutex

	// Background sync
	stopSync chan struct{}

	// Nugget extraction
	extractor         *nuggets.Extractor
	extractionEnabled bool
}

// NewRecorder creates a new session recorder
func NewRecorder(store *Store, toolName, toolCommand, projectDir string) (*Recorder, error) {
	session := &Session{
		ID:          uuid.New().String(),
		ToolName:    toolName,
		ToolCommand: toolCommand,
		ProjectDir:  projectDir,
		StartTime:   time.Now(),
		OutputBytes: 0,
	}

	if err := store.CreateSession(session); err != nil {
		return nil, err
	}

	r := &Recorder{
		store:     store,
		session:   session,
		isRunning: true,
		stopSync:  make(chan struct{}),
	}

	// Start background sync
	go r.backgroundSync()

	log.Printf("LLMHISTORY: Started recording session %s for %s", session.ID[:8], toolName)

	return r, nil
}

// OnScrolledLine receives a clean line from the scrollback buffer
// This is called when content scrolls off the top of the terminal
func (r *Recorder) OnScrolledLine(line string) {
	r.mu.Lock()
	running := r.isRunning
	r.mu.Unlock()

	if !running || line == "" {
		return
	}

	r.outputMu.Lock()
	defer r.outputMu.Unlock()

	// Strip any remaining ANSI codes (scrollback should be clean, but just in case)
	clean := stripANSI(line)
	if clean == "" {
		return
	}

	r.outputBuffer.WriteString(clean)
	r.outputBuffer.WriteByte('\n')

	// DEBUG: Log first few lines and periodically
	lineCount := r.outputBuffer.Len()
	if lineCount <= 500 || lineCount%5000 == 0 {
		truncated := clean
		if len(truncated) > 80 {
			truncated = truncated[:80] + "..."
		}
		log.Printf("LLMHISTORY: OnScrolledLine() stored: %s (buffer: %d bytes)", truncated, lineCount)
	}

	// Feed to extractor if enabled
	if r.extractionEnabled && r.extractor != nil {
		go func(text string) {
			if err := r.extractor.ProcessChunk(text + "\n"); err != nil {
				log.Printf("LLMHISTORY: Extraction error: %v", err)
			}
		}(clean)
	}
}

// FlushLiveScreen captures remaining visible content that hasn't scrolled yet
// Call this when the session is ending to capture final output
func (r *Recorder) FlushLiveScreen(lines []string) {
	r.mu.Lock()
	running := r.isRunning
	r.mu.Unlock()

	if !running {
		return
	}

	r.outputMu.Lock()
	defer r.outputMu.Unlock()

	for _, line := range lines {
		clean := stripANSI(line)
		if clean != "" {
			r.outputBuffer.WriteString(clean)
			r.outputBuffer.WriteByte('\n')
		}
	}

	log.Printf("LLMHISTORY: FlushLiveScreen() added %d lines", len(lines))
}

// backgroundSync periodically syncs session data to DB
func (r *Recorder) backgroundSync() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.syncSession()
		case <-r.stopSync:
			return
		}
	}
}

// syncSession writes current session state to database
func (r *Recorder) syncSession() {
	r.mu.Lock()
	if !r.isRunning {
		r.mu.Unlock()
		return
	}

	// Get current output
	r.outputMu.Lock()
	outputLen := int64(r.outputBuffer.Len())
	r.outputMu.Unlock()

	r.session.OutputBytes = outputLen
	if err := r.store.UpdateSession(r.session); err != nil {
		log.Printf("LLMHISTORY: Failed to sync session: %v", err)
	}
	r.mu.Unlock()
}

// Stop stops recording and finalizes the session
func (r *Recorder) Stop() error {
	r.mu.Lock()
	if !r.isRunning {
		r.mu.Unlock()
		return nil
	}
	r.isRunning = false
	r.mu.Unlock()

	// Stop background sync
	select {
	case <-r.stopSync:
	default:
		close(r.stopSync)
	}

	time.Sleep(50 * time.Millisecond)

	// Get final output
	r.outputMu.Lock()
	output := r.outputBuffer.String()
	r.outputMu.Unlock()

	// DEBUG: Log final state
	log.Printf("LLMHISTORY: Stop() - outputBuffer: %d bytes", len(output))

	// Final sync
	r.mu.Lock()
	r.session.OutputBytes = int64(len(output))
	r.session.EndTime = time.Now()
	r.mu.Unlock()

	// Save output to database
	if err := r.store.SaveSessionOutput(r.session.ID, output); err != nil {
		log.Printf("LLMHISTORY: Failed to save session output: %v", err)
	}

	if err := r.store.UpdateSession(r.session); err != nil {
		log.Printf("LLMHISTORY: Failed to update session: %v", err)
		return err
	}

	log.Printf("LLMHISTORY: Stopped session %s (duration: %v, output: %d bytes)",
		r.session.ID[:8],
		r.session.EndTime.Sub(r.session.StartTime).Round(time.Second),
		r.session.OutputBytes)

	// Finalize nugget extraction if enabled
	r.mu.Lock()
	extractor := r.extractor
	r.mu.Unlock()

	if extractor != nil {
		log.Printf("LLMHISTORY: Finalizing nugget extraction...")
		if err := extractor.Finalize(); err != nil {
			log.Printf("LLMHISTORY: Nugget extraction finalization failed: %v", err)
		} else {
			count := extractor.GetPendingCount()
			if count > 0 {
				log.Printf("LLMHISTORY: Extracted %d nuggets (pending review)", count)
			}
		}
	}

	// Enforce size limit if configured
	maxSize := GetMaxSizeBytes()
	if maxSize > 0 {
		currentSize, _ := r.store.GetDBSize()
		threshold := int64(float64(maxSize) * CleanupThreshold)
		if currentSize > threshold {
			deleted, err := r.store.EnforceSizeLimit(maxSize)
			if err != nil {
				log.Printf("LLMHISTORY: Size enforcement failed: %v", err)
			} else if deleted > 0 {
				log.Printf("LLMHISTORY: Cleaned up %d old sessions to stay under size limit", deleted)
			}
		}
	}

	return nil
}

// SessionID returns the current session ID
func (r *Recorder) SessionID() string {
	return r.session.ID
}

// EnableExtraction enables nugget extraction with the given summarizer
func (r *Recorder) EnableExtraction(summarizer nuggets.Summarizer, nuggetStore *nuggets.Store, threshold int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if threshold <= 0 {
		threshold = nuggets.DefaultExtractionThreshold
	}

	r.extractor = nuggets.NewExtractor(nuggets.ExtractorConfig{
		Summarizer:  summarizer,
		Store:       nuggetStore,
		Threshold:   threshold,
		ProjectRoot: r.session.ProjectDir,
	})
	r.extractionEnabled = true

	log.Printf("LLMHISTORY: Nugget extraction enabled (threshold: %d tokens)", threshold)
}

// DisableExtraction disables nugget extraction
func (r *Recorder) DisableExtraction() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.extractionEnabled = false
}

// GetExtractedNuggetCount returns the number of gems extracted so far
func (r *Recorder) GetExtractedNuggetCount() int {
	r.mu.Lock()
	extractor := r.extractor
	r.mu.Unlock()

	if extractor == nil {
		return 0
	}
	return extractor.GetPendingCount()
}

// stripANSI removes ANSI escape codes from a string
func stripANSI(s string) string {
	// Simple state machine to strip ANSI sequences
	var result bytes.Buffer
	inEscape := false

	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			// End of escape sequence
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEscape = false
			}
			continue
		}
		result.WriteByte(s[i])
	}

	return result.String()
}
