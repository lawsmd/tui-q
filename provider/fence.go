package provider

import "strings"

// Fence writes code as a Markdown fenced block, the form the ticket view
// draws as a code block. lang is the platform's name for the language, or
// empty when it gives none. Blank lines at either end of the code are
// dropped, and the fence is made longer than any run of backticks in the
// code.
func Fence(code, lang string) string {
	lines := strings.Split(strings.ReplaceAll(code, "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	n, run := 3, 0
	for _, line := range lines {
		for _, r := range line {
			if r != '`' {
				run = 0
				continue
			}
			if run++; run >= n {
				n = run + 1
			}
		}
		run = 0
	}
	fence := strings.Repeat("`", n)
	// An info string ends at a space and cannot hold a backtick.
	if words := strings.Fields(strings.ReplaceAll(lang, "`", "")); len(words) > 0 {
		lang = words[0]
	} else {
		lang = ""
	}
	var b strings.Builder
	b.WriteString(fence + lang + "\n")
	for _, line := range lines {
		b.WriteString(line + "\n")
	}
	b.WriteString(fence)
	return b.String()
}
