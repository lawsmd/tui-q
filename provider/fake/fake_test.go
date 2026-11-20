package fake

import (
	"context"
	"errors"
	"testing"

	"github.com/lawsmd/tui-q/provider"
)

func TestSearch(t *testing.T) {
	p := New(0)
	ctx := context.Background()
	cases := map[string]int{
		"":                          12,
		"status=Open":               5,
		"status=open and assignee=": 4,
		"assignee=*":                8,
		"status=In progress":        3,
	}
	for query, want := range cases {
		got, err := p.Search(ctx, query)
		if err != nil {
			t.Fatalf("%q: %v", query, err)
		}
		if len(got) != want {
			t.Errorf("%q: %d tickets, want %d", query, len(got), want)
		}
	}
	if _, err := p.Search(ctx, "nonsense"); err == nil {
		t.Error("a malformed query returned no error")
	}
}

func TestTransitionChangesStatus(t *testing.T) {
	p := New(0)
	ctx := context.Background()
	trs, err := p.Transitions(ctx, "DEMO-101")
	if err != nil {
		t.Fatal(err)
	}
	if len(trs) != len(Statuses)-1 {
		t.Fatalf("%d transitions, want %d", len(trs), len(Statuses)-1)
	}
	if err := p.Transition(ctx, "DEMO-101", trs[0], nil); err != nil {
		t.Fatal(err)
	}
	got, err := p.Get(ctx, "DEMO-101")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != trs[0].To {
		t.Errorf("status = %q, want %q", got.Status, trs[0].To)
	}
}

func TestRegistered(t *testing.T) {
	if _, err := provider.New(provider.Config{Kind: Kind}); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.New(provider.Config{Kind: "nope"}); err == nil {
		t.Error("an unknown kind returned no error")
	}
}

func TestAddCommentAppendsAsTheFirstPerson(t *testing.T) {
	p := New(0)
	ctx := context.Background()
	before, err := p.Get(ctx, "DEMO-101")
	if err != nil {
		t.Fatal(err)
	}
	var c provider.Commenter = p
	if c.CommentFormat() != provider.CommentMarkdown {
		t.Fatalf("format = %q", c.CommentFormat())
	}
	if err := c.AddComment(ctx, "DEMO-101", "", "Looks **good**"); err != nil {
		t.Fatal(err)
	}
	got, err := p.Get(ctx, "DEMO-101")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Comments) != len(before.Comments)+1 {
		t.Fatalf("%d comments, want %d", len(got.Comments), len(before.Comments)+1)
	}
	if last := got.Comments[len(got.Comments)-1]; last.Body != "Looks **good**" || last.Author != People[0] || last.Created.IsZero() {
		t.Errorf("the new comment = %+v", last)
	}
	if len(before.Comments) > 0 && before.Comments[len(before.Comments)-1].Body == "Looks **good**" {
		t.Error("the ticket read earlier changed with the write")
	}

	p.FailWrites = errors.New("refused")
	if err := c.AddComment(ctx, "DEMO-101", "", "again"); err == nil {
		t.Error("a write that should fail added a comment")
	}
	if err := New(0).AddComment(ctx, "DEMO-999", "", "nobody home"); err == nil {
		t.Error("a comment on a missing ticket was accepted")
	}
}
