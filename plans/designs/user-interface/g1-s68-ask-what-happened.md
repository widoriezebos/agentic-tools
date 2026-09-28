# g1-s68: ask what happened — every error on screen opens a conversation with the Partner about what happened, why, and how to recover

- Kind: design
- Id: 01M3MJ8JJPCHB2SN3B1B8FQKVZ
- Status: draft
- Goals: browser-interface ask-what-happened

Wido, 2026-09-28: "whenever an error occurs and it is shown on screen
[we must have] the possibility to ask the project partner about what
happened, why and how to recover. Put on your UX cap and make it
intuitive and very helpful for the user." Author Fable. Every cite
re-read at `6eb54f7e3`.

## 1. What exists and binds

1. **What goes wrong on screen today, in five shapes.** An act the
   engine refuses comes back as the engine's own sentence and the code
   it refused under (`internal/ui/httpd/acts.go:581`, `{error, code}`,
   and `signIn: true` when the remedy is the sign-in sheet, `:591`); a
   pane that could not be read says so in its own words ("The backlog
   could not be read", `backlog/BacklogPane.tsx:101`, and the
   `ms-*-problem` lines of the fleet, the board, the stickies, the
   notifications and the project); a Partner turn that did not complete
   says so under the turn (`partner/Transcript.tsx:245`, `Seeing.tsx:
   175`); a pane that throws is caught and named by the boundary
   (`shell/ErrorBoundary.tsx`); and a failure the engine notifies
   arrives as a toast and a row under the bell (`notifications/
   Toasts.tsx`, `Bell.tsx`). Twenty class names render these in
   twenty-nine components, each a sentence in a paragraph with
   `role="status"` or `role="alert"`, and none of them can be asked
   about.
2. **The refusal register.** Every code the engine refuses under has a
   row: its owner, one emitting site, its shape (identity, warning,
   question, agent), the human verb that carries past it and how many
   commands that takes (`internal/refusal/register.go`, `Row`). The
   register is the engine's own answer to "why" and "how do I recover"
   for a refusal, and the browser never reads it.
3. **The Partner and what it is told.** Every turn carries the page
   (`Page` in `internal/ui/partner/conversation.go:249`: section,
   path, tab, view, tip, subject, title, revision). A selection on a
   reading surface becomes an Ask with a chip above the composer that
   says what travels (`partner/AskSelection.tsx`); a sitting opens with
   one fixed request in the human's name so that what the interface
   says for them is one sentence a reader can find (`OpeningRequest`,
   `service.go`). The Partner reads the board, a goal, a document, the
   records, the overview, the fleet and the notifications (`uitools.go:
   70-87`) and proposes acts as cards the human applies (`propose`,
   g1-s58/s60). The cut guard and the CSP as before.
4. **The paper.** The colleague explains; the human decides. An answer
   that invents a cause is worse than none (ch. 9, the hazards); the
   records are the memory, so "why" is read, not remembered.

## 2. What you want when something goes wrong

- **W1. Ask right there.** The sentence stays where you were looking,
  and one press beside it asks the colleague. No error centre, no log
  to open, no code to copy.
- **W2. Nothing to type.** The press is the question. What you meant
  is obvious: what just happened, why, and how do I get on.
- **W3. An answer in your terms.** What happened named by the act and
  the subject you pressed, not by the endpoint; why, from the rule or
  the state that caused it, read from the records; how to recover as
  the next press, and when the next press is in this interface, as a
  card you can apply.
- **W4. Honest when it does not know.** A crash or a cause not in the
  records is said as such, with what would establish it.
- **W5. It keeps being a conversation.** The answer is a turn; the next
  question is yours to ask, with the trouble still on the table.

## 3. The moment

You press Land on a goal's card. The line under the card reads: "work
land is refused: goal/backlog-ordered-by-priority has moved past the
tip the review named (REVIEW_STALE)." At its end, quietly: **Ask what
happened**. You press it. The drawer opens on your conversation, and
your turn is already there: "What just happened here, why, and how do
I recover?" with a chip under it: *Land · backlog-ordered-by-priority ·
REVIEW_STALE · 19:41*. The Partner answers in three parts. **What
happened:** you pressed Land on backlog-ordered-by-priority, and the
engine refused it under REVIEW_STALE. **Why:** the review record names
tip `7ed3baf`, and the goal's branch is now at `9c1f0a2`; two commits
landed on it after the sitting ended (it read the record and the
branch). **How to recover:** the branch has to be reviewed at its new
tip; the room offers Review the new tip, which appends the old tip as
Previously. Under it, a card: *Open the review room on
backlog-ordered-by-priority* with Apply. You press Apply; the room
opens; the banner says the branch moved. You ask "what changed in
those two commits?" and the conversation goes on.

The same press on "The backlog could not be read": the Partner reads
the board itself and says it got the same refusal, that the checkout's
accepted tip cannot be resolved, and that the fleet page shows the
sync seat stopped ten minutes ago; the recovery is a human's, restart
the seat from an enrolled terminal, and it says the verb. The same
press on "The turn did not complete": the Partner, in a fresh turn,
says its previous turn was stopped by the runtime after the settle
wait, that nothing was written, and offers to answer the question
again. On a pane the boundary caught: "This pane could not be
rendered: TypeError: cannot read properties of undefined (reading
'lane')" — the Partner says what the page was showing when it threw,
that the cause is in the page's code and not in the records, and how
to report it: the sentence and the time.

## 4. Decisions

