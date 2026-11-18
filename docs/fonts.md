# Heading fonts

A queue with buckets can set `heading_font = "block"`. Package `bigtext` then draws
the name of each bucket in solid capitals three rows tall. The shapes are in
`bigtext/bigtext.go`: each is a grid of dots six high, and the half-block characters
`▀`, `▄` and `█` put two dots in one cell.

The font has the letters, the digits, the space and `- / . &`. A lower-case letter is
drawn as its capital. A name with any other character keeps the one-line heading.

```text
▀█▀ █▄ █    █▀▀▄ █▀▀▄ ▄▀▀▄ ▄▀▀▀ █▀▀▄ █▀▀▀ ▄▀▀▀ ▄▀▀▀
 █  █▀▄█    █▄▄▀ █▄▄▀ █  █ █ ▄▄ █▄▄▀ █▀▀   ▀▀▄  ▀▀▄
▄█▄ █ ▀█    █    █ ▀▄ ▀▄▄▀ ▀▄▄█ █ ▀▄ █▄▄▄ ▄▄▄▀ ▄▄▄▀
```
