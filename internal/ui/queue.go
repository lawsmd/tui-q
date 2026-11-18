package ui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/ticket"
)

// Column width limits for a table.
const (
	maxColumnWidth = 22
	minTitleWidth  = 20
	columnGap      = 2
	// moreWidth is the width of the menu button at the end of every row,
	// and of the pair of scroll arrows above it in the header.
	moreWidth = 5
)

// moreGlyph is drawn on the menu button of a row or card.
const moreGlyph = "⋮"

// queue is one top-level tab and the tickets its query returned.
type queue struct {
	cfg      *profile.Queue
	tickets  []ticket.Ticket
	loading  bool
	loadedAt time.Time
	// offset is the index of the first ticket drawn in a table.
	offset int
	// colOffset is how many unpinned table columns are scrolled off to the
	// left, and laneOffset the same for the lanes of a board.
	colOffset  int
	laneOffset int
	// laneScroll is the first card drawn in each lane, by lane name.
	laneScroll map[string]int

	// Set by the last draw, for the handlers that run before the next one.
	cardW, cardH int
}

// layers draws the queue in the layout its profile asks for.
func (q *queue) layers(m *Model, a area) []*lipgloss.Layer {
	if len(q.tickets) == 0 {
		text := "No tickets."
		if q.loading {
			text = "Loading..."
		}
		return []*lipgloss.Layer{lipgloss.NewLayer(m.styles.Faint.Render(" " + text)).X(a.x).Y(a.y + 1).Z(zScenery)}
	}
	if q.cfg.Layout == profile.LayoutBoard {
		return q.boardLayers(m, a)
	}
	return q.tableLayers(m, a)
}

// scrollSideways moves the table columns or the board lanes by n places.
// The next draw clamps the result.
func (q *queue) scrollSideways(n int) {
	if q.cfg.Layout == profile.LayoutBoard {
		q.laneOffset += n
		return
	}
	q.colOffset += n
}

// tableLayers draws a header, then one clickable layer per visible ticket
// with a menu button at its right end.
func (q *queue) tableLayers(m *Model, a area) []*lipgloss.Layer {
	s := m.styles
	avail := a.w - 1 - moreWidth

	// Scrolling right is allowed only while columns are hidden on the
	// right, so step back from an offset that hides nothing.
	q.colOffset = max(q.colOffset, 0)
	for q.colOffset > 0 {
		if _, hidden := q.columns(m, avail, q.colOffset-1); hidden > 0 {
			break
		}
		q.colOffset--
	}
	cols, hidden := q.columns(m, avail, q.colOffset)

	// A column with settings has a gear after its label, drawn as a layer
	// of its own so it can be clicked.
	header := make([]string, len(cols))
	var gears []*lipgloss.Layer
	x := a.x + 1
	for i, c := range cols {
		label := m.label(c.name)
		header[i] = fit(label, c.width)
		if c.gear && c.width > gearWidth {
			room := c.width - gearWidth
			header[i] = fit(label, room) + blank(gearWidth, 1)
			at := x + min(lipgloss.Width(label), room)
			if at+gearWidth <= a.x+a.w-moreWidth {
				gears = append(gears, lipgloss.NewLayer(s.Arrow.Render(" "+gearGlyph)).ID(layerID(idGear, c.index)).X(at).Y(a.y).Z(zButton))
			}
		}
		x += c.width + columnGap
	}
	gap := strings.Repeat(" ", columnGap)
	layers := []*lipgloss.Layer{
		lipgloss.NewLayer(s.Header.Render(fit(" "+strings.Join(header, gap), a.w))).X(a.x).Y(a.y).Z(zScenery),
	}
	layers = append(layers, gears...)
	layers = append(layers, arrows(s, a.x+a.w-moreWidth, a.y, q.colOffset > 0, hidden > 0)...)

	visible := a.h - 1
	q.offset = clamp(q.offset, 0, len(q.tickets)-visible)
	for row := 0; row < visible && q.offset+row < len(q.tickets); row++ {
		index := q.offset + row
		t := q.tickets[index]
		cells := make([]string, len(cols))
		for i, c := range cols {
			cells[i] = fit(m.cell(t, c.name), c.width)
		}
		style := s.Row
		if index%2 == 1 {
			style = s.RowAlt
		}
		y := a.y + 1 + row
		line := style.Render(fit(" "+strings.Join(cells, gap), a.w))
		more := style.Inherit(s.More).Render("  " + moreGlyph + "  ")
		layers = append(layers,
			lipgloss.NewLayer(line).ID(layerID(idRow, index)).X(a.x).Y(y).Z(zItem),
			lipgloss.NewLayer(more).ID(layerID(idMore, index)).X(a.x+a.w-moreWidth).Y(y).Z(zButton),
		)
	}
	return layers
}

// arrows is the pair of sideways scroll buttons, moreWidth cells wide. A
// direction with nothing to scroll to is drawn faint and cannot be clicked.
func arrows(s Styles, x, y int, left, right bool) []*lipgloss.Layer {
	l := lipgloss.NewLayer(s.ArrowOff.Render(" ‹")).X(x).Y(y).Z(zButton)
	if left {
		l = lipgloss.NewLayer(s.Arrow.Render(" ‹")).ID(idLeft).X(x).Y(y).Z(zButton)
	}
	r := lipgloss.NewLayer(s.ArrowOff.Render("› ")).X(x + 3).Y(y).Z(zButton)
	if right {
		r = lipgloss.NewLayer(s.Arrow.Render("› ")).ID(idRight).X(x + 3).Y(y).Z(zButton)
	}
	return []*lipgloss.Layer{l, r}
}

type column struct {
	name  string
	width int
	// index is the column's place in the profile's list.
	index int
	// gear says the column has settings.
	gear bool
}

// columns picks the columns that fit in width cells and sizes them, and
// reports how many were left out on the right.
//
// Every column is as wide as its longest value, up to a cap; the title
// column takes what is left. When the title would get too little room,
// columns are dropped from the end of the list until it fits. The first
// column and the title are pinned. Offset skips that many of the other
// columns from the left, which is how the table scrolls sideways.
func (q *queue) columns(m *Model, width, offset int) (cols []column, hidden int) {
	var picked []column
	for i, name := range q.cfg.Columns {
		if i > 0 && name != ticket.FieldTitle && offset > 0 {
			offset--
			continue
		}
		picked = append(picked, column{name: name, index: i, gear: m.hasSettings(name)})
	}
	for {
		cols = append([]column(nil), picked...)
		used := columnGap * (len(cols) - 1)
		flex := -1
		for i, c := range cols {
			if c.name == ticket.FieldTitle {
				flex = i
				continue
			}
			w := lipgloss.Width(c.name)
			if c.gear {
				w += gearWidth
			}
			for _, t := range q.tickets {
				w = max(w, lipgloss.Width(m.cell(t, c.name)))
			}
			cols[i].width = min(w, maxColumnWidth)
			used += cols[i].width
		}
		if flex < 0 {
			// With no title column the last column takes the slack.
			flex = len(cols) - 1
			used -= cols[flex].width
		}
		rest := width - used
		if rest >= minTitleWidth || len(cols) <= 2 {
			cols[flex].width = max(rest, 1)
			return cols, hidden
		}
		// Drop the last column that is not the title.
		drop := len(picked) - 1
		if picked[drop].name == ticket.FieldTitle {
			drop--
		}
		picked = append(picked[:drop], picked[drop+1:]...)
		hidden++
	}
}
