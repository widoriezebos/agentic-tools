# The Operator Surface — the claw-back design (2026-08-28)

Working Mode: design

Owner: m1 coordinator, ratified in direct discussion with Wido on
the morning of 2026-08-28, immediately after the protective-
mechanism breakdown. This document is the WHOLE design: what
changes, and for every change, the reason — each reason traceable
to a concrete failure of the preceding 48 hours or to a written law
of this repository that was being violated. Nothing here is
invented; it is the recorded philosophy, enforced.

The breakdown this answers (the evidence base):
- The steward runner had been dead since ~2026-08-20 and the
  narrator since 2026-08-23; nobody and nothing noticed until the
  human did. The coordinator FOUND the corpse during a fact sweep
  and recorded it as a design input instead of treating it as an
  outage.
- The coordinator's own goal-note edits destroyed the parseable
  "Appetite:" prefix on the claimed goal, silently disarming the
  breach banners for an entire overnight build (~16 delegate
  passes, ~10 hours, zero alerts).
- One dispatch required knowing an arming sequence, a lease model,
  a census contract, and a driver script's hidden prerequisites;
  the coordinator — the system's daily operator and co-author —
  failed the sequence three times in one night.
- The overnight build itself violated the slicing law: one
  monolithic unlanded diff instead of independently deployable
  slices.
- Work was preserved but not lost: branch wip/custody-launch-machine
  (3fec78a). Wido's ruling: that work MUST integrate into this
  design when its turn comes — never discarded, never landed as-is.

The two failures underneath all five: meaning lived in PROSE that
the machine could not read and the operator could silently break;
and protection lived in CONVENTIONS the operator had to remember,
with no mechanism watching the mechanisms.

## The two laws of this design (Wido's words, made binding)

1. NOTHING IS PROSE. Every value the system acts on is a typed
   field owned by the Go engine. Human prose may exist for humans —
   position notes, commit messages — but it carries ZERO machine
   semantics. Why: the appetite alarm died because its trigger was
   a prose convention inside a free-text field; a structured field
   cannot be disarmed by a note.
2. ALL COMPLEXITY SHIELDED BEHIND THE MINIMAL SET OF EASY VERBS.
   The operator surface is six verbs; everything else is internal.
   Why: the operator provably cannot hold the current surface — the
   lease/arming/census/driver failures were not carelessness, they
   were the predictable result of a surface larger than working
   memory. A system its own author cannot operate intuitively is
   broken regardless of how correct its internals are.

## The Verb Contract (binds every verb, enforced once in the flag layer)

- MANDATORY-IF-IMPORTANT: a field whose absence would silently
  change behavior is required; the verb refuses, naming the field,
  with a one-line example. Optional exists only for conveniences
  with harmless defaults. Why: trial-and-error flag discovery is
  how the coordinator mis-ran the system all week; and the
  strictness law already says refusals guard invariants loudly —
  a silently-defaulted important field is an invariant violation
  waiting to happen.
- IDEMPOTENT: every verb is safe to re-run, always. `up` on an
  armed system reports "already armed", exit 0. `delegate` with
  the same operation binds to the standing job. `land` after
  success reports "already landed". A crash mid-verb leaves a
  state the SAME verb completes on re-run. Why: the companion
  double-fire, the arming join-a-corpse wedge, and the fixture
  whack-a-mole were all non-idempotent seams; the goal-transaction
  machinery already proved the cure (the operation id is the
  truth, not the belief) — this generalizes it.
- FORGIVING: no verb punishes benign variation; every refusal
  names the violated invariant AND the remedy verb; a failed verb
  cleans up what it started. Why: "strictness guards invariants,
  never conveniences" is already written law; and the operator
  inherited cleanup (leaked holds, stale locks, stranded payloads)
  all week — cleanup is the verb's job.
- SELF-VERIFYING: a verb probes its own effect before reporting
  success — "armed" means the armed thing answered a probe.
  Why: completion-claims-require-execution is standing doctrine
  for the coordinator; the verbs must obey the same law.
- SELF-EVIDENT: `--help` is a two-line contract plus one example.
  Why: the current help surfaces are walls of lifecycle text the
  operator demonstrably does not absorb.

## The six verbs (the WHOLE operator surface)

Chosen by replaying the operator's actual last 48 hours — every
verb earns its place by a concrete pain, and two candidate verbs
were REMOVED for the same reason:

1. `metasystem up`
   Arms everything, idempotently: supervision owner, watcher,
   steward runner, session announcement, lease. Runs automatically
   at session start (the hook calls it; a session can never again
   operate unarmed without knowing).
   WHY: acquiring the lease lawfully cost the operator ~40 minutes
   and four attempts, including a join-against-a-dead-owner wedge
   whose error message blamed the census. The arming ORDER was
   knowledge; now it is code. Replaces the operator-facing use of
   arm-supervision.sh, steward arm, and every lease incantation.

