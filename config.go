package main

import (
	"errors"
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

// defaultModel infers profiles. Haiku is the cheapest model that does the job;
// Sonnet or Opus is the costlier fallback when profiles come out weak. See
// SKILL.md.
const defaultModel = "claude-haiku-5-5"

type config struct {
	Profile   string   `toml:"profile"`
	Enumerate []string `toml:"enumerate"`
	Ignore    []string `toml:"ignore"`
	Model     string   `toml:"model"`

	// What the model charges, in dollars per million tokens. Optional: with no
	// prices a run reports tokens but no cost. Read them through prices().
	InputPricePerMTok     float64 `toml:"input_price_per_mtok"`
	OutputPricePerMTok    float64 `toml:"output_price_per_mtok"`
	CacheReadPricePerMTok float64 `toml:"cache_read_price_per_mtok"`

	// WarnRequestTokens and TruncateRequestTokens manage request size. Some
	// models charge much more once a request's prompt passes a size, so the
	// report warns about requests over WarnRequestTokens, and a text document
	// is cut to fit TruncateRequestTokens. Zero leaves each off.
	WarnRequestTokens     int `toml:"warn_request_tokens"`
	TruncateRequestTokens int `toml:"truncate_request_tokens"`

	warnings []string // problems with the file that don't stop a run, such as unrecognized keys
}

// prices is what the config says the model charges, or nil when it gives no
// prices at all.
func (c *config) prices() *pricing {
	p := pricing{c.InputPricePerMTok, c.OutputPricePerMTok, c.CacheReadPricePerMTok}
	if p == (pricing{}) {
		return nil
	}
	return &p
}

// bytesPerToken converts a token budget to a document size. Counting tokens
// exactly would take a second API call per document, so this is a rough rule
// for English prose; dense data like CSV runs more tokens per byte.
const bytesPerToken = 4

// documentLimit is how many bytes of a text document are sent to the model.
func (c *config) documentLimit() int {
	if c.TruncateRequestTokens > 0 {
		return c.TruncateRequestTokens * bytesPerToken
	}
	return maxDocumentBytes
}

// warnsAbout reports whether a request used more prompt tokens than
// warn_request_tokens. Cache reads count: the prompt is what was sent.
func (c *config) warnsAbout(used tokens) bool {
	return c.WarnRequestTokens > 0 && used.input+used.cached > int64(c.WarnRequestTokens)
}

func loadConfig() (*config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"no config at %s — run `catalog init` to create one",
				configPath)
		}
		return nil, err
	}
	cfg, err := parseConfig(data)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", configPath, err)
	}
	for _, w := range cfg.warnings {
		fmt.Fprintf(os.Stderr, "catalog: warning: %s: %s\n", configPath, w)
	}
	return cfg, nil
}

// parseConfig decodes config TOML, applies the model default, checks the
// prices if there are any, and notes keys it does not recognize. Separated from
// the file read so it can be tested without touching the filesystem.
func parseConfig(data []byte) (*config, error) {
	var cfg config
	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return nil, err
	}
	cfg.warnings = unrecognizedKeyWarnings(md.Undecoded())
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}
	if cfg.WarnRequestTokens < 0 || cfg.TruncateRequestTokens < 0 {
		return nil, errors.New("warn_request_tokens and truncate_request_tokens cannot be negative")
	}
	if p := cfg.prices(); p != nil {
		if err := p.validate(); err != nil {
			return nil, err
		}
	}
	return &cfg, nil
}

// unrecognizedKeyWarnings turns the keys the config struct did not take into
// messages. A typo is otherwise silent: the setting just never applies. The
// [pricing] table gets its own message because earlier versions read one.
func unrecognizedKeyWarnings(keys []toml.Key) []string {
	var out []string
	legacyPricing := false
	for _, k := range keys {
		if k[0] == "pricing" {
			if !legacyPricing {
				out = append(out, "[pricing] is no longer read; use input_price_per_mtok, output_price_per_mtok and cache_read_price_per_mtok at the top level")
			}
			legacyPricing = true
			continue
		}
		out = append(out, fmt.Sprintf("unrecognized key %q is ignored", k.String()))
	}
	return out
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
