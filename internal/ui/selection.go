package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/lawsmd/tui-q/internal/hit"
)

// selection is text being selected with the mouse: a left drag that began
// on anything but a ticket tab, the edge of a column or a card. It is held
// in cells of the window. The cells are drawn in reverse video while the
// button is down, and their text is copied when it comes up.
//
// The program takes every mouse event, so the terminal selects nothing in
// it. This stands in for the terminal's own selection.
type selection struct {
	// ax and ay are the cell the button went down on, and x and y the cell
	// the pointer is on now, held inside clip.
	ax, ay, x, y int
	// clip is the part of the window the selection may cover: one column
	// of a ticket, so that a drag down the main column takes nothing of
	// the sidebar, or the whole pane.
	clip area
}

// selectedText names what a selection copied in the status bar.
const selectedText = "the selected text"

// contains reports whether the cell is in the area.
func (a area) contains(x, y int) bool {
	return x >= a.x && x < a.x+a.w && y >= a.y && y < a.y+a.h
}

// spans are the cells selected, as one area for each row. The selection
// runs as a terminal's does: from the first cell to the end of its row,
// the whole of the rows between, and the last row as far as the last cell.
func (s *selection) spans() []area {
	ax, ay, bx, by := s.ax, s.ay, s.x, s.y
	if by < ay || (by == ay && bx < ax) {
		ax, ay, bx, by = bx, by, ax, ay
	}
	spans := make([]area, 0, by-ay+1)
	for y := ay; y <= by; y++ {
		from, to := s.clip.x, s.clip.x+s.clip.w-1
		if y == ay {
			from = ax
		}
		if y == by {
			to = bx
		}
		spans = append(spans, area{x: from, y: y, w: to - from + 1, h: 1})
	}
	return spans
}

// moveTo puts the end of the selection on the cell nearest the pointer
// that the selection may cover.
func (s *selection) moveTo(x, y int) {
	s.x = clamp(x, s.clip.x, s.clip.x+s.clip.w-1)
	s.y = clamp(y, s.clip.y, s.clip.y+s.clip.h-1)
}

// startSelect begins a selection at the cell a drag started on.
func (m *Model) startSelect(ev hit.Event) {
	clip := m.selClip(ev.X, ev.Y)
	if !clip.contains(ev.X, ev.Y) {
		return
	}
	m.notice = ""
	m.sel = &selection{ax: ev.X, ay: ev.Y, x: ev.X, y: ev.Y, clip: clip}
}

// selClip is the part of the window a selection that starts on a cell may
// cover: the column of the ticket on screen the cell is in, or the pane.
func (m *Model) selClip(x, y int) area {
	if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
		tab := m.tabs[m.activeTab]
		for _, col := range []area{tab.mainArea, tab.sideArea} {
			if col.contains(x, y) {
				return col
			}
		}
	}
	return area{w: m.w, h: m.h}
}

// selectGesture handles the mouse while text is being selected. The
// selection follows the pointer, and the release of the button copies it.
// Nothing else the mouse does counts until then: the wheel would move the
// text out from under the selection.
func (m *Model) selectGesture(ev hit.Event) tea.Cmd {
	switch ev.Kind {
	case hit.DragMove:
		m.sel.moveTo(ev.X, ev.Y)
	case hit.Drop:
		m.sel.moveTo(ev.X, ev.Y)
		text := m.selected()
		m.sel = nil
		if text != "" {
			return m.copy(text, selectedText)
		}
	}
	return nil
}

// canvas draws the last frame composed into cells.
func (m *Model) canvas() *lipgloss.Canvas {
	b := m.comp.Bounds()
	return lipgloss.NewCanvas(b.Dx(), b.Dy()).Compose(m.comp)
}

// render draws the frame composed, with the cells of a selection in
// reverse video.
//
// The rows of the comment box and of a picture are left as they are. The
// terminal writes again every cell that changes, and a cell written under
// one of those images wipes that part of it.
func (m *Model) render() string {
	if m.sel == nil {
		return m.comp.Render()
	}
	canvas := m.canvas()
	images := m.imageRects()
	for _, span := range m.sel.spans() {
	cells:
		for x := span.x; x < span.x+span.w; x++ {
			for _, r := range images {
				if r.contains(x, span.y) {
					continue cells
				}
			}
			if c := canvas.CellAt(x, span.y); c != nil {
				c.Style.Attrs ^= uv.AttrReverse
			}
		}
	}
	return canvas.Render()
}

// imageRects are the parts of the window an image is drawn over: the editor
// of the comment box and the pictures in view.
func (m *Model) imageRects() []area {
	var rects []area
	if r := m.composerPlace().rect; r.w > 0 && r.h > 0 {
		rects = append(rects, r)
	}
	for _, p := range m.pics {
		if p.pic == nil {
			continue
		}
		// The rows of the picture that are in view.
		left, right := max(p.whole.X, p.view.X), min(p.whole.X+p.whole.W, p.view.X+p.view.W)
		top, bottom := max(p.whole.Y, p.view.Y), min(p.whole.Y+p.whole.H, p.view.Y+p.view.H)
		if right > left && bottom > top {
			rects = append(rects, area{x: left, y: top, w: right - left, h: bottom - top})
		}
	}
	return rects
}

// selected is the text of the selection, read from the last frame drawn:
// what the user saw in the cells they dragged over. Each row ends at its
// last character, and the blank rows at either end are left out.
//
// The rows that start at the edge of the column lose the margin they
// share, so that the text of a paragraph starts each line where its first
// line does. A first row that starts part of the way along is the text
// from the cell the button went down on, and has no margin to lose.
func (m *Model) selected() string {
	if m.sel == nil || m.comp == nil {
		return ""
	}
	canvas := m.canvas()
	spans := m.sel.spans()
	var rows []string
	for _, span := range spans {
		var row strings.Builder
		for x := span.x; x < span.x+span.w; x++ {
			// The second cell of a character two cells wide is empty.
			if c := canvas.CellAt(x, span.y); c != nil {
				row.WriteString(c.Content)
			}
		}
		rows = append(rows, unframed(row.String()))
	}
	if spans[0].x == m.sel.clip.x {
		dedent(rows)
	} else {
		dedent(rows[1:])
	}
	for len(rows) > 0 && rows[0] == "" {
		rows = rows[1:]
	}
	for len(rows) > 0 && rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
	}
	return strings.Join(rows, "\n")
}

// unframed is a row of selected cells as text: without the spaces that
// fill the row out, and without the side of a box at either end of it and
// the space between the side and the text. A comment is drawn in a box, and
// its text is what was meant.
func unframed(row string) string {
	const side = "│"
	row = strings.TrimRight(row, " ")
	if cut, ok := strings.CutSuffix(row, side); ok {
		row = strings.TrimRight(cut, " ")
	}
	if cut, ok := strings.CutPrefix(row, side); ok {
		row = strings.TrimPrefix(cut, " ")
	}
	return row
}

// dedent takes from every row the spaces that all of them start with. A
// blank row has none to count.
func dedent(rows []string) {
	margin := -1
	for _, row := range rows {
		if row == "" {
			continue
		}
		if n := len(row) - len(strings.TrimLeft(row, " ")); margin < 0 || n < margin {
			margin = n
		}
	}
	for i, row := range rows {
		if margin > 0 && row != "" {
			rows[i] = row[margin:]
		}
	}
}
