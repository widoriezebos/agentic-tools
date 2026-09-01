# The watch verb: one surface, heal-first action — design

- Goal: plans/goals/watch-verb.md (revision 5, claimed by m0)
- Mode: design only; nothing here is implementation
- Author: Fable design lane, job watch-verb-design-r1, 2026-09-01
- Status: DRAFT awaiting design critique (R-25 lanes: Sol critiques)

Wido's order, verbatim from the goal: "I hope you will do more than
alerting if that happens: the system should act! Figure out what went
wrong and act accordingly (recover process if it died, at actual
budget overrun escalate etc etc)".

This design ends the m1 seat's notification-only steward-watch relay.
plans/seat-governance-record.md, "Interim delegations": "Steward-watch
relay: until the watch verb lands (program L14), the seat relays
steward observations and incidents to Wido. Notification only — the
relay holds no custody and kills nothing. The watch verb's landing
ends this delegation without a further ruling." Because the landing
ends a bounded delegation and creates machine action power in its
place, every authority this design grants is enumerated and bounded in
section 6, and none of it is live before the marking-mode trial in
section 7.

## 0. The governing law: Ruling L, located

The authoritative text of Ruling L is in the tree at
plans/operator-surface-design.md:448-465, "RULING L — THE ESCALATION
LAW (Wido, 2026-08-28 afternoon)". Verbatim core:

> "Escalate only to me if you cannot recover yourself. I don't want to
> be bothered by things the machinery should take care of in a safe
> and reliable way." Binding on every alert path: the machinery heals
> first — restarts, re-arms, retries within its lawful authority — and
> the human hears about it only in the heartbeat line's history, never
> as an alert. An alert reaches Wido's desk or phone ONLY when:
> auto-heal has failed (the five-observation breaker ends healing),
> the failure class has no lawful automatic remedy (a human-reserved
> judgment, an unrecoverable state, a safety-uncertain act), or the
> same role keeps cycling through heal-and-fail (flapping is a failure
> of healing, not a success).

Ruling M (same file, lines 468-481) binds the scope: every delegated
job is a tracked job the steward judges by evidence, and when the
judgment is stall, the machinery acts within its lawful authority
before anything reaches the human. Both rulings are cited by the goal
and both are consumed here as law, not re-derived.

## 1. Traced substrate (facts this design consumes, all landed)

Detection — already landed, none of it rebuilt:

- The claimed-goal-delivery health role
  (internal/steward/delivery.go): per claimed goal on this machine, a
  fail-safe verdict from durable evidence — "failed without recovery:
  job X ended N ago with error E" (newest failed job after the claim
  with no newer landing receipt, delivery.go:105-108, 279-287) and
  "burned without delivery" for a job past its own capMin plus half
  (per-slice overrun, delivery.go:111-123) or a claim past 150% of a
  sub-norm elapsed limit (delivery.go:127-134). Every unreadable
  record is a dead verdict, never a skip. The role is explicitly
  marked NoAutomaticRemedy today (delivery.go:297-301) — the acting
  side of this design is exactly the machinery that clause is waiting
  for.
- The health-role pipeline (internal/steward/health.go): fourteen
  typed roles, alive/dead/unknown per role, observation state with
  failure counts and failure episodes, one aggregate line and exit
  code (0 healthy, 1 unhealthy, 2 unknown-present).
- Alert episodes (internal/steward/alert_episode.go): open on first
  failure, notify only at Ruling L's escalation conditions, close as
  history when the health verdict itself heals. The episode store is
  the one durable escalation truth.
- The notify command (internal/steward/notify.go): delivery-gated
  operator notification; `EnsureRunner` refuses to arm a steward
  without a configured channel (runner.go:221), so an armed steward
  implies a live escalation path.
