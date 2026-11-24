package ui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/provider/fake"
	"github.com/lawsmd/tui-q/ticket"
)

// searches counts the queue reads asked of the provider.
type searches struct {
	*fake.Provider
	n int
}

func (s *searches) Search(ctx context.Context, query string) ([]ticket.Ticket, error) {
	s.n++
	return s.Provider.Search(ctx, query)
}

func (s *searches) SearchPage(ctx context.Context, query, cursor string, limit int) (provider.Page, error) {
	s.n++
	return s.Provider.SearchPage(ctx, query, cursor, limit)
}

// newOnly builds a model started on one ticket.
func newOnly(t *testing.T, id string) (*Model, *searches) {
	t.Helper()
	prof, err := profile.Parse(testProfile)
	if err != nil {
		t.Fatal(err)
	}
	prov := &searches{Provider: fake.New(0)}
	m := New(prof, prov, Options{Ticket: id})
	send(t, m, tea.WindowSizeMsg{Width: 110, Height: 30})
	drain(t, m, m.Init())
	return m, prov
}

func TestOneTicketIsShownWithoutTheQueues(t *testing.T) {
	m, prov := newOnly(t, "DEMO-104")
	if len(m.tabs) != 1 || m.activeTab != 0 {
		t.Fatalf("tabs=%d activeTab=%d, want the one ticket showing", len(m.tabs), m.activeTab)
	}
	out := screen(m)
	for _, want := range []string{"DEMO-104", "go to ticket", "refresh", "open in browser", "Status"} {
		if !strings.Contains(out, want) {
			t.Errorf("screen lacks %q\n%s", want, out)
		}
	}
	for _, not := range []string{"Open", "All", "Board"} {
		if strings.Contains(strings.SplitN(out, "\n", 2)[0], not) {
			t.Errorf("the top row shows %q\n%s", not, out)
		}
	}
	// The ticket has a tab, and no mark to close it while it is the last.
	for _, id := range []string{layerID(idQueueTab, 0), layerID(idTabClose, 0), idGear} {
		if m.comp.GetLayer(id) != nil {
			t.Errorf("layer %q is on screen", id)
		}
	}
	for _, id := range []string{layerID(idTicketTab, 0), idJump} {
		if m.comp.GetLayer(id) == nil {
			t.Errorf("layer %q is not on screen", id)
		}
	}
	if title := m.View().WindowTitle; title != "DEMO-104" {
		t.Errorf("window title %q, want the key", title)
	}

	// No key leads to a queue, and none closes the last ticket.
	for _, k := range []tea.KeyPressMsg{
		{Code: tea.KeyEscape},
		{Code: tea.KeyTab},
		{Code: tea.KeyTab, Mod: tea.ModShift},
		{Code: 'w', Mod: tea.ModCtrl},
	} {
		send(t, m, k)
		if len(m.tabs) != 1 || m.activeTab != 0 {
			t.Fatalf("%q left the ticket: tabs=%d activeTab=%d", k.String(), len(m.tabs), m.activeTab)
		}
	}

	// A write reloads the ticket and no queue.
	clickLayer(t, m, idAssign)
	clickItem(t, m, "Nobody")
	if got := stored(t, m, "DEMO-104").Assignee; got != "" {
		t.Fatalf("assignee = %q after Nobody", got)
	}
	clickLayer(t, m, idRefresh)
	if prov.n != 0 {
		t.Fatalf("%d queue reads, want none", prov.n)
	}
	checkFrames(t, m, "one ticket")
}

func TestOneTicketViewHoldsMoreTickets(t *testing.T) {
	m, prov := newOnly(t, "DEMO-104")

	// The jump box opens a second ticket beside the first.
	send(t, m, tea.KeyPressMsg{Code: '/', Text: "/"})
	if !m.jumping {
		t.Fatal("the slash key did not give the jump box the keyboard")
	}
	typeText(t, m, "111")
	checkFrames(t, m, "one ticket, jump box")
	enter(t, m)
	if len(m.tabs) != 2 || m.activeTab != 1 || m.tabs[1].t.Key != "DEMO-111" {
		t.Fatalf("after the jump: tabs=%d activeTab=%d", len(m.tabs), m.activeTab)
	}
	if title := m.View().WindowTitle; title != "DEMO-111" {
		t.Errorf("window title %q, want the key of the ticket on show", title)
	}
	for _, id := range []string{layerID(idTicketTab, 0), layerID(idTicketTab, 1), layerID(idTabClose, 0), layerID(idTabClose, 1), idJump} {
		if m.comp.GetLayer(id) == nil {
			t.Errorf("layer %q is not on screen", id)
		}
	}
	checkFrames(t, m, "one ticket view, two tabs")

	// A tab shows its ticket, and esc leads to no queue.
	clickLayer(t, m, layerID(idTicketTab, 0))
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.activeTab != 0 || !strings.Contains(screen(m), "Webhook retries") {
		t.Fatalf("activeTab=%d after a click on the first tab\n%s", m.activeTab, screen(m))
	}

	// A tab closes while another is left, and the last one stays.
	send(t, m, tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	if len(m.tabs) != 1 || m.activeTab != 0 || m.tabs[0].t.Key != "DEMO-111" {
		t.Fatalf("after ctrl+w: tabs=%d activeTab=%d", len(m.tabs), m.activeTab)
	}
	m.View()
	if m.comp.GetLayer(layerID(idTabClose, 0)) != nil {
		t.Error("the last tab has a mark to close it")
	}
	m.closeTab(0)
	if len(m.tabs) != 1 {
		t.Fatal("the last tab was closed")
	}
	if prov.n != 0 {
		t.Fatalf("%d queue reads, want none", prov.n)
	}
}

func TestOneTicketThatFailsToLoad(t *testing.T) {
	m, prov := newOnly(t, "DEMO-9999")
	if len(m.tabs) != 0 {
		t.Fatalf("%d tabs for a ticket that does not exist", len(m.tabs))
	}
	out := screen(m)
	for _, want := range []string{"DEMO-9999", "refresh", "Failed: DEMO-9999"} {
		if !strings.Contains(out, want) {
			t.Errorf("screen lacks %q\n%s", want, out)
		}
	}
	checkFrames(t, m, "one ticket, not loaded")

	// Reloading asks for the ticket again, and still for no queue.
	send(t, m, tea.KeyPressMsg{Code: 'r', Text: "r"})
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(screen(m), "Failed: DEMO-9999") {
		t.Errorf("no failure shown after a reload\n%s", screen(m))
	}

	// The jump box is there to name another ticket.
	clickLayer(t, m, idJump)
	typeText(t, m, "104")
	enter(t, m)
	if len(m.tabs) != 1 || m.activeTab != 0 || m.tabs[0].t.Key != "DEMO-104" {
		t.Fatalf("after a jump: tabs=%d activeTab=%d", len(m.tabs), m.activeTab)
	}
	if prov.n != 0 {
		t.Fatalf("%d queue reads, want none", prov.n)
	}
}
