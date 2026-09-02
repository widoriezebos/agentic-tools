# Design: turn-verdict hardening — a seat cannot end its turn on ready work

Goal: turn-verdict-hardening (plans/goals/turn-verdict-hardening.md, revision 4,
priority-1). Author: implementer delegate tvh-design-2 under dispatch by
m0b+main-1788250419-3170380-8a1fb3, 2026-09-02; revision 2 by delegate
tvh-design-r2 the same day, worktree at commit 19c61d24.
Revision: 2 — closes the nine material findings of
records/misc/turn-verdict-hardening-critique-r1.md (each cited as TVH-R1-… at
the paragraph that closes it) and carries Wido's two words of ruling R-47-m0b
(memory/rulings.md). Revision 1's requirements stand: the eleven findings of
records/misc/seat-stop-analysis-critique-r1.md (cited by id) and the three
specimens of records/misc/seat-stop-analysis.md.
Wido's order, verbatim: "we need machinery (not you, your behaviour, yourself but
deterministic Go code) that should make this impossible or at least give us the
highest chance of this never happening again."

Every seam cited here was read in this worktree; line numbers are at 19c61d24.

## 0. The object, restated against the code

The gate exists and the specimens walked through it. At the Stop event
`scripts/agents/supervision-hook.sh:274-276` calls `report turn-verdict`; the
decision is `internal/goal/turnverdict.go` (`TurnVerdict` 164-191, `decideRuns`
197-259, `decide` 326-448); the block text is `internal/report/stopblock.go:11-13`;
the scanner is `internal/report/scan.go` with `openwork.go`, `scanjobs.go`,
`runningwork.go`. Four deterministic escapes, each observed in code:

| Escape | Where | What it does |
| --- | --- | --- |
| block-once | `turnverdict.go:365-372` (open work per `OpenWorkSignature`), `393-398` (`BlockedGoalRevisions`), `417-425` (`BlockedQueueDigests`), `430-435` (`BlockedFreeDigests`), `248-257` (`BlockedUnwatchedDigests`); `stopblock.go:13` promises "does not repeat" | a second unchanged Stop passes |
| existential INFLIGHT | `turnverdict.go:355-361`: any `scan.Busy` item (any job, gate, mission, run in this checkout) suppresses every goal clause | one unrelated job launders idleness on every ready goal |
| fail-open | hook `26-31` (engine missing → exit 0), `65` (no git toplevel → exit 0), `62-63` (session-env failure → exit 0), `306-320` (verdict I/O error → `systemMessage`, never `decision:block`); verdict `384-387` (Unreadable → display only), `439-440` (degraded → display only); and every `set -e` exit before line 321 (§3.0) | every unknown becomes an allowed exit |
| relay-minted HUMANSTOP | `internal/humanauthority/authority.go:195-198`: the temporary word pair "cannot verify who supplied the words" | there is no HUMANSTOP today at all; R-47-m0b now admits the relay as a human-ratified exception (§5) |

Identity facts the design relies on: ownership of a claim is the pair
machine+lineage (`internal/goal/verbs.go:176-181` `ownPair`; a second lineage
on the machine is refused by name at `447-449`); the quota is one claim per
machine tree-wide, members of one arc under one claimant counting once
(`internal/goal/validate.go:250-281`, enforced at commit by `ValidateCommit`);
a claim binds only with a positive authenticated claim epoch (`bindClaim`,
`verbs.go:113-115`), which the command layer takes from the checkout lease
only for the holder, a named human, or a human-classified caller
(`cmd/metasystem/goalsync_mutations.go:59-69`; an advisor gets epoch 0); a
queued goal claims only with a complete stored or supplied budget
(`verbs.go:163-174`); the dispatcher's side-effect-free admission evaluates
claimed goals of one machine+lineage pair (`internal/dispatch/admission.go:37-107`);
release, park and done all pass through `clearClaimBinding`
(`verbs.go:128-137`, called at `664`, `770`, `862`), which refuses a
breach-stopped goal — only `goal resume`, a human-only verb
(`internal/goal/stop.go:355-358`), clears a `StopFence`; job records carry
`goalId`, `goalRevision`, `claimEpoch`, `pid`, `pidStartedAt` and the
platform-exact start identity (`pidStartedAtExactMicro` on Darwin,
`pidStartTicks` plus `bootId` on Linux: `internal/dispatch/ownership.go:84-97`,
required on every new record by `validateOwnershipPatch` `152-155`) but no owner
lineage (`jobrecord.go:49, 83, 86`); run records carry `goalId`,
`governed.goalRevision`, `ownerLineage`, `claimEpoch`, `pid`, `pidStartedAt`,
`pidStartTicks`, `bootId`, `mainId`, `status` (`internal/run/run.go:114-134,
160-187`) and the scanner already reads them (`internal/report/scan.go:195-240`);
the announcement carries `ownerLineage` and is deleted at retirement
(`internal/lease/classify.go:21-40`, `verbs.go:214-234`); the projection reads
offline and only banners staleness past thirty minutes
(`internal/goal/project.go:26-31, 66-77`); git subprocesses in the goal package
run unbounded (`internal/goal/txn.go:71-98` `goalGit`, `genesis.go:92` `gitIn`),
while the repository already owns a process-group-killing bound
(`internal/boundedexec/boundedexec.go:84-113`) and a portable nanosecond clock
verb (`cmd/metasystem/slug.go:55-58` `util now-ns`); the three shipped hook
configurations — `scripts/enforcement/codex-hooks.json`, `devin-hooks.json`,
`claude-code-hooks.json` — each give the Stop hook five seconds, and only Claude
declares a live self-check (`internal/runtimes/runtimes.go:161-231`, `SelfCheck`
at 226; `cmd/metasystem/hooks.go:33-36` refuses the others). Specimen 3's goal
`account-provenance` was queued with a complete stored budget (read:
`plans/goals/account-provenance.md:9,14`); the ledger history shows the seat had
released every claim of its pair before that Stop (`missionrunner-terminate-flake.md:21`
at 01:02Z, `failed-job-attention.md:19` at 01:39Z) and claimed
`account-provenance` only after the human's question (`:15`, 05:48Z).

## 1. Closure 1 — READY, and no block-once for it

### 1.1 The seat

A seat is the pair `(machine, lineage)`. `machine` is
`goal.ResolveMachine(root)` (`internal/goal/actor.go:21-28`, the enrolled
nickname). `lineage` is the announcement's `ownerLineage`, or its `mainId` when
`ownerLineage` is absent — the same default the lease applies
(`internal/lease/claim_test.go:78-79`, `lease.go:131-140`
`announcementLineage`). The verdict verb resolves the seat ITSELF: the hook
passes `--caller-pid "$identity_pid"` (the pid it already resolved at
`supervision-hook.sh:109-129`) beside the existing `--main-id`, and the verb
calls `lease.ClassifyVerbAt(root, metasystemRoot, callerPid)`
(`internal/lease/verbs.go:294-322`), which yields in one read the `mainId`,
`class`, `holder` flag, the announcement (with `ownerLineage`) and the lease's
`ClaimEpoch` (`verbs.go:305-309`). The seat record is:

```go
type Seat struct {
    Machine, Lineage, MainId string
    Holder     bool  // ClassifyResult.Holder
    ClaimEpoch int64 // lease ClaimEpoch when Holder, else 0 — exactly the value
                     // goalsync_mutations.go:59-69 gives `goal claim` from this caller
}
```

An empty `--caller-pid`, a classification error, a class other than MAIN, or
an empty lineage is IDENTITY UNKNOWN (§3, F7).

### 1.2 The predicate, as a function

