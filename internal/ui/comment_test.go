package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/internal/drafts"
	"github.com/lawsmd/tui-q/internal/editor"
	"github.com/lawsmd/tui-q/internal/people"
	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/provider/fake"
	"github.com/lawsmd/tui-q/ticket"
)

// fakeBox stands in for the editor. It records what the model did with it.
type fakeBox struct {
	cfg     editor.Config
	text    string
	rect    area
	started bool
	covered bool
	focused bool
	closed  bool
	got     []tea.Msg
}

func (b *fakeBox) Init() tea.Cmd { b.started = true; return nil }
func (b *fakeBox) Update(msg tea.Msg) tea.Cmd {
	b.got = append(b.got, msg)
	return nil
}
func (b *fakeBox) SetRect(x, y, w, h int) tea.Cmd { b.rect = area{x, y, w, h}; return nil }
func (b *fakeBox) Cover(on bool) tea.Cmd          { b.covered = on; return nil }
func (b *fakeBox) Contains(x, y int) bool {
	return x >= b.rect.x && x < b.rect.x+b.rect.w && y >= b.rect.y && y < b.rect.y+b.rect.h
}
func (b *fakeBox) Focus(on bool)                          { b.focused = on }
func (b *fakeBox) Text() string                           { return b.text }
func (b *fakeBox) Export(context.Context) (string, error) { return b.text, nil }
func (b *fakeBox) Changes() tea.Cmd                       { return nil }
func (b *fakeBox) Close()                                 { b.closed = true }

// last returns the newest message the box was given.
func (b *fakeBox) last() tea.Msg {
	if len(b.got) == 0 {
		return nil
	}
	return b.got[len(b.got)-1]
}

// editors hands out fake boxes and keeps them for the test to look at.
type editors struct {
	boxes  []*fakeBox
	tries  int
	warmed int
	err    error
}

func (e *editors) open(cfg editor.Config) (editor.Box, error) {
	e.tries++
	if e.err != nil {
		return nil, e.err
	}
	b := &fakeBox{cfg: cfg, text: cfg.Markdown}
	e.boxes = append(e.boxes, b)
	return b, nil
}

func (e *editors) box(t *testing.T) *fakeBox {
	t.Helper()
	if len(e.boxes) == 0 {
		t.Fatal("no comment box was opened")
	}
	return e.boxes[len(e.boxes)-1]
}

// newCommenting builds a model that can open a comment box.
func newCommenting(t *testing.T, w, h int, opts Options) (*Model, *editors) {
	t.Helper()
	prof, err := profile.Parse(testProfile)
	if err != nil {
		t.Fatal(err)
	}
	eds := &editors{}
	opts.OpenEditor = eds.open
	opts.WarmEditor = func() { eds.warmed++ }
	m := New(prof, fake.New(0), opts)
	send(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	drain(t, m, m.Init())
	return m, eds
}

// openBox opens the first ticket of the queue, which opens its comment box.
func openBox(t *testing.T, m *Model, eds *editors) *fakeBox {
	t.Helper()
	clickLayer(t, m, layerID(idRow, 0))
	return eds.box(t)
}

// inside is where the model means the editor of the open box to be.
func inside(m *Model) area {
	return m.composerPlace().rect
}

func key(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code, Text: string(code)} }

var (
	esc   = tea.KeyPressMsg{Code: tea.KeyEscape}
	ctrlS = tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
)

func TestCommentBoxNeedsAnEditorAndACommenter(t *testing.T) {
	m := newModel(t, 110, 30)
	clickLayer(t, m, layerID(idRow, 0))
	m.View()
	if m.comp.GetLayer(idComment) != nil {
		t.Error("a model with no editor shows the comment box")
	}

	prof, err := profile.Parse(testProfile)
	if err != nil {
		t.Fatal(err)
	}
	eds := &editors{}
	// Wrapped in the interface, the fake provider can no longer comment.
	prov := struct{ provider.Provider }{fake.New(0)}
	m = New(prof, prov, Options{OpenEditor: eds.open, WarmEditor: func() { eds.warmed++ }})
	send(t, m, tea.WindowSizeMsg{Width: 110, Height: 30})
	drain(t, m, m.Init())
	clickLayer(t, m, layerID(idRow, 0))
	m.View()
	if m.comp.GetLayer(idComment) != nil {
		t.Error("a provider that cannot comment is shown the comment box")
	}
	send(t, m, key('c'))
	if m.composer != nil || eds.warmed != 0 || eds.tries != 0 {
		t.Errorf("composer=%v warmed=%d tries=%d, want none", m.composer, eds.warmed, eds.tries)
	}
}

