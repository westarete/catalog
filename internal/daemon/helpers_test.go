package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"westarete.com/catalog/internal/journal"
	"westarete.com/catalog/internal/models"
	"westarete.com/catalog/internal/settings"
	"westarete.com/catalog/internal/store"
)

// start is the time of the first crawl in every test.
var start = time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)

// fakeProvider answers each request with a summary named after the first
// "body-<name>" marker in the request text, which is the document's own body,
// since the profiler puts the document before its neighbors.
type fakeProvider struct {
	mu       sync.Mutex
	requests []models.Request
	fail     map[string]error
}

var bodyMarker = regexp.MustCompile(`body-(\w+)`)

// maxRequests bounds the requests one test may make. Past it the fake fails
// like a service outage, so a daemon that retries a file without end stops
// with an error instead of hanging the test.
const maxRequests = 100

var errTooManyRequests = errors.New("fake provider: too many requests")

func (p *fakeProvider) Complete(_ context.Context, req models.Request) (models.Response, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	if len(p.requests) > maxRequests {
		return models.Response{}, errTooManyRequests
	}
	name := documentName(req)
	if err := p.fail[name]; err != nil {
		return models.Response{}, err
	}
	return models.Response{Text: summaryOf(name), InputTokens: 11, OutputTokens: 7}, nil
}

func (p *fakeProvider) calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.requests)
}

func documentName(req models.Request) string {
	if m := bodyMarker.FindStringSubmatch(req.Text); m != nil {
		return m[1]
	}
	return ""
}

func summaryOf(name string) string { return "Summary of " + name + "." }

// body returns file content that the fake provider recognizes as name.
func body(name string) string { return "body-" + name + "\nsecond line\n" }

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeCatalog(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".catalog"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// chmod changes a path's mode and restores 0o755 when the test ends, so
// t.TempDir can remove it.
func chmod(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission tests don't work as root")
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
}

func newDaemon(t *testing.T, watch ...string) (*Daemon, *fakeProvider) {
	t.Helper()
	g, err := settings.ParseGlobal(nil)
	if err != nil {
		t.Fatal(err)
	}
	g.Watch = watch
	state := t.TempDir()
	s, err := store.Open(filepath.Join(state, "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	p := &fakeProvider{fail: map[string]error{}}
	return &Daemon{
		Global:      g,
		Store:       s,
		Provider:    p,
		JournalPath: filepath.Join(state, "journal.jsonl"),
		FS:          os.DirFS,
	}, p
}

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func once(t *testing.T, d *Daemon, now time.Time) {
	t.Helper()
	if err := d.Once(context.Background(), fixedClock(now)); err != nil {
		t.Fatalf("Once: %v", err)
	}
}

func crawl(t *testing.T, d *Daemon, now time.Time) {
	t.Helper()
	if err := d.Crawl(now); err != nil {
		t.Fatalf("Crawl: %v", err)
	}
}

func work(t *testing.T, d *Daemon, now time.Time, ignoreQuiet bool) bool {
	t.Helper()
	did, err := d.Work(context.Background(), now, ignoreQuiet)
	if err != nil {
		t.Fatalf("Work: %v", err)
	}
	return did
}

func catalogText(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".catalog", "catalog.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertNoCatalog(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, ".catalog", "catalog.md")); !os.IsNotExist(err) {
		t.Fatalf("catalog.md in %s: err = %v, want not exist", dir, err)
	}
}

// entry returns the text of the "### rel" section of a catalog.
func entry(t *testing.T, text, rel string) string {
	t.Helper()
	_, after, ok := strings.Cut(text, "\n### "+rel+"\n")
	if !ok {
		t.Fatalf("no entry for %s in:\n%s", rel, text)
	}
	if i := strings.Index(after, "\n#"); i >= 0 {
		after = after[:i]
	}
	return after
}

func journalEvents(t *testing.T, d *Daemon) []journal.Event {
	t.Helper()
	f, err := os.Open(d.JournalPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var events []journal.Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e journal.Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		events = append(events, e)
	}
	return events
}

func eventsOfKind(t *testing.T, d *Daemon, kind string) []journal.Event {
	t.Helper()
	var out []journal.Event
	for _, e := range journalEvents(t, d) {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

// tree lists every path under root, relative to it.
func tree(t *testing.T, root string) map[string]bool {
	t.Helper()
	paths := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		paths[filepath.ToSlash(rel)] = true
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

// apiError builds the error the Anthropic SDK returns for an HTTP status. Its
// Error method reads Request and Response, so both are set.
func apiError(status int) *anthropic.Error {
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	if err != nil {
		panic(err)
	}
	return &anthropic.Error{StatusCode: status, Request: req, Response: &http.Response{StatusCode: status}}
}
