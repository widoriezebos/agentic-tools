# g1-s56: Use and save, and the Partner's leftovers

- Kind: design
- Id: 01M3F0YA80SGFHCGPYEHC6D500
- Status: done
- Goals: browser-interface

Wido, 2026-09-26: "I want all the open ones designed and implemented
now." Author Fable. Step 3 of g1-s52 and the deferred lists of g1-s51
and g1-s52; every cite re-read at `e4b86cfd8`.

## 1. What exists and binds

The edit sheet's save reads its draft and its "nothing changed" guard
from the render and guards busy on its Save button alone
(src/backlog/EditSheet.tsx); Astra's F1 and F2 on g1-s52 named why a
second caller cannot use it. The proposal block under a field offers
Use this and Undo (src/partner/FieldProposals.tsx). Deferred by the
reads: Undo's final guard compares a trimmed value (store.tsx); two
sheets of one name open at once share a draft attachment by name
(AskSheet.tsx, attachments.ts); a dismissed card stays folded after its
sheet closes, Copy appearing only on reopen (suggesting.ts); "n more"
opens the drawer at the next-newest card rather than unfolding in
place; the chip carries both the field words and "writing in <field>".

## 2. Decisions

- D1. **The sheet owns one submission path.** `EditSheet` gains
  `submit(next: Draft): Promise<Outcome>` with `Outcome = saved |
  refused(words) | unresolved(words)`: it computes the delta from the
  explicit next draft against what the sheet opened with, refuses an
  empty delta in words, guards busy inside itself with a synchronously held ref, not render
  state (a second call while one is in flight answers `refused("a save
  is in flight")`), sends through the existing route with Save's own
  validation, once-only sign-in retry and busy dismissal protection, and
  answers what happened by a conservative mapping of what the route
  returns: a confirmed response is `saved`; a local validation failure
  or a definite server rejection is `refused(words)`; an ambiguous
  engine refusal, a transport failure or an unreadable answer is
  `unresolved(words)`, after which the page rereads and never resubmits
  by itself; the route's `proof-not-recorded` answer, which says the
  act landed and must not be repeated, is shown in those words and never
  treated as a refusal (internal/ui/act/act.go:467-485; acts.go:488).
  Save calls it with the current draft; nothing else changes for Save.
- D2. **Use and save.** A proposal in the edit sheet gains a second
  button: the sheet builds the next draft with the words, sets it and
  calls `submit` with that same value in one act, never waiting on a
  render; the card and the block say "Used
  and saved" only on `saved`; on `refused` the words stay in the field, the block says "used; the
  save was refused: <words>" and the draft stays editable with Save
  under its normal validation; on `unresolved` the block says so and the page re-reads.
  Other sheets get Use this alone. Undo is not offered after a save.
  When a save closes the sheet, as Save does, "Used and saved" survives
  on the card in the transcript.
- D3. **The leftovers.** Undo's final guard reads the field's raw value
  through a raw reader on the registration and compares it whole; draft attachments are keyed, refreshed and retired by opening id, a
  draft living until its own opening closes or its own ✕ is pressed and
  never removed by another sheet of the same name (attachments.ts:103,
  143; AskSheet.tsx:101), the sheet name kept for the chip's words, so
  two same-named sheets keep their own; a card's closed state takes precedence over
  dismissed, so Copy shows once the sheet is gone; "n more" unfolds the
  older proposals in place under the newest, each with its own Use this;
  the chip shows "Draft: Edit goal · g1-s12 · writing in Intent", the
  field words dropped once a field is in hand.
- D4. **Nothing else.** No diffs, no multi-field proposals, no direct
  performance.

## 3. Verification and box

Frontend: submit's delta from an explicit draft, its empty-delta and
in-flight refusals, its three outcomes by the mapping, the landed-but-
unrecorded answer shown in its own words; two calls in one render
refused by the ref guard; Use and save on each outcome
with the words kept on refusal; Undo's raw compare; two same-named sheets' attachments, closing either leaving the other;
"Used and saved" surviving the sheet's close; the closed-over-dismissed card; "n more" in place;
the chip's words; the guards stay green. Walkthrough: a canned refusal
for the save. Screenshots at 1280 and 400: Use and save succeeding, and
refused. Budgets as always. Box: one build lane (Claude on Opus), one
code read (Codex on Sol) with one fix round under R-124, after Astra's
read; two attempts, 120 to 180 job-minutes.

## 4. Self-grade

High on D1: the path the two reads asked for, owned by the sheet.
Medium on D2: one press does two acts, and the words say which landed.
Weakest: the attachments keyed by opening change a lifetime rule the
chip has kept by name since it existed; the test for two same-named
sheets is what holds it.

## Dispositions (Astra read, 2026-09-26, under R-124)

Two material findings, four deferred; every code claim checked.

| id | finding | fold |
|---|---|---|
| F1 | the route has no literal unresolved outcome: the act layer collapses every publication error to `refused`, and one answer says the act landed and must not be repeated | a conservative mapping: confirmed is saved, definite rejection refused, ambiguous is unresolved with a reread and no resubmission, and the landed-not-recorded answer kept in its own words |
| F2 | attachments keyed by opening but retired by name would let one same-named sheet's close remove the other's draft | a draft lives until its own opening closes or its own ✕; name kept for the words only |

Folded because they cost nothing: the busy guard is a synchronous ref;
the sheet builds the next draft itself and submits that value; Save
keeps its validation, sign-in retry and closing; "Used and saved"
survives the sheet's close on the card; the words "Save follows its
normal validation". Deferred: which of two same-named drafts the
question carries, the last attached today; chip wording for other
sheets.

## Built (2026-09-26)

Landed on ui-development and main as 69c5c49c7; the read's one fix
landed as a second merge. Built by Claude on Opus, read by Codex on Sol
under R-124: one material finding, fixed in one round. The Project page
answered an unconfirmed save by reloading itself, which unmounted the
sheet with the human's words and the warning that says not to run the
act again; it now takes the board the sheet already read and sets it in
place, so the sheet stays and the ledger is read once. The Decisions
page and the Board were already right. The fix's wiring test fails
against the unfixed code.

What the read verified: Save and Use and save send the delta from the
opened baseline to the draft given; the busy ref is set before the
request; a completed sign-in retry settles the original press; nothing
unconfirmed is ever called saved or refused.

Later, when it hurts: a 4xx without a readable server sentence is
called refused rather than unresolved (every edit route's 4xx carries a
sentence today); a sign-in the human abandons leaves the card saying
"Used · saving…" until they act again, which claims nothing false and
retries nothing; the walkthrough cannot produce an unresolved outcome,
so that path is proven by the mapping's tests only; a concurrent close
of the goal by someone else would still unmount the Project page's
sheet.

Departures the read accepted: older proposals unfolded in place carry
the newest proposal's whole foot, Use and save included; the chip keeps
the draft's goal id and drops only the second field's words.

