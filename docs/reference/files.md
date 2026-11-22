# Files

The daemon and `catalog setup` keep their files in one state folder, and
the daemon writes one file, `catalog.md`, into each catalog. This page
lists every file and socket it creates, reads, or sends, where each one
is, its mode, who can read it, and what it holds. For the commands that
use them, see [commands](commands.md); for the settings files, see
[configuration](configuration.md).

All paths below are for the account that runs the daemon. A file with
mode 0600 is readable by its owner only, and a folder with mode 0700 is
open to its owner only.

## Summary

| Path                                                 | Created by                      | Mode                                 | What it holds                                     |
| ---------------------------------------------------- | ------------------------------- | ------------------------------------ | ------------------------------------------------- |
| `<state folder>`                                     | the daemon, or `catalog setup`  | 0700                                 | the files below, except `catalog.md` and the log  |
| `<state folder>/global.toml`                         | `catalog setup`, or a person    | 0600 from `catalog setup`            | the API key and the global settings               |
| `<state folder>/store.db`                            | the daemon                      | 0600                                 | the store: files seen, summaries, failures        |
| `<state folder>/store.db-wal`, `store.db-shm`        | SQLite, while the store is open | 0600                                 | recent writes to the store, and its shared memory |
| `<state folder>/journal.jsonl`                       | the daemon                      | 0600                                 | one line for each event the daemon records        |
| `<state folder>/control.sock`                        | the daemon                      | 0600                                 | the status socket                                 |
| `<catalog folder>/.catalog/catalog.md`               | the daemon                      | 0600                                 | the catalog agents read                           |
| `<catalog folder>/.catalog/catalog-*.tmp`            | the daemon, briefly             | 0600                                 | a `catalog.md` being written                      |
| `~/Library/Logs/catalog.log`                         | launchd                         | 0600, from the property list's umask | the daemon's standard error                       |
| `~/Library/LaunchAgents/com.westarete.catalog.plist` | `catalog setup`                 | 0600                                 | the launch agent: the program to run and the log  |

The state folder is
`~/Library/Application Support/com.westarete.catalog`, with `~` the home
folder from `$HOME`.

## Modes the daemon sets

On every start, `catalog daemon` and `catalog daemon --once` first
create the state folder if it is missing and set its mode to 0700, even
if it already exists. If `journal.jsonl` exists, they set it to 0600.
They do this before anything else can fail, so a run that stops at once,
such as one with no `global.toml`, still tightens what an earlier
version left readable.

The store sets its own files to 0600 when it opens, and the daemon sets
`catalog.md` to 0600 whenever it publishes. A file left readable by an
earlier version is tightened the next time the daemon starts or
publishes. The daemon can't take back a file that another account
already has open.

## In the state folder

### store.db

The store is a SQLite database. The daemon creates it with mode 0600 if
it is missing, and sets that mode on `store.db`, `store.db-wal`, and
`store.db-shm` each time it opens the store, so a write-ahead log left
by a daemon that was killed becomes private too. SQLite creates the
`-wal` and `-shm` files while the store is open; the store's tests check
that they are 0600.

The store has three tables:

| Table       | What each row holds                                                                                                       |
| ----------- | ------------------------------------------------------------------------------------------------------------------------- |
| `files`     | a catalog's folder, a file's path in it, its size, modification time, content hash, and the time the daemon saw it change |
| `summaries` | a content hash, the recipe that made the summary, and the summary text                                                    |
| `failures`  | a catalog's folder, a file's path, the number of failures, the last error, and the time of the next retry                 |

The summaries are written about private documents, and the paths and
error text name them.

### journal.jsonl

The journal is a text file with one JSON object per line. The daemon
only appends to it and never truncates it. It creates the file with mode
0600 if it is missing. Each line has a `time` and a `kind`, and the
fields that apply to that kind:

| Field                           | Holds                                                                         |
| ------------------------------- | ----------------------------------------------------------------------------- |
| `time`                          | when the event happened                                                       |
| `kind`                          | `summary`, `failure`, `pause`, `resume`, `config_error`, or `publish_error`   |
| `dir`                           | the catalog's folder, or the watched folder or config file the event is about |
| `rel`                           | the file's path inside the catalog                                            |
| `recipe`                        | the recipe that made a summary                                                |
| `error`                         | the error text                                                                |
| `input_tokens`, `output_tokens` | the tokens the model service reported for a summary                           |
| `millis`                        | how long a summary took, in milliseconds                                      |

A line omits the fields that don't apply. The journal names every file
the daemon summarized and every error it met.

### control.sock

The status socket is a Unix socket. `catalog daemon` creates it when it
starts answering `catalog status`, and removes it when it stops. It is
created in two steps: `net.Listen` creates the socket, with the mode the
process's umask allows, and then the daemon sets its mode to 0600. The
folder around it is 0700 throughout.

