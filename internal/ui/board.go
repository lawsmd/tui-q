package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lawsmd/tui-q/ticket"
)

// Lane width limits for a board.
const (
	minLaneWidth = 26
	maxLaneWidth = 46
)

// noLane names the lane for tickets whose group field is empty.
const noLane = "(none)"

// titleLines is how many lines a card gives the title.
const titleLines = 2

// lane is one column of a board.
type lane struct {
	name string
	// tickets are indexes into the queue's tickets.
	tickets []int
}

// laneOf is the name of the lane a ticket belongs in.
func (q *queue) laneOf(t ticket.Ticket) string {
	if name := t.Field(q.cfg.GroupBy).String(); name != "" {
		return name
	}
	return noLane
}

// lanes groups the tickets. The profile's lanes come first, in its order
// and even when empty. A value the profile does not list gets a lane after
// them, so no ticket is left off the board.
func (q *queue) lanes() []lane {
	var out []lane
	index := map[string]int{}
	for _, name := range q.cfg.Lanes {
		index[strings.ToLower(name)] = len(out)
		out = append(out, lane{name: name})
	}
	for i, t := range q.tickets {
		name := q.laneOf(t)
		n, ok := index[strings.ToLower(name)]
		if !ok {
			n = len(out)
			index[strings.ToLower(name)] = n
			out = append(out, lane{name: name})
		}
		out[n].tickets = append(out[n].tickets, i)
	}
	return out
}

// laneName resolves a layer ID to the lane it lies in: a lane itself, or a
// card or a card's menu button. It returns "" for anything else.
func (q *queue) laneName(id string) string {
	kind, n := splitID(id)
	switch kind {
	case idLane:
		if lanes := q.lanes(); n >= 0 && n < len(lanes) {
			return lanes[n].name
		}
	case idCard, idMore:
		if n >= 0 && n < len(q.tickets) {
			return q.laneOf(q.tickets[n])
		}
	}
	return ""
}

// boardLayers draws the lanes side by side. Each lane is a layer the
// pointer can be over, with a header and a stack of card layers on it.
func (q *queue) boardLayers(m *Model, a area) []*lipgloss.Layer {
	s := m.styles
	lanes := q.lanes()

	visible := clamp((a.w-moreWidth)/minLaneWidth, 1, len(lanes))
	laneW := min((a.w-moreWidth)/visible, maxLaneWidth)
	q.laneOffset = clamp(q.laneOffset, 0, len(lanes)-visible)
	q.cardW = laneW - 1
	q.cardH = q.cardHeight()
	if q.laneScroll == nil {
		q.laneScroll = map[string]int{}
	}

	layers := arrows(s, a.x+a.w-moreWidth, a.y, q.laneOffset > 0, q.laneOffset+visible < len(lanes))

	target := ""
	if m.drag != nil {
		target = m.drag.over
	}
	bodyH := a.h - 1
	for v := 0; v < visible; v++ {
		n := q.laneOffset + v
		ln := lanes[n]
		x := a.x + v*laneW

		head := s.LaneHeader
		if target != "" && strings.EqualFold(target, ln.name) {
			head = s.LaneTarget
		}
		title := fmt.Sprintf(" %s  %d", ln.name, len(ln.tickets))
		layers = append(layers,
			lipgloss.NewLayer(blank(laneW, a.h)).ID(layerID(idLane, n)).X(x).Y(a.y).Z(zScenery),
			// The header has no ID, so the lane under it takes the hit.
			lipgloss.NewLayer(head.Render(fit(title, q.cardW))).X(x).Y(a.y).Z(zItem),
		)

		// When the cards do not all fit, the last row of the lane says
		// how many are out of view.
		fits := bodyH / q.cardH
		if len(ln.tickets) > fits {
			fits = (bodyH - 1) / q.cardH
		}
		key := strings.ToLower(ln.name)
		first := clamp(q.laneScroll[key], 0, len(ln.tickets)-fits)
		q.laneScroll[key] = first
		for i := 0; i < fits && first+i < len(ln.tickets); i++ {
			index := ln.tickets[first+i]
			y := a.y + 1 + i*q.cardH
			border := s.CardBorder
			if m.drag != nil && m.drag.index == index {
				border = s.CardGhost
			}
			card := strings.Join(q.card(m, q.tickets[index], border), "\n")
			layers = append(layers,
				lipgloss.NewLayer(card).ID(layerID(idCard, index)).X(x).Y(y).Z(zItem),
				lipgloss.NewLayer(s.More.Render(" "+moreGlyph+" ")).ID(layerID(idMore, index)).X(x+q.cardW-5).Y(y+1).Z(zButton),
			)
		}
		if above, below := first, len(ln.tickets)-first-max(fits, 0); above > 0 || below > 0 {
			note := fmt.Sprintf(" %d above, %d below", above, below)
			layers = append(layers, lipgloss.NewLayer(s.Faint.Render(fit(note, q.cardW))).X(x).Y(a.y+a.h-1).Z(zItem))
		}
	}
	return layers
}

// cardHeight is the height of every card of the board, border included.
func (q *queue) cardHeight() int {
	h := 2
	for _, name := range q.cfg.Card {
		if name == ticket.FieldTitle {
			h += titleLines
		} else {
			h++
		}
	}
	return h
}

// card draws one ticket as a bordered card cardW cells wide. The first
// line leaves room at its right end for the menu button.
func (q *queue) card(m *Model, t ticket.Ticket, border lipgloss.Style) []string {
	s := m.styles
	inner := q.cardW - 4
	var lines []string
	for _, name := range q.cfg.Card {
		value := t.Field(name).String()
		switch name {
		case ticket.FieldKey:
			lines = append(lines, s.CardKey.Render(fit(value, inner)))
		case ticket.FieldTitle:
			wrapped := strings.Split(lipgloss.Wrap(value, inner, ""), "\n")
			for i := 0; i < titleLines; i++ {
				line := ""
				if i < len(wrapped) {
					line = wrapped[i]
				}
				if i == titleLines-1 && len(wrapped) > titleLines {
					line += "…"
				}
				lines = append(lines, s.CardText.Render(fit(line, inner)))
			}
		default:
			if value == "" {
				value = "no " + strings.ToLower(m.label(name))
			}
			lines = append(lines, s.Faint.Render(fit(m.withIcon(name, value), inner)))
		}
	}
	return box(lines, q.cardW, border)
}

// drag is a card being carried by the pointer.
type drag struct {
	// index is the ticket's place in the active queue.
	index int
	// x and y are the pointer; dx and dy are where in the card it took hold.
	x, y, dx, dy int
	// over is the lane under the pointer, or "" when there is none.
	over string
}

// dragLayer is the copy of the card that follows the pointer. It has no
// ID, so a hit test under it reports the lane.
func (m *Model) dragLayer() *lipgloss.Layer {
	q := m.queues[m.activeQueue]
	d := m.drag
	if d.index >= len(q.tickets) || q.cardW <= 0 {
		return nil
	}
	card := strings.Join(q.card(m, q.tickets[d.index], m.styles.CardDrag), "\n")
	x := clamp(d.x-d.dx, 0, m.w-q.cardW)
	y := clamp(d.y-d.dy, 0, m.h-q.cardH)
	return lipgloss.NewLayer(card).X(x).Y(y).Z(zDrag)
}
