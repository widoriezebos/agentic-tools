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
UX best practices". Author Fable.

Revision 2, the same evening, after Astra's round-1 read (six material
findings, every one accepted and folded in the dispositions at the foot)
and three rulings Wido gave while it ran: "yes, only through verbs. Which
is powerful because all the UI can do can also be done with a verb in
principle"; "so yes, the agent must formulate in verb statements"; and
"do the actual verb statement design last because there is a parallel
seat working on verbs". Revision 3, later the same evening, after
Astra's round 2, the declared failsafe round: six material findings,
every one a consequence of a round-1 fold, every one a bounded choice,
every one folded and named as a fixture obligation in section 6; the
loop is closed at round 2 on those six obligations, as the critique
skill's principled exit provides, and a scoped confirmation read of the
six folds followed: five confirmed, with the D1 sentence, and one held
on a narrow point, folded in revision 4 as a seventh fixture obligation.
Every cite re-read at `b19a413cf`.

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
   (src/shell/Drawer.tsx:78-91). The Partner's message is appended to the
   transcript only when the whole answer has ended: `run` appends it after
   `host.Prompt` returns, and until then the offers live on the running
   turn in memory (service.go:896-950). Neither tool can name a verb:
   `suggest` names an editor and a field, `deposit` one of five record
   entries, and `deposit` refuses the kind `proposal` in words
   (deposit.go:38-52, 116-118).
2. **The interface names every act it has, with the hand each needs.**
   `Acts()` in internal/ui/httpd/describe.go:44-107 lists twenty-six: nine
   on goals under `ledgerHand` (open, approve, withdraw, set priority,
   block, unblock, park, unpark; edit is routed at acts.go:75 and
   write.go:245 but absent from the table and from the completeness
   test's own route list, describe_test.go:22-31, as is the fleet's
   launch), the checkout writes, the notepad, sign-in and the Partner's
   own routes. The Partner reads the table through `interface(part="acts")`
   (uitools/mcp.go:158-175) and its skill says "Claim no act … say where
   in the interface it is done" (internal/ui/partner/project-partner.skill.md:28).
   The nine goal acts are one POST each with a bounded JSON body
   (acts.go:98-165, 300-430): approve `{elapsedLimit, attemptLimit,
   reservedJobMinutesLimit, activeJobLimit, reviewRoundLimit}`, withdraw
   `{reason}`, park `{because}`, unpark `{}`, priority `{priority,
   sequence}`, block and unblock `{blocker}` with the waiting goal in the
   path, edit `{intent?, nextStep?, labels?}`, open `{id, intent,
   nextStep, tier, why, blocks, blockedBy, labels, severity, novelty,
   exposure, accumulation, basis}`. A confirmed act answers with the
   backlog as it then stands (acts.go:486-497); a refusal is `{error,
   code?}` under 403, 400, 409 or 500 (acts.go:500-527), and one that a
   sign-in would remedy adds `signIn: true` (acts.go:533-540).
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
   An approval binds the goal as it stands when the transaction runs:
   `Approve` loads the tree at the tip inside `Mutate` and `bindApproval`
   digests that goal's intent, tier, budget and risk (internal/goal/approval.go:455-466,
   548-560); the route takes the id and the budget and nothing that names
   what was reviewed (acts.go:272-289). The budget the approve sheet
   offers is `prefillFor`'s: the goal's own tuple, else the project's law
   for its tier, else the last approved goal's, else none
   (src/backlog/moves.ts:104-122). The backlog read has a fetch-first
   form: `loadBacklog(signal, true)` makes the server fetch the canonical
   branch once and then observe, which the Refresh button uses and a
   mount does not (src/backlog/api.ts:296-305); the plain read answers
   from the accepted ref as it stands (internal/ui/snapshot/snapshot.go:116-135).
   The payload says what that fetch did: `fetch.outcome` is `advanced`,
   `current`, `failed`, `running` or `never`, with a failure's message
   (src/backlog/api.ts:162-203; internal/ui/snapshot/loop.go:205-220),
   and the route answers 200 from the accepted ledger whether or not the
   fetch succeeded (internal/ui/httpd/backlog.go:170-186).
