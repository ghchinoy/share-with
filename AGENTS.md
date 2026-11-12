# AGENTS.md: share-with Developer & Agent Guide

This guide documents the architecture, silent-sharing safety invariants, Agent-Aware CLI conventions, and verification workflows for contributors and AI agents working on `share-with`.

---

## 1. Architecture Overview

`share-with` grants Google Docs and Google Drive permissions (`reader`, `commenter`, or `editor`) to email recipients without ever sending notification emails:

- **`main.go`**: Entrypoint invoking `cmd.Execute()`.
- **`cmd/`**: Cobra command routing and dual-mode presentation.
  - `root.go`: Root command supporting direct `share-with <doc-url> <email...>` invocation, persistent flags (`--json`, `--credentials`, `--token-file`, `--quota-project`, `--impersonate`, `--timeout`), and command groups.
  - `grant.go`: Explicit `grant` (`share`) subcommand sharing flags and execution flow with root.
  - `list.go`: Scoped `list` subcommand (`--limit 25`, `--role`) for verifying existing file permissions.
  - `auth.go`: Headless `auth status` inspection and interactive `auth login` loopback helper.
  - `output.go`: Deterministic JSON encoder and alignment-safe semantic table renderer.
  - `errors.go`: `CLIError` wrapper and proactive remediation hints.
  - `help.go`: Thread-safe (`sync.Once`) semantic color usage template.
  - `version.go`: Release-synced `Version` (`x-release-please-version`), `Commit`, and `Date` metadata injected via `-ldflags`.
- **`internal/docurl/`**: Extracts Drive file IDs and optional `?resourcekey=...` tokens from Google Docs, Sheets, Slides, Forms, Drawings, Colab notebooks (`colab.research.google.com`), Drive file/folder URLs, and raw IDs; rejects published `/d/e/2PACX-...` web links; normalizes and deduplicates recipient emails and domainless corporate shorthands (`@user`, `user@`, `user` -> `@google.com` by default).
- **`internal/config/`**: XDG Base Directory configuration loader (`$XDG_CONFIG_HOME/share-with/config.yaml`) enforcing `Flag > Environment > Config File > Default` priority.
- **`internal/sharer/`**:
  - `transport.go`: `SilentGuardTransport` (`http.RoundTripper`) blocking any permission creation request unless `sendNotificationEmail=false` is set on the wire.
  - `auth.go`: Headless-first credential resolution (`SHARE_WITH_ACCESS_TOKEN`, service account JSON, authorized user JSON, cached desktop OAuth token, and ADC with automatic `quota_project_id` JSON extraction and active `gcloud` configuration project fallback).
  - `sharer.go`: Role translation (`editor` -> Drive v3 `writer`), pre-grant no-downgrade guard (`already_shared` when a recipient holds an equal or higher role), `X-Goog-Drive-Resource-Keys` propagation, and batch permission grants with Shared Drive support (`supportsAllDrives=true`).
