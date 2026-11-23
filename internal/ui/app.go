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
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lawsmd/tui-q/ext"
	"github.com/lawsmd/tui-q/internal/attach"
	"github.com/lawsmd/tui-q/internal/browser"
	"github.com/lawsmd/tui-q/internal/drafts"
	"github.com/lawsmd/tui-q/internal/editor"
	"github.com/lawsmd/tui-q/internal/hit"
	"github.com/lawsmd/tui-q/internal/ops"
	"github.com/lawsmd/tui-q/internal/people"
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
	// idPicker is the tag box; the others are a tag in it, a search
	// result, and its buttons.
	idPicker     = "picker"
	idPickTag    = "picktag"
	idPickItem   = "pickitem"
	idPickSave   = "picksave"
	idPickCancel = "pickcancel"
	idGear       = "gear"
	idAction     = "action"
	idJump       = "jump"
	idPagePrev   = "pageprev"
	idPageNext   = "pagenext"
	// idFile is the line of an attachment in a ticket.
	idFile = "file"
	// idCommentOrder is the control that turns the order of comments round.
	idCommentOrder = "commentorder"
	// idComment is the line of the comment box, and the two after it are
	// the buttons on that line.
	idComment        = "comment"
	idCommentSend    = "commentsend"
	idCommentDiscard = "commentdiscard"
	// idCommentKind is one of the kinds of comment on the line of the box.
	idCommentKind = "commentkind"
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

// bucketLoadedMsg carries one page of a bucket that has its own query.
type bucketLoadedMsg struct {
	queue, bucket, page int
	result              provider.Page
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
	// only is the ID of the one ticket the program was started to show, or
	// empty. With it set no queue is loaded or drawn, and nothing leads
	// away from the ticket.
	only string

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
	// picker is the open tag box, or nil.
	picker *picker
	drag   *drag
	// resize is the column edge being dragged, or nil.
	resize *resize
	// tabDrag is the ticket tab being dragged, or nil, and tabSpans where
	// the last frame drew each ticket tab.
	tabDrag  *tabDrag
	tabSpans []span
	// jump is the box on the top row that takes a ticket number, key or
	// link, and jumping says it has the keyboard.
	jump    textinput.Model
	jumping bool
	// composer is the comment box of the ticket on screen, or nil. drafts
	// holds the comments started and not sent, openEditor makes the box
	// one is written in, and warm gets the editor ready ahead of the first
	// box. noEditor is why the last box did not open, or empty: no other
	// is opened until the user asks for one.
	composer *composer
	// kinds holds, per ticket, the kind of comment picked for it where
	// that is not the first the provider lists.
	kinds    map[string]string
	noEditor string
	drafts   *drafts.Store
	// people holds the people mentioned in comments sent before.
	people     *people.Store
	openEditor func(editor.Config) (editor.Box, error)
	warm       func()
	// files keeps the attachments read from tickets, and openFile hands
	// one of them to the program for its type.
	files    *attach.Store
	openFile func(path string) error
	// pics are the pictures of the ticket on screen that are drawn in its
	// view, by attachment ID, and picsOf is that ticket. openPicture makes
	// a picture ready to draw, and noPictures is why none can be, or empty.
	pics        map[string]*picture
	picsOf      string
	openPicture func(path, background string) (editor.Picture, error)
	noPictures  string
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
	// Ticket is the ID of one ticket to show by itself. When it is set the
	// interface loads no queue and shows nothing but that ticket.
	Ticket string
	// Drafts holds the comments started and not sent. When it is nil they
	// last until the program exits.
	Drafts *drafts.Store
	// People holds the people mentioned in comments, who are offered first
	// after "@". When it is nil they last until the program exits.
	People *people.Store
	// Files keeps the attachments read from tickets. When it is nil no
	// attachment can be opened.
	Files *attach.Store
	// OpenEditor makes the box a comment is written in. When it is nil, or
	// the provider cannot add comments, the interface offers no comment
	// box.
	OpenEditor func(editor.Config) (editor.Box, error)
	// OpenPicture makes a picture of a ticket ready to draw in the ticket
	// view. When it is nil, or the profile does not ask for pictures, a
	// picture is shown by its name only.
	OpenPicture func(path, background string) (editor.Picture, error)
	// WarmEditor, when set, is called once as the first ticket is shown,
	// to get the editor ready ahead of the first comment.
	WarmEditor func()
}

