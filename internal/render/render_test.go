package render

import (
	"strings"
	"testing"

	"westarete.com/catalog/internal/reader"
)

func TestRenderGroups(t *testing.T) {
	out := Render([]Entry{
		{Rel: "b/x.md", Summary: "X."},
		{Rel: "a.md", Summary: "A."},
		{Rel: "b/y.md", Summary: "Y."},
	})
	want := []string{"## (root)/", "### a.md", "A.", "## b/", "### b/x.md", "X.", "### b/y.md", "Y."}
	pos := 0
	for _, w := range want {
		i := strings.Index(out[pos:], "\n"+w+"\n")
		if i < 0 {
			t.Fatalf("missing or out of order %q in:\n%s", w, out)
		}
		pos += i + len(w)
	}
	for _, h := range []string{"## (root)/", "## b/", "### a.md", "### b/x.md", "### b/y.md"} {
		if strings.Count(out, "\n"+h+"\n") != 1 {
			t.Errorf("%q should appear once", h)
		}
	}
}

func TestRenderSortsDirectoriesTogether(t *testing.T) {
	out := Render([]Entry{
		{Rel: "docs/overview.md", Summary: "o"},
		{Rel: "docs/guides/setup.md", Summary: "s"},
		{Rel: "docs/PLAN.md", Summary: "p"},
	})
	if strings.Count(out, "## docs/\n") != 1 {
		t.Errorf("docs/ heading should appear once:\n%s", out)
	}
	if strings.Index(out, "### docs/PLAN.md") > strings.Index(out, "### docs/overview.md") {
		t.Errorf("PLAN.md should precede overview.md:\n%s", out)
	}
	if strings.Index(out, "## docs/\n") > strings.Index(out, "## docs/guides/\n") {
		t.Errorf("docs/ should precede docs/guides/:\n%s", out)
	}
}

func TestRenderSummaryBody(t *testing.T) {
	out := Render([]Entry{{Rel: "a.md", Summary: "  Plans the launch.\n"}})
	if !strings.Contains(out, "### a.md\n\nPlans the launch.\n") {
		t.Errorf("summary not rendered trimmed:\n%s", out)
	}
	if strings.Contains(out, "Not summarized yet. ") {
		t.Errorf("summarized entry marked unsummarized")
	}
}

func TestRenderListingOnly(t *testing.T) {
	out := Render([]Entry{{Rel: "a.csv", Listing: reader.Listing{Type: "text", Size: 42, FirstLines: "id,name\n1,x"}}})
	want := "### a.csv\n\nNot summarized yet. text, 42 bytes.\n\n```text\nid,name\n1,x\n```\n"
	if !strings.Contains(out, want) {
		t.Errorf("want %q in:\n%s", want, out)
	}
}

func TestRenderListingOnlyNoFirstLines(t *testing.T) {
	out := Render([]Entry{{Rel: "a.pdf", Listing: reader.Listing{Type: "pdf", Size: 9}}})
	if !strings.HasSuffix(out, "### a.pdf\n\nNot summarized yet. pdf, 9 bytes.\n") {
		t.Errorf("unexpected body:\n%s", out)
	}
}

func TestRenderFenceCannotCloseEarly(t *testing.T) {
	out := Render([]Entry{{Rel: "a.md", Listing: reader.Listing{Type: "text", Size: 1, FirstLines: "```go\nx\n````\ny"}}})
	if !strings.Contains(out, "\n`````text\n```go\nx\n````\ny\n`````\n") {
		t.Errorf("fence not lengthened:\n%s", out)
	}
}

func TestRenderFenceMinimumThree(t *testing.T) {
	out := Render([]Entry{{Rel: "a.md", Listing: reader.Listing{Type: "text", Size: 1, FirstLines: "a `b` ``c``"}}})
	if !strings.Contains(out, "\n```text\n") {
		t.Errorf("want three-backtick fence:\n%s", out)
	}
}

func TestRenderDoesNotReorderInput(t *testing.T) {
	in := []Entry{{Rel: "b.md", Summary: "b"}, {Rel: "a.md", Summary: "a"}}
	Render(in)
	if in[0].Rel != "b.md" {
		t.Errorf("input reordered")
	}
}

func TestHeaderNoCommands(t *testing.T) {
	out := Render(nil)
	for _, bad := range []string{"catalog update", "catalog status"} {
		if strings.Contains(out, bad) {
			t.Errorf("header mentions %q", bad)
		}
	}
	if !strings.HasPrefix(out, "# Catalog\n") || !strings.Contains(out, "Not summarized yet.") {
		t.Errorf("header incomplete:\n%s", out)
	}
	if !strings.HasSuffix(out, "\n") || strings.HasSuffix(out, "\n\n") {
		t.Errorf("want exactly one trailing newline: %q", out)
	}
}

func TestDirOfLeadingSlash(t *testing.T) {
	if got := dirOf("/x.md"); got != "/" {
		t.Errorf("dirOf(/x.md) = %q, want /", got)
	}
}
