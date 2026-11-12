---
title: Troubleshooting
description: Solutions for common Google Drive API v3 authentication, scope, quota project, resourcekey, and permission errors.
---

Every error returned by `share-with` includes a proactive `Hint:` line in terminal mode (or a `"hint"` field when `--json` is passed).

## HTTP 403: `ACCESS_TOKEN_SCOPE_INSUFFICIENT`

**Symptom**: `Request had insufficient authentication scopes.`

**Cause**: Running `gcloud auth application-default login` without `--scopes` requests only `https://www.googleapis.com/auth/cloud-platform` and omits `https://www.googleapis.com/auth/drive`.

**Fix**:

```bash
gcloud auth application-default login \
  --scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform
```

If your Google Workspace organization blocks `gcloud`'s default OAuth client from requesting Drive scopes, supply a Desktop OAuth client JSON from a personal or sandbox GCP project:

```bash
# Option A: Pass --client-id-file to gcloud ADC
gcloud auth application-default login \
  --client-id-file=~/secrets/client_secret.json \
  --scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform

# Option B: Use share-with's built-in loopback PKCE login
share-with auth login --credentials ~/secrets/client_secret.json
```

---

## HTTP 403: `SERVICE_DISABLED` / `requires a quota project`

**Symptom**: `Your application is authenticating by using local Application Default Credentials. The drive.googleapis.com API requires a quota project, which is not set by default.`

**Fix**: Enable `drive.googleapis.com` on a GCP project and set it as your ADC quota project (or pass `--quota-project`):

```bash
gcloud services enable drive.googleapis.com --project=<YOUR_PROJECT_ID>
gcloud auth application-default set-quota-project <YOUR_PROJECT_ID>

# Or per-command:
share-with <DOC_ID> @alice --quota-project <YOUR_PROJECT_ID>
```

---

## HTTP 404: `File not found` on Older Link-Shared Items

**Symptom**: Querying a valid Drive file by bare file ID returns `HTTP 404 File not found`.

**Cause**: Older Google Drive files protected by Google's link-sharing security update require a matching resource key in `X-Goog-Drive-Resource-Keys`.

**Fix**: Pass the full URL including `?resourcekey=...` rather than the bare file ID so `share-with` extracts and forwards `<fileID>/<resourceKey>` automatically:

```bash
share-with "https://docs.google.com/document/d/<FILE_ID>/edit?resourcekey=0-abc123" @alice
```

---

## HTTP 403: `insufficientFilePermissions` on Externally Owned Files

**Symptom**: `share-with list` or `share-with grant` returns `HTTP 403` even though you can open and read the document in your browser.

**Cause**: Viewers and commenters without sharing privileges (`capabilities.canShare=false`) are not permitted by Google Drive API v3 to enumerate full permission rosters via `Permissions.List` or create new permissions via `Permissions.Create`.

**Fix**: Ask an Editor or Owner of that document to run the grant.

---

## HTTP 400: `invalidSharingRequest` (No Google Account)

**Symptom**: One recipient in a batch fails with `HTTP 400` stating there is no Google Account associated with the email address.

**Cause**: Google Drive rejects `sendNotificationEmail=false` for non-Google addresses because an email invitation is required for external non-Google sign-in.

**Fix**: In accordance with the [Silent-Only Guarantee](../silent-guarantee/), `share-with` never falls back to sending an invitation email. Share with a Google-backed address or Google Group instead.

---

## Published Web Links (`/d/e/2PACX-.../pub`)

**Symptom**: `published web URL does not contain a shareable Drive file ID`

**Cause**: "Publish to the web" URLs use opaque `2PACX-...` publication tokens rather than Drive file IDs.

**Fix**: Copy the `/document/d/<FILE_ID>/edit` URL (or raw file ID) from the Google Docs editor address bar.
