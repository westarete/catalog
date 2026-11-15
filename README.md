# catalog

Once a repository holds more than a few dozen files, agents do a poor
job of deciding which ones to load into context. They load files they
don't need and miss the ones they do. The catalog fixes that by keeping
a `.catalog.md` in the folder that tells an agent when to open each
file, so it can load what the task needs and leave the rest.

We're rewriting the catalog as a background daemon that keeps
`.catalog.md` current on its own, with no catalog state in Git to
conflict. Nothing in the rewrite is built yet.
[daemon.md](docs/design/daemon.md) explains why we're doing it,
[architecture.md](docs/design/architecture.md) outlines the components,
and the
[first version's spec](docs/superpowers/specs/2026-11-15-daemon-mvp-design.md)
describes it. The released versions are the earlier command-line tool,
whose source and documentation are in Git history.

## How it works

Each entry in `.catalog.md` is a profile: a description of the
conditions under which a file is relevant, the same idea as a skill's
description field, which tells the harness when to load the skill. A
profile says "open this file when the task involves X," not "this file
is about X." That distinction is what makes routing work: a wrong
summary is ugly, but a wrong profile sends the agent to the wrong file.

## Development

[SETUP.md](SETUP.md) covers installing what you need. Run `make test`
after changing Go code and `make markdown` after changing Markdown; CI
runs both. We build the code with the Superpowers plugin, which keeps
its designs in `docs/superpowers/specs/` and its plans in
`docs/superpowers/plans/`.

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
git push origin main
git push origin vX.Y.Z
```

Pushing the tag triggers the GoReleaser workflow, which builds binaries,
creates the GitHub release, and commits the updated cask directly to the
tap repo's `main`.

## macOS Gatekeeper and code signing

Released binaries are code-signed and notarized. They pass Gatekeeper on
macOS 15+ without any security prompt. See
[apple-code-signing.md](docs/release/apple-code-signing.md) for how the
signing and notarization pipeline works.
