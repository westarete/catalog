package daemon

import (
	"cmp"
	"slices"
	"strings"

	"westarete.com/catalog/internal/control"
	"westarete.com/catalog/internal/journal"
	"westarete.com/catalog/internal/model"
)

// noProvider is why the daemon isn't summarizing when it has no API key.
const noProvider = "no API key loaded from keys.toml"

// Snapshot returns the status Run or Once published last, or a zero Snapshot
// before either starts. Any goroutine may call it.
func (d *Daemon) Snapshot() control.Snapshot {
	if s := d.snapshot.Load(); s != nil {
		return *s
	}
	return control.Snapshot{}
}

// publishStatus builds a new snapshot for Snapshot to return. The snapshot
// shares nothing with the loop's own state, so the loop never changes it
// after this. If the Store fails, the last snapshot stays.
func (d *Daemon) publishStatus() error {
	s, err := d.status()
	if err != nil {
		return err
	}
	d.snapshot.Store(&s)
	return nil
}

func (d *Daemon) status() (control.Snapshot, error) {
	s := control.Snapshot{
		Started:      d.started,
		LastCrawl:    d.lastCrawl,
		ConfigErrors: d.configErrorLines(),
		PausedUntil:  d.pausedUntil,
		PauseReason:  d.pauseReason,
		Failing:      d.reportedFailures(),
	}
	// Before the first crawl, global.toml hasn't been read, so the missing
	// Provider says nothing yet.
	if d.Provider == nil && !d.lastCrawl.IsZero() {
		s.PauseReason = noProvider
	}
	for _, dir := range d.crawled {
		c, failing, err := d.catalogStatus(dir)
		if err != nil {
			return s, err
		}
		s.Catalogs = append(s.Catalogs, c)
		s.Failing = append(s.Failing, failing...)
	}
	slices.SortFunc(s.Catalogs, func(a, b control.CatalogStatus) int {
		return strings.Compare(a.Dir, b.Dir)
	})
	slices.SortFunc(s.Failing, func(a, b control.FailingFile) int {
		return cmp.Or(strings.Compare(a.Dir, b.Dir), strings.Compare(a.Rel, b.Rel))
	})
	return s, nil
}

// catalogStatus counts the files the last crawl recorded for the catalog in
// dir and returns the ones with a failure on record. The Store keeps a
// failure after its file is gone, so only current files are checked.
func (d *Daemon) catalogStatus(dir string) (control.CatalogStatus, []control.FailingFile, error) {
	files, err := d.Store.Files(dir)
	if err != nil {
		return control.CatalogStatus{}, nil, err
	}
	c := control.CatalogStatus{Dir: dir, Files: len(files)}
	var failing []control.FailingFile
	for rel, f := range files {
		summarized, toUpdate, err := d.summaryState(f.Hash)
		if err != nil {
			return c, nil, err
		}
		if summarized {
			c.Summarized++
		}
		if toUpdate {
			c.ToUpdate++
		}
		fail, found, err := d.Store.Failure(dir, rel)
		if err != nil {
			return c, nil, err
		}
		if found {
			failing = append(failing, control.FailingFile{Dir: dir, Rel: rel, Error: fail.LastError, RetryAt: fail.RetryAt})
		}
	}
	c.Waiting = c.Files - c.Summarized
	return c, failing, nil
}

// summaryState reports whether content h has a summary for catalog.md to
// show, and whether the daemon will update it: it does when the summary is
// partial or under an older recipe. Only a file without a current summary
// costs a second Store query.
func (d *Daemon) summaryState(h model.Hash) (summarized, toUpdate bool, err error) {
	_, current, err := d.Store.Summary(h, d.recipe())
	if err != nil || current {
		return current, false, err
	}
	_, _, older, err := d.Store.AnySummary(h)
	return older, older, err
}

// configErrorLines describes each config file that failed to load, in order.
func (d *Daemon) configErrorLines() []string {
	var lines []string
	for path, msg := range d.ConfigErrors() {
		lines = append(lines, path+": "+msg)
	}
	slices.Sort(lines)
	return lines
}

// reportedFailures lists the problems the daemon is reporting other than
// config errors: files and folders it can't read, and catalogs it can't
// publish. It tries each again on the next crawl.
func (d *Daemon) reportedFailures() []control.FailingFile {
	var out []control.FailingFile
	for k, p := range d.reported {
		if k.kind != journal.KindConfigError {
			out = append(out, control.FailingFile{Dir: k.dir, Rel: k.rel, Error: p.msg})
		}
	}
	return out
}