2. `metasystem health`
   The roles-alive verdict, computed in Go, typed per role:
   alive | dead | unknown, each with a one-clause reason and — for
   dead — the remedy AS AN EXECUTABLE VERB, never a sentence.
   Roles: steward runner; supervision owner; repo watcher; census
   freshness; narrator freshness; session main announced+alive;
   every claimed goal's appetite present (structured — see below);
   non-terminal job records whose recorded process is provably
   dead; capability snapshot ages.
   EVERY FAILURE CLASS HAS AN OWNER (the paper: every deadline has
   an owner authorized to decide what follows): steward/narrator
   dead → the tick auto-restarts (its own lawful authority);
   supervision owner, watcher, stale arming → the coordinator runs
   the named verb (`up`); anything the coordinator cannot restore
   within one turn, and every repeated failure of the same role →
   Wido, by alert. The verdict names the owner beside the remedy.
   WHY: five days of dead watchdogs, discovered by accident. The
   checks are exactly the failure inventory of the breakdown —
   nothing speculative is checked.

3. `metasystem goal …` (existing family, two changes)
   a. APPETITE BECOMES A STRUCTURED FIELD: `goal set-appetite
      --id X --appetite 4h`, stored beside State; claim requires
      it; breach is computed by the engine from claim age against
      the field. The next-step returns to pure human prose with no
      machine meaning.
      WHY: the alarm was disarmed by a note. A field cannot be.
      Claim-requires-appetite also enforces Wido's 4-hour slice
      ceiling mechanically (set-appetite refuses tokens above 4h
      for non-human actors; the human may override with --by).
      RULE RECORDS (every new refusal is a governed rule, per the
      paper's learning-systems chapter — never a veto by reflex):
      * THE 4H CEILING — stops: agent-sized slices above four
        hours. Must remain possible: human-sized exceptions via
        --by wido (the override IS the appeal route). Owner: Wido.
        Review-by: 30 days or the first overridden refusal.
        Side-effect signal: the Friction rate metric already
        counts refusals — an overridden refusal is evidence of
        miscalibration, watched by machinery that landed
        yesterday.
      * CLAIM-REQUIRES-APPETITE — stops: unsized claims. Must
        remain possible: claiming immediately after set-appetite
        in one breath (the verbs compose). Owner, review, appeal,
        signal: as above.
      * MANDATORY-IF-IMPORTANT FIELDS — stops: silently defaulted
        behavior-changing inputs. Must remain possible: quick
        no-goal probes via an EXPLICIT recorded value
        (--goal none), never by omission. Owner, review, appeal,
        signal: as above.
   b. Nothing else changes. WHY: the goal family was the one
      surface that worked intuitively all week.

4. `metasystem delegate --role <role> --brief <file> --goal <id>`
   The ONE launch verb for every delegated role — build, design
   critique, code critique, investigation. Internally: record with
   proven identity, caps, execution guard, launch, liveness
   registration, and the return contract. `--goal` is mandatory
   (provenance is not optional — the metrics arc proved attribution
   cannot be reconstructed later). Critique rounds are delegations;
   the standalone critique driver dies.
   WHY: the operator hand-rolled sixteen codex launches overnight
   with homemade zombie monitors because the lawful path had
   un-holdable prerequisites. This was the single largest
   operational pain of the entire period. INTEGRATION MANDATE
   (Wido): the wip/custody-launch-machine branch supplies the
   internals — platform-exact identity, the idempotent claim
   (collapsed from nine outcomes to three the operator can think
   in: started | already-running | refused-because), the liveness
   triad, record custody. A keep/delete triage of that branch is
   presented to Wido before this verb is built; the expected
   default for machinery that exists only to satisfy a critique
   finding is deletion, but the identity and custody core is
   exactly what makes this verb honest.

5. `metasystem watch --job <id>`
   Follow a delegate to its terminal state; on failure, SHOW the
   evidence: the record, the log tail, the remedy. Zombie detection
   (work-product freshness + process probe + timed verdict) is the
   tick's job, not the watcher's — watch just displays truthfully.
   WHY: the operator spelunked suite-failure directories by hand
   all night; the digging becomes the verb's output.

6. `metasystem land --message <file> --goal <id> --built-by <who>`
   The landing as a verb: gate, commit, push both remotes, AND the
   receipt row written atomically with the landing. `--goal` and
   `--built-by` are mandatory.
   WHY: the receipt was a separate post-landing ritual with its own
   flags; separate rituals get skipped under pressure (and were).
   Provenance becomes impossible to omit. The receipt verb
   disappears from the operator surface.

REMOVED from the earlier draft, with reasons — and per the
archaeology standard, each removal names the observation that
would REOPEN it:
- A separate `receipt` verb — absorbed into `land`. Reopens if a
  landing is ever observed without its receipt row.
- A `why`/diagnose verb — `watch` owns failure evidence. Reopens
  if the coordinator is ever again observed hand-reading
  suite-failure directories because watch's output did not carry
  the evidence.
- (And for the internalized surfaces: the arming scripts reopen as
  operator surface if `up` ever fails in a way only the raw
  scripts can repair; the critique driver reopens if a critique
  round ever cannot run as a delegation.)

## The heartbeat and the alert path (Wido's ruling, verbatim intent)

- THE HEARTBEAT IS PRIMARILY FOR THE COORDINATOR AND IT LIVES IN
  THE CHAT: the turn-verdict display (the stop-hook surface that
  already reaches every turn) carries one watchdog line, every
  turn: "watchdog HH:MM: all 9 roles alive | goal X 2.1h/4h" or
  the named failure with its remedy verb. The steward tick
  computes it every interval; the turn display makes it
  unmissable.
  WHY: the coordinator is the one who ran blind. A heartbeat the
  operator must go look for is a convention; a heartbeat in every
  turn's display is a mechanism. The human reads the same chat, so
  visibility is shared for free.
  HABITUATION IS THE HEARTBEAT'S OWN FAILURE MODE and it gets a
  named test, not a hope: every retro asks one question — did the
  operator act on the last unhealthy heartbeat within one turn?
  Two consecutive retros showing skimming force a channel
  redesign. (The paper's discriminating-test standard applied to
  the signal's CONSUMER, which is where the original failure
  lived.)
- ALERTS ESCALATE TO THE HUMAN: an unhealthy verdict (a) fires the
  existing platform notifier (macOS desktop banner today), and
  (b) the session pushes to the human's iPhone when it observes
  unhealthy in its turn display. The notify-command seam stays the
  single extension point for telegram/slack later.
  WHY: interval-noise on the human's devices is wrong (his
  ruling); silence-when-healthy plus loud-when-not is right — and
  the chat heartbeat means silence can no longer hide a dead
  watchdog, because the LINE ITSELF disappearing is the signal.
