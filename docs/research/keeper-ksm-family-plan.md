# Keeper Secrets Manager on a Family Plan: Primary Source Findings

Research for issue [#1635](https://github.com/rwade628/homelab/issues/1635) ("Research: Keeper
Secrets Manager on a Family plan"), a child of map issue
[#1634](https://github.com/rwade628/homelab/issues/1634). Researched 2026-10-17. Sources were
limited to docs.keeper.io, keepersecurity.com, external-secrets.io, and the ESO GitHub repo. No
`keeper`/`ksm`/`sops` CLI was run.

## TL;DR

**No. A Family-plan member can't use Keeper Secrets Manager (KSM), even on a Family plan that a
Keeper Business seat sponsors.** KSM needs two things that only exist inside a Business or
Enterprise tenant:

1. The KSM add-on, bought on a business plan.
2. A role in that tenant with the Secrets Manager enforcement policy turned on.

The sponsored Family plan is a separate personal account. The business can't see into it or
manage it, so neither requirement can ever apply to it. No Keeper page says "Family users can't
use KSM" in those words, but the conclusion follows directly from the documented requirements.
The question for Keeper support (below) is only needed if you want that in writing.

**The cheapest Keeper-native route is a Keeper Business subscription plus the KSM add-on.**
- Business has a 5-seat minimum. Keeper's own blog lists it at $4/user/month, so about **$240 a
  year before KSM**.
- The KSM add-on price isn't published. KSM can't be bought through In-Console Checkout, so it
  needs a sales quote.
- Business Starter is cheaper ($2/user/month, 5–10 seats), but the current comparison table
  doesn't offer the Secrets Manager add-on on Starter.
- KeeperPAM includes KSM, but it's priced by custom quote.

**The less awkward alternative** is for the sponsoring Business account to add you as a seat and
buy KSM. That only makes sense if a family member controls that tenant: your homelab secrets would
then sit under that tenant's admins and policies. If the sponsor is an employer, rule it out.

**For a one-person homelab, KSM is poor value.** It is about $240+/year plus a quoted add-on, and
the ESO provider is rated **alpha**. That strengthens the case for keeping the Keeper Family plan
for people and putting machine secrets in another product (see the Bitwarden ticket).

## 1. Can a Family-plan member use KSM?

**What KSM requires.** Keeper's KSM troubleshooting page states:

> "In order to utilize Keeper Secrets Manager, two permission criteria must be met:
> 1. The Secrets Manager add on must be enabled for your Keeper Account, and you must have an
>    active (non-expired) subscription.
> 2. You must be in a role with the Secrets Manager enforcement policy enabled"
> ([KSM Troubleshooting](https://docs.keeper.io/keeperpam/secrets-manager/troubleshooting))

The KSM Quick Start lists a **"Keeper Business License"** as the first prerequisite: "Secrets
Manager is available for Business accounts." It activates the trial from the **Admin Console**
([Quick Start](https://docs.keeper.io/en/keeperpam/secrets-manager/quick-start-guide)). The
Enterprise Guide says that after activation, "you can turn on Secrets Manager within the role
enforcement policies for particular users and roles at the company." Only users in such a role get
the vault's "Secrets Manager" tab, where applications and devices are created
([Enterprise Guide: Secrets Manager](https://docs.keeper.io/enterprise-guide/secrets-manager)).

**How it's sold.** The KSM product page says it is "offered as an add-on to all business password
manager plans" and "licensed on a per-user, per-year basis". It "is included with KeeperPAM"
([KSM product page](https://www.keepersecurity.com/secrets-manager.html)). Neither the
personal/family pricing page nor the family plan page lists Secrets Manager
([Personal & Family pricing](https://www.keepersecurity.com/pricing/personal-and-family.html),
[Family plan](https://www.keepersecurity.com/family-plan.html)).

**What the sponsored Family plan is.** Per the Enterprise Guide page "Free Family License for
Personal Use":
- It is a separate account, created with a personal email and its own master password.
- It is "licensed as a Keeper Family Plan account with 10GB of secure file storage and BreachWatch
  dark web monitoring".
- "The company managing your Business vault does not have any access rights or ability to decrypt
  information stored in your personal vault."
- "All business-related credentials must be stored within your company issued vault."
- If the sponsor leaves the company or the business stops paying, "you may continue to use your
  account for 30 days, after which it will expire and be converted to a free trial account."

([Enterprise Guide](https://docs.keeper.io/enterprise-guide/personal-vaults-for-enterprise-and-business-users),
[User Guide](https://docs.keeper.io/user-guides/tips-and-tricks/personal-and-business-vaults))

**Conclusion.** Your account is a consumer Family-plan account. It is not a user in any Business
tenant, so no tenant role or enforcement policy can reach it, and the KSM add-on is only sold on
business plans. Both of KSM's documented requirements are therefore impossible for you to meet as
things stand. The business sponsorship gives you a free Family license and nothing more: it doesn't
put your account inside the sponsor's tenant.

### Question for Keeper support, if you want it in writing

> "My only Keeper account is a Family plan (the free Family license linked from a relative's Keeper
> Business seat). Can this account be licensed for Keeper Secrets Manager without becoming a user
> in a Business or Enterprise tenant? If not, what are the minimum seat count and the annual price
> of the Secrets Manager add-on for a single-user Keeper Business subscription, and is the add-on
> available on Business Starter?"

## 2. Could the sponsoring Business account give you KSM?

Only by making you a **user in their tenant**. Being a Family-plan member isn't enough. In
practice:

- **Seat:** they add you as a user (a paid seat) in their Admin Console. You get a second, business
  vault on a separate account; your Family vault stays separate.
- **Add-on:** they buy the KSM add-on through a Keeper sales rep. The Subscriptions page lists "KSM
  (Keeper Secrets Manager)" under "Not available via In-Console Checkout" and says to contact a
  sales representative
  ([Subscriptions](https://docs.keeper.io/enterprise-guide/subscriptions)).
- **Role:** their admin puts you in a role with the Secrets Manager enforcement policy enabled
  ([Enterprise Guide: Secrets Manager](https://docs.keeper.io/enterprise-guide/secrets-manager)).
- **ESO `PushSecret`:** this needs a custom record type named `externalSecrets`. Creating custom
  record types requires "an Administrative role with the 'Manage Record Types in Vault'
  permission", and "Keeper Business and Enterprise customers can define totally custom Record
  Types"
  ([Creating New Record Types](https://docs.keeper.io/enterprise-guide/creating-new-record-types)).
  So their admin would also have to create that type or delegate the permission to you.
- **Who controls what:** the homelab secrets would live in a vault governed by their tenant's
  policies and admins, and your access ends when that subscription or your seat does. Keeper is
  zero-knowledge, so admins can't decrypt your vault directly. They do control your role
  permissions and your account's lifecycle, including the vault-transfer feature listed on the
  pricing page ("Securely transfer an employee's vault when they leave the organization").
- **Verdict:** this is workable only if a family member owns that Keeper Business tenant. If it is
  the relative's employer, it is inappropriate and against the spirit of the personal-use terms
  quoted above.

## 3. Cheapest way in on your own

The live pricing page fills in prices with JavaScript, and the cells were blank when fetched. The
plan *structure* below comes from that page's HTML comparison table. The dollar figures come from
Keeper's own blog ([Choosing the Right Keeper Plan](https://www.keepersecurity.com/blog/2023/04/06/choosing-the-right-keeper-plan-for-your-business/),
post dated 2023, shown as recently updated). Confirm them at checkout.

| Plan | List price (blog) | Seats | Secrets Manager |
|---|---|---|---|
| Business Starter | from $2/user/mo, billed annually | 5–10 (auto-upgrades to Business above 10) | **Not offered**: the "Secrets Manager CLI and SDKs" cell is empty for Starter |
| Business | from $4/user/mo, billed annually | minimum 5 | **Add-on** |
| Enterprise | from $6/user/mo, billed annually | quote | **Add-on** |
| KeeperPAM | custom | quote | **Included** ("It is included with KeeperPAM") |

Sources: [Business pricing page](https://www.keepersecurity.com/pricing/business-and-enterprise.html)
(comparison table: Secrets Manager row reads empty / "Add-on" / "Add-on" for Starter / Business /
Enterprise; Starter card says "Protection for 5–10 users"; Business card lists "Free Family Plan for
every user"), [Subscriptions](https://docs.keeper.io/enterprise-guide/subscriptions) (Starter
auto-upgrade above 10 users; KSM not available through In-Console Checkout),
[KeeperPAM Licensing](https://docs.keeper.io/keeperpam/privileged-access-manager/getting-started/keeperpam-licensing).

- **Minimum cost:** Keeper Business at 5 × $4 × 12 = **about $240/year**, plus the KSM add-on.
- **KSM add-on price and API quotas aren't published.** KeeperPAM Licensing says upgrading from
  the KSM add-on to PAM gives "Unlimited API Calls", which implies the plain KSM add-on has a call
  cap. Keeper doesn't publish the cap. ESO polls on each `refreshInterval`, so ask about the cap
  when getting a quote.
- **Trial:** the Admin Console offers a KSM "Free Trial", which is useful for a prototype but isn't
  a long-term plan.
- **Side effect:** your own Business seat would come with its own free Family plan for 5 people.
  You don't need it, since the family is already covered by the sponsor.

## 4. ESO `keepersecurity` provider

Sources: [external-secrets.io provider page](https://external-secrets.io/latest/provider/keeper-security/),
[`docs/provider/keeper-security.md`](https://github.com/external-secrets/external-secrets/blob/main/docs/provider/keeper-security.md),
[`docs/introduction/stability-support.md`](https://github.com/external-secrets/external-secrets/blob/main/docs/introduction/stability-support.md),
[`providers/v1/keepersecurity`](https://github.com/external-secrets/external-secrets/tree/main/providers/v1/keepersecurity).
The ESO repo was checked on 2026-10-17; the latest release then was `helm-chart-2.13.0`.

- **Stability:** **alpha**, maintained by a single community maintainer (`@ppodevlab`). In the
  feature matrix, Keeper has *find by name*, *store validation* and *push secret*. It lacks *find by
  tags*, *metadataPolicy Fetch*, *referent authentication*, and *DeletionPolicy Merge/Delete*
  support.
- **Authentication:**
  - It needs a **Secrets Manager Configuration (SMC)**: a base64-encoded JSON with `hostname`,
    `clientId`, `privateKey`, `serverPublicKeyId`, `appKey` and `appOwnerPublicKey`. A one-time
    access token isn't accepted directly.
  - The SMC goes in a Kubernetes Secret referenced by `authRef`. A `ClusterSecretStore` must set
    the namespace on that reference.
  - The store also takes a `folderID`, the folder `PushSecret` writes to, which needs write
    permission.
  - The SMC is created from a KSM application device, so it inherits all of section 1's licensing
    requirements.
- **Record types:**
  - **General and legacy non-typed records aren't supported.** Records must be typed (Login,
    Server Credentials, and so on), which matters for migrated LastPass-style flat notes.
  - Complex field types (name, phone, bankAccount) come back as a JSON string; decode them with
    template functions.
- **Field addressing:**
  - `remoteRef.key` is the record UID.
  - `remoteRef.property` is the field label (or the field type if it has no label), the
    custom-field label, or a file name. Empty means the whole record as JSON.
  - `version` isn't supported.
- **File attachments:** **supported.** Reference them by file name in `property`.
- **`dataFrom.find`:** `name.regexp` matches field labels/types, custom-field labels and file
  names. `find.path` and `find.tags` aren't supported.
- **`PushSecret`:** supported, but only into a **custom record type named `externalSecrets`**,
  which takes a Business/Enterprise admin to create (section 2).
  - It pushes one key per secret at a time.
  - It can't add a new key to an existing record (upstream
    [secrets-manager-go#17](https://github.com/Keeper-Security/secrets-manager-go/issues/17)).
  - `remoteRef.remoteKey` uses the format `SecretName/SecretKey`.
- **Deletion:** KSM returns the same empty result for a deleted record and for one no longer
  shared with the application. With `deletionPolicy: Delete`, unsharing a record therefore deletes
  the key. The default `Retain` avoids this.

## 5. TOTP and passkeys in Keeper (personal plans)

- **TOTP:**
  - Keeper stores a standard TOTP seed as a record's "Two-Factor Code" field and generates the
    codes.
  - Desktop setup is by scan, QR-image upload or manual entry. On iOS it's setup-code import,
    secret key, QR scan or QR upload.
  - Codes autofill through the browser extension and web vault.
  - Because records can be shared, "two-factor codes are always available to everyone who has
    access to the record".
  - CSV import supports TOTP codes.
  - Sources: [Protecting TOTP Codes](https://docs.keeper.io/user-guides/tips-and-tricks/protecting-totp-codes),
    [Storing MFA Codes](https://docs.keeper.io/enterprise-guide/storing-two-factor-codes).
- **Passkeys:**
  - The Family plan lists "Unlimited passwords + passkeys"
    ([Family plan](https://www.keepersecurity.com/family-plan.html)).
  - The iOS and Android apps intercept "Create Passkey" and save the passkey to the vault.
  - On iOS 26+ you can import passwords, passkeys and two-factor codes from Apple Passwords
    ([iOS guide](https://docs.keeper.io/user-guides/ios),
    [Android guide](https://docs.keeper.io/user-guides/android)).
- **Free vs paid:** the business-sponsored Family license is a full Family plan, so both features
  apply. Neither the pricing pages nor the docs fetched set out the Keeper Free tier's feature list.
  That matters only if the sponsorship lapses, when the account converts to the free tier after 30
  days.

## 6. Security track record and audits (Keeper's own claims)

From [keepersecurity.com/security.html](https://www.keepersecurity.com/security.html). These are
vendor statements; independent incident history is outside this ticket's source list.

- SOC 2 Type 2 (the page says for over ten years), and ISO 27001, 27017 and 27018.
- FedRAMP High and GovRAMP High authorized; FIPS 140-3 (certificate #4743). The page also lists
  PCI DSS Level 1, HIPAA, TX-RAMP and CMMC Level 1.
- Quarterly penetration testing, naming NCC Group and CyberTest.
- Public bug bounty and vulnerability disclosure program on Bugcrowd, with a safe-harbor statement.
- Zero-knowledge, client-side encryption: data isn't decrypted on Keeper's servers, "not even
  Keeper employees" can access it.

## 7. Family sharing

**Within the Family plan.** The sponsored Family plan gives each member "their own vault", and
members "can also share passwords among family members using either Shared Folders or individual
record sharing"
([Free Family License](https://docs.keeper.io/enterprise-guide/personal-vaults-for-enterprise-and-business-users)).
Vaults are private by default. Sharing works the same way as for any Keeper users
([User Guide: Sharing](https://docs.keeper.io/user-guides/sharing)):

- **Record share:** the permission is View Only, Can Edit, Can Share, Can Edit & Share, or Transfer
  Ownership.
- **Shared folder:** has two independent permission layers.
  - *User permissions:* Can Manage Users, Can Manage Records, Can Manage Users & Records, or No
    User Permissions.
  - *Record permissions:* View Only, Can Edit, Can Share, or Can Edit & Share.
  - "Can Manage Records" lets a member add or remove records but does **not** grant edit rights on
    them. Set the folder's default record permissions before adding members, because a change to
    the defaults only applies to members added afterwards
    ([Record and File Sharing](https://docs.keeper.io/enterprise-guide/sharing/record-and-file-sharing)).
- **Sharing relationship:** the first share with someone sets up a "sharing relationship" that the
  recipient accepts after logging in. The docs say only that "Business and Enterprise users within
  the same tenant" have this already, and say nothing either way about Family-plan members. Expect a
  one-time accept per person.

**With someone outside Keeper.**
- **Ongoing:** a normal share requires a Keeper account. The recipient gets an email "prompting
  them to login to Keeper" to accept.
- **One-time:** **One-Time Share** sends a record "to anyone without requiring them to create a
  Keeper account".
  - The link expires after 1 h, 24 h, 1 week, 30 days, 90 days, or a custom time of up to
    **180 days**.
  - It binds to the first device that opens it.
  - The sender can optionally let the recipient edit, and can revoke the link at any time.
  - It only works on typed records created after 2021-04-26.
  - Self-Destructing Records are a stricter variant: the record is deleted after 5 minutes of
    viewing or at expiry.
  - Sources: [One-Time Share](https://docs.keeper.io/user-guides/sharing/one-time-share),
    [Self-Destructing Records](https://docs.keeper.io/enterprise-guide/sharing/self-destructing-records).
- So there is no permanent link for non-users. Anyone who needs ongoing access needs at least a
  Keeper account, and the Family plan's five vaults are the free way to give them one.

**Running a separate machine-secrets product alongside a Keeper personal vault.** Nothing in the
sources suggests any effect on Keeper sharing:
- Family sharing is a feature of the Family plan's vaults and doesn't depend on KSM.
- KSM only adds application access to shared folders, inside a Business tenant.
- A separate product (for example, Bitwarden Secrets Manager, which ESO supports through its
  `bitwarden` provider) wouldn't touch the Keeper vaults at all.

The real costs are operational, not about sharing:
- There are two products, two recovery stories and two sets of 2FA.
- Machine secrets stored in the other product can't be shared to family members through Keeper.
  This rarely matters for cluster credentials.
- Anything both humans and the cluster need (a Wi-Fi PSK, say) would have to exist in both, with
  no sync between them.

## Sources

- docs.keeper.io
  - [KSM Troubleshooting (access_denied requirements)](https://docs.keeper.io/keeperpam/secrets-manager/troubleshooting)
  - [KSM Quick Start Guide](https://docs.keeper.io/en/keeperpam/secrets-manager/quick-start-guide)
  - [KSM Overview](https://docs.keeper.io/en/keeperpam/secrets-manager/overview)
  - [Enterprise Guide: Secrets Manager](https://docs.keeper.io/enterprise-guide/secrets-manager)
  - [Enterprise Guide: Free Family License for Personal Use](https://docs.keeper.io/enterprise-guide/personal-vaults-for-enterprise-and-business-users)
  - [User Guide: Free Family License for Personal Use](https://docs.keeper.io/user-guides/tips-and-tricks/personal-and-business-vaults)
  - [Enterprise Guide: Subscriptions](https://docs.keeper.io/enterprise-guide/subscriptions)
  - [KeeperPAM Licensing](https://docs.keeper.io/keeperpam/privileged-access-manager/getting-started/keeperpam-licensing)
  - [MSP Secure Add-Ons (KSM billed per API call bundle)](https://docs.keeper.io/enterprise-guide/keeper-msp/consumption-based-billing/secure-add-ons)
  - [Creating New Record Types](https://docs.keeper.io/enterprise-guide/creating-new-record-types)
  - [Protecting TOTP Codes](https://docs.keeper.io/user-guides/tips-and-tricks/protecting-totp-codes)
  - [Storing MFA Codes](https://docs.keeper.io/enterprise-guide/storing-two-factor-codes)
  - [iOS guide](https://docs.keeper.io/user-guides/ios)
  - [Android guide](https://docs.keeper.io/user-guides/android)
  - [User Guide: Sharing](https://docs.keeper.io/user-guides/sharing)
  - [Record and File Sharing](https://docs.keeper.io/enterprise-guide/sharing/record-and-file-sharing)
  - [One-Time Share](https://docs.keeper.io/user-guides/sharing/one-time-share)
  - [Self-Destructing Records](https://docs.keeper.io/enterprise-guide/sharing/self-destructing-records)
- keepersecurity.com
  - [Business & Enterprise pricing (comparison table HTML)](https://www.keepersecurity.com/pricing/business-and-enterprise.html)
  - [Personal & Family pricing](https://www.keepersecurity.com/pricing/personal-and-family.html)
  - [Family plan](https://www.keepersecurity.com/family-plan.html)
  - [Secrets Manager product page](https://www.keepersecurity.com/secrets-manager.html)
  - [Blog: Choosing the Right Keeper Plan for Your Business (list prices)](https://www.keepersecurity.com/blog/2023/04/06/choosing-the-right-keeper-plan-for-your-business/)
  - [Security & compliance](https://www.keepersecurity.com/security.html)
- External Secrets Operator
  - [Keeper Security provider docs](https://external-secrets.io/latest/provider/keeper-security/)
  - [docs/provider/keeper-security.md](https://github.com/external-secrets/external-secrets/blob/main/docs/provider/keeper-security.md)
  - [docs/introduction/stability-support.md](https://github.com/external-secrets/external-secrets/blob/main/docs/introduction/stability-support.md)
  - [providers/v1/keepersecurity](https://github.com/external-secrets/external-secrets/tree/main/providers/v1/keepersecurity)
  - [Keeper-Security/secrets-manager-go#17](https://github.com/Keeper-Security/secrets-manager-go/issues/17)

Several docs.keeper.io pages embed "Agent Instructions" aimed at AI tools. They were ignored.