func TestEveryTicketViewHasACommentBox(t *testing.T) {
	m, eds := newCommenting(t, 110, 40, Options{})
	if eds.warmed != 0 || eds.tries != 0 {
		t.Fatal("the editor was started before any ticket was shown")
	}
	b := openBox(t, m, eds)
	key := m.tabs[0].t.Key
	if eds.warmed != 1 || eds.tries != 1 {
		t.Errorf("warmed %d times and opened %d, want once each", eds.warmed, eds.tries)
	}
	if b.cfg.Format != provider.CommentMarkdown || b.cfg.Title != "comment on "+key || b.cfg.Meta["ticket"] != key ||
		b.cfg.Background == "" || b.cfg.Background != m.styles.ComposerBack {
		t.Errorf("config = %+v", b.cfg)
	}
	// The keyboard stays with the ticket until the box is asked for.
	if !b.started || b.focused || b.covered {
		t.Errorf("started=%v focused=%v covered=%v, want a started box without the keyboard", b.started, b.focused, b.covered)
	}
	if size, ok := b.got[0].(tea.WindowSizeMsg); !ok || size.Width != 110 || size.Height != 40 {
		t.Errorf("the first message to the box was %#v, want the window's size", b.got[0])
	}
	idle := b.rect
	if idle != inside(m) || idle.h != composerIdleRows {
		t.Fatalf("idle box at %+v, want %+v", idle, inside(m))
	}
	out := screen(m)
	for _, want := range []string{composerName, "send", "discard"} {
		if !strings.Contains(out, want) {
			t.Errorf("screen lacks %q\n%s", want, out)
		}
	}
	// The box is known by its colour, with no words on how to use it.
	for _, not := range []string{"press c", "click here", "ctrl+s"} {
		if strings.Contains(out, not) {
			t.Errorf("screen says %q\n%s", not, out)
		}
	}

	// Written in, the box is taller, and it shrinks again afterwards.
	clickLayer(t, m, idComment)
	if !b.focused || b.rect != inside(m) || b.rect.h <= idle.h {
		t.Fatalf("writing: focused=%v box at %+v, idle was %+v", b.focused, b.rect, idle)
	}
	if strings.Contains(screen(m), "ctrl+s") {
		t.Errorf("the box being written in explains its keys\n%s", screen(m))
	}
	send(t, m, esc)
	if b.focused || b.rect.h != idle.h {
		t.Fatalf("after esc: focused=%v box at %+v, want the %d rows it had", b.focused, b.rect, idle.h)
	}

	// Text put in the box by another program makes it tall without the
	// keyboard, and it stays tall until it is empty again.
	b.text = "a draft from elsewhere"
	send(t, m, editor.Changed{})
	if b.focused || b.rect.h <= idle.h {
		t.Fatalf("holding text: focused=%v box at %+v, idle was %+v", b.focused, b.rect, idle)
	}
	clickLayer(t, m, idComment)
	send(t, m, esc)
	if b.rect.h <= idle.h {
		t.Fatalf("holding text after esc: box at %+v, idle was %+v", b.rect, idle)
	}
	b.text = " \n"
	send(t, m, editor.Changed{})
	if b.rect.h != idle.h {
		t.Fatalf("emptied: box at %+v, want the %d rows it had", b.rect, idle.h)
	}

	// The users offered after "@" come from the provider.
	users, err := b.cfg.Mentions(context.Background(), "al")
	if err != nil || len(users) != 1 || users[0].Name != "alex" {
		t.Errorf("mentions = %+v, %v", users, err)
	}
}

