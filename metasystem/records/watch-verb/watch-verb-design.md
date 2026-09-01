# The watch verb: one surface, heal-first action — design, revision 2

- Goal: plans/goals/watch-verb.md (revision 5, claimed by m0)
- Mode: design only; nothing here is implementation
- Author: Fable design lane; r1 job watch-verb-design-r1, this revision
  job watch-verb-design-r2, 2026-09-01
- Status: DRAFT, critique round 1 folded (all 13 findings WV-R1B-01..13
  ACCEPTED and folded; the disposition appendix in section 12 joins
  each finding to its fold with the re-verified evidence)
- Revision rule followed: converge by narrowing. Where a response
  class cannot be made safe with existing machinery, it ships later or
  not at all; the read surface and the safest classes do not wait on
  the hardest. W-HEAL is deferred entirely (section 4.4), the
  adopt/re-dispatch pair is merged into one class whose classifier
  never judges product presence (section 4.2), and every acting class
  reaches power only through the two-bars promotion pattern: observe
  writes records and cannot promote itself; a human reviewing the
  exact record range promotes (section 7).

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
section 6, and none of it is live before the per-class promotion in
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

Ruling L's "within its lawful authority" is the load-bearing clause
this revision takes seriously: heal-first does not mean heal with
authority that does not exist yet. Where the lawful authority is
absent (W-HEAL, section 4.4) or the machinery to act safely is absent
(the typed terminal cause, section 4.2), the lawful response today is
escalation, and this design says so instead of promising the heal.

## 1. Traced substrate (facts this design consumes, re-verified
against the tree for this revision)

Detection — already landed, none of it rebuilt:

- The claimed-goal-delivery health role
  (internal/steward/delivery.go): per claimed goal on this machine, a
  fail-safe verdict from durable evidence — "failed without recovery:
  job X ended N ago with error E" (newest failed job after the claim
  with no newer landing receipt, delivery.go:105-108) and "burned
  without delivery" for a job past its own capMin plus half
  (per-slice overrun, delivery.go:110-123) or a claim past 150% of a
  sub-norm elapsed limit (delivery.go:127-134). Every unreadable
  record is a dead verdict, never a skip. The role is explicitly
  marked NoAutomaticRemedy today — the acting side of this design is
  the machinery that clause is waiting for. CORRECTED FACT (WV-R1B-11):
  the burned-without-delivery verdicts are the DELIVERY role's health
  findings; they are NOT among the live-stop predicates. The
  stop predicates are exactly `liveStopReason` — elapsed breach past
  the grace band or corrupt over-limit state — plus an existing fence
  or indeterminate evidence (internal/dispatch/stop.go:79-87,
  269-335), and `EnsureBreachStop` re-checks that predicate and
  refuses a goal with "no live-stop breach" (stop.go:124-139).
- The health-role pipeline (internal/steward/health.go): FOURTEEN
  typed roles (the complete constant set and order,
  health.go:43-76 — there is no reaper role; see section 3.3 for how
  the reaper is displayed), alive/dead/unknown per role, observation
  state with failure counts and episodes, one aggregate line and exit
  code (0 healthy, 1 unhealthy, 2 unknown-present).
- Alert episodes (internal/steward/alert_episode.go): open on first
  failure, notify only at Ruling L's escalation conditions, close as
  history when the health verdict heals. CORRECTED FACT (WV-R1B-10):
  the episode store is the durable truth of ESCALATION DELIVERY only.
  Its schema is digest, message, timestamps, delivery attempts,
  acknowledgment (alert_episode.go:47-64); its lifecycle consumes one
  aggregate health verdict — a healthy pass resolves and clears every
  open episode, a changed digest retires and reopens
  (alert_episode.go:227-266). It cannot carry per-action history and
  this design no longer asks it to; the action ledger in section 7.1
  does.
- The notify seam (internal/steward/notify.go): delivery-gated
  operator notification with TWO callable paths — `Deliver` sends one
  message directly (notify.go:40) and `DeliverPending` drains the
  queued store (notify.go:64). The steward RUNNER owns the queue
  drain in its loop (runner.go:131); `Deliver` does not depend on the
  runner — a fact section 8 now relies on (WV-R1B-04). `EnsureRunner`
  refuses to arm a steward without a configured channel, so an armed
  steward implies a live escalation path.
- The steward verdict ladder and revive path
  (internal/steward/verdict.go, revive.go, intervene.go). CORRECTED
  FACT (WV-R1B-01): the ladder's shipped semantics, restated
  exactly — `stalled-idle` is a LIVE worker without recent progress
  and its action is NOTIFY ONLY, because a live holder is never
  displaced (verdict.go:17, 105-107); `stalled-dead` is PROVEN death
  with open work, and only there, with no active continuation, dry
  revivals under the ceiling, and no provider outage, does the ladder
  return ActRevive (verdict.go:118-132). ActRevive dispatches a
  steward-continuation job through a one-shot intent record minted
  before anything happens and consumed exactly once at launch,
  fence-checked, crash-reconcilable, receipted. The runner's
  `--max-revivals` ceiling (default 3) bounds revivals before
  notify-only. CORRECTED FACT (WV-R1B-02): the intent record
  (intervene.go:19-41) carries goal, role, digests, runtime, model,
  and job id — it has NO source-operation or recovery-lineage field
  today; section 7.2 adds one.
- The per-job watcher and its verdicts. CORRECTED FACT (WV-R1B-09):
  the scan-jobs classification (internal/report/scanjobs.go) emits
  DONE / STALE / CAPPED / NEVER-STARTED / VANISHED as OUTPUT LINES
  only (scanjobs.go:100-105); its durable state is a seen-identifier
  set, its running set lives in a caller-owned scratch file, and the
  shell watcher creates that file with mktemp and deletes it on exit
  (scripts/watch-background-jobs.sh:196-197). Those verdicts are NOT
  reconstructable from a store. What IS durable: the job records
  themselves, and the reaper's terminal verdicts written into them —
  `process-lost` and `budget-cap` stamped as the record's error with
  a supervision phase (scripts/agents/dispatch.sh:1065-1077). Section
  3.3 builds the read surface on the durable records, not on the
  emitted lines.
- The breach-stop machinery, ALREADY OPERATING in the tick: the
  steward tick's breach-stop custodian runs `FindBreachStops` and
  executes `EnsureBreachStop` for every breach route on every tick
  today (internal/steward/tick.go:69-99), under Ruling Q's named
  owners. This is landed, running law — the watch surface displays
  its reports; this design grants nothing here.
- `metasystem job watch --job <id>`: the landed single-job follow —
  block until one delegate job is terminal, pinned exit codes.
  CORRECTED FACT (WV-R1B-12): the landed registration is the `watch`
  verb UNDER THE `job` FAMILY (cmd/metasystem/main.go:155,
  implementation runJobWatchVerb at cmd/metasystem/run.go:412-431).
  Revision 1 wrote the command shape wrongly; section 3.1 now
  specifies the canonical routes and compatibility explicitly. The
  CLI dispatcher already supports top-level verbs beside the
  families — `up`, `health`, and `delegate` are special-cased before
  family lookup (main.go, dispatch function) — so a top-level `watch`
  has an exact landed precedent.
