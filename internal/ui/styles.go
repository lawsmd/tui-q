package ui

import (
	"image/color"
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

	// SubTab is the tab of an open ticket, SubTabActive the tab of the ticket
	// on show, and SubTabDrag a tab being dragged to a new place. SubTabClose
	// is the close button of a SubTab; the other two draw their own.
	SubTabRow    lipgloss.Style
	SubTab       lipgloss.Style
	SubTabActive lipgloss.Style
	SubTabDrag   lipgloss.Style
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
	// Info is the mark on a bucket heading that shows what the bucket holds.
	Info lipgloss.Style
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
	// PickTag is a tag in the tag box, PickCursor the result under the
	// cursor, and PickHint the faint lines of the box.
	PickTag    lipgloss.Style
	PickCursor lipgloss.Style
	PickHint   lipgloss.Style

	Title lipgloss.Style
	Label lipgloss.Style
	Faint lipgloss.Style

	// Rule is the line that divides the sections of a ticket, and Section
	// the name set into it.
	Rule    lipgloss.Style
	Section lipgloss.Style
	// CommentBorder frames one comment, and CommentAuthor names who wrote it.
	// Both are drawn in the commenter's colour when Commenters has one.
	CommentBorder lipgloss.Style
	// ComposerBack is the colour the editor of the comment box is drawn on,
	// as "#rrggbb": a little lighter than the terminal's own. Composer is
	// the cells the editor draws over. ComposerBorder frames the box, in one
	// grey whoever writes. ComposerName is the name in the border, and
	// ComposerFocus the name while the box has the keyboard.
	ComposerBack   string
	Composer       lipgloss.Style
	ComposerBorder lipgloss.Style
	ComposerName   lipgloss.Style
	ComposerFocus  lipgloss.Style
	// PictureBack is the colour behind a picture drawn in a ticket, as
	// "#rrggbb", and Picture the cells the picture is drawn over. The colour
	// is not the terminal's own, so the cells a picture leaves are drawn
	// again, which clears it from them.
	PictureBack string
	Picture     lipgloss.Style
	// ComposerKind is a kind of comment the box can post, and
	// ComposerKindOn the kind it will post.
	ComposerKind   lipgloss.Style
	ComposerKindOn lipgloss.Style
	CommentAuthor  lipgloss.Style
	// Commenters are the colours given to the people who commented on a
	// ticket, in the order they first commented. The colours are used again
	// from the start when a ticket has more commenters than colours.
	Commenters []color.Color

	// Code is the text of a code block, CodeBar the bar down its left edge
	// and CodeLabel the name of its language. The rest colour its tokens.
	Code         lipgloss.Style
	CodeBar      lipgloss.Style
	CodeLabel    lipgloss.Style
	CodeKeyword  lipgloss.Style
	CodeString   lipgloss.Style
	CodeNumber   lipgloss.Style
	CodeComment  lipgloss.Style
	CodeName     lipgloss.Style
	CodeOperator lipgloss.Style

	Status      lipgloss.Style
	StatusBusy  lipgloss.Style
	StatusError lipgloss.Style
}

