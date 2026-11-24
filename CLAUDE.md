# tui-q

A mouse-driven terminal UI for working ticket queues, built on the Charm stack
(Bubble Tea, Lip Gloss, Bubbles, huh, glamour). The command is `tq`.

This repository is the generic core. It supports ticketing platforms (Jira, Azure
DevOps). It knows nothing about any organisation that uses one.

## Naming rule

Nothing in this repository may name an employer, a client, a site hostname, a project
key, a custom field ID, a workflow belonging to one organisation, or a person. That covers
code, comments, tests, fixtures, docs, commit messages, and branch names.

- "Run this code for Jira and that code for Azure DevOps" belongs here.
- How one organisation has set up its Jira or Azure DevOps belongs in that organisation's
  instance module.
- Fixtures use invented data: `DEMO-101`, `example.com`, `field_42`.

`scripts/check-names` enforces the rule from the git hooks against a denylist kept outside
the repository. If a hook refuses a commit, move the specifics out. Do not weaken the
hook, skip it, or edit the denylist to get a commit through.

## Where a change goes

| Request | Destination |
|---|---|
| A feature for every tui-q instance | this repository |
| Names, queries, field IDs, colours, layout choices | the instance's `profile.toml` |
| Behaviour or look wanted by one instance only | Go code in the instance module, through `ext` |
| An instance needs a hook the core lacks | add a generic hook here, then use it from the instance |

Ask which of these a request is when it is not stated.

## Instance modules

Each job has a local Go module named `tq-<codename>`, kept wherever that job's files
live on the machine, with local git and no remote. It imports this module through a `replace` directive, embeds its own
`profile.toml`, and calls `tuiq.Run` with its options. Its `CLAUDE.md` is the only place
that organisation's specifics are written down. Never copy text from an instance module
into this repository.

## Layout

| Path | Contents |
|---|---|
| `cmd/tq` | stock binary and the demo profile |
| `tuiq` | `tuiq.Run`, the one entry point |
| `ext` | what an instance passes to `tuiq.Run` to add behaviour: `ext.Action` so far |
| `ticket` | platform-neutral ticket model |
| `provider` | `Provider` interface and registry |
| `provider/fake` | in-memory provider with fixture tickets |
| `provider/jira` | Jira Cloud REST v3 client, field mapping, rich text to Markdown |
| `provider/ado` | Azure DevOps REST client, WIQL search, HTML to Markdown; reads and edits work items, adds comments |
| `profile` | TOML profile schema and loader |
| `internal/ui` | tabs, table, board, context menu, ticket view, status bar |
| `internal/arrange` | tickets of a queue grouped into buckets or lanes, shared by the interface and `tq order` |
| `internal/hit` | mouse messages to gestures on layer IDs |
| `internal/ops` | operation runner behind the status bar |
| `internal/prefs` | settings changed from inside the interface, saved per site |
| `internal/editor` | the comment box, a slate editor behind the `Box` interface, and a picture drawn in a ticket, behind `Picture` |
| `internal/drafts` | comments started and not sent, as Markdown, saved per site and ticket |
| `internal/people` | people mentioned in sent comments, saved per site and offered first after `@` |
| `internal/attach` | files read from tickets, kept per site in the cache directory |
| `internal/browser` | opens a URL in the default browser, and a file in the program for its type |
| `scripts/check-names` | the naming guard |

## Rules for the code

- Stay on the Charm stack. If a feature needs a UI library outside it, say so and get a
  decision before writing code. The one exception so far is slate
  (`github.com/lawsmd/slate`), the editor in the comment box. Only `internal/editor`
  imports it. See "Working on slate" below.
- `Model.Update` passes every message it has no use for to the comment box
  (`composerPass`), and `syncComposer` runs after every message to open the box for the
  ticket on screen, to place it, and to cover it while a menu or form is open. A message
  dropped on the way leaves the box frozen. The box is a border with the editor inside
  it. The editor holds the text of the comment and nothing else: the name of the box,
  the kinds of comment and the buttons are layers on the top of the border
  (`composerLayers`), and anything new the box needs to show or offer goes there too.
  The rows inside the border hold blank cells of the editor's colour and never text,
  which would wipe out the editor's image. The
  box scrolls with the ticket, so `placeBox` decides where it is for the frame and for
  the editor alike. `TestCommentBoxFitsEveryPane` checks the box across pane sizes and
  scroll positions.
- A comment box that leaves the screen is closed by `Model.retire`, off the UI goroutine:
  closing ends a page in a browser and, under a multiplexer, waits before it wipes the
  image. Boxes close in order, and the command that opens the next box waits for the
  last to have closed. Never call `Box.Close` from `Update`.
- A write names each request before making it, through the `report` function that
  `Model.write` passes, so the status bar says which request the user is waiting on.
- tui-q gives the comment box cells counted from the top left of its own window and
  knows nothing about the terminal or multiplexer it runs in. Code for one terminal or
  multiplexer (where the pane sits on the screen, the pixel size of a cell, how the
  image reaches the screen) belongs in slate, behind `bubble.Painter` and `bubble.Host`.
  `README.md` lists where the box works; update it when slate gains a host.
- A platform that takes comments implements `provider.Commenter` and names the format it
  wants. The interface writes Markdown and never builds a platform's format itself.
  `CommentKinds` returns the kinds of comment the box lets the user pick from, taken
  from `comment_kinds` in the profile with `provider.PickKinds`, or none on a platform
  with one kind.
