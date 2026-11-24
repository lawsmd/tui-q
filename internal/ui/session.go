package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/internal/session"
)

// syncSession tells the session file which tickets are open in tabs, and
// which of them is on screen, when that has changed since it was last
// told. It runs after every message, as syncComposer does, so what it does
// when nothing has changed has to cost next to nothing: it compares the
// tabs with the ones last sent and builds nothing. The file is written
// inside the command it returns, off the UI goroutine.
func (m *Model) syncSession() tea.Cmd {
	if m.session == nil || m.sessionSent != nil && m.sessionCurrent() {
		return nil
	}
	tabs := make([]session.Tab, len(m.tabs))
	for i, tab := range m.tabs {
		t := tab.t
		tabs[i] = session.Tab{Key: t.Key, Status: t.Status, Title: t.Title, Shown: i == m.activeTab}
		for _, l := range t.Links {
			tabs[i].Links = append(tabs[i].Links, session.Link{Relation: relationOf(l), Key: l.Key, Status: l.Status})
		}
	}
	m.sessionSent = tabs
	m.sessionN++
	w, n := m.session, m.sessionN
	return func() tea.Msg {
		// A file that cannot be written leaves "tq tabs" without this
		// interface, and the interface as it is.
		_ = w.Put(n, tabs)
		return nil
	}
}

// sessionCurrent reports whether the tabs last sent to the session file
// are the tabs there are now.
func (m *Model) sessionCurrent() bool {
	if len(m.sessionSent) != len(m.tabs) {
		return false
	}
	for i, tab := range m.tabs {
		t, sent := &tab.t, &m.sessionSent[i]
		if sent.Key != t.Key || sent.Status != t.Status || sent.Title != t.Title ||
			sent.Shown != (i == m.activeTab) || len(sent.Links) != len(t.Links) {
			return false
		}
		for j := range t.Links {
			l, s := &t.Links[j], &sent.Links[j]
			if s.Key != l.Key || s.Status != l.Status || s.Relation != relationOf(*l) {
				return false
			}
		}
	}
	return true
}
