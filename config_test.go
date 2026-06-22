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