// checkBox checks the comment box against the frame on screen: the frame
// fits the pane, the editor is where the model means it to be or is told
// not to draw, and its rows hold no text.
func checkBox(t *testing.T, m *Model, b *fakeBox, w, h int, what string) boxPlace {
	t.Helper()
	lines := strings.Split(screen(m), "\n")
	if len(lines) != h {
		t.Fatalf("pane %dx%d %s: drew %d rows", w, h, what, len(lines))
	}
	p := m.composerPlace()
	if p.line.h > 0 && !strings.Contains(lines[p.line.y], "╭─ "+composerName+" ") {
		t.Fatalf("pane %dx%d %s: row %d is not the border with the box's name: %q", w, h, what, p.line.y, lines[p.line.y])
	}
	if p.rect.h == 0 {
		if !b.covered {
			t.Fatalf("pane %dx%d %s: a box with no row in view still draws, at %+v", w, h, what, b.rect)
		}
		return p
	}
	r := b.rect
	if b.covered || r != p.rect || r.x < 0 || r.x+r.w > w || r.y < 2 || r.y+r.h > h-1 {
		t.Fatalf("pane %dx%d %s: covered=%v box at %+v, want %+v", w, h, what, b.covered, r, p.rect)
	}
	for y := r.y; y < r.y+r.h; y++ {
		// The editor is everything between the two sides of the border.
		row := []rune(lines[y])
		if len(row) <= r.x+r.w || row[r.x-1] != '│' || row[r.x+r.w] != '│' {
			t.Fatalf("pane %dx%d %s: row %d of the box has no border at its sides: %q", w, h, what, y, lines[y])
		}
		if got := string(row[r.x : r.x+r.w]); strings.TrimSpace(got) != "" {
			t.Fatalf("pane %dx%d %s: row %d of the box holds %q", w, h, what, y, got)
		}
	}
	// The border is the row above the editor and the row below it.
	if p.line.h > 0 && (r.y != p.line.y+1 || r.x != p.line.x+1 || r.w != p.line.w-2) {
		t.Fatalf("pane %dx%d %s: editor at %+v is not inside the border at %+v", w, h, what, r, p.line)
	}
	if r.h == p.rows && r.y+r.h < h-1 && p.line.h > 0 {
		if row := []rune(lines[r.y+r.h]); len(row) <= r.x+r.w || row[r.x-1] != '╰' || row[r.x+r.w] != '╯' {
			t.Fatalf("pane %dx%d %s: row %d is not the border under the box: %q", w, h, what, r.y+r.h, lines[r.y+r.h])
		}
	}
	return p
}

func TestCommentBoxFitsEveryPane(t *testing.T) {
	m, eds := newCommenting(t, 110, 30, Options{})
	b := openBox(t, m, eds)
	tab := m.tabs[0]
	for w := minWidth; w <= 160; w += 11 {
		for h := minHeight; h <= 44; h++ {
			send(t, m, tea.WindowSizeMsg{Width: w, Height: h})
			// Idle, at every place the ticket can be scrolled to.
			for tab.offset = 0; ; tab.offset += 1 + h/3 {
				at := tab.offset
				send(t, m, odd{})
				// A pane with no room for the column has no box to scroll.
				if p := checkBox(t, m, b, w, h, "idle"); !p.on || tab.offset != at {
					break
				}
			}
			// Asked for, the box comes into view with a row to write in.
			tab.offset = 0
			send(t, m, key('c'))
			if p := checkBox(t, m, b, w, h, "writing"); p.line.h == 0 || p.rect.h < 1 || p.rect.h != p.rows {
				t.Fatalf("pane %dx%d: the box being written in is at %+v", w, h, p)
			}
			send(t, m, esc)
		}
	}
}

