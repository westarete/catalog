# Move a repository from the earlier tool

This guide moves a repository from the earlier command-line tool to the
daemon. Move all your repositories in one sitting. Both tools are called
`catalog`, and `catalog status` means something different in each. Your
agents also need the updated `knowledge-work:catalog` skill first, so
none of them runs `catalog update` in a repository you've moved.

1. Remove the Homebrew `catalog` and install the new build in its place,
   as [Install and run the daemon](install-and-run.md) describes.
2. In each repository, delete `.catalog/catalog.db` and the old
   `.catalog.md` at the root. Keep `.catalog/config.toml`; the daemon
   reads its `enumerate` and `ignore`. Change the line that loads the
   catalog from `@.catalog.md` to `@.catalog/catalog.md`.
3. Add the folders that hold your repositories to the watch list in
   `global.toml`.
4. Load the launchd agent and check `catalog status`.

If your repository used to commit its catalog, you will probably want
Git to ignore `.catalog/catalog.md` now, since that's the file that used
to conflict. In a shared repository, these steps change files everyone
sees, so tell the others when you move it.
