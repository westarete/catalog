// Package daemon ties the catalog components together. A crawl finds the
// catalogs in each watched folder, records their files, and publishes their
// listings; a work step summarizes the one file the planner puts first.
package daemon

import (
	"context"
	"io/fs"
	"time"

	"westarete.com/catalog/internal/journal"
	"westarete.com/catalog/internal/model"
	"westarete.com/catalog/internal/models"
	"westarete.com/catalog/internal/profiler"
	"westarete.com/catalog/internal/settings"
	"westarete.com/catalog/internal/store"
)

// Daemon keeps the catalogs in the watched folders current. Only one
// goroutine may use a Daemon at a time.
type Daemon struct {
	Global      settings.Global
	Store       *store.Store
	Provider    models.Provider
	JournalPath string
	// FS opens a folder for reading, given its absolute path.
	FS func(dir string) fs.FS

	// crawled lists the catalogs the last crawl recorded, which are the
	// catalogs Work summarizes files in.
	crawled []string
	// lastGood holds each catalog's globs from the last config that parsed.
	lastGood map[string]globs
	// changed holds the files that changed after the last crawl recorded
	// them; Work leaves them for the next crawl.
	changed map[fileKey]bool
	// reported holds the problems found since the last crawl began, and
	// carried the ones from the crawl before, while a crawl runs. A problem
	// that persists from one crawl to the next is journaled only once.
	reported, carried map[problemKey]problem
}

type globs struct{ enumerate, ignore []string }

type fileKey struct{ dir, rel string }

type problemKey struct{ kind, dir, rel string }

// problem is the latest error text for a problem, and whether it has been
// journaled.
type problem struct {
	msg       string
	journaled bool
}

// Once crawls, then summarizes every file waiting in the crawled catalogs,
// without waiting for files to stay unchanged. It stops at the first service
// problem and returns it.
func (d *Daemon) Once(ctx context.Context, clock func() time.Time) error {
	if err := d.Crawl(clock()); err != nil {
		return err
	}
	for {
		did, err := d.Work(ctx, clock(), true)
		if err != nil || !did {
			return err
		}
	}
}

// ConfigErrors returns the catalogs whose config.toml failed to load on the
// last crawl, with the error for each.
func (d *Daemon) ConfigErrors() map[string]string {
	errs := map[string]string{}
	for k, p := range d.reported {
		if k.kind == journal.KindConfigError {
			errs[k.dir] = p.msg
		}
	}
	return errs
}

func (d *Daemon) recipe() model.Recipe {
	return profiler.Recipe(d.Global.Model)
}

// report journals a problem unless the same kind of problem with the same
// file or folder was already journaled since the crawl before this one. It
// compares no error text, since some errors name a new temporary file each
// time; it keeps the latest text for status. If the journal can't be
// written, the next report of the problem tries again.
func (d *Daemon) report(e journal.Event) error {
	k := problemKey{e.Kind, e.Dir, e.Rel}
	p := problem{msg: e.Error, journaled: d.reported[k].journaled || d.carried[k].journaled}
	var err error
	if !p.journaled {
		err = journal.Append(d.JournalPath, e)
		p.journaled = err == nil
	}
	d.reported[k] = p
	return err
}

// resolve forgets a problem, so it is journaled again if it comes back.
func (d *Daemon) resolve(kind, dir, rel string) {
	delete(d.reported, problemKey{kind, dir, rel})
}

// startCrawl moves the problems reported so far aside, so the crawl reports
// afresh and journals only the problems that are new or have changed.
func (d *Daemon) startCrawl() {
	d.carried, d.reported = d.reported, map[problemKey]problem{}
	d.crawled = nil
	d.changed = map[fileKey]bool{}
	if d.lastGood == nil {
		d.lastGood = map[string]globs{}
	}
}

func (d *Daemon) endCrawl() {
	d.carried = nil
}
