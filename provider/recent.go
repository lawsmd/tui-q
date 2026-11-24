package provider

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/lawsmd/tui-q/ticket"
)

// Kinds of change that count as the user acting on a ticket.
const (
	// ChangeStatus is a change of status made by the user.
	ChangeStatus = "status"
	// ChangeTags is a tag added or taken off by the user.
	ChangeTags = "tags"
	// ChangeAssignee is a change of assignee that the user made, or that
	// made the user the assignee or took them off.
	ChangeAssignee = "assignee"
)

// Fields a recent queue fills on each of its tickets, for its columns.
const (
	// FieldTouched is when the user last acted on the ticket.
	FieldTouched = "touched"
	// FieldChange says what that last change was.
	FieldChange = "change"
)

// Touch is the last time the user acted on one ticket.
type Touch struct {
	Ticket ticket.Ticket
	At     time.Time
	// Kind is one of the Change constants, and What says in a few words
	// what changed, such as "New → Active" or "+urgent".
	Kind string
	What string
}

// RecentReader is a Provider that can find the tickets the user acted on
// lately. The interface offers a recent queue only for a provider that
// implements it.
type RecentReader interface {
	// Recent returns the tickets the user changed the status, tags or
	// assignee of since the given time, or that were assigned to the
	// user or taken from them in that time, each once with its latest
	// such change, in any order.
	Recent(ctx context.Context, since time.Time) ([]Touch, error)
}

// RecentTickets runs Recent and returns the tickets newest change first,
// with FieldTouched and FieldChange set on each.
func RecentTickets(ctx context.Context, p Provider, days int) ([]ticket.Ticket, error) {
	r, ok := p.(RecentReader)
	if !ok {
		return nil, fmt.Errorf("this platform cannot list recent tickets")
	}
	touches, err := r.Recent(ctx, time.Now().AddDate(0, 0, -days))
	if err != nil {
		return nil, err
	}
	sort.SliceStable(touches, func(i, j int) bool { return touches[i].At.After(touches[j].At) })
	out := make([]ticket.Ticket, len(touches))
	for i, tc := range touches {
		t := tc.Ticket
		fields := make(map[string]ticket.Value, len(t.Fields)+2)
		for k, v := range t.Fields {
			fields[k] = v
		}
		fields[FieldTouched] = ticket.Text(tc.At.Local().Format("2006-01-02 15:04"))
		fields[FieldChange] = ticket.Text(tc.What)
		t.Fields = fields
		out[i] = t
	}
	return out, nil
}
