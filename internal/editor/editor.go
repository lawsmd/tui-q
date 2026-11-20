// Package editor is the box a comment is written in. It wraps the slate
// editor (github.com/lawsmd/slate), which draws a rich-text editor as an
// image over a rectangle of the terminal, and is the only package that
// imports it. The interface holds a Box, so its tests run with a stand-in
// and need no browser.
package editor

import (
	"context"
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/lawsmd/slate"
	"github.com/lawsmd/slate/bubble"

	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// Config describes the box that Open makes.
type Config struct {
	// Format is the form Export gives the text in: one of the provider's
	// Comment constants.
	Format string
	// Markdown is the text the box starts with, such as a draft.
	Markdown string
	// Title and Meta say what the text is to another program that reads
	// it through slate's link, such as "comment on DEMO-104".
	Title string
	Meta  map[string]string
	// Mentions finds the users offered after "@". Nil turns that off.
	Mentions func(ctx context.Context, query string) ([]ticket.User, error)
	// Background is the colour the box is drawn on, as "#rrggbb". Empty
	// means the terminal's own.
	Background string
}

// Changed is the message Changes gives when the text changed.
type Changed struct{}

// Failed is the message Changes gives when the editor could not carry out
// the call named by Op.
type Failed struct {
	Op  string
	Err error
}

// Box is an open editor. The model that holds it decides where it is, which
// keys and mouse events it gets, and when its text is saved or sent.
type Box interface {
	// Init starts the box. Its command is returned from the model's Update.
	Init() tea.Cmd
	// Update takes every message the model does not use itself, including
	// ones the model does not know: the box's own messages arrive that way.
	Update(tea.Msg) tea.Cmd
	// SetRect places the box at a cell of the window, counted from 0. The
	// model draws nothing else in those cells.
	SetRect(x, y, w, h int) tea.Cmd
	// Cover stops the box drawing while the model shows something over it.
	Cover(on bool) tea.Cmd
	// Contains reports whether a cell of the window is inside the box.
	Contains(x, y int) bool
	// Focus shows or hides the cursor.
	Focus(on bool)
	// Text is the Markdown as the editor last reported it. It does not
	// wait, so a key pressed a moment ago may be missing from it.
	Text() string
	// Export returns the text in the Format of the Config.
	Export(ctx context.Context) (string, error)
	// Changes waits for the next thing the editor reports and returns it
	// as a Changed or a Failed. It returns nil once the box is closed.
	Changes() tea.Cmd
	// Close removes the box. The text is lost unless the model kept it.
	Close()
}

// Warm starts the browser behind the editor ahead of the first Open, which
// then takes no time to speak of.
func Warm() { slate.Warm() }

// Shutdown stops what Warm started.
func Shutdown() { _ = slate.Shutdown() }

// Open makes a box for the terminal the program runs in. It fails where
// the terminal cannot show the image, and for a format the editor cannot
// write.
func Open(cfg Config) (Box, error) {
	switch cfg.Format {
	case provider.CommentMarkdown, provider.CommentJira:
	default:
		return nil, fmt.Errorf("the comment box cannot write the %q format", cfg.Format)
	}
	opts := slate.Options{Target: cfg.Format, Markdown: cfg.Markdown, Theme: slate.Theme{Background: cfg.Background}}
	if find := cfg.Mentions; find != nil {
		opts.Mentions = func(ctx context.Context, query string) ([]slate.Mention, error) {
			users, err := find(ctx, query)
			out := make([]slate.Mention, len(users))
			for i, u := range users {
				out[i] = slate.Mention{ID: u.ID, Name: u.Name}
			}
			return out, err
		}
	}
	m, err := bubble.Open(bubble.Config{Options: opts, Title: cfg.Title, Meta: cfg.Meta})
	if err != nil {
		return nil, err
	}
	return &box{m: m, format: cfg.Format, done: make(chan struct{})}, nil
}

// box is a slate block.
type box struct {
	m      *bubble.Model
	format string
	done   chan struct{}
	once   sync.Once
}

func (b *box) Init() tea.Cmd                  { return b.m.Init() }
func (b *box) Update(msg tea.Msg) tea.Cmd     { return b.m.Update(msg) }
func (b *box) SetRect(x, y, w, h int) tea.Cmd { return b.m.SetRect(x, y, w, h) }
func (b *box) Cover(on bool) tea.Cmd          { return b.m.Cover(on) }
func (b *box) Contains(x, y int) bool         { return b.m.Contains(x, y) }
func (b *box) Focus(on bool)                  { b.m.Editor().Focus(on) }

func (b *box) Text() string {
	text, _ := b.m.Editor().Text()
	return text
}

func (b *box) Export(ctx context.Context) (string, error) {
	out, err := b.m.Editor().Export(ctx, b.format)
	return out.Body, err
}

func (b *box) Changes() tea.Cmd {
	events := b.m.Editor().Events()
	return func() tea.Msg {
		for {
			select {
			case ev, ok := <-events:
				if !ok {
					return nil
				}
				switch ev := ev.(type) {
				case slate.Changed:
					return Changed{}
				case slate.Failed:
					return Failed{Op: ev.Op, Err: ev.Err}
				}
				// The editor restarted its page and kept the text.
			case <-b.done:
				return nil
			}
		}
	}
}

func (b *box) Close() {
	b.once.Do(func() {
		close(b.done)
		b.m.Close()
	})
}
