# The watch verb: persisted truth first, governed action later — design, revision 4

- Goal: `plans/goals/watch-verb.md`, revision 5, claimed by m0.
- Joint-round authority: Wido's R-38-m0 exception for one round holding
  both design and implementation pens.
- Inputs: revision 2 at SHA-256 `b7544394` and all accepted findings
  `WV-R2B-01` through `WV-R2B-09` in
  `records/watch-verb/watch-verb-critique-r2.md`.
- Correction input: Fable's joint critique at landed commit `7c77cfb6`;
  `WVJC-01` and `WVJC-02` are accepted for this correction round. Its five
  non-material lows remain accepted work for later slices, not silent folds.
- Status: slice 1 is implemented in this round. No new acting authority is
  implemented or implied. The only automatic responses visible today remain
  the already-landed breach-stop custodian and steward revival path.
- Revision rule: a response class stays absent or in marking mode until its
  real dispatch, intent, delivery, race, trial, and Law 2 seams are complete.
  No guarantee below is narrowed to fit an implementation shortcut.

Wido's direction remains the product intent: the system should recover what
it can safely recover and escalate what it cannot. Ruling L supplies the
ordering: heal first within lawful authority; alert when healing has failed or
no lawful automatic remedy exists. R-1 supplies the two-sided actor test;
R-21/R-22 forbid the dispatch hand from manufacturing or scoring the truth
that grants its own response power; R-11 favors one existing actor and one
base-action boundary; R-24 requires the weaknesses to be stated.

## 1. Facts that changed revision 2

These are code facts, not desired behavior.

1. The steward revival trigger is repository-wide and lineage-free.
   `internal/steward/openwork.go:23-62` treats a machine-owned claimed goal as
   open work, and `internal/steward/verdict.go:89-132` can produce `ActRevive`
   without reading a failed job or recovery lineage. `Intent` has no
   `recoveryRoot` (`internal/steward/intervene.go:19-41`). A failed original
   job and a failed recovery child can therefore enter the generic revival
   path today.
2. A goal can carry only its one claim-bound obligation
   (`internal/goal/file.go:46-48`). That record cannot also be a fleet-wide
   response-class registry. The live tree has no production caller of
   `Decide(EffectAuthorizeLaunch)`. Revision 2 named a type, not a store or a
   launch gate.
3. `newestUnrecoveredFailure` considers only failed records and never
   suppresses on a newer pending, running, completed, or returned job
   (`internal/steward/delivery.go:279-286`). The goal ledger lock is not shared
   by job terminal transitions or `memory/receipts.log` appends. A second
   launch-time scan would still race a landing.
4. `steward.Deliver` is side-effect-only: it returns an error or nil and
   persists no attempt (`internal/steward/notify.go:38-58`). Durable alert
   attempts are journaled around send in
   `internal/steward/alert_episode.go:324-361`, but the steward tick is their
   retry caller (`internal/steward/tick.go:267-296`). The supervision watcher
   records only a generic failed pass, and component evidence retains attempt
   history only for the supervision hook, not the watcher
   (`internal/steward/component_evidence.go:305-315`).
5. A budget cap is persisted as `status=timeout,error=budget-cap`
   (`internal/supervise/reaper.go:191-209`). Delivery's terminal set omits
   timeout and its unrecovered join accepts only failed, so revision 2's
   budget-cap trigger was unreachable.
6. `StageIntent` fixes the role to `steward-continuation`, permissions to
   `workspace`, and the brief to build mode
   (`internal/steward/stage.go:16-95`). The authorization shell imports only a
   subset of the intent and forces other choices
   (`scripts/agents/dispatch.sh:1175-1195`). It does not transport a goal,
   cap, worktree policy, destructive reach, review target, mission/stream,
   parent, source packet, or reasoning effort. It cannot express revision 2's
   recovery launch.
7. Adapter adjudication collapses a non-zero command result to
   `runtime_error` (`internal/adapter/adjudicate.go:195-203`). The outage
   classifier recognizes overload and framed HTTP 5xx evidence, not
   authentication, network, or native money-cap causes
   (`internal/outage/outage.go:175-205,235-261`). Those causes cannot be added
   to an acting class by prose.
8. Job records carry `mainId`, session and runtime provenance. Revision 2
   nevertheless allowed the same main seat that dispatched a target to label
   the target's trial outcome. That violates R-21/R-22 and the seat's
   dispatch/examination separation.
