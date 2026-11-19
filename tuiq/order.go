package tuiq

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/lawsmd/tui-q/internal/arrange"
	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// Limits on how many tickets "tq order" prints for one bucket or lane.
const (
	orderLimit    = 20
	orderLimitMax = 100
)

// orderNotice opens the output of "tq order". The reader is an LLM agent
// that has its own access to the ticketing platform, and the notice tells
// it what the list is for.
const orderNotice = `Queues are listed in priority order, then buckets, then tickets. Number 1 is next.
This list gives order only. Read ticket details and make changes through your own
access to the ticketing platform, not through tq.
`

// order prints the tickets of each queue in the order the interface shows
// them: the queues of the profile from the top, the buckets or lanes of
// each, and the tickets of each in the order the platform returned them.
// A ticket is printed as its key, status and title and nothing more. It
// reads only. It is run as "tq order" by an agent that needs to know what
// to work on next.
func order(w io.Writer, prof *profile.Profile, prov provider.Provider, args []string) error {
	flags := flag.NewFlagSet("tq order", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	name := flags.String("queue", "", "print this queue alone")
	limit := flags.Int("limit", orderLimit, "the most tickets printed for one bucket or lane")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		flags.Usage()
		return fmt.Errorf("order takes no argument %q", flags.Arg(0))
	}
	if *limit < 1 || *limit > orderLimitMax {
		return fmt.Errorf("--limit must be from 1 to %d", orderLimitMax)
	}

	// A queue named on the command line is printed even when the profile
	// keeps it out of the order.
	var queues []*profile.Queue
	var names []string
	for i := range prof.Queues {
		q := &prof.Queues[i]
		names = append(names, q.Name)
		if *name == "" && q.InOrder() || *name != "" && strings.EqualFold(q.Name, *name) {
			queues = append(queues, q)
		}
	}
	if *name != "" && len(queues) == 0 {
		return fmt.Errorf("no queue named %q; the queues are %s", *name, strings.Join(names, ", "))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	site := prof.Provider.Kind
	if prof.Provider.BaseURL != "" {
		site += ", " + prof.Provider.BaseURL
	}
	fmt.Fprintf(w, "Working order from tq (%s)\n%s", site, orderNotice)

	var failed error
	for n, q := range queues {
		sections, err := orderSections(ctx, prov, q, *limit)
		if err != nil {
			fmt.Fprintf(w, "\nQueue %d: %s (%s)\n  could not be read: %v\n", n+1, q.Name, q.Layout, err)
			if failed == nil {
				failed = fmt.Errorf("queue %q: %w", q.Name, err)
			}
			continue
		}
		printQueue(w, n+1, q, sections)
	}
	return failed
}

// orderSection is one bucket or lane as "tq order" prints it. A table
// without buckets is one section with no name.
type orderSection struct {
	name    string
	tickets []ticket.Ticket
	// total is how many tickets the section holds, or -1 when the platform
	// does not say, and more says some were left out in that case.
	total int
	more  bool
}

// cut makes a section of the first limit tickets.
func cut(name string, all []ticket.Ticket, limit int) orderSection {
	return orderSection{name: name, tickets: all[:min(len(all), limit)], total: len(all)}
}

// size describes how many tickets the section holds and how many of them
// are printed. unit is put after a plain count.
func (s orderSection) size(unit string) string {
	shown := len(s.tickets)
	switch {
	case s.total < 0 && s.more:
		return fmt.Sprintf("first %d shown", shown)
	case s.total > shown:
		return fmt.Sprintf("%d of %d shown", shown, s.total)
	}
	return fmt.Sprintf("%d%s", shown, unit)
}

// orderSections loads a queue the way the interface does and groups its
// tickets. The queue's own query is run unless every bucket has its own,
// and a bucket with its own query is asked for its first limit tickets.
func orderSections(ctx context.Context, prov provider.Provider, q *profile.Queue, limit int) ([]orderSection, error) {
	var own []ticket.Ticket
	if !q.OwnQueries() {
		found, err := prov.Search(ctx, q.Query)
		if err != nil {
			return nil, err
		}
		own = found
	}
	pick := func(indexes []int) []ticket.Ticket {
		out := make([]ticket.Ticket, len(indexes))
		for i, n := range indexes {
			out[i] = own[n]
		}
		return out
	}

	if q.Layout == profile.LayoutBoard {
		var out []orderSection
		for _, l := range arrange.Lanes(q, own) {
			out = append(out, cut(l.Name, pick(l.Tickets), limit))
		}
		return out, nil
	}
	if len(q.Buckets) == 0 {
		return []orderSection{cut("", own, limit)}, nil
	}

	var out []orderSection
	members := arrange.Sections(q, own, nil)
	for j, b := range q.Buckets {
		if b.Query == "" {
			out = append(out, cut(b.Name, pick(members[j]), limit))
			continue
		}
		page, err := prov.SearchPage(ctx, b.Query, "", limit)
		if err != nil {
			return nil, fmt.Errorf("bucket %q: %w", b.Name, err)
		}
		out = append(out, orderSection{name: b.Name, tickets: page.Tickets, total: page.Total, more: page.Next != ""})
	}
	// The tickets that match no bucket, as the interface shows them.
	if other := members[len(q.Buckets)]; len(other) > 0 && !q.HideUnmatched {
		out = append(out, cut("Other", pick(other), limit))
	}
	return out, nil
}

// printQueue prints one queue. The tickets are numbered from 1 through all
// of its sections.
func printQueue(w io.Writer, n int, q *profile.Queue, sections []orderSection) {
	keyW, statusW, count := 0, 0, 0
	for _, s := range sections {
		for _, t := range s.tickets {
			keyW, statusW = max(keyW, len(t.Key)), max(statusW, len(t.Status)+2)
			count++
		}
	}
	numW := len(fmt.Sprint(count))

	indent := "    "
	switch {
	case q.Layout == profile.LayoutBoard:
		fmt.Fprintf(w, "\nQueue %d: %s (board, lanes by %s)\n", n, q.Name, q.GroupBy)
	case len(q.Buckets) == 0:
		indent = "  "
		unit := " tickets"
		if len(sections[0].tickets) == 1 {
			unit = " ticket"
		}
		fmt.Fprintf(w, "\nQueue %d: %s (table, %s)\n", n, q.Name, sections[0].size(unit))
	default:
		fmt.Fprintf(w, "\nQueue %d: %s (table)\n", n, q.Name)
	}

	place := 0
	for _, s := range sections {
		if s.name != "" {
			fmt.Fprintf(w, "  %s (%s)\n", s.name, s.size(""))
		}
		for _, t := range s.tickets {
			place++
			fmt.Fprintf(w, "%s%*d. %-*s  %-*s  %s\n", indent, numW, place, keyW, t.Key, statusW, "["+t.Status+"]", t.Title)
		}
	}
}
