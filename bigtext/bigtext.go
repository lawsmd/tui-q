// Package bigtext draws a short text in a FIGlet font, for headings that
// should stand out. The fonts are in the fonts directory and are at most
// MaxHeight rows tall.
package bigtext

import (
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/lukesampson/figlet/figletlib"
)

// MaxHeight is the most rows a font may have.
const MaxHeight = 3

// maxWidth is wide enough that the layout never wraps a heading.
const maxWidth = 1000

//go:embed fonts/*.flf
var files embed.FS

var (
	mu     sync.Mutex
	parsed = map[string]*figletlib.Font{}
)

// Fonts lists the names of the fonts in order.
func Fonts() []string {
	entries, _ := files.ReadDir("fonts")
	var names []string
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), path.Ext(e.Name())))
	}
	sort.Strings(names)
	return names
}

// Has reports whether there is a font with the name.
func Has(font string) bool {
	_, err := files.ReadFile("fonts/" + font + ".flf")
	return err == nil
}

func load(font string) (f *figletlib.Font, err error) {
	mu.Lock()
	defer mu.Unlock()
	if f, ok := parsed[font]; ok {
		return f, nil
	}
	data, err := files.ReadFile("fonts/" + font + ".flf")
	if err != nil {
		return nil, fmt.Errorf("bigtext: no font named %q", font)
	}
	// The parser indexes past the end of a file that is cut short.
	defer func() {
		if r := recover(); r != nil {
			f, err = nil, fmt.Errorf("bigtext: font %q: %v", font, r)
		}
	}()
	f, err = figletlib.ReadFontFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("bigtext: font %q: %w", font, err)
	}
	parsed[font] = f
	return f, nil
}

// Render draws the text in the font. The rows it returns have the same
// width, and blank rows at the bottom are left out. It fails when the
// font is unknown or draws nothing for the text.
func Render(font, text string) ([]string, error) {
	f, err := load(font)
	if err != nil {
		return nil, err
	}
	s := f.Settings()
	lines := figletlib.GetLines(text, f, maxWidth, s)
	if len(lines) != 1 {
		return nil, fmt.Errorf("bigtext: %q does not fit on one line", text)
	}
	var rows []string
	width := 0
	for _, r := range lines[0].Art() {
		row := strings.TrimRight(strings.ReplaceAll(string(r), string(s.HardBlank()), " "), " \r")
		rows = append(rows, row)
		width = max(width, lipgloss.Width(row))
	}
	for len(rows) > 0 && rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
	}
	if width == 0 || len(rows) > MaxHeight {
		return nil, fmt.Errorf("bigtext: font %q cannot draw %q", font, text)
	}
	for i, row := range rows {
		rows[i] = row + strings.Repeat(" ", width-lipgloss.Width(row))
	}
	return rows, nil
}
