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

// Package sharer implements silent-only Google Drive and Google Docs permission
// management using Google Drive API v3.
package sharer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"

	"github.com/ghchinoy/share-with/internal/docurl"
)

// Role represents a user-facing sharing role supported by share-with.
type Role string

const (
	// RoleReader grants read-only viewer access ("reader" in Drive API v3).
	RoleReader Role = "reader"

	// RoleCommenter grants read and comment/suggestion access ("commenter" in Drive API v3).
	RoleCommenter Role = "commenter"

	// RoleEditor grants full edit access ("writer" in Drive API v3).
	RoleEditor Role = "editor"
)

var (
	// ErrInvalidRole indicates an unsupported role value.
	ErrInvalidRole = errors.New("invalid role")

	// ErrWriterRoleAlias indicates the caller supplied Drive API's internal "writer" role name.
	ErrWriterRoleAlias = errors.New("use role \"editor\" instead of \"writer\"")

	// ErrOwnerRoleForbidden indicates an attempt to transfer ownership, which cannot be silent.
	ErrOwnerRoleForbidden = errors.New("ownership transfer is forbidden because Google Drive forces email notifications")
)

// ParseRole validates a CLI role string and defaults empty values to RoleReader.
func ParseRole(raw string) (Role, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "", string(RoleReader):
		return RoleReader, nil
	case string(RoleCommenter):
		return RoleCommenter, nil
	case string(RoleEditor):
		return RoleEditor, nil
	case "writer":
		return "", fmt.Errorf("%w: %q", ErrWriterRoleAlias, raw)
	case "owner", "organizer", "fileorganizer":
		return "", fmt.Errorf("%w: %q", ErrOwnerRoleForbidden, raw)
	default:
		return "", fmt.Errorf("%w %q: must be reader, commenter, or editor", ErrInvalidRole, raw)
	}
}

// APIRole returns the underlying Google Drive API v3 permission role string.
func (r Role) APIRole() string {
	switch r {
	case RoleCommenter:
		return "commenter"
	case RoleEditor:
		return "writer"
	default:
		return "reader"
	}
}

// DisplayRoleFromAPI maps a Google Drive API v3 permission role back to CLI terminology.
func DisplayRoleFromAPI(apiRole string) string {
	if apiRole == "writer" {
		return string(RoleEditor)
	}
	return apiRole
}

// ShareRequest specifies the target document, normalized email recipients, role,
// and whether to simulate the operation without mutating Drive state.
type ShareRequest struct {
	Target docurl.Target
	Emails []string
	Role   Role
	DryRun bool
}

// RecipientResult records the outcome of sharing a document with a single email address.
type RecipientResult struct {
	Email        string `json:"email"`
	Role         Role   `json:"role"`
	APIRole      string `json:"api_role"`
	ExistingRole string `json:"existing_role,omitempty"`
	PermissionID string `json:"permission_id,omitempty"`
	Silent       bool   `json:"silent"`
	DryRun       bool   `json:"dry_run,omitempty"`
	Status       string `json:"status"`
	Error        string `json:"error,omitempty"`
	Hint         string `json:"hint,omitempty"`
}

// ShareReport summarizes a batch silent-sharing operation across all requested recipients.
type ShareReport struct {
	FileID                string            `json:"file_id"`
	FileName              string            `json:"file_name,omitempty"`
	DocumentURL           string            `json:"document_url"`
	Role                  Role              `json:"role"`
	APIRole               string            `json:"api_role"`
	Silent                bool              `json:"silent"`
	SendNotificationEmail bool              `json:"send_notification_email"`
	DryRun                bool              `json:"dry_run"`
	Succeeded             int               `json:"succeeded"`
	Skipped               int               `json:"skipped,omitempty"`
	Failed                int               `json:"failed"`
	Results               []RecipientResult `json:"results"`
}

