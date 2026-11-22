package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"westarete.com/catalog/internal/journal"
	"westarete.com/catalog/internal/models"
	"westarete.com/catalog/internal/profiler"
	"westarete.com/catalog/internal/store"
)

func TestOnceProducesCatalog(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	writeFile(t, root, "docs/beta.md", body("beta"))
	d, p := newDaemon(t, root)

	once(t, d, start)

	text := catalogText(t, root)
	if got := entry(t, text, "alpha.md"); !strings.Contains(got, summaryOf("alpha")) {
		t.Errorf("alpha entry = %q", got)
	}
	if got := entry(t, text, "docs/beta.md"); !strings.Contains(got, summaryOf("beta")) {
		t.Errorf("beta entry = %q", got)
	}
	if strings.Contains(text, "\nNot summarized yet.") {
		t.Errorf("catalog still has a listing:\n%s", text)
	}
	// alpha is summarized again once beta has a summary.
	assertNames(t, requestedNames(p), "alpha", "beta", "alpha")
}

func TestOnceJournalsEachSummary(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	d, _ := newDaemon(t, root)

	once(t, d, start)

	events := eventsOfKind(t, d, journal.KindSummary)
	if len(events) != 1 {
		t.Fatalf("summary events = %+v", events)
	}
	e := events[0]
	want := journal.Event{
		Time: start, Kind: journal.KindSummary, Dir: root, Rel: "alpha.md",
		Recipe: string(profiler.Recipe(d.Global.Model)), InputTokens: 11, OutputTokens: 7,
	}
	e.Millis = 0
	if !e.Time.Equal(want.Time) {
		t.Errorf("time = %v, want %v", e.Time, want.Time)
	}
	e.Time = want.Time
	if e != want {
		t.Errorf("event = %+v, want %+v", e, want)
	}
}

func TestListingBeforeSummary(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	d, p := newDaemon(t, root)

	crawl(t, d, start)

	got := entry(t, catalogText(t, root), "alpha.md")
	if !strings.HasPrefix(got, "\nNot summarized yet.") || !strings.Contains(got, "body-alpha") {
		t.Errorf("alpha entry = %q", got)
	}
	if p.calls() != 0 {
		t.Errorf("provider calls = %d, want 0", p.calls())
	}
}

func TestQuietPeriod(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	d, p := newDaemon(t, root)
	crawl(t, d, start)

	if work(t, d, start.Add(3*time.Minute-time.Nanosecond), false) {
		t.Fatal("Work did something inside the quiet period")
	}
	if p.calls() != 0 {
		t.Fatalf("provider calls = %d, want 0", p.calls())
	}
	if !work(t, d, start.Add(3*time.Minute), false) {
		t.Fatal("Work did nothing once the quiet period passed")
	}
	if got := entry(t, catalogText(t, root), "alpha.md"); !strings.Contains(got, summaryOf("alpha")) {
		t.Errorf("alpha entry = %q", got)
	}
}

func TestIgnoreQuietSummarizesAtOnce(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	d, _ := newDaemon(t, root)
	crawl(t, d, start)

	if !work(t, d, start, true) {
		t.Fatal("Work with ignoreQuiet did nothing")
	}
}

func TestNestedCatalog(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(a, "b")
	makeCatalog(t, a)
	makeCatalog(t, b)
	writeFile(t, a, "top.md", body("top"))
	writeFile(t, b, "inner.md", body("inner"))
	writeFile(t, b, "deeper/more.md", body("more"))
	d, _ := newDaemon(t, root)

	once(t, d, start)

	aText := catalogText(t, a)
	if strings.Contains(aText, "inner.md") || strings.Contains(aText, "more.md") {
		t.Errorf("a's catalog lists b's files:\n%s", aText)
	}
	entry(t, aText, "top.md")
	bText := catalogText(t, b)
	entry(t, bText, "inner.md")
	entry(t, bText, "deeper/more.md")
	if strings.Contains(bText, "top.md") {
		t.Errorf("b's catalog lists a's files:\n%s", bText)
	}
}