- **`internal/style/`**: Semantic Lipgloss palette (`Accent`, `Command`, `Pass`, `Warn`, `Fail`, `Muted`, `ID`) with automatic degradation for `NO_COLOR`, `SHARE_WITH_NO_TUI`, `--json`, and non-TTY pipes.
- **`plugins/share-with/` & `.claude-plugin/marketplace.json`**: [Agent Plugins v1.0.0](https://agent-plugins.org) package (`plugin.json` and `skills/share-with/SKILL.md`) and marketplace discovery index validated via `scripts/validate-plugin.sh`.

---

## 2. Silent-Sharing Safety Contract

Google Drive API v3 defaults `sendNotificationEmail` to `true` when omitted on `user` and `group` permissions. `share-with` enforces silence at three layers:

1. **CLI Surface**: No `--notify`, `--send-notification-email`, or `--message` flags exist. Ownership transfer roles (`owner`, `organizer`) are rejected because Google Drive forces notifications for ownership changes.
2. **API Call Site**: `Client.Share` unconditionally calls `.SendNotificationEmail(false)` and `.SupportsAllDrives(true)` on every `Permissions.Create` request.
3. **HTTP Wire Guard**: `SilentGuardTransport.RoundTrip` inspects every `POST .../permissions` request and aborts the call before transmission if `sendNotificationEmail != "false"`, if `transferOwnership=true`, or if `emailMessage` is present.

If a recipient address is not associated with a Google Account, Google Drive returns HTTP 400 (`invalidSharingRequest`) when `sendNotificationEmail=false`. `share-with` never retries with notifications enabled; instead, it records `status: "failed"` with an explicit hint and exits non-zero.

---

## 3. Agent-Aware CLI Conventions

Aligned with [`agent-aware-cli`](https://github.com/ghchinoy/agent-skills/blob/main/plugins/agent-aware-cli/skills/agent-aware-cli/SKILL.md) and [`go-readability`](https://github.com/ghchinoy/agent-skills/blob/main/plugins/go-readability/skills/go-readability/SKILL.md) in [`ghchinoy/agent-skills`](https://github.com/ghchinoy/agent-skills):

1. **Logical Grouping**: Every subcommand belongs to a Cobra `GroupID` (`sharing`, `inspection`, `configuration`).
2. **Tri-Part Documentation**: Populate `Short`, `Long`, and `Example` on every command.
3. **Structured JSON**: Support `--json` across `grant`, `list`, `auth status`, `auth login`, and error outputs.
4. **Mutative Safety**: Preserve `--dry-run` on `share-with` and `share-with grant` so agents can verify URL parsing, email deduplication, and Drive role mapping (`editor` -> `writer`) offline.
5. **Alignment Safety**: Pad plain strings via `style.PadRight` before wrapping text in Lipgloss color styles.

---

## 4. Build & Verification Commands

```bash
# Build binary into bin/share-with
make build

# Run unit, CLI contract, and plugin/hygiene tests with race detector
make test

# Format, vet Go source files, and validate Agent Plugin & portable URL hygiene
make fmt
make lint
make validate
```

---

## 5. Editorial Standards

Documentation in `README.md` and `docs/` follows strict house style:
- **README Quality Gate**: Audit `README.md` edits against [`make-readme`](https://github.com/ghchinoy/agent-skills/blob/main/plugins/repo-authoring/skills/make-readme/SKILL.md) from [`ghchinoy/agent-skills`](https://github.com/ghchinoy/agent-skills) (Mark Allen 8-dimension rubric) and maintain a score of **34/40 or higher (Excellent band)**.
- **Plugin Conformance**: Keep `plugins/share-with/` and `.claude-plugin/marketplace.json` conformant with [`skills-to-plugins`](https://github.com/ghchinoy/agent-skills/blob/main/plugins/agent-plugin-authoring/skills/skills-to-plugins/SKILL.md) (`make validate` and `go test ./...` enforce 0 errors).
- **Portable Skill References**: Reference external agent skills and plugins by their canonical source repository URL (such as `https://github.com/ghchinoy/agent-skills`) rather than machine-local filesystem paths.
- Zero em dashes in prose; use commas, colons, or parentheses.
- Active voice with concrete actors.
- Copy-pasteable commands with language-hinted fenced code blocks.

---

## 6. Issue Tracking & Repository Hygiene

- **Beads (`bd`) Parity**: After creating or updating issues in `bd`, run `bd export -o .beads/issues.jsonl` so Git-tracked JSONL matches `.beads/dolt`. If `.beads/dolt` reports Dolt lock contention, follow [`bd-dolt-troubleshooter`](https://github.com/ghchinoy/agent-skills/blob/main/plugins/beads-workflow/skills/bd-dolt-troubleshooter/SKILL.md) to isolate and stop only the `dolt sql-server` PID bound to this repository's `.beads/` directory rather than killing servers belonging to other projects.
- **Root-Anchored Binary Ignore**: Keep `/share-with` root-anchored in `.gitignore` so `plugins/share-with/` and `plugins/share-with/skills/share-with/SKILL.md` remain tracked by Git.
- **Google Docs Hyperlink Extraction**: Plain-text exports of Google Docs strip table-cell hyperlinks and smart chips; use Drive API `Files.Export` with `text/html` when implementing `links` and `audit` commands (`share-with-96s`, `share-with-aqc`).

---

## 7. Release Process (`release-please` + GoReleaser)

Cross-platform binary releases and Agent Plugin version bumps are automated via [`.github/workflows/release.yml`](.github/workflows/release.yml), [`release-please-config.json`](release-please-config.json), and [`.goreleaser.yaml`](.goreleaser.yaml):

1. **Conventional Commits on `main`**: Prefix commits with `feat:`, `fix:`, `perf:`, `docs:`, `test:`, `ci:`, or `chore:`. User-facing `feat:` and `fix:` commits trigger or update a `release-please` Pull Request.
2. **Synchronized Version Files**: The Release PR updates [`.release-please-manifest.json`](.release-please-manifest.json), `CHANGELOG.md`, [`cmd/version.go`](cmd/version.go) (`// x-release-please-version`), and [`plugins/share-with/plugin.json`](plugins/share-with/plugin.json) in lockstep (`repo_hygiene_test.go` verifies these versions never drift).
3. **Automated Cross-Platform Archives**: Merging the Release PR creates the `vX.Y.Z` GitHub Release and immediately runs the `goreleaser` job (`release.mode: keep-existing`) to attach static (`CGO_ENABLED=0`, `-trimpath`) archives for `darwin`, `linux`, and `windows` (`amd64` and `arm64`) along with `checksums.txt` (SHA-256).
4. **Local Dry-Run Verification**: Before changing [`.goreleaser.yaml`](.goreleaser.yaml) or release flags, run:
   ```bash
   goreleaser check
   make snapshot
   ```
