Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal turn-verdict-hardening)
Date: 2026-09-02

# Goal

Revise metasystem/plans/turn-verdict-hardening-design.md (revision 1, landed;
edit it in place, bump the revision line) to close Sol's nine material
findings in metasystem/records/misc/turn-verdict-hardening-critique-r1.md and
to carry Wido's two words in ruling R-47-m0b (metasystem/memory/rulings.md,
last row). Every finding is closed by changing the design text, or refuted
with the code line that refutes it — never by softening the claim. Keep the
original brief's requirements (metasystem/plans/turn-verdict-hardening-design-brief.md).

# Workspace

The delegate worktree the dispatcher created for this job. Edit exactly one
existing file, the design; nothing else.

# Wido's two words (R-47-m0b, decided — not open any more)

1. Relay counts: a relayed human word through the temporary-human-word path
   MAY mint HUMANSTOP. Rewrite section 5 accordingly: the marker records the
   relay provenance verbatim (who relayed, the recorded word, the review
   date), the audit line names it as relayed, and the design states the
   residual Sol named (the path cannot verify the speaker) as a
   human-ratified exception rather than a hole. Section 9 ask 1 is closed.
2. Stored budget only: READY's queued clause requires the budget already on
   the ledger; an unbudgeted queued goal is a one-time notice. Section 9 ask
   2 is closed with this text.

# The nine findings, and what closing each requires

- CLAIM-ADMISSION-OMITS-AUTHORITY-AND-REPLAY: ClaimAdmission must be the
  proof the claim would succeed — carry the authenticated claim epoch and
  lease-holder authority (bindClaim at metasystem/internal/goal/verbs.go line
  475 in the critic's reading), and sit AFTER the opid replay check so an
  AlreadyApplied replay is not refused. Read the verb; specify the exact
  call order and signature.
- R3-NAMES-ILLEGAL-EXIT: release, park and done all refuse a breach-stopped
  goal (clearClaimBinding); only goal resume, a human transition, clears the
  fence. R3 must therefore EXCLUDE fenced and budget-closed claims from
  READY (they are WAITING-ON-HUMAN, reported not blocked) — or name a move
  the engine actually accepts. Remove the `goal park --then` command that
  does not exist.
- SLICE1-IGNORES-GOAL-BOUND-GOVERNED-RUNS: run records already carry
  goalId, governed.goalRevision, ownerLineage, claimEpoch and liveness. The
  run join moves INTO slice 1. Read metasystem/internal/run to cite the
  fields.
- JOB-LIVENESS-DOWNGRADES-EXACT-IDENTITY: the direct process branch must
  consume the record's full native identity (start ticks plus boot id on
  Linux, start microseconds on Darwin); incomplete exact data is UNKNOWN,
  not alive.
- FRESH-CURSOR-IS-NOT-A-CURRENTNESS-WITNESS: a ten-minute cursor allows a
  stale-board exit for ten minutes. Redesign: the allow path on "no READY"
  requires a bounded fetch attempt in THIS verdict (success → fresh; failure
  → not fresh → block with the reason), or an explicit local-sync mode; no
  time window stands in for a fetch.
- HUMANSTOP-CANNOT-RESCUE-DECISION-OWNER-FAILURES: split the table into
  rows HUMANSTOP can rescue and rows that need the decision owner repaired;
  for the latter name the machinery-owned recovery (the steward's
  escalation, the `up` repair path) instead of a universal "unless
  HUMANSTOP".
- FAIL-CLOSED-TABLE-OMITS-PREVERDICT-SHELL-EXITS: the hook runs set -e with
  unguarded mktemp, cat, mkdir and response construction before any verdict.
  Specify the hook's structure so that the FIRST thing it does is arm a
  trap that emits decision:block on any exit before a verdict is emitted;
  every pre-verdict operation maps to a row.
- STOP-DEADLINE-DOES-NOT-BOUND-EMISSION: context deadlines do not cancel
  report.Scan, Project, FetchAdvance or their git subprocesses, and
  `date +%s%N` is not portable to Darwin. Specify: the block decision is
  emitted BEFORE any unbounded ceremony (ceremonies run after emission or
  are each independently bounded), every git subprocess gets a timeout
  argument or a killed child, the clock is portable.
- RUNTIME-HOOK-CHECK-OMITS-TWO-SUPPORTED-RUNTIMES: three shipped
  configurations (Claude, Codex, Devin); the session-start hooks check needs
  Declaration.SelfCheck which only Claude has. Cover all three timeouts and
  specify the check per runtime or the honest residual per runtime. Read
  metasystem/internal/runtimes/runtimes.go (runtime facts).

Also the critic's gap on slice size: give a work breakdown per slice with
minute estimates against recorded precedent so the 240-minute claim is
arguable; if slice 1 does not fit after adding the run join, re-cut into
four slices — the first must still refuse all three specimens.

Self-grade per the house rule.

# Constraints

Wall-clock budget: 45 minutes. Design only. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file.

# Gap Rule

stop and report a gap; never fill it silently.
