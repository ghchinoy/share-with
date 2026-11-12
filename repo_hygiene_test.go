package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ghchinoy/share-with/cmd"
)

func TestAgentPluginAndPortableURLHygiene(t *testing.T) {
	t.Run("legacy_flat_skills_dir_removed", func(t *testing.T) {
		if _, err := os.Stat("skills"); !os.IsNotExist(err) {
			t.Fatalf("legacy top-level 'skills/' directory must not exist once migrated to 'plugins/share-with/skills/'")
		}
	})

	t.Run("plugin_manifest_v1_conformance", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join("plugins", "share-with", "plugin.json"))
		if err != nil {
			t.Fatalf("reading plugins/share-with/plugin.json: %v", err)
		}

		var manifest map[string]any
		if err := json.Unmarshal(raw, &manifest); err != nil {
			t.Fatalf("parsing plugins/share-with/plugin.json: %v", err)
		}

		allowed := map[string]bool{
			"$schema":     true,
			"name":        true,
			"version":     true,
			"description": true,
			"author":      true,
			"homepage":    true,
			"repository":  true,
			"license":     true,
			"keywords":    true,
			"extensions":  true,
		}
		for key := range manifest {
			if !allowed[key] {
				t.Errorf("plugin.json contains disallowed top-level field %q", key)
			}
		}

		const wantSchema = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
		if got, _ := manifest["$schema"].(string); got != wantSchema {
			t.Errorf("plugin.json $schema = %q, want %q", got, wantSchema)
		}
		if got, _ := manifest["name"].(string); got != "share-with" {
			t.Errorf("plugin.json name = %q, want %q", got, "share-with")
		}
	})

	t.Run("release_manifest_version_sync", func(t *testing.T) {
		rpBytes, err := os.ReadFile(".release-please-manifest.json")
		if err != nil {
			t.Fatalf("reading .release-please-manifest.json: %v", err)
		}
		var rp map[string]string
		if err := json.Unmarshal(rpBytes, &rp); err != nil {
			t.Fatalf("parsing .release-please-manifest.json: %v", err)
		}
		rootVer := rp["."]
		if rootVer == "" {
			t.Fatalf(".release-please-manifest.json missing '.' version entry")
		}

		pluginBytes, err := os.ReadFile(filepath.Join("plugins", "share-with", "plugin.json"))
		if err != nil {
			t.Fatalf("reading plugins/share-with/plugin.json: %v", err)
		}
		var pluginManifest struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(pluginBytes, &pluginManifest); err != nil {
			t.Fatalf("parsing plugins/share-with/plugin.json: %v", err)
		}

		if pluginManifest.Version != rootVer {
			t.Errorf("plugin.json version = %q, want %q (matching .release-please-manifest.json)", pluginManifest.Version, rootVer)
		}
		if cmd.Version != rootVer {
			t.Errorf("cmd.Version = %q, want %q (matching .release-please-manifest.json)", cmd.Version, rootVer)
		}
	})

	t.Run("marketplace_manifest_and_skill_frontmatter", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join(".claude-plugin", "marketplace.json"))
		if err != nil {
			t.Fatalf("reading .claude-plugin/marketplace.json: %v", err)
		}

		var mp struct {
			Name    string `json:"name"`
			Plugins []struct {
				Name   string   `json:"name"`
				Source string   `json:"source"`
				Skills []string `json:"skills"`
			} `json:"plugins"`
		}
		if err := json.Unmarshal(raw, &mp); err != nil {
			t.Fatalf("parsing .claude-plugin/marketplace.json: %v", err)
		}
		if len(mp.Plugins) == 0 {
			t.Fatalf(".claude-plugin/marketplace.json must declare at least one plugin")
		}

		for _, p := range mp.Plugins {
			if _, err := os.Stat(filepath.Join(p.Source, "plugin.json")); err != nil {
				t.Errorf("marketplace plugin %q source %q missing plugin.json: %v", p.Name, p.Source, err)
			}
			for _, skillDir := range p.Skills {
				skillMDPath := filepath.Join(skillDir, "SKILL.md")
				contentBytes, err := os.ReadFile(skillMDPath)
				if err != nil {
					t.Errorf("marketplace skill %q missing SKILL.md: %v", skillDir, err)
					continue
				}
				verifySkillFrontmatter(t, skillMDPath, filepath.Base(skillDir), string(contentBytes))
			}
		}
	})

	t.Run("zero_machine_local_paths_and_em_dashes", func(t *testing.T) {
		localPathRE := regexp.MustCompile(`(/Users/[A-Za-z0-9._-]+|/home/[A-Za-z0-9._-]+|~/projects/|[A-Za-z]:\\Users\\)`)
		skipDirs := map[string]bool{
			".git":         true,
			".beads":       true,
			"bin":          true,
			"dist":         true,
			"node_modules": true,
		}
		checkExts := map[string]bool{
			".md":   true,
			".json": true,
			".yaml": true,
			".yml":  true,
		}

		err := filepath.WalkDir(".", func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			ext := filepath.Ext(d.Name())
			if !checkExts[ext] && d.Name() != "Makefile" {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(raw)
			if match := localPathRE.FindString(text); match != "" {
				t.Errorf("%s contains hardcoded machine-local path %q; use portable https://github.com/ghchinoy/agent-skills URLs instead", path, match)
			}
			if ext == ".md" && strings.Contains(text, "—") {
				t.Errorf("%s contains an em dash ('—'); house style requires commas, colons, or parentheses", path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking repository: %v", err)
		}
	})
}

func verifySkillFrontmatter(t *testing.T, path, expectedName, content string) {
	t.Helper()
	if !strings.HasPrefix(content, "---\n") {
		t.Fatalf("%s must start with YAML frontmatter delimiter '---'", path)
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		t.Fatalf("%s has malformed YAML frontmatter delimiters", path)
	}
	fm := parts[1]
	fields := make(map[string]string)
	for _, line := range strings.Split(fm, "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || strings.TrimSpace(line) == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val := strings.TrimSpace(v)
		val = strings.Trim(val, `"'`)
		fields[strings.TrimSpace(k)] = val
	}

	if fields["name"] != expectedName {
		t.Errorf("%s frontmatter name = %q, want %q", path, fields["name"], expectedName)
	}
	desc := fields["description"]
	if len(desc) == 0 || len(desc) > 1024 {
		t.Errorf("%s frontmatter description length = %d, want 1..1024", path, len(desc))
	}
	if fields["license"] == "" {
		t.Errorf("%s frontmatter missing license field", path)
	}
	if _, hasTopVersion := fields["version"]; hasTopVersion {
		t.Errorf("%s frontmatter has top-level version field; nest under metadata.version", path)
	}
}
