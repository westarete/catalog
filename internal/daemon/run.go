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
// then summarizes waiting files until there are none, ctx ends, or the model
// service fails.
//
// After the service fails, Run stops summarizing for a while, starting at a
// minute and doubling each time the service fails again, but it goes on
// crawling and publishing listings. It journals a pause when summarizing
// stops and a resume when the model next answers. Run doesn't stop for any
// other error: it writes the error to Log and tries again on the next tick.
func (d *Daemon) Run(ctx context.Context, clock func() time.Time, tick <-chan time.Time) error {
	for {
		d.step(ctx, clock)
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-tick:
			if !ok {
				return nil
			}
		}
	}
}

// step crawls, then summarizes files until none is waiting, unless
// summarizing is paused.
func (d *Daemon) step(ctx context.Context, clock func() time.Time) {
	d.logError(d.Crawl(clock()))
	if clock().Before(d.pausedUntil) {
		return
	}
	for ctx.Err() == nil {
		now := clock()
		did, err := d.Work(ctx, now, false)
		var service *serviceError
		switch {
		case errors.As(err, &service):
			// A request cut short by shutdown says nothing about the service.
			if ctx.Err() == nil {
				d.pause(now, err)
			}
			return
		case err != nil:
			d.logError(err)
			return
		case !did:
			return
		}
		d.resume(now)
	}
}

// pause stops summarizing until the backoff for the service problems so far
// has passed. It journals only the first problem in a row.
func (d *Daemon) pause(now time.Time, cause error) {
	d.serviceFailures++
	d.pausedUntil = now.Add(planner.Backoff(d.serviceFailures))
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
	d.logError(journal.Append(d.JournalPath, journal.Event{Time: now, Kind: journal.KindResume}))
}

func (d *Daemon) logError(err error) {
	if err != nil && d.Log != nil {
		d.Log.Print(err)
	}
}
