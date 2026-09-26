# g1-s56: Use and save, and the Partner's leftovers

- Kind: design
- Id: 01M3F0YA80SGFHCGPYEHC6D500
- Status: draft
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
  empty delta in words, guards busy inside itself (a second call while
  one is in flight answers `refused("a save is in flight")`), sends
  through the existing route, and answers what happened. Save calls it
  with the current draft; nothing else changes for Save.
- D2. **Use and save.** A proposal in the edit sheet gains a second
  button: it calls the sheet's setter with the words and then `submit`
  with the resulting draft in one act; the card and the block say "Used
  and saved" only on `saved`; on `refused` the words stay in the field,
  the block says "used; the save was refused: <words>" and Save is
  available; on `unresolved` the block says so and the page re-reads.
  Other sheets get Use this alone. Undo is not offered after a save.
- D3. **The leftovers.** Undo's final guard reads the field's raw value
  through a raw reader on the registration and compares it whole; draft
  attachments are keyed and refreshed by opening id, so two same-named
  sheets keep their own; a card's closed state takes precedence over
  dismissed, so Copy shows once the sheet is gone; "n more" unfolds the
  older proposals in place under the newest, each with its own Use this;
  the chip shows "Draft: Edit goal · g1-s12 · writing in Intent", the
  field words dropped once a field is in hand.
- D4. **Nothing else.** No diffs, no multi-field proposals, no direct
  performance.

## 3. Verification and box

Frontend: submit's delta from an explicit draft, its empty-delta and
in-flight refusals, its three outcomes; Use and save on each outcome
with the words kept on refusal; Undo's raw compare; two same-named
sheets' attachments; the closed-over-dismissed card; "n more" in place;
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
