package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// The reporter is the one place that decides how a command talks to the
// person running it. Every command describes what happened to each document
// with the same words — added, modified, deleted, and failed for a document
// the model could not profile — whether status is reporting what is out of
// sync or a run is reporting what it did. Each report ends with an outcome
// line and, when a model was called, a cost line. Everything goes to one
// writer, in order.

// errReported is returned by a command that has already told the person
// everything through the reporter and only needs a non-zero exit. main exits
// without printing it.
var errReported = errors.New("details already reported")

type action int

const (
	actionAdd action = iota
	actionModify
	actionDelete
	actionFail
)

// outcomeOrder is the order counts appear in the outcome line.
var outcomeOrder = []action{actionAdd, actionModify, actionDelete, actionFail}

var words = map[action]string{
	actionAdd:    "added",
	actionModify: "modified",
	actionDelete: "deleted",
	actionFail:   "failed",
}

// wordWidth is the longest verb plus two spaces, so paths line up.
const wordWidth = 10

type reporter struct {
	out     io.Writer
	counts  map[action]int
	seen    map[string]bool // documents already counted, so a two-pass run counts each once
	wrote   bool            // whether anything has been printed yet
	waiting int             // documents reported as pending
	steps   int             // documents to profile this run; 0 means no counter (status, deletions only)
	step    int             // documents profiled so far
}

func newReporter(out io.Writer) *reporter {
	return &reporter{out: out, counts: map[action]int{}, seen: map[string]bool{}}
}

// pending prints a line for a document that is out of sync and still waiting
// for update to deal with it. Used by status.
func (r *reporter) pending(a action, path string) {
	r.waiting++
	r.line(words[a], path, "")
}

// done prints a line for work a run did and counts it toward the outcome
// line. A removal has no counter; profiled documents always get one (see
// startProfiling).
func (r *reporter) done(a action, path, note string) {
	if !r.seen[path] {
		r.seen[path] = true
		r.counts[a]++
	}
	r.line(words[a], path, note)
}

// heading prints a line that introduces a group of document lines, set off
// from whatever came before it.
func (r *reporter) heading(text string) {
	if r.wrote {
		r.printf("\n")
	}
	r.printf("%s\n", text)
}

// startProfiling tells the reporter how many documents the model is about to
// profile, so every profiled line carries a [n/total] counter, even when the
// run is a single document.
func (r *reporter) startProfiling(total int) {
	r.step = 0
	r.steps = total
}

// profiled is done for a document the model profiled, counting it toward the
// run's progress.
func (r *reporter) profiled(a action, path, note string) {
	r.step++
	r.done(a, path, note)
}

// printf writes to the output, ignoring write errors the way fmt.Printf does:
// there is nothing useful to do if the terminal is gone.
func (r *reporter) printf(format string, args ...any) {
	r.wrote = true
	_, _ = fmt.Fprintf(r.out, format, args...)
}

func (r *reporter) line(word, path, note string) {
	var b strings.Builder
	if r.steps > 0 {
		b.WriteString(r.counter())
	}
	fmt.Fprintf(&b, "%-*s%s", wordWidth, word, path)
	if note != "" {
		fmt.Fprintf(&b, " (%s)", note)
	}
	r.printf("%s\n", b.String())
}

// counter is "[ 3/240] ", padded so lines without progress (removals) line up
// with lines that have it.
func (r *reporter) counter() string {
	if r.step == 0 {
		return strings.Repeat(" ", len(fmt.Sprintf("[%d/%d] ", r.steps, r.steps)))
	}
	width := len(strconv.Itoa(r.steps))
	return fmt.Sprintf("[%*d/%d] ", width, r.step, r.steps)
}

// outcome prints a blank line, then what changed: the counts of each action
// that happened, or "up to date" if none did. total is the number of
// documents in the catalog afterward.
func (r *reporter) outcome(total int) {
	var parts []string
	for _, a := range outcomeOrder {
		if n := r.counts[a]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, words[a]))
		}
	}
	if len(parts) == 0 {
		r.upToDate(total)
		return
	}
	r.printf("\ncatalog: %s — %s in .catalog.md\n", strings.Join(parts, ", "), plural(total, "document"))
}

func (r *reporter) upToDate(total int) {
	r.printf("catalog: up to date — %s.\n", plural(total, "document"))
}

// attention prints the end of a status report: either that everything is up
// to date, or what is out of sync and how to fix it. It returns whether
// anything needs attention. total is the number of documents enumerated.
func (r *reporter) attention(total int, drifted bool) bool {
	if r.waiting == 0 && !drifted {
		r.upToDate(total)
		return false
	}
	if drifted {
		r.printf(".catalog.md is out of sync with the database (run `catalog diff` to see what would change)\n")
	}
	if r.waiting == 0 {
		r.printf("\ncatalog: run `catalog update` to rewrite .catalog.md.\n")
		return true
	}
	need := "documents need"
	if r.waiting == 1 {
		need = "document needs"
	}
	r.printf("\ncatalog: %d %s attention — run `catalog update`.\n", r.waiting, need)
	return true
}

// cost prints what a run that called the model consumed, ending in an
// estimated dollar cost when the config gave prices. The estimate uses base
// prices, so when requests went over the config's warning size it adds a note
// that the real cost may be higher.
func (r *reporter) cost(calls int, t tokens, elapsed time.Duration, cfg *config) {
	in := withCommas(t.input) + " tokens in"
	if t.cached > 0 {
		in += fmt.Sprintf(" (%s cached)", withCommas(t.cached))
	}
	line := fmt.Sprintf("%s, %s, %s out, %s",
		plural(calls, "model call"), in, withCommas(t.output), elapsed.Round(time.Second))
	if p := cfg.prices(); p != nil {
		line += ", " + formatDollars(p.cost(t))
	}
	r.printf("%s\n", line)
	if t.long > 0 {
		r.printf("note: %s went over %s tokens; %s\n",
			fmt.Sprintf("%d of %s", t.long, plural(calls, "request")), withCommas(int64(cfg.WarnRequestTokens)), longNote(cfg))
	}
}

// longNote finishes the long-request note: what the size means for the cost.
func longNote(cfg *config) string {
	if cfg.prices() == nil {
		return "some models charge more for long requests."
	}
	return "the estimate uses base prices, so the real cost may be higher."
}

// formatBytes renders a size for people: "950 bytes", "400 KB", "1.2 MB".
func formatBytes(n int) string {
	switch {
	case n < 1_000:
		return plural(n, "byte")
	case n < 1_000_000:
		return fmt.Sprintf("%d KB", (n+999)/1_000) // round up so a document just over a limit never reads as equal to it
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/1_000_000)
	}
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func withCommas(n int64) string {
	s := strconv.FormatInt(n, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
