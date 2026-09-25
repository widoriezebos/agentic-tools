# g1-s48: Decisions — an inbox you can empty

- Kind: design
- Id: 01M3CE0Q0D1M7XKZ3Z6Q6E7ZQF
- Status: draft
- Goals: browser-interface

Wido, 2026-09-25, on the page as g1-s46 left it: "I think it is still a
big mess tbh. This is bad UX. I need much better UX. Put on your UX cap
and understand what I (the user) wants and needs from that page and come
up with a much better UX". Author Fable. Step 3 of the Decisions section,
on g1-s46 as landed at `282ee13e9`; the live page read at 15:40 UTC.

## 1. What exists and binds

1. **The page is still a wall.** 40,827 pixels tall at 1280 wide. The
   "Asked of you" block is 24 cards of one shape: 13 ruling reviews whose
   title is the ruling's id and whose whole content is "Review R-38-m3,
   due 2026-09-01: adopt, revise or withdraw", because the review item
   carries the id and not the words (internal/ui/decisions/decisions.go:635-657)
   while the words sit in `decided.rulings` by the same id; 4 questions
   each printed twice, as title and as asked; 3 drafts and 4 landed
   designs. The queue below it is 122 rows, each with a mono id up to 50
   characters taking a third of the line, an intent cut at two lines, and
   two buttons; 244 buttons on one page. The Decided tabs sit at the
   bottom of the wall.
2. **Everything weighs the same.** A three-week-old review that "stays in
   force as written" and a question asked yesterday are the same card in
   the same place; nothing says what is new, what is cheap, what can wait.
3. **The acts exist for most of it.** Approve, withdraw, park, unpark on a
   goal (internal/ui/httpd/acts.go, describe.go:39-53); a record's status
   (`/api/project/records/<id>/status`, write.go:33-45, describe.go:57;
   the statuses are the resolver's, project/write.go:242-246), which is
   what accepting a draft and marking a landed design done are; settling a
   question (`/api/project/questions/<id>/status`, describe.go:63). Edit
   arrives with g1-s47. Every one of them takes the signed-in session.
4. **New since your last visit is a fact the interface keeps.** Overview
   keeps one last-visit marker per human (internal/ui/overview/visit.go:66-100,
   `Visit(root, human, now)` at 115), and says what changed since the
   visit before.
5. **The idioms are there.** The s46 queue row and its disclosure, the
   Find box, the label and origin chips, the order toggle, select-all and
   the two sheets (src/decisions/QueueBlock.tsx, BulkSheet.tsx); the
   Fleet row's disclosure; the board's `Panel`; the page tabs component
   (src/panes/Tabs.tsx); the goal header on the goal page
   (src/project/ProjectPane.tsx:774-790).

## 2. What the human wants and needs here

The human is the one authority over a system that runs many agents, and
this page is the inbox of everything that authority is asked for. Five
jobs, in the order they happen in a sitting:

- **J1. Is anything waiting on me, and how much?** Answered in one
  screen, in two seconds: what kinds, how many, what is new since I was
  last here, what is quiet.
- **J2. Deal with the fresh, cheap ones now.** Read the thing itself,
  not a label for it; decide with one click; watch it leave; watch the
  count drop.
- **J3. Work the approval queue in a sitting.** Narrow it, read a goal
  without leaving, say yes or not now to one or to twenty, edit a
  wording on the way.
- **J4. Find what I decided.** Rulings, parks, approvals; secondary.
- **J5. Not be nagged.** What stays as it is when ignored must look like
  it: present, quiet, collapsed.

The page as built fails J1 (no summary, no new), J2 (no substance on
the card, acts elsewhere), J3 (noise on every row), and J5 (the quiet
things shout). It serves J4, at the bottom.

Decisions:

- D1. **Two views, Inbox and Decided.** The page header carries them as
  tabs with counts; Inbox is the page at rest. Decided holds the s44 and
  s46 tabs unchanged.
- D2. **The inbox is groups, collapsed.** One line per group: name, count,
  the newest item's age, "n new" since the last visit, and for the quiet
  groups the standing sentence instead of an age: "they stay in force".
  Groups, in this order: Questions, Drafts to accept, Designs landed,
  Asks from a seat, Alerts, Approvals to renew, Stopped goals, Parked by
  a seat, Rulings past review, Goals waiting for approval. An empty
  group is not shown. One group open at a time, kept per viewer under
  `ms.ui.decisions.open` in try/catch; the first visit opens the first
  group with something new, else the first group. The page at rest is
  ten lines and the open group.
- D3. **A row is one line of substance.** The question itself; the
  ruling's own words, first sentence; the record's title; the goal's
  intent. Then, right-aligned and muted: for a goal "yours", the tier
  and up to two labels; the age; a dot where the item is new. The id is
  not on the line; it is in the open row, small. No button on a row.
- D4. **One open row at a time, inline, with the substance whole and the
  acts.** Question: the question, who asked and when, "Open the
  register", "Settle" (the question status route) with a one-line
  confirmation. Draft: the record's title and path, "Accept" (status
  accepted), "Open the record". Landed design: its goals and their
  states, "Mark done" (status done), "Open the record". Ruling review:
  the words, the context behind a disclosure, owner, class and due date,
  "Open the register", which is where a ruling is revised. Goal: intent,
  next step, labels, tier, budget or "no budget recorded", blockers,
  origin, opened when; Approve, Not now, Edit (g1-s47's sheet, when it
  lands), Open the goal. Seat park: reason, "Return to queue". Renewal:
  Approve. Alert: the line, "Open the message". Ask: the recorded facts
  and "answer on the fleet channel". Every act runs through the sheets
  and routes that exist, and the row leaves the list when the re-read
  no longer lists it; the group count and the header count follow.
