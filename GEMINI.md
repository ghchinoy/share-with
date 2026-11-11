# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Quick Reference

```bash
bd ready                              # Find available work
bd show <id>                          # View issue details
bd update <id> --status in_progress   # Claim work
bd close <id>                         # Complete work
bd sync                               # Sync with git
```

## Project Overview & Core Invariant

`share-with` is an Agent-Aware Go CLI tool that shares a Google Doc or Google Drive resource with a list of email addresses using Google Drive API v3 (`permissions.create`).

**Non-Negotiable Invariant:** Every permission grant MUST be silent (`sendNotificationEmail=false`).
1. Never add a `--notify`, `--send-notification-email`, or `--message` flag.
2. Always pass `.SendNotificationEmail(false)` and `.SupportsAllDrives(true)` on `Permissions.Create`.
3. Keep `sharer.SilentGuardTransport` wrapped around the Drive HTTP `RoundTripper` so any outgoing `POST .../permissions` request missing `sendNotificationEmail=false` (or setting `transferOwnership=true` or `emailMessage`) fails before leaving the process.
4. Never fall back to sending an email invitation when a recipient lacks a Google Account; surface an actionable error hint instead.

## Coding Standards and Patterns

- **Directory Layout**:
  - `main.go`: Concise user-facing package doc and entrypoint delegating to `cmd.Execute()`.
  - `cmd/`: Cobra command definitions (`root.go`, `grant.go`, `list.go`, `auth.go`, `output.go`, `errors.go`, `help.go`).
  - `internal/docurl/`: URL/file-ID extraction and RFC 5322 email list parsing and deduplication.
  - `internal/config/`: XDG Base Directory YAML config (`~/.config/share-with/config.yaml`) with `Flag > Env > Config > Default` precedence.
  - `internal/sharer/`: Role mapping (`reader` -> `reader`, `commenter` -> `commenter`, `editor` -> `writer`), headless credential chain, `SilentGuardTransport`, and Drive API v3 client logic.
  - `internal/style/`: Semantic Lipgloss color tokens (`Accent`, `Command`, `Pass`, `Warn`, `Fail`, `Muted`, `ID`) and rune-aware padding prior to ANSI styling.
- **Agent-Aware CLI Mandates**:
  - Group subcommands using Cobra `GroupID` (`sharing`, `inspection`, `configuration`).
  - Populate `Short`, `Long`, and `Example` on every command (do not manually list child subcommands in a parent command's `Long` field).
  - Support `--json` across all commands and `--dry-run` on mutative commands (`share-with` / `share-with grant`).
  - Respect `NO_COLOR` and `SHARE_WITH_NO_TUI` for plain-text output degradation.
  - Reserve `ctx` exclusively for `context.Context` and inline `context.WithTimeout` directly inside `RunE`.
- **Testing**:
  - Run `make test` (`go test -v -race ./...`) and `make lint` before completing changes.
  - Avoid binding `127.0.0.1` in unit tests; inject in-memory `http.RoundTripper` implementations into `drive.NewService` so tests pass inside restricted sandboxes.

## Landing the Plane (Session Completion)

When ending a work session on a tracked repository:
1. File issues in `bd` for any follow-up work.
2. Run quality gates (`make fmt`, `make vet`, `make test`, `make build`).
3. Update `bd` issue statuses and sync when applicable.
