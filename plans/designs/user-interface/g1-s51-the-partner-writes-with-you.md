# g1-s51: the Partner writes with you

- Kind: design
- Id: 01M3D6T9QTZ7WDE2HTXX9TH77M
- Status: draft
- Goals: browser-interface

Wido, 2026-09-25: "Can I discuss the goal while I'm editing the goal with
edit goal? And can the agent, the project partner, contribute to what I'm
writing in the modal dialogue?", then "In general, I want the agent to be
able to read and contribute to what I'm writing in any editor", then
"Please design for all of this, especially the project partner
contributing to something that I'm writing. Also start with the UX design
on this. We need to put the UX central on this feature and design from
great UX. So start with a great UX design on this. How would this work?
Then design and implement the smallest thing possible that works". Author
Fable. Every cite re-read at `be5539616`.

## 1. What exists and binds

1. **Discussing while editing already works.** A board sheet renders into
   the work area's own layer; its scrim dims that box and no more, the
   work behind it is inert, and focus is not held inside, so the Partner
   drawer stays usable (src/backlog/Panel.tsx:20-36). Its head carries
   "Ask about this", which hands the fields as they stand to the Partner
   as a chip above the composer; the chip lives while the sheet does, and
   every question sent while it stands carries the fields as they stand
   at that moment, read through `offerFields` (src/partner/store.tsx:96-130,
   400-430; src/partner/drafting.ts:20-25). The edit sheet offers Goal,
   Intent, Next step and Labels (src/backlog/EditSheet.tsx:94-98). In the
   document editor, a selected passage is sent with "Ask"
   (src/partner/AskSelection.tsx:32-100). The master keeps drafts explicit:
   "share unsaved edits only as explicitly identified draft context…
   keep the saved revision distinguishable from the draft"
   (user-interface-design.md:111).
2. **The Partner cannot write into anything.** Its tools are the
   interface's read tools, one stdio server, whose package rule is
   "Nothing here writes" (internal/ui/uitools/uitools.go:1-30); its
   answers are text, activity and looks (internal/ui/partner/host.go:56-80).
   A tool call becomes a look when it completes; the host reads the
   tool's result text line by line for `Source:` and the like
   (host.go:773-835). The help term says it: "it does not write and it
   does not act".
3. **The master wants exactly this, in this shape.** "Return relevant
   records, evidence, and editable proposals so the frontend can show
   their normal views" (:83); gate 4, "first complete edit: a queued goal
   is edited through the brain, shown in its ordinary editor, edited by
   the human, and discussed again" (:817), with "a reusable result card"
   (:360); "the agent must not report an edit as applied solely because
   it prepared a patch or sent a request" (:123); direct performance only
   on the allowlist, everything else proposal-first (:119).
4. **The editors.** Every board sheet is one `Panel` with its `fields`:
   edit, open, approve, withdraw, park, rank, the two bulk sheets. The
   document editor is one CodeMirror view with `onChange`
   (src/project/Editor.tsx:14-100). The stickies composer is a textarea
   in its panel. The Project pane's decision and question sheets use the
   shell's `Sheet`.

## 2. How it works, from the chair

You are in the edit sheet. The intent reads badly and you know it. You
press "Ask about this", type "tighten this, one line, outcome not work",
Enter. You keep looking at your field; nothing in it moves. A moment
later the drawer shows the Partner's answer and, under it, a card:

```
┌ Suggestion for Intent ─────────────────────────────┐
│ Every refund lands within a day, with nobody        │
│ touching the queue.                                 │
│                                   [Use this] Dismiss │
└─────────────────────────────────────────────────────┘
```

Beside your Intent field, a small link has appeared: "1 suggestion". You
press Use this. Your field now holds the Partner's sentence, the card
says "Used · Undo", and the field's link is gone. Nothing was saved: Save
is still yours, the sheet still says what it would send. You press Undo;
your own words are back. You press Use this again, edit two words, Save.

Everything the human wants from this, in the order it matters:

- **My draft is mine.** Nothing enters a field unless I put it there. A
  suggestion waits in the drawer; the field only says one is waiting.
- **I never lose what I typed.** Use this can be undone; typing after a
  suggestion arrived is never overwritten by it.
- **I can see whose words these are.** In the drawer a suggestion is a
  card, not prose; in the field, after Use this, they are my draft.
- **The Partner knows what I am writing** when I hand it over, and only
  then: Ask about this, or a selection sent with Ask. It reads the fields
  as they stand when I send, not as they stood when I pressed the chip.
- **The Partner never saves.** It proposes; I press Save; the record
  says I edited it. Where the master allows direct performance, that is
  a different act with its own button, not this.
- **It works the same everywhere.** A sheet field, a document passage,
  a sticky: one card, one Use this, one Undo.

The interaction, exactly:

1. **Ask.** With an editor open, "Ask about this" (a sheet) or "Ask" on
   a selection (a document) attaches the draft; the human writes the
   request in the composer, or presses one of two suggested questions
   the chip offers: "Suggest a better wording" and "Suggest the next
   step".
2. **Suggest.** The Partner answers in words and, when it has text to
   offer, calls `suggest` with the field's name and the text, once per
   field, at most three per answer. The card appears under the answer as
   the call completes, before the answer's last word if need be.
3. **Point.** The editor's field that the suggestion names shows "n
   suggestions" beside its label; pressing it opens the drawer at the
   card.
