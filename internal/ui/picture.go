package ui

import (
	"context"
	"errors"
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/internal/editor"
	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// A ticket view draws the pictures of its ticket under the lines that name
// them, when the profile asks for that. A picture is read the first time
// its line comes into view, and has rows of blank cells among the lines of
// the column from then on. It is drawn over those cells as the editor of
// the comment box is drawn over its own, and scrolls with the column.

// pictureRows is the most rows a picture is given. A taller one is scaled
// down, and a click opens it whole.
const pictureRows = 24

// picture is one picture of the ticket on screen.
type picture struct {
	// pic draws the picture. It is nil while the file is read, and stays
	// nil for a file that could not be drawn.
	pic editor.Picture
	// whole and view are what the picture was last told of its place, and
	// covered whether it was last told that it cannot draw.
	whole, view editor.Rect
	covered     bool
}

// pictureOpenedMsg carries a picture of one ticket that is ready to draw,
// or why it is not.
type pictureOpenedMsg struct {
	ticket, id, name string
	pic              editor.Picture
	err              error
}

// drawsPictures reports whether ticket views draw their pictures.
func (m *Model) drawsPictures() bool {
	_, reads := m.prov.(provider.AttachmentReader)
	return reads && m.openPicture != nil && m.prof.Ticket.Pictures && m.noPictures == ""
}

// pictureOf returns the picture of a ticket's attachment when it is ready
// to draw.
func (m *Model) pictureOf(ticketID, id string) editor.Picture {
	if m.picsOf != ticketID || !m.drawsPictures() {
		return nil
	}
	if p := m.pics[id]; p != nil {
		return p.pic
	}
	return nil
}

// attachmentFile returns the file of an attachment, read from the platform
// the first time and from the store after that.
func (m *Model) attachmentFile(ctx context.Context, a ticket.Attachment) (string, error) {
	if path, ok := m.files.Path(a); ok {
		return path, nil
	}
	body, err := m.prov.(provider.AttachmentReader).ReadAttachment(ctx, a)
	if err != nil {
		return "", fmt.Errorf("%s: %w", a.Name, err)
	}
	defer body.Close()
	return m.files.Save(a, body)
}

// loadPicture reads a picture of the ticket on screen and makes it ready to
// draw.
func (m *Model) loadPicture(t ticket.Ticket, id string) tea.Cmd {
	a, _ := t.Attachment(id)
	open, back := m.openPicture, m.styles.PictureBack
	return m.start("Reading "+a.Name, false, func(ctx context.Context) (tea.Msg, error) {
		path, err := m.attachmentFile(ctx, a)
		if err != nil {
			return nil, err
		}
		// A picture that cannot be drawn is no failure: its line still
		// opens it.
		pic, err := open(path, back)
		return pictureOpenedMsg{ticket: t.ID, id: id, name: a.Name, pic: pic, err: err}, nil
	})
}

// pictureOpened takes the picture that loadPicture asked for.
func (m *Model) pictureOpened(msg pictureOpenedMsg) tea.Cmd {
	p := m.pics[msg.id]
	if m.picsOf != msg.ticket || p == nil || p.pic != nil {
		// The ticket left the screen while the picture was read.
		if msg.pic != nil {
			go msg.pic.Close()
		}
		return nil
	}
	if msg.err != nil {
		if errors.Is(msg.err, editor.ErrNoPictures) {
			// No other picture is tried: each would fail in the same way.
			m.noPictures = msg.err.Error()
			m.notice = "Pictures are shown by name only: " + m.noPictures
			return m.closePictures()
		}
		m.notice = msg.name + " is shown by name only: " + msg.err.Error()
		return nil
	}
	p.pic = msg.pic
	m.dropTicketLines()
	// The picture needs the window's size before it can draw.
	return tea.Batch(p.pic.Init(), p.pic.Update(tea.WindowSizeMsg{Width: m.w, Height: m.h}))
}

// dropTicketLines makes every ticket view lay out its lines again.
func (m *Model) dropTicketLines() {
	for _, tab := range m.tabs {
		tab.main = nil
	}
}

// closePictures removes the pictures on screen. Removing one can take a
// moment, so the command does it.
func (m *Model) closePictures() tea.Cmd {
	open := m.openPictures()
	m.pics, m.picsOf = nil, ""
	if len(open) == 0 {
		return nil
	}
	m.dropTicketLines()
	return func() tea.Msg {
		closeAll(open)
		return nil
	}
}

// openPictures are the pictures ready to draw.
func (m *Model) openPictures() []editor.Picture {
	var open []editor.Picture
	for _, p := range m.pics {
		if p.pic != nil {
			open = append(open, p.pic)
		}
	}
	return open
}

// closeAll closes pictures at once and waits for them.
func closeAll(pics []editor.Picture) {
	var wg sync.WaitGroup
	for _, p := range pics {
		wg.Go(p.Close)
	}
	wg.Wait()
}

// picturePass gives every picture a message the model has no use for. The
// pictures' own messages arrive this way.
func (m *Model) picturePass(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for _, p := range m.pics {
		if p.pic != nil {
			cmds = append(cmds, p.pic.Update(msg))
		}
	}
	return tea.Batch(cmds...)
}

// syncPictures brings the pictures in line with the screen after a message.
// It removes them when their ticket is no longer shown, reads each picture
// whose line has come into view, tells each one where it is, and stops them
// drawing while something is shown over the ticket.
func (m *Model) syncPictures() tea.Cmd {
	t, shown := m.shownTicket()
	var cmds []tea.Cmd
	if m.picsOf != "" && (!shown || t.ID != m.picsOf || !m.drawsPictures()) {
		cmds = append(cmds, m.closePictures())
	}
	if !shown || !m.drawsPictures() {
		return tea.Batch(cmds...)
	}
	if m.pics == nil {
		m.pics, m.picsOf = map[string]*picture{}, t.ID
	}
	tab := m.tabs[m.activeTab]
	c, labels := tab.pictureLabels(m)
	// A picture whose size in cells has changed, as it does when the
	// terminal reports the size of a cell, needs the lines laid out again.
	for _, l := range labels {
		if p := m.pics[l.file]; l.pic && p != nil && p.pic != nil {
			if w, h := p.pic.Fit(l.room, pictureRows); w != l.w || h != l.h {
				tab.main = nil
				c, labels = tab.pictureLabels(m)
				break
			}
		}
	}
	covered := m.menu != nil || m.form != nil || m.w < minWidth || m.h < minHeight
	view := editor.Rect{X: c.x, Y: c.y, W: c.w, H: c.h}
	placed := map[string]bool{}
	for _, l := range labels {
		row := l.line - tab.offset
		p := m.pics[l.file]
		if !l.pic {
			// The line that names a picture has come into view.
			if p == nil && row >= 0 && row < c.h {
				m.pics[l.file] = &picture{}
				cmds = append(cmds, m.loadPicture(t, l.file))
			}
			continue
		}
		if p == nil || p.pic == nil {
			continue
		}
		placed[l.file] = true
		whole := editor.Rect{X: c.x + l.x, Y: c.y + row, W: l.w, H: l.h}
		cmds = append(cmds, m.placePicture(p, whole, view, covered))
	}
	// A picture with no rows among the lines is out of view.
	for id, p := range m.pics {
		if p.pic != nil && !placed[id] {
			cmds = append(cmds, m.placePicture(p, editor.Rect{}, editor.Rect{}, covered))
		}
	}
	return tea.Batch(cmds...)
}

// placePicture tells a picture where it is and whether it may draw, when
// either has changed.
func (m *Model) placePicture(p *picture, whole, view editor.Rect, covered bool) tea.Cmd {
	var cmds []tea.Cmd
	if whole != p.whole || view != p.view {
		p.whole, p.view = whole, view
		cmds = append(cmds, p.pic.Show(whole, view))
	}
	if covered != p.covered {
		p.covered = covered
		cmds = append(cmds, p.pic.Cover(covered))
	}
	return tea.Batch(cmds...)
}

// pictureLabels are the lines of the ticket view that name or hold a
// picture, with the area of the column they are in. The offset of the
// column is held to its lines first, as it is when the column is drawn.
func (tab *ticketTab) pictureLabels(m *Model) (area, []labelLine) {
	c, lines, labels, _, _ := tab.boxedColumn(m, m.contentArea())
	if c.h <= 0 {
		return c, nil
	}
	tab.offset = clamp(tab.offset, 0, len(lines)-c.h)
	var out []labelLine
	for _, l := range labels {
		if l.file != "" {
			out = append(out, l)
		}
	}
	return c, out
}
