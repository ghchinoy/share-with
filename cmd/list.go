// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// list.go implements scoped permission inspection for a Google Doc or Drive file.

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ghchinoy/share-with/internal/docurl"
	"github.com/ghchinoy/share-with/internal/style"
)

func newListCmd(app *appState) *cobra.Command {
	var (
		flagURL    string
		roleFilter string
		limit      int
	)

	cmd := &cobra.Command{
		Use:     "list [doc-url]",
		GroupID: "inspection",
		Short:   "List existing permissions on a Google Doc or Drive file",
		Long: `Inspect existing access permissions on a Google Doc or Google Drive item with
bounded pagination and optional role filtering.

Results default to --limit 25 to protect terminal scrollback and AI agent context windows.`,
		Example: `  # List up to 25 permissions on a Google Doc
  share-with list "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit"

  # Filter permissions to readers only and emit JSON
  share-with list "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" \
    --role reader --limit 10 --json

  # List editor permissions via --url flag
  share-with list --url "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit" --role editor`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit <= 0 {
				return withHint(
					fmt.Errorf("invalid --limit %d: must be greater than zero", limit),
					"Specify a positive integer such as '--limit 25'.",
				)
			}

			rawTarget := strings.TrimSpace(flagURL)
			if rawTarget == "" && len(args) > 0 {
				rawTarget = args[0]
			}
			if flagURL != "" && len(args) > 0 {
				return withHint(
					fmt.Errorf("cannot specify both --url and a positional URL argument"),
					"Provide the document URL either via '--url <url>' or as the single positional argument.",
				)
			}

			target, err := docurl.Parse(rawTarget)
			if err != nil {
				return err
			}

			resolvedCfg, err := app.resolveConfig(cmd, "", false, "", false)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), app.timeout)
			defer cancel()

			client, err := app.clientFactory(ctx, resolvedCfg)
			if err != nil {
				return err
			}

			report, err := client.FetchPermissions(ctx, target, roleFilter, limit)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if app.jsonOutput {
				return printJSON(out, report)
			}

			palette := style.ForStream(out, false, app.getenv)
			renderPermissionListReport(out, palette, report)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flagURL, "url", "u", "", "Google Docs or Google Drive URL")
	cmd.Flags().StringVarP(&roleFilter, "role", "r", "", "Filter results by role (reader, commenter, or editor)")
	cmd.Flags().IntVarP(&limit, "limit", "l", 25, "Maximum number of permissions to return")
	return cmd
}