// PermissionSummary describes an existing permission entry on a Google Drive item.
type PermissionSummary struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Role        string `json:"role"`
	APIRole     string `json:"api_role"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Domain      string `json:"domain,omitempty"`
}

// PermissionListReport contains scoped permission entries for a Google Drive item.
type PermissionListReport struct {
	FileID      string              `json:"file_id"`
	FileName    string              `json:"file_name,omitempty"`
	DocumentURL string              `json:"document_url"`
	Limit       int                 `json:"limit"`
	RoleFilter  string              `json:"role_filter,omitempty"`
	Truncated   bool                `json:"truncated"`
	Permissions []PermissionSummary `json:"permissions"`
}

// Client executes silent permission grants and scoped permission queries against Drive v3.
type Client struct {
	svc *drive.Service
}

// NewClient constructs a Client wrapping an initialized Google Drive v3 service.
func NewClient(svc *drive.Service) *Client {
	return &Client{svc: svc}
}

// PlanDryRun produces the exact ShareReport that Share would execute without
// requiring authentication or issuing network calls.
func PlanDryRun(req ShareRequest) (ShareReport, error) {
	if err := validateShareRequest(req); err != nil {
		return ShareReport{}, err
	}

	results := make([]RecipientResult, 0, len(req.Emails))
	for _, email := range req.Emails {
		results = append(results, RecipientResult{
			Email:   email,
			Role:    req.Role,
			APIRole: req.Role.APIRole(),
			Silent:  true,
			DryRun:  true,
			Status:  "dry_run",
		})
	}

	return ShareReport{
		FileID:                req.Target.FileID,
		DocumentURL:           req.Target.CanonicalURL,
		Role:                  req.Role,
		APIRole:               req.Role.APIRole(),
		Silent:                true,
		SendNotificationEmail: false,
		DryRun:                true,
		Succeeded:             len(results),
		Results:               results,
	}, nil
}

// Share grants req.Role silently (sendNotificationEmail=false) to every email in req.Emails.
// It never sends notification emails under any circumstance.
func (c *Client) Share(ctx context.Context, req ShareRequest) (ShareReport, error) {
	if req.DryRun {
		return PlanDryRun(req)
	}
	if err := validateShareRequest(req); err != nil {
		return ShareReport{}, err
	}
	if c == nil || c.svc == nil {
		return ShareReport{}, errors.New("drive client is not initialized")
	}

	report := ShareReport{
		FileID:                req.Target.FileID,
		DocumentURL:           req.Target.CanonicalURL,
		Role:                  req.Role,
		APIRole:               req.Role.APIRole(),
		Silent:                true,
		SendNotificationEmail: false,
		Results:               make([]RecipientResult, 0, len(req.Emails)),
	}

	getCall := c.svc.Files.
		Get(req.Target.FileID).
		SupportsAllDrives(true).
		Fields("id,name,webViewLink").
		Context(ctx)
	applyResourceKeyHeader(getCall.Header(), req.Target)
	fileMeta, err := getCall.Do()
	if err != nil {
		return report, wrapDriveFileError(req.Target.FileID, err)
	}
	if fileMeta != nil {
		report.FileName = fileMeta.Name
		if fileMeta.WebViewLink != "" {
			report.DocumentURL = fileMeta.WebViewLink
		}
	}

	existingByEmail := c.fetchExistingByEmail(ctx, req.Target)

	var recipientErrs []error
	apiRole := req.Role.APIRole()
	targetRank := roleRank(apiRole)

	for _, email := range req.Emails {
		if existing, ok := existingByEmail[email]; ok && roleRank(existing.APIRole) >= targetRank {
			report.Skipped++
			report.Results = append(report.Results, RecipientResult{
				Email:        email,
				Role:         req.Role,
				APIRole:      apiRole,
				ExistingRole: DisplayRoleFromAPI(existing.APIRole),
				PermissionID: existing.ID,
				Silent:       true,
				Status:       "already_shared",
			})
			continue
		}

		permPayload := &drive.Permission{
			Type:         "user",
			Role:         apiRole,
			EmailAddress: email,
		}

		createCall := c.svc.Permissions.
			Create(req.Target.FileID, permPayload).
			SendNotificationEmail(false).
			SupportsAllDrives(true).
			Fields("id,type,role,emailAddress").
			Context(ctx)
		applyResourceKeyHeader(createCall.Header(), req.Target)
		createdPerm, createErr := createCall.Do()

		if createErr != nil {
			errMsg, hint := classifyRecipientError(email, createErr)
			report.Failed++
			report.Results = append(report.Results, RecipientResult{
				Email:   email,
				Role:    req.Role,
				APIRole: apiRole,
				Silent:  true,
				Status:  "failed",
				Error:   errMsg,
				Hint:    hint,
			})
			recipientErrs = append(recipientErrs, fmt.Errorf("sharing with %q: %w", email, createErr))
			continue
		}

		permID := ""
		if createdPerm != nil {
			permID = createdPerm.Id
		}
		report.Succeeded++
		report.Results = append(report.Results, RecipientResult{
			Email:        email,
			Role:         req.Role,
			APIRole:      apiRole,
			PermissionID: permID,
			Silent:       true,
			Status:       "shared",
		})
	}

	if len(recipientErrs) > 0 {
		return report, errors.Join(recipientErrs...)
	}
	return report, nil
}

// FetchPermissions lists permissions on target, optionally filtered by CLI role name
// and bounded to limit entries to protect terminal and agent context windows.
func (c *Client) FetchPermissions(ctx context.Context, target docurl.Target, roleFilter string, limit int) (PermissionListReport, error) {
	if c == nil || c.svc == nil {
		return PermissionListReport{}, errors.New("drive client is not initialized")
	}
	if limit <= 0 {
		limit = 25
	}

	normalizedFilter := strings.ToLower(strings.TrimSpace(roleFilter))
	if normalizedFilter != "" {
		if _, err := ParseRole(normalizedFilter); err != nil {
			return PermissionListReport{}, err
		}
	}

	report := PermissionListReport{
		FileID:      target.FileID,
		DocumentURL: target.CanonicalURL,
		Limit:       limit,
		RoleFilter:  normalizedFilter,
		Permissions: make([]PermissionSummary, 0),
	}

	getCall := c.svc.Files.
		Get(target.FileID).
		SupportsAllDrives(true).
		Fields("id,name,webViewLink").
		Context(ctx)
	applyResourceKeyHeader(getCall.Header(), target)
	fileMeta, err := getCall.Do()
	if err != nil {
		return report, wrapDriveFileError(target.FileID, err)
	}
	if fileMeta != nil {
		report.FileName = fileMeta.Name
		if fileMeta.WebViewLink != "" {
			report.DocumentURL = fileMeta.WebViewLink
		}
	}

	pageToken := ""
	for {
		call := c.svc.Permissions.
			List(target.FileID).
			SupportsAllDrives(true).
			PageSize(100).
			Fields("nextPageToken,permissions(id,type,role,emailAddress,displayName,domain)").
			Context(ctx)
		applyResourceKeyHeader(call.Header(), target)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		listResp, listErr := call.Do()
		if listErr != nil {
			return report, fmt.Errorf("listing permissions for file %q: %w", target.FileID, listErr)
		}

		for _, perm := range listResp.Permissions {
			displayRole := DisplayRoleFromAPI(perm.Role)
			if normalizedFilter != "" && displayRole != normalizedFilter {
				continue
			}
			if len(report.Permissions) >= limit {
				report.Truncated = true
				return report, nil
			}
			report.Permissions = append(report.Permissions, PermissionSummary{
				ID:          perm.Id,
				Type:        perm.Type,
				Role:        displayRole,
				APIRole:     perm.Role,
				Email:       perm.EmailAddress,
				DisplayName: perm.DisplayName,
				Domain:      perm.Domain,
			})
		}

		if listResp.NextPageToken == "" {
			break
		}
		pageToken = listResp.NextPageToken
	}

	return report, nil
}

func validateShareRequest(req ShareRequest) error {
	if strings.TrimSpace(req.Target.FileID) == "" {
		return docurl.ErrEmptyTarget
	}
	if len(req.Emails) == 0 {
		return docurl.ErrNoEmails
	}
	switch req.Role {
	case RoleReader, RoleCommenter, RoleEditor:
		return nil
	default:
		return fmt.Errorf("%w %q: must be reader, commenter, or editor", ErrInvalidRole, req.Role)
	}
}

type existingPermission struct {
	ID      string
	APIRole string
}

func (c *Client) fetchExistingByEmail(ctx context.Context, target docurl.Target) map[string]existingPermission {
	existing := make(map[string]existingPermission)
	pageToken := ""
	for {
		call := c.svc.Permissions.
			List(target.FileID).
			SupportsAllDrives(true).
			PageSize(100).
			Fields("nextPageToken,permissions(id,type,role,emailAddress)").
			Context(ctx)
		applyResourceKeyHeader(call.Header(), target)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil || resp == nil {
			return existing
		}
		for _, perm := range resp.Permissions {
			email := strings.ToLower(strings.TrimSpace(perm.EmailAddress))
			if email == "" {
				continue
			}
			prev, seen := existing[email]
			if !seen || roleRank(perm.Role) > roleRank(prev.APIRole) {
				existing[email] = existingPermission{
					ID:      perm.Id,
					APIRole: perm.Role,
				}
			}
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return existing
}

func roleRank(apiRole string) int {
	switch apiRole {
	case "owner":
		return 5
	case "organizer":
		return 4
	case "fileOrganizer", "writer":
		return 3
	case "commenter":
		return 2
	case "reader":
		return 1
	default:
		return 0
	}
}

func applyResourceKeyHeader(header http.Header, target docurl.Target) {
	if header != nil && target.FileID != "" && target.ResourceKey != "" {
		header.Set("X-Goog-Drive-Resource-Keys", target.FileID+"/"+target.ResourceKey)
	}
}

func wrapDriveFileError(fileID string, err error) error {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case http.StatusNotFound:
			return fmt.Errorf("drive file %q not found (HTTP 404): %w", fileID, err)
		case http.StatusForbidden:
			return fmt.Errorf("access denied for Drive file %q (HTTP 403): %w", fileID, err)
		case http.StatusUnauthorized:
			return fmt.Errorf("drive authentication rejected for file %q (HTTP 401): %w", fileID, err)
		}
	}
	return fmt.Errorf("fetching metadata for Drive file %q: %w", fileID, err)
}

func classifyRecipientError(email string, err error) (string, string) {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		lowerMsg := strings.ToLower(apiErr.Message)
		if apiErr.Code == http.StatusBadRequest && (strings.Contains(lowerMsg, "notify people") || strings.Contains(lowerMsg, "no google account")) {
			return apiErr.Message, fmt.Sprintf(
				"%q is not linked to a Google Account; Google Drive requires sending an email invite for non-Google addresses, and share-with blocks all notifications by design.",
				email,
			)
		}
		if apiErr.Code == http.StatusForbidden {
			return apiErr.Message, "Ensure your authenticated user has Editor/Owner sharing rights and organization sharing policies allow external domains."
		}
		return apiErr.Message, "Verify the recipient email address and Google Workspace sharing policy."
	}
	return err.Error(), "Check network connectivity and recipient address formatting."
}
