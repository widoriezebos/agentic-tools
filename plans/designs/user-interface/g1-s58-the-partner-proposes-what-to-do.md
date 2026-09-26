# g1-s58: the Partner proposes what to do

- Kind: design
- Id: 01M3FF3700CGRZSPJ8MMXYCFPG
- Status: draft
- Goals: browser-interface

Wido, 2026-09-26: "I want the project partner to be able to propose an
action or list of actions that I can approve or ignore. Example retiring
a list of goals. These are basically verbs the partner provides and a
description. I read the description and decide in the UI by clicking or
bulk approving them. The approved ones then get applied by the UI. Note:
any combination of verbs in the list should be allowed. Failures should
also be managed in a good UX way, so I can recover them easily. Another
example is a discussion between me and the partner about a goal; and
then me asking the partner to create the goal. That would then lead to
a single proposed action that will create the entire goal as discussed,
if I approve the action. This is basically the partner doing any action
I can do in the UI, through verbs that I approve in the UI, immediately
visible in the UI. Note: this includes human only/authorized actions
(that should be possible if I'm logged in, through authentication via
TOTP)", and "first design the UX for this, it is crucial the UX follows
UX best practices". Author Fable. Every cite re-read at `ef4d9e6ab`.

## 1. What exists and binds

1. **The Partner already proposes, twice, through one pipeline.** A tool
   that writes nothing hands its arguments back in a fixed textual frame
   (`suggest`: internal/ui/uitools/suggest.go:41-56; `deposit`:
   uitools/deposit.go:86-104); the host reads one completed call into a
   whole object, refused calls yielding none (internal/ui/partner/host.go:952-961,
   986-1048); the turn-owning service admits or refuses it against
   server-held state and records it as its own beat, refusals kept with
   their reason where the human reads (service.go:533-547, 875-889,
   977-1000); the answer's message carries it, persisted (conversation.go:447-475);
   the snapshot and the stream carry it so a card survives a reload
   mid-answer (service.go:29-68, 73-113); the transcript renders it as a
   card whose live state is the store's (src/partner/Transcript.tsx:206-214,
   Suggestion.tsx, Deposit.tsx:36-63). The drawer grows once for a card
   (src/shell/Drawer.tsx:78-91). Neither tool can name a verb: `suggest`
   names an editor and a field, `deposit` one of five record entries, and
   `deposit` refuses the kind `proposal` in words (deposit.go:38-52, 116-118).
2. **The interface names every act it has, with the hand each needs.**
   `Acts()` in internal/ui/httpd/describe.go:44-107 lists twenty-six: nine
   on goals under `ledgerHand` (open, approve, withdraw, set priority,
   block, unblock, park, unpark; edit is routed at acts.go:75 and
   write.go:245 but absent from the table and from the completeness
   test's own route list, describe_test.go:22-31, as is the fleet's
   launch), the checkout writes, the notepad, sign-in and the Partner's
   own routes. The Partner reads the
   table through `interface(part="acts")` (uitools/mcp.go:158-175) and its
   skill says "Claim no act … say where in the interface it is done"
   (internal/ui/partner/project-partner.skill.md:28). The nine goal acts
   are one POST each with a bounded JSON body (acts.go:98-165, 300-430):
   approve `{elapsedLimit, attemptLimit, reservedJobMinutesLimit,
   activeJobLimit, reviewRoundLimit}`, withdraw `{reason}`, park
   `{because}`, unpark `{}`, priority `{priority, sequence}`, block and
   unblock `{blocker}` with the waiting goal in the path, edit `{intent?,
   nextStep?, labels?}`, open `{id, intent, nextStep, tier, why, blocks,
   blockedBy, labels, severity, novelty, exposure, accumulation, basis}`.
   A confirmed act answers with the backlog as it then stands
   (acts.go:486-497); a refusal is `{error, code?}` under 403, 400, 409 or
   500 (acts.go:500-527), and one that a sign-in would remedy adds
   `signIn: true` (acts.go:533-540).