9. The current writer intentionally records a goal-less reservation as
   present JSON `null` for both `goalId` and `goalRevision`
   (`internal/dispatch/build.go:73-108,365-378`; the claim writer has the same
   convention). The public spelling `--goal none-explicit` is removed before
   internal dispatch (`cmd/metasystem/delegate.go:287-305`). An absent field,
   an empty string, and explicit null are therefore different facts. The
   corpus inspected in this joint worktree and the live m0 job directory did
   not contain the none-explicit-era specimen asserted by the brief; this
   revision does not invent one. The writer contract and fixtures establish
   the shape used below.

One more read-side trap matters: `steward.AlertEpisodes` takes a lock whose
read path creates `artifacts/agents/steward/alerts.flock`
(`internal/steward/alert_episode.go:66-86,167-175`). A zero-write command may
not call it.

## 2. Product shape and authority ladder

The product is one command and three deliberately separated rungs.

1. **Slice 1, read:** `metasystem watch` reads bytes already persisted by
   their owners and emits one typed snapshot. It probes nothing, refreshes
   nothing, creates no lock, and writes no reader state. This slice is built
   in the joint round.
2. **Future substrate and marking:** a later implementation may add the
   typed terminal cause, shared response lineage, serializable activity
   admission, complete recovery intent, durable alert outbox, action ledger,
   and per-class Law 2 registry specified in sections 5 through 8. Only after
   every prerequisite is live may `W-RECOVER` enter `OBSERVE`, where it writes
   `WOULD_ACT` records and cannot launch.
3. **Future power:** Wido may promote one class after reviewing its complete,
   independently adjudicated marking range. Promotion writes one class's
   complete Law 2 authorization record. The executable launch boundary still
   rechecks that record and all trigger evidence. Promotion is never inferred
   from sample counts or a prose ruling.

`W-HEAL` is not on the ladder. Untyped adapter failures are not on the
ladder. A goal-less job is visible and escalatable but not automatically
recoverable. Smaller honest coverage is the design choice.

## 3. Slice 1: the zero-write read surface

### 3.1 Commands and compatibility

- `metasystem watch` prints the human projection.
- `metasystem watch --json` prints the same snapshot as JSON.
- `--root <checkout>` selects a checkout for operators and fixtures; the
  default is `.`.
- `metasystem watch --job <id>` delegates to the existing
  `runJobWatchVerb` with its existing flags and exit meanings.
- `metasystem job watch --job <id>` remains registered and unchanged.

The top-level dispatcher owns the distinction. Any exact `--job` or
`--job=<id>` selects the old waiter. No `--job` selects the new read surface.

### 3.2 Closed output and stores

The JSON envelope is:

```text
Snapshot {
  schemaVersion: 1
  aggregate: HEALTHY | ATTENTION | UNKNOWN
  empty: boolean
  sections: [Section, ...]
}
Section {
  class: jobs | completed-rounds | census | health | delivery | alerts | intents | breach-routes
  store: string
  verdict: EMPTY | READABLE | DEGRADED
  items: [Item, ...]
}
Item {
  kind, id, verdict, evidence
  optional role, stage, goalId, goalField, problem, observedAt, pendingJobs
}
```

All eight sections appear in that order even when empty. `items` is never
null. Each item retains its producer's verdict vocabulary. The read package
does not invent a universal liveness vocabulary.

The exact joins are:

| Class | Persisted input | Read rule |
|---|---|---|
| jobs | every regular `artifacts/agents/jobs/*.json` | Require filename/job identity and a known job status. Preserve every historical record, including every terminal goal-less record. Inspect raw `goalId`: non-empty string=`BOUND`, JSON null=`NULL`, absent=`ABSENT`, empty string=`EMPTY_STRING`, other JSON=`INVALID`. The last three degrade the section; they are never silently treated as no goal. |
| completed-rounds | terminal-completed, goal-bound delegate job records plus `memory/receipts.log` | A completed record's positive `round` and `endedAt` are the durable accepted-return facts the job record actually carries. When `endedAt` is later than that goal's newest attributed landing receipt, emit `UNKNOWN-CONSUMPTION`. No job-record or canonical-return field persists whether the seat consumed the return, so this class never guesses consumed or unconsumed. A missing newest receipt does not satisfy the strict postdates predicate. |
| census | `artifacts/agents/supervision/last-census.json` | Decode the schema-version-2 census verdict and retain `SUCCESS` or `CENSUS-FAILED`. No census run or freshness computation occurs. |
| health | `artifacts/agents/steward/health.json` plus the configured steward tick cadence | Decode only the persisted `HealthVerdict`, including its aggregate and role verdicts. Derive its age from `observedAt` and the current UTC clock. Freshness is the steward convention of two tick intervals. A stale, absent, unreadable, or clock-regressed health record emits `health-freshness health-record dead`, names the record and age (`unknown` when no trustworthy timestamp exists), and can never summarize healthy. Do not invoke the health command, which is a producer and writer. |
| delivery | the `claimed-goal-delivery` role in persisted health plus every `artifacts/agents/steward/pending/*.json` notification | Show the role exactly as persisted and pending messages as `PENDING`. Do not rerun the claim/job/receipt join. If a readable health record omits this required role, the section is degraded. |
| alerts | every regular `artifacts/agents/steward/alerts/*.json` | Raw-read `AlertEpisode` JSON and show active/resolved/cleared stage plus the persisted transport result. Do not call `AlertEpisodes`, because its lock helper creates a file. |
| intents | every regular JSON record in `steward/intents`, `steward/consumed`, and `steward/cancelled` | Show `LIVE`, `CONSUMED`, `CANCELLED`, `LAUNCHED`, `REAPED`, or the persisted outcome according to the existing intent fields. |
| breach-routes | every regular `artifacts/agents/goal-stops/*.json` | Use strict `goal.ReadStopBatch`, which is read-only, and retain `OPEN`, `COMPLETE`, or `INDETERMINATE`. These stop batches are the durable route records; the in-memory tick result is not. |

