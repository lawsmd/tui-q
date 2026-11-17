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
	idMore      = "more"
	idLeft      = "left"
	idRight     = "right"
	idLane      = "lane"
	idCard      = "card"
	idMenu      = "menu"
	idMenuItem  = "menuitem"
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

	// menu is the open context menu, and drag the card being carried.
	// Both are nil most of the time.
	menu *menu
	drag *drag
	// notice is a one-line message for the status bar. The next click
	// clears it.
	notice string

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
	return m.openURL(m.tabs[m.activeTab].t)
}

// openURL shows a ticket's web page in the default browser.
func (m *Model) openURL(t ticket.Ticket) tea.Cmd {
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
				tab.set(msg.t)
				tab.loading = false
			}
		}

	case tea.KeyPressMsg:
		return m, m.key(msg)

	case tea.MouseMsg:
		if m.ops.Locked() {
			m.mouse.Cancel()
			m.drag = nil
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
		// Esc backs out of one thing at a time.
		switch {
		case m.menu != nil:
			m.menu = nil
		case m.drag != nil:
			m.drag = nil
			m.mouse.Cancel()
		default:
			m.activeTab = -1
		}
	case "left":
		if m.activeTab < 0 {
			m.queues[m.activeQueue].scrollSideways(-1)
		}
	case "right":
		if m.activeTab < 0 {
			m.queues[m.activeQueue].scrollSideways(1)
		}
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
	if m.menu != nil {
		return m.menuGesture(ev)
	}
	switch ev.Kind {
	case hit.Click:
		m.notice = ""
		return m.click(ev)
	case hit.RightClick:
		m.menuFor(ev)
	case hit.Wheel:
		m.scroll(ev)
	case hit.DragStart:
		m.startDrag(ev)
	case hit.DragMove:
		if m.drag != nil {
			m.drag.x, m.drag.y = ev.X, ev.Y
			m.drag.over = m.queues[m.activeQueue].laneName(ev.Over)
		}
	case hit.Drop:
		m.drop(ev)
	}
	return nil
}

// menuGesture handles the mouse while the context menu is open. A click on
// an item runs it; a click anywhere but the menu closes it and does nothing
// else.
func (m *Model) menuGesture(ev hit.Event) tea.Cmd {
	kind, n := splitID(ev.ID)
	switch ev.Kind {
	case hit.Click:
		if kind == idMenu {
			return nil
		}
		items := m.menu.items
		m.menu = nil
		if kind == idMenuItem && n >= 0 && n < len(items) {
			return items[n].run()
		}
	case hit.RightClick:
		m.menu = nil
		m.menuFor(ev)
	case hit.Wheel:
		m.menu = nil
	}
	return nil
}

// menuFor opens the context menu when the gesture is on a ticket.
func (m *Model) menuFor(ev hit.Event) {
	if m.activeTab >= 0 {
		return
	}
	kind, n := splitID(ev.ID)
	q := m.queues[m.activeQueue]
	switch kind {
	case idRow, idCard, idMore:
		if n >= 0 && n < len(q.tickets) {
			m.notice = ""
			m.openMenu(q.tickets[n], ev.X, ev.Y)
		}
	}
}

func (m *Model) click(ev hit.Event) tea.Cmd {
	kind, n := splitID(ev.ID)
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
	case idRow, idCard:
		q := m.queues[m.activeQueue]
		if n >= 0 && n < len(q.tickets) {
			return m.openTicket(q.tickets[n])
		}
	case idMore:
		m.menuFor(ev)
	case idLeft:
		m.queues[m.activeQueue].scrollSideways(-1)
	case idRight:
		m.queues[m.activeQueue].scrollSideways(1)
	case idRefresh:
		return m.refresh()
	case idStatus:
		m.ops.Dismiss()
	case idOpen:
		return m.openInBrowser()
	}
	return nil
}

// scroll applies one wheel notch to whatever is under the pointer.
func (m *Model) scroll(ev hit.Event) {
	if m.activeTab >= 0 {
		m.tabs[m.activeTab].offset += ev.DY * wheelStep
		return
	}
	q := m.queues[m.activeQueue]
	if ev.DX != 0 {
		q.scrollSideways(ev.DX)
		return
	}
	if q.cfg.Layout != profile.LayoutBoard {
		q.offset += ev.DY * wheelStep
		return
	}
	// On a board the wheel moves the cards of the lane it is over.
	if name := q.laneName(ev.ID); name != "" && q.laneScroll != nil {
		q.laneScroll[strings.ToLower(name)] += ev.DY
	}
}

// startDrag picks up a card. A drag that starts anywhere else is ignored.
func (m *Model) startDrag(ev hit.Event) {
	kind, n := splitID(ev.ID)
	if kind != idCard || m.activeTab >= 0 || m.comp == nil {
		return
	}
	layer := m.comp.GetLayer(ev.ID)
	if layer == nil || n < 0 || n >= len(m.queues[m.activeQueue].tickets) {
		return
	}
	m.notice = ""
	m.drag = &drag{index: n, x: ev.X, y: ev.Y, dx: ev.X - layer.GetX(), dy: ev.Y - layer.GetY()}
}

// drop ends a drag. Moving a ticket to another lane is a write, and writes
// are not built yet, so for now the drop is only reported.
func (m *Model) drop(ev hit.Event) {
	d := m.drag
	m.drag = nil
	q := m.queues[m.activeQueue]
	if d == nil || d.index >= len(q.tickets) {
		return
	}
	t := q.tickets[d.index]
	target := q.laneName(ev.Over)
	if target == "" || strings.EqualFold(target, q.laneOf(t)) {
		return
	}
	m.notice = fmt.Sprintf("Dropped %s on %s. Moving a ticket is not built yet.", t.Key, target)
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
