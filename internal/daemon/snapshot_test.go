package daemon

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"westarete.com/catalog/internal/control"
	"westarete.com/catalog/internal/journal"
	"westarete.com/catalog/internal/models"
	"westarete.com/catalog/internal/settings"
)

func failingPaths(s control.Snapshot) []string {
	var out []string
	for _, f := range s.Failing {
		out = append(out, filepath.Join(f.Dir, f.Rel))
	}
	return out
}

func TestDaemonSnapshot(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	writeFile(t, root, "b.md", body("b"))
	d, p := newDaemon(t, root)
	p.fail["a"] = apiError(400)
	step(t, d, start)

	at := start.Add(quiet)
	step(t, d, at)

	s := d.Snapshot()
	if len(s.Failing) != 1 {
		t.Fatalf("failing = %+v, want a.md", s.Failing)
	}
	f := s.Failing[0]
	want, _, err := d.Store.Failure(root, "a.md")
	if err != nil {
		t.Fatal(err)
	}
	if f.Dir != root || f.Rel != "a.md" || f.Error != want.LastError || f.Error == "" || !f.RetryAt.Equal(at.Add(time.Minute)) {
		t.Errorf("failing file = %+v, want a.md, %q, retry at %v", f, want.LastError, at.Add(time.Minute))
	}
}

func TestSnapshotCountsEachCatalog(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, filepath.Join(root, "one"))
	makeCatalog(t, filepath.Join(root, "two"))
	writeFile(t, root, "one/a.md", body("a"))
	writeFile(t, root, "one/b.md", body("b"))
	writeFile(t, root, "one/c.md", body("c"))
	writeFile(t, root, "two/d.md", body("d"))
	d, p := newDaemon(t, root)
	p.fail["b"] = apiError(400)
	step(t, d, start)
	writeFile(t, root, "one/c.md", body("c2"))

	at := start.Add(quiet)
	step(t, d, at)

	s := d.Snapshot()
	want := []control.CatalogStatus{
		// a's summary is partial, so it's summarized and has an update to
		// come once c has a summary. b failed and c changed, so both wait.
		{Dir: filepath.Join(root, "one"), Files: 3, Summarized: 1, Waiting: 2, ToUpdate: 1},
		{Dir: filepath.Join(root, "two"), Files: 1, Summarized: 1, Waiting: 0},
	}
	if fmt.Sprint(s.Catalogs) != fmt.Sprint(want) {
		t.Errorf("catalogs = %+v\nwant %+v", s.Catalogs, want)
	}
	if !s.Started.Equal(at) || !s.LastCrawl.Equal(at) {
		t.Errorf("started %v, last crawl %v; want both %v", s.Started, s.LastCrawl, at)
	}
}

// A summary under an older recipe still shows in the catalog, so the file
// counts as summarized, with an update to come under the current recipe.
func TestSnapshotCountsOlderRecipeAsToUpdate(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	once(t, d, start)
	d.Global.Model = "claude-other"
	p.fail["a"] = apiError(400)

	step(t, d, start.Add(time.Minute))

	c := d.Snapshot().Catalogs
	want := []control.CatalogStatus{{Dir: root, Files: 1, Summarized: 1, ToUpdate: 1}}
	if !slices.Equal(c, want) {
		t.Errorf("catalogs = %+v, want %+v", c, want)
	}
}

