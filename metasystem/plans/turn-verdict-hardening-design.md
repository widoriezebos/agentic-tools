# Design: turn-verdict hardening — a seat cannot end its turn on ready work

Goal: turn-verdict-hardening (plans/goals/turn-verdict-hardening.md, revision 3,
priority-1). Author: implementer delegate tvh-design-2 under dispatch by
m0b+main-1788250419-3170380-8a1fb3, 2026-09-02, worktree at commit 56be0985.
Requirements: the eleven findings of records/misc/seat-stop-analysis-critique-r1.md
(cited by id below); the three specimens of records/misc/seat-stop-analysis.md.
Wido's order, verbatim: "we need machinery (not you, your behaviour, yourself but
deterministic Go code) that should make this impossible or at least give us the
highest chance of this never happening again."

Every seam cited here was read in this worktree; line numbers are at 56be0985.

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
| fail-open | hook `26-31` (engine missing → exit 0), `65` (no git toplevel → exit 0), `62-63` (session-env failure → exit 0), `306-320` (verdict I/O error → `systemMessage`, never `decision:block`); verdict `384-387` (Unreadable → display only), `439-440` (degraded → display only) | every unknown becomes an allowed exit |
| relay-minted HUMANSTOP | `internal/humanauthority/authority.go:139-157, 195-201`: the temporary word pair "cannot verify who supplied the words" | a seat could mint its own bypass (there is no HUMANSTOP today at all; the analysis proposed relay minting) |

Identity facts the design relies on: ownership of a claim is the pair
machine+lineage (`internal/goal/verbs.go:176-181` `ownPair`; a second lineage
on the machine is refused by name at `447-449`); the quota is one claim per
machine tree-wide (`internal/goal/validate.go:250-281`); a queued goal claims
only with a complete stored or supplied budget (`verbs.go:163-174`); the
dispatcher's side-effect-free admission evaluates claimed goals of one
machine+lineage pair (`internal/dispatch/admission.go:37-107`); job records
carry `goalId`, `goalRevision`, `mainId`, `pid`, `pidStartedAt`, `instanceTag`
as immutable fields (`internal/dispatch/record.go:61-73`, `attest.go:131`) but
no owner lineage; the announcement carries `ownerLineage` and is deleted at
retirement (`internal/lease/classify.go:21-40`, `verbs.go:214-234`); the
projection reads offline and only banners staleness past thirty minutes
(`internal/goal/project.go:26-31, 66-77`); the hook's Codex Stop budget is five
seconds (`scripts/enforcement/codex-hooks.json`), Claude's also five
(`claude-code-hooks.json`). Specimen 3's goal `account-provenance` was queued
with a complete stored budget (read: `plans/goals/account-provenance.md:3,9`).

## 1. Closure 1 — READY, and no block-once for it

### 1.1 The seat

A seat is the pair `(machine, lineage)`. `machine` is
`goal.ResolveMachine(root)` (`internal/goal/actor.go:21-28`, the enrolled
nickname). `lineage` is the announcement's `ownerLineage`, or its `mainId` when
`ownerLineage` is absent — the same default the lease applies
(`internal/lease/claim_test.go:78-79`, `lease.go:131-140`
`announcementLineage`). The hook already resolves the announcement through
`lease classify` (`supervision-hook.sh:130-137`); it gains one field read,
`announcement.ownerLineage`, and passes `--lineage <value>` to the verdict verb
beside the existing `--main-id`. An empty lineage is IDENTITY UNKNOWN (§3).

### 1.2 The predicate, as a function