An absent directory or ordinary singleton/file set is `EMPTY`, not an error.
The health singleton is the deliberate fail-safe exception: absent health is
`DEGRADED` with a typed `dead` freshness item, aggregate `ATTENTION`, and exit
1. Unreadable or malformed other persisted evidence yields an `UNREADABLE`
item, a `DEGRADED` section, aggregate `UNKNOWN` unless a known attention item
also exists, and exit 2.

The aggregate is only a deterministic projection over persisted vocabulary:

- `ATTENTION`: explicit-null goal-less job status `failed` or `timeout`;
  census other than `SUCCESS`; health freshness `dead`; health aggregate
  `unhealthy`; delivery role `dead`; pending notification; active alert not
  transport-submitted; or breach route `OPEN`/`INDETERMINATE`.
- `UNKNOWN`: any degraded section, unreadable item, corrupt/legacy-unknown
  goal field shape, completed-round `UNKNOWN-CONSUMPTION`, health `unknown`,
  or delivery `unknown`, provided no attention item exists.
- `HEALTHY`: otherwise. A fresh owning delivery verdict still determines the
  interpretation of a goal-bound failed job. A missing, stale, unreadable, or
  future-dated owning health record is itself known attention and therefore
  cannot hide that job behind a healthy summary.

Known attention outranks unknown, matching the health surface's rule that a
known failure must not disappear behind another unreadable source. Exit codes
are 0 healthy, 1 attention, 2 unknown.

### 3.3 Zero-write proof boundary

The implementation may use only `os.Stat`, `os.ReadDir`, `DirEntry.Info`,
`os.ReadFile`, JSON/receipt decoding, path sorting, the current UTC clock,
the read-only `steward.TickSeconds` configuration lookup, and the strict
read-only breach-stop reader. It may not use producer commands, file locks,
temporary files, probes, mtime classifiers, health refresh, census refresh,
alert helpers, open-work discovery, or a cache. `TickSeconds` executes only
`git config --get` and defaults to the steward's ten-minute cadence; it does
not refresh or persist a verdict.

The focused command test constructs all eight tracked classes, hashes every
path, mode, symlink target, and file byte, invokes the real top-level
dispatcher, and proves the tree hash is unchanged. A second test does the
same with absent health and proves the fail-safe exit. That is the slice's
executable zero-write claim.

## 4. Response taxonomy after redesign

| Class | Current status | Future typed trigger | Response and bound |
|---|---|---|---|
| `W-BREACH` | Already landed and acting; this design grants nothing | Existing `FindBreachStops` / `EnsureBreachStop` routes only: elapsed breach past grace, corrupt over-limit, existing fence, or indeterminate route | Existing fence and resumable stop batch; mandatory human resume. Displayed through `breach-routes`. |
| `W-REVIVE` | Already landed and acting | Existing steward `ActRevive`, but only after the shared resolver in 5.2 proves there is no terminal recovery candidate or consumed response root | Existing `steward-continuation`; the landed repository-wide maximum of three dry revivals remains. The new per-root bound can only narrow it, never replenish or partition the global counter. |
| `W-RECOVER` | **Deferred; no observe or acting code in slice 1** | Goal-bound, non-mission, root `implementer` job; status/cause exactly `failed/process-lost` or `timeout/budget-cap`; no returned result; unchanged goal claim revision; no breach; no newer job or landing activity; unconsumed response root; complete source artifacts and launch tuple | One fresh implementer recovery round, once ever for that response root. It inspects both source transcript and worktree and adopts intact work or redoes it. A dead recovery child consumes the root and routes to escalation. |
| `W-ESC` | Existing health/alert paths only; the job-specific additions in sections 7 and 8 are prerequisites | Unknown/degraded evidence, goal-less terminal failure, cause outside the two typed reaper causes, missing tuple/artifacts, admission race, consumed root, breaker, or any refusal | Durable incident/alert, no dispatch and no kill. |
| `W-HEAL` | Deferred outside this design | None | None. A future design needs a typed discriminator, proven repair, observe trial, and separate Law 2 record. |

