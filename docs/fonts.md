# Heading fonts

A queue with buckets can set `heading_font` to one of these names. Package `bigtext`
draws the text with the FIGlet font files in `bigtext/fonts`, each at most three rows
tall. `bigtext/fonts/README.md` gives the origin and terms of each file.

## `smbraille`

Braille dots, two rows. Lacks `# $ % & * + , : ; < > ?`.

```text
 ⡇ ⣀⡀   ⣏⡱ ⡀⣀ ⢀⡀ ⢀⡀ ⡀⣀ ⢀⡀ ⢀⣀ ⢀⣀
 ⠇ ⠇⠸   ⠇  ⠏  ⠣⠜ ⣑⡺ ⠏  ⠣⠭ ⠭⠕ ⠭⠕
```

## `twopoint`

ASCII, two rows.

```text
|._   |~)._ _ (~|._ _ _ _
|| |  |~ | (_) _|| }__\_\
```

## `cybersmall`

ASCII, two rows, capitals only, no digits.

```text
 _ __ _   ___  ____ ____ ____ ____ ____ ____ ____
 | | \|   |--' |--< [__] |__, |--< |=== ==== ====
```

## `calvins`

Box drawing characters, three rows.

```text
╦┌┐┌  ╔═╗┬─┐┌─┐┌─┐┬─┐┌─┐┌─┐┌─┐
║│││  ╠═╝├┬┘│ ││ ┬├┬┘├┤ └─┐└─┐
╩┘└┘  ╩  ┴└─└─┘└─┘┴└─└─┘└─┘└─┘
```

## `bigfig`

ASCII, three rows.

```text
___       _        _
 | __    |_) __ _ (_| __ _  _  _
_|_| |   |   | (_)__| | (/__> _>
```
