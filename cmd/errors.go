// errors.go defines structured CLI errors with proactive, copy-pasteable remediation hints.

package cmd

import (
	"errors"
	"strings"

	"github.com/ghchinoy/share-with/internal/docurl"
	"github.com/ghchinoy/share-with/internal/sharer"
)

// CLIError pairs a user-facing error with an executable remediation hint.
type CLIError struct {
	Err           error
	Hint          string
	AlreadyOutput bool
}

// Error implements the error interface.
func (e *CLIError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

// Unwrap exposes the underlying error for errors.Is and errors.As checks.
func (e *CLIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func withHint(err error, hint string) error {
	if err == nil {
		return nil
	}
	var existing *CLIError
	if errors.As(err, &existing) && existing.Hint != "" {
		return err
	}
	return &CLIError{Err: err, Hint: hint}
}

// ClassifyError maps a runtime or CLI error into its message and a proactive,
// copy-pasteable remediation hint.
func ClassifyError(err error) (string, string) {
	return classifyCLIError(err)
}

func classifyCLIError(err error) (string, string) {
	if err == nil {
		return "", ""
	}

	var cliErr *CLIError
	if errors.As(err, &cliErr) && cliErr.Hint != "" {
		return cliErr.Error(), cliErr.Hint
	}

	lowerMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(lowerMsg, "unknown flag") && (strings.Contains(lowerMsg, "notify") || strings.Contains(lowerMsg, "notification") || strings.Contains(lowerMsg, "message")):
		return err.Error(), "share-with permanently enforces silent sharing (sendNotificationEmail=false) and never accepts notification flags."

	case errors.Is(err, docurl.ErrPublishedURL):
		return err.Error(), "Pass the editable Google Docs URL containing '/d/<FILE_ID>/edit' instead of a published '/d/e/2PACX-...' web link."

	case errors.Is(err, docurl.ErrEmptyTarget):
		return err.Error(), "Provide a Google Docs URL as the first argument or via '--url https://docs.google.com/document/d/<FILE_ID>/edit'."

	case errors.Is(err, docurl.ErrInvalidTarget):
		return err.Error(), "Provide a full docs.google.com or drive.google.com URL (e.g. 'https://docs.google.com/document/d/<ID>/edit') or a raw Drive file ID."

	case errors.Is(err, docurl.ErrNoEmails):
		return err.Error(), "Provide recipient emails as arguments, via '--emails alice@example.com,bob@example.com', or via '--emails-file recipients.txt'."

	case errors.Is(err, sharer.ErrWriterRoleAlias):
		return err.Error(), "Use '--role editor' (share-with automatically maps 'editor' to Drive API's 'writer' permission)."

	case errors.Is(err, sharer.ErrOwnerRoleForbidden):
		return err.Error(), "Use '--role reader', '--role commenter', or '--role editor'. Ownership transfer forces notification emails and is disallowed."

	case errors.Is(err, sharer.ErrInvalidRole):
		return err.Error(), "Allowed roles are 'reader' (default), 'commenter', and 'editor'."

	case errors.Is(err, sharer.ErrInteractiveLoginRequired):
		return err.Error(), "Run 'share-with auth login --credentials <client_secret.json>' once in an interactive terminal, or set SHARE_WITH_ACCESS_TOKEN."

	case errors.Is(err, sharer.ErrNoCredentials):
		return err.Error(), "Authenticate headless via 'gcloud auth application-default login --scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform' or pass '--credentials <service-account.json>'."

	case strings.Contains(lowerMsg, "http 404"):
		return err.Error(), "Verify the document URL and confirm the authenticated account has access to the file (Shared Drives are supported automatically)."

	case strings.Contains(lowerMsg, "access_token_scope_insufficient") || strings.Contains(lowerMsg, "insufficient authentication scopes"):
		return err.Error(), "Standard ADC tokens omit the Drive scope. Re-authenticate with 'gcloud auth application-default login --scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform' (or if your Workspace org blocks gcloud's default client ID, pass '--client-id-file=<desktop_client.json>' or run 'share-with auth login --credentials <desktop_client.json>')."

	case strings.Contains(lowerMsg, "requires a quota project") || strings.Contains(lowerMsg, "projects/764086051850"):
		return err.Error(), "ADC user credentials require a quota project with Drive API enabled. Run 'gcloud services enable drive.googleapis.com --project=<PROJECT_ID>' and 'gcloud auth application-default set-quota-project <PROJECT_ID>' (or pass '--quota-project <PROJECT_ID>')."

	case strings.Contains(lowerMsg, "service_disabled") || strings.Contains(lowerMsg, "accessnotconfigured"):
		return err.Error(), "Enable the Google Drive API on your quota project with 'gcloud services enable drive.googleapis.com --project=<PROJECT_ID>' (or pass a project where Drive API is enabled via '--quota-project <PROJECT_ID>')."

	case strings.Contains(lowerMsg, "http 403"):
		return err.Error(), "Ensure the Google Drive API is enabled ('gcloud services enable drive.googleapis.com'), pass '--quota-project <PROJECT_ID>' if using ADC, and verify your account has Editor/Owner sharing permissions."

	case strings.Contains(lowerMsg, "http 401") || strings.Contains(lowerMsg, "invalid_grant"):
		return err.Error(), "Refresh your credentials with 'gcloud auth application-default login --scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform'."

	default:
		return err.Error(), "Run 'share-with --help' for usage examples or 'share-with auth status' to verify credentials."
	}
}
