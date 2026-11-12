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

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_PrecedenceOrder(t *testing.T) {
	tmpDir := t.TempDir()
	cfgDir := filepath.Join(tmpDir, "share-with")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("os.MkdirAll(%q) unexpected error: %v", cfgDir, err)
	}

	cfgPath := filepath.Join(cfgDir, "config.yaml")
	yamlContent := `default_role: commenter
credentials_file: /from/config/creds.json
quota_project: cfg-project
impersonate: cfg-admin@example.com
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) unexpected error: %v", cfgPath, err)
	}

	envMap := map[string]string{
		"XDG_CONFIG_HOME":          tmpDir,
		"SHARE_WITH_ROLE":          "editor",
		"SHARE_WITH_QUOTA_PROJECT": "env-project",
		"SHARE_WITH_ACCESS_TOKEN":  "ya29.secret-token",
	}
	getenv := func(key string) string { return envMap[key] }

	// Flag beats Env, Env beats Config, Config beats Default.
	flags := FlagOverrides{
		Role:    "reader",
		RoleSet: true,
	}

	got, err := Resolve(flags, getenv)
	if err != nil {
		t.Fatalf("Resolve(%+v, env) unexpected error: %v", flags, err)
	}

	if got.Role != "reader" {
		t.Errorf("Resolve(%+v, env).Role = %q, want %q (flag over env)", flags, got.Role, "reader")
	}
	if got.QuotaProject != "env-project" {
		t.Errorf("Resolve(%+v, env).QuotaProject = %q, want %q (env over config)", flags, got.QuotaProject, "env-project")
	}
	if got.CredentialsFile != "/from/config/creds.json" {
		t.Errorf("Resolve(%+v, env).CredentialsFile = %q, want %q (config over default)", flags, got.CredentialsFile, "/from/config/creds.json")
	}
	if got.Impersonate != "cfg-admin@example.com" {
		t.Errorf("Resolve(%+v, env).Impersonate = %q, want %q", flags, got.Impersonate, "cfg-admin@example.com")
	}
	if got.AccessToken != "ya29.secret-token" {
		t.Errorf("Resolve(%+v, env).AccessToken = %q, want %q", flags, got.AccessToken, "ya29.secret-token")
	}
	if !got.ConfigLoaded {
		t.Errorf("Resolve(%+v, env).ConfigLoaded = false, want true", flags)
	}
}

func TestResolve_DefaultsWhenMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	getenv := func(key string) string {
		if key == "XDG_CONFIG_HOME" {
			return tmpDir
		}
		return ""
	}

	got, err := Resolve(FlagOverrides{}, getenv)
	if err != nil {
		t.Fatalf("Resolve(empty, env) unexpected error: %v", err)
	}
	if got.Role != DefaultRole {
		t.Errorf("Resolve(empty, env).Role = %q, want %q", got.Role, DefaultRole)
	}
	if got.DefaultDomain != DefaultDomain {
		t.Errorf("Resolve(empty, env).DefaultDomain = %q, want %q", got.DefaultDomain, DefaultDomain)
	}
	if got.ConfigLoaded {
		t.Errorf("Resolve(empty, env).ConfigLoaded = true, want false")
	}
}
