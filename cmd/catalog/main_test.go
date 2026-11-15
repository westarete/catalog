package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stateDir points the state folder at a temp home and returns it.
func stateDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return filepath.Join(home, "Library", "Application Support", "com.westarete.catalog")
}

func runCatalog(args ...string) (int, string) {
	var stderr bytes.Buffer
	code := run(args, &stderr)
	return code, stderr.String()
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
		{"status"},
		{"daemon"},
		{"daemon", "--once"},
		{"daemon", "--once", "a", "b"},
		{"daemon", "--bogus"},
	}
	for _, args := range tests {
		code, out := runCatalog(args...)
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
