// Package bigtext draws a short text in large letters, for headings that
// should stand out. A letter is a grid of dots six high, and two dots
// share a cell of the terminal, so the text is three rows tall.
package bigtext

import (
	"fmt"
	"strings"
	"unicode"
)

// Height is the number of rows of a drawn text.
const Height = 3

// Block is the name of the one font: solid capitals.
const Block = "block"

// Fonts lists the names of the fonts.
func Fonts() []string { return []string{Block} }

// Has reports whether there is a font with the name.
func Has(font string) bool { return font == Block }

// Render draws the text in the font. The rows it returns have the same
// width. A lower-case letter is drawn as its capital. It fails when the
// font is unknown or has no shape for a character of the text.
func Render(font, text string) ([]string, error) {
	if !Has(font) {
		return nil, fmt.Errorf("bigtext: no font named %q", font)
	}
	if text == "" {
		return nil, fmt.Errorf("bigtext: no text to draw")
	}
	// dots holds the six lines of dots of the whole text.
	var dots [2 * Height]strings.Builder
	for i, r := range text {
		g, ok := glyphs[unicode.ToUpper(r)]
		if !ok {
			return nil, fmt.Errorf("bigtext: font %q has no %q", font, r)
		}
		for y := range dots {
			if i > 0 {
				dots[y].WriteByte('.')
			}
			dots[y].WriteString(g[y])
		}
	}
	rows := make([]string, Height)
	for row := range rows {
		top, bottom := dots[2*row].String(), dots[2*row+1].String()
		var b strings.Builder
		for x := range top {
			switch {
			case top[x] == '#' && bottom[x] == '#':
				b.WriteRune('█')
			case top[x] == '#':
				b.WriteRune('▀')
			case bottom[x] == '#':
				b.WriteRune('▄')
			default:
				b.WriteByte(' ')
			}
		}
		rows[row] = b.String()
	}
	return rows, nil
}

// glyphs maps a character to its six lines of dots, top to bottom. A '#'
// is a filled dot. The lines of one glyph have the same length.
var glyphs = map[rune][2 * Height]string{
	' ': {"..", "..", "..", "..", "..", ".."},
	'A': {".##.", "#..#", "#..#", "####", "#..#", "#..#"},
	'B': {"###.", "#..#", "###.", "#..#", "#..#", "###."},
	'C': {".###", "#...", "#...", "#...", "#...", ".###"},
	'D': {"###.", "#..#", "#..#", "#..#", "#..#", "###."},
	'E': {"####", "#...", "###.", "#...", "#...", "####"},
	'F': {"####", "#...", "###.", "#...", "#...", "#..."},
	'G': {".###", "#...", "#...", "#.##", "#..#", ".###"},
	'H': {"#..#", "#..#", "####", "#..#", "#..#", "#..#"},
	'I': {"###", ".#.", ".#.", ".#.", ".#.", "###"},
	'J': {"..##", "...#", "...#", "...#", "#..#", ".##."},
	'K': {"#..#", "#.#.", "##..", "##..", "#.#.", "#..#"},
	'L': {"#...", "#...", "#...", "#...", "#...", "####"},
	'M': {"#...#", "##.##", "#.#.#", "#...#", "#...#", "#...#"},
	'N': {"#..#", "##.#", "##.#", "#.##", "#.##", "#..#"},
	'O': {".##.", "#..#", "#..#", "#..#", "#..#", ".##."},
	'P': {"###.", "#..#", "#..#", "###.", "#...", "#..."},
	'Q': {".##.", "#..#", "#..#", "#..#", "#.#.", ".#.#"},
	'R': {"###.", "#..#", "#..#", "###.", "#.#.", "#..#"},
	'S': {".###", "#...", ".##.", "...#", "...#", "###."},
	'T': {"#####", "..#..", "..#..", "..#..", "..#..", "..#.."},
	'U': {"#..#", "#..#", "#..#", "#..#", "#..#", ".##."},
	'V': {"#...#", "#...#", "#...#", ".#.#.", ".#.#.", "..#.."},
	'W': {"#...#", "#...#", "#...#", "#.#.#", "##.##", "#...#"},
	'X': {"#..#", "#..#", ".##.", ".##.", "#..#", "#..#"},
	'Y': {"#...#", "#...#", ".#.#.", "..#..", "..#..", "..#.."},
	'Z': {"####", "...#", "..#.", ".#..", "#...", "####"},
	'0': {".##.", "#..#", "#.##", "##.#", "#..#", ".##."},
	'1': {".#.", "##.", ".#.", ".#.", ".#.", "###"},
	'2': {".##.", "#..#", "...#", "..#.", ".#..", "####"},
	'3': {"###.", "...#", ".##.", "...#", "...#", "###."},
	'4': {"#..#", "#..#", "#..#", "####", "...#", "...#"},
	'5': {"####", "#...", "###.", "...#", "...#", "###."},
	'6': {".##.", "#...", "###.", "#..#", "#..#", ".##."},
	'7': {"####", "...#", "..#.", "..#.", ".#..", ".#.."},
	'8': {".##.", "#..#", ".##.", "#..#", "#..#", ".##."},
	'9': {".##.", "#..#", "#..#", ".###", "...#", ".##."},
	'-': {"...", "...", "###", "...", "...", "..."},
	'/': {"...#", "...#", "..#.", ".#..", "#...", "#..."},
	'.': {".", ".", ".", ".", ".", "#"},
	'&': {".#..", "#.#.", ".#..", "#.#.", "#..#", ".##."},
}
