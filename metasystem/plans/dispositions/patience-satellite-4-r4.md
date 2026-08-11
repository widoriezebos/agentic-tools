# Dispositions: patience-satellite-4, round 4

Job: design-critic-20260811t183913z-57a0 (codex gpt-5.6-sol, xhigh).
4 findings, 4 material, all accepted. Convergence 15 → 13 → 4 → 4:
the findings now attack the round-3 fixes' totality, not the pillars.

| id | disposition |
| --- | --- |
| P4-033 | accepted — the table lost r2/P4-019's walk semantics. Restored: the model row is "the newest terminal job that HAS a canonicalizable effectiveModel", so an older good record is found past a newer damaged one; the no-evidence rows apply only when no terminal job canonicalizes. |
| P4-034 | accepted — "newest" gets a total order: jobs sort by (endedAt, startedAt, jobId) descending, missing timestamps sorting oldest, jobId as the final lexicographic tiebreak. Sibling branches select one floor deterministically. |
| P4-035 | accepted, by narrowing — patience's input set is mission jobs with a valid-grammar jobId. An attributable record WITHOUT one is out of patience scope entirely (janitor jurisdiction): patience's only remedies are certify-by-jobId and close-by-chain, neither of which can touch an identity-less record, so counting it would be a nag with no possible act. The invalid-sha display replacement is dropped as no longer needed — every counted identifier is grammar-safe by construction. |
| P4-036 | accepted — orphan singletons (valid jobId, broken parent walk) get their own prompt wording: `Patience: orphan job <id> has uncertified spend — certify landed value or flag it to the human.` The close offer is omitted because dispatch close cannot resolve a broken lineage and the design refuses dispatch changes; a persistent nag over a certifiably worthless orphan is vocal noise pointing at real damage, escalated to the human, which is the system working. |
