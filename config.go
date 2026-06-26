package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// configPath is the write-once input: globs for what to enumerate and ignore,
// and the model to infer profiles with. It lives in the hidden plumbing
// directory because, unlike .catalog.md, it is set once and forgotten.
const configPath = ".catalog/config.toml"

// defaultModel infers profiles. Sonnet matched Opus on this repo's documents at
// about a third the cost; Opus is the costlier fallback when profiles come out
// weak. See SKILL.md.
const defaultModel = "claude-sonnet-4-6"

type config struct {
	Profile   string   `toml:"profile"`
	Enumerate []string `toml:"enumerate"`
	Ignore    []string `toml:"ignore"`
	Model     string   `toml:"model"`
}

func loadConfig() (*config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"no config at %s — copy config.example.toml from the catalog skill and edit it",
				configPath)
		}
		return nil, err
	}
	cfg, err := parseConfig(data)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", configPath, err)
	}
	return cfg, nil
}

// parseConfig decodes config TOML and applies the model default. Separated from
// the file read so it can be tested without touching the filesystem.
func parseConfig(data []byte) (*config, error) {
	var cfg config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}
	return &cfg, nil
}

// includedDocs returns the enumerated documents minus the ignored ones, as
// sorted slash-separated paths relative to the repo root. Globs use the same
// doublestar semantics as the rest of the repo's tooling (** spans
// directories), via the filepath.Match-compatible walk below.
func includedDocs(cfg *config) ([]string, error) {
	keep, err := matchAll(cfg.Enumerate)
	if err != nil {
		return nil, err
	}
	drop, err := matchAll(cfg.Ignore)
	if err != nil {
		return nil, err
	}
	return filterDocs(keep, drop), nil
}

// filterDocs returns the kept paths minus the dropped ones, sorted. Pure set
// arithmetic, separated from the filesystem walk so it can be tested directly.
func filterDocs(keep, drop map[string]bool) []string {
	var out []string
	for p := range keep {
		if !drop[p] {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// matchAll walks the repo once and returns every file matching any of the
// given glob patterns. Patterns are repo-root-relative and may contain ** to
// span directories (e.g. "leadership/**/*.md").
func matchAll(patterns []string) (map[string]bool, error) {
	if len(patterns) == 0 {
		return map[string]bool{}, nil
	}
	out := map[string]bool{}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		rel := filepath.ToSlash(path)
		for _, pat := range patterns {
			if matchGlob(pat, rel) {
				out[rel] = true
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
