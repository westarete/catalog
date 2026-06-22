What needs doing next:

1. Create the `westarete/homebrew-tap` repository on GitHub if it
   doesn't exist. It needs the standard Homebrew tap structure: a
   `Casks/` directory where GoReleaser will write the formula. A minimal
   README explaining it's a tap and how to add it
   (`brew tap westarete/tap`) is all it needs.

1. Create the `HOMEBREW_TAP_TOKEN` secret. It's a fine-grained PAT
   scoped to both repos with Contents read/write permission. It goes in
   `westarete/catalog`'s repository secrets.

1. Do a test release. Tag `v0.1.0`, push the tag, watch the Actions
   workflow, and verify the cask formula lands in the tap repo.
