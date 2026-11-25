package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// cellOf is the first cell of a piece of text on screen. The rows it is
// looked for in hold no character two cells wide.
func cellOf(t *testing.T, m *Model, text string) (x, y int) {
	t.Helper()
	for y, row := range strings.Split(screen(m), "\n") {
		if at := strings.Index(row, text); at >= 0 {
			return len([]rune(row[:at])), y
		}
	}
	t.Fatalf("%q is not on screen\n%s", text, screen(m))
	return 0, 0
}

// endOf is the last cell of a piece of text on screen.
func endOf(t *testing.T, m *Model, text string) (x, y int) {
	t.Helper()
	x, y = cellOf(t, m, text)
	return x + len([]rune(text)) - 1, y
}

// press puts the left button down on one cell and moves it to another,
// and draws the frame.
func press(t *testing.T, m *Model, x0, y0, x1, y1 int) {
	t.Helper()
	m.View()
	send(t, m, tea.MouseClickMsg{X: x0, Y: y0, Button: tea.MouseLeft})
	send(t, m, tea.MouseMotionMsg{X: x1, Y: y1, Button: tea.MouseLeft})
	m.View()
}

// release lets the left button go and returns what was put on the
// clipboard.
func release(m *Model, x, y int) (string, bool) {
	_, cmd := m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
	return clipboard(cmd)
}

// dragOver selects from one cell to another and returns what was copied.
func dragOver(t *testing.T, m *Model, x0, y0, x1, y1 int) string {
	t.Helper()
	press(t, m, x0, y0, x1, y1)
	if m.sel == nil {
		t.Fatalf("a drag from %d,%d selects nothing", x0, y0)
	}
	text, ok := release(m, x1, y1)
	if !ok {
		t.Fatalf("the drag from %d,%d to %d,%d copied nothing", x0, y0, x1, y1)
	}
	if m.sel != nil {
		t.Error("the selection outlived the release")
	}
	if want := "Copied " + selectedText; m.notice != want {
		t.Errorf("notice = %q, want %q", m.notice, want)
	}
	return text
}

// reversed reports, for each cell of the frame, whether it is drawn in
// reverse video.
func reversed(m *Model) [][]bool {
	buf := uv.NewScreenBuffer(m.w, m.h)
	uv.NewStyledString(m.View().Content).Draw(buf, buf.Bounds())
	out := make([][]bool, m.h)
	for y := range out {
		out[y] = make([]bool, m.w)
		for x := range out[y] {
			if c := buf.CellAt(x, y); c != nil {
				out[y][x] = c.Style.Attrs&uv.AttrReverse != 0
			}
		}
	}
	return out
}

// ticketShown opens the second ticket of the queue in a pane with the
// sidebar beside the main column.
func ticketShown(t *testing.T) *Model {
	t.Helper()
	m := newModel(t, 110, 40)
	clickLayer(t, m, layerID(idRow, 1))
	m.View()
	if m.tabs[m.activeTab].sideArea.w == 0 {
		t.Fatal("the sidebar is not beside the main column")
	}
	return m
}

func TestDragAlongARowCopiesItsText(t *testing.T) {
	m := ticketShown(t)
	const text = "Sign in as a normal user."
	x0, y := cellOf(t, m, text)
	x1, _ := endOf(t, m, text)
	if got := dragOver(t, m, x0, y, x1, y); got != text {
		t.Errorf("copied %q, want %q", got, text)
	}
	// Dragged the other way, the same cells are the selection.
	if got := dragOver(t, m, x1, y, x0, y); got != text {
		t.Errorf("dragged backwards: copied %q, want %q", got, text)
	}
}

