package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/ellery/thicc/internal/config"
)

// UpdateState stores persistent state for update checking
type UpdateState struct {
	LastCheck      time.Time `json:"last_check"`
	SkippedVersion string    `json:"skipped_version"`
}

const stateFileName = "update_state.json"

// statePath returns the path to the state file
func statePath() string {
	return filepath.Join(config.ConfigDir, stateFileName)
}

// LoadState loads the update state from disk
func LoadState() (*UpdateState, error) {
	state := &UpdateState{}

	data, err := os.ReadFile(statePath())
	if err != nil {
		if os.IsNotExist(err) {
			// No state file yet, return empty state
			return state, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, state); err != nil {
		// Corrupted state file, return empty state
		return &UpdateState{}, nil
	}

	return state, nil
}

// Save persists the update state to disk
func (s *UpdateState) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(statePath(), data, 0644)
}

// ShouldCheck returns true if enough time has passed since the last check
func (s *UpdateState) ShouldCheck(frequencyDays float64) bool {
	if frequencyDays <= 0 {
		// 0 or negative means check every time
		return true
	}

	if s.LastCheck.IsZero() {
		// Never checked before
		return true
	}

	daysSinceLastCheck := time.Since(s.LastCheck).Hours() / 24
	return daysSinceLastCheck >= frequencyDays
}

// MarkChecked updates the last check time to now
func (s *UpdateState) MarkChecked() {
	s.LastCheck = time.Now()
}

// SkipVersion marks a version as skipped
func (s *UpdateState) SkipVersion(version string) {
	s.SkippedVersion = version
}

// IsVersionSkipped returns true if the given version was skipped by the user
func (s *UpdateState) IsVersionSkipped(version string) bool {
	return s.SkippedVersion == version
}
