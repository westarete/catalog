package main

import (
	"fmt"
	"os"
)

// cmdStatus is the gate: no API key, no model call. It reports what
// happened to the user's files relative to the database — new, modified, or
// deleted, in the same vocabulary and orientation as `git status` — plus
// whether .catalog.md on disk matches a fresh render of the database. It
// cannot judge whether a profile is any good (a current-but-weak profile
// passes); judging profile quality needs a model, which does not belong in
// a deterministic gate. See FUTURE.md's "The profile store" section.
func cmdStatus(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("status takes no arguments")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	docs, err := includedDocs(cfg)
	if err != nil {
		return err
	}
	if !storeExists(storePath) {
		return fmt.Errorf("no database yet (%s not found) — run `catalog bootstrap` to build one", storePath)
	}
	db, err := openStore(storePath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	docHashes, err := hashDocs(docs)
	if err != nil {
		return err
	}
	rows, err := readProfiles(db)
	if err != nil {
		return err
	}
	docsStatus := classify(docHashes, rows)

	drifted, err := catalogMDDrifted(rows)
	if err != nil {
		return err
	}

	if reportStatus(newReporter(os.Stdout), docsStatus, drifted, len(docs)) {
		return errReported
	}
	return nil
}

// reportStatus prints each out-of-sync document with the same words a run
// uses for it, then the closing line. It returns whether
// anything needs attention.
func reportStatus(rep *reporter, s docStatus, drifted bool, total int) bool {
	for _, p := range s.new {
		rep.pending(actionAdd, p)
	}
	for _, p := range s.modified {
		rep.pending(actionModify, p)
	}
	for _, p := range s.deleted {
		rep.pending(actionDelete, p)
	}
	return rep.attention(total, drifted)
}

// catalogMDDrifted reports whether .catalog.md on disk differs from what
// rendering rows right now would produce. A missing file counts as drifted —
// there's nothing on disk to match a non-empty store.
func catalogMDDrifted(rows []profileRow) (bool, error) {
	want := render(rows)
	got, err := os.ReadFile(catalogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	return string(got) != want, nil
}
