package main

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)



func TestTokensAdd(t *testing.T) {
	got := tokens{input: 1, cached: 2, output: 3}.add(tokens{input: 10, cached: 20, output: 30})
	if got != (tokens{input: 11, cached: 22, output: 33}) {
		t.Errorf("add = %+v", got)
	}
}

func TestFormatRunSummary(t *testing.T) {
	s := formatRunSummary(20, 2, tokens{input: 1000, cached: 500, output: 200}, 90*time.Second)
	for _, want := range []string{"20 docs", "2 pass", "40 calls", "1000 input", "500 cached", "200 output", "90s"} {
		if !strings.Contains(s, want) {
			t.Errorf("summary missing %q: %s", want, s)
		}
	}
}

func entrySet(paths ...string) map[string]*entry {
	m := map[string]*entry{}
	for _, p := range paths {
		m[p] = &entry{path: p, profile: "p"}
	}
	return m
}

func TestSelectStale(t *testing.T) {
	docs := []string{"a.md", "b.md", "c.md"}
	entries := entrySet("a.md", "b.md") // c.md has no entry yet
	stale := map[string]bool{"a.md": true}
	got := selectStale(docs, entries, stale)
	// a.md is stale; c.md is missing an entry; b.md is current → skipped.
	want := []string{"a.md", "c.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stale targets = %v, want %v", got, want)
	}
}

func TestSelectStaleNothingToDo(t *testing.T) {
	docs := []string{"a.md", "b.md"}
	got := selectStale(docs, entrySet("a.md", "b.md"), map[string]bool{})
	if len(got) != 0 {
		t.Errorf("expected no targets, got %v", got)
	}
}

func TestRequirePopulated(t *testing.T) {
	// Empty or missing catalog: refuse and point at bootstrap.
	err := requirePopulated(&catalog{entries: map[string]*entry{}})
	if err == nil || !strings.Contains(err.Error(), "bootstrap") {
		t.Fatalf("want error pointing at bootstrap, got %v", err)
	}
	// A populated catalog passes.
	if err := requirePopulated(&catalog{entries: entrySet("a.md")}); err != nil {
		t.Fatalf("populated catalog should pass, got %v", err)
	}
}

func TestResolveForceTargets(t *testing.T) {
	docs := []string{"a.md", "b.md", "c.md"}

	// Named files come back in docs order, not the order they were passed.
	got, err := resolveForceTargets([]string{"c.md", "a.md"}, docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"a.md", "c.md"}) {
		t.Errorf("targets = %v, want [a.md c.md]", got)
	}

	// A name that isn't enumerated is an error naming the config, not a no-op.
	_, err = resolveForceTargets([]string{"a.md", "typo.md"}, docs)
	if err == nil || !strings.Contains(err.Error(), configPath) {
		t.Fatalf("want error naming %s, got %v", configPath, err)
	}
}

func TestBuildProfilePrompt(t *testing.T) {
	// No neighbors: just the document tag, no positioning section.
	solo := buildProfilePrompt("a.md", "body text", "")
	if !strings.Contains(solo, `<document path="a.md">`) || !strings.Contains(solo, "body text") {
		t.Errorf("missing document tag/body:\n%s", solo)
	}
	if strings.Contains(solo, "Existing profiles") {
		t.Errorf("solo prompt should have no neighbor section:\n%s", solo)
	}
	// With neighbors: positioning section appended.
	withN := buildProfilePrompt("a.md", "body", "- b.md: when X\n")
	if !strings.Contains(withN, "Existing profiles for other documents") {
		t.Errorf("missing neighbor section:\n%s", withN)
	}
	if !strings.Contains(withN, "- b.md: when X") {
		t.Errorf("missing neighbor line:\n%s", withN)
	}
}

func TestNeighborsExcludesSelfAndSorts(t *testing.T) {
	c := &catalog{entries: map[string]*entry{
		"c.md": {path: "c.md", profile: "pc"},
		"a.md": {path: "a.md", profile: "pa"},
		"b.md": {path: "b.md", profile: "pb"},
	}}
	got := neighbors(c, "b.md")
	want := "- a.md: pa\n- c.md: pc\n"
	if got != want {
		t.Errorf("neighbors = %q, want %q", got, want)
	}
}

func TestPruneOrphans(t *testing.T) {
	entries := entrySet("a.md", "gone.md", "b.md")
	pruneOrphans(entries, []string{"a.md", "b.md"})
	var got []string
	for p := range entries {
		got = append(got, p)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, []string{"a.md", "b.md"}) {
		t.Errorf("after prune = %v, want [a.md b.md]", got)
	}
}
