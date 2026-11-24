package ui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lawsmd/tui-q/internal/hit"
	"github.com/lawsmd/tui-q/internal/ops"
	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// Limits of the tag box.
const (
	// pickRows is the most search results the box lists at once.
	pickRows = 8
	// pickMaxWidth is the widest the box grows when the field it edits
	// is in a column wider than the sidebar.
	pickMaxWidth = 48
)

// pickDelay is how long the tag box waits after a key before it searches,
// so a word typed quickly runs one search.
var pickDelay = 150 * time.Millisecond

// picker is the box that edits a list of tags. It opens under the label of
// the field, inside the column the field is in, and lists the tags as they
// will be saved, a line to search in, and the tags the search found. It
// takes the keyboard while it is open, and saves the tags in one write.
type picker struct {
	t     ticket.Ticket
	field string
	// start is the tags when the box opened, and items the tags as edited.
	start []string
	items []string
	in    textinput.Model
	// query is the text the results in found were searched for, and seq
	// counts the changes to the text, so a late search is ignored.
	query  string
	found  []string
	seq    int
	cursor int
	offset int
	// at is where the label of the field was drawn when the box was last
	// placed, kept for frames in which the label is scrolled out of view.
	at area
}

// pickSearchMsg asks for a search of the text the tag box held at seq, once
// the box has waited for more keys.
type pickSearchMsg struct{ seq int }

// pickFoundMsg carries the tags a search for query found.
type pickFoundMsg struct {
	seq    int
	query  string
	values []string
}

// openPicker opens the tag box under the label of a field of an open
// ticket.
func (m *Model) openPicker(tab *ticketTab, name string) tea.Cmd {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = "search tags"
	if _, ok := m.prov.(provider.Suggester); !ok {
		in.Placeholder = "type a tag"
	}
	st := in.Styles()
	st.Focused.Text, st.Blurred.Text = m.styles.MenuItem, m.styles.MenuItem
	st.Focused.Placeholder, st.Blurred.Placeholder = m.styles.PickHint, m.styles.PickHint
	in.SetStyles(st)
	current := tab.t.Field(name).Items
	m.picker = &picker{
		t:     tab.t,
		field: name,
		start: slices.Clone(current),
		items: slices.Clone(current),
		in:    in,
		at:    tab.fieldAt[name],
	}
	return m.picker.in.Focus()
}

// changed reports whether the tags differ from those the box opened with.
func (p *picker) changed() bool { return !slices.Equal(p.start, p.items) }

// has reports whether the tags include tag, ignoring case.
func (p *picker) has(tag string) bool {
	return slices.ContainsFunc(p.items, func(s string) bool { return strings.EqualFold(s, tag) })
}

// text is what has been typed, without spaces at either end.
func (p *picker) text() string { return strings.TrimSpace(p.in.Value()) }

// current reports whether the results are those of the text typed.
func (p *picker) current() bool { return p.query == p.text() }

// choices are the lines under the search: the tags found that the ticket
// does not have, then the text typed as a new tag when no tag is spelt
// that way. A search still running has none.
func (p *picker) choices() []string {
	text := p.text()
	if text == "" || !p.current() {
		return nil
	}
	var out []string
	exact := false
	for _, tag := range p.found {
		exact = exact || strings.EqualFold(tag, text)
		if !p.has(tag) {
			out = append(out, tag)
		}
	}
	if !exact && !p.has(text) && !strings.Contains(text, ",") {
		out = append(out, newTag(text))
	}
	return out
}

// newTag is the line that adds text as a tag no search found.
func newTag(text string) string { return fmt.Sprintf("New tag %q", text) }

// add puts a tag on the list and clears the search.
func (m *Model) pickAdd(tag string) {
	p := m.picker
	if tag = strings.TrimSpace(tag); tag != "" && !p.has(tag) {
		p.items = append(p.items, tag)
	}
	p.in.SetValue("")
	p.query, p.found, p.cursor, p.offset = "", nil, 0, 0
	p.seq++
}

// pickChoose adds the choice at n.
func (m *Model) pickChoose(n int) {
	p := m.picker
	choices := p.choices()
	if n < 0 || n >= len(choices) {
		return
	}
	tag := choices[n]
	if n == len(choices)-1 && tag == newTag(p.text()) {
		tag = p.text()
	}
	m.pickAdd(tag)
}

