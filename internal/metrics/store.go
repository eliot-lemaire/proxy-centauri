package metrics

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store persists metric snapshots and infrastructure events to a SQLite database.
type Store struct {
	db *sql.DB
}

// OpenStore opens (or creates) the SQLite database at path, creating any
// intermediate directories as needed.
func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// SQLite supports only one writer at a time. Limiting to one open connection
	// serialises all writes and avoids SQLITE_BUSY errors under concurrent callers.
	db.SetMaxOpenConns(1)
	return &Store{db: db}, nil
}

// Init creates the required tables if they do not already exist.
func (s *Store) Init() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS request_stats (
			ts        INTEGER,
			gate      TEXT,
			req_total INTEGER,
			err_total INTEGER,
			p95_ms    REAL
		)
	`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS events (
			ts     INTEGER,
			gate   TEXT,
			kind   TEXT,
			detail TEXT
		)
	`); err != nil {
		return err
	}
	return nil
}

// Flush writes a per-gate metrics snapshot to request_stats.
func (s *Store) Flush(snap MetricsSnapshot) error {
	_, err := s.db.Exec(
		`INSERT INTO request_stats (ts, gate, req_total, err_total, p95_ms) VALUES (?, ?, ?, ?, ?)`,
		time.Now().Unix(), snap.Gate, snap.ReqTotal, snap.ErrTotal, snap.P95Ms,
	)
	return err
}

// LogEvent records an infrastructure event (backend_up, backend_down, config_reload) to events.
func (s *Store) LogEvent(gate, kind, detail string) error {
	_, err := s.db.Exec(
		`INSERT INTO events (ts, gate, kind, detail) VALUES (?, ?, ?, ?)`,
		time.Now().Unix(), gate, kind, detail,
	)
	return err
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
