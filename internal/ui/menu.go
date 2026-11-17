package ui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lawsmd/tui-q/ticket"
)

// menu is the context menu of one ticket. It is opened by a right-click or
// middle-click on a row or card, or by a click on the menu button.
type menu struct {
	// x and y are where the pointer was when the menu opened.
	x, y  int
	title string
	items []menuItem
}

type menuItem struct {
	label string
	run   func() tea.Cmd
}

// openMenu builds the menu for a ticket at a screen position.
func (m *Model) openMenu(t ticket.Ticket, x, y int) {
	items := []menuItem{
		{"Open", func() tea.Cmd { return m.openTicket(t) }},
	}
	if t.URL != "" {
		items = append(items,
			menuItem{"Open in browser", func() tea.Cmd { return m.openURL(t) }},
			menuItem{"Copy link", func() tea.Cmd { return m.copy(t.URL, "link to "+t.Key) }},
		)
	}
	items = append(items, menuItem{"Copy key", func() tea.Cmd { return m.copy(t.Key, t.Key) }})
	m.menu = &menu{x: x, y: y, title: t.Key, items: items}
}

// copy puts text on the clipboard through the terminal.
func (m *Model) copy(text, what string) tea.Cmd {
	m.notice = "Copied " + what
	return tea.SetClipboard(text)
}

// menuLayers draws the menu as a box over everything else, moved as needed
// to stay inside the pane. The box takes clicks on its border and title;
// each item is its own layer above it.
func (m *Model) menuLayers() []*lipgloss.Layer {
	s := m.styles
	inner := lipgloss.Width(m.menu.title)
	for _, it := range m.menu.items {
		inner = max(inner, lipgloss.Width(it.label))
	}
	w := inner + 4
	lines := []string{s.MenuTitle.Render(fit(m.menu.title, inner))}
	for _, it := range m.menu.items {
		lines = append(lines, s.MenuItem.Render(fit(it.label, inner)))
	}
	rows := box(lines, w, s.MenuBorder)

	x := clamp(m.menu.x, 0, m.w-w)
	y := clamp(m.menu.y, 0, m.h-len(rows))
	text := ""
	for i, row := range rows {
		if i > 0 {
			text += "\n"
		}
		text += row
	}
	layers := []*lipgloss.Layer{lipgloss.NewLayer(text).ID(idMenu).X(x).Y(y).Z(zMenu)}
	for i, it := range m.menu.items {
		item := s.MenuItem.Render(fit(" "+it.label, w-2))
		layers = append(layers, lipgloss.NewLayer(item).ID(layerID(idMenuItem, i)).X(x+1).Y(y+2+i).Z(zMenuItem))
	}
	return layers
}
