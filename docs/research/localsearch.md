# GNOME LocalSearch

LocalSearch is the file indexer in the GNOME desktop on Linux. Until
2024 its maintainers called it Tracker Miners, and called its database
library, now TinySPARQL, Tracker
([Carlos Garnacho, 2024](https://blogs.gnome.org/carlosg/2024/07/14/goodbye-tracker-hello-tinysparql-and-localsearch/)).
It splits the work in two: one process watches the filesystem and
records cheap facts about each file, and a separate, sandboxed process
reads file contents
([overview](https://gnome.pages.gitlab.gnome.org/localsearch/overview.html)).
Garnacho, one of its maintainers, has written openly about what went
wrong, so its trade-offs are better documented than most.

## Noticing changes

The indexer uses FANotify where it can and inotify elsewhere, including
on btrfs
([release notes](https://gitlab.gnome.org/GNOME/localsearch/-/raw/main/NEWS),
[Garnacho, 2022](https://blogs.gnome.org/carlosg/2022/09/11/tracker-3-x-a-retrospect/)).

In 2016 Garnacho described the crawl it runs at startup. It walks every
indexed folder to set up watches, compares each file's modification time
with the database, and checks each folder's modification time to catch
deletions. He called those checks a safety net: directory watches are a
limited resource, and the next startup catches up any folder the indexer
couldn't watch
([Garnacho, 2016](https://mail.gnome.org/archives/tracker-list/2016-March/msg00021.html)).
At that time the indexer crawled only at startup, so a setting named
`crawling-interval` did nothing until the process restarted
([tracker-list, 2016](https://lists.gnome.org/archives/tracker-list/2016-June/msg00005.html)).
The maintainers have kept working on this area since; several recent
releases change how the indexer checks folders after startup
([release notes](https://gitlab.gnome.org/GNOME/localsearch/-/raw/main/NEWS)).
An old issue describes one gap: the indexer can miss a directory created
during the first crawl, because it adds the watch only after reading the
directory
([issue 79](https://gitlab.gnome.org/GNOME/tracker-miners/issues/79)).

## Cheap facts first, content later

The filesystem indexer records each file's name and filesystem
attributes, and extractors then add metadata from the file's contents,
chosen by file type
([indexed data](https://gnome.pages.gitlab.gnome.org/localsearch/indexed-data.html)).
The maintainers moved extraction into its own process for stability and
resource use, before they sandboxed it
([Garnacho, 2023](https://blogs.gnome.org/carlosg/2023/10/10/on-cve-2023-43641/)).

Extraction is the slow part. The plain-text and MP3 extractors handle
thousands of files a second, while the libraries for PDF, media and TIFF
files can be up to four orders of magnitude slower
([Garnacho, 2022](https://blogs.gnome.org/carlosg/2022/09/11/tracker-3-x-a-retrospect/)).
A setting makes the extractor wait until the crawl finishes, so every
file is listed quickly and its contents are read later; it's off by
default. The extractor reads at most 1 MiB of text from each file by
default
([extractor settings](https://sources.debian.org/src/localsearch/3.12.0-1/data/org.freedesktop.Tracker.Extract.gschema.xml/)).

## Isolating the extractor

The extractor runs under a seccomp policy that blocks every system call
the maintainers haven't allowed, so it can't use the network, write
files or start processes
([Garnacho, 2023](https://blogs.gnome.org/carlosg/2023/10/10/on-cve-2023-43641/)).
Until 2023 they sandboxed only the threads that parse files. An attacker
used the unsandboxed main thread to escape through a bug in a library
for CUE sheets, CVE-2023-43641, and the maintainers responded by
sandboxing the whole process ([LWN](https://lwn.net/Articles/947288),
[Garnacho, 2023](https://blogs.gnome.org/carlosg/2023/10/10/on-cve-2023-43641/)).
Since release 3.7, Landlock also limits which files the extractor can
open, and each extraction task has a deadline
([release notes](https://gitlab.gnome.org/GNOME/localsearch/-/raw/main/NEWS)).

When a file crashes the extractor or makes a blocked system call, the
extractor exits, and on its next start it skips that file. LocalSearch
records the failure, and `tracker3 status` shows it
([Garnacho, 2022](https://blogs.gnome.org/carlosg/2022/09/11/tracker-3-x-a-retrospect/)).
The maintainers have avoided such files persistently since release 2.1.0
([release notes](https://gitlab.gnome.org/GNOME/localsearch/-/raw/main/NEWS)).
A user can clear a file's record and ask for it to be indexed again with
`localsearch reset --file`
([command line](https://gnome.pages.gitlab.gnome.org/localsearch/commandline.html)).
We found no documented rule for retrying failed files automatically.

## Keeping state

The documentation calls the database a cache of the filesystem that can
be thrown away and rebuilt, at a cost in CPU and disk work. If
TinySPARQL reports that the database is corrupt, the service exits and
restarts, and TinySPARQL tries to repair the database. If that fails, it
moves the old files aside and the indexer starts again from scratch
([handling corruption](https://gnome.pages.gitlab.gnome.org/localsearch/corruption.html)).

The database records which extractor produced each file's metadata
([commit](https://lists.gnome.org/archives/commits-list/2020-June/msg07645.html)),
and release 2.99.2 added automatic re-extraction when the extractors
change
([release announcement](https://lists.gnome.org/archives/ftp-release-list/2020-June/msg00062.html)).

## Scheduling and power

An application can ask the indexer over D-Bus to index a location right
away. The request lasts only while the application stays connected
([D-Bus API](https://gnome.pages.gitlab.gnome.org/localsearch/dbus-api.html)).
Since release 3.1, the indexer limits how much work it holds at once,
which keeps its memory at about 30 to 40 MB at peak and 5 MB when idle
([Garnacho, 2022](https://blogs.gnome.org/carlosg/2022/09/11/tracker-3-x-a-retrospect/)).
The extractor has also slowed down on battery power since release 3.6
([release notes](https://gitlab.gnome.org/GNOME/localsearch/-/raw/main/NEWS)).

In release 3.10 the maintainers deprecated a set of settings that
controlled when and how hard the indexer worked, including `throttle`,
`initial-sleep`, `crawling-interval` and `index-on-battery`
([3.10 notes](https://download.gnome.org/sources/localsearch/3.10/localsearch-3.10.alpha.news),
[indexer settings](https://sources.debian.org/src/localsearch/3.12.0-1/data/org.freedesktop.Tracker.Miner.Files.gschema.xml/)).
We didn't find their stated reason. An older issue that proposed
removing `throttle` argued that it mostly made the indexer sit idle
waiting, and that systemd's resource limits would do the job better
([issue 82](https://gitlab.gnome.org/GNOME/tracker-miners/issues/82)).

## What it reports

`localsearch status` shows the state of the index and a summary of
failed files, with details for any file you name, and `--follow` shows
each process's progress as it runs
([command line](https://gnome.pages.gitlab.gnome.org/localsearch/commandline.html)).
Over D-Bus, the indexer reports its status, progress and estimated time
remaining, and other programs can ask it to pause
([D-Bus interface](https://sources.debian.org/src/localsearch/3.12.0-1/data/org.freedesktop.Tracker3.Miner.xml/)).

## Known problems

- On one Framework laptop, battery life fell to about an hour,
  apparently from indexing cloud mounts and USB drives
  ([Framework forum](https://community.frame.work/t/tracking-new-heat-problems-battery-life-crashed-gnome-tracker-miner-fs-3/29620)).
- A Fedora 40 user saw constant high CPU use
  ([issue 340](https://gitlab.gnome.org/GNOME/tracker-miners/-/issues/340)),
  and an older ThinkPad slowed at every login from heavy disk use
  ([issue 236](https://gitlab.gnome.org/GNOME/tracker-miners/-/issues/236)).
- Users report stalled or crashing extractors, and files that keep
  failing and cause mostly unchanged files to be indexed again and again
  ([GNOME Discourse, 2025](https://discourse.gnome.org/t/a-more-comprehensive-localsearch-index-for-gnome-50/32030)).

## What the catalog could take from it

These are our own conclusions, not claims from the sources above.

- LocalSearch lists a file the moment it sees it and reads the contents
  later. That's the bottom level of the profiles that deepen over time
  in
  [profiles.md](../explanation/profiles.md#profiles-deepen-over-time),
  where an agent learns a file exists before any model has read it.
- We could borrow the extractor record directly: store the model and
  prompt behind each profile, and re-profile when either changes.
- For files the model can't handle, LocalSearch's approach fits well.
  Skip them, list them in the status, and give people a command to retry
  one.
- Battery is where a local model would hurt. LocalSearch's maintainers
  deprecated their throttle and battery settings, and users still report
  drain. A hosted model puts most of the cost on someone else's server,
  but with a local one we'll need to limit how much work we do on
  battery, not just lower its priority.
