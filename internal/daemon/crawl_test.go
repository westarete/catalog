package daemon

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"westarete.com/catalog/internal/journal"
	"westarete.com/catalog/internal/settings"
	"westarete.com/catalog/internal/trees"
)

func TestEmptyWatchEntryIsSkipped(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	d, _ := newDaemon(t, "", root)
	var opened []string
	d.FS = func(dir string) fs.FS {
		opened = append(opened, dir)
		return os.DirFS(dir)
	}

	once(t, d, start)

	for _, dir := range opened {
		if !strings.HasPrefix(dir, root) {
			t.Errorf("daemon opened %q, outside the watched folder", dir)
		}
	}
	entry(t, catalogText(t, root), "alpha.md")
}

// Under launchd the working folder is /, so a relative watch folder would
// mean a folder no one intended. The daemon reports it and skips it.
func TestRelativeWatchFolderIsConfigError(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	for _, dir := range []string{root, other} {
		makeCatalog(t, dir)
		writeFile(t, dir, "alpha.md", body("alpha"))
	}
	t.Chdir(filepath.Dir(root))
	rel := filepath.Base(root)
	d, _ := newDaemon(t, rel, other)

	crawl(t, d, start)
	crawl(t, d, start.Add(time.Minute))

	assertNoCatalog(t, root)
	entry(t, catalogText(t, other), "alpha.md")
	if msg := d.ConfigErrors()[rel]; !strings.Contains(msg, "absolute") {
		t.Errorf("ConfigErrors = %v, want one for %s", d.ConfigErrors(), rel)
	}
	events := eventsOfKind(t, d, journal.KindConfigError)
	if len(events) != 1 || events[0].Dir != rel {
		t.Errorf("config_error events = %+v, want one for %s", events, rel)
	}
}

func TestUnreadableDirectoryDoesNotStopCrawl(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	two := filepath.Join(root, "two")
	makeCatalog(t, one)
	makeCatalog(t, two)
	writeFile(t, one, "a.md", body("a"))
	writeFile(t, one, "locked/x.md", body("x"))
	writeFile(t, two, "b.md", body("b"))
	d, _ := newDaemon(t, root)
	crawl(t, d, start)

	writeFile(t, one, "new.md", body("new"))
	writeFile(t, two, "new.md", body("new"))
	chmod(t, filepath.Join(one, "locked"), 0o000)
	crawl(t, d, start.Add(time.Minute))
	crawl(t, d, start.Add(2*time.Minute))

	oneText := catalogText(t, one)
	entry(t, oneText, "new.md")
	if strings.Contains(oneText, "locked/x.md") {
		t.Errorf("catalog one lists a file it can no longer read:\n%s", oneText)
	}
	entry(t, catalogText(t, two), "new.md")
	failures := eventsOfKind(t, d, journal.KindFailure)
	if len(failures) != 1 || failures[0].Dir != root || failures[0].Rel != "one/locked" ||
		failures[0].Error == "" || !failures[0].Time.Equal(start.Add(time.Minute)) {
		t.Errorf("failure events = %+v, want one for one/locked", failures)
	}
}

func TestUnreadableWatchedFolder(t *testing.T) {
	bad := t.TempDir()
	good := t.TempDir()
	makeCatalog(t, bad)
	writeFile(t, bad, "a.md", body("a"))
	chmod(t, bad, 0o000)
	makeCatalog(t, good)
	writeFile(t, good, "b.md", body("b"))
	d, _ := newDaemon(t, bad, good)

	crawl(t, d, start)

	entry(t, catalogText(t, good), "b.md")
	failures := eventsOfKind(t, d, journal.KindFailure)
	if len(failures) != 1 || failures[0].Dir != bad || failures[0].Rel != "" || failures[0].Error == "" {
		t.Errorf("failure events = %+v, want one for %s", failures, bad)
	}
}

// unlistableFS opens files but can't list any directory.
type unlistableFS struct{ fs.FS }

func (unlistableFS) ReadDir(string) ([]fs.DirEntry, error) { return nil, fs.ErrPermission }

func TestFailedMemberWalkKeepsLastCatalog(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	two := filepath.Join(root, "two")
	makeCatalog(t, one)
	makeCatalog(t, two)
	writeFile(t, one, "a.md", body("a"))
	writeFile(t, two, "b.md", body("b"))
	d, p := newDaemon(t, root)
	crawl(t, d, start)
	before := catalogText(t, one)

	d.FS = func(dir string) fs.FS {
		if dir == one {
			return unlistableFS{os.DirFS(dir)}
		}
		return os.DirFS(dir)
	}
	writeFile(t, one, "new.md", body("new"))
	writeFile(t, two, "new.md", body("new"))
	later := start.Add(time.Minute)
	crawl(t, d, later)

	if got := catalogText(t, one); got != before {
		t.Errorf("catalog one changed though its walk failed:\n%s", got)
	}
	entry(t, catalogText(t, two), "new.md")
	failures := eventsOfKind(t, d, journal.KindFailure)
	if len(failures) != 1 || failures[0].Dir != one || failures[0].Rel != "" || failures[0].Error == "" ||
		!failures[0].Time.Equal(later) {
		t.Errorf("failure events = %+v", failures)
	}
	for work(t, d, later, true) {
	}
	for _, req := range p.requests {
		if documentName(req) == "a" {
			t.Error("Work summarized a file in a catalog this crawl skipped")
		}
	}
}

