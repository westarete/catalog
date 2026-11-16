package daemon

import (
	"cmp"
	"slices"
	"strings"

	"westarete.com/catalog/internal/control"
	"westarete.com/catalog/internal/journal"
)

// noProvider is why the daemon isn't summarizing when it has no API key.
const noProvider = "no API key loaded from global.toml"

// Snapshot returns the status Run published last, or a zero Snapshot before
// Run starts. Any goroutine may call it.
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
	if d.Provider == nil {
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
		_, ok, err := d.Store.Summary(f.Hash, d.recipe())
		if err != nil {
			return c, nil, err
		}
		if ok {
			c.Summarized++
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
