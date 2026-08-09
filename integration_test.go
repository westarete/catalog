package main

import (
	"os"
	"path/filepath"
	"testing"
)

// These tests drive the store-backed pipeline end to end against a temp
// directory — no API key needed. They cover what update/force do around the
// model call (which is untested by design; see CLAUDE.md and FUTURE.md):
// classify docs against the store, write/delete rows, and regenerate
// .catalog.md from whatever the store now holds.

func newTempRepo(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("cleanup: restore working directory: %v", err)
		}
	})
	if err := os.MkdirAll(".catalog", 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestClassifyAgainstRealStore: a doc with a matching hash is current: a
// changed doc is modified; a doc with no row is new; a row with no doc is
// deleted. Exercises classify against a real temp-file database rather than
// hand-built rows, confirming the store round-trips hashes correctly.
func TestClassifyAgainstRealStore(t *testing.T) {
	newTempRepo(t)
	db, err := openStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	writeFile(t, "current.md", "unchanged content")
	writeFile(t, "changed.md", "new content")
	writeFile(t, "new.md", "brand new")

	for _, r := range []profileRow{
		{path: "current.md", contentHash: contentHash([]byte("unchanged content")), profile: "p1"},
		{path: "changed.md", contentHash: contentHash([]byte("old content")), profile: "p2"},
		{path: "gone.md", contentHash: "h3", profile: "p3"},
	} {
		if err := writeProfile(db, r); err != nil {
			t.Fatal(err)
		}
	}

	docs := []string{"current.md", "changed.md", "new.md"}
	hashes, err := hashDocs(docs)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := readProfiles(db)
	if err != nil {
		t.Fatal(err)
	}
	status := classify(hashes, rows)

	if got := status.new; len(got) != 1 || got[0] != "new.md" {
		t.Errorf("new = %v, want [new.md]", got)
	}
	if got := status.modified; len(got) != 1 || got[0] != "changed.md" {
		t.Errorf("modified = %v, want [changed.md]", got)
	}
	if got := status.deleted; len(got) != 1 || got[0] != "gone.md" {
		t.Errorf("deleted = %v, want [gone.md]", got)
	}
}

// TestRegenerateCatalogMDReflectsStore: write rows into the store, regenerate
// .catalog.md, and confirm the file on disk matches a fresh render of those
// rows — the file is purely a projection, never hand-maintained state.
func TestRegenerateCatalogMDReflectsStore(t *testing.T) {
	newTempRepo(t)
	db, err := openStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, r := range []profileRow{
		{path: "x/one.md", contentHash: "h1", profile: "profile one"},
		{path: "x/two.md", contentHash: "h2", profile: "profile two"},
	} {
		if err := writeProfile(db, r); err != nil {
			t.Fatal(err)
		}
	}

	if err := regenerateCatalogMD(db); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := readProfiles(db)
	if err != nil {
		t.Fatal(err)
	}
	want := render(rows)
	if string(got) != want {
		t.Errorf(".catalog.md on disk doesn't match render(store):\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestUpdateDeletesOrphanRowsWithoutModelCall: a store row for a document
// that's no longer enumerated gets dropped by the same classify+delete path
// cmdUpdate uses, without needing the model-calling half of the command.
func TestUpdateDeletesOrphanRowsWithoutModelCall(t *testing.T) {
	newTempRepo(t)
	db, err := openStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	writeFile(t, "kept.md", "content")
	if err := writeProfile(db, profileRow{path: "kept.md", contentHash: contentHash([]byte("content")), profile: "p"}); err != nil {
		t.Fatal(err)
	}
	if err := writeProfile(db, profileRow{path: "removed.md", contentHash: "h", profile: "p"}); err != nil {
		t.Fatal(err)
	}

	docs := []string{"kept.md"} // removed.md no longer enumerated
	hashes, err := hashDocs(docs)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := readProfiles(db)
	if err != nil {
		t.Fatal(err)
	}
	status := classify(hashes, rows)
	for _, path := range status.deleted {
		if err := deleteProfile(db, path); err != nil {
			t.Fatal(err)
		}
	}

	rows, err = readProfiles(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].path != "kept.md" {
		t.Errorf("rows after delete = %v, want only kept.md", rows)
	}
}