// New builds the model for a profile and its provider.
func New(prof *profile.Profile, prov provider.Provider, opts Options) *Model {
	if opts.Prefs == nil {
		opts.Prefs = prefs.Open("")
	}
	if opts.Drafts == nil {
		opts.Drafts = drafts.Open("")
	}
	if opts.People == nil {
		opts.People = people.Open("")
	}
	if opts.Files == nil {
		opts.Files = attach.Open("")
	}
	m := &Model{
		people:      opts.People,
		prof:        prof,
		prov:        prov,
		prefs:       opts.Prefs,
		actions:     opts.Actions,
		drafts:      opts.Drafts,
		files:       opts.Files,
		openFile:    browser.OpenFile,
		openEditor:  opts.OpenEditor,
		openPicture: opts.OpenPicture,
		warm:        opts.WarmEditor,
		styles:      DefaultStyles(),
		activeTab:   -1,
		only:        opts.Ticket,
		ops:         ops.New(),
		spin:        spinner.New(spinner.WithSpinner(spinner.MiniDot)),
	}
	m.jump = newJump(m.styles)
	for i := range prof.Queues {
		m.queues = append(m.queues, &queue{cfg: &prof.Queues[i]})
	}
	return m
}

// Init loads every queue, the user's own account, and the platform's
// names for the profile's fields. Started on one ticket, it loads that
// ticket in place of the queues.
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
	if m.only != "" {
		return tea.Batch(append(cmds, m.loadByID(m.only))...)
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

// loadQueue runs the query of a queue, and loads the page on screen of
// each bucket that has a query of its own.
func (m *Model) loadQueue(i int) tea.Cmd {
	q := m.queues[i]
	q.loading = true
	var cmds []tea.Cmd
	if !q.cfg.OwnQueries() {
		query := q.cfg.Query
		cmds = append(cmds, m.start("Loading "+q.cfg.Name, false, func(ctx context.Context) (tea.Msg, error) {
			tickets, err := m.prov.Search(ctx, query)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", q.cfg.Name, err)
			}
			return queueLoadedMsg{index: i, tickets: tickets}, nil
		}))
	}
	for j, b := range q.cfg.Buckets {
		if b.Query != "" {
			cmds = append(cmds, m.loadBucket(i, j, q.page(j).page))
		}
	}
	return tea.Batch(cmds...)
}

// loadBucket loads one page of a bucket that has its own query. A page
// past the first that comes back empty or is refused, as it may be after
// a change to the tickets, is answered with the first page.
func (m *Model) loadBucket(i, j, page int) tea.Cmd {
	q := m.queues[i]
	b := q.cfg.Buckets[j]
	st := q.page(j)
	if page < 0 || page >= len(st.cursors) {
		page = 0
	}
	cursor := ""
	label := "Loading " + b.Name
	if page > 0 {
		cursor = st.cursors[page]
		label = fmt.Sprintf("%s, page %d", label, page+1)
	}
	return m.start(label, false, func(ctx context.Context) (tea.Msg, error) {
		result, err := m.prov.SearchPage(ctx, b.Query, cursor, b.PageSize)
		if page > 0 && (err != nil || len(result.Tickets) == 0) {
			page = 0
			result, err = m.prov.SearchPage(ctx, b.Query, "", b.PageSize)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", b.Name, err)
		}
		return bucketLoadedMsg{queue: i, bucket: j, page: page, result: result}, nil
	})
}

// turnPage shows the page before or after the one a bucket is on. A
// bucket with its own query loads it; any other has its tickets already.
func (m *Model) turnPage(j, by int) tea.Cmd {
	q := m.queues[m.activeQueue]
	if j < 0 || j >= len(q.cfg.Buckets) {
		return nil
	}
	st := q.page(j)
	if q.cfg.Buckets[j].Query == "" {
		// The next draw keeps the page inside the bucket.
		st.page = max(st.page+by, 0)
		return nil
	}
	if (by > 0 && !st.more) || (by < 0 && st.page == 0) {
		return nil
	}
	return m.loadBucket(m.activeQueue, j, st.page+by)
}

