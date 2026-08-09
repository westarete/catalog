package main

import (
	"os"
	"strings"
	"testing"
)

func TestCatalogMDDriftedMissingFile(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	drifted, err := catalogMDDrifted([]profileRow{{path: "a.md", contentHash: "h", profile: "p"}})
	if err != nil {
		t.Fatal(err)
	}
	if !drifted {
		t.Error("a missing .catalog.md should count as drifted")
	}
}

func TestCatalogMDDriftedMatches(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	rows := []profileRow{{path: "a.md", contentHash: "h", profile: "p"}}
	if err := os.WriteFile(catalogPath, []byte(render(rows)), 0o644); err != nil {
		t.Fatal(err)
	}

	drifted, err := catalogMDDrifted(rows)
	if err != nil {
		t.Fatal(err)
	}
	if drifted {
		t.Error("a file matching render(rows) should not count as drifted")
	}
}

func TestCatalogMDDriftedHandEdit(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	rows := []profileRow{{path: "a.md", contentHash: "h", profile: "p"}}
	if err := os.WriteFile(catalogPath, []byte(render(rows)), 0o644); err != nil {
		t.Fatal(err)
	}
	// Simulate a hand-edit or a botched write: append something render()
	// would never produce.
	f, err := os.OpenFile(catalogPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\nhand-added text\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	drifted, err := catalogMDDrifted(rows)
	if err != nil {
		t.Fatal(err)
	}
	if !drifted {
		t.Error("a file that no longer matches render(rows) should count as drifted")
	}
}

func TestCmdStatusRejectsArguments(t *testing.T) {
	if err := cmdStatus([]string{"unexpected"}); err == nil {
		t.Error("cmdStatus with arguments should error")
	}
}

// TestCmdStatusNoDatabaseReportsDistinctly guards against a real gap: a repo
// with a real, populated .catalog.md but no database yet (never migrated)
// must not be reported the same way as one where every document has
// genuinely never been profiled. Both would otherwise look identical —
// every document classified "new" — which reads as data loss rather than
// "run bootstrap."
func TestCmdStatusNoDatabaseReportsDistinctly(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	if err := os.MkdirAll(".catalog", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".catalog/config.toml", []byte(`enumerate = ["a.md"]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("a.md", []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A real, populated .catalog.md, as if hand-authored or from before the
	// store existed — deliberately not touching .catalog/catalog.db.
	if err := os.WriteFile(catalogPath, []byte("# Catalog\n\n## (root)\n\n### a.md\n\nexisting profile\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(storePath); err == nil {
		t.Fatal("test setup: storePath should not exist yet")
	}

	err = cmdStatus(nil)
	if err == nil {
		t.Fatal("missing database: cmdStatus should return an error")
	}
	if strings.Contains(err.Error(), "refresh") {
		t.Errorf("missing-database error should be distinct from the normal out-of-sync error, got: %v", err)
	}
}

func TestCmdStatusCleanRepoReportsUpToDate(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	if err := os.MkdirAll(".catalog", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".catalog/config.toml", []byte(`enumerate = ["a.md"]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("a.md", []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := openStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	row := profileRow{path: "a.md", contentHash: contentHash([]byte("content")), profile: "profile text"}
	if err := writeProfile(db, row); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, []byte(render([]profileRow{row})), 0o644); err != nil {
		t.Fatal(err)
	}
	db.Close()

	if err := cmdStatus(nil); err != nil {
		t.Errorf("clean repo: cmdStatus returned an error: %v", err)
	}
}

func TestCmdStatusReportsModifiedAndErrors(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	if err := os.MkdirAll(".catalog", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".catalog/config.toml", []byte(`enumerate = ["a.md"]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("a.md", []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := openStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeProfile(db, profileRow{path: "a.md", contentHash: "stale-hash", profile: "old profile"}); err != nil {
		t.Fatal(err)
	}
	db.Close()

	if err := cmdStatus(nil); err == nil {
		t.Error("modified doc: cmdStatus should return a non-nil error")
	}
}
