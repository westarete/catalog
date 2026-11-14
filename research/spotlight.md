# Spotlight

Spotlight is the search index built into macOS, and the nearest thing to
a catalog daemon that already runs on every Mac. A coordinator process,
`mds`, learns from FSEvents that files changed and hands each file to a
short-lived `mdworker` process, which reads it with a plugin chosen by
file type
([Howard Oakley, 2021](https://eclecticlight.co/2021/01/28/spotlight-on-search-how-spotlight-works/)).
Apple documents very little of how local indexing works. Most of what
follows comes from Oakley, who measures Spotlight from outside and says
which of his conclusions are guesses
([Oakley, 2025](https://eclecticlight.co/2025/07/30/a-deeper-dive-into-spotlight-indexes/)).

## Noticing changes

Apple built FSEvents for Spotlight and introduced it in Mac OS X Tiger
([WWDC07 session 126](https://nonstrict.eu/wwdcindex/wwdc2007/126/)).
The kernel side drops events rather than block when its queue is full,
and its queue is shared, so one slow reader makes every client lose
events (same session). FSEvents reports changes per directory, merges
changes that arrive close together, and keeps a history on each volume
that survives reboots
([FSEvents overview](https://developer.apple.com/library/archive/documentation/Darwin/Conceptual/FSEvents_ProgGuide/TechnologyOverview/TechnologyOverview.html)).

Apple tells FSEvents clients to treat the events as hints. A client
stores the ID of the last event it handled and replays the history from
there when it restarts. It rescans a directory when FSEvents says events
were dropped, and throws away its stored state if the volume's ID
changes
([using FSEvents](https://developer.apple.com/library/archive/documentation/Darwin/Conceptual/FSEvents_ProgGuide/UsingtheFSEventsFramework/UsingtheFSEventsFramework.html)).
Apple hasn't documented what Spotlight itself does after dropped events.

## Processing a file

`mds` coordinates the work, and `mdworker` processes check each file's
type and load the matching importer plugin
([Oakley, 2021](https://eclecticlight.co/2021/01/28/spotlight-on-search-how-spotlight-works/)).
Apple allows one importer per file type, and requires importers to run
without their app and without any user interface
([importer guide](https://developer.apple.com/library/archive/documentation/Carbon/Conceptual/MDImporters/Concepts/Importers.html)).
Apple asks importers to be quick and not leak memory, but sets no time
limit
([importer performance](https://developer.apple.com/library/archive/documentation/Carbon/Conceptual/MDImporters/Concepts/Performance.html)).
Oakley infers that Spotlight starts one worker per file
([Oakley, 2025](https://eclecticlight.co/2025/08/04/a-deeper-dive-into-spotlight-indexing-and-local-search/)).

Because each file gets its own process, a bad file crashes only its
worker. Oakley describes logs that fill with the same worker crashing
and restarting on one file, and the fix he gives is to remove the file
([Oakley, 2022](https://eclecticlight.co/2022/12/08/spotlight-problems-mds_stores-and-mdworker-in-trouble/)).

Spotlight sends slower work, such as reading text in images, to a
separate service, `mediaanalysisd`
([Oakley, 2026](https://eclecticlight.co/2026/02/10/in-the-background-spotlight-indexing/)).

## Where it keeps state

Each volume keeps its own index in a hidden `.Spotlight-V100` folder
([Oakley, 2021](https://eclecticlight.co/2021/01/28/spotlight-on-search-how-spotlight-works/)).
The folder holds about 99 files in an undocumented format, and Oakley
infers from Apple's patents that Spotlight writes new entries to side
journals and merges them into the main index later
([Oakley, 2025](https://eclecticlight.co/2025/07/30/a-deeper-dive-into-spotlight-indexes/)).
Apple tells importer authors to treat the store as volatile
([importer guide](https://developer.apple.com/library/archive/documentation/Carbon/Conceptual/MDImporters/Concepts/Importers.html)).

The only repair is a full rebuild. Apple's support article has the user
add the volume to Spotlight's privacy list and then remove it
([Apple Support](https://support.apple.com/en-us/102321)). Oakley points
out that nothing can check an index for damage, and that a rebuild won't
help while the bad file or importer is still there
([Oakley, 2024](https://eclecticlight.co/2024/11/19/when-and-how-to-rebuild-spotlight-indexes/)).

## Scheduling

Oakley measured indexing on Apple silicon. Spotlight runs it at low
priority, almost all on the efficiency cores. A worker starts within
about a second of a change, and the file's text reaches the index within
about seven seconds
([Oakley, 2026](https://eclecticlight.co/2026/02/10/in-the-background-spotlight-indexing/)).
In another test the worker was active for only about 0.2 seconds, and
`mediaanalysisd` took most of the time
([Oakley, 2025](https://eclecticlight.co/2025/08/04/a-deeper-dive-into-spotlight-indexing-and-local-search/)).

Apple says a first full index can take hours or days, and later updates
go much faster
([Apple Support](https://support.apple.com/en-us/102321)). Oakley says
reading text and recognizing objects in images can take many days
([Oakley, 2024](https://eclecticlight.co/2024/11/29/using-and-troubleshooting-spotlight-in-sequoia-summary/)).
We found no Apple source on how Spotlight behaves on battery.

## What it reports

Users see a progress bar in the Spotlight window while it indexes
([Apple Support](https://support.apple.com/en-us/102321)). Oakley says
that bar appears only while search is unavailable, and otherwise
indexing shows no progress at all
([Oakley, 2024](https://eclecticlight.co/2024/11/19/when-and-how-to-rebuild-spotlight-indexes/)).
When a file fails to index, the only trace is a few lines in the system
log
([Oakley, 2025](https://eclecticlight.co/2025/10/26/last-week-on-my-mac-why-spotlight-cant-find-some-files/)).

Apps that search with `NSMetadataQuery` see a gathering phase with
progress notifications, then live updates, batched once a second by
default
([NSMetadataQuery](https://developer.apple.com/tutorials/data/documentation/foundation/nsmetadataquery.json)).

## Known problems

- Spotlight indexes no content for plain-text files that start with
  "LG", "HPA" or "Draw", from Mojave through Tahoe 26.0.1. It indexed no
  Rich Text content at all from macOS 10.15.6 to 11.3
  ([Oakley, 2025](https://eclecticlight.co/2025/10/26/last-week-on-my-mac-why-spotlight-cant-find-some-files/)).
- `mds_stores` can use CPU without stopping, especially with Time
  Machine volumes attached
  ([Oakley, 2022](https://eclecticlight.co/2022/12/08/spotlight-problems-mds_stores-and-mdworker-in-trouble/)).
- Users can't postpone indexing, and can't exclude a Time Machine disk
  while Spotlight indexes it
  ([Michael Tsai](https://mjtsai.com/blog/?p=36533)).
- Users reported `corespotlightd` writing terabytes overnight in Sequoia
  ([Michael Tsai, 2025](https://mjtsai.com/blog/2025/07/21/spotlight-indexing-running-wild)).

## Core Spotlight works differently

Apps add their own items to Core Spotlight, which keeps a separate index
for each app under `~/Library/Metadata/CoreSpotlight`
([Oakley, 2026](https://eclecticlight.co/2026/07/04/spotlight-and-core-spotlight-are-different/)).
Apple leaves recovery to the app. When the index is lost, Core Spotlight
tells the app, and the app reindexes everything and clears its own state
([CSSearchableIndexDelegate](https://developer.apple.com/tutorials/data/documentation/corespotlight/cssearchableindexdelegate.json)).
An app can save its own state with each batch it indexes, so it can pick
up where it stopped after a crash
([CSSearchableIndex](https://developer.apple.com/tutorials/data/documentation/corespotlight/cssearchableindex.json)).

## What the catalog could take from it

These are our own conclusions, not claims from the sources above.

- A bad file can't stop Spotlight, because each file runs in a worker it
  can throw away. We should copy that, and also give up on a file after
  a few failures. Spotlight's logs show what happens without that limit.
- We should record which files failed and why, somewhere
  `catalog status` and agents can see it, rather than only in a log.
- Spotlight's progress bar disappears once search works. Ours should
  always say how far behind the catalog is.
- Because FSEvents keeps its history across reboots, a daemon that
  stores the last event ID it handled can catch up after being stopped
  without crawling every tree.
- Our daemon may be stopped partway through a large batch. Saving its
  state with each batch, as Core Spotlight lets apps do, would let it
  resume where it stopped.
