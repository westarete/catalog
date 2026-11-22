# Installing the daemon with Homebrew: design

A person installs the catalog daemon with `brew install`, runs
`catalog setup` to give it an API key and the folders to watch, and
after each `brew upgrade` the daemon runs the new version. Nobody edits
a property list or sets a file mode by hand. This design covers the
cask, the new `catalog setup` command, and the lines `catalog status`
adds so a person can see where the daemon's files are. It carries out
[issue #12](https://github.com/westarete/catalog/issues/12).

## Why, and what success looks like

The milestone asks that people can install the daemon and get started
from the docs alone. Today that takes building from source, copying a
property list, filling in two paths with `sed`, creating the state
folder and `global.toml` with the right modes, and loading the agent
with `launchctl`
([Install and run the daemon](../../how-to/install-and-run.md)). A
person also has no way to find `global.toml` without already knowing
where it is.

It works when:

- On a Mac with nothing installed, `brew install` followed by
  `catalog setup` leaves the daemon running with the person's key and
  folders.
- After `brew upgrade`, the daemon runs the new version without anyone
  doing anything.
- `brew uninstall` stops the daemon and removes its launch agent, and
  `brew uninstall --zap` also removes the state folder and the log.
- `catalog status` says where the launch agent, the settings, and the
  log are, and whether the settings hold an API key, even when the
  daemon isn't running.

## Goals

- One install command and one setup command, with no hand edits to a
  property list and no file modes to set.
- Upgrades that move the running daemon to the new version on their own.
- An uninstall that leaves no agent behind.
- A way to see from the command line where the daemon's files are.
- The same `catalog setup` for a person who builds from source, so the
  developer guide loses its `sed` and `launchctl` steps too.

## Non-goals

- Publishing a release. This work produces a cask that the release
  pipeline can publish; the milestone's release comes later, with other
  work in the milestone.
- An app bundle, a menu bar item, or registering the agent with
  `SMAppService`. The
  [architecture](../../explanation/architecture.md#processes-and-macos)
  expects an app eventually; that's a separate project.
- Running the daemon as a service on Linux. The cask still installs the
  program there, but nothing in this design starts it. On any system but
  macOS, `catalog setup` writes `global.toml` as on a Mac, then prints
  `Setup can start the daemon only on a Mac.`, and `catalog status`
  leaves out the launch agent line (our call).
- Changing `global.toml` after it exists. `catalog setup` creates it;
  after that, a person edits it, and status says where it is.
- Checking the API key with the provider during setup.

## Anti-goals

- The API key ending up anywhere but `global.toml`: not in a property
  list, a log, the journal, a process's arguments, the terminal's
  scrollback, or status.
- A file that the install, setup, or the daemon creates being readable
  by other accounts.
- An agent left running, or left for launchd to keep restarting, after
  the program is uninstalled.
- Setup quietly replacing a `global.toml` the person already has.

## Constraints

- Homebrew won't let a cask's install steps load an agent. Steps run
  "without network or general home-directory access"
  ([Cask Cookbook](https://docs.brew.sh/Cask-Cookbook)), and Mike
  McQuaid, a Homebrew maintainer, says a tap "can use the existing
  non-root `installer script:` mechanism for software that needs to
  register an agent during installation. Installer scripts run outside
  the cask sandbox"
  ([Homebrew/brew#23891](https://github.com/Homebrew/brew/issues/23891#issuecomment-5585219358)).
- An `installer script:` "should never be used for interactive
  installations" ([Cask Cookbook](https://docs.brew.sh/Cask-Cookbook)),
  so the step the cask runs can't ask for the key.
- Homebrew doesn't restart a service on upgrade, by choice. Asked to,
  McQuaid answered that "if there's e.g. configuration changes this may
  break the server that's running"
  ([homebrew-services#279](https://github.com/Homebrew/homebrew-services/issues/279)).
- During `brew upgrade`, Homebrew runs the old version's uninstall
  directives, skipping only `signal:`
  ([Cask Cookbook](https://docs.brew.sh/Cask-Cookbook);
  `UPGRADE_REINSTALL_SKIP_DIRECTIVES` in
  [abstract_uninstall.rb](https://github.com/Homebrew/brew/blob/b39ed2c1958182c9fc90fa87366d4510021ecf75/Library/Homebrew/cask/artifact/abstract_uninstall.rb#L41)).
  So `uninstall launchctl:` runs on every upgrade as well as on
  uninstall.
- GoReleaser generates the cask. Its `homebrew_casks` has `uninstall`,
  `zap`, `caveats`, and `custom_block`, which places raw lines in the
  cask; it has no field for `installer`
  ([GoReleaser](https://goreleaser.com/customization/publish/homebrew_casks/)).
- The daemon's own rules still hold: the state folder is 0700, every
  file in it is 0600, and launchd creates the log with the property
  list's `Umask` of 077 ([Files](../../reference/files.md)).

## Approaches

We considered four ways to get from `brew install` to a running agent.

1. **An installer script in the cask (chosen).** The cask runs
   `catalog setup --agent-only`, which writes the property list and
   loads the agent without asking anything, and declares
   `uninstall launchctl:` to remove it. On upgrade, Homebrew removes the
   old agent and the new version's installer loads it again, so the
   daemon moves to the new build with no one doing anything. This is the
   route Homebrew's maintainers recommend for a tap. It costs one thing:
   our program runs outside Homebrew's sandbox at install time, with the
   person's permissions, and without asking.
2. **The daemon restarts itself, as atuin's does.** `catalog setup`
   installs the agent, and the daemon exits when it sees its program has
   changed, so launchd starts the new one. atuin's code says
   `brew upgrade` "doesn't restart it", so its next client has launchd
   restart the daemon
   ([atuin](https://github.com/atuinsh/atuin/blob/d154a94090a7e390c1685c10252128545cdcacbd/crates/atuin-daemon/src/grpc/versioning.rs#L39-L41)).
   Upgrades work, but the cask can't declare `uninstall launchctl:`,
   since that runs on upgrade too. Uninstalling takes two commands, and
   if a person skips the first, launchd keeps trying to start a program
   that's gone.
3. **A formula with Homebrew's `service` block.** People would run
   `brew services start`. Homebrew writes that property list with mode
   0644 and has no umask setting, so the log would be readable by every
   account
   ([services/cli.rb](https://github.com/Homebrew/brew/blob/b39ed2c1958182c9fc90fa87366d4510021ecf75/Library/Homebrew/services/cli.rb#L555);
   [Formula Cookbook](https://docs.brew.sh/Formula-Cookbook#service-block-methods)).
   It also doesn't restart on upgrade or stop on uninstall, and
   GoReleaser has deprecated formulae for prebuilt programs
   ([deprecations](https://goreleaser.com/deprecations/)).
4. **An app that registers the agent with `SMAppService`.** Apple's
   framework handles upgrades cleanly, but it needs an app bundle
   ([SMAppService](https://developer.apple.com/documentation/servicemanagement/smappservice)),
   which is a separate project.

The developer chose the first, because it's the only one in which
upgrading and uninstalling each take a single Homebrew command, and it's
the way Homebrew's maintainers point taps to.

## How it works

### The cask

GoReleaser keeps generating the cask into the tap. The cask:

- installs the `catalog` binary, as it does now
- runs `catalog setup --agent-only` as an `installer script:`, without
  `sudo`, on macOS only, placed through `custom_block`
- declares `uninstall launchctl: "com.westarete.catalog"`
- declares a `zap` that deletes the state folder,
  `~/Library/Application Support/com.westarete.catalog`, and the log,
  `~/Library/Logs/catalog.log` (our call: `delete` rather than `trash`,
  because the folder holds the API key and summaries of the person's
  documents, and Trash would keep them until someone empties it)
- prints a caveat after install: run `catalog setup` to add an API key
  and the folders to watch, and `catalog status` to see how the daemon
  is doing
- gets a description of the daemon in place of today's

The label `com.westarete.catalog` ties the cask to the agent.
`catalog setup` and the cask must use the same one.

### What `catalog setup` does

`catalog setup` has two parts, and `--agent-only` runs only the second.

First, if `global.toml` doesn't exist, it asks for the API key and the
folders to watch, and writes `global.toml`:

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

The person types the text after each `Folder:`. The key they type after
`API key:` doesn't appear.

- It turns off the terminal's echo while the person types the key, so
  the key never appears on screen or in scrollback. When standard input
  isn't a terminal, it reads the lines as they come, so a script or a
  test can supply them. It never takes the key as an argument, where
  `ps` would show it.
- An empty key leaves `api_key` out of `global.toml`, and setup prints
  `To add your API key later, set api_key in that file.` The daemon then
  runs but summarizes nothing, and status already says why
  ([Commands](../../reference/commands.md#status-output)).
- Setup trims spaces around each folder and keeps spaces inside it. An
  empty line or the end of input ends the list, and if the first line is
  empty, setup watches `~/src`. It warns about any folder that doesn't
  exist but writes it anyway, since a person may create it later.
- It creates the state folder as 0700 if it's missing, as the daemon
  does, and creates `global.toml` with mode 0600, holding only the key,
  if the person gave one, and a `[[watch]]` table for each folder. Every
  other setting keeps its default
  ([Configuration](../../reference/configuration.md)).
- If `global.toml` already exists, setup asks nothing and changes
  nothing in it. It says where the file is, so the person can edit it.

Second, it installs the launch agent and starts it:

- It writes `~/Library/LaunchAgents/com.westarete.catalog.plist` from a
  template built into the program, with mode 0600. The template keeps
  today's keys: `RunAtLoad`, `KeepAlive`, `Umask` 63, and the log path
  for standard output and standard error.
- `ProgramArguments` names the program that's running, with symbolic
  links resolved, followed by `daemon`. Under Homebrew that's the
  versioned path inside the cask's folder; the installer script writes a
  new one on each upgrade. For someone who builds from source, it's
  their build.
- If the agent is already loaded, setup unloads it first, so a changed
  property list or program takes effect. Then it loads it with
  `launchctl bootstrap gui/<uid>`.
- It creates the log's folder if it's missing; launchd creates the log
  itself.

Running `catalog setup` again is safe. It asks nothing once
`global.toml` exists, and it rewrites the property list and restarts the
agent, which is also how a person who builds from source picks up a new
build.

A Mac has one agent with this label, so a source build and a Homebrew
install share it. Whichever ran `catalog setup` or the installer script
last is the program the agent runs, so `brew upgrade` switches it back
to Homebrew's build and `brew uninstall` removes it whichever program it
runs. When setup replaces a property list that named a different
program, it says which program the agent ran before and which it runs
now, and goes ahead (our call). The developer guide says this too.

The daemon's own behavior doesn't change.

### What `catalog status` adds

`catalog status` begins with three lines about the install, whether or
not the daemon is running, then a blank line and today's output:

```text
Launch agent: ~/Library/LaunchAgents/com.westarete.catalog.plist
Settings: ~/Library/Application Support/com.westarete.catalog/global.toml (API key set)
Log: ~/Library/Logs/catalog.log

Started Nov 19 09:12; last crawled Nov 19 09:31.
```

- **Launch agent** shows the property list's path, or
  `not installed; run catalog setup` if it's missing.
- **Settings** shows `global.toml`'s path, followed by `(API key set)`,
  `(no API key)`, `(not found; run catalog setup)`, or `(can't be read)`
  with the error. Status reads `global.toml` only to see whether
  `api_key` has a value. It never prints the key, and a parse error is
  reported the way the daemon already reports one, naming a line and a
  key but no value.
- **Log** shows the log's path.
- The home folder is shown as `~`.
- When the daemon isn't running, `The catalog daemon isn't running.`
  follows the blank line, and the exit code stays 1.

The rest of status, its other exit codes, and its standard-error
messages don't change.

### Upgrades

On `brew upgrade`, Homebrew uninstalls the old version and installs the
new one. The old version's `uninstall launchctl:` unloads the agent and
deletes its property list, and the new version's installer script writes
a property list naming the new program and loads it. The daemon stops
for the moment between the two and starts again on the new version. Its
catalogs, store, and settings carry over, since the daemon keeps them in
the state folder and in each catalog's folder.

We read this order in Homebrew's source but haven't run it; the builder
confirms it on a real upgrade (see
[How we'll know it works](#how-well-know-it-works)).

### Uninstalling

`brew uninstall catalog` removes the program and, through
`uninstall launchctl:`, unloads the agent and deletes its property list.
It leaves the state folder, which holds `global.toml` with the API key
and the store of summaries, and it leaves `catalog.md` in each catalog's
folder. `brew uninstall --zap catalog` also deletes the state folder and
the log. A person deletes each `catalog.md` themself, since the daemon
doesn't keep a list of where they are once it's gone. The install guide
says all of this.

## Sensitive data in this change

Each place a copy of something sensitive ends up, and who can read it
there:

- **The API key.** The person types it into `catalog setup` with echo
  off, so it isn't in the terminal's scrollback or the shell's history.
  Setup holds it in memory and writes it only to `global.toml`, which is
  0600 inside the 0700 state folder, readable only by the person's
  account. It isn't an argument to any process, isn't in the property
  list, the log, or the journal, and setup sends it nowhere. Status
  shows only whether it's set.
- **The property list.**
  `~/Library/LaunchAgents/com.westarete.catalog.plist`, mode 0600. It
  holds the program's path and the log's path, which name the person's
  home folder, as today's property list does.
- **The installer script.** At each install and upgrade, Homebrew runs
  our signed and notarized program outside its sandbox with the person's
  permissions. It writes only the property list and the state folder,
  runs `launchctl`, reads no documents, and uses no network. A tampered
  release could do anything the person's account can, but so could the
  program it installs the moment it runs (reasoning, not a change from
  today).
- **Status.** It prints the paths of the property list, `global.toml`,
  and the log to the person's terminal. The socket and who can open it
  don't change.
- **What uninstall leaves.** A plain `brew uninstall` leaves the key,
  the summaries, and the journal in the state folder, and `catalog.md`
  in each catalog. `--zap` deletes the state folder and the log. The
  install guide and the cask's caveat say what each one removes.
- **Watched folders.** Setup warns against folders that cloud drives
  sync and folders macOS protects when it asks for folders, as the
  install guide does today.

Every new file and folder has the mode the program chooses. `gosec`
stays on, and any line it flags on purpose gets `//nolint:gosec` with a
reason.

## Quality, size, and cost

Neither what the daemon summarizes nor how often it calls the model
changes, so quality, the size of each catalog, and cost all stay the
same. An upgrade restarts the daemon, which starts with a crawl; the
store keeps every summary, so the restart costs no model calls.

## Docs to change

- **How-to:**
  [Install and run the daemon](../../how-to/install-and-run.md) becomes
  `brew install`, `catalog setup`, putting a catalog in place, and
  uninstalling, including what a plain uninstall leaves.
  [Set up for development](../../how-to/set-up-for-development.md) gains
  building from source and running `catalog setup`, replacing the `sed`
  and `launchctl` steps. Until the release, the README keeps saying the
  daemon isn't released and points to the developer guide.
- **Reference:** [Commands](../../reference/commands.md) gains
  `catalog setup` and the new status lines.
  [Files](../../reference/files.md) gains the property list and says
  that `catalog setup` creates `global.toml` as 0600.
- **Explanation:**
  [Architecture](../../explanation/architecture.md#processes-and-macos)
  says the property list comes from `catalog setup` and the cask, not
  from a file a person copies.
- **Tutorial:** none yet; the milestone's setup guides will decide
  whether one is needed.

The repository's `launchd/com.westarete.catalog.plist` goes away, since
the program holds the template.

## How we'll know it works

- Go tests cover setup's prompts with injected input, the files it
  writes and their modes, the property list's contents, the `launchctl`
  calls through a fake, and each of the status lines. Gremlins runs over
  the new code, and no mutant may survive.
- On a Mac, the builder installs a cask built from a GoReleaser snapshot
  from a local tap, then checks each step: install leaves the agent
  loaded; `catalog setup` writes `global.toml` as 0600 and the daemon
  reads it; an upgrade to a second snapshot leaves the daemon running
  the new build; uninstall leaves no agent and no property list; `--zap`
  removes the state folder and the log. The builder uses a macOS account
  that isn't running a daemon already, because both would use the label
  `com.westarete.catalog`.
- `/security-review` runs on the branch, along with the whole-branch
  review.

## Left to the builder

- The exact `launchctl` steps for unloading, loading, and re-enabling
  the agent, from `launchctl(1)`.
- The terminal library for reading the key with echo off, from its
  current documentation.
- How to express the installer script and the macOS-only condition in
  GoReleaser's `custom_block`, and checking the generated cask with
  `brew style` and `brew audit`.
- How to install a snapshot cask from a local tap, including Homebrew's
  tap trust.
