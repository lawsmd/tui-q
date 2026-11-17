package ui

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

// Z-order of the frame. Lip Gloss gives no order to layers that share a z,
// so every layer is put on one of these levels above the blank background.
const (
	zScenery = 1
	zItem    = 2
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

	root.AddLayers(m.queueTabRow(w)...)
	root.AddLayers(m.ticketTabRow(w)...)

	content := area{x: 0, y: 2, w: w, h: h - 3}
	if m.activeTab >= 0 {
		root.AddLayers(m.tabs[m.activeTab].layers(m, content)...)
	} else {
		root.AddLayers(m.queues[m.activeQueue].layers(m, content)...)
	}

	root.AddLayers(m.statusBar(w).Y(h - 1).Z(zItem))
	return lipgloss.NewCompositor(root)
}

// queueTabRow is the top row: one tab per queue and a refresh button.
func (m *Model) queueTabRow(w int) []*lipgloss.Layer {
	s := m.styles
	layers := []*lipgloss.Layer{lipgloss.NewLayer(s.TabRow.Render(blank(w, 1))).Z(zScenery)}

	button := s.Button.Render("refresh")
	buttonX := w - lipgloss.Width(button)
	layers = append(layers, lipgloss.NewLayer(button).ID(idRefresh).X(buttonX).Z(zItem))

	x := 0
	for i, q := range m.queues {
		style := s.Tab
		if i == m.activeQueue && m.activeTab < 0 {
			style = s.TabActive
		}
		label := q.cfg.Name
		if len(q.tickets) > 0 || !q.loadedAt.IsZero() {
			label = fmt.Sprintf("%s %d", label, len(q.tickets))
		}
		tab := style.Render(label)
		tw := lipgloss.Width(tab)
		if x+tw > buttonX-1 {
			break
		}
		layers = append(layers, lipgloss.NewLayer(tab).ID(layerID(idQueueTab, i)).X(x).Z(zItem))
		x += tw
	}
	return layers
}

// ticketTabRow is the second row: one closable tab per open ticket.
func (m *Model) ticketTabRow(w int) []*lipgloss.Layer {
	s := m.styles
	layers := []*lipgloss.Layer{lipgloss.NewLayer(s.SubTabRow.Render(blank(w, 1))).Y(1).Z(zScenery)}
	if len(m.tabs) == 0 {
		hint := s.SubTab.Render("click a ticket to open it here")
		if lipgloss.Width(hint) <= w {
			layers = append(layers, lipgloss.NewLayer(hint).Y(1).Z(zItem))
		}
		return layers
	}
	x := 0
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
			lipgloss.NewLayer(label).ID(layerID(idTicketTab, i)).X(x).Y(1).Z(zItem),
			lipgloss.NewLayer(closer).ID(layerID(idTabClose, i)).X(x+lw).Y(1).Z(zItem),
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
		if cur.Write {
			left += " (input locked)"
		}
	} else if failed, ok := m.ops.Failed(); ok {
		style = s.StatusError
		left = "Failed: " + failed.Err.Error() + "  (click to dismiss)"
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
