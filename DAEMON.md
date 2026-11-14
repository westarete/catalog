# A background daemon for the catalog: brief

This is a brief on a design experiment. Nothing here is decided. It
describes the problems that drive the need for the catalog, one
architecture we are considering, and what we still don't know. It grows
from the OPE-54 conversation in Linear.

## What the catalog is for

When an agent starts work in a folder tree that contains more than a few
dozen files (whether it's a git repo or not), the catalog allows it to
already know which documents matter for the task, without reading them
all.

That promise has consequences for the people involved:

- Nobody, staff or agent, should have to look at the catalog or maintain
  it. It should be as invisible as possible.
- When the catalog is out of date, the agent should know how to keep it
  up to date and resolve issues on its own.
- It should feel the same everywhere: in a plain folder, in a git repo
  on `main`, on a branch, in Claude Cowork, on GitHub.

## Who it is for

- Non-technical staff, who will often work in Claude Cowork and may use
  GitHub's merge button. A binary merge conflict is a dead end for them.
- Agents, who work on branches the same way people do.
- Whoever maintains the `catalog` tool and the repos.

## What hurts today

Scott has used the catalog for about half a year and cannot think of a
time it held wrong content. What goes wrong is that it falls behind,
because nobody ran `update`, or that it conflicts in a merge.

- The SQLite database is binary and conflicts whenever both sides
  changed it. We tested this with real git: two branches that each
  changed different documents and ran `update` conflict on
  `.catalog/catalog.db`, in a merge and in a rebase.
- `.catalog.md` conflicts when changes land close together.
- Conflicts can also happen without branches. If Alice and Jane are
  making local commits to the same branch, the second one to push should
  not get stuck because of a catalog conflict.
- Wall-clock time matters, not money: the model is Haiku and cheap, and
  cheaper options may exist. Nobody should wait 15 minutes after cloning
  a repo for the catalog to begin work, and a person importing many
  documents shouldn't have to sit and block while each one is profiled.

## The experiment

Think of this as one example of how to approach the problem, not a plan.

- The catalog files are never checked into git. They are ignored or
  outside the repo, so there is nothing to conflict on.
- A daemon keeps the catalog current. It watches the files in the tree.
  When they change, it reads the new files and profiles them. It always
  catches up eventually.
- The daemon uses a language model that is cheap and that prioritizes
  privacy. That might be a local open-weight model on the laptop, or a
  service that is trusted and close to free.
- One daemon could serve the whole Mac. High level trees such as `~/src`
  are authorized at the daemon level, and then the presence of a
  `.catalog.toml` file (which can be checked into git) tells the daemon
  to index a particular subtree. It still generates `.catalog.md` which
  is referenced by that tree's `CLAUDE.md`. So the "product" that is
  delivered to the agent is the same.
- Catalogs can nest: a subtree is served by the catalog in the closest
  parent directory, and if there is another catalog above it, it leaves
  that subtree alone.
- `catalog update` is now a simple signal to the daemon to prioritize
  the current tree.
- Claude Cowork sees the folder tree through a mount, so it sees the
  same live `.catalog.md` files the daemon writes. Nothing special is
  needed.

## No state in git by default

From the user's perspective, the main difference is that now these
`.catalog.md` files are _not_ typically committed to git, and instead
are maintained "live". And the daemon's catalog state is kept outside of
all the directory trees, in
`~/Library/Application Support/com.westarete.catalog`. Because none of
the dynamic catalog content is kept in git, it is impossible for it to
cause merge conflicts. What we care most about is the user's experience,
and this is a huge win that is worth struggling for.

## Daemon state

The daemon is now liberated to use whatever system it wants to store its
state. It does not have to use something that is friendly to git,
because its state is never committed. The only contract that the daemon
has is the `.catalog.md` convention and interface to the agent, and its
promise not to store state in the folders that it's serving.

Since it has its own state directory under
`~/Library/Application Support/com.westarete.catalog`, we are free to
consider whatever state storage is most advantageous. It could still be
a SQLite database, but now that we are not concerned about filesystem
sprawl that would create noise in a git repo, we could also consider
storing each profile in an individual file in the filesystem, similar to
how git repo state works. That may or may not be a good design, but the
point is that we are free of the constraints of working within the
user's git repository.

## A side challenge: cloud drives

While we're considering daemon state, a "wish list" item that we could
keep in mind is whether it's possible to keep state in a way that's
compatible with cloud drives, such as Google Drive and iCloud Drive.

These use cases are less about git repositories and more about
non-technical knowledge workers. Imagine that someone keeps their work
files in iCloud Drive, so they work seamlessly between their office
machine during the day and their laptop in the evening. It would feel
wasteful to have two copies of the profiles for every file, one on each
machine. If there was a safe way to store the profiles in hidden files
on the cloud drive, that may provide valuable efficiencies in the user
experience when the user transitions from one machine to another.

This is an edge case though, and not a primary driver of the design.

## Our new struggle is wall-clock time

For us, the designers of the daemon, the tradeoff is the wall-clock time
needed to complete the profiling. If we move too slowly, content that
feels like it should have been indexed by now may not be, and agents may
feel clueless. In that case, the user feels like the catalog does
nothing for them.

At the same time, the urgency problem may not be as bad as we imagine.
Large amounts of content don't arrive very often (say from a clone or a
big download), so the user will gain an expectation that it takes a
little time for new content to get indexed. `catalog status` can now
give them a sense of progress for the tree that they're in.

