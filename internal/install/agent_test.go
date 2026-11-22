package install

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

// fakeLaunchctl records each call and fails the subcommands in failOn. It
// acts like launchd: the agent is loaded unless print is in failOn, and
// after bootout, print still finds it for stopping more calls while the old
// daemon exits. Until then, bootstrap fails as launchctl does, with error 5.
type fakeLaunchctl struct {
	calls     [][]string
	failOn    map[string]bool
	stopping  int
	bootedOut bool
}

func (f *fakeLaunchctl) run(args ...string) error {
	f.calls = append(f.calls, args)
	if f.failOn[args[0]] {
		return errors.New(args[0] + " failed")
	}
	switch args[0] {
	case "print":
		if !f.loaded() {
			return errors.New("could not find service")
		}
		if f.bootedOut {
			f.stopping--
		}
	case "bootout":
		f.bootedOut = true
	case "bootstrap":
		if f.loaded() {
			return errors.New("Bootstrap failed: 5: Input/output error")
		}
	}
	return nil
}

// loaded reports whether launchd still has the agent.
func (f *fakeLaunchctl) loaded() bool {
	return !f.failOn["print"] && (!f.bootedOut || f.stopping > 0)
}

// quickStop makes Install check every moment for the old agent to go, and
// give up after checks checks, for the rest of the test.
func quickStop(t *testing.T, checks int) {
	t.Helper()
	oldPoll, oldChecks := stopPoll, stopChecks
	stopPoll, stopChecks = 0, checks
	t.Cleanup(func() { stopPoll, stopChecks = oldPoll, oldChecks })
}

// parsePlist returns the property list's values by key: strings and
// integers as text, arrays as []string, booleans as "true" or "false".
func parsePlist(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test reads its own temp file
	if err != nil {
		t.Fatal(err)
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	out := map[string]any{}
	key, inArray := "", false
	var items []string
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatalf("property list is not XML: %v\n%s", err, data)
		}
		switch el := tok.(type) {
		case xml.StartElement:
			name := el.Name.Local
			switch name {
			case "key", "string", "integer":
				var text string
				if err := dec.DecodeElement(&text, &el); err != nil {
					t.Fatal(err)
				}
				switch {
				case name == "key":
					key = text
				case inArray:
					items = append(items, text)
				default:
					out[key] = text
				}
			case "true", "false":
				out[key] = name
			case "array":
				inArray, items = true, nil
			}
		case xml.EndElement:
			if el.Name.Local == "array" {
				out[key] = items
				inArray = false
			}
		}
	}
}

func newAgent(t *testing.T, f *fakeLaunchctl) Agent {
	t.Helper()
	return Agent{Home: t.TempDir(), UID: 501, Launchctl: f.run}
}

func TestInstallWritesPlist(t *testing.T) {
	f := &fakeLaunchctl{}
	a := newAgent(t, f)
	if _, err := a.Install("/opt/bin/catalog"); err != nil {
		t.Fatal(err)
	}
	got := parsePlist(t, PlistPath(a.Home))
	want := map[string]any{
		"Label":             "com.westarete.catalog",
		"ProgramArguments":  []string{"/opt/bin/catalog", "daemon"},
		"RunAtLoad":         "true",
		"KeepAlive":         "true",
		"Umask":             "63",
		"StandardOutPath":   LogPath(a.Home),
		"StandardErrorPath": LogPath(a.Home),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("property list = %v, want %v", got, want)
	}
	if Label != "com.westarete.catalog" {
		t.Errorf("Label = %q", Label)
	}
}

func TestPaths(t *testing.T) {
	if got, want := PlistPath("/h"), "/h/Library/LaunchAgents/com.westarete.catalog.plist"; got != want {
		t.Errorf("PlistPath = %q, want %q", got, want)
	}
	if got, want := LogPath("/h"), "/h/Library/Logs/catalog.log"; got != want {
		t.Errorf("LogPath = %q, want %q", got, want)
	}
}

func TestInstallEscapesPaths(t *testing.T) {
	f := &fakeLaunchctl{}
	home := filepath.Join(t.TempDir(), "a&b <c>")
	a := Agent{Home: home, UID: 501, Launchctl: f.run}
	program := filepath.Join(home, "a&b c", "catalog")
	if _, err := a.Install(program); err != nil {
		t.Fatal(err)
	}
	got := parsePlist(t, PlistPath(home))
	if args := got["ProgramArguments"].([]string); args[0] != program {
		t.Errorf("program = %q, want %q", args[0], program)
	}
	if got["StandardOutPath"] != LogPath(home) {
		t.Errorf("StandardOutPath = %q, want %q", got["StandardOutPath"], LogPath(home))
	}
}

