# Watchman

Watchman is a file-watching service from Meta. Tools such as Mercurial
and Jest ask it which files changed since they last looked, instead of
scanning the tree themselves
([Mercurial's fsmonitor](https://www.mercurial-scm.org/pipermail/mercurial-devel/2016-March/081067.html),
[Jest's crawler](https://github.com/jestjs/jest/blob/main/packages/jest-haste-map/src/crawlers/watchman.ts)).
It reports changes but never reads file contents. That makes it a good
model for the change-detection half of a catalog daemon, while its
clients do the expensive half.

## One server for many trees and clients

Each user runs a single Watchman server. Clients talk to it over a
socket, and the first client to connect starts it
([CLI options](https://meta.github.io/watchman/docs/cli-options)).

A client asks for a watch with `watch-project`, which walks up from the
given directory to the nearest marker file, such as `.git`, `.hg` or
`.watchmanconfig`, and watches that root instead. Several tools working
in the same project then share one watch. The developers' reason is that
an operating system can only watch so many directories well, and
overlapping watches hurt performance and reliability
([watch-project](https://meta.github.io/watchman/docs/cmd/watch-project)).
Watchman removes a watch after five days without use
([configuration](https://meta.github.io/watchman/docs/config)).

## Noticing changes

On macOS, Watchman asks FSEvents for an event per file; on Linux it uses
inotify ([install](https://meta.github.io/watchman/docs/install),
[FSEvents watcher source](https://raw.githubusercontent.com/facebook/watchman/main/watchman/watcher/fsevents.cpp)).

It keeps its index of the whole tree in memory, as a list ordered by
when each file last changed. Answering "what changed" therefore costs
time in proportion to the number of changed files, not the size of the
tree
([Wez Furlong, 2013](https://wezfurlong.org/blog/2013/Jun/watchman/)).
Memory grows with the tree, and the configuration docs give sizing hints
for large ones
([configuration](https://meta.github.io/watchman/docs/config)).

## When events are lost

When FSEvents reports that it dropped events, or inotify's queue
overflows, Watchman rescans the whole root. The developers call this a
recrawl, and their troubleshooting guide notes that it costs clients as
well, because a recrawl reports most files as changed
([troubleshooting](https://meta.github.io/watchman/docs/troubleshooting),
[FSEvents watcher source](https://raw.githubusercontent.com/facebook/watchman/main/watchman/watcher/fsevents.cpp)).
Every response after a recrawl carries a warning until someone
re-creates the watch or restarts the server
([troubleshooting](https://meta.github.io/watchman/docs/troubleshooting)).
Jest logs that warning and carries on
([Jest's crawler](https://github.com/jestjs/jest/blob/main/packages/jest-haste-map/src/crawlers/watchman.ts)).

The developers also built a cheaper recovery on macOS that replays the
FSEvents journal from the last good event. They turned it off by default
in December 2021 because of possible correctness problems they hadn't
diagnosed
([configuration](https://meta.github.io/watchman/docs/config)).

A few failures can't be recovered from. If Watchman runs out of inotify
watches or open files, it fails every request for every watch until
someone restarts it
([troubleshooting](https://meta.github.io/watchman/docs/troubleshooting)).

## Answering "what changed since"

Every response carries a clock, a token that advances when Watchman sees
a change rather than with wall time. A client stores the clock and sends
it back on its next query, and gets only the files that changed in
between. The docs advise against asking by timestamp, which is prone to
race conditions
([clockspec](https://meta.github.io/watchman/docs/clockspec)).

The clock includes the server's start time and process ID. If the server
has restarted since the client's clock was issued, or the clock is older
than the 12 hours of deletions Watchman remembers, the answer is a fresh
instance: every file that exists, all marked new
([clock source](https://sources.debian.org/src/watchman/4.9.0-9/clockspec.cpp),
[configuration](https://meta.github.io/watchman/docs/config),
[file queries](https://meta.github.io/watchman/docs/file-query)). Jest
rebuilds its file map when it gets one
([Jest's crawler](https://github.com/jestjs/jest/blob/main/packages/jest-haste-map/src/crawlers/watchman.ts)).

On Linux, Watchman makes sure an answer includes every change made
before the query by writing a marker file and waiting to see its event.
It can't do that on macOS, because FSEvents doesn't guarantee the order
of events; clients there ask Watchman to wait for the tree to settle
instead
([query synchronization](https://meta.github.io/watchman/docs/cookies)).

## Waiting for a tree to settle

Before running a trigger, Watchman waits for the tree to be quiet for 20
milliseconds by default. It runs one instance of each trigger at a time
and hands it the whole batch of changed files
([configuration](https://meta.github.io/watchman/docs/config),
[trigger](https://meta.github.io/watchman/docs/cmd/trigger)).

Subscriptions also hold notifications while a Git or Mercurial lock file
exists, so a checkout arrives as one batch rather than thousands of
separate changes. Other tools can mark their own busy periods with
`state-enter` and `state-leave`, and each subscriber chooses whether to
get the held notifications afterward or skip them
([subscribe](https://meta.github.io/watchman/docs/cmd/subscribe)).
Mercurial marks every working-copy update this way
([Martijn Pieters' patch](https://www.mercurial-scm.org/pipermail/mercurial-devel/2016-March/081067.html)).

## What it keeps and what it rebuilds

Watchman saves only the list of watches and triggers. It doesn't save
its file index, so after a restart it crawls each root again and reports
everything as changed
([CLI options](https://meta.github.io/watchman/docs/cli-options),
[watch](https://meta.github.io/watchman/docs/cmd/watch)). Durable state
belongs to the clients, which store the last clock beside their own
data; Waleed Khan recommends this in his talk on
[incremental processing with Watchman](https://docs.google.com/presentation/d/1geE7rGTCpIk5UTjH-sw9aL_Zszi1ncCRnFi55t0CDlk/htmlpresent).

## Known limits

- FSEvents can drop events under heavy load, and can fail outright when
  too many clients use it
  ([troubleshooting](https://meta.github.io/watchman/docs/troubleshooting)).
- Ignoring directories at the FSEvents level works for only the first
  eight entries in `ignore_dirs`
  ([configuration](https://meta.github.io/watchman/docs/config)).
- On macOS, a rename that only changes the case of a name shows up as a
  delete and a change
  ([watch](https://meta.github.io/watchman/docs/cmd/watch)).
- The developers warn that network filesystems are likely to perform
  poorly, and Watchman can be set to refuse NFS and SMB
  ([install](https://meta.github.io/watchman/docs/install),
  [configuration](https://meta.github.io/watchman/docs/config)).

## What the catalog could take from it

These are our own conclusions, not claims from the sources above.

- [DAEMON.md](../DAEMON.md) already proposes Watchman's shape: one
  server per user, finding each tree by walking up to a marker file. The
  developers' reason for it, that an operating system can only watch so
  many directories well, applies to us too.
- When events go missing, a full rescan is the fallback. Ours can be
  cheap if it only compares content hashes with the ones already
  profiled, and calls the model just for files that changed.
- We can throw away the list of files and rebuild it with a crawl, the
  way Watchman does. Profiles are different: each one costs a model
  call, so we should store them durably and key them by content.
- Settling, and holding changes while Git is busy, would keep us from
  re-profiling a file on every save, one of the open questions in
  DAEMON.md. Our delay should be far longer than Watchman's 20
  milliseconds.
- If the catalog falls behind, it could say so on every answer until
  someone fixes it, as Watchman does after a recrawl.
