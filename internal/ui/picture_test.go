package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lawsmd/tui-q/internal/attach"
	"github.com/lawsmd/tui-q/internal/editor"
	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider/fake"
)

// fakePicture stands in for a picture of 48 by 14 cells. It records what
// the model did with it.
type fakePicture struct {
	path        string
	whole, view editor.Rect
	started     bool
	covered     bool
	closed      bool
	got         []tea.Msg
}

func (p *fakePicture) Init() tea.Cmd { p.started = true; return nil }
func (p *fakePicture) Update(msg tea.Msg) tea.Cmd {
	p.got = append(p.got, msg)
	return nil
}
func (p *fakePicture) Fit(cols, rows int) (int, int) {
	w := min(48, cols)
	return w, min(max(14*w/48, 1), rows)
}
func (p *fakePicture) Show(whole, view editor.Rect) tea.Cmd {
	p.whole, p.view = whole, view
	return nil
}
func (p *fakePicture) Cover(on bool) tea.Cmd { p.covered = on; return nil }
func (p *fakePicture) Close()                { p.closed = true }

// newPictures shows the fixture ticket that has a picture, with pictures
// turned on in the profile, and returns the pictures the model opens.
func newPictures(t *testing.T, w, h int, open func(*fakePicture) error) (*Model, *[]*fakePicture) {
	t.Helper()
	prof, err := profile.Parse(testProfile)
	if err != nil {
		t.Fatal(err)
	}
	prof.Ticket.Pictures = true
	pics := &[]*fakePicture{}
	m := New(prof, fake.New(0), Options{
		Ticket: fake.PictureTicket,
		Files:  attach.Open(t.TempDir()),
		OpenPicture: func(path, _ string) (editor.Picture, error) {
			p := &fakePicture{path: path}
			if open != nil {
				if err := open(p); err != nil {
					return nil, err
				}
			}
			*pics = append(*pics, p)
			return p, nil
		},
	})
	send(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	drain(t, m, m.Init())
	// One more message places the picture that the last one opened.
	send(t, m, odd{})
	return m, pics
}

// part is the cells of a picture that are in view.
func part(p *fakePicture) area {
	x0, y0 := max(p.whole.X, p.view.X), max(p.whole.Y, p.view.Y)
	x1, y1 := min(p.whole.X+p.whole.W, p.view.X+p.view.W), min(p.whole.Y+p.whole.H, p.view.Y+p.view.H)
	if x1 <= x0 || y1 <= y0 {
		return area{}
	}
	return area{x0, y0, x1 - x0, y1 - y0}
}

func TestPictureIsDrawnUnderItsLine(t *testing.T) {
	m, pics := newPictures(t, 110, 60, nil)
	if len(*pics) != 1 {
		t.Fatalf("opened %d pictures, want the one in view\n%s", len(*pics), screen(m))
	}
	p := (*pics)[0]
	if !p.started || !strings.HasSuffix(p.path, ".png") || p.whole.W != 48 || p.whole.H != 14 {
		t.Fatalf("picture: %+v", p)
	}
	rows := strings.Split(screen(m), "\n")
	if got := rows[p.whole.Y-1]; !strings.Contains(got, "[image: error-page.png]") {
		t.Errorf("the line over the picture is %q", got)
	}
	if at := strings.Index(rows[p.whole.Y-1], "[image"); at != p.whole.X {
		t.Errorf("the picture starts at column %d and its line at %d", p.whole.X, at)
	}
	// A click on the picture opens it, as its line does.
	var opened []string
	m.openFile = func(path string) error {
		opened = append(opened, path)
		return nil
	}
	send(t, m, tea.MouseClickMsg{X: p.whole.X + 3, Y: p.whole.Y + 3, Button: tea.MouseLeft})
	send(t, m, tea.MouseReleaseMsg{X: p.whole.X + 3, Y: p.whole.Y + 3, Button: tea.MouseLeft})
	if len(opened) != 1 || opened[0] != p.path {
		t.Errorf("a click on the picture opened %v, want %s", opened, p.path)
	}
}

func TestPictureFitsEveryPane(t *testing.T) {
	m, pics := newPictures(t, 110, 60, nil)
	p := (*pics)[0]
	tab := m.tabs[0]
	for w := minWidth; w <= 160; w += 11 {
		for h := minHeight; h <= 44; h += 3 {
			send(t, m, tea.WindowSizeMsg{Width: w, Height: h})
			for tab.offset = 0; ; tab.offset += 1 + h/3 {
				at := tab.offset
				send(t, m, odd{})
				out := m.View().Content
				if gw, gh := lipgloss.Width(out), lipgloss.Height(out); gw != w || gh != h {
					t.Fatalf("pane %dx%d drew %dx%d", w, h, gw, gh)
				}
				// The cells the picture is drawn over are blank, and inside
				// the window.
				rows := strings.Split(screen(m), "\n")
				v := part(p)
				if v.x < 0 || v.y < 0 || v.x+v.w > w || v.y+v.h > h {
					t.Fatalf("pane %dx%d, offset %d: the picture is drawn at %+v", w, h, at, v)
				}
				for y := v.y; y < v.y+v.h; y++ {
					row := []rune(rows[y])
					if got := string(row[v.x : v.x+v.w]); strings.TrimSpace(got) != "" {
						t.Fatalf("pane %dx%d, offset %d: row %d under the picture holds %q", w, h, at, y, got)
					}
				}
				if tab.offset != at {
					break
				}
			}
		}
	}
}

func TestPictureIsCoveredByAMenuAndClosedWithItsTicket(t *testing.T) {
	m, pics := newPictures(t, 110, 60, nil)
	p := (*pics)[0]
	m.menu = &menu{}
	send(t, m, odd{})
	if !p.covered {
		t.Error("the picture draws under an open menu")
	}
	m.menu = nil
	send(t, m, odd{})
	if p.covered {
		t.Error("the picture stays covered after the menu closed")
	}
	if got := p.got[len(p.got)-1]; got != (odd{}) {
		t.Errorf("the picture was not passed a message of its own: last got %T", got)
	}
	m.closeTab(0)
	send(t, m, odd{})
	if !p.closed || m.pics != nil {
		t.Errorf("closed=%v pics=%v after the ticket left the screen", p.closed, m.pics)
	}
}

func TestPictureThatCannotBeDrawnKeepsItsLine(t *testing.T) {
	m, pics := newPictures(t, 110, 60, func(*fakePicture) error { return errors.New("not a picture") })
	if len(*pics) != 0 || !strings.Contains(screen(m), "[image: error-page.png]") {
		t.Fatalf("pictures=%d\n%s", len(*pics), screen(m))
	}
	if !strings.Contains(m.notice, "error-page.png is shown by name only") {
		t.Errorf("notice: %q", m.notice)
	}
	if _, failed := m.ops.Failed(); failed {
		t.Error("a picture that cannot be drawn failed an operation")
	}

	// Where the terminal shows no picture at all, none is tried again.
	tries := 0
	m, _ = newPictures(t, 110, 60, func(*fakePicture) error {
		tries++
		return editor.ErrNoPictures
	})
	m.tabs[0].offset = 1000
	send(t, m, odd{})
	if tries != 1 || m.drawsPictures() {
		t.Errorf("tries=%d drawsPictures=%v", tries, m.drawsPictures())
	}
}

func TestPicturesAreOffUnlessTheProfileAsks(t *testing.T) {
	prof, err := profile.Parse(testProfile)
	if err != nil {
		t.Fatal(err)
	}
	opened := 0
	m := New(prof, fake.New(0), Options{
		Ticket: fake.PictureTicket,
		Files:  attach.Open(t.TempDir()),
		OpenPicture: func(string, string) (editor.Picture, error) {
			opened++
			return &fakePicture{}, nil
		},
	})
	send(t, m, tea.WindowSizeMsg{Width: 110, Height: 60})
	drain(t, m, m.Init())
	send(t, m, odd{})
	if opened != 0 {
		t.Errorf("opened %d pictures with pictures off", opened)
	}
}
