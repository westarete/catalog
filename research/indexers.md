# What other indexers do well and badly

This is side research, kept apart from the daemon design in
[DAEMON.md](../DAEMON.md). In November 2026 we read about how mature
file indexers, build systems and IDEs are built, to see what a catalog
daemon might copy and what it should avoid. Nothing here is a decision.

The systems fall into three groups: Spotlight, other desktop indexers,
and build systems and IDEs that keep derived knowledge up to date. Each
group's sources are listed at the end.

## Spotlight

Apple documents very little about how Spotlight works. Most of what is
known comes from Howard Oakley at Eclectic Light, who measures its
behavior from outside and says which of his conclusions are guesses.

According to Oakley, FSEvents tells a coordinator process, `mds`, that a
file changed. The coordinator starts short-lived `mdworker` processes;
each picks a plugin by file type and extracts the metadata and text.
Each volume keeps its own inverted index in a hidden `.Spotlight-V100`
folder. In his 2025 measurements, the first worker started about 0.9
seconds after a copy and the index was current within 7 to 8 seconds. He
writes that new entries go into side journals that are merged into the
main index later, and labels that as inference. Slow work, such as
reading text in images, is deferred and runs at low priority, almost all
of it on the efficiency cores.

Its weaknesses are all in what happens when something goes wrong:

- Nobody can check that the index is correct. Oakley says the only test
  is to search and see what comes back.
- The only repair is to erase the index and rebuild all of it with
  `mdutil -E`.
- The progress indicator doesn't reflect all indexing activity, so you
  can't tell when it's finished.
- Files that were indexed can drop out of results with no explanation. A
  reader reported this, and Oakley had no tool to diagnose it.
- Logs can fill with `mdworker` crashing and restarting on the same bad
  file.

Craig Hockenberry, a long-time Mac and iOS developer, makes the same
complaint from the outside: iOS offers no rebuild button and no progress
indicator.

Oakley also points out that Core Spotlight is a separate system. There,
each app decides what to index and keeps its own index under
`~/Library/Metadata/CoreSpotlight`, and rebuilding the volume indexes
doesn't touch it.

## Other desktop indexers

We looked at GNOME's Tracker (now LocalSearch), KDE's Baloo, Recoll,
Windows Search, voidtools' Everything and DEVONthink. Their shared
ideas, and their common problems, are in the lessons below.

## Build systems and IDEs

Build systems and IDEs face the same problem as an indexer: keeping
derived knowledge current as files change, without redoing everything.
We looked at the paper Build Systems à la Carte, the Salsa library
behind rust-analyzer, IntelliJ's indexing, Meta's Watchman, Dropbox's
rewritten sync engine (Nucleus), and the code search indexes of Cursor,
GitHub and Sourcegraph.

## Lessons

### Detecting changes

- Watchman combines change events with a full rescan to fall back on.
  When it loses events, it rescans and tells its clients it is starting
  over. Its documentation warns that FSEvents sometimes drops events.
- Users of the desktop indexers report that change events from network
  and cloud folders are unreliable.
- Tying a file's identity to an inode number or an absolute path has
  caused trouble. Baloo has had bugs on Btrfs, where inode numbers
  aren't stable, and DEVONthink's indexed folders depend on absolute
  paths. Nucleus gives each file an ID that stays the same when it
  moves.

### Keeping what's known current

- Spotlight, Tracker and IntelliJ all treat their index as a cache that
  can be deleted and rebuilt. IntelliJ rebuilds an index when the code
  that builds it changes version.
- Spotlight shows the cost of making a full rebuild the only repair.
- Tracker records which version of its extractor produced each result,
  and re-extracts when that changes. Build Systems à la Carte describes
  the general idea as verifying traces: keep a record of the inputs
  behind each result, and check it before trusting the result.
- Salsa avoids redoing work downstream when a recomputed result comes
  out the same as before, which it calls backdating. The paper calls the
  same idea early cutoff.
- The Nucleus team keeps what it has observed about files, not a queue
  of pending work, and works out what to do next from those
  observations.
- Tracker and Windows Search mark files that failed to extract, so they
  aren't retried forever and the failure can be reported.

### Scheduling work

- Tracker, Baloo and Windows Search index in two passes: cheap facts
  like names and metadata first, then content. Everything indexes only
  names, and is fast because of it. Spotlight defers reading text in
  images.
- Windows Search processes change notifications before its background
  crawl, and lets an application ask for a folder to move to the front
  of the queue. Tracker has a similar request.
- Recoll can delay files that change often, and Watchman can wait for a
  folder to stop changing before it reports. Both avoid reprocessing a
  file during every save.
- Rust-analyzer and Salsa cancel work that a newer change has made
  obsolete. A paid model call costs money even if it's cancelled, so for
  a catalog it may make more sense to check that a file is still current
  just before sending it. That point is our own inference, not from a
  source.
- Running at low CPU priority doesn't protect the battery. Tracker users
  on laptops report heavy battery drain.

### Isolating extraction

- Spotlight, Tracker and Windows Search all run extractors in separate
  processes, so a bad file can't crash the indexer. Tracker sandboxes
  its extractor and kills it after 5 seconds without progress. Recoll
  has a time limit per filter. Baloo halves its batch size when an
  extraction fails, to find the bad file.
- Without a limit on retries, a bad file can crash the extractor over
  and over, as Spotlight's logs show.

### Reporting status

- Windows Search keeps a table of what is waiting and why. Spotlight's
  progress indicator, by contrast, can't tell you when it's done.
