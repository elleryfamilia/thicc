package aiterminal

import (
	"fmt"
	"strings"
)

// Menu represents a selection menu for AI tools
type Menu struct {
	Tools         []AITool
	SelectedIndex int
	Title         string
}

// NewMenu creates a new AI tool selection menu
func NewMenu() *Menu {
	return &Menu{
		Tools:         GetAvailableToolsOnly(),
		SelectedIndex: 0,
		Title:         "Select AI Tool to Launch",
	}
}

// Render returns the menu as a string for display
func (m *Menu) Render() string {
	var b strings.Builder

	b.WriteString(m.Title)
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", len(m.Title)))
	b.WriteString("\n\n")

	for i, tool := range m.Tools {
		prefix := "  "
		if i == m.SelectedIndex {
			prefix = "▶ "
		}

		b.WriteString(fmt.Sprintf("%s%s\n", prefix, tool.Name))
		if i == m.SelectedIndex {
			b.WriteString(fmt.Sprintf("  %s\n", tool.Description))
		}
	}

	b.WriteString("\n")
	b.WriteString("Use ↑/↓ to select, Enter to launch, q to cancel")

	return b.String()
}

// MoveUp moves selection up
func (m *Menu) MoveUp() {
	if m.SelectedIndex > 0 {
		m.SelectedIndex--
	}
}

// MoveDown moves selection down
func (m *Menu) MoveDown() {
	if m.SelectedIndex < len(m.Tools)-1 {
		m.SelectedIndex++
	}
}

// GetSelected returns the currently selected tool
func (m *Menu) GetSelected() *AITool {
	if m.SelectedIndex >= 0 && m.SelectedIndex < len(m.Tools) {
		return &m.Tools[m.SelectedIndex]
	}
	return nil
}

// HasTools returns whether any tools are available
func (m *Menu) HasTools() bool {
	return len(m.Tools) > 0
}

// GetToolNames returns a list of tool names for display
func (m *Menu) GetToolNames() []string {
	names := make([]string, len(m.Tools))
	for i, tool := range m.Tools {
		names[i] = tool.Name
	}
	return names
}
