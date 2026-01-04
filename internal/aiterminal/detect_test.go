package aiterminal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Shell Detection Tests
// =============================================================================

func TestGetShell_ReturnsShell(t *testing.T) {
	shell := getShell()
	assert.NotEmpty(t, shell, "getShell should return a shell")
}

// =============================================================================
// GetCommandLine Tests
// =============================================================================

func TestGetCommandLine_ReturnsCommand(t *testing.T) {
	tool := AITool{
		Command: "test-cmd",
		Args:    []string{"--arg1", "--arg2"},
	}

	cmdLine := tool.GetCommandLine()
	assert.Equal(t, []string{"test-cmd", "--arg1", "--arg2"}, cmdLine)
}

func TestGetCommandLine_NoArgs(t *testing.T) {
	tool := AITool{
		Command: "test-cmd",
		Args:    []string{},
	}

	cmdLine := tool.GetCommandLine()
	assert.Equal(t, []string{"test-cmd"}, cmdLine)
}

// =============================================================================
// Shell Default Tool Tests (Related to prompt injection bug)
// =============================================================================

func TestGetAvailableToolsOnly_IncludesShellDefault(t *testing.T) {
	// Regression test: Shell (default) should always be in available tools
	tools := GetAvailableToolsOnly()

	var shellTool *AITool
	for i := range tools {
		if tools[i].Name == "Shell (default)" {
			shellTool = &tools[i]
			break
		}
	}

	assert.NotNil(t, shellTool, "Shell (default) should be in available tools")
	assert.True(t, shellTool.Available, "Shell (default) should be marked as available")
}

func TestShellDefault_CommandIsShell(t *testing.T) {
	// Verify Shell (default) has a valid shell command
	tools := GetAvailableToolsOnly()

	for _, tool := range tools {
		if tool.Name == "Shell (default)" {
			// Command should be a shell (zsh, bash, fish, sh, etc.)
			// May be full path like /bin/zsh or just zsh
			assert.NotEmpty(t, tool.Command, "Shell command should not be empty")

			// Check if command ends with a known shell name
			knownShells := []string{"zsh", "bash", "fish", "sh", "ksh", "csh", "dash"}
			found := false
			for _, shell := range knownShells {
				if tool.Command == shell || strings.HasSuffix(tool.Command, "/"+shell) {
					found = true
					break
				}
			}
			assert.True(t, found, "Shell command should be a known shell: %s", tool.Command)
			return
		}
	}

	t.Fatal("Shell (default) not found in available tools")
}

// =============================================================================
// ParseSelection Tests
// =============================================================================

func TestParseSelection_ValidIndex(t *testing.T) {
	tools := GetAvailableToolsOnly()
	if len(tools) == 0 {
		t.Skip("No available tools")
	}

	tool := ParseSelection(0)
	assert.NotNil(t, tool, "ParseSelection(0) should return first tool")
	assert.Equal(t, tools[0].Name, tool.Name)
}

func TestParseSelection_NegativeIndex(t *testing.T) {
	tool := ParseSelection(-1)
	assert.Nil(t, tool, "ParseSelection(-1) should return nil")
}

func TestParseSelection_OutOfBounds(t *testing.T) {
	tool := ParseSelection(999)
	assert.Nil(t, tool, "ParseSelection(999) should return nil")
}

// =============================================================================
// GetInstallableTools Tests
// =============================================================================

func TestGetInstallableTools_NoShellDefault(t *testing.T) {
	// Shell (default) should never be in installable tools
	tools := GetInstallableTools()

	for _, tool := range tools {
		assert.NotEqual(t, "Shell (default)", tool.Name,
			"Shell (default) should not be in installable tools")
	}
}

func TestGetInstallableTools_AllHaveInstallCommand(t *testing.T) {
	tools := GetInstallableTools()

	for _, tool := range tools {
		assert.NotEmpty(t, tool.InstallCommand,
			"Installable tool %s should have an install command", tool.Name)
		assert.False(t, tool.Available,
			"Installable tool %s should not be marked as available", tool.Name)
	}
}
