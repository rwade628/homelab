# Keeper bulk-edit mechanics: Commander vs Web Vault (Family plan)

Research for the post-LastPass Keeper vault cleanup (secrets stack, homelab#1634; mechanics
frontier #1648). The plan: rename ~400 record titles, rewrite `login_url` from `http://` to the
`https://` origin, move records out of the Classic shared folder `Shared-Ryan's Stuff` (333) and
root (72) into new shared folders and subfolders, convert ~20 records to other types, and merge
duplicates. A human runs any CLI. An agent-built script may generate commands from a reviewed TSV.
Nothing may print a secret value.

**How this was researched.** Only documentation and source were read. No `keeper`, `ksm`, `bw`,
`bws` or `sops -d` was run. The sources are:

- Keeper Commander source at commit
  [`5be2743`](https://github.com/Keeper-Security/Commander/tree/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907)
  (2026-10-08, `__version__ = '18.1.8'`). Links below are permalinks to that commit. `SRC` is
  short for `https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander`.
- docs.keeper.io (Commander reference, user guides, Web Vault release notes).

Where the docs and the source disagree, or the docs say nothing, the source wins. Each such case
is marked **(source only)**.

---

## 1. Commander on a personal/Family account

**Supported for vault commands. Admin commands are gated.**

- The CLI gates commands at one point only: if a command is in the `enterprise_commands`
  registry and the session has no enterprise, it fails with *"This command is restricted to
  Keeper Enterprise administrators."* MSP commands are gated the same way.
  ([SRC/cli.py#L439-L452](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/cli.py#L439-L452))
  Every command this plan needs (`record-update`, `edit`, `mv`, `ln`, `mkdir`, `share-folder`,
  `convert`, `convert-all`, `run-batch`, `ls`, `list`, `search`, `get`, `find-duplicate`,
  `import`, `export`) is registered in the ordinary `commands` registry. So none of them depend
  on plan type.
  ([SRC/commands/base.py#L64-L113](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/base.py#L64-L113)) **(source only)**
- docs.keeper.io does not state which account types Commander supports. The overview lists
  *"Authenticate with Master Password, MFA, SSO or Biometrics with Passkeys"* and says nothing
  about plan type. ([Commander overview](https://docs.keeper.io/en/keeperpam/commander-cli/overview))
- **Login: master password plus 2FA works.** The login flow handles TOTP, SMS, Duo, RSA,
  U2F/WebAuthn, Keeper DNA and backup codes
  ([SRC/loginv3.py#L199-L202](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/loginv3.py#L199-L202)).
  The docs say Commander *"prompts for 2FA code on every login"* unless you set
  `2fa_duration=30_days|forever`. A new device may need approval (email code, Keeper Push, or 2FA).
  They also warn that persistent login ("stay logged in") *"affects all devices that you use with
  Keeper"*. ([Logging in](https://docs.keeper.io/en/keeperpam/commander-cli/commander-installation-setup/logging-in))
- **One possible restriction: record types must be enabled.** `convert` and `convert-all` refuse to
  run with *"Cannot convert record(s) if record types is not enabled"* when the account's
  `record_types_enabled` setting is false
  ([SRC/commands/convert.py#L408-L413, #L578-L581](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/convert.py#L408-L413)).
  No doc ties this setting to plan type. A dry run (`convert -n …`) is a harmless way to check it.

## 2. Commands

### 2a. Change title and URL in place

**Both commands address a record by UID, or by `folder/path/Title` (case-insensitive title match
in that folder).** Use UIDs: titles are about to change, and duplicates share titles.
([SRC/commands/base.py#L911-L935](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/base.py#L911-L935))

| Command | Works on legacy v2 records? | Syntax |
|---|---|---|
| `record-update` (alias `ru`) | **No.** On a v2 `PasswordRecord` it raises *"Legacy record type is not supported. Convert the record to login record type."* ([SRC/commands/record_edit.py#L1354-L1356](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/record_edit.py#L1354-L1356)) **(source only; the docs don't mention it)** | `record-update -r <UID> -t "Chewy" url=https://www.chewy.com` ([parser](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/record_edit.py#L68-L78); field syntax `[f.\|c.]<type>[.<label>]=<value>`, [L119-L138](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/record_edit.py#L119-L138)). It changes only the fields you pass ([docs](https://docs.keeper.io/keeperpam/commander-cli/command-reference/record-commands/creating-and-updating-records.md)). |
| `edit` (deprecated) | **Yes.** On version ≤2, or with any legacy option, it hands off to the v2 editor, which sets `record.title` and `record.login_url` and calls `api.update_record`. ([SRC/commands/recordv3.py#L464-L545](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/recordv3.py#L464-L545), [SRC/commands/recordv2.py#L220-L334](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/recordv2.py#L220-L334)) On a v3 `login`/`general` record it maps `--title`/`--url` to v3 options. | `edit -r <UID> --title "Chewy" --url "https://www.chewy.com"` (`-r` is required; [parser L85-L108](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/recordv3.py#L85-L108)). The docs mark it *"Deprecated: use record-update"* ([Record commands](https://docs.keeper.io/en/keeperpam/commander-cli/command-reference/record-commands)). |
| `rename` | Not a record command. Folder rename is `rndir` ([Record commands](https://docs.keeper.io/en/keeperpam/commander-cli/command-reference/record-commands)). | n/a |

There are two ways to sequence this:

- **Option A:** run `convert-all` first. Then everything is v3 `login`, and the non-deprecated
  `record-update` works throughout.
- **Option B:** use `edit` on the v2 records as they are.

Option A also covers the ~20 type changes: `record-update -rt` changes the type of a record that
is already typed ([L1357-L1365](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/record_edit.py#L1357-L1365)).

### 2b. Move (`mv`) between folders and shared folders

- **Syntax:** `mv [-f] [-s] [-e] [--shared-folder|--user-folder] SRC DST`.
  - `SRC` is a record UID, a folder UID, a path, or a glob pattern on titles in a folder.
  - `DST` must be an existing folder (path or UID).
  - `-s`/`-e` set can-reshare/can-edit when the record lands in a shared folder.
  ([SRC/commands/folder.py#L131-L142](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/folder.py#L131-L142))
- **The UID is preserved.** `mv` sends a single `move` request listing the existing `uid`.
  - When source and destination are in different shared folders, the client re-encrypts the record
    key with the destination shared folder key (a "transition key").
  - The record's data is not rewritten.
  ([SRC/commands/folder.py#L905-L1038](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/folder.py#L905-L1038)) **(source only)**
- **Password history and attachments.** These are both inferred from the move request, not from
  a doc.
  - *History:* because the record body isn't rewritten and the UID stays the same, there is nothing
    that would reset the revision history.
  - *Attachments:* the v2 attachment file keys live in the record's `extra`, encrypted under the
    record key, and the record key itself is kept, only re-wrapped.
  - No Keeper doc says either of these outright. **Test one record first** (`record-history <UID>`
    before and after a move; its default output lists revisions).
- **Other users' access.** Inferred from the data model, not stated in any doc.
  - Access granted *through* the source shared folder ends when the record leaves it.
  - Members of the destination shared folder gain access.
  - Direct record shares belong to the record, not the folder, so they should survive.
  - The docs say only that a private folder moved into a shared folder *"will now inherit the
    permissions set from the shared folder"*.
    ([Shared Folders](https://docs.keeper.io/en/enterprise-guide/sharing/folders))
  - Check with `share-report -r <UID>` after the test move.
- **Pitfalls:**
  - `mv <UID> DST` on a record that has shortcuts in several folders moves it out of the current
    folder, or else the root, or else the *first* folder found
    ([L844-L862](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/folder.py#L844-L862)).
    `cd` into the intended source first, or check shortcuts with `shortcut list`.
  - A single request is capped at ~990 items ([L918-L920](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/folder.py#L918-L920)).
  - `ln` takes the same arguments but creates a shortcut instead of moving.

### 2c. Create shared folders and subfolders, and share them

- `mkdir -sf [-a|-u|-r|-s|-e] "Name"` creates a shared folder at root or under a user folder.
  - Inside a shared folder it fails with *"Shared folders cannot be nested"*.
  - A plain `mkdir "SF Name/Sub"` inside a shared folder makes a `shared_folder_folder` (subfolder)
    automatically.
  - The flags set default permissions: `-a` all, `-u` manage users, `-r` manage records,
    `-s` can share, `-e` can edit.
  ([SRC/commands/folder.py#L116-L126, #L553-L600](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/folder.py#L553-L600))
  The docs agree: *"Shared folders within other shared folders are currently not supported"*,
  subfolders inherit permissions from the parent.
  ([Shared Folders](https://docs.keeper.io/en/enterprise-guide/sharing/folders))
- `share-folder [-a grant|remove] -e <email|team|@existing|*> [-p on|off] [-o on|off] [-r <rec|@existing|*> -s on|off -d on|off] FOLDER...`
  - `-p` is manage-records, `-o` is manage-users.
  - `-s`/`-d` are per-record can-share/can-edit.
  - Alias `sf`.
  ([SRC/commands/register.py#L96-L148](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/register.py#L96-L148))
  Example: `share-folder -e spouse@example.com -p off -o off "Household"`.

### 2d. Change record type, especially legacy v2 to typed v3

- **Commander `convert`:**
  - Syntax: `convert [-t <type>] [-n] [-r] [-f] [-u <url-glob>] [--ignore-ownership] <UID|title-glob>...`.
  - The default type is `login`. `-n` is a dry run.
  - By default it skips records with attachments; `-f` includes them, with the warning *"file
    attachments may not be decrypted by Keeper clients"*.
  - It converts only records you own unless you pass `--ignore-ownership`.
  ([SRC/commands/convert.py#L46-L73](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/convert.py#L46-L73);
  [Record type commands](https://docs.keeper.io/keeperpam/commander-cli/command-reference/record-commands/record-type-commands.md))
- **`convert-all` (alias `ca`, v17+):** converts every v2 record in the vault. Options: `-n` dry
  run, `-f` skip the prompt, `-ia` include attachments, `-io` ignore ownership.
  ([same](https://docs.keeper.io/keeperpam/commander-cli/command-reference/record-commands/record-type-commands.md))
- **The UID is kept.** The `RecordConvertToV3` request carries the *existing* `record_uid` and
  `revision` and goes to `vault/records_convert3`. It also re-sends the record key wrapped under
  each parent shared folder's key, so shared-folder membership is carried over
  ([SRC/commands/convert.py#L258-L276, #L341-L357](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/convert.py#L258-L276)). **(source only)**
- **Field mapping:**
  - `secret1` becomes `login`, `secret2` becomes `password`, `link` becomes `url`.
  - The first TOTP becomes `oneTimeCode`; any further TOTPs become custom `oneTimeCode` fields.
  - v2 custom fields become custom fields of an inferred type (`url`/`email`/`note`/`text`).
  - Attachments become `fileRef`s with **new file UIDs**.
  ([L199-L246, L280-L311](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/convert.py#L199-L246))
- **History is not documented.** No doc says whether revision history survives conversion.
  Check one record with `record-history <UID>` before converting in bulk.
- **Web Vault:**
  - Right-click a record and choose **Change Type**. *"Fields that don't exist in other types will
    be saved as custom fields."* ([Record Types, enterprise guide](https://docs.keeper.io/enterprise-guide/record-types))
  - Since Vault 16.8.0: *"right-click to change the type of any record, including migration from
    the legacy 'General' record types"* and *"Multi-select + right click allows migration of
    multiple records at once."*
    ([Vault 16.8.0 release notes](https://docs.keeper.io/release-notes/desktop/web-vault-+-desktop-app/older/vault-version-16.8.0))
  - UID and history are not documented here either.
  - For the one record with an attachment, the Web Vault's Change Type is the safer path, given
    Commander's warning about attachments.
- **Changing a typed record to another type:** use `record-update -r <UID> -rt <type>`
  ([L1357-L1365](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/record_edit.py#L1357-L1365)).
  Alternatively, convert a v2 record straight to a type with `convert -t <type> <UID>`.
  Common type IDs (confirm with `rti`): `serverCredentials`, `databaseCredentials`,
  `wirelessCredentials` (WiFi), `bankCard`, `encryptedNotes`, `file`.

## 3. Batch execution and metadata-only output

- **`run-batch [-d <sec>] [-q] [-n] <file-glob>...`:**
  - Runs one Commander command per line. Lines starting with `#` are skipped.
  - `-n`/`--dry-run` only *prints* the commands. It does not validate them against the vault.
  - `-d` adds a delay between commands (helps with throttling). `sleep <sec>` also works as a
    line in the file.
  ([SRC/commands/scripting.py#L30-L95](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/scripting.py#L30-L95))
- **Errors don't stop the batch.** Each command's `CommandError` or exception is logged and the
  loop carries on.
- **Commands are echoed.** Unless `-q` is passed, each line is logged as `Executing [<command>]`.
  ([SRC/cli.py#L479-L507](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/cli.py#L479-L507))
  So a batch file must never contain secret values. Title, URL and folder lines are fine.
  Capture the output and grep for `error`/`warning`.
- **Other per-command dry runs:** `convert -n`, `convert-all -n`, `find-duplicate --merge -n`,
  `import --dry-run`. **`record-update`, `edit` and `mv` have no dry run.**

**Which commands print secrets:**

| Command | Prints secrets? | What it prints |
|---|---|---|
| `ls -l [-R] [--format json]` | No | `record_uid, type, title, description, source`. Description is `login @ url`. In recursive/JSON form: `type, uid, name, details`. ([SRC/commands/folder.py#L312-L366](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/folder.py#L312-L366)) Folder UIDs are listed too. |
| `list [--format json\|csv]` | No | `record_uid, type, title, description, shared, record_category` ([SRC/commands/record.py#L1871](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/record.py#L1871)). No folder path. |
| `search [--format json]` | No | UID, type, title, description, category ([L1610-L1618](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/record.py#L1610-L1618)) |
| `find-duplicate [--format json]` | No | group, title, login, url host, uid, owner, shared_to ([SRC/commands/register.py#L2651-L2681](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/register.py#L2651-L2681)) |
| `get <UID>` (default `detail`) | Masked (`********`) unless `--unmask` | ([L648-L731](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/record.py#L648-L731)) |
| **`get <UID> --format json`** | **YES, plaintext** | On v2 it writes `password` unconditionally; on v3 it dumps the full decrypted data. ([L536-L558](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/record.py#L536-L558)) **Do not use it.** |
| `get --format password`, `export`, `find-password` | **YES** | Secret output by design. |

For a UID → folder map, run `ls -l --format json "<folder path>"` once per folder. `list` and
`search` don't report folders.

## 4. Do JSON exports include record UIDs?

- **Commander `export --format json`: yes.** The exporter writes `"uid"` for every record.
  ([SRC/importer/json/json.py#L318-L334](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/importer/json/json.py#L318-L334))
  The UID comes from the vault `record_uid`
  ([SRC/importer/imp_exp.py#L136-L160](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/importer/imp_exp.py#L136-L160)).
  - `$type` is written only when the record has a type. v2 records leave it unset, which matches
    the "`$type` absent → legacy" pattern seen in the existing export.
  - Shared folders are also exported with `uid`.
  - Options: `--folder`, `--owned-only`, `--zip` (adds attachments).
  ([SRC/importer/commands.py#L139-L156](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/importer/commands.py#L139-L156)) **(source only; the docs don't mention record UIDs)**
- **Web Vault export (Settings → Export → JSON): not documented.**
  - The docs say only that it is *"an array of Shared Folder objects, and an array of Record
    objects"*, that it is compatible with Commander, and that it excludes files.
    ([Vault Export](https://docs.keeper.io/user-guides/export-and-reports/vault-export))
  - The import-format examples have no `uid` on records
    ([Import JSON](https://docs.keeper.io/user-guides/import-records-1/import-json)).
  - To check the file already on disk without printing secrets:
    `jq '[.records[] | has("uid")] | group_by(.) | map({(tostring): length})' export.json`.

## 5. Does re-importing an edited export update in place?

**Commander `import`: yes, when the record's `uid` is still in the vault. No flag is needed.**
`prepare_record_add_or_update` handles each imported record in this order
([SRC/importer/imp_exp.py#L2457-L2600](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/importer/imp_exp.py#L2457-L2600)):

1. If the *entire* content matches an existing record (folder-agnostic hash of type, title, login,
   password, url, notes and fields), it is **skipped** and counted as existing.
2. If `import_record.uid` is in the vault, it is **updated in place under that UID**
   (`record_uid_to_update`).
   - v3 records get a `RecordUpdate` carrying the existing revision.
   - v2 records get a v2 update carrying the existing record key and revision.
   ([L1253-L1302](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/importer/imp_exp.py#L1253-L1302))
3. Otherwise, with `--update`, it matches on type, title, login, url and folder, and updates that
   record. Help text: *"update records with common login, url or title"*
   ([SRC/importer/commands.py#L106-L107](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/importer/commands.py#L106-L107)).
   A changed title or URL defeats this match, so it is no use for renames.
4. Otherwise a **new UID** is generated and a new record created, with no history and no shares.

**Folder changes in an import create shortcuts, not moves.** If an existing record's `folders` in
the file differ from where it sits now, import sends `move` with `'link': True`, so the record ends
up in both places
([L2643-L2708](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/importer/imp_exp.py#L2643-L2708)).
Use `mv` for moves.

**`import --dry-run` prints `Folder, Title, Username, URL, Last Modified, Record UID`.** No
password. ([L1197-L1220](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/importer/imp_exp.py#L1197-L1220))

Re-importing still means a plaintext JSON file of every password sits on disk, and the v2 update
rewrites the whole record body from the file. The per-record `edit`/`record-update` batch avoids
both, so it is the better path.

**Web Vault import:** the docs don't say whether a re-import updates or duplicates, and they don't
describe duplicate handling at all.
([Import JSON](https://docs.keeper.io/user-guides/import-records-1/import-json))
Assume it creates new records.

## 6. Web Vault bulk features

- **Multi-select:** Ctrl/Cmd-click, or Shift-click for a range.
  ([Web Vault guide](https://docs.keeper.io/user-guides/web-vault))
- **Bulk move:** *"hold 'shift' and click the items to drag-and-drop"*, then choose Move or Create
  shortcut. ([same](https://docs.keeper.io/user-guides/web-vault);
  [Shared Folders](https://docs.keeper.io/en/enterprise-guide/sharing/folders))
- **Bulk delete:** multi-select, right-click, Delete.
- **Bulk change type:** *"Multi-select + right click allows migration of multiple records at
  once"* ([16.8.0](https://docs.keeper.io/release-notes/desktop/web-vault-+-desktop-app/older/vault-version-16.8.0)).
- **Bulk permission changes:** recursive/retroactive folder-permission application
  ([16.8.0](https://docs.keeper.io/release-notes/desktop/web-vault-+-desktop-app/older/vault-version-16.8.0)).
- **Bulk edit of titles/URLs:** none documented.

## Duplicates: `find-duplicate --merge` deletes, it doesn't merge

`find-duplicate --merge` groups records only when *all* fields match, shares included (unless
`--ignore-shares-on-merge`). It then **deletes** every record in a group except the first, via
`rm --force`. No fields are combined.
([SRC/commands/register.py#L2526-L2541, #L2552-L2566](https://github.com/Keeper-Security/Commander/blob/5be2743fbbc652e48ba0d5bba6e44f7f1fadd907/keepercommander/commands/register.py#L2526-L2541);
[Record commands](https://docs.keeper.io/en/keeperpam/commander-cli/command-reference/record-commands))

- Use `find-duplicate --title --url --format json` to *find* near-duplicates. Its output has no
  passwords.
- Do the real merges by hand: keep one UID, copy any unique fields over in the Web Vault, then
  `rm` the others.

## Suggested sequence (not yet tested on this vault)

1. **Read-only inventory:** `ls -l --format json "<folder>"` for each folder, `find-duplicate
   --title --url --format json`, and `convert-all -n` (confirms record types are enabled).
2. **Pilot on 1–2 records:**
   - Run `record-history <UID>` and `share-report -r <UID>`, then `convert`, `record-update`,
     `mv` across shared folders, then `record-history`/`share-report` again.
   - This confirms the history and access points marked "inferred" above.
3. **Bulk run:** `convert-all -f` (attachment records stay skipped), then a reviewed `run-batch`
   file of
   - `record-update -r <UID> -t "<title>" url=<https-origin>`
   - `mkdir -sf` / `mkdir` lines
   - `mv <UID> "<dest path>"` lines

   Use `-d 1` and capture the log.
4. **Type changes:** `record-update -r <UID> -rt <type>` (field data may need manual re-entry), or
   Web Vault Change Type. Change the one attachment record in the Web Vault.