// openTicket shows a ticket in a tab, adding the tab when the ticket is not
// already open, and loads the full ticket.
func (m *Model) openTicket(t ticket.Ticket) tea.Cmd {
	m.warmEditor()
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
	m.tabDrag = nil
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

// canOpen reports whether an attachment of a ticket can be opened: the
// provider reads attachments, and this one is a picture. Any other kind of
// file is left to the platform's own page, since opening it here would
// hand a file from a ticket to whatever program its type starts.
func (m *Model) canOpen(t ticket.Ticket, id string) bool {
	a, ok := t.Attachment(id)
	_, reads := m.prov.(provider.AttachmentReader)
	return ok && reads && a.Image()
}

// openAttachment shows a picture attached to a ticket in the program the
// machine opens pictures with. The file is read from the platform the
// first time and from the store after that.
func (m *Model) openAttachment(t ticket.Ticket, id string) tea.Cmd {
	if !m.canOpen(t, id) {
		return nil
	}
	a, _ := t.Attachment(id)
	return m.start("Opening "+a.Name, false, func(ctx context.Context) (tea.Msg, error) {
		path, err := m.attachmentFile(ctx, a)
		if err != nil {
			return nil, err
		}
		if err := m.openFile(path); err != nil {
			return nil, err
		}
		return browserOpenedMsg{}, nil
	})
}

// Update handles one message, then brings the comment box and the pictures
// in line with what the message changed.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := m.update(msg)
	return m, tea.Batch(cmd, m.syncComposer(), m.syncPictures())
}

