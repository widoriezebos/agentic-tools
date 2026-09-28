# Dispositions: kill-shell plan, round 24

Job: design-critic-20260812t024828z-10e8 (codex gpt-5.6-sol, xhigh).
1 finding, 1 material (medium), accepted.

| id | disposition |
| --- | --- |
| KS-R24-001 | accepted — the orphaned-consumer path is defined by what the owner-lock already provides: when the bounded wait expires with no usable published binary, the loser re-enters the protocol from registration, and the lock's dead-holder takeover semantics make it the new publisher; a contender that fails to obtain a usable binary after one full re-entry aborts loudly. No new machinery — the shipped takeover is the retry contract. |
