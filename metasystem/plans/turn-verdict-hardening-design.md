# Design: turn-verdict hardening — a seat cannot end its turn on ready work

Goal: turn-verdict-hardening (plans/goals/turn-verdict-hardening.md, revision 4,
priority-1). Author: implementer delegate tvh-design-2 under dispatch by
m0b+main-1788250419-3170380-8a1fb3, 2026-09-02; revision 2 by delegate
tvh-design-r2 the same day, worktree at commit 19c61d24; revision 3 in two
rounds the same day: sections 0–6 by delegate tvh-design-r3b (worktree at
commit bb3a55cd, the round that hit its time cap) and the back matter
(§§7–11 and this header) by delegate tvh-design-r3c (worktree at commit
dbb9d0b1).
Revision: 3 — folds the five material items of
records/misc/turn-verdict-hardening-critique-r2.md: the three PARTIAL closures
re-closed (TVH-R1-R3-NAMES-ILLEGAL-EXIT §1.2.1,
TVH-R1-FAIL-CLOSED-TABLE-OMITS-PREVERDICT-SHELL-EXITS §3.0,
TVH-R1-STOP-DEADLINE-DOES-NOT-BOUND-EMISSION §3.2) and the two NEW findings
closed (TVH-R2-SLICE1-HIDDEN-WRONG-ROOT-DEPENDENCY §3/§10,
TVH-R2-HUMANSTOP-SEAT-AUTHORITY-UNSPECIFIED §5.2), each cited by id at the
paragraph that closes it; plus Sol's declared gap on the slice-1 estimate (§10)
and the dispatching seat's sequencing DECISION behind goal
supervision-hook-wrong-root (§3, §9 ask 7, §10). Revision 2's six closures Sol
certified real stand unchanged. Revision 2 closed the nine material findings of
records/misc/turn-verdict-hardening-critique-r1.md (cited as TVH-R1-…) and
carries Wido's two words of ruling R-47-m0b (memory/rulings.md:91). Revision 1's
requirements stand: the eleven findings of
records/misc/seat-stop-analysis-critique-r1.md (cited by id) and the three
specimens of records/misc/seat-stop-analysis.md.
Wido's order, verbatim: "we need machinery (not you, your behaviour, yourself but
deterministic Go code) that should make this impossible or at least give us the
highest chance of this never happening again."

Every seam cited here was read in this worktree; line numbers are at bb3a55cd
(revision 3 re-read every seam it touches at that commit; the Go and shell files
cited are byte-identical to 19c61d24, only plans and records moved between;
the back-matter round at dbb9d0b1 cited no new code seam — its new sources
are the primary checkout's job records under `artifacts/agents/jobs/`, the
four retained `rounds/1/diff.patch` files, `plans/supervision-hook-root-design.md`
Decision 4, and the two goal files).

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
    Move     []string // the lawful move: one engine-accepted command per element,
                      // in execution order (§1.2.1 R3 names the exact syntax)
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
| R3 held-releasable (SET form, revision 3) | R1 yields nothing for this pair ∧ the pair's UNFENCED claims `P₀ = {f : f.State == claimed ∧ OwnPair(f.Claimed, machine, lineage) ∧ f.StopFence == nil}` is non-empty ∧ EVERY `f ∈ P₀` is exhausted (`EvaluateGoalAdmission` refuses it with `Breaches ≠ ∅` and `Unknown == nil`) ∧ ∃ queued `g` for which `ClaimAdmission` over the CURRENT tree fails ONLY on `machine-quota` (`QuotaOnly()`) ∧ `ClaimAdmission` over the LEDGER-AFTER-MOVE `t′(g)` (below) returns nil | the same two functions, plus `ClaimAdmission` a second time over `t′(g)` | the move over the SET `X(g)` (below): one `park` or `release` line per member, then the claim line — every line in the exact syntax `goalsync_mutations.go` accepts, and only when the engine would then accept the claim |

The set rule, mechanically (closes the PARTIAL half of
TVH-R1-R3-NAMES-ILLEGAL-EXIT: one machine may lawfully hold several claims
sharing one non-empty arc, `validate.go:250-283`, so parking ONE of them can
leave the quota refused and the same advice repeated on every Stop):

1. `X(g) = {f ∈ P₀ : g.Arc == "" ∨ f.Arc ≠ g.Arc}` — every unfenced claim of
   THIS PAIR that does not share `g`'s non-empty arc. (Claims that share
   `g`'s arc count once with `g` under the quota and need no move.)
2. `t′(g)` is a deep copy of `proj.Tree` in which each `f ∈ X(g)` carries the
   state the Move produces: `State = parked`, `Claimed = nil` when
   `f.Origin != OriginHuman`; `State = queued`, `Claimed = nil` when
   `f.Origin == OriginHuman` (park of a human-opened goal is a human act,
   `verbs.go:845-847`, so the agent's lawful verb there is release,
   `verbs.go:653-668`). Nothing else changes: another pair's claims on this
   machine and this pair's FENCED claims stay exactly as they are.
3. `g` is R3-READY iff
   `ClaimAdmission(root, t′(g), g.Id, Actor{machine, lineage}, nil, "", seat.ClaimEpoch) == nil`
   — the FULL admission recomputed over the ledger-after-Move, not the quota
   alone, so the rendered Move is one the engine would accept at every line.
   When it still refuses (a claim outside `X(g)` blocks the quota: another
   pair's claim on this machine, or a fenced claim of this pair), `g` is NOT
   READY and `Reasons` carries one line naming the blocking claim and its
   holder: "`<g>` waits on the machine quota held by `<H>` (`<machine>+<lineage>`)"
   or "… held by `<H>`, breach-stopped; only `goal resume` clears it" (the
   latter also appears under WaitingOnHuman, as revision 2 specified).
4. The first qualifying `g` by id is the `ReadyItem`; every other qualifying
   `g` is named in `Reasons` as "also claimable after the same move".
5. `Move`, in order, every element byte-exact (the engine name is the
   operator-facing `metasystem`; `<root>` is the absolute ledger root the
   verdict read; `<lineage>` is `seat.Lineage`, which already satisfies
   `validLineage`'s `[A-Za-z0-9._-]{1,128}`, so no quoting arises; breach
   names are the `Breaches` identifiers, each matching `[a-z-]+`, joined by
   `", "`):
   - for each `f ∈ X(g)` sorted by id, when `f.Origin != OriginHuman`:
     `metasystem goal park --root <root> --id <f.Id> --lineage <lineage> --because "budget exhausted: <breach names>"`
     (`park` requires `--id` and `--because`, `goalsync_mutations.go:232-241`;
     own pair, no fence → `clearClaimBinding` passes, `verbs.go:849-864`);
   - for each `f ∈ X(g)` sorted by id, when `f.Origin == OriginHuman`:
     `metasystem goal release --root <root> --id <f.Id> --lineage <lineage>`
     (`release` requires `--id`, `goalsync_mutations.go:676-681`; own pair,
     no fence → passes, `verbs.go:653-666`);
   - last: `metasystem goal claim --root <root> --id <g.Id> --lineage <lineage>`
     (`claim` requires `--id`, `goalsync_mutations.go:653-668`; no budget flags
     because READY means the complete STORED budget is on the ledger, R-47-m0b
     word 2, so `budgetTuple(false)` yields nil and `goal.Claim(req, id)` reads
     the stored tuple, `verbs.go:163-174`).
   `--lineage` is rendered explicitly on every line because
   `syncReq` refuses a mutation with neither the flag nor
   `METASYSTEM_OWNER_LINEAGE` (`goalsync_mutations.go:38-44`), and the seat's
   shell may carry neither. No positional goal id exists in this grammar
   (`parseSyncFlags`, `104-159`); revision 2's positional strings are
   withdrawn.

Revision 2's fenced-claim closure stands: a FENCED claim (`StopFence != nil`)
is excluded from every clause. Release, park and done refuse it at
`clearClaimBinding` (`verbs.go:129-131`), steal refuses it (`1229-1231`), and
only `goal resume` — human-only with observed or relayed authority
(`stop.go:355-358`) — clears the fence. Such a claim is listed in
`Frontier.WaitingOnHuman` as "`<H>` is breach-stopped by `<StopID>`; only
`goal resume` (a human) clears it", and every queued goal that
`ClaimAdmission` refuses only on the quota is listed beneath it as waiting
behind the fence. WaitingOnHuman is reported, never blocked (§7 step 9). The
command `goal park <held> --then <g>` stays withdrawn: the converted parser
declares no `--then` flag (`goalsync_mutations.go:104-159`) and routes park
straight to `goal.Park` (`232-241`). Named residual (§8): releasing or parking
an exhausted claim and re-claiming it starts a fresh claim revision whose
budget window is the engine's rule, not this design's; the verdict reports what
the engine admits.

The two-arc fixture is in slice 1a's tests (§10,
`TestReadyHeldReleasableNamesEveryQuotaBlockingClaim`): H1 and H2 held
exhausted by pair `m+A` in arc `A`, queued `g` with a complete stored budget
in arc `B` → the Move names BOTH parks then the claim; variant H1 in arc `A`,
H2 in arc `B`, `g` in arc `B` → the Move names H1 alone; variant seat `m+B`
holds H3 in arc `C` → `g` is not READY for `m+A` and `Reasons` names H3 and
`m+B`; variant H1 human-origin → its line is `release`, not `park`. The test's
decisive assertion executes the rendered Move, line by line, through the REAL
verbs against a copy of the fixture ledger and requires `OutcomeConfirmed` on
every line and `g` claimed by `m+A` afterwards — "a Move the engine would then
accept" is proven by running it, not by reading it.

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

