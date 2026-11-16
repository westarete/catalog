package daemon

import (
	"context"
	"errors"
	"time"

	"westarete.com/catalog/internal/journal"
	"westarete.com/catalog/internal/planner"
)

// Run keeps the catalogs current until ctx ends or tick is closed, and then
// returns nil. It takes one step at once and another on each tick: it crawls,
// then summarizes waiting files until there are none, ctx ends, the model
// service fails, or the next tick arrives. A tick that arrives during a step
// starts the next step as soon as the current file is done, so a long backlog
// doesn't hold up crawling.
//
// After the service fails, Run stops summarizing for a while, starting at a
// minute and doubling each time the service fails again, but it goes on
// crawling and publishing listings. The pause counts from the tick that
// started the step, so a one-minute pause ends on the next tick. A new API key
// or model ends the pause at once. Run journals a pause when summarizing stops
// and a resume when the model next answers. Run doesn't stop for any other
// error: it writes the error to Log and tries again on the next tick. While
// Provider is nil, Run crawls and publishes listings but doesn't summarize.
//
// Run publishes a new Snapshot when it starts, after each crawl, and after
// each file it summarizes.
func (d *Daemon) Run(ctx context.Context, clock func() time.Time, tick <-chan time.Time) error {
	at := clock()
	d.started = at
	d.logError(d.publishStatus())
	for {
		next, ticked := d.step(ctx, clock, at, tick)
		if !ticked {
			if next, ticked = nextTick(ctx, tick); !ticked {
				return nil
			}
		}
		at = next
	}
}

// nextTick waits for the next tick and returns its time. It returns false if
// ctx ends or tick is closed first.
func nextTick(ctx context.Context, tick <-chan time.Time) (time.Time, bool) {
	select {
	case <-ctx.Done():
		return time.Time{}, false
	case t, ok := <-tick:
		return t, ok
	}
}

// step crawls, then summarizes files until none is waiting, unless
// summarizing is paused. at is the time of the tick that started the step. If
// a tick arrives while step summarizes, step stops after the current file and
// returns that tick's time and true.
func (d *Daemon) step(ctx context.Context, clock func() time.Time, at time.Time, tick <-chan time.Time) (time.Time, bool) {
	d.logError(d.Crawl(clock()))
	d.logError(d.publishStatus())
	if d.Provider == nil || at.Before(d.pausedUntil) {
		return time.Time{}, false
	}
	for ctx.Err() == nil && d.summarizeNext(ctx, clock(), at) {
		select {
		case t, ok := <-tick:
			if ok {
				return t, true
			}
			// A closed tick means no more ticks: finish the backlog, and Run
			// stops after it.
		default:
		}
	}
	return time.Time{}, false
}

// summarizeNext summarizes the next waiting file, then pauses, resumes, or
// logs an error as the outcome requires, and publishes the status. It
// reports whether to go on to the next file.
func (d *Daemon) summarizeNext(ctx context.Context, now, at time.Time) bool {
	defer func() { d.logError(d.publishStatus()) }()
	did, err := d.Work(ctx, now, false)
	var service *serviceError
	switch {
	case errors.As(err, &service):
		// A request cut short by shutdown says nothing about the service.
		if ctx.Err() == nil {
			d.pause(now, at, err)
		}
		return false
	case err != nil:
		d.logError(err)
		return false
	case !did:
		return false
	}
	d.resume(now)
	return true
}

// pause stops summarizing until the backoff for the service problems so far
// has passed, counting from at, the tick that started the step. It journals
// only the first problem in a row, at now.
func (d *Daemon) pause(now, at time.Time, cause error) {
	d.serviceFailures++
	d.pausedUntil = at.Add(planner.Backoff(d.serviceFailures))
	d.pauseReason = cause.Error()
	if d.serviceFailures == 1 {
		d.logError(journal.Append(d.JournalPath, journal.Event{Time: now, Kind: journal.KindPause, Error: cause.Error()}))
	}
}

// resume ends a run of service problems, journaling it if there was one.
func (d *Daemon) resume(now time.Time) {
	if d.serviceFailures == 0 {
		return
	}
	d.serviceFailures = 0
	d.pausedUntil, d.pauseReason = time.Time{}, ""
	d.logError(journal.Append(d.JournalPath, journal.Event{Time: now, Kind: journal.KindResume}))
}

func (d *Daemon) logError(err error) {
	if err != nil && d.Log != nil {
		d.Log.Print(err)
	}
}
