package jira

import (
	"encoding/json"
	"fmt"
	"strings"
)

// node is one node of an Atlassian Document Format tree.
type node struct {
	Type    string         `json:"type"`
	Text    string         `json:"text"`
	Attrs   map[string]any `json:"attrs"`
	Marks   []mark         `json:"marks"`
	Content []node         `json:"content"`
}

type mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs"`
}

// isADF reports whether raw is an ADF document.
func isADF(raw json.RawMessage) bool {
	var probe struct {
		Type string `json:"type"`
	}
	return json.Unmarshal(raw, &probe) == nil && probe.Type == "doc"
}

// adfToMarkdown converts an ADF document to Markdown. Nodes with no
// Markdown form (media, cards) are written as short placeholders so the
// reader knows something is there and can open the ticket in a browser.
func adfToMarkdown(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var doc node
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(blocks(doc.Content, ""))
}

// blocks renders block nodes, each followed by a blank line. indent is put
// before every line, which is how nested lists and quotes are built.
func blocks(nodes []node, indent string) string {
	var b strings.Builder
	empty := strings.TrimRight(indent, " ") + "\n"
	for _, n := range nodes {
		text := block(n)
		if text == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString(empty)
		}
		for _, line := range strings.Split(text, "\n") {
			if line == "" {
				b.WriteString(empty)
			} else {
				b.WriteString(indent + line + "\n")
			}
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func block(n node) string {
	switch n.Type {
	case "paragraph":
		return inline(n.Content)
	case "heading":
		level := 1
		if v, ok := n.Attrs["level"].(float64); ok && v >= 1 && v <= 6 {
			level = int(v)
		}
		return strings.Repeat("#", level) + " " + inline(n.Content)
	case "bulletList":
		return list(n.Content, func(int) string { return "- " })
	case "orderedList":
		start := 1
		if v, ok := n.Attrs["order"].(float64); ok {
			start = int(v)
		}
		return list(n.Content, func(i int) string { return fmt.Sprintf("%d. ", start+i) })
	case "taskList":
		return list(n.Content, func(int) string { return "- " })
	case "codeBlock":
		lang, _ := n.Attrs["language"].(string)
		return "```" + lang + "\n" + plain(n.Content) + "\n```"
	case "blockquote":
		return blocks(n.Content, "> ")
	case "panel":
		kind, _ := n.Attrs["panelType"].(string)
		body := blocks(n.Content, "> ")
		if kind == "" {
			return body
		}
		return "> **" + strings.ToUpper(kind[:1]) + kind[1:] + "**\n>\n" + body
	case "rule":
		return "---"
	case "table":
		return table(n)
	case "expand", "nestedExpand":
		title, _ := n.Attrs["title"].(string)
		body := blocks(n.Content, "")
		if title == "" {
			return body
		}
		return "**" + title + "**\n\n" + body
	case "mediaSingle", "mediaGroup", "media":
		return "*[attachment]*"
	case "blockCard", "embedCard":
		if url, _ := n.Attrs["url"].(string); url != "" {
			return "<" + url + ">"
		}
		return ""
	}
	if len(n.Content) > 0 {
		return blocks(n.Content, "")
	}
	return inline([]node{n})
}

// list renders list items. The first line of an item carries the marker;
// the rest are indented to line up under it.
func list(items []node, marker func(i int) string) string {
	var out []string
	for i, item := range items {
		m := marker(i)
		if item.Type == "taskItem" {
			box := "[ ] "
			if state, _ := item.Attrs["state"].(string); state == "DONE" {
				box = "[x] "
			}
			out = append(out, m+box+inline(item.Content))
			continue
		}
		body := blocks(item.Content, strings.Repeat(" ", len(m)))
		// Items are tight: drop the blank lines between an item's blocks
		// when every block is a single paragraph or a nested list.
		body = strings.ReplaceAll(body, "\n\n", "\n")
		if len(body) >= len(m) {
			body = m + body[len(m):]
		}
		out = append(out, body)
	}
	return strings.Join(out, "\n")
}

func table(n node) string {
	var rows []string
	for i, row := range n.Content {
		cells := make([]string, len(row.Content))
		for j, cell := range row.Content {
			text := blocks(cell.Content, "")
			text = strings.ReplaceAll(text, "\n\n", " ")
			text = strings.ReplaceAll(text, "\n", " ")
			cells[j] = strings.ReplaceAll(text, "|", "\\|")
		}
		rows = append(rows, "| "+strings.Join(cells, " | ")+" |")
		if i == 0 {
			sep := make([]string, len(cells))
			for j := range sep {
				sep[j] = "---"
			}
			rows = append(rows, "| "+strings.Join(sep, " | ")+" |")
		}
	}
	return strings.Join(rows, "\n")
}

// inline renders inline nodes on one logical line.
func inline(nodes []node) string {
	var b strings.Builder
	for _, n := range nodes {
		switch n.Type {
		case "text":
			b.WriteString(marked(n))
		case "hardBreak":
			b.WriteString("  \n")
		case "mention", "status", "emoji":
			if text, _ := n.Attrs["text"].(string); text != "" {
				b.WriteString(text)
			} else if name, _ := n.Attrs["shortName"].(string); name != "" {
				b.WriteString(name)
			}
		case "date":
			if ts, _ := n.Attrs["timestamp"].(string); ts != "" {
				b.WriteString(ts)
			}
		case "inlineCard":
			if url, _ := n.Attrs["url"].(string); url != "" {
				b.WriteString("<" + url + ">")
			}
		case "media", "mediaInline":
			b.WriteString("*[attachment]*")
		default:
			b.WriteString(inline(n.Content))
		}
	}
	return b.String()
}

// marked wraps a text node in its marks.
func marked(n node) string {
	text := n.Text
	if strings.TrimSpace(text) == "" {
		return text
	}
	// Marks wrap the words; spaces at either end stay outside, or the
	// Markdown would not parse.
	lead := text[:len(text)-len(strings.TrimLeft(text, " "))]
	trail := text[len(strings.TrimRight(text, " ")):]
	core := strings.TrimSpace(text)
	link := ""
	for _, m := range n.Marks {
		switch m.Type {
		case "code":
			core = "`" + core + "`"
		case "strong":
			core = "**" + core + "**"
		case "em":
			core = "*" + core + "*"
		case "strike":
			core = "~~" + core + "~~"
		case "link":
			link, _ = m.Attrs["href"].(string)
		}
	}
	if link != "" {
		core = "[" + core + "](" + link + ")"
	}
	return lead + core + trail
}

// plain is the text of nodes with no marks applied, for code blocks.
func plain(nodes []node) string {
	var b strings.Builder
	for _, n := range nodes {
		switch n.Type {
		case "text":
			b.WriteString(n.Text)
		case "hardBreak":
			b.WriteString("\n")
		default:
			b.WriteString(plain(n.Content))
		}
	}
	return b.String()
}
