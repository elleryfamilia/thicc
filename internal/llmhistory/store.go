package llmhistory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const (
	// DBFileName is the name of the SQLite database file
	DBFileName = "llm_history.db"

	// DefaultRetentionDays is how long to keep history
	DefaultRetentionDays = 90

	// MaxOutputLength is the max length for tool output stored in DB
	MaxOutputLength = 10000
)

// Store manages LLM history persistence in SQLite
type Store struct {
	db     *sql.DB
	dbPath string
}

// NewStore creates a new Store, initializing the database if needed
func NewStore(configDir string) (*Store, error) {
	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}

	dbPath := filepath.Join(configDir, DBFileName)

	// Open SQLite database (creates if not exists)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	store := &Store{
		db:     db,
		dbPath: dbPath,
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates tables if they don't exist
func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		tool_name TEXT NOT NULL,
		tool_command TEXT,
		project_dir TEXT,
		start_time INTEGER NOT NULL,
		end_time INTEGER,
		output_bytes INTEGER DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project_dir);
	CREATE INDEX IF NOT EXISTS idx_sessions_time ON sessions(start_time DESC);
	CREATE INDEX IF NOT EXISTS idx_sessions_tool ON sessions(tool_name);

	CREATE TABLE IF NOT EXISTS tool_uses (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		tool_name TEXT NOT NULL,
		input TEXT,
		output TEXT,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_tool_uses_session ON tool_uses(session_id);
	CREATE INDEX IF NOT EXISTS idx_tool_uses_time ON tool_uses(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_tool_uses_tool ON tool_uses(tool_name);

	CREATE TABLE IF NOT EXISTS file_touches (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tool_use_id TEXT NOT NULL,
		file_path TEXT NOT NULL,
		FOREIGN KEY (tool_use_id) REFERENCES tool_uses(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_file_touches_path ON file_touches(file_path);
	CREATE INDEX IF NOT EXISTS idx_file_touches_tool_use ON file_touches(tool_use_id);

	CREATE TABLE IF NOT EXISTS session_output (
		session_id TEXT PRIMARY KEY,
		output TEXT NOT NULL,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
	);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Create FTS5 virtual table for full-text search (if not exists)
	// FTS5 doesn't support IF NOT EXISTS, so we check first
	var tableName string
	err := s.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='tool_uses_fts'").Scan(&tableName)
	if err == sql.ErrNoRows {
		ftsSchema := `
		CREATE VIRTUAL TABLE tool_uses_fts USING fts5(
			input,
			output,
			content='tool_uses',
			content_rowid='rowid'
		);

		-- Triggers to keep FTS index in sync
		CREATE TRIGGER IF NOT EXISTS tool_uses_ai AFTER INSERT ON tool_uses BEGIN
			INSERT INTO tool_uses_fts(rowid, input, output) VALUES (NEW.rowid, NEW.input, NEW.output);
		END;

		CREATE TRIGGER IF NOT EXISTS tool_uses_ad AFTER DELETE ON tool_uses BEGIN
			INSERT INTO tool_uses_fts(tool_uses_fts, rowid, input, output) VALUES('delete', OLD.rowid, OLD.input, OLD.output);
		END;

		CREATE TRIGGER IF NOT EXISTS tool_uses_au AFTER UPDATE ON tool_uses BEGIN
			INSERT INTO tool_uses_fts(tool_uses_fts, rowid, input, output) VALUES('delete', OLD.rowid, OLD.input, OLD.output);
			INSERT INTO tool_uses_fts(rowid, input, output) VALUES (NEW.rowid, NEW.input, NEW.output);
		END;
		`
		if _, err := s.db.Exec(ftsSchema); err != nil {
			return fmt.Errorf("failed to create FTS schema: %w", err)
		}
	}

	return nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// --- Session Operations ---

// CreateSession inserts a new session
func (s *Store) CreateSession(session *Session) error {
	_, err := s.db.Exec(`
		INSERT INTO sessions (id, tool_name, tool_command, project_dir, start_time, end_time, output_bytes)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		session.ID,
		session.ToolName,
		session.ToolCommand,
		session.ProjectDir,
		session.StartTime.Unix(),
		nullableTime(session.EndTime),
		session.OutputBytes,
	)
	return err
}

// UpdateSession updates an existing session
func (s *Store) UpdateSession(session *Session) error {
	_, err := s.db.Exec(`
		UPDATE sessions SET
			end_time = ?,
			output_bytes = ?
		WHERE id = ?`,
		nullableTime(session.EndTime),
		session.OutputBytes,
		session.ID,
	)
	return err
}

// GetSession retrieves a session by ID
func (s *Store) GetSession(id string) (*Session, error) {
	row := s.db.QueryRow(`
		SELECT id, tool_name, tool_command, project_dir, start_time, end_time, output_bytes
		FROM sessions WHERE id = ?`, id)

	return scanSession(row)
}

// ListSessions lists recent sessions, optionally filtered by project
func (s *Store) ListSessions(projectDir string, limit int) ([]Session, error) {
	if limit <= 0 {
		limit = 20
	}

	var rows *sql.Rows
	var err error

	if projectDir != "" {
		rows, err = s.db.Query(`
			SELECT id, tool_name, tool_command, project_dir, start_time, end_time, output_bytes
			FROM sessions
			WHERE project_dir = ?
			ORDER BY start_time DESC
			LIMIT ?`, projectDir, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT id, tool_name, tool_command, project_dir, start_time, end_time, output_bytes
			FROM sessions
			ORDER BY start_time DESC
			LIMIT ?`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		session, err := scanSessionRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *session)
	}
	return sessions, rows.Err()
}

// --- Session Output Operations ---

// SaveSessionOutput saves the deduplicated session output text
func (s *Store) SaveSessionOutput(sessionID, output string) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO session_output (session_id, output)
		VALUES (?, ?)`, sessionID, output)
	return err
}

// GetSessionOutput retrieves the session output text
func (s *Store) GetSessionOutput(sessionID string) (string, error) {
	var output string
	err := s.db.QueryRow(`SELECT output FROM session_output WHERE session_id = ?`, sessionID).Scan(&output)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return output, err
}

// --- Tool Use Operations ---

// CreateToolUse inserts a new tool use
func (s *Store) CreateToolUse(tu *ToolUse) error {
	// Truncate output if too long
	output := tu.Output
	if len(output) > MaxOutputLength {
		output = output[:MaxOutputLength] + "\n... [truncated]"
	}

	_, err := s.db.Exec(`
		INSERT INTO tool_uses (id, session_id, timestamp, tool_name, input, output)
		VALUES (?, ?, ?, ?, ?, ?)`,
		tu.ID,
		tu.SessionID,
		tu.Timestamp.Unix(),
		tu.ToolName,
		tu.Input,
		output,
	)
	return err
}

// GetToolUsesForSession retrieves all tool uses for a session
func (s *Store) GetToolUsesForSession(sessionID string) ([]ToolUse, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, timestamp, tool_name, input, output
		FROM tool_uses
		WHERE session_id = ?
		ORDER BY timestamp ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var toolUses []ToolUse
	for rows.Next() {
		tu, err := scanToolUseRows(rows)
		if err != nil {
			return nil, err
		}
		toolUses = append(toolUses, *tu)
	}
	return toolUses, rows.Err()
}

// --- File Touch Operations ---

// CreateFileTouch records a file being touched by a tool use
func (s *Store) CreateFileTouch(toolUseID, filePath string) error {
	_, err := s.db.Exec(`
		INSERT INTO file_touches (tool_use_id, file_path)
		VALUES (?, ?)`, toolUseID, filePath)
	return err
}

// GetFileHistory retrieves tool uses that touched specific files
func (s *Store) GetFileHistory(filePaths []string, limit int) ([]ToolUse, error) {
	if len(filePaths) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}

	// Build placeholders for IN clause
	placeholders := ""
	args := make([]interface{}, len(filePaths)+1)
	for i, fp := range filePaths {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
		args[i] = fp
	}
	args[len(filePaths)] = limit

	query := fmt.Sprintf(`
		SELECT DISTINCT t.id, t.session_id, t.timestamp, t.tool_name, t.input, t.output
		FROM tool_uses t
		JOIN file_touches f ON t.id = f.tool_use_id
		WHERE f.file_path IN (%s)
		ORDER BY t.timestamp DESC
		LIMIT ?`, placeholders)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var toolUses []ToolUse
	for rows.Next() {
		tu, err := scanToolUseRows(rows)
		if err != nil {
			return nil, err
		}
		toolUses = append(toolUses, *tu)
	}
	return toolUses, rows.Err()
}

// --- Search Operations ---

// Search performs full-text search across tool uses
func (s *Store) Search(query string, projectDir string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	var rows *sql.Rows
	var err error

	if projectDir != "" {
		rows, err = s.db.Query(`
			SELECT t.id, t.session_id, t.timestamp, t.tool_name, t.input, t.output,
			       snippet(tool_uses_fts, 0, '<b>', '</b>', '...', 32) as snippet,
			       bm25(tool_uses_fts) as score
			FROM tool_uses t
			JOIN tool_uses_fts fts ON t.rowid = fts.rowid
			JOIN sessions s ON t.session_id = s.id
			WHERE tool_uses_fts MATCH ?
			  AND s.project_dir = ?
			ORDER BY score
			LIMIT ?`, query, projectDir, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT t.id, t.session_id, t.timestamp, t.tool_name, t.input, t.output,
			       snippet(tool_uses_fts, 0, '<b>', '</b>', '...', 32) as snippet,
			       bm25(tool_uses_fts) as score
			FROM tool_uses t
			JOIN tool_uses_fts fts ON t.rowid = fts.rowid
			WHERE tool_uses_fts MATCH ?
			ORDER BY score
			LIMIT ?`, query, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var tu ToolUse
		var ts int64
		var snippet string
		var score float64

		err := rows.Scan(&tu.ID, &tu.SessionID, &ts, &tu.ToolName, &tu.Input, &tu.Output, &snippet, &score)
		if err != nil {
			return nil, err
		}
		tu.Timestamp = time.Unix(ts, 0)

		results = append(results, SearchResult{
			ToolUse:   tu,
			SessionID: tu.SessionID,
			Snippet:   snippet,
			Score:     score,
		})
	}
	return results, rows.Err()
}

// --- Cleanup Operations ---

// Cleanup removes old sessions beyond the retention period
func (s *Store) Cleanup(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = DefaultRetentionDays
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays).Unix()

	result, err := s.db.Exec(`DELETE FROM sessions WHERE start_time < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// --- Size Management Operations ---

// GetDBSize returns the current database file size in bytes
func (s *Store) GetDBSize() (int64, error) {
	info, err := os.Stat(s.dbPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// GetStats returns database statistics
func (s *Store) GetStats() (*DBStats, error) {
	stats := &DBStats{}

	// Get file size
	size, err := s.GetDBSize()
	if err != nil {
		return nil, err
	}
	stats.FileSizeBytes = size
	stats.FileSizeMB = float64(size) / (1024 * 1024)

	// Get session count
	err = s.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&stats.SessionCount)
	if err != nil {
		return nil, err
	}

	// Get tool use count
	err = s.db.QueryRow(`SELECT COUNT(*) FROM tool_uses`).Scan(&stats.ToolUseCount)
	if err != nil {
		return nil, err
	}

	// Get output count
	err = s.db.QueryRow(`SELECT COUNT(*) FROM session_output`).Scan(&stats.OutputCount)
	if err != nil {
		return nil, err
	}

	// Get oldest and newest session times
	var oldestTs, newestTs sql.NullInt64
	err = s.db.QueryRow(`SELECT MIN(start_time), MAX(start_time) FROM sessions`).Scan(&oldestTs, &newestTs)
	if err != nil {
		return nil, err
	}
	if oldestTs.Valid {
		stats.OldestSession = time.Unix(oldestTs.Int64, 0)
	}
	if newestTs.Valid {
		stats.NewestSession = time.Unix(newestTs.Int64, 0)
	}

	return stats, nil
}

// EnforceSizeLimit deletes oldest sessions until database is under the size limit
// Returns the number of sessions deleted
func (s *Store) EnforceSizeLimit(maxBytes int64) (int64, error) {
	if maxBytes <= 0 {
		return 0, nil // Unlimited
	}

	currentSize, err := s.GetDBSize()
	if err != nil {
		return 0, err
	}

	// Only cleanup if over threshold (90% of limit)
	threshold := int64(float64(maxBytes) * CleanupThreshold)
	if currentSize <= threshold {
		return 0, nil
	}

	// Target size is 70% of limit
	target := int64(float64(maxBytes) * CleanupTarget)

	var deleted int64
	for currentSize > target {
		// Get oldest session
		var oldestID string
		err := s.db.QueryRow(`SELECT id FROM sessions ORDER BY start_time ASC LIMIT 1`).Scan(&oldestID)
		if err == sql.ErrNoRows {
			break // No more sessions
		}
		if err != nil {
			return deleted, err
		}

		// Delete the session (cascade deletes related records)
		_, err = s.db.Exec(`DELETE FROM sessions WHERE id = ?`, oldestID)
		if err != nil {
			return deleted, err
		}
		deleted++

		// Re-check size (need to vacuum to actually reclaim space, but estimate for now)
		currentSize, err = s.GetDBSize()
		if err != nil {
			return deleted, err
		}
	}

	return deleted, nil
}

// Vacuum runs VACUUM to reclaim disk space after deletions
func (s *Store) Vacuum() error {
	_, err := s.db.Exec(`VACUUM`)
	return err
}

// --- Helper Functions ---

func nullableTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.Unix()
}

func scanSession(row *sql.Row) (*Session, error) {
	var session Session
	var startTime int64
	var endTime sql.NullInt64
	var toolCommand sql.NullString
	var projectDir sql.NullString

	err := row.Scan(&session.ID, &session.ToolName, &toolCommand, &projectDir,
		&startTime, &endTime, &session.OutputBytes)
	if err != nil {
		return nil, err
	}

	session.StartTime = time.Unix(startTime, 0)
	if endTime.Valid {
		session.EndTime = time.Unix(endTime.Int64, 0)
	}
	if toolCommand.Valid {
		session.ToolCommand = toolCommand.String
	}
	if projectDir.Valid {
		session.ProjectDir = projectDir.String
	}
	return &session, nil
}

func scanSessionRows(rows *sql.Rows) (*Session, error) {
	var session Session
	var startTime int64
	var endTime sql.NullInt64
	var toolCommand sql.NullString
	var projectDir sql.NullString

	err := rows.Scan(&session.ID, &session.ToolName, &toolCommand, &projectDir,
		&startTime, &endTime, &session.OutputBytes)
	if err != nil {
		return nil, err
	}

	session.StartTime = time.Unix(startTime, 0)
	if endTime.Valid {
		session.EndTime = time.Unix(endTime.Int64, 0)
	}
	if toolCommand.Valid {
		session.ToolCommand = toolCommand.String
	}
	if projectDir.Valid {
		session.ProjectDir = projectDir.String
	}
	return &session, nil
}

func scanToolUseRows(rows *sql.Rows) (*ToolUse, error) {
	var tu ToolUse
	var ts int64
	var input sql.NullString
	var output sql.NullString

	err := rows.Scan(&tu.ID, &tu.SessionID, &ts, &tu.ToolName, &input, &output)
	if err != nil {
		return nil, err
	}

	tu.Timestamp = time.Unix(ts, 0)
	if input.Valid {
		tu.Input = input.String
	}
	if output.Valid {
		tu.Output = output.String
	}
	return &tu, nil
}
