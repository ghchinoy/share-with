// grant.go implements the silent permission-granting workflow.

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ghchinoy/share-with/internal/docurl"
	"github.com/ghchinoy/share-with/internal/sharer"
	"github.com/ghchinoy/share-with/internal/style"
)

type grantFlags struct {
	url           string
	emails        []string
	emailsFile    string
	role          string
	defaultDomain string
	dryRun        bool
}

func bindGrantFlags(cmd *cobra.Command, flags *grantFlags) {
	cmd.Flags().StringVarP(&flags.url, "url", "u", "", "Google Docs or Google Drive URL (or pass as first argument)")
	cmd.Flags().StringArrayVarP(&flags.emails, "emails", "e", nil, "Comma-separated or repeatable recipient emails or @handles")
	cmd.Flags().StringVarP(&flags.emailsFile, "emails-file", "f", "", "Path to newline-delimited recipient emails file ('-' for stdin)")
	cmd.Flags().StringVarP(&flags.role, "role", "r", "reader", "Permission role to grant silently: reader, commenter, or editor")
	cmd.Flags().StringVar(&flags.defaultDomain, "default-domain", docurl.DefaultDomain, "Domain appended when a recipient omits a domain (e.g. @user, user@, or user)")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Validate inputs and preview the silent Drive API payload without making changes")
}

func newGrantCmd(app *appState) *cobra.Command {
	var flags grantFlags

	cmd := &cobra.Command{
		Use:     "grant [doc-url] [email...]",
		GroupID: "sharing",
		Aliases: []string{"share"},
		Short:   "Silently grant reader, commenter, or editor access to email recipients",
		Long: `Grant Google Docs or Google Drive access to one or more email addresses with
notifications permanently disabled (sendNotificationEmail=false).

Recipients can be supplied as positional arguments after the URL, via --emails / -e,
or from a file or stdin via --emails-file / -f. Shorthand entries without a domain
(@alice, doc-reviewers@, bob@, or alice) automatically expand using
--default-domain (google.com by default). Duplicate addresses are normalized to
lowercase and deduplicated before calling the Google Drive API.`,
		Example: `  # Grant default silent reader access to a mixed shorthand and email list
  share-with grant "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" \
    @alice, team-leads@google.com, doc-reviewers@, bob@, carol@, dave@

  # Grant silent commenter access using flags
  share-with grant --url "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" \
    --emails "@alice, team-leads@google.com, doc-reviewers@" --role commenter

  # Grant silent editor access reading emails/handles from stdin
  cat team.txt | share-with grant "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" \
    --emails-file - --role editor

  # Dry-run validation with structured JSON output
  share-with grant "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" \
    -e "@alice, bob@" --role editor --dry-run --json`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), app.timeout)
			defer cancel()
			return executeGrant(ctx, cmd, app, &flags, args)
		},
	}

	bindGrantFlags(cmd, &flags)
	return cmd
}

func executeGrant(ctx context.Context, cmd *cobra.Command, app *appState, flags *grantFlags, args []string) error {
	rawTarget, positionalEmails, err := extractTargetAndEmails(flags.url, args)
	if err != nil {
		return err
	}

	target, err := docurl.Parse(rawTarget)
	if err != nil {
		return err
	}

	resolvedCfg, err := app.resolveConfig(
		cmd,
		flags.role, cmd.Flags().Changed("role"),
		flags.defaultDomain, cmd.Flags().Changed("default-domain"),
	)
	if err != nil {
		return withHint(err, fmt.Sprintf("Check YAML syntax in %q.", resolvedCfg.ConfigPath))
	}

	emails, err := collectEmails(app, flags, positionalEmails, resolvedCfg.DefaultDomain)
	if err != nil {
		return err
	}

	role, err := sharer.ParseRole(resolvedCfg.Role)
	if err != nil {
		return err
	}

	shareReq := sharer.ShareRequest{
		Target: target,
		Emails: emails,
		Role:   role,
		DryRun: flags.dryRun,
	}

	out := cmd.OutOrStdout()
	palette := style.ForStream(out, app.jsonOutput, app.getenv)

	if flags.dryRun {
		report, dryErr := sharer.PlanDryRun(shareReq)
		if dryErr != nil {
			return dryErr
		}
		if app.jsonOutput {
			return printJSON(out, report)
		}
		renderShareReport(out, palette, report)
		return nil
	}

	client, err := app.clientFactory(ctx, resolvedCfg)
	if err != nil {
		return err
	}

	report, shareErr := client.Share(ctx, shareReq)
	if len(report.Results) > 0 {
		if app.jsonOutput {
			if jsonErr := printJSON(out, report); jsonErr != nil {
				return jsonErr
			}
		} else {
			renderShareReport(out, palette, report)
		}
		if shareErr != nil {
			return &CLIError{Err: shareErr, AlreadyOutput: true}
		}
		return nil
	}

	return shareErr
}

func extractTargetAndEmails(flagURL string, args []string) (string, []string, error) {
	trimmedFlagURL := strings.TrimSpace(flagURL)
	if trimmedFlagURL != "" {
		return trimmedFlagURL, args, nil
	}
	if len(args) == 0 {
		return "", nil, docurl.ErrEmptyTarget
	}
	return args[0], args[1:], nil
}

func collectEmails(app *appState, flags *grantFlags, positionalEmails []string, defaultDomain string) ([]string, error) {
	var combinedRaw []string
	combinedRaw = append(combinedRaw, flags.emails...)
	combinedRaw = append(combinedRaw, positionalEmails...)

	if fileSpec := strings.TrimSpace(flags.emailsFile); fileSpec != "" {
		var fileEmails []string
		var err error
		if fileSpec == "-" {
			fileEmails, err = docurl.ReadEmailsFromReaderWithDomain(app.stdin, defaultDomain)
			if err != nil {
				return nil, withHint(err, "Pipe one email or @handle per line (or comma-separated) to stdin when using '--emails-file -'.")
			}
		} else {
			fileHandle, openErr := os.Open(fileSpec)
			if openErr != nil {
				return nil, withHint(
					fmt.Errorf("opening emails file %q: %w", fileSpec, openErr),
					fmt.Sprintf("Verify that %q exists and is readable, or pass '-' to read from stdin.", fileSpec),
				)
			}
			defer fileHandle.Close()
			fileEmails, err = docurl.ReadEmailsFromReaderWithDomain(fileHandle, defaultDomain)
			if err != nil {
				return nil, withHint(err, fmt.Sprintf("Ensure %q contains valid emails or handles (one per line or comma-separated).", fileSpec))
			}
		}
		combinedRaw = append(combinedRaw, fileEmails...)
	}

	return docurl.ParseEmailsWithDomain(combinedRaw, defaultDomain)
}
