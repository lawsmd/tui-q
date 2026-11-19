package tuiq

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// checkSample is how many tickets of each queue the check prints.
const checkSample = 3

// check runs every queue of the profile once without the interface and
// prints what came back, with the account, the field names, and the moves
// open to the first ticket. It reads only. It is run as "tq check" to test
// a new profile or new credentials.
func check(w io.Writer, prof *profile.Profile, prov provider.Provider) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	me, err := prov.Me(ctx)
	if err != nil {
		return fmt.Errorf("account: %w", err)
	}
	fmt.Fprintf(w, "Signed in as %s\n\n", me.Name)
	labels, err := prov.FieldLabels(ctx)
	if err != nil {
		return fmt.Errorf("field names: %w", err)
	}
	label := func(name string) string {
		if f, ok := prof.Fields[name]; ok && f.Label != "" {
			return f.Label
		}
		if l := labels[name]; l != "" {
			return l
		}
		return prof.Label(name)
	}

	firstID := ""
	for _, q := range prof.Queues {
		var tickets []ticket.Ticket
		if !q.OwnQueries() {
			found, err := prov.Search(ctx, q.Query)
			if err != nil {
				return fmt.Errorf("queue %q: %w", q.Name, err)
			}
			tickets = found
		}
		// A bucket with its own query adds its first page.
		for _, b := range q.Buckets {
			if b.Query == "" {
				continue
			}
			page, err := prov.SearchPage(ctx, b.Query, "", b.PageSize)
			if err != nil {
				return fmt.Errorf("queue %q, bucket %q: %w", q.Name, b.Name, err)
			}
			fmt.Fprintf(w, "%s, bucket %s: first %d of %d tickets (-1 means the platform gave no total)\n", q.Name, b.Name, len(page.Tickets), page.Total)
			tickets = append(tickets, page.Tickets...)
		}
		fmt.Fprintf(w, "%s (%s): %d tickets\n", q.Name, q.Layout, len(tickets))
		for i, t := range tickets {
			if i == checkSample {
				break
			}
			fmt.Fprintf(w, "  %-12s %-16s %s\n", t.Key, t.Status, t.Title)
		}
		if firstID == "" && len(tickets) > 0 {
			firstID = tickets[0].ID
		}
	}
	if firstID == "" {
		return nil
	}

	t, err := prov.Get(ctx, firstID)
	if err != nil {
		return fmt.Errorf("ticket %s: %w", firstID, err)
	}
	fmt.Fprintf(w, "\n%s loaded in full: %d characters of description, %d comments\n",
		t.Key, len(t.Body), len(t.Comments))
	names := append(append([]string(nil), prof.Ticket.Main...), prof.Ticket.Sidebar...)
	for _, name := range names {
		if name == ticket.FieldBody || name == ticket.FieldTitle {
			continue
		}
		value := t.Field(name).String()
		note := ""
		if spec, ok := t.Spec(name); ok && spec.Kind != ticket.KindNone {
			note = "  (can be edited: " + spec.Kind + ")"
		} else if !ok && !ticket.BuiltIn(name) && value == "" {
			note = "(not on this ticket's screen, so hidden)"
		}
		fmt.Fprintf(w, "  %-36s %s%s\n", label(name), value, note)
	}

	trs, err := prov.Transitions(ctx, firstID)
	if err != nil {
		return fmt.Errorf("moves open to %s: %w", firstID, err)
	}
	fmt.Fprintf(w, "\nMoves open to %s:\n", t.Key)
	for _, tr := range trs {
		required := 0
		for _, f := range tr.Fields {
			if f.Required {
				required++
			}
		}
		fmt.Fprintf(w, "  %-20s %s, %d required fields\n", tr.To, tr.Name, required)
	}
	return nil
}
