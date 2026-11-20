# Install and run the daemon

This guide builds the daemon from source, starts it under launchd, and
puts a catalog in place in one repository. We haven't released the
daemon through Homebrew yet, so you build it from source. You need Go
and a Mac. Run these from a checkout of this repository.

Build the daemon into the folder on your `PATH` where you keep programs
you install yourself, putting that folder in place of `<folder>`:

```sh
go build -o <folder>/catalog ./cmd/catalog
```

Check that `command -v catalog` prints the path you just built to. Then
copy the launchd property list into your agents folder. It needs
absolute paths, so fill in the program's path and your home folder:

```sh
cp launchd/com.westarete.catalog.plist ~/Library/LaunchAgents/
sed -i '' -e "s|CATALOG_PROGRAM|$(command -v catalog)|" -e "s|YOUR_HOME|$HOME|g" ~/Library/LaunchAgents/com.westarete.catalog.plist
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

[Configuration](../reference/configuration.md) lists every key.

## Put a catalog in place

The daemon keeps a catalog wherever a `.catalog/` folder exists inside a
watched folder. It never creates one, so for each repository you want
cataloged:

1. Make an empty `.catalog/` folder at the root of the repository. The
   daemon writes `.catalog/catalog.md` there. An optional
   `.catalog/config.toml` can set `enumerate` and `ignore` for that
   catalog; the same two keys in `global.toml` set the defaults for your
   machine.
2. Have your `AGENTS.md` or `CLAUDE.md` load `@.catalog/catalog.md`. If
   you have both files, import `@AGENTS.md` from `CLAUDE.md`, because by
   default Claude Code reads `AGENTS.md` only when there's no
   `CLAUDE.md` in the folder or above it.
3. If the repository is in Git, you will probably want Git to ignore
   `.catalog/catalog.md`, since that's the file that used to conflict.
   You can set that once for every repository on your Mac in Git's
   global ignore file
   ([gitignore documentation](https://git-scm.com/docs/gitignore)).

[Where catalogs go](../explanation/catalogs.md#what-a-tree-needs) says
why a tree needs each of these.

## Load the agent

Load the agent into your login session, and check on it:

```sh
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.westarete.catalog.plist
catalog status
```

If `catalog status` says the daemon isn't running,
`tail ~/Library/Logs/catalog.log` shows why. To stop it, run
`launchctl bootout gui/$(id -u)/com.westarete.catalog`.

`catalog status` shows when the daemon last crawled, each catalog's file
counts, and any files that are failing. On the first day the daemon
summarizes one file at a time, so expect catalogs to fill in gradually.
Edits to `global.toml` and to each `config.toml` take effect within a
minute, with no restart. [Commands](../reference/commands.md) describes
`catalog status`, and [Files](../reference/files.md) lists where the
daemon keeps its files.

## Update the daemon

launchd keeps running the build it started, so after you pull a newer
version, stop the agent, build over the program where it is, copy the
property list again in case it changed, and load it:

```sh
launchctl bootout gui/$(id -u)/com.westarete.catalog
go build -o "$(command -v catalog)" ./cmd/catalog
cp launchd/com.westarete.catalog.plist ~/Library/LaunchAgents/
sed -i '' -e "s|CATALOG_PROGRAM|$(command -v catalog)|" -e "s|YOUR_HOME|$HOME|g" ~/Library/LaunchAgents/com.westarete.catalog.plist
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.westarete.catalog.plist
```

## Which folders to watch

Watch only folders that no cloud drive syncs, such as `~/src`. The
daemon can't yet tell a synced folder from any other, and it must never
write into one. This version hasn't been made to read the folders macOS
protects, so don't watch Desktop, Documents, Downloads, or iCloud Drive.
[Where catalogs go](../explanation/catalogs.md#why-to-watch-only-folders-that-no-cloud-drive-syncs)
explains why.

A catalog's `enumerate` can cover more of its folder than the global
list does, so a config that arrives with a clone decides which of that
folder's files the daemon summarizes and sends to your provider.
