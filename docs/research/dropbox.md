# Dropbox's sync engine

Dropbox's desktop app keeps a folder on each computer in step with
Dropbox's servers. Its engineers spent about four years rewriting the
core of that app, the sync engine, and described the new engine,
Nucleus, in 2020
([Sujay Jayakar, 2020](https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine)).
Nucleus never reads a file for meaning, but it faces our other problems:
it watches a tree, keeps derived state about every file, and has to
recover from events that arrive late, out of order or not at all. Since
Apple introduced File Provider, macOS does part of that job for it.

## Storing observations, not work to do

The old engine, Sync Engine Classic, stored the work still needed to
sync each file. Nucleus stores what it has observed instead, as three
trees: the latest state on the server, the last state seen on disk, and
the last state both sides agreed on. It works out what to do by
comparing them, and it's finished when all three match
([Isaac Goldberg, 2020](https://dropbox.tech/infrastructure/-testing-our-new-sync-engine)).
Without the third tree, a file added on the server would look the same
as one deleted on disk. A component the engineers call the planner turns
the differences into batches of operations that are safe to run at the
same time (same post).

The engineers check the trees' rules where data enters the engine, so no
file can exist without a parent, even briefly (same post). Jayakar
explains why they rewrote rather than repaired. Classic's loose data
model allowed too many states that were legal but wrong, and the basic
nouns of a system can't be changed piece by piece
([Jayakar, 2020](https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine)).

## Noticing changes

On the Mac, the client watched file system events
([Jayakar on localfirst.fm](https://www.localfirst.fm/23/transcript)).
Jayakar calls filesystem APIs weak. They have no transactions, and their
events can arrive out of order, so a move can show up as an add before
the matching delete (same interview).

A Dropbox patent describes one way to cope. The client crawls the disk
at startup and compares it with its record of the local tree. While
running, it buffers events and applies them only once they form a valid
tree, waiting for a missing parent's event if it has to
([US patent 10,866,964](https://patents.google.com/patent/US10866964B2/en)).
A patent shows a possible design, so this may not be what Dropbox
shipped.

## Identifying files

Classic identified files by their full paths. Moving a folder became a
delete and an add for every file inside it, and an add that failed after
its delete succeeded could lose the file
([Goldberg, 2020](https://dropbox.tech/infrastructure/-testing-our-new-sync-engine),
[Jayakar, 2020](https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine)).
Nucleus gives each file an ID that stays the same when it moves, so a
move is a single update
([Goldberg, 2020](https://dropbox.tech/infrastructure/-testing-our-new-sync-engine)).
In the patent, Dropbox's IDs are separate from the operating system's
inode numbers, which the client uses only to find where a file is now
([patent](https://patents.google.com/patent/US10866964B2/en)). Jayakar
adds two problems the engine had to handle on its own: case-insensitive
paths, and platforms that normalize Unicode differently
([localfirst.fm](https://www.localfirst.fm/23/transcript)).

## Making it testable

Classic left thread scheduling to the operating system, so its tests
weren't reproducible. Nucleus runs almost all its logic on one control
thread and hands network, hashing and disk work to other threads
([Jayakar, 2020](https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine)).
The filesystem, network and timers are passed in, which lets tests swap
in fakes. Every randomized test draws all of its choices from a single
seed, so any failure can be replayed exactly, and one of the test
systems, Trinity, runs tens of millions of these each night
([Goldberg, 2020](https://dropbox.tech/infrastructure/-testing-our-new-sync-engine)).

The engineers are frank about the limits. Tests against the real
filesystem run about ten times slower, and nothing verifies that `fsync`
calls are in the right places (same post).

## When state is lost or wrong

The client kept its local records in SQLite
([localfirst.fm](https://www.localfirst.fm/23/transcript)); the
interview doesn't say whether that meant Classic, Nucleus or both.
Jayakar calls throwing away those records and resyncing from the files
the nuclear option. It loses what users meant to do, so files a teammate
deleted can come back. In production, a consistency checker compared
each pair of state copies with hashes and sampling, and the team's goal
was zero inconsistencies they couldn't explain (same interview).

## Under Apple's File Provider

Dropbox's online-only files used to depend on a kernel extension
([Dropbox forum](https://www.dropboxforum.com/t5/Create-upload-and-share/Os-X-10-13-High-Sierra-amp-Smart-Sync/m-p/265318)).
Apple deprecated kernel extensions and offered File Provider in their
place. Under File Provider, macOS detects local changes itself and calls
the app's extension for each create, modify or delete, combining
low-level events such as a safe save into one change. macOS also owns
the copies on disk, and can remove any file the extension reports as
uploaded
([WWDC21 session 10182](https://developer.apple.com/videos/play/wwdc2021/10182/)).

Dropbox's help center lists what changed for users. The folder moved to
`~/Library/CloudStorage`, macOS may slow syncing to save battery,
accounts with more than 300,000 files may sync slowly, and a file open
in an app may not sync until the app closes it
([Dropbox Help](https://help.dropbox.com/installs/macos-faqs)). Dropbox
hasn't published how much of Nucleus still runs under File Provider.

## What the catalog could take from it

These are our own conclusions, not claims from the sources above.

- Instead of keeping a queue of files to profile, the daemon could store
  two observations per file: the content hash it last saw and the hash
  it last profiled. Whatever differs is the queue. After a crash,
  nothing needs replaying; the daemon compares them again.
- Paths make poor keys for profiles, for the reasons Classic ran into.
  Keying profiles by content hash lets a moved or renamed file keep its
  profile without a model call, though a profile that describes a file
  relative to its neighbors may need another look after a move.
- Nucleus's engineers designed deterministic testing into the
  architecture from the start. Our slowest and least predictable
  dependency is the model, so it belongs behind an interface that tests
  can replace, along with the filesystem and the clock.
- For Dropbox, losing its local state means losing what users meant to
  do. Ours holds nothing like that. Every profile can be regenerated, at
  a cost in time and money, which is why treating the daemon's state as
  a cache, as [daemon.md](../design/daemon.md) does, is safe.
- Cloud drives complicate things. On iCloud Drive or Dropbox, macOS may
  keep only a placeholder on disk. We expect that reading such a file to
  profile it would make macOS download it, though we haven't checked.
  The daemon will need a rule for online-only files.