func TestMissingWatchedFolder(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone")
	good := t.TempDir()
	makeCatalog(t, good)
	writeFile(t, good, "b.md", body("b"))
	d, _ := newDaemon(t, missing, good)

	once(t, d, start)

	entry(t, catalogText(t, good), "b.md")
	if failures := eventsOfKind(t, d, journal.KindFailure); len(failures) != 1 || failures[0].Dir != missing {
		t.Errorf("failure events = %+v", failures)
	}
}

func TestCrawlForgetsVanishedCatalog(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	crawl(t, d, start)
	if err := os.RemoveAll(filepath.Join(root, ".catalog")); err != nil {
		t.Fatal(err)
	}
	crawl(t, d, start.Add(time.Minute))

	if work(t, d, start.Add(time.Minute), true) || p.calls() != 0 {
		t.Error("Work summarized a file whose catalog is gone")
	}
}

func TestBuiltinsNoLongerExcludeREADME(t *testing.T) {
	include, exclude := settings.Lists(settings.Global{}, settings.Catalog{})
	got, err := trees.Members(fstest.MapFS{"README.md": {}, "CLAUDE.md": {}, "sub/AGENTS.md": {}}, include, exclude, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"README.md"}; !slices.Equal(got, want) {
		t.Errorf("Members = %v, want %v", got, want)
	}
}

// A catalog can add what the crawl reads, but it can't undo what the person
// excluded for every catalog.
func TestCatalogIncludeCannotUndoGlobalExclude(t *testing.T) {
	g, err := settings.ParseGlobal([]byte(`exclude = ["secrets.txt"]`))
	if err != nil {
		t.Fatal(err)
	}
	c, err := settings.ParseCatalog([]byte(`include = ["secrets.txt"]`))
	if err != nil {
		t.Fatal(err)
	}
	include, exclude := settings.Lists(g, c)
	got, err := trees.Members(fstest.MapFS{"secrets.txt": {}, "notes.txt": {}}, include, exclude, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"notes.txt"}; !slices.Equal(got, want) {
		t.Errorf("Members = %v, want %v", got, want)
	}
}

func TestCrawlReturnsStoreError(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, _ := newDaemon(t, root)
	_ = d.Store.Close()

	if err := d.Crawl(start); err == nil {
		t.Error("Crawl with a closed store returned nil")
	}
}

func TestWorkReturnsStoreError(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, _ := newDaemon(t, root)
	crawl(t, d, start)
	_ = d.Store.Close()

	if _, err := d.Work(context.Background(), start, true); err == nil {
		t.Error("Work with a closed store returned nil")
	}
}

func TestCrawlReturnsJournalErrorButKeepsGoing(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	two := filepath.Join(root, "two")
	makeCatalog(t, one)
	makeCatalog(t, two)
	writeFile(t, one, ".catalog/config.toml", "include = [[[ garbage\n")
	writeFile(t, two, "b.md", body("b"))
	d, _ := newDaemon(t, root)
	d.JournalPath = filepath.Join(t.TempDir(), "missing", "journal.jsonl")

	if err := d.Crawl(start); err == nil {
		t.Error("Crawl returned nil though the journal could not be written")
	}
	entry(t, catalogText(t, two), "b.md")
}

func TestOnceStopsOnCrawlError(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	_ = d.Store.Close()

	if err := d.Once(context.Background(), fixedClock(start)); err == nil {
		t.Error("Once returned nil")
	}
	if p.calls() != 0 {
		t.Errorf("provider calls = %d", p.calls())
	}
}

func TestNestedPaths(t *testing.T) {
	found := []string{".", "a", "a/b", "a/b/c", "ab"}
	tests := []struct {
		catalog string
		want    []string
	}{
		{".", []string{"a", "a/b", "a/b/c", "ab"}},
		{"a", []string{"b", "b/c"}},
		{"a/b", []string{"c"}},
		{"ab", nil},
	}
	for _, tt := range tests {
		if got := nestedUnder(tt.catalog, found); !slices.Equal(got, tt.want) {
			t.Errorf("nestedUnder(%q) = %v, want %v", tt.catalog, got, tt.want)
		}
	}
}

// A problem whose journal line couldn't be written is journaled on the next
// crawl, and status still knows about it meanwhile.
func TestFailedJournalWriteIsRetried(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, ".catalog/config.toml", "include = [[[ garbage\n")
	d, _ := newDaemon(t, root)
	journalDir := filepath.Join(t.TempDir(), "missing")
	d.JournalPath = filepath.Join(journalDir, "journal.jsonl")

	if err := d.Crawl(start); err == nil {
		t.Fatal("Crawl returned nil though the journal could not be written")
	}
	if _, ok := d.ConfigErrors()[root]; !ok {
		t.Errorf("ConfigErrors = %v, want the catalog's error", d.ConfigErrors())
	}

	if err := os.Mkdir(journalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	crawl(t, d, start.Add(time.Minute))
	crawl(t, d, start.Add(2*time.Minute))

	if n := len(eventsOfKind(t, d, journal.KindConfigError)); n != 1 {
		t.Errorf("config errors journaled = %d, want 1", n)
	}
}
