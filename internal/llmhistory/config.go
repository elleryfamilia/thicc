package llmhistory

import (
	"github.com/ellery/thicc/internal/config"
)

const (
	// Default settings
	DefaultMaxSizeMB = 100

	// Size enforcement thresholds
	CleanupThreshold = 0.9 // Trigger cleanup at 90% of limit
	CleanupTarget    = 0.7 // Clean down to 70% of limit
)

// RegisterSettings registers LLM history settings with the config system
// Must be called before config.InitGlobalSettings()
func RegisterSettings() {
	config.RegisterGlobalOption("llmhistory.enabled", true)
	config.RegisterGlobalOption("llmhistory.maxsizemb", float64(DefaultMaxSizeMB))
	config.RegisterGlobalOption("llmhistory.mcpenabled", true)
}

// IsEnabled returns whether LLM history recording is enabled
func IsEnabled() bool {
	val := config.GetGlobalOption("llmhistory.enabled")
	if val == nil {
		return true // Default enabled
	}
	enabled, ok := val.(bool)
	if !ok {
		return true
	}
	return enabled
}

// IsMCPEnabled returns whether the MCP server is enabled
func IsMCPEnabled() bool {
	val := config.GetGlobalOption("llmhistory.mcpenabled")
	if val == nil {
		return true // Default enabled
	}
	enabled, ok := val.(bool)
	if !ok {
		return true
	}
	return enabled
}

// GetMaxSizeBytes returns the maximum database size in bytes
// Returns 0 if unlimited
func GetMaxSizeBytes() int64 {
	val := config.GetGlobalOption("llmhistory.maxsizemb")
	if val == nil {
		return DefaultMaxSizeMB * 1024 * 1024
	}
	mb, ok := val.(float64)
	if !ok {
		return DefaultMaxSizeMB * 1024 * 1024
	}
	if mb <= 0 {
		return 0 // Unlimited
	}
	return int64(mb * 1024 * 1024)
}

// GetMaxSizeMB returns the maximum database size in MB
func GetMaxSizeMB() int {
	val := config.GetGlobalOption("llmhistory.maxsizemb")
	if val == nil {
		return DefaultMaxSizeMB
	}
	mb, ok := val.(float64)
	if !ok {
		return DefaultMaxSizeMB
	}
	return int(mb)
}
