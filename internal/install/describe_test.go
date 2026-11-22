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

const stateFolder = "/Library/Application Support/com.westarete.catalog/"

func settingsAt(home string) Settings {
	return Settings{Path: home + stateFolder + "global.toml"}
}

func keysAt(home string) Keys {
	return Keys{Path: home + stateFolder + "keys.toml"}
}

// Describe prints absolute paths as they are, the way pwd and git do, so a
// person can paste one in quotes and an agent can open it. A path with ~
// stops working in quotes, and "Application Support" has a space.
func TestDescribeInstalled(t *testing.T) {
	home := t.TempDir()
	installPlist(t, home)
	s := settingsAt(home)
	s.Found = true
	k := keysAt(home)
	k.Found, k.HasKey = true, true

	want := "Launch agent: " + home + "/Library/LaunchAgents/com.westarete.catalog.plist\n" +
		"Settings: " + home + stateFolder + "global.toml (valid)\n" +
		"Keys: " + home + stateFolder + "keys.toml (key set)\n" +
		"Log: " + home + "/Library/Logs/catalog.log\n"
	if got := Describe(home, s, k, true); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDescribeNothingInstalled(t *testing.T) {
	home := t.TempDir()

	want := "Launch agent: not installed; run catalog setup\n" +
		"Settings: " + home + stateFolder + "global.toml (not found; run catalog setup)\n" +
		"Keys: " + home + stateFolder + "keys.toml (not found; run catalog setup)\n" +
		"Log: " + home + "/Library/Logs/catalog.log\n"
	if got := Describe(home, settingsAt(home), keysAt(home), true); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDescribeNoKey(t *testing.T) {
	home := t.TempDir()
	k := keysAt(home)
	k.Found = true

	want := "Keys: " + home + stateFolder + "keys.toml (no key)\n"
	if got := Describe(home, settingsAt(home), k, false); !containsLine(got, want) {
		t.Errorf("got:\n%s\nwant line %q", got, want)
	}
}

func TestDescribeBrokenKeys(t *testing.T) {
	home := t.TempDir()
	k := keysAt(home)
	k.Found, k.HasKey = true, true
	k.Err = errors.New("parse keys.toml: line 2: not valid TOML")

	want := "Keys: " + home + stateFolder + "keys.toml (can't be read: parse keys.toml: line 2: not valid TOML)\n"
	if got := Describe(home, settingsAt(home), k, false); !containsLine(got, want) {
		t.Errorf("got:\n%s\nwant line %q", got, want)
	}
}

func TestDescribeBrokenSettings(t *testing.T) {
	home := t.TempDir()
	s := settingsAt(home)
	s.Found = true
	s.Err = errors.New(`line 2: bad value for key "x"`)

	want := "Settings: " + home + stateFolder + "global.toml (can't be read: line 2: bad value for key \"x\")\n"
	if got := Describe(home, s, keysAt(home), false); !containsLine(got, want) {
		t.Errorf("got:\n%s\nwant line %q", got, want)
	}
}

func TestDescribeNotOnMac(t *testing.T) {
	home := t.TempDir()
	installPlist(t, home)

	want := "Settings: " + home + stateFolder + "global.toml (not found; run catalog setup)\n" +
		"Keys: " + home + stateFolder + "keys.toml (not found; run catalog setup)\n" +
		"Log: " + home + "/Library/Logs/catalog.log\n"
	if got := Describe(home, settingsAt(home), keysAt(home), false); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func containsLine(text, line string) bool {
	return strings.HasPrefix(text, line) || strings.Contains(text, "\n"+line)
}
