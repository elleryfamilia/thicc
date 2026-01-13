package nuggets

import (
	"time"
)

// NuggetType represents the category of insight
type NuggetType string

const (
	NuggetDecision  NuggetType = "decision"  // Architectural or design choice
	NuggetDiscovery NuggetType = "discovery" // Unexpected finding
	NuggetGotcha    NuggetType = "gotcha"    // Non-obvious pitfall
	NuggetPattern   NuggetType = "pattern"   // Reusable solution
	NuggetIssue     NuggetType = "issue"     // Bug and resolution
	NuggetContext   NuggetType = "context"   // Important background info
)

// AllNuggetTypes returns all valid nugget types
func AllNuggetTypes() []NuggetType {
	return []NuggetType{
		NuggetDecision,
		NuggetDiscovery,
		NuggetGotcha,
		NuggetPattern,
		NuggetIssue,
		NuggetContext,
	}
}

// IsValid checks if a nugget type is valid
func (t NuggetType) IsValid() bool {
	for _, valid := range AllNuggetTypes() {
		if t == valid {
			return true
		}
	}
	return false
}

// Description returns a human-readable description of the nugget type
func (t NuggetType) Description() string {
	switch t {
	case NuggetDecision:
		return "Architectural or design choice with lasting impact"
	case NuggetDiscovery:
		return "Unexpected finding during development"
	case NuggetGotcha:
		return "Non-obvious pitfall or edge case"
	case NuggetPattern:
		return "Reusable solution or approach"
	case NuggetIssue:
		return "Bug or problem encountered and how it was resolved"
	case NuggetContext:
		return "Important background info for understanding code"
	default:
		return "Unknown nugget type"
	}
}

// Nugget represents a valuable insight extracted from an AI coding session
type Nugget struct {
	ID           string         `json:"id"`
	Type         NuggetType     `json:"type"`
	Title        string         `json:"title"`        // Short descriptive title (< 60 chars)
	Summary      string         `json:"summary"`      // One-line summary
	Significance string         `json:"significance"` // Why this is worth keeping as a nugget
	Created      time.Time      `json:"created"`
	Commit       string         `json:"commit,omitempty"` // Git commit SHA (null if uncommitted)
	Client       string         `json:"client"`           // AI tool (claude-code, aider, cursor, etc.)
	Model        string         `json:"model"`            // LLM model used
	Tags         []string       `json:"tags"`             // Categorization tags
	Files        []string       `json:"files"`            // Related files
	Content      map[string]any `json:"content"`          // Type-specific content fields
	UserNotes    string         `json:"user_notes,omitempty"` // Manual additions from review
}

// NuggetFile represents the JSON structure of .agent-nuggets.json
type NuggetFile struct {
	Version int      `json:"version"`
	Nuggets []Nugget `json:"nuggets"`
}

// NewNuggetFile creates a new empty nugget file
func NewNuggetFile() *NuggetFile {
	return &NuggetFile{
		Version: 1,
		Nuggets: []Nugget{},
	}
}

// PendingNuggetFile represents the JSON structure of pending-nuggets.json
type PendingNuggetFile struct {
	Version int      `json:"version"`
	Nuggets []Nugget `json:"nuggets"`
}

// NewPendingNuggetFile creates a new empty pending nugget file
func NewPendingNuggetFile() *PendingNuggetFile {
	return &PendingNuggetFile{
		Version: 1,
		Nuggets: []Nugget{},
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
