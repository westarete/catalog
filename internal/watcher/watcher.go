// Package watcher notices which catalog files changed since the last scan and
// records their hashes.
package watcher

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"time"

	"westarete.com/catalog/internal/model"
)

// Scan returns a state for each of rels, the catalog's member files, in the
// same order. A file whose size and mtime match its entry in prev keeps that
// entry, so its content is not read again. Any other file is read and hashed,
// and its ChangedAt is set to now. A file that cannot be stat'ed or read is
// left out.
func Scan(catalog fs.FS, rels []string, prev map[string]model.FileState, now time.Time) []model.FileState {
	states := make([]model.FileState, 0, len(rels))
	for _, rel := range rels {
		state, ok := scanFile(catalog, rel, prev, now)
		if ok {
			states = append(states, state)
		}
	}
	return states
}

func scanFile(catalog fs.FS, rel string, prev map[string]model.FileState, now time.Time) (model.FileState, bool) {
	info, err := fs.Stat(catalog, rel)
	if err != nil {
		return model.FileState{}, false
	}
	if old, ok := prev[rel]; ok && unchanged(old, info) {
		return old, true
	}
	data, err := fs.ReadFile(catalog, rel)
	if err != nil {
		return model.FileState{}, false
	}
	sum := sha256.Sum256(data)
	return model.FileState{
		Rel:       rel,
		Size:      info.Size(),
		ModTime:   info.ModTime(),
		Hash:      model.Hash(hex.EncodeToString(sum[:])),
		ChangedAt: now,
	}, true
}

func unchanged(old model.FileState, info fs.FileInfo) bool {
	return old.Size == info.Size() && old.ModTime.Equal(info.ModTime())
}
