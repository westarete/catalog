# Commands

The `catalog` program has three commands: `catalog daemon`, which keeps
catalogs current, `catalog status`, which asks a running daemon what it
is doing, and `catalog setup`, which gives the daemon its settings and
starts it. This page lists each command's arguments, output, and exit
codes. For the reasons behind the daemon's design, see the
[architecture](../explanation/architecture.md); for the settings it
reads, see [configuration](configuration.md).

## Synopsis

```text
catalog daemon
catalog daemon --once <folder>
catalog status
catalog setup [--agent-only]
```

Any other command line prints the usage text to standard error and exits
with code 2:

```text
usage: catalog daemon [--once <folder>]
       catalog status
       catalog setup [--agent-only]
```

These command lines are usage errors:

- no arguments
- an unknown command
- `status` with any argument
- `setup` with an argument other than `--agent-only`
- `daemon` with an argument but no `--once`
- `daemon --once` with no folder, or with more than one
- an unknown flag, which also prints the flag package's own message
  first

`--once` takes one dash or two, as Go's `flag` package does, and it must
come before the folder.

## Exit codes

| Code | Meaning                                                                                              |
| ---- | ---------------------------------------------------------------------------------------------------- |
| 0    | The command finished. `catalog daemon` returns 0 after it receives SIGINT or SIGTERM and shuts down. |
| 1    | The run failed, or `catalog status` found no daemon running.                                         |
| 2    | The command line wasn't understood.                                                                  |

When a command fails with code 1, it writes `catalog:` and the error to
standard error, except where a command below says otherwise.

## catalog daemon

