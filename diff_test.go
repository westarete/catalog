package main

import (
	"os"
	"strings"
	"testing"
)

func chdirTemp(t *testing.T) {
	t.Helper()
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
}

// writeConfig writes a minimal .catalog/config.toml enumerating the given
// glob. cmdDiff now needs real enumerated documents on disk to project what
// update would change, not just the database's existing rows.
func writeConfig(t *testing.T, enumerate string) {
	t.Helper()
	if err := os.WriteFile(".catalog/config.toml", []byte(`enumerate = [`+enumerate+`]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProjectRowsDropsDeleted(t *testing.T) {
	rows := []profileRow{
		{path: "a.md", contentHash: "h1", profile: "p1"},
		{path: "gone.md", contentHash: "h2", profile: "p2"},
	}
	status := docStatus{deleted: []string{"gone.md"}}
	got := projectRows(rows, status)
	if len(got) != 1 || got[0].path != "a.md" {
		t.Errorf("projectRows = %v, want only a.md", got)
	}
}

func TestProjectRowsAddsNewWithPlaceholder(t *testing.T) {
	rows := []profileRow{{path: "a.md", contentHash: "h1", profile: "p1"}}
	status := docStatus{new: []string{"new.md"}}
	got := projectRows(rows, status)
	if len(got) != 2 {
		t.Fatalf("projectRows = %v, want 2 rows", got)
	}
	var found bool
	for _, r := range got {
		if r.path == "new.md" {
			found = true
			if r.profile != newDocPlaceholder {
				t.Errorf("new.md profile = %q, want placeholder", r.profile)
			}
		}
	}
	if !found {
		t.Error("new.md missing from projectRows output")
	}
}

func TestProjectRowsLeavesModifiedAndUnchangedAlone(t *testing.T) {
	rows := []profileRow{
		{path: "a.md", contentHash: "old-hash", profile: "old profile"},
		{path: "b.md", contentHash: "h2", profile: "p2"},
	}
	status := docStatus{modified: []string{"a.md"}}
	got := projectRows(rows, status)
	if len(got) != 2 {
		t.Fatalf("projectRows = %v, want 2 rows unchanged in count", got)
	}
	for _, r := range got {
		if r.path == "a.md" && r.profile != "old profile" {
			t.Errorf("modified doc's stale profile should be left alone, got %q", r.profile)
		}
	}
}

func TestCmdDiffRejectsArguments(t *testing.T) {
	if err := cmdDiff([]string{"unexpected"}); err == nil {
		t.Error("cmdDiff with arguments should error")
	}
}

func TestCmdDiffNoDatabaseErrorsDistinctly(t *testing.T) {
	chdirTemp(t)
	if err := os.WriteFile(catalogPath, []byte("# Catalog\n\n## (root)\n\n### a.md\n\nexisting profile\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := cmdDiff(nil)
	if err == nil {
		t.Fatal("missing database: cmdDiff should return an error")
	}
	if !strings.Contains(err.Error(), "bootstrap") {
		t.Errorf("missing-database error should point at bootstrap, got: %v", err)
	}
}

func TestCmdDiffNoOutputWhenMatching(t *testing.T) {
	chdirTemp(t)
	writeConfig(t, `"a.md"`)
	if err := os.WriteFile("a.md", []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := openStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	row := profileRow{path: "a.md", contentHash: contentHash([]byte("content")), profile: "p"}
	if err := writeProfile(db, row); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, []byte(render([]profileRow{row})), 0o644); err != nil {
		t.Fatal(err)
	}
	db.Close()

	out := captureStdout(t, func() {
		if err := cmdDiff(nil); err != nil {
			t.Fatal(err)
		}
	})
	if out != "" {
		t.Errorf("matching store/file/disk should produce no diff output, got:\n%s", out)
	}
}

func TestCmdDiffShowsUnifiedFormatWhenDrifted(t *testing.T) {
	chdirTemp(t)
	writeConfig(t, `"a.md"`)
	if err := os.WriteFile("a.md", []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := openStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeProfile(db, profileRow{path: "a.md", contentHash: contentHash([]byte("content")), profile: "new profile"}); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if err := os.WriteFile(catalogPath, []byte("stale content on disk\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := cmdDiff(nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.HasPrefix(out, "--- "+catalogPath) {
		t.Errorf("diff output missing unified header:\n%s", out)
	}
	if !strings.Contains(out, "+++ "+catalogPath) {
		t.Errorf("diff output missing unified header:\n%s", out)
	}
	if !strings.Contains(out, "-stale content on disk") {
		t.Errorf("diff output missing removed line:\n%s", out)
	}
	if !strings.Contains(out, "+new profile") {
		t.Errorf("diff output missing added line:\n%s", out)
	}
}

func TestCmdDiffMissingFileDiffsAsEmpty(t *testing.T) {
	chdirTemp(t)
	writeConfig(t, `"a.md"`)
	if err := os.WriteFile("a.md", []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := openStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeProfile(db, profileRow{path: "a.md", contentHash: contentHash([]byte("content")), profile: "p"}); err != nil {
		t.Fatal(err)
	}
	db.Close()
	// catalogPath deliberately not written.

	out := captureStdout(t, func() {
		if err := cmdDiff(nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "+### a.md") {
		t.Errorf("diff against a missing file should show every line as added:\n%s", out)
	}
}

// TestCmdDiffShowsNewFileAsAddition is the scenario a real repo hit: a file
// created on disk after bootstrap has no row yet. diff must show it as an
// addition with a placeholder, not silently produce no output just because
// the (stale) database hasn't caught up.
func TestCmdDiffShowsNewFileAsAddition(t *testing.T) {
	chdirTemp(t)
	writeConfig(t, `"a.md", "new.md"`)
	if err := os.WriteFile("a.md", []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("new.md", []byte("brand new"), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := openStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	row := profileRow{path: "a.md", contentHash: contentHash([]byte("content")), profile: "existing profile"}
	if err := writeProfile(db, row); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, []byte(render([]profileRow{row})), 0o644); err != nil {
		t.Fatal(err)
	}
	db.Close()

	out := captureStdout(t, func() {
		if err := cmdDiff(nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "+### new.md") {
		t.Errorf("new file should show up as an addition:\n%s", out)
	}
	if !strings.Contains(out, newDocPlaceholder) {
		t.Errorf("new file's addition should use the placeholder profile text:\n%s", out)
	}
}

// TestCmdDiffShowsDeletedFileAsRemoval is the other half of the scenario a
// real repo hit: a document removed from disk still has a row in the
// (stale) database. diff must show that row's section disappearing, not
// silently keep rendering it as current.
func TestCmdDiffShowsDeletedFileAsRemoval(t *testing.T) {
	chdirTemp(t)
	writeConfig(t, `"kept.md"`) // removed.md is no longer enumerated
	if err := os.WriteFile("kept.md", []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := openStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	keptRow := profileRow{path: "kept.md", contentHash: contentHash([]byte("content")), profile: "kept profile"}
	removedRow := profileRow{path: "removed.md", contentHash: "stale-hash", profile: "removed profile"}
	if err := writeProfile(db, keptRow); err != nil {
		t.Fatal(err)
	}
	if err := writeProfile(db, removedRow); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, []byte(render([]profileRow{keptRow, removedRow})), 0o644); err != nil {
		t.Fatal(err)
	}
	db.Close()

	out := captureStdout(t, func() {
		if err := cmdDiff(nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "-### removed.md") {
		t.Errorf("deleted file's section should show up as a removal:\n%s", out)
	}
	if strings.Contains(out, "+### removed.md") {
		t.Errorf("deleted file's section should not appear in the added lines:\n%s", out)
	}
	if strings.Contains(out, "-### kept.md") || strings.Contains(out, "+### kept.md") {
		t.Errorf("unaffected document kept.md should not appear in the diff at all:\n%s", out)
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns what
// was written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	w.Close()
	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	return string(buf[:n])
}