4. **The page already runs a list of acts, and its outcome mapping has two
   false branches.** `runInOrder` sends one publication per goal in order,
   never retries, and stops at the first answer that is not one
   (src/decisions/decisions.ts:671-687); a stopped run is terminal (:654-656);
   the sheet cannot be dismissed mid-run (:642-644); an approval's budget
   is `prefillFor`'s, and a goal with no prefill is listed and excluded
   with "needs its budget first: approve it alone" (:576-603). `outcomeOf`
   maps a failed act: a 4xx with the engine's sentence is refused; 5xx,
   the codes it lists as unsettled (`confirmed-late`, `lost`, `abandoned`,
   `expired`), the `proof-not-recorded` answer and a transport failure are
   unresolved (src/backlog/editing.ts:216-260). Both halves misread the
   transaction. Its terminal outcomes `rejected`, `abandoned`, `expired`
   and `lost` are each marked before any push lands or after every push
   was refused: abandoned at the pre-push gates and on a mutation that
   finds nothing to do, rejected on validation, lost to a competitor
   whose change stands instead of ours, expired when the compare-and-set
   was refused past the deadline (internal/goal/txn.go:717-773, 856-865,
   895-925); every one is a definite non-write, so a redundant block in a
   list (`NothingToDo`, verbs.go:2930) answers 409 `abandoned` and the
   page calls it unresolved. And when a push lands and its confirming
   refetch fails, or the opid is not on the refetched tip, or the
   confirmed follow-on effect fails, the transaction returns an empty
   outcome whose detail begins "pushed;" together with an error
   (txn.go:805-830), and `settle` turns any publish error into
   `KindEngine` with code `refused`, a 409 (internal/ui/act/act.go:515-518,
   510-521 of httpd/acts.go), which the page calls refused. The journal
   knows the difference: the entry is still at `PhasePushed` with no
   terminal outcome, readable by its opid (`goal.ReadEntry`,
   internal/goal/journal.go:217-228, which wraps a missing file and a
   malformed one as errors; `PushedBlocking`, :508-519); `MarkTerminal`
   itself reads the entry first and fails on the same fault
   (journal.go:395-412), so one persistent journal fault can leave a
   landed act unmarked. `proof-not-recorded` is the one answer that says
   the act landed and must not be run again (act.go:533-535), and
   `outcomeOf` folds it into unresolved (editing.ts:256). The act clients
   are one `request()` each (src/backlog/api.ts:259-270, 312-397). No act
   carries a client key: the act layer mints a fresh operation id per
   request (act.go:566), so a repeated POST is a second attempt, and a
   repeated approve is not a no-op but a second approval record
   (approval.go:502-503); park, unpark, open and block refuse a repeat in
   the engine's words. A refusal's `signIn` flag opens the sign-in sheet
   through `askToSignIn(again)`, which runs `again` once when the human
   signs in and discards it, silently, when the sheet is closed instead
   (src/shell/identity.tsx:106-118, 132-136).
5. **A page re-reads whole after an act; four offer that read to the
   shell; every offered re-read blanks the page, and two pages hold text
   outside any sheet.** Decisions, Overview, Fleet and Application offer
   their re-read through `useOffersRefresh` (src/shell/refresh.tsx:44-52),
   and each offered `reload` sets a loading state that unmounts the page
   (DecisionsPane.tsx:150-155, OverviewPane.tsx:107-110,
   ApplicationPane.tsx:89-92, FleetPane.tsx:116-119). Fleet also has an
   in-place re-read, `again`, which its presence event and the stream's
   reconnect use and which "keeps whatever is on screen until the answer
   arrives" (FleetPane.tsx:112-115, 121-142); its failed launch's Retry
   form, an authorization word and a review date, is inline in the page
   and in no sheet (src/fleet/LaunchCard.tsx:136-175). The goal page holds
   two payloads: the project's, from which the goal's title, state and
   intent are shown, and the ledger's, for the relation between goals
   (src/project/ProjectPane.tsx:149-165, 792-800); its `reload` blanks
   the page (:202-205) and its edit sheet's confirmed save re-reads the
   project the same way through `onEdited`, while an unconfirmed save
   sets the board in place and keeps the sheet mounted (:878-891). The
   board carries its own refresh in its strip and offers none. Every
   sheet renders into the work area's modal layer, which makes the
   content under it inert while a sheet is open and counts the sheets
   covering it in a private map (src/shell/workmodal.tsx:78-95, 128-142).
   Nothing on the stream announces a ledger change; the pages read on
   mount, on Refresh and after their own acts.
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
   operation failed or never happened" (:780); "refresh affected saved
   records and lists after a confirmed mutation. Preserve unsaved editor
   buffers" (:346).
7. **A parallel seat is redesigning the public verbs.** Goal
   `verbs-match-intent` (metasystem/plans/goals/verbs-match-intent.md,
   queued, its cleanup in flight) and its design (metasystem/plans/designs/intent-workflows.md)
   rule that "one existing descriptor table owns grammar, audience, help
   group and visibility. No second command inventory", and that "the
   Partner consumes the same public catalogue; no doubled `metasystem`,
   hidden aliases or engine families appear" (:80-97); goal creation,
   editing, approval, budgets, pause and resume, abandonment, dependencies
   and priority "stay explicit public capabilities" (:266-269). Wido's
   ruling: the Partner's verb statements are designed last, against what
   that seat lands.

