package daemon

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"westarete.com/catalog/internal/journal"
)

// Work hashes a file as it reads it and keeps only the part of a text file
// the model reads, so a large file costs little memory.
func TestWorkKeepsOnlyTheTextItSends(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	const size = 32 << 20
	writeFile(t, root, "big.csv", body("big")+strings.Repeat("x,y\n", size/4))
	d, p := newDaemon(t, root)
	crawl(t, d, start)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	did := work(t, d, start, true)
	runtime.ReadMemStats(&after)

	if !did || p.calls() != 1 {
		t.Fatalf("Work = %v with %d calls; want one summary", did, p.calls())
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 8<<20 {
		t.Errorf("Work allocated %d bytes for a %d-byte file", alloc, size)
	}
	if text := p.requests[0].Text; !strings.Contains(text, "[Document truncated") || len(text) > 500_000 {
		t.Errorf("request text is %d bytes, truncated = %v", len(text), strings.Contains(text, "[Document truncated"))
	}
}

func TestWorkSendsAPDFWhole(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	pdf := "%PDF-1.4 " + strings.Repeat("z", 500_000)
	writeFile(t, root, "doc.pdf", pdf)
	d, p := newDaemon(t, root)
	crawl(t, d, start)

	work(t, d, start, true)

	if p.calls() != 1 || !bytes.Equal(p.requests[0].PDF, []byte(pdf)) {
		t.Errorf("request PDF isn't the whole file")
	}
}

// sparsePDF makes a PDF of size bytes that takes no room on disk.
func sparsePDF(t *testing.T, root, rel string, size int64) {
	t.Helper()
	writeFile(t, root, rel, "%PDF-1.4 ")
	if err := os.Truncate(filepath.Join(root, rel), size); err != nil {
		t.Fatal(err)
	}
}

// The API takes a request of at most 128 MB, and base64 makes a PDF a third
// bigger, so a PDF over 95,000,000 bytes is listed and never sent.
func TestTooLargePDFIsListedNotSent(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	sparsePDF(t, root, "huge.pdf", 95_000_001)
	d, p := newDaemon(t, root)

	crawl(t, d, start)
	if work(t, d, start.Add(time.Hour), true) || p.calls() != 0 {
		t.Fatalf("Work sent the PDF: %d calls", p.calls())
	}
	crawl(t, d, start.Add(time.Hour))

	if got := entry(t, catalogText(t, root), "huge.pdf"); !strings.Contains(got, "Not summarized yet.") {
		t.Errorf("huge.pdf entry = %q", got)
	}
	failures := eventsOfKind(t, d, journal.KindFailure)
	if len(failures) != 1 || failures[0].Rel != "huge.pdf" || !strings.Contains(failures[0].Error, "95000001 bytes") {
		t.Errorf("failure events = %+v, want one for huge.pdf giving its size", failures)
	}
	if _, ok, _ := d.Store.Failure(root, "huge.pdf"); ok {
		t.Error("a retry was scheduled for huge.pdf")
	}
}

func TestPDFAtTheLimitIsSent(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	sparsePDF(t, root, "big.pdf", 95_000_000)
	d, p := newDaemon(t, root)
	crawl(t, d, start)

	if !work(t, d, start, true) || p.calls() != 1 || len(p.requests[0].PDF) != 95_000_000 {
		t.Errorf("Work didn't send the whole PDF: %d calls", p.calls())
	}
	if n := len(eventsOfKind(t, d, journal.KindFailure)); n != 0 {
		t.Errorf("failure events = %d, want 0", n)
	}
}

// An empty PDF has nothing for the model to read, so it stays a listing.
func TestEmptyPDFIsListedNotSent(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "empty.pdf", "")
	writeFile(t, root, "note.md", body("note"))
	d, p := newDaemon(t, root)
	crawl(t, d, start)

	for work(t, d, start, true) {
	}

	if p.calls() != 1 || p.requests[0].PDF != nil {
		t.Errorf("calls = %d, want only note.md sent", p.calls())
	}
	if got := entry(t, catalogText(t, root), "empty.pdf"); !strings.Contains(got, "Not summarized yet.") {
		t.Errorf("empty.pdf entry = %q", got)
	}
	if n := len(eventsOfKind(t, d, journal.KindFailure)); n != 0 {
		t.Errorf("failure events = %d, want 0", n)
	}
}

// Work keeps one byte more than the reader sends, so the reader can tell a
// text one byte too long and say it cut it.
func TestWorkTellsTheReaderTextWasCut(t *testing.T) {
	for size, want := range map[int]bool{400_000: false, 400_001: true} {
		root := t.TempDir()
		makeCatalog(t, root)
		head := body("doc")
		writeFile(t, root, "doc.txt", head+strings.Repeat("x", size-len(head)))
		d, p := newDaemon(t, root)
		crawl(t, d, start)

		work(t, d, start, true)

		if got := strings.Contains(p.requests[0].Text, "[Document truncated"); got != want {
			t.Errorf("%d-byte file: truncated = %v, want %v", size, got, want)
		}
	}
}
