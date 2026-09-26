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
Revision 5: the verbs seat landed on main and was merged as `8a22b6284`,
so the grammar unit of D1 and D4, ruled last, is written against its
descriptor table. Every cite re-read at `b19a413cf`; the merge changed
three words in the act layer's comments and none of the cited lines,
and touched no other cited file. Revision 7, on Wido's UX question of
where proposals belong: the conversation is where a proposal is made
and answered, and the wrong place to keep one that waits; step 1 folds
older cards to one line, and the Decisions inbox becomes the waiting
proposals' home in step 2 (D8, section 4). Revision 8, on Wido's word
later the same evening: "The work on the verb system is unfortunately
not finished. There will be complete restructure of the verb system,
so do not yet get into the details of the verbs themselves. Once the
work on the verb system has completed, you can pull that in and then
continue with that." So nothing in step 1 depends on the public verbs'
names, flags or descriptor table: D1's grammar goes back to the
interface's own route ids and route bodies, D4's words back to the
pages' buttons, and the public grammar is the unit that waits for that
work to land (section 4). Revision 5's grammar and Astra's read of it
stand in the record below as what will be taken up then.

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
   list (`NothingToDo`, verbs.go:2926 after the merge) answers 409
   `abandoned` and the page calls it unresolved. And when a push lands and its confirming
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
7. **The public verbs are one table, and that table is being
   restructured.** Wido, 2026-09-26, late: the verb system's work is not
   finished and will be restructured completely; nothing here may depend
   on the verbs' details until it lands. What follows in this item is
   the state of the table as merged, kept so the grammar unit can be
   taken up against its successor. Goal `verbs-match-intent`
   (metasystem/plans/goals/verbs-match-intent.md) and its designs
   (metasystem/plans/designs/intent-workflows.md, verb-cleanup.md,
   agent-help.md) rule that "one existing descriptor table owns grammar,
   audience, help group and visibility. No second command inventory",
   and that "the Partner consumes the same public catalogue; no doubled
   `metasystem`, hidden aliases or engine families appear"
   (intent-workflows.md:80-97). The table is `intentCommand` in
   cmd/metasystem/intent.go:55-70: a name, a summary, flags each marked
   advanced or hidden where they are authority or plumbing, usage forms,
   a help group and a scope; `publicIntentCommands()` (intent.go:902) is
   the 44 current public commands, and `commandCatalogue()`
   (cmd/metasystem/ui_describe.go:87-96) projects them, name, summary,
   scope and usage, into the Partner's `kit` tool. A flag's `advanced`
   mark keeps it off the command's short help, and several data flags
   carry it: `label`, `unlabel`, `blocked-by`, `blocks`, `sequence` and
   `priority`-as-a-flag (intent_planning.go:40, 98, 124, 194); the
   authority flags `by`, `temporary-human-word` and `review-by`, and the
   hidden `lineage`, `fixture-human-authority` and `approved-ref`, carry
   it too (intent.go:71-88), as does `under` on approve and resume. The nine public forms
   the interface's nine acts correspond to, from `help all` of the merged
   executable: `open G --intent TEXT --next TEXT --risk ANSWERS --basis
   TEXT` with `--label`, `--blocked-by` and `--blocks` repeatable;
   `approve G... [--budget BOX]`, whose default is each goal's tier norm
   box; `unapprove G --reason TEXT`; `prioritize G 1|2|3 [--sequence N]`;
   `block G --on G2`; `unblock G --on G2`; `pause G --reason TEXT`;
   `resume G`; `edit G [--intent TEXT] [--next TEXT | --next-append TEXT]`
   with `--label` and `--unlabel` repeatable and `--risk`, `--basis`,
   `--tier` and `--evidence` beside them. Fourteen more public goal verbs
   have no act in the interface: `abandon`, `done`, `reopen`, `budget`,
   `claim`, `release`, `accept-risk`, `pin`, `grant`, `revoke`, `split`,
   `group`, `ungroup`, `notes`. Wido's rulings: the Partner formulates
   proposals as verb statements; everything the interface can do is a
   verb; and the grammar is designed last, against this table.

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
  and unblock; when the public verb system lands, the buttons and the
  card take its words together (section 4). The subject is the goal's
  title, as the pages say it, with its id small. Every argument the act will carry is on the card, whole,
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
- **Ignoring is allowed, and an ignored proposal is not lost.** A card
  you do not act on stays proposed, with its ticks; Dismiss folds it to
  one line that reopens; nothing expires into an act. Once a newer
  answer stands under it, a card with waiting lines folds by itself to
  one line, "The Partner proposed earlier · 3 waiting", that reopens,
  so a stale Apply is never the first thing in view and a conversation
  does not bury a decision. The conversation is where a proposal is
  made and answered; where it waits, when you do not answer it, is the
  Decisions inbox, in step 2, beside every other choice that waits on
  you.
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
  and the explanation opaque to the end. **Step 1's catalogue is the
  interface's own** (revision 8): the nine verbs are the route ids the
  interface's act table names, `open-goal`, `approve-goal`,
  `withdraw-goal`, `set-goal-priority`, `block-goal`, `unblock-goal`,
  `park-goal`, `unpark-goal`, `edit-goal`, and each verb's fields are its
  route body's own, under the body's names (section 1 item 2): open
  takes `id`, `intent`, `nextStep`, `severity`, `novelty`, `exposure`,
  `accumulation`, `basis`, `labels`, `blockedBy`, `blocks`; withdraw
  `reason`; park `because`; priority `priority` and `sequence`; block
  and unblock `blocker`; edit `intent`, `nextStep`, `labels`, at least
  one, the whole label list as the route takes it; approve nothing, the
  card supplying the tuple. The tool's description lists the nine with
  the button word each has on the pages. Nothing in step 1 reads the
  CLI's descriptor table or imports anything of it; the join test holds
  the catalogue to the interface's act table (`Acts()`, describe.go)
  and nowhere else. What the message persists and the runner dispatches
  on is the route id, so the public grammar can be put on later without
  touching a persisted proposal. **The public grammar below waits for
  the verb system to land** (Wido, 2026-09-26: the verb system is being
  restructured; do not get into the verbs' details yet); revision 5's
  grammar and Astra's read of it are kept here as what will be taken up
  against the successor table, and nothing below it binds the build.
  The verb is the public command's name and the fields are its
  public flags, so the Partner, a human at a terminal and the interface's
  help say one word. The nine, each with the route it
  dispatches to and the body the tool composes:

  | verb statement | route | body |
  |---|---|---|
  | `open G --intent TEXT --next TEXT --risk severity=N,novelty=N,exposure=N,accumulation=N --basis TEXT [--label L]… [--blocked-by G2]… [--blocks G3]…` | `open-goal` | id, intent, nextStep, the four answers parsed from `risk` by the command's own owner (`goal.ParseRiskRecord`, internal/goal/file.go:149), basis, labels, blockedBy, blocks; the tier derived on the card; why empty |
  | `approve G` | `approve-goal` | the tuple the card displays (D3) and never one the Partner names: `--budget` is refused in words, the command's own default being the tier's norm box and the interface's the sheet's prefill |
  | `unapprove G --reason TEXT` | `withdraw-goal` | reason |
  | `prioritize G 1|2|3 [--sequence N]` | `set-goal-priority` | priority, sequence |
  | `block G --on G2` | `block-goal` | blocker G2, with G in the path |
  | `unblock G --on G2` | `unblock-goal` | blocker G2, with G in the path |
  | `pause G --reason TEXT` | `park-goal` | because |
  | `resume G` | `unpark-goal` | empty |
  | `edit G [--intent TEXT] [--next TEXT] [--label L]… [--unlabel L]…` | `edit-goal` | intent, nextStep, and the whole label list the route takes, composed at admission by the command's own owner from the goal's labels as read (`goal.ApplyLabelDelta`, internal/goal/goal.go:459, which also refuses a label named in both) |

  The tool's fields are the flags' own names and spellings, `intent`,
  `next`, `risk`, `basis`, `label`, `unlabel`, `blocked-by`, `blocks`,
  `reason`, `on`, `priority`, `sequence`, beside `verb`, `goal` and
  `explanation`, the Partner's own words for the card, named so as not
  to collide with the commands' `--why` alias of `--reason`. The
  descriptors mark five of those data flags advanced, `label`,
  `unlabel`, `blocked-by`, `blocks` and `sequence` (cmd/metasystem/intent_planning.go:40,
  98, 124, 194), and the catalogue admits them by name all the same:
  advanced in the terminal's help means off the short page, not
  reserved. What is refused is named, not classed: the authority and
  plumbing flags `--by`, `--id`, `--temporary-human-word`, `--review-by`,
  `--origin`, `--under`, `--verified`, `--approved-ref`, the `-file`
  forms, and the forms the interface's route cannot carry, `--next-append`,
  `--risk`, `--basis`, `--tier` and `--evidence` on an edit, `--budget`
  on an approve, `--under` and `--verified` on a resume; each refused in
  words that say where it can be done. A public verb the interface has
  no act for is refused with that command's own usage line from the
  catalogue, "not an act the interface has; at a terminal: metasystem
  abandon G --reason TEXT", so the Partner tells the human where. The
  catalogue is one table in the tool server and a projection of the
  descriptor table, held by a join test in `cmd/metasystem`, which has
  both and reads the descriptors themselves, since `commandCatalogue()`
  projects usage and not visibility: every catalogue verb is a current
  public intent command of that name; every catalogue field is a flag of
  that command that is not hidden, under the descriptor's own spelling,
  or its positional; and no catalogue field is one of the authority
  flags named above. The browser's nine acts are narrower than the same
  nine verbs at a terminal, by design and not by the grammar: an open is
  of origin human, an edit is queued-only, a pause cannot displace
  another seat's claim, a resume only unparks and never resumes a
  budget-stopped goal, and an approve carries the displayed tuple; each
  narrowing is the engine's refusal in words on the line. What the
  message persists and the runner dispatches on is the route id; the
  public name is the tool's boundary, so a later rename touches the
  table and never a persisted proposal.
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
  verb word is the button's word on the page that offers the act:
  Approve, Withdraw approval, Set priority, Open goal, Not now, Return
  to queue, Edit, and the goal page's own words for block and unblock,
  kept in one map from route id to word so that the day the verb system
  lands the card and the buttons take the public words together
  (section 4); the subject is the title with the id small, linked to
  the goal page ("Open the goal"); the arguments in the sheets' own
  labels: because, reason, priority and position, waits for, intent,
  next step, labels, and for an open the tier derived from the four
  answers with the answers named, the basis, labels, waits for and
  unblocks. An approve line shows the
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
  uses, and admits a write only as a compare-and-set on the state the
  caller saw, decided inside that writer (revision 9 and 10, from
  Astra's reads of g1-s60): the body is `{from, state, words}`, `from`
  being the state the line showed when the human pressed; the write is
  admitted only when the persisted state equals `from` and the pair is
  one of: to `applying` from `waiting`, `refused`, `unresolved`, or from
  `applying` as the human's explicit Try again on a line the page shows
  as in flight; to `applied`, `refused` or `unresolved` from `applying`;
  to `dismissed` from `waiting`, `refused`, `unresolved` or `applying`.
  Any other write answers 409 with code `state` and the entry as it
  stands, changing nothing, so a stale tab that still shows `waiting`
  cannot move a line that is really `unresolved` or in flight. The
  runner takes that answer as the line's truth: it reconciles the line
  to the entry returned and never sends an act for it; if the entry is
  settled (`applied`, `refused`, `dismissed`) the run goes on, and if it
  is unsettled (`applying`, `unresolved`) the run stops there as an
  unresolved answer stops it, with Continue with the rest offered. Two
  presses at once, or one in each of two tabs, cannot publish one act
  twice, and an in-flight line left by a page that closed is recovered
  by the human's own Try again or Dismiss after Open the goal. An `applying` write that fails stops the run at that line with
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
- D8. **Where the human looks, and where a proposal waits.** The card is
  in the transcript, in the drawer and in the focused view, because it
  is the answer to what the human just asked and the Partner's reasons
  stand beside it; the drawer's one-shot growth for a card
  (Drawer.tsx:78-91) covers it. A card of any answer but the newest that
  still has a waiting line folds to one line, "The Partner proposed
  earlier · n waiting", which reopens, in the idiom the field proposals
  use for older offers (`PROPOSED_EARLIER`, src/partner/suggesting.ts:475;
  the fold and unfold of a dismissed card, :178-179), so the newest
  answer is what is in view and an old Apply is never pressed by
  accident. While any loaded answer has a waiting line and the drawer is
  closed, its bar says "n actions proposed" beside the composer, counted
  across answers, and pressing it opens the drawer at the newest such
  card, as the sitting's counts open the table. The conversation is the
  wrong place to keep a proposal that waits: it scrolls, and "ignored"
  would become "lost". Waiting proposals get the home every other
  waiting choice has, the Decisions inbox, in step 2 (section 4); the
  transcript keeps the card as the record of the exchange, as it keeps a
  deposit's card while the record holds the entry.
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

Step 1 is D1 to D9 for the nine acts by their route ids, with the
pages' button words on the card; the public grammar waits for the verb
system (revision 8). It is what a human can use after one slice: propose
one act or many, tick, apply, watch the page move, read a refusal in the
engine's words, continue, try again, ask the Partner, sign in when
asked, reload and find the truth.

Later, when it hurts: **first, when the verb system lands**, the public
grammar of D1 (revision 5's table, re-read against the successor
descriptor table: the public names and flags at the tool's boundary,
the join test to that table, and the card's words) together with
g1-s59's button alignment, so the card, the buttons, the terminal and
the Partner take one word in one slice; then the checkout writes as verbs (set a record's
status, settle a question, ask a question, write a record: one catalogue
row, one admission rule against the project reader and one client each);
"Review in the sheet" on a line, opening the verb's own sheet prefilled
so an argument can be edited before applying; **step 2, the waiting
proposals' home**: the Decisions inbox gains one group, "Proposed by the
Partner", composed from the conversation's persisted proposals with a
waiting line, one row per line in the inbox's own idiom (the verb word,
the subject, the arguments, the Partner's explanation as the note; one
open row with Apply, Dismiss, Open the goal and Ask the Partner; the
selection bar for many), the group's count in the header's "needs you",
"n new" by the proposal's time, and the same outcome route recording
what was done, so the card in the transcript and the row in the inbox
read one state and an ignored proposal waits where every other choice
waits until it is applied or dismissed; **step 3**, a chip on the
subject's row on the board and in the queue, "Pause proposed", opening
that row at the proposal; abandon, after its ruling;
the Partner's authorship on the ledger's history line; the owner-side
basis compare on the approve and edit routes, checked inside the
transaction on `ApprovalDigest` (gate 2's review basis); keyboard
operation of the card; proposals across several answers as one list;
bulk Try again; the launch route's row in the act table; the Decisions
bulk sheet taking the same refusal-passed run rule; `confirmed-late`
shown as landed rather than unresolved; the pages' own button words
aligned to the public verbs (Not now to Pause, Return to queue to
Resume, Withdraw to Unapprove, Set priority to Prioritize), one
presentation slice; the fourteen public goal verbs the interface has no
act for, each a route, an act function, a client and a catalogue row,
and where the engine asks a terminal-grade proof of a human (abandon,
done, reopen, accept-risk, grant, revoke among them), a ruling in
R-125's shape first.

## 5. Payload and routes

The tool: `propose` as D1, its result in the fixed frame `Proposal: <route id>`,
a `Goal: ` line, then one labelled line per body field given, `Reason: `,
`Because: `, `Priority: `, `Sequence: `, `Blocker: `, `Intent: `,
`Next step: `, `Labels: `, `Id: `, `Severity: `, `Novelty: `, `Exposure: `,
`Accumulation: `, `Basis: `, `Blocked by: `, `Blocks: `, then the
separator and the explanation. `Message.proposals: [{ index, verb, goal, title, fields: {…},
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
join test that every catalogue verb is a route id in the interface's
act table under `ledgerHand` and every goal act there is in the
catalogue; the host reading a completed `propose` into a whole action
and a failed one into none, the explanation opaque past the separator; the service admitting an
action whose goal is at the tip with `read` filled, admitting a block
whose blocker an earlier `open` of the same answer named, refusing an
`open` of an existing id and an act on an unknown goal with their
reasons, the title carried; the message, the stream and the snapshot
carrying it; the outcome route's policy, its six states, its refusal of
a running turn and of an unknown index, each allowed pair admitted with
a matching `from`, a matching pair with a stale `from` refused with the
entry, two writers racing on one line with only one admitted, the
runner stopping on an unsettled conflict and going on past a settled
one, Try again on an in-flight line sending `from: applying`, and the
message rewritten in place with the rest of the transcript untouched; the context block's
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
its page mounted; an older answer's card with a waiting line folded to
its one line and reopened by pressing it, the newest answer's card
never folded, a card with no waiting line not folded but read as it
stands; the bar's count across answers and its press opening the
newest such card; the guards stay green, with the new call sites rowed
in `cuts.test.ts`. Walkthrough: the fake Partner answers
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
two corrections to code every act shares. Weakest: the public grammar
waits for a verb system that is being restructured, so the Partner
speaks route ids until then and the words a human reads on the line
are the pages' buttons; the map from route id to word is kept in one
place for that day.

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

## Revision 5: the grammar, after the verbs seat landed

Written against the descriptor table at `8a22b6284`, as ruled last:
D1's table of nine public verb statements, their routes and bodies, the
refusals for forms the interface cannot carry, the join test, and D4's
one vocabulary on the card.

## Dispositions (Astra's scoped read of the grammar, 2026-09-26)

Checked at `bf4cb91cf`. All nine names and route bodies confirmed; the
refusals, the join test's placement, D4's vocabulary, the `explanation`
field and the execution layer confirmed, the last with the note that
the browser's nine acts are narrower than the terminal's nine verbs,
now said in D1. One material finding, folded in revision 6:

| id | finding | fold |
|---|---|---|
| S58-13 | D1 admitted `label`, `unlabel`, `blocked-by`, `blocks` and `sequence` while refusing "every advanced flag", and the descriptors mark those five advanced; the tool spelt `blockedBy` where the descriptor says `blocked-by` | the refusal names the authority and plumbing flags and the unsupported forms; the five data flags are admitted by name under the descriptor's spelling; the join test reads the descriptors and asserts non-hidden flags under their own spelling and no authority flag (D1, section 6) |

The two existing owners Astra pointed at are taken: `goal.ParseRiskRecord`
for `risk` and `goal.ApplyLabelDelta` for an edit's labels. The design
is closed, read whole by the critic across its mechanism and its
grammar; the build waits on Wido's go.

Superseded for the build by revision 8 until the verb system lands;
taken up then, against the successor table.

## Revision 7: where a proposal waits

Wido, 2026-09-26: "With your UX cap on, is the location of the
proposals indeed the right place, or would you have these not in the
discussion with the project partner but somewhere in the UI?", then
"make sure the design reflects this insight". The answer folded: the
conversation is where a proposal is made and answered, and the wrong
place to keep one that waits. Step 1 gains one presentation rule in the
idiom that exists, older cards with waiting lines fold to one line and
the bar counts across answers (D8, section 2, section 6); step 2 is the
Decisions inbox group "Proposed by the Partner" over the persisted
proposals, and step 3 the subject row's chip (section 4). No mechanism
changes: the fold reuses the field proposals' own, and the inbox group
reads what D6 already persists. Not sent to Astra: a presentation rule
inside a pattern the critic has read twice, and a later-list entry; the
code read covers the fold.

## Revision 8: the verb system is not finished

Wido, 2026-09-26, late: "The work on the verb system is unfortunately
not finished. There will be complete restructure of the verb system, so
do not yet get into the details of the verbs themselves. Once the work
on the verb system has completed, you can pull that in and then
continue with that." Folded: D1's catalogue is the interface's route
ids and route bodies, with a join test to the interface's own act
table; D4's words are the pages' buttons; the public grammar of
revision 5, with Astra's read and its fold, waits in section 4's later
list beside g1-s59, which is on hold. The builder, mid-build, was told
the same in the same words. Not sent to Astra: it restores the form
revisions 2 to 4 carried, which the critic read in rounds 1 and 2 and
in the confirmation read.

## Revision 9: the outcome route admits transitions

From Astra's round-1 read of g1-s60 (S60-01): the outcome route's
write was unconditional, and its Apply guard local to one caller, so a
second tab whose card or inbox still showed a line waiting could record
`applying` again and publish the act twice; a repeated approve is a
second approval record. Folded into D6 as the transition rule, with
the conflict answer and the runner's reconciliation, and into section
6's tests; the builder, mid-build, was told in the same words. Not sent
to Astra as a round: it is the critic's own finding, folded as stated.

## Revision 10: the write carries the state the caller saw

From Astra's failsafe round on g1-s60 (S60-04, S60-05, S60-06): a pair
table alone let a stale tab move a line that was really unresolved,
told the runner to go on past a conflict whose entry was still in
flight, and left no press that could recover a line abandoned at
`applying`. Folded as one mechanism: the write is a compare-and-set on
`from`, the state the caller saw; a conflict's entry decides whether
the run goes on or stops; Try again and Dismiss from `applying` are the
human's explicit presses with `from: applying`. The builder was told in
the same words. Read by Astra in the scoped confirmation on g1-s60.