// A subfolder with a .catalog/ folder but no config.toml isn't a catalog,
// and it isn't part of the catalog above it either.
func TestFolderWithCatalogDirIsLeftOutOfParent(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(a, "b")
	makeCatalog(t, a)
	if err := os.MkdirAll(filepath.Join(b, ".catalog"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, a, "top.md", body("top"))
	writeFile(t, b, "inner.md", body("inner"))
	d, p := newDaemon(t, root)

	once(t, d, start)

	aText := catalogText(t, a)
	entry(t, aText, "top.md")
	if strings.Contains(aText, "inner.md") {
		t.Errorf("a's catalog lists a file under b, which has a .catalog/ of its own:\n%s", aText)
	}
	assertNoCatalog(t, b)
	if p.calls() != 1 {
		t.Errorf("provider calls = %d, want 1 (inner.md belongs to no catalog)", p.calls())
	}
}

func TestWatchedFolderItselfIsNotACatalog(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	makeCatalog(t, sub)
	writeFile(t, root, "outside.md", body("outside"))
	writeFile(t, sub, "inside.md", body("inside"))
	d, p := newDaemon(t, root)

	once(t, d, start)

	assertNoCatalog(t, root)
	if p.calls() != 1 {
		t.Errorf("provider calls = %d, want 1 (outside.md belongs to no catalog)", p.calls())
	}
}

func TestFileFailureDoesNotBlock(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	writeFile(t, root, "beta.md", body("beta"))
	d, p := newDaemon(t, root)
	p.fail["alpha"] = apiError(400)

	once(t, d, start)

	f, ok, err := d.Store.Failure(root, "alpha.md")
	if err != nil || !ok {
		t.Fatalf("Failure: ok=%v err=%v", ok, err)
	}
	if f.Count != 1 || !f.RetryAt.Equal(start.Add(time.Minute)) || f.LastError == "" {
		t.Errorf("failure = %+v, want count 1, retry at %v", f, start.Add(time.Minute))
	}
	if got := entry(t, catalogText(t, root), "beta.md"); !strings.Contains(got, summaryOf("beta")) {
		t.Errorf("beta entry = %q", got)
	}
	failures := eventsOfKind(t, d, journal.KindFailure)
	if len(failures) != 1 || failures[0].Rel != "alpha.md" || failures[0].Dir != root ||
		failures[0].Error != f.LastError || !failures[0].Time.Equal(start) {
		t.Errorf("failure events = %+v", failures)
	}
}

func TestRepeatedFailureBacksOff(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	d, p := newDaemon(t, root)
	p.fail["alpha"] = apiError(400)
	once(t, d, start)

	if work(t, d, start.Add(time.Minute-time.Nanosecond), true) {
		t.Fatal("Work retried before RetryAt")
	}
	second := start.Add(time.Minute)
	if !work(t, d, second, true) {
		t.Fatal("Work did not retry at RetryAt")
	}
	f, _, _ := d.Store.Failure(root, "alpha.md")
	if f.Count != 2 || !f.RetryAt.Equal(second.Add(2*time.Minute)) {
		t.Errorf("failure = %+v, want count 2, retry at %v", f, second.Add(2*time.Minute))
	}
}

func TestSuccessClearsFailure(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	d, p := newDaemon(t, root)
	p.fail["alpha"] = apiError(400)
	once(t, d, start)

	delete(p.fail, "alpha")
	if !work(t, d, start.Add(time.Minute), true) {
		t.Fatal("Work did not retry")
	}
	if _, ok, _ := d.Store.Failure(root, "alpha.md"); ok {
		t.Error("failure still recorded after a summary")
	}
}

func TestUnreadableFile(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	writeFile(t, root, "beta.md", body("beta"))
	d, _ := newDaemon(t, root)
	chmod(t, filepath.Join(root, "alpha.md"), 0o000)

	once(t, d, start)

	text := catalogText(t, root)
	if strings.Contains(text, "alpha.md") {
		t.Errorf("catalog lists the unreadable file:\n%s", text)
	}
	if got := entry(t, text, "beta.md"); !strings.Contains(got, summaryOf("beta")) {
		t.Errorf("beta entry = %q", got)
	}
	if _, ok, _ := d.Store.Failure(root, "alpha.md"); ok {
		t.Error("an unreadable file got a Store failure")
	}
	want := journal.Event{Time: start, Kind: journal.KindFailure, Dir: root, Rel: "alpha.md", Error: "unreadable"}
	failures := eventsOfKind(t, d, journal.KindFailure)
	if len(failures) != 1 || !failures[0].Time.Equal(start) {
		t.Fatalf("failure events = %+v, want [%+v]", failures, want)
	}
	failures[0].Time = start
	if failures[0] != want {
		t.Errorf("failure event = %+v, want %+v", failures[0], want)
	}
}

func TestUnreadableFileJournaledOncePerSpell(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	alpha := filepath.Join(root, "alpha.md")
	writeFile(t, root, "alpha.md", body("alpha"))
	d, _ := newDaemon(t, root)
	chmod(t, alpha, 0o000)

	crawl(t, d, start)
	crawl(t, d, start.Add(time.Minute))
	if n := len(eventsOfKind(t, d, journal.KindFailure)); n != 1 {
		t.Fatalf("failure events after two crawls = %d, want 1", n)
	}

	chmod(t, alpha, 0o644)
	crawl(t, d, start.Add(2*time.Minute))
	// The crawl reads a file only when its size or time changes.
	writeFile(t, root, "alpha.md", body("alpha-edited"))
	chmod(t, alpha, 0o000)
	crawl(t, d, start.Add(3*time.Minute))
	if n := len(eventsOfKind(t, d, journal.KindFailure)); n != 2 {
		t.Errorf("failure events after it became unreadable again = %d, want 2", n)
	}
}

func TestServiceFailureChargesNoFile(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	d, p := newDaemon(t, root)
	p.fail["alpha"] = apiError(529)

	err := d.Once(context.Background(), fixedClock(start))

	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) || models.Classify(err) != models.ServiceProblem {
		t.Fatalf("Once error = %v, want the service problem", err)
	}
	if _, ok, _ := d.Store.Failure(root, "alpha.md"); ok {
		t.Error("a service problem was charged to the file")
	}
	if n := len(eventsOfKind(t, d, journal.KindFailure)); n != 0 {
		t.Errorf("failure events = %d, want 0", n)
	}
	if got := entry(t, catalogText(t, root), "alpha.md"); !strings.Contains(got, "Not summarized yet.") {
		t.Errorf("alpha entry = %q", got)
	}
}

