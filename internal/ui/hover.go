package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// hot holds the kinds of layer that are drawn with their colours swapped
// while the mouse is over them: the buttons, the marks and the labels a
// click acts on, and the items of a menu. Tabs, rows and cards are not in
// it, since one of them is under the mouse almost all the time.
var hot = map[string]bool{
	idRefresh: true, idGear: true, idJump: true, idTabClose: true,
	idMore: true, idRowLink: true, idBucketInfo: true,
	idLeft: true, idRight: true, idPagePrev: true, idPageNext: true,
	idOpen: true, idCopyLink: true, idMove: true, idAssign: true, idAction: true,
	idField: true, idFile: true, idSection: true, idCommentOrder: true, idCommentCopy: true,
	idCommentSend: true, idCommentDiscard: true, idCommentKind: true,
	idMenuItem: true, idFormCancel: true,
	idPickTag: true, idPickItem: true, idPickSave: true, idPickCancel: true,
}

// lit returns the layer to draw for l: l itself, or a copy with its colours
// swapped when the mouse is over it and a click on it does something.
//
// Every layer of the frame passes through here, so a control takes the
// effect by having an ID in hot and needs nothing where it is drawn. A
// button drawn as several layers with one ID, as a wrapped one is, lights
// as a whole.
func (m *Model) lit(l *lipgloss.Layer) *lipgloss.Layer {
	id := l.GetID()
	if id == "" || id != m.hover {
		return l
	}
	kind, n := splitID(id)
	if !hot[kind] {
		return l
	}
	switch {
	case kind == idJump && m.jumping:
		// The box is being typed in.
		return l
	case kind == idMenuItem && m.menu != nil && n >= 0 && n < len(m.menu.items) && m.menu.items[n].run == nil:
		// A heading inside a menu does nothing.
		return l
	case strings.TrimSpace(ansi.Strip(l.GetContent())) == "":
		// Blank cells are the rows of a picture, which is drawn over them.
		return l
	}
	return lipgloss.NewLayer(invert(l.GetContent())).ID(id).X(l.GetX()).Y(l.GetY()).Z(l.GetZ())
}

// invert returns styled text with the two colours of every cell swapped.
// The text keeps its own styles; reverse video is put back after each
// place one of them ends.
//
// The padding of a button has a background and no colour for text, so
// swapped it would take the terminal's own text colour and not the
// button's. Every cell is given the first text colour in s to start from,
// which makes a button one block of that colour.
func invert(s string) string {
	const off = "\x1b[m"
	on := "\x1b[7m"
	if fg := firstForeground(s); fg != "" {
		on += "\x1b[" + fg + "m"
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		line = strings.ReplaceAll(line, "\x1b[0m", off)
		lines[i] = on + strings.ReplaceAll(line, off, off+on) + off
	}
	return strings.Join(lines, "\n")
}

// firstForeground returns the parameters of the first colour for text that
// s sets, as they stand in its escape sequence: "38;2;192;202;245", or
// "38;5;111", or "34". It returns "" when s sets none.
func firstForeground(s string) string {
	for {
		start := strings.Index(s, "\x1b[")
		if start < 0 {
			return ""
		}
		s = s[start+2:]
		end := strings.IndexByte(s, 'm')
		if end < 0 {
			return ""
		}
		params := strings.Split(s[:end], ";")
		for i := 0; i < len(params); i++ {
			switch p := params[i]; {
			case p == "38" || p == "48":
				// An extended colour takes the parameters after it.
				n := 0
				if i+1 < len(params) && params[i+1] == "2" {
					n = 4
				} else if i+1 < len(params) && params[i+1] == "5" {
					n = 2
				}
				if i+n >= len(params) {
					n = len(params) - 1 - i
				}
				if p == "38" && n > 0 {
					return strings.Join(params[i:i+n+1], ";")
				}
				i += n
			case len(p) == 2 && (p[0] == '3' || p[0] == '9') && p[1] >= '0' && p[1] <= '7':
				return p
			}
		}
		s = s[end+1:]
	}
}

// under is the ID of the control of the last frame composed that the mouse
// is over. It is "" over anything that is not drawn differently for it, a
// row or a tab or plain text, so that the mouse crossing those changes
// nothing. It is "" too while no control is lit: before the mouse has
// moved, while input is refused, and while something is being dragged.
func (m *Model) under() string {
	if !m.pointed || m.comp == nil || m.blocked() || m.mouse.Dragging() {
		return ""
	}
	id := m.comp.Hit(m.px, m.py).ID()
	if kind, _ := splitID(id); !hot[kind] {
		return ""
	}
	return id
}
