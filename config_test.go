package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseConfigDefaults(t *testing.T) {
	cfg, err := parseConfig([]byte(`enumerate = ["a/**/*.md"]`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != defaultModel {
		t.Errorf("model default = %q, want %q", cfg.Model, defaultModel)
	}
	if !reflect.DeepEqual(cfg.Enumerate, []string{"a/**/*.md"}) {
		t.Errorf("enumerate = %v", cfg.Enumerate)
	}
}

func TestParseConfigExplicitModel(t *testing.T) {
	cfg, err := parseConfig([]byte(`
model = "claude-opus-4-8"
enumerate = ["x.md"]
ignore = ["y.md"]
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "claude-opus-4-8" {
		t.Errorf("model = %q", cfg.Model)
	}
	if !reflect.DeepEqual(cfg.Ignore, []string{"y.md"}) {
		t.Errorf("ignore = %v", cfg.Ignore)
	}
}

func TestParseConfigBadToml(t *testing.T) {
	if _, err := parseConfig([]byte("this is = = not toml")); err == nil {
		t.Error("expected error on malformed TOML")
	}
}

func TestFilterDocs(t *testing.T) {
	keep := map[string]bool{"a.md": true, "b.md": true, "c.md": true}
	drop := map[string]bool{"b.md": true}
	got := filterDocs(keep, drop)
	if !reflect.DeepEqual(got, []string{"a.md", "c.md"}) {
		t.Errorf("filterDocs = %v, want [a.md c.md]", got)
	}
}

func TestParseConfigPricesOptional(t *testing.T) {
	cfg, err := parseConfig([]byte(`enumerate = ["a.md"]`))
	if err != nil {
		t.Fatal(err)
	}
	if p := cfg.prices(); p != nil {
		t.Errorf("prices = %+v, want nil when none are set", p)
	}
}

func TestParseConfigPrices(t *testing.T) {
	cfg, err := parseConfig([]byte(`
input_price_per_mtok = 0.10
output_price_per_mtok = 0.50
cache_read_price_per_mtok = 0.01
`))
	if err != nil {
		t.Fatal(err)
	}
	want := pricing{InputPerMTok: 0.10, OutputPerMTok: 0.50, CacheReadPerMTok: 0.01}
	if p := cfg.prices(); p == nil || *p != want {
		t.Errorf("prices = %+v, want %+v", p, want)
	}
	if len(cfg.warnings) != 0 {
		t.Errorf("warnings = %v, want none", cfg.warnings)
	}
}

func TestParseConfigRejectsIncompleteOrNegativePrices(t *testing.T) {
	for name, toml := range map[string]string{
		"no output price": "input_price_per_mtok = 0.10\n",
		"no input price":  "output_price_per_mtok = 0.50\n",
		"only cache read": "cache_read_price_per_mtok = 0.01\n",
		"negative cache":  "input_price_per_mtok = 1\noutput_price_per_mtok = 5\ncache_read_price_per_mtok = -1\n",
	} {
		if _, err := parseConfig([]byte(toml)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestParseConfigWarnsAboutUnrecognizedKeys(t *testing.T) {
	cfg, err := parseConfig([]byte("enumerat = [\"a.md\"]\nmodle = \"x\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`unrecognized key "enumerat" is ignored`, `unrecognized key "modle" is ignored`}
	if !reflect.DeepEqual(cfg.warnings, want) {
		t.Errorf("warnings = %q, want %q", cfg.warnings, want)
	}
}

func TestParseConfigPointsTheOldPricingTableAtTheNewKeys(t *testing.T) {
	cfg, err := parseConfig([]byte("[pricing]\ninput_per_mtok = 0.10\noutput_per_mtok = 0.50\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.warnings) != 1 || !strings.Contains(cfg.warnings[0], "[pricing] is no longer read") || !strings.Contains(cfg.warnings[0], "input_price_per_mtok") {
		t.Errorf("warnings = %q, want one pointing at the new price keys", cfg.warnings)
	}
	if cfg.prices() != nil {
		t.Error("the old table should not set prices")
	}
}

func TestParseConfigRequestSizes(t *testing.T) {
	cfg, err := parseConfig([]byte("warn_request_tokens = 100000\ntruncate_request_tokens = 80000\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WarnRequestTokens != 100000 || cfg.TruncateRequestTokens != 80000 {
		t.Errorf("got warn=%d truncate=%d", cfg.WarnRequestTokens, cfg.TruncateRequestTokens)
	}
}

func TestParseConfigRejectsNegativeRequestSizes(t *testing.T) {
	for _, toml := range []string{"warn_request_tokens = -1", "truncate_request_tokens = -1"} {
		if _, err := parseConfig([]byte(toml)); err == nil {
			t.Errorf("%q: expected an error", toml)
		}
	}
}
