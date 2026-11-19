package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider/fake"
)

const bucketProfile = `
[provider]
kind = "fake"

[[queue]]
name = "Work"
query = ""
columns = ["key", "title", "status"]

[[queue.bucket]]
name = "Active"
values = ["in progress", "Waiting"]

[[queue.bucket]]
name = "Open"
values = ["Open"]
`

func TestBucketsAndTheSingleTabRow(t *testing.T) {
	prof, err := profile.Parse(bucketProfile)
	if err != nil {
		t.Fatal(err)
	}
	m := New(prof, fake.New(0), Options{})
	send(t, m, tea.WindowSizeMsg{Width: 110, Height: 40})
	drain(t, m, m.Init())

	rows := strings.Split(screen(m), "\n")
	if !strings.Contains(rows[0], "Work") || !strings.Contains(rows[0], "refresh") {
		t.Fatalf("top row: %q", rows[0])
	}
	if !strings.Contains(rows[2], "Key") || !strings.Contains(rows[4], "Active") || strings.TrimSpace(rows[1]+rows[3]) != "" {
		t.Fatalf("the table does not start under the one tab row:\n%s\n%s", rows[2], rows[4])
	}

	// Every ticket is listed once, under the heading of its bucket.
	q := m.queues[0]
	for j, members := range q.sections() {
		name := "Other"
		if j < len(q.cfg.Buckets) {
			name = q.cfg.Buckets[j].Name
		}
		if want := fmt.Sprintf("%s (%d)", name, len(members)); !strings.Contains(screen(m), want) {
			t.Errorf("screen lacks the heading %q", want)
		}
	}
	section, seen := "", 0
	for _, ln := range q.lines() {
		if ln.ticket < 0 && ln.heading == "" {
			continue
		}
		if ln.ticket < 0 {
			section = strings.Fields(ln.heading)[0]
			continue
		}
		seen++
		status := q.tickets[ln.ticket].Status
		want := map[string]string{"Open": "Open", "In progress": "Active", "Waiting": "Active", "Done": "Other"}[status]
		if section != want {
			t.Errorf("a ticket with status %q is under %q", status, section)
		}
	}
	if seen != len(q.tickets) {
		t.Errorf("%d of %d tickets listed", seen, len(q.tickets))
	}

	// A ticket opens beside the queue on the same row.
	clickLayer(t, m, layerID(idRow, 0))
	rows = strings.Split(screen(m), "\n")
	if !strings.Contains(rows[0], q.tickets[0].Key) || !strings.Contains(rows[0], "Work") {
		t.Fatalf("top row with a ticket open: %q", rows[0])
	}
	clickLayer(t, m, layerID(idQueueTab, 0))
	if m.activeTab >= 0 {
		t.Fatal("the queue tab did not bring the queue back")
	}
	checkFrames(t, m, "buckets")

	// With hide_unmatched the Other section and its tickets are left out
	// of the table and of the count on the tab.
	all := q.count()
	q.cfg.HideUnmatched = true
	shown := q.count()
	out := screen(m)
	if shown >= all || strings.Contains(out, "Other") || !strings.Contains(out, fmt.Sprintf("Work (%d)", shown)) {
		t.Fatalf("%d of %d tickets shown with hide_unmatched:\n%s", shown, all, out)
	}
}
