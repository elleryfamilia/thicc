package nuggets

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testStore creates a temporary store for testing
func testStore(t *testing.T) (*Store, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "nuggets-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	store := NewStore(tmpDir)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func TestNewStore(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	if store.projectRoot == "" {
		t.Error("projectRoot is empty")
	}
}

func TestLoadNuggetsEmpty(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Should return empty file when none exists
	nf, err := store.LoadNuggets()
	if err != nil {
		t.Fatalf("LoadNuggets() error: %v", err)
	}

	if nf.Version != 1 {
		t.Errorf("expected Version 1, got %d", nf.Version)
	}

	if len(nf.Nuggets) != 0 {
		t.Errorf("expected empty nuggets, got %d", len(nf.Nuggets))
	}
}

func TestSaveAndLoadNuggets(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Create nugget file with nuggets
	nf := NewNuggetFile()
	nf.Nuggets = append(nf.Nuggets, Nugget{
		ID:      "test-nugget-1",
		Type:    NuggetDecision,
		Title:   "Test Decision",
		Summary: "This is a test decision",
		Created: time.Now(),
		Tags:    []string{"test", "decision"},
		Files:   []string{"main.go"},
		Content: map[string]any{
			"rationale": []string{"reason 1", "reason 2"},
		},
	})

	// Save
	if err := store.SaveNuggets(nf); err != nil {
		t.Fatalf("SaveNuggets() error: %v", err)
	}

	// Verify file exists
	path := filepath.Join(store.projectRoot, NuggetsFileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("nuggets file was not created")
	}

	// Load
	loaded, err := store.LoadNuggets()
	if err != nil {
		t.Fatalf("LoadNuggets() error: %v", err)
	}

	if len(loaded.Nuggets) != 1 {
		t.Errorf("expected 1 nugget, got %d", len(loaded.Nuggets))
	}

	nugget := loaded.Nuggets[0]
	if nugget.ID != "test-nugget-1" {
		t.Errorf("ID mismatch: got %s", nugget.ID)
	}
	if nugget.Type != NuggetDecision {
		t.Errorf("Type mismatch: got %s", nugget.Type)
	}
	if nugget.Title != "Test Decision" {
		t.Errorf("Title mismatch: got %s", nugget.Title)
	}
}

func TestLoadPendingNuggetsEmpty(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	pf, err := store.LoadPendingNuggets()
	if err != nil {
		t.Fatalf("LoadPendingNuggets() error: %v", err)
	}

	if pf.Version != 1 {
		t.Errorf("expected Version 1, got %d", pf.Version)
	}

	if len(pf.Nuggets) != 0 {
		t.Errorf("expected empty pending nuggets, got %d", len(pf.Nuggets))
	}
}

func TestSaveAndLoadPendingNuggets(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	pf := NewPendingNuggetFile()
	pf.Nuggets = append(pf.Nuggets, Nugget{
		ID:      "pending-nugget-1",
		Type:    NuggetGotcha,
		Title:   "A Gotcha",
		Summary: "Watch out for this",
		Created: time.Now(),
	})

	if err := store.SavePendingNuggets(pf); err != nil {
		t.Fatalf("SavePendingNuggets() error: %v", err)
	}

	// Verify history dir was created
	historyPath := filepath.Join(store.projectRoot, HistoryDir)
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		t.Error("history directory was not created")
	}

	loaded, err := store.LoadPendingNuggets()
	if err != nil {
		t.Fatalf("LoadPendingNuggets() error: %v", err)
	}

	if len(loaded.Nuggets) != 1 {
		t.Errorf("expected 1 pending nugget, got %d", len(loaded.Nuggets))
	}
}

func TestAddPendingNugget(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	nugget := &Nugget{
		Type:    NuggetPattern,
		Title:   "A Pattern",
		Summary: "Reusable pattern",
	}

	if err := store.AddPendingNugget(nugget); err != nil {
		t.Fatalf("AddPendingNugget() error: %v", err)
	}

	// ID should be generated
	if nugget.ID == "" {
		t.Error("ID was not generated")
	}

	// Created should be set
	if nugget.Created.IsZero() {
		t.Error("Created time was not set")
	}

	// Verify it's in pending
	pf, _ := store.LoadPendingNuggets()
	if len(pf.Nuggets) != 1 {
		t.Errorf("expected 1 pending nugget, got %d", len(pf.Nuggets))
	}
}

func TestAcceptNugget(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Add a pending nugget
	nugget := &Nugget{
		ID:      "accept-test-nugget",
		Type:    NuggetDiscovery,
		Title:   "Discovery",
		Summary: "Found something",
		Created: time.Now(),
	}

	if err := store.AddPendingNugget(nugget); err != nil {
		t.Fatalf("AddPendingNugget() error: %v", err)
	}

	// Accept it
	if err := store.AcceptNugget(nugget.ID); err != nil {
		t.Fatalf("AcceptNugget() error: %v", err)
	}

	// Verify it's no longer pending
	pf, _ := store.LoadPendingNuggets()
	if len(pf.Nuggets) != 0 {
		t.Errorf("expected 0 pending nuggets, got %d", len(pf.Nuggets))
	}

	// Verify it's in committed
	nf, _ := store.LoadNuggets()
	if len(nf.Nuggets) != 1 {
		t.Errorf("expected 1 committed nugget, got %d", len(nf.Nuggets))
	}

	if nf.Nuggets[0].ID != nugget.ID {
		t.Errorf("ID mismatch: got %s", nf.Nuggets[0].ID)
	}
}

