# A background daemon for the catalog: brief

This brief describes a design experiment: a background daemon that keeps
the catalog current. We've decided which platforms the daemon runs on
and which languages we'll write it in, but most of the rest is still
open. The brief covers the problems that make people need a catalog, the
architecture we're working toward, and what we still don't know. It grew
out of the OPE-54 conversation in Linear.
[architecture.md](architecture.md) outlines the components that would
make up the daemon.

## What the catalog is for

When an agent starts work in a folder tree with more than a few dozen
files, whether or not it's a Git repository, the catalog tells the agent
which documents matter for the task, so it doesn't have to read them
all, and it does that without showing anyone more than they could
already see.

That promise has consequences for the people involved.

- Nobody, staff or agent, should have to look at the catalog or maintain
  it. We want it to be as invisible as possible.
- An agent only ever reads the catalog. It finds a `.catalog/catalog.md`
  that its instructions already load. It never has to update the
  catalog, talk to the daemon, or know the daemon exists. That keeps
  setup small, gives the agent no skill to learn, and lets the catalog
  work wherever the folder is visible.
- People should get the same experience everywhere: in a plain folder,
  in a Git repository on `main` or on a branch, in Claude Cowork, and on
  GitHub.
- A missing catalog should do no harm. Most people use no catalog today.
  Their agents explore with tools that read and write single files well
  but are poor at finding meaning across many. Where there's no catalog,
  such as in a fresh clone on a machine with no daemon, people get that
  same experience and lose nothing.

We want a person to install the catalog, point it at the folders they
work in, and find that their agents everywhere get better without anyone
telling the agents why.

## Privacy and security

People will trust the catalog with their documents only if it keeps them
safe, so privacy and security are part of what the catalog is for. They
belong in every design, every doc, every piece of code, and every review
in this project, whether or not a doc mentions them.

### What the catalog protects

The catalog must never show anyone more than they could already see. A
profile is a copy of what its document says, so only people who can read
the document may read the profile.

That holds for every copy the catalog makes, and any new feature may
make more. Some of the copies the daemon makes now:

- each file it profiles, and the profiles of its neighbors, sent to the
  model provider the person chose, whether hosted or local
- every profile, in the daemon's state folder
- every file profiled and every error, in the journal and the log
- each catalog's paths and profiles, in `.catalog/catalog.md` inside the
  folder
- the paths and profiles that each agent loading `.catalog/catalog.md`
  sends to its own model provider, which may be a different one; the
  daemon writes the file so that agents will load it, so we count that
  copy as ours

File paths count as much as profiles. A catalog lists every path it
covers, and a file name such as
`people/management/disciplinary/2026-03-02-alex-doe-termination-preparation.md`
says nearly as much as the document does.

The catalog protects everyone whose information is in the files: the
person who runs it, and also the colleagues, clients, and partners whose
words sit in those files. Who should see a document is whatever the
people who shared its folder intended. The catalog can't see that
intent, so a person has to judge it.

Security matters just as much. The catalog must not open a hole in the
machine it runs on, and nobody should be able to break into the daemon,
or watch what it does, to gain access they don't already have.

### Questions for every step

Nobody can list every threat in advance, so we don't protect people by
forbidding a list of things. At every step, whether a design, a doc, a
line of code, or a review, we ask:

- Does this keep information away from people and services that
  shouldn't have it, including in profiles, paths, logs, and the
  journal?
- Could this create a security vulnerability?
- Could someone break into the daemon, or watch its work, to gain access
  they don't already have?

### Limits, and goals we never reach

