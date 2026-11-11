// auth.go resolves headless-safe Google Drive OAuth2, ADC, and Service Account credentials.

package sharer

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"

	"github.com/ghchinoy/share-with/internal/config"
)

var (
	// ErrNoCredentials indicates no usable headless credentials could be found.
	ErrNoCredentials = errors.New("no Google Drive credentials found")

	// ErrInteractiveLoginRequired indicates a desktop OAuth client file exists without a cached token.
	ErrInteractiveLoginRequired = errors.New("desktop OAuth client requires a cached token before headless execution")
)

// AuthStatus summarizes the active authentication source without triggering
// interactive browser flows.
type AuthStatus struct {
	Source          string `json:"source"`
	Ready           bool   `json:"ready"`
	CredentialsFile string `json:"credentials_file,omitempty"`
	TokenFile       string `json:"token_file,omitempty"`
	QuotaProject    string `json:"quota_project,omitempty"`
	Impersonate     string `json:"impersonate,omitempty"`
	Detail          string `json:"detail"`
	Hint            string `json:"hint,omitempty"`
}

// InspectAuth evaluates the credential chain in headless mode and reports whether
// credentials are ready for non-interactive Drive API calls.
func InspectAuth(ctx context.Context, cfg config.Resolved) AuthStatus {
	status := AuthStatus{
		CredentialsFile: cfg.CredentialsFile,
		TokenFile:       cfg.TokenFile,
		QuotaProject:    cfg.QuotaProject,
		Impersonate:     cfg.Impersonate,
	}

	if cfg.AccessToken != "" {
		status.Source = "env:SHARE_WITH_ACCESS_TOKEN"
		status.Ready = true
		status.Detail = "Using bearer access token from SHARE_WITH_ACCESS_TOKEN"
		return status
	}

	if cfg.CredentialsFile != "" {
		rawJSON, err := os.ReadFile(cfg.CredentialsFile)
		if err != nil {
			status.Source = "credentials_file"
			status.Ready = false
			status.Detail = fmt.Sprintf("reading %q: %v", cfg.CredentialsFile, err)
			status.Hint = fmt.Sprintf("Verify that %q exists and contains valid Google JSON credentials.", cfg.CredentialsFile)
			return status
		}
		kind, err := classifyCredentialBytes(rawJSON)
		if err != nil {
			status.Source = "credentials_file"
			status.Ready = false
			status.Detail = err.Error()
			status.Hint = fmt.Sprintf("Verify that %q exists and contains valid Google JSON credentials.", cfg.CredentialsFile)
			return status
		}
		status.Source = "credentials_file:" + kind
		if status.QuotaProject == "" {
			status.QuotaProject = extractJSONQuotaProject(rawJSON)
		}
		if kind == "oauth_client" {
			if _, statErr := os.Stat(cfg.TokenFile); statErr != nil {
				status.Ready = false
				status.Detail = fmt.Sprintf("OAuth client %q found, but cached token %q does not exist", cfg.CredentialsFile, cfg.TokenFile)
				status.Hint = fmt.Sprintf("Run 'share-with auth login --credentials %s' once in an interactive terminal.", cfg.CredentialsFile)
				return status
			}
			status.Ready = true
			status.Detail = fmt.Sprintf("Using OAuth client %q with cached token %q", cfg.CredentialsFile, cfg.TokenFile)
			return status
		}
		status.Ready = true
		status.Detail = fmt.Sprintf("Using %s credentials from %q", kind, cfg.CredentialsFile)
		return status
	}

	creds, err := google.FindDefaultCredentialsWithParams(ctx, google.CredentialsParams{
		Scopes:  []string{drive.DriveScope},
		Subject: cfg.Impersonate,
	})
	if err != nil {
		status.Source = "adc"
		status.Ready = false
		status.Detail = "Application Default Credentials (ADC) not configured"
		status.Hint = "Run 'gcloud auth application-default login --scopes=https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/cloud-platform' or set SHARE_WITH_CREDENTIALS."
		return status
	}

	status.Source = "adc"
	status.Ready = true
	if status.QuotaProject == "" {
		status.QuotaProject = resolveADCQuotaProject(creds, os.Getenv)
	}
	status.Detail = "Using Google Application Default Credentials (ADC)"
	if status.QuotaProject == "" {
		status.Hint = "ADC has no quota project configured; Drive API requires one. Run 'gcloud auth application-default set-quota-project <PROJECT_ID>' or pass '--quota-project <PROJECT_ID>'."
	}
	return status
}