func TestServiceFailureReturnsFromWork(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	d, p := newDaemon(t, root)
	p.fail["alpha"] = errors.New("dial tcp: connection refused")
	crawl(t, d, start)

	did, err := d.Work(context.Background(), start, true)
	if err == nil || did {
		t.Errorf("Work = %v, %v; want false and an error", did, err)
	}
}

func TestBadCatalogConfig(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	two := filepath.Join(root, "two")
	makeCatalog(t, one)
	makeCatalog(t, two)
	writeFile(t, one, ".catalog/config.toml", "exclude = [\"**/*.md\"]\n")
	writeFile(t, one, "a.txt", body("a"))
	writeFile(t, one, "b.md", body("b"))
	writeFile(t, two, "c.md", body("c"))
	d, _ := newDaemon(t, root)
	crawl(t, d, start)

	writeFile(t, one, ".catalog/config.toml", "include = [[[ garbage\n")
	writeFile(t, one, "d.txt", body("d"))
	writeFile(t, two, "e.md", body("e"))
	crawl(t, d, start.Add(time.Minute))

	oneText := catalogText(t, one)
	entry(t, oneText, "a.txt")
	entry(t, oneText, "d.txt")
	if strings.Contains(oneText, "b.md") {
		t.Errorf("catalog one dropped its globs:\n%s", oneText)
	}
	entry(t, catalogText(t, two), "e.md")
	errs := eventsOfKind(t, d, journal.KindConfigError)
	if len(errs) != 1 || errs[0].Dir != one || errs[0].Error == "" || !errs[0].Time.Equal(start.Add(time.Minute)) {
		t.Errorf("config_error events = %+v", errs)
	}
	if got := d.ConfigErrors(); len(got) != 1 || got[one] != errs[0].Error {
		t.Errorf("ConfigErrors = %v", got)
	}
}

