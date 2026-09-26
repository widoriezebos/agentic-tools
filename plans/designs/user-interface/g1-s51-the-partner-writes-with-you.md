# g1-s51: the Partner writes with you

- Kind: design
- Id: 01M3D6T9QTZ7WDE2HTXX9TH77M
- Status: done
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
   Intent, Next step and Labels (src/backlog/EditSheet.tsx:94-98). In the document reader a selected passage of a marked reading surface
   is sent with "Ask"; unsaved editor content is excluded
   (src/partner/AskSelection.tsx:16-100). The master keeps drafts explicit:
   "share unsaved edits only as explicitly identified draft context…
   keep the saved revision distinguishable from the draft"
   (user-interface-design.md:111).
2. **The Partner cannot write into anything.** Its tools are the
   interface's read tools, one stdio server, whose package rule is
   "Nothing here writes" (internal/ui/uitools/uitools.go:1-30); its
   answers are text, activity and looks (internal/ui/partner/host.go:56-80).
   A tool call becomes a look when it completes; the host reads the
   tool's result text line by line for `Source:` and the like
   (host.go:773-835); the call body also carries `RawInput`, which the
   fold discards (host.go:749). The tool server is a separate process
   with the workspace readers and no capture (uitools/mcp.go:310;
   cmd/metasystem/ui_tools.go:66); the capture reaches the conversation
   service, which composes the prompt (partner/service.go:324). The help term says it: "it does not write and it
   does not act".
3. **The master wants exactly this, in this shape.** "Return relevant
   records, evidence, and editable proposals so the frontend can show
   their normal views" (:83); gate 4, "first complete edit: a queued goal
   is edited through the brain, shown in its ordinary editor, edited by
   the human, and discussed again" (:817), with "a reusable result card"
   (:360); "the agent must not report an edit as applied solely because
   it prepared a patch or sent a request" (:123); direct performance only
   on the allowlist, everything else proposal-first (:119).
4. **The editors.** Five board sheets are one `Panel` in seven modes;
   `Panel` takes value-only `fields` and opaque children, so the sheet
   owns its draft state and its labels (src/backlog/Panel.tsx:100, 179).
   The captured draft drops empty fields (src/partner/drafting.ts:39) and
   carries descriptive context such as Goal beside the editable ones
   (EditSheet.tsx:94); a new goal starts with an empty next step
   (src/backlog/opening.ts:53). The
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
   caret (a document). The card turns to "Used · Undo"; Undo restores
   the value the field held at the moment of use, and only while the
   field still holds the suggestion's text: once the human has typed
   over it, the card says "edited since; your words stay" and Undo is
   gone. Dismiss folds the card to one line, "dismissed", which can be
   reopened.
5. **Stale.** A suggestion belongs to the one opening of the sheet it
   was asked from, not to the sheet's name: when that opening closes, the
   card says "The Edit goal sheet is closed" and offers Copy, and opening
   another goal's edit sheet does not wake it. If the field changed since
   the ask, Use this still applies, because the human pressed it; nothing
   applies by itself.
6. **Nothing else moves.** No save, no act, no history line. The
   conversation keeps the suggestion and, in the human's next question,
   whether it was used, so the Partner can build on it.

## 3. Decisions

- D1. **One tool, `suggest`.** In the interface's tool server beside the
  read tools: `suggest {editor, field, text}`, text at most 4000
  characters and the two names non-empty, refused beyond with words. It
  writes nothing and cannot see the capture, so it validates only its
  arguments; its result to the model is one line, "prepared as a
  suggestion for <field>; the human decides whether it is offered and
  used", and its result text carries the suggestion in a fixed form the
  host reads once: `Suggestion-for: <editor> · <field>`, a separator
  line, then the text, opaque to the end. The package rule stays true.
- D2. **The host keeps it whole; the service admits it.** When a
  completed call is `suggest`, the host reads the result once into
  `Suggestion{editor, field, text}` beside the look, not truncated as an
  excerpt is, a failed call yielding none; the turn-owning service,
  which holds the capture, records and emits it only when the capture's
  handed-over draft names that editor opening and a writable field of
  that name, and otherwise records an activity line "a suggestion for
  <field> was not offered: no such field was handed over"; the message
  carries `suggestions`; the stream carries each as it is admitted so
  the card appears before the answer ends.
- D3. **The card is the result card.** In the transcript under the
  answer: the field's name as a small head, the text whole, Use this
  (primary) and Dismiss; after use "Used · Undo"; after dismiss one line
  that reopens; when the editor is closed, the closed line and Copy. It
  is the master's reusable result card, first used here.
- D4. **Each sheet registers its writable fields and their setters,
  under one opening identity.** `offerFields` gains an opening id minted
  when the sheet mounts, the list of writable fields including empty
  ones, and a setter `set(field, text) → previous` supplied by the sheet
  that owns the state; descriptive context such as Goal is handed over
  as before but not writable; a confirmation-only sheet supplies none.
  The handed-over draft carries the opening id and the writable names,
  which is what the service checks. Use this calls the setter of that
  opening and keeps the previous value in the store for Undo; the store
  offers Undo only while the field's value still equals the suggestion.
  The document editor and the stickies composer register the same way
  in step 2.