The recovery class is intentionally limited to regular, root implementer jobs.
It does not recover code critics, design critics, verifiers, follow-ups,
missions, or jobs with a non-null reviews target. Those roles have distinct
closure and parent semantics that the landed steward seam cannot carry.

Kills remain out of scope. The reaper owns exact-PID process loss and timeout
wind-down. Neither the interactive reader nor a future classifier may kill.

## 5. Prerequisites before `W-RECOVER` may even observe

These prerequisites land together. Partial landing leaves the class absent,
not approximately safe.

### 5.1 Typed cause with reachable budget-cap

Add a closed job-record field `terminalCause` with only
`process-lost|budget-cap` in this class version. The reaper writes it in the
same `RecordCAS` patch that writes the terminal status and error:

- process loss: `status=failed`, `terminalCause=process-lost`;
- absolute cap: `status=timeout`, `terminalCause=budget-cap`.

The recovery join is new and typed; it does not call or copy
`newestUnrecoveredFailure`. Its terminal predicate includes both exact pairs.
Contradictory status/cause, a missing cause, or any other cause routes to
`W-ESC`. `native-budget-cap`, authentication, network, and capacity remain
deferred until byte-level per-adapter producer mappings and fixtures exist.
Outage marks never authorize a recovery.

Producer fixtures must prove both pairs enter marking and that generic
`runtime_error`, returned failures, and ambiguous evidence do not.

### 5.2 One resolver and one shared lineage ceiling

Add a response resolver at the existing `ActRevive` base-action boundary,
immediately before `StageIntent`, and invoke the same resolver again at intent
consumption. It orders facts:

1. live breach/fence -> `W-BREACH`, never dispatch;
2. eligible terminal implementer candidate -> `W-RECOVER` or its observe
   would-act, never generic revival;
3. consumed recovery root or failed recovery child -> `W-ESC`;
4. otherwise the existing `ActRevive` predicate may remain `W-REVIVE`;
5. unknown evidence -> `W-ESC`.

The shared root is `goal:<goalId>:claim-revision:<positive revision>`. It is
stamped on every new revival/recovery intent, action entry, and resulting job.
Pre-field active continuations for that goal/revision count as responses and
must terminalize before a recovery candidate is evaluated. All automatic
dispatches count against both (a) the landed repository-wide dry-revival
maximum of three and (b) a new maximum of three for this root. Neither counter
is reset or partitioned by the other. In addition, `W-RECOVER` has a once-ever
bit: once its intent is consumed, no later revive or recover is permitted for
that root. Therefore a dead recovery child can only escalate, and marking
mode cannot be bypassed by the old revival path.

While `W-RECOVER` is in `OBSERVE`, this precedence deliberately suppresses a
generic revival for the same candidate and records the would-refusal. That is
a safe narrowing of existing power and is part of the trial fixture; it is
not permission to launch recovery early.

### 5.3 Serializable newer-work and landing suppression

The current goal lock is insufficient. Before marking starts, add one
goal/revision activity owner under `internal/dispatch`, with a bounded flock
and a monotonically increasing activity generation. Every one of these
writers must use it:

- normal and recovery job reservation publication;
- goal-bound job terminal `RecordCAS` publication;
- the supported landing workflow before it begins a goal-attributed landing
  and when it appends the landing receipt;
- recovery admission and response-root consumption.

The landing workflow first publishes a `LANDING` lease under the lock, before
the Git landing can become visible. A legacy/direct receipt append that did
not originate from such a lease is outside the acting guarantee; the
promotion validator must prove all goal-attributed landing entry points use
the owner or refuse promotion.

