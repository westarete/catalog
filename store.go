package main

import (
	"database/sql"
	"fmt"

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
