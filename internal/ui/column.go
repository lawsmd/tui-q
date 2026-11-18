package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/ticket"
)

// gearGlyph marks a column header that has settings. With the space in
// front of it the mark is gearWidth cells wide.
const (
	gearGlyph = "⚙"
	gearWidth = 2
)

// person reports whether a field holds people.
func (m *Model) person(name string) bool {
	return name == ticket.FieldAssignee || m.prof.Fields[name].Type == "user"
}

// hasSettings reports whether the column for a field has any settings.
func (m *Model) hasSettings(name string) bool { return m.person(name) }

// cell is the text of one field of a ticket as a table shows it, which
// follows the settings of the column.
func (m *Model) cell(t ticket.Ticket, name string) string {
	value := t.Field(name).String()
	if m.person(name) && m.prefs.Column(name).FirstName {
		return firstNames(value)
	}
	return value
}

// firstNames cuts each name in a list of people down to its first word.
func firstNames(value string) string {
	people := strings.Split(value, ", ")
	for i, p := range people {
		if words := strings.Fields(p); len(words) > 0 {
			people[i] = words[0]
		}
	}
	return strings.Join(people, ", ")
}

// columnMenu opens the settings of a column: one line for each setting,
// with a box that says whether it is on. A click on the line changes it.
func (m *Model) columnMenu(name string, x, y int) {
	if !m.person(name) {
		return
	}
	c := m.prefs.Column(name)
	box := "[ ] "
	if c.FirstName {
		box = "[x] "
	}
	m.showMenu(m.label(name), x, y, []menuItem{{box + "First names only", func() tea.Cmd {
		c.FirstName = !c.FirstName
		if err := m.prefs.SetColumn(name, c); err != nil {
			m.notice = "The setting is on for now, but saving it failed: " + err.Error()
		}
		return nil
	}}})
}
