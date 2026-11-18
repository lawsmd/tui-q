package ui

import (
	"slices"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/ticket"
)

// gearGlyph is drawn on the button that opens the settings of a view.
const gearGlyph = "☼"

// icons reports whether the profile maps the values of a field to icons.
func (m *Model) icons(name string) bool {
	return len(m.prof.Fields[name].Icons) > 0
}

// withIcon puts the icon of a value before it, for the places that have
// room for both.
func (m *Model) withIcon(name, value string) string {
	if g, ok := m.prof.Fields[name].Icon(value); ok {
		return g + " " + value
	}
	return value
}

// person reports whether a field holds people.
func (m *Model) person(name string) bool {
	return name == ticket.FieldAssignee || m.prof.Fields[name].Type == "user"
}

// cell is the text of one field of a ticket as a table shows it, which
// follows the settings of the field.
func (m *Model) cell(t ticket.Ticket, name string) string {
	value := t.Field(name).String()
	if m.icons(name) {
		g, _ := m.prof.Fields[name].Icon(value)
		return g
	}
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

// choices lists the fields a queue's table can show: the columns the
// profile gives the queue, then the other built-in fields, then the other
// fields of the profile by name.
func (m *Model) choices(q *queue) []string {
	out := append([]string(nil), q.cfg.Columns...)
	add := func(name string) {
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	for _, name := range []string{ticket.FieldKey, ticket.FieldTitle, ticket.FieldStatus, ticket.FieldType,
		ticket.FieldAssignee, ticket.FieldPriority, ticket.FieldCreated, ticket.FieldUpdated} {
		add(name)
	}
	custom := make([]string, 0, len(m.prof.Fields))
	for name := range m.prof.Fields {
		custom = append(custom, name)
	}
	sort.Strings(custom)
	for _, name := range custom {
		add(name)
	}
	return out
}

// shown lists the columns of a queue's table: the user's pick when there
// is one, in the order of choices, and the profile's columns otherwise.
func (m *Model) shown(q *queue) []string {
	pick := m.prefs.View(q.cfg.Name).Show
	if len(pick) == 0 {
		return q.cfg.Columns
	}
	var out []string
	for _, name := range m.choices(q) {
		if slices.Contains(pick, name) {
			out = append(out, name)
		}
	}
	if len(out) == 0 {
		return q.cfg.Columns
	}
	return out
}

// viewMenu opens the settings of the table on screen: a checkbox for each
// column it can show, then the options of the columns that have any. A
// click changes one setting and leaves the menu open.
func (m *Model) viewMenu(x, y int) {
	if m.activeTab >= 0 {
		return
	}
	q := m.queues[m.activeQueue]
	check := func(on bool) string {
		if on {
			return "[x] "
		}
		return "[ ] "
	}
	// again saves a change and draws the menu anew where it was.
	again := func(err error) tea.Cmd {
		offset := 0
		if m.menu != nil {
			offset = m.menu.offset
		}
		m.viewMenu(x, y)
		if m.menu != nil {
			m.menu.offset = offset
		}
		if err != nil {
			m.notice = "The setting is on for now, but saving it failed: " + err.Error()
		}
		return nil
	}

	shown := m.shown(q)
	items := []menuItem{{label: "Columns"}}
	for _, name := range m.choices(q) {
		on := slices.Contains(shown, name)
		items = append(items, menuItem{check(on) + m.label(name), func() tea.Cmd {
			next := append([]string(nil), shown...)
			if on {
				// A table keeps at least one column.
				if len(next) == 1 {
					return again(nil)
				}
				next = slices.DeleteFunc(next, func(s string) bool { return s == name })
			} else {
				next = append(next, name)
			}
			if sameSet(next, q.cfg.Columns) {
				next = nil
			}
			v := m.prefs.View(q.cfg.Name)
			v.Show = next
			return again(m.prefs.SetView(q.cfg.Name, v))
		}})
	}

	var options []menuItem
	for _, name := range m.choices(q) {
		if !m.person(name) {
			continue
		}
		c := m.prefs.Column(name)
		label := check(c.FirstName) + "Show " + strings.ToLower(m.label(name)) + " first name only"
		options = append(options, menuItem{label, func() tea.Cmd {
			c.FirstName = !c.FirstName
			return again(m.prefs.SetColumn(name, c))
		}})
	}
	if v := m.prefs.View(q.cfg.Name); len(v.Widths) > 0 {
		options = append(options, menuItem{"Reset column widths", func() tea.Cmd {
			v.Widths = nil
			return again(m.prefs.SetView(q.cfg.Name, v))
		}})
	}
	if len(options) > 0 {
		items = append(append(items, menuItem{label: "Options"}), options...)
	}
	m.menu = &menu{x: x, y: y, title: q.cfg.Name + " view", items: items}
}

// sameSet reports whether two lists hold the same names in any order.
func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, name := range a {
		if !slices.Contains(b, name) {
			return false
		}
	}
	return true
}
