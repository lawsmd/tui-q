package ui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lawsmd/tui-q/ext"
	"github.com/lawsmd/tui-q/internal/ops"
	"github.com/lawsmd/tui-q/ticket"
)

// sectState is what a ticket tab shows of one of the instance's sections.
// A section that does not apply to the ticket is not shown.
type sectState struct {
	shown   bool
	loading bool
	lines   []ext.Line
	err     error
}

// sectionReadMsg carries the lines of section i for a ticket.
type sectionReadMsg struct {
	ticketID string
	i        int
	lines    []ext.Line
	err      error
}

// sectHit is the section and line of a clickable section line.
type sectHit struct{ sect, line int }

// readSections reads every section that applies to a ticket, for each tab
// showing it. The lines read before stay on screen until the new ones come.
func (m *Model) readSections(t ticket.Ticket) tea.Cmd {
	if len(m.sections) == 0 || t.ID == "" {
		return nil
	}
	var tabs []*ticketTab
	for _, tab := range m.tabs {
		if tab.t.ID == t.ID {
			tabs = append(tabs, tab)
		}
	}
	if len(tabs) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for i, sec := range m.sections {
		applies := sec.Read != nil && (sec.Applies == nil || sec.Applies(t))
		for _, tab := range tabs {
			if len(tab.sects) != len(m.sections) {
				tab.sects = make([]sectState, len(m.sections))
			}
			st := &tab.sects[i]
			st.shown = applies
			st.loading = applies
			if !applies {
				st.lines, st.err = nil, nil
			}
		}
		if !applies {
			continue
		}
		cmds = append(cmds, m.start("Reading "+sec.Title+" for "+t.Key, ops.Background, func(ctx context.Context) (tea.Msg, error) {
			lines, err := sec.Read(ctx, t)
			return sectionReadMsg{ticketID: t.ID, i: i, lines: lines, err: err}, nil
		}))
	}
	return tea.Batch(cmds...)
}

// sectionRead puts the lines read into each tab still showing the ticket.
func (m *Model) sectionRead(msg sectionReadMsg) {
	for _, tab := range m.tabs {
		if tab.t.ID != msg.ticketID || msg.i >= len(tab.sects) {
			continue
		}
		st := &tab.sects[msg.i]
		if !st.shown {
			continue
		}
		st.loading, st.lines, st.err = false, msg.lines, msg.err
	}
}

// sectionLines draws the sections shown on a tab, each under a rule, to go
// above the profile's fields in the sidebar. A line with actions is cut to
// one line and marked as clickable; the others wrap.
func (tab *ticketTab) sectionLines(m *Model, w int) ([]string, []labelLine) {
	s := m.styles
	var lines []string
	var labels []labelLine
	for i, st := range tab.sects {
		if !st.shown || i >= len(m.sections) {
			continue
		}
		lines = append(lines, s.rule(m.sections[i].Title, w), "")
		wrap := func(text string, style lipgloss.Style) {
			for _, l := range strings.Split(lipgloss.Wrap(text, w, ""), "\n") {
				lines = append(lines, style.Render(l))
			}
		}
		switch {
		case st.err != nil:
			wrap(st.err.Error(), s.Faint)
		case len(st.lines) == 0 && st.loading:
			lines = append(lines, s.Faint.Render("reading..."))
		}
		if st.err == nil {
			for j, l := range st.lines {
				if len(l.Actions) > 0 {
					text := s.Label.Render(fit(l.Text+editMark, w))
					labels = append(labels, labelLine{act: true, sect: i, sline: j, line: len(lines), text: text})
					lines = append(lines, text)
					continue
				}
				style := lipgloss.NewStyle()
				if l.Faint {
					style = s.Faint
				}
				wrap(l.Text, style)
			}
		}
		lines = append(lines, "")
		lines, labels = tab.sideButtons(m, m.sections[i].Title, w, lines, labels)
	}
	return lines, labels
}

// sectionMenu opens the menu of actions of the clickable section line n of
// the last frame drawn.
func (m *Model) sectionMenu(n, x, y int) {
	if m.activeTab < 0 {
		return
	}
	tab := m.tabs[m.activeTab]
	if n < 0 || n >= len(tab.sectHits) {
		return
	}
	h := tab.sectHits[n]
	if h.sect >= len(tab.sects) || h.line >= len(tab.sects[h.sect].lines) {
		return
	}
	t := tab.t
	var items []menuItem
	for _, a := range tab.sects[h.sect].lines[h.line].Actions {
		items = append(items, menuItem{a.Name, func() tea.Cmd { return m.runAction(a, t) }})
	}
	m.showMenu(m.sections[h.sect].Title+": "+t.Key, x, y, items)
}
