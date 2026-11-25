package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"westarete.com/catalog/internal/install"
	"westarete.com/catalog/internal/settings"
)

// instructionLine is what `catalog init` ends with: the line an agent's
// instruction file needs to read the catalog.
const instructionLine = "To have agents read this catalog, add this line to the folder's AGENTS.md or CLAUDE.md: @.catalog/catalog.md"

// runInit carries out `catalog init` with the arguments that follow it: at
// most one folder, which defaults to the current folder. It makes the folder
// a catalog and says whether the daemon watches it.
func runInit(args []string, stdout io.Writer) error {
	var folder string
	switch len(args) {
	case 0:
		folder = "."
	case 1:
		folder = args[0]
	default:
		return errUsage
	}
	abs, err := filepath.Abs(folder)
	if err != nil {
		return err
	}
	if err := install.InitCatalog(abs, stdout); err != nil {
		return err
	}
	stateDir, err := settings.StateDir()
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(stdout, watchLine(filepath.Join(stateDir, settings.GlobalFile), abs)); err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, instructionLine)
	return err
}

// watchLine says whether folder is, or is inside, a folder that the
// global.toml at globalPath watches. It says nothing if global.toml doesn't
// load, since status reports why.
func watchLine(globalPath, folder string) string {
	g, err := settings.ReadGlobalFile(globalPath)
	if err != nil {
		return ""
	}
	for _, watched := range g.Watch {
		if isWithin(folder, watched) {
			return "The daemon watches this folder.\n"
		}
	}
	return fmt.Sprintf("No folder the daemon watches contains this one. Add it, or a folder above it, to watch in %s.\n", globalPath)
}

// isWithin reports whether folder is root or inside it, comparing the
// cleaned paths by component rather than by string prefix.
func isWithin(folder, root string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(folder))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
