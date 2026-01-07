package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// newTestLayoutManager creates a layout manager for testing
func newTestLayoutManager(screenW, screenH int) *LayoutManager {
	lm := NewLayoutManager("/tmp/test")
	lm.ScreenW = screenW
	lm.ScreenH = screenH
	return lm
}

// =============================================================================
// Terminal Width Tests (Bug: Terminal width wrong - was using remaining space)
// =============================================================================

func TestLayout_TerminalWidth_Is45Percent(t *testing.T) {
	// Regression test for: Terminal width wrong - was using remaining space instead of 45%
	lm := newTestLayoutManager(100, 50)

	// Default: all panes visible
	assert.True(t, lm.TreeVisible)
	assert.True(t, lm.EditorVisible)
	assert.True(t, lm.TerminalVisible)

	// Terminal should be 45% of screen
	termWidth := lm.getTermWidth()
	assert.Equal(t, 45, termWidth, "Terminal should be 45%% of screen width (100 * 0.45 = 45)")
}

func TestLayout_TerminalWidth_VariousScreenSizes(t *testing.T) {
	tests := []struct {
		screenW          int
		expectedTermW    int
	}{
		{100, 45},  // 100 * 0.45 = 45
		{200, 90},  // 200 * 0.45 = 90
		{80, 36},   // 80 * 0.45 = 36
		{120, 54},  // 120 * 0.45 = 54
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			lm := newTestLayoutManager(tt.screenW, 50)
			termWidth := lm.getTermWidth()
			assert.Equal(t, tt.expectedTermW, termWidth)
		})
	}
}

func TestLayout_TerminalWidth_Hidden_ReturnsZero(t *testing.T) {
	lm := newTestLayoutManager(100, 50)
	lm.TerminalVisible = false

	termWidth := lm.getTermWidth()
	assert.Equal(t, 0, termWidth, "Hidden terminal should have zero width")
}

// =============================================================================
// Multiple Terminal Tests
// =============================================================================

func TestLayout_MultipleTerminals_SplitEqually(t *testing.T) {
	lm := newTestLayoutManager(100, 50)

	// Enable all 3 terminals
	lm.TerminalVisible = true
	lm.Terminal2Visible = true
	lm.Terminal3Visible = true

	// Total terminal space is 45 (45% of 100)
	// Each terminal should get 45/3 = 15
	totalSpace := lm.getTotalTerminalSpace()
	assert.Equal(t, 45, totalSpace)

	singleWidth := lm.getSingleTerminalWidth()
	assert.Equal(t, 15, singleWidth, "3 terminals should split 45 pixels equally (15 each)")

	assert.Equal(t, 15, lm.getTermWidth())
	assert.Equal(t, 15, lm.getTerm2Width())
	assert.Equal(t, 15, lm.getTerm3Width())
}

func TestLayout_TwoTerminals_SplitEqually(t *testing.T) {
	lm := newTestLayoutManager(100, 50)

	lm.TerminalVisible = true
	lm.Terminal2Visible = true
	lm.Terminal3Visible = false

	// Total terminal space is 45
	// Each terminal should get 45/2 = 22
	singleWidth := lm.getSingleTerminalWidth()
	assert.Equal(t, 22, singleWidth, "2 terminals should split 45 pixels (22 each)")
}

// =============================================================================
// Tree Width Tests
// =============================================================================

func TestLayout_TreeWidth_Fixed30(t *testing.T) {
	lm := newTestLayoutManager(100, 50)

	treeWidth := lm.getTreeWidth()
	assert.Equal(t, 30, treeWidth, "Tree width should be fixed at 30")
}

func TestLayout_TreeWidth_HiddenReturnsZero(t *testing.T) {
	lm := newTestLayoutManager(100, 50)
	lm.TreeVisible = false

	treeWidth := lm.getTreeWidth()
	assert.Equal(t, 0, treeWidth, "Hidden tree should have zero width")
}

// =============================================================================
// Editor Width Tests
// =============================================================================

func TestLayout_EditorWidth_TakesRemainingSpace(t *testing.T) {
	lm := newTestLayoutManager(100, 50)

	// Tree = 30, Terminal = 45, Editor should be = 100 - 30 - 45 = 25
	editorWidth := lm.getEditorWidth()
	assert.Equal(t, 25, editorWidth, "Editor should take remaining space after tree and terminal")
}

func TestLayout_EditorWidth_AllScreenWhenTerminalHidden(t *testing.T) {
	lm := newTestLayoutManager(100, 50)
	lm.TerminalVisible = false
	lm.ActivePanel = 1 // Editor focused (not file browser, so tree doesn't expand)

	// Tree = 40 (expanded because editor-only layout), Editor = 100 - 40 = 60
	editorWidth := lm.getEditorWidth()
	assert.Equal(t, 60, editorWidth, "Editor should expand when terminal is hidden (tree is 40 in single-pane)")
}

func TestLayout_EditorWidth_HiddenReturnsZero(t *testing.T) {
	lm := newTestLayoutManager(100, 50)
	lm.EditorVisible = false

	editorWidth := lm.getEditorWidth()
	assert.Equal(t, 0, editorWidth, "Hidden editor should have zero width")
}

// =============================================================================
// Position Tests
// =============================================================================

func TestLayout_TerminalX_Position(t *testing.T) {
	lm := newTestLayoutManager(100, 50)

	// Terminal X = Tree width + Editor width = 30 + 25 = 55
	termX := lm.getTermX()
	assert.Equal(t, 55, termX, "Terminal should start at tree + editor width")
}