func TestAcceptNuggetByPrefix(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	nugget := &Nugget{
		ID:      "prefix-match-test-nugget",
		Type:    NuggetIssue,
		Title:   "Issue",
		Summary: "Bug found",
		Created: time.Now(),
	}

	if err := store.AddPendingNugget(nugget); err != nil {
		t.Fatalf("AddPendingNugget() error: %v", err)
	}

	// Accept by 8-char prefix
	if err := store.AcceptNugget("prefix-m"); err != nil {
		t.Fatalf("AcceptNugget() with prefix error: %v", err)
	}

	nf, _ := store.LoadNuggets()
	if len(nf.Nuggets) != 1 {
		t.Errorf("expected 1 committed nugget, got %d", len(nf.Nuggets))
	}
}

func TestAcceptNuggetNotFound(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	err := store.AcceptNugget("non-existent")
	if err == nil {
		t.Error("expected error for non-existent nugget")
	}
}

func TestRejectNugget(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	nugget := &Nugget{
		ID:      "reject-test-nugget",
		Type:    NuggetContext,
		Title:   "Context",
		Summary: "Background info",
		Created: time.Now(),
	}

	if err := store.AddPendingNugget(nugget); err != nil {
		t.Fatalf("AddPendingNugget() error: %v", err)
	}

	if err := store.RejectNugget(nugget.ID); err != nil {
		t.Fatalf("RejectNugget() error: %v", err)
	}

	// Verify it's gone from pending
	pf, _ := store.LoadPendingNuggets()
	if len(pf.Nuggets) != 0 {
		t.Errorf("expected 0 pending nuggets, got %d", len(pf.Nuggets))
	}

	// Verify it's NOT in committed
	nf, _ := store.LoadNuggets()
	if len(nf.Nuggets) != 0 {
		t.Errorf("expected 0 committed nuggets, got %d", len(nf.Nuggets))
	}
}

func TestRejectNuggetNotFound(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	err := store.RejectNugget("non-existent")
	if err == nil {
		t.Error("expected error for non-existent nugget")
	}
}

func TestGetNugget(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Add committed nugget
	committedNugget := &Nugget{
		ID:      "committed-nugget-123",
		Type:    NuggetDecision,
		Title:   "Committed",
		Summary: "A committed nugget",
		Created: time.Now(),
	}
	if err := store.AddNugget(committedNugget); err != nil {
		t.Fatalf("AddNugget() error: %v", err)
	}

	// Add pending nugget
	pendingNugget := &Nugget{
		ID:      "pending-nugget-456",
		Type:    NuggetGotcha,
		Title:   "Pending",
		Summary: "A pending nugget",
		Created: time.Now(),
	}
	if err := store.AddPendingNugget(pendingNugget); err != nil {
		t.Fatalf("AddPendingNugget() error: %v", err)
	}

	// Get committed nugget
	got, isPending, err := store.GetNugget("committed-nugget-123")
	if err != nil {
		t.Fatalf("GetNugget() error: %v", err)
	}
	if isPending {
		t.Error("expected committed nugget, got pending")
	}
	if got.ID != committedNugget.ID {
		t.Errorf("ID mismatch: got %s", got.ID)
	}

	// Get pending nugget
	got, isPending, err = store.GetNugget("pending-nugget-456")
	if err != nil {
		t.Fatalf("GetNugget() error: %v", err)
	}
	if !isPending {
		t.Error("expected pending nugget, got committed")
	}
	if got.ID != pendingNugget.ID {
		t.Errorf("ID mismatch: got %s", got.ID)
	}

	// Get by prefix
	got, _, err = store.GetNugget("committe")
	if err != nil {
		t.Fatalf("GetNugget() by prefix error: %v", err)
	}
	if got.ID != committedNugget.ID {
		t.Errorf("ID mismatch: got %s", got.ID)
	}

	// Get non-existent
	_, _, err = store.GetNugget("non-existent")
	if err == nil {
		t.Error("expected error for non-existent nugget")
	}
}

