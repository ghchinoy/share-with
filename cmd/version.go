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

package cmd

import (
	"fmt"
	"strings"
)

var (
	// Version is the semantic version of share-with, updated by release-please
	// and overridden on tagged builds via GoReleaser -ldflags.
	Version = "0.2.0" // x-release-please-version

	// Commit is the short VCS commit hash injected at build time.
	Commit = "dev"

	// Date is the UTC RFC3339 build timestamp injected at build time.
	Date = "unknown"
)

// BuildVersionString formats the version string displayed by 'share-with --version'.
func BuildVersionString() string {
	v := strings.TrimPrefix(strings.TrimSpace(Version), "v")
	if v == "" {
		v = "dev"
	}

	var meta []string
	if c := strings.TrimSpace(Commit); c != "" && c != "dev" && c != "none" {
		meta = append(meta, "commit "+c)
	}
	if d := strings.TrimSpace(Date); d != "" && d != "unknown" {
		meta = append(meta, "built "+d)
	}
	if len(meta) == 0 {
		return v
	}
	return fmt.Sprintf("%s (%s)", v, strings.Join(meta, ", "))
}
