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
	db, err := openStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	row := profileRow{path: "a.md", contentHash: "h", profile: "p"}
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
		t.Errorf("matching store/file should produce no diff output, got:\n%s", out)
	}
}

func TestCmdDiffShowsUnifiedFormatWhenDrifted(t *testing.T) {
	chdirTemp(t)
	db, err := openStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeProfile(db, profileRow{path: "a.md", contentHash: "h", profile: "new profile"}); err != nil {
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
	db, err := openStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeProfile(db, profileRow{path: "a.md", contentHash: "h", profile: "p"}); err != nil {
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
