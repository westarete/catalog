package main

import (
	"strings"
	"testing"
)

func TestParseRoundTrip(t *testing.T) {
	src := strings.Join([]string{
		"# Catalog",
		"",
		"Some header prose that must survive untouched.",
		"",
		"## leadership/",
		"",
		"### leadership/about.md",
		"",
		"When you need basic orientation on the firm.",
		"",
		"### leadership/strategy.md",
		"",
		"When you need the top-level strategic logic.",
		"",
		"## product/",
		"",
		"### product/offering.md",
		"",
		"When you need what the firm sells.",
		"",
	}, "\n")

	c := parseCatalog(src)
	if len(c.entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(c.entries))
	}
	if got := c.entries["leadership/about.md"].profile; got != "When you need basic orientation on the firm." {
		t.Errorf("about profile = %q", got)
	}
	if !strings.Contains(c.header, "must survive untouched") {
		t.Errorf("header lost: %q", c.header)
	}

	// Rendering then re-parsing must preserve the entry set and profiles.
	out := render(c)
	c2 := parseCatalog(out)
	if len(c2.entries) != 3 {
		t.Fatalf("round-trip got %d entries, want 3", len(c2.entries))
	}
	for path, e := range c.entries {
		if c2.entries[path] == nil || c2.entries[path].profile != e.profile {
			t.Errorf("round-trip lost entry %q", path)
		}
	}
	// Sections must be present and ordered.
	if !strings.Contains(out, "## leadership/") || !strings.Contains(out, "## product/") {
		t.Errorf("section headers missing:\n%s", out)
	}
}

func TestParseEmpty(t *testing.T) {
	c := parseCatalog("")
	if len(c.entries) != 0 {
		t.Errorf("empty catalog should have no entries")
	}
}

func TestInPlaceRewritePreservesOthers(t *testing.T) {
	src := strings.Join([]string{
		"# Catalog", "", "Header.", "",
		"## a/", "", "### a/one.md", "", "profile one", "",
		"### a/two.md", "", "profile two", "",
	}, "\n")
	c := parseCatalog(src)
	c.entries["a/one.md"].profile = "REWRITTEN one"
	out := render(c)
	if !strings.Contains(out, "REWRITTEN one") {
		t.Errorf("rewrite not applied")
	}
	if !strings.Contains(out, "profile two") {
		t.Errorf("untouched neighbor lost: %s", out)
	}
}