- Every frame is a tree of Lip Gloss layers. Anything clickable is a layer with an ID;
  `internal/hit` reports gestures against those IDs. Give every layer an explicit `Z`:
  layers that share a z-order are drawn in no fixed order.
- A control is drawn with its colours swapped while the mouse is over it. Every layer of
  the frame passes through `Model.lit` in `compose`, which does this for the kinds of ID
  listed in `hot` (`internal/ui/hover.go`). A new button, mark or clickable label takes
  the effect by having its kind added there, and needs nothing where it is drawn. Tabs,
  rows and cards are left out on purpose.
- Bubble Tea calls `View` after every message, and the terminal sends a message for every
  cell the mouse crosses, so what one message costs is paid many times a second.
  `compose` gathers the layers and gives them to the root in one `NewLayer` call: Lip
  Gloss measures every layer of the tree again on each `AddLayers`. `Model.Update` handles
  a move of the mouse with no button down apart from the rest, and `View` returns the
  last frame when that move left the same control lit (`still`, `fresh`). Rendered
  Markdown is kept by text and width (`rendered` in `markdown.go`), and a command that
  reads a ticket renders it before returning (`Model.prewarm`). The benchmarks in
  `bench_test.go` time one message in each view: run them before and after a change to
  how a frame is built.
- Every provider call goes through `Model.start` in `internal/ui`, never straight from
  `Update`, with a mode from `internal/ops`. `ops.Wait` is a read the user asked for and
  `ops.Write` is a change: both refuse input until they end. `ops.Background` is any
  other read and leaves input free. A queue is read in the background, and the queue
  view refuses input while a read of it is running (`queue.loads`).
- While input is refused (`Model.blocked`) the frame is drawn grey above the status bar,
  and the status bar names the request. Check `blocked` in one place for each kind of
  input, at the top of its case in `Model.update`, and not in the handlers under it.
- The interface reads a ticket and the moves open to it together, through `Model.read`,
  every time it shows a ticket or reads one back after a write. A platform that can do
  that in fewer requests than `Get` and then `Transitions`, or at the same time,
  implements `provider.Loader`. Requests a provider makes at the same time must leave
  what it remembers in the same state as making them in order.
- The platforms' HTTP clients use `provider.Transport`.
- A frame must be exactly the size of the pane. `TestFrameMatchesPaneAtEverySize` checks
  this; extend it when a view is added.
- An instance action runs through `Model.doAction`, which locks input and shows the
  steps the action reports. The core never learns what an action does.
- A platform whose tickets show files implements `provider.AttachmentReader`, lists the
  files in `Ticket.Attachments`, and writes `Attachment.Ref()` where the body or a comment
  shows one. The ticket view opens pictures only (`Attachment.Image`), and a file is
  stored under an extension taken from its media type, never from its name: opening any
  other file would start whatever program its type belongs to.
- With `pictures = true` in the profile, the ticket view draws a picture under the line
  that names it. `mainLines` holds rows of blank cells for it (`Styles.Picture`) and an
  entry in the labels with `pic` set, and `syncPictures` runs after every message, as
  `syncComposer` does: it reads a picture whose line has come into view, tells each one
  where its rows are, and covers them under a menu or form. The rows hold blank cells and
  never text. `Model.Update` passes the pictures every message it has no use for
  (`picturePass`). `TestPictureFitsEveryPane` checks the rows across pane sizes and scroll
  positions.
- A platform that can read the history of a ticket implements `provider.RecentReader`,
  which a queue with `recent = true` loads through `arrange.Own`. The queue is one bucket
  that holds every ticket, made by the profile loader, so it pages as any bucket does.
- A profile never holds a secret. It names the environment variables that do.
- `tq order` tells an LLM agent the order to work in. It prints key, status and title for
  each ticket. Do not add other fields, full ticket loads or writes to it: the agent reads
  and changes tickets through its own access to the platform.

## Working on slate

slate is a separate product with its own repository, and it is used outside tui-q. The
two stay separate repositories. The agent working on tui-q is still the one expected to
fix and extend slate when tui-q needs it, so the work does not pass back and forth
between agents in two projects.

- Find where slate is cloned on the machine before starting (the agent's memory records
  it). Work in that clone, commit there, and push it to its own remote. On a machine
  with no clone, clone it beside this repository with
  `gh repo clone lawsmd/slate ../slate` and give it the same repo-local git identity.
- Change slate only in ways that make sense for slate as a general editor: a bug fix, or
  a feature another program embedding slate could use too.
- A problem that only tui-q's use of slate has is solved in `internal/editor`, not in
  slate.
- After pushing a slate change, update the dependency here with
  `go get github.com/lawsmd/slate@<commit>` and `go mod tidy`. Do not commit a `replace`
  directive that points at a local clone.
- To try a slate change in `tq` before it is pushed, add
  `replace github.com/lawsmd/slate => <clone>` to the `go.mod` of the module being
  built, an instance or this one, and take it out afterwards.
- Push slate before pushing a tui-q commit that needs the new slate version.

## Commands

```
go run ./cmd/tq            run the demo against the fake provider
go run ./cmd/tq DEMO-104   show one ticket, named by a link, key or number, and load no queue
go run ./cmd/tq check      read every queue once and print it, with no interface
go run ./cmd/tq order      print the tickets of each queue in working order, for an LLM agent
go test ./...              tests
go test ./internal/ui -run '^$' -bench .    what one message costs in each view
go vet ./...               static checks
git config core.hooksPath .githooks    turn the hooks on in a new clone
```

The implementation plan and its phases are in `docs/plan.md`.
