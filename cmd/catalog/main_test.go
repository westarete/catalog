package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"westarete.com/catalog/internal/control"
)

// stateDir points the state folder at a temp home and returns it. The home
// is under /tmp because macOS limits a socket's path to 103 bytes, and the
// socket's path under the default temporary folder would be longer.
func stateDir(t *testing.T) string {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "cat")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("HOME", home)
	return filepath.Join(home, "Library", "Application Support", "com.westarete.catalog")
}

func runCatalog(args ...string) (int, string) {
	return runCatalogIn(context.Background(), args...)
}

// runCatalogIn runs the command and returns its exit code and what it wrote
// to stderr.
func runCatalogIn(ctx context.Context, args ...string) (int, string) {
	code, _, stderr := runCatalogOut(ctx, args...)
	return code, stderr
}

func runCatalogOut(ctx context.Context, args ...string) (code int, stdout, stderr string) {
	return runCatalogWith(ctx, strings.NewReader(""), args...)
}

// runCatalogWith runs the command with stdin as its standard input.
func runCatalogWith(ctx context.Context, stdin io.Reader, args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(ctx, args, stdin, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestOnceWithoutGlobalConfig(t *testing.T) {
	dir := stateDir(t)

	code, out := runCatalog("daemon", "--once", t.TempDir())

	if code == 0 {
		t.Error("exit code 0, want non-zero")
	}
	if !strings.Contains(out, filepath.Join(dir, "global.toml")) {
		t.Errorf("message doesn't name global.toml's path:\n%s", out)
	}
}

// --once stops before it opens the Store unless keys.toml holds the key
// global.toml asks for, and its message never repeats a name or a key.
func TestOnceRefusesWithoutKey(t *testing.T) {
	for name, c := range map[string]struct {
		global, keys string
		keysMode     os.FileMode
		want         string
	}{
		"no keys.toml":           {"", "", 0, "no API keys file at "},
		"world-accessible":       {"", `default = "sk-secret"`, 0o604, "world-accessible"},
		"no default key":         {"", `other = "sk-secret"`, 0o600, "has no key under the name global.toml asks for"},
		"no key under that name": {`key = "sk-pasted"`, `default = "sk-secret"`, 0o600, "has no key under the name global.toml asks for"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := stateDir(t)
			writeGlobal(t, dir, c.global)
			if c.keysMode != 0 {
				writeKeys(t, dir, c.keys)
				if err := os.Chmod(filepath.Join(dir, "keys.toml"), c.keysMode); err != nil {
					t.Fatal(err)
				}
			}

			code, out := runCatalog("daemon", "--once", t.TempDir())

			if code == 0 || !strings.Contains(out, filepath.Join(dir, "keys.toml")) || !strings.Contains(out, c.want) {
				t.Errorf("code %d, output:\n%s\nwant keys.toml's path and %q", code, out, c.want)
			}
			if strings.Contains(out, "sk-") {
				t.Errorf("output names a key:\n%s", out)
			}
			if _, err := os.Stat(filepath.Join(dir, "store.db")); !os.IsNotExist(err) {
				t.Errorf("store.db created before the key was checked: %v", err)
			}
		})
	}
}

func TestOnceWithBadGlobalConfig(t *testing.T) {
	writeSettings(t, stateDir(t), "model = [[[", "")

	if code, out := runCatalog("daemon", "--once", t.TempDir()); code == 0 || !strings.Contains(out, "global.toml") {
		t.Errorf("code %d, output:\n%s", code, out)
	}
}

func TestUsageErrors(t *testing.T) {
	stateDir(t)
	tests := [][]string{
		nil,
		{"bogus"},
		{"status", "extra"},
		{"daemon", "extra"},
		{"daemon", "--once"},
		{"daemon", "--once", "a", "b"},
		{"daemon", "--bogus"},
		{"setup", "extra"},
		{"setup", "--bogus"},
	}
	// A stopped context makes a mistake that runs the daemon fail fast
	// instead of running until the test times out.
	for _, args := range tests {
		code, out := runCatalogIn(stopped(), args...)
		if code != 2 || out == "" {
			t.Errorf("run(%q) = %d, %q; want 2 and a message", args, code, out)
		}
	}
}

// writeSettings writes global.toml and keys.toml in dir.
func writeSettings(t *testing.T, dir, global, keys string) {
	t.Helper()
	writeGlobal(t, dir, global)
	writeKeys(t, dir, keys)
}

func writeGlobal(t *testing.T, dir, text string) {
	t.Helper()
	writeStateFile(t, dir, "global.toml", text)
}

func writeKeys(t *testing.T, dir, text string) {
	t.Helper()
	writeStateFile(t, dir, "keys.toml", text)
}

func writeStateFile(t *testing.T, dir, name, text string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// testKeys is a keys.toml with a default key.
const testKeys = `default = "test-key"`

// A folder with no catalog has nothing to summarize, so this run makes no
// model call. It's most likely the wrong folder, so the run fails.
func TestOnceWithoutCatalog(t *testing.T) {
	dir := stateDir(t)
	writeSettings(t, dir, "", testKeys)
	folder := t.TempDir()

	code, stdout, stderr := runCatalogOut(context.Background(), "daemon", "--once", folder)

	if code != 1 || !strings.Contains(stderr, "no catalog in "+folder) || !strings.Contains(stdout, "No catalogs found.") {
		t.Errorf("code %d, stdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "store.db")); err != nil {
		t.Errorf("store.db: %v", err)
	}
}

func TestOnceReportsUnreadableFolder(t *testing.T) {
	writeSettings(t, stateDir(t), "", testKeys)
	folder := filepath.Join(t.TempDir(), "missing")

	code, stdout, stderr := runCatalogOut(context.Background(), "daemon", "--once", folder)

	if code != 1 || !strings.Contains(stdout, "Failing:\n  "+folder+": ") || !strings.Contains(stderr, "no catalog in "+folder) {
		t.Errorf("code %d, stdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
}

// The folder may be relative to where the command runs. The catalog here has
// no files, so this run makes no model call.
func TestOnceWithRelativeFolder(t *testing.T) {
	writeSettings(t, stateDir(t), "", testKeys)
	folder := t.TempDir()
	if err := os.Mkdir(filepath.Join(folder, ".catalog"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(folder)

	code, stdout, stderr := runCatalogOut(context.Background(), "daemon", "--once", ".")
	if code != 0 || !strings.Contains(stdout, ": 0 files, 0 summarized, 0 waiting") {
		t.Fatalf("code %d, stdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(folder, ".catalog", "catalog.md")); err != nil {
		t.Errorf("catalog.md: %v", err)
	}
}

func TestOnceWithUnopenableStore(t *testing.T) {
	dir := stateDir(t)
	writeSettings(t, dir, "", testKeys)
	if err := os.Mkdir(filepath.Join(dir, "store.db"), 0o755); err != nil {
		t.Fatal(err)
	}

	if code, out := runCatalog("daemon", "--once", t.TempDir()); code != 1 || out == "" {
		t.Errorf("code %d, output:\n%s", code, out)
	}
}

// stopped returns a context that has already ended, as if the daemon got
// SIGTERM during its first step.
func stopped() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// With no global.toml, the daemon keeps running, so launchd doesn't restart
// it over and over, and it journals the problem for status to report.
func TestDaemonWithoutGlobalConfig(t *testing.T) {
	dir := stateDir(t)

	code, out := runCatalogIn(stopped(), "daemon")

	if code != 0 {
		t.Errorf("code %d, output:\n%s", code, out)
	}
	journal, err := os.ReadFile(filepath.Join(dir, "journal.jsonl"))
	if err != nil || !strings.Contains(string(journal), "config_error") {
		t.Errorf("journal = %q, %v; want a config error", journal, err)
	}
}

// If the daemon can't listen for status, it says so and goes on working.
func TestDaemonWithoutSocket(t *testing.T) {
	dir := stateDir(t)
	writeSettings(t, dir, "", testKeys)
	if err := os.WriteFile(filepath.Join(dir, "control.sock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	code, out := runCatalogIn(stopped(), "daemon")

	if code != 0 || !strings.Contains(out, "control.sock") {
		t.Errorf("code %d, output:\n%s", code, out)
	}
}

// installLines is what status prints about the install in a home with no
// settings files, with agent saying what the first line says.
func installLines(t *testing.T, agent string) string {
	t.Helper()
	home := os.Getenv("HOME")
	return "Launch agent: " + agent + "\n" +
		"Settings: " + home + "/Library/Application Support/com.westarete.catalog/global.toml (not found; run catalog setup)\n" +
		"Keys: " + home + "/Library/Application Support/com.westarete.catalog/keys.toml (not found; run catalog setup)\n" +
		"Log: " + home + "/Library/Logs/catalog.log\n"
}

func TestStatusWhenDaemonIsNotRunning(t *testing.T) {
	stateDir(t)
	onMacForTest(t, true)

	code, stdout, _ := runCatalogOut(context.Background(), "status")

	want := installLines(t, "not installed; run catalog setup") + "\nThe catalog daemon isn't running.\n"
	if code != 1 || stdout != want {
		t.Errorf("code %d, stdout %q, want %q", code, stdout, want)
	}
}

// Something at the socket's path that isn't a daemon is an error, not a
// daemon that isn't running.
func TestStatusWithBrokenSocket(t *testing.T) {
	dir := stateDir(t)
	writeGlobal(t, dir, "")
	if err := os.WriteFile(filepath.Join(dir, "control.sock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runCatalogOut(context.Background(), "status")

	if code != 1 || !strings.HasSuffix(stdout, "\n\n") || strings.Contains(stdout, "isn't running") || !strings.Contains(stderr, "control.sock is not a socket") {
		t.Errorf("code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

// A daemon started without global.toml answers status with the problem, and
// removes its socket when it stops.
func TestStatusOfRunningDaemon(t *testing.T) {
	dir := stateDir(t)
	onMacForTest(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	exited := make(chan int, 1)
	go func() {
		code, _ := runCatalogIn(ctx, "daemon")
		exited <- code
	}()

	var stdout string
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		var code int
		code, stdout, _ = runCatalogOut(context.Background(), "status")
		if code == 0 && strings.Contains(stdout, "global.toml") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status never reported global.toml; last: code %d\n%s", code, stdout)
		}
	}
	cancel()

	if !strings.HasPrefix(stdout, installLines(t, "not installed; run catalog setup")+"\n") {
		t.Errorf("status doesn't begin with the install lines and a blank line:\n%s", stdout)
	}
	for _, want := range []string{"Config error: " + filepath.Join(dir, "global.toml") + ": not found", "Not summarizing:"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("status doesn't contain %q:\n%s", want, stdout)
		}
	}
	if code := <-exited; code != 0 {
		t.Errorf("daemon exited %d", code)
	}
	if _, err := os.Lstat(filepath.Join(dir, "control.sock")); !os.IsNotExist(err) {
		t.Errorf("socket after exit: %v", err)
	}
}

func TestDaemonWithUnopenableStore(t *testing.T) {
	dir := stateDir(t)
	writeSettings(t, dir, "", testKeys)
	if err := os.Mkdir(filepath.Join(dir, "store.db"), 0o755); err != nil {
		t.Fatal(err)
	}

	if code, out := runCatalogIn(stopped(), "daemon"); code != 1 || out == "" {
		t.Errorf("code %d, output:\n%s", code, out)
	}
}

// The daemon crawls the folders global.toml watches at once, before the
// first tick, and stops when its context ends.
func TestDaemonCrawlsWatchedFolders(t *testing.T) {
	dir := stateDir(t)
	watched := t.TempDir()
	if err := os.MkdirAll(filepath.Join(watched, ".catalog"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(watched, "a.md"), []byte("first line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSettings(t, dir, fmt.Sprintf("[[watch]]\nfolder = %q\n", watched), testKeys)

	if code, out := runCatalogIn(stopped(), "daemon"); code != 0 {
		t.Fatalf("code %d, output:\n%s", code, out)
	}

	data, err := os.ReadFile(filepath.Join(watched, ".catalog", "catalog.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "first line") {
		t.Errorf("catalog.md:\n%s", data)
	}
}

// A second daemon would crawl and summarize everything the first one does,
// so it stops before crawling.
func TestDaemonWhenAnotherIsRunning(t *testing.T) {
	dir := stateDir(t)
	watched := t.TempDir()
	if err := os.MkdirAll(filepath.Join(watched, ".catalog"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSettings(t, dir, fmt.Sprintf("[[watch]]\nfolder = %q\n", watched), testKeys)
	socket := filepath.Join(dir, "control.sock")
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- control.Serve(ctx, socket, func() control.Snapshot { return control.Snapshot{} }) }()
	t.Cleanup(func() {
		cancel()
		if err := <-served; err != nil {
			t.Errorf("stand-in daemon: %v", err)
		}
	})
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if _, err := control.Ask(socket); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the stand-in daemon never answered")
		}
	}

	code, out := runCatalogIn(stopped(), "daemon")

	if code != 1 || !strings.Contains(out, "already running") {
		t.Errorf("code %d, output:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(watched, ".catalog", "catalog.md")); !os.IsNotExist(err) {
		t.Errorf("the second daemon crawled: catalog.md err = %v", err)
	}
}

// The state folder holds the API key, the store, and the journal. Earlier
// setup instructions created it readable by anyone, so the command makes it
// private.
func TestStateFolderOnlyOwnerCanRead(t *testing.T) {
	dir := stateDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSettings(t, dir, "", testKeys)

	runCatalog("daemon", "--once", t.TempDir())

	assertMode(t, dir, 0o700)
}

// A journal and log that an earlier version left readable by anyone become
// private when the daemon starts, even with no events to record and no
// settings file yet.
func TestOldStateFilesBecomePrivate(t *testing.T) {
	dir := stateDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(dir, "journal.jsonl")
	if err := os.WriteFile(journal, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	runCatalog("daemon", "--once", t.TempDir())

	assertMode(t, dir, 0o700)
	assertMode(t, journal, 0o600)
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != want {
		t.Errorf("%s mode = %o, want %o", filepath.Base(path), mode, want)
	}
}

// onMacForTest sets whether the command believes it runs on a Mac, and
// replaces launchctl with a stand-in that records its calls and never runs
// the real program. It returns the recorded calls.
func onMacForTest(t *testing.T, mac bool) *[][]string {
	t.Helper()
	var calls [][]string
	oldMac, oldLaunchctl := onMac, launchctl
	t.Cleanup(func() { onMac, launchctl = oldMac, oldLaunchctl })
	onMac = mac
	launchctl = func(args ...string) error {
		calls = append(calls, args)
		if len(args) > 0 && args[0] == "print" {
			return errors.New("not loaded")
		}
		return nil
	}
	return &calls
}

// testProgram is the path setup writes into the launch agent when the test
// binary runs it.
func testProgram(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	program, err := filepath.EvalSymlinks(exe)
	if err != nil {
		t.Fatal(err)
	}
	return program
}

func homeOf(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	return home
}

func plistOf(t *testing.T) string {
	t.Helper()
	return filepath.Join(homeOf(t), "Library", "LaunchAgents", "com.westarete.catalog.plist")
}

func TestSetupWritesSettingsAndLoadsAgent(t *testing.T) {
	dir := stateDir(t)
	calls := onMacForTest(t, true)

	code, stdout, stderr := runCatalogWith(context.Background(), strings.NewReader("sk-test\n~/src\n\n"), "setup")

	if code != 0 {
		t.Fatalf("code %d, stdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertMode(t, filepath.Join(dir, "global.toml"), 0o600)
	keys := filepath.Join(dir, "keys.toml")
	assertMode(t, keys, 0o600)
	data, err := os.ReadFile(keys)
	if err != nil || !strings.Contains(string(data), "sk-test") {
		t.Errorf("keys.toml = %q, %v; want the key", data, err)
	}
	assertMode(t, plistOf(t), 0o600)
	plist, err := os.ReadFile(plistOf(t))
	if err != nil || !strings.Contains(string(plist), testProgram(t)) {
		t.Errorf("plist = %q, %v; want the test binary %s", plist, err, testProgram(t))
	}
	if !strings.HasSuffix(stdout, "The catalog daemon is running. To see what it's doing, run `catalog status`.\n") {
		t.Errorf("stdout doesn't end with the running line:\n%s", stdout)
	}
	if strings.Contains(stdout, "sk-test") || strings.Contains(stderr, "sk-test") {
		t.Error("the API key appeared in the output")
	}
	if strings.Contains(stdout, "The daemon was running") {
		t.Errorf("stdout names a previous program on a first setup:\n%s", stdout)
	}
	if len(*calls) == 0 || (*calls)[len(*calls)-1][0] != "bootstrap" {
		t.Errorf("launchctl calls %v; want the last to be bootstrap", *calls)
	}
}

// The daemon reads the file setup writes.
func TestSetupSettingsLoadForTheDaemon(t *testing.T) {
	dir := stateDir(t)
	onMacForTest(t, true)

	if code, _, _ := runCatalogWith(context.Background(), strings.NewReader("sk-test\n~/src\n\n"), "setup"); code != 0 {
		t.Fatalf("code %d", code)
	}

	g, key, err := loadSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(homeOf(t), "src")}
	if key != "sk-test" || len(g.Watch) != 1 || g.Watch[0] != want[0] {
		t.Errorf("loadSettings = key %q, watch %v; want sk-test and %v", key, g.Watch, want)
	}
}

// failingReader fails the test if anything reads it.
type failingReader struct{ t *testing.T }

func (r failingReader) Read([]byte) (int, error) {
	r.t.Error("setup read standard input")
	return 0, errors.New("must not be read")
}

func TestSetupAgentOnlyReadsNothing(t *testing.T) {
	dir := stateDir(t)
	onMacForTest(t, true)

	code, stdout, stderr := runCatalogWith(context.Background(), failingReader{t}, "setup", "--agent-only")

	if code != 0 {
		t.Fatalf("code %d, stdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	for _, name := range []string{"global.toml", "keys.toml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s: %v; want it not created", name, err)
		}
	}
	if _, err := os.Stat(plistOf(t)); err != nil {
		t.Errorf("plist: %v", err)
	}
}

func TestSetupWithExistingSettings(t *testing.T) {
	dir := stateDir(t)
	onMacForTest(t, true)
	const global, keys = "model = \"old\"\n", "default = \"old\"\n"
	writeSettings(t, dir, global, keys)

	code, stdout, _ := runCatalogWith(context.Background(), failingReader{t}, "setup")

	if code != 0 {
		t.Fatalf("code %d, stdout:\n%s", code, stdout)
	}
	for name, text := range map[string]string{"global.toml": global, "keys.toml": keys} {
		if data, _ := os.ReadFile(filepath.Join(dir, name)); string(data) != text {
			t.Errorf("%s = %q, want it unchanged", name, data)
		}
	}
	if !strings.Contains(stdout, "Your settings are already in ") || !strings.Contains(stdout, "Your API keys are already in ") {
		t.Errorf("stdout:\n%s", stdout)
	}
}

// Setup never asks for a key when keys.toml exists. If the file lacks the
// key global.toml asks for, or default when global.toml is missing or
// doesn't load, setup says how to add it, and never prints the name
// global.toml gives.
func TestSetupSaysHowToAddMissingKey(t *testing.T) {
	const (
		addDefault = "To add your API key, set default in %s.\n"
		addNamed   = "To add your API key, add it to %s under the name global.toml asks for.\n"
		already    = "Your API keys are already in %s. To change them, edit that file.\n"
	)
	for name, c := range map[string]struct {
		global, keys, want string
		writeGlobal        bool
		keysMode           os.FileMode
	}{
		"no default key":             {`model = "x"`, `other = "sk-secret"`, addDefault, true, 0o600},
		"no global.toml":             {"", `other = "sk-secret"`, addDefault, false, 0o600},
		"global.toml doesn't load":   {"model = [[[", `other = "sk-secret"`, addDefault, true, 0o600},
		"no key under the named one": {`key = "sk-pasted"`, `default = "sk-secret"`, addNamed, true, 0o600},
		"keys.toml can't be read":    {`model = "x"`, `other = "sk-secret"`, already, true, 0o604},
	} {
		t.Run(name, func(t *testing.T) {
			dir := stateDir(t)
			onMacForTest(t, false)
			if c.writeGlobal {
				writeGlobal(t, dir, c.global)
			}
			writeKeys(t, dir, c.keys)
			if err := os.Chmod(filepath.Join(dir, "keys.toml"), c.keysMode); err != nil {
				t.Fatal(err)
			}

			code, stdout, stderr := runCatalogWith(context.Background(), strings.NewReader("~/src\n\n"), "setup")

			keys := filepath.Join(dir, "keys.toml")
			if code != 0 || !strings.Contains(stdout, fmt.Sprintf(c.want, keys)) {
				t.Errorf("code %d, stdout:\n%s\nstderr:\n%s\nwant %q", code, stdout, stderr, fmt.Sprintf(c.want, keys))
			}
			if strings.Contains(stdout, "API key: ") {
				t.Errorf("asked for a key:\n%s", stdout)
			}
			if strings.Contains(stdout+stderr, "sk-") {
				t.Errorf("output names a key:\n%s%s", stdout, stderr)
			}
			if data, _ := os.ReadFile(keys); string(data) != c.keys {
				t.Errorf("keys.toml = %q, want it unchanged", data)
			}
		})
	}
}

func TestSetupSaysWhenProgramChanges(t *testing.T) {
	stateDir(t)
	onMacForTest(t, true)
	if code, _, _ := runCatalogWith(context.Background(), strings.NewReader(""), "setup", "--agent-only"); code != 0 {
		t.Fatal("first setup failed")
	}
	// Point the agent at another program, as an older install would.
	plist, err := os.ReadFile(plistOf(t))
	if err != nil {
		t.Fatal(err)
	}
	old := strings.ReplaceAll(string(plist), testProgram(t), "/old/catalog")
	if err := os.WriteFile(plistOf(t), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runCatalogWith(context.Background(), strings.NewReader(""), "setup", "--agent-only")

	want := "The daemon was running /old/catalog, and now it runs " + testProgram(t) + ".\n" +
		"The catalog daemon is running. To see what it's doing, run `catalog status`.\n"
	if code != 0 || !strings.HasSuffix(stdout, want) {
		t.Errorf("code %d, stdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
}

func TestSetupNotOnMac(t *testing.T) {
	dir := stateDir(t)
	calls := onMacForTest(t, false)

	code, stdout, stderr := runCatalogWith(context.Background(), strings.NewReader("sk-test\n\n"), "setup")

	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "global.toml")); err != nil {
		t.Errorf("global.toml: %v", err)
	}
	if !strings.HasSuffix(stdout, "Setup can start the daemon only on a Mac.\n") {
		t.Errorf("stdout:\n%s", stdout)
	}
	if strings.Contains(stdout, "is running") {
		t.Errorf("stdout claims the daemon runs:\n%s", stdout)
	}
	if len(*calls) != 0 {
		t.Errorf("launchctl called: %v", *calls)
	}
	if _, err := os.Stat(plistOf(t)); !os.IsNotExist(err) {
		t.Errorf("plist: %v; want none off a Mac", err)
	}
}

func TestSetupLaunchctlFails(t *testing.T) {
	stateDir(t)
	onMacForTest(t, true)
	launchctl = func(args ...string) error {
		if args[0] == "bootstrap" {
			return errors.New("bootstrap refused")
		}
		return errors.New("not loaded")
	}

	code, stdout, stderr := runCatalogWith(context.Background(), strings.NewReader(""), "setup", "--agent-only")

	if code != 1 || !strings.HasPrefix(stderr, "catalog: ") || !strings.Contains(stderr, "bootstrap refused") {
		t.Errorf("code %d, stderr %q", code, stderr)
	}
	if strings.Contains(stdout, "is running") {
		t.Errorf("stdout claims the daemon runs:\n%s", stdout)
	}
}

// A read error other than the end of input stops setup before it writes
// global.toml.
func TestSetupReportsReadError(t *testing.T) {
	dir := stateDir(t)
	onMacForTest(t, true)

	code, _, stderr := runCatalogWith(context.Background(), io.MultiReader(strings.NewReader("sk-test\n"), errReader{}), "setup")

	if code != 1 || !strings.HasPrefix(stderr, "catalog: ") {
		t.Errorf("code %d, stderr %q", code, stderr)
	}
	for _, name := range []string{"global.toml", "keys.toml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s: %v; want it not written", name, err)
		}
	}
	if strings.Contains(stderr, "sk-test") {
		t.Error("the API key appeared in stderr")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("terminal gone") }

func TestSetupStateFolderPrivate(t *testing.T) {
	dir := stateDir(t)
	onMacForTest(t, true)
	old := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(old) })

	code, _, stderr := runCatalogWith(context.Background(), strings.NewReader("k\n\n"), "setup")

	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	assertMode(t, dir, 0o700)
	assertMode(t, filepath.Join(dir, "global.toml"), 0o600)
	assertMode(t, filepath.Join(dir, "keys.toml"), 0o600)
}

// Status never shows an API key, even from files it can't parse.
func TestStatusNeverShowsKeyFromBrokenSettings(t *testing.T) {
	dir := stateDir(t)
	onMacForTest(t, true)
	writeSettings(t, dir, "key = \"sk-secret\"\nthis line doesn't parse\n", "default = \"sk-secret\"\nthis line doesn't parse\n")

	_, stdout, stderr := runCatalogOut(context.Background(), "status")

	if strings.Count(stdout, "(can't be read:") != 2 {
		t.Errorf("stdout:\n%s", stdout)
	}
	if strings.Contains(stdout, "sk-secret") || strings.Contains(stderr, "sk-secret") {
		t.Error("status showed the API key")
	}
}

// The Keys line says whether keys.toml has the key global.toml asks for,
// and never names that key.
func TestStatusSaysWhetherKeyIsSet(t *testing.T) {
	dir := stateDir(t)
	onMacForTest(t, false)
	keys := filepath.Join(dir, "keys.toml")
	for _, c := range []struct{ global, keys, want string }{
		{"", `default = "sk-secret"`, keys + " (key set)"},
		{`key = "acme"`, `acme = "sk-secret"`, keys + " (key set)"},
		{`key = "acme"`, `default = "sk-secret"`, keys + " (no key)"},
		{"", `acme = "sk-secret"`, keys + " (no key)"},
		{"model = [[[", `default = "sk-secret"`, keys + " (key set)"},
	} {
		writeSettings(t, dir, c.global, c.keys)
		_, stdout, _ := runCatalogOut(context.Background(), "status")
		if !strings.Contains(stdout, "Keys: "+c.want+"\n") || strings.Contains(stdout, "Launch agent:") {
			t.Errorf("global %q, keys %q: stdout:\n%s\nwant Keys: %s and no launch agent line", c.global, c.keys, stdout, c.want)
		}
		if strings.Contains(stdout, "sk-secret") || strings.Contains(stdout, "acme") {
			t.Errorf("global %q, keys %q: status named a key:\n%s", c.global, c.keys, stdout)
		}
	}
}

func TestStatusSaysSettingsAreValid(t *testing.T) {
	dir := stateDir(t)
	onMacForTest(t, false)
	writeSettings(t, dir, `model = "x"`, testKeys)

	_, stdout, _ := runCatalogOut(context.Background(), "status")

	if want := "Settings: " + filepath.Join(dir, "global.toml") + " (valid)\n"; !strings.Contains(stdout, want) {
		t.Errorf("stdout:\n%s\nwant %q", stdout, want)
	}
}

func TestSettingsStateReadError(t *testing.T) {
	dir := t.TempDir()
	// A folder where a file should be makes the read fail with something
	// other than "not exist".
	for _, name := range []string{"global.toml", "keys.toml"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	s, k := settingsState(dir)

	if !s.Found || s.Err == nil {
		t.Errorf("settings = %+v; want Found with an error", s)
	}
	if !k.Found || k.Err == nil || k.HasKey {
		t.Errorf("keys = %+v; want Found with an error", k)
	}
}

// blockingReader blocks until the test ends, as a terminal does while a
// person hasn't typed.
type blockingReader struct{ done <-chan struct{} }

func (r blockingReader) Read([]byte) (int, error) {
	<-r.done
	return 0, io.EOF
}

// Ctrl-C ends the context. Setup stops at the key prompt or at a folder
// prompt, writes nothing, and loads nothing.
func TestSetupStopsWhenInterrupted(t *testing.T) {
	for name, typed := range map[string]string{"key prompt": "", "folder prompt": "sk-test\n"} {
		t.Run(name, func(t *testing.T) {
			dir := stateDir(t)
			calls := onMacForTest(t, true)
			done := make(chan struct{})
			t.Cleanup(func() { close(done) })
			stdin := io.MultiReader(strings.NewReader(typed), blockingReader{done})
			ctx, cancel := context.WithCancel(context.Background())
			time.AfterFunc(50*time.Millisecond, cancel)

			finished := make(chan struct{})
			var code int
			var stdout, stderr string
			go func() {
				defer close(finished)
				code, stdout, stderr = runCatalogWith(ctx, stdin, "setup")
			}()
			select {
			case <-finished:
			case <-time.After(5 * time.Second):
				t.Fatal("setup didn't return after the context ended")
			}

			if code != 1 || !strings.Contains(stderr, "catalog: setup interrupted") {
				t.Errorf("code %d, stderr %q", code, stderr)
			}
			if strings.Contains(stdout+stderr, "sk-test") {
				t.Error("the API key appeared in the output")
			}
			for _, name := range []string{"global.toml", "keys.toml"} {
				if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
					t.Errorf("%s: %v; want none", name, err)
				}
			}
			if _, err := os.Stat(plistOf(t)); !os.IsNotExist(err) {
				t.Errorf("plist: %v; want none", err)
			}
			if len(*calls) != 0 {
				t.Errorf("launchctl called: %v", *calls)
			}
		})
	}
}

func TestOnceRefusesWorldAccessibleGlobal(t *testing.T) {
	dir := stateDir(t)
	writeSettings(t, dir, `key = "sk-secret"`, `default = "sk-secret"`)
	if err := os.Chmod(filepath.Join(dir, "global.toml"), 0o604); err != nil {
		t.Fatal(err)
	}

	code, out := runCatalog("daemon", "--once", t.TempDir())

	if code == 0 || !strings.Contains(out, "world-accessible") || strings.Contains(out, "sk-secret") {
		t.Errorf("code %d, output:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "store.db")); !os.IsNotExist(err) {
		t.Errorf("store.db created before the mode was checked: %v", err)
	}
}

func TestStatusSaysGlobalIsWorldAccessible(t *testing.T) {
	dir := stateDir(t)
	onMacForTest(t, false)
	writeGlobal(t, dir, `key = "sk-secret"`)
	if err := os.Chmod(filepath.Join(dir, "global.toml"), 0o602); err != nil {
		t.Fatal(err)
	}

	_, stdout, _ := runCatalogOut(context.Background(), "status")

	if !strings.Contains(stdout, "(can't be read: world-accessible") || strings.Contains(stdout, "sk-secret") {
		t.Errorf("stdout:\n%s", stdout)
	}
}

func TestStatusSaysKeysIsWorldAccessible(t *testing.T) {
	dir := stateDir(t)
	onMacForTest(t, false)
	writeSettings(t, dir, "", `default = "sk-secret"`)
	if err := os.Chmod(filepath.Join(dir, "keys.toml"), 0o604); err != nil {
		t.Fatal(err)
	}

	_, stdout, _ := runCatalogOut(context.Background(), "status")

	if !strings.Contains(stdout, "keys.toml (can't be read: world-accessible") || strings.Contains(stdout, "sk-secret") {
		t.Errorf("stdout:\n%s", stdout)
	}
}
