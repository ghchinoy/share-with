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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"

	"github.com/ghchinoy/share-with/cmd"
	"github.com/ghchinoy/share-with/internal/config"
	"github.com/ghchinoy/share-with/internal/sharer"
)

func TestCommandRegistrationAndSilentContract(t *testing.T) {
	root := cmd.NewRootCmd()

	for _, path := range [][]string{
		{"grant"},
		{"list"},
		{"auth"},
		{"auth", "status"},
		{"auth", "login"},
	} {
		found, _, err := root.Find(path)
		if err != nil || found == nil {
			t.Errorf("root.Find(%v) = (%v, %v), want registered command", path, found, err)
		}
	}

	// Ensure no notification flags exist anywhere on root or grant.
	grantCmd, _, _ := root.Find([]string{"grant"})
	for _, forbiddenFlag := range []string{"notify", "send-notification", "send-notification-email", "message", "email-message"} {
		if f := root.Flags().Lookup(forbiddenFlag); f != nil {
			t.Errorf("root.Flags().Lookup(%q) = %v, want nil (notifications must never be configurable)", forbiddenFlag, f)
		}
		if f := grantCmd.Flags().Lookup(forbiddenFlag); f != nil {
			t.Errorf("grant.Flags().Lookup(%q) = %v, want nil (notifications must never be configurable)", forbiddenFlag, f)
		}
	}
}

func TestCLI_DefaultReaderAndEditorRolesAlwaysSilent(t *testing.T) {
	const fileID = "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"
	docURL := "https://docs.google.com/document/d/" + fileID + "/edit"

	tests := []struct {
		name        string
		cliArgs     []string
		stdin       string
		wantRole    sharer.Role
		wantAPIRole string
		wantEmails  []string
	}{
		{
			name:        "root_positional_defaults_to_reader",
			cliArgs:     []string{docURL, "Alice@example.com", "bob@example.com", "--json"},
			wantRole:    sharer.RoleReader,
			wantAPIRole: "reader",
			wantEmails:  []string{"alice@example.com", "bob@example.com"},
		},
		{
			name:        "grant_commenter_comma_emails",
			cliArgs:     []string{"grant", "--url", docURL, "--emails", "alice@example.com,bob@example.com", "--role", "commenter", "--json"},
			wantRole:    sharer.RoleCommenter,
			wantAPIRole: "commenter",
			wantEmails:  []string{"alice@example.com", "bob@example.com"},
		},
		{
			name:        "grant_editor_from_stdin",
			cliArgs:     []string{"grant", docURL, "--emails-file", "-", "--role", "editor", "--json"},
			stdin:       "# team leads\nlead1@example.com\nlead2@example.com\n",
			wantRole:    sharer.RoleEditor,
			wantAPIRole: "writer",
			wantEmails:  []string{"lead1@example.com", "lead2@example.com"},
		},
		{
			name: "shorthand_google_handles_positional_and_flag",
			cliArgs: []string{
				docURL,
				"@alice,",
				"team-leads@google.com,",
				"doc-reviewers@,",
				"bob@,",
				"carol@,",
				"dave@",
				"--json",
			},
			wantRole:    sharer.RoleReader,
			wantAPIRole: "reader",
			wantEmails: []string{
				"alice@google.com",
				"team-leads@google.com",
				"doc-reviewers@google.com",
				"bob@google.com",
				"carol@google.com",
				"dave@google.com",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedQueries []string
			var capturedRoles []string
			var capturedEmails []string

			factory := func(ctx context.Context, _ config.Resolved) (*sharer.Client, error) {
				rt := mockRoundTripper(func(req *http.Request) (*http.Response, error) {
					hdr := make(http.Header)
					hdr.Set("Content-Type", "application/json")

					if req.Method == http.MethodGet {
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     hdr,
							Body:       io.NopCloser(strings.NewReader(`{"id":"` + fileID + `","name":"Team Spec"}`)),
						}, nil
					}

					capturedQueries = append(capturedQueries, req.URL.Query().Get("sendNotificationEmail"))
					rawBody, _ := io.ReadAll(req.Body)
					var perm drive.Permission
					_ = json.Unmarshal(rawBody, &perm)
					capturedRoles = append(capturedRoles, perm.Role)
					capturedEmails = append(capturedEmails, perm.EmailAddress)

					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     hdr,
						Body:       io.NopCloser(strings.NewReader(`{"id":"perm-123","role":"` + perm.Role + `","emailAddress":"` + perm.EmailAddress + `"}`)),
					}, nil
				})

				httpClient := &http.Client{Transport: sharer.NewSilentGuardTransport(rt, "")}
				svc, err := drive.NewService(ctx, option.WithHTTPClient(httpClient), option.WithEndpoint("https://drive.example.test/"))
				if err != nil {
					return nil, err
				}
				return sharer.NewClient(svc), nil
			}

			var stdout, stderr bytes.Buffer
			root := cmd.NewRootCmdWithOptions(cmd.Options{
				Getenv:        func(string) string { return "" },
				Stdin:         strings.NewReader(tc.stdin),
				Stdout:        &stdout,
				Stderr:        &stderr,
				ClientFactory: factory,
			})
			root.SetArgs(tc.cliArgs)

			if err := root.ExecuteContext(t.Context()); err != nil {
				t.Fatalf("root.ExecuteContext(%v) unexpected error: %v (stderr: %s)", tc.cliArgs, err, stderr.String())
			}

			var gotReport sharer.ShareReport
			if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
				t.Fatalf("json.Unmarshal(%q) unexpected error: %v", stdout.String(), err)
			}

			if !gotReport.Silent || gotReport.SendNotificationEmail {
				t.Errorf("ShareReport(%v) silent=(%v, sendNotificationEmail=%v), want (true, false)",
					tc.cliArgs, gotReport.Silent, gotReport.SendNotificationEmail)
			}
			if gotReport.Role != tc.wantRole || gotReport.APIRole != tc.wantAPIRole {
				t.Errorf("ShareReport(%v) role=(%q, api=%q), want (%q, api=%q)",
					tc.cliArgs, gotReport.Role, gotReport.APIRole, tc.wantRole, tc.wantAPIRole)
			}
			for i, q := range capturedQueries {
				if q != "false" {
					t.Errorf("HTTP request[%d] sendNotificationEmail = %q, want %q", i, q, "false")
				}
			}
			if strings.Join(capturedEmails, ",") != strings.Join(tc.wantEmails, ",") {
				t.Errorf("capturedEmails = %v, want %v", capturedEmails, tc.wantEmails)
			}
			for i, gotAPIRole := range capturedRoles {
				if gotAPIRole != tc.wantAPIRole {
					t.Errorf("HTTP request[%d] perm.Role = %q, want %q", i, gotAPIRole, tc.wantAPIRole)
				}
			}
		})
	}
}

