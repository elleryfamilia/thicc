package terminal

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/ellery/thicc/internal/dashboard"
	"github.com/ellery/thicc/internal/thicc"
)

const (
	// SettingsFileName is the name of the legacy terminal settings file
	SettingsFileName = "settings.json"

	// DefaultScrollbackLines is the default number of scrollback lines
	DefaultScrollbackLines = 10000
)

// TerminalSettings stores terminal-specific configuration
type TerminalSettings struct {
	ScrollbackLines int `json:"scrollback_lines"`
}

// DefaultSettings returns default terminal settings
func DefaultSettings() TerminalSettings {
	return TerminalSettings{
		ScrollbackLines: DefaultScrollbackLines,
	}
}

// getLegacySettingsFilePath returns the path to the legacy settings.json file
func getLegacySettingsFilePath() string {
	return filepath.Join(dashboard.GetConfigDir(), SettingsFileName)
}

// LoadSettings loads terminal settings, preferring consolidated settings
func LoadSettings() TerminalSettings {
	// First, try to get from consolidated THICC settings
	if thicc.GlobalThiccSettings != nil {
		scrollback := thicc.GetScrollbackLines()
		if scrollback > 0 {
			return TerminalSettings{
				ScrollbackLines: scrollback,
			}
		}
	}

	// Fall back to legacy settings file for backward compatibility
	settings := DefaultSettings()

	filePath := getLegacySettingsFilePath()
	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("THICC Terminal: Failed to read legacy settings.json: %v", err)
		}
		return settings // Use defaults
	}

	if err := json.Unmarshal(data, &settings); err != nil {
		log.Printf("THICC Terminal: Failed to parse legacy settings.json: %v", err)
		return DefaultSettings()
	}

	// Validate scrollback lines
	if settings.ScrollbackLines <= 0 {
		settings.ScrollbackLines = DefaultScrollbackLines
	}

	return settings
}

// SaveSettings persists terminal settings to disk (legacy method)
func SaveSettings(settings TerminalSettings) error {
	if err := dashboard.EnsureConfigDir(); err != nil {
		log.Printf("THICC Terminal: Failed to create config dir: %v", err)
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		log.Printf("THICC Terminal: Failed to marshal settings.json: %v", err)
		return err
	}

	filePath := getLegacySettingsFilePath()
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Printf("THICC Terminal: Failed to write settings.json: %v", err)
		return err
	}

	return nil
}
