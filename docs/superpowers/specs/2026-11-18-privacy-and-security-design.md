# Privacy and security in catalog: design

Catalog exists so that a language model can read people's documents and
tell their agents which ones to open. People will trust it with those
documents only if it keeps them safe, so privacy and security are part
of catalog's mission. They belong in every design, every doc, every
piece of code, and every review. This design describes how we think
about them and proposes where to write that into the mission first.

## What catalog protects

Catalog must never show anyone more than they could already see. A
summary is a copy of its document, so only people who can read the
document should be able to read the summary.

That holds for every copy catalog makes, and catalog makes copies in
many places; any new feature may make more. Some of the copies the
daemon makes now:

- each file it summarizes, and the summaries of its neighbors, sent to
  the model provider the person chose, which in the first version is
  Anthropic
- every summary, kept in `store.db`
- every file summarized and every error, recorded in `journal.jsonl` and
  the log
- each catalog's paths and summaries, written to `.catalog/catalog.md`
  inside the folder
- the paths and summaries that each agent loading `.catalog/catalog.md`
  sends to its own model provider, which may not be Anthropic; the
  daemon writes the file so that agents will, so we count that copy as
  ours

File paths count as much as summaries. A catalog lists every path it
covers, and a file name such as
`people/management/disciplinary/2026-03-02-alex-doe-termination-preparation.md`
says nearly as much as the document does.

Catalog protects everyone whose information is in the files: the person
who runs it, and also the colleagues, clients, and partners whose words
sit in those files. Who should see a document is whatever the people who
shared its folder intended. Catalog can't see that intent, so a person
has to judge it.

Security is the other half. Catalog must not open a hole in the machine
it runs on, and nobody should be able to break into the daemon, or watch
what it does, to gain access they don't already have.

## Questions for every step

No list of threats can be complete, so we don't protect people by
forbidding a list of things. We ask these questions at every step, in a
design, a doc, a line of code, or a review:

- Does this keep information away from people and services that
  shouldn't have it, including in summaries, paths, logs, and the
  journal?
- Could this create a security vulnerability?
- Could someone break into the daemon, or watch its work, to gain access
  they don't already have?

## Limits, and goals we never reach

Privacy and security aren't traded against quality, size, and cost. They
set the limits inside which we trade those three. To keep them in every
design, each proposal says what it does to privacy and security, the
same way it says what it does to quality, size, and cost
([CLAUDE.md](../../../CLAUDE.md), "Balance quality, size, and cost").
When a change doesn't affect them, the proposal says so, and a reviewer
checks that claim.

Limits are concrete definitions, so they always cover less than we need.
Alongside them, we keep working toward goals we never mark as met:

- stronger assurance that a model provider keeps nothing we send it and
  trains on none of it
- more confidence that we filter confidential information before it
  reaches a model
- stronger checks for security vulnerabilities in what we build

The day we say we've covered privacy or security is the day we start to
fail at it. A stated assumption, like the ones below, is work still to
do, not a reason a leak isn't our problem.

## What we assume

We know of these assumptions, and we expect to find more.

- **No model provider keeps or trains on what it receives.** That covers
  the provider the daemon sends documents to and the providers behind
  the agents that read catalogs, whichever providers those are. A model
  that runs on the person's own machine sends nothing away, so for it
  the question is what it keeps on that machine. We haven't yet written
  down how a person would judge a provider by this measure.
- **No subfolder is shared more narrowly than the folder above it.** Say
  a People Operations team shares `people/`, but only leadership can
  open `people/management/disciplinary/`. If the catalog in `people/`
  covers the disciplinary folder, everyone on the team can read those
  files' paths and summaries. This is rare, since subfolders are usually
  shared more widely and people expect access to a folder to include its
  subfolders, but it happens, and the daemon can't tell when it does.

The subfolder problem appears whenever a catalog reaches people other
than the one who runs catalog. Someone might commit it to a repository
that holds folders with fewer readers, such as other repositories with
fewer collaborators. A cloud drive might sync it. A central server or an
MCP server that delivers catalogs to other people would face it too.

A summary can also repeat a secret from its document, such as a password
or an account number. Preventing that means filtering or redacting
content before the model sees it, which depends on the kind of document.
We've looked at packages for it before, and it's still to be designed.

## Developing catalog

West Arete is a commercial company that does client work and publishes
catalog as open source. Client work and the company's internal
information stay out of this repository and its commits, issues, and
pull requests. Examples are made up.

## Where this goes in the mission

We see three ways to write this into the project's core, and we
recommend the first.

1. **Write privacy and security into the mission itself (recommended).**
   The README's opening and daemon.md's "What the catalog is for" state
   the mission in one breath: help agents find the right files without
   showing anyone more than they could already see. daemon.md gets a
   section with what catalog protects, the questions, the limits and
   goals, and the assumptions, and other docs point to it. A reader
   can't learn what catalog is for without learning this.
2. **A separate privacy document that the mission links to.** It's easy
   to find, but it puts privacy beside the mission instead of in it, and
   a reader who stops at the mission never sees it.
3. **A sentence in the mission and rules in CLAUDE.md.** It adds the
   fewest words, but the reasons would have no home, and agents would
   apply the rules as a checklist.

With the first approach, these are the first places to write it in.
Every later design and change carries it further.

- **README.** The opening paragraph states both halves of the mission.
- **daemon.md.** "What the catalog is for" states both halves, and the
  new section follows it. The line about "the privacy a language model
  gives us" points to that section.
- **architecture.md.** Each component that makes a copy says what it
  keeps or sends and who may read it. The rule applies on every
  platform, including a server that serves a team.
- **The first daemon's spec.** "Who can read what the daemon keeps"
  covers paths, the copies agents send to their providers, and the
  assumptions. Because that version's one provider is Anthropic, it also
  quotes the sentences we rely on from Anthropic's current terms on
  keeping and training on API content, and says "not documented" where
  the terms are silent. Each later provider gets the same check in the
  design that adds it.
- **CLAUDE.md.** "Balance quality, size, and cost" says the three are
  traded inside privacy and security limits, and that every proposal
  states its effect on both. "Security in every design and review"
  covers privacy too and points to daemon.md for why. The development
  rule joins it.

The `knowledge-work:catalog` skill, which lives in another repository,
suggests where catalogs go. That makes it a good place to ask whether a
subfolder is shared more narrowly than its parent.

## Anti-goals

This work has failed, even if every doc changes as planned, if:

- the docs read mostly as warnings, and a reader can no longer tell that
  catalog's job is to help agents find the right files
- someone treats a stated assumption as a reason a leak isn't our
  responsibility
- an agent treats a privacy or security section as a box to tick and
  stops asking the questions
- an agent treats this document as the boundary of where privacy and
  security belong

## Sensitive data in these steps

The steps above edit docs. They read none of the documents catalog
serves and send nothing anywhere. Their risk is that new text carries
something from client work or the company's internal information, so the
builder uses made-up examples, and the reviewer of each step checks for
that.

## Left to the builder

- the exact wording of each change, following
  [writing-style.md](../../../writing-style.md)
- the order of the steps, as long as daemon.md's new section comes
  before the docs that point to it
- whether architecture.md describes copies per component or in one table
