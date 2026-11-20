package ui

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/internal/attach"
	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider/fake"
	"github.com/lawsmd/tui-q/ticket"
)

func TestMarkdownGivesAnAttachmentALineOfItsOwn(t *testing.T) {
	s := DefaultStyles()
	pic := ticket.Attachment{ID: "7", Name: "shot.png"}
	doc := ticket.Attachment{ID: "8", Name: "notes.pdf"}
	text := "Before.\n\n" + pic.Ref() + "\n\nSee " + doc.Ref() + " too.\n\n```\n" + pic.Ref() + "\n```\n\n" + doc.Ref()
	lines, files := s.markdownFiles(text, 60)
	got := plainLines(lines)
	want := []string{
		"  Before.",
		"",
		"  [image: shot.png]",
		"",
		"  See [notes.pdf] too.",
		"",
		"  ▎ " + pic.Ref(),
		"",
		"  [file: notes.pdf]",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if len(files) != 2 || files[0] != (fileLine{line: 2, x: 2, text: "[image: shot.png]", id: "7"}) || files[1].line != 8 || files[1].id != "8" {
		t.Errorf("files %+v", files)
	}
	// A line is never wider than it was asked to be.
	for w := 1; w < 30; w++ {
		lines, _ := s.markdownFiles(pic.Ref(), w)
		if len(lines) != 1 || len([]rune(plainLines(lines)[0])) > w {
			t.Errorf("width %d: %q", w, lines)
		}
	}
}

func TestClickOnAPictureOpensItsFile(t *testing.T) {
	prof, err := profile.Parse(testProfile)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{110, 60}, {70, 80}} {
		m := New(prof, fake.New(0), Options{Ticket: fake.PictureTicket, Files: attach.Open(t.TempDir())})
		var opened []string
		m.openFile = func(path string) error {
			opened = append(opened, path)
			return nil
		}
		send(t, m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		drain(t, m, m.Init())
		checkFrames(t, m, "a ticket with a picture")

		send(t, m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})

		// The picture is in the body, which is in view at the top, and in
		// the oldest comment, which is in view at the end. Each line sits
		// on the text it stands over.
		for _, offset := range []int{0, 1000} {
			m.tabs[0].offset = offset
			rows := strings.Split(screen(m), "\n")
			id := layerID(idFile, len(m.tabs[0].files)-1)
			layer := m.comp.GetLayer(id)
			if layer == nil {
				t.Fatalf("%v: no line for a picture at offset %d\n%s", size, offset, screen(m))
			}
			row := []rune(rows[layer.GetY()])
			if got := string(row[layer.GetX():min(layer.GetX()+len("[image: error-page.png]"), len(row))]); got != "[image: error-page.png]" {
				t.Errorf("%v: the line at offset %d is over %q", size, offset, got)
			}
			clickLayer(t, m, id)
		}
		if _, failed := m.ops.Failed(); failed || len(opened) != 2 || opened[0] != opened[1] {
			t.Fatalf("%v: failed=%v opened=%v", size, failed, opened)
		}
		data, err := os.ReadFile(opened[0])
		if err != nil || !bytes.HasPrefix(data, []byte("\x89PNG")) || !strings.HasSuffix(opened[0], ".png") {
			t.Fatalf("%v: %s: %v, %d bytes", size, opened[0], err, len(data))
		}
	}
}

func TestOnlyAPictureCanBeOpened(t *testing.T) {
	m := newModel(t, 110, 30)
	tk := ticket.Ticket{Attachments: []ticket.Attachment{
		{ID: "1", Name: "shot.png", MIME: "image/png"},
		{ID: "2", Name: "tool.exe", MIME: "application/octet-stream"},
		{ID: "3", Name: "drawing.svg", MIME: "image/svg+xml"},
	}}
	for id, want := range map[string]bool{"1": true, "2": false, "3": false, "4": false} {
		if got := m.canOpen(tk, id); got != want {
			t.Errorf("canOpen(%s) = %v, want %v", id, got, want)
		}
	}
	if m.openAttachment(tk, "2") != nil {
		t.Error("a program file was given a command to open it")
	}
}
