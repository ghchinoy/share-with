// output.go renders deterministic JSON and Tufte-inspired, alignment-safe terminal tables.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/ghchinoy/share-with/internal/sharer"
	"github.com/ghchinoy/share-with/internal/style"
)

func printJSON(out io.Writer, payload any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(payload)
}

func renderShareReport(out io.Writer, palette style.Palette, report sharer.ShareReport) {
	modeBadge := palette.Pass("SILENT SHARE")
	if report.DryRun {
		modeBadge = palette.Warn("DRY RUN (SILENT)")
	}

	title := report.FileID
	if report.FileName != "" {
		title = fmt.Sprintf("%s (%s)", report.FileName, report.FileID)
	}

	_, _ = fmt.Fprintf(out, "%s  %s\n", modeBadge, palette.Bold(title))
	_, _ = fmt.Fprintf(out, "%s  %s %s  %s %s\n\n",
		palette.Muted("Document:"),
		report.DocumentURL,
		palette.Muted("|"),
		palette.Muted("Notifications:"),
		palette.Pass("disabled (sendNotificationEmail=false)"),
	)

	emailWidth := len("RECIPIENT")
	roleWidth := len("ROLE")
	statusWidth := len("STATUS")
	idWidth := len("PERMISSION ID")

	for _, item := range report.Results {
		if n := utf8.RuneCountInString(item.Email); n > emailWidth {
			emailWidth = n
		}
		roleLabel := fmt.Sprintf("%s (%s)", item.Role, item.APIRole)
		if n := utf8.RuneCountInString(roleLabel); n > roleWidth {
			roleWidth = n
		}
		statusLabel := formatShareStatus(item)
		if n := utf8.RuneCountInString(statusLabel); n > statusWidth {
			statusWidth = n
		}
		permID := item.PermissionID
		if permID == "" {
			permID = "-"
		}
		if n := utf8.RuneCountInString(permID); n > idWidth {
			idWidth = n
		}
	}

	_, _ = fmt.Fprintf(out, "  %s  %s  %s  %s\n",
		palette.Accent(style.PadRight("RECIPIENT", emailWidth)),
		palette.Accent(style.PadRight("ROLE", roleWidth)),
		palette.Accent(style.PadRight("STATUS", statusWidth)),
		palette.Accent(style.PadRight("PERMISSION ID", idWidth)),
	)

	for _, item := range report.Results {
		roleLabel := fmt.Sprintf("%s (%s)", item.Role, item.APIRole)
		statusLabel := formatShareStatus(item)
		permID := item.PermissionID
		if permID == "" {
			permID = "-"
		}

		paddedEmail := style.PadRight(item.Email, emailWidth)
		paddedRole := style.PadRight(roleLabel, roleWidth)
		paddedStatus := style.PadRight(statusLabel, statusWidth)
		paddedID := style.PadRight(permID, idWidth)

		var styledStatus string
		switch item.Status {
		case "shared":
			styledStatus = palette.Pass(paddedStatus)
		case "already_shared":
			styledStatus = palette.Muted(paddedStatus)
		case "dry_run":
			styledStatus = palette.Warn(paddedStatus)
		default:
			styledStatus = palette.Fail(paddedStatus)
		}

		styledID := palette.ID(paddedID)
		if permID == "-" {
			styledID = palette.Muted(paddedID)
		}

		_, _ = fmt.Fprintf(out, "  %s  %s  %s  %s\n",
			paddedEmail,
			palette.Command(paddedRole),
			styledStatus,
			styledID,
		)
		if item.Error != "" {
			_, _ = fmt.Fprintf(out, "    %s %s\n", palette.Fail("Error:"), item.Error)
		}
		if item.Hint != "" {
			_, _ = fmt.Fprintf(out, "    %s %s\n", palette.Warn("Hint:"), item.Hint)
		}
	}

	if report.Skipped > 0 {
		_, _ = fmt.Fprintf(out, "\n%s %d succeeded, %d already shared, %d failed\n",
			palette.Muted("Summary:"),
			report.Succeeded,
			report.Skipped,
			report.Failed,
		)
		return
	}

	_, _ = fmt.Fprintf(out, "\n%s %d succeeded, %d failed\n",
		palette.Muted("Summary:"),
		report.Succeeded,
		report.Failed,
	)
}

func formatShareStatus(item sharer.RecipientResult) string {
	if item.Status == "already_shared" && item.ExistingRole != "" {
		return fmt.Sprintf("already_shared (%s)", item.ExistingRole)
	}
	return item.Status
}