// BuildDriveService constructs a Google Drive v3 client wrapped with SilentGuardTransport.
// It never opens a browser or blocks on interactive OAuth prompts.
func BuildDriveService(ctx context.Context, cfg config.Resolved) (*drive.Service, error) {
	tokenSource, quotaProject, err := resolveTokenSource(ctx, cfg)
	if err != nil {
		return nil, err
	}

	oauthClient := oauth2.NewClient(ctx, tokenSource)
	guardedClient := &http.Client{
		Transport: NewSilentGuardTransport(oauthClient.Transport, quotaProject),
	}

	svc, err := drive.NewService(ctx, option.WithHTTPClient(guardedClient))
	if err != nil {
		return nil, fmt.Errorf("initializing Google Drive v3 service: %w", err)
	}
	return svc, nil
}

// RunInteractiveLogin executes a local loopback OAuth2 PKCE flow for a desktop OAuth
// client JSON file and writes the resulting token to tokenPath with 0600 permissions.
func RunInteractiveLogin(ctx context.Context, credentialsPath, tokenPath string, out io.Writer) error {
	if strings.TrimSpace(credentialsPath) == "" {
		return fmt.Errorf("%w: --credentials path to a Desktop OAuth client JSON is required", ErrNoCredentials)
	}
	rawJSON, err := os.ReadFile(credentialsPath)
	if err != nil {
		return fmt.Errorf("reading OAuth client file %q: %w", credentialsPath, err)
	}

	oauthCfg, err := google.ConfigFromJSON(rawJSON, drive.DriveScope)
	if err != nil {
		return fmt.Errorf("parsing OAuth client file %q: %w", credentialsPath, err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("binding local loopback OAuth callback listener: %w", err)
	}
	defer listener.Close()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return errors.New("unexpected listener address type")
	}
	oauthCfg.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d/callback", tcpAddr.Port)

	stateToken, err := generateRandomHex(16)
	if err != nil {
		return fmt.Errorf("generating OAuth state nonce: %w", err)
	}
	verifier := oauth2.GenerateVerifier()

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != stateToken {
			http.Error(w, "invalid OAuth state parameter", http.StatusBadRequest)
			select {
			case errCh <- errors.New("OAuth callback state mismatch"):
			default:
			}
			return
		}
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			http.Error(w, "authorization denied: "+errParam, http.StatusBadRequest)
			select {
			case errCh <- fmt.Errorf("OAuth authorization failed: %s", errParam):
			default:
			}
			return
		}
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			http.Error(w, "missing authorization code", http.StatusBadRequest)
			select {
			case errCh <- errors.New("OAuth callback missing code parameter"):
			default:
			}
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintln(w, "share-with authorization complete. You may close this browser tab.")
		select {
		case codeCh <- code:
		default:
		}
	})

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if serveErr := srv.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			select {
			case errCh <- serveErr:
			default:
			}
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	authURL := oauthCfg.AuthCodeURL(
		stateToken,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
		oauth2.S256ChallengeOption(verifier),
	)
	_, _ = fmt.Fprintf(out, "Open the following URL in your browser to authorize share-with:\n\n  %s\n\nWaiting for callback on %s...\n", authURL, oauthCfg.RedirectURL)

	var authCode string
	select {
	case <-ctx.Done():
		return fmt.Errorf("waiting for OAuth authorization callback: %w", ctx.Err())
	case cbErr := <-errCh:
		return cbErr
	case authCode = <-codeCh:
	}

	token, err := oauthCfg.Exchange(ctx, authCode, oauth2.VerifierOption(verifier))
	if err != nil {
		return fmt.Errorf("exchanging authorization code for token: %w", err)
	}

	if err := saveTokenAtomically(tokenPath, token); err != nil {
		return err
	}
	return nil
}

