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

## Comments

Every ticket view has a comment box under the heading of the comments, beside the newest,
drawn on a grey of its own. `c` or a click in the box gives it the keyboard, and it grows
while it is written in. `ctrl+s` or `send`
posts the comment, `esc` gives the keyboard back to the ticket, and what was typed is
kept as a draft until it is sent or discarded.

On a platform with more than one kind of comment, the profile can turn on a choice:

```toml
[provider]
comment_kinds = ["internal", "public"]
```

The box then shows the kinds on its line and starts on the first. A click on a kind, or
`ctrl+k` while writing, picks the kind the comment is posted as. Jira has `public` and
`internal`, which a service project shows as a reply to the customer and an internal
note. Azure DevOps has one kind, takes the comment as Markdown, and offers the
organisation's users after `@`.

The box is a [slate](https://github.com/lawsmd/slate) editor. It needs Microsoft Edge or
Chrome on the machine, and a terminal that shows Sixel images, such as Windows Terminal
1.22 or later. It does not open under tmux or Zellij, which drop the image.

While the box is open, another program run by the same user can read and change the
draft with `slate ls`, `slate read` and `slate edit`. Set `SLATE_LINK=0` to turn that off.

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
| Click `‹` or `›` on a bucket heading | previous or next page of the bucket |
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

## `tq LINK`

```
tq https://example.com/browse/DEMO-104
```

`tq` followed by the link, key or number of a ticket shows that ticket and nothing else.
It loads no queue, which makes it quick to start on a site whose queues are slow. The
ticket view is the usual one: the ticket can be moved, assigned, edited and reloaded.
The queue tabs, the ticket tabs and the `go to ticket` box are left out, and `esc`,
`tab`, `/` and `ctrl+w` do nothing. `q` quits.

## `tq order`

`tq order` prints the tickets of each queue in the order the interface shows them, without
starting the interface. It is for an LLM agent that already has its own access to the
ticketing platform and needs to know what to work on next. A ticket is printed as its key,
status and title. The command never loads a ticket in full and never writes.

```
Working order from tq (fake)
Queues are listed in priority order, then buckets, then tickets. Number 1 is next.
This list gives order only. Read ticket details and make changes through your own
access to the ticketing platform, not through tq.

Queue 1: Unassigned (table, 4 tickets)
  1. DEMO-101  [Open]  Login page returns 500 after password reset
  2. DEMO-102  [Open]  Export to CSV drops rows with commas in the name
```

| Flag | Effect |
|---|---|
| `--queue NAME` | print one queue |
| `--limit N` | the most tickets printed for one bucket or lane, 20 unless given |

Queues are printed in profile order, and a table's buckets and a board's lanes in theirs.
Set `order = false` on a `[[queue]]` to leave it out, as the demo does for its overview
queue.

To tell an agent about the command, put this in the instructions it reads on every run
(for Claude Code, `~/.claude/CLAUDE.md`):

> Run `tq order` to see my tickets in the order I mean to work on them. Use it for order
> only. Read ticket details and make changes through your access to the ticketing
> platform.

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
