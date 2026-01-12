package gems

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
	// GemsFileName is the name of the committed gems file
	GemsFileName = ".agent-gems.json"
	// HistoryDir is the directory for ephemeral/local files
	HistoryDir = ".agent-history"
	// PendingGemsFileName is the name of the pending gems file
	PendingGemsFileName = "pending-gems.json"
)

// Store handles reading and writing gem files
type Store struct {
	projectRoot string
}

// NewStore creates a new gem store for the given project root
func NewStore(projectRoot string) *Store {
	return &Store{projectRoot: projectRoot}
}

// NewStoreFromCwd creates a new gem store using the current working directory
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

// gemsFilePath returns the path to .agent-gems.json
func (s *Store) gemsFilePath() string {
	return filepath.Join(s.projectRoot, GemsFileName)
}

// historyDir returns the path to .agent-history directory
func (s *Store) historyDir() string {
	return filepath.Join(s.projectRoot, HistoryDir)
}

// pendingGemsFilePath returns the path to pending-gems.json
func (s *Store) pendingGemsFilePath() string {
	return filepath.Join(s.historyDir(), PendingGemsFileName)
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

// LoadGems reads the committed gems from .agent-gems.json
func (s *Store) LoadGems() (*GemFile, error) {
	path := s.gemsFilePath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewGemFile(), nil
		}
		return nil, fmt.Errorf("failed to read gems file: %w", err)
	}

	var gemFile GemFile
	if err := json.Unmarshal(data, &gemFile); err != nil {
		return nil, fmt.Errorf("failed to parse gems file: %w", err)
	}

	return &gemFile, nil
}

// SaveGems writes gems to .agent-gems.json
func (s *Store) SaveGems(gemFile *GemFile) error {
	path := s.gemsFilePath()

	data, err := json.MarshalIndent(gemFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal gems: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write gems file: %w", err)
	}

	return nil
}

// LoadPendingGems reads pending gems from .agent-history/pending-gems.json
func (s *Store) LoadPendingGems() (*PendingGemFile, error) {
	path := s.pendingGemsFilePath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewPendingGemFile(), nil
		}
		return nil, fmt.Errorf("failed to read pending gems file: %w", err)
	}

	var pendingFile PendingGemFile
	if err := json.Unmarshal(data, &pendingFile); err != nil {
		return nil, fmt.Errorf("failed to parse pending gems file: %w", err)
	}

	return &pendingFile, nil
}

// SavePendingGems writes pending gems to .agent-history/pending-gems.json
func (s *Store) SavePendingGems(pendingFile *PendingGemFile) error {
	if err := s.ensureHistoryDir(); err != nil {
		return err
	}

	path := s.pendingGemsFilePath()

	data, err := json.MarshalIndent(pendingFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal pending gems: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write pending gems file: %w", err)
	}

	return nil
}

// AddPendingGem adds a new gem to the pending list
func (s *Store) AddPendingGem(gem *Gem) error {
	pending, err := s.LoadPendingGems()
	if err != nil {
		return err
	}

	// Generate ID if not set
	if gem.ID == "" {
		gem.ID = generateGemID()
	}

	// Set created time if not set
	if gem.Created.IsZero() {
		gem.Created = time.Now()
	}

	pending.Gems = append(pending.Gems, *gem)
	return s.SavePendingGems(pending)
}

// AcceptGem moves a gem from pending to committed
func (s *Store) AcceptGem(id string) error {
	pending, err := s.LoadPendingGems()
	if err != nil {
		return err
	}

	// Find the gem
	var gem *Gem
	var newPending []Gem
	for _, g := range pending.Gems {
		if g.ID == id || (len(id) >= 8 && len(g.ID) >= 8 && g.ID[:8] == id[:8]) {
			gemCopy := g
			gem = &gemCopy
		} else {
			newPending = append(newPending, g)
		}
	}

	if gem == nil {
		return errors.New("gem not found in pending list")
	}

	// Add current commit if available
	if gem.Commit == "" {
		gem.Commit = GetCurrentCommit()
	}

	// Load committed gems
	gemFile, err := s.LoadGems()
	if err != nil {
		return err
	}

	// Add gem to committed list
	gemFile.Gems = append(gemFile.Gems, *gem)

	// Save both files
	if err := s.SaveGems(gemFile); err != nil {
		return err
	}

	pending.Gems = newPending
	return s.SavePendingGems(pending)
}

