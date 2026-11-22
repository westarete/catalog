# Where catalogs go

A catalog helps an agent only if the agent has it when it starts work.
The daemon writes a `.catalog/catalog.md` into each folder where a
person has put a `.catalog/config.toml` file, and an instruction file
the agent already loads has to import it. This page explains where
catalogs go and why, what a folder tree needs before an agent can read
its catalog, how catalogs nest, why one daemon serves a whole Mac, why
it watches only folders that no cloud drive syncs, and what delivering
the catalog without a file would mean. The general rule that the catalog
never shows anyone more than they could already see is in the
[overview](overview.md#privacy-and-security); this page covers what it
means for where a catalog sits.

## Where agents start

The agent gets a catalog only if an instruction file it already loads,
such as `AGENTS.md`, imports `.catalog/catalog.md`. In a Git repository
both usually hold: developers start agents at the root, and they've
already put an instruction file there. Outside a repository we can't
count on either, and we designed the first catalog as if we could.

- People start agents wherever the task is: in a department's folder one
  day, in one project's folder the next, and at the top of everything
  when they don't know where something lives. Software can't tell which
  folders matter to them. A list of departments, or `~/src/west-arete`,
  looks like any other folder. We can't learn from the files where
  people will start. We can learn it only from people, if they tell us
  or if we watch where their sessions start.
- We'd rarely want a catalog to take more than about 10 to 20 percent of
  an agent's context, so a large tree will need several catalogs. The
  largest subfolders would get their own, and an agent could load one of
  those from the parent when it needs it. The same holds for relevance:
  an agent working in `finance/` rarely needs the catalog for
  `outreach/`.
- Agent tools load different instruction files. Claude Code loads the
  files in every folder above where it starts, and the files in
  subfolders as the agent reaches them. Codex reads from the project
  root down to where it starts
  ([research/agent-instruction-files.md](../research/agent-instruction-files.md)).
  If an instruction file high in a tree imports a catalog, one tool may
  load that catalog in every session below it and another tool in none.
- When an agent loads a catalog from a folder above it, the catalog's
  paths start from that folder, not from where the agent is.
- Someone has to add the line that imports the catalog, and a knowledge
  worker has never written an `AGENTS.md`. Each tool we read about also
  loads an instruction file for the person, such as
  `~/.claude/CLAUDE.md`, so that line may not need to be in every
  folder.
- Claude Cowork sees the folder tree through a mount, so it sees the
  same live `.catalog/catalog.md` files the daemon writes. We don't know
  yet what would load the catalog in Cowork, because Anthropic's
  documentation doesn't say whether Cowork reads an instruction file in
  the folder
  ([research/agent-instruction-files.md](../research/agent-instruction-files.md)).

Choosing what to profile and choosing where catalogs go are separate
decisions, and a person can make both.

- A person picks the folder trees to profile, and the parts of them to
  leave out, in `global.toml` and in each catalog's `config.toml`.
- Within those trees, a catalog goes in each folder where people tend to
  start agents. A person marks such a folder with a
  `.catalog/config.toml` file, which `catalog init` writes. The daemon
  crawls every watched folder, so it notices the new file and writes
  `catalog.md` beside it. The same file holds that catalog's settings,
  which add to the global ones. Only the file has to reach the daemon,
  so this works from any tool that can write to the folder, including
  Claude Cowork through its mount.
- The daemon maintains a catalog for each `.catalog/config.toml` it
  finds and creates none itself.
- Intended direction, not built: an agent suggests where catalogs should
  go, through the catalog skill. The agent can see the folders and where
  the person has been working with agents, so it's better placed to
  judge than any rule we could write into the daemon. It would put a
  `.catalog/config.toml` file, and the line that loads the catalog, in
  each place the person accepts.

A developer might ask the daemon to profile everything under `~/src` but
would never want a catalog there. Their agent would suggest the
repositories they actually work in, such as `~/src/acme/website` and
`~/src/home/recipes`. A knowledge worker who chooses
`~/Documents/Reports` might get one catalog in that folder.

### Subfolders shared more narrowly than their parent

A catalog shows the path and profile of every file it covers to anyone
who can read the catalog. Now and then a subfolder is shared with fewer
people than the folder above it. Say a People Operations team shares
`people/`, but only leadership can open
`people/management/disciplinary/`. A catalog in `people/` that covers
the disciplinary folder would show those files' paths and profiles to
the whole team. The daemon can't see how folders are shared, so whoever
places a catalog, a person or the catalog skill, has to weigh this. A
person can keep the narrower folder out of the wider catalog by giving
it a catalog of its own, which nesting makes the wider catalog leave
alone, or by listing it under `exclude` in the wider catalog's
`config.toml`. A catalog's `exclude` adds to the global and built-in
lists, so one line is enough, and the person's own global exclusions
keep applying
([Configuration](../reference/configuration.md#how-lists-combine)).

This matters once a catalog reaches people other than the one who runs
the daemon. By our reasoning, that happens in these cases.

- Someone commits a catalog to a repository whose readers can't open
  some of the folders it covers, such as a clone of another repository
  with fewer collaborators, or files Git ignores. The repository's
  readers then get those paths and profiles.
- A cloud drive syncs the catalog's folder to everyone it's shared with,
  including people who can't open one of its subfolders.
- A central server that delivers catalogs to other people would face it
  too. That server isn't built.

In the first two cases the daemon can't tell when it happens. On the
Mac, `catalog.md` is readable only by its owner, but a commit or a drive
copies its text to people who never see that mode.

## Nesting

Catalogs can nest. The nearest catalog above a subfolder covers it, and
any catalog higher up leaves that subfolder alone. The daemon does this
on each crawl: when it lists the files a catalog covers, it skips every
subfolder that holds a `.catalog` folder of its own, whether or not that
folder holds a `config.toml`. So a subfolder that a person set apart
with a `.catalog` folder stays out of the wider catalog before they add
a `config.toml`, or if they set it up with a version that didn't need
one.

We haven't decided whether or how nested catalogs link to each other,
whether a person links them by hand or the daemon does. That includes
how a catalog that grows too large might split; nobody's catalog has
reached that size yet. See the open questions below.

## One daemon per Mac

One daemon serves the whole Mac, and it always catches up eventually. A
person lists the folders they want watched in one `global.toml` on their
machine, and later in a menu bar app (not built). The daemon never
searches the rest of the machine for configuration. It writes a
`.catalog/catalog.md` for an instruction file in the folder to load, so
the agent receives the same thing as before. We use `AGENTS.md` as the
convention, because most agents read it and nothing about the catalog is
specific to Claude. A Claude Code user can import `AGENTS.md` from
`CLAUDE.md`.

## What a tree needs

Someone has to put three things in place before an agent can read a
tree's catalog. The daemon creates none of them, since its Publisher
writes only `.catalog/catalog.md`.

- The folder sits inside a tree the person chose to watch in their
  settings.
- A `.catalog/config.toml` file marks where the catalog goes, and holds
  that catalog's settings. `catalog init` writes it, with comments that
  set nothing, or a person creates it, even empty. The daemon writes
  `catalog.md` beside it.
- An instruction file the agent loads has a line that imports
  `.catalog/catalog.md`. We use `AGENTS.md` as the convention, since
  most agents read it. By default, Claude Code reads `AGENTS.md` only
  when there's no `CLAUDE.md` in the folder or above it, so a person who
  has both imports `@AGENTS.md` from `CLAUDE.md`
  ([research/agent-instruction-files.md](../research/agent-instruction-files.md)).

A file marks a catalog, not an empty folder, because a marker should say
what it marks. A person who comes across an empty `.catalog/` folder
can't tell what it does or that it matters. A file named for the
catalog's settings, holding comments that explain them, tells a person
who opens the folder what it is and what they can change there. A
project's root is often marked the same way: `go help go.mod` says a Go
module version "is defined by a tree of source files, with a go.mod file
in its root." A file also travels with a Git repository, where an empty
folder doesn't: `git add` records nothing for an empty `.catalog/`
folder, so a clone wouldn't have it. A `.catalog/` folder without a
`config.toml` isn't a catalog; the daemon stops updating a `catalog.md`
left in one, and doesn't delete it.

People who keep a catalog's folder in Git will want Git to ignore
`.catalog/catalog.md`, since that's the file that used to conflict, and
it can name and describe private files. When the folder is in a
repository, `catalog init` writes a `.catalog/.gitignore` that does
that, with no setup outside the repository. A pattern that starts with
`/` is relative to the folder that holds the `.gitignore`
([gitignore documentation](https://git-scm.com/docs/gitignore)), so
`/catalog.md` matches only that catalog's own `catalog.md`. Each nested
catalog needs its own `.gitignore`, which `catalog init` writes when run
in that folder. Outside a repository, `catalog init` writes no
`.gitignore`.

`catalog init` puts the config file in place and prints the line the
instruction file needs; a person adds that line by hand. Intended
direction, not built: the catalog skill would put all three in place,
with the person's approval, and a menu bar item could do the same later.

## Cloud drives

We would like to store the daemon's state in a way that works with cloud
drives such as Google Drive and iCloud Drive. This is a wish, and it
doesn't drive the design. The cases are less about Git repositories and
more about non-technical knowledge workers. Imagine someone who keeps
their work files in iCloud Drive so they can move between their office
machine during the day and their laptop in the evening. Keeping two
copies of the profiles for every file, one on each machine, would feel
wasteful. If we found a safe way to store the profiles in hidden files
on the cloud drive, the person could switch machines without waiting for
a second set of profiles.

### Why to watch only folders that no cloud drive syncs

The daemon can't write `.catalog/catalog.md` into a synced folder. Say
Alice and Bob share a folder through a cloud drive and both run the
daemon. Alice's daemon writes `.catalog/catalog.md`, the drive copies it
to Bob, and Bob's daemon rewrites it. In a repository, Git's ignore rule
keeps `.catalog/catalog.md` on one machine, but a cloud drive has no
such rule. If two machines keep writing the same synced file, people get
the conflicts the daemon exists to end, in a place with no merge tool. A
synced folder also carries the sharing risk described above, because the
drive delivers the catalog to everyone the folder is shared with.

So the daemon should refuse a synced folder when someone lists it. It
doesn't yet: it can't tell a synced folder from any other, so a person
has to watch only folders that no cloud drive syncs, such as `~/src`.
Detecting a synced folder is intended direction, not built. We could
serve those folders only by delivering the catalog without a file, for
example over [MCP](#delivering-the-catalog-without-a-file). For now this
leaves out many knowledge workers, who often keep their files in cloud
drives.

This version hasn't been made to read the folders macOS protects, so
don't watch Desktop, Documents, Downloads, or iCloud Drive. Howard
Oakley lists those among the locations macOS protects from apps reading
them
([Oakley, 2026](https://eclecticlight.co/2026/04/07/privacy-protected-folders/)).
We infer that the daemon will need an app bundle before it can read them
without trouble, which isn't built. The daemon runs as a bare launch
agent today, and nobody has tested what that can read.

## Delivering the catalog without a file

An MCP server is another way to hand an agent the catalog. The agent
asks for the catalog for its current path, so nobody has to write
anything into the tree, tell Git to ignore it, or load it from
`AGENTS.md`. The agent also gets a way to tell the daemon something
back. None of this is built; the daemon has no MCP server.

The two ways differ most in one respect. With the file, the agent has
the catalog in front of it before it decides anything. An MCP server can
send the agent short instructions when a session starts, but the agent
still has to fetch the catalog, and it now knows the catalog is a tool.
We haven't measured how reliably agents fetch it.

For now we'd keep the file as the main way to deliver the catalog and
use MCP as a second way. MCP would serve content that has no folder, and
a central server, which has no other way to deliver.

### What we make of it

[What Claude's documentation says about MCP](../research/mcp.md) records
the sources. This is our reasoning, and we haven't built anything to
test it.

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
  profiles in the [overview](overview.md#open-questions).
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
Cowork, that means a server reachable from the public internet, as the
MCP note quotes. A daemon in a container on a laptop doesn't need MCP,
because it writes `.catalog/catalog.md` into the mounted folder.

## Open questions

- Where should catalogs go, and who adds the line that loads each one? A
  `.catalog/config.toml` file marks a catalog, and the daemon never
  creates one. What's still open is who adds the import line, what loads
  each catalog once it's in place, and whether nested catalogs link to
  each other.
- How do people put what a tree needs in place? `catalog init` writes
  the config file, but a person still adds the import line by hand.
- What happens when a source tree has too many files for one flat
  `.catalog/catalog.md`? We could write entries for folders, or one
  catalog file per folder. Related work is tracked in
  [#3](https://github.com/westarete/catalog/issues/3), which sizes each
  entry by its worth to an agent, and
  [#4](https://github.com/westarete/catalog/issues/4), which profiles
  source code and configuration files.
- Is an MCP server worth building, and what would an agent usefully
  report through one?
