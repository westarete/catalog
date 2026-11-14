# Architecture of the catalog daemon: draft outline

This outline names the components of the catalog daemon, says what each
one is for, and says what each one is kept from knowing. Nothing here is
built, and we expect the outline to change as we work through it.
[DAEMON.md](DAEMON.md) explains the problem and what we want; this file
is about how the application is divided. It runs on macOS first, and we
want most of it to run unchanged on a headless Linux server or in a
Linux container. We haven't chosen a language.

## The contract with agents

An agent's only contact with the catalog is a `.catalog.md` file that
its instructions already load. An agent never talks to the daemon, never
runs a command, and doesn't need to know the daemon exists. Four things
follow from that.

- A missing catalog does no harm. Most people use no catalog today, and
  their agents explore a folder with the search tools they have. Where
  no daemon runs, such as a cloud agent on a fresh clone or a colleague
  who hasn't installed it, the agent works as it would have anyway. The
  same goes for a catalog that has fallen behind.
- The file has to stand on its own. It says how deeply each entry has
  been profiled, because the agent has nowhere else to ask.
- The catalog works anywhere the folder is visible, including through
  Claude Cowork's mount, with no skill or setup on the agent's side.
- Status, settings, and statistics are for people, and for tools that
  help people tune the catalog. None of them are part of what an agent
  sees. Noticing that the daemon has stopped is a job for launchd and
  the menu bar item, not for an agent.

The experience we want is that a person installs the catalog, points it
at the folders they work in, and their agents get better at finding
things without anyone telling the agents why.

The general rule is that the catalog reaches an agent through the same
channel the agent uses to reach the content. For files in a folder, that
channel is a file in the folder. "Delivering the catalog over MCP" below
covers content that has no folder.

## What a tree needs

Three things have to be in place before a tree's catalog reaches an
agent. The daemon creates none of them, since its Publisher writes only
`.catalog.md`.

- A `.catalog.toml` at the top of the tree, which tells the daemon to
  index it.
- A line in the tree's `CLAUDE.md` or `AGENTS.md` that loads
  `.catalog.md`, such as `@.catalog.md`.
