package ado

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lawsmd/tui-q/provider"
)

// The provider lists the work items the user acted on lately.
var _ provider.RecentReader = (*Provider)(nil)

// historyReaders is how many histories Recent reads at once.
const historyReaders = 16

// historyPage is the most updates one read of a history returns.
const historyPage = 200

// fieldRev is the revision number of a work item, which goes up by one
// with each change.
const fieldRev = "System.Rev"

// seenCache holds, for each work item whose history Recent has read, the
// revision it read and the user's latest change found in it. A work item
// whose revision is the same has the same history, and is not read again.
type seenCache struct {
	mu    sync.Mutex
	items map[int]seen
}

type seen struct {
	rev   int
	touch provider.Touch
	found bool
}

func (c *seenCache) get(id, rev int) (seen, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.items[id]
	return s, ok && s.rev == rev
}

func (c *seenCache) put(id int, s seen) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil {
		c.items = map[int]seen{}
	}
	c.items[id] = s
}

// update is one change to a work item, as /updates returns it.
type update struct {
	RevisedBy struct {
		UniqueName string `json:"uniqueName"`
	} `json:"revisedBy"`
	RevisedDate json.RawMessage `json:"revisedDate"`
	Fields      map[string]struct {
		OldValue json.RawMessage `json:"oldValue"`
		NewValue json.RawMessage `json:"newValue"`
	} `json:"fields"`
}

// Recent finds the work items of the project changed since the given time
// that the user ever changed or was assigned, and reads the history of
// each to find the user's latest change of state, tags or assignee in that
// time. A history is read again only after the work item changes.
func (p *Provider) Recent(ctx context.Context, since time.Time) ([]provider.Touch, error) {
	me, err := p.Me(ctx)
	if err != nil {
		return nil, err
	}
	wiql := fmt.Sprintf("SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = @project"+
		" AND [System.ChangedDate] >= '%s'"+
		" AND (EVER [System.ChangedBy] = @me OR EVER [System.AssignedTo] = @me)"+
		" ORDER BY [System.ChangedDate] DESC", since.UTC().Format("2006-01-02"))
	ids, err := p.Query(ctx, wiql)
	if err != nil {
		return nil, err
	}
	revs, err := p.Read(ctx, ids, []string{"System.Id", fieldRev})
	if err != nil {
		return nil, err
	}

	var (
		mu       sync.Mutex
		found    = map[int]provider.Touch{}
		firstErr error
		wg       sync.WaitGroup
		slots    = make(chan struct{}, historyReaders)
	)
	for _, w := range revs {
		id := w.ID
		rev, _ := strconv.Atoi(decode(w.Fields[fieldRev]).Text)
		if s, ok := p.seen.get(id, rev); ok {
			if s.found && !s.touch.At.Before(since) {
				mu.Lock()
				found[id] = s.touch
				mu.Unlock()
			}
			continue
		}
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			ups, err := p.history(ctx, id)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			tc, ok := lastTouch(ups, me.ID, since)
			p.seen.put(id, seen{rev: rev, touch: tc, found: ok})
			if ok {
				found[id] = tc
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	var touched []int
	for _, id := range ids {
		if _, ok := found[id]; ok {
			touched = append(touched, id)
		}
	}
	items, err := p.Read(ctx, touched, p.FieldIDs())
	if err != nil {
		return nil, err
	}
	out := make([]provider.Touch, 0, len(items))
	for _, w := range items {
		tc := found[w.ID]
		tc.Ticket = p.ToTicket(w)
		out = append(out, tc)
	}
	return out, nil
}

// history reads every update of one work item, oldest first.
func (p *Provider) history(ctx context.Context, id int) ([]update, error) {
	var all []update
	for {
		var resp struct {
			Value []update `json:"value"`
		}
		path := fmt.Sprintf("%s/_apis/wit/workItems/%d/updates?$top=%d&$skip=%d", p.projectPath(), id, historyPage, len(all))
		if err := p.client.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Value...)
		if len(resp.Value) < historyPage {
			return all, nil
		}
	}
}

// lastTouch finds the latest update after since that the user made to the
// state, tags or assignee, or that put the user in or out of the assignee.
// me is the user's sign-in name.
func lastTouch(ups []update, me string, since time.Time) (provider.Touch, bool) {
	isMe := func(raw json.RawMessage) bool {
		id := decode(raw).ID
		return id != "" && strings.EqualFold(id, me)
	}
	for i := len(ups) - 1; i >= 0; i-- {
		u := ups[i]
		at := updateTime(u)
		if at.Before(since) {
			return provider.Touch{}, false
		}
		byMe := strings.EqualFold(u.RevisedBy.UniqueName, me)
		if f, ok := u.Fields[fieldState]; ok && byMe {
			return provider.Touch{At: at, Kind: provider.ChangeStatus,
				What: arrow(decode(f.OldValue).Text, decode(f.NewValue).Text)}, true
		}
		if f, ok := u.Fields[fieldAssignee]; ok && (byMe || isMe(f.OldValue) || isMe(f.NewValue)) {
			name := func(raw json.RawMessage) string {
				if isMe(raw) {
					return "me"
				}
				return decode(raw).Text
			}
			return provider.Touch{At: at, Kind: provider.ChangeAssignee,
				What: "Assignee " + arrow(name(f.OldValue), name(f.NewValue))}, true
		}
		if f, ok := u.Fields[fieldTags]; ok && byMe {
			if what := tagChange(decode(f.OldValue).Text, decode(f.NewValue).Text); what != "" {
				return provider.Touch{At: at, Kind: provider.ChangeTags, What: what}, true
			}
		}
	}
	return provider.Touch{}, false
}

// updateTime is when an update was made. It is the new System.ChangedDate
// of the update; revisedDate is when the next update replaced it, and far
// in the future on the latest.
func updateTime(u update) time.Time {
	if f, ok := u.Fields[fieldChanged]; ok {
		if t := parseTime(f.NewValue); !t.IsZero() {
			return t
		}
	}
	return parseTime(u.RevisedDate)
}

// arrow writes a change from one value to another, with a dash for none.
func arrow(from, to string) string {
	if from == "" {
		from = "–"
	}
	if to == "" {
		to = "–"
	}
	return from + " → " + to
}

// tagChange writes the tags added with a plus and the tags taken off with
// a minus, or nothing when the tags are the same.
func tagChange(from, to string) string {
	old, cur := SplitTags(from), SplitTags(to)
	has := func(list []string, tag string) bool {
		return slices.ContainsFunc(list, func(t string) bool { return strings.EqualFold(t, tag) })
	}
	var parts []string
	for _, t := range cur {
		if !has(old, t) {
			parts = append(parts, "+"+t)
		}
	}
	for _, t := range old {
		if !has(cur, t) {
			parts = append(parts, "−"+t)
		}
	}
	return strings.Join(parts, " ")
}
