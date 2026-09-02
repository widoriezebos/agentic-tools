Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal turn-verdict-hardening)
Date: 2026-09-02

# Goal

Author the design for goal turn-verdict-hardening (read
metasystem/plans/goals/turn-verdict-hardening.md first, then the incident
record metasystem/records/misc/seat-stop-analysis.md and Sol's critique of it,
metasystem/records/misc/seat-stop-analysis-critique-r1.md — the eleven
findings there are your requirements list). Wido's order, verbatim: "we need
machinery (not you, your behaviour, yourself but deterministic Go code) that
should make this impossible or at least give us the highest chance of this
never happening again". "This" is a seat ending its turn while ready work
sits on the board and nothing progresses it; three specimens are in the
record.

# Workspace

The delegate worktree the dispatcher created for this job. Read anything;
write exactly one NEW file, turn-verdict-hardening-design.md, in the
metasystem plans directory.

# The object under judgment

The gate already exists and the specimens walked through it. At the Stop
event metasystem/scripts/agents/supervision-hook.sh calls `report
turn-verdict`; the decision is metasystem/internal/goal/turnverdict.go
(Store.TurnVerdict, decideRuns, decide) with the block text in
metasystem/internal/report/stopblock.go and the scanners in
metasystem/internal/report/openwork.go, metasystem/internal/report/scan.go,
metasystem/internal/report/scanjobs.go and
metasystem/internal/report/runningwork.go — read decide() closely: open work
blocks ONCE per OpenWorkSignature; the goal ladder blocks once per
revision/digest (BlockedGoalRevisions, BlockedQueueDigests,
BlockedFreeDigests, BlockedUnwatchedDigests); scan.Busy (any active
checkout) suppresses every block; and the hook's degraded paths allow the
exit.

# What the design must specify (five closures, in this priority)

1. NO BLOCK-ONCE FOR READY WORK. Define READY precisely: the goal ledger
   holds work this seat (machine + owner lineage pair, exactly as claim
   admission scopes it — see metasystem/internal/goal/verbs.go and the
   ledger validation that refused a second claim per machine) can lawfully
   claim or advance now. State the admission predicate as a testable
   function, not as "reuse admission": which existing functions it calls,
   which it cannot, and what it adds. While READY holds and no RELEVANT
   flight exists, every Stop is refused — the signature memory stays only
   for the non-ready clauses (goal-free staleness, queue-change notices).
   Say how a seat lawfully ends a turn under READY: only by starting
   relevant flight or by a HUMANSTOP (closure 5).
2. RELEVANT INFLIGHT. Replace "any Busy checkout / any same-session run" by
   flight joined to the ready frontier: a non-terminal job whose record
   names the ready goal and its current revision and whose liveness is
   proven (a pid or lease that is alive now, not a pending status), or a
   harness monitor registered for such a job. Read
   metasystem/internal/lease/verbs.go on what the announcement proves and
   when it disappears; state the recorded blind spots (cross-checkout
   worktree jobs, non-job processes) honestly as residuals rather than
   claiming totality.
3. FAIL CLOSED. A complete outcome table for: engine missing, root not
   resolved or wrong root (goal supervision-hook-wrong-root and its design
   metasystem/plans/supervision-hook-root-design.md — either sequence
   behind it or carry its resolution), runtime lookup failure, ledger
   unreadable or degraded, stale accepted-tree projection, flock timeout,
   the Codex five-second Stop budget, identity unknown, and emission
   failure. Every row names the hook's decision. The precedent for
   "cannot read, refuse to guess" is the steward; here the analysis
   proposed fail-open and Sol refuted it (SSA-R1-FAIL-OPEN-IS-A-BYPASS):
   decide, with the wedge risk stated — a seat blocked on corrupt state
   needs a path out that is not "guess allow" (the human word, closure 5,
   is the candidate).
4. FRESHNESS. The turn verdict reads the accepted-tree projection offline
   (SSA-R1-STALE-BOARD-ALLOWS-EXIT). Specify the freshness proof or cursor
   the verdict requires before "no READY" may allow an exit, and the
   outcome when freshness cannot be established.
5. HUMANSTOP. A durable, single-use human stop marker: who may set it
   (human-classified caller only; state what the caller classification in
   the hook and the steward already proves and refuse the temporary relay
   path per SSA-R1-HUMANSTOP-RELAY-LAUNDERING unless Wido rules
   otherwise — write that as an open ask, not a decision), its fields
   (world, machine, lineage, runtime session, directive, expiry), and the
   atomic compare-and-consume rule binding it to the one Stop decision it
   authorizes (SSA-R1-HUMANSTOP-CONSUMPTION-RACE).

Also: the Stop hook is a valid re-prompting point but not exclusive or
mandatory (SSA-R1-STOP-HOOK-NOT-MANDATORY-OR-EXCLUSIVE). State what the
item owns (hook enrollment check at `up`, version compatibility) and what
stays a residual. The runtime facts are in
metasystem/internal/runtimes/runtimes.go (runtime facts).

# Slices and tests

Cut the build into slices of at most 240 reserved minutes each, slice 1
being the three closures that would have caught all three specimens
(block-once removal, relevant INFLIGHT, fail-closed table). For every slice
list the Go tests, including two-seat fixtures on one machine (seat A's
flight must not excuse seat B; seat B must not be told seat A's claimed goal
is its READY work) and the three specimens replayed as fixtures against the
new verdict — each must now refuse. Existing tests that encode block-once
must be named with their new expectation.

Self-grade per the house rule: confidence, weakest claim, reject condition.

# Constraints

Wall-clock budget: 40 minutes. Design only; edit nothing but the design
file. R-31: no benchmarks in this VM. Prose stays tight — tables for the
outcome table and the slice plan.

# Expected Return

Version-2 implementer JSON; diffBoundary lists exactly the one new design
file you created.

# Gap Rule

stop and report a gap; never fill it silently.