- The steward verdict ladder and revive path
  (internal/steward/verdict.go, revive.go, intervene.go): per-tick
  verdicts no-work / healthy / stalled-idle / stalled-dead / unknown /
  degraded; death is a proof, never an absence; unknown dominates
  dead; a live worker is never displaced. ActRevive dispatches a
  steward-continuation job through a one-shot intent record minted
  before anything happens and consumed exactly once at launch, fence-
  checked, crash-reconcilable, receipted. The runner's `--max-
  revivals` flag (default 3) already bounds revivals before
  notify-only.
- The per-job watcher (scripts/watch-background-jobs.sh and the
  supervision census): DONE / STALE / CAPPED / VANISHED /
  NEVER-STARTED per job record, census verdict in
  artifacts/agents/supervision/last-census.json, reaper-owned
  absolute caps and lawful process-group wind-down.
- The breach-stop route (cmd/metasystem/dispatch_verbs.go:1003,
  `job breach-stop-routes` listing routes via
  `dispatchcore.FindBreachStops`; `job breach-stop` closing a
  breached revision's fence via `dispatchcore.EnsureBreachStop` and
  initializing its stop batch; stop-batch reconcile/pending/cancel
  verbs). Ruling Q names the steward tick's stop custodian as the
  wind-down owner. This is landed law, not new authority.
- `job watch --job <id>` (main.go:155): block until one delegate job
  is terminal — the settled single-job follow shape from
  plans/operator-surface-design.md verb 5, kept as-is.
- Open-work and running-work reporting (internal/steward/openwork.go,
  `report open-work`, `report running-work`, `report scan-jobs`).

Response recipes proven by hand this week, mechanized here rather
than invented:

- Budget-death adoption: a died delegate round with an intact
  worktree product gets a verification-adoption round. Goal
  budget-death-on-return carries the root cause (the claude adapter's
  $5 native default) and the recorded recipe ("treat cap-death with
  an intact worktree as completed-with-recovery, not runtime_error");
  the goal's own text records the specimen set (six per
  plans/goals/watch-verb.md; budget-death-on-return enumerates the
  three ledger-attention specimens and the five two-bars cap-deaths
  it diagnosed).
- Actual overrun: the breach-stop route above (specimen: the
  alert-escalation-channel breach-stop of 2026-09-01T04:16Z, resumed
  only by Wido's relayed word under R-32-m1).
- Wedged supervision: lawful owner shutdown plus re-arm; goal
  vm-epoch-identity-drift records the sequence ("m0's workaround was
  a lawful owner shutdown plus re-arm") and the drift signature
  (recorded epoch exactly +1s over every fresh probe while
  pidStartTicks and bootId are identical).

Authority laws consumed: R-1 (two-sided conflict test), R-21-m1 /
R-22-m1 (the battery: authoring, operating, and judging one's own
mechanism is the punished combination; attempts spend cause-blind),
Law 2's shape (obligation states DRAFT/OBSERVE record would-refuse
and cannot refuse; LIMITED/ENFORCED require a complete recorded human
authorization — internal/governance, cited by the seat record), goal
gate-governance-records (governance record with owner, review date,
known-bad fixture, appeal route, broken-check response; refusal/action
power only after a marking-mode trial with recorded false-alarm
bounds), R-11 (simplicity), R-24 (self-grade), Ruling Q (grace band),
R-35-m3 (slow is not stalled — progress-based patience), R-37-m3
(standing re-arm order for gate-landed engines, due 2026-09-30).

Fleet seams: alert-escalation-channel (in flight on m0b) owns outward
delivery — this design produces alert episodes and typed escalation
classes and consumes that channel BY CONTRACT (its outbound-only
contract, plans/alert-channel-design.md section 2, with the
delegate-job-failed and stop-awaiting-resume classes 11a.8/11a.9);
this design never builds or touches delivery transports. Until the
channel lands, the landed notify-command is the delivery path.
ledger-attention (m2) is disjoint: its health role
(RoleLedgerAttention) is one more verdict the read surface displays,
nothing more.

Citation honesty note for the critic: the goal names a
"harness-silent-death law" for the read side's verdict shape. No
document in the tree carries that law under that name; the goal
itself inlines its operational content — work-product mtimes + probe
+ timed verdict — and that inline content is what section 3 binds to.
This is a naming note, not a contradiction.

## 2. Shape in one paragraph

One verb, two natures. `metasystem watch` is a ZERO-WRITE projection:
it computes nothing new and mutates nothing — it joins the verdicts
the landed producers already persist (job records, census, health
record, delivery verdicts, alert episodes, steward intents, breach
routes) into one typed surface, so reading it is not an authority and
needs no trial. The ACTING side does not live in the interactive
command at all: it lives in the steward tick, which already holds the
one lawful automatic-response path (one-shot intents, fence checks,
receipts) and is already watched by the health pipeline and the
supervision rings. The tick's decision ladder is extended from
{none, notify, revive} to the typed response-class table in section
4, each class bounded by section 6 and armed only through the section
7 trial. Every action is recorded in stores the read surface already
shows, so the surface displays not only what is running but what the
machinery did about stalls — Ruling L's "history, visible when he
looks."

## 3. The read surface (slice 1)

### 3.1 Command and output

`metasystem watch` — no arguments shows the whole surface. `--json`
emits one machine-readable snapshot (typed enums only, never
because-strings, per the R1-06 nothing-is-prose law); default output
is the human table projection of the same snapshot. `metasystem watch
--job <id>` keeps the landed follow-one-job behavior (block until
terminal, zombie verdict on a dead process behind a non-terminal
record, exit codes for terminal-ok / terminal-failed / timeout /
zombie-suspected) exactly as settled in operator-surface-design verb
5 — this design does not reopen it.

Exit codes mirror the health totality law: 0 nothing tracked is
stalled or unknown; 1 at least one stalled/dead item; 2 at least one
unknown item and nothing dead. Every state is total: each tracked
item always renders a verdict, and an unreadable store renders as its
own named degraded line (fail-safe, matching delivery.go's every-
unreadable-record-is-dead direction), never a silent omission.

### 3.2 What counts as tracked (the closed enumeration)

An item is TRACKED if and only if it appears in one of these landed
stores. The verb enumerates the stores; it never discovers work by
inference.

1. Delegate job chains: every non-terminal record under
   artifacts/agents/jobs/, plus every terminal-failed record newer
   than its goal's claim that no newer landing receipt has recovered
   (the exact join delivery.go already computes).
2. Claimed goals of this machine with their budget tuples and
   delivery verdicts (the goal projection plus the claimed-goal-
   delivery and claimed-goal-appetite roles).
3. Steward intents in flight (artifacts/agents/steward/intents/) and
   their outcomes, including would-act records from section 7.
4. Supervision components: owner, watcher, reaper, steward runner —
   their health-role verdicts.
5. Mission runners and monitored runs (the census custody categories
   and `report running-work`'s sources).
6. Open alert episodes and pending notifications.
7. The last census verdict, including UNTRACKED and ANNOUNCED
   processes. Untracked processes are DISPLAYED and never acted on:
   the census judgment on them is human-reserved (`proc acknowledge`
   stays a human-authority verb per the operator-surface removal
   ledger), and their presence blocks death proofs exactly as the
   verdict ladder already says.
8. The never-idle line: when `report open-work` shows claimable open
   work and no tracked worker is live, the surface says so
   explicitly (the stalled-idle verdict) — folding the standing
   never-idle order into the same surface instead of a separate
   check.

### 3.3 The liveness verdict per item

Per the goal's inline law, every tracked item carries three facts and
one verdict:

- work-product mtime: the newest of the item's own advancing
  artifacts (job record mtime, transcript sidecar mtime, worktree
  newest-file mtime for jobs; ledger/receipt stamps for goals; state
  and census stamps for components);
- probe: the clock-step-immune process identity result where a pid is
  recorded (live / dead / unknown / unprovable), never inferred from
  output silence;
- timed verdict: the store's own typed verdict with its age.

The verdict VOCABULARIES ARE THE LANDED ONES, deliberately not
unified: jobs speak the watcher's DONE / STALE / CAPPED / VANISHED /
NEVER-STARTED (plus the zombie verdict), goals speak the delivery
role's alive/dead reasons, components speak alive/dead/unknown, the
repository speaks the steward ladder. Minting one seventh vocabulary
and mapping six landed ones into it was considered and rejected
(section 9, alternative D): the mapping table would itself become an
adjudication surface, and R-11 says the simplest structure that
answers "what is running and is it alive" wins. The JSON snapshot
carries `{store, verdict, evidence}` triples so a consumer always
knows which vocabulary it is reading.

Progress-based patience binds the read side too (R-35-m3): slow is
rendered as live-with-age, never as stalled; only the producers'
own stall verdicts (which are progress-based, not wall-clock-based
except where a human-set cap says otherwise) say "stalled".

## 4. The stall taxonomy and its response classes (slice 2)

The classifier is a total function from TYPED evidence to exactly one
class. "Died" means mechanically: the job record is terminal without
an adjudicated return — terminal cause in the typed set
{process-lost, watchdog/reaper cap kill, native-budget-cap,
infrastructure(capacity|auth|network) as typed by the adapter's
result record}. A run that RETURNED — even a malformed or red return
— is not dead and is NEVER this machinery's business: its disposition
belongs to the dispatching seat's ordinary review chain. That single
definition is what keeps the watcher out of adjudication (section 5).

| Class | Typed trigger (all facts from durable records) | Response | Bound |
|---|---|---|---|
| W-IDLE idle seat | Steward ladder stalled-idle / open owned work with no live worker | The LANDED revive path unchanged: steward-continuation dispatch via one-shot intent | Landed max-revivals (3) per the runner; already armed law, listed for completeness — no new authority |
| W-ADOPT died with product | Job died (definition above) AND its worktree exists AND its base-to-worktree diff is non-empty | Dispatch ONE verification-adoption round: a fresh, governed dispatch on the same goal whose brief instructs verify-the-intact-product-and-return (the budget-death recipe, mechanized) | Once per operation identity, ever; spends one attempt cause-blind through governed admission; the adoption round dying → W-ESC |
| W-REDISPATCH died without product | Job died AND worktree absent or diff empty | Dispatch ONE re-dispatch of the same brief through governed admission, spending one attempt honestly (R-22: cause-blind). The goal's infrastructure addendum is the same row with its cause recorded: an adapter-typed infrastructure cause (capacity, auth, network — never inferred from silence) authorizes the one retry, recorded as infra-retry | Once per operation identity, ever; a second death of the same operation — infrastructure or not — escalates; admission refusal (any fence) → W-ESC with the breach evidence |
| W-BREACH actual budget overrun | Delivery verdict burned-without-delivery, or elapsed past limit + Ruling Q grace band — i.e., exactly what FindBreachStops lists | EnsureBreachStop closes the fence; the stop batch winds live jobs down through the lawful stop path; then MANDATORY escalation: resume is a human act (R-32-m1), episode class stop-awaiting-resume | Already-landed mechanical law (Ruling Q's named owners); this design only makes the tick run the routes it can already list; no healing is attempted past a closed fence |
| W-HEAL wedged infrastructure | A supervision health role dead (steward-runner, supervision-owner, repo-watcher, census-freshness), or CENSUS-FAILED carrying the recorded identity-drift signature (epoch +1s, identical startTicks and bootId — vm-epoch-identity-drift) | The named heal, exactly the recorded sequence: lawful owner shutdown + re-arm via `metasystem up` with the ENROLLED engine, under R-37-m3's standing word (gate-landed engines only, consumed commit recorded) | Episode failure-count ceiling (the landed five-observation-shape breaker); a repeat wedge of the same role is flapping → W-ESC per Ruling L; R-37 expires 2026-09-30 — after expiry this class reverts to escalate-only until renewed (the governance record carries this dependency) |
| W-ESC unknown / everything else | Verdict unknown or degraded; untracked processes blocking a death proof; unreadable evidence; a terminal cause outside the typed set; any breaker or fence reached above | Escalate, never guess: open or advance the alert episode; deliver per Ruling L's conditions through the notify seam / escalation channel. NO dispatch, NO kill, NO probe beyond the read side's | Unknown is held one interval then escalates (the health totality law); never kill on silence stands absolute |

Ordering within one tick: W-BREACH and W-HEAL are evaluated before
W-ADOPT/W-REDISPATCH (never dispatch new spend into a breached goal
or a wedged repository); W-ESC is the residual for every fact pattern
the table does not name. The goal's remedy-chain words map exactly:
nudge = the one-interval hold that progress-based patience already
gives every stale-not-dead item; revive = the class action; mark =
the would-act record (trial mode) and the episode entry (always);
escalate = W-ESC.

Kills: this design grants NO new kill authority anywhere. The only
process terminations on any path are the ones already landed — the
reaper's owned process-group wind-down at caps, and stop-batch
cancellation through `job stop-cancel-authorize`. The watcher never
signals a process directly.

## 5. The actor boundary (R-21/R-22, and R-1 applied)

The battery's punished combination was one mechanism AUTHORING,
OPERATING, and JUDGING its own workload, with cause classification as
the laundering channel for its own reds. The watch actor is built so
that combination cannot re-form:

- It never judges. Every trigger in section 4 is a mechanical rule
  over durable evidence written by OTHER components (job records by
  dispatch adjudication, receipts by land, census by the supervision
  watcher, health verdicts by the pipeline), and the rules themselves
  land through the full lane loop before the tick runs them. Running
  a landed rule is operating; discretion would be judging. The one
  definitional wall: anything that returned is out of scope (section
  4), so the watcher never touches work products' merit.
- It never accepts. A recovery round it dispatches (W-ADOPT,
  W-REDISPATCH) produces a return that travels the ordinary chain —
  conformance, critique where the class demands it, seat
  certification — exactly as if the seat had dispatched it. The
  watcher never closes chains, never writes dispositions, never marks
  a goal delivered. The evidence that CLOSES an episode is written by
  stores the watcher does not write: a landing receipt, a completed
  job record, a healthy health pass.
- It cannot excuse itself. R-22's cause-blind law is load-bearing
  here: every recovery dispatch spends the goal's human-set attempt
  tuple identically, whatever the cause, through the same governed
  admission as any other dispatch. The battery's laundering channel
  (classifying a red as "orchestration" to not count it) is closed by
  construction — the infra-retry addendum changes what is RECORDED
  about an attempt, never whether it counts.
- Who watches work the watcher itself started: the same detection
  substrate that watches everything else. A watcher-dispatched round
  is an ordinary job record; the delivery role, the per-job watcher,
  and the census judge it with zero knowledge of who dispatched it.
  The once-per-operation ceiling means the watcher can never respond
  to its own response: the second death of any operation — including
  one the watcher started — is W-ESC, a human's.

R-1's two-sided test, run explicitly on the candidate "give the
steward tick the acting power": does the assignment collide powers in
one seat? The tick would hold detect (mechanical rule execution) and
respond (bounded dispatch), but not author (the rules land through
the lanes, and R-25's designer/critic split authors them), not judge
(no discretion, no merit contact), not accept (no closure, no
certification, closure evidence written elsewhere). No collision of
the judge-writing-its-own-law shape → the test PASSES → a new seat is
FORBIDDEN, per R-1's own direction. The reject condition in section
11 names the counter-judgment explicitly, because this is the
design's most consequential claim.

## 6. The authority envelope

What the watcher may do WITHOUT a human word — only after the section
7 trial graduates the class, and always through governed dispatch
admission against the goal's human-set budget tuple:

- W-ADOPT: dispatch one verification-adoption round per operation.
- W-REDISPATCH: dispatch one re-dispatch per operation, including
  the single evidenced-infrastructure retry.
- W-HEAL: lawful owner shutdown + re-arm with the enrolled,
  gate-landed engine, while R-37's word (or its landed successor,
  the provenance-anchored-rearm mechanism) is in force.
- W-BREACH: run the already-landed breach-stop routes and stop-batch
  wind-down (this is existing law with existing owners; listed for
  completeness, not granted here).
- W-IDLE: the already-landed revive path (same note).
- Always: write intents, receipts, would-act records, episode
  updates, narration — the recording that makes the rest auditable.

What ALWAYS requires a human word, forever outside this envelope:

- Resuming a breach-stopped goal (R-32-m1's machinery is the only
  path, and it carries the human's verbatim word).
- Revising any budget tuple, granting any attempt past a fence, or
  dispatching anything a fence refuses.
- A second recovery of the same operation, of any kind.
- Any process termination outside the reaper's and stop batch's
  landed paths; any kill of an UNTRACKED or ANNOUNCED process
  (`proc acknowledge` and everything near it stays human).
- Model or lane substitution (the R-31/R-34 pins stand; a recovery
  round re-uses the dead round's recorded runtime:model exactly).
- Acting on anything outside the section 3.2 tracked enumeration.

What NO word can grant this actor (the R-21 wall): adjudicating,
certifying, or closing work it dispatched; authoring product bytes;
writing dispositions; marking its own heals successful other than by
the independent evidence already named.

## 7. Marking-mode trial and governance records

Action power arrives the Law 2 way, per goal gate-governance-records:

- Slice 2 lands with every new-authority class (W-ADOPT,
  W-REDISPATCH, W-HEAL) in OBSERVE: the tick computes the full
  decision, mints a durable WOULD-ACT record (typed: class, trigger
  evidence, the exact dispatch it would have made) and does not act.
  Would-acts appear on the watch surface and in the heartbeat
  history. W-ESC (escalation) and the already-landed W-BREACH/W-IDLE
  paths operate from day one — escalation is not an authority grant,
  and the landed paths carry their existing authorization.
- Graduation is PER CLASS, by Wido's recorded word, and only after
  the class's trial window: at least 7 days AND at least 10 would-act
  records on the fleet, with a recorded false-alarm bound of ZERO
  wrong-target would-acts (a would-act naming an item that was in
  fact alive, healthy, or already recovered). One false alarm resets
  that class's window. These defaults are per-class judgment the
  governance record may revise, as the goal's freedoms allow.
- Each graduation mints, in one act: (1) a governance record row for
  the class carrying the five required fields — owner: Wido; review
  date; known-bad fixture (W-ADOPT: a failed-job-plus-dirty-worktree
  fixture must produce the adoption would-act/act; W-REDISPATCH:
  failed-job-plus-clean-worktree; W-HEAL: a dead-runner fixture;
  and one known-GOOD fixture per class: a live healthy job must
  produce NO act); appeal route: any actor puts a disputed act to
  Wido by decision-ask, never through the watcher; broken-check
  response: if a class's classifier cannot read its evidence, that
  episode falls through to W-ESC (fail-safe), and a repeating
  classifier error is itself a health finding; and (2) an R-row in
  memory/rulings.md (class=delegated-authority, with the review
  date), because the dated-records watch
  (internal/steward/ruling_sweep.go) parses only R-rows — this is
  exactly the gate-governance-records constraint, no second watch.
- Suggested graduation order, cheapest hazard first: W-HEAL (spends
  no delegate budget), then W-ADOPT (bounded verification spend,
  recovers paid work), then W-REDISPATCH last (spends the most on
  the least-diagnosed class).

## 8. Who watches the watchman

The existing health pipeline, preferred exactly as the brief directs,
and sufficient:

- The acting side is the steward tick. The tick's runner is already a
  named health role (RoleStewardRunner), already inside the mutual
  rings (the supervision watcher checks the steward runner's
  freshness and re-arms it; the tick checks the watcher —
  operator-surface-design Ring 2), and behind them Ring 3's operator-
  owned `up --recover-only --if-down`. A dead acting side is
  therefore detected, healed, and — failing that — escalated by
  machinery that predates this design and shares no execution with
  the interactive verb.
- The interactive `metasystem watch` is stateless and zero-write;
  there is nothing of it to watch. Its failure mode is an error in
  the operator's terminal, which the operator is looking at.
- The one genuinely new failure surface is the classifier itself
  deciding wrongly in silence. Its watchers are: the would-act/act
  records (every decision durable and auditable), the known-good
  fixtures (a healthy item producing an act is a red gate), the
  false-alarm bound with its reset, and the per-class governance
  review date the ruling sweep enforces. No new watching component
  is created, satisfying R-11 and avoiding an infinite regress.

## 9. Rejected alternatives

A. A standalone watch daemon / a new responder seat. Rejected: the
R-1 test in section 5 passes, and a passed test FORBIDS a new seat.
Mechanically it would duplicate the intent machinery, the fence
checks, the receipts, and the mutual-watching rings the steward
already has, and then need its own watchman — the opposite of
consuming landed machinery, and an R-11 failure.

B. Notification-only watch (a status board that alerts well).
Rejected on the order's face: "the system should act!". It would also
merely re-encode in code the seat relay this landing exists to end,
leaving every recovery a hand recipe — the six budget-death
hand-recoveries are the recorded cost of exactly this shape.

C. A diagnostic agent (an LLM session that inspects a stall, decides
what went wrong, and responds free-form). Rejected: W-ESC's law is
escalate-never-guess; a model inferring cause from silence is
guessing with confidence, its "diagnosis" is adjudication (the R-21
wall), and its spend is unbounded. Every cause this design acts on is
a typed record an adapter or health role already wrote.

D. One unified liveness vocabulary mapping all six landed verdict
sets. Rejected in section 3.3: the mapping table becomes a permanent
adjudication surface (every new producer verdict needs a mapping
ruling), and the read surface's job is to present, not translate.

E. Acting from the interactive command (watch --heal, watch --retry
flags). Rejected: it would create a second action path outside the
tick's one-shot intent machinery — two writers to the same
authority, unreconcilable after a crash, and an invitation for a
seat to "just run the flag" past the trial law.

## 10. Migration for the four machines (m0, m1, m2, m3)

1. Slice 1 (read side) lands: per machine, engine rebuild and steward
   re-arm under R-37 (each re-arm records its consumed commit). Zero
   authority change; no trial needed for a zero-write verb. The verb
   joins the six-verb operator surface as verb 5's superset (no-arg
   form new, --job form unchanged).
2. Slice 2 (acting side) lands with W-ADOPT/W-REDISPATCH/W-HEAL in
   OBSERVE on all machines; re-arm again. Would-acts accumulate
   fleet-wide and appear in each machine's heartbeat.
3. The seat relay ends PER MACHINE at step 2's re-arm: from that
   moment the machine's own episodes and heartbeat carry what the
   relay carried, through the armed notify path (an armed steward
   implies a configured channel — runner.go refuses otherwise), and
   later through the escalation channel when it lands, by contract.
   The seat record's clause ("the watch verb's landing ends this
   delegation without a further ruling") needs no new word; this
   design only pins WHEN: at the slice-2 arm, not the slice-1 arm,
   because the relay's content is incidents-and-observations and
   those flow from the acting side's episode writes. A machine whose
   steward is not armed has no relay to end and no watcher either —
   `up` is its remedy, as today.
4. Graduation per class per section 7, fleet-wide by Wido's word (the
   classes are machine-independent law; per-machine graduation would
   mint four diverging authority tables for no hazard-driven reason).
   The m2/m3 shared-Mac caveat rides automatically: R-35-m3's
   progress-based patience is already law on the producers, so load
   slowness never enters the taxonomy as death.

Rollback: any class reverts to OBSERVE by removing its authorization
(the Law 2 direction — demotion is always lawful); the read surface
is deletable without state loss since it owns none.

## 11. Self-grade (R-24)

Confidence: HIGH that the read side is correct and cheap — it is a
join over seven landed stores with no new vocabulary and no writes.
HIGH on W-BREACH and W-IDLE, which are landed law merely surfaced.
MODERATE on the W-ADOPT/W-REDISPATCH triggers: the "died" definition
(terminal without an adjudicated return, typed cause set) is the
load-bearing wall between responding and adjudicating, and its cause
set must exactly match what the adapters actually record — the
implementer must verify each adapter's terminal cause fields against
that set and gap-stop on any adapter that cannot distinguish
process-death from a returned failure.

Weakest claim, declared plainly: that the steward tick may hold both
detection and bounded response without failing R-21 — carried by the
section 5 argument (mechanical rules, no merit contact, no closure,
cause-blind spending, once-per-operation ceiling). Second-weakest:
W-REDISPATCH's value — it spends real budget on the least-diagnosed
death class; that is why it graduates last and is capped at one.

REJECT THIS DESIGN IF: (a) the critic or Wido judges that a tick
executing landed detection rules AND dispatching bounded recovery is
already the punished combination — then R-1's test fails after all, a
separate responder actor is justified and required, and this design
must be rebuilt around that seat rather than patched; or (b) any
response class turns out to require evidence that is not a typed
durable record (i.e., the classifier would have to infer from
silence); or (c) the marking-mode trial is skipped or shortened for
any class under schedule pressure — an unsampled false-alarm bound is
not a bound, and acting authority without it is exactly what
gate-governance-records exists to refuse.
