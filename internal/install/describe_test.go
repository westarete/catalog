package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func installPlist(t *testing.T, home string) {
	t.Helper()
	path := PlistPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("<plist/>"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func settingsAt(home string) Settings {
	return Settings{Path: filepath.Join(home, "Library", "Application Support", "com.westarete.catalog", "global.toml")}
}

func TestDescribeInstalled(t *testing.T) {
	home := t.TempDir()
	installPlist(t, home)
	s := settingsAt(home)
	s.Found, s.HasKey = true, true

	want := "Launch agent: ~/Library/LaunchAgents/com.westarete.catalog.plist\n" +
		"Settings: ~/Library/Application Support/com.westarete.catalog/global.toml (API key set)\n" +
		"Log: ~/Library/Logs/catalog.log\n"
	if got := Describe(home, s, true); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDescribeNothingInstalled(t *testing.T) {
	home := t.TempDir()

	want := "Launch agent: not installed; run catalog setup\n" +
		"Settings: ~/Library/Application Support/com.westarete.catalog/global.toml (not found; run catalog setup)\n" +
		"Log: ~/Library/Logs/catalog.log\n"
	if got := Describe(home, settingsAt(home), true); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDescribeNoAPIKey(t *testing.T) {
	home := t.TempDir()
	s := settingsAt(home)
	s.Found = true

	want := "Settings: ~/Library/Application Support/com.westarete.catalog/global.toml (no API key)\n"
	if got := Describe(home, s, false); !containsLine(got, want) {
		t.Errorf("got:\n%s\nwant line %q", got, want)
	}
}

func TestDescribeBrokenSettings(t *testing.T) {
	home := t.TempDir()
	s := settingsAt(home)
	s.Found, s.HasKey = true, true
	s.Err = errors.New(`line 2: bad value for key "x"`)

	want := "Settings: ~/Library/Application Support/com.westarete.catalog/global.toml (can't be read: line 2: bad value for key \"x\")\n"
	if got := Describe(home, s, false); !containsLine(got, want) {
		t.Errorf("got:\n%s\nwant line %q", got, want)
	}
}

func TestDescribeNotOnMac(t *testing.T) {
	home := t.TempDir()
	installPlist(t, home)

	want := "Settings: ~/Library/Application Support/com.westarete.catalog/global.toml (not found; run catalog setup)\n" +
		"Log: ~/Library/Logs/catalog.log\n"
	if got := Describe(home, settingsAt(home), false); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func containsLine(text, line string) bool {
	return strings.HasPrefix(text, line) || strings.Contains(text, "\n"+line)
}
