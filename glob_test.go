package main

import "testing"

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pat, path string
		want      bool
	}{
		{"leadership/**/*.md", "leadership/about.md", true},
		{"leadership/**/*.md", "leadership/sub/deep.md", true},
		{"leadership/**/*.md", "outreach/about.md", false},
		{"leadership/**/*.md", "leadership/notes.txt", false},
		{"**/README.md", "README.md", true},
		{"**/README.md", "operations/README.md", true},
		{"**/README.md", "operations/skills/x/README.md", true},
		{"**/README.md", "operations/notes.md", false},
		{"operations/skills/**/*", "operations/skills/markdown/SKILL.md", true},
		{"operations/skills/**/*", "operations/skills/a/b/c.txt", true},
		{"operations/skills/**/*", "operations/context-system.md", false},
		{"product/**/*.md", "product/offering.md", true},
		{"*.md", "MANIFEST.md", true},
		{"*.md", "leadership/about.md", false},
	}
	for _, c := range cases {
		if got := matchGlob(c.pat, c.path); got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pat, c.path, got, c.want)
		}
	}
}
