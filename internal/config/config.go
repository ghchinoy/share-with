// Package config loads XDG-compliant configuration files and resolves runtime
// settings using strict Flag > Environment > Config > Default precedence.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	// AppName is the canonical directory and binary name for XDG paths.
	AppName = "share-with"

	// DefaultRole is the default permission role granted when unspecified.
	DefaultRole = "reader"

	// DefaultDomain is the default email domain appended to domainless handles.
	DefaultDomain = "google.com"
)

// FileConfig represents the optional YAML configuration stored in
// $XDG_CONFIG_HOME/share-with/config.yaml (or ~/.config/share-with/config.yaml).
type FileConfig struct {
	DefaultRole     string `yaml:"default_role" json:"default_role,omitempty"`
	DefaultDomain   string `yaml:"default_domain" json:"default_domain,omitempty"`
	CredentialsFile string `yaml:"credentials_file" json:"credentials_file,omitempty"`
	TokenFile       string `yaml:"token_file" json:"token_file,omitempty"`
	QuotaProject    string `yaml:"quota_project" json:"quota_project,omitempty"`
	Impersonate     string `yaml:"impersonate" json:"impersonate,omitempty"`
}

// FlagOverrides captures values explicitly set via CLI flags so they take
// precedence over environment variables and config files.
type FlagOverrides struct {
	Role             string
	RoleSet          bool
	DefaultDomain    string
	DefaultDomainSet bool
	CredentialsFile  string
	CredentialsSet   bool
	TokenFile        string
	TokenFileSet     bool
	QuotaProject     string
	QuotaProjectSet  bool
	Impersonate      string
	ImpersonateSet   bool
}

// Resolved holds the effective configuration after merging flags, environment
// variables, the XDG config file, and built-in defaults.
type Resolved struct {
	Role            string `json:"role"`
	DefaultDomain   string `json:"default_domain"`
	CredentialsFile string `json:"credentials_file,omitempty"`
	TokenFile       string `json:"token_file"`
	AccessToken     string `json:"-"`
	QuotaProject    string `json:"quota_project,omitempty"`
	Impersonate     string `json:"impersonate,omitempty"`
	ConfigPath      string `json:"config_path"`
	ConfigLoaded    bool   `json:"config_loaded"`
}

// ConfigDir returns the XDG base configuration directory for share-with,
// preferring $XDG_CONFIG_HOME/share-with and falling back to ~/.config/share-with.
func ConfigDir(getenv func(string) string, homeDir string) string {
	if xdg := strings.TrimSpace(getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, AppName)
	}
	if homeDir != "" {
		return filepath.Join(homeDir, ".config", AppName)
	}
	return filepath.Join(".", ".config", AppName)
}

// LoadFile reads an XDG YAML configuration file if present. Missing files are
// treated as empty configuration without error.
func LoadFile(path string) (FileConfig, bool, error) {
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FileConfig{}, false, nil
		}
		return FileConfig{}, false, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var cfg FileConfig
	if err := yaml.Unmarshal(rawBytes, &cfg); err != nil {
		return FileConfig{}, false, fmt.Errorf("parsing YAML config %q: %w", path, err)
	}
	return cfg, true, nil
}

// Resolve merges CLI flags, environment variables, XDG file settings, and defaults
// in strict Flag > Environment > Config File > Default order.
func Resolve(flags FlagOverrides, getenv func(string) string) (Resolved, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	homeDir, _ := os.UserHomeDir()
	baseDir := ConfigDir(getenv, homeDir)

	configPath := strings.TrimSpace(getenv("SHARE_WITH_CONFIG"))
	if configPath == "" {
		configPath = filepath.Join(baseDir, "config.yaml")
	}

	fileCfg, loaded, err := LoadFile(configPath)
	if err != nil {
		return Resolved{}, err
	}

	defaultTokenPath := filepath.Join(baseDir, "token.json")

	role := resolveValue(
		flags.Role, flags.RoleSet,
		firstEnv(getenv, "SHARE_WITH_ROLE"),
		fileCfg.DefaultRole,
		DefaultRole,
	)

	defaultDomain := resolveValue(
		flags.DefaultDomain, flags.DefaultDomainSet,
		firstEnv(getenv, "SHARE_WITH_DEFAULT_DOMAIN"),
		fileCfg.DefaultDomain,
		DefaultDomain,
	)

	credentialsFile := resolveValue(
		flags.CredentialsFile, flags.CredentialsSet,
		firstEnv(getenv, "SHARE_WITH_CREDENTIALS", "GOOGLE_APPLICATION_CREDENTIALS"),
		fileCfg.CredentialsFile,
		"",
	)

	tokenFile := resolveValue(
		flags.TokenFile, flags.TokenFileSet,
		firstEnv(getenv, "SHARE_WITH_TOKEN_FILE"),
		fileCfg.TokenFile,
		defaultTokenPath,
	)

	quotaProject := resolveValue(
		flags.QuotaProject, flags.QuotaProjectSet,
		firstEnv(getenv, "SHARE_WITH_QUOTA_PROJECT", "GOOGLE_CLOUD_QUOTA_PROJECT", "GOOGLE_CLOUD_PROJECT", "GCLOUD_PROJECT", "CLOUDSDK_CORE_PROJECT"),
		fileCfg.QuotaProject,
		"",
	)

	impersonate := resolveValue(
		flags.Impersonate, flags.ImpersonateSet,
		firstEnv(getenv, "SHARE_WITH_IMPERSONATE"),
		fileCfg.Impersonate,
		"",
	)

	return Resolved{
		Role:            strings.TrimSpace(role),
		DefaultDomain:   strings.TrimPrefix(strings.ToLower(strings.TrimSpace(defaultDomain)), "@"),
		CredentialsFile: expandHome(strings.TrimSpace(credentialsFile), homeDir),
		TokenFile:       expandHome(strings.TrimSpace(tokenFile), homeDir),
		AccessToken:     strings.TrimSpace(getenv("SHARE_WITH_ACCESS_TOKEN")),
		QuotaProject:    strings.TrimSpace(quotaProject),
		Impersonate:     strings.TrimSpace(impersonate),
		ConfigPath:      configPath,
		ConfigLoaded:    loaded,
	}, nil
}

func resolveValue(flagVal string, flagSet bool, envVal, cfgVal, defaultVal string) string {
	if flagSet {
		return flagVal
	}
	if envVal != "" {
		return envVal
	}
	if strings.TrimSpace(cfgVal) != "" {
		return cfgVal
	}
	return defaultVal
}

func firstEnv(getenv func(string) string, keys ...string) string {
	for _, key := range keys {
		if val := strings.TrimSpace(getenv(key)); val != "" {
			return val
		}
	}
	return ""
}

func expandHome(path, homeDir string) string {
	if path == "" || homeDir == "" {
		return path
	}
	if path == "~" {
		return homeDir
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}
	return path
}