func TestDragDownAColumnStaysInIt(t *testing.T) {
	m := ticketShown(t)
	x0, y0 := cellOf(t, m, "1. Sign in")
	x1, y1 := endOf(t, m, "Compare with the expected result.")
	want := "1. Sign in as a normal user.\n2. Repeat the action from the report.\n3. Compare with the expected result."
	if got := dragOver(t, m, x0, y0, x1, y1); got != want {
		t.Errorf("copied %q, want %q", got, want)
	}

	// A drag that starts in the margin has the margin taken off every row.
	tab := m.tabs[m.activeTab]
	if got := dragOver(t, m, tab.mainArea.x, y0, x1, y1); got != want {
		t.Errorf("from the margin: copied %q, want %q", got, want)
	}

	// The pointer may leave the column: the selection does not.
	side := tab.sideArea
	got := dragOver(t, m, x0, y0, side.x+side.w-1, y1)
	if strings.Contains(got, "│") || !strings.HasSuffix(got, "3. Compare with the expected result.") {
		t.Errorf("with the pointer over the sidebar: copied %q", got)
	}

	// A drag down the sidebar takes nothing of the main column.
	sx, sy := cellOf(t, m, "Type")
	ex, ey := endOf(t, m, "Medium")
	got = dragOver(t, m, sx, sy, ex, ey)
	if !strings.HasPrefix(got, "Type\nBug\n") || !strings.HasSuffix(got, "\nMedium") || strings.Contains(got, "Notes") {
		t.Errorf("down the sidebar: copied %q", got)
	}
}

func TestDragOverACommentLeavesOutItsBorder(t *testing.T) {
	m := ticketShown(t)
	x0, y0 := cellOf(t, m, "A fix is in review:")
	x1, y1 := endOf(t, m, "one new test")
	want := "A fix is in review:\n\n• one change to the handler\n• one new test"
	if got := dragOver(t, m, x0, y0, x1, y1); got != want {
		t.Errorf("copied %q, want %q", got, want)
	}
}

func TestSelectionIsDrawnInReverseVideo(t *testing.T) {
	m := ticketShown(t)
	before, text := reversed(m), screen(m)
	x0, y0 := cellOf(t, m, "1. Sign in")
	x1, y1 := endOf(t, m, "Compare with the expected result.")
	press(t, m, x0, y0, x1, y1)
	if screen(m) != text {
		t.Fatal("the selection changed the text of the frame")
	}
	col := m.tabs[m.activeTab].mainArea
	during := reversed(m)
	for y := range during {
		for x := range during[y] {
			in := y >= y0 && y <= y1 && x >= col.x && x < col.x+col.w &&
				(y > y0 || x >= x0) && (y < y1 || x <= x1)
			if during[y][x] != (before[y][x] != in) {
				t.Fatalf("cell %d,%d: reversed=%v, selected=%v", x, y, during[y][x], in)
			}
		}
	}
	if _, ok := release(m, x1, y1); !ok {
		t.Fatal("the release copied nothing")
	}
	for y, row := range reversed(m) {
		for x := range row {
			if row[x] != before[y][x] {
				t.Fatalf("cell %d,%d is still drawn selected after the release", x, y)
			}
		}
	}
}

func TestSelectionLeavesTheCommentBoxAsItIs(t *testing.T) {
	m, eds := newCommenting(t, 110, 40, Options{})
	openBox(t, m, eds)
	m.View()
	rect := inside(m)
	if rect.h == 0 {
		t.Fatalf("the comment box is not in view\n%s", screen(m))
	}
	col := m.tabs[m.activeTab].mainArea
	before := reversed(m)
	// From the row above the box's border to the row under its border.
	press(t, m, col.x+3, rect.y-2, col.x+col.w-1, rect.y+rect.h+1)
	if m.sel == nil {
		t.Fatal("the drag selects nothing")
	}
	during := reversed(m)
	for y := rect.y - 1; y <= rect.y+rect.h; y++ {
		for x := col.x; x < col.x+col.w; x++ {
			if in := rect.contains(x, y); during[y][x] == (before[y][x] != in) {
				t.Fatalf("cell %d,%d: reversed=%v, in the editor=%v", x, y, during[y][x], in)
			}
		}
	}
}

func TestSelectionLeavesAPictureAsItIs(t *testing.T) {
	m, pics := newPictures(t, 110, 60, nil)
	v := part((*pics)[0])
	if v.h == 0 {
		t.Fatal("the picture is not in view")
	}
	col := m.tabs[m.activeTab].mainArea
	before := reversed(m)
	// From the line that names the picture to the row under it.
	press(t, m, v.x, v.y-1, col.x+col.w-1, v.y+v.h)
	if m.sel == nil {
		t.Fatal("the drag selects nothing")
	}
	during := reversed(m)
	for y := v.y - 1; y <= v.y+v.h; y++ {
		for x := v.x; x < col.x+col.w; x++ {
			if in := v.contains(x, y); during[y][x] == (before[y][x] != in) {
				t.Fatalf("cell %d,%d: reversed=%v, under the picture=%v", x, y, during[y][x], in)
			}
		}
	}
	text, _ := release(m, col.x+col.w-1, v.y+v.h)
	if !strings.HasPrefix(text, "[image: error-page.png]") {
		t.Errorf("copied %q", text)
	}
}

