package ui

import (
	"maps"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/internal/prefs"
)

// resize is the edge of a table column being dragged.
type resize struct {
	queue string
	field string
	// sign is 1 when a move to the right widens the column, -1 when it
	// narrows it.
	sign int
	// x is where the pointer was at the start, width how wide the column
	// was then, and most how wide it can get before the title column is
	// too narrow.
	x, width, most int
	// before is the view as it was, for a drag that is called off.
	before prefs.View
}

// startResize picks up the line after column n of the table on screen.
func (m *Model) startResize(x, n int) {
	q := m.queues[m.activeQueue]
	if n < 0 || n >= len(q.cols)-1 {
		return
	}
	target, sign := q.edge(n)
	c := q.cols[target]
	spare := max(q.cols[q.flex].width-minTitleWidth, 0)
	m.notice = ""
	m.resize = &resize{
		queue: q.cfg.Name, field: c.name, sign: sign,
		x: x, width: c.width, most: max(c.width+spare, minColumnWidth),
		before: m.prefs.View(q.cfg.Name),
	}
}

// resizeTo sizes the column for a pointer at x. Nothing is saved until the
// button is let go.
func (m *Model) resizeTo(x int) {
	r := m.resize
	v := m.prefs.View(r.queue)
	widths := maps.Clone(v.Widths)
	if widths == nil {
		widths = map[string]int{}
	}
	widths[r.field] = clamp(r.width+r.sign*(x-r.x), minColumnWidth, r.most)
	v.Widths = widths
	m.prefs.PutView(r.queue, v)
}

// endResize saves the width the drag ended on.
func (m *Model) endResize() tea.Cmd {
	r := m.resize
	m.resize = nil
	if err := m.prefs.SetView(r.queue, m.prefs.View(r.queue)); err != nil {
		m.notice = "The width is set for now, but saving it failed: " + err.Error()
	}
	return nil
}

// cancelResize puts the widths back as they were before the drag.
func (m *Model) cancelResize() {
	if m.resize == nil {
		return
	}
	m.prefs.PutView(m.resize.queue, m.resize.before)
	m.resize = nil
}
