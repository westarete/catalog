// Package settings reads the daemon's global and per-catalog configuration.
package settings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

// GlobalFile is the name of the global settings file in StateDir.
const GlobalFile = "global.toml"

const defaultModel = "claude-haiku-5-5"

// Global holds the settings that apply to every catalog on this machine.
type Global struct {
	APIKey    string
	Model     string
	Enumerate []string
	Ignore    []string
	Watch     []string
}

type globalFile struct {
	APIKey    string    `toml:"api_key"`
	Model     string    `toml:"model"`
	Enumerate *[]string `toml:"enumerate"`
	Ignore    *[]string `toml:"ignore"`
	Watch     []struct {
		Folder string `toml:"folder"`
	} `toml:"watch"`
}

type catalogFile struct {
	Enumerate *[]string `toml:"enumerate"`
	Ignore    *[]string `toml:"ignore"`
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

// globalParseError says where global.toml fails to parse without the
// decoder's own message, which can quote part of a value such as the API
// key. The error reaches the journal, the log, and catalog status.
func globalParseError(err error) error {
	if pe, ok := errors.AsType[toml.ParseError](err); ok {
		return fmt.Errorf("parse %s: %s: not valid TOML", GlobalFile, errorPlace(pe.Position.Line, pe.LastKey))
	}
	if m := decodeErrorPlace.FindStringSubmatch(err.Error()); m != nil {
		return fmt.Errorf("parse %s: %s: a value has the wrong type", GlobalFile, errorPlace(m[1], m[2]))
	}
	return fmt.Errorf("parse %s: not valid TOML", GlobalFile)
}

// globalKeys are the keys global.toml defines. An error names a key only if
// it is one of these, because a person can paste the API key as a bare key
// and the decoder would report that key.
var globalKeys = map[string]bool{
	"api_key": true, "model": true, "enumerate": true, "ignore": true,
	"watch": true, "folder": true, "watch.folder": true,
}

// errorPlace says the line, and the last key if global.toml defines it.
func errorPlace(line any, key string) string {
	if globalKeys[key] {
		return fmt.Sprintf("line %v (last key %q)", line, key)
	}
	return fmt.Sprintf("line %v", line)
}

// ParseGlobal parses global.toml, filling unset fields with built-in defaults.
func ParseGlobal(data []byte) (Global, error) {
	var f globalFile
	if _, err := toml.Decode(string(data), &f); err != nil {
		return Global{}, globalParseError(err)
	}
	g := Global{
		APIKey:    f.APIKey,
		Model:     defaultModel,
		Enumerate: []string{"**/*.csv", "**/*.md", "**/*.pdf", "**/*.txt"},
		Ignore:    []string{"README.md", "**/CLAUDE.md", "**/AGENTS.md"},
	}
	if f.Model != "" {
		g.Model = f.Model
	}
	if f.Enumerate != nil {
		g.Enumerate = *f.Enumerate
	}
	if f.Ignore != nil {
		g.Ignore = *f.Ignore
	}
	for _, w := range f.Watch {
		folder, err := expandHome(w.Folder)
		if err != nil {
			return Global{}, err
		}
		g.Watch = append(g.Watch, folder)
	}
	return g, nil
}

// ApplyCatalog layers a .catalog/config.toml over g and returns the
// enumerate and ignore lists for that catalog. It reads no other keys.
func ApplyCatalog(g Global, data []byte) (enumerate, ignore []string, err error) {
	var f catalogFile
	if _, err := toml.Decode(string(data), &f); err != nil {
		return nil, nil, fmt.Errorf("parse catalog config: %w", err)
	}
	enumerate, ignore = g.Enumerate, g.Ignore
	if f.Enumerate != nil {
		enumerate = *f.Enumerate
	}
	if f.Ignore != nil {
		ignore = *f.Ignore
	}
	return enumerate, ignore, nil
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