func TestInstallModesIgnoreUmask(t *testing.T) {
	old := syscall.Umask(0)
	defer syscall.Umask(old)
	f := &fakeLaunchctl{}
	a := newAgent(t, f)
	if _, err := a.Install("/opt/bin/catalog"); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{
		PlistPath(a.Home):               0o600,
		filepath.Dir(PlistPath(a.Home)): 0o700,
		filepath.Dir(LogPath(a.Home)):   0o700,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Errorf("%s mode = %o, want %o", path, info.Mode().Perm(), want)
		}
	}
}

func TestInstallLeavesExistingFolderModes(t *testing.T) {
	f := &fakeLaunchctl{}
	a := newAgent(t, f)
	for _, dir := range []string{filepath.Dir(PlistPath(a.Home)), filepath.Dir(LogPath(a.Home))} {
		if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // test mimics macOS's own mode
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0o755); err != nil { //nolint:gosec // test mimics macOS's own mode
			t.Fatal(err)
		}
	}
	if _, err := a.Install("/opt/bin/catalog"); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{filepath.Dir(PlistPath(a.Home)), filepath.Dir(LogPath(a.Home))} {
		info, _ := os.Stat(dir)
		if info.Mode().Perm() != 0o755 {
			t.Errorf("%s mode = %o, want 755", dir, info.Mode().Perm())
		}
	}
}

func TestInstallLoadsWhenNotLoaded(t *testing.T) {
	f := &fakeLaunchctl{failOn: map[string]bool{"print": true}}
	a := newAgent(t, f)
	if _, err := a.Install("/opt/bin/catalog"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"print", "gui/501/com.westarete.catalog"},
		{"bootstrap", "gui/501", PlistPath(a.Home)},
	}
	if !reflect.DeepEqual(f.calls, want) {
		t.Errorf("calls = %v, want %v", f.calls, want)
	}
}

func TestInstallReloadsWhenLoaded(t *testing.T) {
	f := &fakeLaunchctl{}
	a := newAgent(t, f)
	if _, err := a.Install("/opt/bin/catalog"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"print", "gui/501/com.westarete.catalog"},
		{"bootout", "gui/501/com.westarete.catalog"},
		{"print", "gui/501/com.westarete.catalog"},
		{"bootstrap", "gui/501", PlistPath(a.Home)},
	}
	if !reflect.DeepEqual(f.calls, want) {
		t.Errorf("calls = %v, want %v", f.calls, want)
	}
}

// launchctl bootout can return while launchd still has the old agent, and a
// bootstrap then fails with error 5, so Install waits for it to go.
func TestInstallWaitsForOldAgentToStop(t *testing.T) {
	quickStop(t, 10)
	f := &fakeLaunchctl{stopping: 3}
	a := newAgent(t, f)
	if _, err := a.Install("/opt/bin/catalog"); err != nil {
		t.Fatal(err)
	}
	last := f.calls[len(f.calls)-1]
	if last[0] != "bootstrap" {
		t.Errorf("last call = %v, want bootstrap; calls %v", last, f.calls)
	}
}

func TestInstallFailsWhenOldAgentNeverStops(t *testing.T) {
	quickStop(t, 5)
	f := &fakeLaunchctl{stopping: 1000}
	a := newAgent(t, f)
	_, err := a.Install("/opt/bin/catalog")
	if err == nil || !strings.Contains(err.Error(), "still has") {
		t.Fatalf("err = %v, want one saying launchd still has the agent", err)
	}
	prints := 0
	for _, c := range f.calls {
		switch c[0] {
		case "bootstrap":
			t.Error("bootstrap ran though the old agent never stopped")
		case "print":
			prints++
		}
	}
	if prints != 6 {
		t.Errorf("print calls = %d, want 1 before bootout and 5 after", prints)
	}
}

func TestInstallFailsWhenBootoutFails(t *testing.T) {
	f := &fakeLaunchctl{failOn: map[string]bool{"bootout": true}}
	a := newAgent(t, f)
	_, err := a.Install("/opt/bin/catalog")
	if err == nil || !strings.Contains(err.Error(), "bootout failed") {
		t.Fatalf("err = %v, want the bootout error", err)
	}
	for _, c := range f.calls {
		if c[0] == "bootstrap" {
			t.Error("bootstrap ran after bootout failed")
		}
	}
}

