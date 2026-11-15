package daemon

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
)

// countingFS records how many times each file is opened and how many of its
// bytes are read.
type countingFS struct {
	fs.FS
	opened map[string]int
	read   map[string]int
}

func newCountingFS(dir string) *countingFS {
	return &countingFS{FS: os.DirFS(dir), opened: map[string]int{}, read: map[string]int{}}
}

func (c *countingFS) Open(name string) (fs.File, error) {
	c.opened[name]++
	f, err := c.FS.Open(name)
	if err != nil {
		return nil, err
	}
	return &countingFile{File: f, read: func(n int) { c.read[name] += n }}, nil
}

func (c *countingFS) ReadFile(name string) ([]byte, error) {
	c.opened[name]++
	data, err := fs.ReadFile(c.FS, name)
	c.read[name] += len(data)
	return data, err
}

type countingFile struct {
	fs.File
	read func(int)
}

func (f *countingFile) Read(p []byte) (int, error) {
	n, err := f.File.Read(p)
	f.read(n)
	return n, err
}

func TestListingReadsOnlyTheStartOfAFile(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	long := body("long") + strings.Repeat("x", 99) + "\n"
	for len(long) < 100_000 {
		long += strings.Repeat("y", 99) + "\n"
	}
	writeFile(t, root, "long.txt", long)
	writeFile(t, root, "doc.pdf", "%PDF-1.4 "+strings.Repeat("z", 10_000))
	d, _ := newDaemon(t, root)
	crawl(t, d, start)
	before := catalogText(t, root)
	counting := newCountingFS(root)
	d.FS = func(string) fs.FS { return counting }

	if err := d.publishCatalog(start, root); err != nil {
		t.Fatal(err)
	}

	if n := counting.read["long.txt"]; n == 0 || n > 4096 {
		t.Errorf("read %d bytes of long.txt, want 1 to 4096", n)
	}
	if n := counting.opened["doc.pdf"]; n != 0 {
		t.Errorf("opened doc.pdf %d times, want 0", n)
	}
	if got := entry(t, before, "long.txt"); !strings.Contains(got, fmt.Sprintf("%d bytes", len(long))) {
		t.Errorf("long.txt entry = %q, want its full size", got)
	}
	if got := entry(t, before, "doc.pdf"); !strings.Contains(got, "PDF, 10009 bytes") {
		t.Errorf("doc.pdf entry = %q, want its full size", got)
	}
	if got := entry(t, before, "long.txt"); !strings.Contains(got, "body-long\nsecond line\n"+strings.Repeat("x", 99)) {
		t.Errorf("long.txt entry = %q, want its first lines", got)
	}
}

// A listing shows the same first lines whether it reads a prefix or the
// whole file.
func TestListingFirstLinesUnchanged(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	var lines []string
	for i := range 12 {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	writeFile(t, root, "short.md", strings.Join(lines, "\n"))
	writeFile(t, root, "wide.md", strings.Repeat("w", 1000)+"\n")
	d, _ := newDaemon(t, root)

	crawl(t, d, start)

	text := catalogText(t, root)
	want := "\n" + strings.Join(lines[:10], "\n") + "\n"
	if got := entry(t, text, "short.md"); !strings.Contains(got, want) || strings.Contains(got, "line 10") {
		t.Errorf("short.md entry = %q, want the first 10 lines", got)
	}
	if got := entry(t, text, "wide.md"); !strings.Contains(got, strings.Repeat("w", 512)) || strings.Contains(got, strings.Repeat("w", 513)) {
		t.Errorf("wide.md entry = %q, want 512 bytes", got)
	}
}

func TestListingOfUnreadableFileKeepsCrawledSize(t *testing.T) {
	root := t.TempDir()
	makeCatalog(t, root)
	writeFile(t, root, "a.md", body("a"))
	d, _ := newDaemon(t, root)
	crawl(t, d, start)
	if err := os.Remove(root + "/a.md"); err != nil {
		t.Fatal(err)
	}

	if err := d.publishCatalog(start, root); err != nil {
		t.Fatal(err)
	}

	got := entry(t, catalogText(t, root), "a.md")
	if !strings.Contains(got, fmt.Sprintf("Markdown, %d bytes", len(body("a")))) || strings.Contains(got, "body-a") {
		t.Errorf("a.md entry = %q", got)
	}
}
