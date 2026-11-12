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

package sharer

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"

	"github.com/ghchinoy/share-with/internal/docurl"
)

func TestParseRole(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Role
		wantAPI string
		wantErr error
	}{
		{name: "empty_defaults_to_reader", input: "", want: RoleReader, wantAPI: "reader"},
		{name: "reader", input: "reader", want: RoleReader, wantAPI: "reader"},
		{name: "commenter_case_insensitive", input: "Commenter", want: RoleCommenter, wantAPI: "commenter"},
		{name: "editor_maps_to_writer", input: "editor", want: RoleEditor, wantAPI: "writer"},
		{name: "writer_rejected_with_alias_error", input: "writer", wantErr: ErrWriterRoleAlias},
		{name: "owner_rejected_because_not_silent", input: "owner", wantErr: ErrOwnerRoleForbidden},
		{name: "unknown_role_rejected", input: "viewer", wantErr: ErrInvalidRole},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseRole(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("ParseRole(%q) error = %v, want wrapping %v", tc.input, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRole(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseRole(%q) = %q, want %q", tc.input, got, tc.want)
			}
			if got.APIRole() != tc.wantAPI {
				t.Errorf("Role(%q).APIRole() = %q, want %q", got, got.APIRole(), tc.wantAPI)
			}
		})
	}
}

