package dashboard

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

const (
	// PreferencesFileName is the name of the preferences file
	PreferencesFileName = "preferences.json"
)

// Preferences stores user preferences for the dashboard
type Preferences struct {
	// SelectedAITool is the command name of the selected AI tool (e.g., "claude", "aider")
	// Empty string means no tool selected (use default shell)
	SelectedAITool string `json:"selected_ai_tool"`

	// HasSeenOnboarding tracks whether the user has seen the first-run guide
	HasSeenOnboarding bool `json:"has_seen_onboarding"`
}

// PreferencesStore manages persistent storage of dashboard preferences
type PreferencesStore struct {
	Prefs Preferences `json:"preferences"`
}

// NewPreferencesStore creates a new PreferencesStore
func NewPreferencesStore() *PreferencesStore {
	return &PreferencesStore{
		Prefs: Preferences{
			SelectedAITool: "", // Default: no AI tool selected
		},
	}
}

// GetPreferencesFilePath returns the path to the preferences.json file
func GetPreferencesFilePath() string {
	return filepath.Join(GetConfigDir(), PreferencesFileName)
}

// Load reads preferences from disk
func (ps *PreferencesStore) Load() error {
	filePath := GetPreferencesFilePath()

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No preferences file yet, use defaults
			return nil
		}
		log.Printf("THICC Dashboard: Failed to read preferences.json: %v", err)
		return err
	}

	if err := json.Unmarshal(data, ps); err != nil {
		log.Printf("THICC Dashboard: Failed to parse preferences.json: %v", err)
		return err
	}

	return nil
}

// Save writes preferences to disk
func (ps *PreferencesStore) Save() error {
	if err := EnsureConfigDir(); err != nil {
		log.Printf("THICC Dashboard: Failed to create config dir: %v", err)
		return err
	}

	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		log.Printf("THICC Dashboard: Failed to marshal preferences.json: %v", err)
		return err
	}

	filePath := GetPreferencesFilePath()
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Printf("THICC Dashboard: Failed to write preferences.json: %v", err)
		return err
	}

	return nil
}

// GetSelectedAITool returns the command name of the selected AI tool
func (ps *PreferencesStore) GetSelectedAITool() string {
	return ps.Prefs.SelectedAITool
}

// SetSelectedAITool sets the selected AI tool by command name
func (ps *PreferencesStore) SetSelectedAITool(command string) {
	ps.Prefs.SelectedAITool = command
	ps.Save()
}

// ClearSelectedAITool clears the AI tool selection (use default shell)
func (ps *PreferencesStore) ClearSelectedAITool() {
	ps.Prefs.SelectedAITool = ""
	ps.Save()
}

// HasSelectedAITool returns true if an AI tool is selected
func (ps *PreferencesStore) HasSelectedAITool() bool {
	return ps.Prefs.SelectedAITool != ""
}

// HasSeenOnboarding returns true if the user has seen the first-run guide
func (ps *PreferencesStore) HasSeenOnboarding() bool {
	return ps.Prefs.HasSeenOnboarding
}

// MarkOnboardingComplete marks the onboarding guide as seen and saves
func (ps *PreferencesStore) MarkOnboardingComplete() {
	ps.Prefs.HasSeenOnboarding = true
	ps.Save()
}
