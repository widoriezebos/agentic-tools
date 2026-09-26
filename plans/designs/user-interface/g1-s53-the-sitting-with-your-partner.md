# g1-s53: the sitting, with your Partner

- Kind: design
- Id: 01M3EBD883X0CCCEYEB8W9K5AP
- Status: draft
- Goals: browser-interface

Wido, 2026-09-26: "read docs/paper and understand sittings. Then from a
UX perspective: what are my human goals during the sitting and how can
the UI support this? So come up with a UX design that addresses these",
and "I really want to have a close working relationship with the project
partner during the sitting. So focus on collaboration with the project
partner." Author Fable. Every cite re-read at `bf554766c`.

## 1. What a sitting is, and what binds

1. **The paper's definition** (docs/paper/15-the-sitting.md): a working
   conversation between a responsible human and the machinery, whose
   results enter the record. The machinery is there "only in functions
   that judge nothing": it retrieves earlier rulings and standing rules
   that touch the topic, anchors every claim about current behaviour in
   the application itself, produces the cases at the edge and shows where
   two wishes conflict, and writes the record as the conversation moves.
   The room has no authority: "the sitting proposes; only records rule".
   The sitting has no memory: it deposits three kinds of record while it
   runs, facts anchored to where they can be checked, decisions recorded
   then and there with the human's reasons, and open questions named. One
   test covers all: end it at any moment and a fresh worker with the same
   human must continue from the records alone. It ends in recorded intent
   or a framed design with its open questions, and the examiner of that
   work was never in the room. It must not become a status meeting, an
   approval for show, or a preparation that quietly supplies the values:
   "when every option comes pre-weighed, choosing the recommended one
   every time is the approval habit in conversational form".
2. **The master's contract** (user-interface-design.md:93-97, 395-404,
   489): the dock holds the conversation, a visible list of shared
   subjects, current agent activity and proposed or completed actions;
   navigating never replaces the conversation; a sitting maintains four
   visible kinds of working material, facts, proposals, decisions and open
   questions, "saved during the discussion", and "saving a tentative
   statement does not make it an authoritative decision"; Project →
   Sittings is "resumable conversations with their working records and
   resulting artifacts", a view over owned documents, "not a second
   store"; the transcript itself is private sitting material outside the
   checkout (:381), examiners never read it (:896).
3. **What exists.** The Partner reads and explains and, since g1-s51 and
   g1-s52, proposes text into a field the human is writing; it has one
   stdio tool server with read operations and `suggest`
   (internal/ui/uitools/uitools.go:65-84); every read result names its
   source and revision (uitools.go:10-22). The conversation is one
   transcript with captures, looks and suggestions (internal/ui/partner);
   the drawer and the focused view at `/brain` show it. Records are
   documents of kinds intent, doctrine, decision, design, and questions
   (internal/project/project.go:45-72); the interface creates a record,
   sets its status, names goals on it, asks a question, settles one and
   edits a document with a revision check (httpd/write.go:33-45,
   describe.go:55-67; project/edit.go:119). The Project pane's "New
   decision" and "New question" sheets write them (project/Sheet.tsx).
   Nothing today names a sitting, brings what the records hold when one
   starts, or deposits anything while it runs.

## 2. Your goals in a sitting, and the room that serves them

You sit down with a wish, a question or a report, and a partner. What you
want, in the order it happens:

- **G1. Shape it.** Turn "sessions should time out" into an outcome
  precise enough to build against: the outcome, the constraints, the
  cases at the edge, and what you leave open on purpose.
- **G2. Argue from facts.** Every claim about what the application does
  today is anchored where you can check it; you never argue from memory.
- **G3. Decide, in your own words, and be held to it.** A choice is
  recorded when you make it, with the reason you gave, not reconstructed
  later; the pile of decisions is in view.
- **G4. Leave the right things behind, all the time.** If the session
  died this second, a fresh partner and you could go on from the record.
- **G5. Keep the values yours.** The partner brings cases, conflicts and
  consequences; it does not weigh them for you.
- **G6. Never lose the thread.** Across pages, across a session ending,
  the conversation and its subject stay together; you come back to it.
