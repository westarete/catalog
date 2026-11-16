// Package store is the daemon's SQLite database: the files it last saw in
// each catalog, the summaries it has written, and the files it is retrying.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"westarete.com/catalog/internal/model"
)

// busyTimeoutMillis is how long a connection waits for another process's
// lock, such as a maintenance command, before it gives up.
const busyTimeoutMillis = 5000

const schema = `
CREATE TABLE IF NOT EXISTS files (
	dir        TEXT NOT NULL,
	rel        TEXT NOT NULL,
	size       INTEGER NOT NULL,
	mod_time   INTEGER NOT NULL,
	hash       TEXT NOT NULL,
	changed_at INTEGER NOT NULL,
	PRIMARY KEY (dir, rel)
) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS summaries (
	hash   TEXT NOT NULL,
	recipe TEXT NOT NULL,
	text   TEXT NOT NULL,
	PRIMARY KEY (hash, recipe)
) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS failures (
	dir        TEXT NOT NULL,
	rel        TEXT NOT NULL,
	count      INTEGER NOT NULL,
	last_error TEXT NOT NULL,
	retry_at   INTEGER NOT NULL,
	PRIMARY KEY (dir, rel)
) WITHOUT ROWID;
`

// Failure records that summarizing a file failed and when to try again.
type Failure struct {
	Count     int
	LastError string
	RetryAt   time.Time
}

// Store is an open database.
type Store struct {
	db *sql.DB
}

// Open opens the database at path, creating it and its tables if missing.
// It uses WAL journaling and a busy timeout so a second process can open the
// same file while the daemon runs. Only the owner can read the database.
func Open(path string) (*Store, error) {
	if err := makePrivate(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing %s: %w", path, err)
	}
	return &Store{db: db}, nil
}

// makePrivate creates the database file at path if it's missing and makes it
// readable and writable only by its owner, since it holds summaries of
// private documents. Files an earlier version created get the same mode,
// including a write-ahead log and shared-memory file that a killed daemon
// left behind, which SQLite keeps using.
func makePrivate(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o600)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(p, 0o600); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("making %s private: %w", p, err)
		}
	}
	return nil
}

// dsn builds a file: URI for the driver. The driver runs each _pragma on
// every new connection. The URL escaping keeps characters such as "?" and
// "#" in path from being read as part of the query.
func dsn(path string) string {
	u := url.URL{
		Scheme: "file",
		Path:   path,
		RawQuery: fmt.Sprintf("_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)",
			busyTimeoutMillis),
	}
	return u.String()
}

// Close closes the database.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("closing store: %w", err)
	}
	return nil
}

// Files returns the files last recorded for dir, keyed by relative path.
func (s *Store) Files(dir string) (map[string]model.FileState, error) {
	rows, err := s.db.Query(
		`SELECT rel, size, mod_time, hash, changed_at FROM files WHERE dir = ?`, dir)
	if err != nil {
		return nil, fmt.Errorf("reading files of %s: %w", dir, err)
	}
	defer func() { _ = rows.Close() }()
	files := make(map[string]model.FileState)
	for rows.Next() {
		var f model.FileState
		var modTime, changedAt int64
		if err := rows.Scan(&f.Rel, &f.Size, &modTime, &f.Hash, &changedAt); err != nil {
			return nil, fmt.Errorf("reading files of %s: %w", dir, err)
		}
		f.ModTime, f.ChangedAt = fromNanos(modTime), fromNanos(changedAt)
		files[f.Rel] = f
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading files of %s: %w", dir, err)
	}
	return files, nil
}

// SetFiles replaces the recorded files of dir with files, all or nothing.
func (s *Store) SetFiles(dir string, files []model.FileState) (err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("saving files of %s: %w", dir, err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.Exec(`DELETE FROM files WHERE dir = ?`, dir); err != nil {
		return fmt.Errorf("clearing files of %s: %w", dir, err)
	}
	for _, f := range files {
		_, err = tx.Exec(
			`INSERT INTO files (dir, rel, size, mod_time, hash, changed_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			dir, f.Rel, f.Size, toNanos(f.ModTime), string(f.Hash), toNanos(f.ChangedAt))
		if err != nil {
			return fmt.Errorf("saving %s in %s: %w", f.Rel, dir, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("saving files of %s: %w", dir, err)
	}
	return nil
}

// Summary returns the summary written for content with hash h under recipe r.
func (s *Store) Summary(h model.Hash, r model.Recipe) (string, bool, error) {
	var text string
	err := s.db.QueryRow(
		`SELECT text FROM summaries WHERE hash = ? AND recipe = ?`,
		string(h), string(r)).Scan(&text)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("reading summary: %w", err)
	}
	return text, true, nil
}

// AnySummary returns a summary of content with hash h under any recipe, with
// the recipe it was written under. When several exist, it picks the one with
// the lowest recipe text, so the answer is stable.
func (s *Store) AnySummary(h model.Hash) (text string, recipe model.Recipe, ok bool, err error) {
	var r string
	err = s.db.QueryRow(
		`SELECT text, recipe FROM summaries WHERE hash = ? ORDER BY recipe LIMIT 1`,
		string(h)).Scan(&text, &r)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("reading summary: %w", err)
	}
	return text, model.Recipe(r), true, nil
}

// PutSummary stores text as the summary for hash h under recipe r, replacing
// any earlier one.
func (s *Store) PutSummary(h model.Hash, r model.Recipe, text string) error {
	_, err := s.db.Exec(
		`INSERT INTO summaries (hash, recipe, text) VALUES (?, ?, ?)
		 ON CONFLICT (hash, recipe) DO UPDATE SET text = excluded.text`,
		string(h), string(r), text)
	if err != nil {
		return fmt.Errorf("saving summary: %w", err)
	}
	return nil
}

// Failure returns the failure recorded for a file, if any.
func (s *Store) Failure(dir, rel string) (Failure, bool, error) {
	var f Failure
	var retryAt int64
	err := s.db.QueryRow(
		`SELECT count, last_error, retry_at FROM failures WHERE dir = ? AND rel = ?`,
		dir, rel).Scan(&f.Count, &f.LastError, &retryAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Failure{}, false, nil
	}
	if err != nil {
		return Failure{}, false, fmt.Errorf("reading failure of %s in %s: %w", rel, dir, err)
	}
	f.RetryAt = fromNanos(retryAt)
	return f, true, nil
}

// RecordFailure stores f as the failure for a file, replacing any earlier one.
func (s *Store) RecordFailure(dir, rel string, f Failure) error {
	_, err := s.db.Exec(
		`INSERT INTO failures (dir, rel, count, last_error, retry_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (dir, rel) DO UPDATE SET
		   count = excluded.count,
		   last_error = excluded.last_error,
		   retry_at = excluded.retry_at`,
		dir, rel, f.Count, f.LastError, toNanos(f.RetryAt))
	if err != nil {
		return fmt.Errorf("saving failure of %s in %s: %w", rel, dir, err)
	}
	return nil
}

// ClearFailure forgets the failure recorded for a file.
func (s *Store) ClearFailure(dir, rel string) error {
	if _, err := s.db.Exec(`DELETE FROM failures WHERE dir = ? AND rel = ?`, dir, rel); err != nil {
		return fmt.Errorf("clearing failure of %s in %s: %w", rel, dir, err)
	}
	return nil
}

// toNanos stores a time as Unix nanoseconds so it reads back exactly. The
// zero time, which Unix nanoseconds cannot represent, is stored as 0.
func toNanos(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

// fromNanos reverses toNanos.
func fromNanos(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}