- A rule that keeps Git from tracking `.catalog.md`. A person could set
  it once for every repository on their Mac in Git's global ignore file,
  and a repository's own `.gitignore` takes precedence over that file
  when a team wants something different
  ([gitignore documentation](https://git-scm.com/docs/gitignore)).

Putting these in place is a job for a client, such as a setup command or
a button in the menu bar item. We haven't designed it.

## How work moves

Work moves in one direction. The daemon notices that a file may have
changed, records what it saw, decides what to do, reads the file,
profiles it, renders the catalog, and publishes it.

```text
Trees ──► Watcher ──► Store ◄──► Planner ──► Reader ──► Profiler
(scope)   (hints)    (state)    (decides)   (content)   (models)

Store ──► Renderer ──► Publisher (.catalog.md)

Settings feeds every step.      Every step reports to the Journal.

Daemon connects the steps.      CLI, menu bar ──► Control ──► Daemon
```

## Components

### Model

The Model is the vocabulary the other components share. It holds values
and does no reading, writing, or network work.

- A tree is a folder with a `.catalog.toml`, and everything under it
  that no nested tree claims.
- An entry describes a file. Source trees hold thousands of files, so an
  entry may later need to describe a folder, and the vocabulary should
  allow either.
- A content hash identifies what a file contained when we read it.
- A profile has a depth: a listing, a summary, or a position among its
  peers, as described under "Profiles deepen over time" in
  [DAEMON.md](DAEMON.md).
- A recipe records the reader, prompt, and model that produced a
  profile, so we can redo profiles when any of them changes. GNOME's
  indexer records its extractor the same way
  ([research/localsearch.md](research/localsearch.md)).

### Settings

Settings answers one question: what applies to this tree right now? It
combines three sources.

- Defaults built into the application.
- The person's settings for this machine: which folders the daemon may
  look in, which model providers exist, their credentials, and limits on
  battery and spending.
- The tree's `.catalog.toml`, which is committed and shared: which files
  to include, and what the tree's owners prefer.

We propose one rule about trust. A `.catalog.toml` arrives with a clone,
written by someone else. It can narrow what gets indexed and state
preferences, such as "on-device models only" or extra guidance for the
prompt. It can't name a server to send files to. Only the person's own
settings connect a preference to a real provider and its credentials.

### Trees

Trees finds the trees inside the folders the person authorized, and
decides which tree a path belongs to and whether the path is a file we
index. It applies the nesting rule and the include and ignore patterns.
It knows nothing about profiles.

### Watcher

The Watcher reports that paths may have changed. On macOS it listens to
FSEvents, treats the events as hints, and crawls when macOS says events
were dropped, as Apple advises
([research/spotlight.md](research/spotlight.md)). On Linux it would use
inotify. It may have to poll a folder that a Mac shares into a
container, because file events don't always cross that boundary. A
Rancher Desktop issue titled "MacOS file system events don't pass
through to containers" has been open since 2022
([issue 1979](https://github.com/rancher-sandbox/rancher-desktop/issues/1979)),
and in a 2025 test on Docker Desktop a container saw a file's new lines
only after someone ran `sync` or `ls`
([Docker forum](https://forums.docker.com/t/long-lived-tail-f-on-volume-mounts/148436)).
It doesn't know what a tree or a profile is, and it doesn't decide when
to act on a change.

### Store

The Store keeps the daemon's state outside every tree; on a Mac, under
`~/Library/Application Support`. It holds two kinds of state with
different costs.

- What we last saw of each file. A crawl can rebuild this.
- Profiles, keyed by content hash and recipe. Each one cost a model
  call, so these are kept durably.

Keying by content means a moved file keeps its summary. It also means a
Git worktree, which is a second copy of mostly identical files, costs
almost nothing to catalog. No other component knows how the Store keeps
its data.

### Planner

The Planner compares what we've seen with what we've profiled and
returns the work to do, in order. It takes the state, the time, and
conditions such as battery power as inputs and touches nothing else, so
its tests need no files, network, or real clock. Dropbox's engineers
store observations and derive the work for the same reason
([research/dropbox.md](research/dropbox.md)).

The Planner is where most of the open questions in DAEMON.md get
answered: which files come first, how long a file must sit unchanged
before we profile it, how hard to work on battery, and when to give up
on a file that keeps failing. Keeping those answers in one component
lets us change them without touching the others.

### Reader

The Reader turns a file into something a model can take: text, pages, or
an image, along with cheap facts such as its type, its size, and its
first lines. Each kind of file gets its own reader, the way Spotlight
has one importer per file type. Prose, data files such as CSV, source
code, and images all go through the same interface. A file that no
reader understands still gets a listing.

Reading is the step most likely to crash on a bad file, which is why
Spotlight and GNOME's indexer run it in a separate process
([research/spotlight.md](research/spotlight.md),
[research/localsearch.md](research/localsearch.md)). The Reader takes a
file and returns content, and depends on nothing else, so we can move it
into its own process later without changing the components around it.

### Profiler

The Profiler turns what the Reader produced, plus context about
neighboring files, into a profile at a requested depth. It owns the
prompts. A listing needs no model at all.

The Profiler also returns notes on how the profiling went: whether the
file was cut short, how much of it the model needed, and whether the
model could tell what the file was for. Those notes go to the Journal.

### Models

Models is a small interface with several providers behind it: hosted
services and models running on the same machine. A provider accepts
content and returns text. It also declares what it can take (text,
images, PDFs), how much it can take at once, and whether the file leaves
the machine. The person switches providers in Settings, and the Profiler
works the same way with any of them.

### Renderer

The Renderer turns a tree's profiles into the text of a catalog. It
decides the catalog's shape: how entries are grouped, how each entry
shows its depth, and what the header says. It reads nothing and writes
nothing, so anything that delivers a catalog can use it.

### Publisher

The Publisher writes what the Renderer produced to a tree's
`.catalog.md`, replacing the file in one step. It's the only component
that writes inside a person's tree, which keeps the promise in DAEMON.md
checkable in one place.

### Journal

The Journal is a record, kept on the machine, of what the daemon did:
each profiling with its recipe, how long it took, how many tokens it
used, and the Profiler's notes; each failure; each time the daemon
caught up after falling behind. Statistics are questions asked of that
record.

We don't plan to build statistics first. We do want every component to
report through the Journal from the start, so the record exists when we
want it. We see three uses.

- `catalog status` and a menu bar item can show what the daemon is doing
  and how far behind it is. Cursor's users objected when that view was
  taken away ([research/cursor.md](research/cursor.md)).
- A person can compare models and prompts by time, cost, and failures.
- A Claude skill can read the statistics and recommend changes to the
  content itself. If the Profiler's notes show that many files only
  become clear after thousands of lines, the skill can suggest a
  five-line comment at the top of each one.

With the file contract, the Journal can't see which files agents go on
to open, because agents never talk to the daemon. The Journal should
still accept reports from outside the daemon, so that a client such as
an MCP server can pass along what an agent tells it.

### Control

Control defines the requests a client can make of the daemon and the
answers it gets: status, prioritize this tree, retry a file, pause, read
or change settings, read statistics, fetch the catalog for a path, and
record a report in the Journal.

If the core is shared with Linux, Control's messages travel over a Unix
socket on every platform, because XPC exists only on Apple's systems. If
we build for the Mac alone, XPC is still a candidate.

### Daemon

The Daemon starts the other components, connects them, and shuts them
down. It holds no logic of its own. It serializes changes to state, as
Dropbox's engineers did with a single control thread.

### Clients

The CLI, a menu bar item, a settings window, an MCP server, and a skill
that reads statistics are all clients. Each one talks to the Daemon
through Control, and none of them reads the Store or the Journal
directly. Adding an interface then means writing a client, not changing
the daemon.

Whatever shows status should make it easy to see which folders are
cataloged and which aren't, since a tree that never gets indexed looks
the same to its owner as one that's merely behind.

## Rules between components

- Every component may depend on the Model. Trees, Watcher, Store,
  Planner, Reader, Profiler, Models, Publisher, and Journal don't depend
  on each other; the Publisher depends on the Renderer. Only the Daemon
  knows all of them.
- Clients depend on Control and the Model, and on nothing in the daemon.
- The file system, the clock, the model, and the machine's power state
  sit behind interfaces that tests can replace.
- Each component is its own module with declared dependencies, so the
  compiler refuses a dependency we didn't declare. Apple's `container`
  tool splits its Swift package into many targets, and its CLI doesn't
  depend directly on the server's target
  ([research/container.md](research/container.md)).
- Nothing in the shared core, described next, uses a facility that
  exists on only one operating system.

## Platforms and language

We want the daemon on a Mac first, and also on a headless Linux server
and in a Linux container on a Mac or Windows laptop. We aren't sure yet
how much the Linux cases matter.

The parts tied to an operating system are the same parts the rules above
already put behind interfaces for testing, so the split costs little.

| Shared core                                                         | Tied to the operating system                                      |
| ------------------------------------------------------------------- | ----------------------------------------------------------------- |
| Model, Settings, Trees, Store, Planner, Profiler, Renderer, Journal | Watcher: FSEvents on macOS, inotify on Linux                      |
| Control's messages and the socket they travel over                  | Installing and running the service: launchd, systemd, a container |
| Publisher                                                           | Permissions and packaging                                         |
| Providers for hosted models, and for local models served over HTTP  | Providers that run a model through an Apple framework             |
| Readers that need no system framework                               | Readers that use Apple frameworks, such as PDFKit                 |
| CLI and MCP server                                                  | Menu bar item and settings window                                 |

We think sharing the core beats writing each platform separately. The
core holds the prompts, the depths, the scheduling, and the catalog's
format, which together decide how good a catalog is. Two implementations
would drift apart there first, and the contract with agents wouldn't
catch it, because it fixes the file's format and not its quality.

We haven't chosen a language. Three options are in view.

- Go for the daemon and CLI, with a small native app around them on the
  Mac. Tailscale's Mac client is its Go code plus a small GUI wrapper
  ([research/tailscale.md](research/tailscale.md)). We believe little in
  our daemon needs Swift, since FSEvents is a C interface and launchd
  needs only a property list, but we haven't built anything to check.
  This is our leaning if Linux turns out to matter.
- Swift throughout, keeping Apple's frameworks out of the core. Swift
  builds fully static Linux binaries
  ([Static Linux SDK](https://swift.org/documentation/articles/static-linux-getting-started.html)).
  One person reported in 2024 that importing Foundation failed to build
  with that SDK, and the thread ends without a confirmed fix
  ([Swift forums](https://forums.swift.org/t/using-swift-foundation-with-linux-static-sdk/74642));
  we haven't tried a current toolchain. This is our leaning if Linux
  stays a maybe.
- Rust for the core. We haven't looked into it.

## Processes and macOS

On a Mac, the daemon runs as a launchd agent for the logged-in user.
Apple's `container` tool has the same shape, a CLI and a launch agent,
though it connects them with XPC
([research/container.md](research/container.md)).

macOS permissions may affect how we package it. Howard Oakley lists
Desktop, Documents, Downloads, iCloud Drive, and third-party cloud
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

Both routes tie the agent to an app. Our inference is that the daemon
will need an app bundle before it can read those folders without
trouble, even while the CLI is its only interface. We haven't tested
what a bare launch agent can read.

## Delivering the catalog over MCP

An MCP server could give agents the catalog without any file: an agent
asks for the catalog for its current path and gets what `.catalog.md`
would have held.

### What the documentation says

These points come from Claude Code's
[MCP documentation](https://code.claude.com/docs/en/mcp) unless another
source is given.

- Claude Code loads a server's instructions at the start of a session,
  cut off at 4,096 characters. It defers the server's tools by default
  and finds them on demand.
- Claude Code tells a server which folder the session is in. It answers
  the server's request for roots with the session's launch directory,
  and it sets `CLAUDE_PROJECT_DIR` for a server it starts.
- A team can commit a `.mcp.json` so that everyone who opens the
  repository gets the server. Each person approves it once in an
  interactive session. A plugin can also bundle a server.
- Cowork's connectors use MCP. A server that runs as a local command
  loads "when the Cowork session runs on your computer"
  ([plugin platform support](https://claude.com/docs/plugins/platform-support)).
  The documentation we read doesn't say whether that server runs on the
  Mac itself or inside Cowork's virtual machine.
- A custom remote connector is called from Anthropic's servers, not from
  the person's machine, and has to be "reachable over the public
  internet from Anthropic's IP ranges"
  ([custom connectors](https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp)).

### What we make of it

This is our reasoning, and we haven't built anything to test it.

- A server's instructions reach the agent unasked, but 4,096 characters
  is too small for a catalog; twenty entries took about 2,400 tokens in
  [FUTURE.md](FUTURE.md). The instructions can tell the agent to fetch
  the catalog, and the agent then has to do it. The file is in front of
  the agent with no step in between.
- Setup isn't much heavier than the file's. A committed `.mcp.json` does
  for MCP what the import line does for the file, and it removes the
  need for the import line and the Git ignore rule. It works only in
  agent tools that read that file, and the agent now knows the catalog
  is a tool.
- MCP fits content that has no folder to write a file into. When an
  agent reaches a data source through MCP, the catalog for that source
  can come the same way.
- MCP gives agents a way to tell the daemon something, such as which
  file they opened for a task or that a profile misled them. That could
  answer the open question about measuring profiles, if agents report
  reliably.
- A catalog served on request can be cut to the folder the agent asks
  about, which a single flat file can't do. That may matter for large
  source trees.

We'd keep the file as the first way to deliver and treat MCP as a second
one. We hold that loosely: the comparison turns on how reliably agents
fetch the catalog when instructions tell them to, and we haven't
measured it.

The components barely change. An MCP server is a client: it asks the
Daemon, through Control, for the catalog for a path, and the Daemon
answers with the Renderer's output. Anything an agent reports goes to
the Journal the same way.

A headless Linux server depends on this. No agent reads a central
server's disk, so the server has nothing to deliver unless it serves the
catalog over the network. For Cowork, that means a server reachable from
the public internet, as quoted above. A daemon in a container on a
laptop doesn't need MCP, because it writes `.catalog.md` into the
mounted folder.

## Left for implementation

These are unknown, and the components above don't change whichever way
they turn out.

- What the Store uses: SQLite, or a file per profile.
- How the Watcher saves its place in the FSEvents history and recovers.
- Which models we ship with and which is the default.
- Whether the Reader or an on-device model runs in a helper process.
- Which libraries we use for arguments, logging, and service lifecycle.

## Open questions

- We don't know how we'll learn whether profiles lead agents to the
  right files, since agents never talk to the daemon. People could tell
  us what they see. An agent could also review past chat sessions with
  the person's permission, though reading those sessions is invasive and
  we aren't comfortable calling it the answer.
- We haven't designed how the three things under "What a tree needs" get
  put in place, or how a team overrides a global ignore rule.
- A source tree may have too many files for one flat `.catalog.md`.
  Entries for folders, or one file per folder, are possible shapes.
- We don't know how much the Linux cases matter, and the choice of
  language waits on that.
- We don't know whether an MCP server is worth building, or what an
  agent would usefully report through one.
- A central server could hold profiles that other machines fetch by
  content hash, so a fresh clone or a cloud agent wouldn't start empty.
  Cursor shortened its first index that way
  ([research/cursor.md](research/cursor.md)). It would mean profiles
  leave the machine, with the privacy questions Cursor faced.
