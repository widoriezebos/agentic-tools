# g1-s55: the sitting, step 2 — cases, the close, the list

- Kind: design
- Id: 01M3F0YA7PA8PCDHZPGJAXDWC8
- Status: draft
- Goals: browser-interface

Wido, 2026-09-26: "I want all the open ones designed and implemented
now." Author Fable. Step 2 of g1-s53 (its D6, D9, D10 and the small
things its first sitting showed); every cite re-read at `e4b86cfd8`.

## 1. What exists and binds

g1-s53 step 1 as built: the sitting on the conversation with its two
routes; the interface's opening turn; the `deposit` tool for facts,
decisions and open questions; Record it serialized against one reading
of the record, appending to its four headed sections; the table beside
the transcript and the four counts in the drawer's bar; the conversation
store under the account's home. The question route settles a question
with an answer reference or a withdrawal and `askQuestion` opens one
(httpd/write.go, project/Sheet.tsx); the record status route sets
draft, accepted, superseded or done; Project's tabs list records by kind
(project/ProjectPane.tsx). The built page wraps the closed drawer bar's
title at 1280 while a sitting stands, and the table leaves a screen of
blank under itself at phone width (g1-s53 Built).

## 2. Decisions

- D1. **Cases as cards** (g1-s53 D6). The `deposit` tool gains the kind
  `case`: a case at the edge the Partner names, with the text and, where
  it has one, the clause it would become. A case card shows the case and
  two presses: **Decide** opens a small sheet with the clause as heard
  and a reason line, and records a decision; **Leave open** records an
  open question with the case as its text and the consequence the
  Partner gave. Both write through the same serialized Record it. A case
  never lands on the table by itself; a case dismissed is gone.
- D2. **End the sitting** (D9). The drawer's sitting control gains End;
  pressing it submits one more interface turn with a fixed request: the
  closing deposit, a `proposal` of kind `outcome` carrying the outcome
  as decided, the constraints, the open questions with their
  consequences, and what the table holds, drafted from the record's
  sections and the transcript. Its card offers Record it, which appends
  an "Outcome" section to the record (or replaces the one the record
  has, under the same reading rule), then the sitting ends: the
  conversation's sitting mark is cleared and the chip goes. Each open
  question on the table offers **Ask it**, which opens the Project
  pane's question sheet prefilled with the question and the record as
  its scope; the record's status stays the human's act on the record
  page. End without recording is allowed and says the outcome was not
  written.
- D3. **Resume** (D10). A conversation whose sitting mark stands resumes
  with its chip and its table on load; when the Partner's session is
  fresh, the interface submits the opening turn again with "resuming"
  in its request, so the first words say what is on the table. **Project
  → Sittings**: a tab listing the records that carry a sitting section
  (any of the four headings, or Outcome), newest entry first, each row
  the record's title, its kind, the counts of its four piles, when its
  last entry was recorded, and whether a sitting stands on it now;
  pressing a row opens the conversation focused at `/brain` with that
  record as the sitting's subject, starting a sitting on it if none
  stands.
- D4. **The two things the first sitting showed.** The closed drawer bar
  keeps its title on one line at 1280 with a sitting standing, by
  truncating the sitting chip before the title; the table takes only the
  height of its entries at phone width.
- D5. **Nothing else.** No review or learning purposes, no weighing on
  request, no multi-human sittings.

## 3. Payload

`deposit` gains kinds `case` and `outcome` with the fields above; the
conversation's `sitting` is cleared by `/api/partner/sitting/end` as
today; `GET /api/project` gains `sittings: [{record: {kind, id, title},
counts: {facts, proposals, decisions, questions}, lastAt, standing}]`.

## 4. Verification and box

Go: the two new kinds' bounds and forms; the closing turn's fixed
request and its provenance; the Sittings composition over records with
and without the sections. Frontend: the case card's two presses and
their writes; End with and without recording; Ask it prefilling the
question sheet; the Sittings tab's rows and the press that opens the
conversation; resume on load and after a fresh session; the bar at 1280
and the table at 400. Walkthrough: a canned case and a canned outcome.
Screenshots at 1280 and 400: a case card, the Decide sheet, the outcome
card, the Sittings tab. Budgets as always. Box: one build lane (Claude
on Opus), one code read (Codex on Sol) with one fix round under R-124,
after Astra's read; two attempts, 150 to 240 job-minutes.

## 5. Self-grade

High on D1 and D2: two more kinds through the pipeline that exists, and
one more fixed-request turn. Medium on D3's list: a composition over
records by their headings, which is plain and enough. Weakest: End
without an outcome recorded leaves the record as it was, which is
honest but easy to do by accident; the control says so before it ends.
