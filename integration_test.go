package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// These tests drive the real code paths end to end against a throwaway Git
// repo on disk — no API key needed. They cover the two behaviors previously
// only checked by hand: that editing a document makes the staleness gate fail
// and reverting clears it, and that the in-place entry rewrite the writers use
// rewrites one entry while leaving its neighbors untouched.

// newRepo creates a temp Git repo, chdirs into it (restored on cleanup), and
// returns a runGit helper. The program itself chdirs to the repo root via
// --root, so operating from the repo root here matches production.
func newRepo(t *testing.T) func(args ...string) string {
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

	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	return run
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

// TestStalenessGate: a document committed after the catalog is clean; editing
// it (working-tree change) makes it stale; committing the catalog again clears
// it. This is the gate behavior CI relies on.
func TestStalenessGate(t *testing.T) {
	run := newRepo(t)
	docs := []string{"a.md"}

	writeFile(t, "a.md", "alpha\n")
	run("add", "a.md")
	run("commit", "-qm", "add doc")

	writeFile(t, catalogPath, "# Catalog\n\n## ./\n\n### a.md\n\nprofile\n")
	run("add", catalogPath)
	run("commit", "-qm", "add catalog")

	// Clean: catalog is the last commit, doc unchanged since.
	ref, err := lastTouchedCommit(execGit, catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if ref == "" {
		t.Fatal("expected a catalog commit")
	}
	stale, err := staleDocs(execGit, ref, docs)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 0 {
		t.Errorf("expected clean, got stale %v", keys(stale))
	}

	// Edit the doc in the working tree → stale.
	writeFile(t, "a.md", "alpha edited\n")
	stale, err = staleDocs(execGit, ref, docs)
	if err != nil {
		t.Fatal(err)
	}
	if !stale["a.md"] {
		t.Errorf("expected a.md stale after edit, got %v", keys(stale))
	}

	// Commit the edit, then re-commit the catalog: reference advances past the
	// doc change → clean again.
	run("add", "a.md")
	run("commit", "-qm", "edit doc")
	writeFile(t, catalogPath, "# Catalog\n\n## ./\n\n### a.md\n\nprofile v2\n")
	run("add", catalogPath)
	run("commit", "-qm", "refresh catalog")

	ref, _ = lastTouchedCommit(execGit, catalogPath)
	stale, err = staleDocs(execGit, ref, docs)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 0 {
		t.Errorf("expected clean after refresh, got %v", keys(stale))
	}
}

// TestStalenessFirstBuild: with no committed catalog, every document is stale.
func TestStalenessFirstBuild(t *testing.T) {
	run := newRepo(t)
	writeFile(t, "a.md", "alpha\n")
	writeFile(t, "b.md", "beta\n")
	run("add", ".")
	run("commit", "-qm", "docs only")

	ref, err := lastTouchedCommit(execGit, catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if ref != "" {
		t.Fatalf("expected no catalog commit, got %q", ref)
	}
	stale, err := staleDocs(execGit, ref, []string{"a.md", "b.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(keys(stale), []string{"a.md", "b.md"}) {
		t.Errorf("first build should mark all stale, got %v", keys(stale))
	}
}

// TestInPlaceRewriteOnDisk: read a catalog from disk, rewrite one entry, prune
// an orphan, write it back, and re-read — confirming the surviving neighbor's
// profile is untouched. This is what the writers do around the model call.
func TestInPlaceRewriteOnDisk(t *testing.T) {
	newRepo(t) // gives us an isolated cwd
	writeFile(t, catalogPath,
		"# Catalog\n\nHeader.\n\n## x/\n\n### x/one.md\n\nold one\n\n### x/two.md\n\nkeep two\n\n### x/gone.md\n\norphan\n")

	cat, err := readCatalog()
	if err != nil {
		t.Fatal(err)
	}
	cat.entries["x/one.md"].profile = "new one"
	pruneOrphans(cat.entries, []string{"x/one.md", "x/two.md"})
	if err := writeCatalog(cat); err != nil {
		t.Fatal(err)
	}

	got, err := readCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if got.entries["x/one.md"].profile != "new one" {
		t.Errorf("one.md not rewritten: %q", got.entries["x/one.md"].profile)
	}
	if got.entries["x/two.md"].profile != "keep two" {
		t.Errorf("neighbor two.md changed: %q", got.entries["x/two.md"].profile)
	}
	if got.entries["x/gone.md"] != nil {
		t.Errorf("orphan gone.md not pruned")
	}
}
