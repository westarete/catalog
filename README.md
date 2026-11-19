# catalog

Once a folder holds more than a few dozen files, agents do a poor job of
deciding which ones to load into context. They load files they don't
need and miss the ones they do. The catalog fixes that by keeping a
`.catalog/catalog.md` in the folder that tells an agent when to open
each file, so it can load what the task needs and leave the rest. It
does that without showing anyone more than they could already see,
because people will trust it with their documents only if it keeps them
safe. Privacy and security are part of what the catalog is for;
[daemon.md](docs/design/daemon.md#privacy-and-security) says what that
means.

We're rewriting the catalog as a background daemon that keeps
`.catalog/catalog.md` current on its own, with no catalog state in Git
to conflict. The daemon is built but not released; to try it, build it
from source as
[Install and run the daemon](docs/how-to/install-and-run.md) describes.
[daemon.md](docs/design/daemon.md) explains why we're doing it,
[architecture.md](docs/design/architecture.md) outlines the components,
and the
[first version's spec](docs/superpowers/specs/2026-11-15-daemon-mvp-design.md)
describes it. The released versions are the earlier command-line tool,
whose source and documentation are in Git history.

## How it works

Each entry in `.catalog/catalog.md` is a profile: a description of the
conditions under which a file is relevant, the same idea as a skill's
description field, which tells the harness when to load the skill. A
profile says "open this file when the task involves X," not "this file
is about X." That distinction is what makes routing work: a wrong
summary is ugly, but a wrong profile sends the agent to the wrong file.

## Running the daemon

[Install and run the daemon](docs/how-to/install-and-run.md) builds the
daemon from source, starts it, and puts a catalog in place. To move a
repository from the earlier tool, see
[Move a repository from the earlier tool](docs/how-to/move-from-the-earlier-tool.md).

## Development

[Set up for development](docs/how-to/set-up-for-development.md) covers
installing what you need. Run `make test` after changing Go code and
`make markdown` after changing Markdown; CI runs both. We build the code
with the Superpowers plugin, which keeps its designs in
`docs/superpowers/specs/` and its plans in `docs/superpowers/plans/`.

## Releases

[Release catalog](docs/how-to/release.md) covers tagging and publishing
a release, and [Apple code signing](docs/how-to/apple-code-signing.md)
covers signing and notarization.