Immediately before launch, under that lock, recovery admission re-reads the
goal claim revision and stop fence, all job records for that goal, the landing
lease and newest receipt, the source terminal pair, response-root records,
class authorization, and the activity generation captured by the would-act.
It refuses if any job newer than the source is pending-setup, pending,
running, completed, failed, cancelled, timeout, or returned; if any landing
lease/receipt is newer than the source; or if any fact changed. On success it
atomically consumes the response root and publishes the pending recovery
reservation before releasing the lock.

A landing workflow that encounters a pending recovery marks that owned
reservation cancelled before landing. If it encounters a running recovery it
uses the dispatcher's exact-job graceful cancellation and waits for the
terminal record before landing. No raw Git/receipt path may claim coverage.
This is the price of a serializable no-duplicate guarantee; until the shared
owner is universal, `W-RECOVER` stays powerless.

### 5.4 Complete recovery dispatch tuple

Do not generalize the existing `Intent`. Add a versioned
`RecoveryIntent`/authorization path whose digest covers every field below:

- class id/version, manifest digest, action id, response root;
- source job id, operation id, record digest, brief/prompt digest, transcript
  path/digest, worktree path plus existence verdict;
- goal id, exact claim revision, machine and claim epoch;
- role fixed to `implementer`, working mode fixed to `implement`, and
  `reviews=null`, `parentJob=null`, `mission=null`, `stream=null`;
- runtime, model, reasoning effort, cap minutes, permissions preset,
  destructive-reach class, worktree policy, and network choice copied exactly
  from the source record; none may be defaulted or re-resolved;
- recovery brief bytes generated from one versioned fixed template and their
  digest; the template instructs inspection of both transcript and worktree,
  adopt-or-redo, ordinary implementation return, and ordinary later review;
- reserved recovery job id and operation id.

Missing source bytes, a runtime/model no longer rostered, an unsupported
source tuple, or any null/empty required field is `W-ESC`, not a fallback.
`steward authorize-dispatch` must validate and emit this exact tuple.
`dispatch.sh` must import every field and reject extras or differences; it
must no longer force a value that contradicts the intent. The recovery job
record stores `recoveryOf` and `responseRoot` without pretending to be a
follow-up or resumed session.

The second resolver, activity admission, and Law 2 decision all run after
intent consumption and before reservation. Consumption alone never grants
launch.

## 6. Marking records, independent truth, and promotion

### 6.1 Closed class declaration and Law 2 store

The committed class declaration ships at:

`scripts/agents/watch-response-classes.json`

It follows the closed-manifest precedent of
`scripts/agents/landing-classes.json`. Schema version 1 contains exactly one
candidate row, `W-RECOVER`, with trigger version 1, effect
`authorize-governed-launch`, fixed recovery tuple version, trial minimum 10,
trial window 7 days, and false-alarm maximum 0. Unknown/duplicate classes,
effects, or versions make the loader refuse every response.

Per-machine mutable Law 2 records live at:

`artifacts/agents/steward/response-authorizations/W-RECOVER.json`

The record contains schema version, class/version, manifest SHA-256,
generation, and one `governance.GovernedObligation`. Initial installation
writes `OBSERVE`, `Effects=[authorize-governed-launch]`, and no authorization
tuple. A promoted `LIMITED` record must carry every field validated by
`ValidateAuthorizationCompleteness`: `AuthorizedBy`, `AuthorizedAt`,
`AuthorityOperation`, `AuthorizedEffects`, `ReviewPolicy`, and
`ReviewOutcome`, plus the same sole effect.

The only writer is a future `metasystem steward response-authorize` command.
For `LIMITED` it must call `humanauthority.Prove` from an enrolled,
agent-free terminal, verify the complete independently adjudicated trial
range and manifest digest, increment generation, and atomically replace the
one class record. OBSERVE/demotion is always allowed but still uses the same
typed writer. The watcher, steward runner, dispatcher, and delegates have no
write route to promotion.

The executable refusal is `steward authorize-dispatch`, after intent
consumption and inside the serializable activity admission. It loads the
committed manifest and exact per-machine record, verifies both digests and
versions, then calls
`GovernedObligation.Decide(EffectAuthorizeLaunch)`. Only `Apply=true` permits
reservation. `OBSERVE` yields `WouldRefuse`; missing, stale, incomplete, or
uncovered records refuse. The decision is repeated at the final reservation
boundary. A ruling row or prose alone can never make power reachable.

Fleet promotion means Wido runs the human-only writer separately on m0, m1,
m2, and m3 after reviewing one fleet trial range. A missing machine record
leaves that machine in observe.

### 6.2 Action ledger and trial truth

