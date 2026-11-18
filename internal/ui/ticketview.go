package ui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lawsmd/tui-q/ticket"
)

// Ticket view layout limits.
const (
	sidebarWidth = 32
	// The sidebar sits beside the main column at this width and above, and
	// under it below.
	sideBySideWidth = 90
)

// editMark follows the label of a field that can be clicked to change it.
const editMark = " ▾"

// ticketTab is one open ticket.
type ticketTab struct {
	t ticket.Ticket
	// transitions are the moves open to the ticket, loaded with it.
	transitions []ticket.Transition
	loading     bool
	// offset is the first line of the body drawn.
	offset int

	// main caches the rendered main column for one width, with the place
	// of each clickable label in it. Rendering Markdown on every frame
	// would make scrolling slow.
	main       []string
	mainLabels []labelLine
	mainWidth  int

	// fields are the names of the fields that can be clicked in the last
	// frame drawn. A field layer's index points into it.
	fields []string
}

// labelLine is the label of an editable field and the line it is drawn on.
type labelLine struct {
	name string
	line int
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

// layers draws the ticket: a row of buttons, then the scrolling body with
// a clickable layer over the label of each field that can be changed.
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

	body := area{x: a.x + 1, y: a.y + 2, w: a.w - 2, h: a.h - 2}
	tab.fields = nil
	if body.h <= 0 || body.w <= 0 {
		return layers
	}

	// labels are the clickable labels, placed in the coordinates of lines.
	type placed struct {
		labelLine
		x int
	}
	var lines []string
	var labels []placed
	if body.w >= sideBySideWidth {
		mainW := body.w - sidebarWidth - 2
		left, leftLabels := tab.mainLines(m, mainW)
		right, rightLabels := tab.sidebarLines(m, sidebarWidth)
		for i := 0; i < max(len(left), len(right)); i++ {
			l, r := "", ""
			if i < len(left) {
				l = left[i]
			}
			if i < len(right) {
				r = right[i]
			}
			lines = append(lines, pad(l, mainW)+"  "+r)
		}
		for _, l := range leftLabels {
			labels = append(labels, placed{l, 0})
		}
		for _, l := range rightLabels {
			labels = append(labels, placed{l, mainW + 2})
		}
	} else {
		side, sideLabels := tab.sidebarLines(m, body.w)
		main, mainLabels := tab.mainLines(m, body.w)
		lines = append(append(append(lines, side...), ""), main...)
		for _, l := range sideLabels {
			labels = append(labels, placed{l, 0})
		}
		for _, l := range mainLabels {
			l.line += len(side) + 1
			labels = append(labels, placed{l, 0})
		}
	}

	tab.offset = clamp(tab.offset, 0, len(lines)-body.h)
	end := min(tab.offset+body.h, len(lines))
	text := strings.Join(lines[tab.offset:end], "\n")
	layers = append(layers, lipgloss.NewLayer(text).X(body.x).Y(body.y).Z(zScenery))

	for _, l := range labels {
		row := l.line - tab.offset
		if row < 0 || row >= body.h {
			continue
		}
		id := layerID(idField, len(tab.fields))
		tab.fields = append(tab.fields, l.name)
		layers = append(layers, lipgloss.NewLayer(l.text).ID(id).X(body.x+l.x).Y(body.y+row).Z(zItem))
	}
	return layers
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

// mainLines is the main column: the profile's main fields in order, then
// the comments when the profile asks for them.
func (tab *ticketTab) mainLines(m *Model, w int) ([]string, []labelLine) {
	if tab.main != nil && tab.mainWidth == w {
		return tab.main, tab.mainLabels
	}
	s := m.styles
	var lines []string
	var labels []labelLine
	add := func(text string) {
		lines = append(lines, strings.Split(lipgloss.Wrap(text, w, ""), "\n")...)
	}
	for _, name := range m.prof.Ticket.Main {
		if !tab.shows(name) {
			continue
		}
		value := tab.t.Field(name).String()
		switch name {
		case ticket.FieldTitle:
			add(s.Title.Render(tab.t.Key + "  " + value))
		case ticket.FieldBody:
			if value == "" {
				add(s.Faint.Render("(no description)"))
			} else {
				lines = append(lines, markdownLines(value, w)...)
			}
		default:
			text := tab.labelFor(m, name, w)
			if tab.editable(name) {
				labels = append(labels, labelLine{name: name, line: len(lines), text: text})
			}
			lines = append(lines, text)
			if value == "" {
				add(s.Faint.Render("none"))
			} else {
				lines = append(lines, markdownLines(value, w)...)
			}
		}
		lines = append(lines, "")
	}
	if m.prof.Ticket.Comments && len(tab.t.Comments) > 0 {
		lines = append(lines, s.Label.Render("Comments"), "")
		for _, c := range tab.t.Comments {
			lines = append(lines, s.Faint.Render(c.Author+"  "+c.Created.Local().Format("2006-01-02 15:04")))
			lines = append(lines, markdownLines(c.Body, w)...)
			lines = append(lines, "")
		}
	}
	tab.main, tab.mainLabels, tab.mainWidth = lines, labels, w
	return lines, labels
}

// sidebarLines is the profile's sidebar fields, a label over each value.
func (tab *ticketTab) sidebarLines(m *Model, w int) ([]string, []labelLine) {
	s := m.styles
	var lines []string
	var labels []labelLine
	for _, name := range m.prof.Ticket.Sidebar {
		if !tab.shows(name) {
			continue
		}
		value := tab.t.Field(name).String()
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