- D5. **Selection is a bar, not a row of buttons.** In the queue group a
  checkbox per row and "Select all shown"; when one or more are selected
  a bar sits at the group's foot: "3 selected · Approve · Not now ·
  Clear", opening the s46 sheets. The bar is the only place the bulk
  acts live.
- D6. **The queue tools stay, on one line** in the group's head: Find,
  the chips from the rows shown, the order toggle.
- D7. **New means since your last visit.** The payload marks each item
  `new` when its `since` is after the window Overview's marker gives,
  and reading Decisions advances the marker as Overview's read does.
- D8. **Sign-in as today**: the bar at the top when unproven, and an open
  row's acts say "Sign in to act" in their place.
- D9. **Phone width**: groups and rows stack, the open row's acts wrap,
  the selection bar sticks to the bottom.

## 3. The payload: `GET /api/decisions`, schema 3

Additions only: `new: true|false` on every `needsYou` item; the ruling
review item gains `words`, `context`, `owner`, `class`, `due` from the
register row it names, so the row and the open row need no join; the
draft and landed items gain the record's `path` and the landed item its
goals with states. `counts` unchanged. Nothing removed.

## 4. Not here, step 4 and later

Keyboard triage. Snoozing or dismissing an item. Answering a question
with text from the page. Bulk acts on any group but the queue. A reading
pane beside the list instead of the inline row. The Overview's own
needs-you group. Renewing a claimed goal's approval.

## 5. Verification and box

Go: composition tests for `new` against an injected marker window, the
ruling review's words, the record paths; the schema number. Frontend:
the groups and their order with counts and "n new"; one group open, the
choice kept; a row's line for each kind; an open row per kind with its
acts; Settle, Accept and Mark done reaching their routes and the re-read
removing the row; the selection bar appearing and its two sheets; the
queue tools; the guards stay green. The walkthrough fixture already
holds every kind; it gains a last-visit window so half the items are
new. Screenshots at 1280 and 400: the page at rest, a group open, a
question row open, a goal row open, the selection bar, the Decided view.
Budgets as always. Box: one build lane (Claude on Opus), one code read
(Codex on Sol) with one fix round under R-124, after Astra's read; two
attempts, 150 to 240 job-minutes; lands on ui-development only.

## 6. Self-grade

High on D1 to D6 and D8: they rearrange what the page already reads
into the shape an inbox has, and every act is a route that exists.
Medium on D7: it makes a second reader of Overview's marker; the build
proves the marker serves both without a second writer. Medium on D4's
Settle, Accept and Mark done: they are one-click writes to records; the
confirmation line is the whole of their safety, and each names what it
writes. Weakest: the inline open row against a reading pane; the inline
row keeps one column and one pattern the interface already has, and the
page can grow the pane later if a sitting proves it wants one.
