package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ellery/thicc/internal/config"
	"github.com/ellery/thicc/internal/dashboard"
	"github.com/ellery/thicc/internal/llmhistory"
	"github.com/ellery/thicc/internal/mcp"
)

// handleMCPCommand handles the "mcp" subcommand
// Returns true if the command was handled (and program should exit)
func handleMCPCommand(args []string) bool {
	if len(args) < 2 || args[1] != "mcp" {
		return false
	}

	// Initialize config directory (needed for database path)
	_ = config.InitConfigDir("")

	// Parse mcp subcommand
	if len(args) < 3 {
		printMCPUsage()
		os.Exit(1)
	}

	switch args[2] {
	case "serve":
		runMCPServer()
	case "sessions":
		listSessions(args[3:])
	case "session":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: thicc mcp session <session-id>")
			os.Exit(1)
		}
		showSession(args[3])
	case "output":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: thicc mcp output <session-id>")
			os.Exit(1)
		}
		showSessionOutput(args[3])
	case "delete":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: thicc mcp delete <session-id>")
			os.Exit(1)
		}
		deleteSession(args[3])
	case "search":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: thicc mcp search <query>")
			os.Exit(1)
		}
		searchHistory(args[3])
	case "hook":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: thicc mcp hook <hook-name>")
			fmt.Fprintln(os.Stderr, "Available hooks: post-tool-use")
			os.Exit(1)
		}
		handleHook(args[3], args[4:])
	case "stats":
		showStats()
	case "compact":
		runCompact()
	case "cleanup":
		days := 90
		if len(args) >= 4 {
			if n, err := strconv.Atoi(args[3]); err == nil && n > 0 {
				days = n
			}
		}
		runCleanup(days)
	case "limit":
		sizeMB := llmhistory.DefaultMaxSizeMB
		if len(args) >= 4 {
			if n, err := strconv.Atoi(args[3]); err == nil && n > 0 {
				sizeMB = n
			}
		}
		runSizeLimit(sizeMB)
	case "rebuild-fts":
		runRebuildFTS()
	case "help", "-h", "--help":
		printMCPUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown mcp command: %s\n", args[2])
		printMCPUsage()
		os.Exit(1)
	}

	return true
}

// printMCPUsage prints usage information for the mcp subcommand
func printMCPUsage() {
	fmt.Println("Usage: thicc mcp <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  serve              Start the MCP server (stdio)")
	fmt.Println("  sessions [limit]   List recent sessions (default: 20)")
	fmt.Println("  session <id>       Show session details and tool uses")
	fmt.Println("  output <id>        Show raw session output text")
	fmt.Println("  delete <id>        Delete a session by ID")
	fmt.Println("  search <query>     Search history for a term")
	fmt.Println("  hook <name>        Handle Claude CLI hooks (post-tool-use)")
	fmt.Println("  stats              Show database statistics")
	fmt.Println("  compact            Vacuum database to reclaim space")
	fmt.Println("  cleanup [days]     Delete sessions older than N days (default: 90)")
	fmt.Println("  limit [size_mb]    Enforce size limit (default: 100MB)")
	fmt.Println("  rebuild-fts        Rebuild full-text search index")
	fmt.Println("  help               Show this help message")
	fmt.Println("")
	fmt.Println("The MCP server provides LLM history tools for Claude Code and other")
	fmt.Println("MCP-compatible clients.")
	fmt.Println("")
	fmt.Println("To configure Claude Code to use this server, add to ~/.claude/mcp.json:")
	fmt.Println(`  {`)
	fmt.Println(`    "mcpServers": {`)
	fmt.Println(`      "llm-history": {`)
	fmt.Println(`        "command": "thicc",`)
	fmt.Println(`        "args": ["mcp", "serve"]`)
	fmt.Println(`      }`)
	fmt.Println(`    }`)
	fmt.Println(`  }`)
	fmt.Println("")
	fmt.Println("To configure Claude CLI hooks for tool use capture, add to ~/.claude/settings.json:")
	fmt.Println(`  {`)
	fmt.Println(`    "hooks": {`)
	fmt.Println(`      "PostToolUse": [`)
	fmt.Println(`        {`)
	fmt.Println(`          "matcher": "",`)
	fmt.Println(`          "hooks": [{"type": "command", "command": "thicc mcp hook post-tool-use"}]`)
	fmt.Println(`        }`)
	fmt.Println(`      ]`)
	fmt.Println(`    }`)
	fmt.Println(`  }`)
}

