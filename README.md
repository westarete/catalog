# catalog

Once a folder holds more than a few dozen files, agents do a poor job of
deciding which ones to load into context. They load files they don't
need and miss the ones they do. The catalog fixes that by keeping a
`.catalog/catalog.md` in the folder that tells an agent when to open
each file, so it can load what the task needs and leave the rest. It
does that without showing anyone more than they could already see,
because people will trust it with their documents only if it keeps them
safe. Privacy and security are part of what the catalog is for; the
[overview](docs/explanation/overview.md#privacy-and-security) says what
that means.

A background daemon keeps each `.catalog/catalog.md` current on its own,
with no catalog state in Git to conflict. The daemon is built but not
released; to try it, build it from source as
[Set up for development](docs/how-to/set-up-for-development.md#install-the-daemon-from-source)
describes. Catalog is experimental, so we may change its commands,
settings, and files in any release before 1.0.0. The
[documentation index](docs/README.md) says where to find the rest,
including why catalog works as it does. The released versions are the
earlier command-line tool, whose source and documentation are in Git
history.

## How it works

Each entry in `.catalog/catalog.md` is a profile: a description of the
conditions under which a file is relevant, the same idea as a skill's
description field, which tells the harness when to load the skill. A
profile says "open this file when the task involves X," not "this file
is about X." That distinction is what makes routing work: a wrong
summary is ugly, but a wrong profile sends the agent to the wrong file.

## Running the daemon

[Install and run the daemon](docs/how-to/install-and-run.md) installs
the daemon with Homebrew, gives it your key and folders, and puts a
catalog in place. It describes the install we plan for the first
release; until then, build from source as the developer guide says. To
move a repository from the earlier tool, see
[Move a repository from the earlier tool](docs/how-to/move-from-the-earlier-tool.md).

## Development

[Set up for development](docs/how-to/set-up-for-development.md) covers
installing what you need. Run `make test` after changing Go code and
`make markdown` after changing Markdown; CI runs both. We build the code
with the Superpowers plugin, which keeps its designs in
`docs/superpowers/specs/`. Its plans stay in `docs/superpowers/plans/`
only while someone is carrying them out; work still to do is in
[GitHub issues](https://github.com/westarete/catalog/issues).

## Releases

[Release catalog](docs/how-to/release.md) covers tagging and publishing
a release, and [Apple code signing](docs/how-to/apple-code-signing.md)
covers signing and notarization.
