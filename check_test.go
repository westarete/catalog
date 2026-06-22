package main

import (
	"reflect"
	"testing"
)

func TestCheckResult(t *testing.T) {
	docs := []string{"a.md", "b.md", "c.md"}
	entries := entrySet("a.md", "b.md", "gone.md") // c.md missing; gone.md orphan
	stale := map[string]bool{"b.md": true}         // b.md changed

	r := checkResult(docs, entries, stale)
	if r.ok() {
		t.Fatal("expected not ok")
	}
	if !reflect.DeepEqual(r.missing, []string{"c.md"}) {
		t.Errorf("missing = %v, want [c.md]", r.missing)
	}
	if !reflect.DeepEqual(r.changed, []string{"b.md"}) {
		t.Errorf("changed = %v, want [b.md]", r.changed)
	}
	if !reflect.DeepEqual(r.orphans, []string{"gone.md"}) {
		t.Errorf("orphans = %v, want [gone.md]", r.orphans)
	}
}

func TestCheckResultClean(t *testing.T) {
	docs := []string{"a.md", "b.md"}
	r := checkResult(docs, entrySet("a.md", "b.md"), map[string]bool{})
	if !r.ok() {
		t.Errorf("expected ok, got %+v", r)
	}
}