// runMCPServer starts the MCP server
func runMCPServer() {
	// Check if MCP server is enabled
	if !llmhistory.IsMCPEnabled() {
		fmt.Fprintln(os.Stderr, "MCP server is disabled in settings (llmhistory.mcpenabled: false)")
		os.Exit(1)
	}

	// Set up logging to stderr (stdout is for MCP protocol)
	log.SetOutput(os.Stderr)
	log.SetPrefix("[llm-history] ")

	// Get config directory
	configDir := dashboard.GetConfigDir()

	// Create/open the store
	store, err := llmhistory.NewStore(configDir)
	if err != nil {
		log.Fatalf("Failed to open history store: %v", err)
	}
	defer store.Close()

	log.Printf("MCP server starting, database: %s", filepath.Join(configDir, llmhistory.DBFileName))

	// Create and run MCP server
	server := mcp.NewServer(store)
	if err := server.Run(); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}

// openStore opens the history store for CLI commands
func openStore() *llmhistory.Store {
	configDir := dashboard.GetConfigDir()
	store, err := llmhistory.NewStore(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open history store: %v\n", err)
		os.Exit(1)
	}
	return store
}

// listSessions lists recent sessions
func listSessions(args []string) {
	limit := 20
	if len(args) > 0 {
		if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
			limit = n
		}
	}

	store := openStore()
	defer store.Close()

	sessions, err := store.ListSessions("", limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list sessions: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return
	}

	fmt.Printf("Found %d sessions:\n\n", len(sessions))
	for _, s := range sessions {
		duration := ""
		if !s.EndTime.IsZero() {
			d := s.EndTime.Sub(s.StartTime)
			duration = fmt.Sprintf(" (%v)", d.Round(1e9))
		}
		fmt.Printf("  %s  %s  %s%s\n", s.ID[:8], s.StartTime.Format("2006-01-02 15:04"), s.ToolName, duration)
		if s.ProjectDir != "" {
			fmt.Printf("           %s\n", s.ProjectDir)
		}
	}
}

// showSession shows details for a specific session
func showSession(sessionID string) {
	store := openStore()
	defer store.Close()

	// Try to find session by prefix
	sessions, err := store.ListSessions("", 100)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list sessions: %v\n", err)
		os.Exit(1)
	}

	var session *llmhistory.Session
	for _, s := range sessions {
		if s.ID == sessionID || (len(sessionID) >= 8 && s.ID[:8] == sessionID[:8]) {
			session = &s
			break
		}
	}

	if session == nil {
		fmt.Fprintf(os.Stderr, "Session not found: %s\n", sessionID)
		os.Exit(1)
	}

	// Get full session
	fullSession, err := store.GetSession(session.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get session: %v\n", err)
		os.Exit(1)
	}

	toolUses, err := store.GetToolUsesForSession(session.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get tool uses: %v\n", err)
		os.Exit(1)
	}

	// Print session info
	fmt.Printf("Session: %s\n", fullSession.ID)
	fmt.Printf("Tool:    %s\n", fullSession.ToolName)
	fmt.Printf("Command: %s\n", fullSession.ToolCommand)
	fmt.Printf("Project: %s\n", fullSession.ProjectDir)
	fmt.Printf("Started: %s\n", fullSession.StartTime.Format("2006-01-02 15:04:05"))
	if !fullSession.EndTime.IsZero() {
		fmt.Printf("Ended:   %s\n", fullSession.EndTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("Duration: %v\n", fullSession.EndTime.Sub(fullSession.StartTime).Round(1e9))
	}
	fmt.Printf("Output:  %d bytes\n", fullSession.OutputBytes)
	fmt.Println()

	if len(toolUses) == 0 {
		fmt.Println("No tool uses recorded.")
		return
	}

	fmt.Printf("Tool Uses (%d):\n\n", len(toolUses))
	for i, tu := range toolUses {
		fmt.Printf("  %d. %s at %s\n", i+1, tu.ToolName, tu.Timestamp.Format("15:04:05"))
		fmt.Printf("     Input: %s\n", truncateStr(tu.Input, 100))
		if tu.Output != "" {
			output := truncateStr(tu.Output, 200)
			fmt.Printf("     Output: %s\n", output)
		}
		fmt.Println()
	}
}

