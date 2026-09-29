# g1-s70: the landing gate — small goals land by themselves, bigger ones wait for a person; the review room's slice D

- Kind: design
- Id: 01M3MPB4DNX4C1SCWA4X6BVQW9
- Status: draft
- Goals: browser-interface

Wido, 2026-09-28: "ok, build all". This page is the step-1 cut of
slice D of the review room (g1-s65 D15), under his ruling R-132-ui
(`metasystem/memory/rulings.md`): "goals on the validation lane with a
risk tier below the setting get automatically landed after a <setting>
time. higher level tier needs a human to either do a sitting, or
decide to not do a sitting and then move the goal into landing."
Author Fable. Every cite re-read at `12611b98c`. Depends on slice C
(g1-s69) for the verdict line it reads.

## 1. What exists and binds

1. **The Review lane** is a claimed goal whose file carries a Landing
   record, `{At, Opid}` (`internal/backlog/project.go:215`,
   `internal/goal/file.go:363`). The seat holding the claim lands it
   with `work land G` (`internal/landing/advance.go:31`); nothing today
   asks whether a person looked.
2. **The verdict line** (slice C, D1): a human-only `goal review` act
   writes `reviewed verdict=clear-to-land tip=… record=… by=…` on the
   goal's history through the ledger, so every seat reads it.
3. **Settings** are read from `metasystem.conf` by key with a default
   (`internal/config/conf.go:39`, `ConfValue`), and `.local` overrides
   the committed file. The Settings page shows facts from the workspace
   resource (`web/_app/src/panes/Settings.tsx:40`).
4. **The Decisions inbox** lists needs by kind (approval, renewal, ask,
   question, parked, stopped, draft, landed, ruling review, alert;
   `internal/ui/decisions/decisions.go:73-82`), each a `Need` with what
   is asked, by whom and since when.
5. **The paper**: silence is never a decision where a person's judgment
   is required; proportion says most goals do not need one.

## 2. What you want from the gate

- **G1. Small things land without me.** Below the tier, a goal that
  has sat in Review for the grace time lands by itself, and the history
  says under which setting.
- **G2. Big things wait for my word.** At or above the tier, nothing
  lands until I have held a sitting that ended clear to land at this
  tip, or said "land it without a sitting" with a reason.
- **G3. I can see the clock.** On the card: "lands by itself in 3h 12m",
  or "waits for your review"; and a row in my inbox when it waits on
  me.
- **G4. A standing sitting holds everything.** While I am in the room,
  nothing lands, whatever the tier.
- **G5. A moved tip needs my word again.**

## 3. The moment

