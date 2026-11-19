package fake

import (
	"fmt"
	"time"

	"github.com/lawsmd/tui-q/ticket"
)

func fixtures() []ticket.Ticket {
	now := time.Now()
	rows := []struct {
		title, status, typ, assignee, priority string
		labels                                 []string
	}{
		{"Login page returns 500 after password reset", "Open", "Bug", "", "High", []string{"auth"}},
		{"Export to CSV drops rows with commas in the name", "Open", "Bug", "", "Medium", []string{"export"}},
		{"Customer asks for invoice copy from last quarter", "Open", "Task", "sam", "Low", nil},
		{"Webhook retries stop after the first failure", "In progress", "Bug", "sam", "High", []string{"integrations", "webhooks"}},
		{"Add a second admin to the account", "In progress", "Task", "alex", "Low", nil},
		{"Mobile app shows stale unread count", "Waiting", "Bug", "alex", "Medium", []string{"mobile"}},
		{"Scheduled report arrives an hour late since the clock change", "Waiting", "Bug", "sam", "Medium", []string{"reports", "timezone"}},
		{"Bulk import rejects files over 10 MB with no message", "Open", "Bug", "", "Medium", []string{"import"}},
		{"Rename a location without losing its history", "Open", "Task", "", "Low", nil},
		{"Two-factor codes are rejected on the second device", "In progress", "Bug", "sam", "High", []string{"auth", "mobile"}},
		{"Search ignores the date filter on the archive tab", "Done", "Bug", "alex", "Low", []string{"search"}},
		{"Set up single sign-on for a new team", "Done", "Task", "sam", "Medium", []string{"auth"}},
	}
	out := make([]ticket.Ticket, 0, len(rows))
	for i, r := range rows {
		n := 101 + i
		key := fmt.Sprintf("DEMO-%d", n)
		t := ticket.Ticket{
			ID:       key,
			Key:      key,
			Title:    r.title,
			Status:   r.status,
			Type:     r.typ,
			Assignee: r.assignee,
			Priority: r.priority,
			URL:      "https://example.com/browse/" + key,
			Created:  now.Add(-time.Duration(len(rows)-i) * 26 * time.Hour),
			Updated:  now.Add(-time.Duration(len(rows)-i) * 3 * time.Hour),
			Body: "## What happened\n\n" + r.title + ".\n\n" +
				"## Steps to reproduce\n\n1. Sign in as a normal user.\n2. Repeat the action from the report.\n3. Compare with the expected result.\n\n" +
				"## Notes\n\nThis ticket is **fixture data** from the `fake` provider.",
			Fields: map[string]ticket.Value{
				"labels": ticket.List(r.labels...),
			},
			Comments: []ticket.Comment{
				{Author: "reporter", Created: now.Add(-time.Duration(len(rows)-i) * 20 * time.Hour), Body: "Still happening this morning."},
				{Author: "sam", Created: now.Add(-time.Duration(len(rows)-i) * 12 * time.Hour), Body: "I can reproduce it on the **staging** site. Looking into it."},
				{Author: "alex", Created: now.Add(-time.Duration(len(rows)-i) * 4 * time.Hour), Body: "A fix is in review:\n\n- one change to the handler\n- one new test"},
			},
		}
		out = append(out, t)
	}
	return out
}
