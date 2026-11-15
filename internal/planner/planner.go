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
	Dir, Rel   string
	Hash       model.Hash
	ChangedAt  time.Time
	HasCurrent bool
	HasOlder   bool
	RetryAt    time.Time
}

// Plan returns the candidates that are ready to summarize at now, in the
// order to summarize them. Files with no summary come before files whose
// summary is out of date, so new files are not stuck behind a backlog of
// updates. Within each group, the file that changed earliest comes first.
// Plan does not modify cs.
func Plan(cs []Candidate, now time.Time) []Candidate {
	var ready []Candidate
	for _, c := range cs {
		if isReady(c, now) {
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