func TestBadCatalogConfigJournaledOnce(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, _ := newDaemon(t, root)
	crawl(t, d, start)
	writeFile(t, root, ".catalog/config.toml", "include = [[[ garbage\n")

	crawl(t, d, start.Add(time.Minute))
	crawl(t, d, start.Add(2*time.Minute))

	if n := len(eventsOfKind(t, d, journal.KindConfigError)); n != 1 {
		t.Errorf("config_error events = %d, want 1", n)
	}
}

func TestFixedCatalogConfigClearsError(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	writeFile(t, root, "b.txt", body("b"))
	writeFile(t, root, ".catalog/config.toml", "include = [[[ garbage\n")
	d, _ := newDaemon(t, root)
	crawl(t, d, start)

	writeFile(t, root, ".catalog/config.toml", "exclude = [\"**/*.md\"]\n")
	crawl(t, d, start.Add(time.Minute))

	if got := d.ConfigErrors(); len(got) != 0 {
		t.Errorf("ConfigErrors = %v, want none", got)
	}
	text := catalogText(t, root)
	entry(t, text, "b.txt")
	if strings.Contains(text, "a.md") {
		t.Errorf("catalog ignores the fixed config:\n%s", text)
	}
}

func TestBadCatalogConfigWithNoLastGood(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	writeFile(t, root, ".catalog/config.toml", "include = [[[ garbage\n")
	d, p := newDaemon(t, root)

	once(t, d, start)

	assertNoCatalog(t, root)
	if p.calls() != 0 {
		t.Errorf("provider calls = %d, want 0", p.calls())
	}
	if n := len(eventsOfKind(t, d, journal.KindConfigError)); n != 1 {
		t.Errorf("config_error events = %d, want 1", n)
	}
}

func TestUnreadableCatalogConfig(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	writeFile(t, root, ".catalog/config.toml", "")
	chmod(t, filepath.Join(root, ".catalog", "config.toml"), 0o000)
	d, _ := newDaemon(t, root)

	crawl(t, d, start)

	if n := len(eventsOfKind(t, d, journal.KindConfigError)); n != 1 {
		t.Errorf("config_error events = %d, want 1", n)
	}
}

func TestUnwritableCatalog(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	two := filepath.Join(root, "two")
	makeCatalog(t, one)
	makeCatalog(t, two)
	writeFile(t, one, "a.md", body("a"))
	writeFile(t, two, "b.md", body("b"))
	chmod(t, filepath.Join(one, ".catalog"), 0o555)
	d, _ := newDaemon(t, root)

	once(t, d, start)

	errs := eventsOfKind(t, d, journal.KindPublishError)
	if len(errs) == 0 || errs[0].Dir != one || errs[0].Error == "" || !errs[0].Time.Equal(start) {
		t.Errorf("publish_error events = %+v", errs)
	}
	if got := entry(t, catalogText(t, two), "b.md"); !strings.Contains(got, summaryOf("b")) {
		t.Errorf("b entry = %q", got)
	}
}

func TestUnwritableCatalogJournaledOncePerSpell(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	chmod(t, filepath.Join(root, ".catalog"), 0o555)
	d, _ := newDaemon(t, root)

	crawl(t, d, start)
	crawl(t, d, start.Add(time.Minute))

	if n := len(eventsOfKind(t, d, journal.KindPublishError)); n != 1 {
		t.Errorf("publish_error events = %d, want 1", n)
	}
}