New package `internal/readywork` (imports `goal` and `dispatch`; neither imports
it, so no cycle; `report` imports it and fills a new typed fact
`ScanResult.Ready`, keeping the settled contract "the scanner fills ScanResult,
the verdict decides", `turnverdict.go:16-19`).

```go
// Frontier decides, side-effect free, over the ACCEPTED projection.
func Frontier(root, machine, lineage string, now time.Time) (Frontier, error)

type Frontier struct {
    State   string      // "ready" | "none" | "unknown"
    Ready   []ReadyItem // sorted: claimed-admissible, queued-claimable, held-releasable
    Reasons []string    // one line per excluded candidate, for the display
}
type ReadyItem struct {
    GoalId   string
    Revision uint64 // f.Revision of the accepted file (the projected revision)
    Binding  uint64 // f.Claimed.Revision when claimed, else 0 (the dispatch binding)
    Clause   string // "claimed-admissible" | "queued-claimable" | "held-releasable"
    Move     string // the one lawful move, rendered
}
```

Inputs: `goal.NewWorld(root)` must be true (the legacy single-file world keeps
today's contract unchanged — §8 residual). `proj := goal.Project(endpoint,
false, now)` — the same read `convertedGoalFacts` makes (`turnverdict.go:476`).
Any error from any call below → `State: "unknown"` with the error text; the
predicate never guesses.

Three clauses, evaluated over `proj.Tree.Live`:

| Clause | Holds when | Calls | Adds (does not reuse) |
| --- | --- | --- | --- |
| R1 claimed-admissible | `f.State == claimed` ∧ `f.Claimed.Machine == machine` ∧ `f.Claimed.Lineage == lineage` ∧ the goal is NOT among `dispatch.EvaluateGoalAdmission(root, lineage, now).Refusals` | `EvaluateGoalAdmission` (fence, BUDGET_UNKNOWN, elapsed/attempt/minute/active breaches, all for this pair) | the two-field pair equality — `ownPair` is unexported and takes an `Actor`; the design exports it as `goal.OwnPair(c *ClaimRecord, machine, lineage string) bool` and `ownPair` becomes a one-line wrapper, so there is ONE pair rule |
| R2 queued-claimable | `f.State == queued` ∧ `goal.ClaimAdmission(root, proj.Tree, id, goal.Actor{Machine: machine, Lineage: lineage}, nil) == nil` | `ClaimAdmission` — NEW exported, side-effect-free function extracted from `claimRequest`'s `Mutate` (`verbs.go:436-470`: live, not already claimed, queued, pin, blockers done, `budgetForClaim(f, nil)` i.e. a complete STORED budget, `goalNormApproval` with an empty approved ref) plus the machine quota from `validate.go:250-281` extracted as `goal.MachineClaimCount(t, machine)`; `claimRequest.Mutate` is rewritten to call `ClaimAdmission` first, so claim and READY cannot drift (SSA-R1-READY-HAS-NO-EXISTING-ADMISSION) | nothing: R2 is exactly "would `goal claim <id>` with no flags succeed for this seat right now" |
| R3 held-releasable | the pair holds ≥1 claimed goal that R1 excludes (fenced or budget-closed) ∧ ∃ queued `g` for which `ClaimAdmission` fails ONLY on the machine-quota rule (every other check passes) | the same two functions; `ClaimAdmission` returns a typed error, `QuotaOnly bool`, so "only the quota" is mechanical | the move: `goal park <held> --then <g>` or `goal done <held>` then `goal claim <g>` — specimens 2 and 3 are this shape (five chains at lawful walls, a budgeted queued goal waiting) |

Excluded by construction and listed in `Reasons`: queued goals without a stored
budget (Sol: selecting limits is human judgment; the queue-change notice of §1.5
still surfaces them once — OPEN ASK 2 in §9), goals pinned elsewhere, goals
behind open blockers, another pair's claims on this machine (seat B is never
told seat A's goal is READY: SSA-R1-READY-OWNERSHIP-SCOPE), any claim on
another machine.

Labels: `goal.Next`'s `requiredLabels` is a dispatcher preference; READY passes
none. `Next` itself (`project.go:90-124`) is not called: it filters by machine,
not pair, and skips budget.

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
is in flight. Claim it, dispatch it, or release a held claim; only a recorded
human stop ends this turn otherwise." `BlockedGoalRevisions` and
`BlockedQueueDigests` are no longer written for READY items; they persist in
the state file only for the two non-ready notices (§1.5) and the legacy world.

Plan open work (`scan.Open`, the `plans/*.md` streams) also loses its
signature memory: `Open ≠ ∅ ∧ no in-flight job record` blocks every Stop
(SSA-R1-BLOCK-ONCE-BYPASS lists it). `OpenWorkSignature` stays in the Verdict
JSON for the hook's display only. Residual: a plan's `Next step` and `Waiting
on the human` fields are seat-editable text — the legacy stream is not hardened
here (§8).

### 1.4 How a turn lawfully ends under READY

Exactly two ways: (a) start RELEVANT flight (§2) — a dispatched job on the
READY goal whose liveness the verdict can prove; (b) a HUMANSTOP consumed by
this very Stop (§5). There is no third way: not an explanation in the
continuation, not a plan edit, not a second attempt. Slow flight is never
converted into failure (R-35): a live relevant job allows the exit however
long it runs.

### 1.5 What keeps session memory

Only the notices that are not READY: the goal-free staleness block
(`BlockedFreeDigests`, `turnverdict.go:427-435`), the queue-change notice for a
claimed session (`ObservedQueueDigest`, `399-410`, now display-only when READY
already blocks), and the unwatched-work block (`BlockedUnwatchedDigests`,
`decideRuns`) — the last is not a READY clause and its escape (arm the printed
watch) is machinery-verifiable through `WaiterLive`; it is unchanged and named
in §8.

## 2. Closure 2 — RELEVANT INFLIGHT

`scan.Busy` no longer suppresses anything. It remains the "STILL WORKING"
display. Relevance is computed by `readywork.Relevant(root, item ReadyItem,
prober) (RelevantFlight, bool)` over the job records of this world
(`<root>/artifacts/agents/jobs/*.json`, the same files `scan.go:104-146`
reads; with the root fix a mapped worktree reads the primary's records).

A job record J is RELEVANT to `ReadyItem` G iff all of:

| Test | Source | Why |
| --- | --- | --- |
| `J.goalId == G.GoalId` | `dispatch.JobRecord.GoalID()` (`jobrecord.go:49`) | the join Sol required (SSA-R1-UNRELATED-INFLIGHT-LAUNDERS-IDLENESS) |
| `J.goalRevision == G.Binding` | `JobRecord.GoalRevision()` (`jobrecord.go:83`); `G.Binding` is `f.Claimed.Revision`, the value `EvaluateGoalRevisionAdmission` bound at dispatch (`admission.go:124-131`) | a job on a superseded binding is not progress on the current one |
| `G.Clause == "claimed-admissible"` | — | only a claimed goal of this pair can have lawful flight; a queued or held goal has none by construction, so R2 and R3 are never excused by flight |
| `!dispatch.TerminalStatus(J.status)` | `record.go:45-52` | terminal records are history |
| LIVE NOW: `identity.AliveRef(prober, Ref{J.pid, J.pidStartedAt}) == Alive` OR `run.LiveWaiter(root, prober, "job", J.jobId, J.mainId, WaiterTarget{StartedAt: J.startedAt})` | `attest.go:131` names the three custody keys; `run/waiter.go:195-207` | `pending`/`running` is a status, not liveness (SSA-R1-INFLIGHT-PROOF-MISSING); a record with no pid and no live waiter is NEVER-STARTED, not flight |

Not required: `J.mainId == this main`. The goal's claim pair is the ownership
proof; a predecessor process of the same lineage that dispatched on this pair's
claim is this seat's flight, which is exactly the resumed-lineage case Sol
said the announcement cannot prove after retirement. `Alive` means the probe
answered alive; `Unknown` (probe failure) is not alive — fail closed.

Harness monitor: a live waiter registered for the relevant job counts as
liveness even when the adapter child's pid is unreadable (a monitor is a
process the seat armed and the verdict can probe).

Recorded blind spots, stated as residuals, not closed here (KI-34):
- Jobs whose records live in another clone's `artifacts/` on the same machine
  (a second checkout of the same ledger): invisible; a seat there is another
  seat, and the quota keeps its claims disjoint, so the only loss is a
  same-lineage job dispatched from a foreign clone — refused, not laundered.
- Non-job processes (a detached script without a run record): invisible, so
  the verdict blocks; the lawful move is to run it as a governed run.
- Run records (`internal/run`): the scan surface read here (`scan.go:195-240`)
  carries no goal binding; runs are NOT relevant flight in slice 1. If a run
  record does carry a goal field, slice 2 joins it the same way; if not, that
  is a named residual, never a silent exemption (gap listed in §9).
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
refusing to guess"). The block reason always names the repair and the one
lawful human exit (§5). Wedge risk, stated: a seat blocked on corrupt state
burns turns until the state is repaired or a human sets HUMANSTOP; the
steward's stalled-idle escalation surfaces the loop; this is the chosen
direction because the alternative was the bypass the specimens took.

Sequencing: the wrong-root fix (`plans/supervision-hook-root-design.md`, goal
supervision-hook-wrong-root) MUST land before slice 1. Under this table an
unresolved identity is a BLOCK, so slice 1 on a wrong-root fleet would wedge
every seat on every Stop. Its silent-exit rows are re-dispositioned below;
its skew and missing-engine lines keep their text, prefixed into the block
reason.

| # | Condition | Detected where | Hook decision |
| --- | --- | --- | --- |
| F1 | engine missing at the resolved world | hook, literal test | BLOCK via literal `printf` JSON; reason: rebuild `bin/metasystem`, or human stop |
| F2 | world identification fails (git query, parse, common-dir shape, mapping, shape check) | hook (root design Decision 2 rows 1-6, were silent exit 0) | BLOCK via literal JSON: "the hook cannot identify its world" |
| F3 | `path state-root` exit 1: the engine proves an ungoverned installation | engine verb | ALLOW with a visible line (proven absence, not a guess) |
| F4 | `path state-root` other failure or empty (engine/hook skew) | hook | BLOCK: skew line as reason |
| F5 | runtime registry query fails or runtime not registered | hook `32-40` (today exit rc / exit 2) | BLOCK JSON; exit 0. Exit codes are never the delivery channel |
| F6 | runtime session-env lookup fails | deleted by the root fix (cwd no longer participates); until then | BLOCK |
| F7 | identity unknown: no runtime ancestor and no recorded main, or `lease classify` empty, or lineage empty | hook `109-137` | BLOCK: "the hook cannot tell whose turn this is; run `metasystem up`" (this is exactly what the wrong root produced on every fleet seat) |
| F8 | caller classified advisor (MAIN, not holder) | hook `232-247` | today: OWNED-ELSEWHERE message, exit 0. Unchanged: an advisor holds no claim authority in this checkout, so it has no READY here by construction; the verdict still runs for display (removes the early exit's bypass of open-work display) |
| F9 | `steward hook-attempt` fails (turn evidence) | hook `84-104` | BLOCK when combined with any other unknown; alone: verdict proceeds, evidence failure rides the reason. Rationale: attempt evidence is the freshness trail, not the decision input |
| F10 | `report turn-verdict` exits nonzero, empty stdout, or unparsable JSON, or `schemaVersion` the hook does not know | hook `306-320` (today `systemMessage`) | BLOCK: "turn-verdict unavailable: <last stderr line>" |
| F11 | flock timeout or state-file write failure inside `TurnVerdict` (`withLock`, `saveVerdictState`) | verb returns error | F10 |
| F12 | ledger `degraded` (machine unenrolled, endpoint, projection error, unreadable tree, legacy parse problems) | `convertedGoalFacts`/`legacyGoalFacts` | verdict `ShouldBlock = true`, `BlockSource = "degraded"` (was display only, `439-440`) |
| F13 | ledger `absent` (pre-adoption, no baseline) | `legacyGoalFacts:504-508` | ALLOW, advisory (proven: no ledger was ever adopted here) |
| F14 | `scan.Unreadable` or `scan.RunUnreadable` non-empty | `decide:384-387`, `decideRuns` | BLOCK, `BlockSource = "unreadable"` (was display only) |
| F15 | `Frontier.State == "unknown"` (READY could not be computed) | readywork | BLOCK |
| F16 | freshness unproven (§4) and `Frontier.State == "none"` | verdict | BLOCK, `BlockSource = "stale-board"` |
| F17 | Stop budget: the verdict's own deadline exceeded | verb, `--deadline-ms` (§3.1) | verb exits 0 with `ShouldBlock = true`, `BlockSource = "deadline"`; the hook emits immediately |
| F18 | Stop budget: the runtime kills the hook before emission | runtime | RESIDUAL: the runtime's default applies (allow). Mitigated by §3.1; recorded, not closed |
| F19 | emission failure (`printf` to stdout fails) | hook `196-201` | RESIDUAL: `hook-complete --outcome EMISSION_FAILED` is recorded; the steward's hook-freshness sees it; the runtime's default applies |
| F20 | HUMANSTOP marker unreadable or malformed | verdict | treated as absent: normal rules (a broken marker authorizes nothing) |
| F21 | verdict state file malformed | `loadVerdictState:645-647` resets today | unchanged: the state holds only non-ready memory now, so a reset can only cause an extra notice, never an allowed exit |

### 3.1 The Stop budget

The hook today runs `up`, `health`, `digest-pending`, `watchdog-report`,
`evidence-gc`, then the verdict — inside five seconds on Codex and Claude.
Decisions: (a) `runtimes.Declaration` gains `StopHookBudgetSec int` (codex 20,
claude 20, devin 20, fake 5) and `metasystem runtime stop-budget <name>` prints
it; `hooks check` (`cmd/metasystem/hooks.go`) additionally asserts the shipped
hooks JSON `timeout` equals the declaration, and both shipped files move to 20
(Claude Code accepts per-hook timeouts; Codex's field is already per-hook).
(b) The hook measures its own elapsed time from entry (`$SECONDS` is too
coarse; `date +%s%N` once at entry) and passes `--deadline-ms` = 60% of the
declared budget minus elapsed to the verdict verb; the verdict runs the
scanner and READY under `context.WithDeadline` and returns F17 when exceeded.
(c) The ceremonies (`up`, `health`, `digest-pending`, `watchdog-report`) run
AFTER the verdict has been computed, each skipped with a fixed "HEALTH unknown"
line once 80% of the budget is spent; the verdict is the one thing that must
finish. F18 remains a residual because a killed hook emits nothing.

## 4. Closure 4 — FRESHNESS

The verdict reads the accepted tree offline (`turnverdict.go:476`,
SSA-R1-STALE-BOARD-ALLOWS-EXIT). Freshness gates only the ALLOW path:

```
allow-on-no-READY requires FRESH; READY from a stale board still blocks
(a remotely claimed goal fails at claim time with a named refusal — that is
the lawful move being refused, not an idle exit).
```

FRESH holds when one of:
- the projection's root record says `SyncMode == SyncLocal` (`project.go:52-63`):
  a single-machine ledger has no remote to be stale against; or
- the sync cursor `<root>/artifacts/agents/goal-sync-cursor.json`
  `{schemaVersion, tip, checkedAt, outcome:"current"|"advanced"}` was written
  by a successful `FetchAdvance` (`internal/goal/fetchadvance.go:30`, both its
  "already at the canonical tip" and advanced outcomes write it; `goal list
  --fetch` and the steward's sync path go through the same function) within
  `FreshWindow = 10m` of `now`, and `tip` equals the projection's `Tip`; or
- neither, and the verdict itself runs ONE bounded `FetchAdvance` under a
  2000 ms deadline (outside the flock, before it) and it succeeds, writing the
  cursor.

Otherwise FRESHNESS UNKNOWN → F16 BLOCK: "the board's freshness is unproven
(last sync <age or never>); run `goal list --fetch`". An offline machine with no
READY is blocked until network or HUMANSTOP — stated, not softened. The cursor
is written atomically (rename) and read under no lock; a torn read is
UNKNOWN.

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
| `setBy` | `{terminalRef, terminalGeneration, checkedAt}` copied from the human-authority proof |
| `consumed` | `null`, or `{at, session, turnKey, attemptSeq}` |

### 5.2 Who may set it

Verb: `metasystem goal humanstop --directive "<words>" [--ttl 8h] [--session
<id>]`. It requires `humanauthority.Prove(root, invokerPID, KernelReader{}, now)`
to return a proof with `ValidFor(root)` true (`authority.go:89-111`: enrolled
terminal ancestry, no agent runtime on the chain, the installed signature set).
The hook's own `lease classify` HUMAN class (`classify.go:369-377`: "no
recognised ancestor and a controlling terminal") is weaker — a pseudo-terminal
allocated by an agent satisfies it — and is NOT accepted for minting. The
temporary relay pair (`TemporaryGoalProof`, `ProveOrTemporaryGoalAuthority`,
`authority.go:195-237`) is REFUSED by this verb: it records words but "cannot
verify who supplied the words" (SSA-R1-HUMANSTOP-RELAY-LAUNDERING). OPEN ASK 1
(§9): Wido may rule otherwise; until a recorded ruling, refusal is the default.
Redirecting work is not HUMANSTOP: `goal park`, `steal`, `set-pin`, `open` are
the human's redirect verbs and leave READY to be recomputed.

### 5.3 Compare-and-consume, bound to one Stop

Inside `TurnVerdict`'s existing flock (`withLock`), after READY and relevance
are known and before any decision is written:

1. Read the marker for `(machine, lineage)`. Absent, unreadable, wrong `world`,
   wrong pair, `runtimeSession` set and ≠ this session, `now ≥ expiresAt`, or
   `consumed ≠ null` → no HUMANSTOP; an expired or foreign marker is named in
   the display, never consumed.
2. Otherwise write `consumed = {at: now, session, turnKey, attemptSeq}` by
   atomic rename FIRST (the hook passes the `turn_key` it already computes at
   `supervision-hook.sh:86` as `--turn-key`, and `hook_attempt_seq` as
   `--attempt-seq`).
3. Then decide `ShouldBlock = false`, `BlockSource = null`, display
   "HUMAN STOP (<setAt>): <directive>".

The flock serializes concurrent Stop calls, so exactly one consumes
(SSA-R1-HUMANSTOP-CONSUMPTION-RACE); the second sees `consumed` and falls
through to the normal rules. If the process dies between step 2 and emission,
the marker is spent and the turn was not allowed: the human sets another
(fail-closed direction). Nothing consumes at session start: the Stop decision
is the only boundary. A consumed marker is retained for the audit trail and
pruned after 30 days with the session state.

## 6. The Stop hook is not exclusive or mandatory

Owned by this item: (a) enrollment — `up` at session start runs `hooks check
--runtime R <live settings> <shipped hooks>` and its drift outcome rides every
Stop display as `HOOK_DRIFT: <detail>` until fixed (the hook's own execution
proves its presence; drift means a timeout, command, or event edit); (b)
version compatibility — the hook passes `--hook-schema 2`; the verdict returns
`schemaVersion 2`; a mismatch either way is F10; (c) the Stop budget
declaration in `runtimes.go` checked by `hooks check` (§3.1). Residuals,
named (SSA-R1-STOP-HOOK-NOT-MANDATORY-OR-EXCLUSIVE): a user-disabled hook, a
Codex project hook not yet trusted, a competing Stop hook returning
`continue:false`, and Codex live delivery still "declared, observation pending"
(`docs/design/turn-verdict-delivery-contract.md:44`). The steward's
stalled-idle escalation stays the detection-after-the-fact backstop behind
this prevention gate.

## 7. The new precedence ladder (converted world)

| Order | Condition | Outcome |
| --- | --- | --- |
| 1 | any F-row unknown (F7, F10-F16) | BLOCK (fail closed) — unless step 2 |
| 2 | HUMANSTOP compare-and-consume succeeds | ALLOW; display the directive |
| 3 | READY ∧ ¬RELEVANT | BLOCK `ready-work`, every time |
| 4 | plan `Open ≠ ∅` ∧ no in-flight job record | BLOCK `open-work`, every time |
| 5 | unwatched work (unchanged block-once) | BLOCK `unwatched-work` once per digest |
| 6 | READY ∧ RELEVANT | ALLOW; "STILL WORKING: <job> on <goal>@<binding>" |
| 7 | no READY, FRESH | non-ready notices with session memory (queue change, goal-free staleness), Busy and WaitingOnHuman display, then the all-clear naming what was checked |

`decideRuns` warnings and green lines compose into the display as today.

## 8. Residuals (honest list)

Legacy single-file world keeps today's block-once ladder; plan-stream fields
are seat-editable text; run records are not joined to goals in slice 1; gate
and mission activity never excuses READY; F18/F19 runtime-side allow on a
killed or emission-failed hook; hooks disabled or untrusted; a foreign clone's
job records; an offline machine blocks until synced or human-stopped.

## 9. Open asks and gaps (not filled)

1. OPEN ASK (Wido): may the temporary relayed word mint HUMANSTOP? Default: no.
2. OPEN ASK (Wido): does an agent-supplied budget make an unbudgeted queued
   goal READY? Default: no (stored complete budget only). Consequence named: a
   specimen whose queued goal lacks a stored budget would be a once-notice, not
   a block. Specimen 3's goal had one.
3. GAP: whether `internal/run` records carry a goal binding was not read in
   this pass; slice 2's builder verifies and either joins runs or records the
   residual.
4. GAP: whether `goalNormApproval` can refuse a claim with an empty approved
   ref was not traced; `ClaimAdmission` includes it verbatim by extraction, so
   READY and claim agree either way, but the R2 false-READY rate is unknown
   until the extraction test runs.

## 10. Slices and tests

Each slice ≤ 240 reserved minutes; Fable code critique per slice; land with
`--chain`. Slice 1 depends on supervision-hook-wrong-root landing first.

| Slice | Content | Go tests (new) | Existing tests, new expectation |
| --- | --- | --- | --- |
| 1 (≤240) | `--lineage`, `--turn-key`, `--attempt-seq`, `--deadline-ms`, `--hook-schema` flags; `readywork.Frontier` with R1 (via `EvaluateGoalAdmission`) and R2/R3 via an interim `ClaimAdmission` that is the extraction of `claimRequest.Mutate` checks + `MachineClaimCount` (the extraction IS slice 1; slice 2 only adds freshness); `readywork.Relevant`; ladder §7; fail-closed rows F1-F17 in hook and verb; stopblock text; runtimes `StopHookBudgetSec` + hooks check + both JSON timeouts | `TestReadyClaimedAdmissibleForThisPairOnly`; `TestReadyQueuedClaimableRequiresStoredBudgetAndFreeQuota`; `TestReadyHeldReleasableWhenQuotaIsThisPairsFencedClaim`; `TestReadyExcludesOtherPairOnSameMachine`; `TestClaimAdmissionAgreesWithClaim` (every refusal `Claim` produces, `ClaimAdmission` produces first, table-driven); `TestRelevantJoinsGoalAndBinding`; `TestRelevantRequiresLiveProbeOrLiveWaiter` (pending, dead pid, unknown probe → not relevant); `TestRelevantIgnoresSupersededBinding`; `TestRelevantIgnoresMainIdMismatchOnOwnClaim`; `TestReadyBlocksEveryStopWithoutMemory` (five consecutive Stops, five blocks); `TestOpenPlanWorkBlocksEveryStop`; `TestBusyMissionDoesNotExcuseReady`; `TestUnreadableBlocks`; `TestDegradedBlocks`; `TestFrontierUnknownBlocks`; `TestDeadlineExceededBlocks`; two-seat fixtures `TestTwoSeatsOneMachine_SeatAFlightDoesNotExcuseSeatB` and `TestTwoSeatsOneMachine_SeatBIsNotToldSeatAsGoalIsReady` (one bed, machine `m`, lineages `A` and `B`, A holds a claim with a live relevant job: A allows, B has no READY and is judged on freshness/notices only; then B holds nothing and a budgeted queued goal exists: B blocks on R2 only if the quota is free — it is not, so B gets the no-READY path and A's held claim is A's R3, not B's); specimen replays `TestSpecimen1_M3HoldBlocks` (claimed admissible goal, no jobs, two Stops both block), `TestSpecimen2_M0bFenceStopBlocks` (pair's claim attempt-fenced, budgeted queued goal → R3 block), `TestSpecimen3_M0bBoardStopBlocks` (five pair claims parked/fenced, `account-provenance` queued with stored budget → R2 or R3 block, `goal next` output irrelevant); hook fixture `stop-hook-monitor` second-Stop assertion inverted; new hook fixtures for F1, F2, F7, F10 emitting `decision:block` | `TestTurnVerdictConvertedClaimHasTheFloor` → `…ClaimBlocksEveryStop` (second call blocks); `TestTurnVerdictConvertedQueueProdsOnce` → `…UnbudgetedQueueProdsOnce` (unchanged behaviour, renamed for the reason) plus sibling `…BudgetedQueueBlocksEveryStop`; `TestClaimedSessionReblocksOnceWhenTheSharedQueueChanges` → `TestClaimedReadyGoalBlocksEveryStopAndQueueChangeIsNoticedOnce` (every Stop blocks; "queue changed" text appears once); `TestClaimedSessionBaselinesAnUnchangedQueueWithoutFalseChange` → asserts on display text only, `ShouldBlock` true throughout; `TestPrecedenceLadder` → `TestPrecedenceLadderFailClosed` (Busy no longer suppresses a converted READY block; Unreadable blocks in both worlds); `TestInventoryFailureVetoes` → `…Blocks`; `TestVerdictDualSlotSequence` (legacy world) unchanged except Unreadable; `supervision-fixtures.sh:1553-1555` "refused the same open work twice" → must refuse twice, settled step must allow |
| 2 (≤240) | freshness cursor written by `FetchAdvance`; bounded fetch in the verdict; F16; run-record goal join or recorded residual (gap 3); `OwnPair` export | `TestFreshnessLocalModeIsFresh`; `TestFreshnessCursorWithinWindowAllowsNoReady`; `TestFreshnessStaleCursorBlocksNoReady`; `TestFreshnessStaleBoardStillBlocksOnReady`; `TestFreshnessBoundedFetchWritesCursor`; `TestFreshnessTornCursorIsUnknown`; `TestFetchAdvanceWritesCursorOnCurrentAndAdvanced` | none inverted; `Project` staleness banner tests unchanged |
| 3 (≤240) | `goal humanstop` verb with `Prove` only; marker; compare-and-consume in the flock; display; pruning; `--hook-schema`/F10 mismatch; `up` HOOK_DRIFT line | `TestHumanstopRequiresValidForProof`; `TestHumanstopRefusesTemporaryRelayProof`; `TestHumanstopConsumedByExactlyOneOfConcurrentStops` (two goroutines under the real flock); `TestHumanstopBoundToWorldPairAndSession`; `TestHumanstopExpiredIsIgnoredAndNamed`; `TestHumanstopConsumedBeforeAllowSurvivesCrash` (consume then abort: marker spent, next Stop blocks); `TestHumanstopNeverConsumedAtSessionStart`; `TestHookSchemaMismatchBlocks`; hook fixture: marker set by an enrolled-terminal fixture proof, one Stop allowed with the directive in the display, next Stop blocks again | none |

Every fixture wait carries a named ceiling per the suite rule; no benchmarks
(R-31).

## 11. Self-grade

Confidence: high that slice 1 refuses all three specimens as replayed — each is
a READY clause with no relevant flight, and every escape they used (block-once,
Busy suppression, fail-open degraded paths) is removed by name at a cited
line. Moderate on R2/R3's false-refusal rate: `ClaimAdmission` is an extraction
and cannot drift, but gap 4 leaves its refusal set untraced, and an
unexpectedly strict norm-approval check would turn "claim it" into a refused
claim under a standing block — the escape then is the human word only, which
is by design but costly. Weakest claim: §3.1's Stop budget — moving the
ceremonies behind the verdict and raising timeouts to 20 s is a hook
re-ordering not yet traced against `up`'s side effects at line 148, and F18
stays open. Reject this design if: a fixture shows a second unchanged Stop
allowed while a READY item exists and no relevant live job and no consumed
HUMANSTOP; or two seats on one machine produce a refusal loop neither can
lawfully exit (a READY item with no lawful move for the seat it names); or
`ClaimAdmission` and `Claim` disagree on any refusal in the table-driven test;
or the fleet's `internal/run` records prove to carry goal bindings that the
slice-1 verdict ignores while a seat's actual progress on its READY goal is
such a run — that would recreate the false refusal the design exists to avoid.
