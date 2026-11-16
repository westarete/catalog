package daemon

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"westarete.com/catalog/internal/journal"
	"westarete.com/catalog/internal/model"
	"westarete.com/catalog/internal/settings"
	"westarete.com/catalog/internal/trees"
	"westarete.com/catalog/internal/watcher"
)

// catalogConfig is a catalog's own settings file, relative to its folder.
const catalogConfig = ".catalog/config.toml"

// unreadable is the journal error for a member file the crawl couldn't read.
const unreadable = "unreadable"

// Crawl rereads global.toml if GlobalPath is set, then finds the catalogs in
// every watched folder, records each catalog's files in the Store, and
// publishes each catalog. A problem in one folder or catalog is journaled and
// doesn't stop the others. Crawl returns an error only when the Store or the
// journal fails.
func (d *Daemon) Crawl(now time.Time) error {
	d.lastCrawl = now
	d.startCrawl()
	defer d.endCrawl()
	errs := []error{d.reloadGlobal(now)}
	for _, folder := range d.Global.Watch {
		if folder == "" {
			continue
		}
		errs = append(errs, d.crawlFolder(now, folder))
	}
	return errors.Join(errs...)
}

func (d *Daemon) crawlFolder(now time.Time, folder string) error {
	abs, err := filepath.Abs(folder)
	if err != nil {
		return d.report(failureEvent(now, folder, "", err.Error()))
	}
	walk := &tolerantFS{FS: d.FS(abs)}
	found, err := trees.FindCatalogs(walk)
	if err != nil {
		return d.report(failureEvent(now, abs, "", err.Error()))
	}
	var errs []error
	for _, skipped := range walk.skipped {
		errs = append(errs, d.report(failureEvent(now, abs, skipped.dir, skipped.err.Error())))
	}
	for _, c := range found {
		dir := filepath.Join(abs, filepath.FromSlash(c))
		errs = append(errs, d.crawlCatalog(now, dir, nestedUnder(c, found)))
	}
	return errors.Join(errs...)
}

// crawlCatalog records the files of the catalog in dir and publishes it. If
// the catalog's files can't be listed, it keeps the catalog as last published.
func (d *Daemon) crawlCatalog(now time.Time, dir string, nested []string) error {
	g, ok, configErr := d.catalogGlobs(now, dir)
	if !ok {
		return configErr
	}
	files := d.FS(dir)
	// The folder walk already journaled any directory the walk can't read.
	members, err := trees.Members(&tolerantFS{FS: files}, g.enumerate, g.ignore, nested)
	if err != nil {
		return errors.Join(configErr, d.report(failureEvent(now, dir, "", err.Error())))
	}
	prev, err := d.Store.Files(dir)
	if err != nil {
		return errors.Join(configErr, err)
	}
	states := watcher.Scan(files, members, prev, now)
	unreadableErr := d.reportUnreadable(now, dir, members, states)
	if err := d.Store.SetFiles(dir, states); err != nil {
		return errors.Join(configErr, unreadableErr, err)
	}
	if err := d.forgetOldFailures(dir, prev, states); err != nil {
		return errors.Join(configErr, unreadableErr, err)
	}
	tooLargeErr := d.reportTooLarge(now, dir, states)
	d.crawled = append(d.crawled, dir)
	return errors.Join(configErr, unreadableErr, tooLargeErr, d.publishCatalog(now, dir))
}

// catalogGlobs returns the globs for the catalog in dir. If its config.toml
// can't be read or parsed, it journals the error and returns the globs from
// the last config that loaded, if any; ok is false when there are none.
func (d *Daemon) catalogGlobs(now time.Time, dir string) (g globs, ok bool, err error) {
	data, err := fs.ReadFile(d.FS(dir), catalogConfig)
	if errors.Is(err, fs.ErrNotExist) {
		data, err = nil, nil
	}
	if err == nil {
		g.enumerate, g.ignore, err = settings.ApplyCatalog(d.Global, data)
	}
	if err != nil {
		last, found := d.lastGood[dir]
		return last, found, d.report(journal.Event{Time: now, Kind: journal.KindConfigError, Dir: dir, Error: err.Error()})
	}
	d.lastGood[dir] = g
	return g, true, nil
}

// reportUnreadable journals each member that Scan left out because it
// couldn't read it. The file drops out of the catalog until it's readable.
func (d *Daemon) reportUnreadable(now time.Time, dir string, members []string, states []model.FileState) error {
	scanned := make(map[string]bool, len(states))
	for _, s := range states {
		scanned[s.Rel] = true
	}
	var errs []error
	for _, rel := range members {
		if !scanned[rel] {
			errs = append(errs, d.report(failureEvent(now, dir, rel, unreadable)))
		}
	}
	return errors.Join(errs...)
}

// reportTooLarge journals each PDF too big to send to the model. Work never
// sends it, so it stays a listing until the file changes.
func (d *Daemon) reportTooLarge(now time.Time, dir string, states []model.FileState) error {
	var errs []error
	for _, s := range states {
		if isPDF(s.Rel) && s.Size > maxPDFBytes {
			msg := fmt.Sprintf("PDF is %d bytes, more than the %d the daemon sends to the model; it stays listed without a summary", s.Size, maxPDFBytes)
			errs = append(errs, d.report(failureEvent(now, dir, s.Rel, msg)))
		}
	}
	return errors.Join(errs...)
}

// forgetOldFailures clears the failure of each file whose content is new
// since the last crawl. The failure belonged to the old content, so the new
// content waits only for the quiet period.
func (d *Daemon) forgetOldFailures(dir string, prev map[string]model.FileState, states []model.FileState) error {
	for _, s := range states {
		if prev[s.Rel].Hash == s.Hash {
			continue
		}
		if err := d.Store.ClearFailure(dir, s.Rel); err != nil {
			return err
		}
	}
	return nil
}

// nestedUnder returns the catalogs in found that sit below catalog, as paths
// relative to it. All paths are slash paths relative to the watched folder.
func nestedUnder(catalog string, found []string) []string {
	prefix := catalog + "/"
	if catalog == "." {
		prefix = ""
	}
	var nested []string
	for _, other := range found {
		if rest, ok := strings.CutPrefix(other, prefix); ok && other != catalog {
			nested = append(nested, rest)
		}
	}
	return nested
}

func failureEvent(now time.Time, dir, rel, msg string) journal.Event {
	return journal.Event{Time: now, Kind: journal.KindFailure, Dir: dir, Rel: rel, Error: msg}
}

// tolerantFS lets a walk go on past a directory it can't list. fs.WalkDir,
// which trees uses, stops the whole walk at the first such directory; through
// tolerantFS the directory looks empty instead, and the error is kept in
// skipped. The root is the exception: a root that can't be listed still
// fails the walk, since that walk found nothing.
type tolerantFS struct {
	fs.FS
	skipped []skippedDir
}

type skippedDir struct {
	dir string
	err error
}

func (t *tolerantFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(t.FS, name)
	if err != nil && name != "." {
		t.skipped = append(t.skipped, skippedDir{name, err})
		return entries, nil
	}
	return entries, err
}