## 2. How it works, from the chair

You are on Decisions. Six `headless-fleet` goals are three weeks old and
you know why. You tell the Partner: "these six are superseded by the seat
inventory; put them away". It answers in a sentence and, under the
answer, a card fills line by line as it proposes; when its last word
lands, the card's buttons wake:

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
│   The Partner: the five name the fleet inventory g1-s42 built; the  │
│   sixth was approved against a date that has passed.                │
│ 6 selected                          [Apply 6]   Select all · Dismiss│
╰─────────────────────────────────────────────────────────────────────╯
```

You untick the withdrawal; you want to think about that one. "5
selected · Apply 5". You press it. The first line says "applying…", then
"applied"; the row leaves the queue on the page behind the drawer as you
watch, and the count in the block's head drops. The second, the third.
The fourth says "refused: goal fleet-idle-alerts is claimed by
mac-mini/01M3…; edit it at a terminal" in the danger colour, and the run
goes on: the fifth applies. The foot reads "4 applied · 1 refused"; the
refused line offers "Try again" and "Ask the Partner", and the
withdrawal is still there, unticked and pressable whenever you decide.
Had the fourth come back with no answer at all, the line would say
"unresolved: <what was said>; check the goal before trying again", the
run would stop there, the fifth would say "not run", and the foot would
offer "Continue with the rest". The Partner's next answer knows all of
it.

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
  its words. An approval shows the intent and the next step whole, as
  the Decisions open row does before its Approve, and the budget it
  would carry with where that budget came from, exactly as the approve
  sheet does.
- **I choose which.** A checkbox per action; one, some or all. Apply says
  how many. A card of one action has no checkbox and one button.
- **I see each one land, or not, on the page I am looking at.** Each line
  says applying, applied, refused with the engine's sentence, or
  unresolved with what was said; the page in view reads again after each
  confirmed act and when the run ends, keeping what is on the screen
  until the answer arrives, and not at all while a sheet of mine is open
  on it, in which case it reads again the moment I close the sheet.
  Nothing I typed anywhere is touched.
- **A failure tells me what and why, leaves the rest intact, and offers
  the way on.** A refusal is the engine's own sentence on that line, and
  the run goes on to the next, because a refusal is a no with nothing
  behind it. An unresolved answer stops the run, because the act may
  have landed and the next one may depend on it. Continue with the rest
  is one press, Try again on a refused or unresolved line is another,
  and Ask the Partner puts the line's words in the composer so the
  Partner can propose differently.
- **The interface never applies anything twice by itself.** Each action
  is sent at most once per press; a line that says unresolved is never
  re-sent by the page. A line whose answer says the act landed but its
  proof was not recorded says "applied; its authority proof was not
  recorded" and offers no Try again at all. A reload during a run shows
  the line that was in flight as "was being applied when the page left",
  not as fresh.
- **What I approve is what I read, with the budget I read.** Before a
  run the page fetches the canonical branch once. An approval or an edit
  applied against a goal whose intent, next step, tier or labels changed
  since the Partner read it, or whose budget would now differ from the
  one on the card, is refused on the line, "the goal changed since this
  was proposed", with Open the goal, so a stale card never authorises
  work or a budget I did not read.
- **Ignoring is allowed.** A card you do not act on stays proposed, with
  its ticks; Dismiss folds it to one line that reopens; nothing expires
  into an act.
- **Human-only acts need my sign-in and nothing else new.** When the
  session is not proven the button says "Sign in to apply" and opens the
  sign-in sheet; the run starts when the code is accepted. A session that
  expires mid-run opens the sheet at that line; signing in runs on from
  that line; closing the sheet leaves that line and the rest "not run",
  with Continue with the rest waiting.

The practices this follows, so a reader can check them rather than
trust them: visibility of status (every line has one, the foot counts
them, the page moves); match between the words on the card and the
buttons on the pages; user control (select, dismiss, continue, try
again, never an automatic act); error prevention (arguments validated
before the card exists, the engine's checks at the act, the approval's
substance and budget shown, a stale approval refused); recognition over
recall (the subject named and linked, the arguments visible); errors
that name the cause and the recovery in the engine's words; consistency
with the two cards that exist (Use this, Record it) and with the bulk
run the Decisions page already has; and one card, one press, for the
common case.

## 3. Decisions

- D1. **A proposal is a verb statement, and only that.** The Partner
  proposes by calling one tool, `propose`, once per action, with a verb
  from a closed catalogue, the goal it names, the arguments that verb
  takes and its `why`; words in an answer propose nothing. The tool
  validates shape and bounds only, the way `deposit` does (an unknown
  verb refused with the catalogue in words; a missing field refused by
  name; texts bounded as the sheets bound them; line breaks in an intent
  or next step refused as the route refuses them), writes nothing, and
  answers "prepared; preparing applies nothing: the human sees the
  action on a card and decides whether to apply it". Its result carries
  the action in the fixed frame the host already reads once: a header
  line naming the verb, labelled lines for the fields, the separator,
  and the `why` opaque to the end. **The grammar is designed last.** The
  verb names, their fields and the words each line uses are settled
  against the public catalogue the `verbs-match-intent` seat lands,
  because that design rules one descriptor table and no second
  inventory, and the Partner is to consume that same catalogue. Until
  then the mechanism is built and proven against a provisional
  catalogue: the nine goal acts the interface routes today, named by
  their route ids (`open-goal`, `approve-goal`, `withdraw-goal`,
  `set-goal-priority`, `block-goal`, `unblock-goal`, `park-goal`,
  `unpark-goal`, `edit-goal`) with the fields their bodies take, an
  approval taking no budget because the interface supplies the one the
  approve sheet would. Nothing below depends on which names the grammar
  ends with: what the message persists and the runner dispatches on is
  the route id, which does not change; the grammar is the tool's table
  from public name and field to route id and body, and the card's words
  are one map from route id to the page's button word, so a rename
  touches those two and never a persisted proposal.
- D2. **The host keeps it whole; the service admits it against the tip and
  carries what it read.** A completed `propose` call is read into
  `Action{verb, goal, fields, why}` beside its look, a failed call
  yielding none. The service admits it against the observation the turn
  was composed from (`Facts.Observe`, context.go:43-51): every goal id the
  action names, the subject and a blocker, must be at the accepted tip,
  or be the id of an `open` admitted earlier in the same answer, so
  "open X, then block Y by X" is one proposal; an `open` whose id is
  already at the tip is refused "goal <id> already exists". An admitted
  action carries the subject's title as the pages say it and, for an
  approve or an edit, the goal's intent, next step, tier and labels as
  read, under `read`; for an `open` the title is the intent's first
  sentence. A refused action is recorded with `offered: false` and its
  reason, shown on the card as a "not offered" line, never dropped to an
  activity line (g1-s52 D3). Admission checks existence and nothing else:
  whether the act is allowed in the goal's state is the engine's answer
  at the act, in its own words, as it is for every button.
- D3. **The card is the confirmation control, and it wakes when the
  answer ends.** Under the answer, one card for the answer's actions, in
  the order proposed: a checkbox and the line of D4 per action, the
  Partner's why under its action, the foot with "n selected", Apply n
  (primary), Select all, Dismiss. A card of one action has Apply and
  Dismiss and no checkbox. Lines render as they are admitted; the
  checkboxes and buttons are enabled at the answer's terminal beat
  (done, stopped or error), because until then the message the outcome
  of D6 is recorded on does not exist. At that beat the card reads the
  backlog once to fill each approve line's budget from `prefillFor`, and
  keeps the tuple and its source it displayed with the line. The card
  carries the whole reviewed basis, so no sheet repeats it: a second
  dialog listing what the card already lists would be a press that adds
  nothing to read. Pressing Apply runs the ticked actions in order.
- D4. **A line is the verb's word, the subject and every argument.** The
  verb word is the button's on the page that offers the act; the subject
  is the title with the id small, linked to the goal page ("Open the
  goal"); the arguments in the sheets' own labels: because, reason,
  priority and position, waits for, intent, next step, labels, and for an
  open the tier derived from the four answers with the answers named,
  the basis, labels, waits for and unblocks. An approve line shows the
  intent and the next step whole from `read` and "with budget <the tuple
  in the sheet's five words>, from <the source's words>", or "needs its
  budget first: approve it alone", unticked and not sendable
  (decisions.ts:576-603); an edit line shows each field it sets. The tier
  is derived on the card by the new-goal sheet's own function and sent
  as derived; the Partner names no tier.
- D5. **The run: one act per line, refusals passed, the unknown stopped
  at.** The runner sends the ticked lines in order through the existing
  clients (backlog/api.ts:312-397), one act each, never retried. Before
  the first line it reads the backlog once with the fetch-first form
  (`loadBacklog(signal, true)`) and reads the payload's `fetch.outcome`:
  only `advanced` or `current` means the compare below is against the
  canonical branch as of the press, and on `failed` every approve and
  edit line is refused unsent with the fetch's own message, "the
  canonical branch could not be read: <words>; try again", while the
  lines that need no compare run as they would; an approve or edit line
  whose goal's current intent, next step, tier or
  labels differ from `read`, or whose `prefillFor` now answers a tuple
  other than the one the line displays, is refused on the line without
  being sent, "the goal changed since this was proposed; open it and ask
  again"; an approve line that passes sends the tuple it displayed and
  no other. A sent line takes its outcome from `outcomeOf`
  (editing.ts:252-260), corrected as D6 says: a definite refusal, which
  is every 4xx the engine explains, including the transaction's own
  `rejected`, `abandoned`, `expired` and `lost`, is the engine's sentence
  on the line and the run goes on, since nothing landed; an unresolved
  answer stops the run, the lines after it say "not run", and the foot
  offers Continue with the rest, which runs them from the first not-run
  line; the answer whose code is `proof-not-recorded` is recorded as
  applied with its words and offers no Try again. A refused or
  unresolved line offers Try again, which sends that one line again as
  the human's press, and Ask the Partner, which fills the composer with
  the verb, the subject and the sentence. A line is sent at most once per
  press; the card refuses a second Apply while a run is in flight, on a
  synchronously held ref as the edit sheet guards its save (g1-s56 D1).
  After each confirmed act and when the run ends, the store asks the
  section's offered re-read, but only while no sheet covers the work
  area: the modal layer's provider keeps its cover count in state and
  exposes `covered`, and a re-read asked while covered is made when the
  count returns to zero. **An offered re-read keeps the page mounted**:
  it sets what it read in place and never passes through the loading
  state, which is for the first read and for Retry after a failure; the
  four pages that offer one today change their offered function to say
  so, Fleet offering the in-place `again` it already has, so its inline
  Retry form and every other text outside a sheet survives. The board
  and the goal page offer their re-read through `useOffersRefresh` with
  one added flag, `inStrip`, that keeps the header from showing a second
  icon beside the one their strip has; the goal page's offer re-reads
  both its payloads, the project's and the ledger's, and sets both in
  place, so the title, the state chip and the intent it shows follow a
  confirmed act on the goal in view.
- D6. **The outcome is recorded where the proposal is, before and after
  the act; the act layer says what it knows and no more.**
  `Message.proposals[]` on the Partner's message, one entry per admitted
  or refused action, each with `state: waiting | applying | applied |
  refused | unresolved | dismissed`, `words` and `at`. Before sending a
  line the runner records `applying`; after the answer it records the
  outcome; both through one conversation route, `POST
  /api/partner/turns/<turn>/proposals/<index>` with `{state, words}`,
  under the conversation hand: it carries no authority and changes
  nothing but the transcript. The route serves persisted messages only
  and refuses a turn still running, in words. The conversation rewrites
  that one message in place through the atomic writer the trim already
  uses. An `applying` write that fails stops the run at that line with
  its words and sends nothing. The card reads its lines from the message,
  so a reload shows applied as applied, and a line left at `applying` as
  "was being applied when the page left; check the goal before applying
  again", never as fresh: an approve applied twice is two approval
  records, not a no-op, so a card that forgot it was applied would invite
  the one press that writes one. Dismiss records `dismissed` on every
  waiting line. If an outcome write fails after the act, the line keeps
  its state for the page's life and says "the conversation could not
  record this". In the act layer, `settle` learns the distinctions it
  lacks: on a publish error it reads its own journal entry by opid
  (`goal.ReadEntry`); an entry at `PhasePushed` with no terminal outcome
  answers `KindFailed` with code `pushed-unknown`, "pushed; whether it
  landed is unresolved: <the detail>; the page's next read says what the
  ledger did"; an entry the journal cannot read for any reason but its
  absence answers `KindFailed` with code `journal-unreadable` and the
  read's own words, because nothing then proves the push did not land;
  an absent entry, one at `PhaseCreated`, or a terminal one answers
  `refused` as today, because the push never left or was refused. Both
  new codes are 500 and the page's mapping already calls them unresolved.
  The page's mapping itself is corrected in the one place it is written:
  its unsettled list keeps `confirmed-late` alone, so `rejected`,
  `abandoned`, `expired` and `lost` are refused, which is what the
  transaction means by them. Every act from this browser inherits both
  corrections, the edit sheet's Use and save and the Decisions bulk
  sheet included.
- D7. **Sign-in is as every act's, and never holds the run.** With no
  proven session the card's button reads "Sign in to apply" and opens
  the sign-in sheet through `askToSignIn(run)`. A refusal carrying
  `signIn` mid-run ends the run at that line: the line says "not applied:
  sign in to apply", the lines after it say "not run", the foot offers
  Continue with the rest, and the sheet is opened through
  `askToSignIn(again)` with `again` being "run on from this line"; a
  sheet closed without signing in changes nothing, because nothing was
  waiting. The act publishes under the human's session and the ledger
  names the human; the transcript is the record that the Partner
  proposed it and when the human applied it, which is the master's
  "authorship retained separately" for this step.
- D8. **Where the human looks.** The card is in the transcript, in the
  drawer and in the focused view; the drawer's one-shot growth for a card
  (Drawer.tsx:78-91) covers it; while a proposal of the last answer has a
  waiting line and the drawer is closed, its bar says "n actions
  proposed" beside the composer, and pressing it opens the drawer at the
  card, as the sitting's counts open the table.
- D9. **The Partner is told, and told what happened.** Its instructions
  gain: "Everything the interface can do is a verb, and a proposal is a
  verb statement: when the human asks you to do something the interface
  can do to a goal, do not do it and do not say it is done; call
  `propose` once per action with the verb, the goal, its arguments and
  why, beside your answer, and say you have proposed it. Words alone
  propose nothing. The human decides which to apply, and the card is the
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
  catalogue; no editing of an argument on the card; no undo; no
  owner-side basis compare inside the transaction: the fetch-first read
  and the page's compare of D5 leave the card the window between that
  read and the transaction's own compare-and-set, which is the window
  every Approve button from this browser has today and narrower than the
  sheet's, whose row can be five seconds old; the digest `bindApproval`
  already computes is the next step's material; no Partner authorship in
  the ledger's History; no marker on the subject's row on the board.

## 4. Step 1, and what waits

Step 1 is D1 to D9 for the provisional catalogue of nine goal verbs,
with the grammar of D1 as the last unit of the build: if the verbs seat
has landed by then, the names and fields are taken from its descriptor
table; if not, the provisional names ship and the grammar is step 2, one
table and one map. It is what a human can use after one slice: propose
one act or many, tick, apply, watch the page move, read a refusal in the
engine's words, continue, try again, ask the Partner, sign in when
asked, reload and find the truth.

Later, when it hurts: the checkout writes as verbs (set a record's
status, settle a question, ask a question, write a record: one catalogue
row, one admission rule against the project reader and one client each);
"Review in the sheet" on a line, opening the verb's own sheet prefilled
so an argument can be edited before applying; a "proposed" marker on the
subject's row on the board and in the queue; abandon, after its ruling;
the Partner's authorship on the ledger's history line; the owner-side
basis compare on the approve and edit routes, checked inside the
transaction on `ApprovalDigest` (gate 2's review basis); keyboard
operation of the card; proposals across several answers as one list;
bulk Try again; the launch route's row in the act table; the Decisions
bulk sheet taking the same refusal-passed run rule; `confirmed-late`
shown as landed rather than unresolved.

## 5. Payload and routes

The tool: `propose` as D1, its result in the fixed frame `Proposal: <verb>`,
labelled lines `Goal: `, `Reason: `, `Because: `, `Priority: `,
`Sequence: `, `Blocker: `, `Intent: `, `Next step: `, `Labels: `, `Id: `,
`Severity: ` … `Basis: `, `Blocked by: `, `Blocks: `, then the separator
and the why. `Message.proposals: [{ index, verb, goal, title, fields: {…},
read: { intent, nextStep, tier, labels } | null, why, offered, reason,
state, words, at }]` on a Partner message, the same object on the stream
as `proposal` events as each is admitted, and on the snapshot's running
turn as `proposals`. The route `POST /api/partner/turns/<turn>/proposals/<index>`
with `{state, words}`, the conversation hand's policy, answering the
snapshot; a state that is not one of the six, an index the message does
not carry, a turn that is not this conversation's or one still running
is refused with words. The act refusal codes `pushed-unknown` and
`journal-unreadable` under 500; the page's unsettled list reduced to
`confirmed-late`. The describe table gains `{ID: routeEditGoal, Title:
"Edit a goal", Requires: ledgerHand}` and the new route's own row, and
the completeness test's route list (describe_test.go:22-31) gains both,
so the table cannot lose them again; the launch route's missing row is
noted for later, not this slice's. The refresh offer gains `inStrip` and
its contract that the offered function keeps the page mounted; the
work-area provider exposes `covered`. Nothing else changes shape.

## 6. Verification and box

Go: the tool's catalogue, every verb's required fields and bounds, an
unknown verb and a foreign field refused in words, the fixed frame; the
host reading a completed `propose` into a whole action and a failed one
into none, the why opaque past the separator; the service admitting an
action whose goal is at the tip with `read` filled, admitting a block
whose blocker an earlier `open` of the same answer named, refusing an
`open` of an existing id and an act on an unknown goal with their
reasons, the title carried; the message, the stream and the snapshot
carrying it; the outcome route's policy, its six states, its refusal of
a running turn and of an unknown index, and the message rewritten in
place with the rest of the transcript untouched; the context block's
line from recorded states; the describe table's row and a join test
that every ledger act in the catalogue is in the table and every table
row the catalogue admits is in the catalogue. The six fixture
obligations from Astra's round 2 and the one from its confirmation
read, one test each, in the act layer and the page: (S58-07) the run's
first read is the fetch-first form, a goal whose intent changed between
the card and the press is refused on the line unsent, and a payload
whose fetch failed refuses every approve and edit line unsent with its
message while an unguarded line still runs; (S58-08) an approve line sends the tuple it displayed,
and a line whose `prefillFor` now answers another tuple is refused
unsent; (S58-09) `settle` answers `pushed-unknown` for a publish error
whose journal entry is pushed and not terminal, `journal-unreadable` for
one whose entry cannot be read, and `refused` for an absent, created or
terminal entry, through a fixture publish that returns the transaction's
own "pushed;" result with its error and a journal made unreadable;
(S58-10) Fleet's offered re-read keeps its Retry form mounted with its
text; (S58-11) the goal page's offered re-read refreshes the title and
the intent it shows and keeps its columns mounted; (S58-12) a redundant
block answered `abandoned` is refused, not unresolved, and the next line
is sent. Frontend, beyond those: the line's words for each verb, the
tier derived and the answers named, the approval's intent, budget and
the excluded line; the card's states as pure functions with the run's
rules, a refusal passed and the next line sent, an unresolved answer
stopping the run and Continue running the rest from the first not-run
line, `proof-not-recorded` recorded as applied without Try again, Try
again sending one, the ref guard refusing a second Apply mid-run, the
buttons disabled until the terminal beat; the two outcome writes per
line, a failed `applying` write stopping the run unsent, and the reload
showing `applying` as in flight; Sign in to apply, a `signIn` refusal
ending the run with the sheet's success running on from the line, and a
closed sheet leaving the card settled; the offered re-read called after
a confirmed act, deferred while `covered` and made when it clears; the
`inStrip` flag hiding the header's icon; every offered re-read keeping
its page mounted; the bar's count; the guards stay green, with the new
call sites rowed in `cuts.test.ts`. Walkthrough: the fake Partner answers
a canned proposal of three actions and a canned single open; the
fixture's canned acts refuse one goal by name and answer 500 for
another, so a refused line passed and an unresolved line stopping the
run are both seen; screenshots at 1280 and 400: the card of six, four
applied and one refused, the single open, the reload with a line in
flight, the bar's count. Budgets as always. Box: one build lane (Claude
on Opus), one code read (Codex on Sol) with one fix round under R-124,
after Astra's confirmation read; two attempts, 240 to 360 job-minutes.

## 7. Self-grade

High on D1, D2 and D7: the third tool in a pipeline that has carried
two, the act clients and the sign-in path every button uses, with the
run never waiting on a sheet. High on D3 and D4: the card says what the
sheets say, in their words, and is the master's own result card with
the reviewed basis on it. Medium on D5: the run rule differs from the
bulk sheet's in passing a refusal, which rests on the corrected mapping
of D6 being true of the transaction, and the six fixtures of section 6
are what hold it; the freshness guard is the page's compare after a
fetch and not the owner's digest. Medium on D6: the first write into a
Partner message after it was appended, one route with six states, and
two corrections to code every act shares. Weakest: the grammar comes
last by ruling, so the build proves the mechanism against route ids and
the words a human reads on the line are the one thing that may change
after the seat lands; the map from route id to word is kept in one place
for that reason.

## Dispositions (Astra round 1, 2026-09-26, under R-124)

Six material findings, all accepted; one non-material row corrected.
Every code claim re-read before folding.

| id | finding | fold |
|---|---|---|
| S58-01 | a landed push whose confirmation failed comes back as a publish error, which `settle` answers as a 409 `refused` and the page calls refused: a landed approval reported as refused, to the human and to the Partner | `settle` reads its own journal entry on a publish error; pushed-and-not-terminal answers `pushed-unknown` under 500, which the existing mapping calls unresolved; the runner stops there (D6) |
| S58-02 | an approval line shows a title and a budget while the engine binds the goal as it stands at the act, so a stale card authorises work the human never read | an approve or edit action carries the goal's intent, next step, tier and labels as read; the line shows the intent and next step whole; the runner refuses a line whose goal changed, unsent, with words (D2, D4, D5); the owner-side digest stays the next step |
| S58-03 | the goal page's re-read passes through a loading state that unmounts its columns and the edit sheet in them, losing an unsaved draft on an unrelated act | the re-read is deferred while a sheet covers the work area and made when it clears; the goal page's offer re-reads in place (D5) |
| S58-04 | lines are streamed before the Partner's message exists, so an Apply mid-answer has no message to record `applying` on | the card's buttons wake at the answer's terminal beat; the route refuses a running turn; a failed `applying` write sends nothing (D3, D6) |
| S58-05 | Try again on a `proof-not-recorded` answer sends the landed act again | that answer is recorded as applied with its words and offers no Try again (D5) |
| S58-06 | the sign-in sheet has a success callback only, so a run awaiting sign-in never settles when the sheet is closed | the run never waits: a `signIn` refusal ends it at the line, the sheet's success runs on from that line, and a closed sheet leaves the settled card (D7) |

Non-material, corrected: six states named consistently; the first
scene's count. Folded from Wido's rulings during the read: a refusal is
passed and only an unresolved answer stops the run; the principle and
the skill sentence of D9; the grammar of D1 designed last against the
verbs seat's catalogue, with a provisional catalogue for the build.

## Dispositions (Astra round 2, the failsafe round, 2026-09-26, under R-124)

Six material findings, every one a consequence of a round-1 fold, which
the critique skill names as the loop critiquing itself and the point to
stop. Every one is mechanical-grain, a bounded choice rather than an
invariant, and every one is folded and named as a fixture obligation in
section 6, so the loop closes at round 2 on six fixture obligations
under the skill's principled exit; the code read is the arbiter prose
review stopped being. One finding is accepted in part. Every code claim
re-read before folding.

| id | finding | fold |
|---|---|---|
| S58-07 | the freshness compare reads the browser's accepted tip, which can lag the canonical branch, and stands outside the transaction's compare-and-set | accepted in part: the run's first read is the fetch-first form, so the compare is against the canonical branch as of the press (D5); the in-transaction compare is refuted as step 1: the window left is the one every Approve button has today and narrower than the sheet's, and the owner-side digest is the next step (D10, section 4) |
| S58-08 | the card displays one budget and the run takes another from a fresh `prefillFor`, so the confirmed tuple is not the sent one | the card keeps the tuple and source it displayed with the line and sends that tuple; a fresh prefill that differs refuses the line unsent (D3, D5) |
| S58-09 | a journal that cannot be read leaves `MarkTerminal` and the new `ReadEntry` failing alike, so a landed act reads as refused with a retry | an unreadable entry answers `journal-unreadable` under 500, unresolved; only an absent, created or terminal entry is refused (D6) |
| S58-10 | Fleet's inline Retry form, an authorization word and a date, is in no sheet, and Fleet's offered `reload` blanks the page | every offered re-read keeps its page mounted; Fleet offers the in-place `again` it has (D5) |
| S58-11 | the goal page's title, state and intent come from the project payload, which an in-place ledger re-read leaves unchanged | the goal page's offer re-reads both payloads and sets both in place (D5) |
| S58-12 | a redundant block answers 409 `abandoned`, which the page's unsettled list calls unresolved, so a definite no-op stops the run | the unsettled list keeps `confirmed-late` alone; `rejected`, `abandoned`, `expired` and `lost` are refused, as the transaction means them (D6) |

Non-material, folded because it costs one sentence: a rename after the
grammar lands touches the tool's table and the word map and never a
persisted proposal, because the message persists the route id (D1).

## Dispositions (Astra's scoped confirmation read, 2026-09-26)

Checked at `38c8690ea`, on the six folds and the D1 sentence only.
S58-08, S58-09, S58-10, S58-11, S58-12 and the D1 sentence confirmed as
folded; Astra also confirmed D10's premise that the existing Approve
buttons submit without a basis compare (ActSheet.tsx:83, BulkSheet.tsx:102,
DecisionsPane.tsx:178). S58-07 not confirmed on one narrow point,
folded in revision 4:

| id | finding | fold |
|---|---|---|
| S58-07 | the fetch-first read answers 200 from the accepted ledger even when its canonical fetch failed, so the compare can run against a stale tip; the payload records the failure and D5 did not read it | the runner reads `fetch.outcome`; on `failed` every approve and edit line is refused unsent with the fetch's message, unguarded lines run as they would; the seventh fixture obligation in section 6 (D5) |

The loop is closed. No further design round is run on the designer's
own judgment; whether to spend one more scoped read on this one rule,
or to give the go, is Wido's.
