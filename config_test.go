package main

import (
	"reflect"
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

func TestParseConfigPricingOptional(t *testing.T) {
	cfg, err := parseConfig([]byte(`enumerate = ["a.md"]`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pricing != nil {
		t.Errorf("pricing = %+v, want nil when the table is absent", cfg.Pricing)
	}
}

func TestParseConfigPricing(t *testing.T) {
	cfg, err := parseConfig([]byte(`
[pricing]
input_per_mtok = 0.10
output_per_mtok = 0.50
cache_read_per_mtok = 0.01
`))
	if err != nil {
		t.Fatal(err)
	}
	want := pricing{InputPerMTok: 0.10, OutputPerMTok: 0.50, CacheReadPerMTok: 0.01}
	if cfg.Pricing == nil || *cfg.Pricing != want {
		t.Errorf("pricing = %+v, want %+v", cfg.Pricing, want)
	}
}

func TestParseConfigPricingRejectsMissingOrNegativePrices(t *testing.T) {
	for name, toml := range map[string]string{
		"no output price": "[pricing]\ninput_per_mtok = 0.10\n",
		"no input price":  "[pricing]\noutput_per_mtok = 0.50\n",
		"negative cache":  "[pricing]\ninput_per_mtok = 1\noutput_per_mtok = 5\ncache_read_per_mtok = -1\n",
	} {
		if _, err := parseConfig([]byte(toml)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
