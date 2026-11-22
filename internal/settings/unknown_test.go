package settings

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestUnknownKeyGivesLineAndSuggestion(t *testing.T) {
	_, err := ParseCatalog([]byte("# a comment\n\nexlude = []\n"))
	want := "parse config.toml: line 3: unknown key; did you mean exclude?"
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestUnknownKeyNoSuggestionWhenFar(t *testing.T) {
	_, err := ParseCatalog([]byte("colour = 1"))
	want := "parse config.toml: line 1: unknown key"
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestUnknownKeySuggestsAtDistanceTwoOnly(t *testing.T) {
	cases := []struct{ in, want string }{
		{"inclde = []", "parse config.toml: line 1: unknown key; did you mean include?"},
		{"exclu = []", "parse config.toml: line 1: unknown key; did you mean exclude?"},
		{"xcld = []", "parse config.toml: line 1: unknown key"},
	}
	for _, c := range cases {
		_, err := ParseCatalog([]byte(c.in))
		if err == nil || err.Error() != c.want {
			t.Errorf("%q: error = %v, want %q", c.in, err, c.want)
		}
	}
}

func TestUnknownKeySuggestsNearestGlobalKey(t *testing.T) {
	_, err := ParseGlobal([]byte("modle = \"x\""))
	want := "parse global.toml: line 1: unknown key; did you mean model?"
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

// A person can paste a key as a bare TOML key name or leave an api_key in
// global.toml, and the error reaches the journal, the log, and catalog
// status, so it never quotes the key's text.
func TestUnknownKeyQuotesNothing(t *testing.T) {
	cases := []string{
		`sk-pasted-secret = "x"`,
		`api_key = "sk-secret"`,
		`"sk-quoted-secret" = "x"`,
		"model = \"m\"\nsk-ant-secret.nested = 1",
	}
	for _, in := range cases {
		_, err := ParseGlobal([]byte(in))
		if err == nil {
			t.Errorf("%q: want error", in)
			continue
		}
		if strings.Contains(err.Error(), "sk-") || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "api_key") {
			t.Errorf("%q: error quotes the key: %s", in, err)
		}
		if !strings.Contains(err.Error(), "unknown key") {
			t.Errorf("%q: error %q isn't an unknown key error", in, err)
		}
	}
}

func TestUnknownKeyInTable(t *testing.T) {
	_, err := ParseGlobal([]byte("model = \"m\"\n\n[watched]\nfolder = \"x\"\n"))
	want := "parse global.toml: line 3: unknown key; did you mean watch?"
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
	_, err = ParseCatalog([]byte("[[things]]\nx = 1\n"))
	want = "parse config.toml: line 1: unknown key"
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestUnknownKeyIgnoresCommentsAndValues(t *testing.T) {
	in := "# colour = 1\ninclude = [\"colour = 1\"]\ncolour = 2\n"
	_, err := ParseCatalog([]byte(in))
	want := "parse config.toml: line 3: unknown key"
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestUnknownKeyReportsFirstOnly(t *testing.T) {
	_, err := ParseCatalog([]byte("first = 1\nsecond = 2\n"))
	want := "parse config.toml: line 1: unknown key"
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestUnknownKeyDottedAndQuoted(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a.colour = 1", "parse config.toml: line 1: unknown key"},
		{"\n'colour' = 1", "parse config.toml: line 2: unknown key"},
		{"\n  \"colour\"   = 1", "parse config.toml: line 2: unknown key"},
	}
	for _, c := range cases {
		_, err := ParseCatalog([]byte(c.in))
		if err == nil || err.Error() != c.want {
			t.Errorf("%q: error = %v, want %q", c.in, err, c.want)
		}
	}
}

// If the key's line can't be found, the error has no line and still no key.
func TestUnknownKeyWithoutLine(t *testing.T) {
	err := unknownKeyError("config.toml", []byte("nothing to see\n"), undecodedKey(t, "colour = 1"), catalogKeys)
	want := "parse config.toml: unknown key"
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestUnknownKeyNilWhenAllDecoded(t *testing.T) {
	if _, err := ParseCatalog([]byte("include = []\nexclude = []\n")); err != nil {
		t.Error(err)
	}
	if _, err := ParseGlobal([]byte("watch = []\nmodel = \"m\"\nkey = \"k\"\ninclude = []\nexclude = []\n")); err != nil {
		t.Error(err)
	}
}

func TestUnknownKeyRejectsWholeFile(t *testing.T) {
	g, err := ParseGlobal([]byte("model = \"other\"\ncolour = 1\n"))
	if err == nil {
		t.Fatal("want error")
	}
	if g.Model != "" || g.Watch != nil || g.Include != nil {
		t.Errorf("settings = %+v, want none", g)
	}
	c, err := ParseCatalog([]byte("include = [\"a\"]\ncolour = 1\n"))
	if err == nil {
		t.Fatal("want error")
	}
	if c.Include != nil {
		t.Errorf("catalog = %+v, want none", c)
	}
}

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0}, {"abc", "abc", 0}, {"", "abc", 3}, {"abc", "", 3},
		{"kitten", "sitting", 3}, {"exlude", "exclude", 1}, {"ab", "ba", 2},
	}
	for _, c := range cases {
		if got := levenshtein(c.a, c.b); got != c.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// undecodedKey decodes text into a catalog and returns the metadata, which
// lists the keys Catalog doesn't read.
func undecodedKey(t *testing.T, text string) toml.MetaData {
	t.Helper()
	var c Catalog
	md, err := toml.Decode(text, &c)
	if err != nil {
		t.Fatal(err)
	}
	return md
}

func TestUnknownKeySkipsLinesInsideAnArray(t *testing.T) {
	in := "include = [\n  \"a\",\n  \"colour\",\n]\ncolour = 1\n"
	_, err := ParseCatalog([]byte(in))
	want := "parse config.toml: line 5: unknown key"
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}
