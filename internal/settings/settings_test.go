package settings

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestParseGlobalDefaults(t *testing.T) {
	g, err := ParseGlobal([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if g.Model != "claude-haiku-5-5" {
		t.Errorf("Model = %q", g.Model)
	}
	if len(g.Include) != 0 || len(g.Exclude) != 0 || len(g.Watch) != 0 {
		t.Errorf("empty global.toml set lists: %+v", g)
	}
}

func TestParseGlobalKeepsSetValues(t *testing.T) {
	g, err := ParseGlobal([]byte(`
key = "acme"
model = "other-model"
include = ["a"]
exclude = ["b"]
`))
	if err != nil {
		t.Fatal(err)
	}
	if g.Key != "acme" || g.KeyName() != "acme" || g.Model != "other-model" {
		t.Errorf("got %+v", g)
	}
	if !reflect.DeepEqual(g.Include, []string{"a"}) || !reflect.DeepEqual(g.Exclude, []string{"b"}) {
		t.Errorf("Include = %v, Exclude = %v", g.Include, g.Exclude)
	}
}

func TestParseGlobalWatchList(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	g, err := ParseGlobal([]byte(`watch = ["~/a", "/b", "~", "~other/x"]`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(home, "a"), "/b", home, "~other/x"}
	if !reflect.DeepEqual(g.Watch, want) {
		t.Errorf("Watch = %v, want %v", g.Watch, want)
	}
}

// With no key, or an empty one, the daemon uses the key named default.
func TestKeyNameDefaults(t *testing.T) {
	for _, text := range []string{"", `key = ""`} {
		g, err := ParseGlobal([]byte(text))
		if err != nil {
			t.Fatal(err)
		}
		if got := g.KeyName(); got != DefaultKeyName {
			t.Errorf("%q: KeyName = %q, want %q", text, got, DefaultKeyName)
		}
	}
}

// key is a name global.toml defines, so an error may name it.
func TestParseGlobalErrorNamesKey(t *testing.T) {
	_, err := ParseGlobal([]byte("key = 5"))
	if err == nil || !strings.Contains(err.Error(), `last key "key"`) {
		t.Errorf("err = %v; want one naming key", err)
	}
}

func TestParseGlobalMalformed(t *testing.T) {
	if _, err := ParseGlobal([]byte("model = ")); err == nil {
		t.Error("want error")
	}
}

// A person may paste an API key into global.toml, and a parse error reaches
// the journal, the log, and catalog status, so the error must name where the
// problem is without quoting any of the file's values.
func TestParseGlobalErrorQuotesNoValue(t *testing.T) {
	cases := []struct {
		in   string
		line string
	}{
		{"api_key = secretvalue123", "line 1"},
		{"api_key = sk-secret-abc", "line 1"},
		{"api_key = \"secret-abc", "line 1"},
		{"model = \"m\"\napi_key = secretvalue123", "line 2"},
		{"key = secretvalue123", "line 1"},
		{"key = [\"secret\"]", "line 1"},
	}
	for _, c := range cases {
		_, err := ParseGlobal([]byte(c.in))
		if err == nil {
			t.Errorf("%q: want error", c.in)
			continue
		}
		msg := err.Error()
		if strings.Contains(msg, "secret") || strings.Contains(msg, `"sk"`) {
			t.Errorf("%q: error quotes the value: %s", c.in, msg)
		}
		if !strings.Contains(msg, c.line) {
			t.Errorf("%q: error %q doesn't say %s", c.in, msg, c.line)
		}
	}
}

func TestParseCatalogReadsLists(t *testing.T) {
	c, err := ParseCatalog([]byte("include = [\"a\"]\nexclude = [\"b\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c, Catalog{Include: []string{"a"}, Exclude: []string{"b"}}) {
		t.Errorf("catalog = %+v", c)
	}
}

func TestParseCatalogEmpty(t *testing.T) {
	for _, data := range []string{"", "# only a comment\n"} {
		c, err := ParseCatalog([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		if len(c.Include) != 0 || len(c.Exclude) != 0 {
			t.Errorf("%q: catalog = %+v", data, c)
		}
	}
}

// A config.toml may hold a pasted secret, and the error reaches the journal
// and catalog status, so it says where the problem is and nothing else.
func TestParseCatalogErrorQuotesNothing(t *testing.T) {
	cases := []struct{ in, want string }{
		{"include = [sk-secret-4", `parse config.toml: line 1 (last key "include"): not valid TOML`},
		{"sk-secret-4 =", "parse config.toml: line 1: not valid TOML"},
		{"include = 5", `parse config.toml: line 1 (last key "include"): a value has the wrong type`},
		{"sk-secret-4 = 5\nexclude = 5", `parse config.toml: line 2 (last key "exclude"): a value has the wrong type`},
		{"[[", "parse config.toml: line 1: not valid TOML"},
	}
	for _, c := range cases {
		_, err := ParseCatalog([]byte(c.in))
		if err == nil {
			t.Errorf("%q: want error", c.in)
			continue
		}
		if strings.Contains(err.Error(), "sk-secret") {
			t.Errorf("%q: error quotes the value: %s", c.in, err)
		}
		if err.Error() != c.want {
			t.Errorf("%q: error = %q, want %q", c.in, err, c.want)
		}
	}
}

func TestListsAddUp(t *testing.T) {
	g, err := ParseGlobal([]byte(`
include = ["**/*.html"]
exclude = ["drafts/**"]
`))
	if err != nil {
		t.Fatal(err)
	}
	c := Catalog{Include: []string{"notes/**"}, Exclude: []string{"notes.md"}}
	include, exclude := Lists(g, c)
	wantInclude := append(slices.Clone(BuiltinInclude), "**/*.html", "notes/**")
	wantExclude := append(slices.Clone(BuiltinExclude), "drafts/**", "notes.md")
	if !slices.Equal(include, wantInclude) {
		t.Errorf("include = %v, want %v", include, wantInclude)
	}
	if !slices.Equal(exclude, wantExclude) {
		t.Errorf("exclude = %v, want %v", exclude, wantExclude)
	}
}

func TestBuiltinLists(t *testing.T) {
	if want := []string{"**/*.csv", "**/*.md", "**/*.pdf", "**/*.txt"}; !slices.Equal(BuiltinInclude, want) {
		t.Errorf("BuiltinInclude = %v", BuiltinInclude)
	}
	if want := []string{"**/CLAUDE.md", "**/AGENTS.md"}; !slices.Equal(BuiltinExclude, want) {
		t.Errorf("BuiltinExclude = %v", BuiltinExclude)
	}
}

func TestListsDoesNotAliasInputs(t *testing.T) {
	// Extra capacity would let append write into the caller's slices.
	g := Global{Include: make([]string, 1, 8), Exclude: make([]string, 1, 8)}
	c := Catalog{Include: make([]string, 1, 8), Exclude: make([]string, 1, 8)}
	builtinInclude := slices.Clone(BuiltinInclude)
	builtinExclude := slices.Clone(BuiltinExclude)
	include, exclude := Lists(g, c)
	for i := range include {
		include[i] = "changed"
	}
	for i := range exclude {
		exclude[i] = "changed"
	}
	if g.Include[0] != "" || g.Exclude[0] != "" || c.Include[0] != "" || c.Exclude[0] != "" {
		t.Errorf("Lists shares memory with its inputs: %+v %+v", g, c)
	}
	if !slices.Equal(BuiltinInclude, builtinInclude) || !slices.Equal(BuiltinExclude, builtinExclude) {
		t.Errorf("Lists shares memory with the built-in lists: %v %v", BuiltinInclude, BuiltinExclude)
	}
}

func TestStateDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "Library", "Application Support", "com.westarete.catalog")
	if got != want {
		t.Errorf("StateDir = %q, want %q", got, want)
	}
}

// A person can paste the API key as a bare TOML key, so the decoder's last
// key may be the secret itself. The error names a key only when
// global.toml defines it.
func TestParseGlobalErrorNamesOnlyKnownKeys(t *testing.T) {
	_, err := ParseGlobal([]byte("sk-ant-api03-SECRETxyz ="))
	if err == nil {
		t.Fatal("want error")
	}
	if strings.Contains(err.Error(), "SECRETxyz") {
		t.Errorf("error quotes an unknown key: %s", err)
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Errorf("error %q doesn't say line 1", err)
	}

	_, err = ParseGlobal([]byte("model = "))
	if err == nil || !strings.Contains(err.Error(), "model") {
		t.Errorf("error %v should name the known key model", err)
	}

	_, err = ParseGlobal([]byte("model = 5"))
	if err == nil || !strings.Contains(err.Error(), "model") || !strings.Contains(err.Error(), "line 1") {
		t.Errorf("type error %v should name line 1 and key model", err)
	}

	_, err = ParseGlobal([]byte("sk-ant-api03-SECRETxyz = 5\nwatch = 3"))
	if err == nil || strings.Contains(err.Error(), "SECRETxyz") {
		t.Errorf("unknown key in a decode error leaked: %v", err)
	}

	_, err = ParseGlobal([]byte("watch = 3"))
	if err == nil || !strings.Contains(err.Error(), `"watch"`) {
		t.Errorf("error %v should name watch", err)
	}
}

// writeWithMode writes text to global.toml in a new folder and sets its
// mode, whatever the umask.
func writeWithMode(t *testing.T, text string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), GlobalFile)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

// Group bits are the owner's choice, so only the world bits
// stop ReadGlobalFile.
func TestReadGlobalFileAcceptsOwnerAndGroup(t *testing.T) {
	for _, mode := range []os.FileMode{0o600, 0o400, 0o640, 0o660} {
		path := writeWithMode(t, `key = "sk-secret"`, mode)
		g, err := ReadGlobalFile(path)
		if err != nil || g.Key != "sk-secret" {
			t.Errorf("mode %v: ReadGlobalFile = key %q, %v; want sk-secret", mode, g.Key, err)
		}
	}
}

func TestReadGlobalFileRefusesWorldAccessible(t *testing.T) {
	for _, mode := range []os.FileMode{0o604, 0o602, 0o601, 0o644, 0o666, 0o607} {
		path := writeWithMode(t, `key = "sk-secret"`, mode)
		g, err := ReadGlobalFile(path)
		if !errors.Is(err, ErrWorldAccessible) {
			t.Errorf("mode %v: err = %v; want ErrWorldAccessible", mode, err)
			continue
		}
		if g.Key != "" {
			t.Errorf("mode %v: returned key %q with the error", mode, g.Key)
		}
		if msg := err.Error(); strings.Contains(msg, "sk-secret") || !strings.Contains(msg, mode.String()) {
			t.Errorf("mode %v: error %q; want the mode and no key", mode, msg)
		}
	}
}

func TestReadGlobalFileMissing(t *testing.T) {
	_, err := ReadGlobalFile(filepath.Join(t.TempDir(), GlobalFile))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v; want fs.ErrNotExist", err)
	}
}

func TestReadGlobalFileParseErrorQuotesNoValue(t *testing.T) {
	path := writeWithMode(t, "key = sk-secret", 0o600)
	_, err := ReadGlobalFile(path)
	if err == nil || strings.Contains(err.Error(), "sk-secret") {
		t.Errorf("err = %v; want a parse error without the key", err)
	}
}

func TestReadGlobalFileRefusesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), GlobalFile)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := ReadGlobalFile(path)
	if !errors.Is(err, ErrNotRegularFile) {
		t.Errorf("err = %v; want ErrNotRegularFile", err)
	}
}

// Opening a FIFO waits for a writer, so a FIFO at the path would stop the
// daemon and status for good if ReadGlobalFile opened it the usual way.
func TestReadGlobalFileRefusesFIFOWithoutWaiting(t *testing.T) {
	path := filepath.Join(t.TempDir(), GlobalFile)
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	got := make(chan error, 1)
	go func() {
		_, err := ReadGlobalFile(path)
		got <- err
	}()
	select {
	case err := <-got:
		if !errors.Is(err, ErrNotRegularFile) {
			t.Errorf("err = %v; want ErrNotRegularFile", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ReadGlobalFile waited on a FIFO")
	}
}
