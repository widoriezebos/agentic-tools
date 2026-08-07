# Dispositions: coexistence design, round 5

All five accepted (read). MM-5-1's root cause is documented in the design's note of record: the round-4 folds were externally reverted between write and add — the interference itself.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| MM-5-1 | accepted (read) | The folds were written and asserted, then reverted externally before the add; commit e78a5c7 captured only the dispositions. | Rounds 4 and 5 folded together in one atomic write-commit-push-verify block; the incident recorded in the design. |
| MM-5-2 | accepted (read) | A command hash is not lifetime-unique. | The reuse residual is bounded and accepted: start-time is the second factor, collision needs pid and second reuse, and the failure direction is refusal. |
| MM-5-3 | accepted (read) | Authorized-before-death work is live, not inert. | W-5: verify-at-write, generation-tagged records, and a claim sweep that kills stale-generation runners. |
| MM-5-4 | accepted (read) | No specified algorithm distinguished main from delegate. | W-2: the ordered parent-by-parent walk, written down. |
| MM-5-5 | accepted (read) | A blocking probe can hang. | flock -n expecting failure: bounded, unambiguous. |
