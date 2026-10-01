# steward-catch-up-survives-restarts

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="a returning seat can loop forever; moderate novelty; affects any seat after downtime"
- Tier: 2
- Intent: What: A seat that comes back after a long time offline catches up on the shared ledger in steps and keeps its progress, so being restarted halfway does not make it start over. Why: in early September a machine came back after 48 hours offline with 405 ledger changes to catch up on; its steward was replaced every minute and began the whole catch-up again each time, so it never finished. That problem was folded into another goal on 2026-09-13 without a fix, and on main the catch-up still handles all changes in one pass and saves progress only at the end. Pros: seats that were offline recover by themselves instead of looping. Cons: the catch-up needs saved intermediate progress, a little more state to get right.
- Origin: human
- Next step: Next: write a test that replays a long offline window and kills the steward halfway, then make the catch-up save its progress in steps and resume from there. Done when: in that test the second steward continues where the first stopped and finishes.
- OpenedAt: 2026-09-30T19:06:58Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-30T19:06:58Z DKD60PVY5H9NQR7APC58PQVAT5-ui-bc2fda53 open actor=human:Wido targets=steward-catch-up-survives-restarts
Integrity: sha256=21aab1083b5c01dd276c52b354c6cf109917af6a40cefd1b9ae73de048b5bd3d
