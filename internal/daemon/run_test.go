package daemon

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"westarete.com/catalog/internal/journal"
	"westarete.com/catalog/internal/models"
	"westarete.com/catalog/internal/profiler"
)

// quiet is how long a file must stay unchanged before Run summarizes it.
const quiet = 3 * time.Minute

// step runs the loop for exactly one step at now: Run steps once at the
// start, and the closed tick channel then stops it.
func step(t *testing.T, d *Daemon, now time.Time) {
	t.Helper()
	runTicks(t, d, now)
}

// runTicks runs the loop with the given ticks already waiting, then stops it
// when they run out. The clock reads now throughout.
func runTicks(t *testing.T, d *Daemon, now time.Time, ticks ...time.Time) {
	t.Helper()
	tick := make(chan time.Time, len(ticks))
	for _, at := range ticks {
		tick <- at
	}
	close(tick)
	if err := d.Run(context.Background(), fixedClock(now), tick); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func kinds(t *testing.T, d *Daemon, want ...string) []string {
	t.Helper()
	keep := map[string]bool{}
	for _, k := range want {
		keep[k] = true
	}
	var got []string
	for _, e := range journalEvents(t, d) {
		if keep[e.Kind] {
			got = append(got, e.Kind)
		}
	}
	return got
}

func TestRunPausesOnServiceProblem(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	p.fail["a"] = apiError(529)

	step(t, d, start)
	if p.calls() != 0 {
		t.Fatalf("calls before the file settled = %d, want 0", p.calls())
	}
	failedAt := start.Add(quiet)
	step(t, d, failedAt)
	if p.calls() != 1 {
		t.Fatalf("calls = %d, want 1", p.calls())
	}

	writeFile(t, root, "b.md", body("b"))
	step(t, d, failedAt.Add(30*time.Second))
	delete(p.fail, "a")
	step(t, d, failedAt.Add(time.Minute-time.Second))
	if p.calls() != 1 {
		t.Errorf("calls while paused = %d, want 1", p.calls())
	}
	if got := entry(t, catalogText(t, root), "b.md"); !strings.Contains(got, "Not summarized yet.") {
		t.Errorf("no listing published while paused: b.md entry = %q", got)
	}

	step(t, d, failedAt.Add(time.Minute))
	if p.calls() != 2 {
		t.Errorf("calls after the pause = %d, want 2", p.calls())
	}
	if got := kinds(t, d, journal.KindPause, journal.KindResume); fmt.Sprint(got) != "[pause resume]" {
		t.Errorf("journal = %v, want [pause resume]", got)
	}
	pauses := eventsOfKind(t, d, journal.KindPause)
	if !pauses[0].Time.Equal(failedAt) || !strings.Contains(pauses[0].Error, "529") {
		t.Errorf("pause event = %+v", pauses[0])
	}
	if !eventsOfKind(t, d, journal.KindResume)[0].Time.Equal(failedAt.Add(time.Minute)) {
		t.Errorf("resume event = %+v", eventsOfKind(t, d, journal.KindResume)[0])
	}
}

// Each service problem in a row doubles the pause, but only the first one
// journals a pause. A success ends the run of problems, so the next pause is
// short again.
func TestRunBacksOffWhileServiceKeepsFailing(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	p.fail["a"] = apiError(529)
	step(t, d, start)

	t0 := start.Add(quiet)
	step(t, d, t0)                      // fails: paused for 1 minute
	step(t, d, t0.Add(time.Minute))     // fails again: paused for 2 more
	step(t, d, t0.Add(3*time.Minute-1)) // still paused
	if p.calls() != 2 {
		t.Fatalf("calls = %d, want 2", p.calls())
	}
	delete(p.fail, "a")
	step(t, d, t0.Add(3*time.Minute))
	if p.calls() != 3 {
		t.Fatalf("calls after the longer pause = %d, want 3", p.calls())
	}

	t1 := t0.Add(3 * time.Minute)
	writeFile(t, root, "c.md", body("c"))
	p.fail["c"] = apiError(529)
	step(t, d, t1)
	step(t, d, t1.Add(quiet)) // fails: paused for 1 minute, not 4
	delete(p.fail, "c")
	step(t, d, t1.Add(quiet+time.Minute))
	if p.calls() != 5 {
		t.Errorf("calls = %d, want 5", p.calls())
	}
	want := "[pause resume pause resume]"
	if got := kinds(t, d, journal.KindPause, journal.KindResume); fmt.Sprint(got) != want {
		t.Errorf("journal = %v, want %s", got, want)
	}
}

// A file the model rejects shows that the service answered, so it ends a
// pause as a summary would.
func TestRunResumesOnFileProblem(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	p.fail["a"] = apiError(529)
	step(t, d, start)
	step(t, d, start.Add(quiet))

	p.fail["a"] = apiError(400)
	step(t, d, start.Add(quiet+time.Minute))

	if got := kinds(t, d, journal.KindPause, journal.KindResume); fmt.Sprint(got) != "[pause resume]" {
		t.Errorf("journal = %v, want [pause resume]", got)
	}
}

func TestRunSummarizesEveryWaitingFile(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	for _, name := range []string{"a", "b", "c"} {
		writeFile(t, root, name+".md", body(name))
	}
	d, p := newDaemon(t, root)
	step(t, d, start)

	step(t, d, start.Add(quiet))

	// a and b are summarized again once c has a summary.
	assertNames(t, requestedNames(p), "a", "b", "c", "a", "b")
	for _, name := range []string{"a", "b", "c"} {
		if got := entry(t, catalogText(t, root), name+".md"); !strings.Contains(got, summaryOf(name)) {
			t.Errorf("%s entry = %q", name, got)
		}
	}
	if n := len(eventsOfKind(t, d, journal.KindResume)); n != 0 {
		t.Errorf("resume events without a pause = %d", n)
	}
}

func TestRunPicksUpNewCatalog(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	writeFile(t, proj, "a.md", body("a"))
	d, _ := newDaemon(t, root)

	step(t, d, start)
	assertNoCatalog(t, proj)

	makeCatalog(t, proj)
	step(t, d, start.Add(time.Minute))
	entry(t, catalogText(t, proj), "a.md")
}

// useGlobal makes the daemon reread its settings from a global.toml that
// watches root and adds extra, and has NewProvider record each provider it
// builds and hand back p.
func useGlobal(t *testing.T, d *Daemon, p *fakeProvider, root, extra string) *[]string {
	t.Helper()
	d.GlobalPath = filepath.Join(t.TempDir(), "global.toml")
	d.Global.APIKey = "key-1"
	built := &[]string{}
	d.NewProvider = func(apiKey, model string) models.Provider {
		*built = append(*built, apiKey+" "+model)
		return p
	}
	writeGlobal(t, d, root, extra)
	return built
}

func writeGlobal(t *testing.T, d *Daemon, root, extra string) {
	t.Helper()
	text := fmt.Sprintf("api_key = \"key-1\"\n%s\n[[watch]]\nfolder = %q\n", extra, root)
	if err := os.WriteFile(d.GlobalPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRunRereadsGlobal(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	writeFile(t, root, "b.txt", body("b"))
	d, p := newDaemon(t, root)
	useGlobal(t, d, p, root, `enumerate = ["**/*.md"]`)

	step(t, d, start)
	if text := catalogText(t, root); strings.Contains(text, "b.txt") {
		t.Fatalf("b.txt listed before the change:\n%s", text)
	}

	writeGlobal(t, d, root, `enumerate = ["**/*.txt"]`)
	step(t, d, start.Add(time.Minute))

	text := catalogText(t, root)
	entry(t, text, "b.txt")
	if strings.Contains(text, "a.md") {
		t.Errorf("a.md still listed after the change:\n%s", text)
	}
}

func TestRunFollowsNewWatchFolder(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	makeCatalog(t, second)
	writeFile(t, second, "a.md", body("a"))
	d, p := newDaemon(t, first)
	useGlobal(t, d, p, first, "")

	step(t, d, start)
	assertNoCatalog(t, second)

	writeGlobal(t, d, second, "")
	step(t, d, start.Add(time.Minute))
	entry(t, catalogText(t, second), "a.md")
}

// An unparsable global.toml leaves the daemon on the last settings that
// loaded. (One with no api_key is used: see TestRunWithoutAPIKey.)
func TestRunKeepsLastGoodGlobal(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	writeFile(t, root, "b.txt", body("b"))
	d, p := newDaemon(t, root)
	useGlobal(t, d, p, root, `enumerate = ["**/*.md"]`)
	step(t, d, start)
	if err := os.WriteFile(d.GlobalPath, []byte("enumerate = [[["), 0o600); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "c.md", body("c"))

	step(t, d, start.Add(time.Minute))
	step(t, d, start.Add(2*time.Minute))

	catalog := catalogText(t, root)
	entry(t, catalog, "c.md")
	if strings.Contains(catalog, "b.txt") {
		t.Errorf("fell back to the defaults:\n%s", catalog)
	}
	if d.Global.APIKey != "key-1" || d.Global.Model != "claude-haiku-5-5" {
		t.Errorf("global = %+v, want the last good one", d.Global)
	}
	events := eventsOfKind(t, d, journal.KindConfigError)
	if len(events) != 1 || events[0].Dir != filepath.Dir(d.GlobalPath) || events[0].Rel != "global.toml" {
		t.Fatalf("config errors = %+v, want one for global.toml", events)
	}
	if _, ok := d.ConfigErrors()[d.GlobalPath]; !ok {
		t.Errorf("ConfigErrors = %v, want global.toml", d.ConfigErrors())
	}

	writeGlobal(t, d, root, `enumerate = ["**/*.md"]`)
	step(t, d, start.Add(3*time.Minute))
	if errs := d.ConfigErrors(); len(errs) != 0 {
		t.Errorf("ConfigErrors after the fix = %v", errs)
	}
}

func TestRunMissingGlobalIsConfigError(t *testing.T) {
	root := t.TempDir()
	d, p := newDaemon(t, root)
	useGlobal(t, d, p, root, "")
	step(t, d, start)
	if err := os.Remove(d.GlobalPath); err != nil {
		t.Fatal(err)
	}

	step(t, d, start.Add(time.Minute))

	if n := len(eventsOfKind(t, d, journal.KindConfigError)); n != 1 {
		t.Errorf("config errors = %d, want 1", n)
	}
	if len(d.Global.Watch) != 1 || d.Global.Watch[0] != root {
		t.Errorf("watch = %v, want the last good list", d.Global.Watch)
	}
}

func TestRunRebuildsProviderWhenKeyOrModelChanges(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	built := useGlobal(t, d, p, root, "")

	step(t, d, start)
	if len(*built) != 0 {
		t.Fatalf("built %v though nothing changed", *built)
	}

	writeGlobal(t, d, root, `model = "claude-other"`)
	step(t, d, start.Add(time.Minute))
	text := strings.Replace(mustRead(t, d.GlobalPath), "key-1", "key-2", 1)
	if err := os.WriteFile(d.GlobalPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	step(t, d, start.Add(quiet))

	want := "[key-1 claude-other key-2 claude-other]"
	if fmt.Sprint(*built) != want {
		t.Errorf("built %v, want %s", *built, want)
	}
	summaries := eventsOfKind(t, d, journal.KindSummary)
	if len(summaries) != 1 || summaries[0].Recipe != string(profiler.Recipe("claude-other")) {
		t.Errorf("summaries = %+v, want one under the new model's recipe", summaries)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRunStepsOnEachTick(t *testing.T) {
	root := t.TempDir()
	d, _ := newDaemon(t, root)
	crawls := 0
	d.FS = func(dir string) fs.FS {
		if dir == root {
			crawls++
		}
		return os.DirFS(dir)
	}

	runTicks(t, d, start, start.Add(time.Minute), start.Add(2*time.Minute))

	if crawls != 3 {
		t.Errorf("crawls = %d, want 3: one at the start and one per tick", crawls)
	}
}

func TestRunReturnsWhenContextEnds(t *testing.T) {
	d, _ := newDaemon(t, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)

	go func() { done <- d.Run(ctx, fixedClock(start), make(chan time.Time)) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run = %v, want nil", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run didn't return after its context ended")
	}
}

// cancelingProvider ends the run's context in the middle of a request, as a
// SIGTERM would, and fails the request with the context's error.
type cancelingProvider struct {
	cancel context.CancelFunc
	calls  int
}

func (c *cancelingProvider) Complete(ctx context.Context, _ models.Request) (models.Response, error) {
	c.calls++
	c.cancel()
	return models.Response{}, ctx.Err()
}

// A request cut short by shutdown is not the service failing.
func TestRunStopsWithoutPausingWhenContextEnds(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	writeFile(t, root, "b.md", body("b"))
	d, _ := newDaemon(t, root)
	step(t, d, start)
	ctx, cancel := context.WithCancel(context.Background())
	c := &cancelingProvider{cancel: cancel}
	d.Provider = c

	if err := d.Run(ctx, fixedClock(start.Add(quiet)), make(chan time.Time)); err != nil {
		t.Fatalf("Run = %v", err)
	}

	if c.calls != 1 {
		t.Errorf("calls = %d, want 1", c.calls)
	}
	if n := len(eventsOfKind(t, d, journal.KindPause)); n != 0 {
		t.Errorf("pause events = %d, want 0", n)
	}
}

// When a ctx ends between two files, Run summarizes no more of them.
func TestRunStopsSummarizingWhenContextEnds(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	writeFile(t, root, "b.md", body("b"))
	d, p := newDaemon(t, root)
	step(t, d, start)
	ctx, cancel := context.WithCancel(context.Background())
	d.Provider = providerFunc(func(ctx context.Context, req models.Request) (models.Response, error) {
		defer cancel()
		return p.Complete(ctx, req)
	})

	if err := d.Run(ctx, fixedClock(start.Add(quiet)), make(chan time.Time)); err != nil {
		t.Fatalf("Run = %v", err)
	}

	if p.calls() != 1 {
		t.Errorf("calls = %d, want 1", p.calls())
	}
}

type providerFunc func(context.Context, models.Request) (models.Response, error)

func (f providerFunc) Complete(ctx context.Context, req models.Request) (models.Response, error) {
	return f(ctx, req)
}

func TestRunLogsStepErrorsAndGoesOn(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	writeFile(t, root, "b.md", body("b"))
	d, p := newDaemon(t, root)
	var logged bytes.Buffer
	d.Log = log.New(&logged, "", 0)
	step(t, d, start)
	d.JournalPath = filepath.Join(t.TempDir(), "missing", "journal.jsonl")

	step(t, d, start.Add(quiet))

	if p.calls() != 1 {
		t.Errorf("calls = %d, want 1: Run stops summarizing after an error", p.calls())
	}
	if !strings.Contains(logged.String(), "journal.jsonl") {
		t.Errorf("log = %q, want the journal error", logged.String())
	}

	logged.Reset()
	_ = d.Store.Close()
	step(t, d, start.Add(quiet+time.Minute))
	if !strings.Contains(logged.String(), "closed") {
		t.Errorf("log = %q, want the Store error", logged.String())
	}
}

func TestRunWithoutLogDropsErrors(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	d, _ := newDaemon(t, root)
	_ = d.Store.Close()

	step(t, d, start)
}

// Fixing the API key ends a pause at once, however long the backoff had
// grown, but changing nothing doesn't.
func TestRunNewKeyEndsPause(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	useGlobal(t, d, p, root, "")
	p.fail["a"] = apiError(401)
	step(t, d, start)
	failedAt := start.Add(quiet)
	step(t, d, failedAt)
	delete(p.fail, "a")

	step(t, d, failedAt.Add(10*time.Second))
	if p.calls() != 1 {
		t.Fatalf("calls with the same key = %d, want 1", p.calls())
	}

	text := strings.Replace(mustRead(t, d.GlobalPath), "key-1", "key-2", 1)
	if err := os.WriteFile(d.GlobalPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	step(t, d, failedAt.Add(20*time.Second))

	if p.calls() != 2 {
		t.Errorf("calls after the new key = %d, want 2", p.calls())
	}
	if got := kinds(t, d, journal.KindPause, journal.KindResume); fmt.Sprint(got) != "[pause resume]" {
		t.Errorf("journal = %v, want [pause resume]", got)
	}
}

// fakeTime is a clock that moves only when the test moves it.
type fakeTime struct{ now time.Time }

func (f *fakeTime) clock() time.Time { return f.now }

// A long backlog doesn't hold up crawling: when a tick arrives while Run is
// summarizing, Run crawls before it summarizes the next file.
func TestRunCrawlsDuringBacklog(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	makeCatalog(t, proj)
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		writeFile(t, proj, name+".md", body(name))
	}
	d, p := newDaemon(t, root)
	step(t, d, start)
	crawls := 0
	d.FS = func(dir string) fs.FS {
		if dir == root {
			crawls++
		}
		return os.DirFS(dir)
	}
	clock := &fakeTime{now: start.Add(quiet)}
	tick := make(chan time.Time, 1)
	var crawlsSeen []int
	d.Provider = providerFunc(func(ctx context.Context, req models.Request) (models.Response, error) {
		crawlsSeen = append(crawlsSeen, crawls)
		clock.now = clock.now.Add(20 * time.Second)
		if len(crawlsSeen) == 2 {
			tick <- clock.now
			close(tick)
		}
		return p.Complete(ctx, req)
	})

	if err := d.Run(context.Background(), clock.clock, tick); err != nil {
		t.Fatalf("Run = %v", err)
	}

	// Five first summaries, then the updates of a through d.
	if got := fmt.Sprint(crawlsSeen); got != "[1 1 2 2 2 2 2 2 2]" {
		t.Errorf("crawls before each call = %s, want [1 1 2 2 2 2 2 2 2]", got)
	}
}

// A pause counts from the tick that started the step, not from the moment
// the service failed, so a one-minute pause ends on the next tick.
func TestRunPauseCountsFromTick(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	writeFile(t, root, "b.md", body("b"))
	d, p := newDaemon(t, root)
	step(t, d, start)
	stepAt := start.Add(quiet)
	clock := &fakeTime{now: stepAt}
	calls := 0
	tick := make(chan time.Time, 1)
	d.Provider = providerFunc(func(ctx context.Context, req models.Request) (models.Response, error) {
		calls++
		clock.now = clock.now.Add(30 * time.Second)
		if calls == 2 {
			tick <- stepAt.Add(time.Minute)
			close(tick)
			return models.Response{}, apiError(529)
		}
		return p.Complete(ctx, req)
	})

	if err := d.Run(context.Background(), clock.clock, tick); err != nil {
		t.Fatalf("Run = %v", err)
	}

	if calls != 4 {
		t.Errorf("calls = %d, want 4: a, b failing 30s into the step, then b and the update of a on the next tick", calls)
	}
	if got := kinds(t, d, journal.KindPause, journal.KindResume); fmt.Sprint(got) != "[pause resume]" {
		t.Errorf("journal = %v, want [pause resume]", got)
	}
}

func TestRunPausesOnThirdRejectionInARow(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	d, p := newDaemon(t, root)
	for _, name := range []string{"a", "b", "c"} {
		writeFile(t, root, name+".md", body(name))
		p.fail[name] = apiError(400)
	}
	step(t, d, start)
	step(t, d, start.Add(quiet))

	if got := kinds(t, d, journal.KindPause, journal.KindFailure); fmt.Sprint(got) != "[failure failure pause]" {
		t.Errorf("journal = %v, want [failure failure pause]", got)
	}
	if s := d.Snapshot(); !s.PausedUntil.Equal(start.Add(quiet + time.Minute)) {
		t.Errorf("paused until %v, want %v", s.PausedUntil, start.Add(quiet+time.Minute))
	}
}
