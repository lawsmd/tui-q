package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/lawsmd/tui-q/internal/arrange"
	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/ticket"
)

// Column width limits for a table.
const (
	maxColumnWidth = 22
	// minColumnWidth is the least a user can drag a column down to.
	minColumnWidth = 3
	minTitleWidth  = 20
	columnGap      = 2
	// moreWidth is the width of the menu button at the end of every row,
	// and of the pair of scroll arrows above it in the header.
	moreWidth = 5
)

// moreGlyph is drawn on the menu button of a row or card.
const moreGlyph = "⋮"

// linkGlyph is drawn on the button at the left end of a table row that
// copies the ticket's link, and linkWidth is the room it takes before the
// first column.
const (
	linkGlyph = "⧉"
	linkWidth = 2
)

// queue is one top-level tab and the tickets its query returned.
type queue struct {
	cfg *profile.Queue
	// tickets are the tickets of the queue's own query, then the loaded
	// page of each bucket that has a query of its own. from says where
	// each came from: the index of the bucket, or fromQueue.
	tickets []ticket.Ticket
	from    []int
	// pages is the paging state of each bucket, by its index.
	pages []bucketPage
	// infoAt is where the last frame drew the info mark of each bucket
	// that has one, by the index of the bucket.
	infoAt  map[int][2]int
	loading bool
	// loads are the operations reading the queue that have not ended, by
	// their ID. The queue view takes no input and is drawn grey until there
	// are none.
	loads map[int]bool
	// newest is the number of the read started last for each source of
	// tickets: the queue's own query, or a bucket's. An answer to an older
	// read is dropped, since two reads may end in either order.
	newest   map[int]int
	reads    int
	loadedAt time.Time
	// offset is the index of the first ticket drawn in a table.
	offset int
	// colOffset is how many unpinned table columns are scrolled off to the
	// left, and laneOffset the same for the lanes of a board.
	colOffset  int
	laneOffset int
	// laneScroll is the first card drawn in each lane, by lane name.
	laneScroll map[string]int

	// Set by the last draw, for the handlers that run before the next one.
	cardW, cardH int
	// cols are the columns of the table as drawn, and flex is the index
	// of the one that takes the spare width.
	cols []column
	flex int
}

// reading numbers a read of one source of the queue's tickets that is
// about to start.
func (q *queue) reading(src int) int {
	q.reads++
	q.newest[src] = q.reads
	return q.reads
}

// stale reports whether the answer to read number seq of a source has been
// overtaken by a later read of it.
func (q *queue) stale(src, seq int) bool { return seq < q.newest[src] }

// fromQueue marks a ticket that the queue's own query returned.
const fromQueue = arrange.FromQueue

// bucketPage is which page of a bucket is shown. For a bucket with its
// own query it also holds what the platform said about that page.
type bucketPage struct {
	// page is the page shown, counted from zero.
	page int
	// cursors holds the cursor that asks for each page reached so far.
	cursors []string
	// total is the number of tickets in the bucket, or -1 when the
	// platform does not say, and more says a page follows this one.
	total  int
	more   bool
	loaded bool
}

// page returns the paging state of bucket j.
func (q *queue) page(j int) *bucketPage {
	if len(q.pages) < len(q.cfg.Buckets) {
		q.pages = append(q.pages, make([]bucketPage, len(q.cfg.Buckets)-len(q.pages))...)
	}
	return &q.pages[j]
}

// source is where ticket i came from: a bucket, or fromQueue.
func (q *queue) source(i int) int {
	if i < len(q.from) {
		return q.from[i]
	}
	return fromQueue
}

// setSource replaces the tickets that came from one source and keeps the
// rest, with the queue's own tickets first and then each bucket's.
func (q *queue) setSource(src int, tickets []ticket.Ticket) {
	var all []ticket.Ticket
	var from []int
	for s := fromQueue; s < len(q.cfg.Buckets); s++ {
		if s == src {
			for _, t := range tickets {
				all, from = append(all, t), append(from, s)
			}
			continue
		}
		for i, t := range q.tickets {
			if q.source(i) == s {
				all, from = append(all, t), append(from, s)
			}
		}
	}
	q.tickets, q.from = all, from
}

// line is one line of a table: a ticket, or the heading of a bucket.
type line struct {
	// ticket is the index of the ticket, or -1 on a heading.
	ticket  int
	heading string
	// odd says the ticket is on an odd place in its section.
	odd bool
	// pager is set on the heading of a bucket shown one page at a time,
	// to the index of the bucket plus one. prev and next say which ways
	// its pages can be turned.
	pager      int
	prev, next bool
	// about is set on the heading of a bucket whose profile has About,
	// to the index of the bucket plus one.
	about int
}

