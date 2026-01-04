package dashboard

import (
	"testing"

	"github.com/micro-editor/tcell/v2"
	"github.com/stretchr/testify/assert"
)

// newTestDashboard creates a dashboard with SimulationScreen for testing
func newTestDashboard(t *testing.T) *Dashboard {
	sim := tcell.NewSimulationScreen("")
	if err := sim.Init(); err != nil {
		t.Fatalf("Failed to init simulation screen: %v", err)
	}
	sim.SetSize(80, 24)

	d := NewDashboard(sim)
	return d
}

// sendKey sends a key event to the dashboard
func sendKey(d *Dashboard, key tcell.Key, r rune, mod tcell.ModMask) bool {
	ev := tcell.NewEventKey(key, r, mod, "")
	return d.HandleEvent(ev)
}

// sendRune sends a rune key event (like 'j', 'k', '1', etc.)
func sendRune(d *Dashboard, r rune) bool {
	return sendKey(d, tcell.KeyRune, r, tcell.ModNone)
}

// =============================================================================
// Event Routing Tests (Bug: Ctrl+Q from modal)
// =============================================================================

func TestCtrlQ_ExitsFromDashboard(t *testing.T) {
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	exitCalled := false
	d.OnExit = func() { exitCalled = true }

	sendKey(d, tcell.KeyCtrlQ, 0, tcell.ModCtrl)

	assert.True(t, exitCalled, "Ctrl+Q should call OnExit from dashboard")
}

func TestCtrlQ_ExitsFromOnboardingGuide(t *testing.T) {
	// Regression test for: Ctrl+Q not working when onboarding guide is open
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	exitCalled := false
	d.OnExit = func() { exitCalled = true }

	// Show onboarding guide
	d.ShowOnboardingGuide()
	assert.True(t, d.IsOnboardingGuideActive(), "Guide should be active")

	// Send Ctrl+Q - should pass through modal and exit
	sendKey(d, tcell.KeyCtrlQ, 0, tcell.ModCtrl)

	assert.True(t, exitCalled, "Ctrl+Q should exit even when onboarding guide is active")
}

func TestCtrlQ_ExitsFromProjectPicker(t *testing.T) {
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	exitCalled := false
	d.OnExit = func() { exitCalled = true }

	// Show project picker
	d.ShowProjectPicker()
	assert.True(t, d.IsProjectPickerActive(), "Project picker should be active")

	// Send Ctrl+Q - should exit
	sendKey(d, tcell.KeyCtrlQ, 0, tcell.ModCtrl)

	assert.True(t, exitCalled, "Ctrl+Q should exit even when project picker is active")
}

func TestEsc_ClosesOnboardingGuide_DoesNotExit(t *testing.T) {
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	exitCalled := false
	d.OnExit = func() { exitCalled = true }

	// Show onboarding guide
	d.ShowOnboardingGuide()
	assert.True(t, d.IsOnboardingGuideActive(), "Guide should be active")

	// Send Escape - should close guide but NOT exit
	sendKey(d, tcell.KeyEscape, 0, tcell.ModNone)

	assert.False(t, d.IsOnboardingGuideActive(), "Guide should be closed after Escape")
	assert.False(t, exitCalled, "Escape from guide should NOT exit the app")
}

func TestEsc_ExitsDashboard_WhenNoModalOpen(t *testing.T) {
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	exitCalled := false
	d.OnExit = func() { exitCalled = true }

	// Make sure no modal is open
	assert.False(t, d.IsOnboardingGuideActive())
	assert.False(t, d.IsProjectPickerActive())

	// Send Escape - should exit dashboard
	sendKey(d, tcell.KeyEscape, 0, tcell.ModNone)

	assert.True(t, exitCalled, "Escape should exit when no modal is open")
}

// =============================================================================
// Project Selection Tests (Bug: Wrong project loading)
// =============================================================================

func TestOpenRecent_ByNumber_CallsCorrectPath(t *testing.T) {
	// Regression test for: Project selection from dashboard loads wrong directory
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	// Set up test projects
	d.RecentStore.Projects = []RecentProject{
		{Path: "/first/project", Name: "project", IsFolder: true},
		{Path: "/second/project", Name: "project2", IsFolder: true},
		{Path: "/third/project", Name: "project3", IsFolder: true},
	}

	var openedPath string
	var callCount int
	d.OnOpenFolder = func(path string) {
		openedPath = path
		callCount++
	}

	// Press '2' to open second project
	sendRune(d, '2')

	assert.Equal(t, 1, callCount, "OnOpenFolder should be called once")
	assert.Equal(t, "/second/project", openedPath, "Should open second project")
}

func TestOpenRecent_ByNumber_FirstProject(t *testing.T) {
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	d.RecentStore.Projects = []RecentProject{
		{Path: "/first/project", Name: "project", IsFolder: true},
		{Path: "/second/project", Name: "project2", IsFolder: true},
	}

	var openedPath string
	d.OnOpenFolder = func(path string) { openedPath = path }

	sendRune(d, '1')

	assert.Equal(t, "/first/project", openedPath, "Should open first project")
}

