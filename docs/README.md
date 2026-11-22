# Documentation

These docs cover how to install and run catalog, what each command,
setting, and file does, and why catalog works the way it does. If you
want to get catalog running, start with
[Install and run the daemon](how-to/install-and-run.md). If you're
looking up a command, a setting, or a file, go to the reference. If you
want to know why catalog is built this way, begin with the
[overview](explanation/overview.md).

## How-to guides

These are steps for a task, in order.

- [Install and run the daemon](how-to/install-and-run.md) installs the
  daemon with Homebrew, gives it a key and folders with `catalog setup`,
  puts a catalog in place, and uninstalls it.
- [Move a repository from the earlier tool](how-to/move-from-the-earlier-tool.md)
  switches a repository that has a catalog from the earlier command-line
  tool to the daemon.
- [Set up for development](how-to/set-up-for-development.md) installs
  what you need to work on catalog's code and docs, and builds the
  daemon from source.
- [Release catalog](how-to/release.md) covers tagging and publishing a
  release.
- [Apple code signing](how-to/apple-code-signing.md) covers signing and
  notarizing the macOS build.

## Reference

These describe what catalog does, for looking things up.

- [Commands](reference/commands.md) lists `catalog daemon`,
  `catalog daemon --once`, `catalog status`, and `catalog setup`, with
  their arguments, output, and exit codes.
- [Configuration](reference/configuration.md) lists every key in
  `global.toml` and in a catalog's `config.toml`.
- [Files](reference/files.md) lists each file and socket the daemon
  reads or writes, with its location, its mode, and who can read it.
- [Catalog format](reference/catalog-format.md) describes the
  `.catalog/catalog.md` that agents read.

## Explanation

These say why catalog works as it does, and which parts of that design
aren't built yet.

- [Overview of catalog](explanation/overview.md) covers what the catalog
  is for, who it is for, and how it protects privacy and security.
- [Where catalogs go](explanation/catalogs.md) covers where agents
  start, nesting, cloud drives, and delivering the catalog without a
  file.
- [Profiles](explanation/profiles.md) covers what a profile is, and how
  we trade time, quality, and cost.
- [Architecture of the daemon](explanation/architecture.md) covers the
  components, the rules between them, and the platforms.

## Research, best practices, and dated records

These three folders aren't one of the Diátaxis types.

- [research](research/) holds our notes on the sources we learn from,
  such as the documentation of tools and platforms we might build on or
  borrow from.
- [best-practices](best-practices/) holds the people whose practice we
  follow, in [craftspeople.md](best-practices/craftspeople.md).
- [superpowers](superpowers/) holds the specs we write before building.
  Each is dated and records one decision, so it can describe catalog as
  it was then. The explanation and reference docs are the current
  picture. A plan sits beside its spec only while someone is carrying it
  out; work still to do is in
  [GitHub issues](https://github.com/westarete/catalog/issues).

## How we organize these docs

We organize the docs with [Diátaxis](https://diataxis.fr/), which sorts
them by what the reader needs: tutorials, how-to guides, reference, and
explanation. [craftspeople.md](best-practices/craftspeople.md) says what
we take from it. We don't have tutorials yet, so there's no tutorials
folder.
