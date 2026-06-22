# catalog: future directions

**Status: speculative.** This is a design we talked through, written
down so the reasoning survives the conversation. Treat it as a proposal
to argue with, not a spec to implement.

## The limits of a flat catalog

Today there is one `CATALOG.md` per repo, loaded in full at the start of
every session. That works well now (about 20 entries, roughly 2,400
tokens — a 19:1 saving over loading the documents themselves). It stops
working at scale for two reasons:

- **Budget share.** The catalog is a fixed tax on every session's
  context, whatever the task. At roughly 120 tokens per entry, a few
  hundred documents turn the index itself into a meaningful slice of the
  window — most of it irrelevant to the task at hand.
- **The access pattern isn't a tree.** Working in product, you still
  reach for maybe 10% of leadership and 10% of outreach. A strict split
  by department forces every document to have one home, but the
  relationships you actually follow cross those lines. The real shape is
  a graph with weighted edges, and the strong edges don't respect
  department boundaries.

So a plain department split is the wrong cut: it would either drop the
cross-department documents you need or make you load whole neighbouring
departments to get the 10% that matters.

## The idea in one picture

```text
content docs            the actual Markdown (truth)
      │
profile store           one profile record per document (truth, normalized)
      │
edge graph              weighted relationships between documents (truth, normalized)
      │
catalog generate        BUILD TIME: store + graph in, Markdown out
      │
context-scoped          one generated CATALOG per context — own docs plus
catalogs                cross-context docs whose edge weight clears a threshold
      │
agent reads one         RUNTIME: a file read, exactly like today
```

The point that makes this worth doing: **the graph is a build-time
input, and the output is plain committed Markdown.** Nothing runs when
an agent starts a session — it reads a file, the same as now. We keep
the property the whole context system rests on (it's just tracked
files), and we get adjacency-aware indexes without standing up a
retrieval service.

## The profile store

The profile store holds one generated record per document — the single
place a profile lives. Normalized: one fact, one home. When a document
changes, one profile record is updated, and that triggers regeneration
of every catalog that references it.

The store is a generated cache, not an authored artifact. Profiles are
never hand-edited; any correction flows upstream into the generator or
the content docs themselves. The right storage format is a committed
SQLite database rather than a directory of text files.

SQLite fits this role well. Git detects it as binary automatically (it's
not valid UTF-8), so `git diff` produces a clean "binary files differ"
line with no configuration needed. The file travels with the repo on
clone and through subtree exports, so every machine starts with the same
cache state without an expensive bootstrap run. Inspection uses
dedicated commands (`catalog show`, `catalog diff`) rather than
`git diff` — the right tool anyway, since anyone debugging why the wrong
files loaded is already in developer mode.

Merges are the one place that needs a documented procedure. If two
branches both ran `catalog update`, Git will flag a binary conflict on
the database file. The resolution: pick either side
(`git checkout --theirs profiles.db` or `--ours`), then re-run
`catalog update` for the content docs that changed on the other branch.
Which docs those are is a `git diff --name-only MERGE_HEAD` away. You're
not resolving a conflict — you're regenerating, which is always the
right answer for a cache.

## Generate catalogs from a store of profile files

With one profile record per document in the store, catalogs become
projections over it rather than monolithic files. Different catalogs can
be tuned for different contexts, but there is only one copy of each
profile, so the truth stays in one place and the outputs stay
disposable.

- The profile store holds one profile record per document — the single
  place a profile is authored and lives. Normalized: one fact, one home.
- The edge graph holds the weighted relationships between documents.
  Also normalized: each relationship recorded once.
- The context-scoped catalogs are generated from those two. They may
  repeat a profile across several files, but that is duplicated
  _output_, not duplicated truth — regenerated from one source, like a
  built site or a compiled binary. The database word is a materialized
  view; the shorthand is hub and spokes, where the hub is the store and
  the spokes are disposable.

| Layer            | What it is                | Authored by              | Role               |
| ---------------- | ------------------------- | ------------------------ | ------------------ |
| Content docs     | the Markdown              | humans                   | truth              |
| Profile store    | one profile per doc       | generated                | cache              |
| Edge graph       | weighted doc-to-doc links | generated, human-curated | truth (normalized) |
| Context catalogs | what agents read          | generated                | projection (cache) |

