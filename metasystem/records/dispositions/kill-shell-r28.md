# Dispositions: kill-shell plan, round 28

Job: design-critic-20260812t033610z-13af (codex gpt-5.6-sol, xhigh).
1 finding, 1 material, accepted.

| id | disposition |
| --- | --- |
| KS-R28-001 | accepted — one serializer for both kinds: the validator's admitted-flag write and the publisher's rename both happen only under the publication lock, each preceded by an under-lock recheck. The later claimant always sees the earlier one's fact: a publisher finding an admitted validator aborts; a validator finding a fresh publish proceeds against the published binary. No cross-kind window remains because there is exactly one door. |
