package terminal

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/ellery/thicc/internal/dashboard"
)

const (
	// SettingsFileName is the name of the terminal settings file
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

// getSettingsFilePath returns the path to the settings.json file
func getSettingsFilePath() string {
	return filepath.Join(dashboard.GetConfigDir(), SettingsFileName)
}

// LoadSettings loads terminal settings from disk
func LoadSettings() TerminalSettings {
	settings := DefaultSettings()

	filePath := getSettingsFilePath()
	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("THOCK Terminal: Failed to read settings.json: %v", err)
		}
		return settings // Use defaults
	}

	if err := json.Unmarshal(data, &settings); err != nil {
		log.Printf("THOCK Terminal: Failed to parse settings.json: %v", err)
		return DefaultSettings()
	}

	// Validate scrollback lines
	if settings.ScrollbackLines <= 0 {
		settings.ScrollbackLines = DefaultScrollbackLines
	}

	return settings
}

// SaveSettings persists terminal settings to disk
func SaveSettings(settings TerminalSettings) error {
	if err := dashboard.EnsureConfigDir(); err != nil {
		log.Printf("THOCK Terminal: Failed to create config dir: %v", err)
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		log.Printf("THOCK Terminal: Failed to marshal settings.json: %v", err)
		return err
	}

	filePath := getSettingsFilePath()
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Printf("THOCK Terminal: Failed to write settings.json: %v", err)
		return err
	}

	return nil
}
