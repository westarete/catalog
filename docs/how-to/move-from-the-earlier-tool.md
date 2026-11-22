# Move a repository from the earlier tool

This guide moves a repository from the earlier command-line tool to the
daemon. Move all your repositories in one sitting. Both tools are called
`catalog`, and `catalog status` means something different in each. Your
agents also need the updated `knowledge-work:catalog` skill first, so
none of them runs `catalog update` in a repository you've moved.

1. Remove the Homebrew `catalog` and install the new build in its place,
   as [Install and run the daemon](install-and-run.md) describes.
2. In each repository, delete `.catalog/catalog.db` and the old
   `.catalog.md` at the root. Keep `.catalog/config.toml`, and in it
   rename `enumerate`, the list of files to cover, to `include`, and
   `ignore`, the list of files to leave out, to `exclude`. Delete every
   other key the earlier tool's template wrote: `model`, `profile`,
   `input_price_per_mtok`, `output_price_per_mtok`,
   `cache_read_price_per_mtok`, `warn_request_tokens`, and
   `truncate_request_tokens`, or a `[pricing]` table from an older
   version. The daemon won't load a config that has a key it doesn't
   read, and `catalog status` shows a `Config error` line for it. The
   lists now add to the global and built-in ones instead of replacing
   them, so an `include` can't narrow what the catalog covers; to leave
   out a type of file, exclude it
   ([Configuration](../reference/configuration.md#how-lists-combine)).
   Then run `catalog init` in the repository. It writes
   `.catalog/.gitignore`, so Git ignores `catalog.md`, and writes
   `.catalog/config.toml` if the repository has none; it leaves an
   existing `config.toml` alone. Change the line that loads the catalog
   from `@.catalog.md` to `@.catalog/catalog.md`.
3. Add the folders that hold your repositories to the watch list in
   `global.toml`.
4. Load the launchd agent and check `catalog status`.

If your repository used to commit its catalog, Git now ignores the new
one because of the `.gitignore` that `catalog init` wrote, so it can't
conflict in a merge. In a shared repository, these steps change files
everyone sees, so tell the others when you move it.
