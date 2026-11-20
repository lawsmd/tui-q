package ui

import (
	"strings"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lawsmd/tui-q/ticket"
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

// fileLine is a line of rendered Markdown that stands for an attachment:
// the line, the cell its text starts at, the text unstyled, and the ID of
// the attachment.
type fileLine struct {
	line, x int
	text    string
	id      string
}

// markdownLines renders Markdown to styled lines no wider than w. Fenced
// code blocks are drawn by codeBlockLines and the rest by glamour, with a
// blank line between the two.
func (s Styles) markdownLines(text string, w int) []string {
	lines, _ := s.markdownFiles(text, w)
	return lines
}

// markdownFiles is markdownLines, and also returns the lines that stand
// for attachments. A reference to an attachment on a line of its own gets
// a line to itself, naming the file. One inside other text is written as
// the name of the file, in square brackets.
func (s Styles) markdownFiles(text string, w int) ([]string, []fileLine) {
	if w < 1 {
		return nil, nil
	}
	var (
		lines []string
		files []fileLine
	)
	for _, seg := range splitFences(text) {
		if seg.code {
			lines = join(lines, s.codeBlockLines(seg.text, seg.lang, w, markdownMargin))
			continue
		}
		for _, part := range splitFiles(seg.text) {
			if part.file == nil {
				lines = join(lines, proseLines(fileNames(part.text), w))
				continue
			}
			label := fileLabel(part.file.Name)
			if w > markdownMargin {
				label = ansi.Truncate(label, w-markdownMargin, "…")
			} else {
				label = ""
			}
			lines = join(lines, []string{strings.Repeat(" ", min(markdownMargin, w)) + s.Faint.Render(label)})
			files = append(files, fileLine{line: len(lines) - 1, x: markdownMargin, text: label, id: part.file.ID})
		}
	}
	return lines, files
}

// join adds a block of lines to those before it, with a blank line between
// the two. An empty block adds nothing.
func join(lines, part []string) []string {
	if len(part) == 0 {
		return lines
	}
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	return append(lines, part...)
}

// fileLabel is what a line that stands for an attachment says.
func fileLabel(name string) string {
	if ticket.ImageType(name) != "" {
		return "[image: " + name + "]"
	}
	return "[file: " + name + "]"
}

// filePart is a run of prose, or one reference to an attachment that had a
// line to itself.
type filePart struct {
	text string
	file *ticket.Ref
}

// splitFiles divides prose at every line that is one reference to an
// attachment and nothing else. Only a line indented less than four cells
// counts, so a reference inside a list or a quote stays in the prose.
func splitFiles(text string) []filePart {
	var (
		out   []filePart
		prose []string
	)
	flush := func() {
		if strings.TrimSpace(strings.Join(prose, "")) != "" {
			out = append(out, filePart{text: strings.Join(prose, "\n")})
		}
		prose = nil
	}
	for _, line := range strings.Split(text, "\n") {
		body := strings.TrimSpace(line)
		refs := ticket.Refs(body)
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if len(refs) != 1 || refs[0].Start != 0 || refs[0].End != len(body) || indent > 3 {
			prose = append(prose, line)
			continue
		}
		flush()
		out = append(out, filePart{file: &refs[0]})
	}
	flush()
	return out
}

// fileNames writes each reference to an attachment in prose as the name of
// the file in square brackets.
func fileNames(text string) string {
	refs := ticket.Refs(text)
	for i := len(refs) - 1; i >= 0; i-- {
		r := refs[i]
		text = text[:r.Start] + "\\[" + r.Name + "\\]" + text[r.End:]
	}
	return text
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
