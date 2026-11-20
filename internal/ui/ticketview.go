package ui

import (
	"fmt"
	"image/color"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lawsmd/tui-q/ticket"
)

// Ticket view layout limits.
const (
	sidebarWidth = 32
	// The sidebar sits beside the main column at this width and above, and
	// over it below.
	sideBySideWidth = 90
	// titleRows is the most lines the title is given.
	titleRows = 3
	// minColumnRows is the fewest rows the main column has under a title
	// that stays in place.
	minColumnRows = 4
)

// editMark follows the label of a field that can be clicked to change it.
const editMark = " ▾"

// ticketTab is one open ticket.
type ticketTab struct {
	t ticket.Ticket
	// transitions are the moves open to the ticket, loaded with it.
	transitions []ticket.Transition
	loading     bool
	// offset is the first line drawn of the main column, or of the whole
	// body when the sidebar is not beside it.
	offset int
	// sideOffset is the first line of the sidebar drawn when it is beside
	// the main column, and sideX the screen column it started at in the
	// last frame. sideX is 0 when the sidebar is not beside the main column.
	sideOffset int
	sideX      int

	// main caches the rendered main column for one width, with the place
	// of each clickable label in it. Rendering Markdown on every frame
	// would make scrolling slow. mainBox is the line the comment box goes
	// before: under the heading of the comments, beside the newest. mainLoading is whether the ticket was
	// still loading then, which changes what an empty comments section says.
	main        []string
	mainLabels  []labelLine
	mainBox     int
	mainWidth   int
	mainLoading bool

	// fields are the names of the fields that can be clicked in the last
	// frame drawn. A field layer's index points into it.
	fields []string
}

// labelLine is something clickable in a column: the label of an editable
// field, with the line it is drawn on and how far along the line it
// starts. One with no name is the control for the order of the comments.
type labelLine struct {
	name string
	line int
	x    int
	text string
}

// set replaces the ticket and drops the rendered cache.
func (tab *ticketTab) set(t ticket.Ticket) {
	tab.t = t
	tab.main = nil
}

// shows reports whether a field belongs in the view of this ticket. A
// custom field with no value that the ticket's edit screen does not have
// is left out: the platform does not show it on this kind of ticket.
func (tab *ticketTab) shows(name string) bool {
	if ticket.BuiltIn(name) || tab.t.Field(name).String() != "" {
		return true
	}
	_, ok := tab.t.Spec(name)
	return ok
}

// layers draws the ticket: a row of buttons, then the title, the main
// column and the sidebar, the comment box under the heading of the
// comments, with a clickable layer over the label of each
// field that can be changed. In a wide pane the title and the sidebar stay
// in place while the main column scrolls. In a narrow one the title, the
// sidebar and the main column scroll as one column.
func (tab *ticketTab) layers(m *Model, a area) []*lipgloss.Layer {
	s := m.styles
	layers := []*lipgloss.Layer{}

	x := a.x + 1
	button := func(id, text string) {
		b := s.Button.Render(text)
		if x+lipgloss.Width(b) > a.x+a.w {
			return
		}
		layers = append(layers, lipgloss.NewLayer(b).ID(id).X(x).Y(a.y).Z(zItem))
		x += lipgloss.Width(b) + 1
	}
	if tab.t.URL != "" {
		button(idOpen, "open in browser")
		button(idCopyLink, "copy link")
	}
	if len(tab.transitions) > 0 {
		button(idMove, "move"+editMark)
	}
	if tab.editable(ticket.FieldAssignee) {
		button(idAssign, "assign"+editMark)
	}
	for _, i := range m.actionsFor(tab.t) {
		button(layerID(idAction, i), m.actions[i].Name)
	}
	if tab.loading {
		layers = append(layers, lipgloss.NewLayer(s.Faint.Render("loading...")).X(x).Y(a.y).Z(zScenery))
	}

	body := ticketBody(a)
	tab.fields = nil
	tab.sideX = 0
	if body.h <= 0 || body.w <= 0 {
		return layers
	}

	// column draws the lines of one scrolling column in the area c, from
	// the line offset down, and puts a clickable layer over each label in
	// view. It returns the offset, held to the lines there are.
	column := func(c area, lines []string, labels []labelLine, offset int) int {
		offset = clamp(offset, 0, len(lines)-c.h)
		if c.h <= 0 || len(lines) == 0 {
			return offset
		}
		end := min(offset+c.h, len(lines))
		text := strings.Join(lines[offset:end], "\n")
		layers = append(layers, lipgloss.NewLayer(text).X(c.x).Y(c.y).Z(zScenery))
		for _, l := range labels {
			row := l.line - offset
			if row < 0 || row >= c.h {
				continue
			}
			id := idCommentOrder
			if l.name != "" {
				id = layerID(idField, len(tab.fields))
				tab.fields = append(tab.fields, l.name)
			}
			layers = append(layers, lipgloss.NewLayer(l.text).ID(id).X(c.x+l.x).Y(c.y+row).Z(zItem))
		}
		return offset
	}

	c, lines, labels, head, at := tab.mainColumn(m, a)
	if len(head) > 0 {
		layers = append(layers, lipgloss.NewLayer(strings.Join(head, "\n")).X(body.x).Y(body.y).Z(zScenery))
	}
	if c.h <= 0 {
		return layers
	}

	// The comment box has its lines among those of the column, left blank
	// here, and is drawn over the ones in view.
	if box := tab.placeBox(m, c, len(lines), at); box.on {
		lines = slices.Concat(lines[:at], make([]string, box.rows+2), lines[at:])
		labels = slices.Clone(labels)
		for i := range labels {
			if labels[i].line >= at {
				labels[i].line += box.rows + 2
			}
		}
		layers = append(layers, m.composerLayers(tab.t, box)...)
	}
	tab.offset = column(c, lines, labels, tab.offset)
	if head == nil {
		return layers
	}

	// A space, the dividing line, a space, the sidebar.
	divider := strings.TrimSuffix(strings.Repeat(s.Rule.Render("│")+"\n", c.h), "\n")
	layers = append(layers, lipgloss.NewLayer(divider).X(c.x+c.w+1).Y(c.y).Z(zScenery))

	tab.sideX = c.x + c.w + 3
	right, rightLabels := tab.sidebarLines(m, sidebarWidth)
	tab.sideOffset = column(area{tab.sideX, c.y, sidebarWidth, c.h}, right, rightLabels, tab.sideOffset)
	return layers
}

