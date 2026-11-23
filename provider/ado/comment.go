package ado

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// CommentFormat is Markdown, which Azure DevOps stores as written.
func (p *Provider) CommentFormat() string { return provider.CommentMarkdown }

// CommentKinds lists none: Azure DevOps has one kind of comment.
func (p *Provider) CommentKinds() []provider.CommentKind { return nil }

// AddComment adds a Markdown comment to a work item. A mention as the
// comment box writes it, [@Name](mention:ID), is sent as @<ID>, the form
// Azure DevOps reads as a mention of the identity ID.
func (p *Provider) AddComment(ctx context.Context, id, kind, body string) error {
	if _, err := strconv.Atoi(id); err != nil {
		return fmt.Errorf("ado: %q is not a work item number", id)
	}
	if kind != "" {
		return fmt.Errorf("ado: azure devops has no comment kind %q", kind)
	}
	if strings.TrimSpace(body) == "" {
		return errors.New("ado: the comment is empty")
	}
	path := p.projectPath() + "/_apis/wit/workItems/" + id + "/comments?format=markdown&api-version=" + apiVersion + "-preview.4"
	return p.client.Do(ctx, http.MethodPost, path, map[string]string{"text": Mentions(body)}, nil)
}

// mentionLink is a mention as the comment box writes it in Markdown.
var mentionLink = regexp.MustCompile(`\[@[^\]\n]*\]\(mention:([^)\s]+)\)`)

// inlineCode is an inline code span, which keeps its text as written.
var inlineCode = regexp.MustCompile("`[^`\n]+`")

// Mentions rewrites each mention link in Markdown to @<ID>, outside fenced
// code blocks and inline code.
func Mentions(md string) string {
	lines := strings.Split(md, "\n")
	fence := ""
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]+" \t") == "" {
				fence = ""
			}
			continue
		}
		if f := fenceOf(trimmed); f != "" {
			fence = f
			continue
		}
		lines[i] = outsideCode(line)
	}
	return strings.Join(lines, "\n")
}

// fenceOf returns the run of three or more backticks or tildes that opens a
// fenced code block, or "".
func fenceOf(line string) string {
	for _, c := range "`~" {
		n := len(line) - len(strings.TrimLeft(line, string(c)))
		if n >= 3 {
			return line[:n]
		}
	}
	return ""
}

// outsideCode rewrites the mentions of one line outside its code spans.
func outsideCode(line string) string {
	var b strings.Builder
	last := 0
	for _, loc := range inlineCode.FindAllStringIndex(line, -1) {
		b.WriteString(rewriteMentions(line[last:loc[0]]))
		b.WriteString(line[loc[0]:loc[1]])
		last = loc[1]
	}
	b.WriteString(rewriteMentions(line[last:]))
	return b.String()
}

func rewriteMentions(s string) string {
	return mentionLink.ReplaceAllStringFunc(s, func(link string) string {
		raw := mentionLink.FindStringSubmatch(link)[1]
		id, err := url.PathUnescape(raw)
		if err != nil {
			id = raw
		}
		return "@<" + id + ">"
	})
}

// Mentionable finds the people of the organisation whose name or sign-in
// address matches query, for the mentions of the comment box. A user's ID
// is the identity ID a mention names, not the sign-in name that Users
// returns. People of the directory who have never been added to the
// organisation have no identity ID and are left out.
func (p *Provider) Mentionable(ctx context.Context, _ string, query string) ([]ticket.User, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	found, err := p.identities(ctx, query)
	if err != nil {
		return nil, err
	}
	var out []ticket.User
	for _, id := range found {
		if id.LocalID != "" {
			out = append(out, ticket.User{ID: id.LocalID, Name: id.DisplayName})
		}
	}
	return out, nil
}
