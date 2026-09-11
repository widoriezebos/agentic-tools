Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 10: the section row reads its last sample, not its first

Chain phd-build1-20260910. Round 9 was right to stop: readiness already
composes through startNestedSupervisorHelper, and the round-9 brief's
diagnosis was wrong. The seat reproduced the row in a scratch copy with
a debug line: the first gated sample sees both members
(members=[48830 48832], the helper and its Setsid stage child), the
callback closes the helper's stdin, and the gated reader keeps calling
the callback on the samples taken while the helper exits, which see
only the helper (members=[48830]). `observedMembers` is overwritten by
the last call, so the assertion reads 1. Report `round` as 10. Do not
fetch or rebase the worktree.

## Decision D-R9-1, restated (the designer's correction)

Readiness composes as built; no change there. A fixture callback that
runs on every gated sample records the largest member count it has
seen, never the last, because samples continue while a helper exits.
Apply the same reading to any other fixture that asserts on a member
count or a per-member figure across gated samples (the reaped-child and
Setsid rows already keep maxima; make sure they do).

## Mandate for this round

1. In TestSupervisorSectionResultGrowthCountsAsOutput record the maximum
   of len(sample.Members) across callbacks; keep the two-member and
   growth-as-output assertions.
2. Check the other gated fixtures for the same last-write pattern and
   fix any you find the same way.
3. Nothing else changes.

## Proof

go build ./..., go vet ./..., gofmt -l; go test -count=1 and
go test -count=1 -race on internal/proofrun and internal/testpolicy,
green. Report the round as your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Never
delete written work.