// mainColumn is the scrolling column that holds the main fields of the
// ticket: its area, its lines and labels, and the line the comment box
// goes before. In a narrow pane the column is the whole body, and head is
// nil. In a wide one head is the title, which stays in place over the
// column, and the sidebar is beside it. The area has no height when the
// pane has no room for a ticket.
func (tab *ticketTab) mainColumn(m *Model, a area) (c area, lines []string, labels []labelLine, head []string, at int) {
	body := ticketBody(a)
	if body.h <= 0 || body.w <= 0 {
		return area{}, nil, nil, nil, -1
	}
	title := tab.titleLines(m, body.w)
	// A pane too short to hold the title over a column of its own scrolls
	// the title with the rest, as a narrow one does.
	if body.w < sideBySideWidth || body.h-len(title) < minColumnRows {
		lines = title
		side, sideLabels := tab.sidebarLines(m, body.w)
		main, mainLabels := tab.mainLines(m, body.w)
		at = len(lines) + len(side) + tab.mainBox
		for _, part := range []struct {
			lines  []string
			labels []labelLine
		}{{side, sideLabels}, {main, mainLabels}} {
			for _, l := range part.labels {
				l.line += len(lines)
				labels = append(labels, l)
			}
			lines = append(lines, part.lines...)
		}
		return body, lines, labels, nil, at
	}
	head = title
	if head == nil {
		head = []string{}
	}
	c = area{x: body.x, y: body.y + len(head), w: body.w - sidebarWidth - 3, h: body.h - len(head)}
	lines, labels = tab.mainLines(m, c.w)
	return c, lines, labels, head, tab.mainBox
}

