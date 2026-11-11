// Package docurl parses and validates Google Docs and Google Drive URLs, file IDs,
// and recipient email lists (including domainless corporate shorthands).
package docurl

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
)

// DefaultDomain is the domain appended when a recipient token omits a domain
// (for example, "@alice", "bob@", or "alice").
const DefaultDomain = "google.com"

var (
	// ErrEmptyTarget indicates that no document URL or Drive file ID was provided.
	ErrEmptyTarget = errors.New("document URL or file ID is empty")

	// ErrPublishedURL indicates a published-to-web link (/d/e/2PACX-...) that lacks a Drive file ID.
	ErrPublishedURL = errors.New("published web URL does not contain a shareable Drive file ID")

	// ErrInvalidTarget indicates an unrecognized host, path, or malformed file ID.
	ErrInvalidTarget = errors.New("invalid Google Docs or Google Drive URL")

	// ErrNoEmails indicates that no recipient email addresses were supplied.
	ErrNoEmails = errors.New("no recipient email addresses provided")

	fileIDPattern      = regexp.MustCompile(`^[A-Za-z0-9_-]{10,128}$`)
	rawFileIDPattern   = regexp.MustCompile(`^[A-Za-z0-9_-]{15,128}$`)
	resourceKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,128}$`)
	domainLabelPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
)

// Target represents a resolved Google Drive item extracted from a URL or file ID.
type Target struct {
	FileID       string `json:"file_id"`
	ResourceKey  string `json:"resource_key,omitempty"`
	CanonicalURL string `json:"canonical_url"`
	ResourceKind string `json:"resource_kind"`
}

// Parse extracts a Google Drive file ID and resource kind from a Google Docs URL,
// Google Drive URL, or raw Drive file ID.
func Parse(rawInput string) (Target, error) {
	cleaned := strings.TrimSpace(rawInput)
	if cleaned == "" {
		return Target{}, ErrEmptyTarget
	}
	if hasControlChars(cleaned) {
		return Target{}, fmt.Errorf("%w: input contains control characters", ErrInvalidTarget)
	}

	if !strings.Contains(cleaned, "/") && !strings.Contains(cleaned, ":") && !strings.Contains(cleaned, ".") {
		if !rawFileIDPattern.MatchString(cleaned) {
			return Target{}, fmt.Errorf("%w: %q is not a valid Google Drive file ID", ErrInvalidTarget, cleaned)
		}
		return Target{
			FileID:       cleaned,
			CanonicalURL: fmt.Sprintf("https://drive.google.com/file/d/%s/view", cleaned),
			ResourceKind: "file",
		}, nil
	}

	parsedURL, err := url.Parse(cleaned)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return Target{}, fmt.Errorf("%w: %q", ErrInvalidTarget, cleaned)
	}
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return Target{}, fmt.Errorf("%w: unsupported URL scheme %q", ErrInvalidTarget, parsedURL.Scheme)
	}

	host := strings.ToLower(parsedURL.Hostname())
	if host != "docs.google.com" && host != "drive.google.com" && host != "colab.research.google.com" {
		return Target{}, fmt.Errorf("%w: unsupported host %q (expected docs.google.com, drive.google.com, or colab.research.google.com)", ErrInvalidTarget, host)
	}

	resourceKey, err := extractResourceKey(parsedURL.Query())
	if err != nil {
		return Target{}, err
	}

	segments := splitNonEmptySegments(parsedURL.Path)
	if host == "colab.research.google.com" {
		for i := 0; i < len(segments); i++ {
			if segments[i] == "drive" && i+1 < len(segments) {
				notebookID := segments[i+1]
				if !fileIDPattern.MatchString(notebookID) {
					return Target{}, fmt.Errorf("%w: malformed Colab file ID %q", ErrInvalidTarget, notebookID)
				}
				return Target{
					FileID:       notebookID,
					ResourceKey:  resourceKey,
					CanonicalURL: fmt.Sprintf("https://colab.research.google.com/drive/%s", notebookID),
					ResourceKind: "colab",
				}, nil
			}
		}
		return Target{}, fmt.Errorf("%w: could not locate Colab Drive file ID in %q", ErrInvalidTarget, cleaned)
	}

	for i := 0; i < len(segments); i++ {
		seg := segments[i]
		if seg == "d" && i+1 < len(segments) {
			next := segments[i+1]
			if next == "e" || strings.HasPrefix(next, "2PACX-") {
				return Target{}, fmt.Errorf("%w: %q", ErrPublishedURL, cleaned)
			}
			if !fileIDPattern.MatchString(next) {
				return Target{}, fmt.Errorf("%w: malformed file ID %q", ErrInvalidTarget, next)
			}
			kind := detectKindFromSegments(segments[:i])
			return Target{
				FileID:       next,
				ResourceKey:  resourceKey,
				CanonicalURL: canonicalURLForKind(kind, next),
				ResourceKind: kind,
			}, nil
		}
		if seg == "folders" && i+1 < len(segments) {
			folderID := segments[i+1]
			if !fileIDPattern.MatchString(folderID) {
				return Target{}, fmt.Errorf("%w: malformed folder ID %q", ErrInvalidTarget, folderID)
			}
			return Target{
				FileID:       folderID,
				ResourceKey:  resourceKey,
				CanonicalURL: fmt.Sprintf("https://drive.google.com/drive/folders/%s", folderID),
				ResourceKind: "folder",
			}, nil
		}
	}

	if queryID := strings.TrimSpace(parsedURL.Query().Get("id")); queryID != "" {
		if strings.HasPrefix(queryID, "2PACX-") {
			return Target{}, fmt.Errorf("%w: %q", ErrPublishedURL, cleaned)
		}
		if !fileIDPattern.MatchString(queryID) {
			return Target{}, fmt.Errorf("%w: malformed query id %q", ErrInvalidTarget, queryID)
		}
		return Target{
			FileID:       queryID,
			ResourceKey:  resourceKey,
			CanonicalURL: fmt.Sprintf("https://drive.google.com/file/d/%s/view", queryID),
			ResourceKind: "file",
		}, nil
	}

	return Target{}, fmt.Errorf("%w: could not locate file ID in %q", ErrInvalidTarget, cleaned)
}

func extractResourceKey(query url.Values) (string, error) {
	for key, vals := range query {
		if strings.EqualFold(key, "resourcekey") && len(vals) > 0 {
			rk := strings.TrimSpace(vals[0])
			if rk == "" {
				return "", nil
			}
			if !resourceKeyPattern.MatchString(rk) {
				return "", fmt.Errorf("%w: malformed resourcekey %q", ErrInvalidTarget, rk)
			}
			return rk, nil
		}
	}
	return "", nil
}

// ParseEmails validates, normalizes, and deduplicates email addresses using DefaultDomain ("google.com")
// for shorthand entries that omit a domain (such as "@alice", "bob@", or "alice").
func ParseEmails(rawInputs []string) ([]string, error) {
	return ParseEmailsWithDomain(rawInputs, DefaultDomain)
}

// ParseEmailsWithDomain validates, normalizes, and deduplicates email addresses from CLI
// arguments or flag values, expanding domainless handles with defaultDomain (defaulting to
// DefaultDomain when empty).
func ParseEmailsWithDomain(rawInputs []string, defaultDomain string) ([]string, error) {
	resolvedDomain, err := normalizeDefaultDomain(defaultDomain)
	if err != nil {
		return nil, err
	}

	var normalized []string
	seen := make(map[string]bool)

	for _, rawEntry := range rawInputs {
		if hasControlChars(rawEntry) {
			return nil, fmt.Errorf("invalid email input %q: contains ASCII control characters", rawEntry)
		}
		tokens := splitEmailTokens(rawEntry)
		for _, token := range tokens {
			email, parseErr := parseSingleEmail(token, resolvedDomain)
			if parseErr != nil {
				return nil, parseErr
			}
			if !seen[email] {
				seen[email] = true
				normalized = append(normalized, email)
			}
		}
	}

	if len(normalized) == 0 {
		return nil, ErrNoEmails
	}
	return normalized, nil
}

// ReadEmailsFromReader reads email addresses from a text stream using DefaultDomain ("google.com")
// for shorthand entries, ignoring blank lines and '#' line comments.
func ReadEmailsFromReader(r io.Reader) ([]string, error) {
	return ReadEmailsFromReaderWithDomain(r, DefaultDomain)
}

// ReadEmailsFromReaderWithDomain reads email addresses from a text stream, expanding
// domainless entries with defaultDomain while ignoring blank lines and '#' line comments.
func ReadEmailsFromReaderWithDomain(r io.Reader, defaultDomain string) ([]string, error) {
	var rawLines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if commentPrefix, _, hasComment := strings.Cut(line, "#"); hasComment {
			line = commentPrefix
		}
		line = strings.ReplaceAll(line, "\t", " ")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		rawLines = append(rawLines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading emails stream: %w", err)
	}
	return ParseEmailsWithDomain(rawLines, defaultDomain)
}

func normalizeDefaultDomain(rawDomain string) (string, error) {
	cleaned := strings.ToLower(strings.TrimSpace(rawDomain))
	cleaned = strings.TrimPrefix(cleaned, "@")
	if cleaned == "" {
		cleaned = DefaultDomain
	}
	if !isValidDomain(cleaned) {
		return "", fmt.Errorf("invalid default domain %q: must be a valid DNS domain such as %q", rawDomain, DefaultDomain)
	}
	return cleaned, nil
}

func parseSingleEmail(token, defaultDomain string) (string, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return "", errors.New("empty email address")
	}

	// Support RFC 5322 display-name syntax ("Doe, Jane" <jdoe@>) by normalizing the
	// bracketed address first and then verifying the full expression via net/mail.
	if strings.Contains(trimmed, "<") || strings.Contains(trimmed, ">") {
		openIdx := strings.LastIndex(trimmed, "<")
		if openIdx < 0 || !strings.HasSuffix(trimmed, ">") {
			return "", fmt.Errorf("invalid email address %q: unmatched angle brackets", trimmed)
		}
		innerRaw := strings.TrimSpace(trimmed[openIdx+1 : len(trimmed)-1])
		normalizedInner, err := expandShorthandOrAddress(innerRaw, defaultDomain)
		if err != nil {
			return "", fmt.Errorf("invalid email address %q: %w", trimmed, err)
		}
		reconstructed := trimmed[:openIdx+1] + normalizedInner + ">"
		parsed, err := mail.ParseAddress(reconstructed)
		if err != nil {
			return "", fmt.Errorf("invalid email address %q: %w", trimmed, err)
		}
		return strings.ToLower(strings.TrimSpace(parsed.Address)), nil
	}

	normalized, err := expandShorthandOrAddress(trimmed, defaultDomain)
	if err != nil {
		return "", fmt.Errorf("invalid email address %q: %w", trimmed, err)
	}
	return normalized, nil
}

// expandShorthandOrAddress resolves "@handle", "handle@", "handle", "@user@domain.com",
// and standard "user@domain.com" inputs into a validated lowercase email address.
func expandShorthandOrAddress(raw, defaultDomain string) (string, error) {
	if raw == "" {
		return "", errors.New("empty address")
	}
	if strings.ContainsAny(raw, " \t\r\n") {
		return "", errors.New("contains whitespace")
	}
	if strings.Contains(raw, "://") || strings.Contains(raw, "/") {
		return "", errors.New("looks like a URL or path rather than an email or handle")
	}

	candidate := raw

	switch {
	case !strings.Contains(candidate, "@"):
		// Bare handle such as "alice" or "doc-reviewers".
		if !isValidShorthandLocalPart(candidate) {
			return "", errors.New("invalid username or group handle")
		}
		return strings.ToLower(candidate) + "@" + defaultDomain, nil

	case strings.HasPrefix(candidate, "@"):
		rest := candidate[1:]
		if rest == "" || strings.HasPrefix(rest, "@") || strings.HasSuffix(rest, "@") {
			return "", errors.New("malformed '@' handle")
		}
		if !strings.Contains(rest, "@") {
			// Leading-at handle such as "@alice".
			if !isValidShorthandLocalPart(rest) {
				return "", errors.New("invalid '@handle' username")
			}
			return strings.ToLower(rest) + "@" + defaultDomain, nil
		}
		// Leading-at mention of a full email such as "@team-leads@google.com".
		candidate = rest

	case strings.HasSuffix(candidate, "@") && strings.Count(candidate, "@") == 1:
		// Trailing-at handle such as "doc-reviewers@" or "bob@".
		localPart := candidate[:len(candidate)-1]
		if !isValidShorthandLocalPart(localPart) {
			return "", errors.New("invalid 'user@' handle")
		}
		return strings.ToLower(localPart) + "@" + defaultDomain, nil
	}

	parsed, err := mail.ParseAddress(candidate)
	if err != nil {
		return "", err
	}
	address := strings.ToLower(strings.TrimSpace(parsed.Address))
	localPart, domainPart, hasAt := strings.Cut(address, "@")
	if !hasAt || localPart == "" || !isValidDomain(domainPart) {
		return "", errors.New("must include a valid domain")
	}
	return address, nil
}

func isValidShorthandLocalPart(local string) bool {
	if len(local) == 0 || len(local) > 64 {
		return false
	}
	if strings.HasPrefix(local, ".") || strings.HasSuffix(local, ".") || strings.Contains(local, "..") {
		return false
	}
	hasAlphaNum := false
	for _, ch := range local {
		switch {
		case (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9'):
			hasAlphaNum = true
		case ch == '.' || ch == '_' || ch == '-' || ch == '+':
		default:
			return false
		}
	}
	return hasAlphaNum
}

func isValidDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 || !strings.Contains(domain, ".") {
		return false
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || !domainLabelPattern.MatchString(label) {
			return false
		}
	}
	return true
}

// splitEmailTokens splits a string on commas and semicolons outside quoted strings or angle brackets.
func splitEmailTokens(raw string) []string {
	var tokens []string
	var current strings.Builder
	inQuotes := false
	angleDepth := 0

	for _, ch := range raw {
		switch ch {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(ch)
		case '<':
			if !inQuotes {
				angleDepth++
			}
			current.WriteRune(ch)
		case '>':
			if !inQuotes && angleDepth > 0 {
				angleDepth--
			}
			current.WriteRune(ch)
		case ',', ';':
			if !inQuotes && angleDepth == 0 {
				if tok := strings.TrimSpace(current.String()); tok != "" {
					tokens = append(tokens, tok)
				}
				current.Reset()
				continue
			}
			current.WriteRune(ch)
		default:
			current.WriteRune(ch)
		}
	}
	if tok := strings.TrimSpace(current.String()); tok != "" {
		tokens = append(tokens, tok)
	}
	return tokens
}

func hasControlChars(s string) bool {
	for _, ch := range s {
		if ch <= 0x1F || ch == 0x7F {
			return true
		}
	}
	return false
}

func splitNonEmptySegments(path string) []string {
	rawSegments := strings.Split(path, "/")
	var segments []string
	for _, seg := range rawSegments {
		if seg != "" {
			segments = append(segments, seg)
		}
	}
	return segments
}

func detectKindFromSegments(prefix []string) string {
	for _, seg := range prefix {
		switch seg {
		case "document":
			return "document"
		case "spreadsheets":
			return "spreadsheet"
		case "presentation":
			return "presentation"
		case "forms":
			return "form"
		case "drawings":
			return "drawing"
		case "file":
			return "file"
		}
	}
	return "file"
}

func canonicalURLForKind(kind, fileID string) string {
	switch kind {
	case "document":
		return fmt.Sprintf("https://docs.google.com/document/d/%s/edit", fileID)
	case "spreadsheet":
		return fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/edit", fileID)
	case "presentation":
		return fmt.Sprintf("https://docs.google.com/presentation/d/%s/edit", fileID)
	case "form":
		return fmt.Sprintf("https://docs.google.com/forms/d/%s/edit", fileID)
	case "drawing":
		return fmt.Sprintf("https://docs.google.com/drawings/d/%s/edit", fileID)
	default:
		return fmt.Sprintf("https://drive.google.com/file/d/%s/view", fileID)
	}
}
