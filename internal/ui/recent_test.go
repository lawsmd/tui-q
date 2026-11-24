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

const recentProfile = `
[provider]
kind = "fake"

[[queue]]
name = "Work"
query = ""

[[queue]]
name = "Recent"
recent = true
page_size = 2
`

// keysShown lists the keys of the tickets on screen in a queue, top first.
func keysShown(q *queue) []string {
	var out []string
	for _, ln := range q.lines() {
		if ln.ticket >= 0 {
			out = append(out, q.tickets[ln.ticket].Key)
		}
	}
	return out
}

func TestRecentQueueListsLatestChangeFirstAPageAtATime(t *testing.T) {
	prof, err := profile.Parse(recentProfile)
	if err != nil {
		t.Fatal(err)
	}
	prov := fake.New(0)
	m := New(prof, prov, Options{})
	send(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	drain(t, m, m.Init())
	m.activeQueue = 1
	q := m.queues[1]

	if got := strings.Join(keysShown(q), " "); got != "DEMO-104 DEMO-110" {
		t.Fatalf("first page %q", got)
	}
	out := screen(m)
	for _, s := range []string{"Recent  1-2 of 4", "Touched", "Change", "Open → In progress"} {
		if !strings.Contains(out, s) {
			t.Fatalf("screen lacks %q\n%s", s, out)
		}
	}
	// The time of the change stays when dates are shown without times.
	m.prefs.DateOnly = true
	stamp := q.tickets[0].Field(provider.FieldTouched).String()
	if len(stamp) != 16 || !strings.Contains(screen(m), stamp) {
		t.Fatalf("with dates only, the screen lacks the time %q\n%s", stamp, screen(m))
	}
	m.prefs.DateOnly = false

	clickLayer(t, m, layerID(idPageNext, 0))
	if got := strings.Join(keysShown(q), " "); got != "DEMO-106 DEMO-112" {
		t.Fatalf("second page %q", got)
	}

	// A change made from the interface puts the ticket at the top.
	err = prov.Transition(context.Background(), "DEMO-101", ticket.Transition{To: "Waiting"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.loadQueue(1))
	clickLayer(t, m, layerID(idPagePrev, 0))
	if got := keysShown(q); len(got) == 0 || got[0] != "DEMO-101" {
		t.Fatalf("after a change the first page is %v", got)
	}
	checkFrames(t, m, "recent queue")
}

// historyless is a provider that cannot read history.
type historyless struct{ provider.Provider }

func TestRecentQueueLeftOutWithoutHistory(t *testing.T) {
	prof, err := profile.Parse(recentProfile)
	if err != nil {
		t.Fatal(err)
	}
	m := New(prof, historyless{fake.New(0)}, Options{})
	if len(m.queues) != 1 || m.queues[0].cfg.Name != "Work" {
		t.Fatalf("queues %d", len(m.queues))
	}
}
