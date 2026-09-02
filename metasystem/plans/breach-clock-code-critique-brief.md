Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal breach-clock-and-budget-honesty)
Date: 2026-09-02

# Goal

Two-layer implementation critique of the breach-clock build (job
breach-build-2, Sol; commit 0d8e47ef in its worktree, diff.patch in its
round evidence). The design is metasystem/plans/breach-clock-and-budget-honesty-design.md
at revision 6a, certified by Sol's five registers
metasystem/records/misc/breach-design-critique-r1.md to -r5.md, with the
build-gap decisions in metasystem/records/misc/breach-build-1b-gaps.md
(the only-claim invariant at the quota's unit and its exact wording;
resume from the claimed shape keeps the claim; delivery.go unchanged). First
conformance of the diff against the three Fix sections and the proof plan,
then adversarial defect review. The standard is Wido's: hard deterministic
machinery; no refusal weakened, no guarantee narrowed to make a test pass.

# Attack surface

- Fix 1: `rebindClaimKeepEpisode` writes the third episode key exactly as
  the design's mechanics sentence says (live obligation's revision when one
  is live; INHERIT the prior binding's value when none is; never 0 over a
  non-zero value); `bindClaim` and `clearClaimBinding` start it at 0; the
  render, parse and `ValidateClaimRevision` rules; the projection's
  eligibility rule in metasystem/internal/dispatch/budget.go with the
  short-circuit the design states. Trace the five sequences and the
  release-and-reclaim sixth against the code, not the tests.
- Fix 2: the constructor in metasystem/internal/goalbudget/budget.go stores
  m and h verbatim and refuses every d token with the exact wording; the
  legacy reader keeps eight-hour days for stored d; every day-token
  inventory row is converted or added as the proof plan classifies it, none
  skipped, none reinterpreted.
- Fix 3: a breach parks the goal and never the machine; `ResolveStopAuthority`
  and the command seam in metasystem/cmd/metasystem/goalsync_mutations.go;
  the cancellation-duty route in metasystem/internal/dispatch/stop.go and
  the steward tick; the one-claim rule over fenced and unfenced goals; the
  parse invariants and metasystem/internal/goal/reconcilemap.go still
  refusing every altered Claimed line.
- Every test the proof plan names exists by its exact name and asserts the
  behavior, not a tautology; any test the diff renames, deletes or weakens
  is a finding. Read the return's pasted gate output: if a gate did not run
  or a scenario was skipped, say so.
- Any hunk outside the files the design's Fix sections name is a finding.
  No benchmarks (R-31).

# Constraints

Wall-clock budget: 20 minutes. Your sandbox is read-only; verify by reading.
Return per the code-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
