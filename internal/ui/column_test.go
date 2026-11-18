package ui

import (
	"strings"
	"testing"
)

func TestColumnGearShowsFirstNamesOnly(t *testing.T) {
	m := newModel(t, 110, 30)
	clickLayer(t, m, layerID(idQueueTab, 1))
	q := m.queues[1]
	q.tickets[0].Assignee = "Sam Example"

	// The assignee is the fourth column of a queue with the stock columns.
	// No other column has settings.
	gear := layerID(idGear, 3)
	m.View()
	for i := range 3 {
		if m.comp.GetLayer(layerID(idGear, i)) != nil {
			t.Errorf("column %d has a gear", i)
		}
	}
	if out := screen(m); !strings.Contains(out, "Assignee "+gearGlyph) || !strings.Contains(out, "Sam Example") {
		t.Fatalf("before the setting:\n%s", out)
	}

	clickLayer(t, m, gear)
	clickItem(t, m, "[ ] First names only")
	if m.menu != nil {
		t.Fatal("the menu stayed open")
	}
	if out := screen(m); strings.Contains(out, "Sam Example") || !strings.Contains(out, "Sam") {
		t.Fatalf("with the setting on:\n%s", out)
	}
	checkFrames(t, m, "table with first names")

	clickLayer(t, m, gear)
	clickItem(t, m, "[x] First names only")
	if out := screen(m); !strings.Contains(out, "Sam Example") {
		t.Fatalf("with the setting off again:\n%s", out)
	}
}

func TestFirstNames(t *testing.T) {
	for in, want := range map[string]string{
		"":                          "",
		"Sam":                       "Sam",
		"Sam Example":               "Sam",
		"Sam Example, Alex van Dyk": "Sam, Alex",
	} {
		if got := firstNames(in); got != want {
			t.Errorf("firstNames(%q) = %q, want %q", in, got, want)
		}
	}
}
