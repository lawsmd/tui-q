package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/internal/session"
	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider/fake"
)

// listed is the tabs the session file in dir holds for this process.
func listed(t *testing.T, dir string) []session.Tab {
	t.Helper()
	states, err := session.List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 {
		t.Fatalf("%d interfaces listed, want this one", len(states))
	}
	return states[0].Tabs
}

func TestSessionFileFollowsTheTabs(t *testing.T) {
	dir := t.TempDir()
	prof, err := profile.Parse(testProfile)
	if err != nil {
		t.Fatal(err)
	}
	prof.Ticket.Links = true
	m := New(prof, fake.New(0), Options{Session: session.Open(dir)})
	send(t, m, tea.WindowSizeMsg{Width: 110, Height: 60})
	drain(t, m, m.Init())

	// An interface showing a queue is listed with no ticket.
	if tabs := listed(t, dir); len(tabs) != 0 {
		t.Fatalf("tabs = %+v before a ticket is opened", tabs)
	}

	// A ticket read in full is listed with its links.
	q := m.queues[m.activeQueue]
	clickLayer(t, m, layerID(idRow, rowOf(t, q, "DEMO-102")))
	tabs := listed(t, dir)
	if len(tabs) != 1 || tabs[0].Key != "DEMO-102" || !tabs[0].Shown || tabs[0].Status != "Open" || tabs[0].Title == "" {
		t.Fatalf("tabs = %+v, want DEMO-102 on show", tabs)
	}
	if want := (session.Link{Relation: "is blocked by", Key: "DEMO-104", Status: "In progress"}); len(tabs[0].Links) != 1 || tabs[0].Links[0] != want {
		t.Fatalf("links = %+v, want %+v", tabs[0].Links, want)
	}

	// The ticket a link opens is listed after it, and is the one on show.
	clickLayer(t, m, layerID(idLink, 0))
	tabs = listed(t, dir)
	if len(tabs) != 2 || tabs[0].Shown || !tabs[1].Shown || tabs[1].Key != "DEMO-104" || len(tabs[1].Links) != 3 {
		t.Fatalf("tabs = %+v, want DEMO-104 on show after DEMO-102", tabs)
	}

	// Showing a queue leaves both open and neither on show.
	clickLayer(t, m, layerID(idQueueTab, 0))
	if tabs = listed(t, dir); len(tabs) != 2 || tabs[0].Shown || tabs[1].Shown {
		t.Fatalf("tabs = %+v, want two with neither on show", tabs)
	}

	// A message that changes no tab writes nothing.
	n := m.sessionN
	send(t, m, odd{})
	send(t, m, tea.WindowSizeMsg{Width: 100, Height: 40})
	if m.sessionN != n {
		t.Errorf("the file was written %d times for messages that changed no tab", m.sessionN-n)
	}

	// A change to a ticket in a tab is listed once it is read back.
	clickLayer(t, m, layerID(idTicketTab, 0))
	clickLayer(t, m, idMove)
	clickItem(t, m, "Waiting")
	if tabs = listed(t, dir); tabs[0].Status != "Waiting" || !tabs[0].Shown {
		t.Fatalf("tabs = %+v, want DEMO-102 waiting and on show", tabs)
	}

	clickLayer(t, m, layerID(idTabClose, 1))
	if tabs = listed(t, dir); len(tabs) != 1 || tabs[0].Key != "DEMO-102" {
		t.Fatalf("tabs = %+v after a tab was closed", tabs)
	}

	m.Close()
	if states, _ := session.List(dir); len(states) != 0 {
		t.Fatalf("the file is still there after the interface closed: %+v", states)
	}
}
