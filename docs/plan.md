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
- A single click opens a ticket. The context menu opens from a right-click, a
  middle-click, or the menu button on every row and card. The button is there because a
  terminal multiplexer can keep right-clicks for its own menus.
- Credentials come from environment variables named in the profile.

## Extension API

Registered from an instance's `main.go` as options to `tuiq.Run`. Each is added when an
instance first needs it.

| Extension | Purpose |
|---|---|
| `ext.Action` | a button in the ticket view and a context menu entry; gets the ticket, the provider, the answers to its own questions, and a progress reporter that writes to the status bar. Built; passed with `tuiq.WithActions` |
| `ext.FieldRenderer` | custom drawing of one field in tables, cards and the sidebar |
| `ext.Theme` | styles for the whole program |
| `ext.Panel` | an extra section in the ticket view |
| `ext.View` | a custom top-level tab |

Provider packages export their HTTP client so an instance action can call any endpoint.
The Jira client has `Do` for JSON and `Raw` for files.

## Profile

- `[provider]`: kind, base URL, names of the credential environment variables.
- `[[queue]]`: name, query in the platform's own language, `layout` (`table` or `board`),
  columns or card fields, refresh interval. A board names the field its lanes group by.
  Dropping a card on a status lane runs a transition; on any other lane, a field edit.
- `[[queue.bucket]]`: a titled section of a table, with a name, a field (the status by
  default) and the values that put a ticket in it. Buckets are drawn from top to bottom
  with a blank line above each, and scroll as one list. A ticket that matches none is listed under `Other`, or left out
  of the table and the count when the queue sets `hide_unmatched = true`.
- `[ticket]`: main fields and sidebar fields in order; whether comments are shown.
- `[fields.<name>]`: platform field ID, type, label. A field with no label is shown
  under the name the platform gives it.
- `[fields.<name>.icons]`: a map from a value of the field to the name of an icon in
  `docs/icons.md`. The key `"*"` names the icon for every other value; without it such
  a value shows `·`. A table shows the icon in place of the value, in a column one cell
  wide with no heading. A card and the ticket sidebar show the icon before the value.
  A built-in field takes icons too, in a `[fields.<name>]` table with no `id`.

## Interface

- Top row: queue tabs. Second row: open tickets. Last row: status bar. A blank line follows the tab rows. A profile with
  one queue has one tab row, with the open tickets after the queue.
- Every frame is a Lip Gloss compositor tree. `internal/hit` reports click, right-click,
  drag, drop and wheel against layer IDs.
- A dragged card is drawn as a top layer with no ID, so the hit test reports the lane
  under it.
- Tables drop low-priority columns when narrow, then scroll sideways with the first
  column, the title and the icon columns pinned. The line between two column headings
  is a handle: a drag on it sizes the column next to it, and the title takes the rest.
  The widths are saved per queue with the other view settings, and the view menu resets
  them. Boards scroll sideways by lane and vertically inside a
  lane. Sideways scrolling is on the arrow buttons, the arrow keys, a sideways wheel and
  shift with the wheel. The ticket sidebar moves under the main
  column when narrow.
- A table has a gear button beside `refresh` that opens the settings of the view: a
  checkbox for each column it can show, then options such as showing a person's first
  name only. The columns on offer are the profile's for that queue, then every other
  field. These settings are saved per site in the user's configuration directory,
  outside the profile.
- Reads show in the status bar and leave input free. Writes lock input until they end.
  Failures stay in the status bar until dismissed.
- A custom field with no value is left out of a ticket whose edit screen does not have
  it. One profile can then list the fields of several ticket types.
- A field the platform lets the user change has a mark after its label, and a click on
  the label edits it. Status, assignee and fields with a fixed set of values open a menu.
  Text, numbers and lists open a form, which takes the keyboard until it is saved or
  cancelled. Rich text is not edited from the ticket view, since writing it back as
  plain text would drop its formatting.
- The ticket view has `move` and `assign` buttons. The context menu of a row or card has
  the same actions.
- A card dropped on another lane moves there at once and the change is written. When
  the write fails, or the platform asks for values first, the card goes back. A move
  that requires fields opens a form for them.
- After every write the ticket is read back and every queue reloads.
- An instance action runs as a write. When it has questions, a form asks them first and
  cancelling the form cancels the action. The status bar shows the step the action last
  reported. An action may run for five minutes unless it sets its own limit.

## Phases

| Phase | Work | State |
|---|---|---|
| 0 | Mouse test in the real terminal and multiplexer: click, drag, right-click, wheel, resize, colour | done; everything reaches the program except right-click, which the multiplexer keeps |
| 1 | Core skeleton: model, provider interface, fake provider, profile loader, shell, table, ticket view, operation runner, naming guard | done |
| 2 | Jira provider (read): search, ticket load, field mapping, rich text to Markdown; first instance module | done; `tq check` reads every queue of a profile once without the interface |
| 3 | Board layout, drag between lanes, context menu, sideways scrolling | done; a drop reports the move and changes nothing until phase 4 |
| 4 | Writes: transitions with required-field forms, assignment, field edits | done against the fake provider and against recorded requests; not yet run against a live site |
| 5 | `ext.Action` and the first instance-only action | done against the fake provider; the instance action is tested against a local stand-in for the site and not yet run against a live one |
| 6 | Azure DevOps provider and a second instance; macOS and Linux run | |
| 7 | Disk snapshots, timed refresh, themes, remaining extension points, docs | |

## Verification

- `go test ./...`: providers against `httptest` servers with invented fixtures; UI tests
  that send mouse messages to the model and check the result; frame-size checks across
  pane sizes.
- Each phase ends with a manual run in a resizable pane.
- `scripts/check-names -history` before every push.
