package arrange

import (
	"context"

	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// Own loads the tickets of a queue that do not come from the own query of
// a bucket: the results of the queue's query, or for a recent queue the
// tickets the user acted on lately, the latest first.
func Own(ctx context.Context, p provider.Provider, q *profile.Queue) ([]ticket.Ticket, error) {
	if q.Recent {
		return provider.RecentTickets(ctx, p, q.Days)
	}
	return p.Search(ctx, q.Query)
}