func TestLayout_Terminal2X_Position(t *testing.T) {
	lm := newTestLayoutManager(100, 50)
	lm.Terminal2Visible = true

	// With 2 terminals, each gets 22 pixels
	// Terminal2 X = Tree(30) + Editor(25) + Terminal1(22) = 77
	term2X := lm.getTerm2X()
	assert.Equal(t, 77, term2X)
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestLayout_AllPanesHidden_NeedsPlaceholders(t *testing.T) {
	lm := newTestLayoutManager(100, 50)
	lm.EditorVisible = false
	lm.TerminalVisible = false
	lm.Terminal2Visible = false
	lm.Terminal3Visible = false

	assert.True(t, lm.needsPlaceholders(), "Should need placeholders when editor and all terminals hidden")
}

func TestLayout_EditorHidden_TerminalTakesRemainingSpace(t *testing.T) {
	lm := newTestLayoutManager(100, 50)
	lm.EditorVisible = false
	lm.ActivePanel = 2 // Terminal focused

	// When editor is hidden with single terminal, tree expands to 40
	// Terminal space = 100 - 40 = 60
	totalSpace := lm.getTotalTerminalSpace()
	assert.Equal(t, 60, totalSpace, "Terminal should expand to fill space when editor hidden (tree is 40)")
}

func TestLayout_TreeHidden_EditorAndTerminalAdjust(t *testing.T) {
	lm := newTestLayoutManager(100, 50)
	lm.TreeVisible = false

	// Tree = 0, Terminal = 45, Editor = 100 - 0 - 45 = 55
	editorWidth := lm.getEditorWidth()
	assert.Equal(t, 55, editorWidth)

	termWidth := lm.getTermWidth()
	assert.Equal(t, 45, termWidth)
}

// =============================================================================
// Dynamic Tree Width Tests (Expands when focused or single-pane layout)
// =============================================================================

func TestLayout_ShouldExpandTree_WhenFileBrowserFocused(t *testing.T) {
	lm := newTestLayoutManager(100, 50)
	lm.ActivePanel = 0 // File browser focused

	assert.True(t, lm.shouldExpandTree(), "Tree should expand when file browser is focused")
	assert.Equal(t, 40, lm.getTreeWidth(), "Expanded tree width should be 40")
}

func TestLayout_ShouldExpandTree_EditorOnlyLayout(t *testing.T) {
	lm := newTestLayoutManager(100, 50)
	lm.ActivePanel = 1 // Editor focused
	lm.TerminalVisible = false
	lm.Terminal2Visible = false
	lm.Terminal3Visible = false

	assert.True(t, lm.shouldExpandTree(), "Tree should expand with editor-only layout")
	assert.Equal(t, 40, lm.getTreeWidth())
}

func TestLayout_ShouldExpandTree_SingleTerminalOnlyLayout(t *testing.T) {
	lm := newTestLayoutManager(100, 50)
	lm.ActivePanel = 2 // Terminal focused
	lm.EditorVisible = false
	lm.TerminalVisible = true
	lm.Terminal2Visible = false
	lm.Terminal3Visible = false

	assert.True(t, lm.shouldExpandTree(), "Tree should expand with single-terminal-only layout")
	assert.Equal(t, 40, lm.getTreeWidth())
}

func TestLayout_ShouldNotExpandTree_EditorPlusTerminal(t *testing.T) {
	lm := newTestLayoutManager(100, 50)
	lm.ActivePanel = 1 // Editor focused
	lm.EditorVisible = true
	lm.TerminalVisible = true

	assert.False(t, lm.shouldExpandTree(), "Tree should NOT expand with editor + terminal")
	assert.Equal(t, 30, lm.getTreeWidth(), "Normal tree width should be 30")
}

func TestLayout_ShouldNotExpandTree_MultipleTerminals(t *testing.T) {
	lm := newTestLayoutManager(100, 50)
	lm.ActivePanel = 2 // Terminal focused
	lm.EditorVisible = false
	lm.TerminalVisible = true
	lm.Terminal2Visible = true

	assert.False(t, lm.shouldExpandTree(), "Tree should NOT expand with multiple terminals")
	assert.Equal(t, 30, lm.getTreeWidth())
}

func TestLayout_TerminalSpaceReducedWhenTreeExpanded(t *testing.T) {
	lm := newTestLayoutManager(100, 50)
	lm.ActivePanel = 0 // File browser focused (triggers expansion)
	lm.EditorVisible = true
	lm.TerminalVisible = true

	// Normal terminal space would be 45 (45% of 100)
	// With expanded tree, should reduce by 10 (40-30)
	// So terminal space = 45 - 10 = 35
	termSpace := lm.getTotalTerminalSpace()
	assert.Equal(t, 35, termSpace, "Terminal space should reduce when tree is expanded")
}

func TestLayout_EditorWidthUnchangedWhenTreeExpanded(t *testing.T) {
	// When tree expands, the extra space comes from terminal, not editor
	lm := newTestLayoutManager(100, 50)
	lm.EditorVisible = true
	lm.TerminalVisible = true

	// First, get editor width with normal tree
	lm.ActivePanel = 1 // Editor focused, tree not expanded
	normalEditorWidth := lm.getEditorWidth()

	// Now expand tree
	lm.ActivePanel = 0 // File browser focused, tree expands
	expandedEditorWidth := lm.getEditorWidth()

	assert.Equal(t, normalEditorWidth, expandedEditorWidth,
		"Editor width should remain the same when tree expands (terminal absorbs the change)")
}