For large amounts of content that the agent generated, there is no
special urgency. The agent is already aware of what it created, so
there's no special rush on its account.

But we can move too fast, too. We don't want to hover over someone who's
actively editing a file, and wastefully re-index the entire file for
every tiny edit and auto-save. How to strike this balance? This is an
unanswered question.

We have traded the struggle of conflicting git state for the struggle of
speed (and cache expiration). We happily accept this challenge because
merge conflicts always hurt; waiting for synchronous catalog updates
until you can commit always hurts; seeing catalog state in your git
history always hurts. This design will give the user a much nicer
experience, and our first attempts to profile new files fast enough will
probably be adequate.

## Time vs quality vs cost

We care quite a bit about the _quality_ of each profile. In as few
tokens as possible, we want to convey to agents when to read this file
or not, compared to other files. So our new struggle with wall-clock
time is multi-fold:

- We want to minimize the time between when a new file needs to be
  profiled (an open question of how to determine that), and when we
  begin to profile it.
- We want to minimize the time of profiling.
- We value the level of privacy that a given language model affords us,
  which can often be a tradeoff in terms of its speed (or battery usage
  in the case of laptops).
- The quality of a profile is not determined solely in isolation; it's
  also a function of how the agent chooses to load that file _relative
  to its peers_. Not just whether to load file "A"; but whether to load
  file "A" or "B" or "C", which all appear to be similar at first
  glance.
- We care somewhat about the monetary cost of profiling. It can't be
  prohibitively expensive. But we're willing to make tradeoffs here if
  it allows the other factors (time, quality, battery) to work well.

Given these factors, it would probably be worth making the model
configurable. Not just within the Anthropic API like today's
implementation, but across a variety of models, whether open or
proprietary, cloud-based or local.

Taking it a step further, we may consider making the model configurable
at the directory level. The `.catalog.toml` config file for a tree may
specify which model is recommended. It could even customize the prompt
that should be used. It makes sense that we would want to make different
tradeoffs in privacy, speed, price, quality depending on the content at
hand.

## Profiles deepen over time

Today profiling is all or nothing: either the full contents of a file
have gone through a language model and produced a complete profile, or
the catalog knows nothing about the file at all. Since time and cost are
our struggles, there are middle grounds. Picture a hierarchy of
profiling.

- The bottom level is extremely fast and cheap; it can be generated
  almost instantly. It lists the files, their types, maybe some
  additional metadata from the file system, and the first several lines
  or the first 512 bytes of content. This is what a profile contains at
  first, and it's significantly better than nothing for the agent.
- In the middle level, some portion of the content has been processed by
  a language model, so the profile can begin to summarize the file and
  explain when an agent might choose it.
- At the top level, we also understand the file's relevance among its
  peers. Peers could be neighboring files in the same directory, or
  similarly named files elsewhere in the tree that might compete with
  each other. This level is slow and expensive, but it gives extremely
  high quality.

The catalog always knows something about every file in its scope, and
that knowledge deepens over time. The daemon can work through its full
scope while balancing speed, cost, and quality, and new content becomes
visible almost at once even when understanding it well takes longer. An
agent should be able to tell how deeply each file has been profiled, so
it knows how far to trust the entry.

## OS support

It makes sense for the daemon design and implementation to leverage
unique operating system advantages wherever possible. On macOS we can
take advantage of FSEvents to detect changes. We may even be able to
leverage local LLM services that are native to the OS (Apple
Intelligence?).

100% of our users are on macOS, so we do not need to support Windows or
Linux. At the same time, we have deliberately kept our contract with the
agents incredibly simple. So there is nothing stopping us from creating
compatible, independent implementations for different platforms. We can
leverage what Windows' operating system provides, and what Linux
provides for those implementations.

## User interface

Likewise, the user interface can be platform specific. Our initial macOS
implementation may be completely CLI and config file based. But we would
likely progress into `plist` entries, and then a menu bar entry, and
then a GUI configuration. Distribution could happen via Homebrew, and
then via the Mac App Store.

Windows and Linux would be free to have user interfaces that are
idiomatic for their environment.

## Open questions

- A new clone starts with a mostly empty catalog. That is expected. But
  what if it fails to be noticed? How does anyone tell the difference
  between a delay in indexing, and whether it's ever going to be indexed
  at all?
- If the daemon stops, nobody sees it, and the catalog falls behind
  again. How might the catalog or agent say or know how current it is?
- Which documents do we profile first? Do we prioritize certain new
  files, especially when they arrive in a big batch?
- Do we need to detect and consider which files are "in use"? Or which
  directories have active processes?
- Which staff are on a Mac? The experiment assumes macOS.
- Which models, local or hosted, and how good are their profiles
  compared with Haiku?
- What macOS file-change and on-device language model frameworks are
  currently offered?
- What happens to repos that already have a legacy catalog? How are they
  migrated?

## Background

This concept grew out of OPE-54 in Linear, which tries to deal with the
problem of merge conflicts. I ran into a brick wall because while I
could make it easier to resolve conflicts (choose "ours" or "theirs" by
default, and then update the catalog), I couldn't make it fully
automatic, particularly when a user wants to click the "merge" button in
GitHub.

The user friction and implementation friction that I was experiencing
for such a routine occurrence caused me to realize that this was a much
more fundamental problem. That's what prompted me to go all the way back
to the problem space, and reconsider the entire solution space.