func TestInstallReportsPreviousProgram(t *testing.T) {
	f := &fakeLaunchctl{}
	a := newAgent(t, f)
	if prev, err := a.Install("/old/catalog"); err != nil || prev != "" {
		t.Fatalf("first install = %q, %v; want empty, nil", prev, err)
	}
	if prev, err := a.Install("/old/catalog"); err != nil || prev != "" {
		t.Fatalf("same program = %q, %v; want empty, nil", prev, err)
	}
	if prev, err := a.Install("/new/catalog"); err != nil || prev != "/old/catalog" {
		t.Fatalf("changed program = %q, %v; want /old/catalog, nil", prev, err)
	}
}

func TestInstallIgnoresUnreadablePreviousPlist(t *testing.T) {
	f := &fakeLaunchctl{}
	a := newAgent(t, f)
	if err := os.MkdirAll(filepath.Dir(PlistPath(a.Home)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(PlistPath(a.Home), []byte("not xml <"), 0o600); err != nil {
		t.Fatal(err)
	}
	prev, err := a.Install("/new/catalog")
	if err != nil || prev != "" {
		t.Fatalf("got %q, %v; want empty, nil", prev, err)
	}
}

func TestInstallFailsWhenBootstrapFails(t *testing.T) {
	f := &fakeLaunchctl{failOn: map[string]bool{"print": true, "bootstrap": true}}
	a := newAgent(t, f)
	_, err := a.Install("/opt/bin/catalog")
	if err == nil || !strings.Contains(err.Error(), "bootstrap failed") {
		t.Fatalf("err = %v, want the bootstrap error", err)
	}
}

func TestInstallLeavesNoTempFile(t *testing.T) {
	f := &fakeLaunchctl{}
	a := newAgent(t, f)
	if _, err := a.Install("/opt/bin/catalog"); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Dir(PlistPath(a.Home)))
	if len(entries) != 1 {
		t.Errorf("LaunchAgents holds %d entries, want 1", len(entries))
	}
}

func TestRunLaunchctlReportsOutput(t *testing.T) {
	// Not the real /bin/launchctl: the exec seam is a variable.
	old := launchctlPath
	defer func() { launchctlPath = old }()
	launchctlPath = "/bin/sh"
	err := RunLaunchctl("-c", "echo boom; exit 3")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want it to include the output", err)
	}
	if err := RunLaunchctl("-c", "exit 0"); err != nil {
		t.Fatal(err)
	}
}

func TestInstallCreatesFoldersOwnerOnlyUnderOddUmask(t *testing.T) {
	f := &fakeLaunchctl{}
	a := newAgent(t, f)
	if err := os.Mkdir(filepath.Join(a.Home, "Library"), 0o700); err != nil {
		t.Fatal(err)
	}
	old := syscall.Umask(0o100) // mkdir alone would make these 0600
	defer syscall.Umask(old)
	if _, err := a.Install("/opt/bin/catalog"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Dir(PlistPath(a.Home)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("mode = %o, want 700", info.Mode().Perm())
	}
}

func TestInstallFailsWhenPlistCannotBeReplaced(t *testing.T) {
	f := &fakeLaunchctl{}
	a := newAgent(t, f)
	blocker := filepath.Join(PlistPath(a.Home), "keep")
	if err := os.MkdirAll(blocker, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Install("/opt/bin/catalog"); err == nil {
		t.Fatal("Install succeeded, want the rename error")
	}
	if len(f.calls) != 0 {
		t.Errorf("launchctl ran after the write failed: %v", f.calls)
	}
	entries, _ := os.ReadDir(filepath.Dir(PlistPath(a.Home)))
	if len(entries) != 1 {
		t.Errorf("LaunchAgents holds %d entries, want only the blocker", len(entries))
	}
}

func TestInstallIgnoresTruncatedPreviousPlist(t *testing.T) {
	f := &fakeLaunchctl{}
	a := newAgent(t, f)
	if err := os.MkdirAll(filepath.Dir(PlistPath(a.Home)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(PlistPath(a.Home), []byte("<plist><dict><key>Label"), 0o600); err != nil {
		t.Fatal(err)
	}
	if prev, err := a.Install("/new/catalog"); err != nil || prev != "" {
		t.Fatalf("got %q, %v; want empty, nil", prev, err)
	}
}