func TestDragFromTheLastColumnSelectsNothing(t *testing.T) {
	// Herdr passes the drag of the scroll bar beside a pane to the program
	// in it, on the last column.
	m := ticketShown(t)
	x, y := m.w-1, m.h/2
	press(t, m, x, y, x, y+3)
	if m.sel != nil {
		t.Fatal("a drag from the last column began a selection")
	}
	if text, ok := release(m, x, y+3); ok {
		t.Errorf("a drag from the last column copied %q", text)
	}
}

func TestEscCancelsASelection(t *testing.T) {
	m := ticketShown(t)
	x, y := cellOf(t, m, "Sign in")
	press(t, m, x, y, x+6, y)
	send(t, m, esc)
	if m.sel != nil {
		t.Fatal("esc did not cancel the selection")
	}
	if m.activeTab < 0 {
		t.Error("the esc that cancelled the selection also left the ticket")
	}
	if text, ok := release(m, x+6, y); ok {
		t.Errorf("a cancelled selection copied %q", text)
	}
}

func TestRefusedInputCancelsASelection(t *testing.T) {
	m := ticketShown(t)
	x, y := cellOf(t, m, "Sign in")
	press(t, m, x, y, x+6, y)
	_, read := m.Update(key('r'))
	if !m.blocked() {
		t.Fatal("reading the ticket again does not refuse input")
	}
	m.Update(tea.MouseMotionMsg{X: x + 9, Y: y, Button: tea.MouseLeft})
	if m.sel != nil {
		t.Fatal("the selection outlived input being refused")
	}
	if text, ok := release(m, x+9, y); ok {
		t.Errorf("a cancelled selection copied %q", text)
	}
	drain(t, m, read)
}

func TestWheelAndRightClickWaitForASelection(t *testing.T) {
	m := ticketShown(t)
	tab := m.tabs[m.activeTab]
	x, y := cellOf(t, m, "Sign in")
	press(t, m, x, y, x+6, y)
	send(t, m, tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelDown})
	send(t, m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseRight})
	if tab.offset != 0 || m.menu != nil {
		t.Fatalf("during a selection: offset=%d, menu open=%v", tab.offset, m.menu != nil)
	}
	if text, _ := release(m, x+6, y); text != "Sign in" {
		t.Errorf("copied %q, want %q", text, "Sign in")
	}
}

func TestDragOverARowOfTheTableCopiesAndOpensNothing(t *testing.T) {
	m := newModel(t, 110, 30)
	x0, y := cellOf(t, m, "DEMO-101")
	x1, _ := endOf(t, m, "DEMO-101")
	if got := dragOver(t, m, x0, y, x1, y); got != "DEMO-101" {
		t.Errorf("copied %q, want DEMO-101", got)
	}
	if len(m.tabs) != 0 {
		t.Error("the drag opened the ticket")
	}
}

func TestDragOverBlankCellsCopiesNothing(t *testing.T) {
	m := ticketShown(t)
	x, y := endOf(t, m, "Sign in as a normal user.")
	press(t, m, x+3, y, x+9, y)
	if m.sel == nil {
		t.Fatal("the drag selects nothing")
	}
	if text, ok := release(m, x+9, y); ok {
		t.Errorf("copied %q from blank cells", text)
	}
	if m.sel != nil || m.notice != "" {
		t.Errorf("after the release: selection=%v, notice=%q", m.sel != nil, m.notice)
	}
}

func TestUnframedAndDedent(t *testing.T) {
	for row, want := range map[string]string{
		"│ text here      │": "text here",
		"│                │": "",
		"    indented    ":   "    indented",
		"│   code │":         "  code",
		"a │ b":              "a │ b",
	} {
		if got := unframed(row); got != want {
			t.Errorf("unframed(%q) = %q, want %q", row, got, want)
		}
	}
	rows := []string{"  if x {", "", "      y()", "  }"}
	dedent(rows)
	if got, want := strings.Join(rows, "\n"), "if x {\n\n    y()\n}"; got != want {
		t.Errorf("dedent gave %q, want %q", got, want)
	}
}
