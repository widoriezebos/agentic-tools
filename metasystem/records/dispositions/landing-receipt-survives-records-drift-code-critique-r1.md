# Dispositions: landing-receipt-survives-records-drift code critique round 1, chain lrsrd-carry-cc1-20260910

Round 1 (Opus, 2026-09-10 10:17 to 10:33Z) read the carried change on reviewed tree 4acb9609124c629cbe633f364ed27523c48b3915 and returned five low findings and nothing material: the chain closes on this read once the fold below lands in round 2 and is read afresh, as the chain-close law requires for a design-bearing chain.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| LRC-01 | accepted | The tier-one route (`land.sh --tests`, then the command receipt) mints version 3 with a projection on this engine, so the certified round's stronger assertion (worktree bindings equal the projection tree, index bindings equal the tree) holds and had been dropped on a mistaken premise; nothing main landed was weaker. | Folded: round 2 restores the assertion in the tier-one leg of `scripts/agents/land-fixtures.sh` beside main's loop. |
| LRC-02 | noted | The testing-contract receipt survives a register append only while no group's input manifest covers `records/narrator-digest.log` or `memory/receipts.log`; today none of the 61 groups does. A guard belongs to the contract's validator, outside this boundary. | Recorded in the goal's conclusion as a follow-up candidate for the testing contract's owner. |
| LRC-03 | noted | The Go canaries mint through `CreateTestReceipt`, which production no longer calls; the production mint path (`PrepareTestReceipt`, `Complete`, `PublishCommittedReceipt`) is proven under register drift by the land bed's passing canary, which the orchestrator ran green on round 2's tree. | None. |
| LRC-04 | accepted | The landing-time read stays raw for version 1 by the reconciliation rule (main's comparisons kept exactly; a version-1 receipt read as filtered is a named threat). With the testing contract enabled, chain and tier-1 landings require version 2, so the case has no effect here. | None. |
| LRC-05 | accepted | `static-reproof-fixtures.sh` edits three case lists and no sandbox could run it; the orchestrator ran it on the tree. | Proof run supplied: static-reproof fixtures PASSED on the tree (orchestrator, 2026-09-10). |

Orchestrator's own finding, folded in round 2: the land bed's full-width-chain leg was red on round 1 because the carried fixture's second hand-written chain record lacked the `goalId` field the budget projection requires since the testing-contract landing; one line, proven on a probe copy.
