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

	"github.com/lawsmd/tui-q/ext"
	"github.com/lawsmd/tui-q/internal/browser"
	"github.com/lawsmd/tui-q/internal/hit"
	"github.com/lawsmd/tui-q/internal/ops"
	"github.com/lawsmd/tui-q/internal/prefs"
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
	idQueueTab   = "queue"
	idTicketTab  = "tab"
	idTabClose   = "tabx"
	idRow        = "row"
	idRefresh    = "refresh"
	idStatus     = "status"
	idOpen       = "open"
	idCopyLink   = "copylink"
	idMore       = "more"
	idEdge       = "edge"
	idLeft       = "left"
	idRight      = "right"
	idLane       = "lane"
	idCard       = "card"
	idMenu       = "menu"
	idMenuItem   = "menuitem"
	idMove       = "move"
	idAssign     = "assign"
	idField      = "field"
	idForm       = "form"
	idFormCancel = "formcancel"
	idGear       = "gear"
	idAction     = "action"
)

// hitEvent is one mouse gesture.
type hitEvent = hit.Event

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

// ticketLoadedMsg carries one ticket loaded in full and the moves open to
// it.
type ticketLoadedMsg struct {
	t   ticket.Ticket
	trs []ticket.Transition
}

// meMsg carries the user the credentials belong to, and labelsMsg the
// platform's names for the profile's fields.
type (
	meMsg     struct{ me ticket.User }
	labelsMsg struct{ labels map[string]string }
)

// browserOpenedMsg reports that the browser was started.
type browserOpenedMsg struct{}

// Model is the whole application state.
type Model struct {
	prof   *profile.Profile
	prov   provider.Provider
	styles Styles
	// prefs are the settings changed from inside the interface.
	prefs *prefs.Store
	// actions are the instance's own actions on a ticket.
	actions []ext.Action

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

	// me is the user the credentials belong to, once it is known.
	me ticket.User
	// labels are the platform's names for the profile's fields.
	labels map[string]string

	// menu is the open menu, form the open form, and drag the card being
	// carried. All are nil most of the time.
	menu *menu
	form *form
	drag *drag
	// resize is the column edge being dragged, or nil.
	resize *resize
	// undo puts the screen back when the running write fails.
	undo func()
	// notice is a one-line message for the status bar. The next click
	// clears it.
	notice string

	// comp is the last frame drawn. Hit tests run against it, since it is
	// what the user was looking at when they clicked.
	comp *lipgloss.Compositor
}

// Options are the parts of the model that an instance or a machine
// supplies. The zero value is a model with no saved settings.
type Options struct {
	// Prefs holds the settings changed from inside the interface. When it
	// is nil they last until the program exits.
	Prefs *prefs.Store
	// Actions are the instance's own actions on a ticket.
	Actions []ext.Action
}

// New builds the model for a profile and its provider.
func New(prof *profile.Profile, prov provider.Provider, opts Options) *Model {
	if opts.Prefs == nil {
		opts.Prefs = prefs.Open("")
	}
	m := &Model{
		prof:      prof,
		prov:      prov,
		prefs:     opts.Prefs,
		actions:   opts.Actions,
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

// Init loads every queue, the user's own account, and the platform's
// names for the profile's fields.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.start("Loading the account", false, func(ctx context.Context) (tea.Msg, error) {
			me, err := m.prov.Me(ctx)
			if err != nil {
				return nil, fmt.Errorf("account: %w", err)
			}
			return meMsg{me: me}, nil
		}),
		m.start("Loading field names", false, func(ctx context.Context) (tea.Msg, error) {
			labels, err := m.prov.FieldLabels(ctx)
			if err != nil {
				return nil, fmt.Errorf("field names: %w", err)
			}
			return labelsMsg{labels: labels}, nil
		}),
	}
	return tea.Batch(append(cmds, m.reloadQueues())...)
}

// reloadQueues loads every queue again.
func (m *Model) reloadQueues() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.queues))
	for i := range m.queues {
		cmds = append(cmds, m.loadQueue(i))
	}
	return tea.Batch(cmds...)
}

// label is the heading for a field: the profile's label when it gives
// one, then the platform's name for the field, then the field name.
func (m *Model) label(name string) string {
	if f, ok := m.prof.Fields[name]; ok && f.Label != "" {
		return f.Label
	}
	if l := m.labels[name]; l != "" {
		return l
	}
	return m.prof.Label(name)
}

// start runs a provider call through the operation runner and keeps the
// status bar spinner ticking while it runs.
func (m *Model) start(label string, write bool, fn ops.Func) tea.Cmd {
	return m.spinWith(m.ops.Start(label, write, fn))
}

