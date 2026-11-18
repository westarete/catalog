# Architecture of the catalog daemon: draft outline

This outline names the components of the catalog daemon, says what each
one is for, and says what each one shouldn't know. Nothing here is
built, and we expect to change the outline as we work through it.
[daemon.md](daemon.md) explains the problem and what we want; this file
is about how we divide the application. The daemon runs on macOS and on
Linux, both on a headless server and in a container, and we want to keep
Windows open. We'll write the shared core and the CLI in Go, and the
parts that need Apple's frameworks in Swift.

## The contract with agents

An agent's only contact with the catalog is a `.catalog/catalog.md` file
that its instructions already load. An agent never talks to the daemon,
never runs a command, and doesn't need to know the daemon exists. Four
things follow from that.

- A missing catalog does no harm. Most people use no catalog today, and
  their agents explore a folder with the search tools they have. Where
  no daemon runs, such as for a cloud agent on a fresh clone or a
  colleague who hasn't installed it, the agent works as it would have
  anyway. The same goes for a catalog that has fallen behind.
- The file has to stand on its own. We write into it how deeply we've
  profiled each entry, because the agent has nowhere else to ask.
- The catalog needs no skill or setup on the agent's side, so an agent
  can use it anywhere it can see the folder and load the file. That
  includes Claude Cowork's mount, though we don't yet know what would
  load the file there
  ([research/agent-instruction-files.md](../research/agent-instruction-files.md)).
- Status, settings, and statistics are for people, and for tools that
  help people tune the catalog. None of them are part of what an agent
  sees. launchd and the menu bar item, not an agent, should notice when
  the daemon has stopped.

We want a person to install the catalog, point it at the folders they
work in, and find that their agents get better at finding things without
anyone telling the agents why.

As a general rule, the catalog should reach an agent the same way the
content does. For files in a folder, that means a file in the folder.
"Delivering the catalog over MCP" below covers content that has no
folder.

## What a tree needs

Someone has to put these things in place before an agent can read a
tree's catalog. The daemon creates none of them, since its Publisher
writes only `.catalog/catalog.md`.

- The folder sits inside a tree the person chose to profile in their
  settings.
- A `.catalog/` folder marks where the catalog goes. A person or an
  agent creates it, even empty, and the daemon writes `catalog.md`
  inside it. It can also hold a `config.toml` with settings for that
  catalog.
- An instruction file the agent loads has a line that imports
  `.catalog/catalog.md`. We use `AGENTS.md` as the convention, since
  most agents read it. By default, Claude Code reads `AGENTS.md` only
  when there's no `CLAUDE.md` in the folder or above it, so a person who
  has both imports `@AGENTS.md` from `CLAUDE.md`
  ([research/agent-instruction-files.md](../research/agent-instruction-files.md)).

