# g1-s52: the Partner proposes where you write

- Kind: design
- Id: 01M3E64FM8AGQ5K8KSMB2RZ50T
- Status: done
- Goals: browser-interface

Wido, 2026-09-26, after the first sitting with g1-s51: "I asked the
project partner to rewrite the intent, and then it tells me that it
created a suggestion card for the intent field. However, I see nothing
… ideally, I just want to have the project partner update the intent …
Ideally, we have the project partner proposing something that I accept
or similar. So fix this and then come from the UX perspective." Author
Fable. Step 2 of g1-s51; every cite re-read at `3d0bf91d9`, the sitting
read from the live conversation.

## 1. What happened, and what binds

1. **The sitting.** With the edit sheet open and nothing handed over, the
   human asked "can you rewrite for me, make it shorter". The Partner
   called `suggest` for Intent; the service refused it, "a suggestion for
   Intent was not offered: no such field was handed over", as an activity
   line the drawer does not show; the Partner then said it had put a card
   on the sheet. The human saw nothing, asked, and was told the Partner
   cannot change fields. Three failures: a precondition the human never
   saw ("Ask about this" first), a refusal that reached nobody, and a
   proposal that would have lived in a drawer whose default height hides
   a card behind the composer (g1-s51 Built, observed).
2. **What g1-s51 built.** A sheet registers its opening id, writable
   fields and setters on mount (src/partner/AskSheet.tsx); the draft is
   handed over only by "Ask about this" (store.tsx `askAbout`), and the
   service admits a suggestion only against a handed-over draft
   (internal/ui/partner/service.go `admit`); the card renders in the
   transcript (src/partner/Suggestion.tsx); the field shows "n
   suggestions" once one is admitted.
3. **The master's rule on drafts** is that they are shared "only as
   explicitly identified draft context" (user-interface-design.md:111).
   An open sheet whose chip stands above the composer, removable, is an
   identified draft; the rule forbids silent sharing, not visible sharing.
4. **The master's rule on acts** stands: direct performance only on the
   allowlist and on explicit request (:119); "the agent must not report
   an edit as applied solely because it prepared a patch" (:123).

## 2. How it should work, from the chair

You open the edit sheet. A chip appears above the Partner's composer by
itself: "Draft: Edit goal · g1-s12", with an ✕. Beside the Intent field's
label there is a small "Ask the Partner" link. You press it; the
composer fills with "Suggest a better Intent" and you press Enter, or you
just type "make it shorter". The answer arrives. Under your Intent field,
inside the sheet, a proposal appears:

```
Intent                                          Ask the Partner
┌─────────────────────────────────────────────────────────────┐
│ scripts/receipt.sh add writes the MAIN checkout ledger …    │
└─────────────────────────────────────────────────────────────┘
╭ The Partner proposes ───────────────────────────────────────╮
│ A receipt added from a linked worktree lands in that        │
│ worktree's ledger, or the writer refuses and names the      │
│ command that writes it in the right place.                  │
│                                        [Use this]  Dismiss  │
╰─────────────────────────────────────────────────────────────╯
```

You press Use this; the field holds the proposal, the block says "Used ·
Undo", and Save is yours, one press away, at the foot of the same sheet.

What changes, and why:

- **Opening the sheet is the hand-over.** The chip appears when a sheet
  with writable fields opens, without opening the drawer or moving your
  caret, and goes when the sheet closes; its ✕ leaves the draft out of
  every later question. No hidden first press. The rule stays honest
  because the chip stands wherever the composer is shown, and the Seeing
  line says "Edit goal sheet open" whether the drawer is open or not.
- **The proposal is where you write.** It renders under the field it
  names, inside the sheet; the transcript keeps the card as the record.
  The drawer's height stops mattering.
- **A refusal is said where you can read it.** If the Partner proposes
  for a field that is not open, the drawer shows it under the answer as
  a card in its "not offered" state with the reason: "Intent is not open
  for proposals" and, where the draft was detached, "the draft was left
  out; press Ask about this to hand it over again". The Partner's tool
  result says that preparing a suggestion does not confirm it was shown;
  the card is the authoritative answer where the human reads.
- **The entry is on the field.** "Ask the Partner" beside each writable
  field's label fills the composer with a request for that field and
  takes the caret there; the human edits or sends.
- **The Partner knows which field you were in.** The sheet notices
  which writable field last held the caret and tells the store; the chip
  says it, "Draft: Edit goal · writing in Intent"; the capture carries it
  and the Partner is told "the human was writing in Intent", so "make
  this shorter" means that field, and a proposal for another field must
  name it in words. Before any field has held the caret, the chip says
  "writing in nothing yet" and the Partner names the field or asks. Wido,
  2026-09-26: "do you know which field I was editing when I started
  editing in the project partner panel? Because you will have to."
- **Use, then Save.** Two presses, both yours and both on the same
  sheet. One press that does both is step 3, once the sheet owns a
  submission path that takes the next draft explicitly, guards busy and
  reports its real outcome.

## 3. Decisions

- D1. **Hand-over on open.** `AskSheet` attaches the draft when a sheet
  with writable fields mounts, as "Ask about this" does today but
  without opening the drawer or taking the caret; it detaches on unmount,
  and the chip's ✕ leaves the draft out of later questions (a proposal
  already asked for keeps its opening's registration, as today);
  "Ask about this" stays and re-attaches a removed draft. The service's
  admission is unchanged.