Each decision is immutable JSON at
`artifacts/agents/steward/watch-actions/<id>.json`. It records class/version,
machine, time, target job/goal/operation, target `mainId` and session,
response root, activity generation, exact trigger fields, every evidence path
and SHA-256, the complete proposed dispatch tuple/digest, and decision
`WOULD_ACT|REFUSED|ACT`. An act adds intent nonce and recovery job id; it does
not self-label success.

Adjudication is a separate immutable sidecar under
`artifacts/agents/steward/watch-adjudications/<action-id>.json`. The only
writer in this design is a human-only
`metasystem watch trial-adjudicate` command using `humanauthority.Prove`.
This removes the dispatching main seat, watcher, class author/operator,
target delegate, recovery delegate, and all reviewers from the scoring path.
The command records the target's dispatch provenance and refuses an
adjudicator identity matching it. Wido labels
`CORRECT_TARGET|FALSE_ALARM|UNADJUDICATABLE` with cited durable evidence.

Only correct/false labels count. An unadjudicated or unadjudicatable entry is
trial debt and appears on the read surface; it never helps promotion. The
minimum is seven elapsed days and ten adjudicated samples fleet-wide with
zero false alarms. A false alarm resets the window and count. No automatic
graduation exists: the command in 6.1 still requires Wido's separate human
act after review of the complete range, matching the two-bars pattern.

## 7. Durable escalation, including a dead runner and goal-less jobs

### 7.1 Runner-repair delivery

Revision 2's direct `Deliver` call is withdrawn. Before the seat relay can
end, the separately executing supervision watcher gets a dedicated retained
outbox:

`artifacts/agents/supervision/runner-repair-alerts/<episode-id>.json`

After the fifth consecutive failed runner repair, the watcher atomically
journals an episode and a `PENDING` attempt before calling `Deliver`. It then
atomically finishes that same attempt as `SUBMITTED` or `FAILED`; a failed
send never advances `lastSubmittedAt`. Every later watcher pass drains
`PENDING`/`FAILED` before attempting another repair, so retry does not depend
on the dead steward runner. After submission, one reminder may be journaled
every 60 failed repair passes. Successful runner repair marks the episode
`RESOLVED`; records are retained.

The episode itself, not generic `PASS_FAILED`, is the durable evidence trail.
Required fixtures cover: runner dead + repair fails + first delivery fails;
the file contains one completed failed attempt; the next watcher pass retries
the same episode and submits without a runner; watcher restart resumes a
journaled pending attempt; a successful repair resolves without deletion.

The interactive read command owns no daemon state and needs no watchman.

### 7.2 Goal-less delegate incidents

Slice 1 already enumerates every job record. Future escalation scans also
enumerate every terminal failed/timeout record, before any goal join. For the
current valid no-goal shape (`goalId:null`), they create a retained,
job-specific incident at:

`artifacts/agents/steward/job-incidents/<job-id>-<terminal-digest>.json`

The incident is keyed by job id plus terminal-record digest, remains open
until explicit human acknowledgment, and journals outward alert attempts. It
is not placed in aggregate health alert episodes, because a later unrelated
healthy pass would clear those episodes. The response is always `W-ESC`; no
goal means no claim revision, budget admission, or recovery authority.

Absent, empty-string, or invalid `goalId` shapes are degraded evidence. They
open a corrupt-job-record incident and never masquerade as supported
none-explicit jobs. Duplicate scans join the same digest and do not create a
second incident. The action/incident writer must not wait for a goal
projection, which closes the omission in revision 2.

## 8. Actor boundary and authority envelope

The steward remains the future mechanical detector because R-1's conflict
test still passes only after the controls above: it applies fixed typed rules,
does not examine merit, cannot close or accept the recovery, cannot score its
trial, and cannot promote itself. A new responder seat would duplicate the
intent, census, fence, and health machinery.

Future authority after per-machine promotion is only:

- one recovery implementer launch for one eligible response root;
- writes to the response intent, admission, action, and incident stores;
- no kill, no product edit, no review, no acceptance, no disposition, no
  mission mutation, no goal resume, no authority-record write.

Any unsupported fact becomes a named refusal or escalation. Error strings,
silence, mtime, a dirty worktree, and an LLM diagnosis never authorize action.

## 9. Migration and rollback

1. Land slice 1 now. It changes no authority and does not end the interim
   steward-watch relay. Rebuild/re-arm only where existing R-37-m3 permits a
   landed engine change to require it.
2. In a later reviewed slice, land all prerequisites in sections 5 through 7
   together, including the committed class manifest, per-machine OBSERVE
   record writer, universal activity-owner adoption, outbox, fixtures, and
   read-surface extensions. If any producer or landing path is not joined,
   the class remains absent and the relay continues.
