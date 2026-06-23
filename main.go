// Command catalog generates and checks CATALOG.md — a behavior file for
// agents. Each entry is a profile: the conditions under which an agent should
// pull a document into context, the same way a skill's description tells the
// harness when to load the skill body.
//
// Four subcommands, split by whether they need an API key:
//
//	bootstrap  Rebuild every entry from scratch in two passes. For creating
//	           CATALOG.md or rebuilding it wholesale. Needs ANTHROPIC_API_KEY.
//	update     Re-infer profiles for the documents Git reports changed and
//	           rewrite those in place. The routine job. Needs ANTHROPIC_API_KEY.
//	force      Re-infer named documents even when Git thinks they're current
//	           (or all of them, given no names). Needs ANTHROPIC_API_KEY.
//	check      Pure Git query: is every enumerated document present and
//	           un-stale? No API key, no model call. CI's gate.
//
// The three key-using commands differ only in which entries they rewrite and how
// many passes they run; check is the deterministic gate that never calls a model.
//
// Run from the repo root via the bin/catalog shim.
package main

import (
	"fmt"
	"os"
)

// version is set at build time by GoReleaser via -X main.version=<tag>.
// It falls back to "dev" when built locally with `go build`.
var version = "dev"

func main() {
	args := os.Args[1:]

	// An optional --root <dir> prefix lets the shim run the program from the Go
	// module directory (via `go run -C`) while it still resolves config,
	// catalog, and git paths relative to the repo root.
	if len(args) >= 2 && args[0] == "--root" {
		if err := os.Chdir(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "catalog: --root: %v\n", err)
			os.Exit(1)
		}
		args = args[2:]
	}

	if len(args) < 1 {
		usage()
		os.Exit(2)
	}
	cmd := args[0]
	args = args[1:]

	var err error
	switch cmd {
	case "bootstrap":
		err = cmdBootstrap(args)
	case "update":
		err = cmdUpdate(args)
	case "force":
		err = cmdForce(args)
	case "check":
		err = cmdCheck(args)
	case "-h", "--help", "help":
		usage()
		return
	case "-v", "--version", "version":
		fmt.Println("catalog", version)
		return
	default:
		fmt.Fprintf(os.Stderr, "catalog: unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "catalog: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `catalog — generate and check CATALOG.md (agent profiles)

usage:
  catalog bootstrap            rebuild every entry from scratch, two passes (needs ANTHROPIC_API_KEY)
  catalog update               re-infer profiles for docs Git reports changed (needs ANTHROPIC_API_KEY)
  catalog force [file ...]     re-infer named docs even if current; no names = all (needs ANTHROPIC_API_KEY)
  catalog check                staleness gate, pure Git query (no API key)
  catalog version              print version and exit
  catalog help                 show this message

update is the routine job. force redoes specific entries you're unhappy with,
or rebuilds all of them in one pass after a prompt change. bootstrap is the
two-pass build for an empty or missing catalog.
`)
}