A tier-1 goal reaches Review at 14:02. Its card says "eligible to land
in 4h" beside Review it. Nobody opens a room. At 18:02 the card says
"eligible to land, waiting for the holder", and on the holding seat's
next turn it lands; the history line reads "landed under
landing.review.auto-after=4h, tier 1 below human-from-tier=2". A tier-2
goal reaches Review; its card says "waits for your review", with Review
it and Land without a sitting, and your inbox gains "backlog-ordered-by-
priority waits for your review · since 14:02". You open the room, end
clear to land, and the next time the holding seat's loop passes, it
lands, the history naming your verdict and the tip. Another tier-2
goal: you press Land without a sitting, the Decide sheet asks for a
reason, you write "one-line doc fix, read the diff on the card", and
the goal lands the same way, the history carrying your reason. You
start a sitting on a tier-1 goal: its clock stops ("held by your
sitting") until you end it.

## 4. Decisions

- D1. **Two settings.** `landing.review.human-from-tier` (default 2:
  tiers 2 and 3 wait for a person) and `landing.review.auto-after`
  (default 4h), read through the layered resolution every other
  setting uses (`internal/config/resolve.go:172`, `Get`: environment,
  then `.local`, then the committed file, then the default; not
  `ConfValue`, which reads one file and never the overlay,
  `conf.go:39`), so a threshold of 1 set in `.local` over a committed
  2 binds the gate, the clock and the page alike; the Settings page
  shows each with the source the resolution reports.
- D2. **The gate is at the landing's admission, for every form, and the
  hold is a ledger fact.** Landing a goal in the Review lane at or
  above the tier is refused, naming the missing fact, unless the goal's
  history carries, against the current branch tip, either a `reviewed
  verdict=clear-to-land` line or a `landed-without-sitting` decision
  line. A standing review sitting refuses landing at every tier, and
  it is a fact every seat reads: opening a review sitting performs
  `goal review G --hold --record PATH` (a history line, `review-sitting
  opened record=… by=…`) before the room reports the sitting open, Step
  out keeps it, and every way the sitting ends, Clear to land, Send
  back, No verdict and End without recording, performs the release
  (`--release`, or the verdict act of slice C, which releases in the
  same line); a hold that is not released stands until its human
  releases it, and the card says whose it is. The sitting's private
  mark (`partner/review.go:155`, cleared by `conversation.go:1299`) and
  the reviewing checkout's record are not what the gate reads, since
  the landing seat has neither. The gate is enforced where a landing
  admits a governed goal's commits to a batch, so every form that
  publishes a goal's work meets it: `work land G`, `work land j2:J`
  (`intent_delivery.go:1523` resolves the job's goal), the staged
  `--message` form when it names a goal, and the exceptional forms;
  an exception recorded earlier is not a current-tip decision to skip
  the sitting, and `--queue-only` still enters Review. Below the tier
  with no hold, landing proceeds as today. The gate is a rule about
  publishing a governed goal's work, not about the Review lane: a
  claimed goal at or above the tier needs the human's current-tip word
  whether or not it ever carried a Landing record, so a certified
  chain landed directly with `work land j2:J` (the batch binds a
  claimed goal without one, `dispatch/stop.go:77`) meets it the same
  way; `--queue-only` stays the way into Review. And admission is not
  the last word: a batch publishes later and on retries
  (`internal/landing/batch/land.go:306`), and its final authority check
  reads a fresh ledger for the claim, the fences and the budget
  (`landing_batch_prefix.go:47`, at the series boundary,
  `landing_batch_land.go:215`, `:409`), so the gate is evaluated there
  too, against the fresh ledger, at every publication and every retry
  including moved-base recovery: a hold published while the batch was
  proving stops the publication, and a release is judged under the
  grace and permission rules as they stand then. The refusal is a
  register row with the human verb that carries past it (Land without a
  sitting), so the room's own "Ask what happened" can explain it.
- D3. **The clock is the holding seat's, and a human act restarts it.**
  The seat that holds the claim lands a below-tier goal by itself once
  `auto-after` has passed since the clock's start, through the ordinary
  landing, and the history line names the setting and the tier. The
  clock starts at the Landing record's `At` and restarts at every human
  act on the goal's history after it: a hold's release (a sitting that
  ended without a verdict gives the human a fresh grace period, which
  is what "releases the clock" means), a priority change, an edit. It
  does not run while a hold stands. The clock is evaluated where the
  seat already passes over its claims (the resident steward's loop,
  `internal/steward/runner.go:214`, over this machine's claims,
  `goal/project.go:744`), never by a timer of its own; expiry is
  eligibility, and the card says so. The steward's loop never lands a
  goal in its own name: the runner is detached and classified as the
  steward (`runner.go:892`, `lease/classify.go:427`), the holder check
  every landing makes before preparing (`goal_branch.go:614`) admits no
  steward (`lease/verbs.go:440`), and the steward's revival hands work
  to a fresh delegate process, not to a live session (`verdict.go:120`,
  `revive.go:320`, `steward_verbs.go:545`), so there is no route from
  the loop to the holder for this. The execution is the holder's own:
  the session that holds the claim checks its claims' eligibility on
  its next governed turn and on its Stop path, the places it already
  passes over what it holds, and lands an eligible goal with `work land
  G` under its own identity, with the gate of D2 evaluated then against
  the fresh ledger. What the design promises is therefore eligibility
  after the grace time and the landing on the holder's next activity;
  a holder that is idle for hours lands hours late, and the card's
  "lands by itself in 3h 12m" reads "eligible to land in 3h 12m" when
  the wording would otherwise promise a moment. The history says
  "landed" only after the publication is confirmed. The clock never
  runs at or above the tier.
- D4. **Land without a sitting is a human-only act.** `goal land-without
  -sitting G --reason TEXT`, recorded on the history bound to the
  current tip, performed from the card's Decide sheet under the sign-in
  as the other goal acts are; the reason is required.
- D5. **The card and the inbox say it.** Below the tier: "eligible to
  land in 3h 12m" beside Review it, then "eligible to land, waiting for
  the holder", or "held by your sitting"; at or
  above: "waits for your review" with Review it and Land without a
  sitting, and a Decisions inbox need of a new kind, landing, "waits for
  your review", answered by either act. The words are computed from the
  goal file and the settings the project resource already carries; the
  interface never runs the clock.
- D6. **Nothing else changes.** The verdict line (C), the room, the
  Review lane's shape, and the spelling of every `work land` form: the
  forms keep their syntax and gain the one gate of D2 at admission,
  never a bypass.

## 5. Step 1, the smallest thing that works

D1 to D6 in two lanes: the engine's settings, gate and clock (D1 to D3,
D4's verb), and the interface's card words, the Decide sheet and the
inbox kind (D4's press, D5). Not in it: a per-goal override of the
tier; a second reviewer; auto-landing that also runs the candidate's
check first; notifying the human when the clock lands something.

## 6. Payload and routes

`GET /api/project` carries, per Review-lane goal, the gate's reading:
`{waitsForHuman, autoLandsAt?, heldBy?, reviewed?}`, computed
server-side from the goal file's history and the layered settings,
never from the private store. `POST /api/backlog/goals/<id>/land-
without-sitting` `{reason}` performs the act under the sign-in, beside
the other goal acts; the review room's Start performs `goal review
--hold` and its ends perform the release through slice C's act route.
The inbox's needs gain kind `landing`. The Settings page reads the two
keys with their sources from the workspace resource. The proposal
grammar gains `goal land-without-sitting`; the Partner may propose it
with the reason the human wrote. The landing's admission gains the
gate as one function every form calls with the resolved goal and
candidate. The cut guard's call sites gain the one act.

## 7. Not here, later

A tier override per goal; the check run before an auto-landing; a
notification when the clock landed; a second human; the gate for a
design's review.

## 8. Verification and box

Go: the two settings with defaults and sources, a `.local` threshold of
1 over a committed 2 binding the gate; the gate refusing at and above
the tier without the line and proceeding with it, refusing under a
hold at every tier from the history alone, and refusing a line whose
tip is not the current tip; the same gate met by `work land G`, `j2:J`,
the staged form with a goal and an exceptional form, and `--queue-only`
still entering Review; the hold written before the sitting reports
open and released by each of the four ends; the clock landing below
the tier after the grace time from `At`, restarting at a release and
at a human act, never running under a hold and never above the tier,
evaluated on the loop with an injected clock, "landed" written only
after confirmed publication; the act's history line and its required
reason; the register row for the refusal; idempotency rows and public
forms; the gate met at publication: a goal joins a batch, a hold is
published while it proves, the publication and its retry refuse; a
tier-2 claimed goal with a certified chain and no Landing record
landed with `work land j2:J` refuses without the human's word; an
eligible tier-1 goal is landed by its holder on its next governed
turn and on its Stop path with the gate evaluated then, a landing
attempted in the steward's name is refused, and a claim that changed
hands before the holder's next activity lands nothing.
Frontend: the card's four wordings from the project payload; the Decide
sheet's required reason and the act; the inbox row and its two answers;
the Settings page's two facts with sources; the cut guard. Walkthrough:
a below-tier card with its clock, an above-tier card with the two
presses, the inbox row; screenshots at 1280 and 400, light and dark.
Landing checks as g1-s67 §8. Box: Astra's critique (two rounds), two
Opus 5.5 lanes, one Sol read with one fix round; 200 to 320
job-minutes.

## 9. Self-grade

High on D1, D4 and D5: settings, an act, words. High on D2 since
revision 2: the hold is a ledger line like the verdict, so the gate
reads one source every seat has, and one function at admission covers
every form. Medium on D3: a clock on the steward's loop is right by
the cut guard's spirit, its cadence bounds how late an auto-landing can
be, and restarting at a human act is one rule with no second
timestamp. Weakest: the first auto-landing of a goal nobody looked at
is the moment the ruling is tested.

## Dispositions (Astra round 1, 2026-09-28, under R-121 and R-124)

Read of revision 1 at `81acda402`, verbatim in
`g1-s70-astra-critique.md`. Four material findings, all folded; every
cited line re-read at whole-function depth before folding.

| id | finding | fold |
|---|---|---|
| S70-01 | the hold was read from the review record and the sitting's private mark (`partner/review.go:155`, cleared at `conversation.go:1299`; `standingOf` names the lane, `httpd/review.go:197`), which the landing seat never has: a below-tier goal lands while its human sits | D2: the hold is a ledger line, written before the sitting reports open and released by every end; the gate, the clock and the card read the history alone |
| S70-02 | D6 left the other `work land` forms unchanged while the gate sat on the ordinary goal path (`intent_delivery.go:1448`, `j2:J` at `:1523`); `advance.go:31` is checkout rebasing, not a goal's landing | D2 and D6: one gate at the landing's admission of a governed goal's commits, met by every form; syntax unchanged, no bypass; an earlier exception is not a current-tip decision; `--queue-only` still enters Review |
| S70-03 | `ConfValue` reads one file with no `.local` overlay (`conf.go:39`); the layered resolution is `Get` (`resolve.go:172`, the overlay at `:193`): a local threshold of 1 was silently ignored | D1: both keys through `Get`; the page shows the source the resolution reports; a fixture with local 1 over committed 2 |
| S70-04 | "no human act since `At`" never becomes true again after a sitting that ends without a verdict, and `LandingRecord` holds only `At` and `Opid` (`file.go:363`): the clock either stalls for ever or picks an undefined deadline | D3: the clock restarts at every human act on the history after `At`, the release included, and does not run under a hold; one rule, no second timestamp; the card's deadline derives from it |

Astra also verified, and the design leans on, that R-132-ui's boundary
is below versus at or above the threshold and a default of 2 fits it;
that the resident steward's loop passes over this machine's claims
(`runner.go:214`, `goal/project.go:744`) so no timer is needed and
expiry is eligibility on a pass; that `act.SignedIn` validates the
human and the session against the proof (`act.go:190`); and that the
register and the inbox's `Need` are the right extension points, with a
batch join being in-progress until publication is confirmed.

**Round 2, the declared failsafe (2026-09-28, at `04bc1fdd5`):**
S70-01, S70-03 and S70-04 confirmed answered; S70-02 held open
through three new material findings, all folded. S70-05: a gate at
admission does not protect the interval before publication, since a
batch publishes later and on retries with a final authority check that
reads a fresh ledger for the claim, fences and budget but not a hold
(`batch/land.go:306`, `landing_batch_prefix.go:47`,
`landing_batch_land.go:215`, `:409`); fold, D2: the gate is evaluated
again at every publication and retry against the fresh ledger. S70-06:
"in the Review lane" exempted a claimed goal landed directly with
`work land j2:J`, which binds without a Landing record
(`intent_delivery.go:1523`, `dispatch/stop.go:77`); fold, D2: the
threshold applies whenever a governed goal's work is published,
whatever its lane; `--queue-only` stays the entry to Review. S70-07:
the resident steward is detached and classified as the steward
(`runner.go:892`, `classify.go:427`) and the holder check before every
landing admits no steward (`goal_branch.go:614`, `lease/verbs.go:440`),
so the loop could find a goal due but never land it; fold, D3: the loop
only finds the goal due and hands the landing to the claim-holding
session through the steward's continuation, which lands under its own
identity, a changed claim voiding the handoff. Non-material: `Get`
returns no source, so the Settings page's source line is
implementation work. Closed at the failsafe round on three folds,
with one scoped confirmation read on S70-05 to S70-07 alone.

**Confirmation read (2026-09-28, at `a4541e20d`):** S70-05 and S70-06
confirmed answered. S70-07 held open through S70-08: the steward's
"existing continuation" cannot deliver a due landing to a live holder,
since revival is suppressed while a worker is live (`verdict.go:120`),
the one exception needs the seat's own handoff (`revive.go:320`), and
dispatch launches a fresh delegate process rather than reaching a
session (`steward_verbs.go:545`). Folded as Astra's own smallest
honest alternative, D3: the holder checks eligibility on its next
governed turn and on its Stop path and lands under its own identity;
the promise is eligibility after the grace time and the landing on the
holder's next activity, and the card's words say so (D5, §3). Every
cited line re-read. No further read: the fold is the reader's own
alternative and changes no other decision. The loop is CLOSED. One
residual for Wido: unattended landing while the holder is idle for
hours would need a delivery route the steward does not have; "later,
when it hurts".

## Built (2026-09-29)

Step 1 built by Claude on Opus 5.5 in worktree `agentic-tools-gate`,
branch `landing-gate` from main `8d04e59d5`, in six commits: the
engine core (`54a981524`: the two layered settings with their sources,
`goal.Gate` reading the history alone, the hold and release lines, the
clock, `goal land-without-sitting`, the holder's `landed` line), the
gate at every `work land` form and again at the batch's publication and
the hand route's push, with `goal review --hold|--release` and the
register rows (`c1449414f`), the server side (`06fa93100`: the board's
Review rows carrying the gate's reading, the land-without-sitting
route, the hold at Start and the release at the sitting's end), the
interface (`a8e164248`: the card's four wordings, Land without a
sitting with its Decide sheet, the inbox's "Waiting for your review",
the Settings card), the retry-authority bed put below the tier
(`9401147ff`) and the staged form reading the synced ledger where one
exists (`6c4d4bde5`). Report:
`~/LocalStorage/agentic-tools-evidence/review-room-20260928/
opus-build-report-landing-gate.md`; screenshots under `landing-gate/`.

One input was not as the brief said: the hold and release forms of
`goal review` were not on main, so the builder made them here.
Departures, each honest and recorded there: the reading rides
`/api/backlog`, where the cards are, not `/api/project`; the threshold
accepts 4 so that "no tier waits" can be said; the governed turn and
the Stop path are one place, the turn verdict the Stop hook evaluates;
the room holds only where a build wires the gate's acts, which
production and the walkthrough do; the release matches a hold by the
record path's tail, since the ledger and the room name records from
different roots.

**Sol's read** (`g1-s70-sol-read.md`, verbatim): four material
findings, all agreed by the design's author. SOL-S70-01, the staged
and exceptional forms met the gate at admission only, while their
landing driver proves, rebases, pushes and retries; SOL-S70-02, the
holder's Stop was told to land or revise and blocked once, and nothing
took the step; SOL-S70-03, `work land j2:J` had no tip a human word
could bind to, so a certified chain at or above the tier could never
land; SOL-S70-04, a moved branch tip left the card saying cleared and
the inbox empty while the engine refused. **Fix round 1**
(`25a338ad4`, `7a6749471`, `f0d148ace`, `71c911c9e`, `b9514f031`, on
the branch merged with main `49914fec8`): the landing driver is handed
the invocation's own gate and reads it against a fresh ledger
immediately before every push, retries and the carried form included;
the holder's Stop takes its due step itself through the public `work
land G` or `work revise G` under the session's identity, shows what it
did, shows a refusal at every Stop while it stands and never blocks
for a taken step, and a send-back is revised once; the chain form
binds the word to the commit the chain publishes, its candidate
branch's head, carried to the publication gate as the member's tip;
the server reads the newest word against the goal branch's tip at
origin, and a moved tip puts the goal back under "Waiting for your
review" with "cleared by Wido at 9c1f0a2, but the branch moved to
a1b2c3d: needs the word again" and Land without a sitting offered.
One rule changed with the second fix: a held goal that carries the
human's word is due to its holder, so the Stop takes it and shows the
hold's refusal with the release verb, where before a held goal was
never due; below the tier nothing changes, since the clock does not
run under a hold. **Sol's re-read**: SOL-S70-01 fixed; SOL-S70-02 held, the step
taken only on an idle Stop, while a holder with open work is the
ordinary case; SOL-S70-04 held, the interface's tip read from the
checkout's own unfetched ref; SOL-S70-03 held on a chain's word passing
after the goal branch moved beyond the chain's head, which the design's
author rules not material: the gate protects what is published, a
chain publishes its head, and the word at that commit is the word at
this tip for that publication; nothing unreviewed lands. **Fix round
2** (`0f5e05acd`, `84a67f3a6`): the holder's steps are taken on every
Stop, before and independent of the scan's verdict, a Stop with open
work or a busy checkout included; the gate reading fetches the goal
branch from origin for the goals that carry a landing word, once per
reading through the review owner's Git seam, and falls back to the
local ref where the fetch fails. **Sol's re-read 2**: both fixed, no new
finding; one note kept as a residual, the fetch's network timeout can
delay a board reading while origin is unreachable, for the goals that
carry a word only.

Residuals, none material at first use: a chain's word applies only
when the chain's head is the goal branch's tip at origin, since the
word is recorded there, and the chain's head is read at landing time;
on the hand route with no batch root the holder's Stop runs the proof
and the push synchronously, as the public command does; the staged
form is gated only where the synced ledger exists, and no governed
goal can exist without one; the Stop path's identity under the
session's lineage was established by reading, not run live; the
steward-refusal test pins behaviour that predates the slice.

Handed to the engine seat's integration batch as `origin/g1-s70` on
2026-09-29 under the one-batch-per-machine rule of that morning (the
ui lane hands a reviewed branch, the batch runs one VM suite), the
branch merged with main `49914fec8` and the landing checks green at
`84a67f3a6`: every touched package, the structural tests, the static
gate with its dead-code check, the fast build gate, vitest and
typecheck, the bundle rebuilt under Node 24; the whole `cmd/metasystem`
package left to the batch's suite, with
`TestEveryStatefulActionRepeatsAsSuccess/app_stop` named to it, which
failed once under the builder's combined load and passed alone. The
landing commit is the batch's: main `68a7453f4` (batch 18, 2026-09-29
13:15, with g1-s73 and the D14 bridge Part 1; the batch added
`TestJoinGatesFillsOnlyReviewRowsOfAKnownTree` to bring `internal/backlog`
back over its 96.6 floor, which this slice had let slip to 95.6, and
ran the nineteen named tests green on the host).
