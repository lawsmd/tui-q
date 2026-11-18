package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// dragEdge drags the line after column n of the table by dx cells and
// lets go, or presses Esc in place of letting go.
func dragEdge(t *testing.T, m *Model, n, dx int, cancel bool) {
	t.Helper()
	m.View()
	layer := m.comp.GetLayer(layerID(idEdge, n))
	if layer == nil {
		t.Fatalf("no edge %d on screen", n)
	}
	x, y := layer.GetX(), layer.GetY()
	send(t, m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	send(t, m, tea.MouseMotionMsg{X: x + dx, Y: y, Button: tea.MouseLeft})
	m.View()
	// A long drag runs into a limit, so the line stops short of the pointer.
	if got := m.comp.GetLayer(layerID(idEdge, n)).GetX(); got != x+dx && dx > -50 && dx < 50 {
		t.Errorf("edge %d is at %d after a drag from %d by %d", n, got, x, dx)
	}
	if cancel {
		send(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
		return
	}
	send(t, m, tea.MouseReleaseMsg{X: x + dx, Y: y, Button: tea.MouseLeft})
}

func TestDragOnAColumnEdgeSizesTheColumn(t *testing.T) {
	m := newModel(t, 110, 30)
	q := m.queues[0]
	m.View()
	was := append([]column(nil), q.cols...)
	flex := q.flex
	if flex < 1 || flex > len(was)-2 {
		t.Fatalf("the test needs a column on each side of the title: %+v", was)
	}
	name := q.cfg.Name

	// Left of the title the line sizes the column before it.
	dragEdge(t, m, 0, 4, false)
	m.View()
	if q.cols[0].width != was[0].width+4 || q.cols[flex].width != was[flex].width-4 {
		t.Fatalf("after widening the first column: %+v, was %+v", q.cols, was)
	}
	if got := m.prefs.View(name).Widths[was[0].name]; got != was[0].width+4 {
		t.Fatalf("saved width = %d", got)
	}
	checkFrames(t, m, "resized column")

	// From the title on it sizes the column after it.
	dragEdge(t, m, flex, -3, false)
	m.View()
	if q.cols[flex+1].width != was[flex+1].width+3 {
		t.Fatalf("after widening the column past the title: %+v, was %+v", q.cols, was)
	}

	// A column stops at its least width, and the title at its own.
	dragEdge(t, m, 0, -100, false)
	m.View()
	if q.cols[0].width != minColumnWidth {
		t.Fatalf("narrowest first column = %d", q.cols[0].width)
	}
	dragEdge(t, m, 0, 100, false)
	m.View()
	if q.cols[flex].width != minTitleWidth || len(q.cols) != len(was) {
		t.Fatalf("widest first column leaves %+v", q.cols)
	}

	// Esc calls a drag off.
	saved := m.prefs.View(name).Widths[was[0].name]
	dragEdge(t, m, 0, -5, true)
	m.View()
	if got := m.prefs.View(name).Widths[was[0].name]; got != saved || m.resize != nil {
		t.Fatalf("width after Esc = %d, want %d", got, saved)
	}

	// The view menu puts the widths back.
	clickLayer(t, m, idGear)
	clickItem(t, m, "Reset column widths")
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	m.View()
	if len(m.prefs.View(name).Widths) != 0 || q.cols[0].width != was[0].width {
		t.Fatalf("after the reset: %+v", q.cols)
	}
}