Privacy and security aren't traded against quality, size, and cost. They
set the limits inside which we trade those three. Each proposal says
what it does to privacy and security, the same way it says what it does
to quality, size, and cost ([CLAUDE.md](../../CLAUDE.md), "Balance
quality, size, and cost"). When a change doesn't affect them, the
proposal says so, and a reviewer checks that claim.

A limit is a concrete definition, so it always covers less than we need.
Alongside the limits, we keep working toward goals we never mark as met:

- stronger assurance that a model provider keeps nothing we send it and
  trains on none of it
- more confidence that we filter confidential information before it
  reaches a model
- stronger checks for security vulnerabilities in what we build

The day we say we've covered privacy or security is the day we start to
fail at it.

### What we assume

We know of these assumptions, and we expect to find more. Each one is
work still to do, not a reason a leak isn't our problem.

- **No model provider keeps or trains on what it receives.** That
  includes the provider the daemon sends documents to and the providers
  behind the agents that read catalogs, whichever providers those are. A
  model that runs on the person's own machine sends nothing away, so for
  it the question is what it keeps on that machine. We haven't yet
  written down how a person would judge a provider by this measure. The
  provider the first version uses documents keeping API inputs and
  outputs for up to 30 days, and longer in some cases, so the keeping
  half doesn't yet hold for that version
  ([what it says](../superpowers/specs/2026-11-15-daemon-mvp-design.md#what-anthropic-says-about-keeping-and-training)).
- **No subfolder is shared more narrowly than the folder above it.** Say
  a People Operations team shares `people/`, but only leadership can
  open `people/management/disciplinary/`. If the catalog in `people/`
  covers the disciplinary folder, everyone on the team can read those
  files' paths and profiles. We expect this to be uncommon, since people
  usually expect access to a folder to include its subfolders, but it
  can happen, and the daemon can't tell when it does.

The subfolder problem appears whenever a catalog reaches people other
than the one who runs the catalog. By our reasoning, that happens if a
folder holds a clone of another repository with fewer collaborators, or
files that Git ignores, and the catalog in the outer folder is committed
to the outer repository: the outer repository's readers then get those
paths and summaries. It happens too if a cloud drive that syncs the
catalog's folder delivers the catalog to everyone that folder is shared
with. A central server, or an MCP server that delivers catalogs to other
people, would face it too.

A profile can also repeat a secret from its document, such as a password
or an account number. Preventing that means filtering or redacting
content before the model sees it, which depends on the kind of document.
It's still to be designed.

## Who it is for

- Non-technical staff, who will often work in Claude Cowork and may use
  GitHub's merge button. When they hit a binary merge conflict, they
  have no way forward.
- Agents, who work on branches the same way people do.
- Whoever maintains the `catalog` tool and the repositories.

Developers are one audience among many, and Git is one tool among many.
People use word processors, lab software, build tools, and Git, and each
of these apps leaves its own files in a folder beside the content people
care about. Some of those files are copies of that content, such as
Git's worktrees or a word processor's backups. We don't want any part of
the core design to assume a folder is a Git repository. We still have to
tell content apart from the files an app made for itself, and any rule
we write for one app favors one audience.

## What hurts today

We have used today's catalog for about half a year and can't think of a
time it held wrong content. The trouble is that it falls behind when
nobody runs `update`, and that it conflicts in merges.

- The SQLite database is binary, so Git reports a conflict whenever both
  sides changed it. We tested this with real Git: two branches each
  changed different documents and ran `update`, and they conflicted on
  `.catalog/catalog.db`, in a merge and in a rebase.
- `.catalog.md` conflicts when two sets of changes touch nearby entries.
- Conflicts can also happen without branches. If Alice and Jane both
  make local commits to the same branch, whoever pushes second shouldn't
  get stuck on a catalog conflict.
- Wall-clock time matters, not money. The model is Haiku, which is
  cheap, and cheaper options may exist. Nobody should have to wait 15
  minutes after cloning a repository before the catalog starts working,
  and someone importing many documents shouldn't have to sit and wait
  while the tool profiles each one.

## The experiment

We offer this as one example of how to approach the problem, not as a
plan.

- Nobody checks the catalog files into Git. Git ignores them, or they
  live outside the repository, so they have nothing to conflict on.
- A daemon keeps the catalog current. It watches the files in the tree,
  and when they change, it reads and profiles the new versions. It
  always catches up eventually.
- The daemon uses a language model that's cheap and
  [protects privacy](#privacy-and-security). That might be a local
  open-weight model on the laptop, or a trusted service that costs close
  to nothing.
- One daemon could serve the whole Mac. A person lists the folders they
  want profiled in one config on their machine, and later in a menu bar
  app. The daemon never searches the rest of the machine for
  configuration. It still writes a `.catalog/catalog.md` for an
  instruction file in the folder to load, so the agent receives the same
  thing as before. We use `AGENTS.md` as the convention, because most
  agents read it and nothing about the catalog is specific to Claude. A
  Claude Code user can import `AGENTS.md` from `CLAUDE.md`.
- Catalogs can nest. The nearest catalog above a subfolder covers it,
  and any catalog higher up leaves that subfolder alone.
- `catalog update` becomes a simple way to ask the daemon to give the
  current tree priority. People use it; agents don't need it.
- Claude Cowork sees the folder tree through a mount, so it sees the
  same live `.catalog/catalog.md` files the daemon writes. We don't know
  yet what would load the catalog in Cowork, because Anthropic's
  documentation doesn't say whether Cowork reads an instruction file in
  the folder
  ([research/agent-instruction-files.md](../research/agent-instruction-files.md)).

## No state in Git by default

For people using the catalog, the main difference is that they no longer
commit `.catalog/catalog.md` files to Git. The daemon keeps those files
current instead. It also keeps its own state outside every folder it
serves, in `~/Library/Application Support/com.westarete.catalog`. Since
none of that changing content is in Git, it can't cause merge conflicts.
We care most about the experience people have, and this improvement is
worth working hard for.

## Daemon state

Nobody commits the daemon's state, so we can store it however suits the
daemon, and it doesn't need to work well with Git. The daemon has only
two obligations: to keep the `.catalog/catalog.md` convention it shares
with agents, and to keep its state out of the folders it serves.

With its own folder under
`~/Library/Application Support/com.westarete.catalog`, the daemon can
use whatever storage works best. It could still use a SQLite database.
Since a pile of files there won't clutter anyone's repository, it could
also store each profile in a file of its own, much as Git stores its own
data. That may or may not be a good design. The point is that we no
longer have to work within the constraints of the person's repository.

## A side challenge: cloud drives

While we're thinking about the daemon's state, we'd like to keep one
item on a wish list: storing that state in a way that works with cloud
drives such as Google Drive and iCloud Drive.

These cases are less about Git repositories and more about non-technical
knowledge workers. Imagine someone who keeps their work files in iCloud
Drive so they can move between their office machine during the day and
their laptop in the evening. Keeping two copies of the profiles for
every file, one on each machine, would feel wasteful. If we found a safe
way to store the profiles in hidden files on the cloud drive, the person
could switch machines without waiting for a second set of profiles.

This is an edge case, though, and it doesn't drive the design.

We can't write `.catalog/catalog.md` into a synced folder. Say Alice and
Bob share a folder through a cloud drive and both run the daemon.
Alice's daemon writes `.catalog/catalog.md`, the drive copies it to Bob,
and Bob's daemon rewrites it. In a repository, Git's ignore rule keeps
`.catalog/catalog.md` on one machine, but a cloud drive has no such
rule. If two machines keep writing the same synced file, people get the
conflicts the daemon exists to end, in a place with no merge tool. So
the daemon should refuse a synced folder when someone lists it. We could
serve those folders only by delivering the catalog without a file, for
example over MCP. For now this leaves out many knowledge workers, who
often keep their files in cloud drives.

## Our new struggle is wall-clock time

For us, the people designing the daemon, the tradeoff is how long
profiling takes. If the daemon moves too slowly, content that a person
expects to be indexed won't be, their agents may seem clueless, and the
person will feel the catalog does nothing for them.

The urgency may not be as bad as we imagine. Large amounts of content
arrive rarely, say from a clone or a big download, so people will come
to expect new content to take a little while to show up.
`catalog status` can show them progress for the tree they're in.

When an agent generates a lot of content itself, there's no special
urgency, because the agent already knows what it created.

The daemon can also move too fast. We don't want it hovering over
someone who's editing a file, wastefully re-indexing the whole file on
every small edit and auto-save. We don't yet know how to strike that
balance.

We've traded a struggle with conflicting Git state for a struggle with
speed and cache expiration, and we accept that trade gladly. Merge
conflicts always hurt. Waiting for the catalog to update before you can
commit always hurts. Seeing catalog state in your Git history always
hurts. This design gives people a much better experience, and our first
attempts to profile new files quickly will probably be good enough.

## Time vs quality vs cost

We care a lot about the quality of each profile. In as few tokens as
possible, we want to tell an agent when to read a file and when not to,
compared with other files. So our struggle with wall-clock time has
several parts.

- We want to shorten the time between when a new file needs a profile
  and when we start profiling it. We still don't know how to tell when a
  file needs one.
- We want profiling itself to take less time.
- We value
  [the privacy a language model gives us](#privacy-and-security), and
  more privacy often costs speed, or battery on a laptop.
- A profile's quality doesn't stand alone. It also depends on how the
  agent chooses between that file and its peers: not just whether to
  load file A, but whether to load A, B, or C when all three look
  similar at first glance.
- We care somewhat about what profiling costs. It can't be prohibitively
  expensive, but we'll accept some cost if that lets time, quality, and
  battery work out well.

Given all this, we should probably let people choose the model. That
means not just models in the Anthropic API, as today's tool allows, but
a range of models, open or proprietary, hosted or local. A person should
be able to switch models in the daemon's settings without anything else
changing.

We might go further and let people choose the model for each folder. A
folder's entry in the daemon's settings might recommend a model, or even
customize the prompt. People will want to make different tradeoffs among
privacy, speed, price, and quality depending on the content.

## Profiles deepen over time

Today's tool profiles a file all or nothing. Either a language model has
read the whole file and written a complete profile, or the catalog knows
nothing about the file. Since time and cost are what we struggle with,
we see room for something in between. Picture three levels of profiling.

- The daemon can produce the bottom level almost at once, at almost no
  cost. It lists each file with its type, maybe some metadata from the
  file system, and its first several lines or first 512 bytes. Every
  profile starts here, and even this helps an agent far more than
  nothing.
- At the middle level, a language model has read some of the content, so
  the profile can begin to summarize the file and say when an agent
  might choose it.
- At the top level, we also understand how the file compares with its
  peers. Peers could be neighboring files in the same folder, or files
  with similar names elsewhere in the tree that might compete with it.
  This level is slow and expensive, but the quality is very high.

The catalog always knows something about every file in its scope, and
that knowledge deepens over time. The daemon can work through its whole
scope while balancing speed, cost, and quality. New content shows up
almost at once, even when understanding it well takes longer. An agent
should be able to tell how deeply the daemon has profiled each file, so
it knows how far to trust the entry.

## More than documents

We designed the catalog for knowledge work, and that's still the main
case. Today's tool already profiles CSV files alongside prose, and the
same technique should work just as well for research data and source
code. We should plan to profile any kind of file a model can understand,
images included. We don't intend to build all of that at first, but the
design shouldn't assume every file is a prose document.

## Where agents start

A catalog helps only if an agent has it when it starts work. The agent
gets it only if an instruction file it already loads, such as
`AGENTS.md`, imports `.catalog/catalog.md`. In a Git repository, both
usually hold: developers start agents at the root, and they've already
put an instruction file there. Outside a repository we can't count on
either, and we designed the first catalog as if we could.

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

Choosing what to profile and choosing where catalogs go are separate
decisions, and a person can make both.

- A person picks the folder trees to profile, and the parts of them to
  leave out.
- Within those trees, a catalog goes in each folder where people tend to
  start agents. A person, or an agent working for them, marks such a
  folder by creating a `.catalog/` folder in it, even an empty one. The
  daemon already crawls every profiled tree, so it notices the new
  folder and writes `catalog.md` inside it. The `.catalog/` folder can
  also hold a `config.toml` with settings for that catalog, which
  override the global ones. Only the folder has to reach the daemon, so
  this works from any tool that can write to the folder, including
  Claude Cowork through its mount.
- The daemon maintains a catalog in each `.catalog/` folder it finds and
  creates none itself.
- An agent suggests where catalogs should go, through the catalog skill.
  The agent can see the folders and where the person has been working
  with agents, so it's better placed to judge than any rule we could
  write into the daemon. It creates a `.catalog/` folder, and the line
  that loads the catalog, in each place the person accepts.

A developer might ask the daemon to profile everything under `~/src` but
would never want a catalog there. Their agent would suggest the
repositories they actually work in, such as `~/src/acme/website` and
`~/src/home/recipes`. A knowledge worker who chooses
`~/Documents/Reports` might get one catalog in that folder.

We haven't decided whether or how nested catalogs link to each other,
whether a person links them by hand or the daemon does. That includes
how a catalog that grows too large might split; nobody's catalog has
reached that size yet. We also haven't settled what loads each catalog
once it's in place.

## Statistics for tuning

People will want to understand how the catalog is working and tune it.
The daemon should keep a record of what it did: how long each file took
to profile, with which model, what it cost, and what went wrong.

We don't plan to build much on that record at first. Later, we could use
it to make recommendations. For example, a Claude skill could read the
statistics, notice that a set of files is slow or hard to profile, and
suggest adding a five-line comment at the top of each one.

## OS support

We should use each operating system's strengths wherever we can. On
macOS, the daemon can use FSEvents to detect changes. We might even be
able to use language models that come with the operating system, such as
Apple Intelligence; we haven't looked into it yet.

All of our users are on macOS today, so the Mac comes first. The daemon
also runs on Linux in two ways: on a headless server that keeps a
catalog current as a central source, and in a container on a Mac or
Windows laptop.

We don't want to rule out Windows, because knowledge workers using
Copilot on Windows could become our largest audience. We hope the Go
core carries over to Windows. If it can't, we'd write a separate
implementation that honors the same contract with agents.

We deliberately kept our contract with agents very simple, so that
separate implementations for each platform would be possible. We now
think a shared core with a thin layer for each operating system is the
better way, because the quality of a catalog depends on the shared part.
We'll write the shared core in Go and the Mac's native parts in Swift;
"Platforms and language" in [architecture.md](architecture.md) explains
why.

## Delivering the catalog without a file

An MCP server is another way to hand an agent the catalog. The agent
asks for the catalog for its current path, so nobody has to write
anything into the tree, tell Git to ignore it, or load it from
`AGENTS.md`. The agent also gets a way to tell the daemon something
back.

The two ways differ most in one respect. With the file, the agent has
the catalog in front of it before it decides anything. An MCP server can
send the agent short instructions when a session starts, but the agent
still has to fetch the catalog, and it now knows the catalog is a tool.
We haven't measured how reliably agents fetch it.

For now we'd keep the file as the main way to deliver the catalog and
use MCP as a second way. MCP would serve content that has no folder, and
a central server, which has no other way to deliver. "Delivering the
catalog over MCP" in [architecture.md](architecture.md) has what the
documentation says and our reasoning.

## User interface

Each platform can also have its own user interface. Our first macOS
version may use only the command line and a config file. Later we'd
likely add `plist` entries, then a menu bar item, and then a settings
window. We could distribute it through Homebrew first and the Mac App
Store later. Each of these is a client of the same daemon, so adding one
shouldn't change how the daemon works. Because of macOS's rules for
protected folders, we may have to ship the daemon inside an app bundle
sooner than this order suggests; see "Processes and macOS" in
[architecture.md](architecture.md).

Wherever we show status, a person should be able to see at a glance
which folders are cataloged and which aren't. Much later, the menu bar
item might notice someone working in a folder with no catalog and offer
to add one.

On Windows and Linux, we'd build user interfaces that suit each
environment.

## Open questions

- A new clone starts with a mostly empty catalog, and that's expected.
  But what if nobody notices that it never fills in? If a status view
  listed which folders are cataloged, a person could tell a delay from a
  folder that will never be indexed. We haven't designed it.
- If the daemon stops, the catalog falls behind again, and agents
  shouldn't have to notice. launchd can restart the daemon and the menu
  bar item can show its state, but we haven't worked out the details.
- Which documents should the daemon profile first? Should it put certain
  new files first, especially when many arrive at once?
- Does the daemon need to know which files are in use, or which folders
  have active processes?
- Which staff are on a Mac? The experiment assumes macOS.
- Is an MCP server worth building, and what would an agent usefully
  report through one?
- Which models, local or hosted, could we use, and how good are their
  profiles compared with Haiku's?
- What frameworks does macOS offer today for watching file changes and
  for running language models on the device?
- What happens to repositories that already have a catalog from today's
  tool, and how do people move them over?
- How will we learn whether the catalog is helping, when agents never
  talk to the daemon? People could tell us. An agent could review past
  chat sessions with permission, but that's invasive, and we haven't
  settled on it.
- Where should catalogs go, and who adds the line that loads each one?
  See "Where agents start."
- How can the daemon recognize a synced folder so it can refuse it?

## Background

This idea grew out of OPE-54 in Linear, where I tried to deal with merge
conflicts. I got stuck. I could make conflicts easier to resolve, by
choosing "ours" or "theirs" by default and then updating the catalog,
but I couldn't make resolving them fully automatic, especially when
someone clicks the merge button on GitHub.

That much friction, for users and for the implementation, over something
so routine made me realize the problem went deeper. So I went all the
way back to the problem and reconsidered the whole range of solutions.
