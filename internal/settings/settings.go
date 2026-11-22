// Package settings reads the daemon's global and per-catalog configuration.
package settings

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"

	"github.com/BurntSushi/toml"
)

// GlobalFile is the name of the global settings file in StateDir.
const GlobalFile = "global.toml"

// CatalogConfigFile is the name of a catalog's settings file in .catalog.
const CatalogConfigFile = "config.toml"

const defaultModel = "claude-haiku-5-5"

// Global holds the settings that apply to every catalog on this machine.
type Global struct {
	// Key names the API key in keys.toml that the daemon uses. Empty means
	// DefaultKeyName.
	Key   string
	Model string
	// Include and Exclude hold only the patterns global.toml lists, not the
	// built-in ones. Lists combines them.
	Include []string
	Exclude []string
	Watch   []string
}

// Catalog holds the patterns one catalog's .catalog/config.toml lists.
type Catalog struct {
	Include []string `toml:"include"`
	Exclude []string `toml:"exclude"`
}

// BuiltinInclude and BuiltinExclude are the patterns every catalog starts
// with. Treat them as read-only; Lists copies them.
var (
	BuiltinInclude = []string{"**/*.csv", "**/*.md", "**/*.pdf", "**/*.txt"}
	BuiltinExclude = []string{"**/CLAUDE.md", "**/AGENTS.md"}
)

// KeyName returns the name of the API key in keys.toml that g asks for.
func (g Global) KeyName() string {
	if g.Key == "" {
		return DefaultKeyName
	}
	return g.Key
}

type globalFile struct {
	Key     string   `toml:"key"`
	Model   string   `toml:"model"`
	Include []string `toml:"include"`
	Exclude []string `toml:"exclude"`
	Watch   []string `toml:"watch"`
}

// StateDir returns the folder where the daemon keeps its state.
func StateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "com.westarete.catalog"), nil
}

// decodeErrorPlace matches where the TOML decoder says a value of the
// wrong type sits. Those errors aren't a ParseError, so their line and key
// come only from the message.
var decodeErrorPlace = regexp.MustCompile(`line (\d+) \(last key "([^"]*)"\)`)

// parseError says where the file called name fails to parse without the
// decoder's own message, which can quote part of a value, such as an API key
// pasted into the wrong file. It names a key only if it is in known. The
// error reaches the journal, the log, and catalog status.
func parseError(name string, known map[string]bool, err error) error {
	if pe, ok := errors.AsType[toml.ParseError](err); ok {
		return fmt.Errorf("parse %s: %s: not valid TOML", name, errorPlace(known, pe.Position.Line, pe.LastKey))
	}
	if m := decodeErrorPlace.FindStringSubmatch(err.Error()); m != nil {
		return fmt.Errorf("parse %s: %s: a value has the wrong type", name, errorPlace(known, m[1], m[2]))
	}
	return fmt.Errorf("parse %s: not valid TOML", name)
}

// globalKeys are the keys global.toml defines, and catalogKeys the keys a
// catalog's config.toml defines. An error names a key only if it is one of
// these, because a person can paste the API key as a bare key and the
// decoder would report that key.
var (
	globalKeys  = map[string]bool{"watch": true, "model": true, "key": true, "include": true, "exclude": true}
	catalogKeys = map[string]bool{"include": true, "exclude": true}
)

// errorPlace says the line, and the last key if it is in known.
func errorPlace(known map[string]bool, line any, key string) string {
	if known[key] {
		return fmt.Sprintf("line %v (last key %q)", line, key)
	}
	return fmt.Sprintf("line %v", line)
}

// ErrWorldAccessible means a settings file, global.toml or keys.toml, has a
// world permission bit set: it is world-readable, world-writable, or
// world-executable. keys.toml holds the API keys, and global.toml decides
// which files go to the model service, so neither is used while anyone can
// read or change it. The group bits don't count: the owner may share the
// file with a group on purpose.
var ErrWorldAccessible = errors.New("world-accessible")

