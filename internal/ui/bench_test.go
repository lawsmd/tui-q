package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/internal/ops"
	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider/fake"
	"github.com/lawsmd/tui-q/ticket"
)

// These benchmarks time what one message costs the interface: Update, then
// the View that Bubble Tea asks for after every message. With the mouse
// reported on every cell it crosses, that cost is paid many times a second.

// benchModel is a model in a large pane with a comment box, as in use.
func benchModel(b *testing.B) *Model {
	b.Helper()
	prof, err := profile.Parse(testProfile)
	if err != nil {
		b.Fatal(err)
	}
	eds := &editors{}
	m := New(prof, fake.New(0), Options{OpenEditor: eds.open})
	benchSend(m, tea.WindowSizeMsg{Width: 160, Height: 50})
	benchDrain(m, m.Init())
	return m
}

func benchDrain(m *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if strings.Contains(fmt.Sprintf("%T", msg), "cursor.") {
		return
	}
	switch msg := msg.(type) {
	case nil, spinner.TickMsg:
	case tea.BatchMsg:
		for _, c := range msg {
			benchDrain(m, c)
		}
	default:
		_, next := m.Update(msg)
		m.settle()
		benchDrain(m, next)
	}
}

func benchSend(m *Model, msg tea.Msg) {
	_, cmd := m.Update(msg)
	m.settle()
	benchDrain(m, cmd)
}

// longTicket is a ticket with a long Markdown body and 30 comments.
func longTicket(t ticket.Ticket) ticket.Ticket {
	var body strings.Builder
	for i := range 40 {
		fmt.Fprintf(&body, "## Step %d\n\nThe export **drops rows** when a name holds a comma, as in `Smith, J`. See [the report](https://example.com/r/%d).\n\n- first point\n- second point\n\n```go\nfor _, row := range rows {\n\tfmt.Println(row)\n}\n```\n\n", i, i)
	}
	t.Body = body.String()
	t.Comments = nil
	for i := range 30 {
		t.Comments = append(t.Comments, ticket.Comment{
			Author:  "Sam Sample",
			Created: time.Date(2026, 11, 1+i%28, 9, 0, 0, 0, time.UTC),
			Body:    fmt.Sprintf("Tried it again on build %d. It still fails **after** the second page.\n\n> the count is off by one\n\nSee `export.go`.", i),
		})
	}
	return t
}

// benchTicket opens the first ticket of the queue and puts a long one in
// its tab.
func benchTicket(b *testing.B) *Model {
	b.Helper()
	m := benchModel(b)
	m.View()
	row := m.comp.GetLayer(layerID(idRow, 0))
	benchSend(m, tea.MouseClickMsg{X: row.GetX(), Y: row.GetY(), Button: tea.MouseLeft})
	benchSend(m, tea.MouseReleaseMsg{X: row.GetX(), Y: row.GetY(), Button: tea.MouseLeft})
	if len(m.tabs) != 1 {
		b.Fatal("the ticket did not open")
	}
	m.tabs[0].set(longTicket(m.tabs[0].t))
	benchSend(m, tea.FocusMsg{})
	m.View()
	return m
}

// The mouse crossing the text of a ticket, where nothing is to be lit.
func BenchmarkMotionOverTicket(b *testing.B) {
	m := benchTicket(b)
	b.ResetTimer()
	for i := range b.N {
		m.Update(tea.MouseMotionMsg{X: 10 + i%40, Y: 20})
		m.View()
	}
}

// The mouse crossing the buttons over a ticket, where what is lit changes
// with every cell or two.
func BenchmarkMotionOverButtons(b *testing.B) {
	m := benchTicket(b)
	y := m.comp.GetLayer(idCopyLink).GetY()
	b.ResetTimer()
	for i := range b.N {
		m.Update(tea.MouseMotionMsg{X: 1 + i%40, Y: y})
		m.View()
	}
}

// The mouse crossing the rows of a table.
func BenchmarkMotionOverTable(b *testing.B) {
	m := benchModel(b)
	benchSend(m, tea.KeyPressMsg{Code: tea.KeyTab})
	m.View()
	b.ResetTimer()
	for i := range b.N {
		m.Update(tea.MouseMotionMsg{X: 10 + i%40, Y: 6 + i%5})
		m.View()
	}
}

// One turn of the status bar's spinner with a ticket on screen, which is
// 12 messages a second for as long as a request runs.
func BenchmarkSpinnerTick(b *testing.B) {
	m := benchTicket(b)
	m.start("Reading the notes", ops.Background, idle)
	b.ResetTimer()
	for range b.N {
		m.Update(m.spin.Tick())
		m.View()
	}
}

// A ticket drawn with none of its Markdown rendered yet, which is what the
// interface paid on every load before the command that reads a ticket
// rendered it.
func BenchmarkTicketDrawnCold(b *testing.B) {
	m := benchTicket(b)
	b.ResetTimer()
	for range b.N {
		forgetRendered()
		m.tabs[0].main = nil
		m.Update(tea.FocusMsg{})
		m.View()
	}
}

// A ticket drawn again after it was read back: its lines are laid out
// again, from Markdown that is already rendered.
func BenchmarkTicketDrawnAgain(b *testing.B) {
	m := benchTicket(b)
	b.ResetTimer()
	for range b.N {
		m.tabs[0].main = nil
		m.Update(tea.FocusMsg{})
		m.View()
	}
}

// What the command that reads a ticket spends rendering it, off the UI
// goroutine.
func BenchmarkPrewarm(b *testing.B) {
	m := benchTicket(b)
	t := m.tabs[0].t
	b.ResetTimer()
	for range b.N {
		forgetRendered()
		m.prewarm(t)
	}
}

// One turn of the spinner with a table on screen.
func BenchmarkSpinnerTickOverTable(b *testing.B) {
	m := benchModel(b)
	benchSend(m, tea.KeyPressMsg{Code: tea.KeyTab})
	m.start("Reading the notes", ops.Background, idle)
	b.ResetTimer()
	for range b.N {
		m.Update(m.spin.Tick())
		m.View()
	}
}

// One turn of the spinner over a table of 400 tickets, of which a pane
// shows a few dozen.
func BenchmarkSpinnerTickOverLongTable(b *testing.B) {
	m := benchModel(b)
	benchSend(m, tea.KeyPressMsg{Code: tea.KeyTab})
	q := m.queues[m.activeQueue]
	var many []ticket.Ticket
	for i := range 400 {
		t := q.tickets[i%len(q.tickets)]
		t.ID, t.Key = fmt.Sprintf("many-%d", i), fmt.Sprintf("DEMO-%d", 1000+i)
		many = append(many, t)
	}
	q.setSource(fromQueue, many)
	m.start("Reading the notes", ops.Background, idle)
	b.ResetTimer()
	for range b.N {
		m.Update(m.spin.Tick())
		m.View()
	}
}
