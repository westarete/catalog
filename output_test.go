package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestReporterMixedRun(t *testing.T) {
	var buf bytes.Buffer
	r := newReporter(&buf)
	r.startProfiling(2)
	r.done(actionDelete, "JOB-glossary.md", "")
	r.profiled(actionAdd, "research/JOB-list-of-research-software.md", "")
	r.profiled(actionAdd, "research/list-of-research-software.md", "")
	r.outcome(50)
	r.cost(2, tokens{input: 47198, output: 1287}, 8*time.Second, &config{})

	want := `      deleted   JOB-glossary.md
[1/2] added     research/JOB-list-of-research-software.md
[2/2] added     research/list-of-research-software.md

catalog: 2 added, 1 deleted — 50 documents in .catalog.md
2 model calls, 47,198 tokens in, 1,287 out, 8s
`
	if got := buf.String(); got != want {
		t.Errorf("output:\n%s\nwant:\n%s", got, want)
	}
}

func TestReporterEachPathHasItsOwnVerb(t *testing.T) {
	var buf bytes.Buffer
	r := newReporter(&buf)
	r.done(actionDelete, "gone.md", "")
	r.profiled(actionAdd, "new.md", "")
	r.profiled(actionModify, "changed.md", "")

	verbs := map[string]string{"gone.md": "deleted", "new.md": "added", "changed.md": "modified"}
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		fields := strings.Fields(line)
		if want := verbs[fields[1]]; fields[0] != want {
			t.Errorf("line %q: verb %q, want %q", line, fields[0], want)
		}
	}
}

func TestReporterNothingToDo(t *testing.T) {
	var buf bytes.Buffer
	newReporter(&buf).outcome(52)
	if got, want := buf.String(), "catalog: up to date — 52 documents.\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReporterSingularDocument(t *testing.T) {
	var buf bytes.Buffer
	newReporter(&buf).outcome(1)
	if got, want := buf.String(), "catalog: up to date — 1 document.\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReporterFailureAndNotes(t *testing.T) {
	var buf bytes.Buffer
	r := newReporter(&buf)
	r.profiled(actionAdd, "notes/kickoff.md", "")
	r.profiled(actionFail, "reports/q3.pdf", "model stopped before writing a profile")
	r.profiled(actionModify, "big.md", "truncated: profiled the first 200 KB of 1.2 MB")
	r.outcome(52)
	r.cost(3, tokens{input: 61004, cached: 500, output: 2110}, 14*time.Second, &config{})

	want := `added     notes/kickoff.md
failed    reports/q3.pdf (model stopped before writing a profile)
modified  big.md (truncated: profiled the first 200 KB of 1.2 MB)

catalog: 1 added, 1 modified, 1 failed — 52 documents in .catalog.md
3 model calls, 61,004 tokens in (500 cached), 2,110 out, 14s
`
	if got := buf.String(); got != want {
		t.Errorf("output:\n%s\nwant:\n%s", got, want)
	}
}

func TestReporterPendingUsesTheSameWords(t *testing.T) {
	var buf bytes.Buffer
	r := newReporter(&buf)
	r.pending(actionAdd, "a.md")
	r.pending(actionModify, "b.md")
	r.pending(actionDelete, "c.md")

	want := "added     a.md\nmodified  b.md\ndeleted   c.md\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReporterCounterAlwaysShowsEvenForOneDocument(t *testing.T) {
	var one bytes.Buffer
	single := newReporter(&one)
	single.startProfiling(1)
	single.profiled(actionModify, "a.md", "")
	if got, want := one.String(), "[1/1] modified  a.md\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	var buf bytes.Buffer
	r := newReporter(&buf)
	r.startProfiling(120)
	r.done(actionDelete, "gone.md", "")
	r.profiled(actionAdd, "a.md", "")
	r.profiled(actionAdd, "b.md", "")

	want := "          deleted   gone.md\n[  1/120] added     a.md\n[  2/120] added     b.md\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWithCommas(t *testing.T) {
	for n, want := range map[int64]string{0: "0", 999: "999", 1000: "1,000", 47198: "47,198", 1204311: "1,204,311"} {
		if got := withCommas(n); got != want {
			t.Errorf("withCommas(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestCostRoundsElapsedAndHandlesLongRuns(t *testing.T) {
	var buf bytes.Buffer
	newReporter(&buf).cost(1, tokens{input: 5, output: 7}, 6*time.Minute+12*time.Second+400*time.Millisecond, &config{})
	want := fmt.Sprintf("1 model call, 5 tokens in, 7 out, %s\n", "6m12s")
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatBytes(t *testing.T) {
	for n, want := range map[int]string{
		1:         "1 byte",
		950:       "950 bytes",
		400_000:   "400 KB",
		400_001:   "401 KB",
		1_200_000: "1.2 MB",
	} {
		if got := formatBytes(n); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

// A two-pass run prints every document twice but changed it once.
func TestReporterCountsEachDocumentOnce(t *testing.T) {
	var buf bytes.Buffer
	r := newReporter(&buf)
	r.heading("pass 1 of 2: first drafts")
	r.profiled(actionAdd, "a.md", "")
	r.heading("pass 2 of 2: sharpening against the full catalog")
	r.profiled(actionModify, "a.md", "")
	r.outcome(1)

	want := `pass 1 of 2: first drafts
added     a.md

pass 2 of 2: sharpening against the full catalog
modified  a.md

catalog: 1 added — 1 document in .catalog.md
`
	if got := buf.String(); got != want {
		t.Errorf("output:\n%s\nwant:\n%s", got, want)
	}
}
