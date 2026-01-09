package dashboard

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ellery/thicc/internal/config"
)

const (
	// MaxRecentProjects is the maximum number of recent projects to store
	MaxRecentProjects = 10

	// RecentFileName is the name of the recent projects file
	RecentFileName = "recent.json"

	// ThiccConfigSubdir is the subdirectory for thock-specific config
	ThiccConfigSubdir = "thicc"
)

// RecentProject represents a recently opened file or folder
type RecentProject struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	IsFolder   bool      `json:"is_folder"`
	LastOpened time.Time `json:"last_opened"`
}

// RecentStore manages persistent storage of recent projects
type RecentStore struct {
	Projects []RecentProject `json:"projects"`
}

// NewRecentStore creates a new RecentStore
func NewRecentStore() *RecentStore {
	return &RecentStore{
		Projects: make([]RecentProject, 0),
	}
}

// GetConfigDir returns the thock-specific config directory
func GetConfigDir() string {
	return filepath.Join(config.ConfigDir, ThiccConfigSubdir)
}

// GetRecentFilePath returns the path to the recent.json file
func GetRecentFilePath() string {
	return filepath.Join(GetConfigDir(), RecentFileName)
}

// EnsureConfigDir creates the thock config directory if it doesn't exist
func EnsureConfigDir() error {
	dir := GetConfigDir()
	return os.MkdirAll(dir, 0755)
}

// Load reads recent projects from disk
func (rs *RecentStore) Load() error {
	filePath := GetRecentFilePath()

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No recent file yet, that's fine
			return nil
		}
		log.Printf("THOCK Dashboard: Failed to read recent.json: %v", err)
		return err
	}

	if err := json.Unmarshal(data, rs); err != nil {
		log.Printf("THOCK Dashboard: Failed to parse recent.json: %v", err)
		return err
	}

	// Validate and clean up entries (remove non-existent paths)
	rs.cleanup()

	return nil
}

// Save writes recent projects to disk
func (rs *RecentStore) Save() error {
	if err := EnsureConfigDir(); err != nil {
		log.Printf("THOCK Dashboard: Failed to create config dir: %v", err)
		return err
	}

	data, err := json.MarshalIndent(rs, "", "  ")
	if err != nil {
		log.Printf("THOCK Dashboard: Failed to marshal recent.json: %v", err)
		return err
	}

	filePath := GetRecentFilePath()
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Printf("THOCK Dashboard: Failed to write recent.json: %v", err)
		return err
	}

	return nil
}

// AddProject adds or updates a project in the recent list
// Only directories are added; individual files are ignored
func (rs *RecentStore) AddProject(path string, isFolder bool) {
	// Only add directories, not individual files
	if !isFolder {
		return
	}

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Get the name from the path
	name := filepath.Base(absPath)

	// Check if project already exists
	for i, proj := range rs.Projects {
		if proj.Path == absPath {
			// Update existing entry
			rs.Projects[i].LastOpened = time.Now()
			rs.sort()
			rs.Save()
			return
		}
	}

	// Add new entry
	newProject := RecentProject{
		Path:       absPath,
		Name:       name,
		IsFolder:   isFolder,
		LastOpened: time.Now(),
	}

	rs.Projects = append([]RecentProject{newProject}, rs.Projects...)

	// Trim to max size
	if len(rs.Projects) > MaxRecentProjects {
		rs.Projects = rs.Projects[:MaxRecentProjects]
	}

	rs.Save()
}

// RemoveProject removes a project from the recent list
func (rs *RecentStore) RemoveProject(path string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	for i, proj := range rs.Projects {
		if proj.Path == absPath {
			rs.Projects = append(rs.Projects[:i], rs.Projects[i+1:]...)
			rs.Save()
			return
		}
	}
}

// sort orders projects by last opened time (most recent first)
func (rs *RecentStore) sort() {
	sort.Slice(rs.Projects, func(i, j int) bool {
		return rs.Projects[i].LastOpened.After(rs.Projects[j].LastOpened)
	})
}

// cleanup removes entries that no longer exist on disk or are not directories
func (rs *RecentStore) cleanup() {
	validProjects := make([]RecentProject, 0, len(rs.Projects))

	for _, proj := range rs.Projects {
		info, err := os.Stat(proj.Path)
		if err == nil && info.IsDir() {
			validProjects = append(validProjects, proj)
		}
	}

	if len(validProjects) != len(rs.Projects) {
		rs.Projects = validProjects
		rs.Save()
	}
}
