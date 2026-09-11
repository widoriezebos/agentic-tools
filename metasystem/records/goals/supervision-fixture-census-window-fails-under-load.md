# supervision-fixture-census-window-fails-under-load

- State: done
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a correct candidate is refused at the gate by host load, nothing granted or destroyed; novelty 1: a wait the neighbouring scenarios already perform; exposure 2: every landing on a busy seat whose plan selects the section; accumulation 1: one receipt run per refusal"
- Tier: 2
- Intent: The supervision-and-census-fixtures section's stop-everything scenario fails under machine load with 'dispatch refused: census verdict is stale (age=2s window=2s); retry in a moment' (m1, 2026-09-10 13:1xZ, load average 7 to 8 with three other seats' delegates running): the scenario dispatches right after arming and the census verdict it needs is older than its two-second window when the host is busy, so a landing receipt is refused for a timing the candidate did not touch. Done means: the fixture waits for a fresh census verdict (or asks for one) before dispatching, bounded by the fixture budget, and a receipt on a loaded host no longer fails this section for that reason; the window itself stays what the design says.
- Origin: main
- Next step: m1c's lane (supervision fixtures): in scripts/agents/supervision-fixtures.sh (scenario stop-everything) poll the census verdict age before the dispatch that expects admission, as the other scenarios do after re-arm; Sol builds, Opus critiques.
- Concluded: Merged into goal:fixture-waits-name-their-producer in the 2026-09-11 backlog consolidation on Wido's word; Program 11's NEXT routes fixture waits to fixture-waits-name-their-producer (outside the program); scripts/agents/supervision-fixtures.sh:920-1091 (stop-everything) dispatches right after arming with no wait_for_census before the admission-expecting dispatch while the wait_for_census helper (:351) e. Its specific requirement is appended to that goal's next step.
- OpenedAt: 2026-09-10T13:16:47Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T13:16:47Z AKHKFHBXBT78R40CZQ7XQFGH21-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=supervision-fixture-census-window-fails-under-load
- 2026-09-11T22:06:33Z 3MMDHBFPGGTJ9MWRJ1WA76TBHV-m1-c6925449 done actor=human:Wido targets=supervision-fixture-census-window-fails-under-load
Integrity: sha256=a22b6800115dded73ac67f8bb8a058720a7717369eae547600e7ac906331848e
