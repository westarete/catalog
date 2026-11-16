package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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
// returns the error and charges no file; the model rejecting several
// different files in a row counts as the service failing (see
// suspectService). With ignoreQuiet, files that changed
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
			if !sendable(f) {
				continue
			}
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

// maxPDFBytes is the largest PDF the daemon sends. The API takes a request
// of at most 128 MB, and base64 makes a PDF a third bigger
// (https://platform.claude.com/docs/en/build-with-claude/pdf-support). The
// API checks the page count itself.
const maxPDFBytes = 95_000_000

// sendable reports whether the model can read the file the crawl recorded as
// f. An empty PDF has nothing to read, and a PDF over maxPDFBytes is too big
// to send; both stay listings until the file changes.
func sendable(f model.FileState) bool {
	return !isPDF(f.Rel) || (f.Size > 0 && f.Size <= maxPDFBytes)
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

// readUnchanged returns what the model reads of the file, if the file still
// has the hash the crawl recorded. If not, it marks the file to wait for the
// next crawl.
func (d *Daemon) readUnchanged(c planner.Candidate) ([]byte, bool) {
	k := fileKey{c.Dir, c.Rel}
	if d.changed[k] {
		return nil, false
	}
	data, hash, err := readAndHash(d.FS(c.Dir), c.Rel, bytesSent(c.Rel))
	if err != nil || hash != c.Hash {
		d.changed[k] = true
		return nil, false
	}
	return data, true
}

// textBytesSent is how much of a text file the model reads: the reader cuts
// text at 400,000 bytes, and one byte more tells it the text was longer.
const textBytesSent = 400_000 + 1

// bytesSent is how much of the file at rel the model reads.
func bytesSent(rel string) int64 {
	if isPDF(rel) {
		return maxPDFBytes
	}
	return textBytesSent
}

// readAndHash returns up to keep bytes from the start of the file at rel and
// the hash of the whole file, reading it once, so a large file never sits in
// memory whole.
func readAndHash(folder fs.FS, rel string, keep int64) ([]byte, model.Hash, error) {
	f, err := folder.Open(rel)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	data, err := io.ReadAll(io.TeeReader(io.LimitReader(f, keep), h))
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(h, f); err != nil {
		return nil, "", err
	}
	return data, model.Hash(hex.EncodeToString(h.Sum(nil))), nil
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
	path := filepath.Join(c.Dir, c.Rel)
	if err != nil {
		if models.Classify(err) == models.ServiceProblem {
			return false, &serviceError{fmt.Errorf("summarize %s: %w", path, err)}
		}
		if d.suspectService(c) {
			return false, &serviceError{fmt.Errorf("summarize %s: %d files rejected in a row, as when the account reaches a spend limit: %w", path, rejectionsInARow, err)}
		}
		return true, d.recordFailure(now, c, err)
	}
	d.rejected = nil
	return true, d.recordSummary(now, c, text, resp, time.Since(began))
}

// rejectionsInARow is how many different files the model may reject in a
// row before the daemon blames the service instead.
const rejectionsInARow = 3

// suspectService notes that the model rejected c's file and reports whether
// that makes rejectionsInARow different files with no summary between them.
// The API rejects every request with a 400 once the account reaches a spend
// limit, just as it rejects a bad file, so a run of rejections more likely
// means the service. The count then starts again, so a file the model keeps
// rejecting is charged after the pause and can't hold up the others for good.
func (d *Daemon) suspectService(c planner.Candidate) bool {
	if d.rejected == nil {
		d.rejected = map[fileKey]bool{}
	}
	d.rejected[fileKey{c.Dir, c.Rel}] = true
	if len(d.rejected) < rejectionsInARow {
		return false
	}
	d.rejected = nil
	return true
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
