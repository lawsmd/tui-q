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
  actions. Every ticket has "open in browser".
- The sidebar is as wide as its widest label, value or button, between 18 and 32
  cells, and longer text wraps. A list value shows one item to a line.
- A code block in a ticket wraps a long line onto as many rows as it needs.
- A list of tags opens the tag box under the field's label, reaching left out of a
  narrow sidebar. It
  shows the tags as buttons that take one off, a line that searches as it is typed
  when the provider can search the field's values (`provider.Suggester`), and the
  matches the ticket lacks, with the words typed offered as a new tag. Enter adds a
  match, or each of a list typed with commas, and saves when nothing is typed. The box
  writes once, on save. Azure DevOps searches the project's tags.
- Comments can be added from the ticket view. Existing comments cannot be edited: the
  ticket model holds no ID for a comment and not the body as the platform stores it.
- The top border of each comment has `copy` at its right end. A click puts the comment
  on the clipboard as Markdown, with the name of a file where the comment shows one. A
  border under 25 cells wide has the title alone.
- A single click opens a ticket. The context menu opens from a right-click, a
  middle-click, or the menu button on every row and card. The button is there because a
  terminal multiplexer can keep right-clicks for its own menus.
- Credentials come from environment variables named in the profile.

## Extension API

Registered from an instance's `main.go` as options to `tuiq.Run`. Each is added when an
instance first needs it.

| Extension | Purpose |
|---|---|
| `ext.Action` | a button in the ticket view and a context menu entry; gets the ticket, the provider, the answers to its own questions, and a progress reporter that writes to the status bar. `Section` puts its button at the foot of a sidebar section instead of the row over the title. Built; passed with `tuiq.WithActions` |
| `ext.FieldRenderer` | custom drawing of one field in tables, cards and the sidebar |
| `ext.Theme` | styles for the whole program |
| `ext.Panel` | an extra section in the ticket view |
| `ext.View` | a custom top-level tab |

Provider packages export their HTTP client so an instance action can call any endpoint.
The Jira client has `Do` for JSON and `Raw` for files. The Azure DevOps client has `Do`,
and its provider exports `Query` (WIQL to IDs), `Read` (batch reads of chosen fields)
and `ToTicket`, so an instance can build a queue of its own from them.

## Profile

- `[provider]`: kind, base URL, names of the credential environment variables.
  `default_project` names the project a bare ticket number is looked up in, on a
  platform whose ticket keys carry a project. `comment_kinds` lists the kinds of
  comment the comment box offers, the default first. Jira has `public` and `internal`.
  For Azure DevOps (`kind = "ado"`), `base_url` is the organisation URL and
  `default_project` the project every query runs in. With `token_env` set the provider
  signs in with that personal access token; without it, with a token from
  `az account get-access-token`, using the program in `AZURE_CLI_PATH` when that is set.
  A queue's query is WIQL. The moves open to a work item are every state of its type but
  the current one; Azure DevOps applies the process's rules when the move is saved. A
  field of the profile can be edited when the work item's type has it and it is not
  read-only: a person (the sign-in name), a field with allowed values (a menu), a number,
  plain text, or the tags. Azure DevOps has one kind of comment, so `comment_kinds` is an
  error.
- `[[queue]]`: name, query in the platform's own language, `layout` (`table` or `board`),
  columns or card fields, refresh interval. A board names the field its lanes group by.
  Dropping a card on a status lane runs a transition; on any other lane, a field edit.
- `[[queue.bucket]]`: a titled section of a table, with a name, a field (the status by
  default) and the values that put a ticket in it. Buckets are drawn from top to bottom
  with a blank line above each, and scroll as one list. A ticket that matches none is listed under `Other`, or left out
  of the table and the count when the queue sets `hide_unmatched = true`.
  A bucket with `page_size` shows that many tickets at a time. A bucket with its own
  `query` takes its tickets from that query, one page at a time (ten when it names no
  `page_size`), and needs no `values`; this suits a bucket too large to load whole.
  When every bucket of a queue has its own query, the query of the queue is not run.
  A bucket with `about` has an `(i)` mark at the right end of its heading, left of the
  page buttons. With the mouse over the mark,
  a box shows the `about` text: which tickets the bucket holds and which it leaves out.
- `recent = true` on a `[[queue]]`: the queue lists the tickets the user acted on lately,
  the latest first, in place of the results of a query. A ticket counts when the user
  changed its status, added or removed a tag, or changed its assignee, or when anyone
  assigned it to the user or took it from them. `days` sets how far back it looks (30
  unless given) and `page_size` how many tickets a page shows (20 unless given). The
  columns `touched` and `change` give when and what the last such change was;
  `touched` keeps its time when the table shows dates without times. The
  platform must implement `provider.RecentReader`; on one that does not, the tab is left
  out. `tq order` leaves the queue out unless it sets `order = true`. Azure DevOps reads
  the history of each work item changed in that time that the user ever changed or was
  assigned, at most 400 of them, the ones changed most lately.
