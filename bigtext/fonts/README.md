# Font files

Each file is a FIGlet font and keeps the comment block its author wrote. The files
were copied from the `cmatsuoka/figlet-fonts` and `xero/figlet-fonts` collections on
GitHub.

| File | Author | Terms stated in the file |
|---|---|---|
| `twopoint.flf` | Bruce Jakeway, 1994 | None. Distributed with the FIGlet contributed fonts. |
| `cybersmall.flf` | Lennert Stock; FIGlet conversion by Kent Nassen, 1994 | None. Distributed with the FIGlet contributed fonts. |
| `smbraille.flf` | Sam Hocevar, 2006 | WTFPL version 2. Converted from `smbraille.tlf` by changing the first five characters of the header to `flf2a`. |
| `calvins.flf` | Unknown; converted from a TheDraw font | None. |
| `bigfig.flf` | Glenn Chappell, 1993 | Modification permitted if the modifier is named in a comment line. |

A new font must be at most three rows tall; the test in `bigtext` checks this.
