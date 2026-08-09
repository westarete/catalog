// Command catalog generates and checks .catalog.md — a behavior file for
// agents. Each entry is a profile: the conditions under which an agent should
// pull a document into context, the same way a skill's description tells the
// harness when to load the skill body.
//
// Profiles live in a committed SQLite database (.catalog/catalog.db); the
// database is the sole source of truth, and .catalog.md is always a
// generated artifact rendered from it. Staleness is a content-hash
// comparison against that database — catalog has no dependency on Git and
// does not require running inside a repository.
//
// Four subcommands, split by whether they need an API key:
//
//	bootstrap  Rebuild every entry from scratch in two passes. For creating
//	           the database or rebuilding it wholesale. Needs ANTHROPIC_API_KEY.
//	update     Re-infer profiles for new or modified documents and drop rows
//	           for deleted ones. The routine job. Needs ANTHROPIC_API_KEY.
//	force      Re-infer named documents unconditionally (or all of them,
//	           given no names). Needs ANTHROPIC_API_KEY.
//	status     Report new/modified/deleted documents and whether .catalog.md
//	           matches the database. No API key, no model call. CI's gate.
//
// The three key-using commands differ only in which entries they rewrite and how
// many passes they run; status is the deterministic gate that never calls a model.
//
// Run from the repo root.
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
	case "status":
		err = cmdStatus(args)
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
	fmt.Fprint(os.Stderr, `catalog — generate and check .catalog.md (agent profiles)

usage:
  catalog bootstrap            rebuild every entry from scratch, two passes (needs ANTHROPIC_API_KEY)
  catalog update               re-infer profiles for new or modified docs (needs ANTHROPIC_API_KEY)
  catalog force [file ...]     re-infer named docs unconditionally; no names = all (needs ANTHROPIC_API_KEY)
  catalog status               report new/modified/deleted docs and catalog.md drift (no API key)
  catalog version              print version and exit
  catalog help                 show this message

update is the routine job. force redoes specific entries you're unhappy with,
or rebuilds all of them in one pass after a prompt change. bootstrap is the
two-pass build for an empty or missing database.
`)
}
