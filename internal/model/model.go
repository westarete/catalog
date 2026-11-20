// Package model holds the types the catalog daemon's components share.
package model

import "time"

// Hash is a lowercase hex SHA-256 digest of a file's contents.
type Hash string

// Recipe is the text of a document's recipe.
type Recipe string

// FileState records what the daemon last saw of one file.
type FileState struct {
	// Rel is the path relative to the catalog's folder, with "/" separators.
	Rel       string
	Size      int64
	ModTime   time.Time
	Hash      Hash
	ChangedAt time.Time
}

// Catalog is one folder tree that holds a .catalog/ folder.
type Catalog struct {
	// Dir is the absolute folder that holds .catalog/.
	Dir       string
	Enumerate []string
	Ignore    []string
}
