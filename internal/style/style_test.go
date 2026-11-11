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