4. **Use.** Use this replaces the field's whole value with the
   suggestion's text (a sheet field), or the selected passage where it
   is still the passage the suggestion was for, else inserts at the
   caret (a document). The card turns to "Used · Undo"; Undo restores the
   value the field held at the moment of use. Dismiss folds the card to
   one line, "dismissed", which can be reopened.
5. **Stale.** If the editor the suggestion was for is closed, the card
   says "The Edit goal sheet is closed" and offers Copy. If the field
   changed since the ask, Use this still applies, because the human
   pressed it; nothing applies by itself.
6. **Nothing else moves.** No save, no act, no history line. The
   conversation keeps the suggestion and, in the human's next question,
   whether it was used, so the Partner can build on it.

## 3. Decisions

- D1. **One tool, `suggest`.** In the interface's tool server beside the
  read tools: `suggest {editor, field, text}`, text at most 4000
  characters, refused beyond and refused for a field the capture did not
  name as open. It writes nothing; its result to the model is one line,
  "offered to the human as a suggestion for <field>; the human decides",
  and its result text carries the suggestion in the fixed form the host
  already parses line by line: `Suggestion-for: <editor> · <field>`, a
  blank line, the text. The package rule stays true: nothing here
  writes.
- D2. **The host keeps it whole.** When a completed call is `suggest`, the
  host reads the result into `Suggestion{editor, field, text}` on the
  answer beside its looks (host.go:773-835), not truncated as a look's
  excerpt is; the message carries `suggestions`; the stream event
  carries each as it arrives so the card appears before the answer ends.
- D3. **The card is the result card.** In the transcript under the
  answer: the field's name as a small head, the text whole, Use this
  (primary) and Dismiss; after use "Used · Undo"; after dismiss one line
  that reopens; when the editor is closed, the closed line and Copy. It
  is the master's reusable result card, first used here.
- D4. **The editors register setters, in one place each.** `offerFields`
  gains, beside the reader, a setter `set(field, text) → previous`, and a
  way to say which editor and fields are open; `Panel` registers every
  sheet's fields through it (one place for eight sheets); the document
  editor registers its one field with selection-aware set (replace the
  passage if unchanged, else insert at the caret); the stickies composer
  its one field. Use this calls the setter and keeps the previous value
  for Undo, in the store.
- D5. **The field points at the drawer.** A field with pending
  suggestions shows "n suggestions" beside its label, from the store's
  count for that editor and field; pressing it opens the drawer and
  scrolls to the newest card. Panel renders it for every sheet field;
  the document editor above its view; the stickies composer beside its
  hint.
- D6. **The chip offers two questions.** Beside the draft chip:
  "Suggest a better wording" and "Suggest the next step", the store's
  existing suggested-question mechanism, sending with an empty composer.
- D7. **Used and dismissed travel with the next question**, in the
  capture, as "suggestion for Intent used" or "dismissed", so the Partner
  knows; they are not persisted in the transcript in step 1.
- D8. **The Partner is told, once.** Its instructions gain the sentence:
  "When the human asks you to write or improve a field of an open sheet
  or a passage, answer in words and call suggest with the text; the
  human decides whether to use it; you never save." The capture already
  names the open sheet and its fields.
- D9. **Step 1 is the sheets.** Every `Panel` sheet, the card, the field
  link, the chip's two questions, Undo, the closed state. The document
  editor and the stickies composer are step 2, each one registration.

## 4. Payload

`Message.suggestions: [{ "editor": "Edit goal", "field": "Intent", "text": "…" }]`
on a Partner message, and the same object on the live stream as
`suggestion` events. The capture gains `suggestions: [{editor, field,
state: "used" | "dismissed"}]`. Nothing else changes.

## 5. Not here, step 2 and later

The document editor and the stickies composer. Persisting used and
dismissed in the transcript. The Partner performing the allowlisted edit
directly on request, with its own button. Suggestions for a field the
human has not handed over. Diffs against the draft rather than whole
replacement. Keyboard acceptance from the composer. A suggestion that
names several fields at once.

## 6. Verification and box

Go: the `suggest` tool's bounds and refusals and its fixed result form;
the host parsing a completed `suggest` call into a whole suggestion
beside its look, and a failed one into none; the message and the stream
carrying it; the capture carrying used and dismissed. Frontend: the
store's setter registry and Undo value; the card's four states; the
field link's count and its opening of the drawer; the two chip
questions; Panel registering the edit sheet's fields; the guards stay
green. The walkthrough's fake Partner answers a canned suggestion for
the edit sheet's Intent; screenshots at 1280 and 400: the card under an
answer, the field link, the field after Use this with "Used · Undo", the
closed-sheet card. Budgets as always. Box: one build lane (Claude on
Opus), one code read (Codex on Sol) with one fix round under R-124,
after Astra's read; two attempts, 150 to 240 job-minutes.

## 7. Self-grade

High on D1 to D3 and D5: the tool follows the read tools' rules, the
host already parses result lines, the card is the master's own idea.
Medium on D4: one registry serving eight sheets through Panel is one
place, but Undo's previous value must be the field's value at use, not
at ask. Medium on D7: the Partner learns what happened only through the
next question. Weakest: the fixed textual form of the tool's result as
the vehicle for a structured suggestion; it is the mechanism the host
already has, and a second channel would be more machinery than step 1
needs.