func TestSilentGuardTransport_BlocksNotifications(t *testing.T) {
	noopBase := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"id":"perm-1"}`)),
		}, nil
	})
	guard := NewSilentGuardTransport(noopBase, "my-quota-proj")

	tests := []struct {
		name      string
		method    string
		rawURL    string
		wantBlock bool
	}{
		{
			name:      "blocks_missing_sendNotificationEmail",
			method:    http.MethodPost,
			rawURL:    "https://www.googleapis.com/drive/v3/files/abc123456789012/permissions?supportsAllDrives=true",
			wantBlock: true,
		},
		{
			name:      "blocks_sendNotificationEmail_true",
			method:    http.MethodPost,
			rawURL:    "https://www.googleapis.com/drive/v3/files/abc123456789012/permissions?sendNotificationEmail=true",
			wantBlock: true,
		},
		{
			name:      "blocks_transferOwnership_true",
			method:    http.MethodPost,
			rawURL:    "https://www.googleapis.com/drive/v3/files/abc123456789012/permissions?sendNotificationEmail=false&transferOwnership=true",
			wantBlock: true,
		},
		{
			name:      "blocks_emailMessage",
			method:    http.MethodPost,
			rawURL:    "https://www.googleapis.com/drive/v3/files/abc123456789012/permissions?sendNotificationEmail=false&emailMessage=hello",
			wantBlock: true,
		},
		{
			name:      "allows_explicit_silent_permission_create",
			method:    http.MethodPost,
			rawURL:    "https://www.googleapis.com/drive/v3/files/abc123456789012/permissions?sendNotificationEmail=false&supportsAllDrives=true",
			wantBlock: false,
		},
		{
			name:      "allows_get_permissions_list",
			method:    http.MethodGet,
			rawURL:    "https://www.googleapis.com/drive/v3/files/abc123456789012/permissions",
			wantBlock: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), tc.method, tc.rawURL, nil)
			if err != nil {
				t.Fatalf("http.NewRequestWithContext(%q, %q) unexpected error: %v", tc.method, tc.rawURL, err)
			}
			resp, err := guard.RoundTrip(req)
			if resp != nil && resp.Body != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			if tc.wantBlock {
				if !errors.Is(err, ErrNotificationBlocked) {
					t.Errorf("SilentGuardTransport.RoundTrip(%s %q) error = %v, want %v", tc.method, tc.rawURL, err, ErrNotificationBlocked)
				}
				return
			}
			if err != nil {
				t.Fatalf("SilentGuardTransport.RoundTrip(%s %q) unexpected error: %v", tc.method, tc.rawURL, err)
			}
		})
	}
}

func TestClientShare_AlwaysSilentAndMapsRoles(t *testing.T) {
	ctx := t.Context()
	const fileID = "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"

	type capturedCreate struct {
		SendNotificationEmail string
		SupportsAllDrives     string
		Body                  drive.Permission
	}

	for _, roleCase := range []struct {
		role        Role
		wantAPIRole string
	}{
		{role: RoleReader, wantAPIRole: "reader"},
		{role: RoleCommenter, wantAPIRole: "commenter"},
		{role: RoleEditor, wantAPIRole: "writer"},
	} {
		t.Run(string(roleCase.role), func(t *testing.T) {
			var creates []capturedCreate

			mockRT := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				header := make(http.Header)
				header.Set("Content-Type", "application/json")

				if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/files/"+fileID) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     header,
						Body: io.NopCloser(strings.NewReader(
							`{"id":"` + fileID + `","name":"Architecture Design Doc","webViewLink":"https://docs.google.com/document/d/` + fileID + `/edit"}`,
						)),
					}, nil
				}

				if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/files/"+fileID+"/permissions") {
					bodyBytes, err := io.ReadAll(req.Body)
					if err != nil {
						return nil, err
					}
					var perm drive.Permission
					if err := json.Unmarshal(bodyBytes, &perm); err != nil {
						return nil, err
					}
					creates = append(creates, capturedCreate{
						SendNotificationEmail: req.URL.Query().Get("sendNotificationEmail"),
						SupportsAllDrives:     req.URL.Query().Get("supportsAllDrives"),
						Body:                  perm,
					})
					respBody := `{"id":"perm-` + perm.EmailAddress + `","type":"user","role":"` + perm.Role + `","emailAddress":"` + perm.EmailAddress + `"}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     header,
						Body:       io.NopCloser(bytes.NewBufferString(respBody)),
					}, nil
				}

				return &http.Response{
					StatusCode: http.StatusNotFound,
					Header:     header,
					Body:       io.NopCloser(strings.NewReader(`{"error":{"code":404,"message":"unexpected path"}}`)),
				}, nil
			})

			httpClient := &http.Client{Transport: NewSilentGuardTransport(mockRT, "")}
			svc, err := drive.NewService(ctx, option.WithHTTPClient(httpClient), option.WithEndpoint("https://drive.example.test/"))
			if err != nil {
				t.Fatalf("drive.NewService() unexpected error: %v", err)
			}

			client := NewClient(svc)
			target, err := docurl.Parse("https://docs.google.com/document/d/" + fileID + "/edit")
			if err != nil {
				t.Fatalf("docurl.Parse() unexpected error: %v", err)
			}

			req := ShareRequest{
				Target: target,
				Emails: []string{"alice@example.com", "bob@example.com"},
				Role:   roleCase.role,
			}
			report, err := client.Share(ctx, req)
			if err != nil {
				t.Fatalf("Client.Share(%+v) unexpected error: %v", req, err)
			}

			if !report.Silent || report.SendNotificationEmail {
				t.Errorf("Client.Share(%+v) silent flags = (Silent:%v, SendNotificationEmail:%v), want (true, false)",
					req, report.Silent, report.SendNotificationEmail)
			}
			if report.Succeeded != 2 || report.Failed != 0 {
				t.Errorf("Client.Share(%+v) counts = (%d succeeded, %d failed), want (2, 0)", req, report.Succeeded, report.Failed)
			}
			if len(creates) != 2 {
				t.Fatalf("Client.Share(%+v) issued %d create calls, want 2", req, len(creates))
			}

			for i, gotCreate := range creates {
				if gotCreate.SendNotificationEmail != "false" {
					t.Errorf("create[%d].SendNotificationEmail = %q, want %q", i, gotCreate.SendNotificationEmail, "false")
				}
				if gotCreate.SupportsAllDrives != "true" {
					t.Errorf("create[%d].SupportsAllDrives = %q, want %q", i, gotCreate.SupportsAllDrives, "true")
				}
				if gotCreate.Body.Role != roleCase.wantAPIRole {
					t.Errorf("create[%d].Body.Role = %q, want %q", i, gotCreate.Body.Role, roleCase.wantAPIRole)
				}
				if gotCreate.Body.Type != "user" {
					t.Errorf("create[%d].Body.Type = %q, want %q", i, gotCreate.Body.Type, "user")
				}
			}
		})
	}
}

