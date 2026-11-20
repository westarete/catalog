# Catalog format

`.catalog/catalog.md` is the file an agent reads to decide which
documents to open. The daemon writes it in a fixed shape: a header, then
one section for each directory, then one subsection for each file. This
page describes that shape. For where the file lives and who can read it,
see [files](files.md); for what a summary is and how it is made, see
[profiles](../explanation/profiles.md).

## Shape

```text
<header>

## <dir>/

### <path>

<entry>
```

The daemon rewrites the whole file each time it publishes. A catalog
with no entries has only the header.

### Header

Every file starts with this text, exactly, with each paragraph on one
line:

```text
# Catalog

This is an index of the documents in this folder. Read the entries below, and open a file only when your task matches its entry.

Paths are relative to the folder that holds the .catalog/ folder.

An entry marked "Not summarized yet." has no summary. It shows only the start of the file.
```

### Directories

Each directory that holds a covered file gets a `## <dir>/` heading,
once, before its files. `<dir>` is the file's path up to its last `/`,
followed by `/`. A file at the top of the catalog's folder is under
`## (root)/`. A directory's heading names the whole path, so
`notes/2026/` is a heading of its own and not a subheading of `notes/`.

Directories are in string order of their headings' text, and the files
in a directory are in string order of their paths. Because of that, a
file sits under the heading for its own directory, and nothing nested
falls between a directory's files.

### Paths

A path is the file's path relative to the folder that holds `.catalog/`,
written with `/` separators and no leading `./` or `/`. Each file is a
`### <path>` heading. The path is written as it is on disk, with no
escaping.

### Entries

Under a file's heading comes a blank line, then one of two entries.

A summarized entry is the file's summary, with leading and trailing
whitespace removed, followed by a newline. The daemon writes the summary
as the model returned it; it doesn't add to it or escape it. If a
summary for the file's current content under the current recipe isn't
stored but an older one is, the older one appears until the file is
summarized again.

A listing-only entry is for a file with no summary yet. Its first line
is `Not summarized yet.`, then the file's type and size in bytes:

```text
Not summarized yet. <type>, <size> bytes.
```

The type is `Markdown`, `PDF`, `text`, or `CSV` for a file ending in
`.md`, `.pdf`, `.txt`, or `.csv`, in any letter case. For any other
extension it is the extension without its dot, and for no extension it
is `file`. The size is the size the last crawl recorded.

If the file isn't a PDF and has any readable start, a blank line and a
fenced block follow, holding the file's first lines: at most 10 lines
and at most 512 bytes, cut on a character boundary, with invalid UTF-8
replaced by the replacement character. The fence opens with `text` and
is at least three backticks, and longer than any run of backticks in
those lines, so the lines can't close it early. A PDF shows no lines,
and neither does a file that is empty or can't be read when the daemon
publishes.

A file stays a listing-only entry when it can't be summarized, such as
an empty PDF or a PDF over the size the daemon sends.

## Example

A catalog with three files, made up for the example, one of them waiting
for its summary:

````text
# Catalog

This is an index of the documents in this folder. Read the entries below, and open a file only when your task matches its entry.

Paths are relative to the folder that holds the .catalog/ folder.

An entry marked "Not summarized yet." has no summary. It shows only the start of the file.

## (root)/

### budget.csv

Household budget by month for 2026. Open it to check what was planned
against what was spent.

## notes/

### notes/garden.md

Not summarized yet. Markdown, 214 bytes.

```text
# Garden plan

Beds by the fence get tomatoes.
Herbs go near the kitchen door.
```

### notes/taxes.pdf

Not summarized yet. PDF, 48213 bytes.
````