3. **Every act needs the human's own session, and a Partner makes that
   absolute.** `mayAct` admits a live browser session, answers an expired
   one with 403 and `signIn`, and on a seat that runs a Partner refuses a
   cookie-less request with "a Partner runs on this seat, so acts need a
   signed-in human" (acts.go:443-483). The session is the one-time code's
   (g1-s23). The engine admits it for approve, withdraw, priority, open,
   block, unblock, edit, and, under R-125-m1u, park of a human-origin goal
   and unpark of a human park; the ruling says "Nothing else that asks a
   terminal-grade proof admits a session" (metasystem/memory/rulings.md:184).
   Abandon is therefore not an act the interface has, and "retire a goal"
   in this build is the Decisions page's Not now: a park with a reason.
4. **The page already runs a list of acts and says what it can know.**
   `runInOrder` sends one publication per goal in order, never retries,
   and stops at the first answer that is not one (src/decisions/decisions.ts:671-687);
   a stopped run is terminal and a second press is refused, because it
   would send the landed ones again (:654-656); the sheet cannot be
   dismissed mid-run (:642-644); the failed goal is named unresolved,
   never refused, and the page's re-read is what says what the ledger did
   (:697-706); an approval's budget is `prefillFor`'s, and a goal with no
   prefill is listed and excluded with "needs its budget first: approve it
   alone" rather than dropped (:576-603). `outcomeOf` maps a failed act
   conservatively: a 4xx with the engine's sentence is refused and nothing
   landed; 5xx, the journal's unsettled codes, the `proof-not-recorded`
   answer and a transport failure are unresolved, never resubmitted by
   the page (src/backlog/editing.ts:216-260). The act clients are one
   `request()` each (src/backlog/api.ts:259-270, 312-397); a refusal's
   `signIn` flag opens the sign-in sheet through `askToSignIn(again)`,
   which runs the act once more when the human signs in (src/shell/identity.tsx:106-118).
   No act carries a client key: the act layer mints a fresh operation id
   per request (internal/ui/act/act.go:566), so a repeated POST is a
   second attempt, and a repeated approve is not a no-op but a second
   approval record (internal/goal/approval.go:502-503); park, unpark,
   open and block refuse a repeat in the engine's words.
5. **A page re-reads whole after an act; some offer that read to the shell.**
   Decisions, Overview, Fleet and Application offer their re-read through
   `useOffersRefresh` (src/shell/refresh.tsx:44-52; DecisionsPane.tsx:283);
   the board and the goal page carry their own refresh in their strip and
   offer none. Nothing on the stream announces a ledger change; the pages
   read on mount, on Refresh and after their own acts.
6. **The master wants exactly this shape.** The capability inventory
   records whether the brain may read, propose or perform each action;
   "all other changes are proposal-first" (plans/designs/user-interface-design.md:119);
   "Human explicitly accepts an agent proposal: submit a human-authorized
   operation against the reviewed proposal; retain the agent's authorship
   separately" (:226); "a reserved action appears as a proposal until the
   human explicitly uses its normal confirmation control" (:229); "bulk
   actions show scope and per-item results and never imply atomicity that
   the backend does not provide" (:736); "translate a refusal into the
   affected subject, the unmet condition, and available next actions"
   (:738); "retrying after a disconnected browser must not duplicate a
   decision … a timeout must not be presented as evidence that an
   operation failed or never happened" (:780). g1-s28 deferred it in these
   words: "Proposals: the Partner fills a sheet the human confirms, through
   the shared operations, with its authorship recorded and never the
   human's authority."

## 2. How it works, from the chair

You are on Decisions. Six `headless-fleet` goals are three weeks old and
you know why. You tell the Partner: "these six are superseded by the seat
inventory; put them away". It answers in a sentence and, under the
answer, a card:

```
╭ The Partner proposes 6 actions ────────────────────────────────────╮
│ ☑ Not now · Fleet presence is read from the census, not polled      │
│     because: superseded by the seat inventory (g1-s42)              │
│ ☑ Not now · A silent seat's claim is shown, never lifted            │
│     because: superseded by the seat inventory (g1-s42)              │
│ ☑ Not now · …                                                       │
│ ☑ Not now · …                                                       │
│ ☑ Not now · …                                                       │
│ ☑ Withdraw approval · Refunds land within a day                     │
│     reason: the budget assumed a July start                         │
│   The Partner: all six name the fleet inventory that g1-s42 built;  │
│   the seventh was approved against a date that has passed.          │
│ 6 selected                          [Apply 6]   Select all · Dismiss│
╰─────────────────────────────────────────────────────────────────────╯
```

