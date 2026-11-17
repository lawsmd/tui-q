package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Styles is every style the interface draws with.
type Styles struct {
	TabRow    lipgloss.Style
	Tab       lipgloss.Style
	TabActive lipgloss.Style
	Button    lipgloss.Style

	SubTabRow    lipgloss.Style
	SubTab       lipgloss.Style
	SubTabActive lipgloss.Style
	SubTabClose  lipgloss.Style

	Header lipgloss.Style
	Row    lipgloss.Style
	RowAlt lipgloss.Style

	Title lipgloss.Style
	Label lipgloss.Style
	Faint lipgloss.Style

	Status      lipgloss.Style
	StatusBusy  lipgloss.Style
	StatusError lipgloss.Style
}

// DefaultStyles is the stock look.
func DefaultStyles() Styles {
	var (
		bar     = lipgloss.Color("#1f2335")
		barAlt  = lipgloss.Color("#292e42")
		accent  = lipgloss.Color("#7aa2f7")
		text    = lipgloss.Color("#c0caf5")
		dim     = lipgloss.Color("#737aa2")
		onLight = lipgloss.Color("#1a1b26")
		warn    = lipgloss.Color("#e0af68")
		bad     = lipgloss.Color("#f7768e")
		zebra   = lipgloss.Color("#1e2030")
	)
	return Styles{
		TabRow:    lipgloss.NewStyle().Background(bar),
		Tab:       lipgloss.NewStyle().Background(bar).Foreground(dim).Padding(0, 2),
		TabActive: lipgloss.NewStyle().Background(accent).Foreground(onLight).Bold(true).Padding(0, 2),
		Button:    lipgloss.NewStyle().Background(barAlt).Foreground(text).Padding(0, 1),

		SubTabRow:    lipgloss.NewStyle().Background(barAlt),
		SubTab:       lipgloss.NewStyle().Background(barAlt).Foreground(dim).Padding(0, 1),
		SubTabActive: lipgloss.NewStyle().Background(barAlt).Foreground(text).Bold(true).Underline(true).Padding(0, 1),
		SubTabClose:  lipgloss.NewStyle().Background(barAlt).Foreground(bad),

		Header: lipgloss.NewStyle().Foreground(accent).Bold(true),
		Row:    lipgloss.NewStyle().Foreground(text),
		RowAlt: lipgloss.NewStyle().Foreground(text).Background(zebra),

		Title: lipgloss.NewStyle().Foreground(text).Bold(true),
		Label: lipgloss.NewStyle().Foreground(accent),
		Faint: lipgloss.NewStyle().Foreground(dim),

		Status:      lipgloss.NewStyle().Background(bar).Foreground(dim),
		StatusBusy:  lipgloss.NewStyle().Background(bar).Foreground(warn),
		StatusError: lipgloss.NewStyle().Background(bar).Foreground(bad),
	}
}

// fit returns s as exactly w cells on one line: cut with an ellipsis when
// too long, padded with spaces when too short.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = strings.NewReplacer("\r\n", " ", "\n", " ", "\t", " ").Replace(s)
	s = ansi.Truncate(s, w, "…")
	if pad := w - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// blank is a w by h block of spaces.
func blank(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	line := strings.Repeat(" ", w)
	lines := make([]string, h)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
