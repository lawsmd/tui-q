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
| `provider/ado` | Azure DevOps REST client, WIQL search, HTML to Markdown; reads work items and adds comments |
| `profile` | TOML profile schema and loader |
| `internal/ui` | tabs, table, board, context menu, ticket view, status bar |
| `internal/arrange` | tickets of a queue grouped into buckets or lanes, shared by the interface and `tq order` |
| `internal/hit` | mouse messages to gestures on layer IDs |
| `internal/ops` | operation runner behind the status bar |
| `internal/prefs` | settings changed from inside the interface, saved per site |
| `internal/editor` | the comment box: a slate editor behind the `Box` interface |
| `internal/drafts` | comments started and not sent, as Markdown, saved per site and ticket |
| `internal/browser` | opens a URL in the default browser |
| `scripts/check-names` | the naming guard |

## Rules for the code

- Stay on the Charm stack. If a feature needs a UI library outside it, say so and get a
  decision before writing code. The one exception so far is slate
  (`github.com/lawsmd/slate`), the editor in the comment box. Only `internal/editor`
  imports it. See "Working on slate" below.
- `Model.Update` passes every message it has no use for to the comment box
  (`composerPass`), and `syncComposer` runs after every message to open the box for the
  ticket on screen, to place it, and to cover it while a menu or form is open. A message
  dropped on the way leaves the box frozen. The rows under the box's line hold blank
  cells of the box's colour and never text, which would wipe out the editor's image. The
  box scrolls with the ticket, so `placeBox` decides where it is for the frame and for
  the editor alike. `TestCommentBoxFitsEveryPane` checks the box across pane sizes and
  scroll positions.
- A platform that takes comments implements `provider.Commenter` and names the format it
  wants. The interface writes Markdown and never builds a platform's format itself.
  `CommentKinds` returns the kinds of comment the box lets the user pick from, taken
  from `comment_kinds` in the profile with `provider.PickKinds`, or none on a platform
  with one kind.
- Every frame is a tree of Lip Gloss layers. Anything clickable is a layer with an ID;
  `internal/hit` reports gestures against those IDs. Give every layer an explicit `Z`:
  layers that share a z-order are drawn in no fixed order.
- Every provider call goes through `Model.start` in `internal/ui`, never straight from
  `Update`. Reads leave input free; writes lock it until they finish.
- A frame must be exactly the size of the pane. `TestFrameMatchesPaneAtEverySize` checks
  this; extend it when a view is added.
- An instance action runs through `Model.doAction`, which locks input and shows the
  steps the action reports. The core never learns what an action does.
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
  it). Work in that clone, commit there, and push it to its own remote.
- Change slate only in ways that make sense for slate as a general editor: a bug fix, or
  a feature another program embedding slate could use too.
- A problem that only tui-q's use of slate has is solved in `internal/editor`, not in
  slate.
- After pushing a slate change, update the dependency here with
  `go get github.com/lawsmd/slate@<commit>` and `go mod tidy`. Do not commit a `replace`
  directive that points at a local clone.

## Commands

```
go run ./cmd/tq            run the demo against the fake provider
go run ./cmd/tq DEMO-104   show one ticket, named by a link, key or number, and load no queue
go run ./cmd/tq check      read every queue once and print it, with no interface
go run ./cmd/tq order      print the tickets of each queue in working order, for an LLM agent
go test ./...              tests
go vet ./...               static checks
git config core.hooksPath .githooks    turn the hooks on in a new clone
```

The implementation plan and its phases are in `docs/plan.md`.