You untick the withdrawal; you want to think about that one. "5
selected · Apply 5". You press it. The first line says "applying…", then
"applied"; the row leaves the queue on the page behind the drawer as you
watch, and the count in the block's head drops. The second, the third.
The fourth says "refused: goal fleet-idle-alerts is claimed by
mac-mini/01M3…; edit it at a terminal" in the danger colour, the run
stops there, and the fifth says "not run". The foot now reads "3 applied
· 1 refused · 1 not run" with two buttons: "Continue with the rest" and,
on the refused line, "Try again" and "Ask the Partner". You press
Continue; the fifth applies. The card's foot: "4 applied · 1 refused ·
1 waiting" and the withdrawal still ticked-off and pressable whenever
you decide. The Partner's next answer knows all of it.

The other day. You have talked a goal through for ten minutes. "Create
it." One card:

```
╭ The Partner proposes ───────────────────────────────────────────────╮
│ Open goal · refund-worker                                           │
│   Intent: Every refund lands within a day, with nobody touching     │
│           the queue.                                                │
│   First next step: Read the refund worker's retry loop and write    │
│           the case where the bank answers late.                     │
│   Tier 2, from severity 2 · novelty 1 · exposure 2 · accumulation 1 │
│   Basis: payments, one team, one month of history                   │
│   Labels: payments, robustness                                      │
│   Waits for: bank-sandbox                                           │
│   The Partner: as discussed, the July incident and the two open asks│
│                                              [Apply]   Dismiss      │
╰─────────────────────────────────────────────────────────────────────╯
```

You read it whole. You press Apply. "applied · Open the goal", and the
board shows it in To Do. A word you want changed is one press away on
the goal's page, where Edit… already is.

What you want from this, in the order it matters, and how the card
keeps it:

- **Nothing happens unless I press.** The Partner can only propose. Your
  press is the act, under your sign-in; the ledger names you as the
  hand, as it does for every act from this browser.
- **I read what will happen before it happens, in the interface's own
  words.** The verb on the line is the button's word on the page that
  offers it: Approve, Withdraw approval, Set priority, Open goal, Not
  now, Return to queue, Edit, and the goal page's own words for block
  and unblock. The subject is the goal's title, as the pages say it, with
  its id small. Every argument the act will carry is on the card, whole,
  rendered by the interface from the validated proposal and never from
  the Partner's prose; the Partner's own words are its "why", shown as
  its words. An approval shows the budget it would carry and where that
  budget came from, exactly as the approve sheet does.
- **I choose which.** A checkbox per action; one, some or all. Apply says
  how many. A card of one action has no checkbox and one button.
- **I see each one land, or not, on the page I am looking at.** Each line
  says applying, applied, refused with the engine's sentence, or
  unresolved with what was said; the page in view reads again after each
  confirmed act and when the run ends.
- **A failure tells me what and why, leaves the rest intact, and offers
  the way on.** A refusal is the engine's own sentence on that line. The
  run stops there, because the next action may have depended on it, and
  says how many were not run; Continue with the rest is one press, Try
  again on the refused line is another, and Ask the Partner puts the
  refusal in the composer so the Partner can propose differently.
- **The interface never applies anything twice by itself.** Each action
  is sent at most once per press; a line that says unresolved is never
  re-sent by the page: it says "check the goal before trying again" and
  offers Open the goal and Try again, which is your press. A reload
  during a run shows the line that was in flight as "was being applied
  when the page left", not as fresh.
- **Ignoring is allowed.** A card you do not act on stays proposed, with
  its ticks; Dismiss folds it to one line that reopens; nothing expires
  into an act.
- **Human-only acts need my sign-in and nothing else new.** When the
  session is not proven the button says "Sign in to apply" and opens the
  sign-in sheet; the run starts when the code is accepted. A session that
  expires mid-run opens the sheet at that line and continues on sign-in;
  a sign-in abandoned leaves that line and the rest "not run".

