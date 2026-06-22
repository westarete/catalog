## Code signing and notarization

The binary currently triggers a Gatekeeper warning on macOS because it
is not signed or notarized. These steps fix that.

- [x] Renew Apple Developer Program membership
- [ ] Generate a Developer ID Application certificate in the
      [Apple Developer portal](https://developer.apple.com/account/resources/certificates)
      and export it as a `.p12` file
- [ ] Generate an App Store Connect API key (`.p8` file) in
      [App Store Connect](https://appstoreconnect.apple.com/access/integrations/api)
- [ ] Store the certificate, certificate password, API key, API key ID,
      and API issuer ID as secrets in the `westarete/catalog` GitHub
      repository
- [ ] Switch the release workflow runner from `ubuntu-latest` to
      `macos-latest`
- [ ] Add a `notarize` section to `.goreleaser.yaml` referencing those
      secrets
- [ ] Tag a test release and verify the binary passes Gatekeeper without
      a warning
