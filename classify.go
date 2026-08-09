package main

import "sort"

// docStatus is the result of comparing the enumerated documents on disk
// against the store's rows: which ones are new, modified, or deleted,
// stated from the user's point of view — what happened to their files —
// not the store's bookkeeping. See FUTURE.md's "The profile store" section.
type docStatus struct {
	new      []string
	modified []string
	deleted  []string
}

// classify compares docHashes (the enumerated documents on disk, each with
// its current content hash) against rows (the store's current state) and
// sorts every path into exactly one of new, modified, or deleted:
//
//   - new: on disk, no row for it yet.
//   - modified: on disk, but its current hash no longer matches the row's.
//   - deleted: a row exists, but the document isn't in docHashes — whether
//     because the file is gone or because config no longer enumerates it,
//     the store's answer is the same either way (see FUTURE.md).
//
// A path with a row whose hash still matches is current and appears in
// none of the three lists. Pure function, no I/O: docHashes and rows are
// both already in memory by the time this runs.
func classify(docHashes map[string]string, rows []profileRow) docStatus {
	stored := make(map[string]string, len(rows))
	for _, r := range rows {
		stored[r.path] = r.contentHash
	}

	var s docStatus
	for path, hash := range docHashes {
		storedHash, ok := stored[path]
		switch {
		case !ok:
			s.new = append(s.new, path)
		case storedHash != hash:
			s.modified = append(s.modified, path)
		}
	}
	for path := range stored {
		if _, ok := docHashes[path]; !ok {
			s.deleted = append(s.deleted, path)
		}
	}

	sort.Strings(s.new)
	sort.Strings(s.modified)
	sort.Strings(s.deleted)
	return s
}
