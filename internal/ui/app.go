// Package ui is the tui-q interface: two rows of tabs, a content area, and
// a status bar, all drawn as Lip Gloss layers so the mouse can address them.
package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lawsmd/tui-q/internal/browser"
	"github.com/lawsmd/tui-q/internal/hit"
	"github.com/lawsmd/tui-q/internal/ops"
	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// Smallest pane the interface draws in.
const (
	minWidth  = 40
	minHeight = 8
)

// wheelStep is how many rows one wheel notch scrolls.
const wheelStep = 3

// Layer IDs. An ID with a colon carries an index after it.
const (
	idQueueTab  = "queue"
	idTicketTab = "tab"
	idTabClose  = "tabx"
	idRow       = "row"
	idRefresh   = "refresh"
	idStatus    = "status"
	idOpen      = "open"
)

func layerID(kind string, n int) string { return kind + ":" + strconv.Itoa(n) }

func splitID(id string) (kind string, n int) {
	kind, rest, ok := strings.Cut(id, ":")
	if !ok {
		return id, -1
	}
	n, err := strconv.Atoi(rest)
	if err != nil {
		return kind, -1
	}
	return kind, n
}

// queueLoadedMsg carries the result of a queue search.
type queueLoadedMsg struct {
	index   int
	tickets []ticket.Ticket
}

// ticketLoadedMsg carries one ticket loaded in full.
type ticketLoadedMsg struct{ t ticket.Ticket }

// browserOpenedMsg reports that the browser was started.
type browserOpenedMsg struct{}

// Model is the whole application state.
type Model struct {
	prof   *profile.Profile
	prov   provider.Provider
	styles Styles

	w, h int

	queues      []*queue
	activeQueue int
	tabs        []*ticketTab
	// activeTab is the open ticket being shown, or -1 when the queue is.
	activeTab int

	ops      *ops.Runner
	mouse    hit.Tracker
	spin     spinner.Model
	spinning bool

	// comp is the last frame drawn. Hit tests run against it, since it is
	// what the user was looking at when they clicked.
	comp *lipgloss.Compositor
}

// New builds the model for a profile and its provider.
func New(prof *profile.Profile, prov provider.Provider) *Model {
	m := &Model{
		prof:      prof,
		prov:      prov,
		styles:    DefaultStyles(),
		activeTab: -1,
		ops:       ops.New(),
		spin:      spinner.New(spinner.WithSpinner(spinner.MiniDot)),
	}
	for i := range prof.Queues {
		m.queues = append(m.queues, &queue{cfg: &prof.Queues[i]})
	}
	return m
}

// Init loads every queue.
func (m *Model) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.queues))
	for i := range m.queues {
		cmds = append(cmds, m.loadQueue(i))
	}
	return tea.Batch(cmds...)
}

// start runs a provider call through the operation runner and keeps the
// status bar spinner ticking while it runs.
func (m *Model) start(label string, write bool, fn ops.Func) tea.Cmd {
	cmd := m.ops.Start(label, write, fn)
	if m.spinning {
		return cmd
	}
	m.spinning = true
	return tea.Batch(cmd, m.spin.Tick)
}

func (m *Model) loadQueue(i int) tea.Cmd {
	q := m.queues[i]
	q.loading = true
	query := q.cfg.Query
	return m.start("Loading "+q.cfg.Name, false, func(ctx context.Context) (tea.Msg, error) {
		tickets, err := m.prov.Search(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", q.cfg.Name, err)
		}
		return queueLoadedMsg{index: i, tickets: tickets}, nil
	})
}

// openTicket shows a ticket in a tab, adding the tab when the ticket is not
// already open, and loads the full ticket.
func (m *Model) openTicket(t ticket.Ticket) tea.Cmd {
	for i, tab := range m.tabs {
		if tab.t.ID == t.ID {
			m.activeTab = i
			return nil
		}
	}
	m.tabs = append(m.tabs, &ticketTab{t: t, loading: true})
	m.activeTab = len(m.tabs) - 1
	return m.loadTicket(t.ID, t.Key)
}

