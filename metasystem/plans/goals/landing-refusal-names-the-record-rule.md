# landing-refusal-names-the-record-rule

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: one wasted landing attempt and a code lookup; novelty 1: message text; exposure 2: every seat landing a plan file; accumulation 1: repeats per refusal"
- Tier: 2
- Intent: land.sh prints 'agent commit refused: the staged record is not owned by this landing (would-refuse code=record-not-owned)' and the generic line 'carry only new records or records owned by the held goal or actor', but never the specific carriage error the evaluator computed (existing plan record X is frozen / record X is owned by goal Y / handoff X is owned by Z), nor the rule that a plan file's owner is the goal whose id prefixes its filename. Seen 2026-09-06 22:40Z on plans/metasystem-stop-design.md under goal metasystem-stop-verb; the orchestrator had to read internal/landing/observe.go to learn the remedy (land a new file named <goal-id>-...). DONE means the refusal prints the evaluator's specific error and, for a frozen or foreign plan file, the naming rule as the remedy, with a fixture
- Origin: main
- Next step: find where land.sh or the landing observation collapses carriageError into the generic line; print err text and the remedy; fixture in the land fixtures
- OpenedAt: 2026-09-06T22:00:29Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T22:00:29Z YT996HDYBREHXB3T79MYS1CXDR-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=landing-refusal-names-the-record-rule
Integrity: sha256=b9e2ebb61b4fa0f711e6a6a4c50b41622c001db727fb1ade6c25a10a81410adc
