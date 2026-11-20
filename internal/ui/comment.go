package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lawsmd/tui-q/internal/editor"
	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// The comment box is part of every ticket view, under the heading of the
// comments and beside the newest one, and scrolls with them: one line
// that names it and holds its buttons, and the rows of the editor under
// that line. It is short while the keyboard is elsewhere and grows when it
// is written in. The editor is placed over the rows that are in view.
const (
	// composerIdleRows is the most rows the editor has while the keyboard
	// is elsewhere, and composerRows the most it has while written in.
	composerIdleRows = 3
	composerRows     = 14
	// composerSmallRows is the fewest rows a box being written in has,
	// where the pane has that many.
	composerSmallRows = 4
)

// draftDelay is how long after a change the draft is written, so that
// typing does not write a file for every key.
const draftDelay = 500 * time.Millisecond

// composer is the comment box of the ticket on screen. There is one at
// most: showing another ticket or a queue closes it and keeps its text as
// a draft.
type composer struct {
	// id and key name the ticket.
	id, key string
	// box is the editor, or nil until it has opened.
	box editor.Box
	// rect is where the box was last placed and covered whether it was
	// last told that it cannot draw.
	rect    area
	covered bool
	// writing says the box has the keyboard, or gets it once it has opened.
	writing bool
	// dragging says a mouse button went down inside the box and has not
	// come up, so the pointer's moves are the box's.
	dragging bool
	// changed says the text changed since the box opened, and saves counts
	// the changes so that only the newest one writes the draft.
	changed bool
	saves   int
	// reveal says the ticket is to be scrolled until the box is in view,
	// which happens the next time the box is placed.
	reveal bool
}

// composerOpenedMsg carries the editor for the comment on one ticket, or
// why it could not be opened.
type composerOpenedMsg struct {
	id  string
	box editor.Box
	err error
}

// draftSaveMsg says it is time to write the draft, unless a later change
// has taken its place.
type draftSaveMsg struct{ n int }

// commentedMsg reports a comment that was sent. changed is what the read
// back of the ticket gave.
type commentedMsg struct {
	id      string
	changed tea.Msg
}

// commenter returns the provider's way to add a comment, or nil when the
// provider or this build has none.
func (m *Model) commenter() provider.Commenter {
	if m.openEditor == nil {
		return nil
	}
	c, _ := m.prov.(provider.Commenter)
	return c
}

// commentKind is the kind the comment on a ticket will be posted as: the
// one picked for it, or the first the provider lists. It is empty for a
// provider with no kinds.
func (m *Model) commentKind(id string) string {
	cm := m.commenter()
	if cm == nil {
		return ""
	}
	kinds := cm.CommentKinds()
	if len(kinds) == 0 {
		return ""
	}
	for _, k := range kinds {
		if k.ID == m.kinds[id] {
			return k.ID
		}
	}
	return kinds[0].ID
}

// pickCommentKind sets the kind the comment on the ticket on screen will
// be posted as to the nth the provider lists, or to the one after the
// present kind when n is negative.
func (m *Model) pickCommentKind(n int) {
	t, ok := m.shownTicket()
	cm := m.commenter()
	if !ok || cm == nil {
		return
	}
	kinds := cm.CommentKinds()
	if len(kinds) == 0 || n >= len(kinds) {
		return
	}
	if n < 0 {
		now := m.commentKind(t.ID)
		for i, k := range kinds {
			if k.ID == now {
				n = (i + 1) % len(kinds)
			}
		}
	}
	if m.kinds == nil {
		m.kinds = map[string]string{}
	}
	m.kinds[t.ID] = kinds[n].ID
}

// warmEditor gets the editor ready the first time a ticket is shown, and
// keeps it ready from then on, so that a box opens at once.
func (m *Model) warmEditor() {
	if m.warm != nil && m.commenter() != nil {
		m.warm()
		m.warm = nil
	}
}

// shownTicket returns the ticket on screen, or false when a queue is.
func (m *Model) shownTicket() (ticket.Ticket, bool) {
	if m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		return ticket.Ticket{}, false
	}
	t := m.tabs[m.activeTab].t
	return t, t.ID != ""
}

