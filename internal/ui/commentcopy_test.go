package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"

	"github.com/lawsmd/tui-q/ticket"
)

// clipboard runs a command and everything batched in it, and returns what
// was put on the clipboard.
func clipboard(cmd tea.Cmd) (text string, ok bool) {
	if cmd == nil {
		return "", false
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			if text, ok := clipboard(c); ok {
				return text, true
			}
		}
	default:
		if strings.Contains(fmt.Sprintf("%T", msg), "setClipboardMsg") {
			return fmt.Sprint(msg), true
		}
	}
	return "", false
}

func TestEveryCommentHasACopyControlOnItsBorder(t *testing.T) {
	m := newModel(t, 110, 60)
	clickLayer(t, m, layerID(idRow, 1))
	m.View()
	tab := m.tabs[m.activeTab]
	rows := strings.Split(screen(m), "\n")
	for i, c := range tab.t.Comments {
		layer := m.comp.GetLayer(layerID(idCommentCopy, i))
		if layer == nil {
			t.Fatalf("the comment by %s has no copy control\n%s", c.Author, screen(m))
		}
		// The control is on the top border of its own comment, and the
		// border goes on to its corner after it.
		row := rows[layer.GetY()]
		if !strings.Contains(row, "╭─ "+c.Author) || !strings.Contains(row, "─ copy ─╮") {
			t.Errorf("the control of the comment by %s is on the row %q", c.Author, row)
		}
	}
}

func TestCopyControlCopiesItsOwnComment(t *testing.T) {
	// The comments are drawn in either order, and the control of each
	// still copies the comment it is on.
	for _, oldestFirst := range []bool{false, true} {
		m := newModel(t, 110, 60)
		clickLayer(t, m, layerID(idRow, 1))
		if oldestFirst {
			clickLayer(t, m, idCommentOrder)
		}
		m.View()
		rows := strings.Split(screen(m), "\n")
		for i, c := range m.tabs[m.activeTab].t.Comments {
			id := layerID(idCommentCopy, i)
			layer := m.comp.GetLayer(id)
			if layer == nil {
				t.Fatalf("no layer %q", id)
			}
			x, y := layer.GetX(), layer.GetY()
			if !strings.Contains(rows[y], c.Author) {
				t.Fatalf("the control for %s is on the row %q", c.Author, rows[y])
			}
			m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
			_, cmd := m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
			text, ok := clipboard(cmd)
			if !ok || text != c.Body {
				t.Errorf("oldest first %v: the control of the comment by %s copied %q", oldestFirst, c.Author, text)
			}
			if want := "Copied the comment by " + c.Author; m.notice != want {
				t.Errorf("notice = %q, want %q", m.notice, want)
			}
			m.View()
		}
	}
}

func TestCopiedCommentNamesItsFiles(t *testing.T) {
	shot := ticket.Attachment{ID: "7", Name: "page.png"}
	c := ticket.Comment{Body: "Seen here:\n\n" + shot.Ref() + "\n\nand again in " + shot.Ref() + ".\n"}
	if got, want := commentText(c), "Seen here:\n\npage.png\n\nand again in page.png."; got != want {
		t.Errorf("copied %q, want %q", got, want)
	}
}

func TestNarrowCommentKeepsItsTitleAndDropsTheControl(t *testing.T) {
	m := newModel(t, 110, 60)
	clickLayer(t, m, layerID(idRow, 1))
	tab := m.tabs[m.activeTab]
	for w := 8; w <= 60; w++ {
		tab.main = nil
		lines, labels := tab.mainLines(m, w)
		for _, l := range labels {
			if !l.copies {
				continue
			}
			border := []rune(strings.TrimRight(ansi.Strip(lines[l.line]), " "))
			if len(border) != w {
				t.Fatalf("width %d: the border is %d cells: %q", w, len(border), string(border))
			}
			// The control covers border and nothing of the title.
			under := string(border[l.x : l.x+len(copyControl)])
			if strings.Trim(under, "─") != "" {
				t.Errorf("width %d: the control covers %q in %q", w, under, string(border))
			}
			if l.x+len(copyControl) != w-2 || border[l.x-1] != '─' {
				t.Errorf("width %d: the control is at %d in %q", w, l.x, string(border))
			}
		}
	}
	tab.main = nil
	_, labels := tab.mainLines(m, ruleIndent+2+minCommentTitle+len(copyControl)+1)
	for _, l := range labels {
		if l.copies {
			t.Error("a border with no room for the title and the control has the control")
		}
	}
}

func TestRowLinkButtonCopiesTheLink(t *testing.T) {
	m := newModel(t, 110, 40)
	m.View()
	q := m.queues[m.activeQueue]
	for i, tk := range q.tickets[:3] {
		id := layerID(idRowLink, i)
		layer := m.comp.GetLayer(id)
		if layer == nil {
			t.Fatalf("row %d has no copy-link button", i)
		}
		x, y := layer.GetX(), layer.GetY()
		if got := m.comp.Hit(x, y).ID(); got != id {
			t.Fatalf("cell %d,%d hits %q, want %q", x, y, got, id)
		}
		m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
		_, cmd := m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
		if text, ok := clipboard(cmd); !ok || text != tk.URL {
			t.Errorf("row %d copied %q, want %q", i, text, tk.URL)
		}
		if m.activeTab >= 0 {
			t.Fatal("the button opened the ticket")
		}
	}
}