- WHO WATCHES THE WATCHER: the supervision watcher and the steward
  runner check each other each interval and re-arm each other via
  `up`'s internals; a single corpse cannot go unnoticed because
  its peer names it in the next heartbeat.
  WHY: the steward died in silence for five days precisely because
  it was the only thing that would have reported its own death.

## What happens to everything else

- arm-supervision.sh, the lease verbs, critique-round.sh, and
  dispatch.sh's operator surface become INTERNALS of `up` and
  `delegate` (scripts may remain as plumbing the verbs call, per
  the core-vs-plumbing boundary — decisions in Go, invocation in
  shell). The operator never types them again.
- The wip/custody-launch-machine branch is the PARTS INVENTORY for
  `delegate` and `health` internals (Wido: the invested tokens
  must not be worthless — and they are not: identity exactness,
  idempotent claims, custody records, and the liveness triad are
  precisely the hard parts of the two biggest verbs). The triage
  (keep / adapt / delete, with reasons) is its own reviewed
  document before verb four is built.
- The one-page operator contract (the daily loop, the six verbs,
  the three laws: 4h slices; breaches raise and the coordinator
  switches items; land small) lands in docs/ as the FIRST page of
  the ways of working; everything longer becomes reference.

## Build order (Wido's law: what protects against the biggest fuckup, first)

Each slice ≤4h hard, landed alone. THE ACCEPTANCE GATE
(self-application law): every slice is judged by the CURRENT
controls — the existing landing gate and battery are the judge of
the new machinery's adoption, never the new machinery itself; one
independent code-critique chain at full rigor attacks the slice;
and the slice lands only on Wido's explicit acceptance in the
chat. Where an old and a new protection overlap, BOTH run together
until the new one has a week of live evidence (the prose-appetite
banner keeps running beside the structured field; the old arming
path stays callable beside `up`) — authority transfers as evidence
earns it, never at cutover. A slice that exhausts its 4h budget
STOPS and records in the goal note what works, what does not, and
the precise remaining uncertainty, then raises — an honest stopped
slice is a first-class outcome, not a failure to hide.

The slices:

1. SLICE ONE — `health` + the chat heartbeat + the alert path.
   Protects against: running blind (the biggest fuckup). Includes
   the tick integration, the turn-display line, desktop alert,
   session iPhone push on unhealthy, and failing-first fixtures
   that kill each role and prove detection at the next tick.
2. SLICE TWO — the structured appetite field + claim-requires-
   appetite + the 4h ceiling. Protects against: the disarmed
   alarm, permanently.
3. SLICE THREE — `up` (idempotent arming incl. dead-owner
   takeover, absorbing the arming-dead-owner-takeover backlog
   item) + session-start auto-up + mutual watching.
4. SLICE FOUR — the wip-branch triage document (keep/adapt/delete
   with reasons), reviewed by Wido.
5. SLICES FIVE+ — `delegate`, `watch`, `land` on the triaged
   internals, one verb per slice.

## What this design deliberately does NOT do

- No new state machines, registries, or locks beyond what the wip
  triage justifies piece by piece. Why: the overnight loop proved
  mechanism-answering-critique climbs its own ladder.
- No redesign of the goal ledger, the census, or the reapers. They
  worked. Why: the claw-back touches only what failed the
  operator.
- No multi-round prose DESIGN loops for these slices: this
  document is the design, and fixtures arbitrate. But CODE
  critique runs at FULL rigor for every slice — these slices
  change the system's safeguards, and the paper's
  self-application chapter is explicit that safeguard changes
  deserve more independent examination, not less (the earlier
  draft cut critique down out of token-guilt; that was the
  faster-release-rule builder's argument and it is withdrawn).
