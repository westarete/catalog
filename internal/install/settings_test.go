package install

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/BurntSushi/toml"
)

// readBack decodes a settings file written by WriteGlobal. The install
// package may not import the settings package, so it reads the keys itself.
func readBack(t *testing.T, path string) (key string, folders []string, hasKey bool) {
	t.Helper()
	var f struct {
		APIKey string `toml:"api_key"`
		Watch  []struct {
			Folder string `toml:"folder"`
		} `toml:"watch"`
	}
	md, err := toml.DecodeFile(path, &f)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range f.Watch {
		folders = append(folders, w.Folder)
	}
	return f.APIKey, folders, md.IsDefined("api_key")
}

func TestReadFolders(t *testing.T) {
	cases := []struct {
		name, in string
		want     []string
		prompts  int
	}{
		{"nothing given", "\n", []string{"~/src"}, 1},
		{"trims and stops at empty line", "  ~/src  \n~/notes/Project Plans\n\n", []string{"~/src", "~/notes/Project Plans"}, 3},
		{"end of input", "~/src", []string{"~/src"}, 1},
		{"empty input", "", []string{"~/src"}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := ReadFolders(bufio.NewReader(strings.NewReader(c.in)), &out)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("folders = %q, want %q", got, c.want)
			}
			if n := strings.Count(out.String(), "Folder: "); n != c.prompts {
				t.Errorf("printed Folder: %d times, want %d", n, c.prompts)
			}
			if out.String() != strings.Repeat("Folder: ", c.prompts) {
				t.Errorf("output = %q, want only prompts", out.String())
			}
		})
	}
}

func TestWriteGlobalRoundTrips(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "state", "global.toml")
	if err := WriteGlobal(path, "sk-test", []string{"~/src", "/data/notes"}); err != nil {
		t.Fatal(err)
	}
	key, watch, _ := readBack(t, path)
	if key != "sk-test" {
		t.Errorf("api key did not round-trip")
	}
	want := []string{"~/src", "/data/notes"}
	if strings.Join(watch, "|") != strings.Join(want, "|") {
		t.Errorf("watch = %q, want %q", watch, want)
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("folder mode = %v, want 0700", info.Mode().Perm())
	}
}

func TestWriteGlobalLeavesOutEmptyKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "global.toml")
	if err := WriteGlobal(path, "", []string{"~/src"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "api_key") {
		t.Errorf("file has api_key:\n%s", data)
	}
}

func TestWriteGlobalQuotesOddCharacters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "global.toml")
	key := `sk-"a\b`
	folder := "/data/Project Plans"
	if err := WriteGlobal(path, key, []string{folder}); err != nil {
		t.Fatal(err)
	}
	got, watch, _ := readBack(t, path)
	if got != key || len(watch) != 1 || watch[0] != folder {
		t.Errorf("did not round-trip: watch = %q", watch)
	}
}

func TestWriteGlobalModeIgnoresUmask(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	// 0o300 would strip the owner's write and search bits from a mode made
	// without an explicit chmod, so it fails if the code relies on the umask.
	old := syscall.Umask(0o300)
	defer syscall.Umask(old)
	path := filepath.Join(dir, "global.toml")
	if err := WriteGlobal(path, "", []string{"~/src"}); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]fs.FileMode{path: 0o600, dir: 0o700} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode = %v, want %v", filepath.Base(p), got, want)
		}
	}
}

func TestWriteGlobalLeavesExistingFolderMode(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil { //nolint:gosec // a shared folder, to prove it is left alone
		t.Fatal(err)
	}
	if err := WriteGlobal(filepath.Join(dir, "global.toml"), "", []string{"~/src"}); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(dir)
	if info.Mode().Perm() != 0o755 {
		t.Errorf("existing folder mode changed to %v", info.Mode().Perm())
	}
}

func TestWriteGlobalRefusesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "global.toml")
	if err := os.WriteFile(path, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := WriteGlobal(path, "sk-test", []string{"~/src"})
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("err = %v, want fs.ErrExist", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "mine" {
		t.Errorf("file changed: %q", data)
	}
}

func TestWriteGlobalReportsUncreatableFolder(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteGlobal(filepath.Join(blocker, "sub", "global.toml"), "", nil); err == nil {
		t.Error("want an error")
	}
}

func TestTilde(t *testing.T) {
	cases := []struct{ home, path, want string }{
		{"/Users/a", "/Users/a/src", "~/src"},
		{"/Users/a", "/Users/a", "~"},
		{"/Users/a", "/Users/ab/src", "/Users/ab/src"},
		{"/Users/a", "/elsewhere", "/elsewhere"},
		{"", "/elsewhere", "/elsewhere"},
		{"/", "/x", "/x"},
	}
	for _, c := range cases {
		if got := Tilde(c.home, c.path); got != c.want {
			t.Errorf("Tilde(%q, %q) = %q, want %q", c.home, c.path, got, c.want)
		}
	}
}

// askSettings runs AskSettings with a fake key reader and returns the
// output. It never logs the output, which could hold the key.
func askSettings(t *testing.T, home, in, key string, keyErr error, keyCalls *int) (string, error) {
	t.Helper()
	var out bytes.Buffer
	p := Prompt{
		In:   bufio.NewReader(strings.NewReader(in)),
		Out:  &out,
		Home: home,
		ReadKey: func() (string, error) {
			if keyCalls != nil {
				*keyCalls++
			}
			return key, keyErr
		},
	}
	err := AskSettings(p, filepath.Join(home, "state", "global.toml"))
	return out.String(), err
}

func TestAskSettingsWritesNewFile(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	out, err := askSettings(t, home, "~/src\n\n", " sk-test ", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "Paste your Anthropic API key and press Return, or press Return to skip this step.\n" +
		"API key: \n\n" +
		"The daemon will watch the folders you list. Don't list a folder that a cloud drive syncs, or Desktop, Documents, or Downloads.\n" +
		"Enter one folder per line, then press Return on an empty line to finish. To watch only ~/src, press Return now.\n" +
		"Folder: Folder: " +
		"\n" +
		"Your settings are in ~/state/global.toml, and only you can read that file.\n"
	if out != want {
		t.Errorf("output =\n%q\nwant\n%q", out, want)
	}
	key, _, _ := readBack(t, filepath.Join(home, "state", "global.toml"))
	if key != "sk-test" {
		t.Error("key was not trimmed and saved")
	}
}

func TestAskSettingsEndOfInput(t *testing.T) {
	home := t.TempDir()
	out, err := askSettings(t, home, "", "", io.EOF, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, watch, hasKey := readBack(t, filepath.Join(home, "state", "global.toml"))
	if hasKey {
		t.Error("file has api_key")
	}
	if len(watch) != 1 || watch[0] != "~/src" {
		t.Errorf("watch = %q", watch)
	}
	if !strings.Contains(out, "To add your API key later, set api_key in that file.\n") {
		t.Errorf("output lacks the later-key line:\n%s", out)
	}
}

func TestAskSettingsReportsKeyReadError(t *testing.T) {
	home := t.TempDir()
	boom := errors.New("terminal gone")
	_, err := askSettings(t, home, "", "", boom, nil)
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want it to wrap the read error", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, "state")); statErr == nil {
		t.Error("wrote files after a failed key read")
	}
}

func TestAskSettingsWarnsMissingFolder(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, "there"), 0o700); err != nil {
		t.Fatal(err)
	}
	out, err := askSettings(t, home, "~/nope\n~/there\n/also/missing\n\n", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"~/nope", "/also/missing"} {
		if !strings.Contains(out, f+" doesn't exist yet. The daemon will start watching it once you create it.\n") {
			t.Errorf("no warning for %s:\n%s", f, out)
		}
	}
	if strings.Contains(out, "~/there doesn't") {
		t.Error("warned about a folder that exists")
	}
	if strings.Index(out, "~/nope doesn't") > strings.Index(out, "/also/missing doesn't") {
		t.Error("warnings out of order")
	}
	_, watch, _ := readBack(t, filepath.Join(home, "state", "global.toml"))
	if len(watch) != 3 {
		t.Errorf("watch = %q, want all three", watch)
	}
}

func TestAskSettingsKeepsExistingFile(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "state", "global.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not [ toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	out, err := askSettings(t, home, "~/src\n\n", "sk-test", nil, &calls)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Your settings are already in ~/state/global.toml. To change them, edit that file.\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
	if calls != 0 {
		t.Error("asked for the key")
	}
	if data, _ := os.ReadFile(path); string(data) != "not [ toml" {
		t.Errorf("file changed: %q", data)
	}
}

func TestAskSettingsNeverPrintsKey(t *testing.T) {
	home := t.TempDir()
	out, err := askSettings(t, home, "~/src\n\n", "sk-test", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "sk-test") {
		t.Error("output contains the key")
	}
}

func TestWriteGlobalRemovesFileAfterFailedWrite(t *testing.T) {
	boom := errors.New("disk full")
	old := encodeSettings
	encodeSettings = func(w io.Writer, _ any) error {
		_, _ = w.Write([]byte("api_key = \"partial"))
		return boom
	}
	defer func() { encodeSettings = old }()
	path := filepath.Join(t.TempDir(), "global.toml")
	err := WriteGlobal(path, "sk-test", []string{"~/src"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the encode error", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("a partial file was left behind: %v", statErr)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestReadFoldersReturnsReadError(t *testing.T) {
	boom := errors.New("terminal gone")
	got, err := ReadFolders(bufio.NewReader(failingReader{boom}), io.Discard)
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want the read error", err)
	}
	if got != nil {
		t.Errorf("folders = %q, want none", got)
	}
}

func TestAskSettingsReportsFolderReadError(t *testing.T) {
	home := t.TempDir()
	boom := errors.New("terminal gone")
	p := Prompt{
		In:      bufio.NewReader(failingReader{boom}),
		Out:     io.Discard,
		Home:    home,
		ReadKey: func() (string, error) { return "sk-test", nil },
	}
	err := AskSettings(p, filepath.Join(home, "state", "global.toml"))
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want it to wrap the read error", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, "state")); statErr == nil {
		t.Error("wrote files after a failed folder read")
	}
}
