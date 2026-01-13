package llmhistory

import (
	"time"
)

// Session represents a single LLM CLI session (e.g., one invocation of claude)
type Session struct {
	ID          string    `json:"id"`
	ToolName    string    `json:"tool_name"`    // "claude", "aider", "gemini", etc.
	ToolCommand string    `json:"tool_command"` // Full command (e.g., "claude --dangerously-skip-permissions")
	ProjectDir  string    `json:"project_dir"`  // Working directory
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time,omitempty"`
	OutputBytes int64     `json:"output_bytes"` // Total bytes of terminal output
}

// ToolUse represents a single tool invocation within a session
// (e.g., Read, Edit, Bash, Grep, etc.)
type ToolUse struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Timestamp time.Time `json:"timestamp"`
	ToolName  string    `json:"tool_name"` // "Read", "Edit", "Bash", "Grep", etc.
	Input     string    `json:"input"`     // Tool input (file path, command, etc.)
	Output    string    `json:"output"`    // Tool output (truncated if large)
}

// FileTouch records when a tool use touched a specific file
type FileTouch struct {
	ID        int64  `json:"id"`
	ToolUseID string `json:"tool_use_id"`
	FilePath  string `json:"file_path"`
}

// SessionWithToolUses is a session with its associated tool uses
type SessionWithToolUses struct {
	Session  Session   `json:"session"`
	ToolUses []ToolUse `json:"tool_uses"`
}

// SearchResultType indicates the source of a search result
type SearchResultType string

const (
	SearchResultToolUse       SearchResultType = "tool_use"
	SearchResultSessionOutput SearchResultType = "session_output"
)

// SearchResult represents a search hit from FTS5
type SearchResult struct {
	Type      SearchResultType `json:"type"`                 // "tool_use" or "session_output"
	ToolUse   *ToolUse         `json:"tool_use,omitempty"`   // Set if Type == "tool_use"
	SessionID string           `json:"session_id"`           // Always set
	Snippet   string           `json:"snippet"`              // Highlighted snippet from FTS5
	Score     float64          `json:"score"`                // BM25 relevance score
	Timestamp time.Time        `json:"timestamp,omitempty"`  // For tool_use, the tool timestamp
}

// FileHistory represents the history of tool uses for a file
type FileHistory struct {
	FilePath string    `json:"file_path"`
	ToolUses []ToolUse `json:"tool_uses"`
}

// DBStats contains database statistics
type DBStats struct {
	FileSizeBytes   int64     `json:"file_size_bytes"`
	FileSizeMB      float64   `json:"file_size_mb"`
	SessionCount    int       `json:"session_count"`
	ToolUseCount    int       `json:"tool_use_count"`
	OutputCount     int       `json:"output_count"`
	OldestSession   time.Time `json:"oldest_session,omitempty"`
	NewestSession   time.Time `json:"newest_session,omitempty"`
}
