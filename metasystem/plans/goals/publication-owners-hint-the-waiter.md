# publication-owners-hint-the-waiter

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: a dropped hint only delays a wait to its ten-second reread and a false hint triggers one read, but a wrapper that changes an exit mapping breaks every caller of job watch, run watch and delegate --wait; novelty 2: hints after durable writes and wrappers over a working verb are new, the owners and mappings are not; exposure 3: every seat's delegate and proof waits; accumulation 2: three publication owners, three wrappers and their fixtures"
- Tier: 2
- Intent: Member 2 of coordinator-wakes-on-events-not-polls (design page plans/coordinator-wakes-on-events-not-polls-design.md, section 6 row 2). DONE means: the job transition owner, the proof attempt's terminal commit and the confirmed ledger publication each hint the waiter after their durable write; a dropped or false hint changes no wait result; job watch, run watch and delegate --wait become wrappers over the installed metasystem wait and keep their current exit mappings exactly. Fixtures: the hint portions of section 5 row 4, TestWaitHintsOnlyTriggerReads, TestWaitCompatibilityMappings; bed legs wait-native-hint, wait-no-native and wait-compatibility for those assertions.
- Origin: main
- Next step: 2026-09-13 m1b: opened from section 6. Next: a Codex sol build brief from section 6 row 2 and section 5 rows 2 and 4, with the exit mappings read from scripts/agents/dispatch.sh, then a DESIGN-BEARING Opus read, the seat's delivery proof and a human commit. Queued behind Wido's ranked queue for m1b.
- OpenedAt: 2026-09-13T13:41:26Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-13T13:41:26Z CNPC58KYDPJJ4N34NVWCYTBFN9-m1b-30a7e141 open actor=m1b+main-1789191336-90295-e4b24b targets=publication-owners-hint-the-waiter,coordinator-wakes-on-events-not-polls
Integrity: sha256=9f77544b1b3fbfd969ad9b929d90b3220abf273a67c0fbe4aa33b9862d0bfc3c
