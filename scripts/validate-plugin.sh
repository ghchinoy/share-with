#!/usr/bin/env bash
# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Validates Agent Plugins v1.0.0 conformance, Agent Skills metadata,
# marketplace discovery manifests, and portable URL hygiene (zero machine-local paths).

set -euo pipefail

JSON_MODE=0
REPO_ROOT="."

usage() {
  cat <<'EOF'
Usage: validate-plugin.sh [--json] [--help] [repo_root]

Validates Agent Plugins v1.0.0 conformance, Agent Skills (SKILL.md) metadata,
marketplace discovery manifests (.claude-plugin/marketplace.json), and portable
URL hygiene (zero machine-local paths).

Options:
  --json      Emit structured JSON report ({"passed": bool, "score": int, "errors": [...], "warnings": [...]})
  -h, --help  Show this help message and exit
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --json)
      JSON_MODE=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    -*)
      printf 'Unknown option: %s\n' "$1" >&2
      usage >&2
      exit 1
      ;;
    *)
      REPO_ROOT="$1"
      shift
      ;;
  esac
done

cd "$REPO_ROOT"

ERRORS=0
WARNINGS=0
ERR_LIST=()
WARN_LIST=()

info() {
  if [ "$JSON_MODE" -eq 0 ]; then
    printf '==> %s\n' "$1"
  fi
}

ok() {
  if [ "$JSON_MODE" -eq 0 ]; then
    printf '  [OK] %s\n' "$1"
  fi
}

warn() {
  WARN_LIST+=("$1")
  WARNINGS=$((WARNINGS + 1))
  if [ "$JSON_MODE" -eq 0 ]; then
    printf '  [WARN] %s\n' "$1"
  fi
}

err() {
  ERR_LIST+=("$1")
  ERRORS=$((ERRORS + 1))
  if [ "$JSON_MODE" -eq 0 ]; then
    printf '  [ERR] %s\n' "$1"
  fi
}

info "1. Checking Agent Plugins directory layout and symlink containment"
if [ ! -d "plugins" ]; then
  err "Missing top-level 'plugins/' directory."
else
  ok "Found 'plugins/' directory."
fi

if [ -d "skills" ]; then
  err "Legacy top-level 'skills/' directory still exists; skills belong under 'plugins/<plugin>/skills/'."
fi

ABS_ROOT="$(pwd -P)"
while IFS= read -r symlink; do
  [ -n "$symlink" ] || continue
  if git check-ignore -q "$symlink" 2>/dev/null; then
    continue
  fi
  target="$(readlink "$symlink")"
  sym_dir="$(dirname "$symlink")"
  real_target="$(python3 -c 'import os, sys; print(os.path.realpath(os.path.join(sys.argv[1], sys.argv[2])))' "$sym_dir" "$target" 2>/dev/null || true)"
  if [ -z "$real_target" ] || { [ "$real_target" != "$ABS_ROOT" ] && [ "${real_target#"$ABS_ROOT"/}" = "$real_target" ]; }; then
    err "Symlink '$symlink' escapes repository root ($target)"
  fi
done < <(find . -type l -not -path '*/.git/*' -not -path '*/node_modules/*' 2>/dev/null)

