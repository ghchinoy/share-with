---
title: CLI & JSON Reference
description: Complete command, flag, URL format, and deterministic JSON output reference for share-with.
---

Every `share-with` command supports human-readable semantic terminal output (respecting `NO_COLOR` and `SHARE_WITH_NO_TUI`) and deterministic `--json` output for scripts and AI coding agents.

## Commands Overview

| Command | Cobra Group | Purpose |
| :--- | :--- | :--- |
| `share-with [doc-url] [email...]` | Root | Silently grant access (prints help when invoked with zero arguments) |
| `share-with grant [doc-url] [email...]` | `sharing` | Explicit subcommand for silent permission grants (alias: `share`) |
| `share-with list <doc-url>` | `inspection` | List existing file permissions (`--limit 25`, `--role`, `--json`) |
| `share-with auth status` | `configuration` | Check active headless credential readiness without browser prompts |
| `share-with auth login` | `configuration` | Run interactive loopback OAuth2 PKCE login for a Desktop client JSON |

---

## Supported Target Formats

Pass any of the following as the positional `<doc-url>` argument or via `--url` / `-u`:

- **Google Docs, Sheets, Slides, Forms, and Drawings**:
  `https://docs.google.com/document/d/<FILE_ID>/edit` (including `?resourcekey=...` and `&tab=t.0`)
- **Google Drive Files and Folders**:
  `https://drive.google.com/file/d/<FILE_ID>/view`, `https://drive.google.com/drive/folders/<FOLDER_ID>`, or `https://drive.google.com/open?id=<FILE_ID>`
- **Google Colab Notebooks**:
  `https://colab.research.google.com/drive/<FILE_ID>?resourcekey=...`
- **Raw Google Drive File or Folder IDs**:
  `1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms` (15 to 128 alphanumeric, `-`, or `_` characters)

Published-to-web URLs (`/document/d/e/2PACX-.../pub`) are rejected with a clear hint because they do not expose an underlying Drive file ID.

---

## Recipient Formats & Shorthand Expansion

Recipients can be passed as positional arguments, via `--emails` / `-e` (comma- or semicolon-separated, repeatable), or via `--emails-file` / `-f` (`-` reads from `stdin`, `#` starts inline comments).

| Input Example | Resolved Address (`--default-domain google.com`) |
| :--- | :--- |
| `alice@example.com` | `alice@example.com` |
| `@alice` | `alice@google.com` |
| `doc-reviewers@` | `doc-reviewers@google.com` |
| `alice` | `alice@google.com` |
| `@team-leads@google.com` | `team-leads@google.com` |
| `"Jane Doe" <jdoe@>` | `jdoe@google.com` |

All addresses are normalized to lowercase and deduplicated in order before calling the Drive API.

---

## Role Mapping

| CLI `--role` (`-r`) | Drive API v3 Role | Access Granted |
| :--- | :--- | :--- |
| `reader` *(default)* | `reader` | Read-only viewer access |
| `commenter` | `commenter` | View and comment or suggest edits |
| `editor` | `writer` | Full edit access |

Passing `--role writer` returns an explicit hint directing callers to use `--role editor`.

---

## Command Details

### `share-with grant`

```bash
share-with grant [doc-url] [email...] [flags]
```

Flags:
- `-u, --url <string>`: Google Docs/Drive URL or raw file ID
- `-e, --emails <strings>`: Comma-separated or repeatable recipient emails or `@handles`
- `-f, --emails-file <path>`: Path to newline- or comma-delimited recipients file (`-` for `stdin`)
- `-r, --role <role>`: `reader` (default), `commenter`, or `editor`
- `--default-domain <domain>`: Domain appended to domainless handles (default `google.com`)
- `--dry-run`: Validate inputs and emit the planned silent payload without network or auth calls

Sample `--json` output:

```json
{
  "file_id": "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
  "file_name": "Architecture Design Doc",
  "canonical_url": "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit",
  "resource_kind": "document",
  "role": "reader",
  "api_role": "reader",
  "dry_run": false,
  "notifications_disabled": true,
  "results": [
    {
      "email": "alice@google.com",
      "role": "reader",
      "api_role": "reader",
      "permission_id": "12345678901234567890",
      "silent": true,
      "status": "shared"
    },
    {
      "email": "bob@google.com",
      "role": "reader",
      "api_role": "reader",
      "existing_role": "editor",
      "permission_id": "99887766554433221100",
      "silent": true,
      "status": "already_shared"
    }
  ],
  "succeeded": 1,
  "skipped": 1,
  "failed": 0
}
```

### `share-with list`

```bash
share-with list <doc-url> [--limit 25] [--role reader|commenter|editor|owner] [--json]
```

Lists active permissions on a Drive item, bounded to `--limit 25` by default to protect agent context windows.

### `share-with auth status` & `share-with auth login`

```bash
share-with auth status --json
share-with auth login --credentials ~/secrets/client_secret.json
```

---

## Global Flags & Environment Variables

| Flag | Environment Variable | Description |
| :--- | :--- | :--- |
| `--role`, `-r` | `SHARE_WITH_ROLE` | Access role: `reader`, `commenter`, or `editor` |
| `--default-domain` | `SHARE_WITH_DEFAULT_DOMAIN` | Default domain for shorthand handles (`google.com`) |
| `--credentials` | `SHARE_WITH_CREDENTIALS`, `GOOGLE_APPLICATION_CREDENTIALS` | Path to Google credentials JSON |
| `--token-file` | `SHARE_WITH_TOKEN_FILE` | Path to cached OAuth token JSON (`~/.config/share-with/token.json`) |
| *(env only)* | `SHARE_WITH_ACCESS_TOKEN` | Raw OAuth2 bearer token for zero-file headless execution |
| `--quota-project` | `SHARE_WITH_QUOTA_PROJECT`, `GOOGLE_CLOUD_QUOTA_PROJECT`, `GOOGLE_CLOUD_PROJECT`, `GCLOUD_PROJECT`, `CLOUDSDK_CORE_PROJECT` | Quota project sent via `X-Goog-User-Project` |
| `--impersonate` | `SHARE_WITH_IMPERSONATE` | Workspace email for Service Account Domain-Wide Delegation |
| `--timeout` | *(flag only)* | Request timeout (default `30s`) |
| `--json` | *(flag only)* | Emit deterministic JSON to `stdout` (and structured JSON errors) |
| *(env only)* | `NO_COLOR`, `SHARE_WITH_NO_TUI` | Disable ANSI terminal colors |