// pad extends a styled line with spaces to w cells.
func pad(s string, w int) string {
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// labelFor renders the label of a field w cells wide, with the edit mark
// when the field can be changed.
func (tab *ticketTab) labelFor(m *Model, name string, w int) string {
	label := m.label(name)
	if tab.editable(name) {
		label += editMark
	}
	return m.styles.Label.Render(fit(label, w))
}

// titleLines is the key and title of the ticket over one blank line, when
// the profile puts the title in the main column. A title that wraps onto
// more than titleRows lines is cut.
func (tab *ticketTab) titleLines(m *Model, w int) []string {
	if !slices.Contains(m.prof.Ticket.Main, ticket.FieldTitle) {
		return nil
	}
	text := tab.t.Key + "  " + tab.t.Field(ticket.FieldTitle).String()
	lines := strings.Split(lipgloss.Wrap(text, w, ""), "\n")
	if len(lines) > titleRows {
		last := strings.Join(lines[titleRows-1:], " ")
		lines = append(lines[:titleRows-1], ansi.Truncate(last, w, "…"))
	}
	for i, line := range lines {
		lines[i] = m.styles.Title.Render(line)
	}
	return append(lines, "")
}

// mainLines is the main column: the profile's main fields in order under a
// rule each, then the comments when the profile asks for them. The title
// is drawn by titleLines.
func (tab *ticketTab) mainLines(m *Model, w int) ([]string, []labelLine) {
	if tab.main != nil && tab.mainWidth == w && tab.mainLoading == tab.loading {
		return tab.main, tab.mainLabels
	}
	s := m.styles
	lines := []string{}
	var labels []labelLine
	add := func(text string) {
		lines = append(lines, strings.Split(lipgloss.Wrap(text, w, ""), "\n")...)
	}
	for _, name := range m.prof.Ticket.Main {
		if name == ticket.FieldTitle || !tab.shows(name) {
			continue
		}
		value := tab.t.Field(name).String()
		label := m.label(name)
		empty := "none"
		if name == ticket.FieldBody {
			empty = "(no description)"
		} else if tab.editable(name) {
			label += editMark
			text := s.Section.Render(ruleTitle(label, w))
			labels = append(labels, labelLine{name: name, line: len(lines), x: ruleIndent, text: text})
		}
		lines = append(lines, s.rule(label, w), "")
		if value == "" {
			add(s.Faint.Render(empty))
		} else {
			lines = append(lines, s.markdownLines(value, w)...)
		}
		lines = append(lines, "")
	}
	// The comment box goes under the heading of the comments, beside the
	// newest comment: before the comments when the newest is first, after
	// them when it is last. Without comments in the profile it ends the
	// column.
	tab.mainBox = len(lines)
	if m.prof.Ticket.Comments && len(tab.t.Comments) == 0 {
		// A ticket opened from a queue carries no comments until its full
		// load returns, so say so rather than claim there are none.
		empty := "No comments yet."
		if tab.loading {
			empty = "loading..."
		}
		lines = append(lines, s.rule("Comments", w), "")
		tab.mainBox = len(lines)
		lines = append(lines, s.Faint.Render(empty), "")
	}
	if m.prof.Ticket.Comments && len(tab.t.Comments) > 0 {
		oldestFirst := m.prefs.CommentsOldestFirst
		comments := slices.Clone(tab.t.Comments)
		slices.SortStableFunc(comments, func(a, b ticket.Comment) int {
			if oldestFirst {
				return a.Created.Compare(b.Created)
			}
			return b.Created.Compare(a.Created)
		})

		heading := fmt.Sprintf("Comments (%d)", len(comments))
		// The control for the order sits at the right end of the rule,
		// when the rule has room for it after the heading.
		control := " newest first" + editMark + " "
		if oldestFirst {
			control = " oldest first" + editMark + " "
		}
		if x := w - lipgloss.Width(control) - 2; x >= ruleIndent+lipgloss.Width(heading)+2 {
			labels = append(labels, labelLine{line: len(lines), x: x, text: s.Label.Render(control)})
		}
		lines = append(lines, s.rule(heading, w), "")
		tab.mainBox = len(lines)

		colours := commenterColours(tab.t.Comments, s.Commenters)
		for _, c := range comments {
			author, border := s.CommentAuthor, s.CommentBorder
			if colour, ok := colours[c.Author]; ok {
				author, border = author.Foreground(colour), border.Foreground(colour)
			}
			title := author.Render(c.Author) +
				s.Faint.Render(" · "+c.Created.Local().Format("2006-01-02 15:04"))
			if c.Kind != "" {
				title += s.Faint.Render(" · ") + s.Label.Render(c.Kind)
			}
			text := unindent(s.markdownLines(c.Body, w-4+markdownMargin), markdownMargin)
			for i, line := range text {
				text[i] = pad(line, w-4)
			}
			lines = append(lines, titledBox(title, text, w, border)...)
		}
		if oldestFirst {
			tab.mainBox = len(lines)
		}
	}
	tab.main, tab.mainLabels, tab.mainWidth, tab.mainLoading = lines, labels, w, tab.loading
	return lines, labels
}

// commenterColours gives each author in comments a colour from palette. The
// author of the oldest comment gets the first colour, the next new author
// the second, and the palette is used again from the start when the authors
// outnumber it. An author keeps one colour whichever way the comments are
// shown. The map is empty when the palette is.
func commenterColours(comments []ticket.Comment, palette []color.Color) map[string]color.Color {
	colours := map[string]color.Color{}
	if len(palette) == 0 {
		return colours
	}
	byAge := slices.Clone(comments)
	slices.SortStableFunc(byAge, func(a, b ticket.Comment) int {
		return a.Created.Compare(b.Created)
	})
	for _, c := range byAge {
		if _, ok := colours[c.Author]; !ok {
			colours[c.Author] = palette[len(colours)%len(palette)]
		}
	}
	return colours
}

// sidebarLines is the profile's sidebar fields under a rule, a label over
// each value.
func (tab *ticketTab) sidebarLines(m *Model, w int) ([]string, []labelLine) {
	s := m.styles
	lines := []string{s.rule("Details", w), ""}
	var labels []labelLine
	for _, name := range m.prof.Ticket.Sidebar {
		if !tab.shows(name) {
			continue
		}
		value := m.withIcon(name, tab.t.Field(name).String())
		if value == "" {
			value = s.Faint.Render("none")
		}
		text := tab.labelFor(m, name, w)
		if tab.editable(name) {
			labels = append(labels, labelLine{name: name, line: len(lines), text: text})
		}
		lines = append(lines, text)
		lines = append(lines, strings.Split(lipgloss.Wrap(value, w, ""), "\n")...)
		lines = append(lines, "")
	}
	return lines, labels
}
