package control

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// timeLayout is how status shows a time, in local time. It gives the year,
// the seconds, and the offset from UTC, so the time means one moment.
const timeLayout = "2006-01-02 15:04:05-07:00"

// warningSign marks a line that asks the reader to do something.
const warningSign = "⚠️"

// Format describes s in plain lines for a person to read. commandVersion is
// the version of the program that asked for s.
func Format(s Snapshot, commandVersion string) string {
	var b strings.Builder
	b.WriteString(versionLine(s.Version, commandVersion))
	fmt.Fprintf(&b, "Started: %s\n", whenOrNotYet(s.Started))
	fmt.Fprintf(&b, "Last crawled: %s\n", whenOrNotYet(s.LastCrawl))
	if s.LastCrawl.IsZero() {
		// Until the first crawl ends, the daemon knows nothing to report.
		return b.String()
	}
	for _, e := range s.ConfigErrors {
		fmt.Fprintf(&b, "Config error: %s\n", e)
	}
	if s.PauseReason != "" {
		b.WriteString(pauseLine(s.PausedUntil, s.PauseReason))
	}
	writeCatalogs(&b, s.Catalogs)
	writeFailing(&b, s.Failing)
	return b.String()
}

// versionLine says which version the daemon runs. When that isn't the
// command's version, it says how to restart the daemon with the command's,
// which works whichever is older.
func versionLine(daemon, command string) string {
	if daemon == command {
		return "Daemon version: " + daemon + "\n"
	}
	if daemon == "" {
		daemon = "unknown"
	}
	return fmt.Sprintf("Daemon version: %s %s Run `catalog setup` to restart it with %s\n", daemon, warningSign, command)
}

// whenOrNotYet shows t, or says it hasn't happened if t is zero.
func whenOrNotYet(t time.Time) string {
	if t.IsZero() {
		return "not yet"
	}
	return when(t)
}

// pauseLine says why summarizing is paused, and until when if it will
// resume on its own.
func pauseLine(until time.Time, reason string) string {
	if until.IsZero() {
		return "Not summarizing: " + reason + "\n"
	}
	return fmt.Sprintf("Summarizing paused until %s: %s\n", when(until), reason)
}

func writeCatalogs(b *strings.Builder, catalogs []CatalogStatus) {
	if len(catalogs) == 0 {
		b.WriteString("\nNo catalogs found.\n")
		return
	}
	b.WriteString("\nCatalogs:\n")
	for _, c := range catalogs {
		fmt.Fprintf(b, "  %s: %d files, %d summarized, %d waiting, %d to update\n", c.Dir, c.Files, c.Summarized, c.Waiting, c.ToUpdate)
	}
}

func writeFailing(b *strings.Builder, failing []FailingFile) {
	if len(failing) == 0 {
		return
	}
	b.WriteString("\nFailing:\n")
	for _, f := range failing {
		retry := "next crawl"
		if !f.RetryAt.IsZero() {
			retry = when(f.RetryAt)
		}
		fmt.Fprintf(b, "  %s: %s (retry %s)\n", filepath.Join(f.Dir, f.Rel), f.Error, retry)
	}
}

func when(t time.Time) string {
	return t.Local().Format(timeLayout)
}
