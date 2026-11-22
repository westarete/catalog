package install

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"westarete.com/catalog/internal/settings"
)

// readBack reads the folders from a settings file written by WriteGlobal,
// the way the daemon does, but without expanding ~.
func readBack(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	g, err := settings.ParseGlobal(data)
	if err != nil {
		t.Fatal(err)
	}
	return g.Watch
}

// expanded returns folder with ~ replaced by the home folder, as the daemon
// reads it, because readBack reads folders the daemon's way.
func expanded(t *testing.T, folder string) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	return expandTilde(home, folder)
}

// readKeys reads a keys file the way the daemon does.
func readKeys(t *testing.T, path string) settings.Keys {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := settings.ParseKeys(data)
	if err != nil {
		t.Fatal(err)
	}
	return keys
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
	path := filepath.Join(t.TempDir(), "state", "global.toml")
	if err := WriteGlobal(path, []string{"~/src", "/data/notes"}); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "watch = [\n  \"~/src\",\n  \"/data/notes\",\n]\n"; string(text) != want {
		t.Errorf("file = %q, want %q", text, want)
	}
	watch := readBack(t, path)
	want := []string{expanded(t, "~/src"), "/data/notes"}
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

func TestWriteGlobalQuotesOddCharacters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "global.toml")
	folder := "/data/Plans \"Q1\" \\ café é"
	if err := WriteGlobal(path, []string{folder}); err != nil {
		t.Fatal(err)
	}
	watch := readBack(t, path)
	if len(watch) != 1 || watch[0] != folder {
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
	if err := WriteGlobal(path, []string{"~/src"}); err != nil {
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
	if err := WriteGlobal(filepath.Join(dir, "global.toml"), []string{"~/src"}); err != nil {
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
	err := WriteGlobal(path, []string{"~/src"})
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
	if err := WriteGlobal(filepath.Join(blocker, "sub", "global.toml"), nil); err == nil {
		t.Error("want an error")
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
	err := AskSettings(p, globalPathIn(home), keysPathIn(home), keysHaveKey)
	return out.String(), err
}

// keysHaveKey stands in for the command's check of an existing keys.toml.
func keysHaveKey() KeyCheck { return KeyFound }

func globalPathIn(home string) string { return filepath.Join(home, "state", "global.toml") }

func keysPathIn(home string) string { return filepath.Join(home, "state", "keys.toml") }

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
		"Your API key is in " + keysPathIn(home) + ", and only you can read that file.\n" +
		"Your settings are in " + globalPathIn(home) + ", and only you can read that file.\n"
	if out != want {
		t.Errorf("output =\n%q\nwant\n%q", out, want)
	}
	if keys := readKeys(t, keysPathIn(home)); len(keys) != 1 || keys["default"] != "sk-test" {
		t.Error("key was not trimmed and saved as default")
	}
	if watch := readBack(t, globalPathIn(home)); len(watch) != 1 || watch[0] != expanded(t, "~/src") {
		t.Errorf("watch = %q", watch)
	}
}

func TestAskSettingsEndOfInput(t *testing.T) {
	home := t.TempDir()
	out, err := askSettings(t, home, "", "", io.EOF, nil)
	if err != nil {
		t.Fatal(err)
	}
	if keys := readKeys(t, keysPathIn(home)); len(keys) != 0 {
		t.Error("keys.toml has a key")
	}
	if watch := readBack(t, globalPathIn(home)); len(watch) != 1 || watch[0] != expanded(t, "~/src") {
		t.Errorf("watch = %q", watch)
	}
	if !strings.Contains(out, "To add your API key later, set default in "+keysPathIn(home)+".\n") {
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
	if watch := readBack(t, globalPathIn(home)); len(watch) != 3 {
		t.Errorf("watch = %q, want all three", watch)
	}
}

// Setup treats each file on its own: it asks only for what goes in a
// missing file, writes only the missing files, and leaves an existing file
// as it is, even one that doesn't parse.
func TestAskSettingsAsksOnlyForMissingFiles(t *testing.T) {
	const existing = "not [ toml"
	cases := []struct {
		name                    string
		globalExists, keysExist bool
	}{
		{"neither exists", false, false},
		{"only global.toml exists", true, false},
		{"only keys.toml exists", false, true},
		{"both exist", true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			if err := os.MkdirAll(filepath.Join(home, "state"), 0o700); err != nil {
				t.Fatal(err)
			}
			for path, exists := range map[string]bool{globalPathIn(home): c.globalExists, keysPathIn(home): c.keysExist} {
				if exists {
					if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			calls := 0

			out, err := askSettings(t, home, "~/src\n\n", "sk-test", nil, &calls)
			if err != nil {
				t.Fatal(err)
			}

			asked := map[string]bool{
				"key":     calls != 0,
				"folders": strings.Contains(out, "Folder: "),
			}
			if asked["key"] == c.keysExist || asked["folders"] == c.globalExists {
				t.Errorf("asked for %v; want only what goes in a missing file", asked)
			}
			if got := strings.Contains(out, "API key: "); got == c.keysExist {
				t.Errorf("key prompt shown: %v; want %v", got, !c.keysExist)
			}
			for _, f := range []struct {
				path, already string
				exists        bool
			}{
				{globalPathIn(home), "Your settings are already in " + globalPathIn(home) + ". To change them, edit that file.\n", c.globalExists},
				{keysPathIn(home), "Your API keys are already in " + keysPathIn(home) + ". To change them, edit that file.\n", c.keysExist},
			} {
				data, err := os.ReadFile(f.path)
				if err != nil {
					t.Fatal(err)
				}
				if f.exists && string(data) != existing {
					t.Errorf("%s changed: %q", filepath.Base(f.path), data)
				}
				if !f.exists && string(data) == existing {
					t.Errorf("%s not written", filepath.Base(f.path))
				}
				if got := strings.Contains(out, f.already); got != f.exists {
					t.Errorf("says %s is already there: %v; want %v", filepath.Base(f.path), got, f.exists)
				}
			}
			if !c.keysExist && readKeys(t, keysPathIn(home))["default"] != "sk-test" {
				t.Error("keys.toml lacks the key")
			}
			if !c.globalExists {
				if watch := readBack(t, globalPathIn(home)); len(watch) != 1 || watch[0] != expanded(t, "~/src") {
					t.Errorf("watch = %q", watch)
				}
			}
		})
	}
}

// An existing keys.toml is never changed and no key is asked for. If it
// lacks the key global.toml asks for, setup says how to add it, without
// naming a key other than default.
func TestAskSettingsSaysHowToAddMissingKey(t *testing.T) {
	const already = "Your API keys are already in %s. To change them, edit that file.\n"
	for check, want := range map[KeyCheck]string{
		KeyFound:          already,
		KeyUnknown:        already,
		DefaultKeyMissing: "To add your API key, set default in %s.\n",
		NamedKeyMissing:   "To add your API key, add it to %s under the name global.toml asks for.\n",
	} {
		home := t.TempDir()
		if err := os.MkdirAll(filepath.Join(home, "state"), 0o700); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{globalPathIn(home), keysPathIn(home)} {
			if err := os.WriteFile(path, []byte("mine"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		var out bytes.Buffer
		p := Prompt{
			In:  bufio.NewReader(strings.NewReader("")),
			Out: &out,
			ReadKey: func() (string, error) {
				t.Error("asked for the key")
				return "", nil
			},
			Home: home,
		}

		if err := AskSettings(p, globalPathIn(home), keysPathIn(home), func() KeyCheck { return check }); err != nil {
			t.Fatal(err)
		}

		if got := strings.TrimPrefix(out.String(), "Your settings are already in "+globalPathIn(home)+". To change them, edit that file.\n"); got != fmt.Sprintf(want, keysPathIn(home)) {
			t.Errorf("check %v: output %q, want %q", check, out.String(), fmt.Sprintf(want, keysPathIn(home)))
		}
		if data, _ := os.ReadFile(keysPathIn(home)); string(data) != "mine" {
			t.Errorf("check %v: keys.toml changed: %q", check, data)
		}
	}
}

// Setup checks keys.toml for the key only when the file exists.
func TestAskSettingsChecksOnlyExistingKeys(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	p := Prompt{
		In:      bufio.NewReader(strings.NewReader("")),
		Out:     &out,
		ReadKey: func() (string, error) { return "sk-test", nil },
		Home:    home,
	}
	checked := false

	err := AskSettings(p, globalPathIn(home), keysPathIn(home), func() KeyCheck { checked = true; return DefaultKeyMissing })

	if err != nil || checked {
		t.Errorf("err %v, checked %v; want no check of a missing keys.toml", err, checked)
	}
}

// wantKeysComment is what setup writes to keys.toml when the person skips the
// key, so a key they add later goes into a file only they can read.
const wantKeysComment = "# Your Anthropic API keys, one per line, by name. The daemon uses\n" +
	"# the one named default unless global.toml names another with key.\n" +
	"# default = \"your-key\"\n"

func TestAskSettingsSkippedKeyWritesCommentOnlyKeys(t *testing.T) {
	home := t.TempDir()

	out, err := askSettings(t, home, "~/src\n\n", "  ", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(keysPathIn(home))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != wantKeysComment {
		t.Errorf("keys.toml =\n%q\nwant\n%q", data, wantKeysComment)
	}
	if keys := readKeys(t, keysPathIn(home)); len(keys) != 0 {
		t.Errorf("keys.toml has %d keys, want none", len(keys))
	}
	assertMode(t, keysPathIn(home), 0o600)
	if !strings.Contains(out, "To add your API key later, set default in "+keysPathIn(home)+".\n") {
		t.Errorf("output lacks the later-key line:\n%s", out)
	}
	if strings.Contains(out, "Your API key is in") {
		t.Errorf("output says a key was saved:\n%s", out)
	}
}

func assertMode(t *testing.T, path string, want fs.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s mode = %v, want %v", filepath.Base(path), got, want)
	}
}

func TestWriteKeysModeUnderUmask(t *testing.T) {
	old := syscall.Umask(0)
	defer syscall.Umask(old)
	for name, key := range map[string]string{"with a key": "sk-test", "without a key": ""} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "state")
			path := filepath.Join(dir, "keys.toml")
			if err := WriteKeys(path, key); err != nil {
				t.Fatal(err)
			}
			assertMode(t, path, 0o600)
			assertMode(t, dir, 0o700)
		})
	}
}

// A key with characters TOML must escape reads back unchanged.
func TestWriteKeysRoundTrip(t *testing.T) {
	for _, key := range []string{
		"sk-plain",
		`sk-"quoted"`,
		`sk-back\slash`,
		"sk-café",
		"sk-tab\tnull\x00bell\x07esc\x1bdel\x7f",
		"sk-newline\nreturn\r",
	} {
		path := filepath.Join(t.TempDir(), "keys.toml")
		if err := WriteKeys(path, key); err != nil {
			t.Fatal(err)
		}
		keys := readKeys(t, path)
		if len(keys) != 1 || keys["default"] != key {
			t.Errorf("key %q did not round-trip", key)
		}
	}
}

// Only the characters TOML requires are escaped, so the file shows the key
// as the person pasted it.
func TestWriteKeysKeepsOtherCharacters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.toml")
	if err := WriteKeys(path, "sk a~é"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "default = \"sk a~é\"\n"; string(data) != want {
		t.Errorf("keys.toml = %q, want %q", data, want)
	}
}

func TestWriteKeysRefusesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.toml")
	if err := os.WriteFile(path, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := WriteKeys(path, "sk-test")
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("err = %v, want fs.ErrExist", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "mine" {
		t.Errorf("file changed: %q", data)
	}
}

// A file that fails partway through writing is removed, so setup never
// leaves a partial keys.toml or global.toml behind.
func TestCreatePrivateFileRemovesFileAfterFailedWrite(t *testing.T) {
	boom := errors.New("disk full")
	path := filepath.Join(t.TempDir(), "keys.toml")
	err := createPrivateFile(path, func(w io.Writer) error {
		_, _ = w.Write([]byte("default = \"partial"))
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the write error", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("a partial file was left behind: %v", statErr)
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
	err := AskSettings(p, globalPathIn(home), keysPathIn(home), keysHaveKey)
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want it to wrap the read error", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, "state")); statErr == nil {
		t.Error("wrote files after a failed folder read")
	}
}

const fullPathMessage = "Enter a full path that starts with / or ~/.\n"

// launchd starts the daemon in /, so a relative folder would watch the
// wrong place.
func TestReadFoldersRejectsRelativePaths(t *testing.T) {
	const p = "Folder: "
	const m = fullPathMessage
	cases := []struct {
		name, in string
		want     []string
		out      string
	}{
		{"relative", "src\n~/ok\n\n", []string{"~/ok"}, p + m + p + p},
		{"nested relative", "work/notes\n/abs\n\n", []string{"/abs"}, p + m + p + p},
		{"other user", "~user/x\n~/ok\n\n", []string{"~/ok"}, p + m + p + p},
		{"dot", "./src\n\n", []string{"~/src"}, p + m + p},
		{"rejected at end of input", "src", []string{"~/src"}, p + m},
		{"accepted forms", "/abs/path\n~/x\n~\n\n", []string{"/abs/path", "~/x", "~"}, p + p + p + p},
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
			if out.String() != c.out {
				t.Errorf("output = %q, want %q", out.String(), c.out)
			}
		})
	}
}

func TestAskSettingsNeverWritesRelativeFolder(t *testing.T) {
	home := t.TempDir()
	out, err := askSettings(t, home, "src\n~user/x\n~/ok\n\n", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, fullPathMessage) != 2 {
		t.Errorf("output lacks two rejections:\n%s", out)
	}
	if watch := readBack(t, globalPathIn(home)); len(watch) != 1 || watch[0] != expanded(t, "~/ok") {
		t.Errorf("watch = %q, want only ~/ok, expanded", watch)
	}
}

// Status reads global.toml with os.ReadFile, which fails on a dangling
// link, so setup must not call that link "already there".
func TestAskSettingsRefusesDanglingLink(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "state")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "global.toml")
	target := filepath.Join(home, "elsewhere.toml")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	out, err := askSettings(t, home, "~/src\n\n", "sk-test", nil, nil)
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("err = %v, want one wrapping fs.ErrExist", err)
	}
	if strings.Contains(out, "already in") {
		t.Errorf("output says the settings exist:\n%s", out)
	}
	if _, statErr := os.Stat(target); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("a file was created at the link's target: %v", statErr)
	}
	if got, lerr := os.Readlink(path); lerr != nil || got != target {
		t.Errorf("link changed: %q, %v", got, lerr)
	}
}
