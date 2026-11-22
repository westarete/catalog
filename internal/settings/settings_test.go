package settings

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

var (
	defaultEnumerate = []string{"**/*.csv", "**/*.md", "**/*.pdf", "**/*.txt"}
	defaultIgnore    = []string{"README.md", "**/CLAUDE.md", "**/AGENTS.md"}
)

func TestParseGlobalDefaults(t *testing.T) {
	g, err := ParseGlobal([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if g.Model != "claude-haiku-5-5" {
		t.Errorf("Model = %q", g.Model)
	}
	if !reflect.DeepEqual(g.Enumerate, defaultEnumerate) {
		t.Errorf("Enumerate = %v", g.Enumerate)
	}
	if !reflect.DeepEqual(g.Ignore, defaultIgnore) {
		t.Errorf("Ignore = %v", g.Ignore)
	}
}

func TestParseGlobalKeepsSetValues(t *testing.T) {
	g, err := ParseGlobal([]byte(`
api_key = "sk-test"
model = "other-model"
enumerate = ["a"]
ignore = []
`))
	if err != nil {
		t.Fatal(err)
	}
	if g.APIKey != "sk-test" || g.Model != "other-model" {
		t.Errorf("got %+v", g)
	}
	if !reflect.DeepEqual(g.Enumerate, []string{"a"}) {
		t.Errorf("Enumerate = %v", g.Enumerate)
	}
	if len(g.Ignore) != 0 {
		t.Errorf("explicit empty ignore was replaced: %v", g.Ignore)
	}
}

func TestParseGlobalWatch(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	g, err := ParseGlobal([]byte(`
[[watch]]
folder = "~/notes"

[[watch]]
folder = "/abs/path"

[[watch]]
folder = "~"

[[watch]]
folder = "~other/x"
`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(home, "notes"), "/abs/path", home, "~other/x"}
	if !reflect.DeepEqual(g.Watch, want) {
		t.Errorf("Watch = %v, want %v", g.Watch, want)
	}
}

func TestParseGlobalMalformed(t *testing.T) {
	if _, err := ParseGlobal([]byte("model = ")); err == nil {
		t.Error("want error")
	}
}

// global.toml holds the API key, and a parse error reaches the journal, the
// log, and catalog status, so the error must name where the problem is
// without quoting any of the file's values.
func TestParseGlobalErrorQuotesNoValue(t *testing.T) {
	cases := []struct {
		in   string
		line string
	}{
		{"api_key = secretvalue123", "line 1"},
		{"api_key = sk-secret-abc", "line 1"},
		{"api_key = \"secret-abc", "line 1"},
		{"api_key = [\"secret\"]", "line 1"},
		{"model = \"m\"\napi_key = secretvalue123", "line 2"},
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

func TestApplyCatalogReplacesLists(t *testing.T) {
	g, _ := ParseGlobal(nil)
	enumerate, ignore, err := ApplyCatalog(g, []byte(`enumerate = ["**/*.md"]`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(enumerate, []string{"**/*.md"}) {
		t.Errorf("enumerate = %v", enumerate)
	}
	if !reflect.DeepEqual(ignore, g.Ignore) {
		t.Errorf("ignore = %v", ignore)
	}
}

func TestApplyCatalogReplacesIgnore(t *testing.T) {
	g, _ := ParseGlobal(nil)
	enumerate, ignore, err := ApplyCatalog(g, []byte(`ignore = []`))
	if err != nil {
		t.Fatal(err)
	}
	if len(ignore) != 0 {
		t.Errorf("ignore = %v", ignore)
	}
	if !reflect.DeepEqual(enumerate, g.Enumerate) {
		t.Errorf("enumerate = %v", enumerate)
	}
}

func TestApplyCatalogIgnoresOtherKeys(t *testing.T) {
	g, _ := ParseGlobal(nil)
	enumerate, ignore, err := ApplyCatalog(g, []byte(`
api_key = "sk-other"
model = "other"
profile = "profiles/default.md"
input_price = 1.5
output_price = 7.5
`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(enumerate, g.Enumerate) || !reflect.DeepEqual(ignore, g.Ignore) {
		t.Errorf("lists changed: %v %v", enumerate, ignore)
	}
}

func TestApplyCatalogEmpty(t *testing.T) {
	g, _ := ParseGlobal(nil)
	for _, data := range []string{"", "# only a comment\n"} {
		enumerate, ignore, err := ApplyCatalog(g, []byte(data))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(enumerate, g.Enumerate) || !reflect.DeepEqual(ignore, g.Ignore) {
			t.Errorf("lists changed for %q", data)
		}
	}
}

func TestApplyCatalogMalformed(t *testing.T) {
	g, _ := ParseGlobal(nil)
	enumerate, ignore, err := ApplyCatalog(g, []byte("enumerate = ["))
	if err == nil {
		t.Fatal("want error")
	}
	if enumerate != nil || ignore != nil {
		t.Errorf("want nil lists, got %v %v", enumerate, ignore)
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

	_, err = ParseGlobal([]byte("sk-ant-api03-SECRETxyz = 5\n[[watch]]\nfolder = 3"))
	if err == nil || strings.Contains(err.Error(), "SECRETxyz") {
		t.Errorf("unknown key in a decode error leaked: %v", err)
	}

	_, err = ParseGlobal([]byte("[[watch]]\nfolder = 3"))
	if err == nil || !strings.Contains(err.Error(), "folder") {
		t.Errorf("error %v should name folder", err)
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
		path := writeWithMode(t, `api_key = "sk-secret"`, mode)
		g, err := ReadGlobalFile(path)
		if err != nil || g.APIKey != "sk-secret" {
			t.Errorf("mode %v: ReadGlobalFile = key %q, %v; want sk-secret", mode, g.APIKey, err)
		}
	}
}

func TestReadGlobalFileRefusesWorldAccessible(t *testing.T) {
	for _, mode := range []os.FileMode{0o604, 0o602, 0o601, 0o644, 0o666, 0o607} {
		path := writeWithMode(t, `api_key = "sk-secret"`, mode)
		g, err := ReadGlobalFile(path)
		if !errors.Is(err, ErrWorldAccessible) {
			t.Errorf("mode %v: err = %v; want ErrWorldAccessible", mode, err)
			continue
		}
		if g.APIKey != "" {
			t.Errorf("mode %v: returned key %q with the error", mode, g.APIKey)
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
	path := writeWithMode(t, "api_key = sk-secret", 0o600)
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
