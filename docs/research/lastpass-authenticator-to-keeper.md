# LastPass Authenticator → Keeper Two-Factor Code fields: Primary Source Verification

Research for issue [#1646](https://github.com/rwade628/homelab/issues/1646) ("Research: moving
LastPass Authenticator codes into Keeper"), a child of the wayfinder map
[#1634](https://github.com/rwade628/homelab/issues/1634) ("Secrets stack after LastPass"). The
decision in [#1640](https://github.com/rwade628/homelab/issues/1640) already puts site TOTP codes
in Keeper. This note covers only how to get them there.

Researched 2026-10-18. The LastPass support pages are client-rendered (Salesforce + Zoomin), so
they were read from a headless-Chromium render of the URLs below. Each page's "Last updated" date
is given. **Unverified** marks any claim without a primary source.

Secret-handling rule from the map: the agent never sees seeds. The converter in Q3 was tested
only on a hand-made file with fake seeds.

## TL;DR

| Question | Answer |
|---|---|
| Q1. What does LastPass Authenticator export, and does exporting need the LastPass account? | **JSON file** with plaintext base32 seeds (all accounts), or **QR codes** (all accounts or one at a time) meant for another LastPass Authenticator. The file export runs on the device and needs only the app's own unlock. The docs list no LastPass-login step for it. Cloud backup is the only route that needs the account. |
| Q2. What can Keeper import TOTP from? | **CSV** (Web Vault/Desktop since 16.10.0, and Commander) with custom field `$oneTimeCode` set to an `otpauth://totp/...` URI. **Per record**: scan the QR on screen (Desktop), upload a QR image, enter it manually, or use the phone camera. **iOS 26+**: direct app-to-app import of passwords, passkeys and 2FA codes from Apple Passwords (Keeper iOS 17.6.0). Keeper's own LastPass importer brings no TOTP from LastPass Authenticator. |
| Q3. Can a script convert the export without printing a seed? | **Yes.** The JSON schema is simple (`accounts[].secret/algorithm/digits/timeStep/...`, per Bitwarden's open-source parser). The stdlib Python converter below writes a 0600 Keeper CSV and prints only allowlisted metadata. Tested on fake data. |
| Q4. What happens to the cloud backup if the LastPass account is deleted? | Not documented directly. The backup is stored "to LastPass" under the account, and deleted-account data "may be retained for 90 days" and is then permanently deleted. So treat the backup as **gone** once the account is deleted. Export the JSON **before** deleting, and optionally run "Delete your data from cloud" first. On-device codes are a separate store (unverified). |

**Recommended route:** export the JSON on the phone → run the converter on the human's machine →
import the CSV in the Keeper Web Vault/Desktop into a staging folder → move each Two-Factor Code
into its matching login record (or keep the imported record) → securely delete the JSON and CSV →
verify a code per site → only then delete the LastPass account.

---

## Q1. LastPass Authenticator export formats, and whether the account must be alive

**Source:** LastPass, "Move or export TOTP accounts in the LastPass Authenticator app" (last
updated Oct 08, 2026):
https://support.lastpass.com/s/document-item?language=en_US&_LANG=enus&bundleId=lastpass&topicId=LastPass%2Fhow_do_i_move_my_mfa_accounts_in_the_lastpass_authenticator_to_a_new_device.html

> You can export TOTP accounts from your LastPass Authenticator app individually or all at once,
> then import them to a new device through QR scanning. Additionally, you can choose to export your
> TOTP accounts to a JSON file. Alternatively, if you have a LastPass account, you can move your
> TOTP accounts using cloud backup and restore.

> Restriction: You cannot export old TOTP accounts.

The doc lists three export routes:

1. **All accounts → QR code(s):** *Transfer accounts > Export accounts to QR code*. Several QR
   codes may be needed. The documented target is the *Import from other device* screen of another
   **LastPass Authenticator** app, which works only on an empty app. The QR payload format is not
   documented. Whether it's a standard `otpauth://` URI or a proprietary migration blob is
   **unverified**, so don't plan on Keeper scanning the bulk QR.
2. **One account → QR code:** select the account → *Export account to QR*, on both Android and iOS.
   This too is documented only for import into another LastPass Authenticator. Whether Keeper's
   scanner accepts it is **unverified**. It probably works if the payload is a plain `otpauth://`
   URI.
3. **All accounts → JSON file:** *Transfer accounts > Export accounts to file > Download JSON
   file*, then "Authenticate with one of your authentication methods", then pick a folder. "Your
   TOTP accounts are exported and saved as a JSON file on your mobile device."

**Does exporting need the LastPass account?** Neither the file export nor the QR export lists a
LastPass login among its steps. The only gate is "authenticate with one of your authentication
methods", which is presumably the app's own biometric/PIN lock (the page doesn't say). Only
**cloud backup/restore** is documented as needing the LastPass account and the LastPass Password
Manager app being logged in (see Q4). So exporting should keep working after the account is
deleted, as long as the app still holds the entries. This is inferred from absence, so
**unverified**, and the safe plan is to export *before* deleting. Also: "If you have exported a
LastPass account, you must pair the LastPass Authenticator again in Multifactor Options to re-enable
push notifications after export." That matters only while LastPass is still in use.

**JSON file format.** LastPass doesn't document it. The best primary source is the open-source
importer in **Bitwarden Authenticator**, which parses this file:
`bitwarden/android` @ `1470154c`,
[`authenticator/.../imports/model/LastPassJsonExport.kt`](https://github.com/bitwarden/android/blob/1470154cf3d3d0511cdbe5bfc6493971b6478edf/authenticator/src/main/kotlin/com/bitwarden/authenticator/data/platform/manager/imports/model/LastPassJsonExport.kt)
and
[`.../parsers/LastPassExportParser.kt`](https://github.com/bitwarden/android/blob/1470154cf3d3d0511cdbe5bfc6493971b6478edf/authenticator/src/main/kotlin/com/bitwarden/authenticator/data/platform/manager/imports/parsers/LastPassExportParser.kt).

- Top level: `deviceId`, `deviceSecret`, `localDeviceId`, `deviceName`, `version`, `accounts[]`,
  `folders[]`.
- Each account: `accountID`, `issuerName`, `originalIssuerName`, `userName`, `originalUserName`,
  `pushNotification`, **`secret`**, `timeStep`, `digits`, `creationTimestamp`, `isFavorite`,
  `algorithm`, `folderData`, `backupInfo`.
- The parser reads `secret` straight in as the key ("Lastpass only supports TOTP codes"), with no
  decryption step. So **the JSON holds seeds in plaintext** and must be handled like a vault
  export.
- `pushNotification: true` marks the LastPass account's own push-MFA entry, which is useless once
  LastPass is gone.

## Q2. What Keeper can import TOTP from

### a) CSV import with an `otpauth://` URI, in bulk

- **Web Vault / Desktop:** Keeper "Import Text File (.csv, .xls, .tsv)",
  https://docs.keeper.io/user-guides/import-records-1/import-a-.csv-file. Field order is
  `Folder,Title,Login,Password,Website Address,Notes,Shared Folder,CustomField1Name,CustomField1Value…`.
  The page's example puts `$oneTimeCode` in a custom field's name and
  `otpauth://totp/?secret=ABC123…` in its value.
- **Release:** Keeper Vault 16.10.0 (released May 2, 2023) lists "Support for TOTP seeds in CSV
  import method",
  https://docs.keeper.io/release-notes/desktop/web-vault-+-desktop-app/vault-release-16.10.0.
- **Commander CLI:** "CSV Import",
  https://docs.keeper.io/keeperpam/commander-cli/command-reference/import-and-export-commands/csv-import.
  Same column order, and "Custom fields begin with the name in the 8th field, (column H)".
  Source (`Keeper-Security/Commander` @ `5be2743f`,
  [`keepercommander/importer/csv/csv.py`](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/importer/csv/csv.py)):
  - The importer has **no header row**: every row with ≥ 6 columns becomes a record, so a header
    line would import as a record titled "Title".
  - Custom field name `$oneTimeCode` maps to field type `oneTimeCode`, which is Keeper's
    Two-Factor Code field.
  - Our converter emits no header. Whether the Web Vault's column-mapping wizard skips a header is
    unverified, and it doesn't matter here.
- **Duplicates and merging:** neither doc page explains how CSV import treats existing records.
  Commander source
  ([`imp_exp.py` `prepare_record_add_or_update`](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/importer/imp_exp.py))
  skips only *100% identical* records. Without `--update` it otherwise creates a new record,
  "risking creating an almost-duplicate". `--update` matches on type+title+login+url+folder, and
  its docstring says it then "request[s] a change in password only". **So no documented import
  path adds a Two-Factor Code to an existing login record.** Imported TOTP entries arrive as new
  records. The agent doesn't run Commander anyway, and the human can just use the Web Vault.

### b) Adding codes one record at a time

Keeper "Storing Multi-Factor (MFA) Codes",
https://docs.keeper.io/enterprise-guide/storing-two-factor-codes, lists these per-record methods:

- **Scan** (Desktop app only): drag a scanner window over an on-screen QR code.
- **Upload a QR code image** (.jpg, .png, …).
- **Manual Entry (advanced).**
- **Mobile:** *Add Two-Factor Code* on the record edit screen, using the device camera.

Keeper's "Protecting TOTP Codes" page,
https://docs.keeper.io/user-guides/tips-and-tricks/protecting-totp-codes, recommends storing the
code "within your records, alongside your login credentials".

Combined with LastPass's per-account *Export account to QR* (Q1, route 2), this gives a route
**straight into the existing login record**, with no file and no duplicates. It depends on
Keeper's scanner accepting that QR, which is **unverified**; test it on one entry. If it works,
it's the cleanest route for a small number of entries.

### c) iOS 26+ app-to-app import, including from Apple Passwords

- Keeper iOS 17.6.0 (released Oct 16, 2025),
  https://docs.keeper.io/release-notes/mobile/apple-ios/ios-version-17.6.0: "a new and secure way
  to import your passwords directly from other apps … transfer your saved passwords, passkeys, and
  two-factor authentication (2FA) codes from sources like Apple Passwords right into your vault."
  Requires iOS 26 or higher.
- Keeper iOS user guide, https://docs.keeper.io/user-guides/ios (Import & Export → "iOS Version
  26+"): the export always starts in the source app, for example *Export Data to Another App*. You
  pick logins, choose Keeper, and authenticate. Records land in **a new folder named after the
  source app** (for example "Apple Passwords").
- Apple's platform API for this, `ASImportableCredential.TOTP` ("A type to represent a time-based
  one-time password generator"), is available from iOS/macOS 26.0 and carries
  secret/period/digits/userName/algorithm/issuer:
  https://developer.apple.com/documentation/authenticationservices/asimportablecredential/totp.
- **Not useful for LastPass Authenticator:** it isn't documented as a credential-exchange source.
  Apple's iPhone guide pages for iOS 26
  ([export](https://support.apple.com/guide/iphone/iphf28f2e93e/26.0/ios/26.0),
  [import](https://support.apple.com/guide/iphone/iph984b8ac1f/26.0/ios/26.0)) document only
  Safari-settings **CSV** export/import of *passwords*. They don't mention verification codes or
  an `OTPAuth` column, and don't describe the Passwords-app "Export Data to Another App" flow.
  Routing through Apple Passwords would mean loading seeds into Apple Passwords first, either one
  QR/setup key at a time or via a CSV `OTPAuth` column (**unverified**: Apple doesn't document
  it), and then exporting to Keeper. That's an extra hop with nothing gained over (a).

### d) Keeper's LastPass importer

Not a route. LastPass's vault export doesn't carry Authenticator-app entries. LastPass Authenticator
is a separate app with its own export (Q1). The vault inventory (#1639) also shows **0** vault
items with a TOTP seed, so there's nothing to bring over from the vault side.

## Q3. A converter that never prints a seed

The script runs on the human's machine (stdlib Python 3, run with `-I`). It:

- reads the LastPass Authenticator JSON;
- writes a Keeper CSV, with no header, to a **new** file created with mode **0600** (it refuses to
  overwrite or write to stdout);
- puts every record in folder `LastPass Authenticator import`, titled with the LastPass display
  issuer, Login = display username, and custom field `$oneTimeCode` =
  `otpauth://totp/<issuer>:<account>?secret=…&issuer=…&algorithm=…&digits=…&period=…`;
- skips `pushNotification` entries (LastPass's own MFA) unless `--include-lastpass-push` is
  given;
- prints **only** allowlisted metadata (`n,title,login,algorithm,digits,period,action`) plus
  counts. Errors print only the exception *type*, never its message (which could quote a value).
  The seed never enters the report dict, so a bug fails closed.

```python
#!/usr/bin/env python3
"""Convert a LastPass Authenticator JSON export into a Keeper CSV import file.

Seeds go only into the output file (created mode 0600). Nothing printed to the
terminal ever contains a seed: the report is built from an explicit allowlist of
metadata fields, and errors print only the entry index and exception type.

Usage: python3 -I lpa2keeper.py <export.json> <out.csv> [--include-lastpass-push]
"""
import csv, json, os, sys
from urllib.parse import quote, urlencode

FOLDER = "LastPass Authenticator import"
REPORT_FIELDS = ("n", "title", "login", "algorithm", "digits", "period", "action")  # allowlist
ALGOS = {"SHA1", "SHA256", "SHA512"}


def report(row):
    # Fail closed: only allowlisted keys are ever printed.
    print("  ".join(f"{k}={row.get(k, '')!s}" for k in REPORT_FIELDS))


def main(argv):
    if len(argv) < 3 or argv[2] in ("-", "/dev/stdout"):
        sys.exit("usage: lpa2keeper.py <export.json> <out.csv> [--include-lastpass-push]")
    src, dst = argv[1], argv[2]
    include_push = "--include-lastpass-push" in argv[3:]
    with open(src, encoding="utf-8") as f:
        accounts = json.load(f).get("accounts") or []

    fd = os.open(dst, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)  # refuse to overwrite
    written = skipped = 0
    with os.fdopen(fd, "w", newline="", encoding="utf-8") as out:
        w = csv.writer(out)  # Keeper CSV has no header row: every row is a record
        for n, a in enumerate(accounts, 1):
            meta = {"n": n}
            try:
                title = a.get("issuerName") or a.get("originalIssuerName") or "Untitled"
                login = a.get("userName") or a.get("originalUserName") or ""
                algo = str(a.get("algorithm") or "SHA1").upper()
                digits, period = int(a.get("digits") or 6), int(a.get("timeStep") or 30)
                meta.update(title=title, login=login, algorithm=algo, digits=digits, period=period)
                if a.get("pushNotification") and not include_push:
                    meta["action"] = "skipped:lastpass-push"; skipped += 1; report(meta); continue
                if algo not in ALGOS:
                    meta["action"] = "skipped:unsupported-algorithm"; skipped += 1; report(meta); continue
                secret = "".join(str(a.get("secret") or "").split()).upper()
                if not secret:
                    meta["action"] = "skipped:empty-secret"; skipped += 1; report(meta); continue
                issuer = a.get("originalIssuerName") or title
                label = quote(f"{issuer}:{a.get('originalUserName') or login}", safe=":@")
                params = urlencode({"secret": secret, "issuer": issuer, "algorithm": algo,
                                    "digits": digits, "period": period}, quote_via=quote)
                # Folder,Title,Login,Password,Website Address,Notes,Shared Folder,$name,$value
                w.writerow([FOLDER, title, login, "", "", "", "", "$oneTimeCode",
                            f"otpauth://totp/{label}?{params}"])
                del secret, params
                meta["action"] = "written"; written += 1; report(meta)
            except Exception as e:  # never echo the exception text: it could hold field values
                meta["action"] = f"error:{type(e).__name__}"; skipped += 1; report(meta)
    print(f"accounts={len(accounts)} written={written} skipped={skipped} out={dst} (mode 0600)")


if __name__ == "__main__":
    main(sys.argv)
```

**Test (fake data only):** a hand-made 4-entry file with dummy `FAKESEED…` values covered a normal
entry, a LastPass push entry, a SHA256/8-digit entry with a spaced lowercase seed, and a malformed
`timeStep`. Results:

- The run printed 4 metadata lines plus `accounts=4 written=2 skipped=2`.
- `grep -c FAKESEED` on stdout returned **0**.
- The output file was `-rw-------`.
- The CSV rows matched the Commander column layout.
- The malformed entry printed `error:ValueError` with blank metadata, which is the fail-closed
  behaviour.

**Caveats:**
- That the otpauth label and query parameters survive Keeper's import is **unverified**. Keeper's
  own example uses a bare `otpauth://totp/?secret=…`. If an imported code doesn't match the phone,
  check `algorithm`/`digits`/`period` first.
- The script doesn't securely delete anything. Afterward the human should remove the JSON
  (phone + computer) and the CSV, and empty the trash.

## Q4. The cloud backup when the LastPass account is deleted

**Sources:**

- "Manage cloud backup for the LastPass Authenticator app" (last updated Dec 03, 2025):
  https://support.lastpass.com/s/document-item?language=en_US&bundleId=lastpass&_LANG=enus&topicId=LastPass%2Fenable-cloud-backup.html
  - The backup is *to LastPass*: "you can back-up your multifactor accounts to LastPass". It's
    stored "with your logged in account". Enabling it needs the LastPass Password Manager app
    installed **and logged in** with that account's master password.
  - "If you delete your cloud backup … your accounts in the cloud will be removed and cloud sync
    will be disabled." Tip: "Before deleting your cloud backup, you can export all your TOTP
    accounts to a file."
- "Restore MFA accounts from cloud backup" (last updated Jun 08, 2026):
  https://support.lastpass.com/s/document-item?language=en_US&bundleId=lastpass&_LANG=enus&topicId=LastPass%2FLastPass_Authenticator_restore_using_cloud_backup.html
  - Restore goes through *Open LastPass* → "Use <LastPass account email address> account". It
    can't happen without the account.
  - "If you create a new back up to LastPass, the old back up is deleted and LastPass Support
    will not be able to recover the old back up data."
- "Delete your LastPass account" (last updated Jul 02, 2026):
  https://support.lastpass.com/s/document-item?language=en_US&bundleId=lastpass&_LANG=enus&topicId=LastPass%2Fdelete-account.html
  - "Deleting your LastPass account is permanent and cannot be undone."
  - Data LastPass holds outside the vault has a separate deletion request through the Individual
    Rights Management Portal.
- "Do you retain any information on former users of LastPass?" (last updated Feb 26, 2026):
  https://support.lastpass.com/s/document-item?language=en_US&bundleId=lastpass&_LANG=enus&topicId=LastPass%2FFAQ_retain_info_users.html
  - "Once your LastPass account is deleted, your account information may be retained for 90 days.
    After this period, the account data is permanently deleted."
- "Delete your data using the LastPass Authenticator app" (last updated Dec 03, 2025):
  https://support.lastpass.com/s/document-item?language=en_US&bundleId=lastpass&_LANG=enus&topicId=LastPass%2Fdelete_your_data_using_the_lastpass_authenticator_app.html
  - Delete the cloud backup in-app, then delete the app.

**Conclusion:** no page says outright what happens to the Authenticator cloud backup on account
deletion (**unverified**). But the backup belongs to the account, restore requires logging into
that account, and deleted accounts are kept at most 90 days and then purged. So **after account
deletion the backup is unrecoverable**, and it may sit at LastPass for up to 90 days. Whether the
on-device entries keep generating codes after deletion is also undocumented (**unverified**). TOTP
is computed locally, so they probably do, but don't rely on it.

**Ordering for the aftermath ticket:**

1. Export the JSON.
2. Convert and import into Keeper.
3. Verify every code.
4. In the app: *Delete your data from cloud*, so the seeds don't sit at LastPass for 90 more
   days.
5. Delete the LastPass account.
6. Optionally file an Individual Rights Management deletion request.
7. Uninstall LastPass Authenticator.
