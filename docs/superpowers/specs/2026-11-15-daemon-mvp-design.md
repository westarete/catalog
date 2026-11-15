# The first catalog daemon: design

This is the design for the first version of the catalog daemon: the
smallest version that follows
[architecture.md](../../design/architecture.md) and is good enough for
people to stop running today's `catalog` command. It narrows mvp.md, an
earlier draft now removed, and changes it in three ways. A person lists
the folders for the daemon to watch in one config on their machine, and
a `.catalog/` folder inside them marks each place a catalog goes. A
minimal `catalog status` comes forward from mvp.md's "What waits." The
daemon does its work in a single loop, one file at a time.
[daemon.md](../../design/daemon.md) explains why we're building it.

We still don't know where catalogs should go so that agents find them
where people start work. daemon.md describes that problem under "Where
agents start." This version maintains a catalog wherever people put a
`.catalog/` folder. That fits people who catalog Git repositories, as
today's tool does: they start their agents at the root, the root already
has a `.catalog/`, and an instruction file there loads the catalog. This
version doesn't try to work out where catalogs belong in knowledge
workers' folders.

## Why, and what success looks like

Today's catalog falls behind when nobody runs `catalog update`, and its
files conflict in Git ([daemon.md](../../design/daemon.md), "What hurts
today"). This version ends both problems. The daemon keeps each catalog
current in the background and keeps its own state outside the folders it
serves.

We'll judge this version by feel, in daily use by the people trying it.

- It works if their agents find the files a task needs about 80% of the
  time. If an agent finds four of the five documents a task needs,
  that's serviceable.
- People will go back to today's tool if accuracy falls toward 50%, or
  if the same errors keep coming back and they're always fixing them. If
  someone says "let's work on the budget" and the agent misses the budget documents, that counts as well below 50%.

We see reliability as the larger risk, because the daemon is new code
that runs unattended. Each repository starts fresh: the daemon profiles
every file itself instead of reusing today's profiles. That costs some
waiting on the first day. In return, the Journal records how the daemon
profiled every file, and that's the data we want for tuning it.

## Goals

- Keep each catalog current, without anyone running a command.
- Keep the daemon's own state outside the folders it watches.
- List a new file in its catalog within a couple of minutes, with its
  path and first lines, and replace that listing with a summary a few
  minutes after the file stops changing.
- Let people see at a glance whether the daemon is running and which
  files are failing.

## Non-goals

We'll build these in later versions: FSEvents, model providers other
than Anthropic, the peers depth, setup commands, a menu bar item or app
bundle, MCP, Linux, Windows, and profiling source code
([issue #4](https://github.com/westarete/catalog/issues/4)). Control
answers one request, status. We won't release this version through
Homebrew yet; people who try it build it from source.

A catalog's own config can set only `enumerate` and `ignore` in this
version, not a model, key, or truncation cap. One model and one key per
machine are enough for a first version, and more per-catalog settings
can come once people ask for them.

The daemon doesn't detect synced folders yet, and it must never write
into one ([daemon.md](../../design/daemon.md), "A side challenge: cloud
drives"). Until it can tell, people should watch only folders that no
cloud drive syncs, and the README should say so.

This version also watches only folders that macOS doesn't protect, such
as `~/src`. Desktop, Documents, Downloads, and iCloud Drive wait for the
app bundle that architecture.md expects those folders to need
("Processes and macOS"). (Our call.)

## Anti-goals

This version has failed, even if it technically works, if any of these
happen:

- People spend their time fixing failures that keep coming back instead
  of using the catalog.
- One bad file, or a model outage, stops the daemon from updating
  catalogs for everything else.
- An agent finds a catalog empty, half-written, or missing an obvious
  document.
- The daemon writes anything inside a watched folder except the
  `catalog.md` in each `.catalog/`.

## Constraints

- We write everything in Go, as architecture.md says under "Platforms
  and language." The module is `westarete.com/catalog` on Go 1.25
  ([go.mod](../../../go.mod)).
- Each component in architecture.md gets its own package, and only the
  Daemon imports all of them ("Rules between components"). Tests replace
  the file system, the clock, and the model through interfaces.
- We judge a test by whether it catches a fault. We run a mutation
  testing tool over each new package and treat a mutant that no test
  kills as a missing test ([CLAUDE.md](../../../CLAUDE.md), "Tests").
- The builder writes code and configuration for the Anthropic SDK,
  SQLite, launchd, and Gremlins from their current documentation, not
  from memory.
- Developers often keep repositories in a folder such as `~/src`, which
  isn't among the folders Howard Oakley lists as protected by macOS
  ([architecture.md](../../design/architecture.md), "Processes and
  macOS"). Nobody has tested whether a bare launch agent can read it.

## Approaches

We considered three ways to structure the daemon's work and chose the
first.

1. **One loop, one file at a time (chosen).** A single goroutine rereads
   the config, crawls the watched folders, asks the Planner for work,
   and profiles one file at a time. It publishes that file's catalog
   after each one. A second goroutine answers status requests from a
   snapshot the loop hands it. Only one goroutine changes state, so we
   can follow a failure easily and test the loop with a fake clock and
   model. The loop makes one model call at a time, so a large batch of
   new files waits its turn. People will notice that wait on the first
   day, while the daemon profiles each repository from scratch, and
   rarely after that.
2. **One loop that hands profiling to a pool of workers.**
   architecture.md borrows this shape from Dropbox. Workers catch up on
   a batch several times faster. But cancellation, rate limits, and
   retries then span several workers, and two files profiled at the same
   moment can't use each other's summaries as neighbors. Concurrency is
   the likeliest source of the recurring failures that would send people
   back to today's tool. If the first approach proves too slow, we can
   switch by changing only the Daemon, since the Planner already returns
   an ordered list and the Reader and Profiler already handle one file
   at a time.
3. **No long-running process.** launchd would start the work once a
   minute, and the process would exit when done. We dropped it because
   no process would be running to answer `catalog status`. Status would
   have to read the Store directly, and architecture.md forbids that.

## Settings and catalogs

### Where a catalog goes

A `.catalog/` folder inside a watched folder marks a catalog. The daemon
maintains `catalog.md` inside it, covering the files in the folder that
holds `.catalog/` and everything below, except folders that have a
`.catalog/` of their own. The daemon never creates a `.catalog/`; a
person or an agent does. Later versions might also put statistics there,
as output the daemon writes, not state it depends on.

The daemon catalogs whatever the globs match, and never profiles
anything inside a `.catalog/` folder. When it crawls, it skips `.git`
directories, which hold Git's records rather than anyone's content. (Our
call.)

Folders that apps create inside a catalog's folder, such as Claude
Code's worktrees under `.claude/worktrees/`, show up in that catalog
like any other files, unless someone puts a `.catalog/` in them. A
repository with several worktrees gets a longer catalog, but the copies
share summaries, so the model cost stays the same.

The daemon profiles only files that belong to some catalog. It still
crawls the whole watched folder, so it notices a new `.catalog/`
anywhere inside it.

### Settings

Settings come in three layers, and each one overrides the one before:

1. Defaults built into the daemon. The default `enumerate` is
   `**/*.csv`, `**/*.md`, `**/*.pdf`, and `**/*.txt`, and the default
   `ignore` is `README.md`, `**/CLAUDE.md`, and `**/AGENTS.md`, matching
   today's `config.example.toml`.
2. The global config,
   `~/Library/Application Support/com.westarete.catalog/global.toml`.
3. A catalog's own `.catalog/config.toml`, if it has one. It can be
   empty, or hold only comments.

A list in a later layer replaces the list before it instead of adding to
it. (Our call, matching today's per-repository file.) Globs are relative
to the catalog's folder, so they mean what they mean in today's configs.
In practice people mostly use them to choose file types.

The global config lists the folders to watch, the API key, the model,
and any `enumerate` and `ignore` defaults for this machine. A menu bar
app would later edit it.

```toml
api_key = "sk-ant-..."
model = "claude-haiku-5-5"
enumerate = ["**/*.md", "**/*.pdf", "**/*.txt", "**/*.csv"]

[[watch]]
folder = "~/src"
```

- The person keeps the API key in the global config and makes that file
  readable only by them. A catalog's own config can't hold a key or name
  a model, since it may have come from someone else, for example with a
  clone. The daemon doesn't read `ANTHROPIC_API_KEY`, since launchd
  doesn't start it from the person's shell; the builder confirms that in
  the `launchd.plist` manual. Once there's an app, the key belongs in
  the Keychain.
- The daemon ignores keys it doesn't use, so the `.catalog/config.toml`
  files from today's tool work as they are; the daemon reads their
  `enumerate` and `ignore`.
- The daemon rereads every config on every crawl, so edits take effect
  within a minute without a restart. If a file doesn't parse, the daemon
  keeps using the last settings that did and reports the error through
  status.
- We don't version the formats or write migration code yet. In this
  first version, people fix a file by hand when its format changes. We
  expect to change the formats once people use the daemon, so we keep
  every component except Settings independent of them.

## How the parts behave

### Watcher: a crawl every minute

Once a minute, the loop walks each watched folder, finds the catalogs,
applies the globs, and records each file's size and modification time.
If either has changed since the last crawl, or the file is new, the loop
reads the file. It records the file's SHA-256 content hash, the same
hash today's tool stores
([store.go](https://github.com/westarete/catalog/blob/d9a8d1b3a9dd6826224bda2068a5422b37f8bce9/store.go)),
and the time it saw the change. When a file disappears, the loop drops
it on that crawl. We'll add FSEvents later behind the same interface.

### Store

The Store keeps one SQLite file in the Application Support folder, with
three kinds of records.

- What the last crawl saw: each file's size, modification time, and
  content hash, and when it last changed. A crawl can rebuild all of it.
- Summaries, keyed by content hash and recipe. Each one cost a model
  call, and this version never deletes them.
- Failures for each file: how many in a row, the last error, and when to
  try again.

A recipe names the reader, the prompt version, and the model.

The Store keys summaries by content, so every copy of a file shares one
summary. A summary may point to a near-duplicate with "see X.md," and
the model took that path from the catalog where it wrote the summary. If
the same content sits under a different prefix, such as a subtree copied
into another repository, the pointer can be wrong. We accept this for
now and will revisit it if copies under other prefixes turn out to be
common. (Our call.)

`catalog daemon --once` opens the Store directly, even while the daemon
runs. So the Store uses SQLite's write-ahead log and a busy timeout, and
the loop rereads summaries from the Store each time it plans instead of
keeping its own copy. We treat `--once` as a maintenance tool, not a
client; `catalog status` still asks the daemon through the socket. (Our
call.)

### Planner

The Planner is a pure function. Given what the Store holds, the current
recipe, and the time, it returns the files to summarize, in order. A
file needs a summary if all three of these hold:

- The Store has no summary for its content hash under the current
  recipe.
- The file hasn't changed for three minutes.
- The file isn't waiting to retry after a failure.

The Planner puts the files that changed longest ago first. If a file has
a summary only under an older recipe, the catalog still shows that
summary, and the Planner queues the file after every file that has none.
That way, after a prompt change, the daemon gradually redoes the
catalogs without holding up new files.

The three-minute wait is our first answer to the question in daemon.md
about not profiling a file on every auto-save.

### Reader

- The Reader reads text files as text and cuts them at 400,000 bytes,
  today's default cap.
- It sends PDFs to the model whole, as today's tool does. The builder
  checks the API's current limits on PDF size and page count. If a PDF
  exceeds them, the daemon lists it without a summary and notes that in
  the Journal.
- For a listing, the Reader takes the path, the type, the size, and, for
  text files, the first ten lines, cut at 512 bytes. It gives no first
  lines for a PDF, because that would need a PDF text extractor.

### Profiler and Models

The Profiler sends one file's content to the model, with the summaries
of every other file in the same catalog as neighbors, in a single pass.
It starts with today's `profileInstruction`, unchanged
([generate.go](https://github.com/westarete/catalog/blob/d9a8d1b3a9dd6826224bda2068a5422b37f8bce9/generate.go)).
People already use that prompt with today's tool, so it's a known
starting point to tune from. A listing needs no model call.

Models is an interface with one provider, Anthropic, which uses the
model named in the global config. The provider sorts each failure into
one of two kinds.

- A problem with the file, such as a request the API rejects as too
  large. The loop retries that file after 1 minute, doubles the wait
  each time up to 6 hours, and then keeps retrying every 6 hours. Other
  files go on as usual.
- A problem with the service, such as a network error, a rate limit, a
  server error, or a rejected API key. The loop pauses all summarizing
  with the same backoff, keeps crawling, and keeps publishing listings.
  It doesn't count the failure against any file.

### Renderer and Publisher

The Renderer writes entries in today's shape: a `## <dir>/` heading for
each folder and a `### <path>` entry for each file
([catalog.go](https://github.com/westarete/catalog/blob/d9a8d1b3a9dd6826224bda2068a5422b37f8bce9/catalog.go)).
Paths are relative to the folder that holds `.catalog/`. An entry with a
summary shows the summary. An entry with only a listing starts with "Not
summarized yet." and then gives the type, size, and first lines.

The builder rewrites the catalog's header in two ways.

- It tells the agent what a listing-only entry means.
- It drops `catalog update` and `catalog status`, since agents don't run
  commands.

The Publisher writes `.catalog/catalog.md` to a temporary file in the
same folder and renames it into place. It skips the write if the text
hasn't changed. The loop publishes a catalog after each crawl that
changed it and after each new summary in it. If rendering or writing
fails, the Publisher leaves the old `catalog.md` as it was. If someone
stops watching a folder, the daemon leaves its catalogs alone.

### Journal

The Journal is an append-only file, `journal.jsonl`, in the Application
Support folder. The daemon writes one JSON line for each event: each
summary with its recipe, tokens, and time; each failure; each pause and
resume; and each config error. This version doesn't rotate the file.

### Status

The daemon listens on a Unix socket in the Application Support folder
and answers one request. When a person runs `catalog status`, it
connects, asks, and prints:

- when the daemon started and when it last crawled
- any config error, and whether summarizing is paused and why
- for each catalog, its folder, how many files it covers, how many have
  summaries, and how many are waiting
- each failing file, with its last error and when the daemon will retry
  it

After each step, the loop hands the status goroutine a fresh snapshot,
so answering a request never touches the Store. If the socket doesn't
exist, `catalog status` reports that the daemon isn't running. macOS
limits a socket's path to about 104 bytes. The builder checks the exact
figure; the path under Application Support fits well within it.

### Daemon and launchd

`catalog daemon` runs the loop until something stops it. The repository
includes a launchd property list, written by hand, that a person
installs to run the daemon as an agent for their login. launchd starts
it at login and restarts it if it exits. The builder writes the list
from the `launchd.plist` manual, not from memory.

## Commands

- `catalog daemon` runs the loop.
- `catalog daemon --once <folder>` crawls the catalog in that folder
  once, summarizes every file waiting there without the three-minute
  wait, and exits.
- `catalog status` prints the status described above.

## What we expect from the skill

The daemon and the `knowledge-work:catalog` skill in West Arete's
[claude-plugins-internal](https://github.com/westarete/claude-plugins-internal)
work as a pair. The daemon does the mechanical work: it watches folders,
profiles files, and keeps each catalog current. The skill does the work
that takes judgment, and an agent is well placed to do it, since it can
see the person's folders and where they've been working. We'll rewrite
the skill alongside this version; this section says what it has to do,
not how.

The skill helps a person or their agent:

- Decide whether a folder is worth a catalog at all. Today's skill
  already has a guide for this, starting from about 20 documents, and it
  carries over.
- Choose where catalogs go, by suggesting places from the folders and
  from where the person works with agents, and asking before acting.
- Set up a catalog in each place the person accepts: create the
  `.catalog/` folder, add a `config.toml` only if the catalog needs
  settings other than the global ones, and add the line that loads
  `.catalog/catalog.md` to the folder's `AGENTS.md`, or `CLAUDE.md` for
  Claude Code, without disturbing the rest of the file.
- Add a folder to the watch list in `global.toml`, or tell the person
  how when the agent can't reach that file, as in Cowork.
- Move a repository from today's tool, following the steps below.
- Understand what the daemon is doing, by running `catalog status` where
  it can.
- Judge whether profiles are good, as today's skill explains under
  "Profiles, not summaries."

The skill never runs `catalog update`, `bootstrap`, or `force`, which
don't exist anymore. It never writes `catalog.md`, which belongs to the
daemon.

The skill relies on these parts of the daemon, so changing any of them
means changing the skill too: the `.catalog/` folder as the marker,
`catalog.md` and `config.toml` inside it, the layering of `config.toml`
over `global.toml`, and the `catalog status` command.

## Moving from today's tool

Someone moving from today's tool should move all of their repositories
in one sitting, so they never run the old and new tools side by side.
Both are called `catalog`, and `catalog status` means something
different in each. If they stopped halfway, `catalog update` wouldn't
work in the repositories they hadn't moved.

Before anyone moves, we replace the `knowledge-work:catalog` skill with
the rewrite described above, so that no agent runs `catalog update` or
`bootstrap` in a moved repository.

Then each person:

1. Removes the Homebrew `catalog` and installs the new build in its
   place.
2. For each repository:
   1. Deletes today's database, `.catalog/catalog.db`, and the old
      `.catalog.md` at the root. The repository's `.catalog/config.toml`
      stays as it is.
   2. Changes the line that loads the catalog from `@.catalog.md` to
      `@.catalog/catalog.md`.
3. Adds the folders that hold their repositories to the global config's
   watch list.
4. Loads the launchd agent and checks `catalog status`.

Repositories that used to commit their catalog will probably want Git to
ignore `.catalog/catalog.md` from now on, since that's the file that
used to conflict.

In a shared repository, these steps change files everyone sees, so
whoever moves it lets the others know.

## The order to build it in

Each step is usable before the next one starts.

1. Install the mutation testing tool and confirm it runs on this module.
   Delete mvp.md, and point [README.md](../../../README.md) at this
   design.
2. `catalog daemon --once <folder>`: Settings, Trees, Store, Planner,
   Reader, Profiler, Models, Renderer, Publisher, and Journal working
   over one catalog, with tests that use a fake file system, clock, and
   model.
3. A manual run against a scratch clone of a repository that uses
   today's tool, to see the catalog the daemon produces. Working copies
   keep using today's tool until the move.
4. The crawl loop, finding `.catalog/` folders and nested catalogs,
   rereading the configs, and the failure backoff and pause.
5. Status over the socket and the launchd property list, including a
   check that the launch agent can read a watched folder such as
   `~/src`.

The implementation plan covers these five steps in this repository.
Rewriting the skill is separate work in claude-plugins-internal, and the
people using today's tool move their repositories only after both are
done. (Our call.)

## Left to the builder

- Which Go packages to use for SQLite and command-line arguments, chosen
  from their current documentation. A SQLite package that doesn't need
  cgo would keep the build simpler, if a sound one exists.
- How to enforce architecture.md's dependency rules between the
  component packages. golangci-lint's depguard is one way.
- The mutation testing tool.
  [coding-agent-tools.md](../../research/coding-agent-tools.md) names
  Gremlins as the candidate for Go. If Gremlins doesn't run on this
  module, the builder brings that back to the maintainers instead of
  choosing another tool.
- The SQLite schema, the status message format, and the socket's file
  name.
- The exact wording of the catalog header and of listing entries, within
  the shape above.