// sections sorts the tickets of a table with buckets: one list of ticket
// indexes for each bucket, and a last one for the tickets in none. A
// ticket loaded by a bucket's own query is in that bucket, and any other
// goes in the first bucket whose values it matches.
func (q *queue) sections() [][]int {
	return arrange.Sections(q.cfg, q.tickets, q.from)
}

// lines lists the lines of a table from top to bottom. A table with
// buckets has a blank line and a heading over each one, and a last section named Other for
// the tickets that match no bucket.
func (q *queue) lines() []line {
	if len(q.cfg.Buckets) == 0 {
		out := make([]line, len(q.tickets))
		for i := range q.tickets {
			out[i] = line{ticket: i, odd: i%2 == 1}
		}
		return out
	}
	var out []line
	for j, members := range q.sections() {
		head := line{ticket: -1, heading: fmt.Sprintf("Other (%d)", len(members))}
		if j < len(q.cfg.Buckets) {
			b := q.cfg.Buckets[j]
			head.heading = fmt.Sprintf("%s (%d)", b.Name, len(members))
			if b.About != "" {
				head.about = j + 1
			}
			if b.PageSize > 0 {
				st := q.page(j)
				total, more := st.total, st.more
				if b.Query == "" {
					// The whole bucket is loaded: cut the page out of it.
					total = len(members)
					st.page = clamp(st.page, 0, (total-1)/b.PageSize)
					members = members[st.page*b.PageSize:]
					more = len(members) > b.PageSize
					members = members[:min(len(members), b.PageSize)]
				}
				first := st.page * b.PageSize
				switch {
				case st.page == 0 && !more:
					// The bucket fits on one page: the heading gives its size.
					head.heading = fmt.Sprintf("%s (%d)", b.Name, len(members))
				case len(members) == 0:
					head.heading = b.Name + " (0)"
				case total < 0:
					head.heading = fmt.Sprintf("%s  %d-%d", b.Name, first+1, first+len(members))
				default:
					head.heading = fmt.Sprintf("%s  %d-%d of %d", b.Name, first+1, first+len(members), total)
				}
				head.pager, head.prev, head.next = j+1, st.page > 0, more
			}
		} else if len(members) == 0 || q.cfg.HideUnmatched {
			break
		}
		// A blank line sets each section off from what is above it.
		out = append(out, line{ticket: -1}, head)
		for n, i := range members {
			out = append(out, line{ticket: i, odd: n%2 == 1})
		}
	}
	return out
}

// count is the number of tickets in the queue. A bucket with its own
// query counts for its total when the platform gave one, and for the page
// that is loaded otherwise.
func (q *queue) count() int {
	if len(q.cfg.Buckets) == 0 {
		return len(q.tickets)
	}
	n := 0
	for j, members := range q.sections() {
		switch {
		case j == len(q.cfg.Buckets):
			if !q.cfg.HideUnmatched {
				n += len(members)
			}
		case q.cfg.Buckets[j].Query != "" && q.page(j).loaded && q.page(j).total >= 0:
			n += q.page(j).total
		default:
			n += len(members)
		}
	}
	return n
}

// layers draws the queue in the layout its profile asks for.
func (q *queue) layers(m *Model, a area) []*lipgloss.Layer {
	if len(q.tickets) == 0 {
		text := "No tickets."
		if q.loading {
			text = "Loading..."
		}
		return []*lipgloss.Layer{lipgloss.NewLayer(m.styles.Faint.Render(" " + text)).X(a.x).Y(a.y + 1).Z(zScenery)}
	}
	if q.cfg.Layout == profile.LayoutBoard {
		return q.boardLayers(m, a)
	}
	return q.tableLayers(m, a)
}

// scrollSideways moves the table columns or the board lanes by n places.
// The next draw clamps the result.
func (q *queue) scrollSideways(n int) {
	if q.cfg.Layout == profile.LayoutBoard {
		q.laneOffset += n
		return
	}
	q.colOffset += n
}

