# Writing style

These rules cover writing that people read: READMEs, notes, analyses,
plans, and anything sent outside the repo. They don't cover instruction
files such as `CLAUDE.md` or skills, which are written in instruction
voice for agents. Apply them to text you edit, not just text you write.

## Write like a small team, not a corporation

This is a small team, not a corporation. Match that voice everywhere you
write — README sections, replies in chat. The test isn't whether the
writing is correct; it's whether a colleague would want to read it.

- **Don't write for agents when humans will read it.** Phrases like
  "authoritative source," "deeper procedural guidance," "in full before
  acting on its topic," or "pointers, not replacements" read like a
  system prompt. Cut them.

## Open with complete sentences

Open every document with complete sentences that tell the reader what
they need to know. Never open with a fragment that names the topic.

- Wrong: "How the university buys services: who runs it, when it
  requires competition, and how payments work."
- Right: "The university buys services through its central procurement
  office, which decides when a purchase needs competition."

## Keep people as the actors

Only people act. A project, document, form, price, policy, or message
doesn't decide, need, ask, or reject. When one does in your draft, name
the person or office that acts. A policy can still shape what people do.

- Wrong: "A rejection sends the work to bid."
- Right: "If the procurement office rejects it, it puts the work out to
  bid."

## Speak plainly and avoid throwaway metaphors

**Speak plainly. Avoid throwaway metaphors from unrelated domains.**
Phrases like "land it," "load-bearing," "in flight," "lift," "moving the
needle," "north star," "heavy lift," or other shallow aviation,
construction, or sports analogies are annoying when a direct verb will
do. Say "commit it," "essential," "in progress," "effort," "changes the
result," "goal," "hard," etc. This applies to chat as much as to docs.

## Choose each word for what it does

- **Don't reach for "canonical."** It disambiguates one version from
  another; use it only when you're actually doing that. If the
  surrounding sentence already explains what makes the document the one
  we care about, leave the word out.
- **Drop redundant qualifiers.** If the next clause does the work, the
  adjective doesn't.
- **Say what something is for, not its policy status.** "For more
  guidance" beats "for the full policy."
- **Skip forward references the reader will hit on their own.**
- **Use the vocabulary the doc has already defined.**

## Don't be a drama queen

Write to convey an idea, not to walk the reader through alarm and
relief. When a statement holds only under some condition, qualify it in
the same sentence. Never state the alarming general case and undo it in
the next sentence. A lead-in is fine; a withheld qualifier is
manipulation.

- Wrong: "A high-risk rating means backup for every invoice. The
  fixed-price checklist has no such step."
- Right: "A high-risk rating ordinarily means having to provide evidence
  for every invoice, so we use the fixed-price option instead, which has
  no such requirement."

## Back every claim

Point every claim to a place the reader can check: a policy, an email, a
transcript, a document in the repo. Never state how a process or an
organization works without a source; say what you don't know instead.
Use quotation marks only for words you copied exactly.

## Make links clickable and keep emphasis rare

When pointing at web pages or repo files, use **clickable Markdown
links** (e.g. `[README](README.md)`), not bare URLs in backticks or
bold, so nothing requires copy-paste.

- **Don't bold link text just because it's a link.** The link styling is
  already enough.
- Prefer clarity over decoration-heavy Markdown (excessive bold, etc.).

## Write commit messages Tim Pope's way

Commit messages follow Tim Pope's
[A note about Git commit messages](https://tbaggery.com/2008/04/19/a-note-about-git-commit-messages.html).

## No decision logs

Never write a decisions log, or any section that records how a document
came to be. Put the reasoning beside the content it explains, and leave
history to Git.
