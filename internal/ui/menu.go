package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lawsmd/tui-q/ticket"
)

// menu is a list of things to click, drawn over everything else. The
// context menu of a ticket is one; so are the lists of moves, of people to
// assign to, and of the options of a field.
type menu struct {
	// x and y are where the pointer was when the menu opened.
	x, y  int
	title string
	items []menuItem
	// offset is the first item drawn when the pane is too short for all
	// of them.
	offset int
}

// menuItem is one line of a menu. A line with no run is a heading: it is
// drawn faint and a click on it does nothing.
type menuItem struct {
	label string
	run   func() tea.Cmd
}

// showMenu opens a menu at a screen position.
func (m *Model) showMenu(title string, x, y int, items []menuItem) {
	if len(items) > 0 {
		m.menu = &menu{x: x, y: y, title: title, items: items}
	}
}

// openMenu opens the context menu of a ticket: by a right-click or
// middle-click on a row or card, or by a click on the menu button.
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
	items = append(items,
		menuItem{"Copy key", func() tea.Cmd { return m.copy(t.Key, t.Key) }},
		menuItem{"Move to...", func() tea.Cmd { return m.loadTransitions(t, x, y) }},
	)
	if m.me.ID != "" && !strings.EqualFold(t.Assignee, m.me.Name) {
		items = append(items, menuItem{"Assign to me", func() tea.Cmd { return m.assign(t, m.me) }})
	}
	items = append(items, menuItem{"Assign to...", func() tea.Cmd { m.assignMenu(t, x, y); return nil }})
	for _, i := range m.actionsFor(t) {
		items = append(items, menuItem{m.actions[i].Name, func() tea.Cmd { return m.runAction(i, t) }})
	}
	m.showMenu(t.Key, x, y, items)
}

// copy puts text on the clipboard through the terminal.
func (m *Model) copy(text, what string) tea.Cmd {
	m.notice = "Copied " + what
	return tea.SetClipboard(text)
}

// menuLayers draws the menu as a box over everything else, moved as needed
// to stay inside the pane. The box takes clicks on its border and title;
// each item is its own layer above it. A menu taller than the pane shows
// part of its items and scrolls with the wheel.
func (m *Model) menuLayers() []*lipgloss.Layer {
	s := m.styles
	mn := m.menu
	visible := clamp(m.h-3, 1, len(mn.items))
	mn.offset = clamp(mn.offset, 0, len(mn.items)-visible)
	title := mn.title
	if visible < len(mn.items) {
		title += fmt.Sprintf("  %d-%d of %d", mn.offset+1, mn.offset+visible, len(mn.items))
	}

	inner := lipgloss.Width(title)
	for _, it := range mn.items {
		inner = max(inner, lipgloss.Width(it.label))
	}
	inner = min(inner, max(m.w-4, 1))
	w := inner + 4
	lines := []string{s.MenuTitle.Render(fit(title, inner))}
	for range visible {
		lines = append(lines, s.MenuItem.Render(blank(inner, 1)))
	}
	rows := box(lines, w, s.MenuBorder)

	x := clamp(mn.x, 0, m.w-w)
	y := clamp(mn.y, 0, m.h-len(rows))
	layers := []*lipgloss.Layer{lipgloss.NewLayer(strings.Join(rows, "\n")).ID(idMenu).X(x).Y(y).Z(zMenu)}
	for i := 0; i < visible; i++ {
		n := mn.offset + i
		style := s.MenuItem
		if mn.items[n].run == nil {
			style = s.MenuTitle
		}
		item := style.Render(fit(" "+mn.items[n].label, w-2))
		layers = append(layers, lipgloss.NewLayer(item).ID(layerID(idMenuItem, n)).X(x+1).Y(y+2+i).Z(zMenuItem))
	}
	return layers
}