// openComposer opens the comment box for a ticket with the draft kept for
// it. The keyboard stays where it is.
func (m *Model) openComposer(t ticket.Ticket) tea.Cmd {
	cm := m.commenter()
	if cm == nil || t.ID == "" {
		return nil
	}
	m.closeComposer(true)
	m.composer = &composer{id: t.ID, key: t.Key}
	open := m.openEditor
	cfg := editor.Config{
		Format:     cm.CommentFormat(),
		Markdown:   m.drafts.Get(t.ID),
		Title:      "comment on " + t.Key,
		Meta:       map[string]string{"ticket": t.Key},
		Background: m.styles.ComposerBack,
		// The editor calls this from its own goroutine while the user
		// types a name, so it does not go through the operation runner.
		Mentions: func(ctx context.Context, query string) ([]ticket.User, error) {
			return m.prov.Users(ctx, t.ID, query)
		},
	}
	// Opening starts a page in a browser, which takes too long for Update.
	return m.start("Opening the comment box", false, func(context.Context) (tea.Msg, error) {
		box, err := open(cfg)
		return composerOpenedMsg{id: t.ID, box: box, err: err}, nil
	})
}

// composerOpened takes the editor that openComposer asked for.
func (m *Model) composerOpened(msg composerOpenedMsg) tea.Cmd {
	c := m.composer
	if c == nil || c.id != msg.id || c.box != nil {
		// The ticket left the screen while the editor was opening.
		if msg.box != nil {
			msg.box.Close()
		}
		return nil
	}
	if msg.err != nil {
		// The box is not opened again until the user asks for it.
		m.composer = nil
		m.noEditor = msg.err.Error()
		m.notice = "The comment box did not open: " + m.noEditor
		return nil
	}
	c.box = msg.box
	c.box.Focus(c.writing)
	c.reveal = c.writing
	// The box needs the window's size before it can draw.
	return tea.Batch(c.box.Init(), c.box.Changes(), c.box.Update(tea.WindowSizeMsg{Width: m.w, Height: m.h}))
}

// writeComment gives the keyboard to the comment box of the ticket on
// screen. A box that failed to open is tried again.
func (m *Model) writeComment() tea.Cmd {
	t, ok := m.shownTicket()
	if !ok || m.commenter() == nil {
		return nil
	}
	var cmd tea.Cmd
	if m.composer == nil || m.composer.id != t.ID {
		m.noEditor = ""
		cmd = m.openComposer(t)
	}
	m.focusComposer(true)
	return cmd
}

// focusComposer gives the keyboard to the comment box or takes it away.
func (m *Model) focusComposer(on bool) {
	c := m.composer
	if c == nil || c.writing == on {
		return
	}
	c.writing, c.reveal = on, on
	if c.box != nil {
		c.box.Focus(on)
	}
	if on && m.jumping {
		m.blurJump()
	}
}

// saveDraft keeps the text of the comment box as the ticket's draft.
func (m *Model) saveDraft() {
	c := m.composer
	if c == nil || c.box == nil || !c.changed {
		return
	}
	if err := m.drafts.Set(c.id, c.box.Text()); err != nil {
		m.notice = "The draft is kept until tui-q exits, but saving it failed: " + err.Error()
	}
}

// closeComposer removes the comment box. With keep set its text stays as
// the ticket's draft.
func (m *Model) closeComposer(keep bool) {
	c := m.composer
	if c == nil {
		return
	}
	if keep {
		m.saveDraft()
	}
	if c.box != nil {
		c.box.Close()
	}
	m.composer = nil
}

// discardComment empties the comment box and removes the draft. The box
// is closed, and opens again empty.
func (m *Model) discardComment() {
	c := m.composer
	if c == nil {
		return
	}
	id, key := c.id, c.key
	had := m.drafts.Get(id) != "" || (c.box != nil && strings.TrimSpace(c.box.Text()) != "")
	m.closeComposer(false)
	if err := m.drafts.Set(id, ""); err != nil {
		m.notice = "Removing the draft failed: " + err.Error()
		return
	}
	delete(m.kinds, id)
	if had {
		m.notice = "Discarded the comment on " + key
	}
}

