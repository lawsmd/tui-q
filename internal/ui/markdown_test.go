package ui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSplitFences(t *testing.T) {
	cases := []struct {
		name, text string
		want       []segment
	}{
		{"prose only", "one\n\ntwo", []segment{{text: "one\n\ntwo"}}},
		{"one block", "before\n\n```sql\nselect 1\n```\n\nafter", []segment{
			{text: "before\n"},
			{code: true, text: "select 1", lang: "sql"},
			{text: "\nafter"},
		}},
		{"two blocks", "```\na\n```\n```go\nb\n```", []segment{
			{code: true, text: "a"},
			{code: true, text: "b", lang: "go"},
		}},
		{"tildes", "~~~ json\n{}\n~~~", []segment{{code: true, text: "{}", lang: "json"}}},
		{"longer fence holds a shorter one", "````\na\n```\nb\n````", []segment{
			{code: true, text: "a\n```\nb"},
		}},
		{"never closed", "x\n```\na\n\nb", []segment{
			{text: "x"},
			{code: true, text: "a\n\nb"},
		}},
		{"fence indent comes off the code", "  ```\n    a\n  b\n  ```", []segment{
			{code: true, text: "  a\nb"},
		}},
		{"nested block stays prose", "- item\n\n    ```\n    a\n    ```", []segment{
			{text: "- item\n\n    ```\n    a\n    ```"},
		}},
		{"line endings", "```\r\na\r\n```", []segment{{code: true, text: "a"}}},
	}
	for _, c := range cases {
		if got := splitFences(c.text); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got  %+v\n want %+v", c.name, got, c.want)
		}
	}
}

func plainLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = strings.TrimRight(ansi.Strip(line), " ")
	}
	return out
}

func TestCodeBlockIsDrawnCompact(t *testing.T) {
	s := DefaultStyles()
	text := "Proof:\n\n```sql\nSELECT id,\n  name\nFROM license;\n```\n\nDone."
	got := plainLines(s.markdownLines(text, 60))
	want := []string{
		"  Proof:",
		"",
		"  ▎ sql",
		"  ▎ SELECT id,",
		"  ▎   name",
		"  ▎ FROM license;",
		"",
		"  Done.",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestCodeBlockLabel(t *testing.T) {
	s := DefaultStyles()
	cases := []struct{ name, text, first string }{
		{"named", "```go\nx := 1\n```", "  ▎ go"},
		{"unknown name is shown", "```nolang\nx\n```", "  ▎ nolang"},
		{"sql is recognised", "```\nselect *\nfrom t\n```", "  ▎ sql"},
		{"json is recognised", "```\n{\"a\": [1, 2]}\n```", "  ▎ json"},
		{"plain text has no label", "```\nPOST /session 500\n```", "  ▎ POST /session 500"},
		{"a sentence is not sql", "```\nUpdate the config and retry\n```", "  ▎ Update the config and retry"},
		{"text is no language", "```text\nhello\n```", "  ▎ hello"},
	}
	for _, c := range cases {
		got := plainLines(s.markdownLines(c.text, 60))
		if len(got) == 0 || got[0] != c.first {
			t.Errorf("%s: first line %q, want %q", c.name, got, c.first)
		}
	}
}

func TestCodeBlockFitsTheWidth(t *testing.T) {
	s := DefaultStyles()
	long := "select " + strings.Repeat("column_name, ", 30) + "1"
	text := "Before.\n\n```sql\n" + long + "\n\tshort -- tab\n```"
	for w := 1; w <= 120; w++ {
		lines := s.markdownLines(text, w)
		for _, line := range lines {
			if got := ansi.StringWidth(line); got > w {
				t.Fatalf("width %d: line is %d cells: %q", w, got, ansi.Strip(line))
			}
		}
		if w < 20 {
			continue
		}
		plain := plainLines(lines)
		// The long line wraps onto rows that hold every word of it.
		var code []string
		for _, l := range plain[:len(plain)-1] {
			if rest, ok := strings.CutPrefix(l, "  ▎ "); ok && rest != "sql" {
				code = append(code, strings.TrimRight(rest, " "))
			}
		}
		if len(code) < 2 {
			t.Errorf("width %d: long line did not wrap: %q", w, code)
		}
		if got := strings.Join(strings.Fields(strings.Join(code, " ")), " "); got != long {
			t.Errorf("width %d: wrapped rows hold %q", w, got)
		}
		if last := plain[len(plain)-1]; last != "  ▎     short -- tab" {
			t.Errorf("width %d: last line %q", w, last)
		}
		// Every row of the block is tinted to the full width.
		if got := ansi.StringWidth(lines[len(lines)-1]); got != w {
			t.Errorf("width %d: short line is %d cells", w, got)
		}
	}
}

func TestUnindentKeepsCodeBlockWhole(t *testing.T) {
	s := DefaultStyles()
	lines := unindent(s.markdownLines("Text.\n\n```\nabc\n```", 30), markdownMargin)
	got := plainLines(lines)
	want := []string{"Text.", "", "▎ abc"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
