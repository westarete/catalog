package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	var out, errOut bytes.Buffer
	code = run(ctx, args, &out, &errOut)
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
// model call.
func TestOnceWithNothingToDo(t *testing.T) {
	dir := stateDir(t)
	writeGlobal(t, dir, "api_key = \"test-key\"\n")

	if code, out := runCatalog("daemon", "--once", t.TempDir()); code != 0 {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "store.db")); err != nil {
		t.Errorf("store.db: %v", err)
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

func TestStatusWhenDaemonIsNotRunning(t *testing.T) {
	stateDir(t)

	code, stdout, _ := runCatalogOut(context.Background(), "status")

	if code == 0 || stdout != "The catalog daemon isn't running.\n" {
		t.Errorf("code %d, stdout %q", code, stdout)
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

	if code != 1 || stdout != "" || !strings.Contains(stderr, "control.sock") {
		t.Errorf("code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

// A daemon started without global.toml answers status with the problem, and
// removes its socket when it stops.
func TestStatusOfRunningDaemon(t *testing.T) {
	dir := stateDir(t)
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
