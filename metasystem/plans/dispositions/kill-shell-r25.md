# Dispositions: kill-shell plan, round 25

Job: design-critic-20260812t030231z-5d13 (codex gpt-5.6-sol, xhigh).
1 finding, 1 material, accepted — the two protocol halves deadlocked
together and the marker's own gate-name field resolves it.

| id | disposition |
| --- | --- |
| KS-R25-001 | accepted — markers carry KIND in the name they already record: validation runs register as the suite, publication contenders register as publish-bootstrap. The publication fence consult refuses only foreign VALIDATION markers (a live suite must never have its binary swapped) and treats foreign PUBLICATION markers as contention, proceeding to the lock — which adjudicates exactly as rounds 23-24 specified. No mutual refusal, and the mid-run swap protection survives untouched. |
