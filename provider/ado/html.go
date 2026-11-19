package ado

import (
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// HTMLToMarkdown converts the HTML of a description or comment into
// Markdown. It keeps paragraphs, headings, emphasis, code, links and lists;
// it reduces a table to one line per row and drops images to their alt text.
func HTMLToMarkdown(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return s
	}
	var w mdWriter
	w.node(doc)
	return strings.TrimSpace(blankRuns.ReplaceAllString(w.b.String(), "\n\n"))
}

var blankRuns = regexp.MustCompile(`\n[ \t]*(\n[ \t]*)+`)

type mdWriter struct {
	b strings.Builder
	// lists holds, for each open list, its next number, or -1 for bullets.
	lists []int
	pre   bool
}

func (w *mdWriter) block() { w.b.WriteString("\n\n") }

func (w *mdWriter) children(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		w.node(c)
	}
}

func (w *mdWriter) wrap(n *html.Node, mark string) {
	var inner mdWriter
	inner.lists = w.lists
	inner.children(n)
	text := strings.TrimSpace(inner.b.String())
	if text == "" {
		return
	}
	w.b.WriteString(mark + text + mark)
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func (w *mdWriter) node(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		if w.pre {
			w.b.WriteString(n.Data)
			return
		}
		w.b.WriteString(spaces.ReplaceAllString(n.Data, " "))
		return
	case html.DocumentNode:
		w.children(n)
		return
	case html.ElementNode:
	default:
		return
	}
	switch n.Data {
	case "script", "style", "head":
	case "p", "div", "section", "article":
		w.block()
		w.children(n)
		w.block()
	case "br":
		w.b.WriteString("  \n")
	case "hr":
		w.block()
		w.b.WriteString("---")
		w.block()
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level, _ := strconv.Atoi(n.Data[1:])
		var inner mdWriter
		inner.children(n)
		w.block()
		w.b.WriteString(strings.Repeat("#", level) + " " + strings.Join(strings.Fields(inner.b.String()), " "))
		w.block()
	case "b", "strong":
		w.wrap(n, "**")
	case "i", "em":
		w.wrap(n, "*")
	case "code":
		if w.pre {
			w.children(n)
			return
		}
		w.wrap(n, "`")
	case "pre":
		w.block()
		w.b.WriteString("```\n")
		w.pre = true
		w.children(n)
		w.pre = false
		w.b.WriteString("\n```")
		w.block()
	case "a":
		href := attr(n, "href")
		var inner mdWriter
		inner.children(n)
		text := strings.TrimSpace(inner.b.String())
		switch {
		case href == "" || strings.HasPrefix(href, "javascript:"):
			w.b.WriteString(text)
		case text == "" || text == href:
			w.b.WriteString("<" + href + ">")
		default:
			w.b.WriteString("[" + text + "](" + href + ")")
		}
	case "img":
		if alt := attr(n, "alt"); alt != "" {
			w.b.WriteString("[image: " + alt + "]")
		} else {
			w.b.WriteString("[image]")
		}
	case "ul", "ol":
		start := -1
		if n.Data == "ol" {
			start = 1
		}
		if len(w.lists) == 0 {
			w.block()
		}
		w.lists = append(w.lists, start)
		w.children(n)
		w.lists = w.lists[:len(w.lists)-1]
		if len(w.lists) == 0 {
			w.block()
		}
	case "li":
		depth := len(w.lists)
		marker := "- "
		if depth > 0 && w.lists[depth-1] > 0 {
			marker = strconv.Itoa(w.lists[depth-1]) + ". "
			w.lists[depth-1]++
		}
		var inner mdWriter
		inner.lists = w.lists
		inner.children(n)
		text := strings.TrimSpace(blankRuns.ReplaceAllString(inner.b.String(), "\n"))
		// A nested list indents itself under its item: each item indents
		// only its own continuation lines.
		w.b.WriteString("\n" + marker + strings.ReplaceAll(text, "\n", "\n  "))
	case "tr":
		var cells []string
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
				var inner mdWriter
				inner.children(c)
				cells = append(cells, strings.Join(strings.Fields(inner.b.String()), " "))
			}
		}
		w.b.WriteString("\n" + strings.Join(cells, " | ") + "  ")
	case "table":
		w.block()
		w.children(n)
		w.block()
	default:
		w.children(n)
	}
}

var spaces = regexp.MustCompile(`\s+`)
