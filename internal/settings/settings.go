// Package settings reads the daemon's global and per-catalog configuration.
package settings

import (
	"fmt"
	"os"
	"path/filepath"
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

// ParseGlobal parses global.toml, filling unset fields with built-in defaults.
func ParseGlobal(data []byte) (Global, error) {
	var f globalFile
	if _, err := toml.Decode(string(data), &f); err != nil {
		return Global{}, fmt.Errorf("parse %s: %w", GlobalFile, err)
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

func expandHome(path string) (string, error) {
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
