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
	// The header does not round-trip: render always emits defaultHeader(),
	// since the store holds no header field — see render's doc comment.
	out := render(entriesToRows(c.entries))
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

func TestRenderFromRows(t *testing.T) {
	rows := []profileRow{
		{path: "b/two.md", profile: "profile two"},
		{path: "a/one.md", profile: "profile one"},
	}
	out := render(rows)
	if !strings.Contains(out, "### a/one.md") || !strings.Contains(out, "profile one") {
		t.Errorf("a/one.md entry missing:\n%s", out)
	}
	if !strings.Contains(out, "### b/two.md") || !strings.Contains(out, "profile two") {
		t.Errorf("b/two.md entry missing:\n%s", out)
	}
	if strings.Index(out, "a/one.md") > strings.Index(out, "b/two.md") {
		t.Errorf("entries not sorted by path:\n%s", out)
	}
}

func TestRenderAlwaysUsesDefaultHeader(t *testing.T) {
	out := render(nil)
	if !strings.Contains(out, strings.TrimSpace(defaultHeader())) {
		t.Errorf("render output missing default header:\n%s", out)
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
	out := render(entriesToRows(c.entries))
	if !strings.Contains(out, "REWRITTEN one") {
		t.Errorf("rewrite not applied")
	}
	if !strings.Contains(out, "profile two") {
		t.Errorf("untouched neighbor lost: %s", out)
	}
}
