package trees

import (
	"path"
	"strings"
)

// Match reports whether rel, a slash-separated relative path, matches a glob
// pattern that may contain ** as a path-spanning wildcard. Each path segment
// is matched with path.Match (so * and ? work within a segment); a **
// segment matches zero or more segments. This is the subset of glob the
// catalog configs use: "leadership/**/*.md", "**/README.md",
// "operations/skills/**/*".
func Match(pattern, rel string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(rel, "/"))
}

func matchSegments(pat, name []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			return matchAfterDoubleStar(pat[1:], name)
		}
		if len(name) == 0 || !matchSegment(pat[0], name[0]) {
			return false
		}
		pat, name = pat[1:], name[1:]
	}
	return len(name) == 0
}

// matchAfterDoubleStar tries the rest of the pattern at every suffix of name.
// A trailing ** matches anything.
func matchAfterDoubleStar(rest, name []string) bool {
	if len(rest) == 0 {
		return true
	}
	for i := 0; i <= len(name); i++ {
		if matchSegments(rest, name[i:]) {
			return true
		}
	}
	return false
}

func matchSegment(pattern, name string) bool {
	ok, err := path.Match(pattern, name)
	return err == nil && ok
}