The display names the first `ReadyItem` and its `Move`, one command per line,
byte-verbatim in the block reason as today (`supervision-hook.sh:290-297`). `stopblock.go:11-13`
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

Sequencing (closes TVH-R2-SLICE1-HIDDEN-WRONG-ROOT-DEPENDENCY; the dispatching
seat's DECISION, recorded in §9 ask 7): the wrong-root fix
(`plans/supervision-hook-root-design.md` at revision 3, goal
supervision-hook-wrong-root) lands BEFORE slice 1a, the first slice of this
design. Sol is right that on the affected fleet seats today's hook exits at the
missing-engine branch (`supervision-hook.sh:26-30`: the delegate worktree
carries no `bin/metasystem`, so `$ms` is not executable and the hook prints its
HEALTH-unknown line and exits 0) before any verdict runs, so no slice of this
design can refuse a specimen at the deployed Stop boundary until the root fix
is in. Under this table an unresolved identity is a BLOCK, so the hook rows on
a wrong-root fleet would wedge every seat on every Stop. Its silent-exit rows
are re-dispositioned below; its skew and missing-engine lines keep their text,
prefixed into the block reason. What each slice lands on top of the root fix
is stated in §10.

### 3.0 The hook's structure: the trap is the first thing, the emitter is the only writer (re-closes TVH-R1-FAIL-CLOSED-TABLE-OMITS-PREVERDICT-SHELL-EXITS)

`supervision-hook.sh` runs `set -euo pipefail` (line 2) and, before any
verdict, performs unguarded `cd`/`pwd -P` (23-24), `mktemp` and `cat` (42-44),
`mkdir -p` (263-264), a second `mktemp` (273), and command substitutions in
assignments that `set -e` treats as failures (278-280, 297-304). Each is a
silent exit today. Sol's round-2 refutation of revision 2 is accepted: its
rule 4 let only `emit_stop_payload` set `emitted=1` while P2 and rows F1, F2
and F5 printed their own JSON and exited, so the EXIT trap then printed a
SECOND object. Revision 3 makes one function the only thing that can write to
stdout. The rewritten Stop path has this fixed shape, in this order, and an
implementer follows it line for line:

```bash
#!/usr/bin/env bash
set -euo pipefail
# The clock starts HERE, before any external command (§3.2(b)); builtins only.
hook_entry_us=${EPOCHREALTIME:+${EPOCHREALTIME/./}}
runtime=${1:-}; event=${2:-}
emitted=0; fail_line=0
on_err() { fail_line=$1; }
# emit_json is the ONLY statement in this file that writes to stdout on a Stop
# path, and the ONLY place emitted is set. Builtins only.
emit_json() { # one already-serialized JSON object
  if command printf '%s\n' "$1"; then emitted=1; fi
}
block_json() { # a FIXED literal reason from this file (plus digits); never engine or user text
  emit_json '{"decision":"block","reason":"'"$1"'"}'
}
on_exit() {
  rm -f "${payload:-}" "${response_file:-}" "${verdict_stderr:-}" 2>/dev/null || true
  if [[ "$event" == stop && $emitted -ne 1 ]]; then
    block_json "turn-exit gate did not reach a verdict (supervision-hook.sh line ${fail_line}). The metasystem could not judge this turn end, so it refuses it. Repair the hook or engine (metasystem up --repo <path>; rebuild bin/metasystem), then end the turn again. No human stop applies here: the machinery that would honour it did not run."
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
`emit_json`, `block_json` and the trap body use only builtins; `block_json`
accepts only FIXED literals written in this file (the one variable part is
`fail_line`, digits only), so no escaping question exists — any reason that
carries engine or user text (F10's stderr line, the verdict display, the
health line) is serialized by the engine (`report stop-block`, `json object`)
and the whole object is handed to `emit_json`; (3) `exit 0` inside the EXIT
trap overrides the failing status, so exit codes are never the delivery
channel; (4) ONE EMITTER: `emit_json` is the only writer to stdout on a Stop
path and the only place `emitted` is set; `emit_stop_payload` calls
`emit_json` for its write and keeps only the bookkeeping around it (response
file, `hook-complete`, `digest-advance`); P2, F1, F2, F4, F5 and the trap call
`block_json`; so exactly one object leaves the hook on every path — after a
successful `emit_json` the trap prints nothing, and before one only the trap
prints; (5) the existing `trap 'rm -f "$payload"' EXIT` (line 43) is deleted
— it would replace the fallback trap; cleanup lives in `on_exit`; (6) no
`exit` on a Stop path precedes a successful `emit_json`: today's early exits
(30, 62, 65) become `block_json …; exit 0` rows below, and `emit_json` failing
leaves `emitted=0` so the trap tries once more (F19 when that fails too); (7)
STDOUT DISCIPLINE, mechanically checkable: every other command on a Stop path
has its stdout captured by `$(...)`, redirected to a file or `/dev/null`, sent
to `>&2`, or piped; the outputs of `surface_json`, `report stop-block` and
`json object` are captured into `response` and reach stdout only through
`emit_json`. The static test `TestHookStopPathHasOneStdoutWriter` (slice 2)
scans the hook for every `printf`/`echo` and requires each to be the one
inside `emit_json` or to carry a redirection or pipe.

Every pre-verdict operation maps to a row (line numbers today; after the root
fix the world block moves, the rows keep their meaning):

| # | Operation (today's line) | Outcome |
| --- | --- | --- |
| P1 | `script_dir`/`harness_root` resolution fails (23-24) | ERR trap → block, line named |
| P2 | `runtime list` fails or runtime unregistered (32-40) | `block_json` (F5), then `exit 0` |
| P3 | payload `mktemp` fails (42), or the bounded `cat` of stdin fails or expires (44; §3.2(c)) | ERR trap → block |
| P4 | `read_payload session_id` (67) cannot run the engine | `session-$PPID` as today (68); not decision-bearing |
| P5 | session-env query (52-63) | deleted by the root fix (§3 sequencing); no row |
| P6 | world identification or `path state-root` fails (root design Decision 2 rows 1-6 and 9-10, silent exit 0 there; today's 65) | `block_json` (F2, F4), then `exit 0`; the verb's own exit 1 stays F3 |
| P7 | `turn_key`/`hook-attempt` failure (86-101) | rides the reason (F9); never exits |
| P8 | `mkdir -p "$supervision_dir"` fails (264) | ERR trap → block |
| P9 | `mktemp` for verdict stderr fails (273) | ERR trap → block |
| P10 | verdict `json get` field reads fail (278-280) | ERR trap → block (F10) |
| P11 | `report stop-block` / `json object` response construction fails (297-304) | ERR trap → block |
| P12 | `emit_json` cannot print to stdout (196-201) | residual F19 |
| P13 | a bounded call expires (rc 124) or is skipped for lack of budget (rc 125; §3.2(c)) | the call's own row in the §3.2(c) table: display-only calls → their fixed "(bounded)" line; identity calls → F7; the verdict → F17 |

The two `mktemp` calls and `evidence-gc` keep their `|| true` where they have
it; a row's block is the trap's, not a bespoke branch. Fixture
`hook-single-response` (slice 2, `supervision-fixtures.sh`) fires a Stop under
F1 (no engine at the world), F2 (the hook staged outside any git repository),
F5 (a stub engine whose `runtime list` prints nothing), a trap exit (an
unwritable `TMPDIR` so `mktemp` fails), and one ordinary READY block, and
asserts on each: stdout is exactly one line, that line is one JSON object
whose `decision` is `block` (`json get --field decision`), and the exit status
is 0.

### 3.1 The outcome table

| # | Condition | Detected where | Class | Hook decision and recovery named in the reason |
| --- | --- | --- | --- | --- |
| F1 | engine missing at the resolved world | hook, literal test (26-31) | B | BLOCK through `block_json` (§3.0 rule 4; no engine exists to serialize anything): rebuild `bin/metasystem` (`go build -o bin/metasystem ./cmd/metasystem`) |
| F2 | world identification fails (git query, parse, common-dir shape, mapping, shape check) | hook (root design Decision 2 rows 1-6, were silent exit 0) | B | BLOCK through `block_json`: "the hook cannot identify its world"; `metasystem up --repo <path>` |
| F3 | `path state-root` exit 1: the engine proves an ungoverned installation | engine verb | — | ALLOW with a visible line (proven absence, not a guess) |
| F4 | `path state-root` other failure or empty (engine/hook skew) | hook | B | BLOCK through `block_json`: the fixed skew line as reason; rebuild or re-adopt |
| F5 | runtime registry query fails or runtime not registered | hook (32-40, today exit rc / exit 2) | B | BLOCK through `block_json`; exit 0 |
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
| F17 | Stop budget: the verdict's own deadline exceeded at a phase boundary, or the hook's remaining budget is below the verdict floor before the verb is called | verb, `--deadline-ms` (§3.2(d)); hook (§3.2(c)) | A (verb) / B (hook-side: no verb ran, no marker consulted) | verb exits 0 with `ShouldBlock = true`, `BlockSource = "deadline"`, the phase named; the marker phase still runs under its own 500 ms lock cap (§5.3); the hook emits immediately. Hook-side: `block_json` naming "budget exhausted before the verdict", class B |
| F18 | Stop budget: the runtime kills the hook before emission | runtime | — | RESIDUAL: the runtime's default applies (allow). Made improbable by §3.2 (the clock runs from hook entry; every non-exempt command is bounded by min(cap, remaining); the verdict and emission reserves are subtracted before every cap; the exempt steps are named with why they cannot block); recorded, not closed |
| F19 | emission failure (`printf` to stdout fails) | hook (196-201) | — | RESIDUAL: `hook-complete --outcome EMISSION_FAILED` is recorded; the steward's hook-freshness sees it; the runtime's default applies |
| F20 | HUMANSTOP marker unreadable or malformed | verdict | — | treated as absent: normal rules (a broken marker authorizes nothing) |
| F21 | verdict state file malformed | `loadVerdictState:645-647` resets today | — | unchanged: the state holds only non-ready memory now, so a reset can only cause an extra notice, never an allowed exit |
| P1–P11 | pre-verdict shell exits (§3.0) | hook trap | B | BLOCK via the EXIT trap, line named |

### 3.2 The Stop budget (re-closes TVH-R1-STOP-DEADLINE-DOES-NOT-BOUND-EMISSION)

Sol's three round-1 refutations stand accepted: a `context.WithDeadline`
cancels nothing in `report.Scan`, `Project`, `FetchAdvance` or their git
children because none takes a context (`goal.go:543` runs the scan before
`TurnVerdict`; `fetchadvance.go:30`, `project.go:31`, `txn.go:71` take none);
`date +%s%N` is GNU-only; and ceremonies moved behind the verdict can still be
killed before the runtime reads the emission. Sol's round-2 refutations of
revision 2 are accepted too: the clock started after the engine test, so the
world mapping was uncharged; only the named ceremonies and the verdict were
bounded while root mapping, runtime lookup, payload handling, hook-attempt
recording, ancestor discovery, lease classification, JSON extraction and
response construction were not; the marker lock's five-second wait exceeded the
verdict's 3.5 s worst-case allowance; and `up` is sequential, not atomic
(`up.go:412-500`). Decisions:

(a) **Declared budget.** Unchanged from revision 2: `runtimes.Declaration`
gains `StopHookBudgetSec int` (codex 20, devin 20, claude 20, fake 5);
`metasystem runtime stop-budget <name>` prints it; all THREE shipped files move
their Stop `timeout` to 20 (§6 owns the per-runtime check).

(b) **The clock starts at hook entry, in bash.** `hook_entry_us` is captured on
the second line of the script (§3.0 shape), before the runtime-name check,
the world mapping, and every external command, from `$EPOCHREALTIME` (bash ≥
5.0, microseconds; present on the fleet's bash 5.2.15, read on this seat).
`elapsed_ms` is a shell function of builtins only:

```bash
elapsed_prev=0
elapsed_ms() { # monotone non-decreasing; rounds UP; never runs a command
  local now
  if [[ -n "${EPOCHREALTIME:-}" ]]; then
    now=$(( (${EPOCHREALTIME/./} - hook_entry_us + 999) / 1000 ))
  else
    now=$(( (SECONDS + 1) * 1000 ))   # bash 3.2: whole seconds since shell start, rounded up
  fi
  (( now > elapsed_prev )) && elapsed_prev=$now
  printf '%s' "$elapsed_prev"
}
```

Without `EPOCHREALTIME` (a stock Darwin bash 3.2) the charge is coarse but
never under-counts: `SECONDS` counts whole seconds from shell start and the
`+ 1` rounds up, costing at most 1 000 ms of budget, which is charged, never
allowed. The hook no longer calls `util now-ns` (the verb stays; nothing else
used it in the hook). Both clocks are wall clocks: a forward step shortens the
budget (the fail-closed direction); a backward step is clamped by the
`elapsed_prev` rule, so a step of s seconds can extend the budget by at most s
— residual F18, named in §8. The pre-engine git queries of the root design
are charged by this clock even though they are exempt from bounding (c).

(c) **Every non-exempt command bounded by min(cap, remaining); the reserves
subtracted before every cap.** Numbers: `U = 16 000` (80% of the 20 s budget);
`E = 1 500` emission reserve; `V = 3 000` verdict floor; the plumbing
allowance `A = U − E − V = 11 500`. A new verb `metasystem util run-bounded
--deadline-ms N --kill-grace-ms G -- <argv...>` runs the child through
`boundedexec` with stdin, stdout and stderr INHERITED (so `$(bounded …)`
captures stdout exactly as today and `bounded 1000 cat >"$payload"` reads the
runtime's stdin), `Setpgid`, SIGKILL to the process group on expiry
(`boundedexec.go:92, 106-108`), exit 124 on expiry; the reap after the kill
is bounded by `G` (a new `Bound.KillGrace` field; the package constant
`killGraceWindow` of 5 s, `boundedexec.go:38`, stays the default for every
other caller; the hook passes 200 ms — a child that outlives SIGKILL by 200 ms
is in kernel D-state and is abandoned, not waited for). The hook wraps every
engine call and the stdin read in one function:

```bash
bounded() { # cap_ms argv... ; rc = the child's, 124 on expiry, 125 when skipped
  local cap=$1; shift
  local left=$(( 11500 - $(elapsed_ms) ))
  (( left >= 100 )) || return 125
  (( cap <= left )) || cap=$left
  "$ms" util run-bounded --deadline-ms "$cap" --kill-grace-ms 200 -- "$@"
}
```

The table (line numbers today):

| Class | Calls | Cap (ms) | Outcome on rc 124 or 125 |
| --- | --- | --- | --- |
| payload | `cat` of the runtime's stdin (44) | 1 000 | P3 → trap block (a runtime that never closes stdin cannot be judged) |
| pure engine | `runtime list` (32); every `json get` (67, 97-98, 116-117, 123-125, 133-135, 168-170, 223-224, 278-280); `util sha256` (73, 86, 258); `util slug` (143); `report stop-block` / `json object` (140, 297-304) | 300 each | `runtime list` → F5; a `json get` on a decision field → that field's row (F10 for verdict fields, F7 for identity fields); the rest leave the value empty and ride the display as today |
| world | `path state-root` (root design replacement block) | 500 | F4 |
| identity | `proc find-ancestor` (109); `lease classify` (122, 131) | 1 000 each | F7 |
| evidence | `steward hook-attempt` (92) | 1 000 | F9 (rides the reason) |
| lease | `lease protocol-growth` (221); `lease renew` (249) | 500 each | empty or skipped, as today's `|| true` |
| ceremonies | `health --hook-preview` (161) | 2 000 | the fixed unknown line (163) |
| | `steward digest-pending` (166) | 1 500 | "NARRATOR DIGEST unavailable (bounded)" |
| | `supervise watchdog-report` (256) | 1 500 | empty digest |
| | `evidence-gc.sh` (265) | 1 000 | already `|| true` |
| verdict | `report turn-verdict` (274) | `min(8 000, 14 500 − elapsed)`; when that is below `V = 3 000` the hook emits F17 itself through `block_json` without calling the verb | F17 |
| post-emission (inside `E`) | `steward hook-complete` (185-211): 500; `steward digest-advance` (204): 300; `lease protocol-advance` (244, 323): 300 | — | already `|| true`; the emission is recorded as today |

`up` (148-156) is no longer on the Stop path — (e). The construction is NOT
"the caps sum below the budget" (the class caps above sum past `A`, and that is
fine): it is that `A` is subtracted from every cap, so however many calls run,
the verdict starts with at least `V` and the emission with at least `E`, and a
call that finds fewer than 100 ms of allowance is skipped (rc 125) and takes
its row. Worst case in numbers: plumbing ≤ 11 500; verdict ≤ min(8 000,
14 500 − elapsed) and ≥ 3 000; emission ≤ 1 500; total ≤ 16 000 = 80% of
20 000. The remaining 4 000 ms cover what the allowance does not charge
exactly: bash builtins, the exempt commands below, the fork and exec of
`run-bounded` itself (one extra engine start per bounded call, tens of
milliseconds each, charged to elapsed by the next clock read), and the 200 ms
kill grace.

**Exempt steps, and why they cannot block.** Bash builtins (`cd`, `pwd -P`,
`printf`, `read`, `[[ ]]`, arithmetic, `trap`, `exit`) run inside the hook's
own process. The coreutils calls `mkdir -p`, `mktemp`, `rm -f`, `basename`,
`dirname`, `date -u`, `tail -1` on a file the hook itself wrote, `grep -Fxq`
on a here-string, and the pre-engine `git rev-parse` queries of the root design
(`--path-format=absolute --git-dir --git-common-dir`; `--show-toplevel`) each
touch only the local filesystem at an already-resolved path, take no lock,
open no network, and read no stdin from another party; they block only when
the local filesystem itself hangs — and then bash's own reading of the script
file hangs identically, so no in-script bound could fire. That case is F18,
and so is an engine binary whose exec itself hangs (the bounding primitive is
the engine; it cannot bound its own start). Nothing else is exempt.

(d) **The verb bounds itself.** `report turn-verdict --deadline-ms N` (`N ≥
3 000` by (c)) computes a deadline `D` and reserves `M = 600` ms for the
marker phase (a 500 ms lock cap, §5.3, plus 100 ms for the read and rename)
and 100 ms for marshal. The phases identity (§1.1, one lease read) → bounded
fetch (§4) → projection → scan → READY → relevance run under `D − 700`, with
the cooperative check at each boundary and F17 (phase named) when exceeded;
then the marker phase ALWAYS runs under its own cap; then the state-file phase
runs only when at least 50 ms remain before `D − 100`, with `withLock`'s wait
bounded to `min(2 000, D − 100 − now)` through a `withLockDeadline(d)` variant
of `goalverbs.go:96-118` (its constant `lockDeadline` is 2 s, `goalverbs.go:91`)
— a skipped or timed-out state write is F11 (block, `state-unavailable`),
never an exit; then marshal. So `V = 3 000` decomposes as 2 300 for the phases,
600 for the marker, 100 for marshal, and the marker's 500 ms lock cap fits
inside the floor by construction — revision 2's 5 s wait is withdrawn. Every
git child in this path becomes bounded: `goalGit` and `gitIn` gain
context-taking variants (`goalGitContext(ctx, root, extraEnv, args...)`,
`gitInContext`) built on `exec.CommandContext` with the same process-group
kill as `boundedexec` and a `WaitDelay` of 500 ms; `FetchAdvanceContext(ctx,
e)`, `ProjectContext(ctx, e, fetchFirst, now)` and `loadTreeContext` are added,
and the existing functions become `context.Background()` wrappers, so no other
caller changes. `report.Scan` spawns no subprocess (the only `exec` in the
package is `frontier.go:158`, outside the Stop path) and is file reads plus
probes; it moves INSIDE the verb's phase sequence (`goal.go:543` today runs it
before `TurnVerdict`) so its cost is charged to the deadline, with the
cooperative check before and after it.

(e) **`up` leaves the Stop path.** Sol's note is accepted and traced.
`ordinary()` (`up.go:412-500`) runs preflight → enrollment open → session
identity → `AnnounceWithProofAt` under the steward arbitration flock
(kernel-released on death, `arbitration.go:38`) → arming-log append →
`ClassifyVerbAt` → `EnsureArmed` → `EnsureRunner`, sequentially. Inside
`EnsureArmed` (`arming.go:658-763`) the cap-authority lock is a pid-keyed lock
directory whose dead holder a later claimant takes over by probing the recorded
pid (`ownerlock.go:60-102`) — that part self-repairs. But `os.Mkdir(ownerLockDir)`
(684) followed by `launchOwner` (599-647: the owner is spawned with `Setsid`
at 613, so it survives a kill of `up`'s process group; the owner record is
written at 638 and the start gate at 642 only after the owner's identity is
readable, up to 5 s scaled) leaves, on a kill inside that window, a lock
directory WITHOUT an owner record. The next `up` then reads no record, waits
5 s scaled, and fails "supervision lock has no provable owner" (701-710);
nothing removes an ownerless lock directory (`releaseDeadOwnerLock` needs a
record, 715-724). So the next `up` does NOT repair a mid-kill in that window,
and revision 2's idempotence claim — taken from the hook's own comment at
351-353 — is withdrawn. Sol's declared gap is also accepted: today's runtime
timeout may kill the hook alone and leave `up` (a child) to finish, so a group
kill by `run-bounded` would introduce the hazard, not inherit it. DECISION:
the Stop hook calls no `up` in either branch (148-156); `up` stays on
SessionStart (348-355, its 15 s timeout unchanged) and in the operator's
Ring 3 entry (`up --recover-only --if-down`). Consequences, stated:
supervision that dies mid-session is re-armed at the next session start or
scheduler tick rather than at the next Stop (a revival-latency regression,
§8); every Stop still SHOWS supervision state through `health --hook-preview`,
whose line names each supervised role's status, reason and remedy
(`health.go:153-167`); a missing or stale announcement no longer self-heals at
Stop — the verdict then reaches F7 (identity unknown, class-B BLOCK) whose
reason names `metasystem up --repo <path>` as the repair the seat runs in its
next turn, the fail-closed direction §3 chose; the `up_failure` line (157-159)
and its display wiring leave the Stop path; the advisor path (232-247) keeps
working because it reads classification, not `up`. The root design's Decision
4 row for lines 148-155 becomes moot for Stop once slice 2 lands; its
SessionStart rows stand. The ceremonies' worst case drops from 11 000 to
6 000 ms with `up` gone.

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

### 5.2 Who may set it, and for whom (carries R-47-m0b word 1; closes §9 ask 1; closes TVH-R2-HUMANSTOP-SEAT-AUTHORITY-UNSPECIFIED)

Verb: `metasystem goal humanstop --root <root> --directive "<words>" [--ttl 8h]
[--session <id>] [--review-by YYYY-MM-DD]`. It has NO `--machine`, NO
`--lineage` and, under the relay form, NO `--seat` flag; it never reads
`METASYSTEM_OWNER_LINEAGE`; and it does not go through `syncReq`
(`goalsync_mutations.go:26-71`, whose lineage comes from a flag or the
environment) because it mutates no ledger — it writes one marker file.

**Authorization, HUMANSTOP-scoped.** The proof is
`humanauthority.ProveOrTemporaryGoalAuthority(root, int64(os.Getppid()), nil,
directive, reviewBy, now)` (`authority.go:228-237`), the same entry `goal
resume` uses (`goalsync_mutations.go:377-378`): the enrolled-terminal proof
(`Prove`, `ValidFor(root)`, `authority.go:89-111`: enrolled terminal ancestry,
no agent runtime on the chain, the installed signature set) wins when it holds;
when it fails and `--review-by` is present, the temporary relay proof is minted
from the directive as the recorded word (`temporaryGoalProofAt`, `206-222`:
≥ 3 words, a real review date not in the past and inside the governance
horizon, `159-180`). Sol is right that the shipped `Proof` exposes only
`AuthorizesSetObligation` and `AuthorizesResume` (`authority.go:117-131`),
each scoped to its verb, so a new method is added beside them:

```go
// AuthorizesHumanstop accepts enrolled-terminal ancestry or the temporary
// recorded-relay form for exactly one act: minting the single-use HUMANSTOP
// marker. The relay form is admitted here by Wido's ruling R-47-m0b, word 1
// (memory/rulings.md:91), verbatim: "a RELAYED human word carried through the
// temporary-human-word path MAY mint the single-use HUMANSTOP marker that lets
// a seat end its turn on ready work". The relay records the supplied words
// but cannot verify who supplied them (TemporaryGoalProof); the marker's
// provenance and the Stop display name that fact.
func (p Proof) AuthorizesHumanstop(root string) bool {
    return p.ValidFor(root) || p.temporaryValidFor(root)
}
func (p Proof) TemporaryHumanstopFor(root string) bool { return p.temporaryValidFor(root) }
// RecordHumanstopProof stores the proof for the one act that may consume it.
func RecordHumanstopProof(root, operationID string, proof Proof) error {
    return recordProof(root, operationID, "goal humanstop", proof, proof.AuthorizesHumanstop(root))
}
```

The proof's own `Departure` stays `TemporaryWordRuling` (`R-32-m1`,
`governance/types.go:96`, the temporary-word path's ruling, which
`temporaryValidFor` checks at `authority.go:185`); the MARKER's
`provenance.ruling` is `R-47-m0b`, the ruling that admits that path to
HUMANSTOP. Both ids appear in the audit line. `AuthorizesResume` and
`AuthorizesSetObligation` are not reused: a proof is scoped to the act it
authorizes, and `recordProof` refuses to record a proof under an action whose
authorizer does not accept it (`584-586`).

**The seat is derived from the SAME authenticated classification the verdict
uses — never from flags.** The command calls
`lease.ClassifyVerbAt(root, metasystemRoot, int64(os.Getppid()))`
(`lease/verbs.go:294-322`): exactly §1.1's call, over the invoker's parent,
walking ancestry to the authenticated announcement (`classify.go:324-342`).
Then:

- **Relay form** (`proof.TemporaryHumanstopFor(root)`): require `view.Class ==
  MAIN` with `view.Announcement != nil` and a non-empty
  `lease.AnnouncementLineage(view.Announcement)` (the export of
  `announcementLineage`, `lease.go:131-136`: `ownerLineage`, else `mainId` —
  §1.1's rule). `machine = goal.ResolveMachine(root)`. The marker's `machine`,
  `lineage` and `provenance.relayedBy` are all this one pair, by construction
  equal; `relayedBy` stays as an explicit audit field. A `--seat` flag under
  this form is refused: "the relay form mints only for the invoking seat".
  Every other classification is refused by name: HUMAN ("no seat: the relay
  form runs from the seat's own session; at an enrolled terminal use the
  enrolled form"), DELEGATE, STEWARD, SUPERVISION, ADAPTER-SUPERVISOR, an
  empty class, or a classification error. So a caller-controlled lineage
  cannot mint for another seat and cannot forge `relayedBy`: neither value is
  read from anything the caller supplies.
- **Enrolled-terminal form** (`proof.ValidFor(root)`): the chain is
  agent-free by construction (`Prove` refuses `OutcomeAgent`,
  `authority.go:533-538`), and MAIN classification requires an announced
  runtime process among the ancestors (`classify.go:324-342`), so this form
  can NEVER carry a classification-derived seat; the caller classifies HUMAN
  (`369-377`). ASSUMPTION A3-HUMANSTOP-SEAT, flagged for the dispatching seat
  (§9 ask 9): under this form the human names the target with
  `--seat <machine>+<lineage>`; `machine` must equal
  `goal.ResolveMachine(root)` (a marker is machine-local: its path is under
  this root) and `lineage` must satisfy `validLineage`; the marker records
  `provenance.kind = enrolled-terminal`, `seatNamedBy = "enrolled-terminal"`,
  and no `relayedBy`. The human is the authority here, so a named target is
  not the forgery Sol's finding names. If the assumption is overruled, the
  enrolled form is withdrawn entirely and the human stops a seat by speaking
  the word to it (which relays) or by the redirect verbs.

**Minting is for the minting seat's own pair (relay form).** The marker path
`<root>/artifacts/agents/humanstop/<machine>+<lineage>.json` is built from the
derived pair, and the file's `machine`/`lineage` fields are the same values;
`world` is `goal.ExistingLedgerIdentity(root)`. The marker additionally
carries `proofOpid`, the operation id under which `RecordHumanstopProof` wrote
the proof record; §5.3's compare-and-consume requires that record to exist with
action `goal humanstop` and a proof form matching `provenance.kind` — a marker
whose proof record is missing or mismatched authorizes nothing. This raises a
hand-forged marker from one file to two consistent files; it does not close
same-user filesystem authority, which the ledger's announcements and locks
already live under (§8).

Wido's word (R-47-m0b, decided): a RELAYED human word carried through the
temporary-human-word path MAY mint the single-use marker. The path's residual
is exactly Sol's (SSA-R1-HUMANSTOP-RELAY-LAUNDERING; `authority.go:195-198`:
it "cannot verify who supplied the words") and it is recorded here as a
HUMAN-RATIFIED EXCEPTION, not a hole: the marker records the relay provenance
verbatim (`provenance.kind = temporary-relay`, `relayedBy`, `recordedWord`,
`reviewBy`, `ruling = R-47-m0b`), the audit line in the Stop display names it
as relayed — "HUMAN STOP (relayed by <machine>+<lineage>, recorded word:
"<directive>", review by <date>, R-47-m0b over R-32-m1)" — and the marker's
history line in `hooks.log` carries the same text. The hook's own `lease
classify` HUMAN class (`classify.go:369-377`: "no recognised ancestor and a
controlling terminal") remains too weak for minting — a pseudo-terminal
allocated by an agent satisfies it — and is NOT accepted as authority; it is
only the classification the enrolled form is expected to carry. `--session`
is caller-supplied and stays so: it can only NARROW the marker (a wrong
session never consumes), so it needs no authentication. Redirecting work is
not HUMANSTOP: `goal park`, `steal`, `set-pin`, `open` are the human's redirect
verbs and leave READY to be recomputed.

### 5.3 Compare-and-consume, bound to one Stop

At the verb's marker phase (§3.2(d)), under the marker directory's OWN lock
(`humanstop/.lock`: `withHumanstopLock(root, 500*time.Millisecond)`, the same
20 ms flock poll shape as `withWaiterLock`, `waiter.go:66-88`, with a 500 ms
deadline instead of that function's 5 s — revision 2's 5 s exceeded the
verdict's worst-case allowance; 500 ms fits inside the 600 ms marker reserve of
§3.2(d)) — independent of the verdict-state flock so that F11 cannot prevent
consumption — and only when the decision so far is a class-A block (READY,
open work, F12, F14, F15, F16, F17, F11). A lock that is still busy at 500 ms
is "marker unknown": nothing is consumed, the class-A block stands, the display
says "HUMANSTOP marker busy; not consumed", and the human's word is not spent —
the next Stop retries.

1. Read the marker for `(machine, lineage)`. Absent, unreadable, wrong `world`,
   wrong pair, `runtimeSession` set and ≠ this session, `now ≥ expiresAt`,
   `consumed ≠ null`, or a `proofOpid` whose proof record is missing or was not
   recorded under action `goal humanstop` (§5.2) → no HUMANSTOP; an expired,
   foreign or unproven marker is named in the display, never consumed.
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

## 7. The new precedence ladder (converted world; consistent with §§1–5 at revision 3)

The ladder is the DECISION order. The EXECUTION order inside the verb is
§3.2(d)'s phase sequence (identity → bounded fetch → projection → scan →
READY → relevance → marker → state file → marshal); the hook's pre-verdict
rows (§3.0) run before any of it.

| Order | Condition | Class | Outcome |
| --- | --- | --- | --- |
| 1 | any class-B unknown: F1, F2, F4, F5, F6 (until the root fix deletes it), F7, F10, the hook-side half of F17, and P1–P11 | B | BLOCK; no marker consulted; the reason names the machinery-owned repair (§3, §3.0). Emitted through `block_json` or the EXIT trap: exactly one object (§3.0 rule 4) |
| 2 | any class-A unknown reached inside the verb: F11, F12, F14, F15, the verb-side half of F17 | A | BLOCK — unless step 3 consumes |
| 3 | HUMANSTOP compare-and-consume succeeds against a class-A block — §5.3's set, verbatim: READY, open work, F11, F12, F14, F15, F16, F17 | A | ALLOW; display the directive with its provenance; the relayed form is named as relayed (§5.2). A marker that is absent, foreign, expired, consumed, busy (500 ms lock cap) or unproven (`proofOpid`) consumes nothing and the class-A block stands |
| 4 | READY ∧ ¬RELEVANT | A | BLOCK `ready-work`, every Stop, no session memory; the reason carries the first `ReadyItem` and its `Move` byte-verbatim, one engine-accepted line per element (§1.2.1 rule 5, §1.3); step 3 applies |
| 5 | plan `Open ≠ ∅` ∧ no in-flight job record | A | BLOCK `open-work`, every Stop (§1.3); step 3 applies |
| 6 | unwatched work | A-shaped, but OUTSIDE §5.3's set | BLOCK `unwatched-work` once per digest, unchanged (§1.5); the marker is NOT consulted here: the escape is arming the printed watch, which `WaiterLive` proves |
| 7 | READY ∧ RELEVANT | — | ALLOW; "STILL WORKING: <job or run> on <goal>@<binding>". FRESH is not required on this row: F16's condition is `Frontier.State == "none"` (§3.1), and the flight is proven live locally in the exact identity mode (§2.1, §2.2) |
| 8 | no READY ∧ ¬FRESH | A | BLOCK `stale-board` (F16; step 3 applies); READY from a stale board is step 4, never this row (§4) |
| 9 | no READY, FRESH | — | WaitingOnHuman lines (fenced claims of this pair and the queued goals waiting behind them, §1.2.1), the non-ready notices that keep session memory (queue change, goal-free staleness, the unbudgeted-queue once-notice, §1.5), the Busy display, then the all-clear naming what was checked |

Rows that never enter the ladder: F3 (the engine proves an ungoverned
installation) is decided in the hook before any verdict and ALLOWS with its
visible line; F13 (ledger absent) means no converted world exists and the
legacy contract applies; F8 (advisor) runs the ladder in full with READY
empty by `ClaimAdmission`'s `no-claim-epoch` refusal and R1's holder rule
(§1.2.0, §1.2.1), so it lands at step 8 or 9 with the OWNED-ELSEWHERE line in
the display; F20 (broken marker) is "absent" at step 3; F21 (malformed state
file) resets and can only add a notice at step 9. `decideRuns` warnings and
green lines compose into the display as today.

## 8. Residuals (honest list, revision 3)

- Legacy single-file world keeps today's block-once ladder (§1.2); plan-stream
  `Next step` and `Waiting on the human` fields are seat-editable text (§1.3).
- Gate runs and mission runners never excuse READY; a goal-named run without
  a governed attempt is not flight; a foreign clone's job records are
  invisible (§2.2, §2.3, KI-34).
- Darwin run records carry no microsecond identity until slice 2b lands the
  field at the three write sites; until then a Darwin seat's governed run is
  relevant through its live waiter only (§2.2).
- F18: the runtime kills the hook before emission. Made improbable by §3.2,
  not closed: the clock is a wall clock, so a backward step of s seconds can
  extend the budget by at most s (§3.2(b)); the exempt steps (builtins, local
  coreutils, the pre-engine `git rev-parse` queries) block only when the
  local filesystem hangs, and an engine binary whose exec itself hangs cannot
  be bounded by the engine (§3.2(c)); without `EPOCHREALTIME` the charge is
  whole seconds rounded up, never under-counted (§3.2(b)).
- F19: `emit_json` cannot write to stdout; recorded as `EMISSION_FAILED`, the
  runtime's default applies (§3.0, §3.1).
- An unrecognised EVENT argument still exits 2 — the hook cannot know it is a
  Stop; that is hook drift, caught by `hooks check` (§3.0 rule 1).
- `up` leaves the Stop path (§3.2(e)): supervision that dies mid-session is
  re-armed at the next SessionStart or Ring 3 tick, not at the next Stop — a
  revival-latency regression, disclosed; every Stop still shows supervision
  state through `health --hook-preview`. The SessionStart `up` keeps its 15 s
  runtime timeout and therefore keeps the pre-existing ownerless-lock-directory
  hazard §3.2(e) traced (`arming.go:684`, `launchOwner`); this design neither
  introduces nor repairs it.
- Ordering window: between slice 1b and slice 2a the verdict refuses at the
  verdict boundary, but the hook's pre-verdict shell exits and rows F1, F2,
  F5 stay fail-open (§10); the root fix, landed first, is what makes the
  engine reachable in that window.
- Hooks disabled or untrusted; Codex and Devin have no live self-check and
  unobserved live Stop delivery (§6).
- An offline remote-mode machine with no READY blocks until the network
  returns, the ledger is in local mode, or a human sets HUMANSTOP; the
  one-verdict race between the fetch's instant and the decision is named, not
  softened (§4).
- Releasing or parking an exhausted claim and re-claiming it starts a fresh
  claim revision whose budget window is the engine's rule (§1.2.1).
- A fenced claim leaves the seat with no ledger move until a human resumes;
  reported as WaitingOnHuman, by design (§1.2.1).
- The relay-minted HUMANSTOP cannot verify its speaker: a human-ratified
  exception (R-47-m0b), named in the marker's provenance and in every display
  that consumes it (§5.2). A hand-forged marker needs two consistent files
  (marker plus proof record); same-user filesystem authority is not closed,
  and the ledger's announcements and locks already live under it (§5.2).
- Under the enrolled-terminal form the human names the target seat
  (ASSUMPTION A3-HUMANSTOP-SEAT, §5.2, §9 ask 9); if overruled, that form is
  withdrawn and the relay form is the only minting path.

## 9. Open asks and gaps (not filled)

1. CLOSED by R-47-m0b word 1: the relayed word may mint HUMANSTOP (§5.2).
2. CLOSED by R-47-m0b word 2: stored budget only; an unbudgeted queued goal is
   a one-time notice (§1.2.1, §1.5).
3. CLOSED by reading: run records carry the goal binding (`run.go:114-134,
   160-187`); the run join is in slice 1a (§2.2, §10).
4. GAP: whether `goalNormApproval` can refuse a claim with an empty approved
   ref was not traced; `ClaimAdmission` includes it verbatim by extraction, so
   READY and claim agree either way, but the R2 false-READY rate is unknown
   until `TestClaimAdmissionAgreesWithClaim` runs (slice 1a).
5. GAP: whether the dispatcher refuses a non-holder session outright was not
   traced (`dispatch_verbs.go:987` only reports `holder`); R1 requires
   `seat.Holder` as the safe default (§1.2.1). If dispatch admits a non-holder
   pair's own claim, R1 is loosened by removing that conjunct — one line, one
   test — and `TestReadyRequiresHolder` is inverted, never deleted.
6. GAP (per-runtime, honest): Devin's honouring of the per-hook `timeout`
   field, Codex's live Stop delivery, and whether either runtime closes the
   hook's stdin after the payload (the bounded `cat` of §3.2(c) row "payload"
   blocks a runtime that never closes it) are unobserved (§6); named as
   residuals rather than checks.
7. DECIDED by the dispatching seat (m0b+main-1788250419-3170380-8a1fb3,
   2026-09-02; closes TVH-R2-SLICE1-HIDDEN-WRONG-ROOT-DEPENDENCY): slice 1 of
   this design is SEQUENCED BEHIND goal supervision-hook-wrong-root landing
   first. That goal's design, `plans/supervision-hook-root-design.md`, is at
   revision 3 (register `records/misc/hook-root-critique-r3.md`, two findings
   from closure — "two folds from closure" in the seat's words); the machine's
   single claim slot (`activeJobLimit=1`, one claim per machine tree-wide,
   `validate.go:250-281`) moves to that goal next, and this goal's slice 1a
   is not dispatched until the root fix is on main. Consequences are stated
   in §3 (sequencing paragraph) and §10 (slice 1a and 1b rows; slice 2's
   separate dependency row is redundant and withdrawn). Not a gap: a
   decision, recorded here so the back matter and §3 name one authority.
8. CLOSED by decision (§3.2(e)): Sol's declared round-2 gap — whether today's
   runtime timeout kills the hook alone or its process group, and hence
   whether `run-bounded`'s group kill would introduce the `up` mid-transaction
   hazard — is moot for the Stop path because `up` leaves it; the hazard
   remains at SessionStart as a pre-existing residual (§8). The comparison
   itself stays unproved, as Sol said it must from repository source alone.
9. ASK (open, for the dispatching seat): ASSUMPTION A3-HUMANSTOP-SEAT (§5.2) —
   under the enrolled-terminal form the human names the target with
   `--seat <machine>+<lineage>`, machine-local, lineage validated. Two answers
   are mechanical: keep it (slice 4a builds it as written) or overrule it
   (slice 4a omits the enrolled form and the relay form is the only minting
   path; `TestHumanstopEnrolledFormRequiresSeatOnThisMachine` is dropped).
   ANSWERED by the dispatching seat (m0b, 2026-09-02, after revision 3
   returned): KEEP the enrolled-terminal form as written — the human at an
   enrolled terminal is the authority and names the seat. Slice 4a builds it;
   the dispatch gate on this ask is lifted.
10. GAP (declared by Sol, round 2, accepted): the slice-1 builder-minute
    estimate is UNSUPPORTED by the job records. §10 states what the records
    do show and re-cuts every slice so a doubled estimate fits one 120 cap
    with the correction round intact; no replacement rate is invented.
11. GAP (platform): `EPOCHREALTIME` was read on this seat's bash 5.2.15 only
    (§3.2(b)); whether every fleet seat's hook bash has it is unobserved. The
    fallback is specified (whole seconds, rounded up, charged), so a seat
    without it is coarse, never fail-open.

## 10. Slices and tests

"240 reserved minutes" is the goal's budget `reservedJobMinutesLimit=240` with
`attemptLimit=10` (`plans/goals/turn-verdict-hardening.md:9`): the sum of
`capMin` over one slice's dispatches. Chain shape per slice: implementer cap
120 + code critic 40 + one correction 40 + re-critique 40 = 240, four
attempts; a second correction round means the slice is re-cut, not
over-reserved. Fable code critique per slice; land with `--chain`. Every slice
stays at or below 240 reserved minutes.

**The estimate is unsupported, and the slices are cut so that does not
matter.** Sol's declared gap is accepted in full. Read for this revision from
the primary checkout's job records (`artifacts/agents/jobs/implementer-*.json`
and `f51-build-1.json`, 32 implementer records dated 2026-09-01/02, 26
completed and 6 failed, wall clock 1–18 minutes against caps of 20–120):
only four records persisted a computed diff (`<job>/rounds/1/diff.patch`),
and none of the four is an authored code diff of any size —
`implementer-0d40e4f087fbb016d455fd35` (207 added lines, all Markdown, 2
minutes: recovered prewritten work, as Sol said),
`implementer-d1947930c9b516cb64dffdb8` (32 added Go lines across two files, 2
minutes), `implementer-0bc4adc1169d0aae26816254` (12 added Go lines, 8
minutes; the completed 120-cap job) and `f51-build-1` (3 added lines, 4
minutes). For the other 28 jobs the branch tip equals the recorded `baseSha`
(delegates do not commit; their diff lands through conformance review, and
those diffs were not retained), so their authored sizes are NOT recoverable
from the records. Revision 2's "15 added lines per builder minute" rate and
its 85-minute slice-1 figure are therefore withdrawn as unsupported, and no
rate replaces them. The cutting rule instead: every slice's implementer
estimate is held at or below 60 builder minutes, so that a DOUBLED estimate
(120) still fits the 120 cap and the chain keeps its one correction round;
revision 2's slice 1 (85) is re-cut into 1a and 1b, slice 2 (80) into 2a and
2b, slice 4 (75) into 4a and 4b; slice 3 (45) stands. The line counts below
are the design's own tally of what each slice names, not a duration claim.
The first landed slice (1a) is the first authored-diff datum this lane will
have: its job record, wall clock and retained `diff.patch` are read and the
remaining estimates restated in this section before slice 1b is dispatched.

Sequencing (§9 ask 7): goal supervision-hook-wrong-root lands first; slice 1a
is not dispatched before it is on main. Line numbers below are today's
(`supervision-hook.sh` at bb3a55cd); the root fix replaces lines 23-31 and
50-66 of the hook (its Decision 4 rows) and leaves 232-247, 274 and 306-320
in place, so the hook lines this design touches are stated against the
post-root-fix file by today's numbers of the lines that survive. Where §2.2,
§3.0 and §3.2(e) say "slice 2", they mean the pair 2a+2b: the static
one-writer test, the `hook-single-response` fixture and the removal of `up`
from the Stop path are 2a; the Darwin `pidStartedAtExactMicro` field on
`run.Record` is 2b. Where §1.2.1 and §3 say "slice 1a", they mean the row
below. Sections 1–6 are not re-folded by this round.

| Slice | Content and work breakdown (builder minutes, ≤ 60) | Go tests and fixtures (new) | Existing tests, new expectation |
| --- | --- | --- | --- |
| 1a — the predicate, relevance, and their proofs (est. 60; cap 120; DEPENDS on supervision-hook-wrong-root landed on main, §9 ask 7; touches no hook line) | `ClaimAdmission` + `ClaimRefusal` + `MachineQuotaAllows` + Mutate reorder + `OwnPair` export (~120 lines, 12); `readywork.Frontier` R1, R2, R3 in the SET form of §1.2.1 with `X(g)`, `t′(g)`, the full re-admission over `t′(g)`, the `Move` rendered in the exact `goalsync_mutations.go` syntax, WaitingOnHuman and Reasons (~260 lines, 18); `readywork.Relevant` over jobs AND runs with exact identity, `dispatch.IdentityRefOf` export, `RunFact` fields, `scan.go` full-ref fix (~180 lines, 14); package tests (~400 lines, 16) | `TestClaimAdmissionAgreesWithClaim` (table-driven over every `Rule`, plus epoch 0 and the quota with the arc exception); `TestClaimReplayReturnsAlreadyAppliedBeforeAdmission`; `TestClaimAdmissionRefusesZeroClaimEpoch`; `TestReadyClaimedAdmissibleForThisPairOnly`; `TestReadyRequiresHolder`; `TestReadyQueuedClaimableRequiresStoredBudgetAndFreeQuota`; `TestReadyHeldReleasableNamesParkOrReleaseByOrigin`; NEW two-arc fixture `TestReadyHeldReleasableNamesEveryQuotaBlockingClaim` (§1.2.1: H1 and H2 held exhausted by `m+A` in arc `A`, queued `g` with a complete stored budget in arc `B` → the Move names BOTH parks then the claim; variant H1 in `A`, H2 in `B`, `g` in `B` → H1 alone; variant seat `m+B` holds H3 in arc `C` → `g` not READY for `m+A`, Reasons names H3 and `m+B`; variant H1 human-origin → `release`; decisive assertion: the rendered Move is executed line by line through the REAL verbs against a copy of the fixture ledger, every line `OutcomeConfirmed`, `g` claimed by `m+A` afterwards); NEW exact-syntax check `TestReadyMoveLinesParseUnderParseSyncFlags` (every rendered park, release and claim line is tokenized and fed to the real `parseSyncFlags`, `goalsync_mutations.go:104-159`: no error, `--root`, `--id`, `--lineage` present, `--because` present on park only, no positional id, no `--then`); `TestReadyExcludesFencedClaimAsWaitingOnHuman`; `TestReadyExcludesOtherPairOnSameMachine`; `TestRelevantJobJoinsGoalAndBinding`; `TestRelevantJobRequiresNativeExactIdentity` (legacy seconds-only record → not relevant; reused pid same second → Dead); `TestRelevantJobRequiresLiveProbeOrLiveWaiter`; `TestRelevantRunJoinsGoalRevisionAndLineage`; `TestRelevantRunRequiresLaunchingOrRunning`; `TestRelevantUngovernedGoalNamedRunIsNotFlight`; `TestRelevantIgnoresSupersededBinding`; `TestRelevantIgnoresMainIdMismatchOnOwnClaim` | none (no verdict behaviour changes in 1a) |
| 1b — the verdict refuses (est. 55; cap 120; follows 1a) | verdict verb: `--caller-pid` flag, seat via `ClassifyVerbAt` (§1.1), ladder §7 steps 1–2 and 4–9 WITHOUT the marker step, no block-once for READY or open-work, `scan.Busy` display-only, F7/F12/F14/F15 verb-side, stopblock text (~180 lines, 14); hook lines on top of the root fix: line 274 gains `--caller-pid "$identity_pid"` beside `--main-id`; lines 306-320 emit `{"decision":"block",...}` for F10 instead of `systemMessage`; the advisor early exit at 232-247 is removed (F8) (~15 lines, 3); verdict tests, two-seat fixtures, specimen replays, renames, hook fixtures (~480 lines, 24). Specimen claim, honestly: 1a+1b refuse all three specimens AT THE VERDICT BOUNDARY; at the DEPLOYED Stop boundary once the root fix is in — which the sequencing guarantees, since 1a cannot start before it — the hook reaches the verdict on every fleet seat, but its pre-verdict shell exits and rows F1, F2, F5 stay fail-open until 2a (§8 ordering window) | `TestReadyBlocksEveryStopWithoutMemory` (five Stops, five blocks); `TestOpenPlanWorkBlocksEveryStop`; `TestBusyMissionDoesNotExcuseReady`; `TestUnreadableBlocks`; `TestDegradedBlocks`; `TestFrontierUnknownBlocks`; `TestIdentityUnknownBlocks`; two-seat fixtures `TestTwoSeatsOneMachine_SeatAFlightDoesNotExcuseSeatB` and `TestTwoSeatsOneMachine_SeatBIsNotToldSeatAsGoalIsReady` (one bed, machine `m`, lineages `A` and `B`; A holds a claim with a live relevant job: A allows, B has no READY and is judged on notices only; then B holds nothing and a budgeted queued goal exists: B's `ClaimAdmission` fails on `machine-quota` only → B is not READY and A's claim is not B's R3); specimen replays `TestSpecimen1_M3HoldBlocks` (claimed admissible goal, no jobs, two Stops both block), `TestSpecimen2_M0bFenceStopBlocks` (the 2026-09-01 20:30Z ledger shape: the pair holds no claim, thirty-plus queued goals carry complete stored budgets → R2 block; variant: the pair holds an unfenced budget-breached claim → R3 block naming the SET Move then the claim; variant: the claim is `StopFence`d → WaitingOnHuman, not blocked on that ground), `TestSpecimen3_M0bBoardStopBlocks` (the 2026-09-02 05:00Z shape: every pair claim released, `account-provenance` queued with its stored budget → R2 block; `goal next` output irrelevant); hook fixture `stop-hook-monitor` second-Stop assertion inverted; hook fixture for F10 emitting `decision:block` | `TestTurnVerdictConvertedClaimHasTheFloor` → `…ClaimBlocksEveryStop`; `TestTurnVerdictConvertedQueueProdsOnce` → `…UnbudgetedQueueProdsOnce` (unchanged behaviour, renamed for the reason) plus sibling `…BudgetedQueueBlocksEveryStop`; `TestClaimedSessionReblocksOnceWhenTheSharedQueueChanges` → `TestClaimedReadyGoalBlocksEveryStopAndQueueChangeIsNoticedOnce`; `TestClaimedSessionBaselinesAnUnchangedQueueWithoutFalseChange` → asserts display text only, `ShouldBlock` true throughout; `TestPrecedenceLadder` → `TestPrecedenceLadderFailClosed` (Busy no longer suppresses a converted READY block; Unreadable blocks in both worlds); `TestInventoryFailureVetoes` → `…Blocks`; `TestVerdictDualSlotSequence` (legacy world) unchanged except Unreadable; `supervision-fixtures.sh:1553-1555` "refused the same open work twice" → must refuse twice, settled step must allow |
| 2a — one emitter, the entry clock, the bounded hook (est. 55; cap 120; follows 1b. The separate "depends on supervision-hook-wrong-root" row of revision 2 is REDUNDANT and withdrawn: the dependency is carried by 1a, which 2a follows) | hook restructure per §3.0 on top of the root fix — the fixed head (lines 1-2: `hook_entry_us`, `on_err`, `emit_json`, `block_json`, `on_exit`, both traps before the checks at 18-19), F1 at 26-31 and P2/F5 at 32-40 → `block_json`, the root design's silent-exit rows → `block_json` (P6: F2, F4), payload `cat` at 42-44 bounded, line 43's trap deleted, P8–P11 left to the ERR trap, `emit_stop_payload` calling `emit_json`, no `exit` before a successful `emit_json` (~120 lines shell, 15); `util run-bounded --deadline-ms --kill-grace-ms` on `boundedexec` with the new `KillGrace` field (~60 lines, 6); `elapsed_ms` and `bounded` and the §3.2(c) cap table applied to every engine call (32, 67, 73, 86, 92, 97-98, 109, 116-117, 122-125, 131-135, 143, 161, 166, 168-170, 221, 223-224, 249, 256, 258, 265, 274, 278-280, 297-304) and to the post-emission calls (185-211, 204, 244, 323); `up` and `up_failure` removed from the Stop path (148-159, §3.2(e)); `util now-ns` no longer called by the hook (~90 lines shell, 10); `StopHookBudgetSec`, `runtime stop-budget`, three JSON files to 20, `hooks budget` verb, `up` HOOK_CHECK/HOOK_DRIFT lines per §6 (~120 lines, 10); tests and fixtures (~260 lines, 14) | `TestRunBoundedKillsProcessGroupAndExits124`; `TestRunBoundedKillGraceAbandonsDStateChild` (a child that ignores SIGKILL is simulated by a stub reaper: the verb returns within grace); `TestHooksBudgetMatchesDeclarationForEveryShippedConfig` (three files); `TestRuntimeStopBudgetVerb`; `TestUpPrintsHookCheckResidualForCodexAndDevin`; static `TestHookStopPathHasOneStdoutWriter` (§3.0 rule 7); NEW static `TestHookEntryClockPrecedesEveryCommand` (the `hook_entry_us=` assignment is the first statement after `set -euo pipefail`, before the first command substitution or external command in the file); NEW fixture `hook-single-response` (§3.0: a Stop under F1 no engine at the world, F2 staged outside any git repository, F5 a stub engine whose `runtime list` prints nothing, a trap exit from an unwritable `TMPDIR` so `mktemp` fails, and one ordinary READY block; each asserts stdout is exactly one line, that line is one JSON object whose `decision` is `block`, and the exit status is 0); NEW fixture `hook-clock-at-entry` (§3.2(b): a PATH-stubbed `git` whose `rev-parse` sleeps 7 000 ms before answering, a stub engine that records the `--deadline-ms` it receives; asserts the recorded verdict cap is at most 7 500 ms — the sleep was charged although the query is exempt from bounding — and one object was emitted; named ceiling 30 s scaled); fixture `hook-budget-verdict-hang` (a stub engine whose `report turn-verdict` sleeps past its cap; asserts one F17 block object within 16 000 ms of hook entry, replacing revision 2's `up`-hang fixture, since `up` left the Stop path); hook fixtures: F7 emits `decision:block` | none inverted |
| 2b — the verb bounds itself (est. 45; cap 120; follows 2a) | `goalGitContext`/`gitInContext`/`FetchAdvanceContext`/`ProjectContext`/`loadTreeContext` on `exec.CommandContext` with the process-group kill and `WaitDelay` 500 ms; `report.Scan` moved inside the verb's phase sequence; `--deadline-ms`, `D − 700`, the cooperative check at each boundary, F17 verb-side naming the phase; `withLockDeadline` and F11 as a block (`state-unavailable`) instead of an exit (~200 lines, 18); Darwin `pidStartedAtExactMicro` on `run.Record` at the three write sites (`run/verbs.go:140, 223, 321`) and in the readers (~40 lines, 4); tests (~260 lines, 14) | `TestVerdictDeadlineExceededBlocksNamingPhase`; `TestGoalGitContextKillsHungChild` (a fixture git wrapper that sleeps); `TestScanRunsInsideVerdictDeadline`; `TestStateFileLockTimeoutBlocksStateUnavailable` (F11: a goroutine holds the verdict-state flock; the verb returns `ShouldBlock` with `state-unavailable`, exit 0); `TestVerdictFloorDecomposition` (with `--deadline-ms 3000` the phases get 2 300, the marker reserve is 600, marshal 100: asserted from the verb's own accounting output); `TestRunRecordCarriesDarwinExactIdentity` (Darwin build tag) | none |
| 3 — freshness (est. 45; cap 120; follows 2b) | bounded fetch as verdict phase 2 under `min(4000 ms, 40% of remaining)`, `SyncLocal` proof, F16, display and ladder step 8 (~120 lines, 12); tests (~250 lines, 15); documentation of the cursor's withdrawal in the delivery contract (5) | `TestFreshnessLocalModeIsFresh`; `TestFreshnessBoundedFetchSuccessAllowsNoReady`; `TestFreshnessFetchTimeoutBlocksNoReady` (a fixture remote that never answers); `TestFreshnessFetchRefusalBlocksNoReady` (foreign ledger); `TestFreshnessStaleBoardStillBlocksOnReady`; `TestFreshnessReadyComputedOverFetchedTree` (a remote claim removes the item); `TestFreshnessNoTimeWindowExists` (a fetch that succeeded one second ago in another process does not make this verdict fresh); `TestFreshnessNotRequiredForReadyAndRelevant` (§7 step 7: a live relevant job on a stale board allows) | `Project` staleness banner tests unchanged |
| 4a — minting HUMANSTOP (est. 45; cap 120; follows 3; §9 ask 9 answered KEEP) | `goal humanstop` verb; `AuthorizesHumanstop`, `TemporaryHumanstopFor`, `RecordHumanstopProof` beside the existing authorizers; the seat derived from `lease.ClassifyVerbAt(root, metasystemRoot, os.Getppid())` and never from flags or `METASYSTEM_OWNER_LINEAGE`; the relay form (MAIN only, `--seat` refused, every other class refused by name) and the enrolled form under A3 (`--seat` required, machine-local, `validLineage`); the marker with provenance and `proofOpid` (~180 lines, 16); tests (~260 lines, 14) | `TestHumanstopRequiresValidForProofOrRelay`; `TestHumanstopRelayRecordsProvenanceVerbatim` (kind, relayedBy, recordedWord, reviewBy, ruling `R-47-m0b`, and the proof's `TemporaryWordRuling` departure); `TestHumanstopRelayRefusesShortWordOrPastReviewDate`; `TestHumanstopRefusesLeaseHumanClass`; NEW forged-lineage case `TestHumanstopRelayIgnoresCallerLineage` (the invoker sets `METASYSTEM_OWNER_LINEAGE` and passes `--lineage` naming another seat: the flag is unknown to the verb and refused; with the environment alone the marker is written for the CLASSIFIED seat, `machine`, `lineage` and `relayedBy` all equal it, and no marker file for the named seat exists); NEW cross-seat case `TestHumanstopRelayRefusesSeatFlag` (`--seat` under the relay form → refusal "the relay form mints only for the invoking seat", no file written); NEW `TestHumanstopRefusesNonMainClassByName` (DELEGATE, STEWARD, SUPERVISION, ADAPTER-SUPERVISOR, empty, and a classification error: each refused with its class in the message, no file written); `TestHumanstopEnrolledFormRequiresSeatOnThisMachine` (enrolled proof, `--seat` naming another machine → refused; naming this machine → marker with `seatNamedBy = enrolled-terminal`, no `relayedBy`); `TestHumanstopProofRecordScopedToHumanstop` (`recordProof` refuses the proof under `goal resume`'s action and accepts it under `goal humanstop`) | none |
| 4b — consuming HUMANSTOP (est. 40; cap 120; follows 4a) | compare-and-consume at the marker phase under `humanstop/.lock` with `withHumanstopLock` at 500 ms, `--turn-key`/`--attempt-seq` flags on the verb and on hook line 274, the `proofOpid` check, class-A wiring per §5.3's set, display and audit line, the `hooks.log` history line, pruning after 30 days (~160 lines, 14); `--hook-schema 2`/`schemaVersion 2` and the F10 mismatch (~30 lines, 4); tests and a hook fixture (~300 lines, 18) | `TestHumanstopConsumedByExactlyOneOfConcurrentStops` (two goroutines under the real marker lock); `TestHumanstopConsumesOnlyAgainstClassABlock` (an allowed Stop leaves the marker unconsumed; a class-B block never reads it; an unwatched-work block never reads it, §7 step 6); `TestHumanstopRescuesStateFileFailure` (F11); `TestHumanstopBoundToWorldPairAndSession`; `TestHumanstopExpiredIsIgnoredAndNamed`; `TestHumanstopUnprovenMarkerAuthorizesNothing` (`proofOpid` missing, or recorded under another action, or a proof form not matching `provenance.kind`); `TestHumanstopConsumedBeforeAllowSurvivesCrash`; `TestHumanstopNeverConsumedAtSessionStart`; NEW marker-lock-cap assertion `TestHumanstopMarkerLockCapFitsMarkerReserve` (a goroutine holds `humanstop/.lock`; the marker phase returns "marker busy; not consumed" and the marker is unconsumed; elapsed in the marker phase is below 600 ms; and `withHumanstopLock`'s deadline constant is asserted equal to 500 ms so that §3.2(d)'s `M = 600` holds by construction); `TestHookSchemaMismatchBlocks`; hook fixture: marker set through the relay form by the fixture seat, one Stop allowed with the relayed audit line in the display, next Stop blocks again | none |

Every fixture wait carries a named ceiling per the suite rule; no benchmarks
(R-31).

## 11. Self-grade (revision 3)

Confidence: high that slices 1a and 1b refuse all three specimens as replayed
against the ledger facts read in §0 — each is a READY clause (R1 for specimen
1, R2 for specimens 2 and 3 as the history shows their pair holding no claim)
with no relevant flight, and every escape they used (block-once, Busy
suppression, fail-open degraded paths) is removed by name at a cited line;
and that, with the root fix landed first (§9 ask 7), the same refusal is
delivered at the deployed Stop boundary for those three shapes, because the
hook then reaches the verdict on every fleet seat. Moderate on R2/R3's
false-refusal rate: `ClaimAdmission` is an extraction called at the same
point of `Mutate` as its rules and cannot drift, but gap 4 leaves the norm
approval untraced and gap 5 leaves R1's holder conjunct a safe default — a
dispatcher that admits a non-holder pair would make R1 under-report, never
over-block. Moderate on the R3 SET form (§1.2.1): the rule is stated over the
current tree and the ledger-after-Move, and the two-arc fixture proves the
rendered Move by executing it through the real verbs, but the false-READY
rate of the full re-admission over `t′(g)` is unmeasured until that fixture
runs. Moderate on HUMANSTOP seat authority (§5.2): the seat comes from the
same authenticated classification the verdict uses, over the invoker's
parent pid, so a caller-controlled lineage cannot mint for another seat; what
is not traced is whether every shell a seat mints from (a tool call of the
main session, a subshell) classifies MAIN through `classify.go:324-342`'s
ancestry walk — a seat that classifies wrongly is refused, never granted, so
the failure mode is a human word that cannot be recorded, surfaced by the
refusal's class name.

Weakest claim: §3.2's Stop-budget construction. It no longer rests on a sum
of caps or on `up`'s idempotence (both withdrawn); it rests on three things
none of which has run yet: that `run-bounded` bounds every non-exempt call
with the allowance `A` subtracted from each cap, that the extra engine exec
per bounded call (about twenty-five calls on a full Stop path, tens of
milliseconds each, charged to elapsed only at the next clock read) stays
inside the 4 000 ms the allowance does not charge, and that the exempt
steps' "cannot block unless the local filesystem hangs" argument holds on the
fleet. The `hook-clock-at-entry`, `hook-single-response` and
`hook-budget-verdict-hang` fixtures are the tests; until they run, F18 is a
disclosed residual, not a closed row.

Reject this design if any of these is observed:
- `TestReadyBlocksEveryStopWithoutMemory` or a hook fixture shows a second
  unchanged Stop allowed while a READY item exists with no relevant live job
  or run and no consumed HUMANSTOP.
- `TestReadyHeldReleasableNamesEveryQuotaBlockingClaim` shows a rendered
  Move the engine refuses at any line, or a second Stop repeating a Move that
  did not clear the quota — the R3 SET form would then be wrong, not the
  engine.
- `TestReadyMoveLinesParseUnderParseSyncFlags` rejects any rendered line.
- `TestClaimAdmissionAgreesWithClaim` shows `ClaimAdmission` and `Claim`
  disagreeing on any refusal or on the replay case.
- The two-seat fixtures show a refusal loop neither seat can lawfully exit.
- `TestRelevantRunJoinsGoalRevisionAndLineage` or a Linux seat's governed run
  on its READY goal is refused by slice 1a+1b.
- `hook-single-response` shows two objects, a non-block object, or a
  non-zero exit on F1, F2, F5 or the trap exit.
- `hook-clock-at-entry` shows the world-mapping delay uncharged (a recorded
  verdict cap above 7 500 ms), or `hook-budget-verdict-hang` shows no block
  object within 16 000 ms of hook entry, or the runtime's 20 s timeout
  killing the hook before emission.
- `TestHumanstopRelayIgnoresCallerLineage`, `TestHumanstopRelayRefusesSeatFlag`
  or `TestHumanstopRefusesNonMainClassByName` shows a marker written for any
  seat other than the classified invoking one, or a `relayedBy` that differs
  from it.
- `TestHumanstopMarkerLockCapFitsMarkerReserve` shows the marker phase
  exceeding 600 ms or a busy lock consuming the marker.
