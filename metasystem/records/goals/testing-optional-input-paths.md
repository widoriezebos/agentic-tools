# testing-optional-input-paths

- State: done
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="An uncommon declaration shape currently refuses safely with a Git pathspec error; improving it is separate from successful ordinary delivery and must preserve later-appearance and deletion detection."
- Tier: 2
- Intent: Deferred input-boundary work from coordinator-loop-prevention: decide and implement useful handling of declared paths absent from both candidate and working tree.
- Origin: main
- Next step: After delivery and human prioritization, use m1c astra-r2-public-go-20260909T0643Z evidence: test check accepts the contract but test plan refuses an absent declared go.sum. Decide whether absent declarations are supported or diagnosed explicitly. Preserve candidate identity, ignored/untracked input tracking, modes, symlinks, later appearance, deletion and permission errors. Retain any unlanded correction separately; do not expand the current goal with further boundary cases.
- Concluded: The snapshot layer already treats an absent path as no change (7ebb8e308), and the boundary was never hit.
- OpenedAt: 2026-09-09T07:02:58Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-09T07:02:58Z Q13KT9T08KACXB2AC3YWEHXE52-m1c-1274caf4 open actor=m1c+main-1788759014-39092-24fa5c targets=testing-optional-input-paths
- 2026-09-13T08:24:16Z GS089PDCWCPYNX8TCG9EMTCA89-m1-c6925449 park actor=human:Wido targets=testing-optional-input-paths reason=Not now (2026-09-13 backlog consolidation, Wido's word): Not-now: a synthetic boundary (test check accepts, test plan refuses an absent declared go.sum; m1c astra-r2 2026-09-09) that no landing has hit; internal/testpolicy has no absent-input handling (hygiene 09-11); revisit as a ride-along the next time internal/testpolicy changes, preserving candidate
- 2026-09-30T19:02:23Z YN5C655B6NTH0VVT9428DBPA0B-m1e-b6a4eb0a done actor=human:wido targets=testing-optional-input-paths
Integrity: sha256=9d3b8df62d696282c826a313c5f6ddb39161eaed87571822d3296be5b7fc56da
