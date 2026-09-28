# Dispositions: benchmark-validity closure, round 4

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| BV-4-1 | accepted | The folds were at HEAD; the critic's worktree was frozen at round-1 dispatch and follow-ups never sync it — recorded as KI-20, worktree now synced by hand each round. | Interim manual sync; mechanical fix through the loop per KI-20. |
| BV-4-2 | accepted | measuredOutcome had no shape or transition. | Defined shape; resume re-attempts closure only and publishes the preserved outcome. |
| BV-4-3 | accepted | "Candidate commit" collided with candidateSha's meaning. | Renamed measuredCandidateSha, the tree the gate measured. |
| BV-4-4 | accepted | First-machine attribution was unproven across resume. | Per-attempt fingerprints, validity requires all equal. |
| BV-4-5 | accepted | Fix-level ownership misassigned parts. | Ownership is a per-part matrix; each part rides its owner's gate. |
