package daemon

import (
	"strings"
	"testing"
	"time"

	"westarete.com/catalog/internal/journal"
)

// requestedNames lists the documents the provider was asked to summarize, in
// order.
func requestedNames(p *fakeProvider) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	names := make([]string, len(p.requests))
	for i, req := range p.requests {
		names[i] = documentName(req)
	}
	return names
}

// requestText returns the text of the provider's i-th request.
func requestText(p *fakeProvider, i int) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.requests[i].Text
}

func assertNames(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("summarized %v, want %v", got, want)
	}
}

// isPartial reports whether a summary event's recipe marks a summary written
// while another file in its catalog had no summary.
func isPartial(e journal.Event) bool {
	return strings.HasSuffix(e.Recipe, ";neighbors=partial")
}

// A file summarized while other files in its catalog had none is summarized
// again once they have summaries, so the model sees them as neighbors.
func TestNewCatalogUpdatesSummariesWrittenBeforeItsNeighbors(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	for _, name := range []string{"a", "b", "c"} {
		writeFile(t, root, name+".md", body(name))
	}
	d, p := newDaemon(t, root)

	once(t, d, start)

	assertNames(t, requestedNames(p), "a", "b", "c", "a", "b")
	names := requestedNames(p)
	for _, i := range []int{3, 4} {
		text := requestText(p, i)
		for _, n := range []string{"a", "b", "c"} {
			if n != names[i] && !strings.Contains(text, summaryOf(n)) {
				t.Errorf("update of %s didn't show %s's summary", names[i], n)
			}
		}
	}
	var partial []string
	for _, e := range eventsOfKind(t, d, journal.KindSummary) {
		if isPartial(e) {
			partial = append(partial, e.Rel)
		}
	}
	assertNames(t, partial, "a.md", "b.md")
	if got := d.Snapshot().Catalogs[0]; got.Summarized != 3 || got.Waiting != 0 {
		t.Errorf("status = %d summarized, %d waiting; want 3 and 0", got.Summarized, got.Waiting)
	}
	if got := entry(t, catalogText(t, root), "a.md"); !strings.Contains(got, summaryOf("a")) {
		t.Errorf("a.md entry = %q", got)
	}
}

// Files added together to a catalog whose files all have summaries are updated,
// except the last, and the files already there are left alone.
func TestFilesAddedTogetherAreUpdatedOnce(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	once(t, d, start)

	for _, name := range []string{"x", "y", "z"} {
		writeFile(t, root, name+".md", body(name))
	}
	once(t, d, start.Add(time.Hour))

	assertNames(t, requestedNames(p), "a", "x", "y", "z", "x", "y")
}

func TestOneNewFileIsNotUpdated(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	writeFile(t, root, "b.md", body("b"))
	d, p := newDaemon(t, root)
	once(t, d, start)

	writeFile(t, root, "c.md", body("c"))
	once(t, d, start.Add(time.Hour))

	assertNames(t, requestedNames(p), "a", "b", "a", "c")
	if e := eventsOfKind(t, d, journal.KindSummary); isPartial(e[len(e)-1]) {
		t.Errorf("c.md's summary is marked partial: %q", e[len(e)-1].Recipe)
	}
}

// When files are saved a little apart, the first is updated only after the
// later one has a summary, so the update sees it.
func TestUpdateWaitsForTheMissingNeighbor(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	crawl(t, d, start)
	writeFile(t, root, "b.md", body("b"))
	crawl(t, d, start.Add(2*time.Minute))

	for work(t, d, start.Add(3*time.Minute), false) { // a has settled; b hasn't
	}
	assertNames(t, requestedNames(p), "a")

	for work(t, d, start.Add(5*time.Minute), false) {
	}
	assertNames(t, requestedNames(p), "a", "b", "a")
	if !strings.Contains(requestText(p, 2), summaryOf("b")) {
		t.Errorf("update of a didn't show b's summary")
	}
}

// A file that won't get a summary soon doesn't count as missing: a PDF too
// large to send, or a file with a failure on record.
func TestFilesThatCantBeSummarizedDontCauseAnUpdate(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	sparsePDF(t, root, "huge.pdf", 95_000_001)
	writeFile(t, root, "0bad.md", body("bad"))
	writeFile(t, root, "a.md", body("a"))
	d, p := newDaemon(t, root)
	p.fail["bad"] = apiError(400)

	once(t, d, start)

	assertNames(t, requestedNames(p), "bad", "a")
	if e := eventsOfKind(t, d, journal.KindSummary); len(e) != 1 || isPartial(e[0]) {
		t.Errorf("summary events = %+v; want one, not partial", e)
	}
}
