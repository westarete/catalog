# Short-term development plan for Catalog

## Code signing and notarization

The binary currently triggers a Gatekeeper warning on macOS because it
is not signed or notarized. See [APPLE.md](APPLE.md) for background on
the difference between signing and notarization, and why we are
implementing signing first.

### Signing

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
- [x] Tag a test release and verify the binary is signed

### Notarization

On macOS 15+, signing alone is not sufficient — Gatekeeper shows a
"Apple could not verify catalog is free of malware" dialog even on a
properly signed binary. Notarization is required to clear it, and also
covers offline Gatekeeper checks, enterprise MDM policies, and official
homebrew/cask submission.

The approach follows [junegunn/fzf](https://github.com/junegunn/fzf):
use GoReleaser's native `notarize.macos` block (open-source, no Pro
license needed), which signs and notarizes each binary before archiving
via [anchore/quill](https://github.com/anchore/quill). The workflow no
longer needs manual keychain setup steps — Quill reads credentials
directly from environment variables.

- [x] Replace the `codesign` build post-hook with a `notarize.macos`
      block in `.goreleaser.yaml`
- [x] Remove the keychain import and `notarytool store-credentials`
      steps from the release workflow; pass signing and notarization
      secrets as env vars instead
- [ ] In the GitHub repo secrets (Settings → Secrets and variables →
      Actions): rename `MACOS_CERTIFICATE_PWD` →
      `MACOS_CERTIFICATE_PASSWORD`. Delete `MACOS_CERTIFICATE_NAME` and
      `MACOS_CI_KEYCHAIN_PWD` — they are no longer used.
- [ ] Tag a test release and verify the binary passes Gatekeeper without
      a warning
