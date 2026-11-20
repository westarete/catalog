// Package render turns catalog entries into the text of catalog.md.
package render

import (
	"fmt"
	"sort"
	"strings"

	"westarete.com/catalog/internal/reader"
)

// Entry is one file in the catalog. An empty Summary means the file has no
// summary yet, so Render shows its Listing instead.
type Entry struct {
	Rel     string
	Summary string
	Listing reader.Listing
}

const header = `# Catalog

This is an index of the documents in this folder. Read the entries below, and open a file only when your task matches its entry.

Paths are relative to the folder that holds the .catalog/ folder.

An entry marked "Not summarized yet." has no summary. It shows only the start of the file.
`

// Render returns the full text of catalog.md: the header, then the entries
// grouped under "## <dir>/" headings and written as "### <rel>" sections.
func Render(entries []Entry) string {
	sorted := make([]Entry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool { return byDirectoryThenPath(sorted[i].Rel, sorted[j].Rel) })

	var b strings.Builder
	b.WriteString(header)
	lastDir := ""
	for _, e := range sorted {
		if dir := dirOf(e.Rel); dir != lastDir {
			fmt.Fprintf(&b, "\n## %s\n", dir)
			lastDir = dir
		}
		fmt.Fprintf(&b, "\n### %s\n\n", e.Rel)
		b.WriteString(body(e))
	}
	return b.String()
}

func body(e Entry) string {
	if s := strings.TrimSpace(e.Summary); s != "" {
		return s + "\n"
	}
	out := fmt.Sprintf("Not summarized yet. %s, %d bytes.\n", e.Listing.Type, e.Listing.Size)
	if e.Listing.FirstLines == "" {
		return out
	}
	fence := fenceFor(e.Listing.FirstLines)
	return out + "\n" + fence + "text\n" + strings.TrimRight(e.Listing.FirstLines, "\n") + "\n" + fence + "\n"
}

// fenceFor returns a backtick fence longer than any run of backticks in text,
// and at least three long, so text can't close the fence early.
func fenceFor(text string) string {
	longest, run := 0, 0
	for _, r := range text {
		if r == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", max(3, longest+1))
}

// byDirectoryThenPath orders paths so every file in a directory sorts
// together, which lets Render write each directory heading once. A plain
// string sort doesn't: "docs/guides/setup.md" falls between "docs/PLAN.md"
// and "docs/overview.md".
func byDirectoryThenPath(a, b string) bool {
	if dirA, dirB := dirOf(a), dirOf(b); dirA != dirB {
		return dirA < dirB
	}
	return a < b
}

func dirOf(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i] + "/"
	}
	return "(root)/"
}
