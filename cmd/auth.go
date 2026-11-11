// auth.go provides headless credential status checks and interactive desktop OAuth setup.

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ghchinoy/share-with/internal/sharer"
	"github.com/ghchinoy/share-with/internal/style"
)

func newAuthCmd(app *appState) *cobra.Command {
	authCmd := &cobra.Command{
		Use:     "auth",
		GroupID: "configuration",
		Short:   "Inspect headless authentication status or authorize a desktop OAuth client",
		Example: `  # Check active Google Drive credentials without browser prompts
  share-with auth status

  # Emit credential readiness as JSON for an autonomous agent
  share-with auth status --json

  # Authorize a desktop OAuth client JSON once and cache ~/.config/share-with/token.json
  share-with auth login --credentials ~/secrets/client_secret.json`,
	}

	authCmd.AddCommand(
		newAuthStatusCmd(app),
		newAuthLoginCmd(app),
	)
	return authCmd
}

func newAuthStatusCmd(app *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check active Google Drive credential source in headless mode",
		Long: `Evaluate the credential resolution chain (SHARE_WITH_ACCESS_TOKEN, --credentials,
XDG config, and Application Default Credentials) without opening a browser.`,
		Example: `  # Inspect active credentials in human-readable format
  share-with auth status

  # Inspect active credentials in JSON format
  share-with auth status --json

  # Check status for a specific service account JSON file
  share-with auth status --credentials ./service-account.json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolvedCfg, err := app.resolveConfig(cmd, "", false, "", false)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), app.timeout)
			defer cancel()

			status := sharer.InspectAuth(ctx, resolvedCfg)
			out := cmd.OutOrStdout()
			if app.jsonOutput {
				payload := struct {
					sharer.AuthStatus
					ConfigPath   string `json:"config_path"`
					ConfigLoaded bool   `json:"config_loaded"`
				}{
					AuthStatus:   status,
					ConfigPath:   resolvedCfg.ConfigPath,
					ConfigLoaded: resolvedCfg.ConfigLoaded,
				}
				if err := printJSON(out, payload); err != nil {
					return err
				}
			} else {
				palette := style.ForStream(out, false, app.getenv)
				renderAuthStatus(out, palette, status, resolvedCfg.ConfigPath, resolvedCfg.ConfigLoaded)
			}

			if !status.Ready {
				return &CLIError{
					Err:           fmt.Errorf("credentials are not ready: %s", status.Detail),
					Hint:          status.Hint,
					AlreadyOutput: true,
				}
			}
			return nil
		},
	}
}

func newAuthLoginCmd(app *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authorize a desktop OAuth client via local loopback and cache the token",
		Long: `Start a temporary loopback HTTP callback listener on 127.0.0.1, print an OAuth2
PKCE consent URL for the Google Drive scope, and save the resulting refresh token
to ~/.config/share-with/token.json (mode 0600) for subsequent headless use.`,
		Example: `  # Authorize using an explicit desktop client JSON file
  share-with auth login --credentials ~/secrets/desktop_client_credentials.json

  # Authorize and store the token in a custom path
  share-with auth login --credentials ~/secrets/client.json --token-file ~/.config/share-with/work-token.json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolvedCfg, err := app.resolveConfig(cmd, "", false, "", false)
			if err != nil {
				return err
			}
			if resolvedCfg.CredentialsFile == "" {
				return withHint(
					sharer.ErrNoCredentials,
					"Pass '--credentials <desktop_client_secret.json>' or use 'gcloud auth application-default login --scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform' instead.",
				)
			}

			loginTimeout := 5 * time.Minute
			if cmd.Flags().Changed("timeout") {
				loginTimeout = app.timeout
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), loginTimeout)
			defer cancel()

			out := cmd.OutOrStdout()
			if err := sharer.RunInteractiveLogin(ctx, resolvedCfg.CredentialsFile, resolvedCfg.TokenFile, out); err != nil {
				return withHint(err, "Ensure the JSON file is a Desktop ('installed') OAuth client with http://127.0.0.1 redirect allowed.")
			}

			if app.jsonOutput {
				return printJSON(out, map[string]any{
					"status":     "authorized",
					"token_file": resolvedCfg.TokenFile,
				})
			}

			palette := style.ForStream(out, false, app.getenv)
			_, _ = fmt.Fprintf(out, "\n%s Saved OAuth token to %s\n", palette.Pass("Authorized:"), resolvedCfg.TokenFile)
			return nil
		},
	}
}
