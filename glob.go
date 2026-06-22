package main

import (
	"path/filepath"
	"strings"
)

// matchGlob reports whether path matches a glob pattern that may contain ** as
// a path-spanning wildcard. Each path segment is matched with filepath.Match
// (so * and ? work within a segment); a ** segment matches zero or more
// segments. This is the subset of glob the catalog configs use —
// "leadership/**/*.md", "**/README.md", "operations/skills/**/*".
func matchGlob(pattern, path string) bool {
	return matchSegments(
		strings.Split(pattern, "/"),
		strings.Split(path, "/"),
	)
}

func matchSegments(pat, name []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			// Collapse consecutive **; a trailing ** matches anything.
			rest := pat[1:]
			if len(rest) == 0 {
				return true
			}
			// Try to match the remaining pattern at every suffix of name.
			for i := 0; i <= len(name); i++ {
				if matchSegments(rest, name[i:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		ok, err := filepath.Match(pat[0], name[0])
		if err != nil || !ok {
			return false
		}
		pat = pat[1:]
		name = name[1:]
	}
	return len(name) == 0
}
