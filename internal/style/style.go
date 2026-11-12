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

// Package style provides semantic Lipgloss color tokens and alignment-safe
// terminal formatting that degrades cleanly when NO_COLOR, SHARE_WITH_NO_TUI,
// or non-TTY streams are active.
package style

import (
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

// Semantic color hex values aligned with Agent-Aware CLI guidelines.
const (
	HexAccent  = "#39BAE6"
	HexCommand = "#CBCCC6"
	HexPass    = "#04B575"
	HexWarn    = "#FFB454"
	HexFail    = "#FF4D4D"
	HexMuted   = "#6C7680"
	HexID      = "#00D7D7"
)

// Palette renders text with semantic color roles when enabled, or returns
// plain unstyled strings when disabled.
type Palette struct {
	enabled bool
	accent  lipgloss.Style
	command lipgloss.Style
	pass    lipgloss.Style
	warn    lipgloss.Style
	fail    lipgloss.Style
	muted   lipgloss.Style
	id      lipgloss.Style
	bold    lipgloss.Style
}

// New creates a Palette configured for enabled or plain-text rendering.
func New(enabled bool) Palette {
	return Palette{
		enabled: enabled,
		accent:  lipgloss.NewStyle().Foreground(lipgloss.Color(HexAccent)).Bold(true),
		command: lipgloss.NewStyle().Foreground(lipgloss.Color(HexCommand)).Bold(true),
		pass:    lipgloss.NewStyle().Foreground(lipgloss.Color(HexPass)).Bold(true),
		warn:    lipgloss.NewStyle().Foreground(lipgloss.Color(HexWarn)).Bold(true),
		fail:    lipgloss.NewStyle().Foreground(lipgloss.Color(HexFail)).Bold(true),
		muted:   lipgloss.NewStyle().Foreground(lipgloss.Color(HexMuted)),
		id:      lipgloss.NewStyle().Foreground(lipgloss.Color(HexID)),
		bold:    lipgloss.NewStyle().Bold(true),
	}
}

// ForStream builds a Palette for out, disabling ANSI sequences when jsonMode is true,
// NO_COLOR or SHARE_WITH_NO_TUI is set, TERM is "dumb", or out is not a character device.
func ForStream(out io.Writer, jsonMode bool, getenv func(string) string) Palette {
	if getenv == nil {
		getenv = os.Getenv
	}
	if jsonMode || !ShouldColorize(out, getenv) {
		return New(false)
	}
	return New(true)
}

// ShouldColorize reports whether ANSI styling should be emitted to out.
func ShouldColorize(out io.Writer, getenv func(string) string) bool {
	if getenv == nil {
		getenv = os.Getenv
	}
	if getenv("NO_COLOR") != "" {
		return false
	}
	if isTruthy(getenv("SHARE_WITH_NO_TUI")) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(getenv("TERM")), "dumb") {
		return false
	}
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// Enabled reports whether ANSI styling is active on this palette.
func (p Palette) Enabled() bool { return p.enabled }

// Accent styles navigation landmarks, headers, and group titles.
func (p Palette) Accent(text string) string {
	if !p.enabled {
		return text
	}
	return p.accent.Render(text)
}

// Command styles command names, flags, and executable hints.
func (p Palette) Command(text string) string {
	if !p.enabled {
		return text
	}
	return p.command.Render(text)
}

// Pass styles completed operations and healthy states.
func (p Palette) Pass(text string) string {
	if !p.enabled {
		return text
	}
	return p.pass.Render(text)
}

// Warn styles dry-run notices and non-fatal warnings.
func (p Palette) Warn(text string) string {
	if !p.enabled {
		return text
	}
	return p.warn.Render(text)
}

// Fail styles errors and rejected states.
func (p Palette) Fail(text string) string {
	if !p.enabled {
		return text
	}
	return p.fail.Render(text)
}

// Muted styles secondary metadata, types, and default markers.
func (p Palette) Muted(text string) string {
	if !p.enabled {
		return text
	}
	return p.muted.Render(text)
}

// ID styles unique identifiers such as Drive file IDs and permission IDs.
func (p Palette) ID(text string) string {
	if !p.enabled {
		return text
	}
	return p.id.Render(text)
}

// Bold styles primary labels.
func (p Palette) Bold(text string) string {
	if !p.enabled {
		return text
	}
	return p.bold.Render(text)
}

// PadRight pads a plain string with trailing spaces to reach width runes before
// any ANSI color styling is applied, preventing escape sequences from corrupting
// column alignment.
func PadRight(plain string, width int) string {
	runeLen := utf8.RuneCountInString(plain)
	if runeLen >= width {
		return plain
	}
	return plain + strings.Repeat(" ", width-runeLen)
}

func isTruthy(val string) bool {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