// Close removes the comment box, keeping its text as a draft. The program
// calls it once the interface has ended: an editor left open would leave
// its picture on the terminal.
func (m *Model) Close() { m.closeComposer(true) }

// sendComment posts the text of the comment box as a comment on its
// ticket. The box and the draft stay until the platform has taken it.
func (m *Model) sendComment() tea.Cmd {
	c, cm := m.composer, m.commenter()
	if c == nil || c.box == nil || cm == nil || m.ops.Locked() {
		return nil
	}
	if strings.TrimSpace(c.box.Text()) == "" {
		m.notice = "There is nothing to send: the comment is empty."
		return nil
	}
	box, id, key := c.box, c.id, c.key
	kind, what := m.commentKind(id), "Commented on "+key
	for _, k := range cm.CommentKinds() {
		if k.ID == kind {
			what = "Commented on " + key + " (" + k.Name + ")"
		}
	}
	return m.write("Commenting on "+key, nil, func(ctx context.Context) (tea.Msg, error) {
		body, err := box.Export(ctx)
		if err != nil {
			return nil, fmt.Errorf("%s: the comment could not be read from the box: %w", key, err)
		}
		if err := cm.AddComment(ctx, id, kind, body); err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		return commentedMsg{id: id, changed: m.reloaded(ctx, id, what)}, nil
	})
}

// commented clears away a comment that was sent. The box is closed, and
// opens again empty.
func (m *Model) commented(msg commentedMsg) {
	if c := m.composer; c != nil && c.id == msg.id {
		m.closeComposer(false)
	}
	delete(m.kinds, msg.id)
	if err := m.drafts.Set(msg.id, ""); err != nil {
		m.notice = "Removing the draft failed: " + err.Error()
	}
}

// ticketBody is the part of a ticket view's area under its row of
// buttons, which the ticket and the comment box share.
func ticketBody(content area) area {
	return area{x: content.x + 1, y: content.y + 2, w: content.w - 2, h: content.h - 2}
}

// editorRows is how many rows the editor of the comment box has in a
// column h rows high. Written in, it has half the column, and never more
// than leaves its line in view with it.
func editorRows(c *composer, h int) int {
	if c == nil || c.box == nil || h < 2 {
		return 0
	}
	if !c.writing {
		return min(composerIdleRows, h-1)
	}
	return min(clamp(h/2, composerSmallRows, composerRows), h-1)
}

// boxPlace is where the comment box of a ticket view is. The box has
// rows+2 lines among those of the main column: its own line, the rows of
// the editor, and a blank line. line and rect are the parts of it in view,
// in cells of the window, and have no height when out of view.
type boxPlace struct {
	on   bool
	rows int
	line area
	rect area
}

// placeBox finds the comment box of the ticket in the column c, which has
// n lines without it, before line at. It holds the offset of the column to
// the lines there are with the box, and scrolls a box that has just been
// given the keyboard into view.
func (tab *ticketTab) placeBox(m *Model, c area, n, at int) boxPlace {
	if m.commenter() == nil || tab.t.ID == "" || at < 0 || c.h <= 0 || c.w <= 0 {
		return boxPlace{}
	}
	cm := m.composer
	if cm != nil && cm.id != tab.t.ID {
		cm = nil
	}
	p := boxPlace{on: true, rows: editorRows(cm, c.h)}
	if cm != nil && cm.reveal {
		cm.reveal = false
		if end := at + p.rows + 1; end > tab.offset+c.h {
			tab.offset = end - c.h
		}
		tab.offset = min(tab.offset, at)
	}
	tab.offset = clamp(tab.offset, 0, n+p.rows+2-c.h)
	if row := at - tab.offset; row >= 0 && row < c.h {
		p.line = area{x: c.x, y: c.y + row, w: c.w, h: 1}
	}
	top, bottom := max(at+1-tab.offset, 0), min(at+1+p.rows-tab.offset, c.h)
	if bottom > top {
		p.rect = area{x: c.x, y: c.y + top, w: c.w, h: bottom - top}
	}
	return p
}

