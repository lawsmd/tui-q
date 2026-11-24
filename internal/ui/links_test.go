package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider/fake"
)

// newLinks builds a model whose profile lists the tickets a ticket is
// linked to. With an ID it is started on that ticket, and with none on the
// queues.
func newLinks(t *testing.T, id string, w, h int) *Model {
	t.Helper()
	prof, err := profile.Parse(testProfile)
	if err != nil {
		t.Fatal(err)
	}
	prof.Ticket.Links = true
	m := New(prof, fake.New(0), Options{Ticket: id})
	send(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	drain(t, m, m.Init())
	return m
}

func TestLinkedTicketsAreListedUnderWhatTheyAre(t *testing.T) {
	m := newLinks(t, "DEMO-104", 110, 60)
	out := screen(m)
	// DEMO-104 blocks two tickets and relates to a third.
	at := -1
	for _, want := range []string{"Links", "blocks", "DEMO-102", "Export to CSV", "DEMO-107", "relates to", "DEMO-110"} {
		next := strings.Index(out[at+1:], want)
		if next < 0 {
			t.Fatalf("screen lacks %q after the lines before it\n%s", want, out)
		}
		at += 1 + next
	}
	if strings.Count(out, "blocks") != 1 {
		t.Errorf("the relation is not written once for the tickets that share it\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "DEMO-107") && !strings.Contains(line, "Waiting") {
			t.Errorf("the line of a linked ticket lacks its status: %q", line)
		}
	}
	for i := range 3 {
		if m.comp.GetLayer(layerID(idLink, i)) == nil {
			t.Errorf("link %d cannot be clicked", i)
		}
	}
	moveOver(t, m, layerID(idLink, 0))
	if !swapped(t, m, layerID(idLink, 0)) {
		t.Error("the key of a linked ticket is not lit under the mouse")
	}
	checkFrames(t, m, "ticket with links")
	m.tabs[0].offset, m.tabs[0].sideOffset = 1000, 1000
	checkFrames(t, m, "ticket with links, scrolled to the end")
}

func TestLinkOpensItsTicketInATab(t *testing.T) {
	m := newLinks(t, "DEMO-104", 110, 60)
	clickLayer(t, m, layerID(idLink, 0))
	if len(m.tabs) != 2 || m.activeTab != 1 {
		t.Fatalf("tabs=%d activeTab=%d, want the linked ticket in a second tab", len(m.tabs), m.activeTab)
	}
	tab := m.tabs[1]
	if tab.t.Key != "DEMO-102" || tab.loading || tab.t.Body == "" {
		t.Fatalf("second tab: key=%q loading=%v body=%d bytes, want DEMO-102 read in full", tab.t.Key, tab.loading, len(tab.t.Body))
	}
	if title := m.View().WindowTitle; title != "DEMO-102" {
		t.Errorf("window title %q, want the key of the ticket on show", title)
	}

	// DEMO-102 is blocked by DEMO-104, which already has a tab.
	if !strings.Contains(screen(m), "is blocked by") {
		t.Fatalf("the linked ticket does not show its own link\n%s", screen(m))
	}
	clickLayer(t, m, layerID(idLink, 0))
	if len(m.tabs) != 2 || m.activeTab != 0 {
		t.Fatalf("tabs=%d activeTab=%d, want the tab DEMO-104 already has", len(m.tabs), m.activeTab)
	}
}

func TestLinkOpensFromAQueueAndInANarrowPane(t *testing.T) {
	// Below the width where the sidebar has a column of its own, its
	// lines are drawn over the main column.
	m := newLinks(t, "", 70, 60)
	q := m.queues[m.activeQueue]
	clickLayer(t, m, layerID(idRow, rowOf(t, q, "DEMO-102")))
	if m.tabs[0].sideX != 0 {
		t.Fatal("the sidebar has a column of its own in a pane this narrow")
	}
	clickLayer(t, m, layerID(idLink, 0))
	if len(m.tabs) != 2 || m.activeTab != 1 || m.tabs[1].t.Key != "DEMO-104" {
		t.Fatalf("tabs=%d activeTab=%d, want DEMO-104 in a second tab", len(m.tabs), m.activeTab)
	}
	m.View()
	for _, id := range []string{layerID(idQueueTab, 0), layerID(idTicketTab, 1)} {
		if m.comp.GetLayer(id) == nil {
			t.Errorf("layer %q is not on screen", id)
		}
	}
}

func TestLinksAreLeftOutUnlessTheProfileAsks(t *testing.T) {
	m, _ := newOnly(t, "DEMO-104")
	send(t, m, tea.WindowSizeMsg{Width: 110, Height: 60})
	if out := screen(m); strings.Contains(out, "Links") || strings.Contains(out, "DEMO-102") {
		t.Errorf("links are listed without the setting\n%s", out)
	}
	if m.comp.GetLayer(layerID(idLink, 0)) != nil {
		t.Error("a link can be clicked without the setting")
	}

	// A ticket with no links has no section either.
	m = newLinks(t, "DEMO-101", 110, 60)
	if strings.Contains(screen(m), "Links") {
		t.Errorf("a ticket with no links has the section\n%s", screen(m))
	}
}