// pickRemove takes the tag at n off the list.
func (m *Model) pickRemove(n int) {
	p := m.picker
	if n >= 0 && n < len(p.items) {
		p.items = slices.Delete(slices.Clone(p.items), n, n+1)
	}
}

// pickSave closes the box and writes the tags when they changed.
func (m *Model) pickSave() tea.Cmd {
	p := m.picker
	m.picker = nil
	if !p.changed() {
		return nil
	}
	return m.setField(p.t, p.field, ticket.Value{Items: slices.Clone(p.items)}, nil)
}

// pickKey handles a key while the tag box is open. Enter adds the result
// under the cursor, or each of a list typed with commas, and saves when
// nothing is typed. Backspace on an empty line takes off the last tag.
func (m *Model) pickKey(msg tea.KeyPressMsg) tea.Cmd {
	p := m.picker
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit
	case "esc":
		m.picker = nil
		return nil
	case "up":
		p.cursor = max(p.cursor-1, 0)
		return nil
	case "down":
		p.cursor = min(p.cursor+1, max(len(p.choices())-1, 0))
		return nil
	case "enter":
		text := p.text()
		switch {
		case text == "":
			return m.pickSave()
		case strings.Contains(text, ","):
			for _, tag := range strings.Split(text, ",") {
				m.pickAdd(tag)
			}
		default:
			m.pickChoose(p.cursor)
		}
		return nil
	case "backspace":
		if p.in.Value() == "" {
			m.pickRemove(len(p.items) - 1)
			return nil
		}
	}
	before := p.in.Value()
	var cmd tea.Cmd
	p.in, cmd = p.in.Update(msg)
	if p.in.Value() == before {
		return cmd
	}
	return tea.Batch(cmd, m.pickTyped())
}

// pickTyped starts the wait before a search of the text just typed.
func (m *Model) pickTyped() tea.Cmd {
	p := m.picker
	p.seq++
	p.cursor, p.offset = 0, 0
	seq := p.seq
	return tea.Tick(pickDelay, func(time.Time) tea.Msg { return pickSearchMsg{seq: seq} })
}

// pickSearch searches for the text of the box if no key came while it
// waited. A platform that cannot search its tags finds none, and the box
// offers the text as a new tag.
func (m *Model) pickSearch(msg pickSearchMsg) tea.Cmd {
	p := m.picker
	if p == nil || msg.seq != p.seq {
		return nil
	}
	query := p.text()
	sg, ok := m.prov.(provider.Suggester)
	if query == "" || !ok {
		p.query, p.found = query, nil
		return nil
	}
	t, field, seq := p.t, p.field, p.seq
	return m.start("Searching for "+query, ops.Background, func(ctx context.Context) (tea.Msg, error) {
		values, err := sg.Suggest(ctx, t.ID, field, query)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t.Key, err)
		}
		return pickFoundMsg{seq: seq, query: query, values: values}, nil
	})
}

// pickFound shows the results of a search, unless the text has changed
// since it started.
func (m *Model) pickFound(msg pickFoundMsg) {
	if p := m.picker; p != nil && msg.seq == p.seq {
		p.query, p.found = msg.query, msg.values
	}
}

// pickGesture handles the mouse while the tag box is open. A click on a
// tag takes it off, a click on a result adds it, and the wheel scrolls the
// results. A click outside the box closes it when nothing has changed, and
// does nothing otherwise, so a stray click loses no edits.
func (m *Model) pickGesture(ev hit.Event) tea.Cmd {
	p := m.picker
	kind, n := splitID(ev.ID)
	switch ev.Kind {
	case hit.Click:
		switch kind {
		case idPickTag:
			m.pickRemove(n)
		case idPickItem:
			m.pickChoose(n)
		case idPickSave:
			return m.pickSave()
		case idPickCancel:
			m.picker = nil
		case idPicker:
		default:
			if !p.changed() {
				m.picker = nil
			}
		}
	case hit.Wheel:
		p.offset += ev.DY
	}
	return nil
}

