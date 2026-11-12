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
- **`site/`**: Astro + Starlight documentation and landing site (`@astrojs/starlight`, `starlight-llms-txt`, `@material/web` gBreeze stylesheets, self-hosted Google Sans Flex & Google Sans Code fonts) deployed to GitHub Pages via `.github/workflows/site.yml`.

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

# Format, vet, lint (golangci-lint v2), and validate Agent Plugin & portable URL hygiene
make fmt
make lint
make validate

# Apply or verify Apache-2.0 'Copyright 2026 Google LLC' headers via addlicense
make license
make license-check

# Verify zero Go stdlib/module CVEs and zero npm vulnerabilities
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
cd site && npm audit

# Typecheck and build the Astro + Starlight site (ASTRO_TELEMETRY_DISABLED=1)
make site-build
```

---

## 5. Editorial Standards

Documentation in `README.md`, `site/`, and `plugins/` follows strict house style:
- **README Quality Gate**: Audit `README.md` edits against [`make-readme`](https://github.com/ghchinoy/agent-skills/blob/main/plugins/repo-authoring/skills/make-readme/SKILL.md) from [`ghchinoy/agent-skills`](https://github.com/ghchinoy/agent-skills) (Mark Allen 8-dimension rubric), maintain a score of **34/40 or higher (Excellent band)**, and preserve the `## Disclaimer` `[!CAUTION]` callout above `## License`.
- **Plugin & Skill Assessment Conformance**: Keep `plugins/share-with/` and `.claude-plugin/marketplace.json` conformant with [`skills-to-plugins`](https://github.com/ghchinoy/agent-skills/blob/main/plugins/agent-plugin-authoring/skills/skills-to-plugins/SKILL.md) and evaluate skill/validator updates with `skill-assessment` (`skill-readiness-audit` and `skill-ablation-benchmark`, never `SkillForge` or `sss-1` heading boilerplate). Any validator script (`validate*`, `audit*`, `check*`, `lint*`, `verify*`, `assess*`) must support `--json` (`VAL-JSON`), `--help` (`VAL-HELP`), and non-zero exit codes on failure (`VAL-EXIT`).
- **Portable Skill References**: Reference external agent skills and plugins by their canonical source repository URL (such as `https://github.com/ghchinoy/agent-skills`) rather than machine-local filesystem paths.
- **Synthetic-Only Examples & Test Fixtures**: Never commit real corporate handles, internal group aliases, or live Google Doc/Colab file IDs or `resourcekey` tokens into `README.md`, `site/`, `plugins/`, Cobra `Example` strings, GoDoc comments, or `*_test.go` fixtures. Always use synthetic placeholders (`@alice`, `bob@`, `doc-reviewers@`, `team-leads@google.com`, `1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms`, `0-aBcDeFgHiJkLmNoPqRsTuVw`).
- **Docs & Skill Parity**: Whenever a CLI subcommand, flag, or JSON field is added or updated, update `README.md`, [`plugins/share-with/skills/share-with/SKILL.md`](plugins/share-with/skills/share-with/SKILL.md), and `site/src/content/docs/{cli-reference,agent-plugin}/index.md` in the same change.
- Zero em dashes in prose; use commas, colons, or parentheses.
- Active voice with concrete actors.
- Copy-pasteable commands with language-hinted fenced code blocks.

---

## 6. Issue Tracking & Repository Hygiene

- **Beads (`bd`) Parity**: After creating or updating issues in `bd`, run `bd export -o .beads/issues.jsonl` so Git-tracked JSONL matches `.beads/dolt`. If `.beads/dolt` reports Dolt lock contention, follow [`bd-dolt-troubleshooter`](https://github.com/ghchinoy/agent-skills/blob/main/plugins/beads-workflow/skills/bd-dolt-troubleshooter/SKILL.md) to isolate and stop only the `dolt sql-server` PID bound to this repository's `.beads/` directory rather than killing servers belonging to other projects.
- **Root-Anchored Binary & Site SVG Ignore Rules**: Keep `/share-with` root-anchored in `.gitignore` so `plugins/share-with/` and `plugins/share-with/skills/share-with/SKILL.md` remain tracked by Git, and keep `!site/public/**/*.svg` and `!site/src/assets/**/*.svg` negated below `*.svg` so `favicon.svg` and `logo.svg` are tracked for GitHub Pages builds.
- **Apache-2.0 License Headers (`make license`)**: Every `.go`, `.sh`, `.yml`, and `site/` configuration file must carry the `Copyright 2026 Google LLC` Apache-2.0 header (`addlicense -c "Google LLC" -l apache -y 2026`). `TestAgentPluginAndPortableURLHygiene/go_and_script_apache_headers_present` in `repo_hygiene_test.go` enforces this on every `go test ./...` run.
- **Static Analysis & Credential Type Allowlisting (`.golangci.yml` v2)**: Keep `golangci-lint run ./...` (`errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`) at 0 issues, keep `go.mod` pinned to the latest stable Go patch floor (`go 1.26.11`+) so `govulncheck ./...` reports 0 stdlib vulnerabilities, and always load Google JSON credentials via `google.CredentialsFromJSONWithTypeAndParams` with an explicit type allowlist (`ServiceAccount`, `AuthorizedUser`, `ExternalAccount`) rather than deprecated `google.CredentialsFromJSONWithParams` (`SA1019`).
- **Google Docs Hyperlink Extraction**: Plain-text exports of Google Docs strip table-cell hyperlinks and smart chips; use Drive API `Files.Export` with `text/html` when implementing `links` and `audit` commands (`share-with-96s`, `share-with-aqc`).

---

## 7. Release Process (`release-please` + GoReleaser)

Cross-platform binary releases and Agent Plugin version bumps are automated via [`.github/workflows/release.yml`](.github/workflows/release.yml), [`release-please-config.json`](release-please-config.json), and [`.goreleaser.yaml`](.goreleaser.yaml):

1. **Conventional Commits on `main`**: Prefix commits with `feat:`, `fix:`, `perf:`, `docs:`, `test:`, `ci:`, or `chore:`. User-facing `feat:` and `fix:` commits trigger or update a `release-please` Pull Request (requires `can_approve_pull_request_reviews: true` in GitHub Actions workflow permissions while keeping `default_workflow_permissions: read`).
2. **Go-Compatible Semver Tags**: Keep `"include-component-in-tag": false` in [`release-please-config.json`](release-please-config.json) so root module tags are `vX.Y.Z` (required by `go install github.com/ghchinoy/share-with@latest`) rather than `share-with-vX.Y.Z`.
3. **Synchronized Version Files**: The Release PR updates [`.release-please-manifest.json`](.release-please-manifest.json), [`CHANGELOG.md`](CHANGELOG.md), [`cmd/version.go`](cmd/version.go) (`// x-release-please-version`), and [`plugins/share-with/plugin.json`](plugins/share-with/plugin.json) in lockstep (`repo_hygiene_test.go` verifies these versions never drift). After merging a Release PR on GitHub, run `git pull --ff-only origin main` locally before creating new commits.
4. **Automated Cross-Platform Archives**: Merging the Release PR creates the `vX.Y.Z` GitHub Release and immediately runs the `goreleaser` job (`release.mode: keep-existing`) to attach static (`CGO_ENABLED=0`, `-trimpath`) archives for `darwin`, `linux`, and `windows` (`amd64` and `arm64`) along with `checksums.txt` (SHA-256).
5. **Local Dry-Run Verification**: Before changing [`.goreleaser.yaml`](.goreleaser.yaml) or release flags, run:
   ```bash
   goreleaser check
   make snapshot
   ```

---

## 8. Open Roadmap & Implementation Notes (`bd ready`)

Run `bd ready` to inspect unblocked issues. Key domain invariants for the remaining CLI features:

1. **`share-with-96s` (`share-with links <doc>`)**:
   - Export Google Docs as `text/html` via Drive API `Files.Export` (preserving `<a href>` links inside table cells and smart chips) while propagating `X-Goog-Drive-Resource-Keys` when `?resourcekey=...` is present.
   - Unwrap `https://www.google.com/url?q=...` redirect wrappers and classify deduplicated targets into `google_drive` (with `file_id` and `resource_key`), `colab`, `golink` (`http://go/...`, `https://goto.google.com/...`), `cl`, `mailto`, and `web`.
   - Register under `GroupID: "inspection"` with `--drive-only`, `--limit`, and `--json`, tested offline via `http.RoundTripper` mocks.
2. **`share-with-aqc` (`share-with audit <doc> [emails...]`, depends on `share-with-96s`) & `share-with-ylj` (`go/` shortlinks)**:
   - Audit explicit user/group ACLs and domain permissions across a parent packet Doc and its linked Drive/Docs/Sheets/Slides/Colab items (`--recursive-links`) against a target roster (supporting `@user`, `user@`, and `user` shorthands).
   - Classify items into `covered`, `missing_restricted` (Tier 1: `capabilities.canShare=true` with no domain link-sharing), `missing_domain_shared` (Tier 2: `type=domain` link-shared, missing explicit ACLs), and `unshareable` (`capabilities.canShare=false` or `HTTP 403 insufficientFilePermissions` on externally owned files).
   - Flag Drive titles carrying `[deprecated]` prefixes, and separate standalone `go/` shortlinks from `go/` shortlinks paired with a direct Drive URL in the same paragraph or table cell (`share-with-ylj`).
3. **`share-with-rw3` (`grant --targets-file` & `--from-doc-links`, depends on `share-with-aqc`)**:
   - Support multi-target silent grants (`--tier restricted|all`, `--dry-run`, `--json`), skipping `canShare=false` links while preserving `SilentGuardTransport` (`sendNotificationEmail=false`) and the `roleRank` no-downgrade guard (`already_shared`).
4. **`share-with-iu4` (`--expires` / `--ttl` silent permission expiration)**:
   - Expose mutually exclusive `--expires <RFC3339>` and `--ttl <duration>` flags on root and `grant`, setting Drive v3 `Permission.ExpirationTime` (future timestamp `<= 365` days on `user`/`group` grants) while keeping `sendNotificationEmail=false` and surfacing `expiration_time` in `grant --json` and `list --json`.
5. **Skill Assessment Follow-Ups (`share-with-91k`, `share-with-mva`, `share-with-259`)**:
   - **`share-with-91k` (Modality `A0` -> `P1`)**: Bundle a zero-dependency, task-agnostic `V_skill` validator at `plugins/share-with/skills/share-with/scripts/validate_share_with_plan.py` conforming to the 5-part `VALIDATOR_CONTRACT` (`VAL-EXIT`, `VAL-JSON`, `VAL-HELP`, read-only inspection, universal rules) and reference it in `SKILL.md` (`SPEC-ORP` and `SPEC-PTR` clean).
   - **`share-with-mva` (`SKILL.md` Trigger & Failure Recovery)**: Add an explicit `"Use when..."` trigger clause in `SKILL.md` frontmatter `description`, document negative scope boundaries (ownership transfers, non-Google email invites, published `/d/e/2PACX-...` web links), and include copy-pasteable CLI remediation commands for `403 ACCESS_TOKEN_SCOPE_INSUFFICIENT`, `403 SERVICE_DISABLED` quota project, and `404` missing `resourcekey` while staying `<= 200` lines (`MOD-PARADOX` safe).
   - **`share-with-259` (`1x3` Causal Ablation Triad, depends on `share-with-91k`)**: Build an offline-verified Evaluation Triad (`--fixture`, `--validator` `V_skill`, `--oracle` `O_eval`) verified via `run_skill_ablation.py --check-triad` (`distinct_and_leak_free`, `negative_control_baseline`, `read_only_execution`, `behavioral_stub_probe`).
