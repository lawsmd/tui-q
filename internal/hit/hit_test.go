package hit

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// at maps column 0-9 to layer "a" and 10-19 to layer "b".
func at(x, _ int) string {
	switch {
	case x < 10:
		return "a"
	case x < 20:
		return "b"
	}
	return ""
}

func kinds(evs []Event) []Kind {
	out := make([]Kind, len(evs))
	for i, e := range evs {
		out[i] = e.Kind
	}
	return out
}

func equal(a, b []Kind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestClick(t *testing.T) {
	var tr Tracker
	if evs := tr.Handle(tea.MouseClickMsg{X: 3, Y: 1, Button: tea.MouseLeft}, at); len(evs) != 0 {
		t.Fatalf("press reported %v", evs)
	}
	evs := tr.Handle(tea.MouseReleaseMsg{X: 3, Y: 1, Button: tea.MouseLeft}, at)
	if !equal(kinds(evs), []Kind{Click}) || evs[0].ID != "a" {
		t.Fatalf("got %+v, want one Click on a", evs)
	}
}

func TestReleaseOnAnotherLayerIsNotAClick(t *testing.T) {
	var tr Tracker
	tr.Handle(tea.MouseClickMsg{X: 3, Y: 1, Button: tea.MouseLeft}, at)
	if evs := tr.Handle(tea.MouseReleaseMsg{X: 25, Y: 1, Button: tea.MouseLeft}, at); len(evs) != 0 {
		t.Fatalf("got %+v, want nothing", evs)
	}
}

func TestDragAndDrop(t *testing.T) {
	var tr Tracker
	tr.Handle(tea.MouseClickMsg{X: 3, Y: 1, Button: tea.MouseLeft}, at)

	evs := tr.Handle(tea.MouseMotionMsg{X: 8, Y: 1, Button: tea.MouseLeft}, at)
	if !equal(kinds(evs), []Kind{DragStart, DragMove}) {
		t.Fatalf("first motion: got %v", kinds(evs))
	}
	if evs[0].X != 3 || evs[0].ID != "a" {
		t.Fatalf("DragStart = %+v, want the press position on a", evs[0])
	}
	if !tr.Dragging() {
		t.Fatal("Dragging() is false during a drag")
	}

	evs = tr.Handle(tea.MouseMotionMsg{X: 14, Y: 1, Button: tea.MouseLeft}, at)
	if !equal(kinds(evs), []Kind{DragMove}) || evs[0].Over != "b" {
		t.Fatalf("second motion: got %+v", evs)
	}

	evs = tr.Handle(tea.MouseReleaseMsg{X: 14, Y: 1, Button: tea.MouseLeft}, at)
	if !equal(kinds(evs), []Kind{Drop}) || evs[0].ID != "a" || evs[0].Over != "b" {
		t.Fatalf("release: got %+v, want Drop from a over b", evs)
	}
	if tr.Dragging() {
		t.Fatal("Dragging() is true after the drop")
	}
}

func TestMotionWithoutPressIsIgnored(t *testing.T) {
	var tr Tracker
	if evs := tr.Handle(tea.MouseMotionMsg{X: 5, Y: 5}, at); len(evs) != 0 {
		t.Fatalf("got %+v", evs)
	}
}

func TestRightClickAndWheel(t *testing.T) {
	var tr Tracker
	evs := tr.Handle(tea.MouseClickMsg{X: 12, Y: 0, Button: tea.MouseRight}, at)
	if !equal(kinds(evs), []Kind{RightClick}) || evs[0].ID != "b" {
		t.Fatalf("right click: got %+v", evs)
	}
	evs = tr.Handle(tea.MouseWheelMsg{X: 2, Y: 0, Button: tea.MouseWheelDown}, at)
	if !equal(kinds(evs), []Kind{Wheel}) || evs[0].DY != 1 || evs[0].ID != "a" {
		t.Fatalf("wheel: got %+v", evs)
	}
}
