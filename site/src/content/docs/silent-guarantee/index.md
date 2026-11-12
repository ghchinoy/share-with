---
title: Silent-Only Guarantee
description: How share-with prevents Google Drive notification emails across CLI flags, Drive API v3 calls, and HTTP transport inspection.
---

Google Drive API v3 defaults the `sendNotificationEmail` query parameter to **`true`** whenever it is omitted on `user` and `group` permission creation requests. A single forgotten parameter in a script or agent tool call can send unwanted email blasts to dozens of reviewers or mailing lists.

`share-with` makes notification emails structurally impossible by enforcing silence at **three independent layers**.

## Layer 1: CLI Flag & Role Surface

- **Zero Notification Flags**: No `--notify`, `--send-notification-email`, `--message`, or `--email-message` flags exist anywhere on the root command or `share-with grant`. Contract tests in [`main_test.go`](https://github.com/ghchinoy/share-with/blob/main/main_test.go) fail the build if any notification flag is ever registered.
- **Ownership Transfer Rejection**: Google Drive forces `sendNotificationEmail=true` when transferring ownership (`role=owner` or `role=organizer`). `share-with` rejects `owner` and `organizer` roles up front before making any network call.

## Layer 2: Explicit Drive API v3 Call & No-Downgrade Guard

In [`internal/sharer/sharer.go`](https://github.com/ghchinoy/share-with/blob/main/internal/sharer/sharer.go):

1. **Unconditional Query Parameters**: Every `Permissions.Create` request explicitly chains `.SendNotificationEmail(false)` and `.SupportsAllDrives(true)`.
2. **Resource Key Propagation**: When a target URL includes `?resourcekey=...`, `share-with` attaches `X-Goog-Drive-Resource-Keys: <fileID>/<resourceKey>` to `Files.Get`, `Permissions.List`, and `Permissions.Create`.
3. **Pre-Grant No-Downgrade Guard (`already_shared`)**: Calling `Permissions.Create` with `role=reader` on a user who already holds `writer` (`editor`) or `commenter` access can downgrade their role in Google Drive. Before creating permissions, `share-with` queries existing ACLs and ranks roles:
   - `owner` (5) > `organizer` (4) > `fileOrganizer` / `writer` (3) > `commenter` (2) > `reader` (1)
   - Any recipient whose existing rank is **greater than or equal to** the requested role is skipped without calling `Permissions.Create` and recorded as `status: "already_shared"` with their `existing_role`.
   - Recipients holding a **lower** role (for example `reader` when `--role editor` is requested) are silently upgraded with `sendNotificationEmail=false`.

## Layer 3: HTTP Wire Guard (`SilentGuardTransport`)

Even if future code refactored the Drive SDK call, [`internal/sharer/transport.go`](https://github.com/ghchinoy/share-with/blob/main/internal/sharer/transport.go) wraps the underlying `http.RoundTripper` in `SilentGuardTransport`.

Before any HTTP request leaves the process, `SilentGuardTransport.RoundTrip` inspects every `POST .../permissions` request URL and aborts locally in memory if:

- `sendNotificationEmail` is missing or not equal to `"false"`,
- `transferOwnership` is `"true"`, or
- `emailMessage` is non-empty.

## Non-Google Account Behavior

When an email address is not associated with a Google Account or Google Group, Google Drive API v3 returns `HTTP 400 Bad Request` (`invalidSharingRequest`) if `sendNotificationEmail=false`, because Google requires sending an email invitation to non-Google accounts.

:::caution[Zero Fallback Policy]
`share-with` **never** retries failed recipients with `sendNotificationEmail=true`. Instead, it records `status: "failed"` for that address with an actionable remediation hint and exits with code `1` while completing silent grants for all valid Google-backed recipients in the batch.
:::
