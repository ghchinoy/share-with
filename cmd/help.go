// help.go configures Cobra's usage template with semantic color styling.

package cmd

import (
	"os"
	"sync"

	"github.com/spf13/cobra"

	"github.com/ghchinoy/share-with/internal/style"
)

var registerTemplateFuncsOnce sync.Once

const usageTemplate = `{{StyleHeading "Usage:"}}{{if .Runnable}}
  {{StyleCommand .UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{StyleCommand (printf "%s [command]" .CommandPath)}}{{end}}{{if gt (len .Aliases) 0}}

{{StyleHeading "Aliases:"}}
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

{{StyleHeading "Examples:"}}
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

{{StyleHeading "Available Commands:"}}{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{StyleCommand (rpad .Name .NamePadding)}} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{StyleHeading .Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{StyleCommand (rpad .Name .NamePadding)}} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

{{StyleHeading "Additional Commands:"}}{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{StyleCommand (rpad .Name .NamePadding)}} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{StyleHeading "Flags:"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{StyleHeading "Global Flags:"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

{{StyleHeading "Additional help topics:"}}{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

{{StyleMuted (printf "Use \"%s [command] --help\" for more information about a command." .CommandPath)}}{{end}}
`

func applySemanticUsageTemplate(root *cobra.Command) {
	registerTemplateFuncsOnce.Do(func() {
		cobra.AddTemplateFunc("StyleHeading", func(s string) string {
			return style.ForStream(os.Stdout, false, os.Getenv).Accent(s)
		})
		cobra.AddTemplateFunc("StyleCommand", func(s string) string {
			return style.ForStream(os.Stdout, false, os.Getenv).Command(s)
		})
		cobra.AddTemplateFunc("StyleMuted", func(s string) string {
			return style.ForStream(os.Stdout, false, os.Getenv).Muted(s)
		})
	})
	root.SetUsageTemplate(usageTemplate)
}