// RejectGem removes a gem from the pending list
func (s *Store) RejectGem(id string) error {
	pending, err := s.LoadPendingGems()
	if err != nil {
		return err
	}

	found := false
	var newPending []Gem
	for _, g := range pending.Gems {
		if g.ID == id || (len(id) >= 8 && len(g.ID) >= 8 && g.ID[:8] == id[:8]) {
			found = true
		} else {
			newPending = append(newPending, g)
		}
	}

	if !found {
		return errors.New("gem not found in pending list")
	}

	pending.Gems = newPending
	return s.SavePendingGems(pending)
}

// GetGem finds a gem by ID (searches both committed and pending)
func (s *Store) GetGem(id string) (*Gem, bool, error) {
	// Search committed gems
	gemFile, err := s.LoadGems()
	if err != nil {
		return nil, false, err
	}

	for _, g := range gemFile.Gems {
		if g.ID == id || (len(id) >= 8 && len(g.ID) >= 8 && g.ID[:8] == id[:8]) {
			return &g, false, nil // false = not pending
		}
	}

	// Search pending gems
	pending, err := s.LoadPendingGems()
	if err != nil {
		return nil, false, err
	}

	for _, g := range pending.Gems {
		if g.ID == id || (len(id) >= 8 && len(g.ID) >= 8 && g.ID[:8] == id[:8]) {
			return &g, true, nil // true = pending
		}
	}

	return nil, false, errors.New("gem not found")
}

// SearchGems searches gems by text query
func (s *Store) SearchGems(query string) ([]Gem, error) {
	query = strings.ToLower(query)
	var results []Gem

	gemFile, err := s.LoadGems()
	if err != nil {
		return nil, err
	}

	for _, g := range gemFile.Gems {
		if matchesQuery(g, query) {
			results = append(results, g)
		}
	}

	return results, nil
}

// matchesQuery checks if a gem matches a search query
func matchesQuery(g Gem, query string) bool {
	// Check title
	if strings.Contains(strings.ToLower(g.Title), query) {
		return true
	}
	// Check summary
	if strings.Contains(strings.ToLower(g.Summary), query) {
		return true
	}
	// Check tags
	for _, tag := range g.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	// Check files
	for _, file := range g.Files {
		if strings.Contains(strings.ToLower(file), query) {
			return true
		}
	}
	// Check user notes
	if strings.Contains(strings.ToLower(g.UserNotes), query) {
		return true
	}
	return false
}

// generateGemID generates a unique ID for a gem
func generateGemID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("gem-%d", time.Now().UnixNano())
	}
	return "gem-" + hex.EncodeToString(bytes)
}

// AddGem adds a gem directly to committed gems (for manual additions)
func (s *Store) AddGem(gem *Gem) error {
	gemFile, err := s.LoadGems()
	if err != nil {
		return err
	}

	// Generate ID if not set
	if gem.ID == "" {
		gem.ID = generateGemID()
	}

	// Set created time if not set
	if gem.Created.IsZero() {
		gem.Created = time.Now()
	}

	// Set current commit
	if gem.Commit == "" {
		gem.Commit = GetCurrentCommit()
	}

	gemFile.Gems = append(gemFile.Gems, *gem)
	return s.SaveGems(gemFile)
}

// GetProjectRoot returns the project root directory
func (s *Store) GetProjectRoot() string {
	return s.projectRoot
}
