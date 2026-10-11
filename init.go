package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// exampleConfig is the template init writes. It is embedded, not read from the
// catalog skill, so it always matches the keys this binary accepts. See
// TestExampleConfigParses, which also keeps its model in step with defaultModel.
//
//go:embed config.example.toml
var exampleConfig string

func cmdInit(args []string) error {
	return runInit(args, os.Stdout)
}

// runInit creates .catalog/config.toml from the embedded template, or with
// --print writes the template to out and touches nothing, so a person or an
// agent can compare it with the config a repo already has.
func runInit(args []string, out io.Writer) error {
	switch {
	case len(args) == 1 && args[0] == "--print":
		_, err := io.WriteString(out, exampleConfig)
		return err
	case len(args) > 0:
		unexpected := args[0]
		if unexpected == "--print" { // valid on its own, so the culprit is what follows it
			unexpected = args[1]
		}
		return fmt.Errorf("init: unexpected argument %q (the only option is --print)", unexpected)
	}
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("init: %s already exists; `catalog init --print` shows the current template to compare it with", configPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(configPath, []byte(exampleConfig), 0o644); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "created %s\nedit enumerate and ignore for this repo, then run `catalog bootstrap`\n", configPath)
	return err
}