// tableLayers draws a header, then one clickable layer per visible ticket
// with a menu button at its right end.
func (q *queue) tableLayers(m *Model, a area) []*lipgloss.Layer {
	s := m.styles
	avail := a.w - 1 - linkWidth - moreWidth

	// Scrolling right is allowed only while columns are hidden on the
	// right, so step back from an offset that hides nothing.
	q.colOffset = max(q.colOffset, 0)
	for q.colOffset > 0 {
		if _, hidden := q.columns(m, avail, q.colOffset-1); hidden > 0 {
			break
		}
		q.colOffset--
	}
	cols, hidden := q.columns(m, avail, q.colOffset)

	q.cols, q.flex = cols, len(cols)-1
	for i, c := range cols {
		if c.name == ticket.FieldTitle {
			q.flex = i
		}
	}

	header := make([]string, len(cols))
	for i, c := range cols {
		// An icon column is one cell wide and has no heading.
		if m.icons(c.name) {
			header[i] = fit("", c.width)
			continue
		}
		header[i] = fit(m.label(c.name), c.width)
	}
	gap := strings.Repeat(" ", columnGap)
	lead := strings.Repeat(" ", 1+linkWidth)
	layers := []*lipgloss.Layer{
		lipgloss.NewLayer(s.Header.Render(fit(lead+strings.Join(header, gap), a.w))).X(a.x).Y(a.y).Z(zScenery),
	}
	layers = append(layers, arrows(s, a.x+a.w-moreWidth, a.y, q.colOffset > 0, hidden > 0)...)

	// The line between two headings is a handle that sizes a column.
	x := a.x + 1 + linkWidth
	for i := 0; i < len(cols)-1; i++ {
		x += cols[i].width
		if target, _ := q.edge(i); !m.icons(cols[target].name) {
			handle := s.Faint.Render("│ ")
			layers = append(layers, lipgloss.NewLayer(handle).ID(layerID(idEdge, i)).X(x).Y(a.y).Z(zButton))
		}
		x += columnGap
	}

	lines := q.lines()
	q.infoAt = map[int][2]int{}
	visible := a.h - 1
	q.offset = clamp(q.offset, 0, len(lines)-visible)
	for row := 0; row < visible && q.offset+row < len(lines); row++ {
		ln := lines[q.offset+row]
		if ln.ticket < 0 && ln.heading == "" {
			continue
		}
		if ln.ticket < 0 {
			heading := s.Bucket.Render(fit(" "+ln.heading, a.w))
			layers = append(layers, lipgloss.NewLayer(heading).X(a.x).Y(a.y+1+row).Z(zScenery))
			// The info mark sits at the right end, just left of the pager.
			if x := a.x + a.w - moreWidth - lipgloss.Width(infoGlyph); ln.about > 0 && x > a.x+2+lipgloss.Width(ln.heading) {
				y := a.y + 1 + row
				layers = append(layers, lipgloss.NewLayer(s.Info.Render(infoGlyph)).ID(layerID(idBucketInfo, ln.about-1)).X(x).Y(y).Z(zButton))
				q.infoAt[ln.about-1] = [2]int{x, y}
			}
			if ln.pager > 0 {
				layers = append(layers, pager(s, a.x+a.w-moreWidth, a.y+1+row, ln.pager-1, ln.prev, ln.next)...)
			}
			continue
		}
		index := ln.ticket
		t := q.tickets[index]
		cells := make([]string, len(cols))
		for i, c := range cols {
			cells[i] = fit(m.cell(t, c.name), c.width)
		}
		style := s.Row
		if ln.odd {
			style = s.RowAlt
		}
		y := a.y + 1 + row
		line := style.Render(fit(lead+strings.Join(cells, gap), a.w))
		more := style.Inherit(s.More).Render("  " + moreGlyph + "  ")
		layers = append(layers,
			lipgloss.NewLayer(line).ID(layerID(idRow, index)).X(a.x).Y(y).Z(zItem),
			lipgloss.NewLayer(more).ID(layerID(idMore, index)).X(a.x+a.w-moreWidth).Y(y).Z(zButton),
		)
		if t.URL != "" {
			link := style.Inherit(s.More).Render(linkGlyph)
			layers = append(layers, lipgloss.NewLayer(link).ID(layerID(idRowLink, index)).X(a.x+1).Y(y).Z(zButton))
		}
	}
	return layers
}

// arrows is the pair of sideways scroll buttons, moreWidth cells wide. A
// direction with nothing to scroll to is drawn faint and cannot be clicked.
func arrows(s Styles, x, y int, left, right bool) []*lipgloss.Layer {
	l := lipgloss.NewLayer(s.ArrowOff.Render(" ‹")).X(x).Y(y).Z(zButton)
	if left {
		l = lipgloss.NewLayer(s.Arrow.Render(" ‹")).ID(idLeft).X(x).Y(y).Z(zButton)
	}
	r := lipgloss.NewLayer(s.ArrowOff.Render("› ")).X(x + 3).Y(y).Z(zButton)
	if right {
		r = lipgloss.NewLayer(s.Arrow.Render("› ")).ID(idRight).X(x + 3).Y(y).Z(zButton)
	}
	return []*lipgloss.Layer{l, r}
}

