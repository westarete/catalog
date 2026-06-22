package main

import (
	"testing"
)

const testCredsFile = `
[default]
api_key = "sk-default"

[westarete]
api_key = "sk-westarete"

[lwi]
api_key = "sk-lwi"
`

func TestResolveAPIKeyFromEnvVar(t *testing.T) {
	// ANTHROPIC_API_KEY env var wins over everything.
	key, err := resolveAPIKey("sk-env", "", "", testCredsFile)
	if err != nil {
		t.Fatal(err)
	}
	if key != "sk-env" {
		t.Errorf("got %q, want sk-env", key)
	}
}

func TestResolveAPIKeyFromConfigProfile(t *testing.T) {
	// profile in config.toml selects the named section.
	key, err := resolveAPIKey("", "westarete", "", testCredsFile)
	if err != nil {
		t.Fatal(err)
	}
	if key != "sk-westarete" {
		t.Errorf("got %q, want sk-westarete", key)
	}
}

func TestResolveAPIKeyFromEnvProfile(t *testing.T) {
	// CATALOG_PROFILE env var selects the named section when no config profile is set.
	key, err := resolveAPIKey("", "", "lwi", testCredsFile)
	if err != nil {
		t.Fatal(err)
	}
	if key != "sk-lwi" {
		t.Errorf("got %q, want sk-lwi", key)
	}
}

func TestResolveAPIKeyConfigProfileBeatsEnvProfile(t *testing.T) {
	// config profile takes precedence over CATALOG_PROFILE env var.
	key, err := resolveAPIKey("", "westarete", "lwi", testCredsFile)
	if err != nil {
		t.Fatal(err)
	}
	if key != "sk-westarete" {
		t.Errorf("got %q, want sk-westarete", key)
	}
}

func TestResolveAPIKeyDefault(t *testing.T) {
	// No profile set falls back to [default].
	key, err := resolveAPIKey("", "", "", testCredsFile)
	if err != nil {
		t.Fatal(err)
	}
	if key != "sk-default" {
		t.Errorf("got %q, want sk-default", key)
	}
}

func TestResolveAPIKeyMissingProfile(t *testing.T) {
	// Named profile that doesn't exist in the file is an error.
	_, err := resolveAPIKey("", "nonexistent", "", testCredsFile)
	if err == nil {
		t.Error("expected error for missing profile")
	}
}

func TestResolveAPIKeyNoCredentials(t *testing.T) {
	// No env var and no credentials file is an error.
	_, err := resolveAPIKey("", "", "", "")
	if err == nil {
		t.Error("expected error when no key can be found")
	}
}

func TestResolveAPIKeyEmptyDefault(t *testing.T) {
	// A credentials file with no [default] section and no profile set is an error.
	creds := `
[westarete]
api_key = "sk-westarete"
`
	_, err := resolveAPIKey("", "", "", creds)
	if err == nil {
		t.Error("expected error when [default] section is missing")
	}
}

func TestParseCredentials(t *testing.T) {
	creds, err := parseCredentials([]byte(testCredsFile))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		profile string
		want    string
	}{
		{"default", "sk-default"},
		{"westarete", "sk-westarete"},
		{"lwi", "sk-lwi"},
	}
	for _, c := range cases {
		if got := creds[c.profile]; got != c.want {
			t.Errorf("creds[%q] = %q, want %q", c.profile, got, c.want)
		}
	}
}

func TestParseCredentialsBadToml(t *testing.T) {
	if _, err := parseCredentials([]byte("this is = = not toml")); err == nil {
		t.Error("expected error on malformed TOML")
	}
}