// searchHistory searches the history for a term
func searchHistory(query string) {
	store := openStore()
	defer store.Close()

	results, err := store.Search(query, "", 20)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Search failed: %v\n", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Printf("No results found for: %s\n", query)
		return
	}

	fmt.Printf("Found %d results for '%s':\n\n", len(results), query)
	for i, r := range results {
		if r.Type == llmhistory.SearchResultToolUse && r.ToolUse != nil {
			// Tool use result
			fmt.Printf("  %d. %s - %s\n", i+1, r.ToolUse.ToolName, r.Timestamp.Format("2006-01-02 15:04"))
			fmt.Printf("     Session: %s\n", r.SessionID[:8])
			fmt.Printf("     Input: %s\n", truncateStr(r.ToolUse.Input, 80))
		} else {
			// Session output result
			fmt.Printf("  %d. [session output] - %s\n", i+1, r.Timestamp.Format("2006-01-02 15:04"))
			fmt.Printf("     Session: %s\n", r.SessionID[:8])
		}
		if r.Snippet != "" {
			fmt.Printf("     Match: %s\n", r.Snippet)
		}
		fmt.Println()
	}
}

// truncateStr truncates a string to maxLen characters
func truncateStr(s string, maxLen int) string {
	// Remove newlines for display
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// showSessionOutput shows the raw output text for a session
func showSessionOutput(sessionID string) {
	store := openStore()
	defer store.Close()

	// Find session by prefix
	sessions, err := store.ListSessions("", 100)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list sessions: %v\n", err)
		os.Exit(1)
	}

	var session *llmhistory.Session
	for _, s := range sessions {
		if s.ID == sessionID || (len(sessionID) >= 8 && len(s.ID) >= 8 && s.ID[:8] == sessionID[:8]) {
			session = &s
			break
		}
	}

	if session == nil {
		fmt.Fprintf(os.Stderr, "Session not found: %s\n", sessionID)
		os.Exit(1)
	}

	output, err := store.GetSessionOutput(session.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get session output: %v\n", err)
		os.Exit(1)
	}

	if output == "" {
		fmt.Println("No output recorded for this session.")
		return
	}

	fmt.Print(output)
}

// deleteSession deletes a session by ID
func deleteSession(sessionID string) {
	store := openStore()
	defer store.Close()

	deleted, err := store.DeleteSession(sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to delete session: %v\n", err)
		os.Exit(1)
	}

	if !deleted {
		fmt.Fprintf(os.Stderr, "Session not found: %s\n", sessionID)
		os.Exit(1)
	}

	fmt.Printf("Deleted session: %s\n", sessionID)
}

// handleHook handles Claude CLI hooks
func handleHook(hookName string, args []string) {
	switch hookName {
	case "post-tool-use":
		handlePostToolUseHook()
	default:
		fmt.Fprintf(os.Stderr, "Unknown hook: %s\n", hookName)
		fmt.Fprintln(os.Stderr, "Available hooks: post-tool-use")
		os.Exit(1)
	}
}

// handlePostToolUseHook handles the PostToolUse hook from Claude CLI
// It reads JSON from stdin and stores the tool use in the database
func handlePostToolUseHook() {
	// Read JSON from stdin
	var input struct {
		SessionID  string `json:"session_id"`
		ToolName   string `json:"tool_name"`
		ToolInput  any    `json:"tool_input"`
		ToolResult string `json:"tool_result"`
	}

	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(&input); err != nil {
		// Silent fail - don't interrupt Claude
		log.Printf("Failed to decode hook input: %v", err)
		return
	}

	store := openStore()
	defer store.Close()

	// Find the most recent session for this project
	// Claude doesn't provide session ID, so we use the most recent active session
	cwd, _ := os.Getwd()
	sessions, err := store.ListSessions(cwd, 1)
	if err != nil || len(sessions) == 0 {
		log.Printf("No active session found for hook")
		return
	}

	sessionID := sessions[0].ID

	// Convert tool input to string
	var inputStr string
	switch v := input.ToolInput.(type) {
	case string:
		inputStr = v
	case map[string]any:
		// Convert map to JSON string
		b, _ := json.Marshal(v)
		inputStr = string(b)
	default:
		b, _ := json.Marshal(v)
		inputStr = string(b)
	}

	// Create tool use record
	tu := &llmhistory.ToolUse{
		ID:        generateID(),
		SessionID: sessionID,
		Timestamp: time.Now(),
		ToolName:  input.ToolName,
		Input:     inputStr,
		Output:    input.ToolResult,
	}

	if err := store.CreateToolUse(tu); err != nil {
		log.Printf("Failed to save tool use from hook: %v", err)
		return
	}

	log.Printf("Recorded tool use from hook: %s", input.ToolName)
}

