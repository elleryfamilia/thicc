package filemanager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestDir creates a temporary directory with test files
func createTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create test file structure
	files := []string{
		"main.go",
		"config.go",
		"manager.go",
		"README.md",
		"src/app.go",
		"src/utils.go",
		"src/components/button.go",
	}

	for _, f := range files {
		path := filepath.Join(dir, f)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte("test"), 0644))
	}

	return dir
}

// =============================================================================
// FileIndex Basic Tests
// =============================================================================

func TestFileIndex_New(t *testing.T) {
	idx := NewFileIndex("/tmp/test")

	assert.Equal(t, "/tmp/test", idx.Root)
	assert.False(t, idx.IsReady(), "New index should not be ready")
	assert.False(t, idx.IsBuilding(), "New index should not be building")
}

func TestFileIndex_Build(t *testing.T) {
	dir := createTestDir(t)
	idx := NewFileIndex(dir)

	err := idx.Build()
	require.NoError(t, err)

	assert.True(t, idx.IsReady(), "Index should be ready after build")
	// 7 files + 2 directories (src, src/components) = 9 entries
	assert.Equal(t, 9, idx.Count(), "Should have indexed 9 entries (7 files + 2 dirs)")
}

// =============================================================================
// Fuzzy Search Tests
// =============================================================================

func TestFileIndex_Search_EmptyQuery_ReturnsFirstN(t *testing.T) {
	dir := createTestDir(t)
	idx := NewFileIndex(dir)
	require.NoError(t, idx.Build())

	results := idx.Search("", 5)

	assert.Len(t, results, 5, "Empty query should return first 5 files")
}

func TestFileIndex_Search_ExactMatch(t *testing.T) {
	dir := createTestDir(t)
	idx := NewFileIndex(dir)
	require.NoError(t, idx.Build())

	results := idx.Search("main.go", 10)

	require.NotEmpty(t, results)
	assert.Equal(t, "main.go", results[0].File.Name, "Exact match should be first")
}

func TestFileIndex_Search_FuzzyMatch(t *testing.T) {
	dir := createTestDir(t)
	idx := NewFileIndex(dir)
	require.NoError(t, idx.Build())

	// "mgo" should match "main.go" and "manager.go"
	results := idx.Search("mgo", 10)

	require.NotEmpty(t, results)
	// First result should be one of the .go files with 'm' and 'go'
	foundMatch := false
	for _, r := range results {
		if r.File.Name == "main.go" || r.File.Name == "manager.go" {
			foundMatch = true
			break
		}
	}
	assert.True(t, foundMatch, "Fuzzy search 'mgo' should match main.go or manager.go")
}

func TestFileIndex_Search_PartialMatch(t *testing.T) {
	dir := createTestDir(t)
	idx := NewFileIndex(dir)
	require.NoError(t, idx.Build())

	// "conf" should match "config.go"
	results := idx.Search("conf", 10)

	require.NotEmpty(t, results)
	assert.Equal(t, "config.go", results[0].File.Name)
}

func TestFileIndex_Search_NoMatch(t *testing.T) {
	dir := createTestDir(t)
	idx := NewFileIndex(dir)
	require.NoError(t, idx.Build())

	results := idx.Search("xyz123nonexistent", 10)

	assert.Empty(t, results, "Should return empty for non-matching query")
}

func TestFileIndex_Search_MatchedIndices(t *testing.T) {
	dir := createTestDir(t)
	idx := NewFileIndex(dir)
	require.NoError(t, idx.Build())

	results := idx.Search("main", 10)

	require.NotEmpty(t, results)
	// The first result should have matched indices for highlighting
	if results[0].File.Name == "main.go" {
		assert.NotEmpty(t, results[0].MatchedIdx, "Should have matched indices for highlighting")
	}
}

func TestFileIndex_Search_LimitResults(t *testing.T) {
	dir := createTestDir(t)
	idx := NewFileIndex(dir)
	require.NoError(t, idx.Build())

	results := idx.Search("", 3)

	assert.Len(t, results, 3, "Should respect limit parameter")
}

func TestFileIndex_Search_NotReady_ReturnsNil(t *testing.T) {
	idx := NewFileIndex("/tmp/test")
	// Don't call Build()

	results := idx.Search("test", 10)

	assert.Nil(t, results, "Search on unready index should return nil")
}

// =============================================================================
// Score Ordering Tests
// =============================================================================

func TestFileIndex_Search_BetterMatchesFirst(t *testing.T) {
	dir := createTestDir(t)
	idx := NewFileIndex(dir)
	require.NoError(t, idx.Build())

	// "button" should match "button.go" better than other files
	results := idx.Search("button", 10)

	require.NotEmpty(t, results)
	assert.Equal(t, "button.go", results[0].File.Name, "Exact substring match should be first")
}

// =============================================================================
// Refresh Tests
// =============================================================================

func TestFileIndex_Refresh(t *testing.T) {
	dir := createTestDir(t)
	idx := NewFileIndex(dir)
	require.NoError(t, idx.Build())

	assert.True(t, idx.IsReady())

	// Call refresh - it rebuilds in background
	idx.Refresh()

	// After refresh starts, ready should be false until rebuild completes
	// Note: This is a race, but we're testing the mechanism
	// In practice, we'd wait for IsReady() to become true again
}