func TestClientShare_PartialFailureNonGoogleAccountNeverNotifies(t *testing.T) {
	ctx := t.Context()
	const fileID = "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"

	callCount := 0
	mockRT := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		header := make(http.Header)
		header.Set("Content-Type", "application/json")

		if req.Method == http.MethodGet {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       io.NopCloser(strings.NewReader(`{"id":"` + fileID + `","name":"Q4 Roadmap"}`)),
			}, nil
		}

		callCount++
		if req.URL.Query().Get("sendNotificationEmail") != "false" {
			t.Errorf("request %d had sendNotificationEmail=%q, want \"false\"", callCount, req.URL.Query().Get("sendNotificationEmail"))
		}

		bodyBytes, _ := io.ReadAll(req.Body)
		if strings.Contains(string(bodyBytes), "nogoogle@external.test") {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     header,
				Body: io.NopCloser(strings.NewReader(
					`{"error":{"code":400,"message":"Since there is no Google account associated with this email address, you must check the \"Notify people\" box to invite this recipient."}}`,
				)),
			}, nil
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(strings.NewReader(`{"id":"perm-ok","type":"user","role":"reader","emailAddress":"alice@example.com"}`)),
		}, nil
	})

	httpClient := &http.Client{Transport: NewSilentGuardTransport(mockRT, "")}
	svc, err := drive.NewService(ctx, option.WithHTTPClient(httpClient), option.WithEndpoint("https://drive.example.test/"))
	if err != nil {
		t.Fatalf("drive.NewService() unexpected error: %v", err)
	}

	target, _ := docurl.Parse(fileID)
	req := ShareRequest{
		Target: target,
		Emails: []string{"alice@example.com", "nogoogle@external.test"},
		Role:   RoleReader,
	}

	report, err := NewClient(svc).Share(ctx, req)
	if err == nil {
		t.Fatalf("Client.Share(%+v) succeeded, want partial failure error", req)
	}
	if callCount != 2 {
		t.Errorf("Client.Share(%+v) made %d permission calls, want 2 (must never retry with notifications)", req, callCount)
	}
	if report.Succeeded != 1 || report.Failed != 1 {
		t.Errorf("Client.Share(%+v) counts = (%d, %d), want (1, 1)", req, report.Succeeded, report.Failed)
	}
	if !strings.Contains(report.Results[1].Hint, "blocks all notifications") {
		t.Errorf("Client.Share(%+v) failure hint = %q, want explanation of silent-only enforcement", req, report.Results[1].Hint)
	}
}

func TestPlanDryRun(t *testing.T) {
	target := docurl.Target{
		FileID:       "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
		CanonicalURL: "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit",
		ResourceKind: "document",
	}
	req := ShareRequest{
		Target: target,
		Emails: []string{"alice@example.com"},
		Role:   RoleEditor,
		DryRun: true,
	}

	got, err := PlanDryRun(req)
	if err != nil {
		t.Fatalf("PlanDryRun(%+v) unexpected error: %v", req, err)
	}
	want := ShareReport{
		FileID:                target.FileID,
		DocumentURL:           target.CanonicalURL,
		Role:                  RoleEditor,
		APIRole:               "writer",
		Silent:                true,
		SendNotificationEmail: false,
		DryRun:                true,
		Succeeded:             1,
		Results: []RecipientResult{
			{
				Email:   "alice@example.com",
				Role:    RoleEditor,
				APIRole: "writer",
				Silent:  true,
				DryRun:  true,
				Status:  "dry_run",
			},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("PlanDryRun(%+v) mismatch (-want +got):\n%s", req, diff)
	}
}

func TestQuotaProjectExtractionAndGcloudFallback(t *testing.T) {
	adcJSON := []byte(`{"type":"authorized_user","client_id":"x","quota_project_id":"adc-quota-proj"}`)
	if got := extractJSONQuotaProject(adcJSON); got != "adc-quota-proj" {
		t.Errorf("extractJSONQuotaProject(%s) = %q, want %q", adcJSON, got, "adc-quota-proj")
	}

	tmpHome := t.TempDir()
	cfgDir := filepath.Join(tmpHome, ".config", "gcloud", "configurations")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("os.MkdirAll(%q) unexpected error: %v", cfgDir, err)
	}
	if err := os.WriteFile(filepath.Join(tmpHome, ".config", "gcloud", "active_config"), []byte("work\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(active_config) unexpected error: %v", err)
	}
	iniBody := "[core]\naccount = user@google.com\nproject = core-fallback-proj\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config_work"), []byte(iniBody), 0o600); err != nil {
		t.Fatalf("os.WriteFile(config_work) unexpected error: %v", err)
	}

	if got := readGcloudActiveProject(func(string) string { return "" }, tmpHome); got != "core-fallback-proj" {
		t.Errorf("readGcloudActiveProject(work) = %q, want %q", got, "core-fallback-proj")
	}
}

