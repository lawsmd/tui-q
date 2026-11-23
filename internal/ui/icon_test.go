package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider/fake"
)

const iconProfile = `
[provider]
kind = "fake"

[[queue]]
name = "Work"
query = ""
columns = ["type", "key", "title", "status", "assignee", "priority", "created", "updated"]

[ticket]
sidebar = ["type", "status"]

[fields.type.icons]
bug = "bug"
`

func TestIconColumn(t *testing.T) {
	prof, err := profile.Parse(iconProfile)
	if err != nil {
		t.Fatal(err)
	}
	m := New(prof, fake.New(0), Options{})
	send(t, m, tea.WindowSizeMsg{Width: 110, Height: 40})
	drain(t, m, m.Init())

	// The column is one cell wide, has no heading, and the key follows it.
	// The copy-link button comes before every column.
	rows := strings.Split(screen(m), "\n")
	if !strings.HasPrefix(rows[2], "      Key") {
		t.Fatalf("header: %q", rows[2])
	}
	q := m.queues[0]
	for i, tk := range q.tickets[:3] {
		want := " " + linkGlyph + " ·  " + tk.Key
		if tk.Type == "Bug" {
			want = " " + linkGlyph + " ¤  " + tk.Key
		}
		if !strings.HasPrefix(rows[3+i], want) {
			t.Errorf("row %d: %q, want it to start %q", i, rows[3+i], want)
		}
	}

	// The icon column and the key stay when the table scrolls sideways.
	send(t, m, tea.WindowSizeMsg{Width: 70, Height: 40})
	q.colOffset = 1
	cols, _ := q.columns(m, 64, 1)
	if cols[0].name != "type" || cols[1].name != "key" || cols[2].name != "title" {
		t.Errorf("pinned columns: %+v", cols)
	}
	checkFrames(t, m, "icon column")

	// The ticket view names the value after its icon.
	send(t, m, tea.WindowSizeMsg{Width: 110, Height: 40})
	clickLayer(t, m, layerID(idRow, 0))
	if out := screen(m); !strings.Contains(out, "¤ Bug") {
		t.Fatalf("the sidebar does not show the icon with its value:\n%s", out)
	}
}
