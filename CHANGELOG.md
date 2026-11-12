# Changelog

All notable changes to this project will be documented in this file.

## [0.2.0](https://github.com/ghchinoy/share-with/compare/v0.1.0...v0.2.0) (2026-11-12)


### Features

* implement silent-only Google Drive sharing CLI (share-with) ([1e1cf8b](https://github.com/ghchinoy/share-with/commit/1e1cf8bbdf0292243458b784ffd1a1456f1efc1d))
* **plugin:** package share-with as Agent Plugins v1.0.0 and enforce portable URLs ([b1483ea](https://github.com/ghchinoy/share-with/commit/b1483ea8c0c6e751b0098ae72e665a156b74ceef))
* **site:** add Astro + Starlight landing and documentation site ([885f2fa](https://github.com/ghchinoy/share-with/commit/885f2fa278b9fd9cfe4dc8fd393b56ef3f0207da))


### Bug Fixes

* **ci:** track site SVG assets and omit component prefix in release tags ([85cd1f0](https://github.com/ghchinoy/share-with/commit/85cd1f027118872e2c4fa44278f0801b22b8a90c))

## [0.1.0] - 2026-11-11

### Features

- Implement Agent-Aware Go CLI (`share-with`) for silent Google Docs, Sheets, Slides, Forms, Colab, and Google Drive sharing (`reader`, `commenter`, `editor`) with `sendNotificationEmail=false` enforced across CLI flags, Drive API v3 calls, and `SilentGuardTransport`.
- Support `?resourcekey=...` (`X-Goog-Drive-Resource-Keys`), raw Drive file IDs, and `@google.com` corporate handle shorthands (`@user`, `user@`, `user`).
- Add pre-grant no-downgrade permission guard (`already_shared`) preserving existing `editor` and `commenter` roles.
- Support headless credential resolution across `SHARE_WITH_ACCESS_TOKEN`, Service Accounts, OAuth2 Desktop clients, and ADC (`quota_project_id` and active `gcloud` project auto-detection).
