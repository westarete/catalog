# Release catalog

This guide tags a release and publishes it. Releases are tagged from
`main` and published via GoReleaser. The primary install surface is the
`westarete/homebrew-tap` Homebrew tap; GitHub Releases carries the raw
binaries as a secondary surface for CI and non-Homebrew environments.

[Install and run the daemon](install-and-run.md) says how people install
and update catalog with Homebrew, including the step that trusts our
cask. Keep those commands there and nowhere else, so no second copy goes
out of date.

## Make a release

Only tag a release when something changes for the user — new features,
bug fixes, behavior changes, or documentation that affects how they use
the tool. Workflow changes, CI fixes, and internal docs do not warrant a
release.

This project uses [semantic versioning](https://semver.org). Catalog is
experimental, so its versions start with 0, and semver says that in a
0.y.z version "Anything MAY change at any time." Until 1.0.0, a minor
release adds functionality or makes breaking changes, and a patch
release fixes bugs without changing behavior.

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

## Code signing and notarization

Released binaries are code-signed and notarized. They pass Gatekeeper on
macOS 15+ without any security prompt. See
[Apple code signing](apple-code-signing.md) for how the signing and
notarization pipeline works.