func TestSearchNuggets(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Add some nuggets
	nuggets := []*Nugget{
		{Type: NuggetDecision, Title: "Database Choice", Summary: "PostgreSQL for ACID compliance", Tags: []string{"database", "architecture"}},
		{Type: NuggetGotcha, Title: "React Hook", Summary: "useEffect cleanup", Tags: []string{"react", "frontend"}},
		{Type: NuggetPattern, Title: "Repository Pattern", Summary: "Data access pattern", Tags: []string{"architecture", "go"}, Files: []string{"repository.go"}},
	}

	for _, n := range nuggets {
		if err := store.AddNugget(n); err != nil {
			t.Fatalf("AddNugget() error: %v", err)
		}
	}

	// Search by title
	results, err := store.SearchNuggets("database")
	if err != nil {
		t.Fatalf("SearchNuggets() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'database', got %d", len(results))
	}

	// Search by summary
	results, err = store.SearchNuggets("cleanup")
	if err != nil {
		t.Fatalf("SearchNuggets() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'cleanup', got %d", len(results))
	}

	// Search by tag
	results, err = store.SearchNuggets("architecture")
	if err != nil {
		t.Fatalf("SearchNuggets() error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'architecture', got %d", len(results))
	}

	// Search by file
	results, err = store.SearchNuggets("repository.go")
	if err != nil {
		t.Fatalf("SearchNuggets() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'repository.go', got %d", len(results))
	}

	// Search no results
	results, err = store.SearchNuggets("nonexistent")
	if err != nil {
		t.Fatalf("SearchNuggets() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for 'nonexistent', got %d", len(results))
	}

	// Case insensitive search
	results, err = store.SearchNuggets("POSTGRESQL")
	if err != nil {
		t.Fatalf("SearchNuggets() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'POSTGRESQL', got %d", len(results))
	}
}

func TestAddNugget(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	nugget := &Nugget{
		Type:    NuggetDecision,
		Title:   "Direct Add",
		Summary: "Added directly to committed",
	}

	if err := store.AddNugget(nugget); err != nil {
		t.Fatalf("AddNugget() error: %v", err)
	}

	// ID should be generated
	if nugget.ID == "" {
		t.Error("ID was not generated")
	}

	// Created should be set
	if nugget.Created.IsZero() {
		t.Error("Created time was not set")
	}

	// Verify it's in committed (not pending)
	nf, _ := store.LoadNuggets()
	if len(nf.Nuggets) != 1 {
		t.Errorf("expected 1 committed nugget, got %d", len(nf.Nuggets))
	}

	pf, _ := store.LoadPendingNuggets()
	if len(pf.Nuggets) != 0 {
		t.Errorf("expected 0 pending nuggets, got %d", len(pf.Nuggets))
	}
}

func TestGetProjectRoot(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	root := store.GetProjectRoot()
	if root == "" {
		t.Error("GetProjectRoot() returned empty string")
	}
}

func TestMultipleNuggetWorkflow(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Simulate a realistic workflow:
	// 1. Extract some nuggets (add to pending)
	// 2. Accept some, reject others
	// 3. Manually add one
	// 4. Search

	// Add pending nuggets
	pending1 := &Nugget{Type: NuggetDecision, Title: "Use JWT", Summary: "Token-based auth"}
	pending2 := &Nugget{Type: NuggetGotcha, Title: "Race Condition", Summary: "Lock the mutex"}
	pending3 := &Nugget{Type: NuggetPattern, Title: "Singleton", Summary: "Use sync.Once"}

	for _, n := range []*Nugget{pending1, pending2, pending3} {
		if err := store.AddPendingNugget(n); err != nil {
			t.Fatalf("AddPendingNugget() error: %v", err)
		}
	}

	// Verify 3 pending
	pf, _ := store.LoadPendingNuggets()
	if len(pf.Nuggets) != 3 {
		t.Errorf("expected 3 pending, got %d", len(pf.Nuggets))
	}

	// Accept first two
	store.AcceptNugget(pending1.ID)
	store.AcceptNugget(pending2.ID)

	// Reject third
	store.RejectNugget(pending3.ID)

	// Manually add one
	manual := &Nugget{Type: NuggetContext, Title: "Legacy System", Summary: "Quirks of old API"}
	store.AddNugget(manual)

	// Verify final state
	nf, _ := store.LoadNuggets()
	if len(nf.Nuggets) != 3 { // 2 accepted + 1 manual
		t.Errorf("expected 3 committed nuggets, got %d", len(nf.Nuggets))
	}

	pf, _ = store.LoadPendingNuggets()
	if len(pf.Nuggets) != 0 {
		t.Errorf("expected 0 pending nuggets, got %d", len(pf.Nuggets))
	}

	// Search should find all committed
	results, _ := store.SearchNuggets("JWT")
	if len(results) != 1 {
		t.Errorf("expected 1 search result, got %d", len(results))
	}
}

func TestSearchNuggetsByUserNotes(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	nugget := &Nugget{
		Type:      NuggetDecision,
		Title:     "Some Decision",
		Summary:   "A summary",
		UserNotes: "Remember to check the edge case",
	}

	if err := store.AddNugget(nugget); err != nil {
		t.Fatalf("AddNugget() error: %v", err)
	}

	results, err := store.SearchNuggets("edge case")
	if err != nil {
		t.Fatalf("SearchNuggets() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'edge case', got %d", len(results))
	}
}
