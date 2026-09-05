# Supervision custody landing member: code review, second round (chain scp-build1-cc2)

Reviewed tree a857f657bc58eb7fe8754414ec6076b4efac9013 (chain scp-build1, round 2). Critic: Claude Fable 5.1. One material finding; a correction round follows.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SCC-51 | accepted | The live-owner gate now wraps the whole guard, including the pre-existing veto of a lock whose tag lies outside the checkout's prefix; a dead foreign owner reads as Dead, every check is skipped, and shutdown sweeps a lock armed for another repository (suite scenario foreign-owner). | The foreign-tag veto stays unconditional, before liveness; only the recorded-path guard is live-only. |
| SCC-52 | noted | The one preserved suite run hit the stop hook's four-second deadline in census-lifecycle; the fixture's auditor wrapper adds a shell start per engine call. A green seat-side run closes it. | none |
| SCC-53 | noted | Compaction retains open production publications and the reduction gained a projection and a drop rule not written into the registry design's REG-3 text; compaction has no production caller today. | Named in the goal's residue for the design doc. |
| SCC-54 | noted | A live owner with no row in the selected registry is refused without a recovery hint. | Named in the residue. |
