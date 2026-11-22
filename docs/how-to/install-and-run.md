# Install and run the daemon

This guide installs the daemon with Homebrew, gives it your API key and
the folders to watch, and puts a catalog in place in one repository. You
need a Mac. To build the daemon from source instead, follow
[Set up for development](set-up-for-development.md), then pick up here
at `catalog setup`.

Add our Homebrew tap, trust the catalog cask in it, then install the
daemon:

```sh
brew tap westarete/tap
brew trust --cask westarete/tap/catalog
brew install catalog
```

Don't skip the `brew trust` step or replace these commands with a
shorter one. Since Homebrew 6.0.0, installing a cask from a tap that
Homebrew doesn't maintain fails until you trust it, as
[Tap Trust](https://docs.brew.sh/Tap-Trust) explains. The step trusts
only the catalog cask, not everything we might add to the tap later.
Homebrew's maintainers recommend that for software from a vendor you
don't control.

The install puts the `catalog` program on your `PATH` and starts the
daemon's launch agent, which runs the daemon in the background. The
daemon can't summarize anything yet, because it has no API key and no
folders to watch.

## Give the daemon its key and folders

Run:

```sh
catalog setup
```

Setup asks for your Anthropic API key and then for the folders to watch.
What you type is shown after each prompt. Nothing appears as you type
the key, so the line after `API key:` looks empty:

```text
$ catalog setup
Paste your Anthropic API key and press Return, or press Return to skip this step.
API key:

The daemon will watch the folders you list. Don't list a folder that a cloud drive syncs, or Desktop, Documents, or Downloads.
Enter one folder per line, then press Return on an empty line to finish. To watch only ~/src, press Return now.
Folder: ~/src
Folder: ~/notes/Project Plans
Folder:

Your settings are in ~/Library/Application Support/com.westarete.catalog/global.toml, and only you can read that file.
The catalog daemon is running. To see what it's doing, run `catalog status`.
```

Type each folder as a full path that starts with `/` or `~/`. The daemon
starts in `/`, so a path like `src` would name the wrong place, and
setup asks again if you type one. Choose the folders with care; see
"Which folders to watch" below. If you press Return at the first
`Folder:` prompt, the daemon watches `~/src`. If you press Return at the
key prompt, setup leaves the key out, and the daemon runs but summarizes
nothing until you add `api_key` to `global.toml`.

The key lives only in `global.toml`, in a folder and a file that only
you can open. The daemon doesn't read it from `ANTHROPIC_API_KEY` or
from a command-line argument, so setup never asks for it that way. Press
Ctrl-C at any prompt to stop setup with nothing written.

You can run `catalog setup` again whenever you like. If `global.toml`
exists, setup leaves it alone and tells you where it is; to change the
key or the folders, edit that file.
[Configuration](../reference/configuration.md) lists every key it can
hold. Setup also writes the launch agent's property list,
`~/Library/LaunchAgents/com.westarete.catalog.plist`, and loads the
agent again.

Check on the daemon:

```sh
catalog status
```

It begins with where the launch agent, your settings, and the log are,
and whether the settings hold a key. If it says the daemon isn't
running, `tail ~/Library/Logs/catalog.log` shows why. To stop the daemon
without uninstalling it, run
`launchctl bootout gui/$(id -u)/com.westarete.catalog`; `catalog setup`
starts it again.

`catalog status` also shows when the daemon last crawled, each catalog's
file counts, and any files that are failing. On the first day the daemon
summarizes one file at a time, so expect catalogs to fill in gradually.
Edits to `global.toml` and to each `config.toml` take effect within a
minute, with no restart. [Commands](../reference/commands.md) describes
`catalog setup` and `catalog status`, and [Files](../reference/files.md)
lists where the daemon keeps its files.

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

## Update the daemon

Update with Homebrew:

```sh
brew upgrade catalog
```

Then run `catalog status`. If its first line says the launch agent isn't
installed, or the daemon isn't running, run `catalog setup`, which loads
the agent for the new version.

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

## Uninstall the daemon

Uninstalling leaves your key and your summaries on disk unless you ask
for them to be deleted, so choose the command that matches what you
want.

```sh
brew uninstall catalog
```

This removes the program, stops the daemon, and removes the launch agent
and its property list. Our cask asks Homebrew to remove the launchd job
([Cask Cookbook, uninstall](https://docs.brew.sh/Cask-Cookbook#stanza-uninstall)),
and Homebrew's
[source](https://github.com/Homebrew/brew/blob/b39ed2c1958182c9fc90fa87366d4510021ecf75/Library/Homebrew/cask/artifact/abstract_uninstall.rb#L288-L314)
shows that it then deletes the property list. It leaves these behind:

- The state folder,
  `~/Library/Application Support/com.westarete.catalog`, which holds
  `global.toml` with your API key, the store of summaries of your
  documents, and the journal.
- The log, `~/Library/Logs/catalog.log`.
- Each `catalog.md` in your catalogs.

To delete the state folder and the log too, uninstall with `--zap`:

```sh
brew uninstall --zap catalog
```

The Cask Cookbook says zap steps run only when you pass `--zap`
([zap stanza](https://docs.brew.sh/Cask-Cookbook#stanza-zap)). Our
cask's zap deletes the state folder and the log, in place of moving them
to the Trash, where they would stay readable until you emptied it.

Neither command touches your catalogs. The daemon keeps no list of where
they are, so delete each `.catalog/catalog.md` yourself, along with any
`.catalog/` folder you no longer want. To stop the daemon without
deleting anything, see the `launchctl bootout` command above.

If you built the daemon from source, `brew uninstall` removes the
program it installed, not yours. A source build and a Homebrew install
share one launch agent. If the cask is installed alongside a source
build, `brew uninstall` removes that launch agent and its property list
even when the agent was running your build
([`.goreleaser.yaml`](../../.goreleaser.yaml) says what the cask's
uninstall does), but it leaves your build and everything else above.
[Remove a source build](set-up-for-development.md#remove-a-source-build)
gives the steps for that case.
