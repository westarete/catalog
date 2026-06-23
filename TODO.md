## Code signing and notarization

The binary currently triggers a Gatekeeper warning on macOS because it
is not signed or notarized. See [APPLE.md](APPLE.md) for background on
the difference between signing and notarization, and why we are
implementing signing first.

### Signing (in progress)

- [x] Renew Apple Developer Program membership
- [x] Generate a Developer ID Application certificate and export as
      `.p12` (see [APPLE.md](APPLE.md) Steps 1–4)
- [x] Create an app-specific password for `notarytool` at
      [appleid.apple.com](https://appleid.apple.com) (see
      [APPLE.md](APPLE.md) Step 5)
- [x] Store the 7 required secrets in the `westarete/catalog` GitHub
      repository (see [APPLE.md](APPLE.md) Step 6)
- [x] Switch the release workflow runner from `ubuntu-latest` to
      `macos-latest`
- [x] Add a GoReleaser build post-hook to call `codesign` on each macOS
      binary using `{{ .Path }}`
- [x] Add workflow steps to import the certificate into a temporary CI
      keychain and store `notarytool` credentials before GoReleaser runs
- [x] Tag a test release and verify the binary passes Gatekeeper without
      a warning

### Notarization (follow-up)

Signing alone clears the Gatekeeper dialog for most users. Full
notarization additionally covers offline Gatekeeper checks and
enterprise MDM policies, and is required for submission to the official
homebrew/cask tap.

- [ ] Add a post-release workflow step to submit each macOS archive to
      Apple's notarization service via `notarytool submit --wait`
- [ ] Staple the notarization ticket to each binary with `xcrun stapler`
- [ ] Repackage the archives with the stapled binaries and update the
      GitHub Release
- [ ] Tag a test release and verify notarization passes
