// Package cmd defines the Cobra command hierarchy for share-with.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/ghchinoy/share-with/internal/config"
	"github.com/ghchinoy/share-with/internal/sharer"
	"github.com/ghchinoy/share-with/internal/style"
)

const defaultRequestTimeout = 30 * time.Second

// ClientFactory builds a sharer.Client from resolved configuration.
type ClientFactory func(ctx context.Context, cfg config.Resolved) (*sharer.Client, error)

// Options customizes dependencies for command execution and testing.
type Options struct {
	Getenv        func(string) string
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	ClientFactory ClientFactory
	Version       string
}

type appState struct {
	jsonOutput      bool
	credentialsFile string
	tokenFile       string
	quotaProject    string
	impersonate     string
	timeout         time.Duration

	getenv        func(string) string
	stdin         io.Reader
	clientFactory ClientFactory
}

func (a *appState) resolveConfig(cmd *cobra.Command, roleFlag string, roleFlagSet bool, domainFlag string, domainFlagSet bool) (config.Resolved, error) {
	overrides := config.FlagOverrides{
		Role:             roleFlag,
		RoleSet:          roleFlagSet,
		DefaultDomain:    domainFlag,
		DefaultDomainSet: domainFlagSet,
		CredentialsFile:  a.credentialsFile,
		CredentialsSet:   cmd.Flags().Changed("credentials"),
		TokenFile:        a.tokenFile,
		TokenFileSet:     cmd.Flags().Changed("token-file"),
		QuotaProject:     a.quotaProject,
		QuotaProjectSet:  cmd.Flags().Changed("quota-project"),
		Impersonate:      a.impersonate,
		ImpersonateSet:   cmd.Flags().Changed("impersonate"),
	}
	return config.Resolve(overrides, a.getenv)
}

func defaultClientFactory(ctx context.Context, cfg config.Resolved) (*sharer.Client, error) {
	svc, err := sharer.BuildDriveService(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return sharer.NewClient(svc), nil
}

// NewRootCmd constructs the root share-with Cobra command with default OS streams.
func NewRootCmd() *cobra.Command {
	return NewRootCmdWithOptions(Options{})
}

// NewRootCmdWithOptions constructs the root share-with Cobra command with injectable
// streams, environment lookup, and Drive client factory.
func NewRootCmdWithOptions(opts Options) *cobra.Command {
	if opts.Getenv == nil {
		opts.Getenv = os.Getenv
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.ClientFactory == nil {
		opts.ClientFactory = defaultClientFactory
	}
	if opts.Version == "" {
		opts.Version = BuildVersionString()
	}

	app := &appState{
		timeout:       defaultRequestTimeout,
		getenv:        opts.Getenv,
		stdin:         opts.Stdin,
		clientFactory: opts.ClientFactory,
	}

	var rootGrant grantFlags

	root := &cobra.Command{
		Use:   "share-with [doc-url] [email...]",
		Short: "Silently share a Google Doc or Drive file as reader, commenter, or editor",
		Long: `share-with grants Google Docs and Google Drive access to a list of email addresses
without ever sending notification emails.

Every permission mutation hardcodes sendNotificationEmail=false at both the API call
site and an HTTP transport guard. No notification or custom email message flags exist.

Supported roles (--role / -r):
  reader     Read-only viewer access (default)
  commenter  View and comment/suggest access
  editor     Full edit access (mapped to Google Drive API 'writer')

Recipient shorthands without a domain (@user, user@, or user) automatically default
to @google.com (configurable via --default-domain).`,
		Example: `  # Silently share as reader (default) using mixed @handles, user@, and full emails
  share-with "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" \
    @alice, team-leads@google.com, doc-reviewers@, bob@

  # Silently share as commenter using comma-separated --emails
  share-with --url "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" \
    --emails "@alice, doc-reviewers@, team-leads@google.com" --role commenter

  # Silently share as editor from a newline-delimited recipients file
  share-with "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" \
    --emails-file reviewers.txt --role editor

  # Preview the exact silent Drive API payload without mutating permissions
  share-with "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" \
    alice@example.com --dry-run --json`,
		Version:       opts.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && rootGrant.url == "" && len(rootGrant.emails) == 0 && rootGrant.emailsFile == "" {
				return cmd.Help()
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), app.timeout)
			defer cancel()
			return executeGrant(ctx, cmd, app, &rootGrant, args)
		},
	}

	root.SetIn(opts.Stdin)
	root.SetOut(opts.Stdout)
	root.SetErr(opts.Stderr)

	root.AddGroup(
		&cobra.Group{ID: "sharing", Title: "Sharing Commands:"},
		&cobra.Group{ID: "inspection", Title: "Inspection Commands:"},
		&cobra.Group{ID: "configuration", Title: "Authentication & Configuration Commands:"},
	)

	root.PersistentFlags().BoolVar(&app.jsonOutput, "json", false, "Emit deterministic JSON output for scripts and AI agents")
	root.PersistentFlags().StringVar(&app.credentialsFile, "credentials", "", "Path to Google service account, authorized user, or OAuth client JSON")
	root.PersistentFlags().StringVar(&app.tokenFile, "token-file", "", "Path to cached OAuth token JSON (defaults to ~/.config/share-with/token.json)")
	root.PersistentFlags().StringVar(&app.quotaProject, "quota-project", "", "Google Cloud project ID for Drive API quota billing (X-Goog-User-Project)")
	root.PersistentFlags().StringVar(&app.impersonate, "impersonate", "", "Google Workspace email to impersonate via service account delegation")
	root.PersistentFlags().DurationVar(&app.timeout, "timeout", defaultRequestTimeout, "Maximum duration for Google Drive API requests")

	bindGrantFlags(root, &rootGrant)

	root.AddCommand(
		newGrantCmd(app),
		newListCmd(app),
		newAuthCmd(app),
	)

	applySemanticUsageTemplate(root)
	return root
}

// Execute runs the root command and exits with code 1 on error, emitting
// structured JSON or semantic-colored error diagnostics with remediation hints.
func Execute() {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		renderFatalError(root, err)
		os.Exit(1)
	}
}

func renderFatalError(root *cobra.Command, err error) {
	var cliErr *CLIError
	if errors.As(err, &cliErr) && cliErr.AlreadyOutput {
		return
	}

	jsonMode, _ := root.Flags().GetBool("json")
	msg, hint := classifyCLIError(err)

	if jsonMode {
		payload := struct {
			Error string `json:"error"`
			Hint  string `json:"hint,omitempty"`
		}{
			Error: msg,
			Hint:  hint,
		}
		_ = printJSON(root.OutOrStdout(), payload)
		return
	}

	errStream := root.ErrOrStderr()
	palette := style.ForStream(errStream, false, os.Getenv)
	_, _ = fmt.Fprintf(errStream, "%s %s\n", palette.Fail("Error:"), msg)
	if hint != "" {
		_, _ = fmt.Fprintf(errStream, "%s  %s\n", palette.Warn("Hint:"), hint)
	}
}
