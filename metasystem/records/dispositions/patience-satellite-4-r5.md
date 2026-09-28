# Dispositions: patience-satellite-4, round 5

Job: design-critic-20260811t185221z-38db (codex gpt-5.6-sol, xhigh).
4 findings, 4 material, all accepted. Convergence 15 → 13 → 4 → 4 →
4 with falling severity (high, 2× medium, low); every finding attacks
the previous round's new text only.

| id | disposition |
| --- | --- |
| P4-037 | accepted — the kind lives in the durable bytes: detail annotations become two forms, `Patience: chain=<root> rounds=<n> floor=<m>` and `Patience: orphan=<id> rounds=<n> floor=<m>`, so the prompt projection is a pure function of the ledger with no second source. Three write/read forms total with overflow. |
| P4-038 | accepted — timestamp totality: values parse as RFC3339; missing AND unparseable both sort oldest (one bucket); ties fall through endedAt → startedAt → jobId. The order is total over damaged records. |
| P4-039 | accepted — identity is the record's filename stem (the on-disk identity that certification resolution and close actually address); a record whose recorded jobId differs from its filename stem is identity-damaged and out of patience scope (the r4/P4-035 boundary), closing the job-a.json-says-job-b split. |
| P4-040 | accepted — breach ranking is (count − floor) descending — distance past the floor — tiebreak count descending, then root ascending. Named in verification. |
