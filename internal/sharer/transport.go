// transport.go enforces silent-only permission creation at the HTTP RoundTripper layer.

package sharer

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrNotificationBlocked is returned when an outgoing HTTP request attempts to
// create a Google Drive permission without sendNotificationEmail=false.
var ErrNotificationBlocked = errors.New("silent-sharing guard blocked outgoing request that could send a notification")

// SilentGuardTransport wraps an http.RoundTripper and guarantees at the wire
// level that every permission-creation POST explicitly sets sendNotificationEmail=false
// and omits emailMessage and transferOwnership=true.
type SilentGuardTransport struct {
	base         http.RoundTripper
	quotaProject string
}

// NewSilentGuardTransport wraps base (or http.DefaultTransport if nil) with
// strict silent-sharing enforcement and optional quota-project metadata.
func NewSilentGuardTransport(base http.RoundTripper, quotaProject string) *SilentGuardTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &SilentGuardTransport{
		base:         base,
		quotaProject: strings.TrimSpace(quotaProject),
	}
}

// RoundTrip validates outgoing Google Drive permission mutations before delegating
// to the underlying RoundTripper.
func (t *SilentGuardTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, errors.New("nil HTTP request or URL")
	}

	if req.Method == http.MethodPost && isPermissionCreatePath(req.URL.Path) {
		query := req.URL.Query()
		notifyVal := query.Get("sendNotificationEmail")
		if notifyVal != "false" {
			return nil, fmt.Errorf("%w: query parameter sendNotificationEmail=%q, want %q", ErrNotificationBlocked, notifyVal, "false")
		}
		if strings.EqualFold(query.Get("transferOwnership"), "true") {
			return nil, fmt.Errorf("%w: transferOwnership=true forces notification emails", ErrNotificationBlocked)
		}
		if query.Has("emailMessage") {
			return nil, fmt.Errorf("%w: emailMessage parameter is forbidden in silent mode", ErrNotificationBlocked)
		}
	}

	outReq := req
	if t.quotaProject != "" && req.Header.Get("X-Goog-User-Project") == "" {
		outReq = req.Clone(req.Context())
		outReq.Header.Set("X-Goog-User-Project", t.quotaProject)
	}
	return t.base.RoundTrip(outReq)
}

func isPermissionCreatePath(urlPath string) bool {
	trimmed := strings.TrimSuffix(urlPath, "/")
	return strings.HasSuffix(trimmed, "/permissions")
}
