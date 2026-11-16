# catalog

Once a repository holds more than a few dozen files, agents do a poor
job of deciding which ones to load into context. They load files they
don't need and miss the ones they do. The catalog fixes that by keeping
a `.catalog/catalog.md` in the folder that tells an agent when to open
each file, so it can load what the task needs and leave the rest.

We're rewriting the catalog as a background daemon that keeps
`.catalog/catalog.md` current on its own, with no catalog state in Git
to conflict. The daemon is built but not released; to try it, build it
from source as described under "Running the daemon" below.
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

We haven't released the daemon through Homebrew yet, so you build it
from source. You need Go and a Mac. Run these from a checkout of this
repository. They put the binary in `~/bin`, put it on your `PATH` for
zsh, and copy the launchd property list into your agents folder:

```sh
mkdir -p ~/bin
go build -o ~/bin/catalog ./cmd/catalog
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
cp launchd/com.westarete.catalog.plist ~/Library/LaunchAgents/
```

Open a new terminal so the `PATH` change takes effect. The property list
needs absolute paths, so fill in your user name:

```sh
sed -i '' "s/YOUR_NAME/$(whoami)/g" ~/Library/LaunchAgents/com.westarete.catalog.plist
```

The daemon logs to `~/Library/Logs/catalog.log`.

Next, create the settings folder and a `global.toml` in it with your API
key and the folders to watch. The daemon reads the key only from this
file, not from `ANTHROPIC_API_KEY`. The folder also holds summaries of
your documents, so only you can open it:

```sh
mkdir -m 700 -p "$HOME/Library/Application Support/com.westarete.catalog"
```

```toml
api_key = "sk-ant-..."
model = "claude-haiku-5-5"

[[watch]]
folder = "~/src"
```

Choose the watched folders with care; see "Which folders to watch"
below. Because the file holds your API key, make it readable only by
you:

```sh
chmod 600 "$HOME/Library/Application Support/com.westarete.catalog/global.toml"
```

The daemon keeps a catalog wherever a `.catalog/` folder exists inside a
watched folder. It never creates one, so make an empty `.catalog/`
folder at the root of each repository you want cataloged. The daemon
writes `.catalog/catalog.md` there. An optional `.catalog/config.toml`
can set `enumerate` and `ignore` for that catalog; the same two keys in
`global.toml` set the defaults for your machine. Have your `AGENTS.md`
or `CLAUDE.md` load `@.catalog/catalog.md`.

Load the agent into your login session, and check on it:

```sh
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.westarete.catalog.plist
catalog status
```

If `catalog status` says the daemon isn't running,
`tail ~/Library/Logs/catalog.log` shows why. To stop it, run
`launchctl bootout gui/$(id -u)/com.westarete.catalog`.

launchd keeps running the build it started, so after you pull a newer
version, stop the agent, build again, copy the property list again in
case it changed, and load it:

```sh
launchctl bootout gui/$(id -u)/com.westarete.catalog
go build -o ~/bin/catalog ./cmd/catalog
cp launchd/com.westarete.catalog.plist ~/Library/LaunchAgents/
sed -i '' "s/YOUR_NAME/$(whoami)/g" ~/Library/LaunchAgents/com.westarete.catalog.plist
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.westarete.catalog.plist
```

`catalog status` shows when the daemon last crawled, each catalog's file
counts, and any files that are failing. On the first day the daemon
summarizes one file at a time, so expect catalogs to fill in gradually.
Edits to `global.toml` and to each `config.toml` take effect within a
minute, with no restart.

### Which folders to watch

Watch only folders that no cloud drive syncs, such as `~/src`. The
daemon can't yet tell a synced folder from any other, and it must never
write into one. This version also doesn't watch folders that macOS
protects: Desktop, Documents, Downloads, and iCloud Drive.

### Moving a repository from the earlier tool

Move all your repositories in one sitting. Both tools are called
`catalog`, and `catalog status` means something different in each. Your
agents also need the updated `knowledge-work:catalog` skill first, so
none of them runs `catalog update` in a repository you've moved.

1. Remove the Homebrew `catalog` and install the new build in its place.
2. In each repository, delete `.catalog/catalog.db` and the old
   `.catalog.md` at the root. Keep `.catalog/config.toml`; the daemon
   reads its `enumerate` and `ignore`. Change the line that loads the
   catalog from `@.catalog.md` to `@.catalog/catalog.md`.
3. Add the folders that hold your repositories to the watch list in
   `global.toml`.
4. Load the launchd agent and check `catalog status`.

If your repository used to commit its catalog, you will probably want
Git to ignore `.catalog/catalog.md` now, since that's the file that used
to conflict. In a shared repository, these steps change files everyone
sees, so tell the others when you move it.

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