3. Arm marking on all machines. Collect and independently adjudicate the full
   range. Marking suppression of overlapping `W-REVIVE` is part of the trial.
4. Wido may promote `W-RECOVER` per machine through the human-only command.
   The interim relay ends on a machine only after its durable runner-repair
   outbox and goal-less incident path are armed and verified; merely landing
   the read verb or entering observe is insufficient.

Rollback atomically demotes the class record to `OBSERVE`; the next launch
decision refuses. Existing recovery jobs finish or are handled by their
ordinary exact-job lifecycle. The read surface is deletable without state
loss because it owns none.

## 10. Self-grade under R-24

Confidence is high for slice 1: it is a small, closed, zero-write join over
eight typed classes, with byte-tree invariance tests and no process probes.
The one clock-derived decision uses the steward's existing two-tick freshness
convention and fails toward a typed dead record. The other key implementation
risk was the alert helper's creating lock; the raw reader avoids it.

Confidence is deliberately low-to-moderate for future `W-RECOVER`, which is
why it is deferred. The weakest prerequisite is universal serialization of
landing activity: a direct Git/receipt path outside the owner invalidates the
no-duplicate guarantee. The second weakness is operational value: a single
recovery may spend real budget only for two reaper-proven causes and one role.
That small coverage is preferable to inventing adapter classifiers or
generalizing an intent whose missing fields carry authority.

Reject promotion if the activity owner is bypassable, the source tuple needs
a default, any trial truth is written or selected by the dispatching seat,
the runner alert retry still depends on the runner, a goal-less record can be
omitted, or the executable launch check can be bypassed. Those are failed
guarantees, not acceptable implementation shortcuts.

## 11. Joint-round disposition of critique round 2

All nine findings are accepted. Each is resolved by in-place redesign, not
rebuttal.

| Finding | Disposition | Resolution in revision 3 | Code fact that forced it |
|---|---|---|---|
| `WV-R2B-01` | **JOINT-ROUND REDESIGN** | Shared goal/revision response root, total precedence at both existing `ActRevive` boundaries, common three-response ceiling, once-ever recovery bit, and dead recovery child -> `W-ESC`; observe suppresses overlapping revival | `openwork.go:23-62`, `verdict.go:89-132`, and `intervene.go:19-41` show repository-wide revival with no recovery lineage (§1.1, §5.2) |
| `WV-R2B-02` | **JOINT-ROUND REDESIGN** | Shipped closed manifest at `scripts/agents/watch-response-classes.json`; per-machine Law 2 record at `artifacts/agents/steward/response-authorizations/W-RECOVER.json`; human-only writer; exact `Decide(EffectAuthorizeLaunch)` check after intent consumption and at reservation | A goal has one claim-bound obligation and no production launch-effect decision exists (§1.2, §6.1) |
| `WV-R2B-03` | **JOINT-ROUND REDESIGN** | Future action is blocked on one activity owner adopted by reservations, terminal CAS, landing leases/receipts, and recovery admission; complete predicate reruns under its lock and every newer status suppresses | Current failed-only delivery join ignores newer work, and goal/record/receipt writers share no lock (§1.3, §5.3) |
| `WV-R2B-04` | **JOINT-ROUND REDESIGN** | Withdraw direct unjournaled delivery; watcher-owned durable pending/failed/submitted attempt store is retried every pass independently of dead runner | `Deliver` writes nothing; alert/pending retry callers are runner-owned; watcher `PASS_FAILED` is not attempt history (§1.4, §7.1) |
| `WV-R2B-05` | **JOINT-ROUND REDESIGN** | New typed recovery join explicitly admits `timeout/budget-cap` and `failed/process-lost`; it does not depend on delivery's failed-only helper | Reaper writes budget-cap as timeout while delivery omits timeout (§1.5, §5.1) |
| `WV-R2B-06` | **JOINT-ROUND REDESIGN** | New versioned recovery intent has a closed root-implementer-only tuple; all authority-bearing fields are digested and imported exactly; any missing field escalates | Existing staging hard-codes one role/permissions and the shell drops or forces goal, cap, mode, reach, review, mission and lineage choices (§1.6, §5.4) |
| `WV-R2B-07` | **JOINT-ROUND REDESIGN** | Acting cause set narrowed to the two exact reaper-produced causes; native cap/auth/network/capacity are deferred until exact producers and fixtures exist | Adapter returns generic runtime error and outage recognizes only overload/5xx (§1.7, §5.1) |
| `WV-R2B-08` | **JOINT-ROUND REDESIGN** | Trial sidecar writer is human-only through enrolled-terminal proof; target main/session provenance is recorded and matching dispatch provenance is refused; machine main seats and delegates cannot score | Job records retain dispatch provenance, while revision 2 allowed that dispatch hand to label its target (§1.8, §6.2) |
| `WV-R2B-09` | **JOINT-ROUND REDESIGN** | Slice 1 enumerates every job and distinguishes null/absent/empty/invalid raw shapes; future action scan creates retained job incidents before goal joins and never auto-recovers goal-less work | Writer emits explicit null for no goal; typed accessor collapses shapes; revision 2's goal join omitted terminal goal-less records (§1.9, §3.2, §7.2) |