// pager is the pair of buttons on the heading of a bucket that turn its
// pages, moreWidth cells wide. A direction with no page to go to is drawn
// faint and cannot be clicked.
func pager(s Styles, x, y, bucket int, prev, next bool) []*lipgloss.Layer {
	l := lipgloss.NewLayer(s.PageOff.Render(" ‹")).X(x).Y(y).Z(zButton)
	if prev {
		l = lipgloss.NewLayer(s.Page.Render(" ‹")).ID(layerID(idPagePrev, bucket)).X(x).Y(y).Z(zButton)
	}
	r := lipgloss.NewLayer(s.PageOff.Render("› ")).X(x + 3).Y(y).Z(zButton)
	if next {
		r = lipgloss.NewLayer(s.Page.Render("› ")).ID(layerID(idPageNext, bucket)).X(x + 3).Y(y).Z(zButton)
	}
	return []*lipgloss.Layer{l, r}
}

type column struct {
	name  string
	width int
}

// edge says which column the line after column i sizes, and whether a
// move to the right widens it (1) or narrows it (-1). The column that
// takes the spare width is never the one sized, so the line stays under
// the pointer: left of that column the line moves the column before it,
// and from there on the column after it.
func (q *queue) edge(i int) (target, sign int) {
	if i < q.flex {
		return i, 1
	}
	return i + 1, -1
}

// columns picks the columns that fit in width cells and sizes them, and
// reports how many were left out on the right.
//
// Every column is as wide as the user dragged it, or else as its longest
// value up to a cap; the title column takes what is left. When the title would get too little room,
// columns are dropped from the end of the list until it fits. The first
// column, the title and the icon columns are pinned. Offset skips that
// many of the other columns from the left, which is how the table scrolls
// sideways.
func (q *queue) columns(m *Model, width, offset int) (cols []column, hidden int) {
	var picked []column
	first := true
	for _, name := range m.shown(q) {
		pinned := name == ticket.FieldTitle || m.icons(name)
		if !pinned && first {
			pinned, first = true, false
		}
		if !pinned && offset > 0 {
			offset--
			continue
		}
		picked = append(picked, column{name: name})
	}
	widths := m.prefs.View(q.cfg.Name).Widths
	for {
		cols = append([]column(nil), picked...)
		used := columnGap * (len(cols) - 1)
		flex := -1
		for i, c := range cols {
			if c.name == ticket.FieldTitle {
				flex = i
				continue
			}
			if set, ok := widths[c.name]; ok && !m.icons(c.name) {
				cols[i].width = max(set, minColumnWidth)
				used += cols[i].width
				continue
			}
			w := lipgloss.Width(c.name)
			if m.icons(c.name) {
				w = 1
			}
			for _, t := range q.tickets {
				w = max(w, lipgloss.Width(m.cell(t, c.name)))
			}
			cols[i].width = min(w, maxColumnWidth)
			used += cols[i].width
		}
		if flex < 0 {
			// With no title column the last column takes the slack.
			flex = len(cols) - 1
			used -= cols[flex].width
		}
		rest := width - used
		if rest >= minTitleWidth || len(cols) <= 2 {
			cols[flex].width = max(rest, 1)
			return cols, hidden
		}
		// Drop the last column that is not the title.
		drop := len(picked) - 1
		if picked[drop].name == ticket.FieldTitle {
			drop--
		}
		picked = append(picked[:drop], picked[drop+1:]...)
		hidden++
	}
}

// infoGlyph is the mark on the heading of a bucket that explains it.
const infoGlyph = "(i)"

// tooltipLayers is the box with the About text of the bucket whose info
// mark the mouse is over, under the mark, or above it when there is no
// room under it. It is nil when the mouse is over no mark of the queue on
// screen.
func (m *Model) tooltipLayers() []*lipgloss.Layer {
	kind, j := splitID(m.hover)
	if kind != idBucketInfo || m.activeTab >= 0 || m.only != "" || m.menu != nil || m.form != nil || m.picker != nil || m.drag != nil {
		return nil
	}
	q := m.queues[m.activeQueue]
	at, ok := q.infoAt[j]
	if !ok || j >= len(q.cfg.Buckets) {
		return nil
	}
	s := m.styles
	inner := min(56, m.w-4)
	if inner < 1 {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(lipgloss.Wrap(q.cfg.Buckets[j].About, inner, ""), "\n") {
		lines = append(lines, s.MenuItem.Render(fit(l, inner)))
	}
	rows := box(lines, inner+4, s.MenuBorder)
	// The box ends where the mark ends, since the mark is at the right.
	x := clamp(at[0]+lipgloss.Width(infoGlyph)-(inner+4), 0, m.w-(inner+4))
	y := at[1] + 1
	if y+len(rows) > m.floor() {
		y = at[1] - len(rows)
	}
	y = clamp(y, 0, m.floor()-len(rows))
	return []*lipgloss.Layer{lipgloss.NewLayer(strings.Join(rows, "\n")).X(x).Y(y).Z(zMenu)}
}
