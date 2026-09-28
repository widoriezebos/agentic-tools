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
Previously. Under it, a link: *the review room on
backlog-ordered-by-priority*. You follow it; the room opens; the banner
says the branch moved. You ask "what changed in those two commits?"
and the conversation goes on. Another day, Approve on a card is refused
because the goal is parked; you press Ask what happened; the Partner
says who parked it and why, from the history, and under the answer
stands a card, *Unpark backlog-ordered-by-priority*, with Apply, since
unpark is one of the acts a proposal can be.

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
  subject), the act where there was one (the verb, the object, and the
  goal or record it named, as words; never the request's arguments as
  sent), when, the tip the page had, and whether the sign-in sheet is
  the remedy. Secrets never travel: the sign-in sheet's trouble carries
  no code (the sheet exists to keep it, `shell/SignInSheet.tsx:20`,
  `session.ts:105`), and before a trouble is kept or sent, its
  sentence is scrubbed of any value the page holds as a secret at that
  moment (the code being typed, a token field's value), replaced by
  "[withheld]"; a fixture proves a failed sign-in's trouble reaches
  neither the transcript nor the runtime with the code in it. The
  sign-in case keeps its behaviour: the sheet opens, and the line can
  still be asked.
- D2. **One press asks; nothing to type.** The press opens the
  conversation the human is in (the drawer's own, or the room's when
  the address is a room) and sends one fixed request in their name,
  `TroubleRequest`, the way a sitting opens with `OpeningRequest`:
  "What just happened here, why, and how do I recover?" The trouble
  travels as a block beside the page block, composed by the server
  from the payload, so what is said in the human's name is one
  sentence in one place, and the chip under the turn shows what
  travelled: the act or the pane, the code, the time. The press never
  touches the human's own words: a question half-written in the
  composer, and the attachments under it, stay exactly as they were
  whether the request is accepted, refused or has to wait (the store's
  `send` clears the draft and retires attachments on a sent turn,
  `partner/store.tsx:958-984`, so a trouble request is its own send
  path that takes no draft and clears none, the way a suggestion
  respects a half-written sentence, `store.tsx:1282`). If a turn is
  running, the trouble waits as a pending chip above the composer,
  beside the draft, owned by the conversation the press was made in
  (the chip carries it, and the trouble shows it: "waiting for the
  room on backlog-ordered-by-priority"); it is offered and sent only
  there, first, with the draft untouched, and a Send in another
  conversation neither sends it nor loses it, since the selection a
  send names is the address on screen (`store.tsx:850`, `:974`) and
  navigation replaces the shown conversation (`:884`). A press must
  never be refused as busy. A press must also reach a conversation the
  human can read: the bell's panel closes on the press and the drawer
  opens on the answer; the sign-in sheet, which is window-modal by
  design (`SignInSheet.tsx:78`, `Sheet.tsx:68`), hands off on the press
  by closing with its typed handle and its pending retry kept, and
  reopens from the sign-in control as today; and where the pane that
  threw was the room itself, whose boundary replaces the conversation's
  renderer until a reload (`Shell.tsx:416`, `ErrorBoundary.tsx:26`),
  the trouble line says "this room cannot show its conversation; ask
  from the drawer after a reload" and offers the reload, never a
  control whose answer nobody can read. The control is
  rendered only where a press can reach a colleague: the bell's panel
  and the sign-in sheet are drawn by providers that stand above the
  Partner's (`App.tsx:30`, `shell/Shell.tsx:73`, `notifications/
  store.tsx:161`, `shell/identity.tsx:137`), where the Partner context
  is the no-op one (`store.tsx:581`), so the Partner's store registers
  one `ask` on a small trouble context mounted above every provider,
  the trouble line presses that, and until it is registered the line
  shows no control at all rather than one that sends nothing. A press
  from the bell while a room is on screen reaches the room's
  conversation, since the registered `ask` reads the conversation on
  screen as every send does (`store.tsx:846-855`).
- D3. **The answer is three parts and ends with the recovery.** The
  Partner's instructions gain one rule: on a trouble, answer **What
  happened**, **Why** and **How to recover**, each a few sentences,
  in the human's terms (the act and the subject, not the route);
  read before saying why (the register row for the code, the goal, the
  record, the board, the fleet, the notifications, and the resource
  that could not be read, tried again); when the recovery is one of the
  ten goal acts the proposal grammar holds (`uitools/propose.go:44`;
  an unknown act is refused there, `:497`), propose it as a card and
  say what Apply will do; when it is a press somewhere in this
  interface (a room, a sheet, a page), name the press and give the
  place as a link, never a card; when it is a human's verb, say the
  exact verb and that it runs from an enrolled terminal; when it is a
  wait, say what to watch and where; when the cause is not in the
  records, say so and what would establish it. Never a cause that was
  not read, and never a card for something Apply cannot do.
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

The turn route (`/api/partner/turns`, `partner/api.ts:613`) accepts
`trouble` beside `page` and `about`: `{text, code?, where: {section,
path, subject?, kind?}, act?: {verb, object, target?}, at, tip?,
signIn?}`, bounded (the sentence at most 2,000 characters), scrubbed
by the page before it is sent and never carrying request arguments.
The server composes the human's turn from `TroubleRequest` and a
block "The trouble:" with those fields as lines, and records the turn
as it records any; the transcript shows the chip from the same fields.
The `refusal` reader: `refusal code=CODE` → the register row as text,
or "the register has no row for CODE" (the register also has
exclusions and prose rows, `refusal/register.go:391`, `:470`, and the
interface's own codes `one-line` and `partner` have no row; all answer
that honest line). The skill and its four mirrors gain the three-part
rule and the card rule. `Trouble` in `src/shell/Trouble.tsx`, the
trouble context in `src/shell/trouble.tsx` mounted in `App.tsx` above
the providers, the pending chip beside the composer in `src/shell/
Composer.tsx`; the structural rule in `src/trouble.test.ts` walks
`src/` like the cut guard does and refuses a `ms-*-refusal`,
`ms-*-problem`, `ms-partner-failed` or `ms-error-*` class outside it.
The reader joins the reader catalogue (`uitools.go:92`), not the ten
proposal actions. Routes unchanged.

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
room, and sends the request with the chip; a half-written question and
its attachments survive the press unchanged, idle and busy alike; a
running turn leaves the trouble as a pending chip and the next Send
in that conversation sends it first (`pending_trouble_stays_in_origin_
room`: Ask while room A is busy, switch to room B, B cannot send A's
pending trouble, back in A it sends with the draft and attachments
intact); the press reveals a usable conversation
(`trouble_ask_reaches_usable_partner`: from a real bell row the panel
closes and the drawer shows the answer with the composer focusable;
from the sign-in sheet the sheet closes keeping its handle and retry
and the answer is readable; from a room whose renderer the boundary
replaced the line offers the reload and no control); the sign-in refusal opens the sheet, keeps the
control, and its trouble carries no code (asserted on the transcript
and on what the fake runtime received); a sentence holding the secret
the page has is scrubbed; the boundary's trouble carries the error's
name; a press from a real bell row, rendered by the notifications
provider, while a room is on screen reaches the room's conversation,
and before the Partner has registered its ask the row shows no
control; a recovery that is not one of the ten acts arrives as a link
and never as a card (a negative fixture with the fake Partner
proposing "open the room" and the interface refusing the card); the
cut guard's rows; the guards green. Walkthrough: a refused act, a pane that could not be
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

## Dispositions (Astra round 1, 2026-09-28, under R-121 and R-124)

Read of revision 1 at `3e1262633`, verbatim in
`g1-s68-astra-critique.md`. Four material findings, all folded; two
non-material, folded as corrections; every cited line re-read at
whole-function depth before folding.

| id | finding | fold |
|---|---|---|
| S68-01 | "the arguments the page sent" would carry the sign-in code into the transcript and the prompt (`SignInSheet.tsx:20`, `:62`, `session.ts:105`); a bound of 1,000 characters does not protect a six-digit secret | D1: a trouble carries the act's verb, object and target as words, never the request's arguments; the sign-in trouble carries no code; the sentence is scrubbed of any secret the page holds; a fixture proves the code reaches neither destination |
| S68-02 | the store's `send` clears the draft and retires attachments on a sent turn (`store.tsx:958-984`), so reusing it for the fixed request deletes a half-written question; the busy path could overwrite it | D2: the trouble request is its own send path that takes no draft and clears none; while a turn runs the trouble waits as a pending chip beside the draft and the next Send sends it first |
| S68-03 | the bell's panel and the sign-in sheet are drawn by providers above the Partner's (`App.tsx:30`, `Shell.tsx:73`, `notifications/store.tsx:161`, `identity.tsx:137`), where the Partner context is the no-op one (`store.tsx:581`): a mechanical `Trouble` there shows a control that sends nothing | D2: one `ask` registered by the Partner's store on a trouble context above every provider; no control until it is registered; a bell press while a room is on screen reaches the room's conversation; tested from a real bell row |
| S68-04 | a proposal can only be one of the ten goal acts (`propose.go:44`, `:497`, the skill at `:33`), so the "open the review room" card of §3 could never be applied | D3 and §3: Apply cards only for the ten acts; a press elsewhere in the interface is named and linked, never a card; the example is now a link, and an unpark card shows the supported case; a negative fixture |
| S68-05 | non-material: not every code has a row (exclusions, prose rows, the interface's own `one-line` and `partner`) | §6 says so; the honest line "the register has no row for CODE" covers them |
| S68-06 | non-material: pointers were wrong (the composer lives under `shell/`, the turn route is `/turns` with `about`, the readers' catalogue is not the ten proposal actions) | §6 corrected |

Astra also verified, and the design leans on, that a fixed question
sent by an explicit press is the human's act and authorizes no
recovery, that turns already carry server-owned attribution in the
human's name (`service.go:653`), that the conversation on screen is
what a send names (`store.tsx:846`), and that the register can be
matched by `Row.Code` directly (`register_test.go:300`).

**Round 2, the declared failsafe (2026-09-28, at `0324a0cc4`):**
S68-01 to S68-04 confirmed answered. Two new material findings, both
in D2, both folded. S68-07: a pending trouble had no owner, and a Send
names the conversation on screen (`store.tsx:850`, `:974`; navigation
replaces it, `:884`), so a trouble asked in room A while busy could be
sent into room B; fold: the pending chip is owned by the conversation
of the press and is offered and sent only there, with the named
fixture. S68-08: registration proves a live store, not a readable
conversation: the bell's panel and the sign-in sheet are window-modal
(`workmodal.tsx:48`, `Sheet.tsx:68`, `SignInSheet.tsx:78`), and a room
whose renderer the boundary replaced keeps the store above it
(`Shell.tsx:416`, `ErrorBoundary.tsx:26`); fold: the press reveals a
usable conversation (the panel closes, the sheet hands off keeping its
form and retry, the broken room offers the reload and no control),
with the named fixture. S68-09 non-material: the class guard cannot
prove future sites; the converted sites are named. Closed at the
failsafe round on two folds, with one scoped confirmation read on
S68-07 and S68-08 alone (recorded below when it returns).
