package ui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/provider/fake"
)

// The first bucket pages through tickets the queue has loaded. The second
// has its own query and loads one page at a time.
const pageProfile = `
[provider]
kind = "fake"

[[queue]]
name = "Work"
query = "status=in progress"
columns = ["key", "title", "status"]

[[queue.bucket]]
name = "Active"
values = ["in progress"]
page_size = 2

[[queue.bucket]]
name = "Rest"
query = "status=*"
page_size = 5
`

// counting counts the pages asked of the provider.
type counting struct {
	*fake.Provider
	pages int
}

func (c *counting) SearchPage(ctx context.Context, query, cursor string, limit int) (provider.Page, error) {
	c.pages++
	return c.Provider.SearchPage(ctx, query, cursor, limit)
}

// rowOf is the index of the ticket with a key among the lines on screen.
func rowOf(t *testing.T, q *queue, key string) int {
	t.Helper()
	for _, ln := range q.lines() {
		if ln.ticket >= 0 && q.tickets[ln.ticket].Key == key {
			return ln.ticket
		}
	}
	t.Fatalf("%s is not listed", key)
	return -1
}

func TestBucketPages(t *testing.T) {
	prof, err := profile.Parse(pageProfile)
	if err != nil {
		t.Fatal(err)
	}
	prov := &counting{Provider: fake.New(0)}
	m := New(prof, prov, Options{})
	send(t, m, tea.WindowSizeMsg{Width: 110, Height: 40})
	drain(t, m, m.Init())
	q := m.queues[0]

	shows := func(what string, want []string, not []string) {
		t.Helper()
		out := screen(m)
		for _, s := range want {
			if !strings.Contains(out, s) {
				t.Fatalf("%s: screen lacks %q\n%s", what, s, out)
			}
		}
		for _, s := range not {
			if strings.Contains(out, s) {
				t.Fatalf("%s: screen has %q\n%s", what, s, out)
			}
		}
	}
	has := func(id string) bool {
		m.View()
		return m.comp.GetLayer(id) != nil
	}

	// The first page of each bucket, and the whole size on the tab.
	shows("start", []string{"Active  1-2 of 3", "Rest  1-5 of 12", "Work (15)", "15 tickets", "DEMO-101", "DEMO-105"}, []string{"DEMO-106", "Other"})
	if prov.pages != 1 {
		t.Fatalf("%d pages loaded at the start, want 1", prov.pages)
	}
	if has(layerID(idPagePrev, 1)) || !has(layerID(idPageNext, 1)) {
		t.Fatal("the first page offers a page before it, or none after")
	}

	// The next page is loaded, and a click on one of its rows opens it.
	clickLayer(t, m, layerID(idPageNext, 1))
	shows("page 2", []string{"Rest  6-10 of 12", "DEMO-106", "DEMO-109", "Active  1-2 of 3"}, []string{"DEMO-101", "DEMO-111"})
	clickLayer(t, m, layerID(idRow, rowOf(t, q, "DEMO-107")))
	if len(m.tabs) != 1 || m.tabs[0].t.Key != "DEMO-107" {
		t.Fatal("a click on a row of the second page opened another ticket")
	}
	clickLayer(t, m, layerID(idQueueTab, 0))

	// The last page has no page after it.
	clickLayer(t, m, layerID(idPageNext, 1))
	shows("page 3", []string{"Rest  11-12 of 12", "DEMO-112"}, []string{"DEMO-109"})
	if has(layerID(idPageNext, 1)) || !has(layerID(idPagePrev, 1)) {
		t.Fatal("the last page offers a page after it, or none before")
	}
	clickLayer(t, m, layerID(idPagePrev, 1))
	shows("back to page 2", []string{"Rest  6-10 of 12"}, nil)

	// A bucket of the queue's own tickets turns its pages with no load.
	before := prov.pages
	clickLayer(t, m, layerID(idPageNext, 0))
	shows("loaded bucket", []string{"Active  3-3 of 3", "Rest  6-10 of 12"}, []string{"Active  1-2"})
	if prov.pages != before || has(layerID(idPageNext, 0)) {
		t.Fatal("turning a page of loaded tickets asked the provider, or offers a page past the last")
	}
	checkFrames(t, m, "pages")
	send(t, m, tea.WindowSizeMsg{Width: 110, Height: 40})

	// A reload, as after a write, keeps the page on screen.
	drain(t, m, m.reloadQueues())
	shows("reload", []string{"Rest  6-10 of 12", "Work (15)"}, nil)

	// A page that is no longer there is answered with the first one.
	q.page(1).cursors[1] = "50"
	drain(t, m, m.reloadQueues())
	shows("page gone", []string{"Rest  1-5 of 12", "DEMO-101"}, []string{"DEMO-106"})
}

func TestQueueOfBucketQueriesSkipsItsOwnQuery(t *testing.T) {
	prof, err := profile.Parse(`
[provider]
kind = "fake"

[[queue]]
name = "Work"
query = "this is not a query the provider can read"

[[queue.bucket]]
name = "Open"
query = "status=Open"
page_size = 3

[[queue.bucket]]
name = "Done"
query = "status=Done"
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := prof.Queues[0].Buckets[1].PageSize; got != profile.DefaultPageSize {
		t.Fatalf("page size = %d, want the default", got)
	}
	m := New(prof, fake.New(0), Options{})
	send(t, m, tea.WindowSizeMsg{Width: 110, Height: 40})
	drain(t, m, m.Init())
	out := screen(m)
	for _, want := range []string{"Open  1-3 of 5", "Done (2)", "Work (7)", "Ready"} {
		if !strings.Contains(out, want) {
			t.Fatalf("screen lacks %q\n%s", want, out)
		}
	}
}
