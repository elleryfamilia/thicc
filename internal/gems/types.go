package gems

import (
	"time"
)

// GemType represents the category of insight
type GemType string

const (
	GemDecision  GemType = "decision"  // Architectural or design choice
	GemDiscovery GemType = "discovery" // Unexpected finding
	GemGotcha    GemType = "gotcha"    // Non-obvious pitfall
	GemPattern   GemType = "pattern"   // Reusable solution
	GemIssue     GemType = "issue"     // Bug and resolution
	GemContext   GemType = "context"   // Important background info
)

// AllGemTypes returns all valid gem types
func AllGemTypes() []GemType {
	return []GemType{
		GemDecision,
		GemDiscovery,
		GemGotcha,
		GemPattern,
		GemIssue,
		GemContext,
	}
}

// IsValid checks if a gem type is valid
func (t GemType) IsValid() bool {
	for _, valid := range AllGemTypes() {
		if t == valid {
			return true
		}
	}
	return false
}

// Description returns a human-readable description of the gem type
func (t GemType) Description() string {
	switch t {
	case GemDecision:
		return "Architectural or design choice with lasting impact"
	case GemDiscovery:
		return "Unexpected finding during development"
	case GemGotcha:
		return "Non-obvious pitfall or edge case"
	case GemPattern:
		return "Reusable solution or approach"
	case GemIssue:
		return "Bug or problem encountered and how it was resolved"
	case GemContext:
		return "Important background info for understanding code"
	default:
		return "Unknown gem type"
	}
}

// Gem represents a valuable insight extracted from an AI coding session
type Gem struct {
	ID        string         `json:"id"`
	Type      GemType        `json:"type"`
	Title     string         `json:"title"`   // Short descriptive title (< 60 chars)
	Summary   string         `json:"summary"` // One-line summary
	Created   time.Time      `json:"created"`
	Commit    string         `json:"commit,omitempty"`  // Git commit SHA (null if uncommitted)
	Client    string         `json:"client"`            // AI tool (claude-code, aider, cursor, etc.)
	Model     string         `json:"model"`             // LLM model used
	Tags      []string       `json:"tags"`              // Categorization tags
	Files     []string       `json:"files"`             // Related files
	Content   map[string]any `json:"content"`           // Type-specific content fields
	UserNotes string         `json:"user_notes,omitempty"` // Manual additions from review
}

// GemFile represents the JSON structure of .agent-gems.json
type GemFile struct {
	Version int   `json:"version"`
	Gems    []Gem `json:"gems"`
}

// NewGemFile creates a new empty gem file
func NewGemFile() *GemFile {
	return &GemFile{
		Version: 1,
		Gems:    []Gem{},
	}
}

// PendingGemFile represents the JSON structure of pending-gems.json
type PendingGemFile struct {
	Version int   `json:"version"`
	Gems    []Gem `json:"gems"`
}

// NewPendingGemFile creates a new empty pending gem file
func NewPendingGemFile() *PendingGemFile {
	return &PendingGemFile{
		Version: 1,
		Gems:    []Gem{},
	}
}

// ContentRationale is a helper for decision-type content
type ContentRationale struct {
	Rationale      []string `json:"rationale,omitempty"`
	Implementation []string `json:"implementation,omitempty"`
	Gotchas        []string `json:"gotchas,omitempty"`
}

// ContentIssue is a helper for issue-type content
type ContentIssue struct {
	Cause      string `json:"cause,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

// ContentPattern is a helper for pattern-type content
type ContentPattern struct {
	Pattern string   `json:"pattern,omitempty"`
	Usage   []string `json:"usage,omitempty"`
}
