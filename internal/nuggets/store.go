package nuggets

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// NuggetsFileName is the name of the committed nuggets file
	NuggetsFileName = ".agent-nuggets.json"
	// HistoryDir is the directory for ephemeral/local files
	HistoryDir = ".agent-history"
	// PendingNuggetsFileName is the name of the pending nuggets file
	PendingNuggetsFileName = "pending-nuggets.json"
)

// Store handles reading and writing nugget files
type Store struct {
	projectRoot string
}

// NewStore creates a new nugget store for the given project root
func NewStore(projectRoot string) *Store {
	return &Store{projectRoot: projectRoot}
}

// NewStoreFromCwd creates a new nugget store using the current working directory
func NewStoreFromCwd() (*Store, error) {
	root, err := FindProjectRoot()
	if err != nil {
		return nil, err
	}
	return NewStore(root), nil
}

// FindProjectRoot finds the git repository root from the current directory
func FindProjectRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		// Not in a git repo, use current working directory
		return os.Getwd()
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCurrentCommit returns the current git HEAD commit SHA
func GetCurrentCommit() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// nuggetsFilePath returns the path to .agent-nuggets.json
func (s *Store) nuggetsFilePath() string {
	return filepath.Join(s.projectRoot, NuggetsFileName)
}

// historyDir returns the path to .agent-history directory
func (s *Store) historyDir() string {
	return filepath.Join(s.projectRoot, HistoryDir)
}

// pendingNuggetsFilePath returns the path to pending-nuggets.json
func (s *Store) pendingNuggetsFilePath() string {
	return filepath.Join(s.historyDir(), PendingNuggetsFileName)
}

// ensureHistoryDir creates the .agent-history directory if it doesn't exist
func (s *Store) ensureHistoryDir() error {
	dir := s.historyDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create history directory: %w", err)
		}
	}
	return nil
}

// LoadNuggets reads the committed nuggets from .agent-nuggets.json
func (s *Store) LoadNuggets() (*NuggetFile, error) {
	path := s.nuggetsFilePath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewNuggetFile(), nil
		}
		return nil, fmt.Errorf("failed to read nuggets file: %w", err)
	}

	var nuggetFile NuggetFile
	if err := json.Unmarshal(data, &nuggetFile); err != nil {
		return nil, fmt.Errorf("failed to parse nuggets file: %w", err)
	}

	return &nuggetFile, nil
}

// SaveNuggets writes nuggets to .agent-nuggets.json
func (s *Store) SaveNuggets(nuggetFile *NuggetFile) error {
	path := s.nuggetsFilePath()

	data, err := json.MarshalIndent(nuggetFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal nuggets: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write nuggets file: %w", err)
	}

	return nil
}

// LoadPendingNuggets reads pending nuggets from .agent-history/pending-nuggets.json
func (s *Store) LoadPendingNuggets() (*PendingNuggetFile, error) {
	path := s.pendingNuggetsFilePath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewPendingNuggetFile(), nil
		}
		return nil, fmt.Errorf("failed to read pending nuggets file: %w", err)
	}

	var pendingFile PendingNuggetFile
	if err := json.Unmarshal(data, &pendingFile); err != nil {
		return nil, fmt.Errorf("failed to parse pending nuggets file: %w", err)
	}

	return &pendingFile, nil
}

// SavePendingNuggets writes pending nuggets to .agent-history/pending-nuggets.json
func (s *Store) SavePendingNuggets(pendingFile *PendingNuggetFile) error {
	if err := s.ensureHistoryDir(); err != nil {
		return err
	}

	path := s.pendingNuggetsFilePath()

	data, err := json.MarshalIndent(pendingFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal pending nuggets: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write pending nuggets file: %w", err)
	}

	return nil
}

// AddPendingNugget adds a new nugget to the pending list
func (s *Store) AddPendingNugget(nugget *Nugget) error {
	pending, err := s.LoadPendingNuggets()
	if err != nil {
		return err
	}

	// Generate ID if not set
	if nugget.ID == "" {
		nugget.ID = generateNuggetID()
	}

	// Set created time if not set
	if nugget.Created.IsZero() {
		nugget.Created = time.Now()
	}

	pending.Nuggets = append(pending.Nuggets, *nugget)
	return s.SavePendingNuggets(pending)
}