- Job records and terminal causes. CORRECTED FACT (WV-R1B-06): the
  job record exposes status and a free-form error text
  (internal/dispatch/jobrecord.go:33, 100-101) and nothing typed
  about WHY a run ended; a nonzero adapter exit becomes the generic
  `runtime_error` (internal/adapter/adjudicate.go:196-203); outage
  marks are repository-level health hints that identify no job and
  authorize nothing (internal/outage/outage.go:1-15, 42-49). The
  typed terminal-cause set revision 1 asserted DOES NOT EXIST in the
  substrate. Section 4.2 makes it an explicit prerequisite with named
  producers instead of assuming it.
- Open-work and running-work reporting (internal/steward/openwork.go,
  `report open-work`, `report running-work`, `report scan-jobs`).
- Supervision component heartbeats: every supervised component writes
  `<component>.heartbeat.json` under
  artifacts/agents/supervision/ (path rule
  internal/supervise/proc.go:56), and freshness-within-twice-interval
  is the landed liveness convention (health.go:525-526 uses exactly
  that window for the watcher). Section 3.3 derives the reaper line
  from this store (WV-R1B-13).

Response recipes proven by hand this week — consumed HONESTLY, with
what each record actually says:

- Budget-death recovery: plans/goals/budget-death-on-return.md
  records three Fable runs that completed their whole product and
  died on the native cap during the return protocol, and says the
  products were "recovered whole from stream or worktree by hand"
  (goal Intent line). CONSUMED CORRECTION (WV-R1B-07): because a
  complete product can live in the OUTPUT STREAM with an EMPTY
  worktree diff, and a non-empty diff can be partial or unrelated,
  diff presence proves nothing in either direction. The mechanized
  recipe therefore never branches on the diff: one recovery round
  examines worktree AND transcript and adopts or redoes under review
  (section 4.2). The root cause (the claude adapter's $5 native
  default) and the write-early-return fix direction belong to that
  goal, not this design.
