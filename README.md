# tui-q

A terminal UI for working ticket queues with the mouse. The command is `tq`.

Queues are tabs across the top. Clicking a ticket opens it in a second row of tabs. A
status bar at the foot of the pane, under a rule, shows what the program is doing. While
it waits on a request the user asked for (a ticket opening, a queue being read again, a
change being written to the platform), the view is drawn grey and input is locked until
the request ends.

It is built on [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Lip Gloss](https://github.com/charmbracelet/lipgloss) and
[Bubbles](https://github.com/charmbracelet/bubbles).

## Status

Early. The shell, the table view, and a read-only ticket view work against an in-memory
demo provider. Platform providers, boards with drag and drop, and writes are planned; see
`docs/plan.md`.

## Comments

Every ticket view has a comment box under the heading of the comments, beside the newest.
It is a grey border named "Write a comment" around an editor, and the editor holds the
text of the comment and nothing else. `c` or a click in the box gives it the keyboard,
and it grows while it is written in. `ctrl+s`, `ctrl+enter` or `send` on the border
posts the comment, `esc` gives the keyboard back to the ticket, and what was typed is
kept as a draft until it is sent or discarded.

Each comment already on the ticket has `copy` at the right end of its top border. A
click puts the text of that comment on the clipboard as Markdown.

On a platform with more than one kind of comment, the profile can turn on a choice:

```toml
[provider]
comment_kinds = ["internal", "public"]
```

The box then shows the kinds on its border, after its name, and starts on the first. A click on a kind, or
`ctrl+k` while writing, picks the kind the comment is posted as. Jira has `public` and
`internal`, which a service project shows as a reply to the customer and an internal
note. Azure DevOps has one kind and takes the comment as Markdown.

Typing `@` and part of a name lists people to mention. People mentioned in a comment sent
before are listed first, from a file kept per site in the user's configuration directory
(`tui-q/people`), and no request is made while one of them matches. When none matches,
Jira is searched for people who can see the ticket and Azure DevOps for the
organisation's users. To reach a person who is not yet in the file, type more of the
name.

The box is a [slate](https://github.com/lawsmd/slate) editor. It needs Microsoft Edge or
Chrome on the machine.

While the box is open, another program run by the same user can read and change the
draft with `slate ls`, `slate read` and `slate edit`. Set `SLATE_LINK=0` to turn that off.

### Terminals and multiplexers

slate draws the box as a Sixel image, so where the box works depends on what passes that
image to the screen.

| Where `tq` runs | Comment box |
|---|---|
| Windows Terminal 1.22 or later | works |
| Herdr, inside Windows Terminal | works; slate writes the image to the terminal behind Herdr |
| tmux, Zellij | does not open; they drop the image |

In Herdr, set this in `config.toml` and apply it with `herdr server reload-config`.
Without it the box blinks when the window comes back to the front:

```toml
[ui]
redraw_on_focus_gained = false
```

The rest of `tq` runs in any terminal that reports mouse events. slate's README says how
to add another multiplexer.

## Pictures

A picture that a ticket shows in its body or in a comment is a line with the name of the
file, `[image: name.png]`. A click on the line reads the file once, keeps it in the cache
directory, and opens it in the program the machine opens pictures with.

The profile can ask for the pictures to be drawn in the ticket as well. This is an
experiment:

```toml
[ticket]
pictures = true
```

A picture is then read when its line first comes into view and drawn under the line, no
wider than the column and at most 24 rows high. A larger picture is scaled down, and a
click on it opens the file at full size. The picture scrolls with the ticket. PNG, JPEG
and GIF files are drawn; any other picture keeps its line only.

slate draws a picture as it draws the comment box, as a Sixel image, so pictures are
drawn in the terminals where the box works. Elsewhere they keep their lines.

## Linked tickets

A ticket read from Jira carries its links to other issues. The profile can list them in
the sidebar of the ticket view:

```toml
[ticket]
links = true
```

The Links section shows each relation once, in Jira's words for this ticket's side of it
("blocks", "is blocked by"), and under it the tickets that have that relation: key and
status on one line, title on the next. A click on a key opens that ticket in a tab,
whichever project it is in. A ticket read from Azure DevOps lists no links yet.

## Try it

```
go run ./cmd/tq
```

| Input | Action |
|---|---|
| Click a queue tab | show that queue |
| Click a ticket | open it in a ticket tab |
| Click the key of a linked ticket | open that ticket in a ticket tab |
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

A queue with `recent = true` lists the tickets you acted on lately, the latest first, 20
to a page: the ones whose status, tags or assignee you changed, and the ones assigned to
you or taken from you. The demo's `Recent` tab is one. `docs/plan.md` gives the keys.

## `tq LINK`

```
tq https://example.com/browse/DEMO-104
```

`tq` followed by the link, key or number of a ticket starts on that ticket. It loads no
queue, which makes it quick to start on a site whose queues are slow. The ticket view is
the usual one: the ticket can be moved, assigned, edited and reloaded.

The top row holds a tab for each open ticket and the `go to ticket` box. A linked ticket
that is clicked, or a ticket named in the box, opens in a tab beside the first. `ctrl+w`
and the `x` on a tab close a tab while another is open, and the last tab stays. There are
no queue tabs, and `esc`, `tab` and `shift+tab` do nothing. `q` quits.

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

## `tq tabs`

`tq tabs` prints the tickets open in the tabs of every `tq` running on the profile's site,
without starting the interface. It is for an LLM agent asked about the tickets on screen,
for example to draft a comment on one ticket from what another says. A ticket is printed
as its key, status and title, and each ticket it is linked to as the relation, the key and
the status. The command reads files that the running interfaces keep in the cache
directory. It asks the platform nothing and needs no credentials.

```
Tickets open in tq
Each block is one running tq, named by its process ID, with the tickets open in its
tabs. A * marks the ticket on screen. Under a ticket are the tickets it is linked to.
The comment box of the ticket on screen is the editor that "slate ls" lists with the
same process ID and ticket: read and change its draft with "slate read" and "slate edit".
This list gives keys, statuses and titles only. Read ticket details through your own
access to the ticketing platform, not through tq.

tq 50084
  1. DEMO-104  [In progress]  Webhook retries stop after the first failure
       blocks      DEMO-102  [Open]
       blocks      DEMO-107  [Waiting]
       relates to  DEMO-110  [In progress]
* 2. DEMO-102  [Open]  Export to CSV drops rows with commas in the name
       is blocked by  DEMO-104  [In progress]
```

A ticket's links are listed once `tq` has read the ticket in full, whether or not the
profile shows them in the sidebar. `slate ls` prints the same process ID beside the
comment box of the ticket on screen, which is how an agent finds the box to write in.

To tell an agent about the command, put this in the instructions it reads on every run:

> Run `tq tabs` to see the tickets I have open in tq and the links between them. The
> ticket marked `*` is on screen, and `slate ls` lists its comment box under the same
> process ID. Read ticket details through your access to the ticketing platform. Write a
> comment for me with `slate edit`. I send it myself.

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
