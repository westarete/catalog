package main

import (
	"fmt"
	"os"

	"github.com/aymanbagabas/go-udiff"
)

// cmdDiff shows exactly what running update would change: a unified diff
// between .catalog.md on disk and a fresh render of the database. No API
// key, no model call — the same read-only comparison status makes, shown in
// full rather than summarized. See FUTURE.md's "The profile store" section.
func cmdDiff(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("diff takes no arguments")
	}
	if !storeExists(storePath) {
		return fmt.Errorf("no database yet (%s not found) — run `catalog bootstrap` to build one", storePath)
	}
	db, err := openStore(storePath)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := readProfiles(db)
	if err != nil {
		return err
	}
	want := render(rows)

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
