# catalog

Once you have 20 or more files in a repository, agents do a pretty poor
job of deciding which files to load into context. They'll randomly load
ones files they don't need, and they won't load the ones that they do
need.

`catalog` is a command-line tool that profiles the documents in your
repo provides agents with a catalog so they can load precisely what they
need. This means better answers, faster responses, and more context
window left for the actual work.

If you want to add catalog capabilities to your repo, install the
[catalog skill](https://github.com/westarete/hq/tree/main/skill/catalog)
and then use it to set your repo up. It will prompt you to install this
utility and set up the necessary configuration.

If you want to do development on this repo, See [SETUP.md](SETUP.md) for
developer installation and first-run instructions.

## How it works

Each entry in `CATALOG.md` is a **profile**: a description of the
conditions under which a document is relevant — the same idea as a
skill's description field, which tells the harness when to load the
skill body. A profile says "open this file when the task involves X,"
not "this file is about X." That distinction is what makes routing work:
a wrong summary is ugly; a wrong profile sends the agent to the wrong
file.

The tool infers profiles by reading each document with the surrounding
catalog as context, so it can distinguish near-neighbors from each other
rather than describing them in isolation.

## Commands

Run from the repo root. Three commands write profiles and need
`ANTHROPIC_API_KEY`. `check` reads only Git and needs no key — it's the
one you run in CI.

- `catalog update` — re-infer profiles for the documents Git reports
  changed, rewrite those entries in place, leave the rest of
  `CATALOG.md` alone. Also drops entries for deleted files. Run this
  after editing or removing a document, the same way you'd run a
  formatter.
- `catalog bootstrap` — generate `CATALOG.md` from scratch: infer
  profiles for every enumerated document in two passes (the second pass
  sharpens each profile using the full catalog as context), then write
  the file. Use this when setting up a new repo or after major
  reorganization.
- `catalog force [file ...]` — re-infer the named documents (or all
  documents) even when Git thinks they're current. Use it to redo a few
  entries you're unhappy with, or to rebuild everything after a prompt
  change.
- `catalog check` — verify that `CATALOG.md` is up to date: every
  enumerated document has an entry, every entry points to a file that
  still exists, and no entry is stale relative to Git. Exits non-zero if
  anything is wrong. No model call needed.

## Repository layout

```text
catalog/
  *.go                  Go source (at the repo root)
  .goreleaser.yaml      release config
```

## Installing in a repo

The skill walks through the decision and setup. Install it from the `hq`
repo alongside the other West Arete skills. The short version: create
`.catalog/config.toml`, run `catalog bootstrap` to generate the first
`CATALOG.md`, wire `@CATALOG.md` into `CLAUDE.md`, and register the
pre-commit hook.

## Releases

Releases are tagged from `main` and published via GoReleaser. The
primary install surface is the `westarete/homebrew-tap` Homebrew tap;
GitHub Releases carries the raw binaries as a secondary surface for CI
and non-Homebrew environments.

```sh
brew tap westarete/tap
brew trust westarete/tap
brew install catalog
```

To upgrade later:

```sh
brew upgrade catalog
```

### Making a release

Only tag a release when something changes for the user — new features,
bug fixes, behavior changes, or documentation that affects how they use
the tool. Workflow changes, CI fixes, and internal docs do not warrant a
release.

This project uses [semantic versioning](https://semver.org). Patch
releases fix bugs without changing behavior. Minor releases add
functionality. Major releases make breaking changes.

Before picking a version number, fetch tags from upstream so you have
the full picture:

```sh
git fetch --tags
git tag --sort=-v:refname
```

Increment the appropriate part of the latest tag, create an annotated
tag with a one-line summary of what changed, and push it:

```sh
git tag -a vX.Y.Z -m "Short summary of what changed"
git push origin vX.Y.Z
```

Pushing the tag triggers the GoReleaser workflow, which builds binaries,
creates the GitHub release, and opens a PR against the tap repo to
update the cask.

## macOS Gatekeeper and code signing

Released binaries are code-signed but not yet notarized. On macOS 15+,
Gatekeeper shows a "Apple could not verify catalog is free of malware"
dialog and blocks the binary — notarization is required to clear it.
This is tracked in [TODO.md](TODO.md).

If you are a West Arete developer, do not bypass Gatekeeper with
`xattr -d com.apple.quarantine`. Use a local build (see
[Development workflow](#development-workflow)) until notarization is in
place.

## Why Go

The Anthropic SDK requires a recent version of Python, and managing
Python versions on macOS is fragile — the system Python is too old, and
juggling pyenv, virtualenvs, or Homebrew Python versions adds friction
for every user who wants to use the tool. Go sidesteps all of that: a
single static binary with no runtime dependencies, distributed via
Homebrew with no version management required on the user's end.

## Development workflow

The typical cycle when working on the binary:

1. **Build locally** into `~/.local/bin` to shadow the Homebrew version:

   ```sh
   go build -o ~/.local/bin/catalog
   ```

2. **Verify** you're running the local build:

   ```sh
   which catalog   # should show ~/.local/bin/catalog
   ```

3. **Test in real repos** — run `catalog update`, `catalog check`, etc.
   against actual content repositories to confirm the change works as
   expected in practice, not just in unit tests.

4. **Release** — when satisfied, commit, tag, and push. GoReleaser
   publishes the signed binary to Homebrew.

5. **Remove the local build** to fall back to the released Homebrew
   version:

   ```sh
   rm ~/.local/bin/catalog
   brew upgrade catalog
   ```
