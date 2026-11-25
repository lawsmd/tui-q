package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/ext"
	"github.com/lawsmd/tui-q/provider/fake"
	"github.com/lawsmd/tui-q/ticket"
)

// sidebarText is the sidebar of the open ticket as plain text.
func sidebarText(m *Model) string {
	side, _ := m.tabs[m.activeTab].sidebarLines(m, sidebarMaxWidth)
	return strings.Join(plainLines(side), "\n")
}

func TestInstanceFieldsFollowTheProfilesOnTheTicketsTheyApplyTo(t *testing.T) {
	m := newModel(t, 120, 40)
	profiles := len(m.prof.Ticket.Sidebar)
	// The profile's sidebar has the labels already.
	m.fields = []ext.Fields{{
		Names:   []string{fake.FieldParent, "labels"},
		Applies: func(t ticket.Ticket) bool { return t.Type == "Task" },
	}}

	// The first row is a bug and the third a task.
	clickLayer(t, m, layerID(idRow, 0))
	if out := sidebarText(m); m.tabs[0].t.Type != "Bug" || strings.Contains(out, "Parent") {
		t.Fatalf("a %s shows the field of a task\n%s", m.tabs[0].t.Type, out)
	}
	clickLayer(t, m, layerID(idQueueTab, 0))
	clickLayer(t, m, layerID(idRow, 2))
	out := sidebarText(m)
	if m.tabs[1].t.Type != "Task" {
		t.Fatalf("row 2 is a %s", m.tabs[1].t.Type)
	}
	labels, parent := strings.Index(out, "Labels"), strings.Index(out, "Parent")
	if labels < 0 || parent < labels {
		t.Fatalf("the parent is not under the profile's fields\n%s", out)
	}
	if strings.Count(out, "Labels") != 1 {
		t.Fatalf("a field of the profile is drawn again\n%s", out)
	}
	if len(m.prof.Ticket.Sidebar) != profiles || len(m.tabs[1].sideFields(m)) != profiles+1 {
		t.Fatalf("sidebar fields %v, profile's %v", m.tabs[1].sideFields(m), m.prof.Ticket.Sidebar)
	}
	// The field can be clicked, as one of the profile's can.
	clickField(t, m, fake.FieldParent)
	if m.picker == nil {
		t.Fatal("a click on the field opened nothing")
	}
	checkFrames(t, m, "ticket with a field of the instance and the search for a ticket")
}

func TestTicketFieldIsPickedFromASearch(t *testing.T) {
	m := newModel(t, 120, 40)
	m.fields = []ext.Fields{{Names: []string{fake.FieldParent}}}
	clickLayer(t, m, layerID(idRow, 0))
	key := m.tabs[0].t.Key
	parent := func() ticket.Value { return stored(t, m, key).Field(fake.FieldParent) }

	// The box opens on the likeliest tickets, which for the fixtures are
	// all the others, and has neither a save nor a clear button.
	clickField(t, m, fake.FieldParent)
	if m.picker == nil || !m.picker.one {
		t.Fatalf("no search for a ticket: %+v", m.picker)
	}
	choices := m.picker.choices()
	if len(choices) != 11 || !strings.HasPrefix(choices[0], "DEMO-102  Export to CSV") {
		t.Fatalf("choices before a search: %q", choices)
	}
	m.View()
	if m.comp.GetLayer(idPickSave) != nil || m.comp.GetLayer(idPickClear) != nil || m.comp.GetLayer(idPickCancel) == nil {
		t.Fatal("the buttons of an empty field are not cancel alone")
	}
	if out := screen(m); !strings.Contains(out, "search tickets") || !strings.Contains(out, "enter picks") {
		t.Fatalf("box does not say how it works\n%s", out)
	}

	// Backspace on an empty line does nothing, and enter picks the ticket
	// under the cursor of a search, which is written at once.
	send(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	typeText(t, m, "webhook")
	if got := strings.Join(m.picker.choices(), "|"); got != "DEMO-104  Webhook retries stop after the first failure" {
		t.Fatalf("choices for a search: %q", got)
	}
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.picker != nil {
		t.Fatal("picking a ticket left the box open")
	}
	if got := parent(); got.ID != "DEMO-104" || got.Text != "DEMO-104  Webhook retries stop after the first failure" {
		t.Fatalf("parent = %+v", got)
	}
	if out := sidebarText(m); !strings.Contains(out, "DEMO-104  Webhook retries") {
		t.Fatalf("sidebar does not show the parent\n%s", out)
	}

	// A click picks one too. A search that finds nothing says so.
	clickField(t, m, fake.FieldParent)
	if out := screen(m); !strings.Contains(out, "DEMO-104  Webhook retries") {
		t.Fatalf("box does not show the ticket the field holds\n%s", out)
	}
	typeText(t, m, "no such ticket")
	if out := screen(m); len(m.picker.choices()) != 0 || !strings.Contains(out, "no tickets found") {
		t.Fatalf("an empty search: %q\n%s", m.picker.choices(), out)
	}
	for range "no such ticket" {
		send(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	clickLayer(t, m, layerID(idPickItem, 0))
	if got := parent(); m.picker != nil || got.ID != "DEMO-102" {
		t.Fatalf("after a click: box %v, parent %+v", m.picker != nil, got)
	}

	// Esc and a click outside close the box and write nothing.
	clickField(t, m, fake.FieldParent)
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	clickField(t, m, fake.FieldParent)
	send(t, m, tea.MouseClickMsg{X: 1, Y: 10, Button: tea.MouseLeft})
	send(t, m, tea.MouseReleaseMsg{X: 1, Y: 10, Button: tea.MouseLeft})
	if got := parent(); m.picker != nil || got.ID != "DEMO-102" {
		t.Fatalf("after esc and a click outside: box %v, parent %+v", m.picker != nil, got)
	}

	// Clear empties a field that holds a ticket.
	clickField(t, m, fake.FieldParent)
	clickLayer(t, m, idPickClear)
	if got := parent(); m.picker != nil || got.String() != "" {
		t.Fatalf("after clear: box %v, parent %+v", m.picker != nil, got)
	}
}
