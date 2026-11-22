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

// defaultKeyName is the name setup gives the API key in keys.toml, the one
// the daemon uses when global.toml names none.
const defaultKeyName = "default"

// keysComment is all setup writes to keys.toml when the person skips the
// key. The file then exists with a private mode, so a key added by hand
// later is never in a file others can read.
const keysComment = `# Your Anthropic API keys, one per line, by name. The daemon uses
# the one named default unless global.toml names another with key.
# default = "your-key"
`

// WriteGlobal creates the global settings file at path, readable only by
// its owner, with a watch list of the folders, one per line. It never
// changes a file that exists; it returns an error wrapping fs.ErrExist
// instead. It creates the file's folder, closed to other accounts, if that
// is missing.
func WriteGlobal(path string, folders []string) error {
	return createPrivateFile(path, func(w io.Writer) error {
		var b strings.Builder
		b.WriteString("watch = [\n")
		for _, folder := range folders {
			b.WriteString("  " + tomlString(folder) + ",\n")
		}
		b.WriteString("]\n")
		_, err := io.WriteString(w, b.String())
		return err
	})
}

// WriteKeys creates the API keys file at path, readable only by its owner,
// with apiKey as the default key, or, if apiKey is empty, only a comment
// that says how to add one. Like WriteGlobal, it never changes a file that
// exists and creates a missing folder closed to other accounts.
func WriteKeys(path, apiKey string) error {
	return createPrivateFile(path, func(w io.Writer) error {
		text := keysComment
		if apiKey != "" {
			text = defaultKeyName + " = " + tomlString(apiKey) + "\n"
		}
		_, err := io.WriteString(w, text)
		return err
	})
}

// tomlString returns s as a TOML basic string. It escapes the quotation
// mark, the backslash, and the control characters TOML doesn't allow in a
// basic string, and keeps every other byte as it is.
func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := range len(s) {
		c := s[i]
		if c == '"' || c == '\\' {
			b.WriteByte('\\')
			b.WriteByte(c)
			continue
		}
		if c < 0x20 || c == 0x7f {
			fmt.Fprintf(&b, `\u%04X`, c)
			continue
		}
		b.WriteByte(c)
	}
	b.WriteByte('"')
	return b.String()
}

// createPrivateFile creates the file at path, readable only by its owner,
// and fills it with write. It never changes a file that exists; it returns
// an error wrapping fs.ErrExist instead. If write fails, it removes the
// file. It creates the file's folder, closed to other accounts, if that is
// missing.
func createPrivateFile(path string, write func(io.Writer) error) error {
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
	if err := write(f); err != nil {
		return discard(err)
	}
	if err := f.Close(); err != nil {
		return errors.Join(err, os.Remove(path))
	}
	return nil
}

// KeyCheck says whether an existing keys.toml has the key global.toml asks
// for, so AskSettings can say how to add a missing one.
type KeyCheck int

const (
	// KeyFound means keys.toml has the key.
	KeyFound KeyCheck = iota
	// KeyUnknown means keys.toml can't be read, so no one knows whether it
	// has the key; status reports why.
	KeyUnknown
	// DefaultKeyMissing means keys.toml lacks default, the key global.toml
	// asks for when it names none.
	DefaultKeyMissing
	// NamedKeyMissing means keys.toml lacks the other key global.toml names.
	NamedKeyMissing
)

// AskSettings asks for what goes in whichever of keysPath and globalPath is
// missing, an API key and then folders, and writes only those files. It
// leaves an existing file alone and says it's there. For an existing
// keys.toml, it calls checkKeys and, if the key global.toml asks for is
// missing, says how to add it, never naming a key other than default.
func AskSettings(p Prompt, globalPath, keysPath string, checkKeys func() KeyCheck) error {
	needGlobal, err := isMissing(globalPath)
	if err != nil {
		return err
	}
	needKeys, err := isMissing(keysPath)
	if err != nil {
		return err
	}
	if !needGlobal {
		_, _ = fmt.Fprintf(p.Out, "Your settings are already in %s. To change them, edit that file.\n", globalPath)
	}
	if !needKeys {
		_, _ = fmt.Fprintf(p.Out, existingKeysMessage(checkKeys()), keysPath)
	}

	var key string
	if needKeys {
		if key, err = askKey(p); err != nil {
			return err
		}
	}
	var folders []string
	if needGlobal {
		if folders, err = askFolders(p); err != nil {
			return err
		}
	}

	if needKeys {
		if err := WriteKeys(keysPath, key); err != nil {
			return err
		}
		if key != "" {
			_, _ = fmt.Fprintf(p.Out, "Your API key is in %s, and only you can read that file.\n", keysPath)
		} else {
			_, _ = fmt.Fprintf(p.Out, "To add your API key later, set default in %s.\n", keysPath)
		}
	}
	if needGlobal {
		if err := WriteGlobal(globalPath, folders); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(p.Out, "Your settings are in %s, and only you can read that file.\n", globalPath)
	}
	return nil
}

// existingKeysMessage is what AskSettings says about an existing keys.toml,
// as a format for its path.
func existingKeysMessage(check KeyCheck) string {
	switch check {
	case DefaultKeyMissing:
		return "To add your API key, set default in %s.\n"
	case NamedKeyMissing:
		return "To add your API key, add it to %s under the name global.toml asks for.\n"
	case KeyFound, KeyUnknown:
	}
	return "Your API keys are already in %s. To change them, edit that file.\n"
}

// isMissing reports whether nothing is at path. A dangling link counts as
// missing, so setup tries to create the file and reports the link.
func isMissing(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	return false, err
}

// askKey asks for the API key and returns it trimmed, or empty if the
// person skips it.
func askKey(p Prompt) (string, error) {
	_, _ = fmt.Fprint(p.Out, "Paste your Anthropic API key and press Return, or press Return to skip this step.\nAPI key: ")
	key, err := p.ReadKey()
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read API key: %w", err)
	}
	// Echo is off in a terminal: the first newline ends the prompt line and
	// the second leaves a blank line.
	_, _ = fmt.Fprint(p.Out, "\n\n")
	return strings.TrimSpace(key), nil
}

// askFolders asks for the folders to watch and warns about any that don't
// exist yet.
func askFolders(p Prompt) ([]string, error) {
	_, _ = fmt.Fprint(p.Out, "The daemon will watch the folders you list. Don't list a folder that a cloud drive syncs, or Desktop, Documents, or Downloads.\n"+
		"Enter one folder per line, then press Return on an empty line to finish. To watch only ~/src, press Return now.\n")
	folders, err := ReadFolders(p.In, p.Out)
	if err != nil {
		return nil, err
	}
	for _, folder := range folders {
		if _, err := os.Stat(expandTilde(p.Home, folder)); errors.Is(err, fs.ErrNotExist) {
			_, _ = fmt.Fprintf(p.Out, "%s doesn't exist yet. The daemon will start watching it once you create it.\n", folder)
		}
	}
	// The last empty Return was echoed, so this newline is a blank line.
	_, _ = fmt.Fprintln(p.Out)
	return folders, nil
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