// composerBack is the stock colour of the comment box: a grey that stands
// out from a black terminal and still takes the terminal's light text.
const composerBack = "#34363d"

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
		chip    = lipgloss.Color("#3b4261")
		bright  = lipgloss.Color("#e6ebff")
		purple  = lipgloss.Color("#bb9af7")
		green   = lipgloss.Color("#9ece6a")
		orange  = lipgloss.Color("#ff9e64")
		cyan    = lipgloss.Color("#89ddff")
	)
	code := lipgloss.NewStyle().Background(zebra)
	return Styles{
		TabRow:    lipgloss.NewStyle().Background(bar),
		Tab:       lipgloss.NewStyle().Background(bar).Foreground(dim).Padding(0, 2),
		TabActive: lipgloss.NewStyle().Background(accent).Foreground(onLight).Bold(true).Padding(0, 2),
		Button:    lipgloss.NewStyle().Background(barAlt).Foreground(text).Padding(0, 1),

		Jump:     lipgloss.NewStyle().Background(barAlt).Foreground(text),
		JumpHint: lipgloss.NewStyle().Background(barAlt).Foreground(dim),

		SubTabRow:    lipgloss.NewStyle().Background(barAlt),
		SubTab:       lipgloss.NewStyle().Background(chip).Foreground(text).Padding(0, 1),
		SubTabActive: lipgloss.NewStyle().Background(accent).Foreground(onLight).Bold(true).Padding(0, 1),
		SubTabDrag:   lipgloss.NewStyle().Background(warn).Foreground(onLight).Bold(true).Padding(0, 1),
		SubTabClose:  lipgloss.NewStyle().Background(chip).Foreground(bad),

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
		Info:       lipgloss.NewStyle().Background(band).Foreground(cyan),
		LaneTarget: lipgloss.NewStyle().Background(accent).Foreground(onLight).Bold(true),
		CardBorder: lipgloss.NewStyle().Foreground(dim),
		CardKey:    lipgloss.NewStyle().Foreground(accent).Bold(true),
		CardText:   lipgloss.NewStyle().Foreground(text),
		CardGhost:  lipgloss.NewStyle().Foreground(barAlt),
		CardDrag:   lipgloss.NewStyle().Foreground(warn).Background(bar),

		MenuBorder: lipgloss.NewStyle().Foreground(accent).Background(bar),
		MenuTitle:  lipgloss.NewStyle().Foreground(accent).Background(bar).Bold(true),
		MenuItem:   lipgloss.NewStyle().Foreground(text).Background(bar),
		PickTag:    lipgloss.NewStyle().Foreground(bright).Background(chip).Padding(0, 1),
		PickCursor: lipgloss.NewStyle().Foreground(onLight).Background(accent),
		PickHint:   lipgloss.NewStyle().Foreground(dim).Background(bar),

		Title: lipgloss.NewStyle().Foreground(text).Bold(true),
		Label: lipgloss.NewStyle().Foreground(accent),
		Faint: lipgloss.NewStyle().Foreground(dim),

		Rule:           lipgloss.NewStyle().Foreground(dim),
		Section:        lipgloss.NewStyle().Foreground(accent).Bold(true),
		CommentBorder:  lipgloss.NewStyle().Foreground(dim),
		ComposerBack:   composerBack,
		Composer:       lipgloss.NewStyle().Background(lipgloss.Color(composerBack)).Foreground(text),
		ComposerBorder: lipgloss.NewStyle().Foreground(lipgloss.Color("#8a8d98")),
		ComposerName:   lipgloss.NewStyle().Foreground(text).Bold(true),
		ComposerFocus:  lipgloss.NewStyle().Foreground(accent).Bold(true),
		PictureBack:    composerBack,
		Picture:        lipgloss.NewStyle().Background(lipgloss.Color(composerBack)),
		ComposerKind:   lipgloss.NewStyle().Background(lipgloss.Color(composerBack)).Foreground(dim),
		ComposerKindOn: lipgloss.NewStyle().Background(accent).Foreground(onLight).Bold(true),
		CommentAuthor:  lipgloss.NewStyle().Foreground(text).Bold(true),
		// Pastels in colour-wheel order of rank: the primaries, then the
		// secondaries, then the tertiaries, each tertiary placed away from
		// the one before it.
		Commenters: []color.Color{
			lipgloss.Color("#f59e9e"), // red
			lipgloss.Color("#f2e49b"), // yellow
			lipgloss.Color("#9ebcf5"), // blue
			lipgloss.Color("#a3e4ae"), // green
			lipgloss.Color("#f5c49e"), // orange
			lipgloss.Color("#c9a5f2"), // purple
			lipgloss.Color("#9be8e1"), // blue-green
			lipgloss.Color("#f2a5d8"), // red-violet
			lipgloss.Color("#d3ec9b"), // yellow-green
			lipgloss.Color("#a9a5f5"), // blue-violet
			lipgloss.Color("#f5d79e"), // yellow-orange
			lipgloss.Color("#f5b09e"), // red-orange
		},

		Code:         code.Foreground(text),
		CodeBar:      code.Foreground(accent),
		CodeLabel:    code.Foreground(dim),
		CodeKeyword:  code.Foreground(purple),
		CodeString:   code.Foreground(green),
		CodeNumber:   code.Foreground(orange),
		CodeComment:  code.Foreground(dim),
		CodeName:     code.Foreground(accent),
		CodeOperator: code.Foreground(cyan),

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
	return frame(title, lines, w, " ", border)
}

// frame draws a rounded border around lines, with pad between the border
// and each line. Every line must already be as wide as that leaves: w less
// the two borders and the two pads.
func frame(title string, lines []string, w int, pad string, border lipgloss.Style) []string {
	out := make([]string, 0, len(lines)+2)
	out = append(out, frameTop(title, w, border))
	for _, line := range lines {
		out = append(out, border.Render("│"+pad)+line+border.Render(pad+"│"))
	}
	return append(out, border.Render("╰"+strings.Repeat("─", max(w-2, 0))+"╯"))
}

// frameTop is the top border of a frame w cells wide, with the title set
// into it when it has room for one.
func frameTop(title string, w int, border lipgloss.Style) string {
	if title = ruleTitle(title, w); title == "" {
		return border.Render("╭" + strings.Repeat("─", max(w-2, 0)) + "╮")
	}
	rest := strings.Repeat("─", w-5-lipgloss.Width(title))
	return border.Render("╭─ ") + title + border.Render(" "+rest+"╮")
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