- D2. **The proposal renders under its field.** The sheet places one
  `FieldProposals` beside each writable field, from the store's
  suggestions for that opening and field: the newest waiting proposal
  whole, Use this, Dismiss; after use "Used · Undo" with g1-s51's Undo
  rule; older ones behind "n more". The transcript card stays and both
  read the same store state.
- D3. **A refused proposal is shown.** The service records a refused
  suggestion with a reason instead of dropping it to an activity line;
  the transcript renders it as a card in a "not offered" state with the
  reason; the tool's result says "prepared; preparing does not confirm
  it was shown: it is offered only for a field of a draft the human has
  handed over".
- D4. **"Ask the Partner" on the field.** One small link the sheet places
  beside each writable field's label; pressing it puts "Suggest a better
  <field>" in the composer, or "Suggest a <field>" where the field is
  empty, and takes the caret there; it sends nothing.
- D5. **Use this, then the sheet's own Save.** No combined button in
  this step: the edit sheet's save reads its draft from the render and
  guards busy on its button only, so a second caller would save the
  previous words or report a save it cannot confirm. Step 3 gives the
  sheet a submission path that takes the next draft explicitly, guards
  busy and returns its outcome, and then one press may do both.
- D6. **The field in hand travels.** Each writable field reports focus
  to the sheet, which keeps the last one and hands it to the store beside
  the draft; the chip shows it; the capture's draft carries `writing`;
  the Partner's context block says "writing in <field>" or "in no field
  yet"; the skill sentence gains "a request that names no field is about
  the field the human is writing in; when there is none, ask or name it".
  The field link of D4 sets it too.
- D7. **Nothing else moves.** No direct performance by the Partner, no
  document editor, no stickies composer.

## 4. Payload

The capture's draft gains `writing: "Intent" | ""`. Beyond that, D3 only: `Message.suggestions[].offered: true|false`
and `reason` on a refused one, and the same on the stream event.

## 5. Not here, later

"Use and save" as one press, on a sheet-owned submission path. Two
sheets of one name open at once, whose attachments share a name.
Revoking a proposal already asked for when the chip is removed. The
document editor and the stickies composer. The Partner saving on its
own request. Proposals for several
fields at once as one act. Diffs.

## 6. Verification and box

Go: the service recording a refused suggestion with its reason and the
stream carrying it; the tool's result words. Frontend: the chip appearing on open and going on close and on ✕, the
draft carried by a question without a prior press; the field in hand
following focus, shown on the chip and carried in the capture, empty
before any focus; the proposal block under the field with its
states; the field link filling the composer; the drawer not opening and the caret not moving on hand-over; the
transcript's not-offered card with its reason; the guards stay green. Walkthrough: the fake Partner's canned
suggestion arrives without "Ask about this"; screenshots at 1280 and 400:
the chip on open, the proposal under Intent, "Used · Undo", the
not-offered card. Budgets as always. Box: one build lane (Claude on
Opus), one code read (Codex on Sol) with one fix round under R-124,
after Astra's read; two attempts, 120 to 180 job-minutes.

## 7. Self-grade

High on D1, D2, D4: they move existing pieces to where the human looks.
Medium on D3: a refusal becomes a visible object, which is right, and a
second card state. Medium on D5: two presses where the human asked for one; the one
press waits for a submission path that can report the truth. Weakest: the chip on open is the human's consent by being
seen; if a sitting shows it is not noticed, "Ask about this" returns as
the gate.

## Dispositions (Astra read, 2026-09-26, under R-124)

Two material findings, both in the combined "Use and save", five
deferred; every code claim checked.

| id | finding | fold |
|---|---|---|
| F1 | the edit sheet's save reads its draft and its "nothing changed" guard from the render, so a second caller that first sets the words would save the previous delta or refuse | the combined press is step 3, on a sheet-owned submission path that takes the next draft explicitly; step 2 is Use this, then Save |
| F2 | the save reports no outcome and guards busy on its button only, so "Used and saved" could be said of a refused or unresolved request | the same: no combined button until the path returns its real outcome |

Folded because they cost nothing: hand-over on open attaches the chip
without opening the drawer or moving the caret; the ✕ leaves the draft
out of later questions rather than revoking a proposal already asked
for; the tool's result says preparing does not confirm showing, and the
refusal card names the detached case. Deferred, step 2 works without
them: two same-named sheets open at once; last-writer saves on one field,
inherited from Save. The field in hand (D6) was added after the read
began; Astra read the tip that carried it and called it what serves the
actual interaction.

## Built (2026-09-26)

Landed on ui-development and main; the slice went live before the read
and the read's fix landed as a second merge. Built by Claude on Opus,
read by Codex on Sol under R-124: one material finding, the draft chip
appeared only with the drawer open, so a sheet opened over a collapsed
drawer handed itself over invisibly with its ✕ unreachable; the chip now
stands beside the collapsed composer too. One deferred item folded
because it cost one line: a suggestion recorded before this build read
as refused; a refusal is now known by its reason.

Later, when it hurts: "n more" opens the drawer at the next-newest card
rather than unfolding in place; the chip carries both the field words
and "writing in <field>".

Departures the read accepted: the fixture needs `--partner fake`; the
chip appends the field-in-hand clause to its label; g1-s51's "n
suggestions" link is gone, superseded by the proposal under the field;
testing.json is unchanged because the touched paths fall to groups that
run the whole packages.
