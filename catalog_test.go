package main

import (
	"strings"
	"testing"
)

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

func TestRenderGroupsByDirectory(t *testing.T) {
	rows := []profileRow{
		{path: "leadership/about.md", profile: "p1"},
		{path: "leadership/strategy.md", profile: "p2"},
		{path: "product/offering.md", profile: "p3"},
	}
	out := render(rows)
	if !strings.Contains(out, "## leadership/") || !strings.Contains(out, "## product/") {
		t.Errorf("section headers missing:\n%s", out)
	}
}

func TestRenderWritesEachDirectoryHeadingOnce(t *testing.T) {
	rows := []profileRow{
		{path: "docs/PLAN.md", profile: "plan"},
		{path: "docs/guides/setup.md", profile: "setup"},
		{path: "docs/overview.md", profile: "overview"},
	}
	out := render(rows)
	for _, heading := range []string{"\n## docs/\n", "\n## docs/guides/\n"} {
		if n := strings.Count(out, heading); n != 1 {
			t.Errorf("heading %q appears %d times, want 1:\n%s", strings.TrimSpace(heading), n, out)
		}
	}
	want := []string{
		"## docs/",
		"### docs/PLAN.md",
		"### docs/overview.md",
		"## docs/guides/",
		"### docs/guides/setup.md",
	}
	last := -1
	for _, line := range want {
		i := strings.Index(out, "\n"+line+"\n")
		if i <= last {
			t.Errorf("%q is missing or out of order:\n%s", line, out)
		}
		last = i
	}
}
