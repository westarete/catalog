package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"westarete.com/catalog/internal/journal"
	"westarete.com/catalog/internal/model"
	"westarete.com/catalog/internal/models"
	"westarete.com/catalog/internal/planner"
	"westarete.com/catalog/internal/profiler"
	"westarete.com/catalog/internal/reader"
	"westarete.com/catalog/internal/store"
)

// Work summarizes the file the planner puts first among the catalogs the last
// crawl recorded, and publishes that file's catalog. It reports whether it
// did anything. A file that changed or went away since the crawl is left for
// the next crawl, and Work moves on to the next file.
//
// When the model rejects the file, Work records a failure, so the file waits
// before it's tried again, and journals it. When the service fails, Work
// returns the error and charges no file. With ignoreQuiet, files that changed
// recently are summarized without waiting for them to settle.
func (d *Daemon) Work(ctx context.Context, now time.Time, ignoreQuiet bool) (did bool, err error) {
	cs, err := d.candidates(ignoreQuiet)
	if err != nil {
		return false, err
	}
	for _, c := range planner.Plan(cs, now) {
		data, ok := d.readUnchanged(c)
		if ok {
			return d.summarize(ctx, now, c, data)
		}
	}
	return false, nil
}

// candidates describes every file in the crawled catalogs for the planner.
func (d *Daemon) candidates(ignoreQuiet bool) ([]planner.Candidate, error) {
	var cs []planner.Candidate
	for _, dir := range d.crawled {
		files, err := d.Store.Files(dir)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			c, err := d.candidate(dir, f)
			if err != nil {
				return nil, err
			}
			if ignoreQuiet {
				c.ChangedAt = time.Time{}
			}
			cs = append(cs, c)
		}
	}
	return cs, nil
}

func (d *Daemon) candidate(dir string, f model.FileState) (planner.Candidate, error) {
	c := planner.Candidate{Dir: dir, Rel: f.Rel, Hash: f.Hash, ChangedAt: f.ChangedAt}
	var err error
	if _, c.HasCurrent, err = d.Store.Summary(f.Hash, d.recipe()); err != nil {
		return c, err
	}
	if _, _, c.HasOlder, err = d.Store.AnySummary(f.Hash); err != nil {
		return c, err
	}
	failure, _, err := d.Store.Failure(dir, f.Rel)
	c.RetryAt = failure.RetryAt
	return c, err
}

// readUnchanged returns the file's bytes if they still have the hash the
// crawl recorded. If not, it marks the file to wait for the next crawl.
func (d *Daemon) readUnchanged(c planner.Candidate) ([]byte, bool) {
	k := fileKey{c.Dir, c.Rel}
	if d.changed[k] {
		return nil, false
	}
	data, err := fs.ReadFile(d.FS(c.Dir), c.Rel)
	if err != nil || hashOf(data) != c.Hash {
		d.changed[k] = true
		return nil, false
	}
	return data, true
}

func hashOf(data []byte) model.Hash {
	sum := sha256.Sum256(data)
	return model.Hash(hex.EncodeToString(sum[:]))
}

// summarize profiles one file and stores, journals, and publishes the result.
func (d *Daemon) summarize(ctx context.Context, now time.Time, c planner.Candidate, data []byte) (bool, error) {
	files, err := d.Store.Files(c.Dir)
	if err != nil {
		return false, err
	}
	neighbors, err := d.neighbors(files)
	if err != nil {
		return false, err
	}
	began := time.Now()
	text, resp, err := profiler.Profile(ctx, d.Provider, c.Rel, reader.Read(c.Rel, data), neighbors)
	if err != nil {
		if models.Classify(err) == models.ServiceProblem {
			return false, &serviceError{fmt.Errorf("summarize %s: %w", filepath.Join(c.Dir, c.Rel), err)}
		}
		return true, d.recordFailure(now, c, err)
	}
	return true, d.recordSummary(now, c, text, resp, time.Since(began))
}

// serviceError is a failure of the model service rather than of a file. Work
// charges no file for it, and Run pauses summarizing.
type serviceError struct{ err error }

func (e *serviceError) Error() string { return e.err.Error() }
func (e *serviceError) Unwrap() error { return e.err }

func (d *Daemon) recordSummary(now time.Time, c planner.Candidate, text string, resp models.Response, took time.Duration) error {
	if err := d.Store.PutSummary(c.Hash, d.recipe(), text); err != nil {
		return err
	}
	if err := d.Store.ClearFailure(c.Dir, c.Rel); err != nil {
		return err
	}
	err := journal.Append(d.JournalPath, journal.Event{
		Time: now, Kind: journal.KindSummary, Dir: c.Dir, Rel: c.Rel,
		Recipe:       string(d.recipe()),
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		Millis:       took.Milliseconds(),
	})
	if err != nil {
		return err
	}
	return d.publishCatalog(now, c.Dir)
}

// recordFailure counts one more failure for the file and sets when to retry.
func (d *Daemon) recordFailure(now time.Time, c planner.Candidate, cause error) error {
	prev, _, err := d.Store.Failure(c.Dir, c.Rel)
	if err != nil {
		return err
	}
	count := prev.Count + 1
	f := store.Failure{Count: count, LastError: cause.Error(), RetryAt: now.Add(planner.Backoff(count))}
	if err := d.Store.RecordFailure(c.Dir, c.Rel, f); err != nil {
		return err
	}
	return journal.Append(d.JournalPath, failureEvent(now, c.Dir, c.Rel, f.LastError))
}
