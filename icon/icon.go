// Package icon is the set of named one-cell glyphs a profile can map field
// values to.
//
// Every glyph is in the ranges a monospace font draws itself: the symbols
// of code page 437, the geometric shapes, the box and block pieces, and
// ASCII. A character that also has an emoji form is kept out, since a
// terminal may draw it with an emoji font.
package icon

import "sort"

// Other is drawn for a value that has no icon of its own.
const Other = "·"

var set = map[string]string{
	// Named by use.
	"bug":      "¤",
	"gear":     "☼",
	"data":     "≡",
	"swap":     "↔",
	"home":     "⌂",
	"mail":     "@",
	"question": "?",
	"alert":    "‼",
	"warn":     "!",
	"plug":     "╪",
	"plug2":    "╫",
	"link":     "∞",
	"note":     "♪",
	"section":  "§",
	"para":     "¶",
	"money":    "$",
	"tag":      "#",
	"percent":  "%",
	"and":      "&",
	"plus":     "+",
	"minus":    "−",
	"times":    "×",
	"check":    "√",
	"sum":      "∑",
	"about":    "≈",
	"differ":   "≠",
	"change":   "±",
	"wave":     "~",
	"asterisk": "*",
	"flag":     "▌",
	"play":     "►",
	"back":     "◄",
	"user":     "☺",
	"user2":    "☻",
	"heart":    "♥",
	"spade":    "♠",
	"club":     "♣",

	// Named by shape.
	"up":          "↑",
	"down":        "↓",
	"left":        "←",
	"right":       "→",
	"updown":      "↕",
	"tri-up":      "▲",
	"tri-down":    "▼",
	"tri-up-o":    "△",
	"tri-down-o":  "▽",
	"tri-right":   "▶",
	"tri-right-o": "▷",
	"square":      "■",
	"square-o":    "□",
	"dot-square":  "▪",
	"circle":      "●",
	"circle-o":    "○",
	"half":        "◐",
	"half2":       "◑",
	"diamond":     "◆",
	"diamond-o":   "◇",
	"lozenge":     "◊",
	"star":        "★",
	"star-o":      "☆",
	"cross":       "╬",
	"grid":        "┼",
	"light":       "░",
	"medium":      "▒",
	"dark":        "▓",
	"block":       "█",
	"bar":         "▬",
	"delta":       "∆",
	"omega":       "Ω",
	"pi":          "π",
	"mu":          "µ",
	"lambda":      "λ",
	"sigma":       "σ",
	"phi":         "φ",
}

// Glyph returns the glyph of a named icon.
func Glyph(name string) (string, bool) {
	g, ok := set[name]
	return g, ok
}

// Names lists the icons in alphabetical order.
func Names() []string {
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
