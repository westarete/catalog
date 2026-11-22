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

func TestOnceWithoutAPIKey(t *testing.T) {
	dir := stateDir(t)
	writeGlobal(t, dir, "model = \"x\"\n")

	code, out := runCatalog("daemon", "--once", t.TempDir())

	if code == 0 || !strings.Contains(out, "api_key") {
		t.Errorf("code %d, output:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "store.db")); !os.IsNotExist(err) {
		t.Errorf("store.db created before the key was checked: %v", err)
	}
}

func TestOnceWithBadGlobalConfig(t *testing.T) {
	writeGlobal(t, stateDir(t), "api_key = [[[")

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

func writeGlobal(t *testing.T, dir, text string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "global.toml"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A folder with no catalog has nothing to summarize, so this run makes no
// model call. It's most likely the wrong folder, so the run fails.
func TestOnceWithoutCatalog(t *testing.T) {
	dir := stateDir(t)
	writeGlobal(t, dir, "api_key = \"test-key\"\n")
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
	writeGlobal(t, stateDir(t), "api_key = \"test-key\"\n")
	folder := filepath.Join(t.TempDir(), "missing")

	code, stdout, stderr := runCatalogOut(context.Background(), "daemon", "--once", folder)

	if code != 1 || !strings.Contains(stdout, "Failing:\n  "+folder+": ") || !strings.Contains(stderr, "no catalog in "+folder) {
		t.Errorf("code %d, stdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
}

// The folder may be relative to where the command runs. The catalog here has
// no files, so this run makes no model call.
func TestOnceWithRelativeFolder(t *testing.T) {
	writeGlobal(t, stateDir(t), "api_key = \"test-key\"\n")
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
	writeGlobal(t, dir, "api_key = \"test-key\"\n")
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
	writeGlobal(t, dir, "api_key = \"test-key\"\n")
	if err := os.WriteFile(filepath.Join(dir, "control.sock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	code, out := runCatalogIn(stopped(), "daemon")

	if code != 0 || !strings.Contains(out, "control.sock") {
		t.Errorf("code %d, output:\n%s", code, out)
	}
}

// installLines is what status prints about the install in a home with no
// settings file, with agent saying what the first line says.
func installLines(agent string) string {
	return "Launch agent: " + agent + "\n" +
		"Settings: ~/Library/Application Support/com.westarete.catalog/global.toml (not found; run catalog setup)\n" +
		"Log: ~/Library/Logs/catalog.log\n"
}

func TestStatusWhenDaemonIsNotRunning(t *testing.T) {
	stateDir(t)
	onMacForTest(t, true)

	code, stdout, _ := runCatalogOut(context.Background(), "status")

	want := installLines("not installed; run catalog setup") + "\nThe catalog daemon isn't running.\n"
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

	if !strings.HasPrefix(stdout, installLines("not installed; run catalog setup")+"\n") {
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
	writeGlobal(t, dir, "api_key = \"test-key\"\n")
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
	writeGlobal(t, dir, fmt.Sprintf("api_key = \"test-key\"\n[[watch]]\nfolder = %q\n", watched))

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
	writeGlobal(t, dir, fmt.Sprintf("api_key = \"test-key\"\n[[watch]]\nfolder = %q\n", watched))
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
	if err := os.WriteFile(filepath.Join(dir, "global.toml"), []byte(`api_key = "test"`), 0o600); err != nil {
		t.Fatal(err)
	}

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
	global := filepath.Join(dir, "global.toml")
	assertMode(t, global, 0o600)
	data, err := os.ReadFile(global)
	if err != nil || !strings.Contains(string(data), "sk-test") {
		t.Errorf("global.toml = %q, %v; want the key", data, err)
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

	g, err := loadGlobal(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(homeOf(t), "src")}
	if g.APIKey != "sk-test" || len(g.Watch) != 1 || g.Watch[0] != want[0] {
		t.Errorf("loadGlobal = key %q, watch %v; want sk-test and %v", g.APIKey, g.Watch, want)
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
	if _, err := os.Stat(filepath.Join(dir, "global.toml")); !os.IsNotExist(err) {
		t.Errorf("global.toml: %v; want it not created", err)
	}
	if _, err := os.Stat(plistOf(t)); err != nil {
		t.Errorf("plist: %v", err)
	}
}

func TestSetupWithExistingSettings(t *testing.T) {
	dir := stateDir(t)
	onMacForTest(t, true)
	const text = "api_key = \"old\"\n"
	writeGlobal(t, dir, text)

	code, stdout, _ := runCatalogWith(context.Background(), failingReader{t}, "setup")

	if code != 0 {
		t.Fatalf("code %d, stdout:\n%s", code, stdout)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "global.toml")); string(data) != text {
		t.Errorf("global.toml = %q, want it unchanged", data)
	}
	if !strings.Contains(stdout, "Your settings are already in ") {
		t.Errorf("stdout:\n%s", stdout)
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
	if _, err := os.Stat(filepath.Join(dir, "global.toml")); !os.IsNotExist(err) {
		t.Errorf("global.toml: %v; want it not written", err)
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
}

// Status never shows an API key, even from a file it can't parse.
func TestStatusNeverShowsKeyFromBrokenSettings(t *testing.T) {
	dir := stateDir(t)
	onMacForTest(t, true)
	writeGlobal(t, dir, "api_key = \"sk-secret\"\nthis line doesn't parse\n")

	_, stdout, stderr := runCatalogOut(context.Background(), "status")

	if !strings.Contains(stdout, "(can't be read:") {
		t.Errorf("stdout:\n%s", stdout)
	}
	if strings.Contains(stdout, "sk-secret") || strings.Contains(stderr, "sk-secret") {
		t.Error("status showed the API key")
	}
}

func TestStatusSaysWhetherKeyIsSet(t *testing.T) {
	dir := stateDir(t)
	onMacForTest(t, false)
	for text, want := range map[string]string{
		"api_key = \"k\"\n": "(API key set)",
		"model = \"x\"\n":   "(no API key)",
	} {
		writeGlobal(t, dir, text)
		_, stdout, _ := runCatalogOut(context.Background(), "status")
		if !strings.Contains(stdout, want) || strings.Contains(stdout, "Launch agent:") {
			t.Errorf("settings %q: stdout:\n%s\nwant %s and no launch agent line", text, stdout, want)
		}
	}
}

func TestSettingsStateReadError(t *testing.T) {
	dir := t.TempDir()
	// A folder where the file should be makes the read fail with something
	// other than "not exist".
	if err := os.Mkdir(filepath.Join(dir, "global.toml"), 0o700); err != nil {
		t.Fatal(err)
	}

	s := settingsState(dir)

	if !s.Found || s.Err == nil || s.HasKey {
		t.Errorf("settingsState = %+v; want Found with an error", s)
	}
}
