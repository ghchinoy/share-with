# share-with

`share-with` is a Go CLI that shares Google Docs, Sheets, Slides, Forms, Colab notebooks, and Drive files or folders (by URL or raw Drive ID) with a list of recipients as `reader` (default), `commenter`, or `editor`, without ever sending notification emails.

[![Go 1.26+](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go)](go.mod)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [Authentication & Configuration](#authentication--configuration)
- [Silent-Only Guarantee](#silent-only-guarantee)
- [Local Development](#local-development)
- [Publishing](#publishing)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

## Installation

Install directly with Go 1.26 or newer:

```bash
go install github.com/ghchinoy/share-with@latest
```

Or build a local binary from source:

```bash
git clone https://github.com/ghchinoy/share-with.git
cd share-with
make build
# Binary is written to ./bin/share-with
```

### Install the Agent Plugin & Skill

`share-with` ships an [Agent Plugins v1.0.0](https://agent-plugins.org) package under [`plugins/share-with/`](plugins/share-with/plugin.json) indexed by [`.claude-plugin/marketplace.json`](.claude-plugin/marketplace.json):

```bash
# Open Agent Skills CLI (npx skills)
npx skills add ghchinoy/share-with

# Antigravity CLI
agy plugin install https://github.com/ghchinoy/share-with

# Claude Code
claude plugin marketplace add ghchinoy/share-with
claude plugin install share-with@share-with

# Codex CLI
codex plugin marketplace add ghchinoy/share-with
codex plugin add share-with@share-with

# Gemini CLI (from a local clone)
gemini skills install --path plugins/share-with/skills/share-with
```

## Usage

Authenticate once with Google Cloud Application Default Credentials including the `drive` scope (a plain `gcloud auth application-default login` without `--scopes` only grants `cloud-platform` and will fail with `403 ACCESS_TOKEN_SCOPE_INSUFFICIENT`):

```bash
gcloud services enable drive.googleapis.com --project=<YOUR_PROJECT_ID>
gcloud auth application-default login \
  --scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform
gcloud auth application-default set-quota-project <YOUR_PROJECT_ID>
```

> **Google Workspace / `@google.com` accounts:** If your organization blocks `gcloud`'s built-in OAuth client from requesting the `drive` scope, pass your own Desktop OAuth client JSON via `gcloud auth application-default login --client-id-file=~/secrets/client_secret.json --scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform`, or run `share-with auth login --credentials ~/secrets/client_secret.json` (see [Authentication & Configuration](#authentication--configuration)).

Share a Google Doc silently as `reader` (the default role) using either a full URL (including `?resourcekey=...` links) or a raw Drive file ID. Domainless handles (`@user`, `user@`, or `user`) automatically expand to `@google.com`, and recipients who already hold an equal or higher role are skipped (`already_shared`) so existing `editor` or `commenter` collaborators are never downgraded:

```bash
share-with "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit?resourcekey=0-abc123" \
  @alice, team-leads@google.com, doc-reviewers@, bob@

# Or pass the raw Drive file ID directly:
share-with 1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms \
  @alice, team-leads@google.com, doc-reviewers@, bob@
```

Sample output:

```text
SILENT SHARE  Architecture Design Doc (1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms)
Document:  https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit |  Notifications: disabled (sendNotificationEmail=false)

  RECIPIENT                  ROLE             STATUS                   PERMISSION ID
  alice@google.com           reader (reader)  shared                   12345678901234567890
  team-leads@google.com      reader (reader)  shared                   09876543210987654321
  doc-reviewers@google.com   reader (reader)  already_shared (reader)  11223344556677889900
  bob@google.com             reader (reader)  already_shared (editor)  99887766554433221100

Summary: 2 succeeded, 2 already shared, 0 failed
```

Supported target inputs:
- Google Docs, Sheets, Slides, Forms, and Drawings URLs (`docs.google.com/.../d/<ID>/edit`, with optional `?resourcekey=...`)
- Google Drive file and folder URLs (`drive.google.com/file/d/<ID>/view`, `drive.google.com/drive/folders/<ID>`, `drive.google.com/open?id=<ID>`)
- Google Colab Drive notebook URLs (`colab.research.google.com/drive/<ID>`)
- Raw Google Drive file or folder IDs (`1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms`)

You can also test URL parsing and role mapping immediately with zero credentials using `--dry-run`:

```bash
share-with "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" \
  alice@example.com --role editor --dry-run --json
```

### Role Options & Input Modes

Pass `--role` (or `-r`) to choose one of the three supported access levels:

| CLI `--role` | Drive API v3 Role | Access Granted |
| :--- | :--- | :--- |
| `reader` *(default)* | `reader` | Read-only viewer access |
| `commenter` | `commenter` | View and comment or suggest edits |
| `editor` | `writer` | Full edit access |

```bash
# Silently grant commenter access using comma-separated --emails
share-with --url "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" \
  --emails alice@example.com,bob@example.com \
  --role commenter

# Silently grant editor access from a newline-delimited file (# comments supported, '-' for stdin)
share-with "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" \
  --emails-file reviewers.txt \
  --role editor

# Inspect existing document permissions (bounded to 25 results by default)
share-with list "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" --limit 25
```

### Subcommands

| Command | Group | Purpose |
| :--- | :--- | :--- |
| `share-with [doc-url] [email...]` | Root | Share a document silently (prints help when invoked with no arguments) |
| `share-with grant [doc-url] [email...]` | Sharing | Explicit subcommand for silent permission grants (`--dry-run`, `--emails-file`) |
| `share-with list <doc-url>` | Inspection | List current document permissions (`--limit 25`, `--role`, `--json`) |
| `share-with auth status` | Configuration | Check active headless credential readiness (`--json`) |
| `share-with auth login` | Configuration | Run loopback OAuth2 PKCE login for a Desktop client JSON |

## Authentication & Configuration

`share-with` never blocks on interactive browser prompts during `grant` or `list` calls. It resolves credentials in the following order:

1. **Bearer Token (`SHARE_WITH_ACCESS_TOKEN`)**: Pass a short-lived OAuth2 token via environment variable rather than CLI flags to keep secrets out of shell history:
   ```bash
   SHARE_WITH_ACCESS_TOKEN="$(gcloud auth print-access-token)" \
     share-with "https://docs.google.com/document/d/<FILE_ID>/edit" alice@example.com
   ```
2. **Credentials JSON (`--credentials`, `SHARE_WITH_CREDENTIALS`, or `GOOGLE_APPLICATION_CREDENTIALS`)**: Accepts Service Account keys (with optional `--impersonate user@domain.com` for Workspace Domain-Wide Delegation), Authorized User files, or Desktop OAuth client JSON paired with `~/.config/share-with/token.json`:
   ```bash
   share-with auth login --credentials ~/secrets/client_secret.json
   share-with auth status
   ```
3. **Application Default Credentials (ADC)**: Uses `gcloud` default credentials with `https://www.googleapis.com/auth/drive` scope, automatically extracting `quota_project_id` from `~/.config/gcloud/application_default_credentials.json` or falling back to your active `gcloud` configuration (`~/.config/gcloud/configurations/config_<active>`).

Optional persistent defaults live in `$XDG_CONFIG_HOME/share-with/config.yaml` (defaulting to `~/.config/share-with/config.yaml`) and resolve in `Flag > Environment > Config File > Default` order:

```yaml
default_role: reader
default_domain: google.com
credentials_file: ~/.config/share-with/service-account.json
quota_project: my-gcp-project
impersonate: workspace-owner@example.com
```

| Flag | Environment Variable | Description |
| :--- | :--- | :--- |
| `--role`, `-r` | `SHARE_WITH_ROLE` | Access role: `reader` (default), `commenter`, or `editor` |
| `--default-domain` | `SHARE_WITH_DEFAULT_DOMAIN` | Domain appended to `@user`, `user@`, or `user` handles (default `google.com`) |
| `--credentials` | `SHARE_WITH_CREDENTIALS`, `GOOGLE_APPLICATION_CREDENTIALS` | Path to Google JSON credentials |
| `--token-file` | `SHARE_WITH_TOKEN_FILE` | Path to cached OAuth token (`~/.config/share-with/token.json`) |
| *(env only)* | `SHARE_WITH_ACCESS_TOKEN` | Raw OAuth2 bearer token for zero-file headless execution |
| `--quota-project` | `SHARE_WITH_QUOTA_PROJECT`, `GOOGLE_CLOUD_QUOTA_PROJECT`, `GOOGLE_CLOUD_PROJECT`, `GCLOUD_PROJECT`, `CLOUDSDK_CORE_PROJECT` | Quota project sent via `X-Goog-User-Project` |
| `--impersonate` | `SHARE_WITH_IMPERSONATE` | User email for Service Account domain-wide delegation |
| `--json` | *(flag only)* | Emit deterministic JSON output |
| *(env only)* | `NO_COLOR`, `SHARE_WITH_NO_TUI` | Disable ANSI colors in human output |

## Silent-Only Guarantee

Google Drive API v3 defaults `sendNotificationEmail` to `true` when omitted on `user` and `group` permissions. `share-with` blocks notifications at three independent layers:

1. **No Notification Flags**: The CLI exposes no `--notify`, `--send-notification-email`, or custom message flags, and rejects `owner` and `organizer` roles because Google Drive forces email notifications on ownership transfers.
2. **Explicit API Call & No-Downgrade Guard**: Every `Permissions.Create` call sets `sendNotificationEmail=false` and `supportsAllDrives=true` (plus `X-Goog-Drive-Resource-Keys` when a URL carries `?resourcekey=...`). Before creating permissions, `share-with` checks existing ACLs and skips recipients who already hold an equal or higher role (`already_shared`), preventing accidental downgrades of `editor` or `commenter` collaborators.
3. **HTTP Wire Guard**: `SilentGuardTransport` in [`internal/sharer/transport.go`](internal/sharer/transport.go) inspects every outgoing `POST .../permissions` request and aborts locally if `sendNotificationEmail != "false"`, if `transferOwnership=true`, or if `emailMessage` is present.

## Local Development

Prerequisites:
- **Go 1.26+** (`go version`)
- **GNU Make** (`make --version`)
- No network sockets or external Google credentials are required to run the test suite.

Clone, build, run in dry-run mode, and execute the full test suite:

```bash
git clone https://github.com/ghchinoy/share-with.git
cd share-with
make tidy
make run ARGS='"https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" alice@example.com --dry-run'
make fmt
make lint
make test
```

For architectural invariants and AI agent workflows, see [`AGENTS.md`](AGENTS.md), [`GEMINI.md`](GEMINI.md), and the Agent Plugin package in [`plugins/share-with/`](plugins/share-with/skills/share-with/SKILL.md).

## Publishing

Tag a semantic version on `main` after `make lint && make test` pass so consumers can install the release via `go install`:

```bash
make fmt && make lint && make test
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

## Troubleshooting

- **HTTP 403 (`ACCESS_TOKEN_SCOPE_INSUFFICIENT` / `Request had insufficient authentication scopes`)**: Standard `gcloud auth application-default login` omits the Google Drive scope. Re-run ADC login with explicit scopes:
  ```bash
  gcloud auth application-default login \
    --scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform
  ```
  If your Google Workspace organization blocks `gcloud`'s default OAuth client from requesting Drive scopes, supply a Desktop OAuth client JSON (for example from a personal or sandbox GCP project) either to `gcloud` (`--client-id-file=<desktop_client.json>`) or directly via `share-with auth login --credentials <desktop_client.json>`.
- **HTTP 403 (`The drive.googleapis.com API requires a quota project` / `projects/764086051850` / `SERVICE_DISABLED`)**: Local ADC user tokens require a quota project where the Google Drive API is enabled. `share-with` automatically reads `quota_project_id` from `~/.config/gcloud/application_default_credentials.json` and falls back to your active `gcloud` project, or you can set one explicitly:
  ```bash
  gcloud services enable drive.googleapis.com --project=<YOUR_PROJECT_ID>
  gcloud auth application-default set-quota-project <YOUR_PROJECT_ID>
  # Or pass per-command / via config:
  share-with <DOC_ID> @alice --quota-project <YOUR_PROJECT_ID>
  ```
- **HTTP 404 (`File not found`) on older link-shared items**: Drive files protected by a security update resource key return HTTP 404 when queried by bare file ID alone. Pass the full URL including `?resourcekey=...` so `share-with` sends the `X-Goog-Drive-Resource-Keys` header automatically.
- **HTTP 403 (`insufficientFilePermissions`) on externally owned files**: Viewers or commenters without sharing rights (`canShare=false`) cannot list full permission rosters via `share-with list` or grant access via `share-with grant`. Ask an Editor or Owner of that document to run the grant.
- **Recipient fails with HTTP 400 (`no Google account associated`)**: Google Drive refuses `sendNotificationEmail=false` for email addresses without a Google Account because it requires sending an invitation email. `share-with` never falls back to notifying; share with a Google-backed email or Google Group instead.
- **`published web URL does not contain a shareable Drive file ID`**: Links matching `/document/d/e/2PACX-.../pub` are read-only web publications rather than Drive files. Copy the `/document/d/<FILE_ID>/edit` URL (or raw file ID) from the editor address bar instead.

## Contributing

Pull requests are welcome. For major changes or new subcommands, please open an issue first to discuss the proposed design.

Before submitting a pull request:
1. Run `make fmt && make lint && make test` and confirm all race-enabled tests pass.
2. Preserve the three-layer silent-sharing contract documented in [`AGENTS.md`](AGENTS.md) and avoid binding `127.0.0.1` in unit tests.

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
