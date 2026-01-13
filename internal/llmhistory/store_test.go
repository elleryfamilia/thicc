package llmhistory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testStore creates a temporary store for testing
func testStore(t *testing.T) (*Store, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "llmhistory-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	store, err := NewStore(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func TestNewStore(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Verify database file was created
	dbPath := filepath.Join(store.dbPath)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("database file not created at %s", dbPath)
	}
}

func TestCreateAndGetSession(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	session := &Session{
		ID:          "test-session-123",
		ToolName:    "claude",
		ToolCommand: "claude --dangerously-skip-permissions",
		ProjectDir:  "/home/user/project",
		StartTime:   time.Now().Truncate(time.Second),
		OutputBytes: 0,
	}

	// Create session
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Get session by exact ID
	got, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if got.ID != session.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, session.ID)
	}
	if got.ToolName != session.ToolName {
		t.Errorf("ToolName mismatch: got %s, want %s", got.ToolName, session.ToolName)
	}
	if got.ProjectDir != session.ProjectDir {
		t.Errorf("ProjectDir mismatch: got %s, want %s", got.ProjectDir, session.ProjectDir)
	}
}

func TestGetSessionByPrefix(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	session := &Session{
		ID:        "abcd1234-full-uuid-here",
		ToolName:  "claude",
		StartTime: time.Now(),
	}

	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Get by 8-char prefix
	got, err := store.GetSession("abcd1234")
	if err != nil {
		t.Fatalf("failed to get session by prefix: %v", err)
	}

	if got.ID != session.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, session.ID)
	}
}

func TestUpdateSession(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	session := &Session{
		ID:          "update-test-123",
		ToolName:    "aider",
		StartTime:   time.Now().Truncate(time.Second),
		OutputBytes: 0,
	}

	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Update session
	session.EndTime = time.Now().Add(time.Hour).Truncate(time.Second)
	session.OutputBytes = 1024

	if err := store.UpdateSession(session); err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	// Verify update
	got, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if got.EndTime.Unix() != session.EndTime.Unix() {
		t.Errorf("EndTime mismatch: got %v, want %v", got.EndTime, session.EndTime)
	}
	if got.OutputBytes != session.OutputBytes {
		t.Errorf("OutputBytes mismatch: got %d, want %d", got.OutputBytes, session.OutputBytes)
	}
}

func TestListSessions(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Create sessions in different projects
	sessions := []*Session{
		{ID: "s1", ToolName: "claude", ProjectDir: "/project/a", StartTime: time.Now().Add(-3 * time.Hour)},
		{ID: "s2", ToolName: "aider", ProjectDir: "/project/b", StartTime: time.Now().Add(-2 * time.Hour)},
		{ID: "s3", ToolName: "claude", ProjectDir: "/project/a", StartTime: time.Now().Add(-1 * time.Hour)},
	}

	for _, s := range sessions {
		if err := store.CreateSession(s); err != nil {
			t.Fatalf("failed to create session: %v", err)
		}
	}

	// List all sessions
	all, err := store.ListSessions("", 10)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(all))
	}

	// Verify order (most recent first)
	if all[0].ID != "s3" {
		t.Errorf("expected most recent session first, got %s", all[0].ID)
	}

	// List by project
	projectA, err := store.ListSessions("/project/a", 10)
	if err != nil {
		t.Fatalf("failed to list sessions by project: %v", err)
	}
	if len(projectA) != 2 {
		t.Errorf("expected 2 sessions for project A, got %d", len(projectA))
	}

	// Test limit
	limited, err := store.ListSessions("", 2)
	if err != nil {
		t.Fatalf("failed to list sessions with limit: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("expected 2 sessions with limit, got %d", len(limited))
	}
}

