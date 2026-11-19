package ui

import (
	"strings"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/x/ansi"
)

// markdownMargin is the blank cells glamour's style puts before every line.
const markdownMargin = 2

// unindent cuts n blank cells from the start of every line. When any line
// has something in those cells the lines are cut at the end instead, so
// that either way each is n cells narrower.
func unindent(lines []string, n int) []string {
	blank := strings.Repeat(" ", n)
	margin := true
	for _, line := range lines {
		if plain := ansi.Strip(line); plain != "" && !strings.HasPrefix(plain, blank) {
			margin = false
			break
		}
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		w := ansi.StringWidth(line)
		if margin {
			out[i] = ansi.Cut(line, n, max(w, n))
		} else {
			out[i] = ansi.Truncate(line, max(w-n, 0), "")
		}
	}
	return out
}

// renderers holds one glamour renderer per wrap width. Building one is
// slow next to using it, and a pane has few widths in a session.
var renderers = map[int]*glamour.TermRenderer{}

// markdownLines renders Markdown to styled lines no wider than w. Fenced
// code blocks are drawn by codeBlockLines and the rest by glamour, with a
// blank line between the two.
func (s Styles) markdownLines(text string, w int) []string {
	if w < 1 {
		return nil
	}
	var lines []string
	for _, seg := range splitFences(text) {
		var part []string
		if seg.code {
			part = s.codeBlockLines(seg.text, seg.lang, w, markdownMargin)
		} else {
			part = proseLines(seg.text, w)
		}
		if len(part) == 0 {
			continue
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, part...)
	}
	return lines
}

// proseLines renders Markdown with glamour to lines no wider than w. When
// glamour fails the text is returned unstyled.
func proseLines(text string, w int) []string {
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
	// Tables and nested code blocks are not wrapped, so cut what runs over.
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, w, "")
	}
	return lines
}
