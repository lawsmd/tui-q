package ui

import (
	"fmt"

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
	root := lipgloss.NewLayer(blank(w, h))
	if w < minWidth || h < minHeight {
		notice := fmt.Sprintf("Pane too small (%dx%d).\nNeeds %dx%d.", w, h, minWidth, minHeight)
		root = lipgloss.NewLayer(lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, notice))
		return lipgloss.NewCompositor(root)
	}

	// A profile with one queue has one tab row: the queue, then the open
	// tickets. With more queues the tickets have a row of their own.
	queueTabs, end, limit := m.queueTabRow(w)
	root.AddLayers(queueTabs...)
	top := 1
	if len(m.queues) == 1 {
		root.AddLayers(m.ticketTabs(end, 0, limit)...)
	} else {
		top = 2
		root.AddLayers(lipgloss.NewLayer(m.styles.SubTabRow.Render(blank(w, 1))).Y(1).Z(zScenery))
		root.AddLayers(m.ticketTabs(0, 1, w)...)
	}

	// One blank line sets the tab rows off from what is under them.
	top++
	content := area{x: 0, y: top, w: w, h: h - top - 1}
	if m.activeTab >= 0 {
		root.AddLayers(m.tabs[m.activeTab].layers(m, content)...)
	} else {
		root.AddLayers(m.queues[m.activeQueue].layers(m, content)...)
	}

	root.AddLayers(m.statusBar(w).Y(h - 1).Z(zItem))
	if m.menu != nil {
		root.AddLayers(m.menuLayers()...)
	}
	if m.form != nil {
		root.AddLayers(m.formLayers()...)
	}
	if m.drag != nil && m.activeTab < 0 {
		if l := m.dragLayer(); l != nil {
			root.AddLayers(l)
		}
	}
	return lipgloss.NewCompositor(root)
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
			names[i] = fmt.Sprintf("%s (%d)", q.cfg.Name, len(q.tickets))
		}
	}

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
func (m *Model) ticketTabs(x, y, limit int) []*lipgloss.Layer {
	s := m.styles
	var layers []*lipgloss.Layer
	if len(m.tabs) == 0 {
		hint := s.SubTab.Render("click a ticket to open it here")
		if x+lipgloss.Width(hint) <= limit {
			layers = append(layers, lipgloss.NewLayer(hint).X(x).Y(y).Z(zItem))
		}
		return layers
	}
	w := limit
	for i, tab := range m.tabs {
		style := s.SubTab
		if i == m.activeTab {
			style = s.SubTabActive
		}
		label := style.Render(tab.t.Key)
		closer := s.SubTabClose.Render("x ")
		lw, cw := lipgloss.Width(label), lipgloss.Width(closer)
		if x+lw+cw > w {
			break
		}
		layers = append(layers,
			lipgloss.NewLayer(label).ID(layerID(idTicketTab, i)).X(x).Y(y).Z(zItem),
			lipgloss.NewLayer(closer).ID(layerID(idTabClose, i)).X(x+lw).Y(y).Z(zItem),
		)
		x += lw + cw
	}
	return layers
}

// statusBar is the last row. It shows the running operation, or the last
// failure until it is clicked away, or a summary of what is on screen.
func (m *Model) statusBar(w int) *lipgloss.Layer {
	s := m.styles
	var left string
	style := s.Status
	if cur, ok := m.ops.Current(); ok {
		style = s.StatusBusy
		left = m.spin.View() + " " + cur.Label
		if step := cur.Step(); step != "" {
			left += ": " + step
		}
		if cur.Write {
			left += " (input locked)"
		}
	} else if failed, ok := m.ops.Failed(); ok {
		style = s.StatusError
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
			right = fmt.Sprintf("%d tickets, loaded %s", len(q.tickets), q.loadedAt.Format("15:04:05"))
		}
	}
	rw := lipgloss.Width(right)
	if rw > 0 && rw+20 > w {
		right, rw = "", 0
	}
	line := " " + fit(left, w-rw-3) + " " + right + " "
	return lipgloss.NewLayer(style.Render(fit(line, w))).ID(idStatus)
}
