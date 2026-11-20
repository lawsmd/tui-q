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
	for _, want := range []string{"DEMO-104", "refresh", "open in browser", "Status"} {
		if !strings.Contains(out, want) {
			t.Errorf("screen lacks %q\n%s", want, out)
		}
	}
	for _, not := range []string{"Open", "All", "Board", "go to ticket"} {
		if strings.Contains(strings.SplitN(out, "\n", 2)[0], not) {
			t.Errorf("the top row shows %q\n%s", not, out)
		}
	}
	for _, id := range []string{layerID(idQueueTab, 0), layerID(idTicketTab, 0), layerID(idTabClose, 0), idJump, idGear} {
		if m.comp.GetLayer(id) != nil {
			t.Errorf("layer %q is on screen", id)
		}
	}
	if title := m.View().WindowTitle; title != "DEMO-104" {
		t.Errorf("window title %q, want the key", title)
	}

	// No key leads away from the ticket.
	for _, k := range []tea.KeyPressMsg{
		{Code: tea.KeyEscape},
		{Code: tea.KeyTab},
		{Code: tea.KeyTab, Mod: tea.ModShift},
		{Code: '/', Text: "/"},
		{Code: 'w', Mod: tea.ModCtrl},
	} {
		send(t, m, k)
		if len(m.tabs) != 1 || m.activeTab != 0 || m.jumping {
			t.Fatalf("%q left the ticket: tabs=%d activeTab=%d jumping=%v", k.String(), len(m.tabs), m.activeTab, m.jumping)
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
	if prov.n != 0 {
		t.Fatalf("%d queue reads, want none", prov.n)
	}
}