- **G7. End well.** Know when it is done; the result goes to the governed
  loop, and the examiner is someone who was not in the room.

The room, from your chair:

You open a design record, or a goal, or nothing yet, and press **Start a
sitting**. You name what it is for: shape intent, shape a design, review,
learn. The Partner's first turn is not a greeting: it brings what the
records hold. "Two rulings touch this: R-93 on blocker parks and R-105 on
attorney unparks. The intent record says sessions last twelve hours; the
code under internal/session says the mobile client renews differently
(session.go:212). Nothing recorded says what the current limit
protects." Each claim carries its anchor as a chip you can open.

Beside the conversation stands **the room's table**: four piles, Facts,
Proposals, Decisions, Open questions, each card with its words, its
anchor or its reason, who put it there, and when. They are not a second
store: they are sections of the subject's record, written as you go. The
table is empty when you start and fills as you talk.

You say the wish. The Partner answers with the cases at the edge: "a
person reads a long page without touching anything; a laptop sleeps with
the page open; a session created last week meets the new rule". Each
case is a card with two buttons: **Decide** and **Leave open**. You press
Decide on the first; a small sheet holds the clause the Partner heard,
"the limit counts from last activity", and a line for your reason, which
you write or accept as heard. It lands on the Decisions pile and in the
record, then and there. You press Leave open on the last; it lands on
Open questions, named, with the consequence the Partner attached.

Whenever the Partner states a fact, it offers it as a Facts card with its
anchor; whenever it hears you decide, it offers a Decisions card with the
reason it heard; whenever you decline to settle something, an Open
question card. It never records by itself: **Record it** is your press,
and the card says "recorded" only after the record took it. A card you
edit before recording keeps your words.

The Partner never says "I recommend". When it lays out options, each
carries its consequences and nothing else; the help term says why. If a
run of sittings shows your decisions always matching the first option,
that is evidence, and the record can show it later.

You move pages: open the code the anchor points at, look at the board,
come back. The conversation is where you left it; the subject chip still
says which sitting this is. The session dies; you reopen the browser; the
sitting resumes from the record and the transcript, and the Partner's
first turn says what is on the table.

You press **End the sitting**. The Partner drafts the closing deposit into
the record: the outcome as decided, the constraints, the open questions
named with their consequences, and what the table holds. You read it in
the record, edit it as you like, and set the record's status yourself:
draft stays draft, or you accept it. Open questions go to the register
with one press each. The design then goes where designs go, to an
examiner who was not in the room. Project → Sittings lists the sitting
under its subject, resumable.

## 3. Decisions

- D1. **A sitting is a record with a conversation attached.** Its subject
  is one record, intent or design, existing or created at the start as a
  draft; the sitting's working material is four sections of that record,
  "Facts", "Proposals", "Decisions", "Open questions", each a list of
  dated entries. The transcript stays the private sitting store. Nothing
  is a second store; the end-at-any-moment test holds because the record
  is the memory.
- D2. **Start a sitting** from a record page, a goal page or the drawer's
  header: purpose (shape intent, shape a design, review, learn) and
  subject (this record, a new draft named now, or none for a learning
  sitting). It attaches the subject as the draft chip and marks the
  conversation with the sitting, so every turn carries it.
- D3. **The opening turn is the Partner's, automatic:** what the records
  hold about the subject: rulings touching it, decisions about it, open
  questions on it, the intent and design records that name it, and the
  facts its own read tools can anchor; nothing weighed. It uses the read
  tools that exist, in one turn the interface submits on the human's
  behalf with a fixed request, shown in the transcript as such.
- D4. **One tool, `deposit`,** beside `suggest`: `{kind: fact | proposal
  | decision | question, text, anchor?, reason?, consequence?}`, prepared
  the way a suggestion is, admitted by the service against the sitting's
  subject, shown as a card in the transcript and on the table, never
  written by the tool. A decision card requires a reason before Record
  it; a fact card requires an anchor; a question card carries its
  consequence where the Partner gave one.