// spinWith adds the spinner's tick to the command of a new operation.
func (m *Model) spinWith(cmd tea.Cmd) tea.Cmd {
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
		// A ticket with no moves open is still worth showing.
		trs, _ := m.prov.Transitions(ctx, id)
		return ticketLoadedMsg{t: t, trs: trs}, nil
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
		m.sizeForm()

	case spinner.TickMsg:
		if !m.ops.Busy() {
			m.spinning = false
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case ops.DoneMsg:
		if entry := m.ops.Finish(msg); entry.Write {
			// A write that failed, or that stopped to ask for values,
			// changed nothing: put back what was shown ahead of it.
			_, asked := msg.Msg.(needFieldsMsg)
			if m.undo != nil && (msg.Err != nil || asked) {
				m.undo()
			}
			m.undo = nil
		}
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
				tab.transitions = msg.trs
				tab.loading = false
			}
		}

	case meMsg:
		m.me = msg.me

	case labelsMsg:
		m.labels = msg.labels
		for _, tab := range m.tabs {
			tab.main = nil
		}

	case transitionsMsg:
		m.moveMenu(msg.t, msg.trs, msg.x, msg.y)

	case usersMsg:
		m.userMenu(msg)

	case needFieldsMsg:
		return m, m.askFields(msg.t, msg.tr)

	case changedMsg:
		if msg.t.ID != "" {
			m.replaceTicket(msg.t)
			for _, tab := range m.tabs {
				if tab.t.ID == msg.t.ID {
					tab.transitions = msg.trs
				}
			}
		}
		m.notice = msg.note
		// The change may have moved the ticket into or out of any queue.
		return m, m.reloadQueues()

	case tea.KeyPressMsg:
		if m.form != nil {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.form = nil
				return m, nil
			}
			return m.formUpdate(msg)
		}
		return m, m.key(msg)

	case tea.MouseMsg:
		if m.ops.Locked() {
			m.mouse.Cancel()
			m.drag = nil
			m.cancelResize()
			return m, nil
		}
		var cmds []tea.Cmd
		for _, ev := range m.mouse.Handle(msg, m.at) {
			if m.form != nil {
				// The form takes the keyboard; the mouse can only cancel it.
				if ev.Kind == hit.Click && ev.ID == idFormCancel {
					m.form = nil
				}
				continue
			}
			cmds = append(cmds, m.gesture(ev))
		}
		return m, tea.Batch(cmds...)

	default:
		// Anything else is a message one of the form's fields sent itself.
		if m.form != nil {
			return m.formUpdate(msg)
		}
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
		case m.resize != nil:
			m.cancelResize()
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
		if m.resize != nil {
			m.resizeTo(ev.X)
		}
		if m.drag != nil {
			m.drag.x, m.drag.y = ev.X, ev.Y
			m.drag.over = m.queues[m.activeQueue].laneName(ev.Over)
		}
	case hit.Drop:
		if m.resize != nil {
			return m.endResize()
		}
		return m.drop(ev)
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
		if kind == idMenuItem && n >= 0 && n < len(items) && items[n].run == nil {
			// A heading inside the menu does nothing.
			return nil
		}
		m.menu = nil
		if kind == idMenuItem && n >= 0 && n < len(items) {
			return items[n].run()
		}
	case hit.RightClick:
		m.menu = nil
		m.menuFor(ev)
	case hit.Wheel:
		m.menu.offset += ev.DY
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
	case idGear:
		m.viewMenu(ev.X, ev.Y)
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
	case idCopyLink:
		if m.activeTab >= 0 {
			t := m.tabs[m.activeTab].t
			return m.copy(t.URL, "link to "+t.Key)
		}
	case idMove, idAssign, idField, idAction:
		if m.activeTab < 0 {
			return nil
		}
		tab := m.tabs[m.activeTab]
		switch {
		case kind == idAction:
			return m.runAction(n, tab.t)
		case kind == idMove:
			m.moveMenu(tab.t, tab.transitions, ev.X, ev.Y)
		case kind == idAssign:
			m.assignMenu(tab.t, ev.X, ev.Y)
		case n >= 0 && n < len(tab.fields):
			return m.editField(tab, tab.fields[n], ev.X, ev.Y)
		}
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

// startDrag picks up a card or the edge of a table column. A drag that
// starts anywhere else is ignored.
func (m *Model) startDrag(ev hit.Event) {
	kind, n := splitID(ev.ID)
	if kind == idEdge && m.activeTab < 0 {
		m.startResize(ev.X, n)
		return
	}
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

// View draws the frame and keeps it for hit testing.
func (m *Model) View() tea.View {
	m.comp = m.compose()
	v := tea.NewView(m.comp.Render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "tui-q"
	return v
}