func (m *Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.sizeForm()
		return m, tea.Batch(m.composerPass(msg), m.picturePass(msg))

	case composerOpenedMsg:
		return m, m.composerOpened(msg)

	case pictureOpenedMsg:
		return m, m.pictureOpened(msg)

	case editor.Changed:
		return m, m.composerChanged()

	case editor.Failed:
		if c := m.composer; c != nil && c.box != nil {
			m.notice = "The comment box could not " + msg.Op + ": " + msg.Err.Error()
			return m, c.box.Changes()
		}

	case draftSaveMsg:
		if c := m.composer; c != nil && c.saves == msg.n {
			m.saveDraft()
		}

	case commentedMsg:
		m.commented(msg)
		return m.update(msg.changed)

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
			return m.update(msg.Msg)
		}

	case queueLoadedMsg:
		q := m.queues[msg.index]
		q.setSource(fromQueue, msg.tickets)
		q.loading = false
		q.loadedAt = time.Now()

	case bucketLoadedMsg:
		q := m.queues[msg.queue]
		st := q.page(msg.bucket)
		st.page, st.total, st.more, st.loaded = msg.page, msg.result.Total, msg.result.Next != "", true
		// Keep the cursors up to this page, and add the one for the next.
		st.cursors = append(st.cursors, "")[:max(min(len(st.cursors), msg.page+1), 1)]
		if st.more {
			st.cursors = append(st.cursors, msg.result.Next)
		}
		q.setSource(msg.bucket, msg.result.Tickets)
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

	case jumpedMsg:
		m.showLoaded(msg)

	case transitionsMsg:
		m.moveMenu(msg.t, msg.trs, msg.x, msg.y)

	case usersMsg:
		m.userMenu(msg)

	case pickSearchMsg:
		return m, m.pickSearch(msg)

	case pickFoundMsg:
		m.pickFound(msg)

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
		if m.only != "" {
			return m, nil
		}
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
		if m.picker != nil {
			return m, m.pickKey(msg)
		}
		if m.jumping {
			return m, m.jumpKey(msg)
		}
		if cmd, ok := m.composerKey(msg); ok {
			return m, cmd
		}
		return m, m.key(msg)

	case tea.MouseMsg:
		if m.ops.Locked() {
			m.mouse.Cancel()
			m.drag = nil
			m.tabDrag = nil
			m.cancelResize()
			return m, nil
		}
		if cmd, ok := m.composerMouse(msg); ok {
			return m, cmd
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
			if m.picker != nil {
				cmds = append(cmds, m.pickGesture(ev))
				continue
			}
			cmds = append(cmds, m.gesture(ev))
		}
		return m, tea.Batch(cmds...)

	default:
		// Anything else is a message one of the form's fields sent itself,
		// pasted text or a cursor blink for the jump box, or a message of
		// the comment box or of a picture, which get every one of them.
		cmd := tea.Batch(m.composerPass(msg), m.picturePass(msg))
		if m.form != nil {
			_, formCmd := m.formUpdate(msg)
			return m, tea.Batch(formCmd, cmd)
		}
		if m.picker != nil {
			var pickCmd tea.Cmd
			m.picker.in, pickCmd = m.picker.in.Update(msg)
			return m, tea.Batch(pickCmd, cmd)
		}
		if m.jumping {
			var jumpCmd tea.Cmd
			m.jump, jumpCmd = m.jump.Update(msg)
			return m, tea.Batch(jumpCmd, cmd)
		}
		return m, cmd
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
	if m.only != "" {
		// Nothing leads away from the one ticket.
		switch k {
		case "/", "tab", "shift+tab", "ctrl+w":
			return nil
		}
	}
	switch k {
	case "q":
		return tea.Quit
	case "r":
		return m.refresh()
	case "/":
		return m.focusJump()
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
		case m.tabDrag != nil:
			m.cancelTabDrag()
			m.mouse.Cancel()
		default:
			if m.only == "" {
				m.activeTab = -1
			}
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
	case "c":
		return m.writeComment()
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
	if m.only != "" {
		// The one ticket has not loaded yet: try it again.
		return m.loadByID(m.only)
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
		// A click anywhere but the jump box gives the keyboard back.
		if kind, _ := splitID(ev.ID); m.jumping && kind != idJump {
			m.blurJump()
		}
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
		if m.tabDrag != nil {
			m.dragTabTo(ev.X)
		}
		if m.drag != nil {
			m.drag.x, m.drag.y = ev.X, ev.Y
			m.drag.over = m.queues[m.activeQueue].laneName(ev.Over)
		}
	case hit.Drop:
		if m.resize != nil {
			return m.endResize()
		}
		if m.tabDrag != nil {
			m.tabDrag = nil
			return nil
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
	case idJump:
		return m.focusJump()
	case idPagePrev:
		return m.turnPage(n, -1)
	case idPageNext:
		return m.turnPage(n, 1)
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
	case idComment:
		return m.writeComment()
	case idCommentSend:
		return m.sendComment()
	case idCommentDiscard:
		m.discardComment()
	case idCommentKind:
		m.pickCommentKind(n)
	case idFile:
		if m.activeTab >= 0 {
			if tab := m.tabs[m.activeTab]; n >= 0 && n < len(tab.files) {
				return m.openAttachment(tab.t, tab.files[n])
			}
		}
	case idCommentOrder:
		err := m.prefs.SetCommentsOldestFirst(!m.prefs.CommentsOldestFirst)
		for _, tab := range m.tabs {
			tab.main = nil
		}
		if err != nil {
			m.notice = "The setting is on for now, but saving it failed: " + err.Error()
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
		// Beside the main column the sidebar scrolls on its own.
		tab := m.tabs[m.activeTab]
		if tab.sideX > 0 && ev.X >= tab.sideX {
			tab.sideOffset += ev.DY * wheelStep
		} else {
			tab.offset += ev.DY * wheelStep
		}
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

// startDrag picks up a card, a ticket tab or the edge of a table column. A
// drag that starts anywhere else is ignored.
func (m *Model) startDrag(ev hit.Event) {
	kind, n := splitID(ev.ID)
	if kind == idTicketTab {
		m.startTabDrag(n)
		return
	}
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
	// The comment box draws itself again when the window gets the keyboard
	// back, which is when a terminal most often loses its picture.
	v.ReportFocus = true
	v.WindowTitle = "tui-q"
	if m.only != "" {
		v.WindowTitle = m.onlyKey()
	}
	return v
}
