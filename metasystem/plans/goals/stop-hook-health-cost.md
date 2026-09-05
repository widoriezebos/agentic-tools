# stop-hook-health-cost

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: an expired deadline refuses a turn end that was safe, which costs a seat a full turn and teaches it to distrust the refusal, but nothing unsafe is permitted; novelty 1: this is profiling and caching inside one existing verb, not new machinery; exposure 3: every seat on every machine pays this cost at every turn end; accumulation 1: first time this has been measured, though it is a plausible contributor to refusals already blamed on other causes"
- Tier: 3
- Intent: The Stop hook gets four seconds and one health call spends two of them: metasystem health --hook-preview costs a steady 2.0s on m1 (measured three times, 2.04/2.01/2.07), and the hook also pays turn-verdict at 0.83s plus arming, digest, watchdog, evidence and sha256 work inside the same budget. On 2026-09-05 that budget expired and the turn end was refused with 'Stop deadline expired before a safe turn verdict'. DONE means the hook's health input costs a small fraction of its budget, proven by a measurement in a fixture rather than by hand, with the refusal path unchanged when the budget really is exceeded
- Origin: main
- Next step: Measured on m1 2026-09-05 but not diagnosed: health --hook-preview is a steady 2.0s while goal fetch is 0.5s and proc probe is 0.006s, so the cost is inside health's own component set rather than the ledger read. Next: profile which health components dominate (the spend fence prices 647 usage records and the job scan reads 112 records are the first suspects), then decide between caching a tick-owned observation and narrowing what --hook-preview computes. WAITS ON m2's supervisor repo-identity landing before any diagnosis, per Wido 2026-09-05
- OpenedAt: 2026-09-05T10:09:49Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-05T10:09:49Z YM1KHGTB9X8C64HM9WQJ3KJNZ5-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=stop-hook-health-cost
Integrity: sha256=11b50682cd26fc2d6e85edfa1fbc3359782e07a21cd265410be9e883daa14f0f
