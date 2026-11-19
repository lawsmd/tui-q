// Package arrange groups the tickets of a queue into the buckets of a table
// or the lanes of a board. The interface and "tq order" both use it, so the
// two show the same tickets in the same places.
package arrange

import (
	"slices"
	"strings"

	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/ticket"
)

// FromQueue marks a ticket that the queue's own query returned.
const FromQueue = -1

// NoLane names the lane for tickets whose group field is empty.
const NoLane = "(none)"

// Sections sorts the tickets of a table with buckets: one list of ticket
// indexes for each bucket, and a last one for the tickets in none. from
// says where each ticket came from: the index of the bucket whose own query
// loaded it, or FromQueue. A ticket past the end of from counts as
// FromQueue. A ticket loaded by a bucket's own query is in that bucket, and
// any other goes in the first bucket whose values it matches.
func Sections(cfg *profile.Queue, tickets []ticket.Ticket, from []int) [][]int {
	sections := make([][]int, len(cfg.Buckets)+1)
	for i, t := range tickets {
		at := len(cfg.Buckets)
		if i < len(from) && from[i] != FromQueue {
			at = from[i]
		} else {
			for j, b := range cfg.Buckets {
				if b.Query != "" {
					continue
				}
				value := t.Field(b.Field).String()
				if slices.ContainsFunc(b.Values, func(v string) bool { return strings.EqualFold(v, value) }) {
					at = j
					break
				}
			}
		}
		sections[at] = append(sections[at], i)
	}
	return sections
}

// Lane is one column of a board.
type Lane struct {
	Name string
	// Tickets are indexes into the queue's tickets.
	Tickets []int
}

// LaneOf is the name of the lane a ticket belongs in.
func LaneOf(cfg *profile.Queue, t ticket.Ticket) string {
	if name := t.Field(cfg.GroupBy).String(); name != "" {
		return name
	}
	return NoLane
}

// Lanes groups the tickets of a board. The profile's lanes come first, in
// its order and even when empty. A value the profile does not list gets a
// lane after them, so no ticket is left off the board.
func Lanes(cfg *profile.Queue, tickets []ticket.Ticket) []Lane {
	var out []Lane
	index := map[string]int{}
	for _, name := range cfg.Lanes {
		index[strings.ToLower(name)] = len(out)
		out = append(out, Lane{Name: name})
	}
	for i, t := range tickets {
		name := LaneOf(cfg, t)
		n, ok := index[strings.ToLower(name)]
		if !ok {
			n = len(out)
			index[strings.ToLower(name)] = n
			out = append(out, Lane{Name: name})
		}
		out[n].Tickets = append(out[n].Tickets, i)
	}
	return out
}