// generateID generates a simple unique ID
func generateID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
}

// showStats shows database statistics
func showStats() {
	store := openStore()
	defer store.Close()

	stats, err := store.GetStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get stats: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("LLM History Database Statistics")
	fmt.Println("================================")
	fmt.Printf("Database size:   %.2f MB (%d bytes)\n", stats.FileSizeMB, stats.FileSizeBytes)
	fmt.Printf("Sessions:        %d\n", stats.SessionCount)
	fmt.Printf("Tool uses:       %d\n", stats.ToolUseCount)
	fmt.Printf("Output records:  %d\n", stats.OutputCount)
	if !stats.OldestSession.IsZero() {
		fmt.Printf("Oldest session:  %s\n", stats.OldestSession.Format("2006-01-02 15:04"))
	}
	if !stats.NewestSession.IsZero() {
		fmt.Printf("Newest session:  %s\n", stats.NewestSession.Format("2006-01-02 15:04"))
	}

	// Show configured limits
	maxMB := llmhistory.GetMaxSizeMB()
	if maxMB > 0 {
		pct := (stats.FileSizeMB / float64(maxMB)) * 100
		fmt.Printf("\nSize limit:      %d MB (%.1f%% used)\n", maxMB, pct)
	} else {
		fmt.Println("\nSize limit:      unlimited")
	}
}

// runCompact vacuums the database to reclaim space
func runCompact() {
	store := openStore()
	defer store.Close()

	// Get size before
	sizeBefore, _ := store.GetDBSize()

	fmt.Println("Running VACUUM to reclaim space...")
	if err := store.Vacuum(); err != nil {
		fmt.Fprintf(os.Stderr, "Vacuum failed: %v\n", err)
		os.Exit(1)
	}

	// Get size after
	sizeAfter, _ := store.GetDBSize()

	saved := sizeBefore - sizeAfter
	fmt.Printf("Compaction complete.\n")
	fmt.Printf("  Before: %.2f MB\n", float64(sizeBefore)/(1024*1024))
	fmt.Printf("  After:  %.2f MB\n", float64(sizeAfter)/(1024*1024))
	if saved > 0 {
		fmt.Printf("  Saved:  %.2f MB\n", float64(saved)/(1024*1024))
	}
}

// runCleanup deletes sessions older than N days
func runCleanup(days int) {
	store := openStore()
	defer store.Close()

	fmt.Printf("Deleting sessions older than %d days...\n", days)

	deleted, err := store.Cleanup(days)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cleanup failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Deleted %d sessions.\n", deleted)

	if deleted > 0 {
		fmt.Println("Run 'thicc mcp compact' to reclaim disk space.")
	}
}

// runSizeLimit enforces the database size limit
func runSizeLimit(sizeMB int) {
	store := openStore()
	defer store.Close()

	maxBytes := int64(sizeMB) * 1024 * 1024

	// Get current size
	currentSize, _ := store.GetDBSize()
	fmt.Printf("Current size: %.2f MB\n", float64(currentSize)/(1024*1024))
	fmt.Printf("Size limit:   %d MB\n", sizeMB)

	if currentSize <= maxBytes {
		fmt.Println("Database is within size limit. No cleanup needed.")
		return
	}

	fmt.Println("Enforcing size limit...")
	deleted, err := store.EnforceSizeLimit(maxBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Size enforcement failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Deleted %d sessions to meet size limit.\n", deleted)

	// Vacuum after deletion
	fmt.Println("Running VACUUM...")
	if err := store.Vacuum(); err != nil {
		fmt.Fprintf(os.Stderr, "Vacuum failed: %v\n", err)
	}

	newSize, _ := store.GetDBSize()
	fmt.Printf("New size: %.2f MB\n", float64(newSize)/(1024*1024))
}

// runRebuildFTS rebuilds the full-text search index
func runRebuildFTS() {
	store := openStore()
	defer store.Close()

	fmt.Println("Rebuilding FTS index...")
	if err := store.RebuildFTS(); err != nil {
		fmt.Fprintf(os.Stderr, "Rebuild failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("FTS index rebuilt successfully.")
}
