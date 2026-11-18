# Privacy and Security in the Mission Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps
> use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Write privacy and security into catalog's mission and into the
first docs that agents and people read, as the design's recommended
approach describes.

**Architecture:** daemon.md gets the mission in one breath and a new
section on privacy and security; the README, architecture.md, the first
daemon's spec, and CLAUDE.md then carry it into their own parts and
point to that section. Each task edits one document and ends in one
commit.

**Tech Stack:** Markdown, formatted and checked by `make markdown`.

**Spec:**
[docs/superpowers/specs/2026-11-18-privacy-and-security-design.md](../specs/2026-11-18-privacy-and-security-design.md)

## Global Constraints

- Read the spec in full before any task. Its sections "What catalog
  protects," "Questions for every step," "Limits, and goals we never
  reach," and "What we assume" are the content; the tasks say where it
  goes.
- Load `knowledge-work:markdown` before editing any `.md` file, and run
  `make markdown` after every edit. Write each paragraph as one long
  line and let Prettier wrap it.
- Follow [writing-style.md](../../../writing-style.md): open with
  complete sentences, keep people as the actors, speak plainly, back
  every claim.
- Client work and West Arete's internal information stay out of every
  file and commit message. Examples are made up, like the spec's
  `people/management/disciplinary/2026-03-02-alex-doe-termination-preparation.md`.
- Write privacy and security as something that belongs everywhere. Don't
  state what a document or section leaves out, and don't describe
  today's scope as a limit.
- Catalog is designed to work with any model provider, hosted or local.
  Name a provider only where the text describes a version that uses it,
  such as the first daemon's spec.
