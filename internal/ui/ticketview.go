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

// ticketTab is one open ticket.
type ticketTab struct {
	t       ticket.Ticket
	loading bool
	// offset is the first line of the body drawn.
	offset int
}

// layers draws the ticket: a row of buttons, then the scrolling body.
func (tab *ticketTab) layers(m *Model, a area) []*lipgloss.Layer {
	s := m.styles
	layers := []*lipgloss.Layer{}

	x := a.x + 1
	if tab.t.URL != "" {
		button := s.Button.Render("open in browser")
		layers = append(layers, lipgloss.NewLayer(button).ID(idOpen).X(x).Y(a.y).Z(zItem))
		x += lipgloss.Width(button) + 1
	}
	if tab.loading {
		layers = append(layers, lipgloss.NewLayer(s.Faint.Render("loading...")).X(x).Y(a.y).Z(zScenery))
	}

	body := area{x: a.x + 1, y: a.y + 2, w: a.w - 2, h: a.h - 2}
	if body.h <= 0 || body.w <= 0 {
		return layers
	}

	var lines []string
	if body.w >= sideBySideWidth {
		mainW := body.w - sidebarWidth - 2
		left := tab.mainLines(m, mainW)
		right := tab.sidebarLines(m, sidebarWidth)
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
	} else {
		lines = append(lines, tab.sidebarLines(m, body.w)...)
		lines = append(lines, "")
		lines = append(lines, tab.mainLines(m, body.w)...)
	}

	tab.offset = clamp(tab.offset, 0, len(lines)-body.h)
	end := min(tab.offset+body.h, len(lines))
	text := strings.Join(lines[tab.offset:end], "\n")
	return append(layers, lipgloss.NewLayer(text).X(body.x).Y(body.y).Z(zScenery))
}

// pad extends a styled line with spaces to w cells.
func pad(s string, w int) string {
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// mainLines is the main column: the profile's main fields in order, then
// the comments when the profile asks for them.
func (tab *ticketTab) mainLines(m *Model, w int) []string {
	s := m.styles
	var lines []string
	add := func(text string) {
		lines = append(lines, strings.Split(lipgloss.Wrap(text, w, ""), "\n")...)
	}
	for _, name := range m.prof.Ticket.Main {
		value := tab.t.Field(name).String()
		switch name {
		case ticket.FieldTitle:
			add(s.Title.Render(tab.t.Key + "  " + value))
		case ticket.FieldBody:
			if value == "" {
				value = s.Faint.Render("(no description)")
			}
			add(value)
		default:
			lines = append(lines, s.Label.Render(m.prof.Label(name)))
			add(value)
		}
		lines = append(lines, "")
	}
	if m.prof.Ticket.Comments && len(tab.t.Comments) > 0 {
		lines = append(lines, s.Label.Render("Comments"), "")
		for _, c := range tab.t.Comments {
			lines = append(lines, s.Faint.Render(c.Author+"  "+c.Created.Local().Format("2006-01-02 15:04")))
			add(c.Body)
			lines = append(lines, "")
		}
	}
	return lines
}

// sidebarLines is the profile's sidebar fields, a label over each value.
func (tab *ticketTab) sidebarLines(m *Model, w int) []string {
	s := m.styles
	var lines []string
	for _, name := range m.prof.Ticket.Sidebar {
		value := tab.t.Field(name).String()
		if value == "" {
			value = s.Faint.Render("none")
		}
		lines = append(lines, s.Label.Render(fit(m.prof.Label(name), w)))
		lines = append(lines, strings.Split(lipgloss.Wrap(value, w, ""), "\n")...)
		lines = append(lines, "")
	}
	return lines
}
