package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ellery/thicc/internal/llmhistory"
)

// ToolDefinition defines an MCP tool
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Tools implements the MCP tools for LLM history
type Tools struct {
	store *llmhistory.Store
}

// NewTools creates a new Tools instance
func NewTools(store *llmhistory.Store) *Tools {
	return &Tools{store: store}
}

// List returns all available tools
func (t *Tools) List() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "search_history",
			Description: "Search past LLM sessions and tool uses for relevant context. Use this to find previous discussions, decisions, or work done on specific topics or files.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search term to find in tool inputs and outputs",
					},
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Optional: filter by project directory path",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results (default: 10)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "list_sessions",
			Description: "List recent LLM sessions. Shows when sessions occurred, which tool was used, and the project directory.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Optional: filter by project directory path",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of sessions to return (default: 20)",
					},
				},
				"required": []string{},
			},
		},
		{
			Name:        "get_session",
			Description: "Get detailed information about a specific session, including all tool uses within that session.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session ID to retrieve",
					},
				},
				"required": []string{"session_id"},
			},
		},
		{
			Name:        "get_file_history",
			Description: "Get history of tool uses that touched specific files. Useful for understanding what changes were made to files and when.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"files": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "List of file paths to search for",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results (default: 50)",
					},
				},
				"required": []string{"files"},
			},
		},
	}
}