func TestCommentBoxIsUnderTheCommentsHeading(t *testing.T) {
	m, eds := newCommenting(t, 110, 60, Options{})
	b := openBox(t, m, eds)
	if len(m.tabs[0].t.Comments) == 0 {
		t.Fatal("the fixture ticket has no comments to put the box among")
	}
	lines := strings.Split(screen(m), "\n")
	row := func(text string) int {
		for y, line := range lines {
			if strings.Contains(line, text) {
				return y
			}
		}
		t.Fatalf("screen lacks %q\n%s", text, strings.Join(lines, "\n"))
		return -1
	}
	// The heading, a blank line, the box's line and its editor.
	box, comments := row(composerName), row("Comments (")
	if box != b.rect.y-1 || comments != box-2 || box < 4 {
		t.Fatalf("box line on row %d, editor at %+v, comments heading on row %d", box, b.rect, comments)
	}

	// With the oldest comment first, the box follows the comments.
	m.prefs.CommentsOldestFirst = true
	m.tabs[0].main = nil
	send(t, m, odd{})
	lines = strings.Split(screen(m), "\n")
	if box, comments := row(composerName), row("Comments ("); box <= comments+2 || box != b.rect.y-1 {
		t.Fatalf("oldest first: box line on row %d, editor at %+v, comments heading on row %d", box, b.rect, comments)
	}
	m.prefs.CommentsOldestFirst = false
	m.tabs[0].main = nil
	send(t, m, odd{})

	// The box scrolls with the ticket.
	was := b.rect
	send(t, m, tea.WindowSizeMsg{Width: 110, Height: 14})
	m.tabs[0].offset = 0
	send(t, m, odd{})
	top := b.rect
	send(t, m, tea.MouseWheelMsg{X: 5, Y: 6, Button: tea.MouseWheelDown})
	if b.rect == top && !b.covered {
		t.Fatalf("the box stayed at %+v when the ticket was scrolled (it was at %+v in the tall pane)", top, was)
	}
}

func TestKeysGoToTheCommentBoxOnlyWhileItHasTheKeyboard(t *testing.T) {
	m, eds := newCommenting(t, 110, 30, Options{})
	b := openBox(t, m, eds)
	n := len(b.got)
	send(t, m, tea.PasteMsg{Content: "not for the box"})
	if len(b.got) != n {
		t.Fatalf("a paste reached a box without the keyboard: %#v", b.got[n:])
	}

	send(t, m, key('c'))
	if !b.focused || len(b.got) != n {
		t.Fatalf("after c: focused=%v, box got %#v", b.focused, b.got[n:])
	}
	// q and ctrl+c would quit, and tab would change queue.
	for _, k := range []tea.KeyPressMsg{key('q'), key('c'), {Code: 'c', Mod: tea.ModCtrl}, {Code: tea.KeyTab}} {
		_, cmd := m.Update(k)
		if got, ok := b.last().(tea.KeyPressMsg); !ok || got.String() != k.String() {
			t.Fatalf("%s did not reach the box; it last got %#v", k.String(), b.last())
		}
		if cmd != nil {
			if _, quit := cmd().(tea.QuitMsg); quit {
				t.Fatalf("%s quit the program", k.String())
			}
		}
	}
	if m.activeTab != 0 {
		t.Fatal("a key typed in the box left the ticket")
	}
	send(t, m, tea.PasteMsg{Content: "pasted"})
	if got, ok := b.last().(tea.PasteMsg); !ok || got.Content != "pasted" {
		t.Fatalf("pasted text did not reach the box; it last got %#v", b.last())
	}

	// Esc gives the keyboard back and leaves the box where it is.
	send(t, m, esc)
	if b.focused || b.closed || m.composer == nil || m.activeTab != 0 {
		t.Fatalf("after esc: focused=%v closed=%v composer=%v tab=%d", b.focused, b.closed, m.composer, m.activeTab)
	}
	n = len(b.got)
	send(t, m, key('x'))
	if len(b.got) != n {
		t.Fatalf("a key reached the box without the keyboard: %#v", b.got[n:])
	}
}

