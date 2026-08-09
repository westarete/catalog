package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

// storePath is the database that holds one row per document: its current
// content hash and its profile. It is the sole source of truth; .catalog.md
// is always a generated artifact rendered from it. See FUTURE.md's "The
// profile store" section for the design.
const storePath = ".catalog/catalog.db"

const createProfilesTable = `
CREATE TABLE IF NOT EXISTS profiles (
	path         TEXT PRIMARY KEY,
	content_hash TEXT NOT NULL,
	profile      TEXT NOT NULL
)`

// storeExists reports whether a database file already sits at path, before
// openStore's create-if-missing behavior would otherwise hide that fact. A
// missing database means every enumerated document would report as new —
// status and diff check this first so that case gets its own message
// instead of reading like every profile was lost.
func storeExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// openStore opens (creating if necessary) the database at path and ensures
// the profiles table exists. A file that exists but isn't a valid SQLite
// database is a hard error, never silently treated as missing — see
// FUTURE.md: a broken database is a sign something else went wrong, and
// rebuilding over it would hide that.
func openStore(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	if _, err := db.Exec(createProfilesTable); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing %s: %w", path, err)
	}
	return db, nil
}

// contentHash returns the hash a document's row is compared against to
// decide staleness: identical content always hashes identically, and any
// change to the content changes the hash. Not a security boundary — just
// change detection — so a fast, collision-resistant hash is all this needs.
func contentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// profileRow is one document's current state in the store: current state
// only, no history — see FUTURE.md's "The profile store" section.
type profileRow struct {
	path        string
	contentHash string
	profile     string
}

// readProfiles fetches every row in the store.
func readProfiles(db *sql.DB) ([]profileRow, error) {
	rows, err := db.Query(`SELECT path, content_hash, profile FROM profiles`)
	if err != nil {
		return nil, fmt.Errorf("reading profiles: %w", err)
	}
	defer rows.Close()

	var out []profileRow
	for rows.Next() {
		var r profileRow
		if err := rows.Scan(&r.path, &r.contentHash, &r.profile); err != nil {
			return nil, fmt.Errorf("reading profiles: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading profiles: %w", err)
	}
	return out, nil
}

// writeProfile upserts one row by path: insert if the path is new, overwrite
// its hash and profile if it already exists.
func writeProfile(db *sql.DB, r profileRow) error {
	_, err := db.Exec(`
		INSERT INTO profiles (path, content_hash, profile) VALUES (?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET content_hash = excluded.content_hash, profile = excluded.profile
	`, r.path, r.contentHash, r.profile)
	if err != nil {
		return fmt.Errorf("writing profile for %s: %w", r.path, err)
	}
	return nil
}

// deleteProfile removes the row for path, if one exists. Used when a
// document is deleted or no longer enumerated — see FUTURE.md: both cases
// are treated the same way.
func deleteProfile(db *sql.DB, path string) error {
	if _, err := db.Exec(`DELETE FROM profiles WHERE path = ?`, path); err != nil {
		return fmt.Errorf("deleting profile for %s: %w", path, err)
	}
	return nil
}
