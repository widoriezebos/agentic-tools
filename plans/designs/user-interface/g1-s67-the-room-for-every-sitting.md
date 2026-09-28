# g1-s67: the room for every sitting — shaping a design in the same room as reviewing the work

- Kind: design
- Id: 01M3MGGN6WHCZ9C9Z4CXH7XW7Q
- Status: draft
- Goals: browser-interface

Wido, 2026-09-28, after his first look at the review room: "but I also
want to be able to discuss goal designs this way; does that not make
sense?" and, on the answer, "ok, continue". Author Fable. Every cite
re-read at `7ccd22250`.

## 1. What exists and binds

1. **The paper.** The sitting that shapes intent or a design and the
   sitting that reviews built work are one form, the human in a
   different chair (docs/paper/15-the-sitting.md). What differs is what
   the machinery brings and what leaves: recorded intent or a framed
   design with its open questions from the first, findings answered and
   a verdict from the second.
2. **The room as landed** (g1-s65 slice A, Built): a screen of its own at
   `/review/<record>` with the desk (`DeskItem` is a source range, the
   change index, one file's diff, or a record section;
   `web/_app/src/review/room.ts:22-29`), the sitting's own conversation
   beside it (D16), five walks fixed as requests (`Walks` and
   `WalkRequest` in `internal/ui/partner/review.go:35-84`), anchor chips
   that put a file on the desk, the Partner presenting through
   `present`, selection to Ask and Finding, the finding card with its
   four answers, the room's state kept on the sitting's mark, the door,
   the moved-tip banner, and End with the verdict. Three of its pieces
   are gated to the review purpose: `present` is admitted only in a
   review sitting (`review.go:194-204`), `finding` is the review's own
   deposit (`review.go:20-23`), and the desk's reads take a `Reviewed`
   tree, a branch tip or trailer commits (`internal/ui/review/review.go:
   138`), so the room's address only ever names a review record.
3. **The shaping sitting as it still is** (g1-s53, g1-s55): Start a
   sitting on an intent or design record from the record's page or the
   drawer's header, with two purposes (`PURPOSES` in
   `partner/sitting.ts:626`); the conversation in the drawer, the four
   piles as a table in the focused view at `/brain` and four counts in
   the drawer's bar (`partner/StartSitting.tsx`, `partner/Table.tsx`);
   the Partner's opening turn brings what the records hold; cases with
   Decide and Leave open; End drafts the Outcome; Project → Sittings is
   the door. Since D16 that sitting has a conversation of its own, and
   the drawer shows it while it stands, with "Your conversation" to
   return to the ordinary one (g1-s65 Built, D16).
4. **What binds on the code side.** The document reader serves a record's
   sections (`internal/ui/project/document.go`); the review owner reads
   source at a commit through a `Git` interface `gittree.Workspace`
   satisfies, and `ChangesSince` compares two commits (`review.go:298`);
   the checkout's head is one `ResolveCommit("HEAD")` away. The cut
   guard and the CSP as before.

## 2. What you want when you shape a design

- **S1. The same room.** One place to sit, whatever the chair: the
  record and the code on the desk, the colleague beside it, the piles
  on the board.
- **S2. The record on the desk.** The design you are shaping, section by
  section, large, beside what the code does today.
- **S3. Arguing from the code.** "What does the application do today?"
  answered with the lines on the desk, anchored at the checkout's head.
- **S4. The cases at the edge as cards**, decided or left open then and
  there, as g1-s55 built them, in the room.
- **S5. Two sittings side by side**, a design and a review, each in its
  own room with its own conversation, which D16 already gives.

## 3. The room, for a design

You open a design record and press Start a sitting, shape a design. The
room opens at `/sitting/<record>`: the header says "Shaping the design
g1-s66", the desk shows the record's first section, the strip shows its
sections as items you can put up one at a time, the conversation opens
with the Partner's turn on what the records hold. Four walks stand
under it: **Records**, what earlier rulings, decisions and open
questions touch this subject; **Today**, what the code does now, with
the lines on the desk; **Cases**, the cases at the edge, each a card
with Decide and Leave open; **Open**, what the room has not settled and
what would settle it. You ask "where is the lock taken today?" and the
Partner answers with `internal/ui/act/owner.go:41-88` as a chip; the
desk shows the lines at head. You select four of them and press Fact:
a fact card with the anchor filled, your words to write, Record it. A
case arrives; you Decide it with your reason. You step out; the design
page shows the door; you come back to the same desk and board. You
press End; the Partner drafts the Outcome; you record it and the sitting
ends, as it does today. For an intent record the room is the same with
"Shaping the intent" in the header.

## 4. Decisions

- D1. **Every sitting opens in the room.** Start on an intent or a design
  record, from the record's page, the drawer's header or the Sittings
  tab, opens the room at `/sitting/<record>`; a review keeps
  `/review/<record>`. One component, one chrome (no rail, no drawer,
  the bell), one word in the header per purpose: Reviewing, Shaping the
  design, Shaping the intent. The drawer keeps the ordinary conversation
  and never shows a sitting's; "Your conversation" goes.
