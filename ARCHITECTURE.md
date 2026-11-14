# Architecture of the catalog daemon: draft outline

This outline names the components of the catalog daemon, says what each
one is for, and says what each one is kept from knowing. Nothing here is
built, and we expect the outline to change as we work through it.
[DAEMON.md](DAEMON.md) explains the problem and what we want; this file
is about how the application is divided. We plan to write it in Swift
for macOS.

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
profiles it, and publishes the result.

```text
Trees ──► Watcher ──► Store ◄──► Planner ──► Reader ──► Profiler ──► Publisher
(scope)   (hints)    (state)    (decides)   (content)   (models)    (.catalog.md)

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
- The person's settings for this Mac: which folders the daemon may look
  in, which model providers exist, their credentials, and limits on
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

The Watcher reports that paths may have changed. It listens to FSEvents,
treats the events as hints, and crawls when macOS says events were
dropped, as Apple advises
([research/spotlight.md](research/spotlight.md)). It doesn't know what a
tree or a profile is, and it doesn't decide when to act on a change.

### Store

The Store keeps the daemon's state under
`~/Library/Application Support`, outside every tree. It holds two kinds
of state with different costs.

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
services and models running on the Mac. A provider accepts content and
returns text. It also declares what it can take (text, images, PDFs),
how much it can take at once, and whether the file leaves the machine.
The person switches providers in Settings, and the Profiler works the
same way with any of them.

### Publisher

The Publisher renders a tree's `.catalog.md` from the Store and replaces
the file in one step. It's the only component that writes inside a
person's tree, which keeps the promise in DAEMON.md checkable in one
place. It also decides the file's shape: how entries are grouped, how
each entry shows its depth, and what the header says.

### Journal

The Journal is a record, kept on the Mac, of what the daemon did: each
profiling with its recipe, how long it took, how many tokens it used,
and the Profiler's notes; each failure; each time the daemon caught up
after falling behind. Statistics are questions asked of that record.

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

The Journal can't see which files agents go on to open, because agents
never talk to the daemon.

### Control

Control defines the requests a client can make of the daemon and the
answers it gets: status, prioritize this tree, retry a file, pause, read
or change settings, read statistics. It defines messages, not how they
travel.

### Daemon

The Daemon starts the other components, connects them, and shuts them
down. It holds no logic of its own. It serializes changes to state,
which Dropbox's engineers did with a single control thread and which a
Swift actor gives us.

### Clients

The CLI, a menu bar item, a settings window, and a skill that reads
statistics are all clients. Each one talks to the Daemon through
Control, and none of them reads the Store or the Journal directly.
Adding an interface then means writing a client, not changing the
daemon.

Whatever shows status should make it easy to see which folders are
cataloged and which aren't, since a tree that never gets indexed looks
the same to its owner as one that's merely behind.

## Rules between components

- Every component may depend on the Model. Trees, Watcher, Store,
  Planner, Reader, Profiler, Models, Publisher, and Journal don't depend
  on each other. Only the Daemon knows all of them.
- Clients depend on Control and the Model, and on nothing in the daemon.
- The file system, the clock, the model, and the Mac's power state sit
  behind interfaces that tests can replace.
- Each component is a library target in one Swift package, so the
  compiler refuses a dependency we didn't declare. Apple's `container`
  tool splits its package the same way, and its CLI depends on client
  targets only ([research/container.md](research/container.md)).

## Processes and macOS

The daemon runs as a launchd agent for the logged-in user. Apple's
`container` tool has the same shape: a CLI, a launch agent, and XPC
between them ([research/container.md](research/container.md)).

macOS permissions affect how we package it. macOS protects Desktop,
Documents, Downloads, iCloud Drive, and third-party cloud storage
([Howard Oakley, 2026](https://eclecticlight.co/2026/04/07/privacy-protected-folders/)),
and those are the folders where knowledge workers keep files. An answer
on Apple's developer forums says macOS decides access by the code
responsible for a process, and that an agent installed through
`SMAppService` has its app as the responsible code
([Apple Developer Forums](https://developer.apple.com/forums/thread/756510)).
`SMAppService` expects the agent's property list inside an app bundle
([Apple Developer Forums](https://developer.apple.com/forums/thread/745009?page=2)).
We haven't tested any of this ourselves. If it holds, the daemon should
ship inside an app bundle early, even while the CLI is its only
interface, and the folders a person authorizes are also the folders
macOS asks them about.

## Left for implementation

These are unknown, and the components above don't change whichever way
they turn out.

- How Control's messages travel: XPC or a socket.
- What the Store uses: SQLite, or a file per profile.
- How the Watcher saves its place in the FSEvents history and recovers.
- Which models we ship with and which is the default.
- Whether the Reader or an on-device model runs in a helper process.
- Which Swift packages we use for arguments, logging, and service
  lifecycle.

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
