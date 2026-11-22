package settings

import (
	"fmt"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// maxSuggestDistance is the largest edit distance at which a known key is
// offered as the likely intended one.
const maxSuggestDistance = 2

// unknownKeyError returns an error for the first key in md that the decoder
// didn't read, or nil if it read them all. known holds the keys the file
// may define. The error gives the key's line and, if a known key is close,
// that key's name. It never quotes the unknown key, because a person can
// paste a secret as a bare TOML key, and the error reaches the journal, the
// log, and catalog status.
func unknownKeyError(file string, data []byte, md toml.MetaData, known map[string]bool) error {
	undecoded := md.Undecoded()
	if len(undecoded) == 0 {
		return nil
	}
	name := undecoded[0][len(undecoded[0])-1]
	msg := "unknown key"
	if line := keyLine(data, name); line > 0 {
		msg = fmt.Sprintf("line %d: %s", line, msg)
	}
	if suggestion := nearestKey(name, known); suggestion != "" {
		msg += "; did you mean " + suggestion + "?"
	}
	return fmt.Errorf("parse %s: %s", file, msg)
}

// keyLine returns the number of the first line of data that defines a key or
// table whose last part is name, or 0 if there is none. Comment lines don't
// count.
func keyLine(data []byte, name string) int {
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		var key string
		switch {
		case strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "["):
			key, _, _ = strings.Cut(line, "]")
			key = strings.TrimLeft(key, "[")
		default:
			var found bool
			key, _, found = strings.Cut(line, "=")
			if !found {
				continue
			}
		}
		parts := strings.Split(key, ".")
		last := strings.Trim(strings.TrimSpace(parts[len(parts)-1]), `"'`)
		if last == name {
			return i + 1
		}
	}
	return 0
}

// nearestKey returns the key in known closest to name, or "" if none is
// within maxSuggestDistance. Among equally close keys it returns the first
// in alphabetical order, so the message doesn't vary between runs.
func nearestKey(name string, known map[string]bool) string {
	keys := make([]string, 0, len(known))
	for k := range known {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	best, bestDistance := "", maxSuggestDistance+1
	for _, k := range keys {
		if d := levenshtein(name, k); d < bestDistance {
			best, bestDistance = k, d
		}
	}
	return best
}

// levenshtein returns the number of single-character edits that turn a into b.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}
