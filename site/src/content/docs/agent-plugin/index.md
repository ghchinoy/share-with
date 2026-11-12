---
title: Agent Plugin & Skill
description: Install and use the share-with Agent Plugins v1.0.0 package and SKILL.md across Antigravity, Claude Code, Codex, Gemini CLI, OpenCode, and npx skills.
---

`share-with` is designed from the ground up as an **Agent-Aware CLI** paired with a spec-conformant [Agent Plugins v1.0.0](https://agent-plugins.org) package:

- **Plugin Manifest**: [`plugins/share-with/plugin.json`](https://github.com/ghchinoy/share-with/blob/main/plugins/share-with/plugin.json)
- **Agent Skill Instructions**: [`plugins/share-with/skills/share-with/SKILL.md`](https://github.com/ghchinoy/share-with/blob/main/plugins/share-with/skills/share-with/SKILL.md)
- **Marketplace Index**: [`.claude-plugin/marketplace.json`](https://github.com/ghchinoy/share-with/blob/main/.claude-plugin/marketplace.json)

While the `share-with` Go binary enforces silent-only safety on the wire (`SilentGuardTransport`), the bundled `SKILL.md` gives AI coding assistants procedural instructions on **when** and **how** to check credentials, run offline dry-runs, parse structured `--json` outputs, and verify permissions without polluting their context window.

---

## Prerequisites

1. Install the `share-with` CLI binary on your `PATH` (via [GitHub Releases](https://github.com/ghchinoy/share-with/releases) or `go install github.com/ghchinoy/share-with@latest`).
2. Authenticate once so headless Drive API v3 calls succeed without interactive browser prompts (see [Getting Started](../getting-started/#2-authenticate-once-with-google-drive-scope)):
   ```bash
   share-with auth status --json
   ```

---

## Install the Plugin in Your Agent Harness

Choose the installation command matching your coding assistant:

### Antigravity CLI & App

```bash
agy plugin install https://github.com/ghchinoy/share-with
```

In the **Antigravity App** or **Antigravity for IDEs**, you can also run `/plugin` and install `https://github.com/ghchinoy/share-with`.

### Claude Code & Claude Cowork

Register the repository marketplace and install the `share-with` plugin:

```bash
claude plugin marketplace add ghchinoy/share-with
claude plugin install share-with@share-with
```

### Codex CLI

```bash
codex plugin marketplace add ghchinoy/share-with
codex plugin add share-with@share-with
```

### Open Agent Skills CLI (`npx skills`)

`npx skills` reads `.claude-plugin/marketplace.json` directly and installs `./plugins/share-with/skills/share-with` into your active agent skill directory:

```bash
npx skills add ghchinoy/share-with
```

### Gemini CLI

From a local clone of `share-with`, install or symlink the skill path directly:

```bash
git clone https://github.com/ghchinoy/share-with.git
gemini skills install --path share-with/plugins/share-with/skills/share-with
```

### OpenCode

Point `~/.config/opencode/opencode.jsonc` at the cloned `plugins/` directory for live reload:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "skills": [
    "~/src/share-with/plugins"
  ]
}
```

---

## How to Use the Skill with Your Agent

Once the plugin or skill is installed, you do **not** need to memorize flags or construct Drive URLs manually. Your agent activates the `share-with` skill automatically whenever your prompt mentions sharing Google Docs, Sheets, Slides, Colab notebooks, or Drive files silently.

### 1. Share a Document Using Corporate Shorthands

Ask your agent in natural language:

```text
Silently share https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit?resourcekey=0-abc123
with @alice, team-leads@google.com, doc-reviewers@, and bob@ as reader.
```

Your agent will:
1. Validate URL parsing, `?resourcekey=...` extraction, and `@google.com` handle expansion with `--dry-run --json`.
2. Execute `share-with grant ... --role reader --json` with `sendNotificationEmail=false`.
3. Confirm which recipients were newly granted access (`shared`) versus skipped to prevent downgrades (`already_shared`).

### 2. Upgrade Collaborators to `commenter` or `editor`

```text
Grant commenter access on Drive file 1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms
to @alice and @carol without sending notification emails, then verify the ACL list.
```

If `@carol` is already an `editor` (`writer` in Drive API v3), `share-with` preserves her `editor` role (`already_shared`) while upgrading `@alice` from `reader` to `commenter` silently.

### 3. Preview Changes Offline (`--dry-run`)

```text
Do a dry-run of sharing all recipients in reviewers.txt on
https://colab.research.google.com/drive/1AbCdEfGhIjKlMnOpQrStUvWxYz012345
as editor and show me the resolved email list before making any API calls.
```

### 4. Audit & Remediate Multi-Document Review Packets

```text
Read my packet doc, check every linked Google Doc and Sheet against the target
reviewer roster (@alice, doc-reviewers@, team-leads@google.com), and use share-with
to silently grant reader access on every owned doc that is missing explicit access.
```

Because `share-with` outputs deterministic JSON on both `list --json` and `grant --json` and enforces the no-downgrade guard, an agent can safely loop across dozens of supporting packet documents with zero notification spam and zero role downgrades.

---

## The Four-Step Protocol Inside `SKILL.md`

When an agent loads [`SKILL.md`](https://github.com/ghchinoy/share-with/blob/main/plugins/share-with/skills/share-with/SKILL.md), it follows a deterministic four-step sequence:

### Step 1: Dry-Run First

Validate target URL/ID extraction, recipient deduplication, and role mapping before touching the network:

```bash
share-with grant "https://docs.google.com/document/d/<FILE_ID>/edit" \
  --emails "@alice, team-leads@google.com, doc-reviewers@, bob@" \
  --role reader \
  --dry-run --json
```

### Step 2: Check Headless Auth Readiness (If Needed)

Confirm credentials without risking an interactive browser hang:

```bash
share-with auth status --json
```

### Step 3: Execute the Silent Grant

```bash
share-with grant "https://docs.google.com/document/d/<FILE_ID>/edit" \
  --emails "@alice, team-leads@google.com, doc-reviewers@, bob@" \
  --role reader \
  --json
```

### Step 4: Verify Post-Grant Permissions

```bash
share-with list "https://docs.google.com/document/d/<FILE_ID>/edit" --limit 25 --json
```

---

## Headless CI & Subagent Delegation

When running coding agents inside ephemeral containers or CI runners without local `gcloud` user credentials, inject a short-lived OAuth2 access token via environment variable so secrets never appear in CLI arguments or process tables:

```bash
export SHARE_WITH_ACCESS_TOKEN="$(gcloud auth print-access-token)"
share-with grant "<DOC_ID>" --emails "alice@example.com" --role reader --json
```

For Google Workspace service accounts with Domain-Wide Delegation, configure `--credentials` and `--impersonate` via environment variables:

```bash
export SHARE_WITH_CREDENTIALS="/run/secrets/drive-sa.json"
export SHARE_WITH_IMPERSONATE="workspace-owner@example.com"
share-with grant "<DOC_ID>" --emails-file reviewers.txt --json
```

---

## Machine-Readable Documentation (`llms.txt`)

This documentation site publishes Starlight LLM context bundles so agents and retrieval tools can ingest the complete reference in one fetch:

- [`llms.txt`](../llms.txt): Concise index of all documentation pages.
- [`llms-small.txt`](../llms-small.txt): Compact single-file documentation bundle optimized for smaller context windows.
- [`llms-full.txt`](../llms-full.txt): Complete single-file Markdown bundle of every guide and CLI reference table.
