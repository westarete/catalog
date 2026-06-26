# Apple code signing setup

This document records the steps to set up macOS code signing and
notarization for the catalog release pipeline. Follow them in order.
They assume you are the Apple Developer Program Account Holder for West
Arete Computing, Inc. (Team ID: HH6PJ9ECM).

## Background: signing vs. notarization

These are two separate things that are easy to conflate.

**Code signing** attaches a cryptographic signature to the binary using
your Developer ID Application certificate. macOS uses it to verify the
binary came from a known developer and hasn't been tampered with.

**Notarization** is an additional step where Apple scans your binary for
malware and issues a ticket. Stapling that ticket to the binary means
Gatekeeper can verify it even without an internet connection, and
satisfies stricter enterprise security policies.

On macOS 15+, **signing alone is not sufficient** to clear the
Gatekeeper dialog. The dialog that reads "Apple could not verify catalog
is free of malware" is the notarization check — it fires even on a
properly signed binary. Notarization is required to run without any
Gatekeeper prompt.

Full notarization is also needed for:

- Enterprise Macs with strict MDM policies
- Offline environments where Gatekeeper can't phone home
- Official Homebrew (homebrew/cask) submission, which requires it

Useful background:

- [Signing Mac Software with Developer ID — Apple Developer](https://developer.apple.com/developer-id/)
- [Customizing the notarization workflow — Apple Developer Documentation](https://developer.apple.com/documentation/security/customizing-the-notarization-workflow)
- [Automatic Code-signing and Notarization for macOS apps using GitHub Actions — Federico Terzi](https://federicoterzi.com/blog/automatic-code-signing-and-notarization-for-macos-apps-using-github-actions/)

## Prerequisites

- An Apple Developer Program membership ($99/year). Renew at
  [developer.apple.com](https://developer.apple.com). Auto-renew is
  currently off — next renewal is June 22, 2027.
- Admin access to the `westarete/catalog` GitHub repository.

## Step 1 — Generate a certificate signing request

The CSR proves to Apple that you hold the private key that will be
paired with the certificate they issue. You generate it locally in
Keychain Access, which also creates and stores the private key.

1. Open **Keychain Access**
   (`/Applications/Utilities/Keychain Access.app`)
2. From the menu: **Keychain Access → Certificate Assistant → Request a
   Certificate from a Certificate Authority**
3. Fill in the dialog:
   - **User Email Address**: your Apple Developer account email (e.g.
     `user@westarete.com`)
   - **Common Name**: a label for the private key in your keychain (e.g.
     `West Arete Signing Key`) — this is internal only, users never see
     it
   - **CA Email Address**: leave empty
   - **Request is**: Saved to disk
4. Click **Continue** and save the `.certSigningRequest` file somewhere
   temporary (e.g. `~/Downloads`)

Note: do not install Xcode just for this. The Xcode route automates the
CSR but is a 10GB download. The manual route above takes two minutes.

## Step 2 — Generate the Developer ID Application certificate

The Developer ID Application certificate is per-developer (or
organization), not per-app. You use the same certificate to sign any
number of binaries.

1. Go to
   [Certificates, Identifiers & Profiles → Certificates](https://developer.apple.com/account/resources/certificates/list)
   in the Apple Developer portal
2. Click **+** to create a new certificate
3. Choose **Developer ID Application** and click **Continue**
4. Choose **G2 Sub-CA (Xcode 11.4.1 or later)** and click **Continue** —
   the Previous Sub-CA is only needed for Xcode versions before 11.4.1
5. Upload the `.certSigningRequest` file from Step 1
6. Download the resulting `.cer` file

## Step 3 — Install the intermediate certificate

The Developer ID G2 intermediate certificate must be installed before
your certificate will be trusted locally. Without it, Keychain Access
shows a red X on the certificate and the .p12 export option is
unavailable.

1. Download **Developer ID - G2 (Expiring 09/17/2031)** from
   [Apple PKI](https://www.apple.com/certificateauthority/DeveloperIDG2CA.cer)
2. Double-click the downloaded `.cer` file to install it in Keychain
   Access

## Step 4 — Install the certificate and export as .p12

The `.p12` format bundles the certificate and private key together — CI
needs both to sign binaries.

1. Double-click the `.cer` file from Step 2 to install it in Keychain
   Access. It will appear under the **My Certificates** category with
   the private key nested beneath it as a disclosure triangle.
2. In Keychain Access, click **My Certificates** in the left sidebar
3. Find **Developer ID Application: West Arete Computing, Inc.**
4. Right-click it and choose **Export**
5. Set the format to **Personal Information Exchange (.p12)**
6. Choose a filename and save location
7. Set a strong password — you will need this when storing the
   certificate as a GitHub secret

Store the `.p12` file and its password in the West Arete 1Password vault
so other team members can access them. You will also need them when
storing secrets in GitHub.

## Step 5 — Get an App Store Connect API key for notarytool

Apple's notarization service requires authentication. There are two
options: App Store Connect API key, or Apple ID + app-specific password.

We initially chose app-specific password to avoid the App Store Connect
API ToS, which restricts use to internal team workflows and does not
clearly cover publishing open source software publicly. However, after
persistent HTTP 500 errors across multiple days, we switched to API key
authentication. Quinn "The Eskimo!", a well-known Apple DTS engineer,
[explicitly recommends API key auth over app-specific passwords for notarytool](https://developer.apple.com/forums/thread/816270)
when the latter produces 500 errors. The ToS restriction is aimed at
reselling the API to third parties — notarizing your own software in
your own CI pipeline is not that use case.

**Getting the API key:**

The App Store Connect API requires an opt-in step. Go to
[App Store Connect → Users and Access → Integrations](https://appstoreconnect.apple.com/access/integrations/api)
and click **Request Access**. The Account Holder must do this. Once
approved:

1. Click **+** to generate a new key
2. Give it a name (e.g. `westarete/catalog notarization`) and assign the
   **Developer** role — sufficient for notarization
3. Download the `.p8` file immediately — it is shown only once
4. Note the **Key ID** (10-character alphanumeric string)
5. Note the **Issuer ID** shown at the top of the page (UUID format)

Store the `.p8` file, Key ID, and Issuer ID in the West Arete 1Password
vault.

## Step 6 — Store secrets in GitHub

Go to the `westarete/catalog` repository → **Settings → Secrets and
variables → Actions** and add these as **repository secrets**.

| Secret                       | Value                                                                                                                                                         |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `HOMEBREW_TAP_TOKEN`         | fine-grained PAT with Contents read/write on `westarete/homebrew-tap` and `westarete/catalog`; used by GoReleaser to push the updated cask after each release |
| `MACOS_CERTIFICATE`          | base64-encoded `.p12` file: `base64 -i cert.p12 \| pbcopy`                                                                                                    |
| `MACOS_CERTIFICATE_PASSWORD` | the `.p12` export password                                                                                                                                    |
| `MACOS_NOTARY_ISSUER_ID`     | the Issuer ID UUID from App Store Connect                                                                                                                     |
| `MACOS_NOTARY_KEY`           | base64-encoded `.p8` file: `base64 -i key.p8 \| pbcopy`                                                                                                       |
| `MACOS_NOTARY_KEY_ID`        | the 10-character Key ID from App Store Connect                                                                                                                |

## Step 7 — Update the release workflow and GoReleaser config

The release workflow uses
[GoReleaser's native `notarize.macos` block](https://goreleaser.com/customization/sign/notarize/),
which handles both signing and notarization via
[anchore/quill](https://github.com/anchore/quill). This is the same
approach used by [junegunn/fzf](https://github.com/junegunn/fzf).

Quill reads credentials directly from environment variables — no
Keychain setup or `notarytool store-credentials` step needed. The
workflow passes the five signing and notarization secrets as env vars to
the GoReleaser action, and the `.goreleaser.yaml` `notarize.macos` block
picks them up.

## Known issue: HTTP 500 from Apple's notarization service

Apple's notarization service periodically returns HTTP 500 errors during
notarization submissions even when credentials are correct. This is a
server-side issue, not a configuration problem.

```text
Error: HTTP status code: 500. Internal Server Error
Request ID: ...
Please try again at a later time.
```

If this happens: wait and retry. The service typically recovers on its
own.

We use App Store Connect API key authentication (Step 5) rather than
app-specific password for this reason. Quinn "The Eskimo!" from Apple
DTS
[explicitly recommends API keys over app-specific passwords](https://developer.apple.com/forums/thread/816270)
when the latter produces persistent 500s.

References:

- [notarytool returns HTTP 500 — Apple Developer Forums](https://developer.apple.com/forums/thread/816270)
- [Notary server down - 500 internal — Apple Developer Forums](https://developer.apple.com/forums/thread/706539)