// Problems the crawl finds show in status while they last.
func TestSnapshotShowsCrawlProblems(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	makeCatalog(t, proj)
	writeFile(t, proj, "a.md", body("a"))
	writeFile(t, proj, "locked.md", body("locked"))
	writeFile(t, root, "private/x.md", body("x"))
	d, _ := newDaemon(t, root)
	chmod(t, filepath.Join(proj, "locked.md"), 0o000)
	chmod(t, filepath.Join(root, "private"), 0o000)
	chmod(t, filepath.Join(proj, ".catalog"), 0o555)

	step(t, d, start)

	s := d.Snapshot()
	want := []string{filepath.Join(root, "private"), proj, filepath.Join(proj, "locked.md")}
	if fmt.Sprint(failingPaths(s)) != fmt.Sprint(want) {
		t.Fatalf("failing = %+v, want %v", s.Failing, want)
	}
	for _, f := range s.Failing {
		if f.Error == "" || !f.RetryAt.IsZero() {
			t.Errorf("failing %+v, want an error and no retry time", f)
		}
	}
	if s.Failing[2].Error != unreadable {
		t.Errorf("locked.md error = %q, want %q", s.Failing[2].Error, unreadable)
	}

	for _, p := range []string{filepath.Join(proj, "locked.md"), filepath.Join(root, "private"), filepath.Join(proj, ".catalog")} {
		if err := os.Chmod(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	step(t, d, start.Add(time.Minute))

	if s := d.Snapshot(); len(s.Failing) != 0 {
		t.Errorf("failing after the fix = %+v", s.Failing)
	}
}

// The Store keeps a file's failure after the file is gone, but status lists
// only the files a catalog still has.
func TestSnapshotForgetsDeletedFile(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	p.fail["a"] = apiError(400)
	once(t, d, start)
	if err := os.Remove(filepath.Join(root, "a.md")); err != nil {
		t.Fatal(err)
	}

	step(t, d, start.Add(time.Minute))

	if s := d.Snapshot(); len(s.Failing) != 0 {
		t.Errorf("failing = %+v, want none", s.Failing)
	}
}

func TestSnapshotShowsConfigErrors(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, ".catalog/config.toml", "enumerate = [[[")
	d, p := newDaemon(t, root)
	useGlobal(t, d, p, root, "")
	step(t, d, start)
	if err := os.WriteFile(d.GlobalPath, []byte("garbage = [[["), 0o600); err != nil {
		t.Fatal(err)
	}

	step(t, d, start.Add(time.Minute))

	errs := d.Snapshot().ConfigErrors
	want := []string{d.GlobalPath + ": parse global.toml", root + ": parse catalog config"}
	slices.Sort(want)
	if len(errs) != 2 || !strings.HasPrefix(errs[0], want[0]) || !strings.HasPrefix(errs[1], want[1]) {
		t.Errorf("config errors = %q, want lines starting %q in order", errs, want)
	}
}

func TestSnapshotShowsPause(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	p.fail["a"] = apiError(529)
	step(t, d, start)
	at := start.Add(quiet)

	step(t, d, at)

	s := d.Snapshot()
	if !strings.Contains(s.PauseReason, "529") || !s.PausedUntil.Equal(at.Add(time.Minute)) {
		t.Errorf("paused until %v for %q, want %v and the 529 error", s.PausedUntil, s.PauseReason, at.Add(time.Minute))
	}

	delete(p.fail, "a")
	step(t, d, at.Add(time.Minute))

	if s := d.Snapshot(); s.PauseReason != "" || !s.PausedUntil.IsZero() {
		t.Errorf("after resuming: paused until %v for %q, want no pause", s.PausedUntil, s.PauseReason)
	}
}

// Once a pause has passed, status stops showing it, even if no file is ready
// to summarize. The next success still journals the resume.
func TestSnapshotClearsPauseWhenItEnds(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	p.fail["a"] = apiError(529)
	step(t, d, start)
	at := start.Add(quiet)
	step(t, d, at)
	delete(p.fail, "a")
	if err := os.Remove(filepath.Join(root, "a.md")); err != nil {
		t.Fatal(err)
	}

	step(t, d, at.Add(time.Minute))

	if s := d.Snapshot(); s.PauseReason != "" || !s.PausedUntil.IsZero() {
		t.Errorf("after the pause: paused until %v for %q, want no pause", s.PausedUntil, s.PauseReason)
	}
	writeFile(t, root, "b.md", body("b"))
	step(t, d, at.Add(2*time.Minute))
	step(t, d, at.Add(2*time.Minute+quiet))
	if got := kinds(t, d, journal.KindPause, journal.KindResume); fmt.Sprint(got) != "[pause resume]" {
		t.Errorf("journal = %v, want [pause resume]", got)
	}
}

// A new API key ends a pause, so status stops showing it before the daemon
// tries the new key.
func TestSnapshotClearsPauseOnNewKey(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	useGlobal(t, d, p, root, "")
	p.fail["a"] = apiError(401)
	step(t, d, start)
	step(t, d, start.Add(quiet))
	text := strings.Replace(mustRead(t, d.GlobalPath), "key-1", "key-2", 1)
	if err := os.WriteFile(d.GlobalPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	var seen []control.Snapshot
	d.NewProvider = func(string, string) models.Provider {
		return providerFunc(func(ctx context.Context, req models.Request) (models.Response, error) {
			seen = append(seen, d.Snapshot())
			return p.Complete(ctx, req)
		})
	}

	step(t, d, start.Add(quiet+10*time.Second))

	if len(seen) != 1 || seen[0].PauseReason != "" || !seen[0].PausedUntil.IsZero() {
		t.Errorf("snapshots while trying the new key = %+v, want one with no pause", seen)
	}
}

// With no API key, the daemon still lists files but summarizes none, and
// status says why.
func TestRunWithoutAPIKey(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	useGlobal(t, d, p, root, "")
	text := strings.Replace(mustRead(t, d.GlobalPath), `api_key = "key-1"`, "", 1)
	if err := os.WriteFile(d.GlobalPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	d.Global, d.Provider = noGlobal(), nil

	step(t, d, start)
	step(t, d, start.Add(quiet))

	if p.calls() != 0 {
		t.Errorf("calls without a key = %d, want 0", p.calls())
	}
	entry(t, catalogText(t, root), "a.md")
	s := d.Snapshot()
	if len(s.ConfigErrors) != 1 || !strings.Contains(s.ConfigErrors[0], "no api_key") {
		t.Errorf("config errors = %q, want no api_key", s.ConfigErrors)
	}
	if s.PauseReason == "" || !s.PausedUntil.IsZero() {
		t.Errorf("paused until %v for %q, want stopped with a reason", s.PausedUntil, s.PauseReason)
	}

	writeGlobal(t, d, root, "")
	step(t, d, start.Add(quiet+time.Minute))

	if p.calls() != 1 {
		t.Errorf("calls after adding the key = %d, want 1", p.calls())
	}
	if s := d.Snapshot(); len(s.ConfigErrors) != 0 || s.PauseReason != "" {
		t.Errorf("after adding the key: config errors %q, pause %q", s.ConfigErrors, s.PauseReason)
	}
}

// Removing the key from global.toml stops summarizing, though the daemon had
// a key before.
func TestRunStopsWhenKeyRemoved(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	useGlobal(t, d, p, root, "")
	step(t, d, start)
	text := strings.Replace(mustRead(t, d.GlobalPath), `api_key = "key-1"`, "", 1)
	if err := os.WriteFile(d.GlobalPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}

	step(t, d, start.Add(quiet))

	if p.calls() != 0 || d.Provider != nil {
		t.Errorf("calls = %d, provider %v; want none", p.calls(), d.Provider)
	}
}

// A daemon started with no global.toml keeps running, reports the missing
// file, and starts work once the file appears.
func TestRunWithoutGlobal(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t)
	useGlobal(t, d, p, root, "")
	if err := os.Remove(d.GlobalPath); err != nil {
		t.Fatal(err)
	}
	d.Global, d.Provider = noGlobal(), nil

	step(t, d, start)

	s := d.Snapshot()
	if len(s.ConfigErrors) != 1 || !strings.HasPrefix(s.ConfigErrors[0], d.GlobalPath+": ") {
		t.Errorf("config errors = %q, want global.toml's", s.ConfigErrors)
	}
	if len(s.Catalogs) != 0 || s.PauseReason == "" {
		t.Errorf("catalogs %+v, pause %q; want none and a reason", s.Catalogs, s.PauseReason)
	}

	writeGlobal(t, d, root, "")
	step(t, d, start.Add(time.Minute))
	step(t, d, start.Add(time.Minute+quiet))

	if p.calls() != 1 {
		t.Errorf("calls once global.toml exists = %d, want 1", p.calls())
	}
}

// noGlobal is the daemon's settings before any global.toml has loaded.
func noGlobal() settings.Global { return settings.Global{} }

func TestSnapshotBeforeRun(t *testing.T) {
	d, _ := newDaemon(t)

	if s := d.Snapshot(); !s.Started.IsZero() || s.Catalogs != nil {
		t.Errorf("snapshot before Run = %+v, want zero", s)
	}
}

// Run publishes a snapshot as soon as it starts, before its first crawl ends.
// That snapshot doesn't blame a missing API key, since global.toml hasn't
// been read yet.
func TestRunPublishesStartAtOnce(t *testing.T) {
	root := t.TempDir()
	d, _ := newDaemon(t, root)
	d.Provider = nil
	var seen control.Snapshot
	fsys := d.FS
	d.FS = func(dir string) fs.FS {
		seen = d.Snapshot()
		return fsys(dir)
	}

	step(t, d, start)

	if !seen.Started.Equal(start) || !seen.LastCrawl.IsZero() || seen.PauseReason != "" {
		t.Errorf("snapshot during the first crawl = %+v, want started at %v, no crawl, and no pause", seen, start)
	}
	if s := d.Snapshot(); s.PauseReason == "" {
		t.Errorf("snapshot after the first crawl = %+v, want the missing key", s)
	}
}

// Another goroutine may read snapshots while Run works. Run this test with
// -race.
func TestSnapshotWhileRunning(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	for i := range 5 {
		writeFile(t, root, fmt.Sprintf("f%d.md", i), body(fmt.Sprintf("f%d", i)))
	}
	d, _ := newDaemon(t, root)
	step(t, d, start)
	tick := make(chan time.Time)
	done := make(chan error, 1)
	go func() { done <- d.Run(t.Context(), fixedClock(start.Add(quiet)), tick) }()

	deadline := time.Now().Add(5 * time.Second)
	for {
		s := d.Snapshot()
		if len(s.Catalogs) == 1 && s.Catalogs[0].Summarized == 5 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("snapshot never showed all five summarized: %+v", s)
		}
		time.Sleep(time.Millisecond)
	}
	close(tick)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// Once leaves its status in Snapshot, so `catalog daemon --once` can say
// what it found, even when it stops at a service problem.
func TestOncePublishesStatus(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	writeFile(t, root, "b.md", body("b"))
	d, p := newDaemon(t, root)
	p.fail["a"] = apiError(400)
	p.fail["b"] = apiError(529)

	if err := d.Once(context.Background(), fixedClock(start)); err == nil {
		t.Fatal("Once returned no error for the service problem")
	}

	s := d.Snapshot()
	if !s.Started.Equal(start) || !s.LastCrawl.Equal(start) {
		t.Errorf("started %v, last crawl %v; want both %v", s.Started, s.LastCrawl, start)
	}
	want := []control.CatalogStatus{{Dir: root, Files: 2, Waiting: 2}}
	if !slices.Equal(s.Catalogs, want) {
		t.Errorf("catalogs = %+v, want %+v", s.Catalogs, want)
	}
	if got := failingPaths(s); !slices.Equal(got, []string{filepath.Join(root, "a.md")}) {
		t.Errorf("failing = %v, want a.md", got)
	}
}

// A world-accessible global.toml stops summarizing,
// though the daemon had a key before, and a fixed mode starts it again.
func TestRunStopsWhileGlobalIsWorldAccessible(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	built := useGlobal(t, d, p, root, "")
	if err := os.Chmod(d.GlobalPath, 0o644); err != nil {
		t.Fatal(err)
	}

	step(t, d, start)
	step(t, d, start.Add(quiet))

	if p.calls() != 0 {
		t.Errorf("calls while global.toml is open = %d, want 0", p.calls())
	}
	if d.Global.APIKey != "" {
		t.Errorf("kept the key %q while global.toml is open", d.Global.APIKey)
	}
	s := d.Snapshot()
	if len(s.ConfigErrors) != 1 || !strings.Contains(s.ConfigErrors[0], "world-accessible") {
		t.Errorf("config errors = %q, want one about world access", s.ConfigErrors)
	}

	if err := os.Chmod(d.GlobalPath, 0o600); err != nil {
		t.Fatal(err)
	}
	step(t, d, start.Add(quiet+time.Minute))

	if p.calls() != 1 {
		t.Errorf("calls after the fix = %d, want 1", p.calls())
	}
	if fmt.Sprint(*built) != "[key-1 claude-haiku-5-5]" {
		t.Errorf("built %v, want one provider after the fix", *built)
	}
	if s := d.Snapshot(); len(s.ConfigErrors) != 0 {
		t.Errorf("config errors after the fix = %q", s.ConfigErrors)
	}
}