## Where the edges come from

Edges are generated, the same way profiles already are — and they have
to be, since the number of possible pairs grows with the square of the
document count. The sources, strongest and cheapest first:

- **Directory structure (start here).** The tree is real, intentional
  metadata that already exists: someone deliberately put these files in
  these folders. Siblings in a directory are close, parent and child are
  one hop out, and the number of directory hops is a decent proxy for
  distance after that. This costs nothing — no model call, no embedding
  — and it's a far stronger first signal than anything inferred, because
  it reflects a human's actual filing decision rather than a guess. It's
  blind to cross-department relationships, which is exactly what the
  later sources and the curated indexes are for, but it gets most of the
  edges right for free.
- **Usage statistics.** If we ever log which documents get loaded
  together in a session, co-occurrence is the truest edge weight we can
  get — it's measured access, not predicted relevance, and it catches
  the cross-department relationships the tree can't see. We don't
  collect this today; it folds in as another weight source when we do.
- **Semantic similarity.** "Text embeddings" are numerical
  representations of text that capture meaning — two passages that mean
  similar things produce numbers that are close together. Run each
  document's profile prose through an embedding model, then keep the
  pairs whose numbers are closest. Mechanical and scales to thousands of
  documents. The catch is that shared vocabulary is a proxy for
  relevance, not relevance itself — usually but not always the same
  thing. See the note below on which model to use, because the answer
  changed recently.
- **Ask the model (weakest on its own).** For each document, ask the
  model which others it connects to. Tempting because it needs no new
  vendor or key, but on its own it produces superficial and often
  inaccurate relationships — free association over profiles, with no
  grounding in how the files are actually filed or used. Useful only to
  refine edges the stronger sources already proposed, never as the
  foundation.

In every case the generator produces a **draft** and a human produces a
**veto**: you review the diff and delete or reweight the wrong edges,
rather than authoring them from scratch. Reviewing a generated list is
bounded work; authoring every pair is not.

## Which embedding model to use

The semantic similarity option above is the one place that needs a model
we don't already call. Worth recording the current state (June 2026),
because it's moving and an older note here will mislead:

- **Anthropic still has no first-party embedding model** and points to
  Voyage AI — but Anthropic has since acquired Voyage, so that's now the
  embedding arm of the same company, not an outside vendor. It's still a
  separate API and a separate `VOYAGE_API_KEY`.
- **There is now an open-weight option in that family:**
  `voyage-4-nano`, Apache 2.0, on Hugging Face. That matters because it
  lets us run a Voyage-family model locally with no key at all.
- **Other strong local, no-key options:** Nomic Embed v2 (small enough
  to run on CPU), Qwen3-Embedding (tops the open-weight leaderboard),
  BGE-M3, Jina v5. Any of these runs at build time inside the generator,
  writes weights into the committed edge file, and needs no runtime and
  no credentials.

So the similarity route no longer forces a second vendor: a local model
(Nomic v2 on CPU, or `voyage-4-nano`) gives build-time edges with zero
runtime and zero new keys. Confirm model availability against current
docs before committing — this is exactly the kind of fact that ages.

## What changes in the commands

The existing command shape survives; each one gains an edge-and-fan-out
step:

- `bootstrap` — build the profile store and the edge graph from scratch,
  then generate every context catalog. Same "from nothing, extra pass"
  spirit it has now.
- `update` — a document changed, so re-infer its profile record and
  recompute just its edges against the rest, then regenerate the
  catalogs that reference it. Same incremental, Git-driven logic.
- `force` — re-infer named (or all) profile records and their edges,
  then regenerate.
- `check` — still no model call. It verifies the store, the edge graph,
  and the generated catalogs are mutually consistent: no edge pointing
  at a deleted document, every catalog matching what regeneration would
  produce — the same way it checks the single catalog today.

## Costs to go in with eyes open

- **Similarity edges are a proxy.** The first generated graph will be
  decent, not perfect: some false edges (easy to delete on review) and
  some missed true ones (harder, because you don't see what isn't
  there). It improves with curation and with the model-asked and
  usage-statistics inputs.
- **More bookkeeping on write.** A document edit can now mark several
  catalogs stale. That's the denormalization trade — cheap reads for
  slightly more work tracking which spokes a record feeds. It's the new
  work in the program, so it's named here rather than discovered later.

