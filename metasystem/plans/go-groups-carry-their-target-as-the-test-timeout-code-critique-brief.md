Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal go-groups-carry-their-target-as-the-test-timeout)
Date: 2026-09-10

# Review brief: the go adapter's test timeout (chain gotimeout-build1-20260910)

FINDING IDS: chain-unique GTO-01, GTO-02, ... never F-n. You are the
first critic on this chain. Report `round` as 1 in your return. One
focused round.

Why: the receipt's go adapter in internal/proofrun/test_go.go ran
every unit group without -timeout, so a group sat on Go's ten-minute
default whatever its targetMs; today goal-full-coverage (targetMs
1800000) failed a landing receipt at 600.466 s with every test that
ran passing. The build brief (in the plans directory under this
goal's name) asks for -timeout derived from targetMs when it exceeds
the default, the timeout recorded on the group's result record, and a
unit test for both sides.

Attack: the duration arithmetic (milliseconds to a Go duration, no
truncation to zero, no flag at or below the default); that the flag
sits in the argv before the package list and after -count=1 so the
execution identity of unchanged groups does not move (or, if it
must move for groups above the default, that nothing reuses a stale
record); that the record field is absent, not zero, for the default;
that the test would fail without the change; that nothing else
changed.

Scope: the computed diff of the implementer job. Contract: the goal
record and the build brief.

# Mandate

1. Groups above ten minutes run with their target as the timeout;
   groups at or below keep the default; the record says which.
2. Nothing outside the boundary changed.

If nothing material remains, say so; that closes the chain and it
lands.

# Constraints

Wall-clock budget: 15 minutes. Return per the code-critic schema with
the reviewedTree from the review record beside the computed diff (the
conformance validator refuses from a sandbox; say so).

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.
