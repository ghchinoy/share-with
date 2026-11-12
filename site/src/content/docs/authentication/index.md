---
title: Authentication & Config
description: Headless credential chain, ADC quota-project auto-detection, Service Account impersonation, and XDG YAML configuration.
---

`share-with` never opens a browser unexpectedly during `grant` or `list` execution, making it safe inside non-interactive SSH sessions, CI pipelines, and autonomous coding agent sandboxes.

## Headless Credential Resolution Order

Credentials are resolved in the following strict priority order:

### 1. Bearer Token (`SHARE_WITH_ACCESS_TOKEN`)

Pass a short-lived OAuth2 token via environment variable rather than a CLI flag so secrets never leak into shell history or `ps` output:

```bash
SHARE_WITH_ACCESS_TOKEN="$(gcloud auth print-access-token)" \
  share-with "https://docs.google.com/document/d/<FILE_ID>/edit" alice@example.com
```

### 2. Explicit Credentials File (`--credentials` / `SHARE_WITH_CREDENTIALS` / `GOOGLE_APPLICATION_CREDENTIALS`)

Supports three Google JSON formats:

- **Service Account Key (`type: "service_account"`)**: Supports optional `--impersonate user@domain.com` (or `SHARE_WITH_IMPERSONATE`) for Google Workspace Domain-Wide Delegation.
- **Authorized User JSON (`type: "authorized_user"`)**: Uses refresh token credentials directly and extracts `quota_project_id` if present.
- **Desktop OAuth Client (`installed` / `web`)**: Paired with a cached token at `~/.config/share-with/token.json` created once via `share-with auth login`:
  ```bash
  share-with auth login --credentials ~/secrets/client_secret.json
  share-with auth status
  ```

### 3. Application Default Credentials (ADC)

Uses `~/.config/gcloud/application_default_credentials.json` authenticated with the Google Drive scope:

```bash
gcloud services enable drive.googleapis.com --project=<YOUR_PROJECT_ID>
gcloud auth application-default login \
  --scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform
gcloud auth application-default set-quota-project <YOUR_PROJECT_ID>
```

#### Automatic ADC Quota Project Detection

In Go's `golang.org/x/oauth2/google` package, `Credentials.ProjectID` is populated only for Service Account JSON (`"project_id"`) and remains empty for `authorized_user` ADC files (`"quota_project_id"`). `share-with` handles this automatically by resolving the `X-Goog-User-Project` header in order:

1. `--quota-project` flag
2. `SHARE_WITH_QUOTA_PROJECT`, `GOOGLE_CLOUD_QUOTA_PROJECT`, `GOOGLE_CLOUD_PROJECT`, `GCLOUD_PROJECT`, or `CLOUDSDK_CORE_PROJECT`
3. `quota_project` in `~/.config/share-with/config.yaml`
4. `"quota_project_id"` parsed directly from the ADC JSON payload
5. Active `gcloud` configuration (`~/.config/gcloud/configurations/config_<active>`)

---

## Persistent XDG Configuration (`config.yaml`)

Optional defaults can be stored in `$XDG_CONFIG_HOME/share-with/config.yaml` (defaulting to `~/.config/share-with/config.yaml`). Settings follow **`Flag > Environment > Config File > Default`** precedence:

```yaml
default_role: reader
default_domain: google.com
credentials_file: ~/.config/share-with/service-account.json
token_file: ~/.config/share-with/token.json
quota_project: my-gcp-project
impersonate: workspace-owner@example.com
```
