// Package tuiq is the entry point of tui-q. The stock binary and every
// instance module call Run; an instance passes options to change what the
// program does.
package tuiq

import (
	"errors"
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

// Run starts the interface and returns when the user quits. When the
// program's first argument is "check", Run reads every queue once, prints
// the result and returns without starting the interface.
func Run(opts ...Option) error {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
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
	if len(os.Args) > 1 && os.Args[1] == "check" {
		return check(os.Stdout, prof, prov)
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
