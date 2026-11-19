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

	// Jump is the box that takes a ticket number, and JumpHint the text in
	// it while it is empty.
	Jump     lipgloss.Style
	JumpHint lipgloss.Style

	SubTabRow    lipgloss.Style
	SubTab       lipgloss.Style
	SubTabActive lipgloss.Style
	SubTabClose  lipgloss.Style

	Header lipgloss.Style
	Row    lipgloss.Style
	RowAlt lipgloss.Style

	// More is the menu button on a row or card. Arrow is a sideways scroll
	// button, and ArrowOff the same button with nothing left to scroll to.
	More     lipgloss.Style
	Arrow    lipgloss.Style
	ArrowOff lipgloss.Style

	LaneHeader lipgloss.Style
	// Bucket is the heading of a section of a table.
	Bucket lipgloss.Style
	// Page is a button on a bucket heading that turns its pages, and
	// PageOff the same button with no page to turn to.
	Page    lipgloss.Style
	PageOff lipgloss.Style
	// LaneTarget marks the lane a dragged card would be dropped on.
	LaneTarget lipgloss.Style
	CardBorder lipgloss.Style
	CardKey    lipgloss.Style
	CardText   lipgloss.Style
	// CardGhost draws a card in its old place while it is being dragged,
	// and CardDrag the copy that follows the pointer.
	CardGhost lipgloss.Style
	CardDrag  lipgloss.Style

	MenuBorder lipgloss.Style
	MenuTitle  lipgloss.Style
	MenuItem   lipgloss.Style

	Title lipgloss.Style
	Label lipgloss.Style
	Faint lipgloss.Style

	// Rule is the line that divides the sections of a ticket, and Section
	// the name set into it.
	Rule    lipgloss.Style
	Section lipgloss.Style
	// CommentBorder frames one comment, and CommentAuthor names who wrote it.
	CommentBorder lipgloss.Style
	CommentAuthor lipgloss.Style

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
		band    = lipgloss.Color("#394b70")
		bright  = lipgloss.Color("#e6ebff")
	)
	return Styles{
		TabRow:    lipgloss.NewStyle().Background(bar),
		Tab:       lipgloss.NewStyle().Background(bar).Foreground(dim).Padding(0, 2),
		TabActive: lipgloss.NewStyle().Background(accent).Foreground(onLight).Bold(true).Padding(0, 2),
		Button:    lipgloss.NewStyle().Background(barAlt).Foreground(text).Padding(0, 1),

		Jump:     lipgloss.NewStyle().Background(barAlt).Foreground(text),
		JumpHint: lipgloss.NewStyle().Background(barAlt).Foreground(dim),

		SubTabRow:    lipgloss.NewStyle().Background(barAlt),
		SubTab:       lipgloss.NewStyle().Background(barAlt).Foreground(dim).Padding(0, 1),
		SubTabActive: lipgloss.NewStyle().Background(barAlt).Foreground(text).Bold(true).Underline(true).Padding(0, 1),
		SubTabClose:  lipgloss.NewStyle().Background(barAlt).Foreground(bad),

		Header: lipgloss.NewStyle().Foreground(accent).Bold(true),
		Row:    lipgloss.NewStyle().Foreground(text),
		RowAlt: lipgloss.NewStyle().Foreground(text).Background(zebra),

		More:     lipgloss.NewStyle().Foreground(dim),
		Arrow:    lipgloss.NewStyle().Foreground(accent).Bold(true),
		ArrowOff: lipgloss.NewStyle().Foreground(barAlt),

		LaneHeader: lipgloss.NewStyle().Background(barAlt).Foreground(text).Bold(true),
		Bucket:     lipgloss.NewStyle().Background(band).Foreground(bright).Bold(true),
		Page:       lipgloss.NewStyle().Background(band).Foreground(bright).Bold(true),
		PageOff:    lipgloss.NewStyle().Background(band).Foreground(dim),
		LaneTarget: lipgloss.NewStyle().Background(accent).Foreground(onLight).Bold(true),
		CardBorder: lipgloss.NewStyle().Foreground(dim),
		CardKey:    lipgloss.NewStyle().Foreground(accent).Bold(true),
		CardText:   lipgloss.NewStyle().Foreground(text),
		CardGhost:  lipgloss.NewStyle().Foreground(barAlt),
		CardDrag:   lipgloss.NewStyle().Foreground(warn).Background(bar),

		MenuBorder: lipgloss.NewStyle().Foreground(accent).Background(bar),
		MenuTitle:  lipgloss.NewStyle().Foreground(accent).Background(bar).Bold(true),
		MenuItem:   lipgloss.NewStyle().Foreground(text).Background(bar),

		Title: lipgloss.NewStyle().Foreground(text).Bold(true),
		Label: lipgloss.NewStyle().Foreground(accent),
		Faint: lipgloss.NewStyle().Foreground(dim),

		Rule:          lipgloss.NewStyle().Foreground(dim),
		Section:       lipgloss.NewStyle().Foreground(accent).Bold(true),
		CommentBorder: lipgloss.NewStyle().Foreground(dim),
		CommentAuthor: lipgloss.NewStyle().Foreground(text).Bold(true),

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

// box draws a rounded border around lines. Every line must already be
// w-4 cells wide; the border and one cell of padding on each side make the
// result w wide. Border draws the frame and the padding.
func box(lines []string, w int, border lipgloss.Style) []string {
	return titledBox("", lines, w, border)
}

// titledBox is box with a title set into the top border. The title may be
// styled; it is cut when the border has no room for it.
func titledBox(title string, lines []string, w int, border lipgloss.Style) []string {
	rule := strings.Repeat("─", max(w-2, 0))
	top := border.Render("╭" + rule + "╮")
	if title = ruleTitle(title, w); title != "" {
		rest := strings.Repeat("─", w-5-lipgloss.Width(title))
		top = border.Render("╭─ ") + title + border.Render(" "+rest+"╮")
	}
	out := make([]string, 0, len(lines)+2)
	out = append(out, top)
	for _, line := range lines {
		out = append(out, border.Render("│ ")+line+border.Render(" │"))
	}
	return append(out, border.Render("╰"+rule+"╯"))
}

// ruleIndent is how far into a rule or a titled border its title starts.
const ruleIndent = 3

// ruleTitle cuts a title to what a rule or a border w cells wide can hold:
// the indent before it, and a space and one cell of line after it.
func ruleTitle(title string, w int) string {
	if w < ruleIndent+3 {
		return ""
	}
	return ansi.Truncate(title, w-ruleIndent-2, "…")
}

// rule is a horizontal line w cells wide with the name of a section set
// into it.
func (s Styles) rule(title string, w int) string {
	if w <= 0 {
		return ""
	}
	title = ruleTitle(title, w)
	if title == "" {
		return s.Rule.Render(strings.Repeat("─", w))
	}
	rest := strings.Repeat("─", w-ruleIndent-1-lipgloss.Width(title))
	return s.Rule.Render("── ") + s.Section.Render(title) + s.Rule.Render(" "+rest)
}
