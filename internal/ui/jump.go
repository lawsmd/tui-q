package ui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lawsmd/tui-q/internal/ops"
	"github.com/lawsmd/tui-q/ticket"
)

// Widths of the jump box with the keyboard and without it.
const (
	jumpOpenWidth = 52
	jumpIdleWidth = 24
)

// jumpHint is the text in the empty jump box.
const jumpHint = "ticket number, key or link"

// jumpedMsg carries a ticket loaded from the jump box, and the moves open
// to it.
type jumpedMsg struct {
	t   ticket.Ticket
	trs []ticket.Transition
}

func newJump(s Styles) textinput.Model {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = jumpHint
	st := in.Styles()
	st.Focused.Text, st.Blurred.Text = s.Jump, s.Jump
	st.Focused.Placeholder, st.Blurred.Placeholder = s.JumpHint, s.JumpHint
	in.SetStyles(st)
	return in
}

// focusJump gives the keyboard to the jump box.
func (m *Model) focusJump() tea.Cmd {
	if m.jumping {
		return nil
	}
	m.menu = nil
	m.jumping = true
	m.jump.SetValue("")
	return m.jump.Focus()
}

func (m *Model) blurJump() {
	m.jumping = false
	m.jump.Blur()
}

// jumpKey handles a key while the jump box has the keyboard.
func (m *Model) jumpKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit
	case "esc":
		m.blurJump()
		return nil
	case "enter":
		text := m.jump.Value()
		m.blurJump()
		return m.jumpTo(text)
	}
	var cmd tea.Cmd
	m.jump, cmd = m.jump.Update(msg)
	return cmd
}

// jumpTo opens the ticket that text names: a number, a key or a link. A
// ticket that is in no queue is opened all the same.
func (m *Model) jumpTo(text string) tea.Cmd {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	id, err := m.prov.Resolve(text)
	if err != nil {
		m.notice = err.Error()
		return nil
	}
	for i, tab := range m.tabs {
		if strings.EqualFold(tab.t.ID, id) || strings.EqualFold(tab.t.Key, id) {
			m.activeTab = i
			return nil
		}
	}
	return m.loadByID(id)
}

// loadByID loads a ticket that no queue has supplied, and shows it in a
// tab once it has loaded.
func (m *Model) loadByID(id string) tea.Cmd {
	return m.start("Loading "+id, ops.Wait, func(ctx context.Context) (tea.Msg, error) {
		t, trs, err := m.read(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", id, err)
		}
		return jumpedMsg{t: t, trs: trs}, nil
	})
}

// showLoaded shows a ticket from the jump box in a tab of its own, or in
// the tab it already has.
func (m *Model) showLoaded(msg jumpedMsg) {
	m.warmEditor()
	for i, tab := range m.tabs {
		if tab.t.ID == msg.t.ID {
			tab.set(msg.t)
			tab.transitions = msg.trs
			m.activeTab = i
			return
		}
	}
	tab := &ticketTab{transitions: msg.trs}
	tab.set(msg.t)
	m.tabs = append(m.tabs, tab)
	m.activeTab = len(m.tabs) - 1
}

// jumpLayer draws the jump box on the top row so that it ends before
// column right, and returns the column it starts at. The
// queue tabs need the first need columns of the row. Without the keyboard
// the box is drawn short, or as a button, or left out, whichever is the
// most that leaves the queue tabs their room; the slash key opens it in
// every case. With the keyboard it is wide enough for a link and covers
// the tabs beside it.
func (m *Model) jumpLayer(right, need int) (*lipgloss.Layer, int) {
	s := m.styles
	idle, x := "", right+1
	for _, form := range []string{s.JumpHint.Render(fit(" go to ticket", jumpIdleWidth)), s.Button.Render("go to")} {
		if at := right - lipgloss.Width(form); at-1 >= need {
			idle, x = form, at
			break
		}
	}
	if !m.jumping {
		if idle == "" {
			return nil, x
		}
		return lipgloss.NewLayer(idle).ID(idJump).X(x).Z(zItem), x
	}
	// One cell on each side of the text, and one for the cursor at its end.
	bw := max(min(jumpOpenWidth, right), 4)
	m.jump.SetWidth(bw - 3)
	// The cursor ends the colours, so the cells around the text are drawn
	// on their own.
	view := ansi.Truncate(m.jump.View(), bw-2, "")
	box := s.Jump.Render(" ") + view + s.Jump.Render(strings.Repeat(" ", max(bw-1-lipgloss.Width(view), 0)))
	return lipgloss.NewLayer(box).ID(idJump).X(right - bw).Z(zButton), x
}
