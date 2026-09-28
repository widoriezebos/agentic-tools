# g1-s67: the room for every sitting — shaping a design in the same room as reviewing the work

- Kind: design
- Id: 01M3MGGN6WHCZ9C9Z4CXH7XW7Q
- Status: draft
- Goals: browser-interface

Wido, 2026-09-28, after his first look at the review room: "but I also
want to be able to discuss goal designs this way; does that not make
sense?" and, on the answer, "ok, continue". Author Fable. Every cite
re-read at `7ccd22250`. Revision 2 folds Astra's round 1 (Dispositions
at the foot): the shaping desk reads the checkout as it stands rather
than a pinned commit, and End is by purpose.

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
   the document reader opens the checkout root for a record's bytes as they stand. The cut
   guard and the CSP as before.

## 2. What you want when you shape a design

- **S1. The same room.** One place to sit, whatever the chair: the
  record and the code on the desk, the colleague beside it, the piles
  on the board.
- **S2. The record on the desk.** The design you are shaping, section by
  section, large, beside what the code does today.
- **S3. Arguing from the code.** "What does the application do today?"
  answered with the lines on the desk, as the checkout has them today,
  the same file the colleague read before answering.
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
desk shows those lines as the checkout has them, the file the Partner
just read. You select four of them and press Fact:
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
- D2. **The desk of a shaping sitting reads the checkout as it stands.**
  Records and source alike come from the checkout's own files, through
  the root the document reader already opens, so an anchor `path:lines`
  shows the lines as the checkout has them now: the same bytes the
  Partner's own reads see (its permission owner grants native reads
  anywhere inside the checkout, `internal/ui/partner/conversation.go:
  40`), uncommitted edits included. Nothing is pinned: no commit on the
  mark, no compare, no banner; a sitting that stood before this build
  opens like any other. The change index and the diff are not offered
  in a shaping sitting, since there is no change; the strip starts with
  the record's own sections, the current one on the desk. On return the
  desk reads its item again, so what it shows is what the code does
  today; a recorded fact's anchor names lines that may drift afterwards,
  as every cite in every record here does, and the record's date says
  when it was true. (Revision 1 pinned the desk to the head commit at
  Start; Astra S67-01 showed the Partner cannot be pinned, so the two
  would argue from different files.)
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
- D7. **Nothing else changes, and End is by purpose.** The record model,
  the recorder, the conversation store, the door line and Step out are
  the landed ones; a review sitting behaves exactly as it does today.
  The room's End sheet is the sitting's own: a review room keeps the
  landed sheet with its three verdicts and the nod line; a shaping room
  mounts the landed shaping sheet, End (the Partner drafts the Outcome)
  and End without recording, with no verdict control, exactly the two
  actions the drawer's sheet offers today (`partner/StartSitting.tsx:
  283-350`). The sheet moves; its behaviour does not.

## 5. Step 1, the smallest thing that works

All of D1 to D7 as one slice; it is half the size of the room's slice
A because every mechanism exists. Not in it: an intent record's users
and outcomes as structured desk items; drawings (room slice B);
several records on one desk; a sitting on a goal's intent line rather
than a record.

## 6. Payload and routes

`POST /api/partner/sitting` answers the room's address for every
purpose; the mark is unchanged. The desk's source read
(`/api/review/<record>/source`) accepts a record of any sittable kind:
a review reads the reviewed tree as today; an intent or a design reads
the checkout as it stands, one function beside `Owner.Source` that
opens the file under the checkout root and shares `Source`'s path
check, binary refusal, bounds and marks (`internal/ui/review/review.go:
527-560`). `/changes` is refused for a shaping sitting in words ("a
sitting on a design has no change to index; its desk reads the checkout
as it stands"). `Walks` keyed by purpose; the walk route unchanged; the
Today request says the Partner reads the checkout with its own reads
and answers with `path:lines` chips, the files the desk reads. `present`
admitted for every sitting: the landed test's "no desk item outside a
review" expectation and its activity line invert
(`internal/ui/partner/review_service_test.go:444-451`), the finding
refusal beside them stays. Routes: `/sitting/<record>` beside
`/review/<record>`, both `inTheRoom`; the room's End sheet is chosen by
the sitting's purpose (D7). The Start sheet, the record page's Start
control and the Sittings tab navigate to the room; the drawer's
`SittingControl`, `SittingCounts` and `SittingTable` are removed with
their tests, `EndSittingSheet` moves into the room with its test, and
their help terms are retired or moved.

## 7. Not here, later

Structured intent items on the desk; drawings and remarks (slice B);
two records on one desk; a learning sitting; the room at phone width
beyond the stack. The head commit as provenance on a fact card's
anchor, when a drifted anchor has misled someone. "Read again" on a
desk item while the sitting stands, when the code has moved under
someone who did not step out. A review Partner's native reads of the
checkout beside the candidate's tree, which is g1-s65's state and not
this design's.

## 8. Verification and box

Go: `Walks` per purpose and a refused unknown part; `present` admitted
in every sitting and `finding` still refused outside a review; the
source read of a shaping sitting returns the checkout's bytes as they
stand (a test edits a file in the checkout without committing and reads
the edit), with `Source`'s bounds, the binary refusal and a path that
escapes the root refused; a review's source read unchanged; `/changes`
refused for a shaping sitting in words; a shaping Outcome recorded
without a verdict and a review Outcome refused without one, as landed.
Frontend: Start on a design and on an intent opens the room with the
purpose's word; the desk opens on the record's first section with the
sections in the strip; anchors put lines on the desk as they stand;
the four walks; selection offers Ask and Fact and the fact card carries
the anchor; a case card's two presses; the board shows the piles; the
door on the record page and the Sittings tab, including a sitting whose
mark carries no room state; End in a shaping room offers End and End
without recording and no verdict, End in a review room unchanged; the
drawer shows no sitting and `/brain` no table; the cut guard's rows;
the guards green. Walkthrough:
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
gate, the piles the table already shows and a sheet that moves. High on
D6: retiring code. High on D2 since revision 2: one read of the
checkout's own files, the rule the document reader already follows, and
nothing to keep in step. Weakest: a fact's anchor can drift after the
sitting, and the intent record on the desk is prose sections, where a
structured view of users and outcomes would serve better; both wait
until a sitting has shown what it wants.

## Dispositions (Astra round 1, 2026-09-28, under R-121 and R-124)

Read of revision 1 at `bea2168e1`, verbatim in
`g1-s67-astra-critique.md`. Three material findings; every cited line
was re-read at whole-function depth before folding. Two are answered by
one smaller decision, the third by moving a sheet.

| id | finding | fold |
|---|---|---|
| S67-01 | the desk pinned to the head commit at Start while the Partner reads the checkout as it stands (`uitools.go:353`, `document.go:139`, native reads granted anywhere in the checkout, `conversation.go:40`): the answer and the lines beside it can be different code, silently when the head has not moved | D2 rewritten: the pin goes; a shaping sitting's desk reads the checkout as it stands, the same files the Partner reads, records and source alike. Astra's own change, a pinned Partner-facing read, was not taken: the Partner's native reads cannot be pinned, so one source is the only way both argue from the same lines, and it is the smaller design |
| S67-02 | a sitting that stood before this build has no `tree` on its mark (`conversation.go:552`), and opening it resumes rather than restarts (`service.go:1177`, `ProjectPane.tsx:1590`): its desk is either refused or shows an invented baseline | dissolved by the D2 fold: nothing is pinned, no mark carries a commit, and a sitting from before this build opens like any other; §8 keeps a test for a mark without room state |
| S67-03 | §6 deleted `EndSittingSheet` while D7 promised End unchanged; the room mounts the review sheet unconditionally (`ReviewRoom.tsx:291`), so a shaping sitting would inherit verdicts and lose End without recording (`StartSitting.tsx:293`, `:333`) | D7: End by purpose; the shaping sheet moves into the room with its two actions and its test; §6 and §8 say so |

Astra also verified, and the design leans on, that the Outcome
recorder refuses a missing verdict only for a review (`room.ts:328`)
and the service writes a verdict only on a review's Outcome
(`service.go:1293`), so a shaping Outcome records as it does today.
