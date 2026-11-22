package editor

import (
	"errors"
	"fmt"
	"image"
	_ "image/gif" // the kinds of picture OpenPicture reads
	_ "image/jpeg"
	_ "image/png"
	"os"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/lawsmd/slate/bubble"
)

// MaxPixels is the most pixels a picture may have to be drawn. A larger one
// would take too long to scale for a place in a ticket.
const MaxPixels = 64 << 20

// ErrNoPictures is the error of OpenPicture where the terminal cannot show
// any picture, as under a multiplexer that drops images. Another file would
// fail in the same way.
var ErrNoPictures = errors.New("the terminal shows no pictures")

// Rect is a rectangle of cells of the window, counted from 0.
type Rect struct{ X, Y, W, H int }

// Picture is a still picture drawn over a rectangle of the terminal, as the
// box is. The model that holds it draws blank cells where it goes.
type Picture interface {
	// Init starts the picture. Its command is returned from the model's
	// Update.
	Init() tea.Cmd
	// Update takes every message the model does not use itself.
	Update(tea.Msg) tea.Cmd
	// Fit is the cells the picture takes, scaled down to fit in cols by
	// rows. The answer can change after any message.
	Fit(cols, rows int) (w, h int)
	// Show places the picture in the cells whole, which may reach outside
	// the window, and draws the part of it inside view.
	Show(whole, view Rect) tea.Cmd
	// Cover stops the picture drawing while the model shows something over
	// it.
	Cover(on bool) tea.Cmd
	// Close removes the picture. It can take a moment, and is safe to call
	// from a goroutine of its own.
	Close()
}

// OpenPicture reads the picture in a file and makes it ready to draw on the
// terminal the program runs in. background is the colour behind it, as
// "#rrggbb". It reads PNG, JPEG and GIF.
func OpenPicture(path, background string) (Picture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, fmt.Errorf("the picture could not be read: %w", err)
	}
	if cfg.Width*cfg.Height > MaxPixels {
		return nil, fmt.Errorf("the picture is too large to draw: %d by %d pixels", cfg.Width, cfg.Height)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("the picture could not be read: %w", err)
	}
	p, err := bubble.OpenPicture(img, bubble.PictureConfig{Background: background})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoPictures, err)
	}
	return &picture{p: p}, nil
}

// picture is a slate picture block.
type picture struct {
	p    *bubble.Picture
	once sync.Once
}

func (p *picture) Init() tea.Cmd                 { return p.p.Init() }
func (p *picture) Update(msg tea.Msg) tea.Cmd    { return p.p.Update(msg) }
func (p *picture) Fit(cols, rows int) (w, h int) { return p.p.Fit(cols, rows) }
func (p *picture) Cover(on bool) tea.Cmd         { return p.p.Cover(on) }
func (p *picture) Close()                        { p.once.Do(p.p.Close) }

func (p *picture) Show(whole, view Rect) tea.Cmd {
	return p.p.Show(bubble.Rect(whole), bubble.Rect(view))
}
