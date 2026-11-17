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
)

// queue is one top-level tab and the tickets its query returned.
type queue struct {
	cfg      *profile.Queue
	tickets  []ticket.Ticket
	loading  bool
	loadedAt time.Time
	// offset is the index of the first ticket drawn.
	offset int
}

// layers draws the queue as a table: a header, then one clickable layer
// per visible ticket.
func (q *queue) layers(m *Model, a area) []*lipgloss.Layer {
	s := m.styles
	if len(q.tickets) == 0 {
		text := "No tickets."
		if q.loading {
			text = "Loading..."
		}
		return []*lipgloss.Layer{lipgloss.NewLayer(s.Faint.Render(" " + text)).X(a.x).Y(a.y + 1).Z(zScenery)}
	}

	cols := q.columns(a.w - 1)
	header := make([]string, len(cols))
	for i, c := range cols {
		header[i] = fit(m.prof.Label(c.name), c.width)
	}
	gap := strings.Repeat(" ", columnGap)
	layers := []*lipgloss.Layer{
		lipgloss.NewLayer(s.Header.Render(fit(" "+strings.Join(header, gap), a.w))).X(a.x).Y(a.y).Z(zScenery),
	}

	visible := a.h - 1
	q.offset = clamp(q.offset, 0, len(q.tickets)-visible)
	for row := 0; row < visible && q.offset+row < len(q.tickets); row++ {
		index := q.offset + row
		t := q.tickets[index]
		cells := make([]string, len(cols))
		for i, c := range cols {
			cells[i] = fit(t.Field(c.name).String(), c.width)
		}
		style := s.Row
		if index%2 == 1 {
			style = s.RowAlt
		}
		line := style.Render(fit(" "+strings.Join(cells, gap), a.w))
		layers = append(layers, lipgloss.NewLayer(line).ID(layerID(idRow, index)).X(a.x).Y(a.y+1+row).Z(zItem))
	}
	return layers
}

type column struct {
	name  string
	width int
}

// columns picks the columns that fit in width cells and sizes them. Every
// column is as wide as its longest value, up to a cap; the title column
// takes what is left. When the title would get too little room, columns are
// dropped from the end of the profile's list until it fits.
func (q *queue) columns(width int) []column {
	names := append([]string(nil), q.cfg.Columns...)
	for {
		cols := make([]column, len(names))
		used := columnGap * (len(names) - 1)
		flex := -1
		for i, name := range names {
			cols[i].name = name
			if name == ticket.FieldTitle {
				flex = i
				continue
			}
			w := lipgloss.Width(name)
			for _, t := range q.tickets {
				w = max(w, lipgloss.Width(t.Field(name).String()))
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
		if rest >= minTitleWidth || len(names) <= 2 {
			cols[flex].width = max(rest, 1)
			return cols
		}
		// Drop the last column that is not the title.
		drop := len(names) - 1
		if names[drop] == ticket.FieldTitle {
			drop--
		}
		names = append(names[:drop], names[drop+1:]...)
	}
}