func TestNothingElseWritten(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	makeCatalog(t, a)
	makeCatalog(t, filepath.Join(a, "b"))
	writeFile(t, a, ".catalog/config.toml", "exclude = []\n")
	writeFile(t, a, "README.md", body("readme"))
	writeFile(t, a, "notes/one.md", body("one"))
	writeFile(t, a, "b/two.txt", body("two"))
	writeFile(t, root, "loose.md", body("loose"))
	before := tree(t, root)
	d, p := newDaemon(t, root)
	p.fail["one"] = apiError(400)

	once(t, d, start)

	for path := range tree(t, root) {
		if before[path] {
			continue
		}
		if path != "a/.catalog/catalog.md" && path != "a/b/.catalog/catalog.md" {
			t.Errorf("daemon wrote %s", path)
		}
	}
}

func TestOlderRecipeSummaryIsShownAndUsedAsNeighbor(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	writeFile(t, root, "beta.md", body("beta"))
	d, p := newDaemon(t, root)
	crawl(t, d, start)
	files, err := d.Store.Files(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Store.PutSummary(files["alpha.md"].Hash, "older recipe", "Old summary of alpha."); err != nil {
		t.Fatal(err)
	}
	p.fail["alpha"] = apiError(400)

	if !work(t, d, start, true) {
		t.Fatal("Work did nothing")
	}
	if !work(t, d, start, true) {
		t.Fatal("second Work did nothing")
	}

	if len(p.requests) != 2 || documentName(p.requests[0]) != "beta" {
		t.Fatalf("requests = %+v, want beta first (alpha has an older summary)", p.requests)
	}
	if !strings.Contains(p.requests[0].Text, "alpha.md: Old summary of alpha.") {
		t.Errorf("beta's request lacks alpha as a neighbor:\n%s", p.requests[0].Text)
	}
	if got := entry(t, catalogText(t, root), "alpha.md"); !strings.Contains(got, "Old summary of alpha.") {
		t.Errorf("alpha entry = %q", got)
	}
}

func TestNeighborsPreferCurrentRecipe(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	writeFile(t, root, "beta.md", body("beta"))
	d, p := newDaemon(t, root)
	crawl(t, d, start)
	files, _ := d.Store.Files(root)
	h := files["alpha.md"].Hash
	_ = d.Store.PutSummary(h, "a-older", "Old summary of alpha.")
	_ = d.Store.PutSummary(h, profiler.Recipe(d.Global.Model), "Current summary of alpha.")

	if !work(t, d, start, true) {
		t.Fatal("Work did nothing")
	}

	if len(p.requests) != 1 || !strings.Contains(p.requests[0].Text, "alpha.md: Current summary of alpha.") {
		t.Errorf("requests = %+v", p.requests)
	}
	if got := entry(t, catalogText(t, root), "alpha.md"); !strings.Contains(got, "Current summary of alpha.") {
		t.Errorf("alpha entry = %q", got)
	}
}

func TestNeighborsComeFromSameCatalogOnly(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	two := filepath.Join(root, "two")
	makeCatalog(t, one)
	makeCatalog(t, two)
	writeFile(t, one, "alpha.md", body("alpha"))
	writeFile(t, two, "beta.md", body("beta"))
	d, p := newDaemon(t, root)

	once(t, d, start)

	for _, req := range p.requests {
		if strings.Contains(req.Text, "Summary of") {
			t.Errorf("request for %s has a neighbor from another catalog:\n%s", documentName(req), req.Text)
		}
	}
}

func TestFileChangedSinceCrawlWaitsForNextCrawl(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	d, p := newDaemon(t, root)
	crawl(t, d, start)
	writeFile(t, root, "alpha.md", body("changed"))

	if work(t, d, start, true) {
		t.Fatal("Work summarized a file that changed since the crawl")
	}
	if p.calls() != 0 {
		t.Fatalf("provider calls = %d, want 0", p.calls())
	}

	crawl(t, d, start.Add(time.Minute))
	if !work(t, d, start.Add(time.Minute), true) {
		t.Fatal("Work did nothing after the next crawl")
	}
	if got := entry(t, catalogText(t, root), "alpha.md"); !strings.Contains(got, summaryOf("changed")) {
		t.Errorf("alpha entry = %q", got)
	}
}

func TestChangedFileDoesNotBlockOthers(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	writeFile(t, root, "beta.md", body("beta"))
	d, _ := newDaemon(t, root)
	crawl(t, d, start)
	writeFile(t, root, "alpha.md", body("changed"))

	if !work(t, d, start, true) {
		t.Fatal("Work did nothing")
	}
	if got := entry(t, catalogText(t, root), "beta.md"); !strings.Contains(got, summaryOf("beta")) {
		t.Errorf("beta entry = %q", got)
	}
}

func TestFileDeletedSinceCrawl(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	d, p := newDaemon(t, root)
	crawl(t, d, start)
	if err := os.Remove(filepath.Join(root, "alpha.md")); err != nil {
		t.Fatal(err)
	}

	if work(t, d, start, true) {
		t.Fatal("Work reported work for a deleted file")
	}
	if p.calls() != 0 {
		t.Errorf("provider calls = %d", p.calls())
	}
	if _, ok, _ := d.Store.Failure(root, "alpha.md"); ok {
		t.Error("a deleted file got a failure")
	}
}

func TestWorkBeforeCrawlDoesNothing(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	d, _ := newDaemon(t, root)

	if work(t, d, start, true) {
		t.Error("Work did something before any crawl")
	}
}

func TestSummaryIsStoredUnderCurrentRecipe(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	d, _ := newDaemon(t, root)

	once(t, d, start)

	files, _ := d.Store.Files(root)
	got, ok, err := d.Store.Summary(files["alpha.md"].Hash, profiler.Recipe(d.Global.Model))
	if err != nil || !ok || got != summaryOf("alpha") {
		t.Errorf("Summary = %q, %v, %v", got, ok, err)
	}
}

func TestPublishErrorJournaledAgainAfterRecovery(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	writeFile(t, root, "b.md", body("b"))
	catalogDir := filepath.Join(root, ".catalog")
	chmod(t, catalogDir, 0o555)
	d, _ := newDaemon(t, root)
	crawl(t, d, start)

	chmod(t, catalogDir, 0o755)
	if !work(t, d, start, true) {
		t.Fatal("Work did nothing")
	}
	chmod(t, catalogDir, 0o555)
	if !work(t, d, start, true) {
		t.Fatal("second Work did nothing")
	}

	if n := len(eventsOfKind(t, d, journal.KindPublishError)); n != 2 {
		t.Errorf("publish_error events = %d, want 2", n)
	}
}

func TestFailedFileWaitsForRetry(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "alpha.md", body("alpha"))
	d, p := newDaemon(t, root)
	p.fail["alpha"] = apiError(413)
	crawl(t, d, start)

	if !work(t, d, start, true) {
		t.Fatal("Work did nothing")
	}
	if work(t, d, start, true) {
		t.Error("Work retried a failed file at once")
	}
	if f, _, _ := d.Store.Failure(root, "alpha.md"); f.Count != 1 {
		t.Errorf("failure count = %d, want 1", f.Count)
	}
}