- `[ticket]`: main fields and sidebar fields in order; whether comments are shown;
  whether pictures are drawn in the ticket (`pictures`, off unless set); whether the
  sidebar lists the tickets a ticket is linked to (`links`, off unless set).
- `[fields.<name>]`: platform field ID, type, label. A field with no label is shown
  under the name the platform gives it.
- `[fields.<name>.icons]`: a map from a value of the field to the name of an icon in
  `docs/icons.md`. The key `"*"` names the icon for every other value; without it such
  a value shows `·`. A table shows the icon in place of the value, in a column one cell
  wide with no heading. A card and the ticket sidebar show the icon before the value.
  A built-in field takes icons too, in a `[fields.<name>]` table with no `id`.

## Interface

- Top row: queue tabs. Second row: open tickets. Last two rows: a rule, then the status bar. Menus, forms and tooltips stay above the rule. A blank line follows the tab rows. A profile with
  one queue has one tab row, with the open tickets after the queue.
- The top row has a jump box left of the gear. A click on it or the `/` key gives it
  the keyboard, and it widens to hold a link. `enter` opens the ticket named by a
  number, a key or a pasted link in a ticket tab, whether or not a queue lists it;
  `esc` or a click elsewhere closes the box. The provider's `Resolve` reads the text.
  When the queue tabs leave too little room the box is drawn as a `go to` button, and
  in a narrower pane it is left out and `/` still opens it.
- The heading of a bucket shown one page at a time gives the range on screen and the
  size of the bucket, such as `11-20 of 412`, and has a pair of arrows at its right end
  that turn the page. A bucket that fits on one page shows its size alone, such as
  `Open (8)`. A bucket with its own query loads the page as a read; any other
  has its tickets already. The queue tab counts the whole bucket. After a write the
  page on screen is loaded again, and a page that is no longer there is answered with
  the first.
- A provider pages with `SearchPage`, which takes a cursor: empty for the first page,
  then the `Next` of the page before. A platform that pages by position can use the
  position as its cursor. `Total` is -1 when the platform gives no count, and the
  heading then shows the range alone.
- Every frame is a Lip Gloss compositor tree. `internal/hit` reports click, right-click,
  drag, drop and wheel against layer IDs.
- A dragged card is drawn as a top layer with no ID, so the hit test reports the lane
  under it.
- Tables drop low-priority columns when narrow, then scroll sideways with the first
  column, the title and the icon columns pinned. The line between two column headings
  is a handle: a drag on it sizes the column next to it, and the title takes the rest.
  The widths are saved per queue with the other view settings, and the view menu resets
  them. An option in the same menu shows every date and time in a table or on a card
  as the date alone. Boards scroll sideways by lane and vertically inside a
  lane. Sideways scrolling is on the arrow buttons, the arrow keys, a sideways wheel and
  shift with the wheel. The ticket sidebar moves under the main
  column when narrow.
- A table has a gear button beside `refresh` that opens the settings of the view: a
  checkbox for each column it can show, then options such as showing a person's first
  name only. The columns on offer are the profile's for that queue, then every other
  field. These settings are saved per site in the user's configuration directory,
  outside the profile.
- A button, a mark, a clickable label or a menu item is drawn with its colours swapped
  while the mouse is over it.
- Every request shows in the status bar. A read the user asked for and a write lock input
  and grey the view until they end. Reads in the background leave input free.
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
- Every ticket view has a comment box in its main column, under the heading of the
  comments and beside the newest one, and it scrolls with them. The box is a border, drawn
  as the border of a comment is, around the rows of a
  slate editor (`github.com/lawsmd/slate`), which a headless browser draws and the
  terminal shows as a Sixel image. `internal/editor` is the only package that imports it.
- The border is one grey whoever writes (`Styles.ComposerBorder`). Its top holds the name
  of the box, "Write a comment", then the kinds of comment, and the `discard` and `send`
  buttons at its right-hand end. The editor is the rectangle inside the border and holds
  the text of the comment only: tui-q draws every name, choice and button of the box
  itself, in cells. The border carries no words on how to use the box.
- The editor is drawn on a grey of its own (`Styles.ComposerBack`), lighter than a dark
  terminal's own colour.
- Idle, the editor has three rows. Written in, it has three quarters of the rows of the
  column, twenty-one at most, and the ticket scrolls until the whole box is in view. The
  editor is placed over the rows of the box that are in view, and stops drawing when none
  is.
- The rows of the editor are drawn as blank cells of the editor's grey. The editor's image
  goes over them, and they clear what the image leaves where the box was before a scroll.
- In a wide pane too short to keep the title in place over the main column, the title
  and the sidebar scroll with the column as they do in a narrow pane.
- The wheel over the box scrolls the ticket, and scrolls the editor while the box has the
  keyboard.
- `c`, a click in the box or a click on the top of its border gives it the keyboard, and `esc` or a
  click elsewhere takes it away. While the box has the keyboard every key but `esc`,
  `ctrl+s` and `ctrl+k` is the editor's, `ctrl+c` included, which copies.