- D5. **Record it is the human's press,** and it appends the entry to the
  record's section through the existing document edit with its revision
  check; a conflict is shown as the sheet shows one; "recorded" appears
  only after the write returns. Edit before recording keeps the human's
  words; Dismiss folds the card.
- D6. **Decide and Leave open on the Partner's cases.** A case the Partner
  lists is a card; Decide opens a small sheet with the clause as heard
  and a reason line, recording a decision; Leave open records an open
  question with the consequence.
- D7. **The table** is a panel beside the transcript in the focused view
  at `/brain` and a strip of four counts in the drawer that opens it;
  it reads the record's four sections and shows each entry with its
  anchor or reason, who and when; pressing an entry opens the record at
  it.
- D8. **No recommendation by default.** The Partner's instructions gain
  the paper's rule for sittings: options carry consequences, not a
  weighting; it may weigh only when the human asks it to. The help term
  says why.
- D9. **End the sitting** drafts the closing deposit as one proposal card
  (outcome, constraints, open questions, what the table holds); Record it
  writes it as the record's "Outcome" section; the record's status is the
  human's existing act; each open question offers "Ask it" through the
  existing question route.
- D10. **Resume.** A conversation marked with a sitting resumes with its
  subject chip; a fresh session's first turn is the opening turn again,
  from the record as it stands. Project → Sittings lists records that
  carry a sitting section, by subject, newest first, opening the
  conversation at that sitting.
- D11. **The examiner was not in the room.** Nothing changes in how a
  design reaches Astra; the transcript is never an input, as today.

## 4. Step 1, the smallest thing that works

D1, D2 from a record page and the drawer header, D3 the opening turn with
rulings, decisions, open questions and records that name the subject, D4
and D5 for facts, decisions and open questions (proposals wait), D7's
table in the focused view with the drawer's four counts, D8's rule. Not in
step 1: D6's Decide and Leave open on cases (the human records them
through D4 by asking), D9's closing deposit and "Ask it", D10's Sittings
list (the record itself is the resumable thing), purposes other than
"shape a design" and "shape intent". Step 2: D6, D9, D10.

## 5. The payload and the routes

The conversation gains `sitting: {subject: {kind, id}, purpose, startedAt}`
or null; `POST /api/partner/sitting` starts one (subject or a new draft's
title, purpose) and `/api/partner/sitting/end` ends it, both with the
checkout-write policy; `Message.deposits[]` and a `deposit` stream event
in the shape of suggestions with `offered` and `reason`; Record it is
`POST /api/project/documents/<id>/edit` with the section appended and the
revision the card read, through the existing route. The record's four
sections are plain headings the document reader already renders.

## 6. Not here, later

Review and learning sittings' own moves (the narrator's report in the
room; withheld diagnoses). Multi-human sittings and their negotiation
record. The Partner weighing options on request. Budgets and usage in
the dock header. The "evidence worth examining" view of rulings that
always match the preparation.

## 7. Verification and box

Go: the deposit tool's bounds and fixed form; the host and service
carrying deposits like suggestions; the sitting on the conversation and
its routes; the opening turn's fixed request and its capture. Frontend:
Start a sitting from a record page and the header; the subject chip; the
opening turn shown as the interface's; the deposit card's states and the
Record it write with a revision conflict; the table reading the four
sections; the drawer's counts; the guards stay green. Walkthrough: the
fake Partner answers a canned opening turn and two deposits; screenshots
at 1280 and 400: the start sheet, the opening turn, a fact card with its
anchor, a decision card with its reason, the table. Budgets as always.
Box: one build lane (Claude on Opus), one code read (Codex on Sol) with
one fix round under R-124, after Astra's read; two attempts, 180 to 300
job-minutes.

## 8. Self-grade

High on D1 and D5: the record is the memory and the human's press is the
write, both through what exists. Medium on D3: an automatic first turn is
the first time the interface submits a request on the human's behalf; it
is shown as such and it reads only. Medium on D4: a second prepared-result
tool in the suggest pattern. Weakest: the table as four sections of a
record is plain, and a sitting that produces forty entries will want
structure the sections do not give; that is the "later, when it hurts"
list's first line.
