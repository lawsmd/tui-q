package ui

import (
	"strings"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/x/ansi"
)

// renderers holds one glamour renderer per wrap width. Building one is
// slow next to using it, and a pane has few widths in a session.
var renderers = map[int]*glamour.TermRenderer{}

// markdownLines renders Markdown to styled lines no wider than w. When
// glamour fails the text is returned unstyled.
func markdownLines(text string, w int) []string {
	if w < 1 {
		return nil
	}
	r, ok := renderers[w]
	if !ok {
		var err error
		r, err = glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(w))
		if err != nil {
			r = nil
		}
		renderers[w] = r
	}
	out := text
	if r != nil {
		if rendered, err := r.Render(text); err == nil {
			out = rendered
		}
	}
	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	// Drop the blank lines glamour puts around a document.
	for len(lines) > 0 && strings.TrimSpace(ansi.Strip(lines[0])) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(ansi.Strip(lines[len(lines)-1])) == "" {
		lines = lines[:len(lines)-1]
	}
	// Code blocks and tables are not wrapped, so cut what runs over.
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, w, "")
	}
	return lines
}
