package aiterminal

import (
	"os"
	"os/exec"
	"strings"
)

// AITool represents an AI CLI tool
type AITool struct {
	Name           string   // Display name
	Command        string   // Command to execute
	Args           []string // Default arguments
	Description    string
	Available      bool
	InstallCommand string // Command to install this tool (empty if not installable)
}

// GetAvailableAITools detects which AI CLI tools are installed
func GetAvailableAITools() []AITool {
	tools := []AITool{
		{
			Name:           "Claude Code",
			Command:        "claude",
			Args:           []string{},
			Description:    "Anthropic's Claude Code CLI",
			InstallCommand: "curl -fsSL https://raw.githubusercontent.com/elleryfamilia/thicc/main/scripts/install-tool.sh | sh -s -- @anthropic-ai/claude-code",
		},
		{
			Name:        "Claude Code (YOLO)",
			Command:     "claude",
			Args:        []string{"--dangerously-skip-permissions"},
			Description: "Claude Code with auto-accept permissions",
		},
		{
			Name:           "Gemini CLI",
			Command:        "gemini",
			Args:           []string{},
			Description:    "Google's Gemini CLI",
			InstallCommand: "curl -fsSL https://raw.githubusercontent.com/elleryfamilia/thicc/main/scripts/install-tool.sh | sh -s -- @google/gemini-cli",
		},
		{
			Name:           "Codex CLI",
			Command:        "codex",
			Args:           []string{},
			Description:    "OpenAI Codex CLI",
			InstallCommand: "curl -fsSL https://raw.githubusercontent.com/elleryfamilia/thicc/main/scripts/install-tool.sh | sh -s -- @openai/codex",
		},
		{
			Name:        "OpenCode",
			Command:     "opencode",
			Args:        []string{},
			Description: "OpenCode AI coding assistant",
		},
		{
			Name:        "Aider",
			Command:     "aider",
			Args:        []string{},
			Description: "AI pair programming in your terminal",
		},
		{
			Name:           "GitHub Copilot",
			Command:        "copilot",
			Args:           []string{},
			Description:    "GitHub Copilot CLI",
			InstallCommand: "curl -fsSL https://raw.githubusercontent.com/elleryfamilia/thicc/main/scripts/install-tool.sh | sh -s -- @github/copilot",
		},
		{
			Name:        "Ollama",
			Command:     "ollama",
			Args:        []string{},
			Description: "Run LLMs locally",
		},
		{
			Name:        "Kiro CLI",
			Command:     "kiro-cli",
			Args:        []string{},
			Description: "AI coding assistant",
		},
		{
			Name:        "Shell (default)",
			Command:     getShell(),
			Args:        []string{},
			Description: "Your default shell",
			Available:   true, // Always available
		},
	}

	// Check which tools are available
	for i := range tools {
		if tools[i].Available {
			continue // Already marked as available
		}
		tools[i].Available = isCommandAvailable(tools[i].Command)
	}

	// YOLO variant inherits availability from regular Claude
	for i := range tools {
		if tools[i].Name == "Claude Code (YOLO)" {
			for _, t := range tools {
				if t.Name == "Claude Code" {
					tools[i].Available = t.Available
					break
				}
			}
			break
		}
	}

	return tools
}

// GetAvailableToolsOnly returns only the tools that are installed
func GetAvailableToolsOnly() []AITool {
	all := GetAvailableAITools()
	available := make([]AITool, 0)

	for _, tool := range all {
		if tool.Available {
			available = append(available, tool)
		}
	}

	return available
}

// GetInstallableTools returns tools that are not installed but can be installed
func GetInstallableTools() []AITool {
	all := GetAvailableAITools()
	installable := make([]AITool, 0)

	for _, tool := range all {
		if !tool.Available && tool.InstallCommand != "" {
			installable = append(installable, tool)
		}
	}

	return installable
}

// isCommandAvailable checks if a command is available in PATH
func isCommandAvailable(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// getShell returns the user's default shell
func getShell() string {
	// Check common shell environment variables
	if shell := lookupEnv("SHELL"); shell != "" {
		return shell
	}

	// Fallback to common shells
	shells := []string{"zsh", "bash", "fish", "sh"}
	for _, shell := range shells {
		if isCommandAvailable(shell) {
			return shell
		}
	}

	return "sh" // Ultimate fallback
}

// lookupEnv is a helper to get environment variables
func lookupEnv(key string) string {
	return os.Getenv(key)
}

// FormatToolName returns a formatted name for display in menu
func (t *AITool) FormatToolName() string {
	if t.Available {
		return "✓ " + t.Name
	}
	return "  " + t.Name
}

// GetCommandLine returns the full command line to execute
func (t *AITool) GetCommandLine() []string {
	result := []string{t.Command}
	result = append(result, t.Args...)
	return result
}

// DetectAndFormat returns a formatted list of available tools for menu display
func DetectAndFormat() []string {
	tools := GetAvailableToolsOnly()
	result := make([]string, len(tools))

	for i, tool := range tools {
		result[i] = tool.Name + " - " + tool.Description
	}

	return result
}

// ParseSelection returns the selected tool from a menu choice
func ParseSelection(choice int) *AITool {
	tools := GetAvailableToolsOnly()
	if choice < 0 || choice >= len(tools) {
		return nil
	}
	return &tools[choice]
}

// QuickLaunchFirst launches the first available AI tool
// Useful for default behavior when no selection is made
func QuickLaunchFirst() *AITool {
	tools := GetAvailableToolsOnly()
	if len(tools) == 0 {
		return nil
	}

	// Prefer Claude if available, otherwise first tool
	for _, tool := range tools {
		if strings.Contains(strings.ToLower(tool.Name), "claude") {
			return &tool
		}
	}

	return &tools[0]
}
