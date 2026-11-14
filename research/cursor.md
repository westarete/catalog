# Cursor's codebase indexing

Cursor is a code editor built around AI models. From 2023 until mid-2026
it kept a semantic index of each project: it split changed files into
chunks, sent them to its servers to be embedded by a model, and searched
the result by meaning
([Cursor forum, 2023](https://forum.cursor.com/t/codebase-indexing/36),
[Jeremy Stribling, 2026](https://cursor.com/blog/secure-codebase-indexing)).
Of the systems in this survey, it's the only one that ran a paid model
over changed files, as our daemon would. In July 2026 Cursor's staff
said they had replaced it with a local index for grep, because models
had become good at using grep and the semantic index no longer helped
much
([Cursor forum, 2026](https://forum.cursor.com/t/what-do-you-think-about-cursor-removing-the-codebase-indexing-settings/165899)).
Why they dropped it matters to us as much as how it worked.

## Noticing changes

Cursor hashed every file, and computed each folder's hash from its
children's, a structure known as a Merkle tree. An edit changes only
that file's hash and the hashes of the folders above it, so the client
and server could compare their trees and sync just the branches that
differed. For 50,000 files, the paths and hashes came to about 3.2 MB
([Stribling, 2026](https://cursor.com/blog/secure-codebase-indexing)).
Cursor never published how often the client checked; secondary sources
say every five or ten minutes
([Engineer's Codex](https://read.engineerscodex.com/p/how-cursor-indexes-codebases-fast)).
Indexing skipped files listed in `.gitignore` or Cursor's own
`.cursorignore`
([mirror of Cursor's old docs](https://cursor-ai.cadn.net.cn/context_codebase-indexing.en.html)).

## Processing a file

Changed files were split into chunks along the code's syntax and then
embedded. Since embedding was the expensive step, it ran in the
background, and Cursor cached each embedding by the chunk's content so
that unchanged text was never embedded twice
([Stribling, 2026](https://cursor.com/blog/secure-codebase-indexing)).
According to Cursor's old security page, as quoted by Simon Willison,
the server stored each vector with an encrypted path and a line range
but no code. At query time it sent back paths and line ranges, and the
client read the code from disk itself
([Simon Willison, 2025](https://simonwillison.net/2025/May/11/cursor-security)).
Later, Cursor trained its own embedding model on records of agent
sessions ([Cursor, 2025](https://cursor.com/blog/semsearch)).

## Time and cost

Indexing a large repository from scratch could take hours, and Cursor
kept semantic search turned off until at least 80% of a project was
indexed
([Stribling, 2026](https://cursor.com/blog/secure-codebase-indexing)).

To shorten the wait, Cursor reused indexes between teammates. Clones of
a repository within one organization were 92% similar on average. A new
client computed a single similarity hash from its Merkle tree, and the
server copied the closest existing index as a starting point. Reuse cut
the median time to a first query from 7.87 seconds to 525 milliseconds,
and the slowest 1% from about four hours to 21 seconds (same post).

Two limits applied. The semantic index stopped at 250,000 files
([Cursor forum, 2026](https://forum.cursor.com/t/what-do-you-think-about-cursor-removing-the-codebase-indexing-settings/165899)),
and Cursor deleted a server index after six weeks without use, so
returning to an old project meant indexing it again
([Cursor forum](https://forum.cursor.com/t/has-anybody-had-to-try-to-cleanup-after-having-privacy-mode-disabled/145681)).

## Privacy

Cursor kept embeddings and encrypted paths on its servers rather than
code
([Stribling, 2026](https://cursor.com/blog/secure-codebase-indexing)).
Its old security page admitted that someone who broke into the vector
database could partly reconstruct code from the embeddings
([Willison, 2025](https://simonwillison.net/2025/May/11/cursor-security)).
Reused indexes raised a further risk, that one person's search could
return code from a teammate's copy. Cursor's server guarded against this
by dropping any result for a file the client couldn't prove it had,
checked against the client's Merkle tree
([Stribling, 2026](https://cursor.com/blog/secure-codebase-indexing)).

## What users saw

Cursor's settings showed indexing status, a list of included files, and
buttons to resync or delete the index
([old docs](https://cursor-ai.cadn.net.cn/context_codebase-indexing.en.html)).
When the semantic index went, Cursor removed that page too, and its
staff said they had done so on purpose
([Cursor forum](https://forum.cursor.com/t/delete-index-button-disappeared/166835)).
Users objected that they'd lost sight of what was indexed and any
control over it. Some found that searching for a concept got worse,
since grep needs a name to start from
([Cursor forum, 2026](https://forum.cursor.com/t/what-do-you-think-about-cursor-removing-the-codebase-indexing-settings/165899)).

## The replacement

Cursor's new index lives in each repository, as binary files under
`.git/cursor/`. It updates incrementally from a Git commit and is about
half the size of the source
([Cursor forum, 2026](https://forum.cursor.com/t/what-do-you-think-about-cursor-removing-the-codebase-indexing-settings/165899)).
It uploads no paths or code
([Cursor docs](https://cursor.com/docs/context/semantic-search)). A beta
of it first shipped in Cursor 2.1, in November 2025
([changelog](https://cursor.com/changelog/2-1)).

## What the catalog could take from it

These are our own conclusions, not claims from the sources above.

- Cursor's experience is a reason to measure whether profiles help, not
  assume it. Cursor's staff dropped a model-built index once models
  could search well on their own, and our catalog makes the opposite
  bet: that a profile helps an agent choose files better than searching
  does. Code is full of names that grep can find. Prose often isn't, so
  the answer may differ for us.
- Cursor kept semantic search hidden until 80% of a project was indexed.
  [DAEMON.md](../DAEMON.md) does the opposite, showing every file at
  once with a shallow profile and deepening it later. If we go that way,
  agents need to know how deep each profile is.
- Keeping catalog state out of Git means every clone starts empty.
  Cursor's numbers show how much a team gains by reusing work instead. A
  shared store of profiles, keyed by content hash, could let a new clone
  find most of its profiles already written by a teammate, though it
  would bring Cursor's privacy questions with it.
- Users complained as soon as Cursor took away its status page. People
  want to see what an indexer is doing.