func resolveTokenSource(ctx context.Context, cfg config.Resolved) (oauth2.TokenSource, string, error) {
	quotaProject := cfg.QuotaProject

	if cfg.AccessToken != "" {
		staticTS := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.AccessToken})
		return staticTS, quotaProject, nil
	}

	if cfg.CredentialsFile != "" {
		rawJSON, err := os.ReadFile(cfg.CredentialsFile)
		if err != nil {
			return nil, "", fmt.Errorf("reading credentials file %q: %w", cfg.CredentialsFile, err)
		}
		kind, err := classifyCredentialBytes(rawJSON)
		if err != nil {
			return nil, "", fmt.Errorf("inspecting credentials file %q: %w", cfg.CredentialsFile, err)
		}

		params := google.CredentialsParams{
			Scopes:  []string{drive.DriveScope},
			Subject: cfg.Impersonate,
		}

		switch kind {
		case "service_account":
			creds, credErr := google.CredentialsFromJSONWithTypeAndParams(ctx, rawJSON, google.ServiceAccount, params)
			if credErr != nil {
				return nil, "", fmt.Errorf("loading service account credentials %q: %w", cfg.CredentialsFile, credErr)
			}
			if quotaProject == "" {
				quotaProject = extractCredsQuotaProject(creds)
			}
			return creds.TokenSource, quotaProject, nil

		case "authorized_user":
			creds, credErr := google.CredentialsFromJSONWithTypeAndParams(ctx, rawJSON, google.AuthorizedUser, params)
			if credErr != nil {
				return nil, "", fmt.Errorf("loading authorized user credentials %q: %w", cfg.CredentialsFile, credErr)
			}
			if quotaProject == "" {
				quotaProject = resolveADCQuotaProject(creds, os.Getenv)
			}
			return creds.TokenSource, quotaProject, nil

		case "oauth_client":
			oauthCfg, cfgErr := google.ConfigFromJSON(rawJSON, drive.DriveScope)
			if cfgErr != nil {
				return nil, "", fmt.Errorf("parsing OAuth client %q: %w", cfg.CredentialsFile, cfgErr)
			}
			tokBytes, tokErr := os.ReadFile(cfg.TokenFile)
			if tokErr != nil {
				if errors.Is(tokErr, os.ErrNotExist) {
					return nil, "", fmt.Errorf("%w: missing token file %q for OAuth client %q", ErrInteractiveLoginRequired, cfg.TokenFile, cfg.CredentialsFile)
				}
				return nil, "", fmt.Errorf("reading cached OAuth token %q: %w", cfg.TokenFile, tokErr)
			}
			var tok oauth2.Token
			if unmarshalErr := json.Unmarshal(tokBytes, &tok); unmarshalErr != nil {
				return nil, "", fmt.Errorf("parsing cached OAuth token %q: %w", cfg.TokenFile, unmarshalErr)
			}
			return oauthCfg.TokenSource(ctx, &tok), quotaProject, nil

		default:
			creds, credErr := google.CredentialsFromJSONWithParams(ctx, rawJSON, params)
			if credErr != nil {
				return nil, "", fmt.Errorf("loading credentials %q: %w", cfg.CredentialsFile, credErr)
			}
			if quotaProject == "" {
				quotaProject = extractCredsQuotaProject(creds)
			}
			return creds.TokenSource, quotaProject, nil
		}
	}

	creds, err := google.FindDefaultCredentialsWithParams(ctx, google.CredentialsParams{
		Scopes:  []string{drive.DriveScope},
		Subject: cfg.Impersonate,
	})
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrNoCredentials, err)
	}
	if quotaProject == "" {
		quotaProject = resolveADCQuotaProject(creds, os.Getenv)
	}
	return creds.TokenSource, quotaProject, nil
}

