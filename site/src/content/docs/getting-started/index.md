---
title: Getting Started
description: Install the share-with CLI, configure Google Cloud Drive scopes once, and silently share your first Google Doc or Drive file.
---

`share-with` is a static Go CLI and [Agent Plugins v1.0.0](https://agent-plugins.org) package that grants Google Docs, Sheets, Slides, Forms, Drawings, Colab notebooks, and Google Drive file or folder permissions (`reader`, `commenter`, or `editor`) without ever sending notification emails.

## 1. Install the CLI Binary

Download a prebuilt static archive (`darwin`, `linux`, or `windows` for `amd64` and `arm64`, verified with `checksums.txt`) from [GitHub Releases](https://github.com/ghchinoy/share-with/releases), or install directly with Go 1.26+:

```bash
go install github.com/ghchinoy/share-with@latest
```

To build from source instead:

```bash
git clone https://github.com/ghchinoy/share-with.git
cd share-with
make build
# Binary is written to ./bin/share-with
```

Verify the installation:

```bash
share-with --version
```

:::tip[Using an AI coding assistant?]
After installing the `share-with` binary on your `PATH`, install the bundled plugin so Claude Code, Antigravity, Codex, Gemini CLI, or OpenCode can run silent grants and permission audits on your behalf. See the [Agent Plugin & Skill Guide](../agent-plugin/).
:::

:::note[Sister CLI: drivectl]
Need to search Google Drive, upload files, convert Markdown into formatted Google Docs, read or write Google Sheets A1 ranges, or run a Google Workspace MCP server? Use the sister CLI [`drivectl`](https://github.com/ghchinoy/drivectl) ([documentation](https://ghchinoy.github.io/drivectl/)). Use `share-with` whenever you need silent-only permission grants (`sendNotificationEmail=false` enforced on the wire) and no-downgrade collaborator protection.
:::

## 2. Authenticate Once with Google Drive Scope

Enable the Google Drive API on a Google Cloud project you control and log in with Application Default Credentials (ADC) **including the `drive` scope**. A bare `gcloud auth application-default login` without `--scopes` grants only `cloud-platform` and will fail with `HTTP 403 ACCESS_TOKEN_SCOPE_INSUFFICIENT`:

```bash
gcloud services enable drive.googleapis.com --project=<YOUR_PROJECT_ID>

gcloud auth application-default login \
  --scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform

gcloud auth application-default set-quota-project <YOUR_PROJECT_ID>
```

Confirm headless credential readiness at any time without opening a browser:

```bash
share-with auth status
```

:::note[Google Workspace organization policy]
If your Google Workspace organization restricts `gcloud`'s default OAuth client from requesting the `drive` scope, pass a Desktop OAuth client JSON from a personal or sandbox GCP project either via `gcloud auth application-default login --client-id-file=~/secrets/client_secret.json --scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform` or via `share-with auth login --credentials ~/secrets/client_secret.json`.
:::

## 3. Preview with `--dry-run` (Zero Credentials Required)

Test URL extraction, recipient shorthand expansion, deduplication, and Drive API v3 role translation (`editor` -> `writer`) completely offline:

```bash
share-with "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" \
  @alice, team-leads@google.com, doc-reviewers@ \
  --role commenter \
  --dry-run
```

## 4. Share a Document Silently

Grant `reader` access (the default role) using either a full URL (including `?resourcekey=...` links) or a raw Drive file ID. Domainless handles (`@user`, `user@`, or `user`) automatically expand to `@google.com` (configurable via `--default-domain`):

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

Notice how `bob@google.com` is reported as `already_shared (editor)`: `share-with` inspects existing permissions before calling `Permissions.Create` and skips collaborators who already hold equal or higher access so nobody is accidentally downgraded.

## Next Steps

- **[Agent Plugin & Skill Guide](../agent-plugin/)**: Install the plugin in Claude Code, Antigravity, Codex, Gemini CLI, or OpenCode and learn how to prompt your agent.
- **[Silent-Only Guarantee](../silent-guarantee/)**: Understand the three-layer safeguard preventing notification emails.
- **[CLI & JSON Reference](../cli-reference/)**: Explore all subcommands, flags, environment variables, and JSON schemas.
