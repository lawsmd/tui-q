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
	"github.com/lawsmd/tui-q/internal/prefs"
	"github.com/lawsmd/tui-q/internal/ui"
	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider"

	// Platforms available to every build.
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
  tq order [--queue NAME] [--limit N] print the tickets of each queue in working order
  tq check                            read every queue once and print what came back
  tq help                             print this
`

// Run starts the interface and returns when the user quits. The program's
// first argument can name a command that runs without the interface:
// "order" prints the working order, "check" tests a profile, and "help"
// prints the usage. Any other argument is an error, so that a mistyped
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
	case "", "check", "order":
	case "help", "-h", "-help", "--help":
		fmt.Print(usage)
		return nil
	default:
		fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("no command named %q", command)
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
	if path, err := prefs.Path(prof.Provider.Kind + " " + prof.Provider.BaseURL); err == nil {
		uiOpts.Prefs = prefs.Open(path)
	}
	_, err = tea.NewProgram(ui.New(prof, prov, uiOpts)).Run()
	return err
}
