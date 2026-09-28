# Dispositions: verification chain, round 1 (MV-1)

All nine accepted; each incorporated into the consolidated rewrite (IL-21 level: read — reproduced against the cited code lines before folding).

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| MV-1-1 | accepted | The fence ended at record creation; launch was outside it. | D-5: the launch helper re-takes the lease lock and re-verifies generation immediately before spawn. |
| MV-1-2 | accepted | A pre-check child cannot fence git's own mutation. | D-6: the agent commit wrapper holds the lock across git; the guard enforces the wrapper via a would-block probe; humans pass. |
| MV-1-3 | accepted | Side effects preceded the refusal. | D-5: an at-entry check refuses non-holders before any write. |
| MV-1-4 | accepted | Three nouns did not cover the surface. | D-4: the authority matrix, one row per caller class, every dispatcher verb placed. |
| MV-1-5 | accepted | Pair and triple rules contradicted; announcements lacked the data. | D-1: announcements gain mainId and commandHash; authentication uses the triple, liveness the pair, each stated. |
| MV-1-6 | accepted | The evidence segment was undefined and the collector unaddressed. | D-8: hash-segmented layout, legacy glob kept permanently, two-checkout distinctness in proof. |
| MV-1-7 | accepted | Version 2 had no shape. | D-9: full field, requiredness, literal, selection, and retirement contract with four fixtures. |
| MV-1-8 | accepted | No key, location, or ordering contract. | D-10: keyed entries appended in the same locked write as the terminal transition; cursor lifecycle defined. |
| MV-1-9 | accepted | A superseded paragraph still instructed the deleted mechanism. | The rewrite removed every superseded passage; the document carries the marker words nowhere. |