func renderPermissionListReport(out io.Writer, palette style.Palette, report sharer.PermissionListReport) {
	title := report.FileID
	if report.FileName != "" {
		title = fmt.Sprintf("%s (%s)", report.FileName, report.FileID)
	}
	_, _ = fmt.Fprintf(out, "%s  %s\n", palette.Accent("PERMISSIONS"), palette.Bold(title))
	_, _ = fmt.Fprintf(out, "%s  %s\n\n", palette.Muted("Document:"), report.DocumentURL)

	if len(report.Permissions) == 0 {
		_, _ = fmt.Fprintln(out, palette.Muted("  No matching permissions found."))
		return
	}

	whoWidth := len("GRANTEE")
	typeWidth := len("TYPE")
	roleWidth := len("ROLE")
	idWidth := len("PERMISSION ID")

	granteeLabels := make([]string, len(report.Permissions))
	for i, perm := range report.Permissions {
		who := perm.Email
		if who == "" && perm.Domain != "" {
			who = "@" + perm.Domain
		}
		if who == "" && perm.DisplayName != "" {
			who = perm.DisplayName
		}
		if who == "" {
			who = perm.Type
		}
		granteeLabels[i] = who

		if n := utf8.RuneCountInString(who); n > whoWidth {
			whoWidth = n
		}
		if n := utf8.RuneCountInString(perm.Type); n > typeWidth {
			typeWidth = n
		}
		if n := utf8.RuneCountInString(perm.Role); n > roleWidth {
			roleWidth = n
		}
		if n := utf8.RuneCountInString(perm.ID); n > idWidth {
			idWidth = n
		}
	}

	_, _ = fmt.Fprintf(out, "  %s  %s  %s  %s\n",
		palette.Accent(style.PadRight("GRANTEE", whoWidth)),
		palette.Accent(style.PadRight("TYPE", typeWidth)),
		palette.Accent(style.PadRight("ROLE", roleWidth)),
		palette.Accent(style.PadRight("PERMISSION ID", idWidth)),
	)

	for i, perm := range report.Permissions {
		_, _ = fmt.Fprintf(out, "  %s  %s  %s  %s\n",
			style.PadRight(granteeLabels[i], whoWidth),
			palette.Muted(style.PadRight(perm.Type, typeWidth)),
			palette.Command(style.PadRight(perm.Role, roleWidth)),
			palette.ID(style.PadRight(perm.ID, idWidth)),
		)
	}

	if report.Truncated {
		_, _ = fmt.Fprintf(out, "\n%s\n",
			palette.Warn(fmt.Sprintf("Showing first %d permissions (pass --limit to adjust).", report.Limit)),
		)
	}
}

func renderAuthStatus(out io.Writer, palette style.Palette, status sharer.AuthStatus, cfgPath string, cfgLoaded bool) {
	badge := palette.Pass("READY")
	if !status.Ready {
		badge = palette.Fail("NOT CONFIGURED")
	}

	_, _ = fmt.Fprintf(out, "%s  %s\n\n", badge, palette.Bold(status.Detail))
	_, _ = fmt.Fprintf(out, "  %s  %s\n", palette.Accent(style.PadRight("Source:", 18)), palette.Command(status.Source))
	if status.CredentialsFile != "" {
		_, _ = fmt.Fprintf(out, "  %s  %s\n", palette.Accent(style.PadRight("Credentials File:", 18)), status.CredentialsFile)
	}
	if status.TokenFile != "" {
		_, _ = fmt.Fprintf(out, "  %s  %s\n", palette.Accent(style.PadRight("Token Cache:", 18)), status.TokenFile)
	}
	if status.QuotaProject != "" {
		_, _ = fmt.Fprintf(out, "  %s  %s\n", palette.Accent(style.PadRight("Quota Project:", 18)), palette.ID(status.QuotaProject))
	}
	if status.Impersonate != "" {
		_, _ = fmt.Fprintf(out, "  %s  %s\n", palette.Accent(style.PadRight("Impersonate:", 18)), status.Impersonate)
	}

	cfgState := "not present (using defaults)"
	if cfgLoaded {
		cfgState = "loaded"
	}
	_, _ = fmt.Fprintf(out, "  %s  %s %s\n", palette.Accent(style.PadRight("XDG Config:", 18)), cfgPath, palette.Muted("("+cfgState+")"))

	if status.Hint != "" {
		_, _ = fmt.Fprintf(out, "\n%s %s\n", palette.Warn("Hint:"), status.Hint)
	}
}