Revision 2's round-1 disposition remains historical evidence. Where revision
2's folds conflict with this joint redesign, this revision controls: the read
surface no longer reruns producer liveness, W-RECOVER no longer claims broad adapter
causes or the generic intent, the direct-delivery claim is withdrawn, and the
class authorization is no longer attached to a target goal.

## 12. Joint-critique correction dispositions

The correction round changes only the two material findings. The five lows
remain accepted and explicitly ride later slices.

| Finding | Disposition | Resolution in revision 4 | Code fact that forced it |
|---|---|---|---|
| `WVJC-01` | **JOINT-ROUND REDESIGN** | Health freshness is now a typed item. At two configured steward ticks, or when `health.json` is absent, unreadable, or future-dated, the item is `dead`; its message names `artifacts/agents/steward/health.json`, prints the observed timestamp when trustworthy, and prints its age. Known dead wins the aggregate and exits 1, including over a persisted goal-bound failed job. | `internal/steward/health.go:114-123` persists `HealthVerdict.ObservedAt`; `internal/steward/runner.go:51-61` owns the tick cadence; `internal/steward/health.go:581-612` establishes the two-tick, `age >= window` fail-safe freshness convention. Slice 1 had decoded the record without applying any of those persisted freshness facts. |
| `WVJC-02` | **JOINT-ROUND REDESIGN** | `completed-rounds` is the eighth closed class. A goal-bound completed delegate record with a positive round and `endedAt` newer than the goal's newest `memory/receipts.log` entry emits `UNKNOWN-CONSUMPTION`. A newer/equal receipt suppresses the candidate. No receipt does not satisfy “postdates.” | `internal/dispatch/record.go:44-52,460-461` makes `completed` terminal and stamps `endedAt`; `internal/steward/delivery.go:147-169` identifies the newest goal receipt. Search of the job writer and canonical return schema finds no persisted return-consumption marker; the `consumedAt` fields in `internal/dispatch/launch_capability.go:130-131,266-267` belong only to launch authority. The read surface therefore cannot truthfully distinguish consumed from unconsumed. |
| `WVJC-03` | **ACCEPTED LOW — RIDES LATER SLICES** | No correction-round change. Move the job-status vocabulary to its declared dispatch owner before a later watch extension. | `internal/dispatch/record.go:49-52` forbids consumers from re-declaring the vocabulary; the current read remains fail-safe as `UNKNOWN` on drift. |
| `WVJC-04` | **ACCEPTED LOW — RIDES LATER SLICES** | No correction-round change. A later slice must either narrow the proof claim to persistent state or add transient-write instrumentation. | `cmd/metasystem/watch_verb_test.go` hashes paths, modes, symlink targets, and bytes, but cannot observe create/delete or a byte-identical rewrite. |
| `WVJC-05` | **ACCEPTED LOW — RIDES LATER SLICES** | No correction-round code change. A later documentation slice will make the top-level `--job` waiter's registration writes more prominent. | `internal/dispatch/watch.go:23-28` registers and removes waiter state; the no-`--job` snapshot remains the zero-write surface. |
| `WVJC-06` | **ACCEPTED LOW — RIDES LATER SLICES** | The three future acting-slice open points stay prerequisites to close before any slice-2 brief: universal landing-owner proof, the record-plus-return predicate, and the persisted expanded permission envelope. | Current dispatch records have no `returned` status and persist `permissions.requested`, not a preset name; no universal landing-owner validator exists. |
| `WVJC-07` | **ACCEPTED LOW — RIDES LATER SLICES** | No correction-round code change. A later reader revision must retain readable siblings while typing an irregular entry as degraded. | `internal/watch/watch.go` currently stops a directory scan when `jsonFiles` encounters an irregular entry. Its result is fail-safe `UNKNOWN`, not silent healthy. |