info "2. Validating Plugin Package Manifests (plugin.json)"
for plugin_dir in plugins/*; do
  [ -d "$plugin_dir" ] || continue
  plugin_name="$(basename "$plugin_dir")"
  manifest="$plugin_dir/plugin.json"

  if [ ! -f "$manifest" ]; then
    err "Plugin '$plugin_name' missing plugin.json"
    continue
  fi

  check_out="$(python3 - "$manifest" "$plugin_name" <<'PY'
import json, re, sys

manifest_path, expected_name = sys.argv[1], sys.argv[2]
allowed = {
    "$schema", "name", "version", "description", "author",
    "homepage", "repository", "license", "keywords", "extensions",
}
with open(manifest_path, encoding="utf-8") as fh:
    data = json.load(fh)

errs = []
if data.get("$schema") != "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json":
    errs.append(f"invalid $schema: {data.get('$schema')!r}")
name = data.get("name", "")
if name != expected_name:
    errs.append(f"name {name!r} != directory {expected_name!r}")
if not re.match(r"^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$", name) or len(name) > 64:
    errs.append(f"name {name!r} violates Agent Plugins v1.0.0 naming pattern")
unknown = sorted(set(data.keys()) - allowed)
if unknown:
    errs.append(f"unknown top-level fields: {unknown}")
print("\n".join(errs))
PY
)"
  if [ -n "$check_out" ]; then
    while IFS= read -r line; do
      [ -n "$line" ] && err "Plugin '$plugin_name': $line"
    done <<< "$check_out"
  else
    ok "Plugin '$plugin_name' plugin.json conforms to Agent Plugins v1.0.0"
  fi
done

info "3. Validating Agent Skills (SKILL.md)"
for skill_md in plugins/*/skills/*/SKILL.md; do
  [ -f "$skill_md" ] || continue
  skill_dir="$(dirname "$skill_md")"
  expected_name="$(basename "$skill_dir")"

  skill_out="$(python3 - "$skill_md" "$expected_name" <<'PY'
import re, sys

filepath, expected_name = sys.argv[1], sys.argv[2]
content = open(filepath, encoding="utf-8").read()
errs = []
warns = []
if not content.startswith("---\n"):
    errs.append("missing YAML frontmatter delimiter at start")
else:
    parts = content.split("---", 2)
    if len(parts) < 3:
        errs.append("malformed YAML frontmatter delimiters")
    else:
        fm = parts[1]
        def val(key):
            m = re.search(r"^" + key + r":\s*(.*)$", fm, re.MULTILINE)
            if not m:
                return ""
            v = m.group(1).strip()
            if (v.startswith('"') and v.endswith('"')) or (v.startswith("'") and v.endswith("'")):
                v = v[1:-1]
            return v
        name = val("name")
        desc = val("description")
        lic = val("license")
        top_ver = val("version")
        if name != expected_name:
            errs.append(f"frontmatter name {name!r} != directory {expected_name!r}")
        if not desc:
            errs.append("missing description in frontmatter")
        elif len(desc) > 1024:
            errs.append(f"description exceeds 1024 chars ({len(desc)})")
        if top_ver:
            errs.append("top-level version: field must be nested under metadata:")
        if not lic:
            warns.append("missing license in frontmatter")
for e in errs:
    print("ERR:" + e)
for w in warns:
    print("WARN:" + w)
PY
)"
  if [ -n "$skill_out" ]; then
    while IFS= read -r line; do
      case "$line" in
        ERR:*) err "Skill '$expected_name': ${line#ERR:}" ;;
        WARN:*) warn "Skill '$expected_name': ${line#WARN:}" ;;
      esac
    done <<< "$skill_out"
  else
    ok "Skill '$expected_name' SKILL.md frontmatter valid"
  fi
done

info "4. Validating .claude-plugin/marketplace.json"
if [ ! -f ".claude-plugin/marketplace.json" ]; then
  err "Missing .claude-plugin/marketplace.json"
else
  mp_out="$(python3 - <<'PY'
import json, os

with open(".claude-plugin/marketplace.json", encoding="utf-8") as fh:
    mp = json.load(fh)

errs = []
plugins = mp.get("plugins")
if not isinstance(plugins, list) or not plugins:
    errs.append("plugins array must be non-empty")
else:
    for entry in plugins:
        src = entry.get("source", "")
        if not src or not os.path.isfile(os.path.join(src, "plugin.json")):
            errs.append(f"plugin source {src!r} missing plugin.json")
        for skill_path in entry.get("skills", []):
            if not os.path.isfile(os.path.join(skill_path, "SKILL.md")):
                errs.append(f"declared skill path {skill_path!r} missing SKILL.md")
print("\n".join(errs))
PY
)"
  if [ -n "$mp_out" ]; then
    while IFS= read -r line; do
      [ -n "$line" ] && err "marketplace.json: $line"
    done <<< "$mp_out"
  else
    ok ".claude-plugin/marketplace.json paths and syntax verified"
  fi
fi

info "5. Checking documentation & manifests for machine-local filesystem paths"
local_path_hits="$(python3 - <<'PY'
import os, re

pattern = re.compile(r"(/Users/[A-Za-z0-9._-]+|/home/[A-Za-z0-9._-]+|~/projects/|[A-Za-z]:\\Users\\)")
skip_dirs = {".git", ".beads", ".astro", "bin", "dist", "node_modules"}
check_exts = {".md", ".json", ".yaml", ".yml"}
check_names = {"Makefile"}

hits = []
for root, dirs, files in os.walk("."):
    dirs[:] = [d for d in dirs if d not in skip_dirs]
    for fname in sorted(files):
        _, ext = os.path.splitext(fname)
        if ext not in check_exts and fname not in check_names:
            continue
        fpath = os.path.join(root, fname)
        with open(fpath, encoding="utf-8", errors="ignore") as fh:
            for lineno, line in enumerate(fh, 1):
                if pattern.search(line):
                    hits.append(f"{fpath}:{lineno}: {line.strip()}")
print("\n".join(hits))
PY
)"
if [ -n "$local_path_hits" ]; then
  while IFS= read -r line; do
    [ -n "$line" ] && err "Hardcoded local path: $line"
  done <<< "$local_path_hits"
else
  ok "Zero machine-local filesystem paths in documentation, manifests, or Makefile"
fi

if [ "$JSON_MODE" -eq 1 ]; then
  ERR_SERIALIZED="$(printf '%s\n' "${ERR_LIST[@]:-}")"
  WARN_SERIALIZED="$(printf '%s\n' "${WARN_LIST[@]:-}")"
  python3 - "$ERRORS" "$WARNINGS" "$ERR_SERIALIZED" "$WARN_SERIALIZED" <<'PY'
import json, sys

err_count = int(sys.argv[1])
warn_count = int(sys.argv[2])
errs = [line for line in sys.argv[3].splitlines() if line]
warns = [line for line in sys.argv[4].splitlines() if line]
score = max(0, 100 - (err_count * 25) - (warn_count * 5))
print(json.dumps({
    "passed": err_count == 0,
    "score": score,
    "errors": errs,
    "warnings": warns,
}, indent=2))
PY
  if [ "$ERRORS" -eq 0 ]; then
    exit 0
  else
    exit 1
  fi
fi

echo
if [ "$ERRORS" -eq 0 ]; then
  printf 'All plugin, skill, and portable-URL validations passed (%d warning(s)).\n' "$WARNINGS"
  exit 0
else
  printf 'Validation failed with %d error(s) and %d warning(s).\n' "$ERRORS" "$WARNINGS"
  exit 1
fi
