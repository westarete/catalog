package main

import (
	"os"
	"sort"
	"strings"
)

// catalogPath is the agent-facing artifact, kept at the repo root: it is the
// path anchor (entries resolve relative to this file's directory, so a bare
// "leadership/about.md" is correct in hq and resolves to "hq/leadership/..."
// from a repo that consumes hq as a subtree), and it is a maintained file whose
// visibility is the reminder to keep it current.
const catalogPath = ".catalog.md"

// An entry is one document's profile: the bare path that heads it, and the
// profile prose an agent reasons over to decide whether to load the document.
type entry struct {
	path    string
	profile string
}

// catalog holds the parsed entries plus the verbatim header (everything before
// the first directory section), so generate can rewrite individual profiles
// without disturbing the header or hand edits elsewhere.
type catalog struct {
	header  string
	entries map[string]*entry
}

// readCatalog parses .catalog.md. A missing file yields an empty catalog with
// the default header — the first-generation case.
func readCatalog() (*catalog, error) {
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &catalog{header: defaultHeader(), entries: map[string]*entry{}}, nil
		}
		return nil, err
	}
	return parseCatalog(string(data)), nil
}

// parseCatalog splits the document on "### " headings. Each such heading's
// text is a document path (a stable, greppable key); the lines until the next
// "##"/"###" heading are its profile. Everything before the first "## " section
// header is preserved verbatim as the header.
func parseCatalog(text string) *catalog {
	c := &catalog{entries: map[string]*entry{}}
	lines := strings.Split(text, "\n")

	var header []string
	headerDone := false
	var curPath string
	var curBody []string

	flush := func() {
		if curPath != "" {
			c.entries[curPath] = &entry{
				path:    curPath,
				profile: strings.TrimSpace(strings.Join(curBody, "\n")),
			}
		}
		curPath, curBody = "", nil
	}

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "### "):
			headerDone = true
			flush()
			curPath = strings.TrimSpace(strings.TrimPrefix(line, "### "))
		case strings.HasPrefix(line, "## "):
			// A directory section header: ends the previous entry, but the
			// section title itself is regenerated on render, so drop it.
			headerDone = true
			flush()
		default:
			if curPath != "" {
				curBody = append(curBody, line)
			} else if !headerDone {
				header = append(header, line)
			}
		}
	}
	flush()
	c.header = strings.TrimRight(strings.Join(header, "\n"), "\n") + "\n"
	return c
}

// render produces the full .catalog.md from the store's rows: the header,
// then entries grouped under "## <dir>/" section headers and emitted as
// "### <path>" stanzas. Profiles are written as single-line paragraphs and
// left for bin/format (Prettier) to wrap, the same discipline as every other
// Markdown file in the repo.
//
// The header is always defaultHeader() — the store holds no header field, so
// there is nothing per-repo to preserve. The header is generic boilerplate
// describing what the file is, not project-specific prose anyone hand-tunes.
func render(rows []profileRow) string {
	var b strings.Builder
	b.WriteString(strings.TrimRight(defaultHeader(), "\n"))
	b.WriteString("\n")

	sorted := make([]profileRow, len(rows))
	copy(sorted, rows)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].path < sorted[j].path })

	lastDir := ""
	for _, r := range sorted {
		dir := dirOf(r.path)
		if dir != lastDir {
			b.WriteString("\n## ")
			b.WriteString(dir)
			b.WriteString("\n")
			lastDir = dir
		}
		b.WriteString("\n### ")
		b.WriteString(r.path)
		b.WriteString("\n\n")
		b.WriteString(strings.TrimSpace(r.profile))
		b.WriteString("\n")
	}
	return b.String()
}

func dirOf(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i] + "/"
	}
	return "(root)"
}

// entriesToRows is a bridge kept only until bootstrap/update/force are
// rewired against the store directly (see TODO.md): it lets check.go and
// generate.go keep compiling against *catalog while render itself already
// takes the store's row shape.
func entriesToRows(entries map[string]*entry) []profileRow {
	rows := make([]profileRow, 0, len(entries))
	for path, e := range entries {
		rows = append(rows, profileRow{path: path, profile: e.profile})
	}
	return rows
}

func writeCatalog(c *catalog) error {
	return os.WriteFile(catalogPath, []byte(render(entriesToRows(c.entries))), 0o644)
}

func defaultHeader() string {
	return strings.TrimSpace(`
# Catalog

This is the index of the durable content in this repo. Each entry below is a
profile: read it at the start of a conversation, then load a document only when
your task matches its profile — you don't need to read the others.

Paths are relative to this file's own location. In this repo the catalog sits
at the root, so `+"`leadership/about.md`"+` is correct as written. In a repo
that consumes this one as a subtree, the catalog sits in a subdirectory, so the
same entry resolves relative to that subdirectory.

Generated by `+"`catalog update`"+` from the source documents. Run
`+"`catalog check`"+` to find entries that have gone stale.
`) + "\n"
}
