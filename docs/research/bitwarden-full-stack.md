# Bitwarden as the full stack: primary-source research

Research for issue [#1636](https://github.com/rwade628/homelab/issues/1636) ("Research: Bitwarden as
the full stack"), a child of the map issue [#1634](https://github.com/rwade628/homelab/issues/1634).
Sources were read on 2026-10-17. Bracketed numbers like [S1] refer to the Sources list at the end.

## TL;DR verdict

**Yes, Bitwarden fits both halves, with one structural quirk and one maturity caveat.**

- **Machine secrets cost $0.** The Secrets Manager (SM) Free plan allows **2 users, 3 projects and 3
  machine accounts**, with unlimited secrets [S2][S3]. One project and one machine account for the
  cluster fit well inside that.
- **SM needs its own organization.** SM is attached to an *organization*. Bitwarden offers it only
  on Free, Teams and Enterprise orgs, **not on Families**. Bitwarden's own instruction to Families
  customers is to "simply create a new Free organization and sign up for Secrets Manager" [S3][S4].
  The same route works if the personal vault stays on Keeper: a free Bitwarden account can own a free
  org that holds only SM.
- **The personal vault fits.** Families costs **$47.88/yr** for 6 users, with Premium for everyone:
  in-vault TOTP generation, emergency access, and 5 GB personal plus 5 GB shared attachment storage.
  Passkeys are available on every tier, Free included. Premium alone costs $19.80/yr [S1].
- **The ESO provider is native but alpha.** `bitwardensecretsmanager` is in-tree in ESO, but it is
  labelled **alpha**, with one maintainer [S8]. Keeper's ESO provider has the same label. The
  provider also needs a sidecar service, `bitwarden-sdk-server`, which must run over **HTTPS**
  (cert-manager is recommended) [S6]. Its chart is a dependency of the ESO chart. ESO authenticates
  with a machine-account **access token** stored in a Kubernetes Secret; that token is the bootstrap
  credential [S6][S7].
- **TOTP can move over.** The standalone Bitwarden Authenticator imports **LastPass Authenticator
  JSON** on both iOS and Android [S12][S13]. It can also show the vault's TOTP codes through
  Password Manager to Authenticator sync [S11].
- **Imports lose some things.** Folders and TOTP seeds come across from LastPass and Keeper. Custom
  fields come across from Keeper JSON, and LastPass note fields become custom fields.
  **Attachments are never imported**, and neither file importer handles **passkeys** [S14]–[S18].
- **Security track record is strong but not spotless.** Bitwarden publishes many third-party audits
  (Cure53, Fracture Labs, ETH Zurich and others) [S19]. ETH Zurich's 2026 malicious-server paper
  found 12 attacks on Bitwarden; all were fixed, in remediation, or accepted as design decisions
  [S20][S21]. In April 2026 a malicious `@bitwarden/cli@2026.4.0` was published to npm for about
  90 minutes (CVE-2026-42994), with no vault data affected [S22].

## Cluster context (this repo)

- ESO is installed by `bootstrap/helmfile/apps.yaml` (release `external-secrets`, which `needs`
  `cert-manager/cert-manager`). cert-manager is therefore already available for the SDK server's
  TLS certificate.
- `flux-operator` still `needs: external-secrets/onepassword-connect`, a leftover from the template.
  Moving to Bitwarden would replace that dependency with the ESO chart's `bitwarden-sdk-server`
  sub-chart.

## 1. Password Manager tiers

From the pricing page [S1] (USD, billed annually, tax excluded):

| | Free | Premium | Families |
|---|---|---|---|
| Price | $0 | $1.65/mo, **$19.80/yr** | $3.99/mo, **$47.88/yr** |
| Users | 1 (plus "free sharing with another user") | 1 | **6 premium accounts** |
| Passkeys | "Passkey management and usage" | yes | yes |
| TOTP in vault | **store** keys only | store **and generate** ("Integrated authenticator") | store and generate |
| Emergency access | no | yes | yes |
| File attachments | no | 5 GB personal (expandable) | 5 GB personal + 5 GB family items (expandable) |
| Send | text only | text and files | text and files |

- On TOTP, the help docs say: "Storing keys in Password Manager integrated authenticator is
  available to all accounts", but "Generating TOTP codes is available with Premium or membership to
  a paid organization (Families, Teams, or Enterprise)" [S10]. Membership of a *free* org therefore
  does not unlock code generation.
- The plans help page says Families gives "all premium features for all six users" and "unlimited
  collections". A free org allows "two users" sharing "in up to two collections" [S5][S9].

## 2. Secrets Manager free tier, and the organization question

**Limits** (product page [S2] and plans help [S3]):

| SM plan | Price | Users | Projects | Machine accounts | Secrets |
|---|---|---|---|---|---|
| **Free** | $0 | up to 2 | up to 3 | up to 3 | unlimited |
| Teams | $6/user/mo | unlimited | unlimited | 20 included, then $1 each | unlimited |
| Enterprise | $12/user/mo | unlimited | unlimited | 50 included, then $1 each | unlimited |

**Does SM need a separate org? Yes, if the personal side is Families.**

- "Bitwarden offers Secrets Manager subscriptions for Free, Teams, and Enterprise organizations"
  [S3]. Families is not on that list.
- In the sign-up guide, the advice for Families customers is: "simply create a new Free organization
  and sign up for Secrets Manager" [S4]. You add SM from the org's **Admin Console → Billing →
  Subscription → Subscribe to Secrets Manager** [S4].
- The practical layout is one Bitwarden account (the human) that is a member of two orgs:
  1. A **Families org** for the personal and family vault (paid).
  2. A **Free org** that holds only Secrets Manager, with one project and one machine account for
     the cluster.
- A user can belong to several organizations; [S4] relies on this when it tells a Families customer
  to create a second org. None of the pages read states a cap on the number of free orgs per
  account. That is irrelevant here, because only one is needed.

## 3. ESO `bitwardensecretsmanager` provider

**Architecture** (ESO provider docs on `main` [S6]):

- ESO does not link the Rust SDK. It calls a separate REST wrapper,
  [`external-secrets/bitwarden-sdk-server`](https://github.com/external-secrets/bitwarden-sdk-server).
  The docs give the reason: "the bitwarden Rust SDK libraries are over 150MB in size" and need CGO.
- **Install:** set `bitwarden-sdk-server.enabled=true` on the ESO chart. The ESO `Chart.yaml` on
  `main` declares the dependency `bitwarden-sdk-server` **v0.6.0** from
  `oci://ghcr.io/external-secrets/charts`, behind `condition: bitwarden-sdk-server.enabled` [S6a].
  The service name is hardcoded to `bitwarden-sdk-server`.
- **TLS is mandatory.** The docs say: "The Bitwarden SDK Server _NEEDS_ to run as an HTTPS service
  ... The best approach for that is to use cert-manager." The SDK server repository's test setup
  includes a self-signed cert-manager issuer [S6]. The store trusts the certificate in one of two
  ways [S6b]:
  - `caBundle`: an inline base64 CA certificate.
  - `caProvider`: a reference to a Secret or ConfigMap. This is the better fit for GitOps, because
    the CA can come from the Secret that cert-manager writes. The SDK server repository has an open
    issue (#59) asking for a `caProvider` sample.

**Store spec** (`apis/externalsecrets/v1/secretsstore_bitwarden_types.go` [S6b]):

```yaml
spec:
  provider:
    bitwardensecretsmanager:
      apiURL: https://api.bitwarden.com            # optional
      identityURL: https://identity.bitwarden.com  # optional
      bitwardenServerSDKURL: https://bitwarden-sdk-server.external-secrets.svc.cluster.local:9998
      caProvider: {...}            # or caBundle
      organizationID: <uuid>       # required
      projectID: <uuid>            # required
      auth:
        secretRef:
          credentials: {name: ..., key: token}   # machine-account access token
```

- **Scope:** "1 store == 1 organization/project". To read across projects you need several stores,
  for example one `ClusterSecretStore` per project. The docs also say: "Make sure that the machine
  account has Read-Write access to the Project" [S6].
- **Auth and bootstrap:** the provider's only auth method is a machine-account **access token** held
  in a Kubernetes Secret. That Secret is the one credential that must exist before ESO can work, so
  it is the bootstrap secret, and it would stay in SOPS or be applied by hand under the map's
  bootstrap plan. Properties of access tokens [S7]:
  - Expiry defaults to "Never" and can be set when the token is created.
  - A token is shown once: it "will not be stored and cannot be retrieved later".
  - Tokens can be revoked. After revocation, an already-authenticated client can keep access "for up
    to one hour".
- **Lookup semantics:** `remoteRef.key` is either a UUID or a **name**. A name lookup is scoped to
  the store's org and project. It errors if two secrets share the name, which Bitwarden allows.
- **Known gap:** `dataFrom.find` returns *every* secret in the org, keyed by ID, and ignores the
  `name.regexp`, `tags` and `path` selectors. This is ESO issue
  [#6550](https://github.com/external-secrets/external-secrets/issues/6550), still OPEN.
- **PushSecret** is supported. Because duplicate names are allowed, the provider creates a new secret
  unless name, project, org, value and note all match.

**Maturity:**

- The ESO stability table ([S8], `docs/introduction/stability-support.md` on `main`) lists Bitwarden
  Secrets Manager as **alpha**, maintained by **@skarlso**. Keeper Security and both 1Password
  providers are also alpha.
- Feature matrix from the same table:

  | Provider | find by name | find by tags | store validation | push secret | DeletionPolicy |
  |---|---|---|---|---|---|
  | Bitwarden SM | x | | x | x | x |
  | Keeper | x | | x | x | |

- `bitwarden-sdk-server` [S6c]: Apache-2.0, 38 stars, 2 open issues.
  - Latest release **v0.8.1** (2026-10-10). Earlier releases: v0.8.0 (2026-10-01), v0.7.0
    (2026-06), v0.6.0 (2026-02), and earlier ones back to 2025.
  - The ESO chart on `main` still pins **v0.6.0**. To use a newer server you would override the
    sub-chart's image tag or deploy the chart separately.
  - Open issue #70 reports that the chart's `ingress` values do nothing. This is irrelevant here,
    because the server should stay in-cluster only.

## 4. Bitwarden Authenticator vs TOTP in the vault

- **Bitwarden Authenticator** is a standalone app for iOS 15+ and Android 9+ [S12]. It works "with
  or without a Bitwarden Password Manager account". Its codes live in "a local unencrypted database
  on your device", which "is not synced to Bitwarden servers". Backup relies on the device's cloud
  backup or a manual export.
- **TOTP in the vault** (the integrated authenticator) syncs with the vault and fills codes on every
  client. Generating codes needs Premium or a paid org [S10].
- **Bridging the two** [S11], available since 2025.6.2 on iOS and Android 12+:
  - In Password Manager, turn on **Settings → Account security → Allow authenticator syncing**.
    Vault codes then appear in Authenticator. Sync runs in that direction only, and the synced codes
    stop showing if Password Manager logs out.
  - Codes created in Authenticator stay local. You can copy one into the vault with "Copy to
    Bitwarden vault".
- **Importing from LastPass Authenticator: yes.** The Authenticator source code lists these import
  formats:
  - **iOS** [S12a]: Authenticator Export (JSON), Google Authenticator (QR), **LastPass (JSON)**,
    Raivo (JSON), 2FAS (no password).
  - **Android** [S12b]: Bitwarden JSON, 2FAS JSON, **LastPass JSON**, Aegis.

  The LastPass parser has its own test suite in both repositories.
- **Implication for the map:** with Families, TOTP can live in the vault, so codes sync and fill on
  every client and an emergency-access path exists. Authenticator then serves as the phone-side
  viewer, and as the import path for the LastPass Authenticator JSON export. Codes for the
  Bitwarden account's own 2FA should not depend on that same login. [S11] warns about this
  explicitly.

## 5. Importing from LastPass and Keeper

| Data | From LastPass | From Keeper |
|---|---|---|
| Folders | `grouping` becomes a folder, or collections if importing into an org (free org limit is 2 collections) [S14][S15] | Folders and shared folders become folders, or collections in an org [S16][S17] |
| TOTP seed | `totp` CSV column goes to `login.totp` [S15] | First `oneTimeCode` goes to `login.totp`; extra codes become hidden custom fields named "TOTP" [S17] |
| Custom fields | Secure-note template fields (`key:value` lines) become custom fields [S15] | Custom fields are kept; names and types are used as-is [S16] |
| Attachments | **Not imported**; "need to be manually uploaded" [S14] | **Not imported** ("File attachments (not supported)") [S16][S18] |
| Passkeys | Not handled by the CSV importer [S15] | Not handled by the JSON, CSV or direct importer (no passkey or FIDO code) [S17] |
| Sends | Not imported [S14] | Not imported [S18] |

- **LastPass** can be imported two ways [S14]:
  - "Direct import" from the browser extension or desktop app.
  - A CSV file.

  Neither checks for duplicates. Item mapping [S15]:
  - Credit Card notes become Card items, and Address notes become Identity items.
  - Behind feature flag `PM32009NewItemTypes`, Bank Account, Passport and Driver's License notes
    become new dedicated types (see §6).
  - All other note types, including LastPass "Wi-Fi Password", "Server", "SSH Key" and "Database",
    become **Secure notes** whose fields are custom fields.
- **Keeper** can be imported from a JSON file, a CSV file, or the newer **direct importer**, all in
  `libs/importer/src/importers/keeper/` [S16][S17]. The README maps Keeper record types:
  - `sshKeys` becomes a native **SshKey** item, or a Secure note if the key does not parse.
  - `wifiCredentials`, `serverCredentials` and `databaseCredentials` become **Login** items.
  - `file` and `photo` records become Secure notes without their bytes.
  - Keeper JSON keeps shared folders and custom record types.
- **Passkeys** need a different route: the FIDO Credential Exchange Protocol (CXP), which runs
  device-to-device on iOS 26+ and newer Android. Both apps must support it, and passkeys that use a
  signature counter cannot be transferred [S23]. These sources do not establish whether Keeper or
  LastPass can export over CXP. For a few passkeys, re-enrolling on each site is the reliable path.

## 6. Item types

- The help docs list **five item types**: Login, Card, Identity, Secure note and **SSH key**. The
  SSH key type works with Bitwarden's SSH agent [S24].
- The source enum (`libs/common/src/vault/enums/cipher-type.ts`) also contains `BankAccount`,
  `DriversLicense` and `Passport` [S25]. These are behind a feature flag and not yet in the help
  docs.
- There is **no native Wi-Fi or Server type**. Wi-Fi networks and servers become a Login (username,
  password and URI, plus custom fields such as SSID, security type or host) or a Secure note with
  custom fields. Bitwarden's own Keeper importer makes the same choice, mapping Wi-Fi and server
  records to Login [S16].
- An SSH agent is out of scope for the map. The SSH key type is still useful as a typed store for
  key material.

## 7. Security track record and audits

- **Published third-party assessments** [S19]:
  - 2025: Cure53 (browser extension, core app, desktop app, RustCrypto crate and library, web
    vault), ETH Zurich cryptography report, Unit 42 (mobile), Fracture Labs (web app and network).
  - 2024: IOActive, Mandiant, Fracture Labs, Cure53, PIE.
  - 2018–2023: Cure53 and Insight Risk Consulting.
  - Compliance: SOC 2 Type 2 and SOC 3, ISO 27001, and an annual third-party HIPAA audit.
  - No audit specific to Secrets Manager is listed. SM uses the same core, SDK and server stack that
    these audits cover.
- **ETH Zurich, "Zero Knowledge (About) Encryption"** (IACR ePrint 2026/058; USENIX Security '26)
  [S20]:
  - Threat model: a *fully malicious server*. The paper found 12 attacks on Bitwarden, 7 on LastPass
    and 6 on Dashlane.
  - Bitwarden's findings included unauthenticated org/sharing public keys, downgrade through legacy
    code paths, and field-level vault integrity.
  - Bitwarden's response (2026-02-16) [S21]: severity was medium or low. Seven issues were resolved
    or are in remediation, and three were accepted as intentional design decisions.
- **Supply-chain incident, 2026-04-22** [S22]:
  - A malicious `@bitwarden/cli@2026.4.0` was distributed through npm from 5:57 to 7:30 PM ET, as
    part of the broader Checkmarx/"Shai-Hulud" campaign. It was a credential stealer that also
    targeted `~/.claude.json` and MCP configs, per third-party analyses.
  - Bitwarden found "no evidence that end user vault data was accessed". CVE-2026-42994 was issued
    and 2026.4.1 is the clean release.
  - Lesson for this stack: install `bw` and `bws` from Nix or pinned GitHub release binaries, not
    from npm `latest`.
- **Comparison with Keeper** (for the Keeper SM ticket):
  - Both ESO providers are alpha.
  - Bitwarden's clients, server and SDK are open source, and its audit reports are published as
    PDFs.
  - Bitwarden has a $0 machine-secrets path. The map notes say Keeper's KSM looks business-only.

## 8. Family sharing

**How sharing works**

- **Families org** [S5][S9]:
  - Up to **6 users**, **unlimited collections**, and Premium for all members.
  - Each member keeps a **private personal vault**. Shared items live in the org's collections, and
    a member sees only the collections assigned to them.
  - Each collection assignment carries one permission level [S26]: View items, View items with
    hidden passwords, Edit items, Edit items with hidden passwords, or Manage collection.
  - Roles are Owner, Admin and User. Custom roles are Enterprise-only [S27].
- **Free and Premium**: an individual can create a **free org** for "two users" sharing in "up to
  two collections" [S5][S9]. Free-org members do not get Premium features. Premium is a single-user
  plan, and sharing it with another person still goes through a free org.
- **Compared with Keeper Family**: Bitwarden's collections are roughly Keeper's shared folders,
  and the 6-seat limit is the main constraint to check against the family's size.

**Sharing with people not on Bitwarden**

- **Send** [S28]:
  - End-to-end encrypted, temporary links: text up to 1,000 characters, or files up to 500 MB
    (100 MB on mobile).
  - Lifespan of at most **31 days**, with an optional password and access-count limit.
  - Free accounts can send **text only**. File Sends need Premium [S9].
  - Recipients open a link. The help page does not mention a recipient account.
- **No ongoing cross-vendor sharing.** None of the Bitwarden pages read describe a live shared item
  between a Bitwarden vault and another vendor's vault. Org sharing requires every member to have a
  Bitwarden account, and Send is one-off and expiring. CXP [S23] is a one-time migration channel,
  not a sync.
- **Consequence:** if part of the family stays on Keeper, shared items would have to be duplicated
  by hand or sent one at a time. A mixed-vendor family vault is not a supported steady state.

**SM in a separate free org while the vault stays elsewhere (for example Keeper)**

- **Supported, and $0.**
  - Owning any org requires a Bitwarden account. A **free** individual account is enough.
  - From that account, create a **Free org** and subscribe it to Secrets Manager at no cost [S3][S4].
    Bitwarden's Families guidance describes exactly this pattern of a separate free org.
  - Nothing in the SM docs requires the account to hold any Password Manager data. The cluster uses
    only the SM project and the machine-account token.
- **Costs of this split:**
  - One more master password and 2FA setup to protect. That login controls the cluster's secrets, so
    it needs strong 2FA and a recovery-code plan.
  - The SM web app is the editing UI for cluster secrets.
  - The free tier's 2 *users* means you plus one other human with SM access. Machine accounts do
    not count toward that limit.

## Open points for the decision ticket (#1640)

- Bootstrap: where the one SM access token lives before ESO runs (SOPS vs a manual `kubectl create`
  through a wizard). This choice determines whether `sops-age` can ever be fully removed.
- Whether to pin `bitwarden-sdk-server` above the chart's v0.6.0.
- One project or several. Each project needs its own store. With `dataFrom.find` ignoring selectors
  (#6550), prefer explicit `data:` entries by name or UUID.
- Families ($47.88/yr) vs Premium ($19.80/yr). This depends on whether the family moves off Keeper.

## Sources

Bitwarden (first-party):

- [S1] Pricing: https://bitwarden.com/pricing/
- [S2] Secrets Manager product and pricing: https://bitwarden.com/products/secrets-manager/
- [S3] Secrets Manager plans: https://bitwarden.com/help/secrets-manager-plans/
- [S4] Sign up for Secrets Manager: https://bitwarden.com/help/sign-up-for-secrets-manager/
- [S5] Organizations overview: https://bitwarden.com/help/about-organizations/
- [S7] SM access tokens: https://bitwarden.com/help/access-tokens/
- [S9] Password Manager plans: https://bitwarden.com/help/password-manager-plans/
- [S10] Integrated authenticator: https://bitwarden.com/help/integrated-authenticator/
- [S11] Sync verification codes: https://bitwarden.com/help/totp-sync/
- [S12] Bitwarden Authenticator: https://bitwarden.com/help/bitwarden-authenticator/
- [S12a] iOS import formats: https://github.com/bitwarden/ios/blob/main/AuthenticatorShared/Core/Vault/Models/Enum/ImportFormatType.swift
- [S12b] Android import formats: https://github.com/bitwarden/android/blob/main/authenticator/src/main/kotlin/com/bitwarden/authenticator/ui/platform/util/ImportFormatExtensions.kt
- [S13] Android LastPass parser: https://github.com/bitwarden/android/blob/main/authenticator/src/main/kotlin/com/bitwarden/authenticator/data/platform/manager/imports/parsers/LastPassExportParser.kt
- [S14] Import from LastPass: https://bitwarden.com/help/import-from-lastpass/
- [S15] LastPass CSV importer source: https://github.com/bitwarden/clients/blob/main/libs/importer/src/importers/lastpass/lastpass-csv-importer.ts
- [S16] Keeper importer README: https://github.com/bitwarden/clients/blob/main/libs/importer/src/importers/keeper/README.md
- [S17] Keeper importers (JSON, CSV, direct): https://github.com/bitwarden/clients/tree/main/libs/importer/src/importers/keeper
- [S18] Import from Keeper: https://bitwarden.com/help/import-from-keeper/
- [S19] Compliance, audits and certifications: https://bitwarden.com/help/is-bitwarden-audited/
- [S21] Bitwarden on the ETH Zurich audit (2026-02-16): https://bitwarden.com/blog/security-through-transparency-eth-zurich-audits-bitwarden-cryptography/
- [S22] Bitwarden statement on the Checkmarx supply-chain incident: https://community.bitwarden.com/t/bitwarden-statement-on-checkmarx-supply-chain-incident/96127 (and https://github.com/bitwarden/clients/issues/20353)
- [S23] Passkeys, import and export: https://bitwarden.com/help/storing-passkeys/, https://bitwarden.com/help/export-your-data/
- [S24] Managing items: https://bitwarden.com/help/managing-items/
- [S25] CipherType enum: https://github.com/bitwarden/clients/blob/main/libs/common/src/vault/enums/cipher-type.ts
- [S26] Collection permissions: https://bitwarden.com/help/collection-permissions/
- [S27] Member roles: https://bitwarden.com/help/user-types-access-control/
- [S28] About Send: https://bitwarden.com/help/about-send/

External Secrets Operator:

- [S6] Provider docs: https://github.com/external-secrets/external-secrets/blob/main/docs/provider/bitwarden-secrets-manager.md (published at https://external-secrets.io/latest/provider/bitwarden-secrets-manager/)
- [S6a] Chart dependency: https://github.com/external-secrets/external-secrets/blob/main/deploy/charts/external-secrets/Chart.yaml
- [S6b] Store types: https://github.com/external-secrets/external-secrets/blob/main/apis/externalsecrets/v1/secretsstore_bitwarden_types.go
- [S6c] bitwarden-sdk-server: https://github.com/external-secrets/bitwarden-sdk-server (releases, issues #59 and #70)
- [S8] Stability and support table: https://github.com/external-secrets/external-secrets/blob/main/docs/introduction/stability-support.md
- ESO issue #6550: https://github.com/external-secrets/external-secrets/issues/6550

Third-party research:

- [S20] ETH Zurich Applied Cryptography Group and USI Lugano, "Zero Knowledge (About) Encryption", IACR ePrint 2026/058: https://eprint.iacr.org/2026/058.pdf