Runs the daemon in the foreground until it receives SIGINT or SIGTERM.
It does not detach; launchd runs it in the background, once
[`catalog setup`](#catalog-setup) has installed the launch agent.

On start, before it does anything else, the daemon creates the state
folder if it is missing and sets the folder's mode to 0700. If a journal
exists there, it sets the journal's mode to 0600.
[Configuration](configuration.md#the-state-folder) gives the folder's
path.

Then it:

1. Fails with code 1 if another daemon already answers on the status
   socket, with the message
   `another catalog daemon is already running; it answers on <socket path>`.
2. Opens the store, `store.db`, in the state folder. If it can't, it
   fails with code 1.
3. Starts answering `catalog status` on the socket `control.sock` in the
   state folder. If it can't listen there, it writes
   `status won't answer: <error>` to standard error and keeps working.
4. Crawls the watched folders at once and then every minute, summarizing
   waiting files between crawls. It rereads `global.toml` on each crawl,
   so a missing or broken file doesn't stop it; `catalog status` reports
   the problem instead.

When the daemon stops, it removes the socket.

The daemon writes to standard error each error it can't record in its
journal or act on, one line each, prefixed with a timestamp and the word
`catalog:`. It writes nothing to standard output. Under the launch
agent's property list, which `catalog setup` writes, both go to
`~/Library/Logs/catalog.log`.

## catalog daemon --once

```text
catalog daemon --once <folder>
```

Crawls `<folder>` as the only watched folder, summarizes every file
waiting in the catalogs it finds there, prints the result, and exits. It
doesn't wait for a file to stay unchanged first, which the running
daemon does. It doesn't answer status requests.

It reads the state folder's `global.toml` for the API key, the model,
and the `enumerate` and `ignore` lists. It ignores the `watch` entries
and watches only `<folder>`, made absolute against the current
directory. It doesn't reread `global.toml` during the run.

Unlike the running daemon, it stops at once, with code 1, in these
cases:

| Case                                              | Message on standard error                                                                              |
| ------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| `global.toml` doesn't exist                       | `catalog: no settings file at <path>; create it with your api_key`                                     |
| `global.toml` doesn't parse                       | `catalog: <path>: parse global.toml: line <n>: not valid TOML`, or another reason that quotes no value |
| `global.toml` has no `api_key`                    | `catalog: <path> has no api_key`                                                                       |
| `global.toml` is world-readable or world-writable | `catalog: <path>: world-readable or world-writable (mode <mode>); to fix it, run chmod o-rw on it`     |

When the run reaches the crawl, it prints a report in the format of
`catalog status` (below) to standard output, even if the run fails.
These cases then exit with code 1:

- The model service fails. The run stops at the first service problem
  and writes the error to standard error.
- The folder holds no catalog, whether because it can't be read or
  because nothing in it has a `.catalog/` folder. The error is
  `catalog: no catalog in <absolute folder>: it can't be read, or nothing in it has a .catalog/ folder`,
  and the report says `No catalogs found.`
- The store or the journal fails.

Otherwise it exits with code 0.

## catalog setup

```text
catalog setup [--agent-only]
```

Gives the daemon its settings and starts it. It has two parts, and
`--agent-only` runs only the second. The Homebrew cask runs
`catalog setup --agent-only` when it installs, so the person is asked
nothing at install time.

### Asking for settings

If `global.toml` doesn't exist in the state folder, setup asks for the
Anthropic API key and the folders to watch, then writes the file:

```text
Paste your Anthropic API key and press Return, or press Return to skip this step.
API key:

The daemon will watch the folders you list. Don't list a folder that a cloud drive syncs, or Desktop, Documents, or Downloads.
Enter one folder per line, then press Return on an empty line to finish. To watch only ~/src, press Return now.
Folder: ~/src
Folder: ~/notes/Project Plans
Folder:

Your settings are in ~/Library/Application Support/com.westarete.catalog/global.toml, and only you can read that file.
```

- When standard input is a terminal, setup turns off echo while the key
  is typed, so the key never appears on screen. Otherwise it reads the
  key as one line from standard input. It takes no key as an argument.
- An empty key leaves `api_key` out of the file, and setup adds
  `To add your API key later, set api_key in that file.` The daemon then
  runs but summarizes nothing, and `catalog status` says why.
- Setup trims spaces around each folder and keeps spaces inside it. A
  folder must start with `/` or `~/`, or be exactly `~`, because launchd
  starts the daemon in `/`. For anything else, setup prints
  `Enter a full path that starts with / or ~/.` on its own line and
  prompts `Folder:` again; the rejected line doesn't end the list. An
  empty line or the end of input ends the list. If the first line is
  empty, setup watches `~/src`. For a folder that doesn't exist, it
  prints
  `<folder> doesn't exist yet. The daemon will start watching it once you create it.`
  and writes it anyway.
- Setup creates the state folder with mode 0700 if it's missing, and
  creates `global.toml` with mode 0600, holding the key if one was given
  and a `[[watch]]` table for each folder. Every other setting keeps its
  default ([Configuration](configuration.md)).
- If `global.toml` already exists, setup asks nothing, changes nothing
  in it, and prints
  `Your settings are already in <path>. To change them, edit that file.`
- If the person presses Ctrl-C at a prompt, setup stops with exit code 1
  and `catalog: setup interrupted` on standard error, before it writes
  `global.toml` or the property list or loads the agent.

### Installing the launch agent

On a Mac, setup then writes
`~/Library/LaunchAgents/com.westarete.catalog.plist` with mode 0600,
naming the running program, with symbolic links resolved, followed by
`daemon`. It unloads the agent first if launchd has it loaded, then
loads it with `launchctl bootstrap`, so a running daemon restarts on the
program that ran setup. It creates the folder for the log if that is
missing. [Files](files.md#the-launch-agents-property-list) describes the
property list.

If the replaced property list named a different program, setup prints
`The daemon was running <old program>, and now it runs <new program>.`
It then prints
``The catalog daemon is running. To see what it's doing, run `catalog status`.``

A Mac has one agent with this label, so a source build and a Homebrew
install share it, and the agent runs whichever program ran setup last.
Off a Mac, setup prints `Setup can start the daemon only on a Mac.` and
exits with code 0, after asking for settings unless `--agent-only` was
given.

### Setup's exit codes

| Code | Meaning                                                                                      |
| ---- | -------------------------------------------------------------------------------------------- |
| 0    | Setup finished. Off a Mac, that is after it writes `global.toml`.                            |
| 1    | Setup failed, or the person pressed Ctrl-C. The error is on standard error after `catalog:`. |
| 2    | The command line wasn't understood, such as an argument other than `--agent-only`.           |

## catalog status

```text
catalog status
```

Connects to the daemon's socket, `control.sock` in the state folder, and
prints the daemon's latest status to standard output. It waits at most 5
seconds for the connection and for the answer. It takes no arguments.

| Situation                                                | Standard output                     | Standard error                                                                  | Code |
| -------------------------------------------------------- | ----------------------------------- | ------------------------------------------------------------------------------- | ---- |
| The daemon answers                                       | The status, as described below      | Nothing                                                                         | 0    |
| There is no socket, or nothing accepts connections on it | `The catalog daemon isn't running.` | Nothing                                                                         | 1    |
| Something that isn't a socket is at the socket's path    | The install lines                   | `catalog: <path> is not a socket; move it aside so the daemon can listen there` | 1    |
| The connection or the read fails                         | The install lines                   | `catalog: <error>`                                                              | 1    |

The install lines described below come first in standard output in every
row. The first case is the only one in which the not-running message
goes to standard output.

### Status output

Whether or not the daemon is running, the output begins with three lines
about the install, then a blank line:

```text
Launch agent: ~/Library/LaunchAgents/com.westarete.catalog.plist
Settings: ~/Library/Application Support/com.westarete.catalog/global.toml (API key set)
Log: ~/Library/Logs/catalog.log

```

- `Launch agent:` shows the property list's path, or
  `not installed; run catalog setup` if the file is missing. Off a Mac,
  this line is left out.
- `Settings:` shows `global.toml`'s path, then `(API key set)`,
  `(no API key)`, `(not found; run catalog setup)`, or
  `(can't be read: <error>)`. Status reads the file only to see whether
  `api_key` has a value, and never prints the key.
- `Log:` shows the log's path.

The home folder is shown as `~`. When the daemon isn't running,
`The catalog daemon isn't running.` follows the blank line.

The rest of the status is plain lines. Times are in the local time zone,
in the form `Jan 2 15:04`.

```text
Started Nov 19 09:12; last crawled Nov 19 09:31.
Config error: /Users/alex/Library/Application Support/com.westarete.catalog/global.toml: not found; create it with your api_key and the folders to watch
Summarizing paused until Nov 19 09:33: the model service is not answering

Catalogs:
  /Users/alex/src/notes: 41 files, 38 summarized, 3 waiting, 5 to update

Failing:
  /Users/alex/src/notes/drafts/plan.pdf: unreadable (retry next crawl)
```

The lines appear in this order:

1. The first line. Before the first crawl ends, it reads
   `Started <time>; the first crawl is under way.`, or
   `Starting; the first crawl is under way.` if the daemon hasn't
   recorded a start time, and nothing else follows. After the first
   crawl it reads `Started <time>; last crawled <time>.`
2. One `Config error: <path>: <error>` line for each config file that
   didn't load on the last crawl, sorted by path. The path is
   `global.toml`'s path, or the folder of a catalog whose
   `.catalog/config.toml` didn't load.
3. A pause line, if the daemon isn't summarizing. It reads
   `Summarizing paused until <time>: <reason>` when the daemon is
   waiting out a model service problem, and `Not summarizing: <reason>`
   when it can't summarize until someone fixes `global.toml`. With no
   API key loaded, the reason is `no API key loaded from global.toml`.
4. A blank line, then `Catalogs:` with one line per catalog, sorted by
   path, indented two spaces:
   `<folder>: <n> files, <n> summarized, <n> waiting, <n> to update`. A
   file counts as summarized when `catalog.md` has a summary of its
   content, under any recipe, and as waiting otherwise. Of the
   summarized files, `to update` counts the ones whose summary the
   daemon will write again: a summary written while another file in the
   catalog had none, as
   [the second pass](../explanation/profiles.md#the-second-pass)
   explains, or one under an older recipe. The recipe includes the
   model's name, so a change of `model` counts every summarized file as
   one to update. If the last crawl found no catalogs, the blank line is
   followed by `No catalogs found.` instead.
5. If any file or folder is failing, a blank line, then `Failing:` with
   one line per problem, sorted by folder and path, indented two spaces:
   `<path>: <error> (retry <time>)`. The retry is `next crawl` when the
   daemon tries again on every crawl.

The daemon's status reflects its state after the last crawl or the last
file it summarized.
