---
name: share-with
description: Silently share Google Docs, Sheets, Slides, and Drive files or folders with email recipients as reader (default), commenter, or editor with notifications permanently disabled.
---

# share-with Skill

Use `share-with` whenever an operator or autonomous agent needs to grant Google Docs or Google Drive access (`reader`, `commenter`, or `editor`) to one or more email addresses without sending notification emails.

## Prerequisites & Headless Authentication

1. Verify credential readiness without opening a browser:
   ```bash
   share-with auth status --json
   ```
2. Supported headless credential sources (resolved in order):
   - `SHARE_WITH_ACCESS_TOKEN` environment variable (raw OAuth2 bearer token, e.g. `export SHARE_WITH_ACCESS_TOKEN="$(gcloud auth print-access-token)"`)
   - `--credentials <path>` or `SHARE_WITH_CREDENTIALS` / `GOOGLE_APPLICATION_CREDENTIALS` (Service Account JSON, Authorized User JSON, or Desktop OAuth Client JSON paired with `~/.config/share-with/token.json`)
   - Application Default Credentials (`gcloud auth application-default login --scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform`)

## Recommended Agent Workflow

1. **Dry-Run First (`--dry-run --json`)**:
   Validate the document URL, recipient list (shorthands like `@alice`, `doc-reviewers@`, and `bob@` automatically expand to `@google.com` unless `--default-domain` overrides it), and role mapping before mutating permissions:
   ```bash
   share-with grant "https://docs.google.com/document/d/<FILE_ID>/edit" \
     --emails "@alice, team-leads@google.com, doc-reviewers@, bob@" \
     --role reader \
     --dry-run --json
   ```
2. **Execute Silent Grant (`--json`)**:
   ```bash
   share-with grant "https://docs.google.com/document/d/<FILE_ID>/edit" \
     --emails "alice@example.com,bob@example.com" \
     --role reader \
     --json
   ```
   Supported `--role` values:
   - `reader` (default): Viewer access (`reader` in Drive API v3)
   - `commenter`: Commenter/suggester access (`commenter` in Drive API v3)
   - `editor`: Full edit access (`writer` in Drive API v3)
3. **Verify Permissions (`list --json`)**:
   ```bash
   share-with list "https://docs.google.com/document/d/<FILE_ID>/edit" --limit 25 --json
   ```

## Safety Guarantees

- `share-with` accepts full Google Docs/Sheets/Slides/Forms/Colab/Drive URLs (preserving `?resourcekey=...` via `X-Goog-Drive-Resource-Keys`) as well as raw Drive file IDs.
- `share-with` always sends `sendNotificationEmail=false` and blocks notification parameters at the HTTP transport layer.
- Before calling `Permissions.Create`, `share-with` checks existing file permissions and skips recipients who already hold an equal or higher role (`status: "already_shared"`), so re-running a full reviewer roster never downgrades `editor` or `commenter` collaborators.
- If a target address has no Google Account, Google Drive rejects silent sharing with HTTP 400. `share-with` reports the failure in `results[].error` and never falls back to sending an invitation email.
