# Install and run the daemon

This guide installs the daemon with Homebrew, gives it your API key and
the folders to watch, and puts a catalog in place in one folder. You
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

Your API key is in /Users/alex/Library/Application Support/com.westarete.catalog/keys.toml, and only you can read that file.
Your settings are in /Users/alex/Library/Application Support/com.westarete.catalog/global.toml, and only you can read that file.
The catalog daemon is running. To see what it's doing, run `catalog status`.
```

Type each folder as a full path that starts with `/` or `~/`. The daemon
starts in `/`, so a path like `src` would name the wrong place, and
setup asks again if you type one. Choose the folders with care; see
"Which folders to watch" below. If you press Return at the first
`Folder:` prompt, the daemon watches `~/src`. If you press Return at the
key prompt, setup still creates `keys.toml`, holding only a comment, and
the daemon runs but summarizes nothing until you add a line like
`default = "sk-example-not-a-real-key"` to that file, with your own key.

Setup keeps the key in `keys.toml` under the name `default`, and the
folders in `global.toml`. The key lives only in `keys.toml`, in a folder
and a file that only you can open, so `global.toml` holds no secret. The
daemon doesn't read it from `ANTHROPIC_API_KEY` or from a command-line
argument, so setup never asks for it that way. Press Ctrl-C at any
prompt to stop setup with nothing written.

You can run `catalog setup` again whenever you like. Setup leaves alone
whichever of `keys.toml` and `global.toml` exists and tells you where it
is, and asks only for what goes in a missing one. To change the key,
edit `keys.toml`; to change the folders, edit `global.toml`. If
`keys.toml` exists without your key, setup says how to add it.
[Configuration](../reference/configuration.md) lists every key both
files can hold. Setup also writes the launch agent's property list,
`~/Library/LaunchAgents/com.westarete.catalog.plist`, and loads the
agent again.

Check on the daemon:

```sh
catalog status
```

It begins with where the launch agent, your settings, your keys, and the
log are, whether your settings load, and whether `keys.toml` has your
key. If it says the daemon isn't running,
`tail ~/Library/Logs/catalog.log` shows why. To stop the daemon without
uninstalling it, run
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

The daemon keeps a catalog in each folder that has a
`.catalog/config.toml` file, inside the folders it watches. It never
creates one, so for each folder you want cataloged, such as a project's
folder or a repository:

1. Run `catalog init` with the folder, or with no folder to use the one
   you're in:

   ```text
   $ catalog init ~/src/acme/website
   Created /Users/alex/src/acme/website/.catalog/config.toml
   Created /Users/alex/src/acme/website/.catalog/.gitignore
   The daemon watches this folder.
   To have agents read this catalog, add this line to the folder's AGENTS.md or CLAUDE.md: @.catalog/catalog.md
   ```

   `catalog init` writes `.catalog/config.toml`, which makes the folder
   a catalog. Its comments show the settings you can add there to cover
   more files or leave some out; the same settings in `global.toml`
   apply to every catalog on your Mac. The daemon writes
   `.catalog/catalog.md` beside it on its next crawl, usually within a
   minute.

   In a Git repository, as in this example, `catalog init` also writes
   `.catalog/.gitignore`, which keeps `catalog.md` out of your commits.
   Outside a repository it writes no `.gitignore`. If `catalog init`
   says no folder the daemon watches contains this one, add the folder,
   or one above it, to `watch` in `global.toml`.

2. Have your `AGENTS.md` or `CLAUDE.md` load `@.catalog/catalog.md`, as
   `catalog init` says. If you have both files, import `@AGENTS.md` from
   `CLAUDE.md`, because by default Claude Code reads `AGENTS.md` only
   when there's no `CLAUDE.md` in the folder or above it.

`catalog init` never changes a file that exists, so it's safe to run in
a folder that is already a catalog.
[Commands](../reference/commands.md#catalog-init) describes what it
writes, and
[Where catalogs go](../explanation/catalogs.md#what-a-tree-needs) says
why a folder needs each of these.

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

A catalog's `include` can cover more of its folder than the global lists
do, so a config that arrives with a clone decides which more of that
folder's files the daemon summarizes and sends to your provider. It
can't bring back a file you excluded in `global.toml`.

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
  `keys.toml` with your API key, `global.toml`, the store of summaries
  of your documents, and the journal.
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