func TestOpenRecent_ByNumber_OutOfBounds(t *testing.T) {
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	d.RecentStore.Projects = []RecentProject{
		{Path: "/only/project", Name: "project", IsFolder: true},
	}

	callCount := 0
	d.OnOpenFolder = func(path string) { callCount++ }

	// Press '5' when there's only 1 project
	sendRune(d, '5')

	assert.Equal(t, 0, callCount, "Should not call OnOpenFolder for out-of-bounds index")
}

func TestOpenRecent_ByEnter_CallsCorrectPath(t *testing.T) {
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	d.RecentStore.Projects = []RecentProject{
		{Path: "/first/project", Name: "project", IsFolder: true},
		{Path: "/second/project", Name: "project2", IsFolder: true},
	}

	var openedPath string
	d.OnOpenFolder = func(path string) { openedPath = path }

	// Navigate to recent pane and select second item
	d.SwitchToRecentPane()
	d.RecentIdx = 1 // Second item

	sendKey(d, tcell.KeyEnter, 0, tcell.ModNone)

	assert.Equal(t, "/second/project", openedPath, "Enter should open selected project")
}

func TestOpenRecent_File_CallsOnOpenFile(t *testing.T) {
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	d.RecentStore.Projects = []RecentProject{
		{Path: "/path/to/file.txt", Name: "file.txt", IsFolder: false},
	}

	var openedFilePath string
	d.OnOpenFile = func(path string) { openedFilePath = path }

	sendRune(d, '1')

	assert.Equal(t, "/path/to/file.txt", openedFilePath, "Should call OnOpenFile for files")
}

// =============================================================================
// Navigation Tests
// =============================================================================

func TestNavigation_MenuPane_DownKey(t *testing.T) {
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	// Start in menu pane
	assert.False(t, d.InRecentPane)
	assert.False(t, d.InAIToolsPane)
	assert.Equal(t, 0, d.SelectedIdx)

	// Move down
	sendKey(d, tcell.KeyDown, 0, tcell.ModNone)

	assert.Equal(t, 1, d.SelectedIdx, "Down should move to next menu item")
}

func TestNavigation_JK_VimStyle(t *testing.T) {
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	assert.Equal(t, 0, d.SelectedIdx)

	// j moves down
	sendRune(d, 'j')
	assert.Equal(t, 1, d.SelectedIdx, "j should move down")

	// k moves up
	sendRune(d, 'k')
	assert.Equal(t, 0, d.SelectedIdx, "k should move up")
}

func TestNavigation_EmptyRecentSkipped(t *testing.T) {
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	// Clear AI tools and recent projects
	d.AITools = nil
	d.InstallTools = nil
	d.RecentStore.Projects = nil

	// Start at last menu item
	d.SelectedIdx = len(d.MenuItems) - 1

	// Move next should wrap to first menu item (skip empty sections)
	sendKey(d, tcell.KeyDown, 0, tcell.ModNone)

	assert.False(t, d.InRecentPane, "Should not enter empty recent pane")
	assert.False(t, d.InAIToolsPane, "Should not enter empty AI tools pane")
	assert.Equal(t, 0, d.SelectedIdx, "Should wrap to first menu item")
}

// =============================================================================
// AI Tool Selection Tests
// =============================================================================

func TestGetSelectedAIToolCommand_ShellDefault_ReturnsNil(t *testing.T) {
	// Regression test for: Powerline prompt not showing because command wasn't nil
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	// Find and select "Shell (default)" if it exists
	for i, tool := range d.AITools {
		if tool.Name == "Shell (default)" {
			d.AIToolsIdx = i
			d.PrefsStore.SetSelectedAITool(tool.Command)
			break
		}
	}

	// GetSelectedAIToolCommand should return nil for shell
	cmd := d.GetSelectedAIToolCommand()
	assert.Nil(t, cmd, "Shell (default) should return nil command to trigger prompt injection")
}

func TestGetSelectedAIToolCommand_NoSelection_ReturnsNil(t *testing.T) {
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	// Clear any selection
	d.PrefsStore.ClearSelectedAITool()

	cmd := d.GetSelectedAIToolCommand()
	assert.Nil(t, cmd, "No selection should return nil command")
}

// =============================================================================
// State Consistency Tests
// =============================================================================

func TestState_OnlyOnePaneActive(t *testing.T) {
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	// Add some projects so all panes are available
	d.RecentStore.Projects = []RecentProject{
		{Path: "/test/project", Name: "project", IsFolder: true},
	}

	// Switch to menu pane
	d.SwitchToMenuPane()
	assert.False(t, d.InRecentPane)
	assert.False(t, d.InAIToolsPane)

	// Switch to recent pane
	d.SwitchToRecentPane()
	assert.True(t, d.InRecentPane)
	assert.False(t, d.InAIToolsPane)

	// Switch to AI tools pane
	d.SwitchToAIToolsPane()
	assert.False(t, d.InRecentPane)
	assert.True(t, d.InAIToolsPane)
}

func TestState_RecentIdxResetOnPaneSwitch(t *testing.T) {
	d := newTestDashboard(t)
	defer d.Screen.Fini()

	d.RecentStore.Projects = []RecentProject{
		{Path: "/test/project", Name: "project", IsFolder: true},
	}

	// Enter recent pane
	d.SwitchToRecentPane()
	assert.Equal(t, 0, d.RecentIdx)

	// Switch away
	d.SwitchToMenuPane()
	assert.Equal(t, -1, d.RecentIdx, "RecentIdx should be -1 when not in recent pane")
}
