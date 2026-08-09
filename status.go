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
	db, err := openStore(storePath)
	if err != nil {
		return err
	}
	defer db.Close()

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

	if len(docsStatus.new) == 0 && len(docsStatus.modified) == 0 && len(docsStatus.deleted) == 0 && !drifted {
		fmt.Printf("catalog: up to date (%d docs).\n", len(docs))
		return nil
	}
	for _, p := range docsStatus.new {
		fmt.Printf("new: %s (no profile yet)\n", p)
	}
	for _, p := range docsStatus.modified {
		fmt.Printf("modified: %s (content changed since the profile was written)\n", p)
	}
	for _, p := range docsStatus.deleted {
		fmt.Printf("deleted: %s (profile exists but the document is gone or no longer enumerated)\n", p)
	}
	if drifted {
		fmt.Println("catalog.md: out of sync with the database (run `catalog diff` to see what would change)")
	}
	return fmt.Errorf("run `catalog update` to refresh")
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
