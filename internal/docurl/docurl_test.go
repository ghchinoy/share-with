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

package docurl

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParse(t *testing.T) {
	const sampleID = "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"

	tests := []struct {
		name    string
		input   string
		want    Target
		wantErr error
	}{
		{
			name:  "google_doc_edit_url",
			input: "https://docs.google.com/document/d/" + sampleID + "/edit?usp=sharing",
			want: Target{
				FileID:       sampleID,
				CanonicalURL: "https://docs.google.com/document/d/" + sampleID + "/edit",
				ResourceKind: "document",
			},
		},
		{
			name:  "google_doc_multi_account_url",
			input: "https://docs.google.com/document/u/2/d/" + sampleID + "/edit#heading=h.abc",
			want: Target{
				FileID:       sampleID,
				CanonicalURL: "https://docs.google.com/document/d/" + sampleID + "/edit",
				ResourceKind: "document",
			},
		},
		{
			name:  "google_sheet_url",
			input: "https://docs.google.com/spreadsheets/d/" + sampleID + "/edit#gid=0",
			want: Target{
				FileID:       sampleID,
				CanonicalURL: "https://docs.google.com/spreadsheets/d/" + sampleID + "/edit",
				ResourceKind: "spreadsheet",
			},
		},
		{
			name:  "google_slides_url",
			input: "https://docs.google.com/presentation/d/" + sampleID + "/edit",
			want: Target{
				FileID:       sampleID,
				CanonicalURL: "https://docs.google.com/presentation/d/" + sampleID + "/edit",
				ResourceKind: "presentation",
			},
		},
		{
			name:  "drive_file_view_url",
			input: "https://drive.google.com/file/d/" + sampleID + "/view?usp=drive_link",
			want: Target{
				FileID:       sampleID,
				CanonicalURL: "https://drive.google.com/file/d/" + sampleID + "/view",
				ResourceKind: "file",
			},
		},
		{
			name:  "drive_folder_url",
			input: "https://drive.google.com/drive/u/0/folders/" + sampleID + "?usp=sharing",
			want: Target{
				FileID:       sampleID,
				CanonicalURL: "https://drive.google.com/drive/folders/" + sampleID,
				ResourceKind: "folder",
			},
		},
		{
			name:  "drive_open_query_id",
			input: "https://drive.google.com/open?id=" + sampleID,
			want: Target{
				FileID:       sampleID,
				CanonicalURL: "https://drive.google.com/file/d/" + sampleID + "/view",
				ResourceKind: "file",
			},
		},
		{
			name:  "raw_drive_file_id",
			input: sampleID,
			want: Target{
				FileID:       sampleID,
				CanonicalURL: "https://drive.google.com/file/d/" + sampleID + "/view",
				ResourceKind: "file",
			},
		},
		{
			name:  "google_doc_with_resourcekey",
			input: "https://docs.google.com/document/d/" + sampleID + "/edit?resourcekey=0-aBcDeFgHiJkLmNoPqRsTuVw&tab=t.0",
			want: Target{
				FileID:       sampleID,
				ResourceKey:  "0-aBcDeFgHiJkLmNoPqRsTuVw",
				CanonicalURL: "https://docs.google.com/document/d/" + sampleID + "/edit",
				ResourceKind: "document",
			},
		},
		{
			name:  "colab_drive_notebook_with_resourcekey",
			input: "https://colab.research.google.com/drive/1AbCdEfGhIjKlMnOpQrStUvWxYz012345?resourcekey=0-xYz9876543210AbCdEfGhIj&usp=sharing",
			want: Target{
				FileID:       "1AbCdEfGhIjKlMnOpQrStUvWxYz012345",
				ResourceKey:  "0-xYz9876543210AbCdEfGhIj",
				CanonicalURL: "https://colab.research.google.com/drive/1AbCdEfGhIjKlMnOpQrStUvWxYz012345",
				ResourceKind: "colab",
			},
		},
		{
			name:    "empty_input",
			input:   "   ",
			wantErr: ErrEmptyTarget,
		},
		{
			name:    "published_to_web_url_rejected",
			input:   "https://docs.google.com/document/d/e/2PACX-1vR9xyz1234567890/pub",
			wantErr: ErrPublishedURL,
		},
		{
			name:    "unsupported_host_rejected",
			input:   "https://example.com/document/d/" + sampleID + "/edit",
			wantErr: ErrInvalidTarget,
		},
		{
			name:    "control_character_rejected",
			input:   "https://docs.google.com/document/d/" + sampleID + "\x00/edit",
			wantErr: ErrInvalidTarget,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("Parse(%q) error = %v, want error wrapping %v", tc.input, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tc.input, err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Parse(%q) mismatch (-want +got):\n%s", tc.input, diff)
			}
		})
	}
}

