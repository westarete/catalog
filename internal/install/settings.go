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

// ReadFolders asks for folders, one per line, until an empty line or the
// end of input. If the person lists none, it returns ~/src.
func ReadFolders(in *bufio.Reader, out io.Writer) []string {
	var folders []string
	for {
		_, _ = fmt.Fprint(out, "Folder: ")
		line, err := in.ReadString('\n')
		line = strings.TrimSpace(line)
		if line != "" {
			folders = append(folders, line)
		}
		if line == "" || err != nil {
			break
		}
	}
	if len(folders) == 0 {
		return []string{defaultFolder}
	}
	return folders
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
	if err := makePrivateDir(filepath.Dir(path)); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, privateFileMode)
	if err != nil {
		return err
	}
	// The umask can only remove bits from the mode above, so set it again.
	if err := f.Chmod(privateFileMode); err != nil {
		return errors.Join(err, f.Close())
	}
	settings := globalSettings{APIKey: apiKey}
	for _, folder := range folders {
		settings.Watch = append(settings.Watch, watchFolder{Folder: folder})
	}
	// The key goes to this file on purpose; it is the only place it is kept.
	return errors.Join(toml.NewEncoder(f).Encode(settings), f.Close()) //nolint:gosec // G117: the API key belongs in global.toml
}

// makePrivateDir creates dir and its parents if missing. It sets the mode
// of dir itself to 0700 only when it created it.
func makePrivateDir(dir string) error {
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

// AskSettings asks for an API key and folders, then writes them to
// globalPath. If globalPath exists, it says so and leaves it alone.
func AskSettings(p Prompt, globalPath string) error {
	shown := Tilde(p.Home, globalPath)
	if _, err := os.Lstat(globalPath); err == nil {
		_, _ = fmt.Fprintf(p.Out, "Your settings are already in %s. To change them, edit that file.\n", shown)
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
	folders := ReadFolders(p.In, p.Out)
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
	_, _ = fmt.Fprintf(p.Out, "Your settings are in %s, and only you can read that file.\n", shown)
	if key == "" {
		_, _ = fmt.Fprintln(p.Out, "To add your API key later, set api_key in that file.")
	}
	return nil
}

// Tilde replaces a leading home folder in path with ~.
func Tilde(home, path string) string {
	if home == "" || home == "/" {
		return path
	}
	if path == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(path, home+"/"); ok {
		return "~/" + rest
	}
	return path
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