New package `internal/readywork` (imports `goal`, `dispatch`, `run`,
`identity`; none imports it, so no cycle; `report` imports it and fills a new
typed fact `ScanResult.Ready`, keeping the settled contract "the scanner fills
ScanResult, the verdict decides", `turnverdict.go:16-19`).

```go
// Frontier decides, side-effect free, over the ACCEPTED projection.
func Frontier(root string, seat Seat, now time.Time) (Frontier, error)

type Frontier struct {
    State          string      // "ready" | "none" | "unknown"
    Ready          []ReadyItem // sorted: claimed-admissible, queued-claimable, held-releasable
    WaitingOnHuman []string    // fenced claims of this pair: reported, never blocked (§1.2.1)
    Reasons        []string    // one line per excluded candidate, for the display
}
type ReadyItem struct {
    GoalId   string
    Revision uint64 // f.Revision of the accepted file (the projected revision)
    Binding  uint64 // f.Claimed.Revision when claimed, else 0 (the dispatch binding)
    Clause   string // "claimed-admissible" | "queued-claimable" | "held-releasable"
    Move     string // the one lawful move, rendered from verbs the engine accepts
}
```

Inputs: `goal.NewWorld(root)` must be true (the legacy single-file world keeps
today's contract unchanged — §8 residual). `proj := goal.Project(endpoint,
false, now)` — the same read `convertedGoalFacts` makes (`turnverdict.go:476`);
under §4 the verdict's bounded fetch runs before this read. Any error from any
call below → `State: "unknown"` with the error text; the predicate never
guesses.

#### 1.2.0 ClaimAdmission — the proof a claim would confirm (closes TVH-R1-CLAIM-ADMISSION-OMITS-AUTHORITY-AND-REPLAY)

Sol's finding stands: revision 1's extraction omitted the claim epoch
(`bindClaim`, `verbs.go:113-115`) and sat before the replay check
(`opidLanded`, `verbs.go:440-442`). Corrected signature and placement:

```go
// ClaimRefusal is one named rule Claim would refuse on, in Mutate's order.
type ClaimRefusal struct{ Rule, Detail string }
// Rule ∈ not-live | not-queued | claimed-by-this-pair | claimed-by-this-machine |
//        claimed-elsewhere | pinned-elsewhere | blocked | no-stored-budget |
//        invalid-budget | norm-approval | no-claim-epoch | machine-quota
func (r *ClaimRefusal) Error() string
func (r *ClaimRefusal) QuotaOnly() bool { return r.Rule == "machine-quota" }

// ClaimAdmission is side-effect free and reads only its arguments.
func ClaimAdmission(root string, t *TreeGoals, id string, actor Actor,
    supplied *Budget, approvedRef string, claimEpoch int64) *ClaimRefusal

// MachineQuotaAllows is validate.go:250-281 as a predicate over ONE candidate:
// true when the machine holds no claim, or every claim it holds shares the
// candidate's non-empty arc.
func MachineQuotaAllows(t *TreeGoals, machine string, candidate *GoalFile) bool
```

Rule order inside `ClaimAdmission` is the order of `claimRequest.Mutate` at
`verbs.go:436-470`, then the two rules Mutate reaches later or elsewhere:
not-live (`436-439`) → state (`443-454`: a claimed goal maps to
claimed-by-this-pair / claimed-by-this-machine / claimed-elsewhere by the same
tests; any other non-queued state is not-queued) → pinned-elsewhere (`455-457`)
→ blocked (`458-462`) → `budgetForClaim(f, supplied)` (`463-466`; a nil
`supplied` requires the complete STORED budget, `163-174`) → `goalNormApproval`
(`467-470`) → no-claim-epoch (`bindClaim`, `113-115`: `claimEpoch < 1`) →
machine-quota (`MachineQuotaAllows`; today enforced only when `ValidateCommit`
rejects the commit).

`claimRequest.Mutate` is rewritten to this exact order:

1. `t := loadTree(root, tip)`; `f, exists := t.Live[id]`; `!exists` → the
   not-live error (unchanged text, `438`).
2. `opidLanded(f, r)` → `AlreadyApplied{}` (`440-442`). The replay check
   stays FIRST: a replay of a landed claim returns AlreadyApplied even when
   admission would now refuse — Sol's replay case.
3. `f.State == StateClaimed` → the three transaction outcomes verbatim
   (`NothingToDo`, the same-machine refusal, `LostToCompetitor`, `443-451`);
   they are competitor outcomes, not admission rules, and stay in Mutate.
4. `if refusal := ClaimAdmission(root, t, id, r.Actor, supplied, r.ApprovedRef, r.ClaimEpoch); refusal != nil { return nil, refusal }`.
5. The mutation as today (`471-478`); `bindClaim`'s own epoch test stays as a
   defensive duplicate that can no longer fire.

So the two call sites now agree on every input that decides admission: READY
passes the seat's `ClaimEpoch` (§1.1, the same value the claim command would
carry from the same caller), `supplied = nil` and `approvedRef = ""` (READY is
"would `goal claim <id>` with no flags succeed for this seat right now"), and
the replay input differs only where it must (READY has no opid, so no replay).
Consequence for F8 (§3): an advisor is NOT without READY "by construction";
it is without READY because `ClaimAdmission` refuses `no-claim-epoch` for its
zero epoch — the same refusal `goal claim` would print. Revision 1's F8 text is
withdrawn.

Test `TestClaimAdmissionAgreesWithClaim` is table-driven over every rule above
plus the epoch and quota rules and the replay case (§10).

#### 1.2.1 The three clauses, evaluated over `proj.Tree.Live`

| Clause | Holds when | Calls | Adds (does not reuse) |
| --- | --- | --- | --- |
| R1 claimed-admissible | `f.State == claimed` ∧ `goal.OwnPair(f.Claimed, machine, lineage)` ∧ `seat.Holder` ∧ `f.StopFence == nil` ∧ the goal is NOT among `dispatch.EvaluateGoalAdmission(root, lineage, now).Refusals` | `EvaluateGoalAdmission` (fence, BUDGET_UNKNOWN, elapsed/attempt/minute/active breaches, all for this pair) | `goal.OwnPair(c *ClaimRecord, machine, lineage string) bool` exported; `ownPair` becomes a one-line wrapper, so there is ONE pair rule. `seat.Holder` is required because dispatch binds the lease holder's claim epoch into every reservation; whether the dispatcher additionally refuses a non-holder was not traced (§9 gap 5), so the safe default is that a non-holder pair's claim is displayed ("claim held by a session that is not the checkout holder; run `metasystem up`") and never READY |
| R2 queued-claimable | `f.State == queued` ∧ `ClaimAdmission(root, proj.Tree, id, Actor{machine, lineage}, nil, "", seat.ClaimEpoch) == nil` | `ClaimAdmission` (§1.2.0) | nothing |
| R3 held-releasable | the pair holds a claimed `H` with `H.StopFence == nil` that R1 excludes because `EvaluateGoalAdmission` refuses it with `Breaches ≠ ∅` and `Unknown == nil` (a budget-exhausted, UNFENCED claim) ∧ ∃ queued `g` for which `ClaimAdmission` fails ONLY on `machine-quota` (`QuotaOnly()`) | the same two functions | the move, rendered from verbs the engine accepts for the pair's own unfenced claim: `goal park <H> --because "budget exhausted: <breach names>"` when `H.Origin != OriginHuman` (`verbs.go:842-862`: own pair, no fence → `clearClaimBinding` passes), else `goal release <H>` (`verbs.go:653-666`); then `goal claim <g>` |

Closes TVH-R1-R3-NAMES-ILLEGAL-EXIT: a FENCED claim (`StopFence != nil`) is
excluded from every clause. Release, park and done refuse it at
`clearClaimBinding` (`verbs.go:129-131`), steal refuses it (`1229-1231`), and
only `goal resume` — human-only with observed or relayed authority
(`stop.go:355-358`) — clears the fence. Such a claim is listed in
`Frontier.WaitingOnHuman` as "`<H>` is breach-stopped by `<StopID>`; only
`goal resume` (a human) clears it", and every queued goal that
`ClaimAdmission` refuses only on the quota is listed beneath it as waiting
behind the fence. WaitingOnHuman is reported, never blocked (§7 step 7). The
command `goal park <held> --then <g>` is withdrawn: the converted parser
declares no `--then` flag (`goalsync_mutations.go:104-159`) and routes park
straight to `goal.Park` (`232-241`). Named residual (§8): releasing or parking
an exhausted claim and re-claiming it starts a fresh claim revision whose
budget window is the engine's rule, not this design's; the verdict reports what
the engine admits.

Excluded by construction and listed in `Reasons`: queued goals without a
complete stored budget (R-47-m0b word 2: READY means claimable now with the
budget already on the ledger; such a goal yields the once-notice of §1.5, never
a block — the 4h/8h rule of R-44-m0b stays a conduct rule the seat applies
before the gate sees the goal), goals pinned elsewhere, goals behind open
blockers, another pair's claims on this machine (seat B is never told seat A's
goal is READY: SSA-R1-READY-OWNERSHIP-SCOPE), any claim on another machine, and
a non-holder's own claim (R1).

Labels: `goal.Next`'s `requiredLabels` is a dispatcher preference; READY passes
none. `Next` itself (`project.go:90-124`) is not called: it filters by machine,
not pair, and skips budget, quota and epoch.

### 1.3 No block-once for READY

In `decide`, a new first-class clause replaces the goal ladder's `ok` and
`queued-only` blocks for the converted world:

```
READY(seat) ∧ ¬RELEVANT-INFLIGHT(seat) ∧ ¬HUMANSTOP-CONSUMED → ShouldBlock = true,
    BlockSource = "ready-work", every Stop, no session memory consulted or written.
```

The display names the first `ReadyItem` and its `Move`, byte-verbatim in the
block reason as today (`supervision-hook.sh:290-297`). `stopblock.go:11-13`
loses the sentence "This refusal does not repeat for the same work." and reads:
"The goal ledger holds work this seat can lawfully advance and nothing relevant
is in flight. Claim it, dispatch it, or park or release an exhausted claim;
only a recorded human stop ends this turn otherwise." `BlockedGoalRevisions`
and `BlockedQueueDigests` are no longer written for READY items; they persist
in the state file only for the two non-ready notices (§1.5) and the legacy
world.

Plan open work (`scan.Open`, the `plans/*.md` streams) also loses its
signature memory: `Open ≠ ∅ ∧ no in-flight job record` blocks every Stop
(SSA-R1-BLOCK-ONCE-BYPASS lists it). `OpenWorkSignature` stays in the Verdict
JSON for the hook's display only. Residual: a plan's `Next step` and `Waiting
on the human` fields are seat-editable text — the legacy stream is not hardened
here (§8).

### 1.4 How a turn lawfully ends under READY

Exactly two ways: (a) start RELEVANT flight (§2) — a dispatched job or a
governed run on the READY goal whose liveness the verdict can prove; (b) a
HUMANSTOP consumed by this very Stop (§5). There is no third way: not an
explanation in the continuation, not a plan edit, not a second attempt. Slow
flight is never converted into failure (R-35): a live relevant job allows the
exit however long it runs.

### 1.5 What keeps session memory

Only the notices that are not READY: the goal-free staleness block
(`BlockedFreeDigests`, `turnverdict.go:427-435`), the queue-change notice for a
claimed session (`ObservedQueueDigest`, `399-410`, now display-only when READY
already blocks), the once-notice for a queued goal WITHOUT a stored budget
(R-47-m0b word 2: the `queued-only` block-once at `411-425` is retained
exactly for that goal set and its digest), and the unwatched-work block
(`BlockedUnwatchedDigests`, `decideRuns`) — the last is not a READY clause and
its escape (arm the printed watch) is machinery-verifiable through
`WaiterLive`; it is unchanged and named in §8.

## 2. Closure 2 — RELEVANT INFLIGHT

`scan.Busy` no longer suppresses anything. It remains the "STILL WORKING"
display. Relevance is computed by `readywork.Relevant(root, seat Seat, item
ReadyItem, prober identity.Prober) (RelevantFlight, bool)` over BOTH record
kinds of this world: job records (`<root>/artifacts/agents/jobs/*.json`, the
files `scan.go:104-146` reads) and run records (`run.Store.List()`,
`internal/run/waiter.go:270-282`, the files `scan.go:195-240` reads). With the
root fix a mapped worktree reads the primary's records.

### 2.1 Job records

A job record J is RELEVANT to `ReadyItem` G iff all of:

| Test | Source | Why |
| --- | --- | --- |
| `J.goalId == G.GoalId` | `dispatch.JobRecord.GoalID()` (`jobrecord.go:49`) | the join Sol required (SSA-R1-UNRELATED-INFLIGHT-LAUNDERS-IDLENESS) |
| `J.goalRevision == G.Binding` | `JobRecord.GoalRevision()` (`jobrecord.go:83`); `G.Binding` is `f.Claimed.Revision`, the value `EvaluateGoalRevisionAdmission` bound at dispatch (`admission.go:124-131`) | a job on a superseded binding is not progress on the current one |
| `G.Clause == "claimed-admissible"` | — | only a claimed goal of this pair can have lawful flight; a queued or held goal has none by construction, so R2 and R3 are never excused by flight |
| `!dispatch.TerminalStatus(J.status)` | `record.go:45-52` | terminal records are history |
| LIVE NOW, exact: `ref, ok := dispatch.IdentityRefOf(J)` (export of `identityRefFromObject`, `ownership.go:99-116`) ∧ `ok` ∧ `ref.NativeExact()` (`identity.go:101-110`) ∧ `identity.AliveRefComparison(prober, ref)` returns `(Alive, mode)` with `mode` the native exact mode; OR `run.LiveWaiter(root, prober, "job", J.jobId, J.mainId, WaiterTarget{StartedAt: J.startedAt})` (`waiter.go:195-209`, already exact) | `attest.go:131` names the custody keys; `pending`/`running` is a status, not liveness (SSA-R1-INFLIGHT-PROOF-MISSING); a record with no pid and no live waiter is NEVER-STARTED, not flight |

Closes TVH-R1-JOB-LIVENESS-DOWNGRADES-EXACT-IDENTITY: the direct-process
branch consumes the record's FULL native identity — `pidStartTicks` plus
`bootId` on Linux, `pidStartedAtExactMicro` on Darwin — never a reconstructed
`Ref{pid, pidStartedAt}`. A record whose exact shape is missing, partial or
foreign to this platform (`Mode() == CompareInvalid` or `!NativeExact()`) is
UNKNOWN, and UNKNOWN is not alive (`identity.go:5-6`): every new ownership
write carries the exact shape (`validateOwnershipPatch`, `ownership.go:152-155`),
so a seconds-only record is pre-rule history and "never grants signal
authority" (`ownership.go:20-22`). A reused pid within the recorded second
therefore compares Dead (`identity.go:212-215`), not Alive.

### 2.2 Run records (closes TVH-R1-SLICE1-IGNORES-GOAL-BOUND-GOVERNED-RUNS)

Revision 1 deferred runs as an unread gap; the record was read in this pass
(`run.go:160-187`) and carries every join key. The run join is in SLICE 1. A
run record R is RELEVANT to `ReadyItem` G iff all of:

| Test | Source | Why |
| --- | --- | --- |
| `R.GoalId == G.GoalId` | `run.Record.GoalId` (`run.go:180`) | the goal join |
| `R.Governed != nil ∧ R.Governed.GoalRevision == G.Binding` | `GovernedAttempt.GoalRevision` (`run.go:115`), written by governed admission | a goal-named run WITHOUT a governed attempt carries no binding proof: it is displayed as "unbound run `<id>` names `<goal>`" and is NOT flight — the same rule as a job on a superseded binding |
| `R.OwnerLineage != nil ∧ *R.OwnerLineage == seat.Lineage` | `run.go:177` (runs carry the lineage; jobs do not, hence §2.3) | a foreign lineage's run on this goal is a stranger's work |
| `G.Clause == "claimed-admissible"` | — | as for jobs |
| `R.Status ∈ {launching, running}` | `run.StatusLaunching`, `StatusRunning` (`scan.go:234`) | `draining` marks the leader's death (`run.go:198-199`): wind-down is not progress |
| LIVE NOW, exact: `identity.AliveRefComparison(prober, Ref{Pid, StartedAtSec: PidStartedAt, StartTicks: PidStartTicks, BootID})` (`run.go:168-171`) Alive in the native exact mode; OR `run.LiveWaiter(root, prober, "run", R.RunId, mainId, WaiterTarget{Generation, LaunchNonce})` (`scan.go:230-231`) | today's reader builds a legacy seconds ref (`scan.go:219`) although the record carries ticks and boot id — fixed here for the verdict's `RunFact.ProbeState` as well |

`goal.RunFact` (`turnverdict.go:61-74`) gains `GoalId string`, `GoalRevision
uint64` (0 when ungoverned), `OwnerLineage string`, `ClaimEpoch int64`, and
`ProbeState` is computed with the full ref; `runFactsFor` fills them from the
fields it already decodes.

Platform residual, dated: `run.Record` carries no Darwin microsecond field
(`run.go:168-171`; the writers at `internal/run/verbs.go:140, 223, 321` record
ticks and boot id only), so on Darwin a run's direct-process branch is UNKNOWN
until slice 2 adds `pidStartedAtExactMicro` to the record at those three write
sites; until then a Darwin seat's governed run is relevant through its live
waiter only (the `run watch` the seat arms). Linux, the recorded fleet
platform, is exact from slice 1.

### 2.3 Ownership of jobs

Not required for jobs: `J.mainId == this main`. The goal's claim pair is the
ownership proof; a predecessor process of the same lineage that dispatched on
this pair's claim is this seat's flight, which is exactly the resumed-lineage
case Sol said the announcement cannot prove after retirement. `Alive` means
the probe answered alive in the exact mode; `Unknown` (probe failure, legacy
shape) is not alive — fail closed.

Harness monitor: a live waiter registered for the relevant job or run counts
as liveness even when the child's pid is unreadable (a monitor is a process
the seat armed and the verdict can probe).

Recorded blind spots, stated as residuals, not closed here (KI-34):
- Jobs whose records live in another clone's `artifacts/` on the same machine
  (a second checkout of the same ledger): invisible; a seat there is another
  seat, and the quota keeps its claims disjoint, so the only loss is a
  same-lineage job dispatched from a foreign clone — refused, not laundered.
- Non-job, non-run processes (a detached script without a record): invisible,
  so the verdict blocks; the lawful move is to run it as a governed run.
- Gate runs and mission runners (`scan.go:38-54`): not joined to a goal; they
  stay in the display and never suppress a READY block. A synchronous gate
  inside a tool call never meets a Stop event, so the cost is nil there.

## 3. Closure 3 — FAIL CLOSED

Sol refuted fail-open (SSA-R1-FAIL-OPEN-IS-A-BYPASS). Decision: every outcome
the hook cannot vouch for is a BLOCK, delivered as the one shape the contract
already proves (`{"decision":"block","reason":...}` on stdout, exit 0), never
an exit code and never a `systemMessage`. Proven absence is not a guess: the
two rows marked ALLOW carry positive proof from the engine. The precedent is
the steward's `Decide` (`internal/steward/verdict.go:91-94`: "cannot be read;
refusing to guess").

Closes TVH-R1-HUMANSTOP-CANNOT-RESCUE-DECISION-OWNER-FAILURES: revision 1's
universal "unless HUMANSTOP" was false. Every row now carries a CLASS:

- **Class A — HUMANSTOP can rescue.** The verdict verb runs to its decision
  step, where the compare-and-consume of §5.3 executes under the marker's own
  lock. A consumed marker allows the Stop; otherwise the row blocks.
- **Class B — the decision owner must be repaired.** No verdict verb reaches
  the marker (engine missing, world unknown, verb unavailable or unparsable,
  pre-verdict shell exit). The block reason names the machinery-owned
  recovery: the repair command the seat runs in its next turn (`metasystem up
  --repo <path>`, `cmd/metasystem/up.go:70`, or its restricted form `up
  --recover-only --if-down`, `up.go:82-83`; rebuilding the engine with `go build
  -o bin/metasystem ./cmd/metasystem`, the command the suite itself uses,
  `scripts/validate-metasystem.sh:124`; fixing the hook configuration named by
  `hooks check`), and the steward's stalled-idle escalation is the
  detection-after-the-fact backstop: a live seat with no progress becomes
  `stalled-idle` with `ActNotify` (`verdict.go:100-108`), the notification is
  queued (`internal/steward/tick.go:215-226`) and delivered through `steward
  pending` at the next session start (`supervision-hook.sh:332-333`) and the
  alert channel of R-45-m0b. Class B never consults a marker — a marker set in
  advance cannot authorize a Stop the machinery cannot judge — and this is
  stated in every class-B reason.

Wedge risk, stated: a seat blocked on corrupt state burns turns until the
state is repaired (class B) or a human sets HUMANSTOP (class A); the steward
escalation surfaces the loop; this is the chosen direction because the
alternative was the bypass the specimens took.

Sequencing: the wrong-root fix (`plans/supervision-hook-root-design.md`, goal
supervision-hook-wrong-root) MUST land before slice 2 (the hook slice). Under
this table an unresolved identity is a BLOCK, so the hook rows on a wrong-root
fleet would wedge every seat on every Stop. Its silent-exit rows are
re-dispositioned below; its skew and missing-engine lines keep their text,
prefixed into the block reason.

### 3.0 The hook's structure: the trap is the first thing (closes TVH-R1-FAIL-CLOSED-TABLE-OMITS-PREVERDICT-SHELL-EXITS)

`supervision-hook.sh` runs `set -euo pipefail` (line 2) and, before any
verdict, performs unguarded `cd`/`pwd -P` (23-24), `mktemp` and `cat` (42-44),
`mkdir -p` (263-264), a second `mktemp` (273), and command substitutions in
assignments that `set -e` treats as failures (278-280, 297-304). Each is a
silent exit today. The rewritten Stop path has this fixed shape, in this
order, and an implementer follows it line for line:

```bash
#!/usr/bin/env bash
set -euo pipefail
runtime=${1:-}; event=${2:-}
emitted=0; fail_line=0
on_err() { fail_line=$1; }
on_exit() {
  rm -f "${payload:-}" "${response_file:-}" "${verdict_stderr:-}" 2>/dev/null || true
  if [[ "$event" == stop && $emitted -ne 1 ]]; then
    command printf '{"decision":"block","reason":"turn-exit gate did not reach a verdict (supervision-hook.sh line %s). The metasystem could not judge this turn end, so it refuses it. Repair the hook or engine (metasystem up --repo <path>; rebuild bin/metasystem), then end the turn again. No human stop applies here: the machinery that would honour it did not run."}\n' "$fail_line"
    exit 0
  fi
}
trap 'on_err $LINENO' ERR
trap on_exit EXIT
```

Rules: (1) the two traps are armed before the runtime-name and event checks
(today's lines 18-19), so a Stop event with a malformed runtime argument emits
the block instead of `exit 2`; an unrecognised EVENT still exits 2 (the hook
cannot know it is a Stop) — that is hook drift, caught by `hooks check`; (2)
the trap body uses only builtins and `command printf`; the reason is a fixed
literal plus a line number (digits only), so no escaping question exists; (3)
`exit 0` inside the EXIT trap overrides the failing status, so exit codes are
never the delivery channel; (4) `emit_stop_payload` sets `emitted=1`
immediately after its successful `printf` to stdout and nowhere else; (5) the
existing `trap 'rm -f "$payload"' EXIT` (line 43) is deleted — it would replace
the fallback trap; cleanup lives in `on_exit`; (6) no `exit 0` remains on a
Stop path before emission (today's 30, 62, 65 become block rows below).

Every pre-verdict operation maps to a row:

| # | Operation (today's line) | Outcome under the trap |
| --- | --- | --- |
| P1 | `script_dir`/`harness_root` resolution fails (23-24) | ERR trap → block, line named |
| P2 | `runtime list` fails or runtime unregistered (32-40) | explicit block JSON via `command printf` (F5), exit 0 |
| P3 | payload `mktemp` or `cat` fails (42-44) | ERR trap → block |
| P4 | `read_payload` (47) cannot run the engine | already `|| true`; empty cwd falls to F6/F2 |
| P5 | session-env query failure (62-63) | block (F6) |
| P6 | `git rev-parse --show-toplevel` fails (65) | block (F2) |
| P7 | `turn_key`/`hook-attempt` failure (86-101) | rides the reason (F9); never exits |
| P8 | `mkdir -p "$supervision_dir"` fails (264) | ERR trap → block |
| P9 | `mktemp` for verdict stderr fails (273) | ERR trap → block |
| P10 | verdict `json get` field reads fail (278-280) | ERR trap → block (F10) |
| P11 | `report stop-block` / `json object` response construction fails (297-304) | ERR trap → block |
| P12 | `emit_stop_payload` cannot print to stdout (196-201) | residual F19 |

The two `mktemp` calls and `evidence-gc` keep their `|| true` where they have
it; a row's block is the trap's, not a bespoke branch.

### 3.1 The outcome table

| # | Condition | Detected where | Class | Hook decision and recovery named in the reason |
| --- | --- | --- | --- | --- |
| F1 | engine missing at the resolved world | hook, literal test (26-31) | B | BLOCK via literal `printf` JSON: rebuild `bin/metasystem` (`go build -o bin/metasystem ./cmd/metasystem`) |
| F2 | world identification fails (git query, parse, common-dir shape, mapping, shape check) | hook (root design Decision 2 rows 1-6, were silent exit 0) | B | BLOCK: "the hook cannot identify its world"; `metasystem up --repo <path>` |
| F3 | `path state-root` exit 1: the engine proves an ungoverned installation | engine verb | — | ALLOW with a visible line (proven absence, not a guess) |
| F4 | `path state-root` other failure or empty (engine/hook skew) | hook | B | BLOCK: skew line as reason; rebuild or re-adopt |
| F5 | runtime registry query fails or runtime not registered | hook (32-40, today exit rc / exit 2) | B | BLOCK JSON; exit 0 |
| F6 | runtime session-env lookup fails | deleted by the root fix (cwd no longer participates); until then | B | BLOCK |
| F7 | identity unknown: no runtime ancestor and no recorded main, `lease classify` empty or erroring, class not MAIN, or lineage empty | verb (§1.1) | B | BLOCK: "the hook cannot tell whose turn this is; run `metasystem up`" (exactly what the wrong root produced on every fleet seat) |
| F8 | caller classified advisor (MAIN, not holder) | verb (`ClassifyResult.Holder == false`) | A | the verdict runs in full: READY is empty by `ClaimAdmission`'s `no-claim-epoch` refusal and R1's holder rule (§1.2), so the outcome is the no-READY path (F16 applies); the OWNED-ELSEWHERE line rides the display (removes today's early exit at 232-247, which bypassed open-work display) |
| F9 | `steward hook-attempt` fails (turn evidence) | hook (84-104) | — | evidence failure rides the reason; alone it never decides (attempt evidence is the freshness trail, not the decision input) |
| F10 | `report turn-verdict` exits nonzero, empty stdout, or unparsable JSON, or `schemaVersion` the hook does not know | hook (306-320, today `systemMessage`) | B | BLOCK: "turn-verdict unavailable: <last stderr line>"; the verb's stderr names its own failing phase |
| F11 | verdict-state flock timeout (`withLock`, `goalverbs.go:96-118`, 2 s) or state-file write failure (`saveVerdictState`) | verb | A | the verb no longer exits nonzero for these: it returns `ShouldBlock = true`, `BlockSource = "state-unavailable"`, having run the marker step under the marker's OWN lock (§5.3); the state file holds only non-ready memory, so losing it costs an extra notice, never an allowed exit |
| F12 | ledger `degraded` (machine unenrolled, endpoint, projection error, unreadable tree, legacy parse problems) | `convertedGoalFacts`/`legacyGoalFacts` | A | `ShouldBlock = true`, `BlockSource = "degraded"` (was display only, `439-440`) |
| F13 | ledger `absent` (pre-adoption, no baseline) | `legacyGoalFacts:504-508` | — | ALLOW, advisory (proven: no ledger was ever adopted here) |
| F14 | `scan.Unreadable` or `scan.RunUnreadable` non-empty | `decide:384-387`, `decideRuns` | A | BLOCK, `BlockSource = "unreadable"` (was display only) |
| F15 | `Frontier.State == "unknown"` (READY could not be computed) | readywork | A | BLOCK |
| F16 | freshness unproven (§4) and `Frontier.State == "none"` | verdict | A | BLOCK, `BlockSource = "stale-board"`; recovery: the network returns and the next Stop's fetch succeeds, or the ledger is in local sync mode (§4) |
| F17 | Stop budget: the verdict's own deadline exceeded at a phase boundary | verb, `--deadline-ms` (§3.2) | A | verb exits 0 with `ShouldBlock = true`, `BlockSource = "deadline"`, the phase named; the marker step still runs (a single bounded file read); the hook emits immediately |
| F18 | Stop budget: the runtime kills the hook before emission | runtime | — | RESIDUAL: the runtime's default applies (allow). Made improbable by §3.2 (every step bounded, sum below the budget); recorded, not closed |
| F19 | emission failure (`printf` to stdout fails) | hook (196-201) | — | RESIDUAL: `hook-complete --outcome EMISSION_FAILED` is recorded; the steward's hook-freshness sees it; the runtime's default applies |
| F20 | HUMANSTOP marker unreadable or malformed | verdict | — | treated as absent: normal rules (a broken marker authorizes nothing) |
| F21 | verdict state file malformed | `loadVerdictState:645-647` resets today | — | unchanged: the state holds only non-ready memory now, so a reset can only cause an extra notice, never an allowed exit |
| P1–P11 | pre-verdict shell exits (§3.0) | hook trap | B | BLOCK via the EXIT trap, line named |

### 3.2 The Stop budget (closes TVH-R1-STOP-DEADLINE-DOES-NOT-BOUND-EMISSION)

Sol's three refutations are accepted: a `context.WithDeadline` cancels nothing
in `report.Scan`, `Project`, `FetchAdvance` or their git children because none
takes a context (`goal.go:543` runs the scan before `TurnVerdict`;
`fetchadvance.go:30`, `project.go:31`, `txn.go:71` take none); `date +%s%N` is
GNU-only; and ceremonies moved behind the verdict can still be killed before
the runtime reads the emission. Decisions:

(a) **Declared budget.** `runtimes.Declaration` gains `StopHookBudgetSec int`
(codex 20, devin 20, claude 20, fake 5); `metasystem runtime stop-budget
<name>` prints it; all THREE shipped files move their Stop `timeout` to 20
(§6 owns the per-runtime check).

(b) **Portable clock.** The hook reads `"$ms" util now-ns`
(`cmd/metasystem/slug.go:55-58`, Go `time.Now().UnixNano()`, identical on
Linux and Darwin) once at entry — after the engine test, since without an
engine F1 emits at once and no budget arithmetic is needed — and computes
`elapsed_ms=$(( ($("$ms" util now-ns) - entry_ns) / 1000000 ))` in bash
64-bit integer arithmetic. `date` is not used for timing.

(c) **Every step bounded, and the sum below the budget.** A new verb
`metasystem util run-bounded --deadline-ms N -- <argv...>` runs the child
through `boundedexec.Run` (`internal/boundedexec/boundedexec.go:84-113`:
`Setpgid`, SIGKILL to the process group on expiry, bounded reap) and exits 124
on expiry. The hook wraps each ceremony in it with a fixed cap, in this order,
each skipped with its fixed "HEALTH unknown" line when its cap exceeds the
remaining budget or when it returns 124:

| Step | Cap (ms) | On expiry |
| --- | --- | --- |
| `up` (148-156) | 5000 | "supervision arming unfinished (bounded)"; `up` is idempotent and re-runs next Stop — a kill mid-way is what today's 5 s runtime timeout already does to it |
| `health --hook-preview` (161) | 2000 | the fixed unknown line (163) |
| `steward digest-pending` (166) | 1500 | "NARRATOR DIGEST unavailable (bounded)" |
| `supervise watchdog-report` (256) | 1500 | empty digest |
| `evidence-gc` (265) | 1000 | already `|| true` |
| `report turn-verdict` | reserve: `min(8000, 20000×0.8 − elapsed − 1500)`; never below 3000 — if less remains, the hook emits F17 itself without calling the verb | F17 |
| emission | 1500 reserved | — |

Worst case: the ceremonies consume their full 11 000 ms, leaving the verdict
`min(8 000, 16 000 − 11 000 − 1 500) = 3 500` ms, above its 3 000 ms floor;
11 000 + 3 500 + the 1 500 ms emission reserve is 16 000 ms, 80% of the 20 s
budget. The block is therefore emitted before the runtime's timeout by
construction, without any ceremony running after emission. Any residual is
F18.

(d) **The verb bounds itself.** `report turn-verdict --deadline-ms N` computes
a deadline and runs phases in this order, checking the deadline at each phase
boundary and returning F17 (phase named) when exceeded: identity (§1.1, one
lease read) → bounded fetch (§4) → projection → scan → READY → relevance →
marker (§5.3) → state file under `withLock` → marshal. Every git child in this
path becomes bounded: `goalGit` and `gitIn` gain context-taking variants
(`goalGitContext(ctx, root, extraEnv, args...)`, `gitInContext`) built on
`exec.CommandContext` with the same process-group kill as `boundedexec` and a
`WaitDelay` of 500 ms; `FetchAdvanceContext(ctx, e)`, `ProjectContext(ctx, e,
fetchFirst, now)` and `loadTreeContext` are added, and the existing functions
become `context.Background()` wrappers, so no other caller changes.
`report.Scan` spawns no subprocess (the only `exec` in the package is
`frontier.go:158`, outside the Stop path) and is file reads plus probes; it
moves INSIDE the verb's phase sequence (`goal.go:543` today runs it before
`TurnVerdict`) so its cost is charged to the deadline, with the cooperative
check before and after it. The marker step is one bounded file read and one
rename and always runs, even under F17.

## 4. Closure 4 — FRESHNESS (closes TVH-R1-FRESH-CURSOR-IS-NOT-A-CURRENTNESS-WITNESS)

The verdict reads the accepted tree offline (`turnverdict.go:476`,
SSA-R1-STALE-BOARD-ALLOWS-EXIT). Sol's refutation of the ten-minute cursor is
accepted: a cursor written at tip A is a recent-history banner; another machine
can publish at tip B one second later and the cursor still says FRESH. No time
window stands in for a fetch. The cursor and `FreshWindow` are withdrawn.

Freshness gates only the ALLOW path:

```
allow-on-no-READY requires FRESH; READY from a stale board still blocks
(a remotely claimed goal fails at claim time with a named refusal — that is
the lawful move being refused, not an idle exit).
```

FRESH holds in THIS verdict when exactly one of:
- the projection's root record says `SyncMode == SyncLocal` (`project.go:52-63`):
  the explicit local-sync mode — a single-machine ledger has no remote to be
  stale against; or
- the verdict's own bounded fetch succeeded: phase 2 of §3.2(d) runs
  `FetchAdvanceContext(ctx, e)` with `ctx` bounded to `min(4000 ms, 40% of the
  verb's remaining deadline)`; both outcomes of `FetchAdvance` — "already at
  the canonical tip" (`fetchadvance.go:51-53`) and advanced (`78-81`) — prove
  the accepted tip equals the canonical tip at that instant, and the projection
  that follows reads it.

Otherwise (fetch refused, timed out, killed, or skipped because under 1000 ms
remained) FRESHNESS is UNKNOWN → F16 BLOCK: "the board's freshness is unproven
(fetch: <error or timeout>); the next turn end fetches again, or run `goal list
--fetch`". The fetch runs before the projection so READY is computed over the
fresh tree (a remote claim removes a goal; a remote open adds one). The residual
that no fetch can close is the interval between the fetch's instant and the
decision — one verdict's execution, not ten minutes — and it is named, not
softened. An offline remote-mode machine with no READY is blocked until the
network returns or a human sets HUMANSTOP (class A); its lawful in-turn action
is to retry `goal list --fetch`. Wedge stated: a network that never answers
within the fetch bound blocks every no-READY Stop; the escape is the human's
word or the ledger's local mode, and the steward's stalled-idle escalation
surfaces the loop.

## 5. Closure 5 — HUMANSTOP

The one lawful exception is the human saying stop. Today it lives nowhere a
program can read.

### 5.1 The marker

Path: `<root>/artifacts/agents/humanstop/<machine>+<lineage>.json`, one per
seat, written atomically. Fields:

| Field | Meaning |
| --- | --- |
| `schemaVersion` | 1 |
| `world` | `goal.ExistingLedgerIdentity(root)` (`actor.go`), so a marker cannot be carried to another ledger |
| `machine`, `lineage` | the seat it stops (the same pair as §1.1) |
| `runtimeSession` | the normalized session id it binds to, or `""` for any session of this lineage |
| `directive` | the human's words, ≥ 3 words, verbatim |
| `setAt`, `expiresAt` | RFC 3339 UTC; TTL ≤ 24 h, default 8 h |
| `nonce` | 16 random bytes hex |
| `provenance` | `{kind: "enrolled-terminal" \| "temporary-relay", terminalRef, terminalGeneration, checkedAt}` for the enrolled form (copied from the proof), or `{kind: "temporary-relay", relayedBy: "<machine>+<lineage>" of the invoking seat, recordedWord: <the directive, verbatim>, reviewBy: <YYYY-MM-DD>, checkedAt, ruling: "R-47-m0b"}` for the relay form |
| `consumed` | `null`, or `{at, session, turnKey, attemptSeq}` |

### 5.2 Who may set it (carries R-47-m0b word 1; closes §9 ask 1)

Verb: `metasystem goal humanstop --directive "<words>" [--ttl 8h] [--session
<id>] [--review-by YYYY-MM-DD]`. Authority is
`humanauthority.ProveOrTemporaryGoalAuthority(root, invokerPID, KernelReader{},
directive, reviewBy, now)` (`authority.go:228-237`): the enrolled-terminal
proof (`Prove`, `ValidFor(root)`, `authority.go:89-111`: enrolled terminal
ancestry, no agent runtime on the chain, the installed signature set) wins
when it holds; when it fails and `--review-by` is present, the temporary relay
proof is minted from the directive as the recorded word (`TemporaryGoalProof`,
`199-222`: ≥ 3 words, a real review date not in the past and inside the
governance horizon, `159-180`). Wido's word (R-47-m0b, decided): a RELAYED
human word carried through the temporary-human-word path MAY mint the
single-use marker. The path's residual is exactly Sol's
(SSA-R1-HUMANSTOP-RELAY-LAUNDERING; `authority.go:195-198`: it "cannot verify
who supplied the words") and it is recorded here as a HUMAN-RATIFIED EXCEPTION,
not a hole: the marker records the relay provenance verbatim (`provenance.kind
= temporary-relay`, `relayedBy`, `recordedWord`, `reviewBy`), the audit line in
the Stop display names it as relayed — "HUMAN STOP (relayed by
<machine>+<lineage>, recorded word: "<directive>", review by <date>)" — and the
marker's history line in `hooks.log` carries the same text. The hook's own
`lease classify` HUMAN class (`classify.go:369-377`: "no recognised ancestor
and a controlling terminal") remains too weak for minting — a pseudo-terminal
allocated by an agent satisfies it — and is NOT accepted. Redirecting work is
not HUMANSTOP: `goal park`, `steal`, `set-pin`, `open` are the human's redirect
verbs and leave READY to be recomputed.

### 5.3 Compare-and-consume, bound to one Stop

At the verb's marker phase (§3.2(d)), under the marker directory's OWN lock
(`humanstop/.lock`, flock with the 5 s bound of `withWaiterLock`,
`waiter.go:66-88`) — independent of the verdict-state flock so that F11 cannot
prevent consumption — and only when the decision so far is a class-A block
(READY, open work, F12, F14, F15, F16, F17, F11):

1. Read the marker for `(machine, lineage)`. Absent, unreadable, wrong `world`,
   wrong pair, `runtimeSession` set and ≠ this session, `now ≥ expiresAt`, or
   `consumed ≠ null` → no HUMANSTOP; an expired or foreign marker is named in
   the display, never consumed.
2. Otherwise write `consumed = {at: now, session, turnKey, attemptSeq}` by
   atomic rename FIRST (the hook passes the `turn_key` it already computes at
   `supervision-hook.sh:86` as `--turn-key`, and `hook_attempt_seq` as
   `--attempt-seq`).
3. Then decide `ShouldBlock = false`, `BlockSource = null`, display
   "HUMAN STOP (<setAt>): <directive>" or the relayed form of §5.2.

A Stop that would be allowed anyway never consumes. The lock serializes
concurrent Stop calls, so exactly one consumes
(SSA-R1-HUMANSTOP-CONSUMPTION-RACE); the second sees `consumed` and falls
through to the normal rules. If the process dies between step 2 and emission,
the marker is spent and the turn was not allowed: the human sets another
(fail-closed direction). Nothing consumes at session start: the Stop decision
is the only boundary. A consumed marker is retained for the audit trail and
pruned after 30 days with the session state. Class-B rows never reach this
phase (§3).

## 6. The Stop hook is not exclusive or mandatory (closes TVH-R1-RUNTIME-HOOK-CHECK-OMITS-TWO-SUPPORTED-RUNTIMES)

The shipped universe is three real runtimes plus the fixture runtime
(`runtimes.go:161-231`); "both JSON files" in revision 1 was wrong. Owned by
this item, per runtime:

| Runtime | Shipped config | Stop timeout | Live self-check at `up` | Residual |
| --- | --- | --- | --- | --- |
| claude | `claude-code-hooks.json` | 5 → 20 | `hooks check --runtime claude <live settings> <shipped>` as today (`SelfCheck` declared, `runtimes.go:226`; `hooks.go:37`), plus the budget check below; drift rides every Stop display as `HOOK_DRIFT: <detail>` | the first Stop entry (the receipt check) declares no timeout and is a competing hook; user-disabled hooks |
| codex | `codex-hooks.json` | 5 → 20 | NO live self-check exists: `Declaration.SelfCheck` is nil and `hooks check` refuses (`hooks.go:33-36`). `up` runs only the budget check on the SHIPPED file and prints the honest line `HOOK_CHECK codex: live self-check undeclared; shipped Stop budget 20 s verified` | live delivery "declared, observation pending" (`docs/design/turn-verdict-delivery-contract.md:44-47`); project-hook trust |
| devin | `devin-hooks.json` | 5 → 20 | as codex: `HOOK_CHECK devin: live self-check undeclared; shipped Stop budget 20 s verified` | as codex; whether Devin honours the per-hook `timeout` field is unobserved |
| fake | none (`ShippedEnforcementConfig` empty) | declaration 5 | none | fixture only |

The budget check is a new engine verb `metasystem hooks budget --runtime R
<shipped hooks>` that needs no `SelfCheck`: it asserts every Stop entry's
`timeout` in the shipped file equals `Declaration.StopHookBudgetSec`, and the
suite's conformance test runs it for every runtime with a shipped config so
the three files cannot drift from the registry. Version compatibility: the hook
passes `--hook-schema 2`; the verdict returns `schemaVersion 2`; a mismatch
either way is F10. The steward's stalled-idle escalation stays the
detection-after-the-fact backstop behind this prevention gate
(SSA-R1-STOP-HOOK-NOT-MANDATORY-OR-EXCLUSIVE).

## 7. The new precedence ladder (converted world)

| Order | Condition | Class | Outcome |
| --- | --- | --- | --- |
| 1 | any class-B unknown (F1, F2, F4–F7, F10, P-rows) | B | BLOCK; no marker consulted; the reason names the repair |
| 2 | any class-A unknown (F11, F12, F14, F15, F17) | A | BLOCK — unless step 3 consumes |
| 3 | HUMANSTOP compare-and-consume succeeds against a class-A block | A | ALLOW; display the directive with its provenance |
| 4 | READY ∧ ¬RELEVANT | A | BLOCK `ready-work`, every time (step 3 applies) |
| 5 | plan `Open ≠ ∅` ∧ no in-flight job record | A | BLOCK `open-work`, every time (step 3 applies) |
| 6 | unwatched work (unchanged block-once) | A | BLOCK `unwatched-work` once per digest |
| 7 | READY ∧ RELEVANT | — | ALLOW; "STILL WORKING: <job or run> on <goal>@<binding>" |
| 8 | no READY ∧ ¬FRESH | A | BLOCK `stale-board` (F16; step 3 applies) |
| 9 | no READY, FRESH | — | WaitingOnHuman lines (fenced claims of this pair, §1.2.1), non-ready notices with session memory (queue change, goal-free staleness, unbudgeted queue once-notice), Busy display, then the all-clear naming what was checked |

`decideRuns` warnings and green lines compose into the display as today.

## 8. Residuals (honest list)

Legacy single-file world keeps today's block-once ladder; plan-stream fields
are seat-editable text; gate and mission activity never excuses READY; a
goal-named run without a governed attempt is not flight; Darwin run records
carry no microsecond identity until slice 2 (waiter-only relevance there);
F18/F19 runtime-side allow on a killed or emission-failed hook; hooks disabled
or untrusted; Codex and Devin have no live self-check and unobserved live Stop
delivery; a foreign clone's job records; an offline remote-mode machine blocks
until synced, local-mode, or human-stopped; the one-verdict race between fetch
and decision; releasing or parking an exhausted claim and re-claiming it starts
a fresh budget window by the engine's rule; a fenced claim leaves the seat with
no ledger move until a human resumes (reported as WaitingOnHuman, by design);
the relay-minted HUMANSTOP cannot verify its speaker (human-ratified, R-47-m0b).

## 9. Open asks and gaps (not filled)

1. CLOSED by R-47-m0b word 1: the relayed word may mint HUMANSTOP (§5.2).
2. CLOSED by R-47-m0b word 2: stored budget only; an unbudgeted queued goal is
   a one-time notice (§1.2.1, §1.5).
3. CLOSED by reading: run records carry the goal binding (`run.go:114-134,
   160-187`); the run join is in slice 1 (§2.2).
4. GAP: whether `goalNormApproval` can refuse a claim with an empty approved
   ref was not traced; `ClaimAdmission` includes it verbatim by extraction, so
   READY and claim agree either way, but the R2 false-READY rate is unknown
   until the extraction test runs.
5. GAP: whether the dispatcher refuses a non-holder session outright was not
   traced (`dispatch_verbs.go:987` only reports `holder`); R1 requires
   `seat.Holder` as the safe default (§1.2.1). If dispatch admits a non-holder
   pair's own claim, R1 is loosened by removing that conjunct — one line, one
   test.
6. GAP (per-runtime, honest): Devin's honouring of the per-hook `timeout`
   field and Codex's live Stop delivery are unobserved (§6); the design names
   them as residuals rather than checks.

## 10. Slices and tests

"240 reserved minutes" is the goal's budget `reservedJobMinutesLimit=240` with
`attemptLimit=10` (`plans/goals/turn-verdict-hardening.md:9`): the sum of
`capMin` over one slice's dispatches. Chain shape per slice: implementer cap
120 + code critic 40 + one correction 40 + re-critique 40 = 240, four
attempts; a second correction round means the slice is re-cut, not
over-reserved. Fable code critique per slice; land with `--chain`.

Recorded precedent (read from the primary checkout's job records,
`artifacts/agents/jobs/*.json`, 2026-09-01/02, this lane): 27 completed
implementer jobs ran 1–18 minutes wall clock (median 8) against caps of 20–120;
the largest computed diff read was 207 added lines in 2 minutes
(`implementer-0d40e4f087fbb016d455fd35`); the two 120-cap jobs ran 8 and 9
minutes; design critics ran 2–23 minutes against 40–60; code critics 3–6
against 20. Builder-minute estimates below assume the SLOW end of that record
(about 15 added lines per builder minute including tests) so the numbers are
conservative, and each slice's implementer estimate is held under 90 minutes
so a correction round fits the 120 cap. Revision 1's three slices did not fit
once the run join moved into slice 1 (its estimate below is 85 minutes before
the hook work); the build is re-cut into FOUR slices. Slice 1 alone refuses all
three specimens.

| Slice | Content and work breakdown (builder minutes) | Go tests (new) | Existing tests, new expectation |
| --- | --- | --- | --- |
| 1 — specimen refusal (est. 85; cap 120) | `ClaimAdmission` + `ClaimRefusal` + `MachineQuotaAllows` + Mutate reorder + `OwnPair` export (~120 lines, 12); `readywork.Frontier` R1/R2/R3/WaitingOnHuman (~220 lines, 15); `readywork.Relevant` over jobs AND runs with exact identity, `dispatch.IdentityRefOf` export, `RunFact` fields, `scan.go` full-ref fix (~180 lines, 14); verdict: `--caller-pid`/`--turn-key`/`--attempt-seq` flags, seat via `ClassifyVerbAt`, ladder §7 steps 2–9 without marker, no block-once for READY/open-work, F7/F12/F14/F15 verb-side, stopblock text (~180 lines, 14); hook: pass `--caller-pid`, F10 → block (~15 lines, 3); tests (~600 lines, 27) | `TestClaimAdmissionAgreesWithClaim` (table-driven over every `Rule`, plus epoch 0 and the quota with the arc exception); `TestClaimReplayReturnsAlreadyAppliedBeforeAdmission`; `TestClaimAdmissionRefusesZeroClaimEpoch`; `TestReadyClaimedAdmissibleForThisPairOnly`; `TestReadyRequiresHolder`; `TestReadyQueuedClaimableRequiresStoredBudgetAndFreeQuota`; `TestReadyHeldReleasableNamesParkOrReleaseByOrigin`; `TestReadyExcludesFencedClaimAsWaitingOnHuman` (fence set → no READY, WaitingOnHuman named, Stop not blocked on that ground); `TestReadyExcludesOtherPairOnSameMachine`; `TestRelevantJobJoinsGoalAndBinding`; `TestRelevantJobRequiresNativeExactIdentity` (legacy seconds-only record → not relevant; reused pid same second → Dead); `TestRelevantJobRequiresLiveProbeOrLiveWaiter`; `TestRelevantRunJoinsGoalRevisionAndLineage`; `TestRelevantRunRequiresLaunchingOrRunning`; `TestRelevantUngovernedGoalNamedRunIsNotFlight`; `TestRelevantIgnoresSupersededBinding`; `TestRelevantIgnoresMainIdMismatchOnOwnClaim`; `TestReadyBlocksEveryStopWithoutMemory` (five Stops, five blocks); `TestOpenPlanWorkBlocksEveryStop`; `TestBusyMissionDoesNotExcuseReady`; `TestUnreadableBlocks`; `TestDegradedBlocks`; `TestFrontierUnknownBlocks`; `TestIdentityUnknownBlocks`; two-seat fixtures `TestTwoSeatsOneMachine_SeatAFlightDoesNotExcuseSeatB` and `TestTwoSeatsOneMachine_SeatBIsNotToldSeatAsGoalIsReady` (one bed, machine `m`, lineages `A` and `B`; A holds a claim with a live relevant job: A allows, B has no READY and is judged on notices only; then B holds nothing and a budgeted queued goal exists: B's `ClaimAdmission` fails on `machine-quota` only → B is not READY and A's claim is not B's R3); specimen replays `TestSpecimen1_M3HoldBlocks` (claimed admissible goal, no jobs, two Stops both block), `TestSpecimen2_M0bFenceStopBlocks` (the 2026-09-01 20:30Z ledger shape: the pair holds no claim, thirty-plus queued goals carry complete stored budgets → R2 block; variant: the pair holds an unfenced budget-breached claim → R3 block naming park-or-release then claim; variant: the claim is `StopFence`d → WaitingOnHuman, not blocked on that ground), `TestSpecimen3_M0bBoardStopBlocks` (the 2026-09-02 05:00Z shape: every pair claim released, `account-provenance` queued with its stored budget → R2 block; `goal next` output irrelevant); hook fixture `stop-hook-monitor` second-Stop assertion inverted; hook fixture for F10 emitting `decision:block` | `TestTurnVerdictConvertedClaimHasTheFloor` → `…ClaimBlocksEveryStop`; `TestTurnVerdictConvertedQueueProdsOnce` → `…UnbudgetedQueueProdsOnce` (unchanged behaviour, renamed for the reason) plus sibling `…BudgetedQueueBlocksEveryStop`; `TestClaimedSessionReblocksOnceWhenTheSharedQueueChanges` → `TestClaimedReadyGoalBlocksEveryStopAndQueueChangeIsNoticedOnce`; `TestClaimedSessionBaselinesAnUnchangedQueueWithoutFalseChange` → asserts display text only, `ShouldBlock` true throughout; `TestPrecedenceLadder` → `TestPrecedenceLadderFailClosed` (Busy no longer suppresses a converted READY block; Unreadable blocks in both worlds); `TestInventoryFailureVetoes` → `…Blocks`; `TestVerdictDualSlotSequence` (legacy world) unchanged except Unreadable; `supervision-fixtures.sh:1553-1555` "refused the same open work twice" → must refuse twice, settled step must allow |
| 2 — fail closed and the Stop budget (est. 80; cap 120; depends on supervision-hook-wrong-root landing first) | hook restructure per §3.0 (traps, `emitted`, P-rows, no pre-emission `exit 0`) and rows F1–F9 (~120 lines of shell, 15); `util run-bounded` on `boundedexec` (~60 lines, 6); `util now-ns` clock arithmetic and bounded ceremonies with the cap table (~60 lines shell, 8); `goalGitContext`/`gitInContext`/`ProjectContext`/`loadTreeContext`, scan moved inside the verb, phase deadline and F17 (~160 lines, 14); `StopHookBudgetSec`, `runtime stop-budget`, three JSON files to 20, `hooks budget` verb, `up` HOOK_CHECK/HOOK_DRIFT lines per §6 (~120 lines, 10); Darwin `pidStartedAtExactMicro` on `run.Record` at the three write sites and in the readers (~40 lines, 4); tests and hook fixtures (~450 lines, 23) | `TestRunBoundedKillsProcessGroupAndExits124`; `TestVerdictDeadlineExceededBlocksNamingPhase`; `TestGoalGitContextKillsHungChild` (a fixture git wrapper that sleeps); `TestScanRunsInsideVerdictDeadline`; `TestHooksBudgetMatchesDeclarationForEveryShippedConfig` (three files); `TestRuntimeStopBudgetVerb`; `TestUpPrintsHookCheckResidualForCodexAndDevin`; `TestRunRecordCarriesDarwinExactIdentity` (Darwin build tag); hook fixtures: F1, F2, F5, F7 emit `decision:block`; a fixture that fails `mktemp` (unwritable `TMPDIR`) emits the trap block with a line number; a fixture that makes `up` hang proves emission within the budget | none inverted |
| 3 — freshness (est. 45; cap 120) | `FetchAdvanceContext` bounded fetch as verdict phase 2, `SyncLocal` proof, F16, display and ladder step 8 (~120 lines, 12); tests (~250 lines, 15); documentation of the cursor's withdrawal in the delivery contract (5) | `TestFreshnessLocalModeIsFresh`; `TestFreshnessBoundedFetchSuccessAllowsNoReady`; `TestFreshnessFetchTimeoutBlocksNoReady` (a fixture remote that never answers); `TestFreshnessFetchRefusalBlocksNoReady` (foreign ledger); `TestFreshnessStaleBoardStillBlocksOnReady`; `TestFreshnessReadyComputedOverFetchedTree` (a remote claim removes the item); `TestFreshnessNoTimeWindowExists` (a fetch that succeeded one second ago in another process does not make this verdict fresh) | `Project` staleness banner tests unchanged |
| 4 — HUMANSTOP (est. 75; cap 120) | `goal humanstop` verb with `ProveOrTemporaryGoalAuthority`, marker with provenance (~140 lines, 12); compare-and-consume under `humanstop/.lock` at the marker phase, class-A wiring, F11 as a block instead of an exit, display and audit line, pruning (~160 lines, 14); `--hook-schema`/F10 mismatch (~30 lines, 4); tests and a hook fixture (~400 lines, 22) | `TestHumanstopRequiresValidForProofOrRelay`; `TestHumanstopRelayRecordsProvenanceVerbatim` (kind, relayedBy, recordedWord, reviewBy, ruling); `TestHumanstopRelayRefusesShortWordOrPastReviewDate`; `TestHumanstopRefusesLeaseHumanClass`; `TestHumanstopConsumedByExactlyOneOfConcurrentStops` (two goroutines under the real marker lock); `TestHumanstopConsumesOnlyAgainstClassABlock` (an allowed Stop leaves the marker unconsumed; a class-B block never reads it); `TestHumanstopRescuesStateFileFailure` (F11); `TestHumanstopBoundToWorldPairAndSession`; `TestHumanstopExpiredIsIgnoredAndNamed`; `TestHumanstopConsumedBeforeAllowSurvivesCrash`; `TestHumanstopNeverConsumedAtSessionStart`; `TestHookSchemaMismatchBlocks`; hook fixture: marker set through the relay form by the fixture seat, one Stop allowed with the relayed audit line in the display, next Stop blocks again | none |

Every fixture wait carries a named ceiling per the suite rule; no benchmarks
(R-31).

## 11. Self-grade

Confidence: high that slice 1 refuses all three specimens as replayed against
the ledger facts read in §0 — each is a READY clause (R1 for specimen 1, R2 for
specimens 2 and 3 as the history shows their pair holding no claim) with no
relevant flight, and every escape they used (block-once, Busy suppression,
fail-open degraded paths) is removed by name at a cited line. Moderate on
R2/R3's false-refusal rate: `ClaimAdmission` is an extraction called at the
same point of `Mutate` as its rules and cannot drift, but gap 4 leaves the norm
approval untraced and gap 5 leaves R1's holder conjunct a safe default rather
than a traced fact — a dispatcher that admits a non-holder pair would make R1
under-report, never over-block. Weakest claim: §3.2(c)'s budget arithmetic —
the cap table sums under 20 s on paper, but `up`'s behaviour when killed by
`run-bounded` mid-transaction is asserted from its idempotence (the hook's own
words at `supervision-hook.sh:351-353`: "one idempotent transaction") and
today's identical 5 s runtime kill, not from a fixture yet; slice 2's hang
fixture is the test.
Reject this design if: a fixture shows a second unchanged Stop allowed while a
READY item exists and no relevant live job or run and no consumed HUMANSTOP; or
two seats on one machine produce a refusal loop neither can lawfully exit (a
READY item whose rendered `Move` the engine refuses — R3's park-or-release is
now read from the verbs, so this would be a new engine rule); or
`ClaimAdmission` and `Claim` disagree on any refusal or on the replay case in
the table-driven test; or a Linux seat's governed run on its READY goal is
refused by slice 1 (the run join is in the slice for exactly this reason); or
the slice-2 hang fixture shows the runtime's 20 s timeout killing the hook
before emission.
