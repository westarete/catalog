package daemon

import (
	"io"
	"io/fs"
	"time"

	"westarete.com/catalog/internal/journal"
	"westarete.com/catalog/internal/model"
	"westarete.com/catalog/internal/profiler"
	"westarete.com/catalog/internal/publish"
	"westarete.com/catalog/internal/reader"
	"westarete.com/catalog/internal/render"
)

// publishCatalog renders the catalog in dir from the Store and writes its
// catalog.md. A write that fails is journaled, and the old file stays.
func (d *Daemon) publishCatalog(now time.Time, dir string) error {
	entries, err := d.entries(dir)
	if err != nil {
		return err
	}
	if _, err := publish.Publish(dir, render.Render(entries)); err != nil {
		return d.report(journal.Event{Time: now, Kind: journal.KindPublishError, Dir: dir, Error: err.Error()})
	}
	d.resolve(journal.KindPublishError, dir, "")
	return nil
}

// entries returns one entry for each file recorded for dir: its summary if
// it has one, else a listing.
func (d *Daemon) entries(dir string) ([]render.Entry, error) {
	files, err := d.Store.Files(dir)
	if err != nil {
		return nil, err
	}
	summaries, err := d.summaries(files)
	if err != nil {
		return nil, err
	}
	folder := d.FS(dir)
	entries := make([]render.Entry, 0, len(files))
	for rel, f := range files {
		e := render.Entry{Rel: rel, Summary: summaries[rel]}
		if e.Summary == "" {
			e.Listing = listing(folder, f)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// listingPrefix is how much of a file a listing reads. A listing shows at
// most 10 lines and 512 bytes, so this is plenty, and publishing a catalog
// doesn't read whole files again.
const listingPrefix = 4096

// listing describes f with the size the crawl recorded. It reads the start of
// the file for its first lines; for a PDF, which shows none, it reads
// nothing. If the file can't be read now, the listing has no first lines.
func listing(folder fs.FS, f model.FileState) reader.Listing {
	var start []byte
	if !isPDF(f.Rel) {
		start = readPrefix(folder, f.Rel)
	}
	l := reader.List(f.Rel, start)
	l.Size = f.Size
	return l
}

// isPDF reports whether the file at rel is a PDF. reader decides what counts
// as one; reading no bytes costs nothing.
func isPDF(rel string) bool {
	return reader.Read(rel, nil).PDF
}

// readPrefix returns up to listingPrefix bytes from the start of the file at
// rel, or as much as it could read before an error.
func readPrefix(folder fs.FS, rel string) []byte {
	file, err := folder.Open(rel)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()
	data, _ := io.ReadAll(io.LimitReader(file, listingPrefix))
	return data
}

// neighbors turns summaries, keyed by path, into neighbors for the profiler to
// tell a document apart from the others in its catalog.
func neighbors(summaries map[string]string) []profiler.Neighbor {
	out := make([]profiler.Neighbor, 0, len(summaries))
	for rel, text := range summaries {
		out = append(out, profiler.Neighbor{Rel: rel, Summary: text})
	}
	return out
}

// summaries returns the summary of each file that has one, keyed by path.
func (d *Daemon) summaries(files map[string]model.FileState) (map[string]string, error) {
	out := make(map[string]string, len(files))
	for rel, f := range files {
		text, ok, err := d.summary(f.Hash)
		if err != nil {
			return nil, err
		}
		if ok {
			out[rel] = text
		}
	}
	return out, nil
}

// summary returns the summary for content h under the current recipe, or
// under an older one if that's all there is.
func (d *Daemon) summary(h model.Hash) (string, bool, error) {
	text, ok, err := d.Store.Summary(h, d.recipe())
	if err != nil || ok {
		return text, ok, err
	}
	text, _, ok, err = d.Store.AnySummary(h)
	return text, ok, err
}
