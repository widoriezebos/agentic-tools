# g1-s52: the Partner proposes where you write

- Kind: design
- Id: 01M3E64FM8AGQ5K8KSMB2RZ50T
- Status: draft
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
Undo", Save is yours. Or you press "Use and save" and the sheet saves in
the same press, which is the one act the master allows the Partner's
words to become, on your press, for a queued goal's intent, next step
and labels.

What changes, and why:

- **Opening the sheet is the hand-over.** The chip appears when a sheet
  with writable fields opens and goes when it closes or when you press
  its ✕, and the Partner may propose for those fields from then on. No
  hidden first press. The rule stays honest because the chip is on
  screen the whole time.
- **The proposal is where you write.** It renders under the field it
  names, inside the sheet; the transcript keeps the card as the record.
  The drawer's height stops mattering.
- **A refusal is said where you can read it.** If the Partner proposes
  for a field that is not open, the drawer shows it under the answer as
  a card in its closed state: "Intent is not open for proposals; open
  the sheet and ask again". The Partner's tool result says the same, so
  it never narrates a card that was not offered.
- **The entry is on the field.** "Ask the Partner" beside each writable
  field's label fills the composer with a request for that field and
  takes the caret there; the human edits or sends.
- **Use and save.** Beside Use this on the proposal, when the sheet is
  the edit sheet: one press that uses the words and saves the sheet with
  its existing save, the same act as pressing Save after Use this. Never
  without the press; never on any other sheet in this step.

## 3. Decisions

- D1. **Hand-over on open.** `AskSheet` hands the draft over when a sheet
  with writable fields mounts, exactly what "Ask about this" does today,
  and takes it back on unmount or on the chip's ✕; "Ask about this" stays
  and does the same for a sheet whose chip was removed. The service's
  admission is unchanged: a handed-over draft with its opening and
  writable fields.
- D2. **The proposal renders under its field.** The sheet places one
  `FieldProposals` beside each writable field, from the store's
  suggestions for that opening and field: the newest waiting proposal
  whole, Use this, Dismiss; after use "Used · Undo" with g1-s51's Undo
  rule; older ones behind "n more". The transcript card stays and both
  read the same store state.
- D3. **A refused proposal is shown.** The service records a refused
  suggestion with a reason instead of dropping it to an activity line;
  the transcript renders it as a card in a "not offered" state with the
  reason; the tool's result says "prepared; it is offered only if that
  field is open on a sheet the human has on screen".
- D4. **"Ask the Partner" on the field.** One small link the sheet places
  beside each writable field's label; pressing it puts "Suggest a better
  <field>" in the composer, or "Suggest a <field>" where the field is
  empty, and takes the caret there; it sends nothing.
- D5. **Use and save on the edit sheet.** A second button on a proposal
  in the edit sheet only: uses the words and calls the sheet's existing
  save, so the same route, refusals and busy guard apply; the card and
  the block say "Used and saved". Other sheets get Use this alone.
- D6. **Nothing else moves.** No direct performance by the Partner, no
  document editor, no stickies composer.

## 4. Payload

None on the wire beyond D3: `Message.suggestions[].offered: true|false`
and `reason` on a refused one, and the same on the stream event.

## 5. Not here, later

The document editor and the stickies composer (g1-s51 step 2 items, now
step 3). The Partner saving on its own request. Proposals for several
fields at once as one act. Diffs.

## 6. Verification and box

Go: the service recording a refused suggestion with its reason and the
stream carrying it; the tool's result words. Frontend: the chip appearing
on open and going on close and on ✕, the draft carried by a question
without a prior press; the proposal block under the field with its
states; the field link filling the composer; Use and save calling the
sheet's save once and refusing while busy; the transcript's not-offered
card; the guards stay green. Walkthrough: the fake Partner's canned
suggestion arrives without "Ask about this"; screenshots at 1280 and 400:
the chip on open, the proposal under Intent, "Used · Undo", the
not-offered card. Budgets as always. Box: one build lane (Claude on
Opus), one code read (Codex on Sol) with one fix round under R-124,
after Astra's read; two attempts, 120 to 180 job-minutes.

## 7. Self-grade

High on D1, D2, D4: they move existing pieces to where the human looks.
Medium on D3: a refusal becomes a visible object, which is right, and a
second card state. Medium on D5: one press does two things the human
could do in two; it is the act the human asked for by name, and it is
one sheet. Weakest: the chip on open is the human's consent by being
seen; if a sitting shows it is not noticed, "Ask about this" returns as
the gate.
