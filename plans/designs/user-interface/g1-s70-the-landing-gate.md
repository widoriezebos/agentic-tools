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

A tier-1 goal reaches Review at 14:02. Its card says "lands by itself
in 4h" beside Review it. Nobody opens a room. At 18:02 the seat holding
the claim lands it; the history line reads "landed under
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
  eligibility on the next pass. The loop only finds the goal due; it
  never lands it in its own name, because the runner is detached and
  classified as the steward (`runner.go:892`, `lease/classify.go:427`)
  and the holder check that every landing makes before preparing
  (`goal_branch.go:614`) admits no steward (`lease/verbs.go:440`). The
  due landing is handed to the session that holds the claim through
  the steward's existing continuation, the same way its other work
  reaches a holder: the holder runs `work land G` under its own
  identity, with the gate of D2 evaluated then; a claim that changed
  hands or was released between the finding and the landing
  invalidates the handoff, and nothing is inferred from the machine's
  identity or the runner's remembered lineage. The history says
  "landed" only after the publication is confirmed. The clock never
  runs at or above the tier.
- D4. **Land without a sitting is a human-only act.** `goal land-without
  -sitting G --reason TEXT`, recorded on the history bound to the
  current tip, performed from the card's Decide sheet under the sign-in
  as the other goal acts are; the reason is required.
- D5. **The card and the inbox say it.** Below the tier: "lands by
  itself in 3h 12m" beside Review it, or "held by your sitting"; at or
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
landed with `work land j2:J` refuses without the human's word; a due
tier-1 landing found by a detached steward with a distinct live holder
is landed by the holder and refused in the steward's name, and a claim
that changed hands in between voids the handoff.
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
with one scoped confirmation read on S70-05 to S70-07 alone (recorded
below when it returns).