- When the editor cannot open, as under tmux, the border says why and `c` tries again.
- `ctrl+s` or the `send` button posts the comment as a write. The provider names the
  format it takes (`provider.Commenter`), and the editor converts its Markdown to that
  format when the comment is sent. When the write fails the box and its text stay.
- A platform can have kinds of comment, such as a public reply and an internal note.
  `comment_kinds` in `[provider]` lists the kinds the box offers, and a new comment
  starts as the first. The border of the box shows them after its name with the chosen
  one marked. A click on a kind picks it, and so does `ctrl+k` while the box has the
  keyboard. The choice is kept per ticket until the comment is sent or discarded, or
  `tq` exits. With one kind listed every comment is that kind and the border shows no
  choice. A comment not everyone can read has its kind after its date in the list.
- Azure DevOps takes the comment as Markdown (`format=markdown`). `@` in the box offers
  the organisation's users from the identity picker, and a mention is sent as
  `@<identity ID>`. Comments are read from their rendered HTML, so a mention reads back
  as the person's name.
- On Jira, `@` searches the people who can see the issue (`provider.Mentioner`), and the
  editor writes the mention as a `mention` node with the account ID.
- The people mentioned in a sent comment are kept per site (`internal/people`, at most
  50, most recent first). `@` lists the ones that match what is typed and asks the
  platform only when none does.
- What is typed is kept as a draft per ticket, as Markdown, in the user's configuration
  directory. Leaving the ticket or quitting closes the box and keeps the draft. `discard`
  removes it, and so does sending.
- One editor exists at a time, for the ticket on screen. Showing another ticket closes
  it and opens one for that ticket. A menu or a form shown over it stops it drawing
  until the menu or form closes.
- The browser behind the editor starts when the first ticket is shown.
- A picture in the body or a comment is a line with its name, and a click opens the file
  in the program for its type. With `pictures = true` in `[ticket]` the picture is also
  drawn under its line. This is an experiment, to find out what it costs to draw and
  scroll several pictures and whether to keep it.
- A picture is read when its line first comes into view, through the operation runner as
  a read. slate scales it to fit the column and 24 rows (`pictureRows`) and draws it as a
  Sixel image, with no browser. The ticket view holds rows of blank cells for it, in a
  colour of their own (`Styles.Picture`), and tells the picture where those rows are and
  which part of the window the column has (`syncPictures`). The picture draws the part of
  itself in view.
- An attachment shown twice in a ticket is drawn the first time and named the second.
  A file that cannot be drawn keeps its line, and the status bar says why. Where the
  terminal shows no image, no further picture is tried.
- The pictures belong to the ticket on screen. Showing another ticket or a queue removes
  them, and a menu or form over the ticket stops them drawing.
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
| 6 | Azure DevOps provider and a second instance; macOS and Linux run | provider with comments, moves, assignment and field edits done against `httptest`, and reads run against a live organisation; writes other than comments not yet run live; second instance, macOS and Linux run not yet |
| 7 | Disk snapshots, timed refresh, themes, remaining extension points, docs | |

Outside the phases: `tq order` prints the tickets of each queue in working order for an
LLM agent, as key, status and title. It shares the bucket and lane grouping with the
interface through `internal/arrange`.

Also outside the phases: `tq LINK` starts the interface on one ticket, named by a link, a
key or a number that the provider's `Resolve` reads. `ui.Options.Ticket` carries the ID.
The model then loads the account, the field names and that ticket, and never a queue. The
top row holds a tab for each open ticket, the jump box and the refresh button. A ticket
opened from a link or from the jump box takes a tab beside the first. The last tab cannot
be closed, and no key or layer leads to a queue.

Also outside the phases: linked tickets. A provider lists the tickets a ticket is linked
to in `Ticket.Links` on a full load, each with the relation worded from the ticket's side.
Jira reads them from `issuelinks`; Azure DevOps lists none yet. With `links = true` in the
profile's `[ticket]` table the sidebar lists them under a Links rule, and a click on a
key opens that ticket in a tab.

Also outside the phases: `tq tabs` prints the tickets open in the tabs of every running
interface for an LLM agent, as key, status and title, with the links of each. The
interface keeps the list in a file of its own under the cache directory
(`internal/session`), writes it when a tab changes, and removes it on exit. The command
prints the process ID of each interface, which slate's listing of open editors carries
too.

Also outside the phases: a queue with `recent = true` lists the tickets the user acted on
lately, through `provider.RecentReader`. Azure DevOps and the fake provider implement it.
Jira does not yet; the next step is a Jira `Recent` that reads the changelog of the
issues the user changed or was assigned in the period.

## Verification

- `go test ./...`: providers against `httptest` servers with invented fixtures; UI tests
  that send mouse messages to the model and check the result; frame-size checks across
  pane sizes.
- Each phase ends with a manual run in a resizable pane.
- `scripts/check-names -history` before every push.
