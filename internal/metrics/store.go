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

// ThreatSignal is a single Oracle alert stored in SQLite.
type ThreatSignal struct {
	ID        int64
	Ts        int64
	Gate      string
	Kind      string // "threat" | "scaling"
	Level     string // "low" | "medium" | "high" | "critical"
	Summary   string
	Reasoning string
	Action    string
	Resolved  bool
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
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS threat_signals (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			ts        INTEGER NOT NULL,
			gate      TEXT NOT NULL,
			kind      TEXT NOT NULL,
			level     TEXT NOT NULL,
			summary   TEXT NOT NULL,
			reasoning TEXT NOT NULL,
			action    TEXT NOT NULL,
			resolved  INTEGER NOT NULL DEFAULT 0
		)
	`); err != nil {
		return err
	}
	return nil
}

// SaveSignal persists a new Threat Signal from The Oracle.
func (s *Store) SaveSignal(sig ThreatSignal) error {
	_, err := s.db.Exec(
		`INSERT INTO threat_signals (ts, gate, kind, level, summary, reasoning, action)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		time.Now().Unix(), sig.Gate, sig.Kind, sig.Level, sig.Summary, sig.Reasoning, sig.Action,
	)
	return err
}

// ListSignals returns the N most recent unresolved signals, newest first.
func (s *Store) ListSignals(limit int) ([]ThreatSignal, error) {
	rows, err := s.db.Query(
		`SELECT id, ts, gate, kind, level, summary, reasoning, action, resolved
		 FROM threat_signals WHERE resolved = 0
		 ORDER BY id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signals []ThreatSignal
	for rows.Next() {
		var sig ThreatSignal
		var resolved int
		if err := rows.Scan(&sig.ID, &sig.Ts, &sig.Gate, &sig.Kind, &sig.Level,
			&sig.Summary, &sig.Reasoning, &sig.Action, &resolved); err != nil {
			return nil, err
		}
		sig.Resolved = resolved != 0
		signals = append(signals, sig)
	}
	return signals, rows.Err()
}

// ResolveSignal marks a signal as resolved by ID.
func (s *Store) ResolveSignal(id int64) error {
	_, err := s.db.Exec(`UPDATE threat_signals SET resolved = 1 WHERE id = ?`, id)
	return err
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
