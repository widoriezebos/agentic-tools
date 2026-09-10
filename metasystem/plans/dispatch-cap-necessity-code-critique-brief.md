Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-09

# Review brief: reservation caps settle to the minutes a job ran (chain dispatch-cap-build1-20260909)

FINDING IDS: chain-unique, DCN-01, DCN-02, ... never F-n. Report `round`
as 1 in your return: it is this job's own round.

Round budget: 1 focused round, then at most one correction and its
re-review (tier 3). R-60-m1's rule: material only if it changes what
gets built and names the artifact.

Why: every dispatch added its cap (120 minutes) to the goal's reserved
job-minutes and kept charging it after the job ended, so short rounds
consumed whole pools (Wido's R-49-m1b, highest priority). The design
metasystem/plans/dispatch-cap-settlement-design.md revision 4, cut to
its box by metasystem/plans/dispatch-cap-settlement-scope-cut.md, was
critiqued four times and closed by the stop criterion; the build brief
metasystem/plans/dispatch-cap-settlement-build-brief.md (carried by the
seat's own brief in the plans directory under this goal's name) binds
the box: a job that has ended charges the minutes it ran from the
launcher's ownership proof to its end stamp, rounded up, floor 1,
clamped to its cap; an open job charges its cap; a job that never
launched charges nothing; endedAt becomes transition-owned; every
budget refusal prints "reserved observed=<n> open-caps=<m> limit=<L>";
sections 4.3 and 1.9 are OUT.

Threat model: a terminal record still charging its cap (any status
missed: completed, failed, cancelled, timeout, process-lost); the
charge measured from startedAt where provenAt exists (over-count
without bound, the round-four finding); a missing or unparseable
provenAt not failing closed as the design says; the floor or the clamp
off by one at a minute boundary; the two projection fields not summing
to ReservedJobMinutes, or the overflow guards missing; the refusal line
absent on any refusal path (admission, governed); the endedAt patch
refusal weaker than the exact message, or a lawful writer patching
endedAt on an open record (a stop condition); tests T1-T12 present in
name but asserting less than the design's sentence for each; the
eight-record specimen not yielding 50 and 20; existing tests changed
beyond the listed changes; anything from the cut sections built; any
path outside the declared boundary touched; internal/dispatch below
its coverage floor.

Scope: the computed diff of the implementer job under review.
Contract: the design, the scope cut, the landed build brief and the
goal record metasystem/plans/goals/dispatch-cap-necessity.md.

# Mandate

1. The charge rule, settlement, projection fields and refusal line are
   exactly the design's box, nothing from the cut sections.
2. Tests T1-T12 assert the design's sentences; the eight-record
   specimen yields 50 at revision 26 and 20 at revision 28.
3. Nothing outside the boundary changed; coverage holds.

If nothing material remains, say so; that closes the chain and it
lands.

# Constraints

Wall-clock budget: 25 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
dispatch-cap-build1-20260909; if your sandbox cannot run it, read the
tree from the review record beside the diff and say so. The
orchestrator runs the dispatch and obligationstate packages, the
dispatch fixtures and the hook suite on the seat before your return
lands.

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.
