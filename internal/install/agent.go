// Package install sets up the catalog daemon on a Mac: its launch agent,
// its settings, and a report of how it is doing.
package install

import (
	"bytes"
	_ "embed"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

// Label names the daemon's launch agent to launchd.
const Label = "com.westarete.catalog"

const (
	// privateFileMode lets only the owner read the property list.
	privateFileMode = 0o600
	// privateDirMode lets only the owner open a folder Install creates.
	privateDirMode = 0o700
)

//go:embed agent.plist.tmpl
var plistSource string

var plistTemplate = template.Must(template.New("agent").
	Funcs(template.FuncMap{"xml": escapeXML}).
	Parse(plistSource))

// PlistPath returns where the launch agent's property list lives.
func PlistPath(home string) string {
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist")
}

// LogPath returns the file the daemon's output goes to.
func LogPath(home string) string {
	return filepath.Join(home, "Library", "Logs", "catalog.log")
}

// Launchctl runs launchctl with args and returns an error for a failure.
type Launchctl func(args ...string) error

// launchctlPath is the program RunLaunchctl runs. Tests point it at a
// harmless stand-in; nothing else changes it.
var launchctlPath = "/bin/launchctl"

// RunLaunchctl runs /bin/launchctl with args. If it exits nonzero, the
// error includes the command's combined output.
func RunLaunchctl(args ...string) error {
	out, err := exec.Command(launchctlPath, args...).CombinedOutput() //nolint:gosec // fixed program path; args come from this package
	if err != nil {
		return fmt.Errorf("launchctl %s: %w: %s", strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return nil
}

// Agent installs the daemon's launch agent for one account.
type Agent struct {
	Home      string
	UID       int
	Launchctl Launchctl
}

// Install writes the property list naming program, unloads the agent if
// launchd has it loaded, and loads it again. It returns the program the
// old property list named, or "" if there was none or it named program.
// Only the owner can read or change the files and folders Install creates.
func (a Agent) Install(program string) (previous string, err error) {
	previous = programIn(PlistPath(a.Home))
	if previous == program {
		previous = ""
	}
	if err := a.writePlist(program); err != nil {
		return "", err
	}
	if err := a.reload(); err != nil {
		return "", err
	}
	return previous, nil
}

func (a Agent) writePlist(program string) error {
	for _, dir := range []string{filepath.Dir(PlistPath(a.Home)), filepath.Dir(LogPath(a.Home))} {
		if err := ensureDir(dir); err != nil {
			return err
		}
	}
	var buf bytes.Buffer
	err := plistTemplate.Execute(&buf, map[string]string{
		"Label": Label, "Program": program, "LogPath": LogPath(a.Home),
	})
	if err != nil {
		return err
	}
	return writeFileAtomically(PlistPath(a.Home), buf.Bytes())
}

// reload unloads the agent if launchd has it, then loads the new property
// list, so a running daemon restarts with the new program.
func (a Agent) reload() error {
	domain := "gui/" + strconv.Itoa(a.UID)
	service := domain + "/" + Label
	if a.Launchctl("print", service) == nil {
		if err := a.Launchctl("bootout", service); err != nil {
			return err
		}
	}
	return a.Launchctl("bootstrap", domain, PlistPath(a.Home))
}

// ensureDir creates dir with a private mode, whatever the umask. It
// leaves the mode of a folder that already exists alone.
func ensureDir(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, privateDirMode); err != nil {
		return err
	}
	// gosec's G302 expects a file; a folder needs its execute bit to be
	// opened at all, so 0700 is the private mode for one.
	return os.Chmod(dir, privateDirMode) //nolint:gosec // see above
}

// writeFileAtomically writes data through a private temporary file in
// path's folder and renames it into place, so launchd never reads a
// partial property list.
func writeFileAtomically(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".agent-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	fail := func(err error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if err := tmp.Chmod(privateFileMode); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		return fail(err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// programIn returns the first ProgramArguments string of the property list
// at path, or "" if the file is missing or isn't a property list.
func programIn(path string) string {
	data, err := os.ReadFile(path) //nolint:gosec // path is the agent's own property list
	if err != nil {
		return ""
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	afterKey := false
	for {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if afterKey && start.Name.Local == "array" {
			return firstString(dec)
		}
		if start.Name.Local == "key" {
			var key string
			if dec.DecodeElement(&key, &start) != nil {
				return ""
			}
			afterKey = key == "ProgramArguments"
		}
	}
}

// firstString returns the text of the first <string> in the current array.
func firstString(dec *xml.Decoder) string {
	for {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		if start, ok := tok.(xml.StartElement); ok && start.Name.Local == "string" {
			var s string
			if dec.DecodeElement(&s, &start) != nil {
				return ""
			}
			return s
		}
	}
}

func escapeXML(s string) (string, error) {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(s)); err != nil {
		return "", err
	}
	return buf.String(), nil
}