// pickerLayers draws the tag box under the label of its field, as wide as
// the column the field is in and no wider than pickMaxWidth. Where there is
// no room under the label the box goes above it.
func (m *Model) pickerLayers() []*lipgloss.Layer {
	s := m.styles
	p := m.picker
	if m.activeTab >= 0 {
		if at, ok := m.tabs[m.activeTab].fieldAt[p.field]; ok {
			p.at = at
		}
	}
	w := clamp(p.at.w, min(sidebarMaxWidth, m.w), min(pickMaxWidth, m.w))
	inner := max(w-4, 1)

	type spot struct {
		id   string
		text string
		x, y int
	}
	var lines []string
	var spots []spot

	// The tags, as many to a line as fit, each a button that takes it off.
	if len(p.items) == 0 {
		lines = append(lines, s.PickHint.Render(fit("no tags", inner)))
	}
	var line strings.Builder
	used := 0
	flush := func() {
		lines = append(lines, s.MenuItem.Render(pad(line.String(), inner)))
		line.Reset()
		used = 0
	}
	for i, tag := range p.items {
		chip := s.PickTag.Render(ansi.Truncate(tag, inner-4, "…") + " ×")
		cw := lipgloss.Width(chip)
		if used > 0 && used+1+cw > inner {
			flush()
		}
		if used > 0 {
			line.WriteString(s.MenuItem.Render(" "))
			used++
		}
		spots = append(spots, spot{layerID(idPickTag, i), chip, used, len(lines)})
		line.WriteString(blank(cw, 1))
		used += cw
	}
	if used > 0 {
		flush()
	}

	lines = append(lines, s.MenuBorder.Render(strings.Repeat("─", inner)))
	p.in.SetWidth(inner - 3)
	lines = append(lines, s.MenuItem.Render(pad(s.MenuTitle.Render("+ ")+p.in.View(), inner)))

	// The results, scrolled to keep the cursor in view.
	choices := p.choices()
	switch {
	case p.text() == "":
	case !p.current():
		lines = append(lines, s.PickHint.Render(fit("searching...", inner)))
	case len(choices) == 0:
		lines = append(lines, s.PickHint.Render(fit("the ticket has every tag that matches", inner)))
	}
	rows := min(len(choices), pickRows, max(m.floor()-11, 1))
	p.cursor = clamp(p.cursor, 0, len(choices)-1)
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+rows {
		p.offset = p.cursor - rows + 1
	}
	p.offset = clamp(p.offset, 0, len(choices)-rows)
	for i := range rows {
		n := p.offset + i
		style := s.MenuItem
		if n == p.cursor {
			style = s.PickCursor
		}
		spots = append(spots, spot{layerID(idPickItem, n), style.Render(fit(" "+choices[n], inner)), 0, len(lines)})
		lines = append(lines, s.MenuItem.Render(blank(inner, 1)))
	}
	if len(choices) > rows {
		lines = append(lines, s.PickHint.Render(fit(fmt.Sprintf("%d-%d of %d", p.offset+1, p.offset+rows, len(choices)), inner)))
	}

	hint := "enter adds, esc cancels"
	if p.text() == "" {
		hint = "enter saves, esc cancels"
	}
	lines = append(lines, s.PickHint.Render(fit(hint, inner)))
	save, cancel := s.Button.Render("save"), s.Button.Render("cancel")
	spots = append(spots,
		spot{idPickSave, save, 0, len(lines)},
		spot{idPickCancel, cancel, lipgloss.Width(save) + 1, len(lines)})
	lines = append(lines, s.MenuItem.Render(blank(inner, 1)))

	// In a short pane the box loses lines from the top, the tags first, and
	// keeps the search and the buttons.
	if room := max(m.floor()-2, 1); len(lines) > room {
		cut := len(lines) - room
		lines = lines[cut:]
		kept := spots[:0]
		for _, sp := range spots {
			if sp.y -= cut; sp.y >= 0 {
				kept = append(kept, sp)
			}
		}
		spots = kept
	}
	boxRows := titledBox(s.MenuTitle.Render(m.label(p.field)+" on "+p.t.Key), lines, w, s.MenuBorder)
	x := clamp(p.at.x-2, 0, m.w-w)
	y := p.at.y + 1
	if bottom := m.floor(); y+len(boxRows) > bottom {
		if p.at.y-len(boxRows) >= 0 {
			y = p.at.y - len(boxRows)
		} else {
			y = max(bottom-len(boxRows), 0)
		}
	}
	// The label may be from a frame of another size.
	y = clamp(y, 0, m.floor()-len(boxRows))
	layers := []*lipgloss.Layer{lipgloss.NewLayer(strings.Join(boxRows, "\n")).ID(idPicker).X(x).Y(y).Z(zMenu)}
	for _, sp := range spots {
		layers = append(layers, lipgloss.NewLayer(sp.text).ID(sp.id).X(x+2+sp.x).Y(y+1+sp.y).Z(zMenuItem))
	}
	return layers
}