- D5. **The field points at the drawer.** A field with pending
  suggestions shows "n suggestions" beside its label, from the store's
  count for that opening and field; pressing it opens the drawer and
  scrolls to the newest card. The sheet that owns the label renders it,
  through one small component the store supplies.
- D6. **The chip offers two questions.** Beside the draft chip:
  "Suggest a better wording" and "Suggest the next step", the store's
  existing suggested-question mechanism, sending with an empty composer.
- D7. **The Partner learns what happened from the draft itself.** The
  handed-over draft is re-read at every send, so a used suggestion is
  simply the field's value; no used or dismissed marks travel in step 1.
- D8. **The Partner is told, once.** Its instructions gain the sentence:
  "When the human asks you to write or improve a field of an open sheet
  or a passage, answer in words and call suggest with the text; the
  human decides whether to use it; you never save." The capture already
  names the open sheet and its fields.
- D9. **Step 1 is the sheets.** The five `Panel` sheets that have
  writable fields, the card, the field link, the chip's two questions,
  Undo, the closed state. The document
  editor and the stickies composer are step 2, each one registration.

## 4. Payload

`Message.suggestions: [{ "opening": "…", "editor": "Edit goal", "field": "Intent", "text": "…" }]`
on a Partner message, and the same object on the live stream as
`suggestion` events. The handed-over draft in the capture gains
`opening` and `writable: ["Intent", "Next step", "Labels"]`. Nothing
else changes.

## 5. Not here, step 2 and later

The document editor and the stickies composer. Used and dismissed marks in the capture or the transcript. The Partner performing the allowlisted edit
directly on request, with its own button. Suggestions for a field the
human has not handed over. Diffs against the draft rather than whole
replacement. Keyboard acceptance from the composer. A suggestion that
names several fields at once.

## 6. Verification and box

Go: the `suggest` tool's bounds and refusals and its fixed result form;
the host parsing a completed `suggest` call into a whole suggestion
beside its look, and a failed one into none, the framing read once and
the text opaque even where it holds a `Source:` line; the service
admitting one for a handed-over writable field of the named opening and
refusing one for a field not handed over, an empty field admitted; the
message and the stream carrying it. Frontend: the store's registry by opening with writable fields and
setters; Use this on a closed opening refused and on a reopened sheet of
the same name refused; Undo offered only while the field still holds
the suggestion; the card's four states; the
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
Medium on D4: each sheet supplies its own setters, so five
registrations rather than one, and Undo is offered only while the field
still holds the suggestion. Medium on D2: admission lives in the service, the one place that holds
the capture. Weakest: the fixed textual form of the tool's result as
the vehicle for a structured suggestion; it is the mechanism the host
already has, and a second channel would be more machinery than step 1
needs.

## Dispositions (Astra read, 2026-09-25, under R-124)

Four material findings, four deferred; every code claim checked.

| id | finding | fold |
|---|---|---|
| F1 | a sheet's name cannot identify the draft a suggestion belongs to: close goal A's edit sheet, open goal B's, press A's Use this, and B's field is overwritten | suggestions, Use and Undo bind to one opening id minted at mount; a closed opening's cards are Copy-only and a reopened sheet of the same name does not wake them |
| F2 | Undo restoring the value at use would erase words typed after | Undo only while the field still holds the suggestion; otherwise "edited since; your words stay" |
| F3 | the captured draft drops empty fields and carries descriptive context, so "suggest the next step" on an empty next step would be refused and Goal could be a target | each sheet registers its writable fields, empty ones included, with their setters; context is handed over but not writable |
| F4 | the tool server has no capture and cannot judge membership | the tool validates its arguments and says "prepared"; the turn-owning service admits or refuses against the handed-over draft |

Folded because they cost nothing: five `Panel` sheets in seven modes,
not eight; the sheets own their setters and labels, so the field link is
one component the sheets place; used and dismissed marks dropped from
step 1, the re-read draft says it; the document selection sentence
corrected. Astra judged the fixed result form sound when read once with
the text opaque, and `RawInput`, which the host receives, a cleaner
vehicle only if the provider supplies it, which the checked-in schema
leaves optional; not required for step 1.

## Built (2026-09-26)

Landed on ui-development and main; the slice went live before the read
because Wido was waiting on it, and the read, delayed by a rejected
OpenAI key that turned out to be a hiccup on the provider's side, found
nothing material. Built by Claude on Opus, read by Codex on Sol under
R-124: zero material findings, four deferred.

The first sitting failed on UX, not on code: with nothing handed over,
the Partner's suggestion was refused where the human could not see it
and the Partner narrated a card that was never shown. That is g1-s52.

Later, when it hurts: Undo's final guard compares a trimmed value, so a
field differing only in outer whitespace would pass it; two sheets of
one name open at once share a draft attachment by name; a repeated press
of the field link may not scroll to the card again; a dismissed card
stays folded after its sheet closes, Copy appearing only on reopen.

Departures the read accepted: Set priority, Approve and Approve-selected
register no writable fields; empty suggestion text is refused; the
context block names the writable fields; the snapshot carries
suggestions for a mid-answer reload; "once per field, at most three" is
not enforced; the opening id is minted with the turn key's own minting
after the browser showed a per-page counter would wake an old card.
