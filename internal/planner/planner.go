// Package planner decides which files the daemon summarizes next and how long
// to wait after a failed summary. It does no I/O and reads no clock; the
// caller passes in the current time.
package planner

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"westarete.com/catalog/internal/model"
)

// quietPeriod is how long a file must stay unchanged before it is summarized.
const quietPeriod = 3 * time.Minute

// Backoff timing: the first failure waits firstBackoff, each later failure
// doubles the wait, and the wait never exceeds maxBackoff.
const (
	firstBackoff = time.Minute
	maxBackoff   = 6 * time.Hour
)

// Candidate is one file the daemon could summarize.
type Candidate struct {
	Dir, Rel  string
	Hash      model.Hash
	ChangedAt time.Time
	// HasCurrent means the file has a summary under the current recipe, and
	// HasOlder that it has one under any recipe, the current one included.
	HasCurrent bool
	HasOlder   bool
	// HasPartial means the file has a summary written with the current
	// reader, prompt, and model while another file in its catalog was
	// waiting for a summary, so it is to be updated.
	HasPartial bool
	// RetryAt is when to retry after a failure, or zero with no failure.
	RetryAt time.Time
}

// Plan returns the candidates that are ready to summarize at now, in the
// order to summarize them. Files with no summary come before files whose
// summary is out of date, so new files are not stuck behind a backlog of
// updates. Within each group, the file that changed earliest comes first.
// A file with a partial summary waits until no catalog with a copy of it
// has a file waiting, so its update sees every neighbor it missed. Plan does
// not modify cs.
func Plan(cs []Candidate, now time.Time) []Candidate {
	held := heldUpdates(cs)
	var ready []Candidate
	for _, c := range cs {
		if isReady(c, now) && !held[c.Hash] {
			ready = append(ready, c)
		}
	}
	slices.SortFunc(ready, compareOrder)
	return ready
}

// Backoff returns how long to wait after the given number of consecutive
// failures. Zero or fewer failures means no wait.
func Backoff(failures int) time.Duration {
	if failures < 1 {
		return 0
	}
	d := firstBackoff
	for i := 1; i < failures && d < maxBackoff; i++ {
		d *= 2
	}
	return min(d, maxBackoff)
}

// WouldBePartial reports whether a summary of c written now would be
// partial: it is the first summary of c's content, and a file in c's catalog
// with other content is waiting for one. An identical copy doesn't count: the
// update would show the model nothing new. A file is updated only once per
// content, so a summary of content that had one already is never partial.
func WouldBePartial(cs []Candidate, c Candidate) bool {
	if c.HasOlder {
		return false
	}
	return slices.ContainsFunc(cs, func(o Candidate) bool {
		return o.Dir == c.Dir && o.Hash != c.Hash && waiting(o)
	})
}

// waiting reports whether c's content has no summary yet and should get one.
// A file with a failure on record may never get one, so it doesn't count.
func waiting(c Candidate) bool {
	return !c.HasOlder && c.RetryAt.IsZero()
}

// heldUpdates returns the content whose partial summary must wait to be updated,
// because a catalog with a copy of it has a file waiting for its first
// summary. Copies share one summary, so a copy in another catalog waits too;
// otherwise its update would see the wrong neighbors and end the wait for all.
func heldUpdates(cs []Candidate) map[model.Hash]bool {
	withWaiting := map[string]bool{}
	for _, c := range cs {
		if waiting(c) {
			withWaiting[c.Dir] = true
		}
	}
	held := map[model.Hash]bool{}
	for _, c := range cs {
		if c.HasPartial && withWaiting[c.Dir] {
			held[c.Hash] = true
		}
	}
	return held
}

func isReady(c Candidate, now time.Time) bool {
	return !c.HasCurrent &&
		now.Sub(c.ChangedAt) >= quietPeriod &&
		!now.Before(c.RetryAt)
}

func compareOrder(a, b Candidate) int {
	return cmp.Or(
		compareBool(a.HasOlder, b.HasOlder),
		a.ChangedAt.Compare(b.ChangedAt),
		strings.Compare(a.Dir, b.Dir),
		strings.Compare(a.Rel, b.Rel),
	)
}

// compareBool orders false before true.
func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case b:
		return -1
	default:
		return 1
	}
}