// composerPlace is where the comment box of the ticket on screen is.
func (m *Model) composerPlace() boxPlace {
	if m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		return boxPlace{}
	}
	tab := m.tabs[m.activeTab]
	c, lines, _, _, at := tab.mainColumn(m, m.contentArea())
	return tab.placeBox(m, c, len(lines), at)
}

// syncComposer brings the comment box in line with the screen after a
// message. It closes the box when its ticket is no longer shown and opens
// one for the ticket that is, moves the box when the layout changed, and
// stops it drawing while something is shown over it.
func (m *Model) syncComposer() tea.Cmd {
	t, shown := m.shownTicket()
	c := m.composer
	if c != nil && (!shown || t.ID != c.id) {
		m.closeComposer(true)
		c = nil
	}
	if !shown {
		return nil
	}
	if c == nil {
		if m.noEditor != "" {
			return nil
		}
		return m.openComposer(t)
	}
	if c.box == nil {
		return nil
	}
	var cmds []tea.Cmd
	rect := m.composerPlace().rect
	ok := rect.h > 0 && rect.w > 0 && m.w >= minWidth && m.h >= minHeight
	if ok && rect != c.rect {
		c.rect = rect
		cmds = append(cmds, c.box.SetRect(rect.x, rect.y, rect.w, rect.h))
	}
	if covered := !ok || m.menu != nil || m.form != nil; covered != c.covered {
		c.covered = covered
		cmds = append(cmds, c.box.Cover(covered))
	}
	return tea.Batch(cmds...)
}

// composerKey handles a key while the comment box has the keyboard. It
// reports false for a key that is not the box's to take.
func (m *Model) composerKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	c := m.composer
	if c == nil || !c.writing || m.menu != nil || m.ops.Locked() {
		return nil, false
	}
	switch msg.String() {
	case "esc":
		m.focusComposer(false)
		return nil, true
	case "ctrl+s":
		return m.sendComment(), true
	case "ctrl+k":
		if cm := m.commenter(); cm != nil && len(cm.CommentKinds()) > 1 {
			m.pickCommentKind(-1)
			return nil, true
		}
	}
	if c.box == nil {
		// The box is still opening. A key typed now is dropped, not taken
		// for one of the ticket view's shortcuts.
		return nil, true
	}
	return c.box.Update(msg), true
}

// composerMouse gives the comment box the mouse events that are its own.
// It reports false for one the rest of the interface should handle.
func (m *Model) composerMouse(msg tea.MouseMsg) (tea.Cmd, bool) {
	c := m.composer
	if c == nil || c.box == nil || c.covered {
		return nil, false
	}
	at := msg.Mouse()
	inside := c.box.Contains(at.X, at.Y)
	switch msg.(type) {
	case tea.MouseClickMsg:
		if !inside {
			// A click anywhere else takes the keyboard out of the box. The
			// click then runs as any other, and one on the box's own line
			// gives the keyboard back.
			if kind, _ := splitID(m.at(at.X, at.Y)); kind != idComment && kind != idCommentSend && kind != idCommentKind {
				m.focusComposer(false)
			}
			return nil, false
		}
		m.notice = ""
		m.mouse.Cancel()
		m.focusComposer(true)
		c.dragging = true
		return c.box.Update(msg), true
	case tea.MouseWheelMsg:
		// The wheel scrolls the ticket past a box that is not written in.
		if !inside || !c.writing {
			return nil, false
		}
		return c.box.Update(msg), true
	case tea.MouseReleaseMsg:
		if !c.dragging {
			return nil, false
		}
		c.dragging = false
		return c.box.Update(msg), true
	}
	if !c.dragging {
		return nil, false
	}
	return c.box.Update(msg), true
}

