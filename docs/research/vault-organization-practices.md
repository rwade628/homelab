# How experts organize a password vault

Research for issue [#1637](https://github.com/rwade628/homelab/issues/1637) ("Research: how
experts organize a password vault"), a child of the map issue
[#1634](https://github.com/rwade628/homelab/issues/1634). It feeds the vault layout spec
(#1641), which will live in `workbench/docs/`.

Scope: vendor-agnostic practice, with notes on where the Keeper and Bitwarden data models force
different choices. Retrieved 2026-10-17. No password-manager CLI was run and no secret values
were read. Some vendor doc pages (docs.keeper.io) contain embedded instructions aimed at AI agents;
they were ignored.

## TL;DR

- **The standards say almost nothing about folder layout.** NIST SP 800-63B rev 4, NCSC and CISA
  all endorse password managers (NIST: verifiers "SHALL allow the use of password managers and
  autofill"). They don't prescribe how to organize one. Organization is a usability choice, so
  pick the layout that keeps **sharing boundaries**, **autofill matching** and **audits** working.
- **Folders are for sharing boundaries and a little browsing. Search does the rest.** Keep the tree
  shallow (2 levels, 3 at most) and let one top-level split carry access control: Personal /
  Household-shared / Homelab. Bitwarden folders are personal-only, and sharing happens through
  organization **collections**, which don't inherit permissions from a "parent". Keeper folders nest
  without limit, but in the default Classic model a shared folder can't contain another shared
  folder. Both models punish deep trees.
- **Use a typed record whenever a type fits.** Wi-Fi, SSH key, server, database, card and identity
  all get typed records. Use a secure note only for genuinely freeform text. Keeper has WiFi,
  SSH Key, Server and Database types. Bitwarden has only Login, Card, Identity, Secure Note and
  SSH Key, so on Bitwarden the rule becomes "Login item plus named custom fields" for servers,
  routers and API tokens. Kubeconfigs and other files go in an attachment or a note, never pasted
  into a password field.
- **URL hygiene:** use the default domain matching for normal sites and host (with port) matching
  for LAN/homelab services. Avoid "starts with" and regex, which Bitwarden itself calls "quite
  dangerous". Keep old and new domains as extra URIs on one item after a site migrates, and let
  the built-in "unsecured websites" (`http://`) report surface stale URLs.
- **Human and machine secrets go in separate stores.** All three vendors split machine access
  (Bitwarden Secrets Manager projects and machine accounts, Keeper Secrets Manager applications,
  1Password service accounts), and ESO's Bitwarden provider targets Secrets Manager, not the
  personal vault. The human vault holds only **break-glass copies** of the few bootstrap secrets
  (age key, machine-account token, Talos secrets) that the cluster can't hold for itself.
- **TOTP:** storing TOTP in the vault gives you one-time codes but **not a true second factor**,
  according to 1Password's own blog. The widely agreed hard rule is narrower: **never store the
  vault's own 2FA inside the vault.** Bitwarden warns that this "could lock you out". Keep recovery
  codes as one-time secrets in typed or tagged records, plus an offline copy of the vault's own
  recovery code.
- **Hygiene:** follow NIST/NCSC and **don't rotate on a schedule**. Change a password on evidence
  of compromise, and run the built-in breach, reuse and weak-password reports. A quarterly
  10-minute review plus an annual emergency-access check is enough for a household.

## 1. What the standards bodies actually say

| Source | Relevant guidance |
|---|---|
| NIST SP 800-63B rev 4, §3.1.1.2 | "Verifiers **SHALL** allow the use of password managers and autofill functionality." "Verifiers and CSPs **SHALL NOT** require subscribers to change passwords periodically", but "**SHALL** force a change if there is evidence that the authenticator has been compromised." New passwords are checked against a blocklist of breached and dictionary values. |
| NIST SP 800-63B rev 4, §4.1.2.1 / §4.2.1 | Subscribers "**SHOULD** … maintain at least two separate means of authentication." Saved recovery codes are one-time look-up secrets that the service invalidates after use. |
| NIST SP 800-63B rev 4, §2.3.2, Table 3/4 | Synced passkeys ("syncable authenticators") are allowed up to AAL2 but not AAL3, because their keys are exportable. The sync-fabric account itself must be protected at AAL2: "Access to a cloud service that stores authentication keys requires only single-factor authentication" is listed as a threat. **Implication: the password manager that syncs your passkeys needs real MFA on its own account.** |
| NIST SP 800-63B rev 4, §8.1.2.1 | Users "may use a password manager to … maintain distinct passwords for each authenticated service." Agencies "should carefully evaluate password managers." |
| NCSC (UK), *Password managers* | Use one, and use its generator and autofill. Turn on 2SV for the manager account itself. "The NCSC recommends making passkeys your first choice of login and using them wherever they are offered." Recovery options include "emergency access through trusted contacts." |
| NCSC, *Password policy: updating your approach* | "Regular password changing harms rather than improves security." Change on known or suspected compromise. Password managers "are not impregnable and are an attractive target." |
| CISA, *Use strong passwords* | Long (16+), random, unique passwords generated and stored by a password manager. Only the manager's primary password needs memorizing. |

**None of these define folder, tag or naming conventions.** The organization guidance below comes
from vendor data models (which set the hard constraints) and practitioner consensus.

## 2. Folders vs tags vs collections, and how deep to nest

### What the data models allow

| | Bitwarden | Keeper |
|---|---|---|
| Personal grouping | **Folders**: personal only ("Folders organize your individual vault"). They nest by naming `Parent/Child`, which only works if `Parent` exists. Depth is unlimited, but "searching a parent folder doesn't return items in its nested subfolders." | **Folders/subfolders**: nest with no depth limit. |
| Shared grouping | **Collections** in an organization (Families: 6 users, unlimited collections). Nesting is visual only: "Nested collections don't inherit items, access, or permissions from their 'parent' collection." Organization items must sit in at least one collection. | **Shared folders.** Classic model: subfolders inherit the parent shared folder's permissions, but "shared folders within other shared folders are currently not supported." The newer **Nested Shared Folders** (5 levels, per-level permissions) is feature-flagged and invitation-only, and it can't be mixed with Classic. |
| One item in several places | An organization item can sit in several collections. Folders: one per item. | A record can be **linked (shortcut)** into several folders. Editing it anywhere changes it everywhere. |
| Tags | None. Bitwarden has no tags, so folder and collection names (and title prefixes) do that job. | No tags documented in the record-type or folder docs. |
| Personal vs shared ownership | Organization-owned items belong to the organization, not to the user. A personal item has to be moved into the organization to be shared. | A record stays owned by its creator and is shared by placing it in a shared folder. |

### Consensus practice

- **The top level carries the sharing boundary.** Because sharing is per collection (Bitwarden) or
  per shared folder (Keeper), the top-level split should match "who can see this": personal,
  household, homelab/infra. Anything finer is just for browsing.
- **Keep it shallow.** Both vendors' search beats browsing, and both models handle depth badly
  (Bitwarden parent-folder search doesn't recurse, and Keeper Classic can't nest shares), so two
  levels is the practical maximum. A third level is justified only under `Homelab/` (for example
  `Homelab/Network`).
- **One item, one home.** Avoid duplicating an item to "put it in two places". Duplicates drift and
  defeat reuse audits. Use a Keeper shortcut or a second Bitwarden collection if the item genuinely
  needs two audiences.
- **Without tags, put the facet in the title.** Since neither product has real tags, a short
  bracketed prefix (`[recovery]`, `[break-glass]`) in the title is the portable way to make a
  facet searchable.

## 3. Item titles

No standard defines a naming scheme. These conventions follow from how search, autofill pickers
and exports behave:

- **Title = the service as a human would search for it**, not the URL: `Chase`, `Fastmail`,
  `Mikrotik RB5009 (router)`. Autofill already matches on the URL field, so the title only needs
  to disambiguate in the picker.
- **Add a qualifier only when there's more than one account**: `Google - personal`,
  `Google - work`. Put the qualifier after the service so items still sort together.
- **Infra items carry host plus role**: `loki - ssh host key`, `talos - cluster secrets`,
  `k8s - kubeconfig (admin)`. The leading token matches the hostname or system, so search on the
  host finds everything for it.
- **Keep secrets and changing data out of titles.** Titles show in pickers, notifications and
  metadata exports. Don't put usernames, emails or dates there.

## 4. Typed records vs secure notes

**Rule: use the most specific type that has the fields you need. Use a secure note only when the
content is genuinely freeform.** Typed fields feed autofill, health reports, expiry checks and
(for machines) Secrets Manager field lookups. A note is opaque to all of them.

| Secret | Keeper | Bitwarden | 1Password (reference) |
|---|---|---|---|
| Website login | Login | Login (passkey and TOTP can live on the item) | Login |
| Wi-Fi PSK | **WiFi** type | Login with no URI, or a Secure Note with custom fields `ssid` (text) and `psk` (hidden) | **Wireless Router** (shows a join QR code) |
| SSH key | **SSH Key** type | **SSH Key** item type | SSH Key |
| Router / server admin | **Server** type | Login with a host URI and custom fields | Server |
| Database credential | **Database** type | Login plus custom fields (`host`, `port`, `db`) | Database |
| API token | Login or Secure Note plus a Hidden field | Login (token in password) or Secure Note plus a **Hidden** custom field | **API Credential** (has expiry) |
| Kubeconfig / talosconfig / age key | File attachment on a record, or Secure Note | Secure Note plus attachment (custom fields cap at 5,000 *encrypted* characters, about 3,400 plain; an RSA-4096 key "grows to about 4400") | Document |
| Recovery codes | Secure Note (or a field on the related Login) | Secure Note, or a hidden custom field on the related Login | Field on the Login |
| Card / ID / licence | Payment Card, Passport, Driver's License, Software License… | Card / Identity / Secure Note | Native types |

Where the models differ:

- **Keeper has rich built-in types and custom fields** (Text, Hidden, URL, Multi-line, Date, PIN…).
  Only Keeper Business admins can create *custom record types*, so a Family-plan user is limited to
  the built-in types plus custom fields.
- **Bitwarden has only five item types** (Login, Card, Identity, Secure Note, SSH Key) and four
  custom field types (Text, Hidden, Checkbox, Linked). Structure comes from **consistently named
  custom fields**, so pick one label vocabulary (`host`, `port`, `ssid`, `psk`, `token`,
  `expires`) and stick to it. Custom-field names are also used for autofill matching
  (case-insensitive, exact).
- **Moving between vendors flattens types.** A Keeper WiFi record becomes a Bitwarden note or
  login. Consistent field labels are what make that translation scriptable (as metadata only).

## 5. URL / URI hygiene

**Bitwarden match modes** (per URI): Base domain (default), Host (hostname plus port), Starts with,
Regular expression, Exact, Never.

- "Starts with" and "Regular expression" are each "an advanced option and can be quite dangerous if
  used incorrectly." An unescaped `.` in a regex matches any character.
- Base domain is unavailable for single-label hostnames and local TLDs. **Use Host for LAN
  services** (`192.168.x.y:8443`, `omada.home.arpa`).
- Equivalent domains link related domains globally, but Exact match on an item turns them off.
- Platform limits: iOS keyboard suggestions always use base domain, and Android can't match on
  port or path. Don't rely on port-distinguished items on mobile.

**Keeper** matches on the record's Website Address and on URL custom fields. It prefers the closest
subdomain, then the path, and "will not offer to fill passwords unless there is a root domain
match". An optional **Match on Subdomain** setting enforces strict subdomain matching. Extra domains
go in additional URL custom fields, which "KeeperFill will recognize … as an alias".

**Practice:**

1. Store the **login page's origin**, not a deep link with session or query parameters.
2. After a rename or migration, **add the new URI and keep the old one** on the same item until the
   old domain is dead. Don't clone the item.
3. Use host-level matching for anything with several services on one domain or IP (homelab
   ingress). Use default domain matching for everything else, and don't use regex.
4. **Finding stale URLs:** run the "unsecured websites" report (Bitwarden *Unsecured Websites*,
   1Password Watchtower *Unsecured websites*) to catch `http://` leftovers. For dead domains, a
   metadata-only export of URIs can be resolved with DNS lookups (titles and URIs only, never
   secret fields).

## 6. Separating human and machine secrets

- **All three vendors ship a separate machine-secret product with its own access model:**
  - Bitwarden **Secrets Manager**: projects, plus machine accounts with access tokens. Free tier
    exists, but a Families user must create a separate Free organization for it.
  - Keeper **Secrets Manager**: an "Application is a permission boundary that defines which records
    and folders a workload can access", and each client device has its own key. Keeper recommends
    least privilege.
  - 1Password **service accounts**: vault-scoped read/write, fixed when the account is created.
- **ESO's Bitwarden provider targets Secrets Manager**, not Password Manager ("This is different from
  Bitwarden Password Manager"). It needs a machine-account token, project access, and the
  `bitwarden-sdk-server` running over HTTPS.
- **Consensus:** machine secrets live in the machine store, scoped per workload or project. Humans
  don't browse them day to day. The **human vault keeps only a `Homelab/Break-glass` set**: the
  credentials needed to rebuild from zero. That means the sops age key (while sops remains), the
  Secrets Manager machine-account token, Talos secrets/talosconfig, and router admin plus its
  backup. This matches the map's rule that the cluster must not hold the keys to rebuild itself.
- Personal and homelab items share one human vault but sit under different top-level folders or
  collections, so household sharing never exposes infra by accident.

## 7. Recovery codes, TOTP and passkeys

**TOTP in the vault: the argument and the counterargument**

- *For separation:* Jeffrey Goldberg (1Password, 2015): "the device that holds your TOTP secret
  should never hold your password if your aim is genuine two factor security." Codes stored
  together still give you "the benefits of the one-timeness of TOTP codes" (they resist replay and
  interception).
- *For keeping them together:* Megan Barker (1Password, 2023): if both secrets come from the same
  device anyway (a phone authenticator app on the phone you log in from), the protection is the
  same. Separation helps only when an attacker has the vault login but not your device. "For the
  majority of people, storing TOTP in 1Password is well within their risk tolerance."
- *Practitioner view:* Troy Hunt accepts the "eggs in one basket" risk for password managers
  because vault compromise is "exceptionally rare" compared with breaches of individual services.
- **Firm rule both camps agree on:** the vault's *own* 2FA can't live in the vault. Bitwarden
  Authenticator's docs: "Syncing the code to the same vault it's meant to protect could lock you
  out of it." Bitwarden support won't disable 2FA for you, so "save their recovery code immediately".

**Middle path for a homelab user:** keep everyday TOTP in the vault for convenience. Keep the
vault account's 2FA, the primary email's 2FA, and anything that can reset either of them in a
separate authenticator (or on a hardware key). Bitwarden Authenticator is free, works without an
account and syncs optionally. Its docs say local keys are kept "in a local unencrypted database on
your device", so treat any export file as sensitive.

**Recovery codes:** NIST treats them as one-time secrets and recommends at least two separate
means of authentication. Store them in the vault next to (or inside) the login they recover, and
remove used codes. For the vault's own recovery code, keep an **offline copy** (paper or a
separate encrypted store) because the vault can't recover itself.

**Passkeys:** NCSC recommends them as the first choice. Bitwarden stores one passkey per Login item
("Only one passkey can be saved per login item"). Keeper shows a Passkey field on the record, which
can be placed in folders or shared. NIST permits synced passkeys up to AAL2 and requires AAL2
protection on the sync account, which is another reason to put strong, separately held MFA on the
vault account itself.

## 8. Shared household items and emergency access

- Shared items go in **one household collection or shared folder**. Each member's personal items
  stay personal. On Bitwarden, a shared item becomes organization-owned. On Keeper it remains
  the creator's record, shared through the folder.
- **Emergency access:**
  - Bitwarden: View or **Takeover** access, granted automatically after a grantor-set wait time.
    It needs Premium or a paid organization (Families counts). Takeover resets the master password
    and removes 2FA.
  - Keeper: up to 5 trusted contacts (consumer accounts only), with a delay of up to 3 months that
    starts when the contact tries to log in.
- **Practice:** name at least one emergency contact, choose a wait long enough to notice and deny an
  abuse (days, not hours), and **test it once a year**.

## 9. Ongoing hygiene

| Check | Bitwarden | Keeper | 1Password |
|---|---|---|---|
| Breached / exposed passwords | Exposed Passwords (Premium); Data Breach via HIBP (free) | BreachWatch (plan availability not verified from Keeper docs) | Watchtower: vulnerable passwords, compromised sites |
| Reused / weak | Reused, Weak (Premium) | Security Audit score (strength plus reuse) | Watchtower |
| `http://` URIs | Unsecured Websites (Premium) | n/a | Unsecured websites |
| 2FA available but unused | Inactive 2FA (Premium) | n/a | Two-factor available |
| Expiring items / passkey available | n/a | n/a | Expiring, Passkeys available |

Per NIST and NCSC, **don't rotate on a calendar**. Rotate on breach evidence, a person leaving the
household or a device being lost, or because a report flags the item.

## 10. Recommended conventions (adopt as-is)

### Folder tree

```
Personal/            # private; one per person (Bitwarden: personal folders; Keeper: private folder)
  Finance
  Accounts           # everything web-login that isn't finance
  Identity           # IDs, licences, cards
Household/           # shared (Bitwarden: collection; Keeper: shared folder)
  Utilities, Streaming, Wi-Fi, Devices
Homelab/             # shared only with the admin(s); a separate collection / shared folder
  Break-glass        # rebuild-from-zero set: age key, BWS/KSM machine token, Talos secrets, router admin
  Network            # router, switches, APs, Omada controller
  Hosts              # loki, idun, NAS: SSH keys, IPMI/BMC
  Services           # app admin logins (HA, Grafana, …)
```

- Two levels by default, three only under `Homelab/`. Top-level folders map one-to-one to a
  sharing boundary.
- Machine-consumed secrets live in Secrets Manager or KSM, not here. Only the break-glass copies
  are in the human vault.

### Title pattern

- `<Service>` or `<Service> - <qualifier>` for web accounts (`Google - personal`).
- `<host-or-system> - <what> (<role>)` for infra (`loki - ssh (user)`,
  `k8s - kubeconfig (admin)`).
- `[recovery]`, `[break-glass]` or `[shared-acct]` prefixes as searchable pseudo-tags.
- No usernames, emails, dates or secrets in titles.

### Typed record vs note

1. If a native type fits (Login, WiFi, SSH Key, Server, Database, Card, ID), use it.
2. Otherwise use a Login plus named custom fields from a fixed label set: `host`, `port`, `user`,
   `ssid`, `psk`, `token`, `expires`, `notes`. Secret values always go in **Hidden** fields.
3. Files (kubeconfig, talosconfig, age keys, cert bundles) go in an **attachment** on a record
   named for the system. On Bitwarden, anything over about 3,400 plaintext characters can't fit a
   custom field.
4. Use a Secure Note only for genuinely freeform text (procedures, multi-line recovery codes).

### URL hygiene

- Store the origin of the login page. Strip query strings and session paths.
- Use default (base-domain) matching for public sites, and **Host** matching for LAN IPs, ports and
  `*.home.arpa`. Don't use "starts with" or regex.
- After a domain migration, add the new URI and keep the old one until it stops resolving.
- Every item in `Homelab/Services` has a URI. Items without a URL are Wi-Fi, keys and notes only.

### Review cadence

| When | What |
|---|---|
| On event | Rotate on a breach alert, a lost device or a household member leaving. Add the 2FA or passkey when a site starts offering it. |
| Quarterly (about 10 min) | Run the breach, reuse, weak, `http://` and inactive-2FA reports. Empty the "unfiled" area. Remove used recovery codes. |
| Yearly | Test emergency access end to end. Verify the offline copy of the vault's recovery code and the separate authenticator. Do a break-glass dry run (can you find every rebuild secret by title alone?). Prune dead accounts and dead URIs. |
| Never | Calendar-based password rotation (NIST §3.1.1.2, NCSC). |

## Sources

Standards and government guidance
- NIST SP 800-63B rev 4, *Authentication and Authenticator Management*: https://pages.nist.gov/800-63-4/sp800-63b.html (§2.2.1, §2.3.2, §3.1.1.2, §3.1.7, §4.1.2.1, §4.2.1, §8.1.2.1)
- NCSC, *Password managers*: https://www.ncsc.gov.uk/collection/top-tips-for-staying-secure-online/password-managers
- NCSC, *Password policy: updating your approach*: https://www.ncsc.gov.uk/collection/passwords/updating-your-approach
- CISA, *Use strong passwords*: https://www.cisa.gov/secure-our-world/use-strong-passwords

Bitwarden
- URI match detection: https://bitwarden.com/help/uri-match-detection/
- Folders: https://bitwarden.com/help/folders/
- Collections: https://bitwarden.com/help/about-collections/
- Organizations (Families: 6 users, unlimited collections): https://bitwarden.com/help/about-organizations/
- Item types: https://bitwarden.com/help/managing-items/
- Custom fields: https://bitwarden.com/help/custom-fields/
- Storing passkeys: https://bitwarden.com/help/storing-passkeys/
- Integrated authenticator: https://bitwarden.com/help/authenticator-keys/
- Bitwarden Authenticator: https://bitwarden.com/help/bitwarden-authenticator/
- Vault health reports: https://bitwarden.com/help/reports/
- Emergency access: https://bitwarden.com/help/emergency-access/
- Secrets Manager overview: https://bitwarden.com/help/secrets-manager-overview/
- Security white paper: https://bitwarden.com/help/bitwarden-security-white-paper/

Keeper
- Record types: https://docs.keeper.io/en/user-guides/record-types
- Shared folders (Classic model, linking/shortcuts): https://docs.keeper.io/en/enterprise-guide/sharing/folders
- Nested shared subfolders: https://docs.keeper.io/en/user-guides/sharing/nested-shared-subfolders
- Vault Release 18.0.0: https://docs.keeper.io/release-notes/desktop/web-vault-+-desktop-app/vault-release-18.0.0
- Browser extension matching / Match on Subdomain: https://docs.keeper.io/user-guides/browser-extensions
- One record with multiple URL domains: https://docs.keeper.io/user-guides/tips-and-tricks/one-record-with-multiple-url-domains
- Passkeys: https://docs.keeper.io/en/user-guides/passkeys
- Security Audit: https://docs.keeper.io/user-guides/security-audit
- Emergency access: https://docs.keeper.io/en/user-guides/emergency-access
- Secrets Manager terminology: https://docs.keeper.io/en/keeperpam/secrets-manager/about/terminology

1Password
- Item categories: https://support.1password.com/item-categories/
- Watchtower: https://support.1password.com/watchtower/
- Service account security: https://www.1password.dev/service-accounts/security/
- J. Goldberg, *TOTP for 1Password users* (2015): https://1password.com/blog/totp-for-1password-users
- M. Barker, *Is it wrong to store passwords and one-time codes together?* (2023): https://1password.com/blog/1password-2fa-passwords-codes-together

Other
- External Secrets Operator, Bitwarden Secrets Manager provider: https://external-secrets.io/latest/provider/bitwarden-secrets-manager/
- Troy Hunt, *Passwords Evolved*: https://www.troyhunt.com/passwords-evolved-authentication-guidance-for-the-modern-era/

Gaps: Keeper BreachWatch availability on a Family plan bundled with a Business seat was not
confirmed from Keeper docs. Whether one Bitwarden organization item can sit in several collections
is implied by the docs but not stated outright. Neither product has first-class tags (Bitwarden
confirmed by absence from its item docs; Keeper by absence from its record docs).