func TestClientShare_NoDowngradeSkipsEqualOrHigherRolesAndSendsResourceKey(t *testing.T) {
	ctx := t.Context()
	const (
		fileID      = "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"
		resourceKey = "0-aBcDeFgHiJkLmNoPqRsTuVw"
	)

	var createdEmails []string
	var sawResourceKeyHeaders []string

	mockRT := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		sawResourceKeyHeaders = append(sawResourceKeyHeaders, req.Header.Get("X-Goog-Drive-Resource-Keys"))
		header := make(http.Header)
		header.Set("Content-Type", "application/json")

		if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/files/"+fileID) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       io.NopCloser(strings.NewReader(`{"id":"` + fileID + `","name":"Architecture Review Packet"}`)),
			}, nil
		}

		if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/files/"+fileID+"/permissions") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body: io.NopCloser(strings.NewReader(`{"permissions":[
					{"id":"p-editor","type":"user","role":"writer","emailAddress":"carol@google.com"},
					{"id":"p-commenter","type":"user","role":"commenter","emailAddress":"bob@google.com"},
					{"id":"p-reader","type":"group","role":"reader","emailAddress":"doc-reviewers@google.com"}
				]}`)),
			}, nil
		}

		if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/files/"+fileID+"/permissions") {
			rawBody, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			var perm drive.Permission
			if err := json.Unmarshal(rawBody, &perm); err != nil {
				return nil, err
			}
			createdEmails = append(createdEmails, perm.EmailAddress)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       io.NopCloser(strings.NewReader(`{"id":"p-new","role":"reader","emailAddress":"` + perm.EmailAddress + `"}`)),
			}, nil
		}

		return &http.Response{StatusCode: http.StatusNotFound, Header: header, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})

	httpClient := &http.Client{Transport: NewSilentGuardTransport(mockRT, "")}
	svc, err := drive.NewService(ctx, option.WithHTTPClient(httpClient), option.WithEndpoint("https://drive.example.test/"))
	if err != nil {
		t.Fatalf("drive.NewService() unexpected error: %v", err)
	}

	target, err := docurl.Parse("https://docs.google.com/document/d/" + fileID + "/edit?resourcekey=" + resourceKey)
	if err != nil {
		t.Fatalf("docurl.Parse() unexpected error: %v", err)
	}

	req := ShareRequest{
		Target: target,
		Emails: []string{
			"carol@google.com",
			"bob@google.com",
			"doc-reviewers@google.com",
			"alice@google.com",
		},
		Role: RoleReader,
	}

	report, err := NewClient(svc).Share(ctx, req)
	if err != nil {
		t.Fatalf("Client.Share(%+v) unexpected error: %v", req, err)
	}

	if diff := cmp.Diff([]string{"alice@google.com"}, createdEmails); diff != "" {
		t.Errorf("Client.Share(%+v) createdEmails mismatch (-want +got):\n%s", req, diff)
	}
	if report.Succeeded != 1 || report.Skipped != 3 || report.Failed != 0 {
		t.Errorf("Client.Share(%+v) counts = (succeeded=%d, skipped=%d, failed=%d), want (1, 3, 0)",
			req, report.Succeeded, report.Skipped, report.Failed)
	}
	if report.Results[0].ExistingRole != "editor" || report.Results[0].Status != "already_shared" {
		t.Errorf("carol result = %+v, want status=already_shared existing_role=editor", report.Results[0])
	}
	wantRKHeader := fileID + "/" + resourceKey
	for i, gotHdr := range sawResourceKeyHeaders {
		if gotHdr != wantRKHeader {
			t.Errorf("request[%d] X-Goog-Drive-Resource-Keys = %q, want %q", i, gotHdr, wantRKHeader)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
