package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// headerLine is the header row of the table on screen.
func headerLine(m *Model) string {
	return strings.Split(screen(m), "\n")[3]
}

func TestViewGearPicksColumnsAndOptions(t *testing.T) {
	m := newModel(t, 110, 30)
	clickLayer(t, m, layerID(idQueueTab, 1))
	q := m.queues[1]
	q.tickets[0].Assignee = "Sam Example"
	if h := headerLine(m); !strings.Contains(h, "Assignee") || strings.Contains(h, "Priority") || strings.Contains(h, gearGlyph) {
		t.Fatalf("header before any change: %q", h)
	}

	clickLayer(t, m, idGear)
	if m.menu == nil || m.menu.title != "All view" {
		t.Fatal("the gear did not open the settings of the view")
	}
	// A heading does nothing and leaves the menu open.
	clickItem(t, m, "Columns")
	if m.menu == nil {
		t.Fatal("a click on a heading closed the menu")
	}

	// Each change leaves the menu open with the new state.
	clickItem(t, m, "[ ] Priority")
	clickItem(t, m, "[x] Status")
	clickItem(t, m, "[ ] Show assignee first name only")
	if m.menu == nil {
		t.Fatal("the menu closed after a change")
	}
	checkFrames(t, m, "view settings")
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})

	h := headerLine(m)
	if !strings.Contains(h, "Priority") || strings.Contains(h, "Status") {
		t.Fatalf("header after the changes: %q", h)
	}
	if out := screen(m); strings.Contains(out, "Sam Example") || !strings.Contains(out, "Sam") {
		t.Fatalf("with first names only:\n%s", out)
	}
	// The other table keeps its own columns.
	clickLayer(t, m, layerID(idQueueTab, 0))
	if h := headerLine(m); !strings.Contains(h, "Type") || !strings.Contains(h, "Created") {
		t.Fatalf("header of the other queue: %q", h)
	}

	// Putting the columns back clears the saved pick.
	clickLayer(t, m, layerID(idQueueTab, 1))
	clickLayer(t, m, idGear)
	clickItem(t, m, "[x] Priority")
	clickItem(t, m, "[ ] Status")
	if got := m.prefs.View("All").Show; len(got) != 0 {
		t.Fatalf("saved pick = %v, want none", got)
	}
}

func TestViewKeepsItsLastColumn(t *testing.T) {
	m := newModel(t, 110, 30)
	clickLayer(t, m, layerID(idQueueTab, 1))
	clickLayer(t, m, idGear)
	for _, label := range []string{"[x] Title", "[x] Status", "[x] Assignee", "[x] Key"} {
		clickItem(t, m, label)
	}
	if got := m.shown(m.queues[1]); len(got) != 1 || got[0] != "key" {
		t.Fatalf("columns = %v, want the last one kept", got)
	}
	checkFrames(t, m, "one column")
}

func TestGearIsOnTablesOnly(t *testing.T) {
	m := newModel(t, 110, 30)
	clickLayer(t, m, layerID(idQueueTab, 2))
	m.View()
	if m.comp.GetLayer(idGear) != nil {
		t.Error("a board has the gear")
	}
	clickLayer(t, m, layerID(idQueueTab, 0))
	clickLayer(t, m, layerID(idRow, 0))
	m.View()
	if m.comp.GetLayer(idGear) != nil {
		t.Error("a ticket view has the gear")
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
