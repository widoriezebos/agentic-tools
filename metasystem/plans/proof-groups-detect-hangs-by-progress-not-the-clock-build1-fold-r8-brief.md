Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 8: real-reader rows carry no window

Chain phd-build1-20260910. The seat gate on round 7 (plain and under
-race, with the real Darwin reader) fails three rows, and the cause is a
rule the designer had not written: a real-reader fixture with a
shortened wall window is load dependent by construction, because the
latency of a real sample (two `ps` calls on Darwin) under load is
unbounded, so the window can elapse between the event and the sample
that would have seen it. Observed on the seat:

- TestSupervisorDescendantAndReapedChildCPUAreCounted/reaped_child under
  -race: the child consumed 0.36 s and exited; no sample saw it gone
  before the 400 ms window ended the group `dead` (seenAfter=false).
- TestSupervisorRunawayUnderMeasuredContention, finite phase, under
  -race with eight competitors per processor: `busy-for 600ms` ended
  `dead` ("no CPU or output progress for 400ms", CPUSeconds 0.61).
- TestSupervisorSectionResultGrowthCountsAsOutput, plain and -race:
  members=0 at the first gated sample; the fixture counts
  `sample.Members`, which the Darwin reader no longer fills since round
  7 keyed members by pid and start time in `MemberCPU`.

Report `round` as 8. Do not fetch or rebase the worktree.

## Decisions

D-R8-1. Real-reader rows run with no zero-consumption window (window 0:
wait without bound) and no budget unless the row proves the budget. The
zero-consumption rule, the stopped and host-waiting readings, the dump
ladder and reader failures are proven with the scripted reader, whose
samples are deterministic. The real-reader rows prove accounting (the
tree, Setsid descendants, retained members, stage-results growth) and
the runaway budget under contention, and every one of them ends by the
process's own exit or by the budget, never by a wall window. This rule
goes on the design page.

D-R8-2. The sample carries `Members` (the process ids) on both
platforms in every sample, derived from the same scan that fills
`MemberCPU`; fixtures may read either. The section row asserts at least
two members (the helper and its Setsid stage child) on the first gated
sample and that the stage-results growth was counted as output.

## Mandate for this round

1. Apply D-R8-1 to every real-reader row: the reaped-child and Setsid
   rows, the section row, and the contention row's three phases (quiet,
   finite, loaded: budget only, no window). Keep the measured-share
   escalation and the stdin custody as they are.
2. Apply D-R8-2 in the readers and the section row.
3. Nothing else changes.

## Proof

go build ./..., go vet ./..., gofmt -l; go test -count=1 and
go test -count=1 -race on internal/proofrun and internal/testpolicy,
green. The seat gate runs the same with the real reader. Report the
round as your own.

## Constraints

Wall-clock budget: 25 minutes. Return per the implementer schema. Never
delete written work.