func TestParseEmails(t *testing.T) {
	tests := []struct {
		name    string
		inputs  []string
		want    []string
		wantErr bool
	}{
		{
			name:   "mixed_shorthand_handles_default_to_google_com",
			inputs: []string{"@alice, team-leads@google.com, doc-reviewers@, bob@, carol@, dave@"},
			want: []string{
				"alice@google.com",
				"team-leads@google.com",
				"doc-reviewers@google.com",
				"bob@google.com",
				"carol@google.com",
				"dave@google.com",
			},
		},
		{
			name: "positional_shell_tokens_with_trailing_commas_and_dedup",
			inputs: []string{
				"@alice,",
				"alice@,",
				"alice,",
				"ALICE@google.com,",
				"@team-leads@google.com",
			},
			want: []string{
				"alice@google.com",
				"team-leads@google.com",
			},
		},
		{
			name:   "comma_and_semicolon_deduped_lowercase",
			inputs: []string{"Alice@Example.com, bob@example.com; alice@example.com", "carol@example.org"},
			want:   []string{"alice@example.com", "bob@example.com", "carol@example.org"},
		},
		{
			name:   "display_name_addresses_including_shorthand",
			inputs: []string{`"Doe, Jane" <Jane.Doe@example.com>, Team Group <doc-reviewers@>, Lead <@alice>`},
			want:   []string{"jane.doe@example.com", "doc-reviewers@google.com", "alice@google.com"},
		},
		{
			name:    "empty_slice",
			inputs:  nil,
			wantErr: true,
		},
		{
			name:    "missing_domain_dot_when_explicit_domain_present",
			inputs:  []string{"alice@localhost"},
			wantErr: true,
		},
		{
			name:    "bare_at_sign_rejected",
			inputs:  []string{"@"},
			wantErr: true,
		},
		{
			name:    "double_at_handle_rejected",
			inputs:  []string{"@alice@"},
			wantErr: true,
		},
		{
			name:    "url_in_emails_rejected",
			inputs:  []string{"https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit"},
			wantErr: true,
		},
		{
			name:    "embedded_newline_injection_blocked",
			inputs:  []string{"alice@example.com\nBcc:attacker@evil.com"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseEmails(tc.inputs)
			if tc.wantErr {
				if err == nil {
					t.Errorf("ParseEmails(%q) = %v, want error", tc.inputs, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseEmails(%q) unexpected error: %v", tc.inputs, err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ParseEmails(%q) mismatch (-want +got):\n%s", tc.inputs, diff)
			}
		})
	}
}

func TestParseEmailsWithDomain(t *testing.T) {
	inputs := []string{"@alice, bob@, external@partner.io"}
	got, err := ParseEmailsWithDomain(inputs, "@corp.example.com")
	if err != nil {
		t.Fatalf("ParseEmailsWithDomain(%q, %q) unexpected error: %v", inputs, "@corp.example.com", err)
	}
	want := []string{"alice@corp.example.com", "bob@corp.example.com", "external@partner.io"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ParseEmailsWithDomain(%q, %q) mismatch (-want +got):\n%s", inputs, "@corp.example.com", diff)
	}
}

func TestReadEmailsFromReader(t *testing.T) {
	rawFile := `
# Reviewers list for Q4 spec
@alice
  bob@, doc-reviewers@ # inline comment
team-leads@google.com
ALICE@
`
	got, err := ReadEmailsFromReader(strings.NewReader(rawFile))
	if err != nil {
		t.Fatalf("ReadEmailsFromReader(%q) unexpected error: %v", rawFile, err)
	}
	want := []string{
		"alice@google.com",
		"bob@google.com",
		"doc-reviewers@google.com",
		"team-leads@google.com",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ReadEmailsFromReader(%q) mismatch (-want +got):\n%s", rawFile, diff)
	}
}
