package main

import (
	"fmt"
	"os"

	"github.com/aymanbagabas/go-udiff"
)

// newDocPlaceholder is the profile text shown for a document that's new on
// disk but has no row yet — diff makes no model call, so there's no real
// profile to show. The placeholder still makes the addition visible, rather
// than the diff silently omitting a document nobody has profiled yet.
const newDocPlaceholder = "(profile not yet generated — run `catalog update`)"

// cmdDiff shows what running update would change on disk: a unified diff
// between .catalog.md and a projection of the database that reflects the
// current filesystem — deleted documents' rows dropped, new documents added
// with a placeholder profile. A modified document keeps its existing
// (stale) profile text, since diff makes no model call and can't produce
// the profile update would actually infer; the point is showing the real
// difference a person would see, not a preview of the model's next answer.
// No API key. See FUTURE.md's "The profile store" section.
func cmdDiff(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("diff takes no arguments")
	}
	if !storeExists(storePath) {
		return fmt.Errorf("no database yet (%s not found) — run `catalog bootstrap` to build one", storePath)
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	docs, err := includedDocs(cfg)
	if err != nil {
		return err
	}
	db, err := openStore(storePath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	rows, err := readProfiles(db)
	if err != nil {
		return err
	}
	docHashes, err := hashDocs(docs)
	if err != nil {
		return err
	}
	status := classify(docHashes, rows)
	want := render(projectRows(rows, status))

	got, err := os.ReadFile(catalogPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		got = nil // a missing file diffs as empty, same as any new file
	}

	out := udiff.Unified(catalogPath, catalogPath, string(got), want)
	if out != "" {
		fmt.Print(out)
	}
	return nil
}

// projectRows applies status to rows without touching the database: drops
// deleted documents, adds a placeholder row for each new document, and
// leaves modified/unchanged rows as they are. This is a pure projection of
// what update would leave in place before the model call — the model call
// itself is what turns a placeholder or a stale profile into the real thing.
func projectRows(rows []profileRow, status docStatus) []profileRow {
	deleted := make(map[string]bool, len(status.deleted))
	for _, path := range status.deleted {
		deleted[path] = true
	}

	out := make([]profileRow, 0, len(rows)+len(status.new))
	for _, r := range rows {
		if !deleted[r.path] {
			out = append(out, r)
		}
	}
	for _, path := range status.new {
		out = append(out, profileRow{path: path, profile: newDocPlaceholder})
	}
	return out
}
