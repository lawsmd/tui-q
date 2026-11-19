# tui-q

A terminal UI for working ticket queues with the mouse. The command is `tq`.

Queues are tabs across the top. Clicking a ticket opens it in a second row of tabs. A
status bar on the last line shows what the program is doing; while a change is being
written to the platform, input is locked until it finishes.

It is built on [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Lip Gloss](https://github.com/charmbracelet/lipgloss) and
[Bubbles](https://github.com/charmbracelet/bubbles).

## Status

Early. The shell, the table view, and a read-only ticket view work against an in-memory
demo provider. Platform providers, boards with drag and drop, and writes are planned; see
`docs/plan.md`.

## Try it

```
go run ./cmd/tq
```

| Input | Action |
|---|---|
| Click a queue tab | show that queue |
| Click a ticket | open it in a ticket tab |
| Click `x` on a ticket tab | close it |
| Mouse wheel | scroll |
| `tab`, `shift+tab` | next and previous queue |
| `esc` | back to the queue |
| `ctrl+w` | close the ticket tab |
| `r` | reload |
| Click `go to ticket`, or `/` | type or paste a ticket number, key or link; `enter` opens it |
| `o` | open the ticket in the browser |
| `q` | quit |

## Profiles

A profile is a TOML file naming the platform, the queues, and the fields a ticket shows.
`cmd/tq/profile.toml` is the demo. Set `TQ_PROFILE` to the path of another file to load it
in place of the built-in one.

## Instance modules

Anything specific to one organisation stays out of this repository. Each installation has
its own small Go module that imports this one, embeds its own profile, and calls
`tuiq.Run`:

```go
package main

import (
	_ "embed"
	"log"

	"github.com/lawsmd/tui-q/tuiq"
)

//go:embed profile.toml
var profile string

func main() {
	if err := tuiq.Run(tuiq.WithProfile(profile)); err != nil {
		log.Fatal(err)
	}
}
```

## Development

```
git config core.hooksPath .githooks
go test ./...
```

The hooks run `scripts/check-names`, which refuses a commit that matches a private
denylist. See `CLAUDE.md`.