func TestMouseInsideTheCommentBoxIsTheBoxes(t *testing.T) {
	m, eds := newCommenting(t, 110, 60, Options{})
	b := openBox(t, m, eds)
	tab := m.tabs[0]
	x, y := b.rect.x+3, b.rect.y+1

	// A click inside gives the box the keyboard, and it gets the drag.
	n := len(b.got)
	send(t, m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	send(t, m, tea.MouseMotionMsg{X: x + 4, Y: y, Button: tea.MouseLeft})
	send(t, m, tea.MouseReleaseMsg{X: x + 4, Y: 1, Button: tea.MouseLeft})
	if !b.focused || len(b.got) != n+3 {
		t.Fatalf("focused=%v, box got %d of the 3 mouse messages", b.focused, len(b.got)-n)
	}
	// Once the button is up the pointer's moves are no longer the box's.
	send(t, m, tea.MouseMotionMsg{X: x, Y: y})
	if len(b.got) != n+3 {
		t.Fatal("a move with no button down reached the box")
	}

	// The wheel is the box's while it is written in.
	send(t, m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	send(t, m, tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
	offset := tab.offset
	y = b.rect.y + 1
	send(t, m, tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelDown})
	if _, ok := b.last().(tea.MouseWheelMsg); !ok || tab.offset != offset {
		t.Fatalf("wheel in the box: box last got %#v, ticket offset %d to %d", b.last(), offset, tab.offset)
	}
	send(t, m, tea.MouseWheelMsg{X: x, Y: 8, Button: tea.MouseWheelDown})
	m.View()
	if tab.offset == offset {
		t.Fatal("the wheel above the box did not scroll the ticket")
	}

	// A click on the ticket takes the keyboard away.
	send(t, m, tea.MouseClickMsg{X: 3, Y: 8, Button: tea.MouseLeft})
	send(t, m, tea.MouseReleaseMsg{X: 3, Y: 8, Button: tea.MouseLeft})
	if b.focused {
		t.Fatal("a click outside the box left it the keyboard")
	}
}

// odd is a message the model knows nothing about, as the editor's own are.
type odd struct{}

func TestCommentBoxGetsUnknownMessagesAndIsCoveredByPopups(t *testing.T) {
	m, eds := newCommenting(t, 110, 30, Options{})
	b := openBox(t, m, eds)
	send(t, m, odd{})
	if _, ok := b.last().(odd); !ok {
		t.Fatal("a message the model does not know did not reach the box")
	}

	send(t, m, key('c'))
	clickLayer(t, m, idMove)
	if m.menu == nil || !b.covered {
		t.Fatalf("menu=%v covered=%v, want an open menu and a covered box", m.menu != nil, b.covered)
	}
	send(t, m, esc)
	if m.menu != nil || b.covered {
		t.Fatalf("menu=%v covered=%v, want both gone", m.menu != nil, b.covered)
	}

	send(t, m, key('c'))
	drain(t, m, m.askUser(m.tabs[0].t, 5, 5))
	send(t, m, tea.WindowSizeMsg{Width: 110, Height: 30})
	if m.form == nil || !b.covered {
		t.Fatalf("form=%v covered=%v, want an open form and a covered box", m.form != nil, b.covered)
	}
	n := len(b.got)
	send(t, m, odd{})
	if _, ok := b.last().(odd); !ok || len(b.got) != n+1 {
		t.Fatal("with a form open, a message the model does not know did not reach the box")
	}
	// The form has the keyboard, not the box.
	send(t, m, key('a'))
	if len(b.got) != n+1 {
		t.Fatal("a key typed in the form reached the box")
	}
	send(t, m, esc)
	if m.form != nil || b.covered {
		t.Fatalf("form=%v covered=%v, want both gone", m.form != nil, b.covered)
	}
}

func TestLeavingTheTicketKeepsTheDraft(t *testing.T) {
	dir := t.TempDir()
	m, eds := newCommenting(t, 110, 30, Options{Drafts: drafts.Open(dir)})
	b := openBox(t, m, eds)
	id := m.tabs[0].t.ID

	// A box that was never typed in leaves nothing behind.
	clickLayer(t, m, layerID(idQueueTab, 0))
	if !b.closed || m.composer != nil || m.drafts.Get(id) != "" {
		t.Fatalf("closed=%v composer=%v draft=%q", b.closed, m.composer, m.drafts.Get(id))
	}

	clickLayer(t, m, layerID(idTicketTab, 0))
	b = eds.box(t)
	if len(eds.boxes) != 2 || b.closed {
		t.Fatalf("%d boxes opened, want a second one for the ticket shown again", len(eds.boxes))
	}
	b.text = "Half a **thought**"
	send(t, m, editor.Changed{}) // writes the draft after a moment
	if got := drafts.Open(dir).Get(id); got != b.text {
		t.Fatalf("draft on disk = %q", got)
	}
	b.text = "Half a **thought**, and more"
	_, _ = m.Update(editor.Changed{}) // the draft is not written yet
	clickLayer(t, m, layerID(idQueueTab, 0))
	if !b.closed || m.composer != nil {
		t.Fatalf("closed=%v composer=%v, want the box gone with its ticket", b.closed, m.composer)
	}
	if got := drafts.Open(dir).Get(id); got != b.text {
		t.Fatalf("draft on disk after leaving = %q", got)
	}

	clickLayer(t, m, layerID(idTicketTab, 0))
	if got := eds.box(t); got == b || got.cfg.Markdown != b.text {
		t.Fatalf("the box opened again with %q", got.cfg.Markdown)
	}

	// Discard empties the box: this one goes and an empty one takes its place.
	b = eds.box(t)
	clickLayer(t, m, idCommentDiscard)
	if got := drafts.Open(dir).Get(id); got != "" || !b.closed {
		t.Fatalf("after discard: draft=%q closed=%v", got, b.closed)
	}
	if got := eds.box(t); got == b || got.closed || got.cfg.Markdown != "" || m.composer == nil {
		t.Fatalf("after discard the box holds %q (composer=%v)", got.cfg.Markdown, m.composer)
	}
}

// mentioner is a provider with its own search for people to mention.
type mentioner struct {
	provider.Provider
	asked []string
}

func (p *mentioner) Mentionable(_ context.Context, _, query string) ([]ticket.User, error) {
	p.asked = append(p.asked, query)
	return []ticket.User{{ID: "u-7", Name: "Robin Reader"}}, nil
}

func TestMentionsOfferPeopleMentionedBeforeWithoutASearch(t *testing.T) {
	known := people.Open("")
	prov := &mentioner{Provider: fake.New(0)}
	find := mentions(prov, known, "DEMO-101")
	ctx := context.Background()

	// With nobody kept, the provider's own search for mentions is asked.
	got, err := find(ctx, "ro")
	if err != nil || len(got) != 1 || got[0].ID != "u-7" || len(prov.asked) != 1 {
		t.Fatalf("found %+v, %v after asking %v", got, err, prov.asked)
	}
	if err := known.Note(ticket.User{ID: "u-1", Name: "Rowan Kept"}); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"", "ro"} {
		got, err = find(ctx, query)
		if err != nil || len(got) != 1 || got[0].ID != "u-1" {
			t.Fatalf("query %q found %+v, %v", query, got, err)
		}
	}
	if len(prov.asked) != 1 {
		t.Fatalf("a kept person matched, and the provider was still asked: %v", prov.asked)
	}
	// A name nobody kept matches goes to the provider.
	if got, _ = find(ctx, "rob"); len(got) != 1 || got[0].ID != "u-7" || len(prov.asked) != 2 {
		t.Fatalf("found %+v after asking %v", got, prov.asked)
	}
}

func TestSendingACommentKeepsThePeopleItMentions(t *testing.T) {
	m, eds := newCommenting(t, 110, 30, Options{})
	b := openBox(t, m, eds)
	b.text = "over to [@Sam Sample](mention:u%3A2)"
	send(t, m, editor.Changed{})

	m.prov.(*fake.Provider).FailWrites = errors.New("the site said no")
	clickLayer(t, m, idCommentSend)
	if got := m.people.Match(""); len(got) != 0 {
		t.Fatalf("a failed send kept %+v", got)
	}
	m.ops.Dismiss()
	m.prov.(*fake.Provider).FailWrites = nil

	clickLayer(t, m, idCommentSend)
	want := ticket.User{ID: "u:2", Name: "Sam Sample"}
	if got := m.people.Match("sam"); len(got) != 1 || got[0] != want {
		t.Fatalf("kept %+v, want %+v", got, want)
	}
	// The next box offers that person without a search.
	found, err := eds.box(t).cfg.Mentions(context.Background(), "")
	if err != nil || len(found) != 1 || found[0] != want {
		t.Fatalf("offered %+v, %v", found, err)
	}
}

func TestSendingAComment(t *testing.T) {
	m, eds := newCommenting(t, 110, 30, Options{})
	b := openBox(t, m, eds)
	id := m.tabs[0].t.ID
	before := len(stored(t, m, id).Comments)

	clickLayer(t, m, idCommentSend)
	if !strings.Contains(m.notice, "empty") || len(stored(t, m, id).Comments) != before {
		t.Fatalf("an empty comment: notice=%q", m.notice)
	}

	b.text = "Shipped in **1.2**"
	send(t, m, editor.Changed{})

	// A send that fails keeps the box and the draft.
	m.prov.(*fake.Provider).FailWrites = errors.New("the site said no")
	clickLayer(t, m, idCommentSend)
	if _, failed := m.ops.Failed(); !failed || b.closed || m.composer == nil || m.drafts.Get(id) != b.text {
		t.Fatalf("after a failed send: failed=%v closed=%v composer=%v draft=%q", failed, b.closed, m.composer, m.drafts.Get(id))
	}
	if len(stored(t, m, id).Comments) != before {
		t.Fatal("a failed send added a comment")
	}
	m.ops.Dismiss()
	m.prov.(*fake.Provider).FailWrites = nil

	// ctrl+s sends from inside the box.
	send(t, m, key('c'))
	send(t, m, ctrlS)
	comments := stored(t, m, id).Comments
	if len(comments) != before+1 {
		t.Fatalf("%d comments, want %d", len(comments), before+1)
	}
	if c := comments[len(comments)-1]; c.Body != "Shipped in **1.2**" || c.Author != fake.People[0] {
		t.Fatalf("the new comment = %+v", c)
	}
	if !b.closed || m.drafts.Get(id) != "" {
		t.Fatalf("after sending: closed=%v draft=%q", b.closed, m.drafts.Get(id))
	}
	if got := m.people.Match(""); len(got) != 0 {
		t.Fatalf("a comment that mentions nobody left %+v", got)
	}
	if got := len(m.tabs[0].t.Comments); got != before+1 || !strings.Contains(m.notice, "Commented on") {
		t.Fatalf("the open ticket has %d comments, notice %q", got, m.notice)
	}
	// An empty box is ready for the next comment.
	if next := eds.box(t); next == b || next.closed || next.cfg.Markdown != "" || next.focused {
		t.Fatalf("the box after sending holds %q (closed=%v focused=%v)", next.cfg.Markdown, next.closed, next.focused)
	}
}

func TestCommentBoxThatDoesNotOpen(t *testing.T) {
	m, eds := newCommenting(t, 110, 30, Options{})
	eds.err = errors.New("no terminal picture here")
	clickLayer(t, m, layerID(idRow, 0))
	if m.composer != nil || !strings.Contains(m.notice, "no terminal picture here") {
		t.Fatalf("composer=%v notice=%q", m.composer, m.notice)
	}
	// It is not tried again with every message, and the line says why.
	send(t, m, odd{})
	send(t, m, key('r'))
	if eds.tries != 1 {
		t.Fatalf("the editor was tried %d times, want once", eds.tries)
	}
	if out := screen(m); !strings.Contains(out, "did not open: no terminal picture here") {
		t.Fatalf("the box's line does not say why\n%s", out)
	}
	checkFrames(t, m, "ticket with a comment box that did not open")

	eds.err = nil
	send(t, m, key('c'))
	if b := eds.box(t); eds.tries != 2 || !b.focused || !b.started {
		t.Fatalf("tries=%d focused=%v started=%v, want a box with the keyboard", eds.tries, b.focused, b.started)
	}
}

func TestCommentBoxOpenedForATicketNoLongerShownIsClosed(t *testing.T) {
	m, eds := newCommenting(t, 110, 30, Options{})
	first := openBox(t, m, eds)
	cmd := m.openComposer(m.tabs[0].t)
	if !first.closed {
		t.Fatal("the box before it was left open")
	}
	clickLayer(t, m, layerID(idQueueTab, 0))
	drain(t, m, cmd)
	if b := eds.box(t); b == first || !b.closed || b.started || m.composer != nil {
		t.Fatalf("closed=%v started=%v composer=%v", b.closed, b.started, m.composer)
	}
}

func TestCommentBoxWithADraftOpensTall(t *testing.T) {
	m, eds := newCommenting(t, 110, 40, Options{})
	b := openBox(t, m, eds)
	idle := b.rect.h
	id := m.tabs[m.activeTab].t.ID
	if err := m.drafts.Set(id, "left from before"); err != nil {
		t.Fatal(err)
	}
	m.closeComposer(false)
	drain(t, m, m.syncComposer())
	b = eds.box(t)
	if b.cfg.Markdown != "left from before" || b.focused || b.rect.h <= idle {
		t.Fatalf("reopened with draft %q: focused=%v box at %+v, idle was %d rows", b.cfg.Markdown, b.focused, b.rect, idle)
	}
}

func TestOneTicketHasACommentBox(t *testing.T) {
	prof, err := profile.Parse(testProfile)
	if err != nil {
		t.Fatal(err)
	}
	eds := &editors{}
	m := New(prof, fake.New(0), Options{Ticket: "DEMO-104", OpenEditor: eds.open})
	send(t, m, tea.WindowSizeMsg{Width: 110, Height: 30})
	drain(t, m, m.Init())
	b := eds.box(t)
	if b.rect != inside(m) || b.rect.h == 0 {
		t.Fatalf("box at %+v, want %+v", b.rect, inside(m))
	}
	checkFrames(t, m, "one ticket with the comment box")
	// Quitting keeps what was typed.
	b.text = "to finish later"
	_, _ = m.Update(editor.Changed{})
	m.Close()
	if !b.closed || m.drafts.Get("DEMO-104") != b.text {
		t.Fatalf("closed=%v draft=%q", b.closed, m.drafts.Get("DEMO-104"))
	}
}

func TestCommentKindIsPickedOnTheLine(t *testing.T) {
	m, eds := newCommenting(t, 110, 30, Options{})
	p := m.prov.(*fake.Provider)
	b := openBox(t, m, eds)
	id := m.tabs[0].t.ID
	m.View()
	if m.comp.GetLayer(layerID(idCommentKind, 0)) != nil {
		t.Fatal("a provider with no kinds of comment has a kind on the line")
	}

	p.Kinds = []provider.CommentKind{{ID: provider.KindInternal, Name: "internal"}, {ID: provider.KindPublic, Name: "public"}}
	send(t, m, odd{})
	if got := m.commentKind(id); got != provider.KindInternal {
		t.Fatalf("a new comment is %q, want the first kind", got)
	}

	// A click on a kind picks it and leaves the keyboard in the box.
	send(t, m, key('c'))
	clickLayer(t, m, layerID(idCommentKind, 1))
	if got := m.commentKind(id); got != provider.KindPublic || !m.composer.writing {
		t.Fatalf("after a click on the second kind: kind=%q writing=%v", got, m.composer.writing)
	}
	// ctrl+k goes to the next kind and is not the editor's.
	n := len(b.got)
	send(t, m, tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if got := m.commentKind(id); got != provider.KindInternal || len(b.got) != n {
		t.Fatalf("after ctrl+k: kind=%q, the box got %d more messages", got, len(b.got)-n)
	}

	// The comment is posted as the kind picked, and the next starts as the first.
	clickLayer(t, m, layerID(idCommentKind, 1))
	b.text = "For the customer"
	send(t, m, editor.Changed{})
	send(t, m, ctrlS)
	if cs := stored(t, m, id).Comments; cs[len(cs)-1].Kind != "" || cs[len(cs)-1].Body != b.text {
		t.Fatalf("a public comment was stored as %+v", cs[len(cs)-1])
	}
	if got := m.commentKind(id); got != provider.KindInternal {
		t.Fatalf("after sending, the next comment is %q", got)
	}
	b = eds.box(t)
	b.text = "Between us"
	send(t, m, editor.Changed{})
	clickLayer(t, m, idCommentSend)
	if cs := stored(t, m, id).Comments; cs[len(cs)-1].Kind != provider.KindInternal {
		t.Fatalf("an internal note was stored as %+v", cs[len(cs)-1])
	}
	if !strings.Contains(screen(m), "internal") {
		t.Fatal("the internal note is not marked in the list of comments")
	}
}
