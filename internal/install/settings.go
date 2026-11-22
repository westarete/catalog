package install

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// defaultFolder is the one folder watched when a person lists none.
const defaultFolder = "~/src"

// Prompt holds what AskSettings needs to talk to a person.
type Prompt struct {
	In  *bufio.Reader
	Out io.Writer
	// ReadKey reads the API key without showing it. It never writes to Out.
	ReadKey func() (string, error)
	// Home is the person's home folder.
	Home string
}

// isFullPath reports whether the daemon, which launchd starts in /, will
// find folder wherever it is started: it starts with / or ~/, or is ~.
func isFullPath(folder string) bool {
	return folder == "~" || strings.HasPrefix(folder, "/") || strings.HasPrefix(folder, "~/")
}

// ReadFolders asks for folders, one per line, until an empty line or the
// end of input. It rejects a folder that isn't a full path and asks again,
// and the rejected line doesn't end the list. If the person lists none, it returns ~/src. Any other read
// error is returned, so a broken terminal never picks a folder for them.
func ReadFolders(in *bufio.Reader, out io.Writer) ([]string, error) {
	var folders []string
	for {
		_, _ = fmt.Fprint(out, "Folder: ")
		line, err := in.ReadString('\n')
		line = strings.TrimSpace(line)
		blank := line == ""
		if !blank && isFullPath(line) {
			folders = append(folders, line)
		} else if !blank {
			_, _ = fmt.Fprintln(out, "Enter a full path that starts with / or ~/.")
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read folders: %w", err)
		}
		if blank {
			break
		}
	}
	if len(folders) == 0 {
		return []string{defaultFolder}, nil
	}
	return folders, nil
}

// encodeSettings writes v as TOML. Tests replace it to force a failed write.
var encodeSettings = func(w io.Writer, v any) error {
	return toml.NewEncoder(w).Encode(v)
}

type globalSettings struct {
	APIKey string        `toml:"api_key,omitempty"`
	Watch  []watchFolder `toml:"watch"`
}

type watchFolder struct {
	Folder string `toml:"folder"`
}

// WriteGlobal creates the global settings file at path, readable only by
// its owner, with the API key (if any) and a watch entry for each folder.
// It never changes a file that exists; it returns an error wrapping
// fs.ErrExist instead. It creates the file's folder, closed to other
// accounts, if that is missing.
func WriteGlobal(path, apiKey string, folders []string) error {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, privateFileMode)
	if err != nil {
		return err
	}
	// Setup never repairs a file that exists, so a partial one must not
	// stay behind. O_EXCL means this call created the file, so it is ours
	// to remove.
	discard := func(cause error) error {
		return errors.Join(cause, f.Close(), os.Remove(path))
	}
	// The umask can only remove bits from the mode above, so set it again.
	if err := f.Chmod(privateFileMode); err != nil {
		return discard(err)
	}
	settings := globalSettings{APIKey: apiKey}
	for _, folder := range folders {
		settings.Watch = append(settings.Watch, watchFolder{Folder: folder})
	}
	// The key goes to this file on purpose; it is the only place it is kept.
	if err := encodeSettings(f, settings); err != nil { //nolint:gosec // G117: the API key belongs in global.toml
		return discard(err)
	}
	if err := f.Close(); err != nil {
		return errors.Join(err, os.Remove(path))
	}
	return nil
}

// AskSettings asks for an API key and folders, then writes them to
// globalPath. If globalPath exists, it says so and leaves it alone.
func AskSettings(p Prompt, globalPath string) error {
	if _, err := os.Stat(globalPath); err == nil {
		_, _ = fmt.Fprintf(p.Out, "Your settings are already in %s. To change them, edit that file.\n", globalPath)
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	_, _ = fmt.Fprint(p.Out, "Paste your Anthropic API key and press Return, or press Return to skip this step.\nAPI key: ")
	key, err := p.ReadKey()
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read API key: %w", err)
	}
	key = strings.TrimSpace(key)
	// Echo is off in a terminal: the first newline ends the prompt line and
	// the second leaves a blank line.
	_, _ = fmt.Fprint(p.Out, "\n\n")

	_, _ = fmt.Fprint(p.Out, "The daemon will watch the folders you list. Don't list a folder that a cloud drive syncs, or Desktop, Documents, or Downloads.\n"+
		"Enter one folder per line, then press Return on an empty line to finish. To watch only ~/src, press Return now.\n")
	folders, err := ReadFolders(p.In, p.Out)
	if err != nil {
		return err
	}
	for _, folder := range folders {
		if _, err := os.Stat(expandTilde(p.Home, folder)); errors.Is(err, fs.ErrNotExist) {
			_, _ = fmt.Fprintf(p.Out, "%s doesn't exist yet. The daemon will start watching it once you create it.\n", folder)
		}
	}
	// The last empty Return was echoed, so this newline is a blank line.
	_, _ = fmt.Fprintln(p.Out)

	if err := WriteGlobal(globalPath, key, folders); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(p.Out, "Your settings are in %s, and only you can read that file.\n", globalPath)
	if key == "" {
		_, _ = fmt.Fprintln(p.Out, "To add your API key later, set api_key in that file.")
	}
	return nil
}

// expandTilde replaces a leading ~ in path with home.
func expandTilde(home, path string) string {
	if path == "~" {
		return home
	}
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(home, rest)
	}
	return path
}