// A failure belongs to the content that failed. When a file changes, the
// new content waits only for the quiet period, not for the old backoff; a
// file that didn't change keeps its failure.
func TestChangedFileForgetsFailure(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	for _, name := range []string{"alpha", "beta", "gamma"} {
		writeFile(t, root, name+".md", body(name))
	}
	d, p := newDaemon(t, root)
	crawl(t, d, start)
	for _, rel := range []string{"alpha.md", "beta.md", "gamma.md"} {
		err := d.Store.RecordFailure(root, rel, store.Failure{Count: 9, LastError: "bad", RetryAt: start.Add(6 * time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
	}

	changed := start.Add(time.Minute)
	writeFile(t, root, "alpha.md", body("alpha")+"more\n")
	writeFile(t, root, "beta.md", body("beta")+"more\n")
	crawl(t, d, changed)

	later := changed.Add(3 * time.Minute)
	for work(t, d, later, false) {
	}
	// alpha is summarized again once beta has a summary; gamma waits.
	assertNames(t, requestedNames(p), "alpha", "beta", "alpha")
	for _, rel := range []string{"alpha.md", "beta.md"} {
		if _, ok, _ := d.Store.Failure(root, rel); ok {
			t.Errorf("failure still recorded for changed %s", rel)
		}
	}
	if _, ok, _ := d.Store.Failure(root, "gamma.md"); !ok {
		t.Error("crawl forgot the failure of unchanged gamma.md")
	}
}

func isServiceError(err error) bool {
	_, ok := errors.AsType[*serviceError](err)
	return ok
}

// workErr runs Work with ignoreQuiet and returns its error.
func workErr(t *testing.T, d *Daemon, now time.Time) (bool, error) {
	t.Helper()
	return d.Work(context.Background(), now, true)
}

// The API answers 400 when the account reaches a spend limit, which looks
// like a bad file. Three different files rejected in a row look like that, so
// the third is treated as a service problem and charged nothing.
func TestThirdRejectionInARowIsAServiceProblem(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	d, p := newDaemon(t, root)
	for _, name := range []string{"a", "b", "c"} {
		writeFile(t, root, name+".md", body(name))
		p.fail[name] = apiError(400)
	}
	crawl(t, d, start)

	for _, rel := range []string{"a.md", "b.md"} {
		if did, err := workErr(t, d, start); !did || err != nil {
			t.Fatalf("Work on %s = %v, %v; want a file failure", rel, did, err)
		}
	}
	did, err := workErr(t, d, start)
	if did || !isServiceError(err) {
		t.Fatalf("third Work = %v, %v; want a service error", did, err)
	}
	for rel, want := range map[string]bool{"a.md": true, "b.md": true, "c.md": false} {
		if _, ok, _ := d.Store.Failure(root, rel); ok != want {
			t.Errorf("failure for %s recorded = %v, want %v", rel, ok, want)
		}
	}
	if n := len(eventsOfKind(t, d, journal.KindFailure)); n != 2 {
		t.Errorf("failure events = %d, want 2", n)
	}

	// The pause starts the count again, so a file the model keeps rejecting
	// is charged and can't hold up the others for good.
	if did, err := workErr(t, d, start); !did || err != nil {
		t.Errorf("Work after the pause = %v, %v; want a file failure", did, err)
	}
	if _, ok, _ := d.Store.Failure(root, "c.md"); !ok {
		t.Error("c.md not charged after the pause")
	}
}

func TestSuccessEndsARunOfRejections(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	d, p := newDaemon(t, root)
	for _, name := range []string{"a", "b", "c", "d"} {
		writeFile(t, root, name+".md", body(name))
		if name != "b" {
			p.fail[name] = apiError(400)
		}
	}
	crawl(t, d, start)

	for range 4 {
		if did, err := workErr(t, d, start); !did || err != nil {
			t.Fatalf("Work = %v, %v; want it to go on", did, err)
		}
	}
}

// A file rejected twice is one file, not two.
func TestRejectionsInARowCountDifferentFiles(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	d, p := newDaemon(t, root)
	for _, name := range []string{"a", "b", "c"} {
		p.fail[name] = apiError(400)
	}
	writeFile(t, root, "a.md", body("a"))
	crawl(t, d, start)
	if did, err := workErr(t, d, start); !did || err != nil {
		t.Fatalf("first Work = %v, %v", did, err)
	}
	retry := start.Add(time.Minute)
	writeFile(t, root, "b.md", body("b"))
	crawl(t, d, retry)
	for range 2 {
		if did, err := workErr(t, d, retry); !did || err != nil {
			t.Fatalf("Work = %v, %v; want a file failure", did, err)
		}
	}
	if f, _, _ := d.Store.Failure(root, "a.md"); f.Count != 2 {
		t.Fatalf("a.md failure count = %d, want 2", f.Count)
	}

	writeFile(t, root, "c.md", body("c"))
	crawl(t, d, retry)
	if _, err := workErr(t, d, retry); !isServiceError(err) {
		t.Errorf("Work on the third file = %v, want a service error", err)
	}
}