- D2. **The desk of a shaping sitting reads the checkout's head.** The
  sitting's mark records the head commit at Start (`tree`); the desk's
  reads take that commit as the tree, so an anchor `path:lines` shows
  the lines as they were when the sitting began, and the review owner
  serves both kinds through one `Tree` (a review's tip or trailer
  commits, a shaping sitting's head). The change index is not offered in
  a shaping sitting, since there is no change; the strip starts with
  the record's own sections, the current one on the desk. On return, a
  head that moved since Start shows the banner "the code moved while you
  were out" with Show what changed (`ChangesSince` from the recorded
  head to the head now) and "Read at the head now", which rewrites the
  mark's `tree`; the record's head lines are untouched, since nothing
  was reviewed.
- D3. **Walks by purpose.** `Walks` is keyed by purpose: review keeps its
  five; shape a design and shape intent get Records, Today, Cases,
  Open, as fixed requests with the interface's provenance. The opening
  turn stays g1-s53's opening request.
- D4. **The cards are the sitting's own.** Facts, proposals, decisions,
  cases and open questions as built, with Record it, Decide and Leave
  open, and Ask it on the board. `present` is admitted in every sitting,
  so the Partner can put on the desk what it explains; `finding` stays
  the review's own. Selection on the desk offers Ask and **Fact**, a
  fact card with the anchor filled and the words empty.
- D5. **The board is the record's piles.** The room's Board face shows
  the piles of the record's kind, Facts, Proposals, Decisions and Open
  questions, each entry pinned to its anchor where it has one.
- D6. **The drawer's sitting pieces retire.** The four counts, the table
  at `/brain` and the drawer's Start and End controls go; `/brain` stays
  the focused ordinary conversation. The Start sheet keeps its two
  fields (purpose and subject) and opens the room. The Sittings tab
  lists every standing sitting and opens its room.
- D7. **Nothing else changes.** The record model, the recorder, the
  conversation store, the door line, Step out and End are the landed
  ones; a review sitting behaves exactly as it does today.

## 5. Step 1, the smallest thing that works

All of D1 to D7 as one slice; it is half the size of the room's slice
A because every mechanism exists. Not in it: an intent record's users
and outcomes as structured desk items; drawings (room slice B);
several records on one desk; a sitting on a goal's intent line rather
than a record.

## 6. Payload and routes

`POST /api/partner/sitting` answers the room's address for every
purpose and records `tree` on the mark for a shaping sitting (the
checkout's head at Start, through the existing `ResolveCommit`). The
desk reads (`/api/review/<record>/source`, `/changes?since=1`) accept a
record of any sittable kind and take the tree from the review head or
the mark's `tree`; `/changes` without `since` is refused for a shaping
sitting in words. `Walks` keyed by purpose; the walk route unchanged.
`present` admitted for every sitting. Routes: `/sitting/<record>` beside
`/review/<record>`, both `inTheRoom`. The Start sheet, the record page's
Start control and the Sittings tab navigate to the room; the drawer's
`SittingControl`, `SittingCounts`, `SittingTable` and `EndSittingSheet`
are removed with their tests, and their help terms retired or moved.

## 7. Not here, later

Structured intent items on the desk; drawings and remarks (slice B);
two records on one desk; a learning sitting; the room at phone width
beyond the stack.

## 8. Verification and box

Go: `Walks` per purpose and a refused unknown part; `present` admitted
in every sitting; the mark's `tree` written at Start from head; source
reads at the mark's tree for a shaping sitting and at the review head
for a review; `/changes` refused without `since` for a shaping sitting
and `since` comparing the mark's tree with head; the moved-head banner's
compare. Frontend: Start on a design and on an intent opens the room
with the purpose's word; the desk opens on the record's first section
with the sections in the strip; anchors put lines on the desk at the
mark's tree; the four walks; selection offers Ask and Fact and the fact
card carries the anchor; a case card's two presses; the board shows the
piles; the door on the record page and the Sittings tab; the moved-head
banner and Read at the head now; the drawer shows no sitting and
`/brain` no table; the cut guard's rows; the guards green. Walkthrough:
a canned opening turn on a design record, a Today walk that presents a
file, a case decided; screenshots at 1280 and 400, light and dark.
Landing checks: the structural tests the fast gate does not run (the
idempotency rows, the public surface, the path manifest, the wall-time
audit) beside devgate static, vitest, typecheck and the rebuilt bundle.
Box: one build lane (Claude on Opus 5.5), one code read (Codex on Sol)
with one fix round under R-124, after Astra's read of this page; two
attempts, 180 to 300 job-minutes.

## 9. Self-grade

High on D1, D3, D4, D5 and D7: an address, a key on a table, a lifted
gate and the piles the table already shows. High on D6: retiring code.
Medium on D2: the one new idea is the mark's `tree`, which makes a
shaping sitting's desk as stable as a review's; it is one field and one
compare. Weakest: the intent record on the desk is prose sections,
where a structured view of users and outcomes would serve better; that
waits until a sitting on intent has shown what it wants.