## Near future

The full graph-driven build above isn't worth it at today's scale (about
20 documents) — a single flat catalog is the right tool, and a two-layer
build would add a generation step and a maintenance surface for no real
return. The practical ceiling for a flat catalog is roughly 150–250
entries, set by routing precision and budget share rather than any token
limit. But the path there isn't a single yes/no decision at the ceiling;
it's a ladder of small steps, each useful on its own and each setting up
the next:

**1. A size warning in `check`.** Have `catalog check` warn when the
catalog crosses a threshold, so "the index feels too big" becomes a
measured signal — the same way staleness is already a Git query rather
than a judgement call. It's independent of everything below and tells us
when the rest starts to matter.

**2. The profile store.** Migrate the existing catalog entries into the
SQLite store and regenerate `CATALOG.md` from it. That's nearly free
work — and it's the unlock for everything after it. Sibling context, the
per-department indexes, and the graph all need per-document records;
with the store in place, each of those is an addition rather than a
rebuild.

**3. Sibling context when inferring profiles.** Because these are
Markdown files in a tree, the tree itself is a free proximity hint:
sibling files in a directory are close, parent and child are one hop
out, and the number of directory hops is a decent proxy for distance
after that. We get that signal for nothing — no model call, no
embedding. So when inferring a profile, process a file with its folder
loaded around it rather than on its own: a profile's job is mostly to
distinguish a document from the things it'll be confused with, and those
are usually its siblings. Today's neighbor context treats the whole
catalog as equidistant; weighting it toward near neighbors in the tree
should produce sharper profiles at no extra cost.

**4. Curated cross-departmental indexes.** Generate a separate catalog
per department, where each one is mostly its own department's documents
plus a curated handful of cross-references to documents in other
departments — so in outreach you're reading roughly 80% outreach and the
20% of leadership or product you actually reach for, and in leadership
the reverse. The cross-references here are **human-guided**: a person
decides which out-of-department documents each index pulls in, drawing
on real knowledge of which work crosses lines. No edge weights, no
generated relationships — just the profile store composed into
department-scoped views with hand-picked links across. This delivers the
cross-department value early, through judgment, and is exactly the work
the next step would eventually automate.

**5. A generated graph of edges.** The fancy version — edges,
context-scoped fan-out, and the embedding or usage-statistics sources
described above — replaces the hand-picked cross-references of step 4
with generated ones. This is the largest step and the least settled;
parts of it probably aren't fully worked out yet. Reach for it only once
the curated indexes are straining under their own upkeep. By then it's
an addition on top of the store, not a rebuild.

## How this differs from Google Workspace Intelligence

Google launched Workspace Intelligence in April 2026 — a context layer
that gives Gemini continuous awareness of a user's activity across
Gmail, Drive, Calendar, Chat, Docs, Sheets, and Slides. It is worth
understanding what that does and what it does not do, because the two
approaches solve different problems.

Workspace Intelligence is **reactive and activity-driven**: it reads
what a user has been working on and assembles context dynamically from
that signal. It does not know which documents are authoritative for a
given topic; it knows which documents were recently touched or are
broadly accessible. A team's most important strategy doc and a draft
someone created yesterday and never finished look the same to it.

The context catalog is **deliberate and curation-driven**: a person (or
a generator under human review) decides which documents matter, what
each one is for, and how to describe it precisely enough to distinguish
it from the things it might be confused with. The result is a committed
artifact — not a live index assembled at session start, but a file that
reflects explicit human judgment about the knowledge that is worth
loading. Workspace Intelligence cannot produce that, and it does not try
to.

The two approaches are complementary. Workspace Intelligence makes an
agent aware of recent activity and broadly available content. A catalog
tells it which content deserves weight when the task at hand requires
reaching for the team's considered knowledge. Neither replaces the
other: a team running both gets ambient awareness from Workspace
Intelligence and deliberate context routing from the catalog.

One practical note: real-world Workspace Intelligence quality is mixed.
User testing reports roughly 74% accuracy on factual questions, and
custom instructions are frequently ignored. The catalog technique
sidesteps both failure modes — it does not depend on the model deciding
what to load, and it does not rely on instructions surviving to the
moment of retrieval. The catalog is already loaded before any
instruction could override it.
