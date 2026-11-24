package ui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lawsmd/tui-q/profile"
)

// Z-order of the frame. Lip Gloss gives no order to layers that share a z,
// so every layer is put on one of these levels above the blank background.
const (
	zScenery  = 1
	zItem     = 2
	zButton   = 3
	zMenu     = 10
	zMenuItem = 11
	zDrag     = 20
)

// area is a rectangle of the screen handed to a view to draw in.
type area struct{ x, y, w, h int }

// compose builds the frame as a tree of layers. Layers with an ID can be
// clicked; the rest are scenery.
func (m *Model) compose() *lipgloss.Compositor {
	w, h := m.w, m.h
	if w <= 0 || h <= 0 {
		return lipgloss.NewCompositor(lipgloss.NewLayer(""))
	}
	if w < minWidth || h < minHeight {
		notice := fmt.Sprintf("Pane too small (%dx%d).\nNeeds %dx%d.", w, h, minWidth, minHeight)
		root := lipgloss.NewLayer(lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, notice))
		return lipgloss.NewCompositor(root)
	}
	// Every layer goes in through lit, which swaps the colours of the
	// control the mouse is over. The layers are gathered and given to the
	// root in one call: Lip Gloss measures every layer of the tree again
	// each time layers are added to it.
	var all []*lipgloss.Layer
	add := func(layers ...*lipgloss.Layer) {
		for _, l := range layers {
			all = append(all, m.lit(l))
		}
	}

	// A profile with one queue has one tab row: the queue, then the open
	// tickets. With more queues the tickets have a row of their own.
	if m.only != "" {
		add(m.onlyRow(w)...)
	} else if queueTabs, end, limit := m.queueTabRow(w); len(m.queues) == 1 {
		add(queueTabs...)
		add(m.ticketTabs(end+tabGap, 0, limit)...)
	} else {
		add(queueTabs...)
		add(lipgloss.NewLayer(m.styles.SubTabRow.Render(blank(w, 1))).Y(1).Z(zScenery))
		add(m.ticketTabs(tabGap, 1, w)...)
	}

	content := m.contentArea()
	if m.activeTab >= 0 {
		add(m.tabs[m.activeTab].layers(m, content)...)
	} else if m.only == "" {
		add(m.queues[m.activeQueue].layers(m, content)...)
	}

	add(m.statusBar(w).Y(m.floor()).Z(zItem))
	add(m.tooltipLayers()...)
	if m.menu != nil {
		add(m.menuLayers()...)
	}
	if m.picker != nil {
		add(m.pickerLayers()...)
	}
	if m.form != nil {
		add(m.formLayers()...)
	}
	if m.drag != nil && m.activeTab < 0 {
		if l := m.dragLayer(); l != nil {
			add(l)
		}
	}
	return lipgloss.NewCompositor(lipgloss.NewLayer(blank(w, h), all...))
}

// contentArea is the part of the pane between the tab rows and the status
// bar. One blank line sets the tab rows off from what is under them.
func (m *Model) contentArea() area {
	top := 2
	if m.only == "" && len(m.queues) != 1 {
		top = 3
	}
	return area{x: 0, y: top, w: m.w, h: m.floor() - top}
}

// onlyKey is the key of the ticket on show in a program started on one
// ticket, or the ID of that ticket until it has loaded.
func (m *Model) onlyKey() string {
	if m.activeTab >= 0 && m.tabs[m.activeTab].t.Key != "" {
		return m.tabs[m.activeTab].t.Key
	}
	return m.only
}

// onlyRow is the top row of a program started on one ticket: a tab for
// each open ticket, the jump box and a refresh button. No queue has a tab.
// Until the first ticket has loaded, its name stands where its tab will be.
func (m *Model) onlyRow(w int) []*lipgloss.Layer {
	s := m.styles
	layers := []*lipgloss.Layer{lipgloss.NewLayer(s.TabRow.Render(blank(w, 1))).Z(zScenery)}
	button := s.Button.Render("refresh")
	buttonX := w - lipgloss.Width(button)
	layers = append(layers, lipgloss.NewLayer(button).ID(idRefresh).X(buttonX).Z(zItem))

	// The jump box takes what the tabs leave. It is kept as a button when
	// they leave nothing, since it is the way to a ticket no link names.
	waiting := s.TabActive.Render(m.only)
	need := lipgloss.Width(waiting)
	if len(m.tabs) > 0 {
		need = tabGap
		for _, tab := range m.tabs {
			need += lipgloss.Width(s.SubTab.Render(tab.t.Key)) + lipgloss.Width(tabCloser) + tabGap
		}
	}
	right := buttonX - 1
	need = clamp(need, 0, right-lipgloss.Width(s.Button.Render(jumpButton))-1)
	jump, jumpX := m.jumpLayer(right, need)
	if jump != nil {
		layers = append(layers, jump)
	}
	if len(m.tabs) == 0 {
		if lipgloss.Width(waiting) < jumpX {
			layers = append(layers, lipgloss.NewLayer(waiting).Z(zItem))
		}
		return layers
	}
	return append(layers, m.ticketTabs(tabGap, 0, jumpX-1)...)
}

