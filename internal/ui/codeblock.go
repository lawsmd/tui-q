package ui

import (
	"encoding/json"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/x/ansi"
)

// codeBar is drawn down the left edge of a code block.
const codeBar = "▎"

// segment is a run of Markdown: prose for glamour, or the inside of one
// fenced code block.
type segment struct {
	code bool
	text string
	// lang is the first word after the opening fence of a code block.
	lang string
}

var fenceOpen = regexp.MustCompile("^( {0,3})(`{3,}|~{3,})[ \t]*([^ \t`]*)[^`]*$")

// splitFences divides Markdown into prose and fenced code blocks. Only a
// fence indented less than four cells counts, so a block inside a list or
// a quote stays in the prose. A fence that is never closed runs to the end
// of the text.
func splitFences(text string) []segment {
	var (
		out   []segment
		prose []string
	)
	flush := func() {
		if strings.TrimSpace(strings.Join(prose, "")) != "" {
			out = append(out, segment{text: strings.Join(prose, "\n")})
		}
		prose = nil
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		open := fenceOpen.FindStringSubmatch(lines[i])
		if open == nil {
			prose = append(prose, lines[i])
			continue
		}
		flush()
		indent, fence := open[1], open[2]
		var code []string
		for i++; i < len(lines); i++ {
			if closesFence(lines[i], fence) {
				break
			}
			// A block keeps its own indentation less the indent of its fence.
			line := lines[i]
			for n := 0; n < len(indent) && strings.HasPrefix(line, " "); n++ {
				line = line[1:]
			}
			code = append(code, line)
		}
		out = append(out, segment{code: true, text: strings.Join(code, "\n"), lang: open[3]})
	}
	flush()
	return out
}

// closesFence reports whether line ends a block opened with fence: the same
// mark, at least as many times, and nothing after it.
func closesFence(line, fence string) bool {
	rest := strings.TrimRight(strings.TrimLeft(line, " "), " \t")
	if len(line)-len(strings.TrimLeft(line, " ")) > 3 || len(rest) < len(fence) {
		return false
	}
	return strings.Trim(rest, fence[:1]) == ""
}

// Names a platform gives a block that is in no language.
var noLanguage = map[string]bool{"": true, "none": true, "text": true, "plain": true, "plaintext": true}

var sqlStart = regexp.MustCompile(`(?is)^\s*(select\s|with\s+\w+\s+as\b|insert\s+into\s|update\s+\S+\s+set\s|delete\s+from\s|create\s+(or\s+replace\s+)?(table|view|index|function|procedure)\s|alter\s+table\s)`)

// lexerFor picks the lexer for a code block and the label to show over it.
// A language the platform named is trusted. With none, SQL and JSON are
// recognised from the text. A nil lexer means no colouring, and an empty
// label means no label row.
func lexerFor(code, lang string) (chroma.Lexer, string) {
	name := strings.ToLower(strings.TrimSpace(lang))
	if !noLanguage[name] {
		return lexers.Get(name), name
	}
	switch trimmed := strings.TrimSpace(code); {
	case sqlStart.MatchString(code):
		return lexers.Get("sql"), "sql"
	case (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) && json.Valid([]byte(trimmed)):
		return lexers.Get("json"), "json"
	}
	return nil, ""
}

// codeStyle is the style a token of a code block is drawn in.
func (s Styles) codeStyle(t chroma.TokenType) lipgloss.Style {
	switch {
	case t.InCategory(chroma.Comment):
		return s.CodeComment
	case t.InCategory(chroma.Keyword):
		return s.CodeKeyword
	case t.InSubCategory(chroma.LiteralString):
		return s.CodeString
	case t.InSubCategory(chroma.LiteralNumber):
		return s.CodeNumber
	case t.InCategory(chroma.Operator), t.InCategory(chroma.Punctuation):
		return s.CodeOperator
	case t == chroma.NameFunction, t == chroma.NameBuiltin, t == chroma.NameClass,
		t == chroma.NameTag, t == chroma.NameDecorator:
		return s.CodeName
	}
	return s.Code
}

// codeLine is one line of a code block: its text, and the same text
// coloured.
type codeLine struct{ plain, styled string }

// highlight colours code line by line. With no lexer, or when the lexer
// fails, every line is drawn in the plain code style.
func (s Styles) highlight(code string, lexer chroma.Lexer) []codeLine {
	var out []codeLine
	if lexer != nil {
		if it, err := chroma.Coalesce(lexer).Tokenise(nil, code); err == nil {
			for _, tokens := range chroma.SplitTokensIntoLines(it.Tokens()) {
				var line codeLine
				for _, tok := range tokens {
					value := strings.TrimRight(tok.Value, "\n")
					if value == "" {
						continue
					}
					line.plain += value
					line.styled += s.codeStyle(tok.Type).Render(value)
				}
				out = append(out, line)
			}
			// The lexer is given a last newline the code does not have.
			if n := len(out); n > 0 && out[n-1].plain == "" && !strings.HasSuffix(code, "\n") {
				out = out[:n-1]
			}
			return out
		}
	}
	for _, line := range strings.Split(code, "\n") {
		out = append(out, codeLine{line, s.Code.Render(line)})
	}
	return out
}

// codeBlockLines draws a code block as lines exactly w cells wide: margin
// blank cells, a bar, and the code on a tint. The first row names the
// language when one is known. A line too long for the width wraps onto as
// many rows as it needs, so the whole block is always shown.
func (s Styles) codeBlockLines(code, lang string, w, margin int) []string {
	code = strings.ReplaceAll(ansi.Strip(code), "\t", "    ")
	code = strings.ReplaceAll(code, "\r", "")
	raw := strings.Split(code, "\n")
	for len(raw) > 0 && strings.TrimSpace(raw[0]) == "" {
		raw = raw[1:]
	}
	for len(raw) > 0 && strings.TrimSpace(raw[len(raw)-1]) == "" {
		raw = raw[:len(raw)-1]
	}
	if len(raw) == 0 {
		return nil
	}
	code = strings.Join(raw, "\n")

	// The bar and the space after it take two cells.
	inner := w - margin - 2
	if inner < 1 {
		return nil
	}
	lead := strings.Repeat(" ", margin) + s.CodeBar.Render(codeBar) + s.Code.Render(" ")
	// rows wraps one line of code at spaces where it can and mid-word where
	// it cannot, and tints each row to the full width.
	rows := func(plain, styled string) []string {
		parts := []string{styled}
		if ansi.StringWidth(plain) > inner {
			parts = strings.Split(lipgloss.Wrap(styled, inner, ""), "\n")
		}
		out := make([]string, len(parts))
		for i, p := range parts {
			// Wrap can leave a run of leading spaces wider than a very
			// narrow block; only blank cells are cut.
			p = ansi.Truncate(p, inner, "")
			gap := max(inner-ansi.StringWidth(p), 0)
			out[i] = lead + p + s.Code.Render(strings.Repeat(" ", gap))
		}
		return out
	}

	lexer, label := lexerFor(code, lang)
	var out []string
	if label != "" {
		out = append(out, rows(label, s.CodeLabel.Render(label))...)
	}
	for _, line := range s.highlight(code, lexer) {
		out = append(out, rows(line.plain, line.styled)...)
	}
	return out
}
