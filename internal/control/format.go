package control

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// timeLayout is how status shows a time, in local time.
const timeLayout = "Jan 2 15:04"

// Format describes s in plain lines for a person to read.
func Format(s Snapshot) string {
	var b strings.Builder
	crawled := "hasn't crawled yet"
	if !s.LastCrawl.IsZero() {
		crawled = "last crawled " + when(s.LastCrawl)
	}
	fmt.Fprintf(&b, "Started %s; %s.\n", when(s.Started), crawled)
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
		fmt.Fprintf(b, "  %s: %d files, %d summarized, %d waiting\n", c.Dir, c.Files, c.Summarized, c.Waiting)
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