// lastTab reports whether one ticket is open in a program started on one
// ticket. That tab cannot be closed: no queue is there to show in its place.
func (m *Model) lastTab() bool {
	return m.only != "" && len(m.tabs) <= 1
}

// queueTabRow is the top row: one tab per queue and a refresh button. It
// also returns where the tabs end and where the room for more tabs ends.
func (m *Model) queueTabRow(w int) (row []*lipgloss.Layer, end, limit int) {
	s := m.styles
	layers := []*lipgloss.Layer{lipgloss.NewLayer(s.TabRow.Render(blank(w, 1))).Z(zScenery)}

	button := s.Button.Render("refresh")
	buttonX := w - lipgloss.Width(button)
	layers = append(layers, lipgloss.NewLayer(button).ID(idRefresh).X(buttonX).Z(zItem))
	// A table has a gear beside the refresh button for the settings of the
	// view: which columns it shows and how.
	if m.activeTab < 0 && m.queues[m.activeQueue].cfg.Layout == profile.LayoutTable {
		gear := s.Button.Render(gearGlyph)
		buttonX -= lipgloss.Width(gear) + 1
		layers = append(layers, lipgloss.NewLayer(gear).ID(idGear).X(buttonX).Z(zItem))
	}

	// Each tab carries the number of tickets in its queue, in brackets to
	// keep it apart from the name.
	names := make([]string, len(m.queues))
	for i, q := range m.queues {
		names[i] = q.cfg.Name
		if len(q.tickets) > 0 || !q.loadedAt.IsZero() {
			names[i] = fmt.Sprintf("%s (%d)", q.cfg.Name, q.count())
		}
	}

	// The jump box takes what the queue tabs leave.
	need := 0
	for _, name := range names {
		need += lipgloss.Width(s.Tab.Render(name))
	}
	jump, jumpX := m.jumpLayer(buttonX-1, need)
	if jump != nil {
		layers = append(layers, jump)
	}
	buttonX = jumpX

	x := 0
	for i := range m.queues {
		style := s.Tab
		if i == m.activeQueue && m.activeTab < 0 {
			style = s.TabActive
		}
		tab := style.Render(names[i])
		tw := lipgloss.Width(tab)
		if x+tw > buttonX-1 {
			break
		}
		layers = append(layers, lipgloss.NewLayer(tab).ID(layerID(idQueueTab, i)).X(x).Z(zItem))
		x += tw
	}
	return layers, x, buttonX - 1
}

// ticketTabs is one closable tab per open ticket, drawn on row y from x
// up to limit.
//
// Each tab is a block of its own colour with one cell of the row showing
// between it and the next. The tab on show takes the accent colour, and a
// tab being dragged the colour of a carried card. The place of each tab is
// kept in m.tabSpans for the drag that reorders them. A tab that cannot be
// closed has no mark to close it.
func (m *Model) ticketTabs(x, y, limit int) []*lipgloss.Layer {
	s := m.styles
	var layers []*lipgloss.Layer
	m.tabSpans = m.tabSpans[:0]
	closable := !m.lastTab()
	for i, tab := range m.tabs {
		style, closeStyle := s.SubTab, s.SubTabClose
		switch {
		case m.tabDrag != nil && i == m.tabDrag.index:
			style = s.SubTabDrag
			closeStyle = style.Padding(0)
		case i == m.activeTab:
			style = s.SubTabActive
			closeStyle = style.Padding(0)
		}
		label := style.Render(tab.t.Key)
		closer := ""
		if closable {
			closer = closeStyle.Render(tabCloser)
		}
		lw, cw := lipgloss.Width(label), lipgloss.Width(closer)
		if x+lw+cw > limit {
			break
		}
		layers = append(layers, lipgloss.NewLayer(label).ID(layerID(idTicketTab, i)).X(x).Y(y).Z(zItem))
		if closable {
			layers = append(layers, lipgloss.NewLayer(closer).ID(layerID(idTabClose, i)).X(x+lw).Y(y).Z(zItem))
		}
		m.tabSpans = append(m.tabSpans, span{x: x, w: lw + cw})
		x += lw + cw + tabGap
	}
	return layers
}