// worldBits are the world read, write, and execute bits of a file's mode.
const worldBits = 0o007

// ErrNotRegularFile means something other than a regular file, such as a
// folder or a FIFO, is at a settings file's path.
var ErrNotRegularFile = errors.New("not a regular file")

// ReadGlobalFile reads and parses the global.toml at path. It returns an error
// wrapping ErrWorldAccessible, and no settings, if the file is
// world-accessible, one wrapping ErrNotRegularFile if the
// path holds something other than a regular file, and one wrapping
// fs.ErrNotExist if there is no file.
func ReadGlobalFile(path string) (Global, error) {
	data, err := readPrivateFile(path)
	if err != nil {
		return Global{}, err
	}
	return ParseGlobal(data)
}

// readPrivateFile reads the file at path if it is a regular file with no
// world permission bit set. It returns ErrNotRegularFile or an error
// wrapping ErrWorldAccessible otherwise, so no settings file that may hold
// a secret or steer what leaves the machine is read from a place others
// can change.
func readPrivateFile(path string) ([]byte, error) {
	// O_NONBLOCK keeps the open from waiting for a writer if the path is a
	// FIFO. A regular file ignores it.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0) //nolint:gosec // path is a settings file in the state folder
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	// Stat the open file, not the path, so the mode checked is the mode of
	// the bytes read.
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrNotRegularFile
	}
	if mode := info.Mode().Perm(); mode&worldBits != 0 {
		return nil, fmt.Errorf("%w (mode %v); to fix it, run chmod o-rwx on it", ErrWorldAccessible, mode)
	}
	return io.ReadAll(f)
}

// ParseGlobal parses global.toml, filling unset fields with built-in defaults.
// It rejects the file if it has a key the daemon doesn't read.
func ParseGlobal(data []byte) (Global, error) {
	var f globalFile
	md, err := toml.Decode(string(data), &f)
	if err != nil {
		return Global{}, parseError(GlobalFile, globalKeys, err)
	}
	if err := unknownKeyError(GlobalFile, data, md, globalKeys); err != nil {
		return Global{}, err
	}
	g := Global{Key: f.Key, Model: defaultModel, Include: f.Include, Exclude: f.Exclude}
	if f.Model != "" {
		g.Model = f.Model
	}
	for _, folder := range f.Watch {
		folder, err := expandHome(folder)
		if err != nil {
			return Global{}, err
		}
		g.Watch = append(g.Watch, folder)
	}
	return g, nil
}

// ParseCatalog parses a catalog's .catalog/config.toml. It reads no keys but
// include and exclude, and rejects the file if it has any other.
func ParseCatalog(data []byte) (Catalog, error) {
	var c Catalog
	md, err := toml.Decode(string(data), &c)
	if err != nil {
		return Catalog{}, parseError(CatalogConfigFile, catalogKeys, err)
	}
	if err := unknownKeyError(CatalogConfigFile, data, md, catalogKeys); err != nil {
		return Catalog{}, err
	}
	return c, nil
}

// Lists returns the include and exclude patterns for a catalog: the built-in
// patterns, then those in global.toml, then those in the catalog's
// config.toml. Each result is a new slice. A file is read if it matches an
// include pattern and no exclude pattern, so a catalog can add to what a
// person excluded in global.toml but never undo it.
func Lists(g Global, c Catalog) (include, exclude []string) {
	include = slices.Concat(BuiltinInclude, g.Include, c.Include)
	exclude = slices.Concat(BuiltinExclude, g.Exclude, c.Exclude)
	return include, exclude
}

// expandHome replaces a leading ~/ in path, or a path that is only ~, with
// the home folder.
func expandHome(path string) (string, error) {
	if path == "~" {
		path = "~/"
	}
	rest, ok := strings.CutPrefix(path, "~/")
	if !ok {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, rest), nil
}
