# Changelog

All notable changes to this project will be documented in this file.

## [0.1.0] - 2026-11-11

### Features

- Implement Agent-Aware Go CLI (`share-with`) for silent Google Docs, Sheets, Slides, Forms, Colab, and Google Drive sharing (`reader`, `commenter`, `editor`) with `sendNotificationEmail=false` enforced across CLI flags, Drive API v3 calls, and `SilentGuardTransport`.
- Support `?resourcekey=...` (`X-Goog-Drive-Resource-Keys`), raw Drive file IDs, and `@google.com` corporate handle shorthands (`@user`, `user@`, `user`).
- Add pre-grant no-downgrade permission guard (`already_shared`) preserving existing `editor` and `commenter` roles.
- Support headless credential resolution across `SHARE_WITH_ACCESS_TOKEN`, Service Accounts, OAuth2 Desktop clients, and ADC (`quota_project_id` and active `gcloud` project auto-detection).