// tabGap is the room between two ticket tabs, and tabCloser the mark at
// the end of a tab that closes it.
const (
	tabGap    = 1
	tabCloser = "x "
)

// span is the cells a ticket tab takes on its row.
type span struct{ x, w int }

// tabDrag is a ticket tab being dragged along its row.
type tabDrag struct {
	// index is where the tab is now, and from where it was picked up.
	index, from int
}

// moved returns s with the item at from put at to.
func moved[T any](s []T, from, to int) []T {
	item := s[from]
	s = slices.Delete(s, from, from+1)
	return slices.Insert(s, to, item)
}

// moveTab puts the tab at from in place to. The ticket on show stays on show.
func (m *Model) moveTab(from, to int) {
	if from == to || from < 0 || to < 0 || from >= len(m.tabs) || to >= len(m.tabs) {
		return
	}
	var shown *ticketTab
	if m.activeTab >= 0 {
		shown = m.tabs[m.activeTab]
	}
	m.tabs = moved(m.tabs, from, to)
	m.activeTab = slices.Index(m.tabs, shown)
}

// startTabDrag picks up ticket tab n and shows its ticket.
func (m *Model) startTabDrag(n int) {
	if n < 0 || n >= len(m.tabs) {
		return
	}
	m.notice = ""
	m.activeTab = n
	m.tabDrag = &tabDrag{index: n, from: n}
}

// dragTabTo moves the dragged tab for a pointer at column x. The tab takes
// the place of the tab under the pointer, but only once it would itself be
// under the pointer afterwards. Without that rule a narrow tab dragged onto
// a wide one would change places with it again on every movement.
func (m *Model) dragTabTo(x int) {
	d := m.tabDrag
	if d.index >= len(m.tabSpans) {
		return
	}
	own := m.tabSpans[d.index]
	for j, sp := range m.tabSpans {
		if j == d.index || x < sp.x || x >= sp.x+sp.w {
			continue
		}
		if (j > d.index && x >= sp.x+sp.w-own.w) || (j < d.index && x < sp.x+own.w) {
			m.moveTab(d.index, j)
			// More movement may arrive before the next frame is drawn, so
			// the places are worked out again here.
			at := m.tabSpans[0].x
			m.tabSpans = moved(m.tabSpans, d.index, j)
			for k := range m.tabSpans {
				m.tabSpans[k].x = at
				at += m.tabSpans[k].w + tabGap
			}
			d.index = j
		}
		return
	}
}

// cancelTabDrag ends the drag and puts the tab back where it was picked up.
func (m *Model) cancelTabDrag() {
	if m.tabDrag == nil {
		return
	}
	m.moveTab(m.tabDrag.index, m.tabDrag.from)
	m.tabDrag = nil
}

// statusRows is the height of the status bar: a rule, then its line.
const statusRows = 2

// floor is the row the status bar starts on. Nothing else is drawn on it
// or under it, so the bar is in view whatever is open over the rest.
func (m *Model) floor() int { return m.h - statusRows }

// statusBar is the last rows: a rule across the pane, then a line that
// shows the operation the user is waiting on, or the last failure until it
// is clicked away, or an operation running in the background, or a summary
// of what is on screen. The rule takes the colour of the line under it.
func (m *Model) statusBar(w int) *lipgloss.Layer {
	s := m.styles
	var left string
	style, rule := s.Status, s.StatusRule
	cur, running := m.ops.Current()
	failed, failure := m.ops.Failed()
	if running && (m.blocked() || !failure) {
		style, rule = s.StatusBusy, s.StatusRuleBusy
		left = m.spin.View() + " " + cur.Label
		if step := cur.Step(); step != "" {
			left += ": " + step
		}
		if others := m.ops.Count() - 1; others > 0 {
			left += fmt.Sprintf("  +%d more", others)
		}
	} else if failure {
		style, rule = s.StatusError, s.StatusRuleError
		left = "Failed: " + failed.Err.Error() + "  (click to dismiss)"
	} else if m.notice != "" {
		left = m.notice
	} else {
		left = "Ready"
	}

	right := ""
	if m.activeTab < 0 {
		q := m.queues[m.activeQueue]
		if !q.loadedAt.IsZero() {
			right = fmt.Sprintf("%d tickets, loaded %s", q.count(), q.loadedAt.Format("15:04:05"))
		}
	}
	rw := lipgloss.Width(right)
	if rw > 0 && rw+20 > w {
		right, rw = "", 0
	}
	line := " " + fit(left, w-rw-3) + " " + right + " "
	return lipgloss.NewLayer(rule.Render(strings.Repeat("─", w)) + "\n" + style.Render(fit(line, w))).ID(idStatus)
}
