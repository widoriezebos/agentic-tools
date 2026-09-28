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
  (default 4h), read by `ConfValue`, shown on the Settings page with
  their source (committed, local or default).
- D2. **The gate is inside `work land`.** Landing a goal in the Review
  lane at or above the tier is refused, naming the missing fact, unless
  the goal's history carries, against the current branch tip, either a
  `reviewed verdict=clear-to-land` line or a `landed-without-sitting`
  decision line; a standing review sitting on the goal (a review record
  whose sitting has not ended, read from the record) refuses landing at
  every tier, naming the sitting. Below the tier with no standing
  sitting, `work land` proceeds as today. The refusal is a register
  row with the human verb that carries past it (Land without a
  sitting), so the room's own "Ask what happened" can explain it.
- D3. **The clock is the holding seat's.** The seat that holds the claim
  lands a below-tier goal by itself once `auto-after` has passed since
  the Landing record's `At` with no human act on the goal in between,
  through the ordinary `work land`, and the history line names the
  setting and the tier. The clock is evaluated where the seat already
  passes over its claims (the steward's loop), never by a timer of its
  own. The clock never runs at or above the tier.
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
  Review lane's shape, `work land`'s other forms.

## 5. Step 1, the smallest thing that works

D1 to D6 in two lanes: the engine's settings, gate and clock (D1 to D3,
D4's verb), and the interface's card words, the Decide sheet and the
inbox kind (D4's press, D5). Not in it: a per-goal override of the
tier; a second reviewer; auto-landing that also runs the candidate's
check first; notifying the human when the clock lands something.

## 6. Payload and routes

`GET /api/project` carries, per Review-lane goal, the gate's reading:
`{waitsForHuman, autoLandsAt?, heldBySitting?, reviewed?}`, computed
server-side from the goal file, the settings and the standing sittings.
`POST /api/goals/<id>/land-without-sitting` `{reason}` performs the
act under the sign-in. The inbox's needs gain kind `landing`. The
Settings page reads the two keys from the workspace resource. The
proposal grammar gains `goal land-without-sitting`; the Partner may
propose it with the reason the human wrote. The cut guard's call sites
gain the one act.

## 7. Not here, later

A tier override per goal; the check run before an auto-landing; a
notification when the clock landed; a second human; the gate for a
design's review.

## 8. Verification and box

Go: the two settings with defaults and sources; `work land` refusing at
and above the tier without the line and proceeding with it, refusing
under a standing sitting at every tier, and refusing a line whose tip
is not the current tip; the clock landing below the tier after the
grace time from `At` and never above it, evaluated on the loop with an
injected clock; the act's history line and its required reason; the
register row for the refusal; idempotency rows and public forms.
Frontend: the card's four wordings from the project payload; the Decide
sheet's required reason and the act; the inbox row and its two answers;
the Settings page's two facts with sources; the cut guard. Walkthrough:
a below-tier card with its clock, an above-tier card with the two
presses, the inbox row; screenshots at 1280 and 400, light and dark.
Landing checks as g1-s67 §8. Box: Astra's critique (two rounds), two
Opus 5.5 lanes, one Sol read with one fix round; 200 to 320
job-minutes.

## 9. Self-grade

High on D1, D4 and D5: settings, an act, words. Medium on D2: the gate
reads the ledger and the record, and the standing-sitting check reads
a file only the reviewing checkout has, so "held by your sitting" is
honest only where the record is visible; Astra should say whether the
hold must also be a ledger line. Medium on D3: a clock on the
steward's loop is right by the cut guard's spirit, and its cadence
bounds how late an auto-landing can be. Weakest: the first auto-landing
of a goal nobody looked at is the moment the ruling is tested.