People who keep a catalog's folder in Git will probably want Git to
ignore `.catalog/catalog.md`, since that's the file that used to
conflict. They could set that once for every repository on their Mac in
Git's global ignore file
([gitignore documentation](https://git-scm.com/docs/gitignore)).

The catalog skill puts these in place, with the person's approval; an
agent can see the folders and where the person works, so it's well
placed to suggest where catalogs go. A menu bar item could do the same
later. We also don't know which folders should get a catalog at all.
Agents start wherever people start them, and an instruction file has to
load the catalog there. "Where agents start" in [daemon.md](daemon.md)
describes the problem.

The daemon doesn't write `.catalog/catalog.md` into a folder that a
cloud drive syncs, because two machines would keep rewriting the same
file. daemon.md explains why under "A side challenge: cloud drives."

## How work moves

Work moves in one direction. The daemon notices that a file may have
changed, records what it saw, decides what to do, reads the file,
profiles it, renders the catalog, and publishes it.

```text
Trees ──► Watcher ──► Store ◄──► Planner ──► Reader ──► Profiler
(scope)   (hints)    (state)    (decides)   (content)   (models)

Store ──► Renderer ──► Publisher (.catalog/catalog.md)

Settings feeds every step.      Every step reports to the Journal.

Daemon connects the steps.      CLI, menu bar ──► Control ──► Daemon
```

## Components

A profile is a copy of what its file says, so any component that keeps
or sends one has a second job beside the one named here: to say what it
keeps or sends, and who may read it. The answer is the same on every
platform this outline covers, including a server that serves a team. A
profile may be read by the people who can read its file, and by no one
else ([daemon.md](daemon.md#privacy-and-security)). Paths count too,
since a path can say nearly as much as the file. A component that makes
a new kind of copy states the same things for it.

### Model

The Model is the vocabulary the other components share. It holds values
and does no reading, writing, or network work.

- A tree is a folder that gets its own catalog, along with everything
  under it that no nested tree claims. Trees sit inside the folders a
  person chose to profile, as daemon.md describes under "Where agents
  start."
- An entry describes a file. Source trees hold thousands of files, so we
  may later need entries that describe folders, and the vocabulary
  should allow either.
- A content hash identifies what a file contained when we read it.
- A profile has a depth: a listing, a summary, or a position among its
  peers, as daemon.md describes under "Profiles deepen over time."
- A recipe records the reader, prompt, and model that produced a
  profile, so we can redo profiles when any of them changes. GNOME's
  indexer records its extractor the same way
  ([research/localsearch.md](../research/localsearch.md)).

### Settings

Settings answers one question: what applies to this tree right now? It
combines three sources, and each overrides the one before.

- Defaults built into the application.
- The person's global settings for this machine: which folders to
  profile and which parts to leave out, which model providers exist,
  their credentials, and limits on battery and spending.
- The catalog's own `.catalog/config.toml`, if it has one: which files
  to include, and what the catalog's owners prefer.

A tool that runs inside one repository knows where to look for its
configuration. A daemon serving a whole machine would have to search the
machine for it, and a knowledge worker would add folders through a menu
bar app, not by committing a file. So the person keeps one list of
folders to profile on the machine. Inside those folders, the daemon
already crawls every file, so a `.catalog/` folder there is enough to
mark where a catalog goes. We expect people to change the shape of these
settings once they use them, so we keep every component except Settings
independent of it.

A catalog's `config.toml` may come from someone else, for example with a
clone, so the daemon trusts it less than the person's global settings.
It can narrow what gets indexed or state preferences, but it can never
name a server to send files to or hold a credential.

### Trees

Trees takes the folders the person chose to profile and finds the trees
inside them: each folder that has a `.catalog/` folder. It decides which
tree a path belongs to and whether the path is a file we index, applying
the nesting rule and the include and ignore patterns. It knows nothing
about profiles, and nothing about Git.

### Watcher

The Watcher reports that paths may have changed. On macOS it listens to
FSEvents, treats the events as hints, and crawls when macOS says it
dropped events, as Apple advises
([research/spotlight.md](../research/spotlight.md)). On Linux it would
use inotify. It may have to poll a folder that a Mac shares into a
container, because file events don't always reach the container. A
Rancher Desktop issue titled "MacOS file system events don't pass
through to containers" has been open since 2022
([issue 1979](https://github.com/rancher-sandbox/rancher-desktop/issues/1979)),
and in a 2025 test on Docker Desktop, a container saw a file's new lines
only after someone ran `sync` or `ls`
([Docker forum](https://forums.docker.com/t/long-lived-tail-f-on-volume-mounts/148436)).
The Watcher doesn't know what a tree or a profile is, and it doesn't
decide when to act on a change.

### Store

The Store keeps the daemon's state outside every tree; on a Mac, under
`~/Library/Application Support`. It holds two kinds of state that cost
different amounts to rebuild.

- What we last saw of each file. A crawl can rebuild this.
- Profiles, keyed by content hash and recipe. Each one cost a model
  call, so the Store keeps them durably.

The Store holds a copy of every file's profile and every path, so its
readers are the people who can read those files. On a machine several
people share, that means the Store belongs to one account and no other
account can read it. On a server that serves a team, a profile reaches
only those who can read its file; a content hash alone doesn't show that
someone can.

The Store keys profiles by content, so a moved file keeps its summary. A
Git worktree, which is a second copy of mostly identical files, also
costs almost nothing to catalog. No other component knows how the Store
keeps its data.

### Planner

The Planner compares what we've seen with what we've profiled and
returns the work to do, in order. It takes the state, the time, and
conditions such as battery power as inputs and touches nothing else, so
its tests need no files, network, or real clock. Dropbox's engineers
store observations and derive the work from them for the same reason
([research/dropbox.md](../research/dropbox.md)).

We answer most of the open questions in daemon.md in the Planner: which
files come first, how long a file must sit unchanged before we profile
it, how hard to work on battery, and when to give up on a file that
keeps failing. With those answers in one component, we can change them
without touching the others.

### Reader

The Reader turns a file into something a model can take: text, pages, or
an image, along with cheap facts such as its type, its size, and its
first lines. Each kind of file gets its own reader, the way Spotlight
has one importer per file type. Prose, data files such as CSV, source
code, and images all go through the same interface. If no reader
understands a file, the file still gets a listing.

Reading is the step most likely to crash on a bad file, which is why
Spotlight and GNOME's indexer run it in a separate process
([research/spotlight.md](../research/spotlight.md),
[research/localsearch.md](../research/localsearch.md)). The Reader takes
a file, returns content, and depends on nothing else, so we can move it
into its own process later without changing the components around it.

### Profiler

The Profiler turns what the Reader produced, plus context about
neighboring files, into a profile at a requested depth. It owns the
prompts. A listing needs no model at all.

The Profiler also returns notes on how the profiling went: whether it
cut the file short, how much of the file the model needed, and whether
the model could tell what the file was for. Those notes go to the
Journal, so they describe the run and don't quote the file.

Because the Profiler decides what a model receives, it sends a file and
its neighbors' profiles and nothing a profile doesn't need. A profile
can repeat a secret from its file, and how the Profiler would filter
those out before the model sees them is still to be designed.

### Models

Models is a small interface with several providers behind it: hosted
services, and models running on the same machine. A provider accepts
content and returns text. It also declares what it can take (text,
images, PDFs), how much it can take at once, and whether the content
leaves the machine. The person switches providers in Settings, and the
Profiler works the same way with any of them.

Every provider also says what it keeps of the content it receives and
whether it trains on it, and a person can read that before choosing it.
That holds for a hosted service, and it holds for a model on the same
machine, which sends nothing away but may still keep prompts, caches, or
logs there. We rely on the provider's own account of these things; how a
person would judge a provider by that measure is still to be designed
(see "What we assume" in [daemon.md](daemon.md#privacy-and-security)). A
provider's credential stays in Settings and out of everything the other
components write.

### Renderer

The Renderer turns a tree's profiles into the text of a catalog. It
decides the catalog's shape: how entries are grouped, how each entry
shows its depth, and what the header says. It reads nothing and writes
nothing, so anything that delivers a catalog can use it.

### Publisher

The Publisher writes what the Renderer produced to a tree's
`.catalog/catalog.md`, replacing the file in one step. It's the only
component that writes inside a person's tree, so we can check the
promise in daemon.md in one place, and the promise about who may read
the file it writes. The catalog lists every path in its tree with a
profile of each file, so the people who can open the catalog file should
be people who may read everything it describes. The Publisher can't see
how a folder is shared; "What we assume" in
[daemon.md](daemon.md#privacy-and-security) says what follows when a
subfolder is shared more narrowly than its parent, and where a catalog
that is committed or synced may end up.

### Journal

The Journal is a record, kept on the machine, of what the daemon did:
each profiling with its recipe, how long it took, how many tokens it
used, and the Profiler's notes; each failure; and each time the daemon
caught up after falling behind. When we want statistics, we'll ask
questions of that record.

The record names every file the daemon touched and quotes errors that
may name more, so the people who may read it are the people who can read
those files. It never holds a credential, and what a report from a
client may add to it is limited the same way.

We don't plan to build statistics first. We do want every component to
report through the Journal from the start, so the record exists when we
want it. We see three uses.

- `catalog status` and a menu bar item can show what the daemon is doing
  and how far behind it is. Cursor's users objected when Cursor took
  that view away ([research/cursor.md](../research/cursor.md)).
- A person can compare models and prompts by time, cost, and failures.
- A Claude skill can read the statistics and recommend changes to the
  content itself. If the Profiler's notes show that many files only
  become clear after thousands of lines, the skill can suggest a
  five-line comment at the top of each one.

Under the file contract, agents never talk to the daemon, so the Journal
can't see which files agents go on to open. The Journal should still
accept reports from outside the daemon, so that a client such as an MCP
server can pass along what an agent tells it.

### Control

Control defines the requests a client can make of the daemon and the
answers it gets: status, prioritize this tree, retry a file, pause, read
or change settings, read statistics, fetch the catalog for a path, and
record a report in the Journal.

Control's messages travel over a Unix socket on every platform, because
XPC exists only on Apple's systems, and the Linux daemon uses the same
core.

Several of those requests return profiles, paths, or the Journal.
Control answers each requester with what that person could already read,
and nobody on the machine or the network can ask it for more. If a
server serves a team, a request from another client carries the same
rule. How Control tells who is asking is still to be designed.

### Daemon

The Daemon starts the other components, connects them, and shuts them
down. It holds no logic of its own. It makes changes to state one at a
time, as Dropbox's engineers did with a single control thread.

### Clients

Several programs act as clients: the CLI, a menu bar item, a settings
window, an MCP server, and a skill that reads statistics. Each one talks
to the Daemon through Control, and none of them reads the Store or the
Journal directly. To add an interface, we write a client instead of
changing the daemon.

What a client shows or passes along is a copy as well, so each client
states what it keeps and who may read it. An MCP server that delivers
catalogs to other people would meet the subfolder problem described in
[daemon.md](daemon.md#privacy-and-security).

Whatever shows status should make it easy to see which folders are
cataloged and which aren't. To its owner, a tree that never gets indexed
looks the same as one that's merely behind.

## Rules between components

- Every component may depend on the Model.
- These components don't depend on each other: Trees, Watcher, Store,
  Planner, Reader, Profiler, Models, Publisher, and Journal. The
  Publisher does depend on the Renderer. Only the Daemon knows all of
  them.
- Clients depend on Control and the Model, and on nothing in the daemon.
- Tests can replace the file system, the clock, the model, and the
  machine's power state, because each one sits behind an interface.
- Each component is its own module with declared dependencies, so the
  compiler refuses a dependency we didn't declare. Apple's `container`
  tool splits its Swift package into many targets, and its CLI doesn't
  depend directly on the server's target
  ([research/container.md](../research/container.md)).
- Nothing in the shared core, described next, uses a facility that
  exists on only one operating system.

## Platforms and language

The daemon runs on a Mac, on a headless Linux server, and in a Linux
container on a Mac or Windows laptop. The Mac comes first, because
everyone who uses the catalog today is on one. We also want to keep
Windows open as a native platform, because knowledge workers on Windows
could become our largest audience. We hope the shared core carries over
to Windows. If it can't, we'd write a separate implementation that
honors the same contract with agents.

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

We think sharing the core beats writing each platform separately. The
core holds the prompts, the depths, the scheduling, and the catalog's
format, and those determine how good a catalog is. Two separate
implementations would drift apart there first. Holding both to the
contract with agents wouldn't prevent that, because the contract covers
the file's format, not its quality.

We'll write the shared core and the CLI in Go, and the Mac's native
parts in Swift. This follows the shape of Tailscale's client, whose Mac
app is the same Go code as everywhere else plus a small GUI wrapper
([research/tailscale.md](../research/tailscale.md)). On the Mac, we'd
use Swift for the parts of the table's right-hand column that Go can't
reach well: the menu bar item and settings window, the app bundle that
macOS's permissions may require (see "Processes and macOS" below), and
readers and model providers built on Apple's frameworks. We believe the
Watcher and the launchd agent need no Swift, since FSEvents is a C
interface and launchd needs only a property list, but we haven't built
anything to check.

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

On a Mac, the daemon runs as a launchd agent for the logged-in user.
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
launch agent can read.

## Delivering the catalog over MCP

An MCP server could give agents the catalog without any file. An agent
would ask for the catalog for its current path and get what
`.catalog/catalog.md` would have held.

### What the documentation says

These points come from Claude Code's
[MCP documentation](https://code.claude.com/docs/en/mcp) unless we give
another source.

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
- Anthropic's servers, not the person's machine, call a custom remote
  connector, so the connector has to be "reachable over the public
  internet from Anthropic's IP ranges"
  ([custom connectors](https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp)).

### What we make of it

This is our reasoning, and we haven't built anything to test it.

- An agent gets a server's instructions without asking, but 4,096
  characters is too small for a catalog; twenty entries took about 2,400
  tokens in
  [FUTURE.md](https://github.com/westarete/catalog/blob/d9a8d1b3a9dd6826224bda2068a5422b37f8bce9/FUTURE.md).
  The instructions can tell the agent to fetch the catalog, and the
  agent then has to do it. With the file, the agent has the catalog with
  no step in between.
- Setting up MCP isn't much harder than setting up the file. A committed
  `.mcp.json` does for MCP what the import line does for the file, and
  with it nobody needs the import line or the Git ignore rule. It works
  only in agent tools that read `.mcp.json`, and the agent now knows the
  catalog is a tool.
- MCP suits content that has no folder to write a file into. When an
  agent reaches a data source through MCP, it can get the catalog for
  that source the same way.
- MCP gives agents a way to tell the daemon something, such as which
  file they opened for a task or that a profile misled them. If agents
  report reliably, that could answer the open question about measuring
  profiles.
- A server can cut a catalog down to the folder the agent asks about,
  which a single flat file can't do. That may matter for large source
  trees.

We'd keep the file as the main way to deliver the catalog and treat MCP
as a second way. We hold that view loosely. The choice depends on how
reliably agents fetch the catalog when instructions tell them to, and we
haven't measured that.

The components barely change. An MCP server is a client: it asks the
Daemon, through Control, for the catalog for a path, and the Daemon
answers with the Renderer's output. Anything an agent reports reaches
the Journal the same way.

A headless Linux server needs MCP. No agent reads a central server's
disk, so the server can deliver catalogs only over the network. For
Cowork, that means a server reachable from the public internet, as
quoted above. A daemon in a container on a laptop doesn't need MCP,
because it writes `.catalog/catalog.md` into the mounted folder.

## Left for implementation

We don't know these yet, and the components above stay the same
whichever way they turn out.

- What the Store uses: SQLite, or a file per profile.
- How the Watcher saves its place in the FSEvents history and recovers.
- Which models we ship with and which is the default.
- Whether the Reader or an on-device model runs in a helper process.
- Which libraries we use for arguments, logging, and service lifecycle.

## Open questions

- We don't know how we'll learn whether profiles lead agents to the
  right files, since agents never talk to the daemon. People could tell
  us what they see. An agent could also review past chat sessions with
  the person's permission, but reading those sessions is invasive, and
  we aren't comfortable calling it the answer.
- We haven't designed how people put the things under "What a tree
  needs" in place, or how a team overrides a global ignore rule.
- We don't know where catalogs should go so that agents find them where
  people start work. See "Where agents start" in [daemon.md](daemon.md).
- A source tree may have too many files for one flat
  `.catalog/catalog.md`. We could write entries for folders, or one
  catalog file per folder.
- We don't know how much the Linux cases matter.
- We don't know whether an MCP server is worth building, or what an
  agent would usefully report through one.
- A central server could hold profiles that other machines fetch by
  content hash, so that a fresh clone or a cloud agent wouldn't start
  empty. Cursor shortened its first index that way
  ([research/cursor.md](../research/cursor.md)). Profiles would then
  leave the machine, which raises the privacy questions Cursor faced and
  the ones in [daemon.md](daemon.md#privacy-and-security): who may read
  a profile on the server, and how the server knows a requester may read
  the file behind it.