func TestCLI_FlagValidationAndDryRun(t *testing.T) {
	const docURL = "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit"

	t.Run("dry_run_skips_auth_and_network", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		factoryCalled := false
		root := cmd.NewRootCmdWithOptions(cmd.Options{
			Getenv: func(key string) string {
				if key == "NO_COLOR" {
					return "1"
				}
				return ""
			},
			Stdout: &stdout,
			Stderr: &stderr,
			ClientFactory: func(context.Context, config.Resolved) (*sharer.Client, error) {
				factoryCalled = true
				return nil, nil
			},
		})
		args := []string{docURL, "alice@example.com", "--role", "editor", "--dry-run"}
		root.SetArgs(args)

		if err := root.ExecuteContext(t.Context()); err != nil {
			t.Fatalf("root.ExecuteContext(%v) unexpected error: %v", args, err)
		}
		if factoryCalled {
			t.Errorf("root.ExecuteContext(%v) invoked ClientFactory during --dry-run, want skipped", args)
		}
		if !strings.Contains(stdout.String(), "DRY RUN (SILENT)") || !strings.Contains(stdout.String(), "editor (writer)") {
			t.Errorf("dry-run output = %q, want 'DRY RUN (SILENT)' and 'editor (writer)'", stdout.String())
		}
	})

	t.Run("list_rejects_conflicting_url_flag_and_arg", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		root := cmd.NewRootCmdWithOptions(cmd.Options{
			Getenv: func(string) string { return "" },
			Stdout: &stdout,
			Stderr: &stderr,
		})
		args := []string{"list", "--url", docURL, docURL}
		root.SetArgs(args)

		err := root.ExecuteContext(t.Context())
		if err == nil || !strings.Contains(err.Error(), "cannot specify both --url and a positional URL") {
			t.Errorf("root.ExecuteContext(%v) error = %v, want conflicting URL error", args, err)
		}
	})

	t.Run("rejects_writer_role_with_editor_error", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		root := cmd.NewRootCmdWithOptions(cmd.Options{
			Getenv: func(string) string { return "" },
			Stdout: &stdout,
			Stderr: &stderr,
		})
		args := []string{"grant", docURL, "alice@example.com", "--role", "writer", "--dry-run"}
		root.SetArgs(args)

		err := root.ExecuteContext(t.Context())
		if err == nil || !strings.Contains(err.Error(), `use role "editor" instead of "writer"`) {
			t.Errorf("root.ExecuteContext(%v) error = %v, want editor-alias error", args, err)
		}
	})

	t.Run("insufficient_scope_error_produces_adc_scopes_hint", func(t *testing.T) {
		scopeErr := errors.New(`access denied for Drive file "1bDd" (HTTP 403): googleapi: Error 403: Request had insufficient authentication scopes. ACCESS_TOKEN_SCOPE_INSUFFICIENT`)
		_, gotHint := cmd.ClassifyError(scopeErr)
		if !strings.Contains(gotHint, "--scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform") {
			t.Errorf("cmd.ClassifyError(%q) hint = %q, want gcloud --scopes remediation", scopeErr, gotHint)
		}
	})
}

type mockRoundTripper func(*http.Request) (*http.Response, error)

func (m mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req)
}
