// Package tuiq is the entry point of tui-q. The stock binary and every
// instance module call Run; an instance passes options to change what the
// program does.
package tuiq

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/ext"
	"github.com/lawsmd/tui-q/internal/attach"
	"github.com/lawsmd/tui-q/internal/drafts"
	"github.com/lawsmd/tui-q/internal/editor"
	"github.com/lawsmd/tui-q/internal/people"
	"github.com/lawsmd/tui-q/internal/prefs"
	"github.com/lawsmd/tui-q/internal/ui"
	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider"

	// Platforms available to every build.
	_ "github.com/lawsmd/tui-q/provider/ado"
	_ "github.com/lawsmd/tui-q/provider/fake"
	_ "github.com/lawsmd/tui-q/provider/jira"
)

type config struct {
	profileText string
	actions     []ext.Action
}

// Option changes how Run behaves.
type Option func(*config)

// WithProfile gives the profile as TOML text, usually embedded in the
// binary with go:embed. The TQ_PROFILE environment variable, when set,
// names a file that is loaded in its place.
func WithProfile(text string) Option {
	return func(c *config) { c.profileText = text }
}

// WithActions adds the instance's own actions to every ticket: a button in
// the ticket view and a line in the context menu for each.
func WithActions(actions ...ext.Action) Option {
	return func(c *config) { c.actions = append(c.actions, actions...) }
}

// usage is printed by "tq help" and with the error for an unknown command.
const usage = `usage:
  tq                                  start the interface
  tq LINK                             show one ticket, named by a link, key or number
  tq order [--queue NAME] [--limit N] print the tickets of each queue in working order
  tq check                            read every queue once and print what came back
  tq help                             print this
`

// Run starts the interface and returns when the user quits. The program's
// first argument can name a command that runs without the interface:
// "order" prints the working order, "check" tests a profile, and "help"
// prints the usage. Any other argument has to name a ticket, by link, key
// or number: the interface then shows that ticket alone and loads no
// queue. An argument that names neither is an error, so that a mistyped
// command does not start a full-screen program.
func Run(opts ...Option) error {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}
	command, args := "", []string(nil)
	if len(os.Args) > 1 {
		command, args = os.Args[1], os.Args[2:]
	}
	switch command {
	case "help", "-h", "-help", "--help":
		fmt.Print(usage)
		return nil
	}
	if cfg.profileText == "" {
		return errors.New("tuiq: no profile given; pass WithProfile")
	}
	prof, err := profile.Load(cfg.profileText)
	if err != nil {
		return err
	}
	prov, err := provider.New(prof.Provider)
	if err != nil {
		return err
	}
	switch command {
	case "check":
		return check(os.Stdout, prof, prov)
	case "order":
		return order(os.Stdout, prof, prov, args)
	}
	// Settings changed in the interface are kept per site. Without a place
	// to keep them they last until the program exits.
	uiOpts := ui.Options{Actions: cfg.actions}
	if command != "" {
		id, err := ticketArg(prov, command, args)
		if err != nil {
			fmt.Fprint(os.Stderr, usage)
			return err
		}
		uiOpts.Ticket = id
	}
	site := prof.Provider.Kind + " " + prof.Provider.BaseURL
	if path, err := prefs.Path(site); err == nil {
		uiOpts.Prefs = prefs.Open(path)
	}
	if dir, err := drafts.Dir(site); err == nil {
		uiOpts.Drafts = drafts.Open(dir)
	}
	if path, err := people.Path(site); err == nil {
		uiOpts.People = people.Open(path)
	}
	if dir, err := attach.Dir(site); err == nil {
		uiOpts.Files = attach.Open(dir)
	}
	// The comment box is an editor drawn by a browser, which is started
	// when the first ticket is shown and stopped here.
	uiOpts.OpenEditor, uiOpts.WarmEditor = editor.Open, editor.Warm
	uiOpts.OpenPicture = editor.OpenPicture
	defer editor.Shutdown()
	model := ui.New(prof, prov, uiOpts)
	_, err = tea.NewProgram(model).Run()
	model.Close()
	return err
}

// ticketArg reads a first argument that is not a command as the link, key
// or number of a ticket, and returns the ticket's ID. Nothing may follow
// it on the command line.
func ticketArg(prov provider.Provider, arg string, rest []string) (string, error) {
	id, err := prov.Resolve(arg)
	if err != nil {
		return "", fmt.Errorf("no command named %q, and it names no ticket: %w", arg, err)
	}
	if len(rest) > 0 {
		return "", fmt.Errorf("unexpected argument %q after the ticket", rest[0])
	}
	return id, nil
}
