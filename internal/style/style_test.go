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

package style

import (
	"bytes"
	"strings"
	"testing"
)

func TestShouldColorize_RespectsEnvAndNonTTY(t *testing.T) {
	var buf bytes.Buffer
	if got := ShouldColorize(&buf, func(string) string { return "" }); got {
		t.Errorf("ShouldColorize(bytes.Buffer, emptyEnv) = %v, want false", got)
	}

	p := ForStream(&buf, false, func(key string) string {
		if key == "NO_COLOR" {
			return "1"
		}
		return ""
	})
	if p.Enabled() {
		t.Errorf("ForStream(buf, false, NO_COLOR=1).Enabled() = true, want false")
	}
	if got := p.Pass("shared"); strings.Contains(got, "\x1b[") {
		t.Errorf("Palette.Pass(%q) with disabled palette emitted ANSI sequence: %q", "shared", got)
	}
}

func TestPadRight(t *testing.T) {
	got := PadRight("reader", 10)
	want := "reader    "
	if got != want {
		t.Errorf("PadRight(%q, 10) = %q, want %q", "reader", got, want)
	}
}
