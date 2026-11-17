# Implementation plan

## Goal

One terminal UI for ticket queues on more than one platform, driven mostly with the
mouse, that stays quick to use while the platform's API is slow.

The core is generic. Each installation adds its own local Go module (an instance) that
holds everything specific to one organisation and can change how the program behaves and
looks.

## Decisions

- Built on the Charm stack: Bubble Tea v2, Lip Gloss v2, Bubbles v2, huh, glamour.
- An instance is a Go module that imports the core, compiled into its own binary. It is
  kept in local git with no remote.
- Windows, macOS and Linux.
- Writes in the first version: status transitions, assignment and field edits, custom
  actions. Comments are read-only; every ticket has "open in browser".
- A single click opens a ticket. A right-click opens a context menu.
- Credentials come from environment variables named in the profile.

## Extension API

Registered from an instance's `main.go` as options to `tuiq.Run`. Each is added when an
instance first needs it.

| Extension | Purpose |
|---|---|
| `ext.Action` | a button in the ticket view and a context menu entry; gets the ticket, the provider client, and a progress reporter that writes to the status bar |
| `ext.FieldRenderer` | custom drawing of one field in tables, cards and the sidebar |
| `ext.Theme` | styles for the whole program |
| `ext.Panel` | an extra section in the ticket view |
| `ext.View` | a custom top-level tab |

Provider packages export their HTTP client so an instance action can call any endpoint.

## Profile

- `[provider]`: kind, base URL, names of the credential environment variables.
- `[[queue]]`: name, query in the platform's own language, `layout` (`table` or `board`),
  columns or card fields, refresh interval. A board names the field its lanes group by.
  Dropping a card on a status lane runs a transition; on any other lane, a field edit.
- `[ticket]`: main fields and sidebar fields in order; whether comments are shown.
- `[fields.<name>]`: platform field ID, type, label.

## Interface

- Top row: queue tabs. Second row: open tickets. Last row: status bar.
- Every frame is a Lip Gloss compositor tree. `internal/hit` reports click, right-click,
  drag, drop and wheel against layer IDs.
- A dragged card is drawn as a top layer with no ID, so the hit test reports the lane
  under it.
- Tables drop low-priority columns when narrow, then scroll sideways. Boards scroll
  sideways by lane and vertically inside a lane. The ticket sidebar moves under the main
  column when narrow.
- Reads show in the status bar and leave input free. Writes lock input until they end.
  Failures stay in the status bar until dismissed.

## Phases

| Phase | Work | State |
|---|---|---|
| 0 | Mouse test in the real terminal and multiplexer: click, drag, right-click, wheel, resize, colour | program written; needs a run with a real pointer |
| 1 | Core skeleton: model, provider interface, fake provider, profile loader, shell, table, ticket view, operation runner, naming guard | done |
| 2 | Jira provider (read): search, ticket load, field mapping, rich text to Markdown; first instance module | provider done and tested against a stub server; instance module and a run against a real site still to do |
| 3 | Board layout, drag between lanes, context menu, sideways scrolling | |
| 4 | Writes: transitions with required-field forms, assignment, field edits | |
| 5 | `ext.Action` and the first instance-only action | |
| 6 | Azure DevOps provider and a second instance; macOS and Linux run | |
| 7 | Disk snapshots, timed refresh, themes, remaining extension points, docs | |

## Verification

- `go test ./...`: providers against `httptest` servers with invented fixtures; UI tests
  that send mouse messages to the model and check the result; frame-size checks across
  pane sizes.
- Each phase ends with a manual run in a resizable pane.
- `scripts/check-names -history` before every push.
