# Overview of catalog

Catalog keeps a `.catalog/catalog.md` file in a folder that tells an AI
agent which files to open for the task at hand, so the agent doesn't
have to read them all. A background daemon on your Mac keeps that file
current, and it does so without showing anyone more than they could
already see. This page says what catalog is for, how it protects the
people whose files it reads, who it's for, the problem it solves, and
what an agent can count on from it.

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
in this project, whether or not a doc mentions them. Each doc in this
folder also covers the privacy and security of its own topic, where
someone making that decision will read it. This section covers what
applies to all of catalog.

### What the catalog protects

The catalog must never show anyone more than they could already see. A
profile is a copy of what its document says, so only people who can read
the document may read the profile.

That holds for every copy the catalog makes, and any new feature may
make more. Some of the copies the daemon makes now:

- each file it profiles, and the profiles of its neighbors (profiling a
  file in light of its neighbors is intended direction, not built), sent
  to the model provider the person chose
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
Alongside the limits, we keep working toward goals we never mark as met.
None of these is built yet.

- clearer ways for a person to learn what a model provider keeps of what
  we send it and whether it trains on it, so they can choose a provider
  that suits the content
- more confidence that a profile doesn't repeat a secret from its
  document, such as a password or an account number; that means
  filtering or redacting content before the model sees it, which depends
  on the kind of document and is still to be designed
- stronger checks for security vulnerabilities in what we build

The day we say we've covered privacy or security is the day we start to
fail at it.

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

## The problem catalog solves

Agents do a poor job of deciding which files to load once a folder holds
more than a few dozen. They load files they don't need and miss the ones
they do. A catalog entry says when a file is worth opening, so the agent
loads what the task needs and leaves the rest.

The earlier command-line tool kept its catalog in the repository, and
that caused two troubles. The catalog fell behind when nobody ran
`update`, and it conflicted in merges.

- The SQLite database was binary, so Git reported a conflict whenever
  both sides changed it. We tested this with real Git: two branches each
  changed different documents and ran `update`, and they conflicted on
  `.catalog/catalog.db`, in a merge and in a rebase.
- `.catalog.md` conflicted when two sets of changes touched nearby
  entries.
- Conflicts could also happen without branches. If Alice and Jane both
  make local commits to the same branch, whoever pushes second shouldn't
  get stuck on a catalog conflict.
- Wall-clock time matters, not money. The model is cheap, and cheaper
  options may exist. Nobody should have to wait 15 minutes after cloning
  a repository before the catalog starts working, and someone importing
  many documents shouldn't have to sit and wait while the tool profiles
  each one.

## No state in Git by default

For people using the catalog, the main difference is that they no longer
commit `.catalog/catalog.md` files to Git. Nobody checks the catalog
files into Git. Git ignores them, or they live outside the repository,
so they have nothing to conflict on. The daemon keeps those files
current instead. It also keeps its own state outside every folder it
serves, in `~/Library/Application Support/com.westarete.catalog`. Since
none of that changing content is in Git, it can't cause merge conflicts.
We care most about the experience people have, and this improvement is
worth working hard for.

A person who keeps a catalog's folder in a Git repository will probably
want Git to ignore `.catalog/catalog.md` themselves, since that's the
file that used to conflict. The daemon doesn't set that up.

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
  profiled each entry, because the agent has nowhere else to ask. Today
  an entry is either summarized or marked "Not summarized yet." and
  shows only the start of the file; a deeper level that compares a file
  with its neighbors is intended direction, not built.
- The catalog needs no skill or setup on the agent's side, so an agent
  can use it anywhere it can see the folder and load the file. That
  includes Claude Cowork's mount, though we don't yet know what would
  load the file there
  ([research/agent-instruction-files.md](../research/agent-instruction-files.md)).
- Status, settings, and statistics are for people, and for tools that
  help people tune the catalog. None of them are part of what an agent
  sees. launchd, not an agent, restarts the daemon when it stops; a menu
  bar item that shows the daemon's state is intended direction, not
  built.

As a general rule, the catalog should reach an agent the same way the
content does. For files in a folder, that means a file in the folder.
Content that has no folder would need another delivery route, such as an
MCP server, which isn't built.

## Open questions

- Which staff are on a Mac? We assume macOS, and the daemon runs only
  there.
- How will we learn whether the catalog is helping, when agents never
  talk to the daemon? People could tell us. An agent could review past
  chat sessions with permission, but that's invasive, and we haven't
  settled on it.
