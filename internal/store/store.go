// Package store is the SQLite layer: one global database per machine in WAL
// mode; every write runs in an immediate transaction. Concurrency safety is
// SQLite's job, not the CLI's.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the global database. All operations scope by the board key.
type Store struct {
	DB *sql.DB
}

// DefaultPath returns the database location: $STICKIT_DB if set, else
// stickit/stickit.db under the XDG data home.
func DefaultPath() (string, error) {
	if p := os.Getenv("STICKIT_DB"); p != "" {
		return p, nil
	}
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "stickit", "stickit.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "stickit", "stickit.db"), nil
}

// Open opens the global database, creating parent directories and schema as
// needed. A cold start may race: several processes initializing the same
// fresh database can trip over each other's journal-mode transition, which
// fails fast with SQLITE_BUSY regardless of busy_timeout. Initialization is
// therefore retried until one of the racers wins.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := "file:" + path +
		"?_pragma=busy_timeout(10000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=foreign_keys(1)" +
		"&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{DB: db}
	var initErr error
	for i := 0; i < 12; i++ {
		if initErr = s.init(); initErr == nil {
			return s, nil
		}
		if !locked(initErr) {
			break
		}
		time.Sleep(time.Duration(20+i*20) * time.Millisecond)
	}
	db.Close()
	return nil, fmt.Errorf("open %s: %w", path, initErr)
}

func (s *Store) init() error {
	if err := s.DB.Ping(); err != nil {
		return err
	}
	return s.migrate()
}

// locked reports whether err is lock contention a retry can still win.
func locked(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "locked") || strings.Contains(msg, "busy")
}

// Close releases the database handle.
func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) migrate() error {
	if _, err := s.DB.Exec(schema); err != nil {
		return err
	}
	return addColumn(s.DB, "notes", "commit", "TEXT")
}

// addColumn idempotently adds one column: CREATE TABLE IF NOT EXISTS only
// runs on fresh databases, so databases opened by earlier versions need the
// ALTER. Two openers can race past the check; whoever loses the ALTER sees
// the column the winner just added, which is success.
func addColumn(db *sql.DB, table, column, decl string) error {
	has := func() (bool, error) {
		rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
		if err != nil {
			return false, err
		}
		defer rows.Close()
		for rows.Next() {
			var cid int
			var name, typ string
			var notNull, pk int
			var dflt sql.NullString
			if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
				return false, err
			}
			if name == column {
				return true, nil
			}
		}
		return false, rows.Err()
	}
	present, err := has()
	if err != nil || present {
		return err
	}
	if _, err := db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN "` + column + `" ` + decl); err != nil {
		if present, e := has(); e == nil && present {
			return nil
		}
		return err
	}
	return nil
}

const schema = `
CREATE TABLE IF NOT EXISTS notes (
  id           TEXT PRIMARY KEY,
  repo         TEXT NOT NULL,        -- hash of the board root path
  file         TEXT NOT NULL,        -- board-relative, slash-separated
  start_line   INTEGER,              -- NULL for file-level notes
  end_line     INTEGER,
  content_hash TEXT,                 -- hash of the exact anchored lines
  norm_hash    TEXT,                 -- whitespace-collapsed, for re-anchoring
  body         TEXT NOT NULL,
  tags         TEXT,                 -- space-joined, parsed from #hashtags
  author       TEXT NOT NULL,        -- agent name or git user
  branch       TEXT,                 -- branch at write time (provenance)
  "commit"     TEXT,                 -- HEAD revision at write time (provenance)
  status       TEXT NOT NULL DEFAULT 'active',  -- active | stale | archived
  drifted_at   TEXT,                 -- set when an anchor silently moved
  created_at   TEXT NOT NULL,
  updated_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_notes_repo ON notes(repo, file, status);

CREATE TABLE IF NOT EXISTS threads (
  note_id    TEXT NOT NULL REFERENCES notes(id),
  seq        INTEGER NOT NULL,
  author     TEXT NOT NULL,
  body       TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (note_id, seq)
);

-- Full-text index over note and reply bodies; ref_id is the owning note.
CREATE VIRTUAL TABLE IF NOT EXISTS fts USING fts5(body, kind UNINDEXED, ref_id UNINDEXED);
`
