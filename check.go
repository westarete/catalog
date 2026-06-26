package main

import (
	"fmt"
	"sort"
)

// cmdCheck is the gate: a pure Git query, no API key, no model call. It catches
// two failures — a document with no entry, and an entry gone stale since the
// catalog was last committed — and points at the fix. It cannot judge whether
// a profile is any good (a current-but-weak profile passes); judging profile
// quality needs a model, which does not belong in a deterministic gate.
func cmdCheck(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("check takes no arguments")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	docs, err := includedDocs(cfg)
	if err != nil {
		return err
	}
	cat, err := readCatalog()
	if err != nil {
		return err
	}
	ref, err := lastTouchedCommit(execGit, catalogPath)
	if err != nil {
		return err
	}
	stale, err := staleDocs(execGit, ref, docs)
	if err != nil {
		return err
	}

	r := checkResult(docs, cat.entries, stale)
	if r.ok() {
		fmt.Printf("catalog: up to date (%d docs).\n", len(docs))
		return nil
	}
	for _, p := range r.missing {
		fmt.Printf("missing: %s (no entry yet)\n", p)
	}
	for _, p := range r.changed {
		fmt.Printf("stale: %s (changed since the catalog was last committed)\n", p)
	}
	for _, p := range r.orphans {
		fmt.Printf("orphan: %s (entry exists but the document is gone or now ignored)\n", p)
	}
	return fmt.Errorf("run `catalog update` to refresh")
}

// checkResult is the gate's verdict, separated from the I/O and printing so it
// can be asserted directly: which enumerated documents have no entry, which
// have gone stale, and which entries are orphaned (no enumerated document).
type checkOutcome struct {
	missing []string
	changed []string
	orphans []string
}

func (r checkOutcome) ok() bool {
	return len(r.missing) == 0 && len(r.changed) == 0 && len(r.orphans) == 0
}

func checkResult(docs []string, entries map[string]*entry, stale map[string]bool) checkOutcome {
	var r checkOutcome
	present := map[string]bool{}
	for _, d := range docs {
		present[d] = true
		switch {
		case entries[d] == nil:
			r.missing = append(r.missing, d)
		case stale[d]:
			r.changed = append(r.changed, d)
		}
	}
	for p := range entries {
		if !present[p] {
			r.orphans = append(r.orphans, p)
		}
	}
	sort.Strings(r.orphans)
	return r
}
