# testing-optional-input-paths

- State: parked
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="An uncommon declaration shape currently refuses safely with a Git pathspec error; improving it is separate from successful ordinary delivery and must preserve later-appearance and deletion detection."
- Tier: 2
- Intent: Deferred input-boundary work from coordinator-loop-prevention: decide and implement useful handling of declared paths absent from both candidate and working tree.
- Origin: main
- Next step: After delivery and human prioritization, use m1c astra-r2-public-go-20260909T0643Z evidence: test check accepts the contract but test plan refuses an absent declared go.sum. Decide whether absent declarations are supported or diagnosed explicitly. Preserve candidate identity, ignored/untracked input tracking, modes, symlinks, later appearance, deletion and permission errors. Retain any unlanded correction separately; do not expand the current goal with further boundary cases.
- OpenedAt: 2026-09-09T07:02:58Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Parked: by=human:Wido at=2026-09-13T08:24:16Z because=Not now (2026-09-13 backlog consolidation, Wido's word): Not-now: a synthetic boundary (test check accepts, test plan refuses an absent declared go.sum; m1c astra-r2 2026-09-09) that no landing has hit; internal/testpolicy has no absent-input handling (hygiene 09-11); revisit as a ride-along the next time internal/testpolicy changes, preserving candidate

History:
- 2026-09-09T07:02:58Z Q13KT9T08KACXB2AC3YWEHXE52-m1c-1274caf4 open actor=m1c+main-1788759014-39092-24fa5c targets=testing-optional-input-paths
- 2026-09-13T08:24:16Z GS089PDCWCPYNX8TCG9EMTCA89-m1-c6925449 park actor=human:Wido targets=testing-optional-input-paths reason=Not now (2026-09-13 backlog consolidation, Wido's word): Not-now: a synthetic boundary (test check accepts, test plan refuses an absent declared go.sum; m1c astra-r2 2026-09-09) that no landing has hit; internal/testpolicy has no absent-input handling (hygiene 09-11); revisit as a ride-along the next time internal/testpolicy changes, preserving candidate
Integrity: sha256=1bcd999350ae35aa61bd6cb37a15e4bde7c0f2771abd6dc6ee0db85b47773529