// resolveADCQuotaProject extracts quota_project_id (or project_id) from creds.JSON
// because golang.org/x/oauth2/google only populates Credentials.ProjectID for
// service_account files and leaves it empty on gcloud authorized_user ADC files.
// When ADC JSON omits quota_project_id (common after 'gcloud auth application-default login --scopes=...'),
// it falls back to the active gcloud CLI configuration (~/.config/gcloud/configurations/config_<active>).
func resolveADCQuotaProject(creds *google.Credentials, getenv func(string) string) string {
	if proj := extractCredsQuotaProject(creds); proj != "" {
		return proj
	}
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return ""
	}
	return readGcloudActiveProject(getenv, homeDir)
}

func extractCredsQuotaProject(creds *google.Credentials) string {
	if creds == nil {
		return ""
	}
	if proj := extractJSONQuotaProject(creds.JSON); proj != "" {
		return proj
	}
	return strings.TrimSpace(creds.ProjectID)
}

func extractJSONQuotaProject(rawJSON []byte) string {
	if len(rawJSON) == 0 {
		return ""
	}
	var fields struct {
		QuotaProjectID string `json:"quota_project_id"`
		ProjectID      string `json:"project_id"`
	}
	if err := json.Unmarshal(rawJSON, &fields); err != nil {
		return ""
	}
	if qp := strings.TrimSpace(fields.QuotaProjectID); qp != "" {
		return qp
	}
	return strings.TrimSpace(fields.ProjectID)
}

func readGcloudActiveProject(getenv func(string) string, homeDir string) string {
	if getenv == nil {
		getenv = os.Getenv
	}
	gcloudDir := strings.TrimSpace(getenv("CLOUDSDK_CONFIG"))
	if gcloudDir == "" {
		gcloudDir = filepath.Join(homeDir, ".config", "gcloud")
	}

	activeName := strings.TrimSpace(getenv("CLOUDSDK_ACTIVE_CONFIG_NAME"))
	if activeName == "" {
		if rawActive, err := os.ReadFile(filepath.Join(gcloudDir, "active_config")); err == nil {
			activeName = strings.TrimSpace(string(rawActive))
		}
	}
	if activeName == "" {
		activeName = "default"
	}

	cfgFile, err := os.Open(filepath.Join(gcloudDir, "configurations", "config_"+activeName))
	if err != nil {
		return ""
	}
	defer cfgFile.Close()

	var coreProject, billingQuotaProject string
	section := ""
	scanner := bufio.NewScanner(cfgFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		if section == "billing" && key == "quota_project" && val != "" {
			billingQuotaProject = val
		}
		if section == "core" && key == "project" && val != "" {
			coreProject = val
		}
	}
	if billingQuotaProject != "" {
		return billingQuotaProject
	}
	return coreProject
}

func classifyCredentialBytes(rawJSON []byte) (string, error) {
	var probe struct {
		Type      string          `json:"type"`
		Installed json.RawMessage `json:"installed"`
		Web       json.RawMessage `json:"web"`
	}
	if err := json.Unmarshal(rawJSON, &probe); err != nil {
		return "", fmt.Errorf("invalid JSON credentials: %w", err)
	}
	if probe.Type != "" {
		return probe.Type, nil
	}
	if len(probe.Installed) > 0 || len(probe.Web) > 0 {
		return "oauth_client", nil
	}
	return "", errors.New("unrecognized Google credential JSON structure")
}

func saveTokenAtomically(tokenPath string, token *oauth2.Token) error {
	dir := filepath.Dir(tokenPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating token directory %q: %w", dir, err)
	}
	encoded, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding OAuth token: %w", err)
	}
	tmpPath := tokenPath + ".tmp"
	if err := os.WriteFile(tmpPath, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("writing temporary token file %q: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, tokenPath); err != nil {
		return fmt.Errorf("renaming token file to %q: %w", tokenPath, err)
	}
	return nil
}

func generateRandomHex(numBytes int) (string, error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
