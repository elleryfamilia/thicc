package thicc

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// ThiccConfigSubdir is the subdirectory for THICC-specific config
	ThiccConfigSubdir = "thicc"
	// SettingsFileName is the name of the settings file
	SettingsFileName = "settings.json"

	// Default values
	DefaultScrollbackLines        = 10000
	DefaultBackgroundColor        = "#0b0614"
	DefaultDoubleClickThresholdMs = 400
)

// TerminalSettings contains terminal-specific settings
type TerminalSettings struct {
	ScrollbackLines int `json:"scrollback_lines"`
}

// AppearanceSettings contains appearance-related settings
type AppearanceSettings struct {
	BackgroundColor string `json:"background_color"`
}

// EditorSettings contains editor behavior settings
type EditorSettings struct {
	DoubleClickThresholdMs int `json:"double_click_threshold_ms"`
}

// ThiccSettings holds all THICC-specific configuration
type ThiccSettings struct {
	Terminal   TerminalSettings   `json:"terminal"`
	Appearance AppearanceSettings `json:"appearance"`
	Editor     EditorSettings     `json:"editor"`
}

// GlobalThiccSettings is the loaded settings instance
var GlobalThiccSettings *ThiccSettings

// DefaultSettings returns the default THICC settings
func DefaultSettings() *ThiccSettings {
	return &ThiccSettings{
		Terminal: TerminalSettings{
			ScrollbackLines: DefaultScrollbackLines,
		},
		Appearance: AppearanceSettings{
			BackgroundColor: DefaultBackgroundColor,
		},
		Editor: EditorSettings{
			DoubleClickThresholdMs: DefaultDoubleClickThresholdMs,
		},
	}
}

// getBaseConfigDir returns the base config directory for thicc
func getBaseConfigDir() string {
	// Check for THICC_CONFIG_HOME first
	if dir := os.Getenv("THICC_CONFIG_HOME"); dir != "" {
		return dir
	}
	// Check for MICRO_CONFIG_HOME (compatibility)
	if dir := os.Getenv("MICRO_CONFIG_HOME"); dir != "" {
		return dir
	}
	// Use XDG_CONFIG_HOME if set
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "thicc")
	}
	// Default to ~/.config/thicc
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "thicc")
}

// GetConfigDir returns the THICC settings config directory path
func GetConfigDir() string {
	return filepath.Join(getBaseConfigDir(), ThiccConfigSubdir)
}

// GetSettingsFilePath returns the path to the settings file
func GetSettingsFilePath() string {
	return filepath.Join(GetConfigDir(), SettingsFileName)
}

// EnsureConfigDir creates the THICC config directory if it doesn't exist
func EnsureConfigDir() error {
	dir := GetConfigDir()
	return os.MkdirAll(dir, 0755)
}

// EnsureSettingsFile creates the settings file with defaults if it doesn't exist
func EnsureSettingsFile() error {
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	filePath := GetSettingsFilePath()
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return SaveSettings(DefaultSettings())
	}
	return nil
}

// LoadSettings loads THICC settings from disk
func LoadSettings() *ThiccSettings {
	settings := DefaultSettings()

	filePath := GetSettingsFilePath()
	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("THICC Settings: Failed to read settings.json: %v", err)
		}
		GlobalThiccSettings = settings
		return settings
	}

	if err := json.Unmarshal(data, settings); err != nil {
		log.Printf("THICC Settings: Failed to parse settings.json: %v", err)
		GlobalThiccSettings = DefaultSettings()
		return GlobalThiccSettings
	}

	// Validate and apply defaults for missing/invalid values
	if settings.Terminal.ScrollbackLines <= 0 {
		settings.Terminal.ScrollbackLines = DefaultScrollbackLines
	}
	if settings.Appearance.BackgroundColor == "" {
		settings.Appearance.BackgroundColor = DefaultBackgroundColor
	}
	if settings.Editor.DoubleClickThresholdMs <= 0 {
		settings.Editor.DoubleClickThresholdMs = DefaultDoubleClickThresholdMs
	}

	GlobalThiccSettings = settings
	return settings
}

// SaveSettings persists THICC settings to disk
func SaveSettings(settings *ThiccSettings) error {
	if err := EnsureConfigDir(); err != nil {
		log.Printf("THICC Settings: Failed to create config dir: %v", err)
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		log.Printf("THICC Settings: Failed to marshal settings.json: %v", err)
		return err
	}

	filePath := GetSettingsFilePath()
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Printf("THICC Settings: Failed to write settings.json: %v", err)
		return err
	}

	GlobalThiccSettings = settings
	return nil
}

