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
| `ticket` | platform-neutral ticket model |
| `provider` | `Provider` interface and registry |
| `provider/fake` | in-memory provider with fixture tickets |
| `provider/jira` | Jira Cloud REST v3 client, field mapping, rich text to Markdown |
| `profile` | TOML profile schema and loader |
| `internal/ui` | tabs, table, board, context menu, ticket view, status bar |
| `internal/hit` | mouse messages to gestures on layer IDs |
| `internal/ops` | operation runner behind the status bar |
| `internal/prefs` | settings changed from inside the interface, saved per site |
| `internal/browser` | opens a URL in the default browser |
| `scripts/check-names` | the naming guard |

## Rules for the code

- Stay on the Charm stack. If a feature needs a UI library outside it, say so and get a
  decision before writing code.
- Every frame is a tree of Lip Gloss layers. Anything clickable is a layer with an ID;
  `internal/hit` reports gestures against those IDs. Give every layer an explicit `Z`:
  layers that share a z-order are drawn in no fixed order.
- Every provider call goes through `Model.start` in `internal/ui`, never straight from
  `Update`. Reads leave input free; writes lock it until they finish.
- A frame must be exactly the size of the pane. `TestFrameMatchesPaneAtEverySize` checks
  this; extend it when a view is added.
- A profile never holds a secret. It names the environment variables that do.

## Commands

```
go run ./cmd/tq            run the demo against the fake provider
go run ./cmd/tq check      read every queue once and print it, with no interface
go test ./...              tests
go vet ./...               static checks
git config core.hooksPath .githooks    turn the hooks on in a new clone
```

The implementation plan and its phases are in `docs/plan.md`.
