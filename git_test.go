package main

import (
	"reflect"
	"sort"
	"testing"
)

func keys(m map[string]bool) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func TestComputeStaleNoRef(t *testing.T) {
	// No prior catalog commit → everything is stale.
	docs := []string{"a.md", "b.md"}
	got := computeStale("", docs, "", "")
	if !reflect.DeepEqual(keys(got), docs) {
		t.Errorf("no-ref stale = %v, want all %v", keys(got), docs)
	}
}

func TestComputeStaleCommittedAndWorking(t *testing.T) {
	docs := []string{"a.md", "b.md", "c.md"}
	// a.md changed in a commit since ref; c.md has a working-tree edit; b.md is
	// untouched. A path not in docs (z.md) must be ignored.
	committed := "a.md\nz.md\n"
	working := " M c.md\n?? scratch.txt\n"
	got := computeStale("abc123", docs, committed, working)
	want := []string{"a.md", "c.md"}
	if !reflect.DeepEqual(keys(got), want) {
		t.Errorf("stale = %v, want %v", keys(got), want)
	}
}

func TestParsePorcelainPaths(t *testing.T) {
	status := " M leadership/about.md\n" +
		"A  product/new.md\n" +
		"?? untracked.md\n" +
		"R  old/path.md -> new/path.md\n" +
		"\n"
	got := parsePorcelainPaths(status)
	want := []string{
		"leadership/about.md",
		"product/new.md",
		"untracked.md",
		"new/path.md", // rename → destination
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parsePorcelainPaths = %v, want %v", got, want)
	}
}

func TestStaleDocsWithFakeRunner(t *testing.T) {
	// staleDocs wires the git calls to computeStale; verify the wiring with a
	// fake runner so no real repo is needed.
	run := func(args ...string) (string, error) {
		switch args[0] {
		case "diff":
			return "a.md", nil
		case "status":
			return " M b.md", nil
		}
		return "", nil
	}
	got, err := staleDocs(run, "ref", []string{"a.md", "b.md", "c.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(keys(got), []string{"a.md", "b.md"}) {
		t.Errorf("staleDocs = %v, want [a.md b.md]", keys(got))
	}
}
