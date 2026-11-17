package tuiq

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider"
)

// checkSample is how many tickets of each queue the check prints.
const checkSample = 3

// check runs every queue of the profile once without the interface and
// prints what came back. It reads only. It is run as "tq check" to test a
// new profile or new credentials.
func check(w io.Writer, prof *profile.Profile, prov provider.Provider) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	firstID := ""
	for _, q := range prof.Queues {
		tickets, err := prov.Search(ctx, q.Query)
		if err != nil {
			return fmt.Errorf("queue %q: %w", q.Name, err)
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
		if name == "body" || name == "title" {
			continue
		}
		fmt.Fprintf(w, "  %-18s %s\n", prof.Label(name), t.Field(name).String())
	}
	return nil
}
