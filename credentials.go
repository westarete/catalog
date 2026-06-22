package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// credentialsPath is where the user stores named API key profiles. Follows the
// XDG Base Directory spec: $XDG_CONFIG_HOME/catalog/credentials, defaulting to
// ~/.config/catalog/credentials.
func credentialsPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "catalog", "credentials")
}

// credentialsFile is the on-disk format: one [profile] section per named key.
type credentialsFile map[string]struct {
	APIKey string `toml:"api_key"`
}

// parseCredentials decodes the credentials file TOML into a map of profile
// name → API key. Separated from file I/O so it can be tested without touching
// the filesystem.
func parseCredentials(data []byte) (map[string]string, error) {
	var f credentialsFile
	if err := toml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(f))
	for name, section := range f {
		out[name] = section.APIKey
	}
	return out, nil
}

// resolveAPIKey applies the precedence rules and returns the first key found.
// All inputs are passed explicitly so the logic can be tested without touching
// the environment or filesystem.
//
// Precedence (highest to lowest):
//  1. envKey       — ANTHROPIC_API_KEY env var (CI, one-off overrides)
//  2. configProfile — profile name from .catalog/config.toml
//  3. envProfile   — CATALOG_PROFILE env var
//  4. "default"    — the [default] section of the credentials file
//
// credsFileContents is the raw contents of the credentials file; pass "" if
// the file does not exist.
func resolveAPIKey(envKey, configProfile, envProfile, credsFileContents string) (string, error) {
	if envKey != "" {
		return envKey, nil
	}

	if credsFileContents != "" {
		creds, err := parseCredentials([]byte(credsFileContents))
		if err != nil {
			return "", fmt.Errorf("parsing credentials file: %w", err)
		}

		if configProfile != "" {
			key, ok := creds[configProfile]
			if !ok {
				return "", fmt.Errorf("credentials file has no [%s] section", configProfile)
			}
			return key, nil
		}

		if envProfile != "" {
			key, ok := creds[envProfile]
			if !ok {
				return "", fmt.Errorf("credentials file has no [%s] section (from CATALOG_PROFILE)", envProfile)
			}
			return key, nil
		}

		key, ok := creds["default"]
		if !ok {
			return "", fmt.Errorf("credentials file has no [default] section and no profile is set")
		}
		return key, nil
	}

	return "", fmt.Errorf(
		"no API key found — set ANTHROPIC_API_KEY, or add a profile to %s and set profile in %s",
		credentialsPath(), configPath)
}

// loadAPIKey reads the real environment and credentials file, then delegates to
// resolveAPIKey. This is the call site used by the commands; resolveAPIKey is
// the pure function used by tests.
func loadAPIKey(cfg *config) (string, error) {
	envKey := os.Getenv("ANTHROPIC_API_KEY")
	envProfile := os.Getenv("CATALOG_PROFILE")

	credsFileContents := ""
	data, err := os.ReadFile(credentialsPath())
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading credentials file: %w", err)
	}
	if err == nil {
		credsFileContents = string(data)
		warnIfInsecure(credentialsPath())
	}

	return resolveAPIKey(envKey, cfg.Profile, envProfile, credsFileContents)
}

// warnIfInsecure prints a warning if the credentials file is readable by
// anyone other than the owner.
func warnIfInsecure(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.Mode().Perm()&0o077 != 0 {
		fmt.Fprintf(os.Stderr, "catalog: warning: %s is readable by others (run: chmod 0600 %s)\n", path, path)
	}
}
