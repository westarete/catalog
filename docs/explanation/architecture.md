# Architecture of the daemon

The catalog daemon is a Go program that runs on a Mac as a launchd
agent. Once a minute it crawls the folders a person chose, profiles the
files that changed one at a time, and writes each catalog. This page
says how we divide the daemon into components, what each one is for,
what it shouldn't know, and what each one copies from people's files and
who may read that copy. It describes what is built in the present tense
and marks the rest as direction we intend. The general rule that the
catalog never shows anyone more than they could already see is in the
[overview](overview.md#privacy-and-security); this page covers what it
means for each component. [Where catalogs go](catalogs.md) and
[Profiles](profiles.md) cover the topics that meet this one.

## How work moves

Work moves in one direction. The daemon notices that a file may have
changed, records what it saw, decides what to do, reads the file,
profiles it, renders the catalog, and publishes it.

```text
Trees ──► Watcher ──► Store ◄──► Planner ──► Reader ──► Profiler
(scope)   (changes)  (state)    (decides)   (content)   (models)

Store ──► Renderer ──► Publisher (.catalog/catalog.md)

Settings feeds every step.      Every step reports to the Journal.

Daemon connects the steps.      CLI, menu bar ──► Control ──► Daemon
```

The Daemon does this work in a single loop, one file at a time. A ticker
fires every minute. At each tick the loop rereads the settings, crawls
the watched folders, asks the Planner for the files that need a profile,
and profiles them one by one until none is waiting. It publishes a
catalog after each crawl that changed it and after each new profile. If
a tick arrives while the loop is profiling, the loop stops after the
current file and starts the next crawl. A second goroutine answers
status requests from a snapshot the loop hands it, so answering never
touches the Store.

We chose one loop over two other shapes, in
[the first daemon's spec](../superpowers/specs/2026-11-15-daemon-mvp-design.md#approaches).

- Only one goroutine changes state, so we can follow a failure easily
  and test the loop with a fake clock and model. The cost is that one
  model call runs at a time, so a large batch of new files waits its
  turn. People notice that wait on the first day, while the daemon
  profiles each repository from scratch, and rarely after that.
- A pool of workers catches up on a batch several times faster, which is
  how Dropbox's engineers structure their sync engine
  ([research/dropbox.md](../research/dropbox.md)). It's direction, not
  built. Cancellation, rate limits, and retries would then span several
  workers, and two files profiled at the same moment couldn't use each
  other's profiles as neighbors. If one loop proves too slow, we can
  switch by changing only the Daemon, because the Planner already
  returns an ordered list and the Reader and Profiler already handle one
  file at a time.
- Having launchd start the work once a minute, with no long-running
  process, would leave nothing running to answer `catalog status`, and
  status would have to read the Store directly, which the rules below
  forbid.

## Components

A profile is a copy of what its file says, so any component that keeps
or sends one has a second job besides its main one: to say what it keeps
or sends, and who may read it. The answer is the same on every platform.
A profile may be read by the people who can read its file, and by no one
else ([overview](overview.md#privacy-and-security)). Paths count too,
since a path can say nearly as much as the file. A component that makes
a new kind of copy states the same things for it.

In the version that is built, one person runs the daemon, and everything
it keeps is readable only by that person's account. Their agents run as
that person too, so nothing needs to be readable by another account.
Serving a team from one machine is direction, not built, and each
component says below what it would need.

Every component is a package under `internal/`, which is Go's convention
for packages that only this module can import (`go help importpath`).

### Model

The Model is the vocabulary the other components share. It holds values
and does no reading, writing, or network work. It's the `internal/model`
package.

- A catalog is a folder that holds a `.catalog/config.toml` file, and
  its tree is everything under that folder except subfolders with a
  `.catalog` folder of their own, catalogs or not. The daemon maintains
  one `catalog.md` for each.
  [Where catalogs go](catalogs.md#where-agents-start) says how people
  choose them.
- An entry describes a file. Source trees hold thousands of files, so we
  may later need entries that describe folders, and the vocabulary
  should allow either.
- A content hash identifies what a file contained when the daemon read
  it. It's a SHA-256 digest.
- A profile has a depth: a listing or a summary. A third depth, a
  position among its peers, is direction, not built
  ([Profiles](profiles.md#profiles-deepen-over-time) describes it).
- A recipe records the reader, prompt, and model that produced a
  profile, so the daemon can update profiles when any of them changes.
  It also marks a profile written while another file in its catalog was
  still waiting for its first one
  ([Profiles](profiles.md#the-second-pass)). GNOME's indexer records its
  extractor the same way
  ([research/localsearch.md](../research/localsearch.md)).

### Settings

Settings answers one question: what applies to this catalog right now?
It reads three files.

- `keys.toml`, in the state folder: the person's API keys, each under a
  name.
- `global.toml`, in the state folder: the person's settings for this
  machine. These are the folders to watch, the model, the name of the
  key to use, and which files every catalog should include and exclude.
- The catalog's own `.catalog/config.toml`, which also marks the folder
  as a catalog: which more files that catalog should include and
  exclude.

A catalog's `include` and `exclude` lists add to those in `global.toml`,
which add to the lists built into the daemon. A file is in the catalog
if it matches an `include` pattern and no `exclude` pattern from any of
them. The daemon rereads every settings file on every crawl, so a change
takes effect within a minute without a restart. If a file doesn't load,
including when it holds a key the daemon doesn't read, the daemon keeps
using the last settings that did and reports the error through status.
[Configuration](../reference/configuration.md) lists the keys and rules.

A tool that runs inside one repository knows where to look for its
configuration. A daemon serving a whole machine would have to search the
machine for it, and a knowledge worker would add folders through a menu
bar app, not by committing a file. So the person keeps one list of
folders to watch on the machine. Inside those folders, the daemon
already crawls every file, so a `.catalog/config.toml` file there is
enough to mark where a catalog goes. We expect people to change the
shape of these settings once they use them, so we keep every component
except Settings independent of it. The built version has no limits on
battery or spending, and no more than one model provider; both are
direction.

The API keys live in a file of their own so that the other two files can
be shared. A catalog's `config.toml` travels with its folder: people
share it, copy it, and commit it. With the keys in the state folder, no
catalog folder or watched folder ever holds one, even in a file Git
ignores, and the daemon reads a key from nowhere else. Keys have names
so that a shared config could later choose one by name while each person
keeps their own key under that name; that choice is direction, not
built. The daemon doesn't read the key from the environment, because
launchd doesn't start it from the person's shell. Keeping keys in the
Keychain is direction, for when there's an app.

The lists add up, rather than each replacing the one before, so that a
catalog can leave out one more file, or cover one more type of file,
with one line. Under the earlier rule, a catalog's list replaced the
global one, so a catalog that set its own list lost the global entries
unless it repeated them. In
[issue #17](https://github.com/westarete/catalog/issues/17), because a
repository's `config.toml` had its own `ignore` list, the only way to
leave out one file was to change a tracked file everyone shares.
Exclusion wins over inclusion at every level, with no patterns that undo
an exclusion, so the order of patterns never matters.

A catalog's `config.toml` may come from someone else, for example with a
clone, so the daemon trusts it less than the person's own files. Because
exclusion wins, nothing in it can bring back a file the person excluded
in `global.toml`. It can still widen the lists, because the folder's
config knows its contents, and so decide which more of that folder's
files the daemon sends to the model service. The daemon reads only its
`include` and `exclude` lists and rejects any other key, so it can never
name a model or a server to send files to, or hold a credential.

### Trees

Trees takes the folders the person chose to watch and finds the catalogs
inside them: each folder whose `.catalog/` folder holds a `config.toml`
file. It decides which catalog a path belongs to and whether the path is
a file to index, applying the nesting rule and the include and exclude
patterns. It never descends into `.git` or `.catalog/`, so it never
profiles Git's records or the catalog itself. It knows nothing about
profiles. The daemon creates no `.catalog/` folder; `catalog init` or a
person does, and what a catalog needs is in
[Where catalogs go](catalogs.md#what-a-tree-needs).

### Watcher

The Watcher reports which files have changed. Today it does this by
comparing each file's size and modification time with what the Store
last recorded, and by rereading and hashing only the files that differ.
It keeps nothing: it reads each changed file once to hash it and hands
the hash and the times to the Store. It doesn't know what a catalog or a
profile is, and it doesn't decide when to act on a change.

The built version doesn't use FSEvents. A crawl every minute leaves no
history to save, and after a restart the next crawl rebuilds the picture
from the Store. The Planner holds a file back until it has stayed
unchanged for three minutes, so a crawl that sees a file in the middle
of saving doesn't send it to a model.

Using each operating system's own change notices is direction, not
built. On macOS, the Watcher would listen to FSEvents, treat the events
as hints, and crawl when macOS says it dropped events, as Apple advises
([research/spotlight.md](../research/spotlight.md)). It would add the
FSEvents behind the interface the crawl already uses. On Linux it would
use inotify. It may have to poll a folder that a Mac shares into a
container, because file events don't always reach the container. A
Rancher Desktop issue titled "MacOS file system events don't pass
through to containers" has been open since 2022
([issue 1979](https://github.com/rancher-sandbox/rancher-desktop/issues/1979)),
and in a 2025 test on Docker Desktop, a container saw a file's new lines
only after someone ran `sync` or `ls`
([Docker forum](https://forums.docker.com/t/long-lived-tail-f-on-volume-mounts/148436)).

### Store

The Store keeps the daemon's state outside every tree; on a Mac, under
`~/Library/Application Support`. Nobody commits the daemon's state, so
we can store it however suits the daemon, and it doesn't need to work
well with Git. The daemon has only two obligations: to keep the
`.catalog/catalog.md` convention it shares with agents, and to keep its
state out of the folders it serves. Because a pile of files there
wouldn't clutter anyone's repository, we could have stored each profile
in a file of its own, much as Git stores its own data. We chose one
SQLite file, through a driver written in Go, so the daemon needs no C
library.

The database holds three kinds of records, which cost different amounts
to rebuild.

- What the last crawl saw of each file: its size, modification time,
  content hash, and when it last changed. A crawl can rebuild this.
- Profiles, keyed by content hash and recipe. Each one cost a model
  call, so the Store keeps them durably and never deletes them.
- Failures for each file: how many in a row, the last error, and when to
  try again.

The Store holds a profile and a path for every file, and the last error
for a file that failed, which may quote text from the file or from the
service. So no one may read an entry for a file they couldn't read
themselves. The Store creates its database file, and the write-ahead log
and shared-memory files SQLite keeps beside it, readable only by their
owner, and the state folder that holds them is closed to other accounts.
On a server that serves a team, the Store would give each person only
the entries for files that person can read; a content hash alone doesn't
show that someone can. That is direction, not built.

The Store keys profiles by content, so a moved file keeps its profile. A
Git worktree, which is a second copy of mostly identical files, also
costs almost nothing to catalog. No other component knows how the Store
keeps its data. One consequence is that a profile may point to a
near-duplicate with "see X.md," and the model took that path from the
catalog where it wrote the profile. If the same content sits under a
different prefix, such as a subtree copied into another repository, the
pointer can be wrong. We accept that for now, and we'll revisit it if
copies under other prefixes turn out to be common.

`catalog daemon --once` opens the Store directly, even while the daemon
runs, so the Store uses SQLite's write-ahead log and a busy timeout, and
the loop rereads profiles from the Store each time it plans instead of
keeping its own copy. We treat `--once` as a maintenance tool, not a
client; `catalog status` still asks the daemon through the socket.

A central server could hold profiles that other machines fetch by
content hash. That is an open question below.

### Planner

The Planner compares what the daemon has seen with what it has profiled
and returns the work to do, in order. It takes the state and the time as
inputs and touches nothing else, so its tests need no files, network, or
real clock. Dropbox's engineers store observations and derive the work
from them for the same reason
([research/dropbox.md](../research/dropbox.md)).

A file needs a profile when all of these hold.

- The Store has no profile for its content hash under the current
  recipe.
- The file hasn't changed for three minutes, so the daemon doesn't
  profile a file on every auto-save.
- The file isn't waiting to retry after a failure.
- If its profile is a partial one, no catalog with a copy of it has a
  file waiting for its first profile
  ([Profiles](profiles.md#the-second-pass) explains why).

Files with no profile come first. A file that has a profile only under
an older recipe goes after every file that has none, so after a prompt
change the daemon gradually updates the catalogs without holding up new
files. Within each group, the file that changed earliest comes first.
When a file fails, the Planner sets the wait before the next try: a
minute for the first failure, doubling each time up to six hours, and
every six hours after that.

We put most of the questions we once left open about scheduling in the
Planner, so we can change the answers without touching the other
components: which files come first, how long a file must sit unchanged,
and when to give up on a file that keeps failing. How hard to work on
battery is not built. Whether the daemon needs to know which files are
in use, or which folders have active processes, is an open question
below; the three-minute wait is the only answer so far.

### Reader

The Reader turns a file into something a model can take, along with
cheap facts for a listing. It's a pure function in the daemon's own
process; no code in the daemon starts another process. Today it handles
two kinds of file: text, which it reads as text and cuts at 400,000
bytes, and PDFs, which it sends to the model whole. A PDF over the size
the daemon sends stays listed without a profile, and the journal says
so. For a listing, the Reader takes the path, the type, the size, and,
for a text file, the first ten lines, cut at 512 bytes. A PDF has no
first lines, since that would take a PDF text extractor.

Each kind of file getting its own reader, the way Spotlight has one
importer per file type, is direction, not built. Prose, data files such
as CSV, source code, and images would all go through the same interface,
and if no reader understood a file, the file would still get a listing.

Reading is the step most likely to crash on a bad file, which is why
Spotlight and GNOME's indexer run it in a separate process
([research/spotlight.md](../research/spotlight.md),
[research/localsearch.md](../research/localsearch.md)). The Reader takes
a file, returns content, and depends on nothing else, so we can move it
into its own process later without changing the components around it. A
reader or model that needs an Apple framework would run as a helper
process the same way. Neither is built.

### Profiler

The Profiler turns what the Reader produced, plus the profiles of the
other files in the same catalog, into a profile. It owns the prompt. A
listing needs no model at all.

Because the Profiler decides what a model receives, it sends one file,
its path, and the other files' profiles, and nothing more than that. A
profile can repeat a secret from its file, such as a password or an
account number. How the Profiler would filter secrets out before the
model sees them is still to be designed.

The Profiler would also return notes on how the profiling went: whether
it cut the file short, how much of the file the model needed, and
whether the model could tell what the file was for. The notes would go
to the Journal and describe the run without quoting the file. That's
direction, not built.

### Models

Models is a small interface with a provider behind it. A provider
accepts content and returns text, and it sorts each failure into one of
two kinds: a problem with the file, which will keep failing, or a
problem with the service, such as a network error, a rate limit, or a
rejected API key, which may clear. The Planner backs off per file for
the first kind. For the second, the loop pauses all profiling with the
same doubling wait, keeps crawling, and keeps publishing listings.

The built version has one provider, Anthropic, and the model it calls is
the one named in the global settings. Providers for other hosted
services and for models on the same machine are direction, not built.
Each would declare what it can take (text, images, PDFs), how much it
can take at once, and whether the content leaves the machine. The
Profiler would work the same way with any of them. Whether to use a
model that ships with the operating system, such as Apple Intelligence,
is an open question below.

Every provider also has a policy for what it keeps of the content it
receives and whether it trains on it, and a person can read that before
choosing it. That holds for a hosted service, and it holds for a model
on the same machine, which sends nothing away but may still keep
prompts, caches, or logs there. The person decides what's acceptable for
the content they send it; [Profiles](profiles.md#what-a-provider-keeps)
describes that choice and quotes what the provider we use says. The
daemon sends each file and its neighbors' profiles to that provider, and
nowhere else. The provider's credential stays in the person's
`keys.toml`. The daemon never formats it into the journal, the log, or
status itself, though
[error text from other sources can still carry part of it](../reference/files.md#what-errors-leave-out).

### Renderer

The Renderer turns a catalog's profiles into the text of a catalog. It
decides the catalog's shape: a `## <dir>/` heading for each folder and a
`### <path>` entry for each file, how each entry shows its depth (an
entry with only a listing starts with "Not summarized yet."), and what
the header says. It touches no file and uses no network, so anything
that delivers a catalog can use it.

### Publisher

The Publisher writes what the Renderer produced to a catalog's
`.catalog/catalog.md`, through a temporary file in `.catalog/` that it
renames into place, so an agent never reads a half-written file. It
skips the write if the text hasn't changed, and if writing fails it
leaves the old file as it was. It's the only component that writes
inside a person's tree, so we can check the promise that the daemon
writes nothing else there, and the promise about who may read the file
it writes, in one place.

The catalog file is one file with one set of readers, so everyone who
can open it has to be able to read every file it describes. No one may
read an entry for a file they couldn't read themselves. The Publisher
makes the file readable only by its owner, but it can't see how a folder
is shared.
[Where catalogs go](catalogs.md#subfolders-shared-more-narrowly-than-their-parent)
says what follows when a subfolder is shared more narrowly than its
parent, and where a catalog that is committed or synced may end up. The
daemon doesn't yet recognize a synced folder, so it relies on people not
to watch one ([cloud drives](catalogs.md#cloud-drives)).

### Journal

The Journal is a record, kept on the machine, of what the daemon did.
It's an append-only file with one JSON line for each event: each profile
with its recipe, how long it took, and how many tokens it used; each
failure; each pause and resume of profiling; each settings error; and
each catalog that couldn't be written. It never rotates the file, and
nothing in the daemon reads it back yet.

The record names every file the daemon touched and quotes errors that
may name more, so no one may read an entry about a file they couldn't
read themselves. The journal is readable only by its owner. On a server
that serves a team, the Journal would give each person only the entries
about files that person can read, so a person could read all of it only
if they could read all of those files. That is direction, not built. The
daemon never formats a credential into the Journal itself
([what errors leave out](../reference/files.md#what-errors-leave-out)),
and a report from a client would be held to the same reader rule.

#### Statistics for tuning

People will want to understand how the catalog is working and tune it.
We don't plan to build much on the Journal at first. We do want every
component to report through it from the start, so the record exists when
we want it. We see three uses, none of them built.

- A menu bar item could show what the daemon is doing and how far behind
  it is. `catalog status` shows that today from the daemon's snapshot,
  not from the Journal. Cursor's users objected when Cursor took that
  view away ([research/cursor.md](../research/cursor.md)).
- A person could compare models and prompts by time, cost, and failures.
- A Claude skill could read the statistics and recommend changes to the
  content itself. If the Profiler's notes show that many files only
  become clear after thousands of lines, the skill could suggest a
  five-line comment at the top of each one.

Under the file contract
([overview](overview.md#the-contract-with-agents)), agents never talk to
the daemon, so the Journal can't see which files agents go on to open.
It should still accept reports from outside the daemon, so that a client
such as an MCP server can pass along what an agent tells it. That is
direction too.

### Control

Control defines the requests a client can make of the daemon and the
answers it gets. Today it answers one request, status, over a Unix
socket in the state folder, which only the person's account can open. We
use a Unix socket instead of XPC, which exists only on Apple's systems,
so a Linux daemon could use the same core.

The answer to a status request is a snapshot: the daemon's version, when
the daemon started and last crawled, any settings error, whether
profiling is paused and why, and for each catalog its folder and how
many of its files are profiled, waiting, or to update, along with each
failing file, its last error, and when the daemon will retry it. That
names folders and files and quotes errors, so it's a copy like the
others. Control answers only the account that runs the daemon, and the
socket's mode is how it checks. If the socket doesn't exist,
`catalog status` reports that the daemon isn't running. Control imports
nothing else in the module, because the daemon and the command both use
it.

More requests are direction, not built: prioritize this catalog
(`catalog update` would become a simple way for a person to ask the
daemon to give the current tree priority; agents don't need it), retry a
file, pause, read or change settings, read statistics, fetch the catalog
for a path, and record a report in the Journal. Several of them return
profiles, paths, or the Journal. Control would answer each requester
with what that person could already read, and nobody on the machine or
the network could ask it for more. If a server served a team, a request
from another client would carry the same rule. How Control would tell
who is asking is still to be designed.

### Daemon

The Daemon starts the other components, connects them, and shuts them
down. It holds no logic of its own beyond running the loop described
above. It makes changes to state one at a time, as Dropbox's engineers
did with a single control thread. It stops on an interrupt or a
termination signal, and it refuses to start if another daemon already
answers on the socket, since a second one would crawl and profile
everything the first does.

### Clients

A client talks to the Daemon through Control, and none of them reads the
Store or the Journal directly. To add an interface, we write a client
instead of changing the daemon. Today there is one, `catalog status`,
which asks the daemon and prints the snapshot. The others are direction:
a menu bar item, a settings window, an MCP server
([Where catalogs go](catalogs.md#delivering-the-catalog-without-a-file)
describes that one), and a skill that reads statistics.

What a client shows or passes along is a copy as well, so each client
states what it keeps and who may read it. `catalog status` keeps
nothing; it prints to the terminal of the person who ran it. An MCP
server that delivers catalogs to other people would meet the subfolder
problem described in
[Where catalogs go](catalogs.md#subfolders-shared-more-narrowly-than-their-parent).

## Rules between components

- Every component may depend on the Model.
- These components don't depend on each other: Trees, Watcher, Store,
  Planner, Reader, Settings, Models, and Journal. The Profiler may use
  the Reader and Models, the Renderer may use the Reader for a listing,
  and the Publisher may use the Renderer. Control uses nothing. Only the
  Daemon knows all of them.
- Clients depend on Control and nothing in the daemon. The `catalog`
  command is one binary that holds both the daemon and its client, so it
  imports the Daemon, Settings, the Store, Models, and Control to wire
  the daemon up, and `catalog status` uses only Control.
- Tests can replace the file system, the clock, and the model, because
  each one sits behind an interface. Replacing the machine's power state
  would need the battery handling, which isn't built.
- A linter holds each component to its declared dependencies. Every
  component is a package in one Go module, where any package could
  otherwise import any other, so `depguard` in `.golangci.yml` fails the
  build on a dependency we didn't declare. Apple's `container` tool
  splits its Swift package into many targets, and its CLI doesn't depend
  directly on the server's target
  ([research/container.md](../research/container.md)).
- Nothing in the shared core, described next, uses a facility that
  exists on only one operating system. The built core uses none.

## Platforms and language

The daemon runs on a Mac, as a launchd agent. The Mac comes first,
because everyone who uses the catalog today is on one. We intend it to
run on a headless Linux server that keeps a catalog current as a central
source, and in a Linux container on a Mac or Windows laptop. We also
want to keep Windows open as a native platform, because knowledge
workers on Windows, such as those using Copilot, could become our
largest audience. We hope the shared core carries over to Windows. If it
can't, we'd write a separate implementation that honors the same
contract with agents. Neither Linux nor Windows is built: the launchd
agent is the only service manager the daemon supports, and the daemon
has no Linux-specific code.

We believe we should use each operating system's strengths wherever we
can, and that a shared core with a thin layer for each operating system
is better than a separate implementation for each. The core holds the
prompts, the depths, the scheduling, and the catalog's format, and those
determine how good a catalog is. Two separate implementations would
drift apart there first. We kept the contract with agents very simple,
so that separate implementations would be possible, but holding both to
the contract wouldn't prevent that drift, because the contract covers
the file's format, not its quality.

Following the rules above, we already put the parts tied to an operating
system behind interfaces for testing, so splitting them out costs
little.

| Shared core                                                         | Tied to the operating system                                      |
| ------------------------------------------------------------------- | ----------------------------------------------------------------- |
| Model, Settings, Trees, Store, Planner, Profiler, Renderer, Journal | Watcher: FSEvents on macOS, inotify on Linux                      |
| Control's messages and the socket they travel over                  | Installing and running the service: launchd, systemd, a container |
| Publisher                                                           | Permissions and packaging                                         |
| Providers for hosted models, and for local models served over HTTP  | Providers that run a model through an Apple framework             |
| Readers that need no system framework                               | Readers that use Apple frameworks, such as PDFKit                 |
| CLI and MCP server                                                  | Menu bar item and settings window                                 |

Of the table, what's built is the shared core's Model, Settings, Trees,
Store, Planner, Profiler, Renderer, Journal, Publisher, one hosted
provider, a text and PDF reader, Control's socket, and the CLI, plus
launchd. The Watcher built is a crawl, which has no operating-system
part. The rest of the table is direction.

The shared core and the CLI are in Go, on the version `go.mod` names (Go
1.26.0), with SQLite, the Anthropic SDK, and a TOML parser as
dependencies. The standard library covers arguments (`flag`), logging
(`log`, to stderr, which launchd sends to a log file), and the signal
handling that shuts the daemon down. This follows the shape of
Tailscale's client, whose Mac app is the same Go code as everywhere else
plus a small GUI wrapper
([research/tailscale.md](../research/tailscale.md)).

On the Mac, we'd use Swift for the parts of the table's right-hand
column that Go can't reach well: the menu bar item and settings window,
the app bundle that macOS's permissions may require (see "Processes and
macOS" below), and readers and model providers built on Apple's
frameworks. None of that is built. We believe the Watcher and the
launchd agent need no Swift, since FSEvents is a C interface and launchd
needs only a property list, but we haven't built FSEvents to check. The
launchd agent needs no Swift today, and the repository holds its
property list.

We considered two other options. We could write everything in Swift,
since Swift builds for Linux too, with fully static binaries
([Static Linux SDK](https://swift.org/documentation/articles/static-linux-getting-started.html)).
But one person reported in 2024 that importing Foundation failed to
build with that SDK, and the thread ends without a confirmed fix
([Swift forums](https://forums.swift.org/t/using-swift-foundation-with-linux-static-sdk/74642)).
We didn't look into the other option, Rust for the core.

We'll decide during implementation how the Swift parts talk to the Go
core. A menu bar item is a client, so it goes through Control like any
other. A reader or model provider that needs an Apple framework would
run as a helper process, which the Reader's design already allows.

## Processes and macOS

On a Mac, the daemon runs as a launchd agent for the logged-in user. The
property list that names the program comes from `catalog setup`, which
the Homebrew cask runs when it installs and a person who builds from
source runs themself. launchd starts the daemon when the person logs in
and starts it again if it exits. launchd sends the daemon's log to a
file, and the property list sets the umask so that any file the daemon
creates without choosing a mode is readable only by its owner. launchd
creates the log itself with mode 0644, ignoring that umask, so the
daemon sets the log's mode to 0600 when it starts. The log names files
and quotes errors, like the journal, so it follows the same reader rule.
Apple's `container` tool has the same shape, a CLI and a launch agent,
though it connects them with XPC
([research/container.md](../research/container.md)).

macOS permissions may affect how we package the daemon. Howard Oakley
lists Desktop, Documents, Downloads, iCloud Drive, and third-party cloud
storage among the locations macOS protects from apps reading them, and
says he couldn't find an official list
([Oakley, 2026](https://eclecticlight.co/2026/04/07/privacy-protected-folders/)).
Knowledge workers keep files in those folders.

Quinn, an engineer in Apple's developer support, says macOS looks for
the "responsible code" behind a process when it decides access, and that
the exact algorithm isn't documented. If an agent was installed by
`SMAppService`, the app is the responsible code; otherwise the agent
"should include AssociatedBundleIdentifiers in its launchd property
list"
([Apple Developer Forums](https://developer.apple.com/forums/thread/756510)).
Apple's documentation says the property list for an `SMAppService` agent
must be in the calling app's `Contents/Library/LaunchAgents` directory
([SMAppService](<https://developer.apple.com/documentation/servicemanagement/smappservice/agent(plistname:)>)).

Either way, the agent is tied to an app. We infer that the daemon will
need an app bundle before it can read those folders without trouble,
even while the CLI is its only interface. We haven't tested what a bare
launch agent can read, and the first daemon's spec plans to watch only
folders such as `~/src` that Oakley doesn't list
([the spec](../superpowers/specs/2026-11-15-daemon-mvp-design.md#constraints)).
Because of these rules, we may have to ship the daemon inside an app
bundle sooner than the order of interfaces below suggests.

## User interface

Each platform can have its own user interface, and each is a client of
the same daemon, so adding one shouldn't change how the daemon works.
The first macOS version uses only the command line and config files.
Later we'd likely add `plist` entries, then a menu bar item, and then a
settings window. We could distribute it through Homebrew first and the
Mac App Store later. None of those later steps is built. On Windows and
Linux, we'd build user interfaces that suit each environment.

Wherever we show status, a person should be able to see at a glance
which folders are cataloged and which aren't. To its owner, a folder
that never gets indexed looks the same as one that's merely behind.
Today `catalog status` lists each catalog with how many of its files are
profiled, waiting, and to update, and the files that are failing, so a
person can tell a catalog that is filling in from one that is stuck. It
doesn't list a watched folder with no catalog, which the daemon skips
silently, so that case is not built. Much later, the menu bar item might
notice someone working in a folder with no catalog and offer to add one.

## Open questions

- If the daemon stops for longer than a restart can cover, such as when
  the person quits it or it can't start, the catalog falls behind again,
  and agents shouldn't have to notice. launchd restarts the daemon
  today. The menu bar item could show the daemon's state, but we haven't
  worked out the details.
- Does the daemon need to know which files are in use, or which folders
  have active processes? The three-minute wait before it profiles a
  changed file is the only answer so far.
- What frameworks does macOS offer today for watching file changes and
  for running language models on the device? The daemon uses neither
  FSEvents nor an on-device model, so we haven't looked into them. The
  same answer settles whether an on-device model would run in a helper
  process.
- We don't know how much the Linux cases matter.
- A central server could hold profiles that other machines fetch by
  content hash, so that a fresh clone or a cloud agent wouldn't start
  empty. Cursor shortened its first index that way
  ([research/cursor.md](../research/cursor.md)). Profiles would then
  leave the machine, which raises the privacy questions Cursor faced and
  the ones in the [overview](overview.md#privacy-and-security): who may
  read a profile on the server, and how the server knows a requester may
  read the file behind it.