// AcceptNugget moves a nugget from pending to committed
func (s *Store) AcceptNugget(id string) error {
	pending, err := s.LoadPendingNuggets()
	if err != nil {
		return err
	}

	// Find the nugget
	var nugget *Nugget
	var newPending []Nugget
	for _, n := range pending.Nuggets {
		if n.ID == id || (len(id) >= 8 && len(n.ID) >= 8 && n.ID[:8] == id[:8]) {
			nuggetCopy := n
			nugget = &nuggetCopy
		} else {
			newPending = append(newPending, n)
		}
	}

	if nugget == nil {
		return errors.New("nugget not found in pending list")
	}

	// Add current commit if available
	if nugget.Commit == "" {
		nugget.Commit = GetCurrentCommit()
	}

	// Load committed nuggets
	nuggetFile, err := s.LoadNuggets()
	if err != nil {
		return err
	}

	// Add nugget to committed list
	nuggetFile.Nuggets = append(nuggetFile.Nuggets, *nugget)

	// Save both files
	if err := s.SaveNuggets(nuggetFile); err != nil {
		return err
	}

	pending.Nuggets = newPending
	return s.SavePendingNuggets(pending)
}

// RejectNugget removes a nugget from the pending list
func (s *Store) RejectNugget(id string) error {
	pending, err := s.LoadPendingNuggets()
	if err != nil {
		return err
	}

	found := false
	var newPending []Nugget
	for _, n := range pending.Nuggets {
		if n.ID == id || (len(id) >= 8 && len(n.ID) >= 8 && n.ID[:8] == id[:8]) {
			found = true
		} else {
			newPending = append(newPending, n)
		}
	}

	if !found {
		return errors.New("nugget not found in pending list")
	}

	pending.Nuggets = newPending
	return s.SavePendingNuggets(pending)
}

// GetNugget finds a nugget by ID (searches both committed and pending)
func (s *Store) GetNugget(id string) (*Nugget, bool, error) {
	// Search committed nuggets
	nuggetFile, err := s.LoadNuggets()
	if err != nil {
		return nil, false, err
	}

	for _, n := range nuggetFile.Nuggets {
		if n.ID == id || (len(id) >= 8 && len(n.ID) >= 8 && n.ID[:8] == id[:8]) {
			return &n, false, nil // false = not pending
		}
	}

	// Search pending nuggets
	pending, err := s.LoadPendingNuggets()
	if err != nil {
		return nil, false, err
	}

	for _, n := range pending.Nuggets {
		if n.ID == id || (len(id) >= 8 && len(n.ID) >= 8 && n.ID[:8] == id[:8]) {
			return &n, true, nil // true = pending
		}
	}

	return nil, false, errors.New("nugget not found")
}

// SearchNuggets searches nuggets by text query
func (s *Store) SearchNuggets(query string) ([]Nugget, error) {
	query = strings.ToLower(query)
	var results []Nugget

	nuggetFile, err := s.LoadNuggets()
	if err != nil {
		return nil, err
	}

	for _, n := range nuggetFile.Nuggets {
		if matchesQuery(n, query) {
			results = append(results, n)
		}
	}

	return results, nil
}

// matchesQuery checks if a nugget matches a search query
func matchesQuery(n Nugget, query string) bool {
	// Check title
	if strings.Contains(strings.ToLower(n.Title), query) {
		return true
	}
	// Check summary
	if strings.Contains(strings.ToLower(n.Summary), query) {
		return true
	}
	// Check tags
	for _, tag := range n.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	// Check files
	for _, file := range n.Files {
		if strings.Contains(strings.ToLower(file), query) {
			return true
		}
	}
	// Check user notes
	if strings.Contains(strings.ToLower(n.UserNotes), query) {
		return true
	}
	return false
}

// generateNuggetID generates a unique ID for a nugget
func generateNuggetID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("nugget-%d", time.Now().UnixNano())
	}
	return "nugget-" + hex.EncodeToString(bytes)
}

// AddNugget adds a nugget directly to committed nuggets (for manual additions)
func (s *Store) AddNugget(nugget *Nugget) error {
	nuggetFile, err := s.LoadNuggets()
	if err != nil {
		return err
	}

	// Generate ID if not set
	if nugget.ID == "" {
		nugget.ID = generateNuggetID()
	}

	// Set created time if not set
	if nugget.Created.IsZero() {
		nugget.Created = time.Now()
	}

	// Set current commit
	if nugget.Commit == "" {
		nugget.Commit = GetCurrentCommit()
	}

	nuggetFile.Nuggets = append(nuggetFile.Nuggets, *nugget)
	return s.SaveNuggets(nuggetFile)
}

// GetProjectRoot returns the project root directory
func (s *Store) GetProjectRoot() string {
	return s.projectRoot
}
