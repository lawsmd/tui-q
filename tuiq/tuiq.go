// Package tuiq is the entry point of tui-q. The stock binary and every
// instance module call Run; an instance passes options to change what the
// program does.
package tuiq

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/internal/ui"
	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider"

	// Platforms available to every build.
	_ "github.com/lawsmd/tui-q/provider/fake"
	_ "github.com/lawsmd/tui-q/provider/jira"
)

type config struct {
	profileText string
}

// Option changes how Run behaves.
type Option func(*config)

// WithProfile gives the profile as TOML text, usually embedded in the
// binary with go:embed. The TQ_PROFILE environment variable, when set,
// names a file that is loaded in its place.
func WithProfile(text string) Option {
	return func(c *config) { c.profileText = text }
}

// Run starts the interface and returns when the user quits.
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
	_, err = tea.NewProgram(ui.New(prof, prov)).Run()
	return err
}