- D1. **One shape for everything that goes wrong on screen: the
  trouble line.** Every refusal, problem, failed turn, caught throw
  and failure notification renders through one component, `Trouble`,
  with the sentence exactly as today, `role` as today, and one control
  at its end, "Ask what happened". The twenty class names collapse to
  one; a vitest structural rule refuses a refusal or problem line
  rendered any other way, so the next site cannot forget. A trouble
  carries what a reader needs and no more: the sentence, the code where
  there is one, where it happened (the page's section, path and
  subject), the act where there was one (the verb, the object and the
  arguments the page sent, as words), when, the tip the page had, and
  whether the sign-in sheet is the remedy. The sign-in case keeps its
  behaviour: the sheet opens, and the line can still be asked.
- D2. **One press asks; nothing to type.** The press opens the
  conversation the human is in (the drawer's own, or the room's when
  the address is a room) and sends one fixed request in their name,
  `TroubleRequest`, the way a sitting opens with `OpeningRequest`:
  "What just happened here, why, and how do I recover?" The trouble
  travels as a block beside the page block, composed by the server
  from the payload, so what is said in the human's name is one
  sentence in one place, and the chip under the turn shows what
  travelled: the act or the pane, the code, the time. If a turn is
  running, the request waits in the composer with its chip and Send
  sends it, since a press must never be refused as busy.
- D3. **The answer is three parts and ends with the recovery.** The
  Partner's instructions gain one rule: on a trouble, answer **What
  happened**, **Why** and **How to recover**, each a few sentences,
  in the human's terms (the act and the subject, not the route);
  read before saying why (the register row for the code, the goal, the
  record, the board, the fleet, the notifications, and the resource
  that could not be read, tried again); when the recovery is an act
  this interface has, propose it as a card and say what Apply will do;
  when it is a human's verb, say the exact verb and that it runs from
  an enrolled terminal; when it is a wait, say what to watch and where;
  when the cause is not in the records, say so and what would
  establish it. Never a cause that was not read.
- D4. **The Partner reads the register.** One new reader, `refusal`,
  answers a code with its row: the owner, the shape and what the shape
  means, the human verb that carries past it and how many commands it
  takes, the H1 standing where the shape is a question. An unknown code
  is refused in words. The row is the engine's own account of the
  refusal; the Partner quotes it and adds the state it read.
- D5. **A toast that vanished can still be asked.** The bell's rows are
  trouble lines too, so a failure that flashed by is asked about from
  the list; the list already keeps them.
- D6. **Nothing is recorded.** A trouble and its answer live in the
  conversation. No record, no sticky, no receipt is written by asking.

## 5. Step 1, the smallest thing that works

D1 to D6 as one slice. `Trouble` and its structural rule, every site
converted (mechanical: a sentence in a paragraph becomes a `Trouble`
with what the site already knows), `TroubleRequest` and the trouble
block, the `refusal` reader, the skill rule in every mirror, the chip.
Not in it: the Partner opening a goal from a trouble (the New goal
sheet and `propose goal open` exist; the rule that a trouble may
become a goal waits for a case); a server log reader; a trouble that
happened in another human's browser.

## 6. Payload and routes

`POST /api/partner/turn` accepts `trouble` beside `page`: `{text,
code?, where: {section, path, subject?, kind?}, act?: {verb, object,
args?}, at, tip?, signIn?}`, bounded (the sentence at most 2,000
characters, the arguments at most 1,000). The server composes the
human's turn from `TroubleRequest` and a block "The trouble:" with
those fields as lines, and records the turn as it records any; the
transcript shows the chip from the same fields. The `refusal` reader:
`refusal code=CODE` → the register row as text, or "the register has
no row for CODE". The skill and its four mirrors gain the three-part
rule. `Trouble` in `src/shell/Trouble.tsx`; the structural rule in
`src/trouble.test.ts` walks `src/` like the cut guard does and refuses
a `ms-*-refusal`, `ms-*-problem`, `ms-partner-failed` or `ms-error-*`
class outside it. Routes unchanged.

## 7. Not here, later

A goal from a trouble; the server log as a reader; a troubles list
under the bell beyond what it keeps; a trouble reported across seats;
the Partner acting on a recovery without a press (never).

## 8. Verification and box

Go: `TroubleRequest` is one fixed sentence; the trouble block carries
every field and refuses a sentence over the bound; a turn with a
trouble is recorded like any turn and the Partner's runtime receives
the block (the fake runtime); the `refusal` reader answers a known
code with owner, shape, override and commands and refuses an unknown
one in words; the reader is in the catalogue and the Partner's ten
actions test grows by one read. Frontend: every site renders
`Trouble` (the structural rule fails against the code before the
change); the press opens the drawer, or the room's conversation in a
room, and sends the request with the chip; a running turn leaves it in
the composer with the chip and Send sends it; the sign-in refusal
opens the sheet and keeps the control; the boundary's trouble carries
the error's name; a bell row's press asks; the cut guard's rows; the
guards green. Walkthrough: a refused act, a pane that could not be
read and a failed turn, each asked, the fake Partner answering in
three parts with one card; screenshots at 1280 and 400, light and
dark. Landing checks as g1-s67 §8. Box: Astra's critique of this
page (two rounds), one build lane (Claude on Opus 5.5), one code read
(Codex on Sol) with one fix round; 120 to 200 job-minutes.

## 9. Self-grade

High on D1, D2 and D6: one component, one fixed sentence, nothing
written. High on D4: the register exists and is the right source.
Medium on D3: the three-part rule is words in a skill, and the fake
Partner will honour it while a real one may ramble; the walkthrough
checks the shape, not the quality, which only use will show. Weakest:
a pane that could not be read is asked about in a drawer that may be
in the same trouble; when the Partner itself cannot be reached the
line says so, and there is nobody to ask.
