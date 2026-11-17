// Package hit turns raw mouse messages into gestures on named layers.
//
// The UI gives every clickable thing a layer ID. A Tracker is told which
// layer sits under a cell and reports clicks, right-clicks, drags, drops and
// wheel movement against those IDs.
package hit

import tea "charm.land/bubbletea/v2"

// Kind is the type of a gesture.
type Kind int

// Gestures a Tracker reports.
const (
	// Click is a left press and release with no movement between them.
	Click Kind = iota
	// RightClick is reported on the press of the right button, and of the
	// middle button, which stands in where something between the terminal
	// and the program keeps right-clicks for itself.
	RightClick
	// DragStart is reported once, when a held left button first moves.
	DragStart
	// DragMove follows DragStart for every further movement.
	DragMove
	// Drop ends a drag.
	Drop
	// Wheel is one notch of scrolling.
	Wheel
)

// Event is one gesture.
type Event struct {
	Kind Kind
	// ID is the layer where the gesture began. For Click, RightClick and
	// Wheel that is the layer under the pointer.
	ID string
	// Over is the layer under the pointer now. During a drag it is the
	// candidate drop target.
	Over string
	X, Y int
	// DX and DY are the wheel direction: -1, 0 or 1.
	DX, DY int
}

// At reports the ID of the top layer at a cell, or "" when there is none.
type At func(x, y int) string

// Tracker holds the state of the gesture in progress. The zero value is ready.
type Tracker struct {
	pressed  bool
	dragging bool
	id       string
	x, y     int
}

// Dragging reports whether a drag is in progress.
func (t *Tracker) Dragging() bool { return t.dragging }

// Cancel abandons the gesture in progress without reporting a Drop.
func (t *Tracker) Cancel() { *t = Tracker{} }

// Handle consumes one mouse message and returns the gestures it completes.
func (t *Tracker) Handle(msg tea.MouseMsg, at At) []Event {
	m := msg.Mouse()
	switch msg.(type) {
	case tea.MouseClickMsg:
		switch m.Button {
		case tea.MouseLeft:
			*t = Tracker{pressed: true, id: at(m.X, m.Y), x: m.X, y: m.Y}
		case tea.MouseRight, tea.MouseMiddle:
			id := at(m.X, m.Y)
			return []Event{{Kind: RightClick, ID: id, Over: id, X: m.X, Y: m.Y}}
		}

	case tea.MouseMotionMsg:
		if !t.pressed || (m.X == t.x && m.Y == t.y && !t.dragging) {
			return nil
		}
		over := at(m.X, m.Y)
		move := Event{Kind: DragMove, ID: t.id, Over: over, X: m.X, Y: m.Y}
		if t.dragging {
			return []Event{move}
		}
		t.dragging = true
		return []Event{{Kind: DragStart, ID: t.id, Over: t.id, X: t.x, Y: t.y}, move}

	case tea.MouseReleaseMsg:
		if !t.pressed {
			return nil
		}
		was := *t
		*t = Tracker{}
		over := at(m.X, m.Y)
		if was.dragging {
			return []Event{{Kind: Drop, ID: was.id, Over: over, X: m.X, Y: m.Y}}
		}
		// A release on a different layer than the press is not a click.
		if over == was.id {
			return []Event{{Kind: Click, ID: was.id, Over: over, X: m.X, Y: m.Y}}
		}

	case tea.MouseWheelMsg:
		ev := Event{Kind: Wheel, X: m.X, Y: m.Y}
		ev.ID = at(m.X, m.Y)
		ev.Over = ev.ID
		switch m.Button {
		case tea.MouseWheelUp:
			ev.DY = -1
		case tea.MouseWheelDown:
			ev.DY = 1
		case tea.MouseWheelLeft:
			ev.DX = -1
		case tea.MouseWheelRight:
			ev.DX = 1
		default:
			return nil
		}
		// Shift turns an up-and-down wheel into a sideways one.
		if m.Mod.Contains(tea.ModShift) {
			ev.DX, ev.DY = ev.DY, 0
		}
		return []Event{ev}
	}
	return nil
}