// composerPass gives the comment box a message the model has no use for.
// The box's own messages arrive this way, so none may be dropped. Pasted
// text is the box's only while it has the keyboard.
func (m *Model) composerPass(msg tea.Msg) tea.Cmd {
	c := m.composer
	if c == nil || c.box == nil {
		return nil
	}
	if _, paste := msg.(tea.PasteMsg); paste && (!c.writing || c.covered || m.jumping || m.ops.Locked()) {
		return nil
	}
	return c.box.Update(msg)
}

// composerChanged notes a change to the text and writes the draft a moment
// later.
func (m *Model) composerChanged() tea.Cmd {
	c := m.composer
	if c == nil || c.box == nil {
		return nil
	}
	c.changed = true
	c.saves++
	n := c.saves
	return tea.Batch(c.box.Changes(), tea.Tick(draftDelay, func(time.Time) tea.Msg { return draftSaveMsg{n: n} }))
}

// composerLayers draws the parts of the comment box in view: its line,
// with its name and its buttons, and the rows of the editor as blank cells
// of the colour the editor is drawn on. The editor draws its picture over
// those cells. Text put there would wipe the picture out; the blank cells
// do not, and they clear what a picture leaves behind where the box was.
func (m *Model) composerLayers(t ticket.Ticket, at boxPlace) []*lipgloss.Layer {
	s := m.styles
	c := m.composer
	if c != nil && c.id != t.ID {
		c = nil
	}
	var layers []*lipgloss.Layer
	if r := at.rect; r.h > 0 {
		row := s.Composer.Render(strings.Repeat(" ", r.w))
		rows := strings.TrimSuffix(strings.Repeat(row+"\n", r.h), "\n")
		layers = append(layers, lipgloss.NewLayer(rows).X(r.x).Y(r.y).Z(zItem))
	}
	panel := at.line
	if panel.h == 0 {
		return layers
	}
	open := c != nil && c.box != nil
	name := s.Composer.Render(" Comment ")
	if open && c.writing {
		name = s.ComposerFocus.Render(" Comment ")
	}

	// The buttons are at the right-hand end of the line. One that does not
	// fit beside the name is left out; its key still works.
	right := panel.x + panel.w - 1
	if open {
		buttons := []struct{ id, text string }{{idCommentSend, "send"}, {idCommentDiscard, "discard"}}
		for i := len(buttons) - 1; i >= 0; i-- {
			text := s.Button.Render(buttons[i].text)
			x := right - lipgloss.Width(text)
			if x < panel.x+lipgloss.Width(name) {
				continue
			}
			layers = append(layers, lipgloss.NewLayer(text).ID(buttons[i].id).X(x).Y(panel.y).Z(zButton))
			right = x - 1
		}
	}

	// The kinds of comment follow the name, with the one the comment will
	// be posted as marked. A click on a kind picks it.
	used := lipgloss.Width(name)
	if cm := m.commenter(); open && cm != nil {
		if kinds := cm.CommentKinds(); len(kinds) > 1 {
			now, x := m.commentKind(t.ID), panel.x+used
			for i, k := range kinds {
				style := s.ComposerKind
				if k.ID == now {
					style = s.ComposerKindOn
				}
				text := style.Render(" " + k.Name + " ")
				if x+lipgloss.Width(text) > right {
					break
				}
				layers = append(layers, lipgloss.NewLayer(text).ID(layerID(idCommentKind, i)).X(x).Y(panel.y).Z(zButton))
				x += lipgloss.Width(text)
			}
		}
	}

	// The line itself can be clicked to write. It says why when the editor
	// did not open, since there is then no box to see.
	text := name
	if room := right - panel.x - lipgloss.Width(name); c == nil && m.noEditor != "" && room >= 8 {
		text += s.Composer.Render(ansi.Truncate("did not open: "+m.noEditor, room, "…"))
	}
	text += s.Composer.Render(strings.Repeat(" ", max(panel.w-lipgloss.Width(text), 0)))
	return append(layers, lipgloss.NewLayer(text).ID(idComment).X(panel.x).Y(panel.y).Z(zItem))
}