func TestToolUses(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Create session first
	session := &Session{
		ID:        "tool-use-session",
		ToolName:  "claude",
		StartTime: time.Now(),
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Create tool uses
	toolUses := []*ToolUse{
		{ID: "tu1", SessionID: session.ID, Timestamp: time.Now().Add(-2 * time.Minute), ToolName: "Read", Input: "/path/to/file.go", Output: "file contents"},
		{ID: "tu2", SessionID: session.ID, Timestamp: time.Now().Add(-1 * time.Minute), ToolName: "Edit", Input: "/path/to/file.go", Output: "edited"},
		{ID: "tu3", SessionID: session.ID, Timestamp: time.Now(), ToolName: "Bash", Input: "go build", Output: "success"},
	}

	for _, tu := range toolUses {
		if err := store.CreateToolUse(tu); err != nil {
			t.Fatalf("failed to create tool use: %v", err)
		}
	}

	// Get tool uses for session
	got, err := store.GetToolUsesForSession(session.ID)
	if err != nil {
		t.Fatalf("failed to get tool uses: %v", err)
	}

	if len(got) != 3 {
		t.Errorf("expected 3 tool uses, got %d", len(got))
	}

	// Verify order (oldest first)
	if got[0].ID != "tu1" {
		t.Errorf("expected oldest tool use first, got %s", got[0].ID)
	}
}

func TestToolUseOutputTruncation(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	session := &Session{
		ID:        "truncation-test",
		ToolName:  "claude",
		StartTime: time.Now(),
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Create tool use with very long output
	longOutput := make([]byte, MaxOutputLength+1000)
	for i := range longOutput {
		longOutput[i] = 'x'
	}

	tu := &ToolUse{
		ID:        "long-output",
		SessionID: session.ID,
		Timestamp: time.Now(),
		ToolName:  "Bash",
		Input:     "cat huge-file",
		Output:    string(longOutput),
	}

	if err := store.CreateToolUse(tu); err != nil {
		t.Fatalf("failed to create tool use: %v", err)
	}

	// Verify truncation
	got, err := store.GetToolUsesForSession(session.ID)
	if err != nil {
		t.Fatalf("failed to get tool uses: %v", err)
	}

	if len(got[0].Output) > MaxOutputLength+100 { // Allow for truncation message
		t.Errorf("output not truncated: got %d chars", len(got[0].Output))
	}
}

func TestFileHistory(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	session := &Session{
		ID:        "file-history-session",
		ToolName:  "claude",
		StartTime: time.Now(),
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Create tool uses with file touches
	tu1 := &ToolUse{ID: "fh-tu1", SessionID: session.ID, Timestamp: time.Now(), ToolName: "Read", Input: "/app/main.go"}
	tu2 := &ToolUse{ID: "fh-tu2", SessionID: session.ID, Timestamp: time.Now(), ToolName: "Edit", Input: "/app/main.go"}
	tu3 := &ToolUse{ID: "fh-tu3", SessionID: session.ID, Timestamp: time.Now(), ToolName: "Read", Input: "/app/utils.go"}

	for _, tu := range []*ToolUse{tu1, tu2, tu3} {
		if err := store.CreateToolUse(tu); err != nil {
			t.Fatalf("failed to create tool use: %v", err)
		}
	}

	// Create file touches
	if err := store.CreateFileTouch("fh-tu1", "/app/main.go"); err != nil {
		t.Fatalf("failed to create file touch: %v", err)
	}
	if err := store.CreateFileTouch("fh-tu2", "/app/main.go"); err != nil {
		t.Fatalf("failed to create file touch: %v", err)
	}
	if err := store.CreateFileTouch("fh-tu3", "/app/utils.go"); err != nil {
		t.Fatalf("failed to create file touch: %v", err)
	}

	// Get history for main.go
	history, err := store.GetFileHistory([]string{"/app/main.go"}, 10)
	if err != nil {
		t.Fatalf("failed to get file history: %v", err)
	}

	if len(history) != 2 {
		t.Errorf("expected 2 tool uses for main.go, got %d", len(history))
	}

	// Get history for multiple files
	multiHistory, err := store.GetFileHistory([]string{"/app/main.go", "/app/utils.go"}, 10)
	if err != nil {
		t.Fatalf("failed to get multi-file history: %v", err)
	}

	if len(multiHistory) != 3 {
		t.Errorf("expected 3 tool uses for multiple files, got %d", len(multiHistory))
	}
}

func TestSearch(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	session := &Session{
		ID:         "search-session",
		ToolName:   "claude",
		ProjectDir: "/search/project",
		StartTime:  time.Now(),
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Create tool uses with searchable content
	toolUses := []*ToolUse{
		{ID: "search1", SessionID: session.ID, Timestamp: time.Now(), ToolName: "Read", Input: "config.yaml", Output: "database connection settings"},
		{ID: "search2", SessionID: session.ID, Timestamp: time.Now(), ToolName: "Grep", Input: "findError", Output: "error handling in main.go"},
		{ID: "search3", SessionID: session.ID, Timestamp: time.Now(), ToolName: "Bash", Input: "go test", Output: "all tests passed"},
	}

	for _, tu := range toolUses {
		if err := store.CreateToolUse(tu); err != nil {
			t.Fatalf("failed to create tool use: %v", err)
		}
	}

	// Search for "database"
	results, err := store.Search("database", "", 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result for 'database', got %d", len(results))
	}

	// Search for "error"
	errorResults, err := store.Search("error", "", 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(errorResults) != 1 {
		t.Errorf("expected 1 result for 'error', got %d", len(errorResults))
	}

	// Search with project filter
	projectResults, err := store.Search("test", "/search/project", 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(projectResults) != 1 {
		t.Errorf("expected 1 result for 'test' in project, got %d", len(projectResults))
	}

	// Search in non-existent project
	noResults, err := store.Search("test", "/other/project", 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(noResults) != 0 {
		t.Errorf("expected 0 results for other project, got %d", len(noResults))
	}
}

func TestSessionOutput(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	session := &Session{
		ID:        "output-session",
		ToolName:  "claude",
		StartTime: time.Now(),
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Save output
	output := "This is the session output\nWith multiple lines\n"
	if err := store.SaveSessionOutput(session.ID, output); err != nil {
		t.Fatalf("failed to save session output: %v", err)
	}

	// Get output
	got, err := store.GetSessionOutput(session.ID)
	if err != nil {
		t.Fatalf("failed to get session output: %v", err)
	}

	if got != output {
		t.Errorf("output mismatch: got %q, want %q", got, output)
	}

	// Test non-existent session
	empty, err := store.GetSessionOutput("non-existent")
	if err != nil {
		t.Fatalf("unexpected error for non-existent session: %v", err)
	}
	if empty != "" {
		t.Errorf("expected empty output for non-existent session, got %q", empty)
	}
}

func TestCleanup(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	now := time.Now()

	// Create old and new sessions
	oldSession := &Session{
		ID:        "old-session",
		ToolName:  "claude",
		StartTime: now.AddDate(0, 0, -100), // 100 days ago
	}
	newSession := &Session{
		ID:        "new-session",
		ToolName:  "claude",
		StartTime: now.AddDate(0, 0, -10), // 10 days ago
	}

	if err := store.CreateSession(oldSession); err != nil {
		t.Fatalf("failed to create old session: %v", err)
	}
	if err := store.CreateSession(newSession); err != nil {
		t.Fatalf("failed to create new session: %v", err)
	}

	// Cleanup with 30-day retention
	deleted, err := store.Cleanup(30)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("expected 1 deleted session, got %d", deleted)
	}

	// Verify old session is gone
	sessions, err := store.ListSessions("", 10)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("expected 1 remaining session, got %d", len(sessions))
	}

	if sessions[0].ID != "new-session" {
		t.Errorf("expected new-session to remain, got %s", sessions[0].ID)
	}
}

func TestGetStats(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Create some data
	session := &Session{
		ID:        "stats-session",
		ToolName:  "claude",
		StartTime: time.Now(),
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	tu := &ToolUse{
		ID:        "stats-tu",
		SessionID: session.ID,
		Timestamp: time.Now(),
		ToolName:  "Read",
		Input:     "test.go",
	}
	if err := store.CreateToolUse(tu); err != nil {
		t.Fatalf("failed to create tool use: %v", err)
	}

	if err := store.SaveSessionOutput(session.ID, "output text"); err != nil {
		t.Fatalf("failed to save output: %v", err)
	}

	// Get stats
	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats.SessionCount != 1 {
		t.Errorf("expected 1 session, got %d", stats.SessionCount)
	}
	if stats.ToolUseCount != 1 {
		t.Errorf("expected 1 tool use, got %d", stats.ToolUseCount)
	}
	if stats.OutputCount != 1 {
		t.Errorf("expected 1 output, got %d", stats.OutputCount)
	}
	if stats.FileSizeBytes <= 0 {
		t.Errorf("expected positive file size, got %d", stats.FileSizeBytes)
	}
}

func TestCascadeDelete(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Create session with tool uses and file touches
	session := &Session{
		ID:        "cascade-session",
		ToolName:  "claude",
		StartTime: time.Now().AddDate(0, 0, -100), // Old session
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	tu := &ToolUse{
		ID:        "cascade-tu",
		SessionID: session.ID,
		Timestamp: time.Now(),
		ToolName:  "Read",
		Input:     "test.go",
	}
	if err := store.CreateToolUse(tu); err != nil {
		t.Fatalf("failed to create tool use: %v", err)
	}

	if err := store.CreateFileTouch("cascade-tu", "/app/test.go"); err != nil {
		t.Fatalf("failed to create file touch: %v", err)
	}

	if err := store.SaveSessionOutput(session.ID, "output"); err != nil {
		t.Fatalf("failed to save output: %v", err)
	}

	// Delete session via cleanup
	deleted, err := store.Cleanup(1)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	// Verify all related data is gone
	toolUses, err := store.GetToolUsesForSession(session.ID)
	if err != nil {
		t.Fatalf("failed to get tool uses: %v", err)
	}
	if len(toolUses) != 0 {
		t.Errorf("expected 0 tool uses after cascade delete, got %d", len(toolUses))
	}

	output, err := store.GetSessionOutput(session.ID)
	if err != nil {
		t.Fatalf("failed to get output: %v", err)
	}
	if output != "" {
		t.Errorf("expected empty output after cascade delete, got %q", output)
	}
}

func TestRebuildFTS(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	session := &Session{
		ID:        "fts-session",
		ToolName:  "claude",
		StartTime: time.Now(),
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	tu := &ToolUse{
		ID:        "fts-tu",
		SessionID: session.ID,
		Timestamp: time.Now(),
		ToolName:  "Read",
		Input:     "searchable content here",
		Output:    "more searchable output",
	}
	if err := store.CreateToolUse(tu); err != nil {
		t.Fatalf("failed to create tool use: %v", err)
	}

	// Rebuild FTS
	if err := store.RebuildFTS(); err != nil {
		t.Fatalf("failed to rebuild FTS: %v", err)
	}

	// Verify search still works
	results, err := store.Search("searchable", "", 10)
	if err != nil {
		t.Fatalf("search failed after FTS rebuild: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result after FTS rebuild, got %d", len(results))
	}
}

func TestVacuum(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Just verify vacuum doesn't error
	if err := store.Vacuum(); err != nil {
		t.Fatalf("vacuum failed: %v", err)
	}
}

func TestEmptyFileHistory(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Empty file list
	history, err := store.GetFileHistory([]string{}, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if history != nil {
		t.Errorf("expected nil for empty file list, got %v", history)
	}

	// Non-existent files
	history, err = store.GetFileHistory([]string{"/non/existent/file.go"}, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected empty history for non-existent file, got %d", len(history))
	}
}