- Rust-analyzer answers with what it has while indexing continues.
  IntelliJ's "dumb mode" marks which features work while its index is
  being built.
- Cursor and GitHub take the other approach. Cursor hides semantic
  search until about 80% of a codebase is indexed, and GitHub doesn't
  show a repository's results until it's fully processed.
- Sourcegraph dropped embeddings from its code assistant's search.

### Growth and tuning

- Microsoft's troubleshooting guide for Windows Search ties performance
  problems to index size, with thresholds at 400,000 and 1 million
  items.
- Tracker's developers deprecated its throttle and battery settings in
  version 3.10.

## Sources

### Sources on Spotlight

- [Spotlight on search: How Spotlight works](https://eclecticlight.co/2021/01/28/spotlight-on-search-how-spotlight-works/),
  Howard Oakley
- [A deeper dive into Spotlight indexing and local search](https://eclecticlight.co/2025/08/04/a-deeper-dive-into-spotlight-indexing-and-local-search/),
  Howard Oakley
- [In the background: Spotlight indexing](https://eclecticlight.co/2026/02/10/in-the-background-spotlight-indexing/),
  Howard Oakley
- [Spotlight and Core Spotlight are different](https://eclecticlight.co/2026/07/04/spotlight-and-core-spotlight-are-different/),
  Howard Oakley
- [Spotlight: Not Right](https://furbo.org/?p=3052), Craig Hockenberry
- [Staring into the Spotlight](https://blog.doyensec.com/2017/11/15/osx-spotlight.html),
  Doyensec

### Sources on Tracker and LocalSearch

- [LocalSearch overview](https://gnome.pages.gitlab.gnome.org/localsearch/overview.html)
- [LocalSearch D-Bus API](https://gnome.pages.gitlab.gnome.org/localsearch/dbus-api.html)
- [Handling corruption](https://gnome.pages.gitlab.gnome.org/localsearch/corruption.html)
- [localsearch-status manual page](https://www.mankier.com/1/localsearch-status)
- [LocalSearch release notes](https://gitlab.gnome.org/GNOME/localsearch/-/raw/main/NEWS)
- [Tracker 3.x, a retrospect](https://blogs.gnome.org/carlosg/2022/09/11/tracker-3-x-a-retrospect/),
  Carlos Garnacho
- [On CVE-2023-43641](https://blogs.gnome.org/carlosg/2023/10/10/on-cve-2023-43641/),
  Carlos Garnacho
- [Recorded failures in Tracker 3](https://discourse.gnome.org/t/is-it-expected-to-get-recorded-failures-at-tracker-3/4440)
- [Tracker battery drain on a Framework laptop](https://community.frame.work/t/tracker-miner-fs-3-consumes-a-lot-of-battery/53468)

### Sources on Baloo

- [Baloo architecture](https://community.kde.org/Baloo/Architecture)
- [Baloo extractor process source](https://invent.kde.org/frameworks/baloo/-/raw/master/src/file/extractorprocess.cpp)
- [Discussion on Baloo defaults](https://discuss.kde.org/t/discussion-on-baloo-defaults/39526)

### Sources on Recoll, Windows Search, Everything and DEVONthink

- [Recoll user manual](https://www.recoll.org/usermanual/usermanual.html)
- [Indexing prioritization and rowset events](https://learn.microsoft.com/en-us/windows/win32/search/indexing-prioritization-and-rowset-events),
  Microsoft
- [The indexing process](https://learn.microsoft.com/en-us/windows/win32/search/-search-indexing-process-overview),
  Microsoft
- [Windows Search performance issues](https://learn.microsoft.com/en-us/troubleshoot/windows-client/shell-experience/windows-search-performance-issues),
  Microsoft
- [Everything indexes](https://www.voidtools.com/support/everything/indexes/),
  voidtools
- [DEVONthink and Spotlight](https://www.devontechnologies.com/blog/devonthink-and-spotlight),
  DEVONtechnologies

### Sources on build systems and IDEs

- [Build Systems à la Carte: Theory and Practice](https://www.microsoft.com/en-us/research/uploads/prod/2020/04/build-systems-jfp.pdf),
  Andrey Mokhov, Neil Mitchell and Simon Peyton Jones
- [Salsa's algorithm](https://salsa-rs.github.io/salsa/reference/algorithm.html)
- [Three architectures for a responsive IDE](https://rust-analyzer.github.io/blog/2020/07/20/three-architectures-for-responsive-ide.html),
  rust-analyzer
- [Durable incrementality](https://rust-analyzer.github.io/blog/2023/07/24/durable-incrementality.html),
  rust-analyzer
- [Indexing and PSI stubs](https://plugins.jetbrains.com/docs/intellij/indexing-and-psi-stubs.html),
  JetBrains
- [File-based indexes](https://plugins.jetbrains.com/docs/intellij/file-based-indexes.html),
  JetBrains
- [Watchman clock specs](https://meta.github.io/watchman/docs/clockspec),
  [subscribe](https://meta.github.io/watchman/docs/cmd/subscribe) and
  [troubleshooting](https://meta.github.io/watchman/docs/troubleshooting),
  Meta
- [Rewriting the heart of our sync engine](https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine),
  Dropbox
- [Codebase indexing](https://cursor.com/docs/context/codebase-indexing),
  Cursor
- [The technology behind GitHub's new code search](https://github.blog/engineering/architecture-optimization/the-technology-behind-githubs-new-code-search/),
  GitHub
- [Cody FAQ](https://sourcegraph.com/docs/cody/faq), Sourcegraph