The practices this follows, so a reader can check them rather than
trust them: visibility of status (every line has one, the foot counts
them, the page moves); match between the words on the card and the
buttons on the pages; user control (select, dismiss, continue, try
again, never an automatic act); error prevention (arguments validated
before the card exists, the engine's checks at the act, the approval's
budget shown); recognition over recall (the subject named and linked,
the arguments visible); errors that name the cause and the recovery in
the engine's words; consistency with the two cards that exist (Use this,
Record it) and with the bulk run the Decisions page already has; and
one card, one press, for the common case.

## 3. Decisions

- D1. **One tool, `propose`, one action per call.** In the interface's
  tool server beside `suggest` and `deposit`: `propose {verb, goal?,
  why, …}` with the verb one of a closed catalogue, `approve`,
  `withdraw`, `priority`, `block`, `unblock`, `park`, `unpark`, `edit`,
  `open`, and the fields that verb takes and no others: withdraw
  `reason`, park `because`, priority `priority` and `sequence`, block and
  unblock `blocker`, edit `intent`, `nextStep`, `labels` (at least one),
  open `id`, `intent`, `nextStep`, `severity`, `novelty`, `exposure`,
  `accumulation`, `basis`, `labels`, `blockedBy`, `blocks`, `why`. An
  approval takes no budget: the interface supplies the one the approve
  sheet would, and shows it. The tool validates shape and bounds only,
  the way `deposit` does (an unknown verb refused with the catalogue in
  words; a missing field refused by name; texts bounded as the sheets
  bound them; line breaks in an intent or next step refused as the route
  refuses them), writes nothing, and answers "prepared; preparing applies
  nothing: the human sees the action on a card and decides whether to
  apply it". Its result carries the action in the fixed frame the host
  already reads once: a header line naming the verb, labelled lines for
  the fields, the separator, and the `why` opaque to the end. The Partner
  calls it once per action, beside its answer in words; a list is many
  calls of one answer.
- D2. **The host keeps it whole; the service admits it against the tip.**
  A completed `propose` call is read into `Action{verb, goal, fields,
  why}` beside its look, a failed call yielding none. The service admits
  it against the observation the turn was composed from (`Facts.Observe`,
  context.go:43-51): every goal id the action names, the subject and a
  blocker, must be at the accepted tip, or be the id of an `open` action
  admitted earlier in the same answer, so "open X, then block Y by X" is
  one proposal; an `open` whose id is already at the tip is refused
  "goal <id> already exists". An admitted action carries the subject's
  title as the pages say it (the first sentence of the intent) and, for
  an `open`, the intent's own first sentence. A refused action is
  recorded with `offered: false` and its reason, shown on the card as a
  "not offered" line, never dropped to an activity line (g1-s52 D3).
  Admission checks existence and nothing else: whether the act is
  allowed in the goal's state is the engine's answer at the act, in its
  own words, as it is for every button.
- D3. **The card is the confirmation control.** Under the answer, one
  card for the answer's actions, in the order proposed: a checkbox and
  the line of D4 per action, the Partner's why under its action, the
  foot with "n selected", Apply n (primary), Select all, Dismiss. A card
  of one action has Apply and Dismiss and no checkbox. The card carries
  the whole reviewed basis, so no sheet repeats it: a second dialog
  listing what the card already lists would be a press that adds nothing
  to read. Pressing Apply runs the ticked actions in order.
