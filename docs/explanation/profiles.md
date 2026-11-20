# Profiles

Each entry in a catalog is a profile: a short note, written by a
language model, that tells an agent what a file is and when to open it.
This page explains how the daemon makes profiles, why it trades time,
quality, and cost the way it does, what a person should weigh when
choosing a model, how a profile deepens over time, and which kinds of
files it can profile.

## What a profile is

A profile is two short sentences. The first says what the document is,
in a few words. The second says when to open it: the task or question
that should send an agent there. The model writes it the way a colleague
would say which file to grab, and it doesn't summarize the file. A wrong
profile is worse than an ugly one, because it makes the agent load the
wrong context or miss the right one, which is the failure the catalog
exists to prevent.

The daemon sends the model one file's content together with the profiles
of the other files in the same catalog, so the model can tell the file
apart from its neighbors. The prompt allows a pointer to another file
only between near-duplicates an agent could confuse.

A profile is a copy of what its file says, so the daemon applies the
same rule to it as to the file: only people who can read the file may
read the profile.
[Privacy and security](overview.md#privacy-and-security) in the overview
covers that rule for all of catalog.

## Wall-clock time

For us, the people designing the daemon, the tradeoff is how long
profiling takes. If the daemon moves too slowly, content that a person
expects to be indexed won't be, their agents may seem clueless, and the
person will feel the catalog does nothing for them.

The urgency may not be as bad as we imagine. Large amounts of content
arrive rarely, say from a clone or a big download, so people will come
to expect new content to take a little while to show up.
`catalog status` shows them progress for each catalog: how many files it
has, how many have a summary, and how many are waiting.

When an agent generates a lot of content itself, there's no special
urgency, because the agent already knows what it created.

The daemon can also move too fast. We don't want it hovering over
someone who's editing a file, wastefully re-indexing the whole file on
every small edit and auto-save. The daemon waits until a file has stayed
unchanged for 3 minutes before it profiles it, and it records each
summary's tokens and time in its journal so we can tune that. When
several files are ready, it profiles files with no summary first, so new
files aren't stuck behind a backlog of updates, and within each group
the file that changed earliest goes first.

We've traded a struggle with conflicting Git state for a struggle with
speed and cache expiration, and we accept that trade gladly. Merge
conflicts always hurt. Waiting for the catalog to update before you can
commit always hurts. Seeing catalog state in your Git history always
hurts. This design gives people a much better experience, and our first
attempts to profile new files quickly will probably be good enough.

## Time, quality, and cost

We care a lot about the quality of each profile. In as few tokens as
possible, we want to tell an agent when to read a file and when not to,
compared with other files. So our struggle with wall-clock time has
several parts.

- We want to shorten the time between when a new file needs a profile
  and when we start profiling it. The daemon crawls the watched folders
  every minute and treats a file as new or changed when its size or
  modification time differs from what it last recorded. We still don't
  know how to tell, beyond that, when a file needs a profile.
- We want profiling itself to take less time.
- We value [privacy](overview.md#privacy-and-security), and the more
  private choice of model often costs speed, or battery on a laptop.
- A profile's quality doesn't stand alone. It also depends on how the
  agent chooses between that file and its peers: not just whether to
  load file A, but whether to load A, B, or C when all three look
  similar at first glance.
- We care somewhat about what profiling costs. It can't be prohibitively
  expensive, but we'll accept some cost if that lets time, quality, and
  battery work out well.

### Choosing a model

The daemon has one model provider, and it calls that provider's model
named in `global.toml`. A person switches models by changing that
setting, and nothing else has to change. A new model gets a new set of
profiles: the daemon keeps a file's older profile in the catalog until
the new model has written its replacement, and it profiles files that
have no profile first.

Other providers are intended direction, not built. The model might be a
local open-weight model on the laptop, or a trusted service that costs
close to nothing. Given the tradeoffs above, we think people should
choose from a range of models, open or proprietary, hosted or local.
Which of them write profiles as good as the default model's is an open
question below.

We might go further and let people choose the model for each folder. A
folder's entry in the daemon's settings might recommend a model, or even
customize the prompt. People will want to make different tradeoffs among
privacy, speed, price, and quality depending on the content. That is
also direction, not built: today one model serves every catalog on the
Mac.

### What a provider keeps

Part of the privacy tradeoff is what a provider does with what we send
it: whether it keeps the content, for how long, and whether it trains on
it. A model on the person's own machine sends nothing away, though it
may still keep prompts or logs there. The right answer depends on the
content and on whose it is. Someone might accept a hosted model that
keeps requests for a few weeks for work they already trust that provider
with, run a local model for a client's documents, and use a free hosted
model that trains on what it reads for material that's already public.
The person makes that choice, for each kind of content; catalog doesn't
impose a rule. How the catalog shows them what each provider says it
does is still to be designed.

What the daemon sends is the text of one file, or a PDF whole, with its
path and the profiles of the other files in the same catalog. The agents
that read a catalog make the same kind of copy, since each one sends the
catalog's paths and profiles to its own provider.

The first version uses Anthropic's API, and
[its spec](../superpowers/specs/2026-11-15-daemon-mvp-design.md#what-anthropic-says-about-keeping-and-training)
quotes what Anthropic says it keeps and trains on.

## Profiles deepen over time

The earlier tool profiled a file all or nothing. Either a language model
had read the whole file and written a complete profile, or the catalog
knew nothing about the file. Since time and cost are what we struggle
with, the daemon profiles in levels.

- The daemon produces the bottom level almost at once, at almost no
  cost. It lists each file with its type, its size in bytes, and, for
  text files, its first lines, up to 10 lines or 512 bytes, whichever is
  shorter. Every profile starts here, and even this helps an agent far
  more than nothing. The catalog marks such an entry "Not summarized
  yet." so the agent knows how far to trust it.
- At the middle level, a language model has read the file and written
  the profile described above. This is built.
- At the top level, we would also understand how the file compares with
  its peers. Peers could be neighboring files in the same folder, or
  files with similar names elsewhere in the tree that might compete with
  it. This level would be slow and expensive, but the quality would be
  very high. It is intended direction, not built. The second pass below
  is the nearest thing the daemon does today.

The catalog always knows something about every file in its scope, and
that knowledge deepens over time. The daemon works through its whole
scope while balancing speed, cost, and quality. New content shows up
almost at once, even when understanding it well takes longer.

### The second pass

A profile is meant to tell a file apart from its neighbors, but the
model can only see neighbors that already have profiles. On a catalog's
first run, the files profiled first see few or none. The earlier tool's
`bootstrap` command addressed this with a second pass over every file.
The daemon does it without a command.

If another file in the catalog was still waiting for its first profile
when the daemon profiled a file's content for the first time, the daemon
marks that profile as partial in its recipe. A changed file counts the
same as a new one, since the model can't see any profile of its new
content. The daemon holds the update until no other file in that catalog
is waiting for its first profile, so the model sees the neighbors that
were missing.

- When someone saves two files a minute apart, the first is updated only
  after the second has a profile.
- While someone keeps editing a file, the updates in its catalog wait. A
  file that changes more often than every three minutes, such as a log,
  holds them for as long as it keeps changing, and status counts them as
  waiting.
- Copies of a file share one profile, so a copy in another catalog waits
  too; otherwise its update would see the wrong neighbors.
  `catalog daemon --once` sees only its own folder, so it can still
  update a copy there; we accept that for a maintenance tool.
- An identical copy doesn't count as a missing neighbor, since an update
  would show the model nothing new.
- The daemon updates a file only once for the same content: the second
  profile is never marked partial.

What the second pass costs:

- A new catalog of 102 files costs about 101 extra model calls, once, as
  `bootstrap` did.
- Ten files added together to a catalog with profiles for every other
  file cost nine extra calls. The first nine are updated, and the
  existing files aren't touched.
- One file added to such a catalog costs nothing extra.
- Twenty files changed by one `git pull` cost nineteen extra calls, the
  same as twenty new files.

A file that can't get a profile doesn't count as missing: a PDF the
daemon doesn't send, or a file with a failure on record. Otherwise, once
the model rejects one file, every new file in that catalog would cost
two calls. A file profiled earlier still won't point to a near-duplicate
that arrives later, as with the earlier tool's `update`. (Our call.)

A second pass gives the model a chance to tell a file apart from
neighbors it couldn't see; it doesn't guarantee it will. In the first
run with it, one entry gained a pointer to a related file it had missed,
and another still didn't mention a related file it had seen. Whether
that is worth about twice the calls and more than twice the tokens for a
new catalog is open; we'll judge it by whether agents find the right
files.

How much an entry should say about related documents is also open. A
pointer adds words to every request and to every agent's context, and
keeping pointers current would mean updating older entries when new
files arrive. Documents, file names, and Git already record some of
these relationships, but not all of them. For now the prompt allows a
pointer only between near-duplicates an agent could confuse.

## More than documents

We designed the catalog for knowledge work, and that's still the main
case. By default the daemon profiles Markdown, plain text, CSV, and PDF
files, and a person can change that list in the settings. A CSV or a
text file goes to the model as text, and a PDF goes whole.

The daemon limits what it sends. It cuts a text file at 400,000 bytes,
at a line boundary, and tells the model the rest was left out; a profile
needs only enough of a file to say what it is and when to open it, and a
larger file can pass the API's limit on a prompt. It sends no PDF larger
than 95,000,000 bytes, or an empty one, and that file stays a listing
until it changes.

The same technique should work just as well for research data and source
code, and we should plan to profile any kind of file a model can
understand, images included. We don't intend to build all of that at
first, but the design shouldn't assume every file is a prose document.
Profiling source code is tracked in
[issue #4](https://github.com/westarete/catalog/issues/4). Images aren't
built.

## Open questions

- Which models, local or hosted, could we use, and how good are their
  profiles compared with Haiku's, the default model today?