func (m *Model) loadTicket(id, key string) tea.Cmd {
	return m.start("Loading "+key, false, func(ctx context.Context) (tea.Msg, error) {
		t, err := m.prov.Get(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		return ticketLoadedMsg{t: t}, nil
	})
}

func (m *Model) closeTab(i int) {
	if i < 0 || i >= len(m.tabs) {
		return
	}
	m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
	switch {
	case m.activeTab == i:
		// Show the neighbour on the left, or the queue when none is left.
		m.activeTab = min(i, len(m.tabs)) - 1
		if m.activeTab < 0 && len(m.tabs) > 0 {
			m.activeTab = 0
		}
	case m.activeTab > i:
		m.activeTab--
	}
}

func (m *Model) openInBrowser() tea.Cmd {
	if m.activeTab < 0 {
		return nil
	}
	t := m.tabs[m.activeTab].t
	if t.URL == "" {
		return nil
	}
	return m.start("Opening "+t.Key+" in the browser", false, func(context.Context) (tea.Msg, error) {
		if err := browser.Open(t.URL); err != nil {
			return nil, err
		}
		return browserOpenedMsg{}, nil
	})
}

// Update handles one message.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height

	case spinner.TickMsg:
		if !m.ops.Busy() {
			m.spinning = false
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case ops.DoneMsg:
		m.ops.Finish(msg)
		if msg.Err != nil {
			m.clearLoading()
			return m, nil
		}
		if msg.Msg != nil {
			return m.Update(msg.Msg)
		}

	case queueLoadedMsg:
		q := m.queues[msg.index]
		q.tickets = msg.tickets
		q.loading = false
		q.loadedAt = time.Now()

	case ticketLoadedMsg:
		for _, tab := range m.tabs {
			if tab.t.ID == msg.t.ID {
				tab.t = msg.t
				tab.loading = false
			}
		}

	case tea.KeyPressMsg:
		return m, m.key(msg)

	case tea.MouseMsg:
		if m.ops.Locked() {
			m.mouse.Cancel()
			return m, nil
		}
		var cmds []tea.Cmd
		for _, ev := range m.mouse.Handle(msg, m.at) {
			cmds = append(cmds, m.gesture(ev))
		}
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// clearLoading drops the loading marks after a failed read, so a failure
// does not leave a view saying "Loading" for ever.
func (m *Model) clearLoading() {
	if m.ops.Busy() {
		return
	}
	for _, q := range m.queues {
		q.loading = false
	}
	for _, tab := range m.tabs {
		tab.loading = false
	}
}

func (m *Model) key(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	if k == "ctrl+c" {
		return tea.Quit
	}
	if m.ops.Locked() {
		return nil
	}
	switch k {
	case "q":
		return tea.Quit
	case "r":
		return m.refresh()
	case "tab":
		m.activeQueue = (m.activeQueue + 1) % len(m.queues)
		m.activeTab = -1
	case "shift+tab":
		m.activeQueue = (m.activeQueue + len(m.queues) - 1) % len(m.queues)
		m.activeTab = -1
	case "esc":
		m.activeTab = -1
	case "ctrl+w":
		m.closeTab(m.activeTab)
	case "o":
		return m.openInBrowser()
	}
	return nil
}

// refresh reloads what is on screen: the open ticket, or the active queue.
func (m *Model) refresh() tea.Cmd {
	if m.activeTab >= 0 {
		tab := m.tabs[m.activeTab]
		tab.loading = true
		return m.loadTicket(tab.t.ID, tab.t.Key)
	}
	return m.loadQueue(m.activeQueue)
}

// at reports the layer under a cell in the last frame drawn.
func (m *Model) at(x, y int) string {
	if m.comp == nil {
		m.comp = m.compose()
	}
	return m.comp.Hit(x, y).ID()
}

func (m *Model) gesture(ev hit.Event) tea.Cmd {
	switch ev.Kind {
	case hit.Click:
		return m.click(ev.ID)
	case hit.Wheel:
		m.scroll(ev.DY * wheelStep)
	}
	return nil
}

func (m *Model) click(id string) tea.Cmd {
	kind, n := splitID(id)
	switch kind {
	case idQueueTab:
		if n >= 0 && n < len(m.queues) {
			m.activeQueue = n
			m.activeTab = -1
		}
	case idTicketTab:
		if n >= 0 && n < len(m.tabs) {
			m.activeTab = n
		}
	case idTabClose:
		m.closeTab(n)
	case idRow:
		q := m.queues[m.activeQueue]
		if n >= 0 && n < len(q.tickets) {
			return m.openTicket(q.tickets[n])
		}
	case idRefresh:
		return m.refresh()
	case idStatus:
		m.ops.Dismiss()
	case idOpen:
		return m.openInBrowser()
	}
	return nil
}

func (m *Model) scroll(rows int) {
	if m.activeTab >= 0 {
		m.tabs[m.activeTab].offset += rows
		return
	}
	m.queues[m.activeQueue].offset += rows
}

// View draws the frame and keeps it for hit testing.
func (m *Model) View() tea.View {
	m.comp = m.compose()
	v := tea.NewView(m.comp.Render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "tui-q"
	return v
}