- Don't write rules for capabilities nobody has designed, such as
  filtering secrets or a configurable state folder
  ([issue #7](https://github.com/westarete/catalog/issues/7)). Name them
  as still to be designed.
- Before every commit, run `make markdown` and `make test`. Commit
  messages follow Tim Pope's style, with no attribution trailers.

## Review Focus

Copy these into every task reviewer's brief.

- **Security:** no edit weakens or blurs a protection the docs already
  state, such as the file modes in the first daemon's spec, the API key
  staying out of logs, or a catalog's `config.toml` never naming a
  server or holding a credential.
- **Client and internal information:** no name, path, figure, or detail
  in the new text or the commit message comes from client work or West
  Arete's internal information.
- **Fences:** no sentence reads as a limit on where privacy and security
  belong, such as "this section covers," "only," "none of them says," or
  a list presented as complete.
- **Mission balance:** a reader still learns first that catalog's job is
  to help agents find the right files; the new text doesn't turn the doc
  into a list of warnings.
- **One provider:** no new text ties catalog's design to one model
  provider; a provider's name appears only where the text describes a
  version that uses it.
- **Claims:** every statement about how a model provider, macOS, Git, or
  another product behaves cites a source the reviewer can open, or says
  it's not documented.

---

### Task 1: daemon.md — the mission and a section on privacy and security

**Files:**

- Modify: `docs/design/daemon.md`, "What the catalog is for" (lines
  12–39), "The experiment" (line 88), and "Time vs quality vs cost"
  (lines 201–202)

**Interfaces:**

- Produces: a `## Privacy and security` heading in daemon.md, directly
  after "What the catalog is for". Tasks 2 to 5 link to it as
  `daemon.md#privacy-and-security`.

- [ ] **Step 1:** Rewrite the opening of "What the catalog is for" so it
      states both halves of the mission in one sentence: the catalog
      tells an agent which documents matter for the task, without
      showing anyone more than they could already see. Keep the rest of
      the section.
- [ ] **Step 2:** Add `## Privacy and security` after "What the catalog
      is for". Carry the spec's four content sections into it, rewritten
      for daemon.md's readers: what catalog protects (the rule, copies
      including what agents send their own providers, paths, whose
      information it is, security), the three questions, limits and the
      goals we never reach, and what we assume. Describe the model
      provider as whichever one the person chooses, hosted or local.
      Leave out the spec's approaches and steps.
- [ ] **Step 3:** In "The experiment" (line 88) and "Time vs quality vs
      cost" (line 201), link the mentions of privacy to the new section.
- [ ] **Step 4:** Run `make markdown`. Expected: "Markdown OK".
- [ ] **Step 5:** Commit.

### Task 2: README — the mission's opening

**Files:**

- Modify: `README.md`, the first paragraph (lines 3–8)

**Interfaces:**

- Consumes: `daemon.md#privacy-and-security` from Task 1.

- [ ] **Step 1:** Rewrite the first paragraph so it states both halves
      of the mission, the way daemon.md now does, and links to
      `docs/design/daemon.md#privacy-and-security`. Keep its problem
      statement about agents loading the wrong files.
- [ ] **Step 2:** Run `make markdown`. Expected: "Markdown OK".
- [ ] **Step 3:** Commit.

### Task 3: architecture.md — what each component copies

**Files:**

- Modify: `docs/design/architecture.md`, under "Components" (lines
  99–307) and the last open question (lines 513–517)

**Interfaces:**

- Consumes: `daemon.md#privacy-and-security` from Task 1.

- [ ] **Step 1:** For each component that keeps or sends a copy of
      what's in people's files (at least Profiler, Models, Store,
      Journal, Publisher, Control, and Clients), say what it keeps or
      sends and who may read it. State it as a requirement on every
      platform the doc covers, including a server that serves a team and
      Control's requests from other clients. For Models, the requirement
      applies to every provider, and a provider that runs on the machine
      still has to say what it keeps there. Leave how the Mac meets it
      to the first daemon's spec. Choose per-component text or one
      table, whichever reads better.
- [ ] **Step 2:** Link the open question about a central server holding
      profiles (line 517) to `daemon.md#privacy-and-security`.
- [ ] **Step 3:** Run `make markdown`. Expected: "Markdown OK".
- [ ] **Step 4:** Commit.

### Task 4: the first daemon's spec — who can read what it keeps

**Files:**

- Modify: `docs/superpowers/specs/2026-11-15-daemon-mvp-design.md`, "Who
  can read what the daemon keeps" (lines 435–466)

**Interfaces:**

- Consumes: `../../design/daemon.md#privacy-and-security` from Task 1.

- [ ] **Step 1:** Keep the file modes as they are. Add that paths count
      as much as summaries; that each agent loading `catalog.md` sends
      its paths and summaries to the agent's own model provider; and the
      two assumptions, with how the subfolder assumption can fail for
      this version: a catalog committed to a repository that holds
      folders with fewer readers, or a catalog in a folder a cloud drive
      syncs. Link to the new section in daemon.md for the reasons.
- [ ] **Step 2:** Find Anthropic's current published terms or
      documentation on whether it keeps API inputs and outputs, for how
      long, and whether it trains models on them. This version's one
      provider is Anthropic. Read each page in full; don't rely on a
      search summary.
- [ ] **Step 3:** In the same section, quote the exact sentences we rely
      on, each linked to its page, and say what they cover: commercial
      API use, the retention period, and training. Where the pages say
      nothing on a point, write that it's not documented.
- [ ] **Step 4:** Run `make markdown`. Expected: "Markdown OK".
- [ ] **Step 5:** Commit.

### Task 5: CLAUDE.md — every proposal weighs privacy and security

**Files:**

- Modify: `CLAUDE.md`, "Balance quality, size, and cost" (lines 161–167)
  and "Security in every design and review" (lines 169–187)

**Interfaces:**

- Consumes: `docs/design/daemon.md#privacy-and-security` from Task 1.

- [ ] **Step 1:** Load `knowledge-work:maintaining-agent-instructions`
      and follow it for every edit to CLAUDE.md.
- [ ] **Step 2:** In "Balance quality, size, and cost," say that the
      three are traded inside the limits privacy and security set, and
      that every proposal says what it does to privacy and security as
      well, even when the answer is that it changes nothing.
- [ ] **Step 3:** Widen "Security in every design and review" to privacy
      and security. Add the three questions as what to ask at every
      step, link to `docs/design/daemon.md#privacy-and-security` for
      why, and add the rule for developing catalog from the spec's
      "Developing catalog". Keep the existing bullets.
- [ ] **Step 4:** Run `make markdown`. Expected: "Markdown OK".
- [ ] **Step 5:** Commit.
