# The first catalog daemon: design

This is the design for the first version of the catalog daemon: the
smallest version that follows
[architecture.md](../../design/architecture.md) and is good enough for
people to stop running today's `catalog` command. It narrows
[mvp.md](../../design/mvp.md) and changes it in three ways. A person
lists the folders to catalog in one config on their machine, instead of
putting a `.catalog.toml` in each tree. A minimal `catalog status` comes
forward from mvp.md's "What waits." The daemon does its work in a single
loop, one file at a time. [daemon.md](../../design/daemon.md) explains
why we're building it.

We still don't know where catalogs should go so that agents find them
where people start work. daemon.md describes that problem under "Where
agents start." This version puts one catalog at the root of each listed
folder. That fits people who catalog Git repositories, as today's tool
does: they start their agents at the root, and the root already has an
instruction file that loads `.catalog.md`. It doesn't fit knowledge
workers' folders, and we don't offer it as an answer for them.

## Why, and what success looks like

Today's catalog falls behind when nobody runs `catalog update`, and its
files conflict in Git ([daemon.md](../../design/daemon.md), "What hurts
today"). This version ends both problems. The daemon keeps `.catalog.md`
current in the background and keeps no catalog state in any repository.

We'll judge this version by feel, in daily use by the people trying it.

- It works if their agents find the files a task needs about 80% of the
  time. If an agent finds four of the five documents a task needs,
  that's serviceable.
- People will go back to today's tool if accuracy falls toward 50%, or
  if the same errors keep coming back and they're always fixing them. If
  someone says "let's work on the budget" and the agent misses the budget documents, that counts as well below 50%.

We see reliability as the larger risk, because the daemon is new code
that runs unattended. Accuracy should start where today's catalog is,
because `catalog import` carries over every existing profile. It can
drift only as the daemon profiles files itself.

## Goals

- Keep `.catalog.md` current in each folder a person lists, without them
  running a command.
- Keep all of the daemon's state outside those folders.
- List a new file in the catalog within a couple of minutes, with its
  path and first lines, and replace that listing with a summary a few
  minutes after the file stops changing.
- Reuse every existing profile on the day someone moves a repository
  over, with no model calls.
- Let people see at a glance whether the daemon is running and which
  files are failing.

## Non-goals

We'll build these in later versions: FSEvents, model providers other
than Anthropic, the peers depth, setup commands, a menu bar item or app
bundle, MCP, Linux, Windows, and profiling source code
([issue #4](https://github.com/westarete/catalog/issues/4)). Control
answers one request, status. We won't release this version through
Homebrew yet; people who try it build it from source.

A person can set only globs per folder in this version, not a model,
key, or truncation cap. Today's per-repository config allows all three.
One model and one key per machine are enough for a first version, and
per-folder settings can come once people ask for them.

The daemon doesn't detect synced folders yet, and it must never write
into one ([daemon.md](../../design/daemon.md), "A side challenge: cloud
drives"). Until it can tell, people should list only folders that no
cloud drive syncs, and the README should say so.

## Anti-goals

This version has failed, even if it technically works, if any of these
happen:

- People spend their time fixing failures that keep coming back instead
  of using the catalog.
- One bad file, or a model outage, stops the daemon from updating the
  catalog for everything else.
- An agent finds `.catalog.md` empty, half-written, or missing an
  obvious document.
- The daemon writes anything inside a listed folder except
  `.catalog.md`.

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
   the config, crawls the trees, asks the Planner for work, and profiles
   one file at a time. It publishes the tree's catalog after each file.
   A second goroutine answers status requests from a snapshot the loop
   hands it. Only one goroutine changes state, so we can follow a
   failure easily and test the loop with a fake clock and model. The
   loop makes one model call at a time, so a large batch of new files
   waits its turn. People moving from today's tool won't feel that wait
   on the first day, because import brings over their existing profiles.
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

## Settings and trees

### One config on the machine

A person lists the folders to catalog in one TOML file,
`~/Library/Application Support/com.westarete.catalog/config.toml`. The
daemon never searches the machine for configuration. A menu bar app
would later edit the same list. We expect to change the config's shape
once people use the daemon, so we keep every component except Settings
independent of it.

```toml
api_key = "sk-ant-..."
model = "claude-haiku-5-5"

[[folder]]
path = "~/src/acme/handbook"

[[folder]]
path = "~/src/acme/proposals"
enumerate = ["docs/**/*.md", "**/*.pdf"]
```

- If a folder's entry has no `enumerate` or `ignore`, the daemon uses
  the built-in defaults, which match today's `config.example.toml`. The
  default `enumerate` is `**/*.csv`, `**/*.md`, `**/*.pdf`, and
  `**/*.txt`, and the default `ignore` is `README.md`, `.catalog.md`,
  `**/CLAUDE.md`, and `**/AGENTS.md`. A list in the entry replaces the
  default list instead of adding to it. (Our call, matching today's
  per-repository file.)
- The person keeps the API key in this file and makes the file readable
  only by them. The daemon doesn't read `ANTHROPIC_API_KEY`, since
  launchd doesn't start it from the person's shell; the builder confirms
  that in the `launchd.plist` manual. Once there's an app, the key
  belongs in the Keychain.
- The daemon rereads the file on every crawl, so edits take effect
  within a minute without a restart. If the file doesn't parse, the
  daemon keeps using the last settings that did and reports the error
  through status.
- We don't version the format or write migration code yet. In this first
  version, people fix the file by hand when the format changes;
  migration code waits until the format settles.

### Which folders are trees

Each listed folder is a tree. If someone lists a folder inside another
listed folder, the inner one is a tree of its own. The daemon has no
rules for any app's files, Git's included, and catalogs whatever the
globs match. It does skip `.git` directories when it crawls, because
they hold Git's records rather than anyone's content. Skipping them
saves time and doesn't change what an agent sees. (Our call.)

So Claude Code's worktrees under `.claude/worktrees/` show up in their
repository's catalog, under paths that start with
`.claude/worktrees/<name>/`. We trust the agent to see from those paths
that the copies aren't its own. Agents in a project's root can already
come across those files, so we expect Anthropic built its agents to tell
them apart. We haven't checked whether Claude Code's search tools skip
them. We accept two costs:

- Each worktree adds a copy of every entry. With three worktrees, a
  repository's catalog is four times as long. The copies share summaries
  with the main checkout, so the model cost stays the same.
- An agent working in a worktree finds no `.catalog.md` at the
  worktree's root. Claude Code loads the `CLAUDE.md` in every folder
  above where it starts
  ([research/agent-instruction-files.md](../../research/agent-instruction-files.md)),
  so that agent probably gets the repository's catalog anyway. The
  header then has to keep the agent working in its own copy of each
  file, as described under "Renderer and Publisher."

If a listed folder has a `.catalog/` directory at its root, its owner
hasn't moved it from today's tool yet. The daemon skips that folder and
shows it as skipped in status. (Our call.)

## How the parts behave

### Watcher: a crawl every minute

Once a minute, the loop walks each tree, applies the globs, and records
each file's size and modification time. If either has changed since the
last crawl, or the file is new, the loop reads the file. It records the
file's SHA-256 content hash, the same hash today's tool stores
([store.go](https://github.com/westarete/catalog/blob/d9a8d1b3a9dd6826224bda2068a5422b37f8bce9/store.go)),
and the time it saw the change. When a file disappears, the loop drops
it from the tree on that crawl. We'll add FSEvents later behind the same
interface.

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
`catalog import` stores the profiles it copies under the recipe
`legacy`.

The Store keys summaries by content, so every copy of a file shares one
summary. A summary may point to a near-duplicate with "see X.md," and
the model took that path from the tree where it wrote the summary. If
the same content sits under a different prefix, such as a subtree copied
into another repository, the pointer can be wrong. Worktrees keep the
same paths. We accept this for now and will revisit it if copies under
other prefixes turn out to be common. (Our call.)

`catalog import` and `catalog daemon --once` open the Store directly,
even while the daemon runs. So the Store uses SQLite's write-ahead log
and a busy timeout, and the loop rereads summaries from the Store each
time it plans instead of keeping its own copy. We treat these two
commands as maintenance tools, not clients; `catalog status` still asks
the daemon through the socket. (Our call.)

### Planner

The Planner is a pure function. Given what the Store holds, the current
recipe, and the time, it returns the files to summarize, in order. A
file needs a summary if all three of these hold:

- The Store has no summary for its content hash under the current recipe
  or under `legacy`.
- The file hasn't changed for three minutes.
- The file isn't waiting to retry after a failure.

The Planner puts the files that changed longest ago first. If a file has
a summary only under an older recipe, the catalog still shows that
summary, and the Planner queues the file after every file that has none.
That way, after a prompt change, the daemon gradually redoes the catalog
without holding up new files.

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
of every other file in the same tree as neighbors, in a single pass. It
starts with today's `profileInstruction`, unchanged
([generate.go](https://github.com/westarete/catalog/blob/d9a8d1b3a9dd6826224bda2068a5422b37f8bce9/generate.go)).
People already use that prompt with today's tool, and keeping it makes
new summaries read like the imported ones. A listing needs no model
call.

Models is an interface with one provider, Anthropic, which uses the
model named in the config. The provider sorts each failure into one of
two kinds.

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
An entry with a summary shows the summary. An entry with only a listing
starts with "Not summarized yet." and then gives the type, size, and
first lines.

The builder rewrites the catalog's header in three ways.

- It tells the agent what a listing-only entry means.
- It drops `catalog update` and `catalog status`, since agents don't run
  commands.
- It tells the agent how to read paths. Today's header says paths are
  relative to the catalog's own folder. An agent in a worktree that
  reads the repository's catalog would follow that to the main
  checkout's copy of a file, and could edit the wrong copy. The new
  header says paths are relative to the catalog's folder, and that an
  agent working in a copy of that folder should open the same path in
  its own copy.

The Publisher writes `.catalog.md` to a temporary file in the same
folder and renames it into place. It skips the write if the text hasn't
changed. The loop publishes a tree after each crawl that changed the
tree and after each new summary in it. If rendering or writing fails,
the Publisher leaves the old `.catalog.md` as it was. If someone removes
a folder from the config, the daemon leaves that folder's `.catalog.md`
alone.

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
- for each tree, its path, how many files it has, how many have
  summaries, and how many are waiting
- each failing file, with its last error and when the daemon will retry
  it
- each listed folder the daemon skipped, and why

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
- `catalog daemon --once <folder>` crawls that folder's tree once,
  summarizes every file waiting there without the three-minute wait, and
  exits.
- `catalog import <folder>` copies the profiles from the folder's
  `.catalog/catalog.db` into the Store under the `legacy` recipe, keyed
  by the content hash stored with each one. It reads the old files and
  changes nothing in the folder.
  - If a file changed after someone last ran `catalog update`, its hash
    won't match its old profile, and the daemon summarizes it again.
  - If the Store already has a `legacy` summary for the same hash, from
    another path or repository, import keeps the one it has.
  - If the folder's `.catalog/config.toml` has `enumerate` or `ignore`
    lists that differ from the defaults, import prints the `[[folder]]`
    entry to add to the config.
- `catalog status` prints the status described above.

## Moving from today's tool

Someone moving from today's tool should move all of their repositories
in one sitting, so they never run the old and new tools side by side.
Both are called `catalog`, and `catalog status` means something
different in each. If they stopped halfway, `catalog update` wouldn't
work in the repositories they hadn't moved.

Before anyone moves, we update or retire the `knowledge-work:catalog`
skill in West Arete's
[claude-plugins-internal](https://github.com/westarete/claude-plugins-internal),
so that no agent runs `catalog update` or `bootstrap` in a moved
repository and recreates `.catalog/`.

Then each person:

1. Removes the Homebrew `catalog` and installs the new build in its
   place.
2. For each repository:
   1. Runs `catalog import <folder>`.
   2. Deletes `.catalog/`, stops tracking `.catalog.md` with
      `git rm --cached`, and commits.
   3. Adds the folder to the config, with any globs import printed. The
      daemon skips a folder that still has `.catalog/`, so the folder
      goes into the config only after `.catalog/` is gone.
   4. Leaves the line in `CLAUDE.md` that loads `.catalog.md` as it is.
      Repositories set up for today's tool load the catalog from
      `CLAUDE.md`, and that line keeps working. New setups use
      `AGENTS.md`.
3. Adds `.catalog.md` to Git's global ignore file, once.
4. Loads the launchd agent and checks `catalog status`.

## The order to build it in

Each step is usable before the next one starts.

1. Install the mutation testing tool and confirm it runs on this module.
   Delete [mvp.md](../../design/mvp.md), and point
   [README.md](../../../README.md) at this design.
2. `catalog daemon --once <folder>`: Settings, Trees, Store, Planner,
   Reader, Profiler, Models, Renderer, Publisher, and Journal working
   over one tree, with tests that use a fake file system, clock, and
   model.
3. `catalog import`, then a manual run against a scratch clone of a
   repository that uses today's tool, with its `.catalog/` deleted.
   Working copies keep using today's tool until the move.
4. The crawl loop, listed folders nested inside one another, rereading
   the config, and the failure backoff and pause.
5. Status over the socket and the launchd property list. Then the skill
   change, and the people using today's tool move their repositories as
   described above.

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