// GetScrollbackLines returns the terminal scrollback lines setting
func GetScrollbackLines() int {
	if GlobalThiccSettings == nil {
		return DefaultScrollbackLines
	}
	return GlobalThiccSettings.Terminal.ScrollbackLines
}

// GetBackgroundColor returns the appearance background color setting
func GetBackgroundColor() string {
	if GlobalThiccSettings == nil {
		return DefaultBackgroundColor
	}
	return GlobalThiccSettings.Appearance.BackgroundColor
}

// GetDoubleClickThreshold returns the double-click threshold in milliseconds
func GetDoubleClickThreshold() int {
	if GlobalThiccSettings == nil {
		return DefaultDoubleClickThresholdMs
	}
	return GlobalThiccSettings.Editor.DoubleClickThresholdMs
}

// ValidationError represents a settings validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateSettingsJSON validates JSON content and returns parsed settings or errors
func ValidateSettingsJSON(data []byte) (*ThiccSettings, []ValidationError) {
	var errors []ValidationError

	// First check if it's valid JSON
	var settings ThiccSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		errors = append(errors, ValidationError{
			Field:   "json",
			Message: "Invalid JSON: " + err.Error(),
		})
		return nil, errors
	}

	// Validate individual fields
	errors = append(errors, validateSettings(&settings)...)

	if len(errors) > 0 {
		return &settings, errors
	}
	return &settings, nil
}

// validateSettings validates a ThiccSettings struct
func validateSettings(settings *ThiccSettings) []ValidationError {
	var errors []ValidationError

	// Validate scrollback lines
	if settings.Terminal.ScrollbackLines < 0 {
		errors = append(errors, ValidationError{
			Field:   "terminal.scrollback_lines",
			Message: "must be non-negative",
		})
	} else if settings.Terminal.ScrollbackLines > 1000000 {
		errors = append(errors, ValidationError{
			Field:   "terminal.scrollback_lines",
			Message: "must be <= 1000000",
		})
	}

	// Validate background color (hex format)
	if settings.Appearance.BackgroundColor != "" {
		if !isValidHexColor(settings.Appearance.BackgroundColor) {
			errors = append(errors, ValidationError{
				Field:   "appearance.background_color",
				Message: "must be a valid hex color (e.g., #0b0614)",
			})
		}
	}

	// Validate double-click threshold
	if settings.Editor.DoubleClickThresholdMs < 0 {
		errors = append(errors, ValidationError{
			Field:   "editor.double_click_threshold_ms",
			Message: "must be non-negative",
		})
	} else if settings.Editor.DoubleClickThresholdMs > 2000 {
		errors = append(errors, ValidationError{
			Field:   "editor.double_click_threshold_ms",
			Message: "must be <= 2000ms",
		})
	}

	return errors
}

// isValidHexColor checks if a string is a valid hex color
func isValidHexColor(color string) bool {
	if !strings.HasPrefix(color, "#") {
		return false
	}
	matched, _ := regexp.MatchString(`^#[0-9a-fA-F]{6}$`, color)
	return matched
}

// ReloadSettings reloads settings from disk and returns validation errors if any
func ReloadSettings() []ValidationError {
	filePath := GetSettingsFilePath()
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No settings file, use defaults
			GlobalThiccSettings = DefaultSettings()
			return nil
		}
		return []ValidationError{{
			Field:   "file",
			Message: "Failed to read settings file: " + err.Error(),
		}}
	}

	settings, errors := ValidateSettingsJSON(data)
	if len(errors) > 0 {
		return errors
	}

	// Apply defaults for zero values
	if settings.Terminal.ScrollbackLines == 0 {
		settings.Terminal.ScrollbackLines = DefaultScrollbackLines
	}
	if settings.Appearance.BackgroundColor == "" {
		settings.Appearance.BackgroundColor = DefaultBackgroundColor
	}
	if settings.Editor.DoubleClickThresholdMs == 0 {
		settings.Editor.DoubleClickThresholdMs = DefaultDoubleClickThresholdMs
	}

	GlobalThiccSettings = settings
	log.Printf("THICC Settings: Reloaded settings successfully")
	return nil
}

// IsSettingsFile checks if the given path is the THICC settings file
func IsSettingsFile(path string) bool {
	settingsPath := GetSettingsFilePath()
	// Compare absolute paths
	absPath, err1 := filepath.Abs(path)
	absSettings, err2 := filepath.Abs(settingsPath)
	if err1 != nil || err2 != nil {
		return path == settingsPath
	}
	return absPath == absSettings
}