- Actual overrun: the breach-stop route above (specimen: the
  alert-escalation-channel breach-stop of 2026-09-01T04:16Z, resumed
  only by Wido's relayed word under R-32-m1).
- Wedged supervision: CONSUMED CORRECTION (WV-R1B-03). What
  plans/goals/vm-epoch-identity-drift.md actually records: re-arm
  did NOT recover the identity drift ("arm-supervision --rearm did
  NOT recover"), the interim repair was a HAND EDIT of the owner
  record, and the now-proven root cause is an epoch rounding mismatch
  (the arm path rounds the start time up, probes truncate) whose fix
  direction is a CODE CHANGE at the epoch write, not a runtime heal.
  Additionally, R-37-m3 (memory/rulings.md, row R-37-m3) authorizes
  re-arm only "when a landed change requires the engine rebuilt" —
  not owner shutdown and re-arm on an arbitrary wedge — and the
  census-freshness health check collapses every non-SUCCESS census
  into one cause-free dead verdict ("the latest census did not
  succeed", health.go:529-540), so no landed signal can even
  discriminate the cited wedge from unrelated census failures. All
  three legs of revision 1's W-HEAL are therefore absent from the
  tree; the class is deferred (section 4.4).

Authority laws consumed: R-1 (two-sided conflict test), R-21-m1 /
R-22-m1 (the battery: authoring, operating, and judging one's own
mechanism is the punished combination; attempts spend cause-blind),
Law 2's executable shape (WV-R1B-05): obligation states DRAFT/OBSERVE
record would-refuse and CANNOT refuse
(internal/governance/types.go:212-218); LIMITED/ENFORCED apply an
effect only through a COMPLETE authorization tuple — AuthorizedBy,
AuthorizedAt, AuthorityOperation, AuthorizedEffects, ReviewPolicy,
ReviewOutcome — validated at the consequence decision itself
(types.go:164-206, and `Decide` at types.go:208-236, which refuses on
an incomplete tuple and on any effect outside AuthorizedEffects). The
effect vocabulary is typed (types.go:26-36; the recovery-dispatch
effect used here is `authorize-governed-launch`, types.go:35). Goal
gate-governance-records (governance record with owner, review date,
known-bad fixture, appeal route, broken-check response), R-11
(simplicity), R-24 (self-grade), Ruling Q (grace band), R-35-m3 (slow
is not stalled), R-37-m3 (rebuild re-arm only, due 2026-09-30). The
two-bars promotion pattern is consumed verbatim as the promotion
model: observe is non-refusing code writing durable records; nothing
in the observe slice can promote itself; promotion is a later human
act in which the human reviews the range's complete record, with no
automatic zero-unresolved claim
(records/two-bars/two-bars-design-r4-joint.md:280-287).

Fleet seams: alert-escalation-channel (in flight on m0b) owns outward
delivery — this design produces alert episodes and typed escalation
classes and consumes that channel BY CONTRACT (its outbound-only
contract, plans/alert-channel-design.md section 2, with the
delegate-job-failed and stop-awaiting-resume classes 11a.8/11a.9);
this design never builds or touches delivery transports. Until the
channel lands, the landed notify seam is the delivery path.
ledger-attention (m2) is disjoint: its health role is one more
verdict the read surface displays, nothing more.

Citation honesty note, carried from revision 1: the goal names a
"harness-silent-death law" for the read side's verdict shape. No
document in the tree carries that law under that name; the goal
itself inlines its operational content — work-product mtimes + probe
+ timed verdict — and that inline content is what section 3 binds to.

## 2. Shape in one paragraph

One verb, and a three-rung authority ladder that narrows instead of
assuming. RUNG 1 (slice 1): `metasystem watch` is a ZERO-WRITE
projection — it persists nothing and mutates nothing; it projects the
durable stores (job records, census, health record, delivery
verdicts, alert episodes, steward intents, breach-stop reports,
supervision heartbeats) through the producers' own classification
rules at read time, so reading it is not an authority and needs no
trial. RUNG 2 (slice 2): the acting side lives in the steward tick —
the one lawful automatic-response path (one-shot intents, fence
checks, receipts), already watched by the health pipeline and the
supervision rings — and lands in OBSERVE: it computes decisions and
writes durable WOULD-ACT records to a new typed action ledger, and
acts on nothing. RUNG 3 (slice 3+): each response class is promoted
to acting SEPARATELY, by Wido's recorded word over that class's exact
adjudicated would-act record, minting the complete Law 2 tuple —
never before, never in bulk, and never by the machinery itself.
Classes whose safety machinery does not exist yet are not on the
ladder at all: W-HEAL ships later or not at all (section 4.4). Every
action and would-act is recorded in stores the read surface shows, so
the surface displays not only what is running but what the machinery
did — Ruling L's "history, visible when he looks."

## 3. The read surface (slice 1)

### 3.1 Command and output (WV-R1B-12 folded)

Canonical routes, stated exactly against the landed tree:

- `metasystem watch` — NEW top-level verb, registered in the CLI
  dispatcher's pre-family special cases exactly like `health`
  (cmd/metasystem/main.go, dispatch function). No arguments shows the
  whole surface; `--json` emits one machine-readable snapshot (typed
  enums only, never because-strings, per the R1-06 nothing-is-prose
  law); default output is the human table projection of the same
  snapshot.
- `metasystem watch --job <id>` — runs the IDENTICAL landed
  implementation runJobWatchVerb (cmd/metasystem/run.go:412-431):
  same flags, same poll behavior, same pinned exit codes (terminal-ok
  / terminal-failed / timeout / zombie-suspected). One
  implementation, two routes.
- `metasystem job watch --job <id>` — the landed registration
  (main.go:155) REMAINS, unchanged and undeprecated. No script, hook,
  or caller changes; no removal is proposed by this design. Any
  future consolidation is a separate decision.

Exit codes of the no-argument surface mirror the health totality law:
0 nothing tracked is stalled or unknown; 1 at least one stalled/dead
item; 2 at least one unknown item and nothing dead. Every state is
total: each tracked item always renders a verdict, and an unreadable
store renders as its own named degraded line (fail-safe, matching
delivery.go's every-unreadable-record-is-dead direction), never a
silent omission.

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
   their outcomes, plus the action ledger of section 7.1 (would-acts,
   acts, and their adjudications).
4. Supervision components: owner, watcher, and steward runner
   through their health-role verdicts; the reaper through its
   heartbeat-derived line (section 3.3).
5. Mission runners and monitored runs (the census custody categories
   and `report running-work`'s sources).
6. Open alert episodes and pending notifications.
7. The last census verdict, including UNTRACKED and ANNOUNCED
   processes. Untracked processes are DISPLAYED and never acted on:
   the census judgment on them is human-reserved (`proc acknowledge`
   stays a human-authority verb per the operator-surface removal
   ledger), and their presence blocks death proofs exactly as the
   verdict ladder already says.
8. The never-idle line (vocabulary corrected per WV-R1B-01): when
   `report open-work` shows claimable open work and no tracked
   worker is live, the surface renders one explicit
   open-work-no-live-worker line, ALONGSIDE the steward ladder's own
   repository verdict (which will read stalled-dead or unknown in
   that state, per verdict.go:110-132 — the surface never re-labels
   the ladder's judgment, and never calls this state "stalled-idle",
   which is the ladder's live-worker-without-progress verdict).

### 3.3 The liveness verdict per item (WV-R1B-09 and -13 folded)

Per the goal's inline law, every tracked item carries three facts and
one verdict:

- work-product mtime: the newest of the item's own advancing
  artifacts (job record mtime, transcript sidecar mtime, worktree
  newest-file mtime for jobs; ledger/receipt stamps for goals; state,
  heartbeat, and census stamps for components);
- probe: the clock-step-immune process identity result where a pid is
  recorded (live / dead / unknown / unprovable), never inferred from
  output silence;
- timed verdict: computed at read time from durable stores as
  specified below, with its age.

Durability rule, replacing revision 1's false join promise: the
surface displays ONLY verdicts that are either (a) persisted by a
landed producer, or (b) computable at read time from durable stores
alone. The scan-jobs watcher's emitted DONE / STALE / CAPPED /
NEVER-STARTED / VANISHED lines are neither — they are output-only and
their running-set history dies with the shell watcher
(scanjobs.go:100-105; watch-background-jobs.sh:196-197) — so the
surface does NOT promise to reconstruct them or any verdict history.
Instead, per job, the surface computes the same classifications
fresh, from the record alone:

- terminal record → the record's own status (the DONE equivalent),
  with the reaper's durable terminal verdicts (`process-lost`,
  `budget-cap` — dispatch.sh:1065-1077) surfaced as typed lines. The
  shell watcher's VANISHED (was-running-now-gone) needs predecessor
  state a zero-write reader cannot have; its durable equivalent IS
  the reaper's process-lost terminalization, and that is what the
  surface shows.
- running record → live-with-age when the probe proves life;
  stale-with-age when the newest work-product mtime exceeds the
  configured stale threshold (display only — R-35-m3: slow is never
  rendered as stalled); cap-exceeded when now is past startedAt plus
  capMin (the reaper owns the consequence; the surface only shows the
  fact); zombie-suspected when the probe proves death behind a
  non-terminal record (the landed `job watch` vocabulary).
- pending/pending-setup record older than the start-verify window →
  never-started, computed from the record's own timestamps.

Components: the supervision owner, repo watcher, and steward runner
render their landed health-role verdicts (RoleSupervisionOwner,
RoleRepoWatcher, RoleStewardRunner). The REAPER has no health role in
the shipped fourteen (health.go:43-76), and this design does NOT add
a fifteenth public role, change the health JSON vocabulary, or touch
failure aggregation or breaker behavior. The surface derives one
reaper line directly from the supervision store: the reaper's
heartbeat file (artifacts/agents/supervision/reaper.heartbeat.json,
path rule supervise/proc.go:56) — fresh within twice its recorded
interval renders alive-with-age (the exact convention
componentFreshness uses, health.go:525-526), older renders
stale-with-age, missing or unreadable renders unknown. The line is
labeled `derived` in the JSON snapshot so no consumer mistakes it for
a health-pipeline verdict.

Goals speak the delivery role's alive/dead reasons; the repository
speaks the steward ladder. The verdict VOCABULARIES ARE THE LANDED
ONES, deliberately not unified; minting one umbrella vocabulary was
considered and rejected (section 9, alternative D). The JSON snapshot
carries `{store, verdict, evidence}` triples so a consumer always
knows which vocabulary and which producer it is reading.

## 4. The stall taxonomy and its response classes (slice 2+)

### 4.1 The "died" wall and its typed prerequisite (WV-R1B-06 folded)

"Died" means mechanically: the job record is terminal without an
adjudicated return, with a TYPED terminal cause. A run that RETURNED
— even a malformed or red return — is not dead and is NEVER this
machinery's business: its disposition belongs to the dispatching
seat's ordinary review chain. That single definition is what keeps
the watcher out of adjudication (section 5).

The typed terminal cause DOES NOT EXIST in today's substrate: the
record carries only free-form error text (jobrecord.go:100-101), a
nonzero adapter exit collapses to `runtime_error`
(adjudicate.go:196-203), and outage marks identify no job
(outage.go:1-15). Therefore slice 2 carries, as its FIRST work item
and a hard prerequisite for even OBSERVING the W-RECOVER class, a
durable `terminalCause` field on the job record with an exhaustive
producer enumeration:

- the reaper stamps `process-lost` and `budget-cap` (it already
  writes exactly these as error strings, dispatch.sh:1065-1077; the
  typed field mirrors them at the same write);
- adapter adjudication stamps `native-budget-cap` where the adapter's
  own result identifies its native cap, and
  `infra-capacity` / `infra-auth` / `infra-network` from the same
  classifier that already writes outage marks on overload
  (adjudicate.go, the outage.Record call site); everything else
  nonzero remains `runtime-error`;
- a returned run stamps `returned` (or the field stays empty on
  clean completion).

The W-RECOVER classifier reads ONLY this field. Error text is never
parsed. A terminal-failed record without a typed cause in the died
set {process-lost, budget-cap, native-budget-cap, infra-capacity,
infra-auth, infra-network} is OUT of W-RECOVER and routes to W-ESC.
The field's exact producer wiring is implementation, but the
implementer has no judgment call: every producer above is named, and
any adapter that cannot distinguish process-death from a returned
failure is a gap-stop, not a guess.

### 4.2 The response-class table (WV-R1B-01, -02, -07, -11 folded)

The classifier is a total function from TYPED evidence to exactly one
class.

| Class | Typed trigger (all facts from durable records) | Response | Bound |
|---|---|---|---|
| W-REVIVE proven-dead seat (revision 1's W-IDLE, renamed to match the shipped semantics) | The steward ladder's OWN ActRevive decision: verdict stalled-dead — proven death with open work, census complete, nothing untracked or unprovable — with no active continuation, dry revivals under the ceiling, and no provider outage (verdict.go:118-132). `stalled-idle` (a LIVE worker without progress) is NOTIFY-ONLY and never triggers dispatch: a live holder is never displaced (verdict.go:105-107) | The LANDED revive path unchanged: steward-continuation dispatch via one-shot intent | Landed max-revivals (3) per the runner; already armed law, listed for display completeness — no new authority, and no broadened trigger; any broader trigger would be a NEW class requiring its own observe trial and promotion |
| W-RECOVER died delegate round (merges revision 1's W-ADOPT and W-REDISPATCH) | The delivery role's failed-without-recovery join (newest failed job after the claim, no newer landing receipt — delivery.go:105-108) AND the job's terminalCause is in the died set of 4.1 AND the record names a goal (a goal-less job has no budget tuple to admit against → W-ESC) AND the operation's recovery-root is unconsumed (section 7.2) | Dispatch ONE governed verify-adopt-or-redo round on the same goal: a fresh dispatch whose brief embeds the dead round's brief and points at the dead round's worktree AND transcript, instructing: adopt any intact product found in either, otherwise redo the work; the round's return travels the ordinary review chain. The watcher NEVER judges product presence — a worktree diff proves nothing in either direction (products have been recovered whole from the stream with an empty diff, and a dirty diff can be partial or unrelated: budget-death-on-return goal, Intent line) — so there is no adopt/re-dispatch branch to get wrong. Same runtime:model as the dead round (R-31/R-34 pins), same role, same capMin. An infra-typed cause is recorded as infra-retry on the ledger entry — R-22: it changes what is RECORDED, never whether the attempt counts | ONCE per recovery-root, ever (section 7.2 — the ceiling that survives follow-up identity churn); spends one attempt cause-blind through governed admission; admission refusal (any fence) → W-ESC with the breach evidence; the recovery round dying → W-ESC (its recovery-root is already consumed) |
| W-BREACH actual budget overrun | EXACTLY the typed output of FindBreachStops: a live elapsed breach past Ruling Q's grace band, corrupt over-limit state, an existing fence, or indeterminate evidence (stop.go:79-87, 269-335) — and NOTHING else. A delivery burned-without-delivery verdict (per-job cap burn or 150% elapsed, delivery.go:110-134) is NOT a stop trigger: it is a health finding that flows through the delivery role's episode and escalation path, and where its underlying job died it is W-RECOVER's business. The tick never broadens liveStopReason, and an EnsureBreachStop refusal "no live-stop breach" (the mandatory re-check, stop.go:124-139) is a healthy-world no-op, never a failed response | Already landed AND already running: the tick's breach-stop custodian executes these routes every tick today (tick.go:69-99); the fence closes, the stop batch winds live jobs down through the lawful stop path, then MANDATORY escalation: resume is a human act (R-32-m1), episode class stop-awaiting-resume | Existing law with existing owners (Ruling Q); this design only DISPLAYS the custodian's reports; nothing is granted; no healing is attempted past a closed fence |
| W-ESC unknown / everything else | Verdict unknown or degraded; untracked processes blocking a death proof; unreadable evidence; a terminal cause outside the typed died set or missing; a goal-less died job; a consumed recovery-root; wedged supervision (section 4.4); any breaker or fence reached above | Escalate, never guess: open or advance the alert episode; deliver per Ruling L's conditions through the notify seam / escalation channel. NO dispatch, NO kill, NO probe beyond the read side's | Unknown is held one interval then escalates (the health totality law); never kill on silence stands absolute |

Ordering within one tick: W-BREACH before W-RECOVER (never dispatch
new spend into a breached goal); W-ESC is the residual for every fact
pattern the table does not name. The goal's remedy-chain words map
exactly: nudge = the one-interval hold that progress-based patience
already gives every stale-not-dead item; revive = W-REVIVE's landed
action; mark = the would-act ledger entry (observe mode) and the
episode entry (always); escalate = W-ESC.

### 4.3 Kills

This design grants NO new kill authority anywhere. The only process
terminations on any path are the ones already landed — the reaper's
owned process-group wind-down at caps, and stop-batch cancellation
through `job stop-cancel-authorize`. The watcher never signals a
process directly.

### 4.4 W-HEAL: deferred, not designed around (WV-R1B-03 folded)

Revision 1's W-HEAL (wedged supervision → lawful owner shutdown +
re-arm) is REMOVED from the ladder. Each of its three legs is absent
from the tree, verified for this revision:

1. No authority: R-37-m3 authorizes re-arm only when a landed change
   requires the engine rebuilt (memory/rulings.md, R-37-m3 row), not
   owner shutdown and re-arm on a wedge; the row expires 2026-09-30.
2. No working recipe: the one recorded incident says re-arm did NOT
   recover and a hand edit of the owner record was required
   (plans/goals/vm-epoch-identity-drift.md, Intent and Next step);
   the now-proven root cause (epoch round-up at write vs truncation
   at probe) is fixed by a code change at the epoch write, which is
   that goal's work, not a runtime heal.
3. No discriminator: the census-freshness check collapses every
   non-SUCCESS census into one cause-free dead verdict
   (health.go:529-540), so a heal triggered on it would fire on
   unrelated census failures and still miss the named incident.

Until a typed incident discriminator, a mechanically proven recipe,
and matching human authority ALL exist, wedged supervision routes to
W-ESC: the machinery escalates with the evidence, and the human (or
the seat under the human's standing orders) heals by hand. W-HEAL may
return only as its own future design — carrying those three legs —
and then enters the SAME ladder at observe, promoted only by Wido's
word like every other class. The relay-ending claim in section 10
does not depend on it.

## 5. The actor boundary (R-21/R-22, and R-1 applied)

The battery's punished combination was one mechanism AUTHORING,
OPERATING, and JUDGING its own workload, with cause classification as
the laundering channel for its own reds. The watch actor is built so
that combination cannot re-form:

- It never judges. Every trigger in section 4 is a mechanical rule
  over durable TYPED evidence written by OTHER components (job
  records and terminal causes by dispatch adjudication and the
  reaper, receipts by land, census by the supervision watcher, health
  verdicts by the pipeline), and the rules themselves land through
  the full lane loop before the tick runs them. Running a landed rule
  is operating; discretion would be judging. The one definitional
  wall: anything that returned is out of scope (section 4.1), so the
  watcher never touches work products' merit. The W-RECOVER round's
  adopt-or-redo judgment is made by the DELEGATE inside that round,
  reviewed by the ordinary chain — never by the watcher.
- It never accepts. A recovery round it dispatches produces a return
  that travels the ordinary chain — conformance, critique where the
  class demands it, seat certification — exactly as if the seat had
  dispatched it. The watcher never closes chains, never writes
  dispositions, never marks a goal delivered. The evidence that
  CLOSES an episode or an action-ledger entry is written by stores
  the watcher does not write: a landing receipt, a completed job
  record, a healthy health pass.
- It never adjudicates its own trial. The would-act records that feed
  graduation are labeled by an independent adjudicator (section 7.3),
  never by the watcher; the watcher's ledger entries are append-once
  and the adjudication is a separate record written by a different
  actor.
- It cannot excuse itself. R-22's cause-blind law is load-bearing:
  every recovery dispatch spends the goal's human-set attempt tuple
  identically, whatever the cause, through the same governed
  admission as any other dispatch. The infra-retry annotation changes
  what is RECORDED about an attempt, never whether it counts.
- Who watches work the watcher itself started: the same detection
  substrate that watches everything else, PLUS a lineage ceiling that
  survives identity churn. A watcher-dispatched round is an ordinary
  job record; the delivery role, the reaper, and the census judge it
  with zero knowledge of who dispatched it. The recovery-root ceiling
  (section 7.2) means the watcher can never respond to its own
  response: the death of a recovery round consumes the same
  recovery-root as the original, and the next event on that root is
  W-ESC, a human's.

R-1's two-sided test, run explicitly on the candidate "give the
steward tick the acting power": does the assignment collide powers in
one seat? The tick would hold detect (mechanical rule execution) and
respond (bounded dispatch), but not author (the rules land through
the lanes, and R-25's designer/critic split authors them), not judge
(no discretion, no merit contact, no trial self-adjudication), not
accept (no closure, no certification, closure evidence written
elsewhere). No collision of the judge-writing-its-own-law shape → the
test PASSES → a new seat is FORBIDDEN, per R-1's own direction. The
reject condition in section 11 names the counter-judgment explicitly,
because this is the design's most consequential claim.

## 6. The authority envelope

What the watcher may do WITHOUT a human word — only per class, only
after THAT class's promotion (section 7), and always through governed
dispatch admission against the goal's human-set budget tuple:

- W-RECOVER: dispatch one verify-adopt-or-redo round per
  recovery-root, with the infra-retry annotation where typed.
- W-BREACH: nothing new — the tick's custodian already runs the
  breach-stop routes under existing law and owners (tick.go:69-99);
  listed for display completeness.
- W-REVIVE: nothing new — the landed revive path under its landed
  ceiling (same note).
- Always, from slice 2 on: write would-act and act entries to the
  action ledger, intents, receipts, episode updates, narration — the
  recording that makes the rest auditable. Recording is not acting.

What ALWAYS requires a human word, forever outside this envelope:

- Resuming a breach-stopped goal (R-32-m1's machinery is the only
  path, and it carries the human's verbatim word).
- Revising any budget tuple, granting any attempt past a fence, or
  dispatching anything a fence refuses.
- A second recovery on the same recovery-root, of any kind.
- Supervision heal of any form (W-HEAL is deferred; wedges escalate).
- Any process termination outside the reaper's and stop batch's
  landed paths; any kill of an UNTRACKED or ANNOUNCED process
  (`proc acknowledge` and everything near it stays human).
- Model or lane substitution (the R-31/R-34 pins stand; a recovery
  round re-uses the dead round's recorded runtime:model exactly).
- Acting on anything outside the section 3.2 tracked enumeration.
- Promotion itself: no code path, configuration, or record the
  machinery can write may move a class from observe to acting (the
  two-bars rule, verbatim).

What NO word can grant this actor (the R-21 wall): adjudicating,
certifying, or closing work it dispatched; adjudicating its own trial
samples; authoring product bytes; writing dispositions; marking its
own actions successful other than by the independent evidence already
named.

## 7. Observe mode, the action ledger, and per-class promotion

### 7.1 The action ledger (WV-R1B-10 folded)

The durable action history revision 1 wrongly assigned to alert
episodes is a NEW typed store: the watcher action ledger, one
append-once JSON record per decision under
artifacts/agents/steward/actions/ (sibling of the intents store,
which stays as-is). Each record carries, typed and complete:

- `id`, `machine`, `mintedAt`, `class` (W-RECOVER; future classes as
  they are designed in);
- `target`: goalId, jobId, operationId, recoveryRoot;
- `triggerEvidence`: the exact typed facts that fired the classifier
  (terminalCause, delivery join result, timestamps) plus the paths of
  the evidence files consulted and each file's sha256 at decision
  time — the snapshot that makes later adjudication possible without
  trusting memory;
- `decision`: `would-act` (observe) or `act` (promoted), with the
  proposed dispatch pinned: role, runtime, model, capMin,
  briefDigest;
- when acted: the dispatched jobId and the one-shot intent nonce.

Entries are immutable. Nothing in the ledger closes itself: an
entry's outcome is COMPUTED at read time by joining the stores other
components write (the recovery round's job record and review chain, a
landing receipt, a healthy delivery verdict), and the watch surface
displays that join. Alert episodes remain exactly what they are — the
escalation-delivery lifecycle (alert_episode.go) — and are not
extended, not re-keyed, and not consumed by graduation.

### 7.2 The recovery-root ceiling (WV-R1B-02 folded)

The once-ever ceiling cannot bind to operation identity: every
follow-up round derives a DISTINCT operation id because its parent
and brief participate in the identity (operation.go:10-37), and
budget projection treats distinct ids as distinct operations
(budget.go:293-365). A dead recovery child would be the first death
of its new identity, and the watcher could chain recoveries forever.

Therefore: `recoveryRoot` is an immutable lineage identity. For an
operation with no recovery ancestry, recoveryRoot = its own
operationId. Every watcher-created recovery dispatch carries its
target's recoveryRoot forward unchanged — stamped on the action
ledger entry, on the recovery job's record (a new record field,
landed in slice 2 alongside terminalCause), and on the minted intent
(a new intent field beside the existing Goal/JobId fields,
intervene.go:19-41). The ceiling is: before minting any recovery
intent (and again at the one-shot intent's consumption at launch —
the base-action boundary), the watcher scans the goal's job records
and the action ledger for ANY entry bearing the same recoveryRoot
with decision `act` (or any recovery job record stamped with it); if
one exists, the classifier's output is W-ESC, mechanically. In
observe mode the same rule applies to would-acts, so the trial
samples exercise the ceiling too. A record predating the field has no
recoveryRoot and therefore equals its own operationId — the rule is
total over old and new records.

### 7.3 Observe mode and independent adjudication (WV-R1B-08 folded)

Slice 2 lands with W-RECOVER in OBSERVE: the tick computes the full
decision, writes the would-act ledger entry, and does not act.
Would-acts appear on the watch surface and in the heartbeat history.
W-ESC (escalation) operates from day one — escalation is not an
authority grant — and W-BREACH/W-REVIVE continue operating under
their existing landed authorization, unchanged.

The trial's truth record is adjudicated, durably, by an independent
actor:

- Label owner: the SEAT (the machine's main orchestrator session), or
  Wido directly — never the watcher, never any delegate the watcher
  dispatched. This is the same actor-boundary rule as section 5,
  applied to the trial.
- Adjudication record: a separate append-once sidecar file per ledger
  entry (actions/<id>.adjudication.json) carrying {verdict:
  correct-target | false-alarm, adjudicator identity, adjudicatedAt,
  evidence cited}. The watcher never writes this file; the surface
  joins it to the entry.
- Evidence source: the entry's own triggerEvidence snapshot (paths +
  digests, 7.1) plus the durable stores as of adjudication. A
  false-alarm is a would-act whose named target was in fact alive,
  healthy, or already recovered at mintedAt — decidable from the
  snapshot without trusting anyone's memory.
- Deadline and debt: an unadjudicated would-act older than 7 days
  renders as `trial-debt` on the watch surface and in the heartbeat.
  Unadjudicated samples count toward NOTHING — not the sample
  minimum, not the false-alarm count.

Graduation input per class: at least 7 days of observe AND at least
10 ADJUDICATED would-act records fleet-wide with ZERO false-alarms.
One false-alarm resets that class's window and count. These defaults
are per-class judgment the governance record may revise, as the
goal's freedoms allow. There is no automatic graduation claim: per
the two-bars pattern (consumed verbatim,
two-bars-design-r4-joint.md:280-287), promotion is a later human act
in which Wido reviews the class's COMPLETE ledger range — including
trial-debt and any unadjudicatable samples — and nothing the observe
slice writes can promote itself.

### 7.4 The Law 2 binding and the refusing seam (WV-R1B-05 folded)

Authority is granted ONLY through the shipped governance mechanism,
per class:

- At slice 2, each acting-candidate class (today: W-RECOVER alone) is
  registered as a GovernedObligation in state OBSERVE with Effects =
  [authorize-governed-launch] (the typed effect, types.go:35). In
  OBSERVE, `Decide` returns would-refuse and CANNOT authorize
  (types.go:212-218) — matching the class's marking mode exactly.
- Promotion mints, in one human act: (1) the obligation moved to
  LIMITED with the COMPLETE authorization tuple — AuthorizedBy,
  AuthorizedAt, AuthorityOperation naming the class, AuthorizedEffects
  = [authorize-governed-launch], ReviewPolicy, ReviewOutcome — the
  exact fields ValidateAuthorizationCompleteness demands
  (types.go:164-206); (2) the governance record row for the class
  with the five gate-governance-records fields — owner: Wido; review
  date; known-bad fixture (W-RECOVER: a died-job fixture with a typed
  cause must produce the would-act/act, and a known-GOOD fixture — a
  live healthy job, and separately a died job with a consumed
  recovery-root — must produce NO act); appeal route: any actor puts
  a disputed act to Wido by decision-ask, never through the watcher;
  broken-check response: a classifier that cannot read its evidence
  falls through to W-ESC (fail-safe), and a repeating classifier
  error is itself a health finding; and (3) an R-row in
  memory/rulings.md (class=delegated-authority, with the review
  date), because the dated-records watch parses only R-rows. The
  PROSE records (2) and (3) grant nothing: they are the human-readable
  registration of the grant; power flows only through (1).
- The refusing seam, named: the watcher's action path calls
  `Decide(authorize-governed-launch)` on the class's obligation at
  BOTH the intent-minting step and the one-shot intent's consumption
  at launch — the latter is the base-action boundary, inside the same
  governed dispatch admission every launch already passes. Anything
  but Apply=true (draft, observe, incomplete tuple, uncovered effect
  — types.go:208-236) refuses the action; the refusal is recorded on
  the ledger entry and routes to W-ESC. An implementer has no
  question about whether an R-row or a prose record can make action
  reachable: it cannot, because the seam consults only the obligation
  tuple.
- W-HEAL has no obligation, no effects mapping, and no seam in this
  design, because it has no class (section 4.4). Its future design
  must bring all three.

Suggested promotion order, cheapest hazard first: W-RECOVER is the
only candidate this design puts on the ladder. Revision 1's order
(heal first, adopt, then redispatch) dissolved with the deferral of
W-HEAL and the adopt/redispatch merge.

## 8. Who watches the watchman (WV-R1B-04 folded)

The existing health pipeline, preferred exactly as the brief directs
— with revision 1's silent hole closed:

- The acting side is the steward tick, whose runner is a named health
  role (RoleStewardRunner) inside the mutual rings: the supervision
  watcher checks and repairs the enrolled runner every pass
  (supervise_component.go:202-205), the tick checks the watcher, and
  behind them Ring 3's operator-owned `up --recover-only --if-down`.
- THE HOLE, verified: when the watcher's runner repair FAILS, today
  the failure is only a PASS_FAILED component attempt and an error
  returned to a loop that logs and continues
  (supervise_component.go:206-210, 23-35 and the loop at 151-153);
  Ring 3 likewise returns recovery-partial without alerting
  (up.go:502-546). But the dead runner is exactly the component that
  delivers pending notifications (runner.go:131) — so runner death
  plus failed repair had NO live owner able to deliver the promised
  escalation. Slice 2 closes this: on a failed runner repair (error,
  AUTO_HEAL_ENDED, NOT_ENROLLED, or ENROLLMENT_CHANGED), the
  supervision watcher calls the notify seam's DIRECT send —
  `steward.Deliver` (notify.go:40), which does not pass through the
  dead runner's pending queue — from its own separately executing
  process. Cadence, mechanical: the watcher keeps a small durable
  escalation state under the supervision directory
  (runner-repair-escalation.json: consecutiveFailures,
  lastDeliveredAt); it delivers on the FIFTH consecutive failed
  repair (Ruling L's five-observation breaker shape — the per-pass
  repair attempts ARE the healing) and once per 60 passes thereafter
  while the failure persists; any successful repair resets the state.
  The PASS_FAILED attempt records remain the durable evidence trail.
  ACCEPTANCE FIXTURE, required by this design: with the steward
  runner dead AND its repair forced to fail, an operator delivery
  must occur — proven with no live runner in the fixture.
- The interactive `metasystem watch` is stateless and zero-write;
  there is nothing of it to watch. Its failure mode is an error in
  the operator's terminal, which the operator is looking at.
- The one genuinely new failure surface is the classifier itself
  deciding wrongly in silence. Its watchers are: the action ledger
  (every decision durable, snapshot-evidenced, and independently
  adjudicated — section 7), the known-good fixtures (a healthy item
  producing an act is a red gate), the false-alarm bound with its
  reset, and the per-class governance review date the ruling sweep
  enforces. No new watching component is created, satisfying R-11
  and avoiding an infinite regress.

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
leaving every recovery a hand recipe — the budget-death
hand-recoveries are the recorded cost of exactly this shape. The
narrowing in this revision is not this alternative: W-RECOVER reaches
real action through the ladder; only the classes whose safety
machinery does not exist are deferred.

C. A diagnostic agent (an LLM session that inspects a stall, decides
what went wrong, and responds free-form). Rejected: W-ESC's law is
escalate-never-guess; a model inferring cause from silence is
guessing with confidence, its "diagnosis" is adjudication (the R-21
wall), and its spend is unbounded. Every cause this design acts on is
a typed record an adapter, the reaper, or a health role wrote.

D. One unified liveness vocabulary mapping all the landed verdict
sets. Rejected in section 3.3: the mapping table becomes a permanent
adjudication surface (every new producer verdict needs a mapping
ruling), and the read surface's job is to present, not translate.

E. Acting from the interactive command (watch --heal, watch --retry
flags). Rejected: it would create a second action path outside the
tick's one-shot intent machinery — two writers to the same
authority, unreconcilable after a crash, and an invitation for a
seat to "just run the flag" past the trial law.

F. Extending the alert-episode store into the action history
(considered for WV-R1B-10). Rejected: the episode key is a health
digest and its lifecycle is aggregate — one healthy pass clears
everything, a changed digest swaps episodes
(alert_episode.go:227-266) — so tracking N concurrent recoveries
would mean rebuilding the store's key and lifecycle inside a schema
other consumers already read. A separate append-once ledger is
smaller and touches nothing landed.

G. Keeping W-ADOPT and W-REDISPATCH as separate classes with a
product-manifest test (considered for WV-R1B-07). Rejected: no landed
producer records a product manifest, so the branch would either
consume evidence that does not exist or fall back to the refuted diff
test. One merged class with the adopt-or-redo judgment made inside
the reviewed recovery round needs no product evidence at classify
time at all.

## 10. Migration for the four machines (m0, m1, m2, m3)

1. Slice 1 (read side) lands: per machine, engine rebuild and steward
   re-arm under R-37-m3 (a rebuild-triggered re-arm is exactly its
   scope; each re-arm records its consumed commit). Zero authority
   change; no trial needed for a zero-write verb. The verb joins the
   operator surface as the top-level `watch` beside `up`/`health`/
   `delegate`; `job watch` unchanged.
2. Slice 2 (observe side) lands with the terminalCause and
   recoveryRoot record fields, the action ledger, the watcher's
   direct-deliver escalation path (section 8), the W-RECOVER
   obligation in OBSERVE, and W-RECOVER marking; re-arm again (again
   under R-37-m3's rebuild scope). Would-acts accumulate fleet-wide
   and appear in each machine's heartbeat.
3. The seat relay ends PER MACHINE at step 2's re-arm: from that
   moment the machine's own episodes and heartbeat carry what the
   relay carried, through the armed notify path — now including the
   watcher's runner-death direct delivery, which removes the one case
   where the machine could not speak for itself (section 8). The seat
   record's clause ("the watch verb's landing ends this delegation
   without a further ruling") needs no new word; this design only
   pins WHEN: at the slice-2 arm, not the slice-1 arm, because the
   relay's content is incidents-and-observations and those flow from
   the observe side's episode and ledger writes. A machine whose
   steward is not armed has no relay to end and no watcher either —
   `up` is its remedy, as today.
4. Slice 3+: promotion per class per section 7.4, fleet-wide by
   Wido's word (the classes are machine-independent law; per-machine
   graduation would mint four diverging authority tables for no
   hazard-driven reason). The m2/m3 shared-Mac caveat rides
   automatically: R-35-m3's progress-based patience is already law on
   the producers, so load slowness never enters the taxonomy as
   death. W-HEAL has no slice: it waits on its own design (section
   4.4) and, if it comes, enters at observe like everything else.

Rollback: any class reverts to OBSERVE by demoting its obligation
(the Law 2 direction — demotion is always lawful, and the refusing
seam of 7.4 makes the demotion effective at the next decision); the
read surface is deletable without state loss since it owns none.

## 11. Self-grade (R-24)

Confidence: HIGH that the read side is correct and cheap — after the
WV-R1B-09/12/13 folds it promises only what durable stores can give:
producers' own persisted verdicts plus read-time computation, one new
top-level route with a landed precedent, and a derived reaper line
that touches no health vocabulary. HIGH on W-BREACH and W-REVIVE,
which are landed, already-running law merely displayed — the r1 error
of mapping stalled-idle to revival is gone. MODERATE on W-RECOVER:
its wall (typed terminalCause, recovery-root ceiling, no product
judgment) is now buildable from named producers, but the terminalCause
field is new substrate and its producer enumeration must survive
implementation contact — any adapter that cannot distinguish
process-death from a returned failure is a gap-stop. MODERATE on the
section 8 direct-deliver path: it is small and uses an existing
callable seam (notify.go:40), but it is the one place slice 2 adds
behavior to a supervision component, and its fixture (delivery with
no live runner) is the proof the claim stands on.

Honest trajectory note: revision 1 claimed four acting classes and a
day-one heal; this revision ships one acting candidate, defers the
heal outright, and re-grounds every remaining claim in file:line
facts re-read for this round. That narrowing is the fold's center of
mass, applied.

Weakest claim, declared plainly: that the steward tick may hold both
detection and bounded response without failing R-21 — carried by the
section 5 argument (mechanical rules, no merit contact, no closure,
no trial self-adjudication, cause-blind spending, recovery-root
ceiling). Second-weakest: W-RECOVER's value on the redo side — it
spends real budget on the least-diagnosed death class; that is why
its trigger is the narrowest (typed causes only) and its ceiling is
one per lineage, ever.

REJECT THIS DESIGN IF: (a) the critic or Wido judges that a tick
executing landed detection rules AND dispatching bounded recovery is
already the punished combination — then R-1's test fails after all, a
separate responder actor is justified and required, and this design
must be rebuilt around that seat rather than patched; or (b) any
response class turns out to require evidence that is not a typed
durable record (i.e., the classifier would have to infer from
silence); or (c) the observe trial, the independent adjudication, or
the per-class Law 2 promotion is skipped or shortened for any class
under schedule pressure — an unsampled false-alarm bound is not a
bound, and acting authority without a complete authorization tuple is
exactly what gate-governance-records and Law 2 exist to refuse.

## 12. Critique round 1 disposition appendix

Every finding in records/watch-verb/watch-verb-critique-r1.md,
WV-R1B-01 through WV-R1B-13, was ACCEPTED by the seat; this revision
FOLDS all thirteen. Each finding's cited evidence was re-read against
the tree for this revision; every citation held, so none is rebutted.

| Finding | Fold | Where in this revision | Evidence re-verified |
|---|---|---|---|
| WV-R1B-01 (critical) | stalled-idle stays notification-only; revival binds to stalled-dead through the ladder's own ActRevive decision; the never-idle line no longer misuses the stalled-idle name; any broader trigger is declared a new class needing its own trial | §1 (ladder facts), §4.2 W-REVIVE row, §3.2 item 8 | verdict.go:17, 105-107 (stalled-idle → notify, live holder never displaced); verdict.go:118-132 (ActRevive only on proven death) |
| WV-R1B-02 (critical) | recoveryRoot lineage identity: immutable across the original operation and all watcher-created descendants; stamped on ledger, record, and intent; enforced at mint AND at intent consumption; total over pre-field records | §7.2, §5 (who-watches bullet), §6 | operation.go:10-37 (follow-ups get distinct ids); budget.go:293-365 (distinct ids = distinct operations); intervene.go:19-41 (no lineage field today) |
| WV-R1B-03 (critical) | W-HEAL removed from the ladder; wedged supervision → W-ESC; return path defined (own design with typed discriminator, proven recipe, human authority; enters at observe) | §4.4, §6, §10 item 4 | memory/rulings.md R-37-m3 row (rebuild-only scope, due 2026-09-30); vm-epoch-identity-drift.md Intent + Next step (re-arm did not recover; hand edit; root cause = epoch rounding); health.go:529-540 (census failure causes collapsed) |
| WV-R1B-04 (critical) | watcher's failed runner repair now escalates via the direct `Deliver` seam from the watcher's own process, breaker-cadenced, with a required no-live-runner fixture | §8 | supervise_component.go:202-210 (repair error joins passErrors), 151-153 (loop logs and continues); runner.go:131 (runner owns pending delivery); up.go:502-546 (Ring 3 returns recovery-partial); notify.go:40 (direct Deliver exists) |
| WV-R1B-05 (critical) | per-class GovernedObligation with the complete shipped tuple; effect typed as authorize-governed-launch; refusing seam named at intent mint and intent-consumption launch; prose records grant nothing | §7.4 | types.go:164-206 (tuple completeness), 208-236 (Decide refuses incomplete/uncovered), 35 (effect); seat-governance-record.md "May not promote" clause |
| WV-R1B-06 (high) | typed terminalCause field made an explicit slice-2 prerequisite with an exhaustive named producer set; W-RECOVER reads only the field; untyped/missing cause → W-ESC; no observation before the field lands | §4.1, §10 item 2 | jobrecord.go:100-101 (ErrorText only); adjudicate.go:196-203 (nonzero exit → runtime_error); outage.go:1-15 (marks identify no job); dispatch.sh:1065-1077 (reaper's cause strings) |
| WV-R1B-07 (high) | adopt/redispatch merged into W-RECOVER; the classifier never judges product presence; one verify-adopt-or-redo round examines worktree AND transcript inside the ordinary review chain; the diff-branch fixture is gone | §4.2 W-RECOVER row, §9 alternative G | budget-death-on-return.md Intent (products recovered whole from stream or worktree — refutes both diff branches) |
| WV-R1B-08 (high) | independent adjudication defined: seat/Wido as label owner, per-entry evidence snapshot with digests, adjudication sidecar the watcher never writes, 7-day trial-debt visibility, only adjudicated samples count, human reviews the complete range at promotion | §7.1, §7.3 | health.go:78-89 (role verdicts carry no trial adjudication); two-bars-design-r4-joint.md:280-287 (promotion pattern) |
| WV-R1B-09 (high) | zero-write read surface re-founded on durable stores + read-time computation; scan-jobs emitted verdicts and history explicitly NOT promised; VANISHED's durable equivalent is the reaper's process-lost terminalization | §3.3, §1 (per-job watcher fact) | scanjobs.go:29-40 (state = seen ids + caller-owned running file), 100-105 (verdicts are output lines), 258-281 (VANISHED needs prior running state); watch-background-jobs.sh:196-197 (running file mktemp + deleted on exit); dispatch.sh:1065-1077 (durable reaper verdicts) |
| WV-R1B-10 (high) | new append-once action ledger (actions/) with typed schema; outcomes computed by read-time join, never self-closed; alert episodes left untouched as escalation-delivery truth | §7.1, §9 alternative F | alert_episode.go:47-64 (schema: digest/message/attempts, no action fields), 227-266 (aggregate clear-or-replace lifecycle) |
| WV-R1B-11 (medium) | W-BREACH bound to the exact FindBreachStops typed routes, already run by the tick's custodian; delivery burn verdicts stay on the health/escalation path; EnsureBreachStop's refusal is a no-op, never a failed response; job-cap delivery death classified under delivery role + W-RECOVER | §4.2 W-BREACH row, §1 (delivery correction) | delivery.go:110-134 (per-job burn and 150% elapsed are delivery verdicts); stop.go:79-87 (liveStopReason), 124-139 (mandatory recheck), 269-335 (route predicates); tick.go:69-99 (custodian already runs the routes) |
| WV-R1B-12 (medium) | canonical command shape specified: new top-level `watch` (precedent: up/health/delegate special cases); `watch --job` runs the identical runJobWatchVerb; `job watch` retained unchanged, no deprecation, no script changes | §3.1, §1 (command fact) | main.go:155 (watch under the job family), dispatch function (top-level special cases); run.go:412-431 (runJobWatchVerb flags and exit path) |
| WV-R1B-13 (medium) | no fifteenth health role; reaper line derived read-time from reaper.heartbeat.json freshness (2× recorded interval, the landed convention), labeled `derived` in JSON | §3.3 | health.go:43-76 (complete fourteen-role set, no reaper); supervise/proc.go:56 (heartbeat path rule); health.go:525-526 (2× interval freshness convention) |