If a socket is already at the path and nothing answers on it, the daemon
removes it first and listens anew. If a daemon answers there, or if
something that isn't a socket is at the path, the daemon doesn't listen
and writes `status won't answer: <error>` to standard error; it keeps
working. [Commands](commands.md#catalog-daemon) describes the second
daemon's refusal to start.

Each connection gets the daemon's latest status as one JSON object and
is closed. The status holds the start and last crawl times, the config
errors, the pause reason, each catalog's folder with its counts of
files, and each failing file with its error. It holds no summary text.

### global.toml

The daemon only reads `global.toml`. It holds the API key. If the file
is missing, `catalog setup` creates it with mode 0600, in the state
folder that it creates as 0700 if needed; a person can also write it.
The daemon doesn't create it, check its mode, or change its mode, so for
a file a person wrote, its readers are whoever that person allowed; the
0700 state folder keeps other accounts out of it. `catalog setup` never
changes a `global.toml` that exists.
[Configuration](configuration.md#globaltoml) lists its keys.

#### The API key in errors

Errors in the journal, the log, and the status name file paths and quote
the error text they come with. The daemon never formats the API key into
an error itself.

When `global.toml` doesn't parse, the error gives only the line and the
last key the parser read, for example
`parse global.toml: line 1 (last key "api_key"): not valid TOML`, or
`a value has the wrong type` in place of `not valid TOML`. It quotes
none of the file's values. The journal records it as a `config_error`,
and `catalog status` shows it as a `Config error` line.

The text of errors that the model provider's library returns hasn't been
checked for the key.

## In each catalog

### .catalog/catalog.md

The daemon writes `catalog.md` into the catalog's `.catalog/` folder. It
never creates `.catalog/`; a person does. The file has mode 0600,
because it lists every path the catalog covers and summarizes files that
may be private. The agents that read it run as the same account.
[Catalog format](catalog-format.md) describes its contents.

To publish, the daemon first checks the current file. If it is a regular
file with mode 0600 and already holds the new text, it writes nothing.
Otherwise it removes any `catalog-*.tmp` files in `.catalog/` that an
earlier write left behind, writes the new text to a new temporary file
in `.catalog/`, sets its mode to 0600, and renames it over `catalog.md`.
A reader never sees a partial file. If `catalog.md` is a symbolic link
or has another mode, the rename replaces the link itself, not what it
points to. If the daemon can't write the file, it keeps the old one and
journals a `publish_error`.

The daemon doesn't set the mode of `.catalog/` or of the catalog's
folder.

### .catalog/config.toml

A person writes this optional file; the daemon only reads it, on each
crawl. [Configuration](configuration.md#catalogconfigtoml) lists its
keys.

## The launch agent's property list

`catalog setup` writes
`~/Library/LaunchAgents/com.westarete.catalog.plist` with mode 0600,
from a template built into the program. It names the program that ran
setup, with symbolic links resolved, followed by `daemon`, and has these
keys:

| Key                                    | Value                                           |
| -------------------------------------- | ----------------------------------------------- |
| `Label`                                | `com.westarete.catalog`                         |
| `ProgramArguments`                     | the program's path, then `daemon`               |
| `RunAtLoad`, `KeepAlive`               | true                                            |
| `Umask`                                | 63, which is octal 077                          |
| `StandardOutPath`, `StandardErrorPath` | `~/Library/Logs/catalog.log`, with `~` expanded |

Setup writes the file through a temporary file in the same folder and
renames it into place. It holds the program's path and the log's path,
which name the home folder, and no key or other setting.
[Commands](commands.md#catalog-setup) describes setup.

## The log

The property list sends the daemon's standard output and standard error
to `~/Library/Logs/catalog.log`. The daemon writes only to standard
error: one line for each error it can't journal or act on, with a
timestamp. launchd creates the log, not the daemon, and the daemon
doesn't set its mode. The property list sets `Umask` to 63, which is
octal 077, so launchd creates the log with mode 0600; a comment in the
template cites `launchd.plist(5)` for this. The umask also applies to
any file the daemon creates without choosing a mode. `catalog setup`
creates the log's folder, closed to other accounts, if it is missing. It
does the same for `~/Library/LaunchAgents`, with mode 0700.

## Files the daemon only reads

- `global.toml` in the state folder.
- `.catalog/config.toml` in each catalog.
- Each file a catalog covers. The crawl reads the folder's names, sizes,
  and modification times. To summarize a file, the daemon reads it to
  compute a hash of the whole file and keeps up to 400,001 bytes of a
  text file, or up to 95,000,000 bytes of a PDF, to send. To write a
  listing, it reads the first 4,096 bytes of a file that isn't a PDF.
- The existing `catalog.md`, to compare it with the new text.

The daemon never writes to a file it summarizes.

## What leaves the machine

For each file it summarizes, the daemon sends one request to the
Anthropic Messages API, with the API key from `global.toml`. The request
holds:

- the instruction text the daemon ships with
- the file's path relative to its catalog, and its text cut at 400,000
  bytes, or the whole PDF
- the path and summary of every other file in the catalog that has one

The provider's reply becomes the file's summary. The daemon sends
nothing else to any service: not the journal, the store, a catalog's
config, or the status. Each agent that loads `catalog.md` sends its
paths and summaries on to that agent's own model provider.
[Profiles](../explanation/profiles.md#what-a-provider-keeps) covers what
a provider keeps and how a person chooses one.