- D4. **A line is the verb's word, the subject and every argument.** The
  verb word is the button's on the page that offers the act; the subject
  is the title with the id small, linked to the goal page ("Open the
  goal"); the arguments in the sheets' own labels: because, reason,
  priority and position, waits for, intent, next step, labels, and for an
  open the tier derived from the four answers with the answers named,
  the basis, labels, waits for and unblocks; an approval "with budget
  <the tuple in the sheet's five words>, from <the source's words>", or
  "needs its budget first: approve it alone", unticked and not sendable
  (decisions.ts:576-603). The tier is derived on the card by the new-goal
  sheet's own function and sent as derived; the Partner names no tier.
- D5. **The run is the page's run, with the way on as the human's press.**
  The runner is `runInOrder` (decisions.ts:671-687): the ticked actions
  in order, one act each through the existing clients (backlog/api.ts:312-397),
  never retried, stopping at the first answer that is not one. The failed
  line takes `outcomeOf`'s word (editing.ts:252-260): refused with the
  engine's sentence, or unresolved with what was said and "check the goal
  before trying again". Lines after it say "not run"; the foot offers
  Continue with the rest, which runs them from the first not-run one;
  the refused or unresolved line offers Try again, which sends that one
  action again, and Ask the Partner, which fills the composer with the
  verb, the subject and the sentence. A line is sent at most once per
  press; the card refuses a second Apply while a run is in flight, on a
  synchronously held ref as the edit sheet guards its save (g1-s56 D1).
  After each confirmed act and when the run ends, the store asks the
  section's offered re-read; the board and the goal page offer theirs
  through `useOffersRefresh` with one added flag, `inStrip`, that keeps
  the header from showing a second icon beside the one their strip has.
- D6. **The outcome is recorded where the proposal is.** `Message.proposals[]`
  on the Partner's message, one entry per admitted or refused action,
  each with `state: waiting | applying | applied | refused | unresolved |
  dismissed`, `words` and `at`. Before sending an action the runner
  records `applying`; after the answer it records the outcome, through
  one conversation route, `POST /api/partner/turns/<turn>/proposals/<index>`
  with `{state, words}`, under the conversation hand: it carries no
  authority and changes nothing but the transcript. The conversation
  rewrites that one message in place through the atomic writer the trim
  already uses. The card reads its lines from the message, so a reload
  shows applied as applied, and a line left at `applying` as "was being
  applied when the page left; check the goal before applying again",
  never as fresh: an approve applied twice is two approval records, not a
  no-op, so a card that forgot it was applied would invite the one press
  that writes one. Dismiss records `dismissed` on every waiting line. If
  the outcome write itself fails, the line keeps its state for the page's
  life and says "the conversation could not record this".
- D7. **Sign-in is as every act's.** With no proven session the card's
  button reads "Sign in to apply" and opens the sign-in sheet through
  `askToSignIn(run)`; a refusal carrying `signIn` mid-run opens the sheet
  through `askToSignIn(again)` for that action once, as every act does; a
  sheet closed without signing in leaves that line "not applied: sign in
  to apply" and the rest "not run". The act publishes under the human's
  session and the ledger names the human; the transcript is the record
  that the Partner proposed it and when the human applied it, which is
  the master's "authorship retained separately" for this step.
- D8. **Where the human looks.** The card is in the transcript, in the
  drawer and in the focused view; the drawer's one-shot growth for a card
  (Drawer.tsx:78-91) covers it; while a proposal of the last answer has a
  waiting line and the drawer is closed, its bar says "n actions
  proposed" beside the composer, and pressing it opens the drawer at the
  card, as the sitting's counts open the table.
- D9. **The Partner is told, and told what happened.** Its instructions
  gain: "When the human asks you to do something the interface can do to
  a goal, do not do it and do not say it is done: call `propose` once per
  action with the verb, the goal and why, beside your answer, and say you
  have proposed it; the human decides which to apply, and the card is the
  only place it is applied. Any combination of verbs may be proposed
  together; a goal you propose to open may be named by a later action of
  the same answer." The `interface()` act table gains the row for edit.
  The next question's context block gains one line per proposal of the
  last two answers that carries one: "Of the n actions you proposed, a
  applied, r refused (<verb> <goal>: <words>), u unresolved, w waiting, d
  dismissed", from the message's recorded states, so the Partner builds
  on what actually happened and never on what it prepared. `deposit`'s
  refusal of the kind `proposal` gains "; an act on a goal is proposed
  with propose".
- D10. **Nothing else moves.** No abandon (a ruling in R-125's shape would
  be needed first); no checkout writes, stickies or questions in the
  catalogue; no editing of an argument on the card; no undo; no basis
  digest; no Partner authorship in the ledger's History; no marker on the
  subject's row on the board.

## 4. Step 1, and what waits

Step 1 is D1 to D9 for the nine goal verbs. It is what a human can use
after one slice: propose one act or many, tick, apply, watch the page
move, read a refusal in the engine's words, continue, try again, ask the
Partner, sign in when asked, reload and find the truth.

Later, when it hurts: the checkout writes as verbs (set a record's
status, settle a question, ask a question, write a record: one catalogue
row, one admission rule against the project reader and one client each);
"Review in the sheet" on a line, opening the verb's own sheet prefilled
so an argument can be edited before applying; a "proposed" marker on the
subject's row on the board and in the queue; abandon, after its ruling;
the Partner's authorship on the ledger's history line; a basis digest per
action (gate 2's review basis); keyboard operation of the card; proposals
across several answers as one list; bulk Try again; the launch route's
row in the act table; continuing past a definite refusal without a press.

## 5. Payload and routes

The tool: `propose` as D1, its result in the fixed frame `Proposal: <verb>`,
labelled lines `Goal: `, `Reason: `, `Because: `, `Priority: `,
`Sequence: `, `Blocker: `, `Intent: `, `Next step: `, `Labels: `, `Id: `,
`Severity: ` … `Basis: `, `Blocked by: `, `Blocks: `, then the separator
and the why. `Message.proposals: [{ index, verb, goal, title, fields: {…},
why, offered, reason, state, words, at }]` on a Partner message, the same
object on the stream as `proposal` events as each is admitted, and on the
snapshot's running turn as `proposals`. The route `POST
/api/partner/turns/<turn>/proposals/<index>` with `{state, words}`, the
conversation hand's policy, answering the snapshot; a state that is not
one of the five, an index the message does not carry, or a turn that is
not this conversation's is refused with words. The describe table gains
`{ID: routeEditGoal, Title: "Edit a goal", Requires: ledgerHand}` and the
new route's own row, and the completeness test's route list
(describe_test.go:22-31) gains both, so the table cannot lose them again;
the launch route's missing row is noted for later, not this slice's.
Nothing else changes shape.

## 6. Verification and box

Go: the tool's catalogue, every verb's required fields and bounds, an
unknown verb and a foreign field refused in words, the fixed frame; the
host reading a completed `propose` into a whole action and a failed one
into none, the why opaque past the separator; the service admitting an
action whose goal is at the tip, admitting a block whose blocker an
earlier `open` of the same answer named, refusing an `open` of an
existing id and an act on an unknown goal with their reasons, the title
carried; the message, the stream and the snapshot carrying it; the
outcome route's policy, its five states, its refusals, and the message
rewritten in place with the rest of the transcript untouched; the
context block's line from recorded states; the describe table's row and
a join test that every ledger act in the catalogue is in the table and
every table row the catalogue admits is in the catalogue. Frontend: the
line's words for each verb, the tier derived and the answers named, the
approval's budget and the excluded line; the card's states as pure
functions with the run's rules, a refusal stopping the run and Continue
running the rest from the first not-run line, Try again sending one,
unresolved never re-sent, the ref guard refusing a second Apply
mid-run; the two outcome writes per line and the reload showing
`applying` as in flight; Sign in to apply and the mid-run sign-in; the
offered re-read called after a confirmed act; the `inStrip` flag hiding
the header's icon; the bar's count; the guards stay green, with the new
call sites rowed in `cuts.test.ts`. Walkthrough: the fake Partner answers
a canned proposal of three actions and a canned single open; the
fixture's canned acts refuse one goal by name and answer 500 for
another, so a refusal and an unresolved line are seen; screenshots at
1280 and 400: the card of six, three applied and one refused, the single
open, the reload with a line in flight, the bar's count. Budgets as
always. Box: one build lane (Claude on Opus), one code read (Codex on
Sol) with one fix round under R-124, after Astra's read; two attempts,
240 to 360 job-minutes.

## 7. Self-grade

High on D1, D2, D5 and D7: the third tool in a pipeline that has carried
two, the page's own run and outcome mapping, the act clients and the
sign-in retry every button uses. High on D3 and D4: the card says what
the sheets say, in their words, and is the master's own "reasonable
result card" with the reviewed basis on it. Medium on D6: the first
write into a Partner message after it was appended, one route with five
states, needed so a reload cannot show an applied act as fresh. Medium
on D9: a context line about earlier proposals is the first time the
Partner is told a consequence of its own offer. Weakest: stopping the
run at a refusal costs one press per failure in a long list, chosen over
continuing by itself because a later action may depend on the refused
one and the engine, not the page, is the judge of that; if sittings show
the press is a nuisance, continuing past a definite refusal is one rule
in the runner.