// Call invokes a tool by name with the given arguments
func (t *Tools) Call(name string, args map[string]interface{}) (string, error) {
	switch name {
	case "search_history":
		return t.searchHistory(args)
	case "list_sessions":
		return t.listSessions(args)
	case "get_session":
		return t.getSession(args)
	case "get_file_history":
		return t.getFileHistory(args)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// searchHistory implements the search_history tool
func (t *Tools) searchHistory(args map[string]interface{}) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	project, _ := args["project"].(string)
	limit := getIntArg(args, "limit", 10)

	results, err := t.store.Search(query, project, limit)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return "No results found for query: " + query, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d results for '%s':\n\n", len(results), query))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("## Result %d\n", i+1))
		sb.WriteString(fmt.Sprintf("- **Tool**: %s\n", r.ToolUse.ToolName))
		sb.WriteString(fmt.Sprintf("- **Time**: %s\n", r.ToolUse.Timestamp.Format("2006-01-02 15:04:05")))
		sb.WriteString(fmt.Sprintf("- **Session**: %s\n", r.SessionID))
		sb.WriteString(fmt.Sprintf("- **Input**: %s\n", truncate(r.ToolUse.Input, 200)))
		if r.Snippet != "" {
			sb.WriteString(fmt.Sprintf("- **Snippet**: %s\n", r.Snippet))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// listSessions implements the list_sessions tool
func (t *Tools) listSessions(args map[string]interface{}) (string, error) {
	project, _ := args["project"].(string)
	limit := getIntArg(args, "limit", 20)

	sessions, err := t.store.ListSessions(project, limit)
	if err != nil {
		return "", fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		return "No sessions found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d sessions:\n\n", len(sessions)))

	for i, s := range sessions {
		duration := ""
		if !s.EndTime.IsZero() {
			duration = fmt.Sprintf(" (duration: %v)", s.EndTime.Sub(s.StartTime).Round(1e9))
		}

		sb.WriteString(fmt.Sprintf("## Session %d: %s\n", i+1, s.ID[:8]))
		sb.WriteString(fmt.Sprintf("- **Tool**: %s\n", s.ToolName))
		sb.WriteString(fmt.Sprintf("- **Started**: %s%s\n", s.StartTime.Format("2006-01-02 15:04:05"), duration))
		sb.WriteString(fmt.Sprintf("- **Project**: %s\n", s.ProjectDir))
		sb.WriteString(fmt.Sprintf("- **Output**: %d bytes\n", s.OutputBytes))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// getSession implements the get_session tool
func (t *Tools) getSession(args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id is required")
	}

	session, err := t.store.GetSession(sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to get session: %w", err)
	}

	// Use the full session ID for getting tool uses
	toolUses, err := t.store.GetToolUsesForSession(session.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get tool uses: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Session %s\n\n", session.ID))
	sb.WriteString(fmt.Sprintf("- **Tool**: %s\n", session.ToolName))
	sb.WriteString(fmt.Sprintf("- **Command**: %s\n", session.ToolCommand))
	sb.WriteString(fmt.Sprintf("- **Project**: %s\n", session.ProjectDir))
	sb.WriteString(fmt.Sprintf("- **Started**: %s\n", session.StartTime.Format("2006-01-02 15:04:05")))
	if !session.EndTime.IsZero() {
		sb.WriteString(fmt.Sprintf("- **Ended**: %s\n", session.EndTime.Format("2006-01-02 15:04:05")))
		sb.WriteString(fmt.Sprintf("- **Duration**: %v\n", session.EndTime.Sub(session.StartTime).Round(1e9)))
	}
	sb.WriteString(fmt.Sprintf("- **Output**: %d bytes\n", session.OutputBytes))
	sb.WriteString("\n")

	if len(toolUses) > 0 {
		sb.WriteString(fmt.Sprintf("## Tool Uses (%d)\n\n", len(toolUses)))
		for i, tu := range toolUses {
			sb.WriteString(fmt.Sprintf("### %d. %s\n", i+1, tu.ToolName))
			sb.WriteString(fmt.Sprintf("- **Time**: %s\n", tu.Timestamp.Format("15:04:05")))
			sb.WriteString(fmt.Sprintf("- **Input**: %s\n", truncate(tu.Input, 500)))
			if tu.Output != "" {
				sb.WriteString(fmt.Sprintf("- **Output** (first 500 chars):\n```\n%s\n```\n", truncate(tu.Output, 500)))
			}
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("No tool uses recorded for this session.\n")
	}

	return sb.String(), nil
}

// getFileHistory implements the get_file_history tool
func (t *Tools) getFileHistory(args map[string]interface{}) (string, error) {
	filesRaw, ok := args["files"]
	if !ok {
		return "", fmt.Errorf("files is required")
	}

	// Convert to []string
	var files []string
	switch v := filesRaw.(type) {
	case []interface{}:
		for _, f := range v {
			if s, ok := f.(string); ok {
				files = append(files, s)
			}
		}
	case []string:
		files = v
	default:
		// Try JSON unmarshaling as fallback
		data, _ := json.Marshal(filesRaw)
		if err := json.Unmarshal(data, &files); err != nil {
			return "", fmt.Errorf("files must be an array of strings")
		}
	}

	if len(files) == 0 {
		return "", fmt.Errorf("files array is empty")
	}

	limit := getIntArg(args, "limit", 50)

	toolUses, err := t.store.GetFileHistory(files, limit)
	if err != nil {
		return "", fmt.Errorf("failed to get file history: %w", err)
	}

	if len(toolUses) == 0 {
		return fmt.Sprintf("No history found for files: %v", files), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# File History\n\nFiles: %v\n\n", files))
	sb.WriteString(fmt.Sprintf("Found %d tool uses:\n\n", len(toolUses)))

	for i, tu := range toolUses {
		sb.WriteString(fmt.Sprintf("## %d. %s\n", i+1, tu.ToolName))
		sb.WriteString(fmt.Sprintf("- **Time**: %s\n", tu.Timestamp.Format("2006-01-02 15:04:05")))
		sb.WriteString(fmt.Sprintf("- **Session**: %s\n", tu.SessionID[:8]))
		sb.WriteString(fmt.Sprintf("- **Input**: %s\n", truncate(tu.Input, 200)))
		if tu.Output != "" {
			sb.WriteString(fmt.Sprintf("- **Output** (first 300 chars):\n```\n%s\n```\n", truncate(tu.Output, 300)))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// getIntArg extracts an int argument with a default value
func getIntArg(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case int64:
			return int(n)
		}
	}
	return defaultVal
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
