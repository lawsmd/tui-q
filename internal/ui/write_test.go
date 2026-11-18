package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider/fake"
	"github.com/lawsmd/tui-q/ticket"
)

// stored reads a ticket straight from the fake provider.
func stored(t *testing.T, m *Model, id string) ticket.Ticket {
	t.Helper()
	got, err := m.prov.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// typeText sends one key press per character.
func typeText(t *testing.T, m *Model, text string) {
	t.Helper()
	for _, r := range text {
		send(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// clickItem clicks the menu item with the given label.
func clickItem(t *testing.T, m *Model, label string) {
	t.Helper()
	if m.menu == nil {
		t.Fatalf("no menu open to click %q in", label)
	}
	for i, it := range m.menu.items {
		if it.label == label {
			clickLayer(t, m, layerID(idMenuItem, i))
			return
		}
	}
	t.Fatalf("menu %q has no item %q", m.menu.title, label)
}

// clickField clicks the label of an editable field of the open ticket.
func clickField(t *testing.T, m *Model, name string) {
	t.Helper()
	m.View()
	for i, f := range m.tabs[m.activeTab].fields {
		if f == name {
			clickLayer(t, m, layerID(idField, i))
			return
		}
	}
	t.Fatalf("field %q cannot be clicked; clickable: %v", name, m.tabs[m.activeTab].fields)
}

func TestDropMovesTheTicket(t *testing.T) {
	m := newModel(t, 130, 40)
	clickLayer(t, m, layerID(idQueueTab, 2))
	q := m.queues[2]
	card := q.lanes()[0].tickets[0]
	key := q.tickets[card].Key

	dragTo(t, m, layerID(idCard, card), layerID(idLane, 2))
	if m.drag != nil || len(m.tabs) != 0 {
		t.Fatalf("after the drop: drag=%v tabs=%d", m.drag != nil, len(m.tabs))
	}
	if got := stored(t, m, key).Status; got != "Waiting" {
		t.Fatalf("status = %q, want Waiting", got)
	}
	if want := "Moved " + key + " to Waiting"; !strings.Contains(screen(m), want) {
		t.Fatalf("status bar lacks %q\n%s", want, screen(m))
	}
	if n := len(m.queues[2].lanes()[2].tickets); n != 3 {
		t.Fatalf("Waiting lane holds %d cards, want 3", n)
	}

	// A drop on the lane the card is already in writes nothing.
	m.notice = ""
	card = m.queues[2].lanes()[0].tickets[0]
	dragTo(t, m, layerID(idCard, card), layerID(idLane, 0))
	if m.notice != "" {
		t.Fatalf("drop on the same lane reported %q", m.notice)
	}
}

func TestFailedDropPutsTheCardBack(t *testing.T) {
	m := newModel(t, 130, 40)
	m.prov.(*fake.Provider).FailWrites = errors.New("the platform said no")
	clickLayer(t, m, layerID(idQueueTab, 2))
	q := m.queues[2]
	card := q.lanes()[0].tickets[0]
	key := q.tickets[card].Key

	dragTo(t, m, layerID(idCard, card), layerID(idLane, 1))
	if got := q.tickets[card].Status; got != "Open" {
		t.Fatalf("card shows %q after a failed move, want Open", got)
	}
	out := screen(m)
	if !strings.Contains(out, "Failed") || !strings.Contains(out, "the platform said no") || !strings.Contains(out, key) {
		t.Fatalf("status bar does not report the failure\n%s", out)
	}
}

func TestMoveThatNeedsAFieldAsksForIt(t *testing.T) {
	m := newModel(t, 130, 40)
	clickLayer(t, m, layerID(idQueueTab, 2))
	q := m.queues[2]
	card := q.lanes()[0].tickets[0]
	key := q.tickets[card].Key

	dragTo(t, m, layerID(idCard, card), layerID(idLane, 3))
	if m.form == nil {
		t.Fatal("no form after a move that needs a resolution")
	}
	if got := q.tickets[card].Status; got != "Open" {
		t.Fatalf("card shows %q while the form is open, want Open", got)
	}
	if got := stored(t, m, key).Status; got != "Open" {
		t.Fatalf("status = %q before the form is saved", got)
	}
	out := screen(m)
	for _, want := range []string{"Move " + key + " to Done", "Resolution", "Fixed", "cancel"} {
		if !strings.Contains(out, want) {
			t.Errorf("form lacks %q\n%s", want, out)
		}
	}

	// The mouse reaches nothing but the cancel button while the form is open.
	clickLayer(t, m, idFormCancel)
	if m.form != nil {
		t.Fatal("cancel left the form open")
	}

	card = m.queues[2].lanes()[0].tickets[0]
	dragTo(t, m, layerID(idCard, card), layerID(idLane, 3))
	send(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.form != nil {
		t.Fatalf("form still open after enter\n%s", screen(m))
	}
	got := stored(t, m, key)
	if got.Status != "Done" || got.Field(fake.FieldResolution).String() != "Won't fix" {
		t.Fatalf("status=%q resolution=%q", got.Status, got.Field(fake.FieldResolution).String())
	}
}

func TestMenuMovesAndAssigns(t *testing.T) {
	m := newModel(t, 110, 30)
	key := m.queues[0].tickets[0].Key

	clickLayer(t, m, layerID(idMore, 0))
	clickItem(t, m, "Assign to me")
	if got := stored(t, m, key).Assignee; got != fake.People[0] {
		t.Fatalf("assignee = %q, want %q", got, fake.People[0])
	}

	clickLayer(t, m, layerID(idMore, 0))
	for _, it := range m.menu.items {
		if it.label == "Assign to me" {
			t.Fatal("menu offers Assign to me for a ticket that is already mine")
		}
	}
	clickItem(t, m, "Assign to...")
	clickItem(t, m, "Someone else...")
	if m.form == nil {
		t.Fatal("no form asking for a name")
	}
	typeText(t, m, "al")
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.form != nil || m.menu == nil {
		t.Fatalf("after the search: form=%v menu=%v\n%s", m.form != nil, m.menu != nil, screen(m))
	}
	clickItem(t, m, "alex")
	if got := stored(t, m, key).Assignee; got != "alex" {
		t.Fatalf("assignee = %q, want alex", got)
	}

	clickLayer(t, m, layerID(idMore, 0))
	clickItem(t, m, "Move to...")
	if m.menu == nil || !strings.Contains(m.menu.title, "Move "+key) {
		t.Fatal("no menu of moves")
	}
	clickItem(t, m, "In progress")
	if got := stored(t, m, key).Status; got != "In progress" {
		t.Fatalf("status = %q", got)
	}
	// The Open queue was reloaded and no longer holds the ticket.
	for _, tk := range m.queues[0].tickets {
		if tk.Key == key {
			t.Fatal("moved ticket is still in the Open queue")
		}
	}
}

func TestTicketViewEditsFields(t *testing.T) {
	m := newModel(t, 120, 40)
	clickLayer(t, m, layerID(idRow, 0))
	key := m.tabs[0].t.Key

	clickLayer(t, m, idMove)
	clickItem(t, m, "Waiting")
	if got := m.tabs[0].t.Status; got != "Waiting" {
		t.Fatalf("tab shows status %q", got)
	}

	clickLayer(t, m, idAssign)
	clickItem(t, m, "Me")
	clickField(t, m, ticket.FieldAssignee)
	clickItem(t, m, "Nobody")
	if got := stored(t, m, key).Assignee; got != "" {
		t.Fatalf("assignee = %q after Nobody", got)
	}

	clickField(t, m, ticket.FieldStatus)
	clickItem(t, m, "Open")

	clickField(t, m, ticket.FieldPriority)
	if m.menu == nil || len(m.menu.items) != len(fake.Priorities) {
		t.Fatalf("priority menu: %+v", m.menu)
	}
	clickItem(t, m, "Low")
	if got := stored(t, m, key).Priority; got != "Low" {
		t.Fatalf("priority = %q", got)
	}

	clickField(t, m, "labels")
	if m.form == nil {
		t.Fatal("no form for a list of tags")
	}
	typeText(t, m, ", billing")
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := stored(t, m, key).Field("labels").String(); got != "auth, billing" {
		t.Fatalf("labels = %q", got)
	}
	if !strings.Contains(screen(m), "auth, billing") {
		t.Fatalf("tab does not show the new labels\n%s", screen(m))
	}

	// Esc closes a form without writing.
	clickField(t, m, "labels")
	typeText(t, m, ", gone")
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.form != nil || stored(t, m, key).Field("labels").String() != "auth, billing" {
		t.Fatal("esc did not abandon the edit")
	}
}

func TestFieldsOffTheTicketScreenAreHidden(t *testing.T) {
	m := newModel(t, 120, 40)
	send(t, m, labelsMsg{labels: map[string]string{"region": "Sales region of the customer", "labels": "Tags"}})
	clickLayer(t, m, layerID(idRow, 2)) // a ticket with no labels
	out := screen(m)
	if strings.Contains(out, "Sales region") || strings.Contains(out, "Region") {
		t.Fatalf("an empty field the ticket's screen lacks is drawn\n%s", out)
	}
	// An empty field that is on the screen stays, under the platform's name.
	if !strings.Contains(out, "Tags"+editMark) {
		t.Fatalf("empty editable field missing or not under the platform's name\n%s", out)
	}
	// The profile's own label wins over the platform's.
	m.prof.Fields = map[string]profile.Field{"labels": {Label: "Mine"}}
	if got := m.label("labels"); got != "Mine" {
		t.Fatalf("label = %q, want the profile's", got)
	}
}

func TestLongMenuScrolls(t *testing.T) {
	m := newModel(t, 60, 10)
	var items []menuItem
	picked := -1
	for i := 0; i < 30; i++ {
		items = append(items, menuItem{label: "option " + string(rune('A'+i)), run: func() tea.Cmd { picked = i; return nil }})
	}
	m.showMenu("Pick", 5, 2, items)
	checkFrames(t, m, "long menu")
	send(t, m, tea.WindowSizeMsg{Width: 60, Height: 10})
	m.View()
	box := m.comp.GetLayer(idMenu)
	for i := 0; i < 12; i++ {
		send(t, m, tea.MouseWheelMsg{X: box.GetX() + 2, Y: box.GetY() + 2, Button: tea.MouseWheelDown})
	}
	if m.menu == nil || m.menu.offset != 12 {
		t.Fatalf("menu offset after scrolling: %+v", m.menu)
	}
	clickLayer(t, m, layerID(idMenuItem, 12))
	if picked != 12 {
		t.Fatalf("picked %d, want 12", picked)
	}
}

func TestFormFitsThePane(t *testing.T) {
	m := newModel(t, 110, 30)
	clickLayer(t, m, layerID(idRow, 0))
	clickField(t, m, "labels")
	if m.form == nil {
		t.Fatal("no form")
	}
	checkFrames(t, m, "form")
}
