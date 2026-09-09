# brain-summary-leads-the-stop-display

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: a red bed in the full battery refuses every full-width landing, no work is lost; novelty 1: a line order in a renderer landed today; exposure 3: every seat's full-width landing and every brain seat's Stop; accumulation 1: nothing compounds"
- Tier: 3
- Intent: The bounded Stop display landed in d533caf17 (stop-refusal-fits-on-one-screen) moved the brain seat's summary from the head of the display to after the actionable lines, so goal-cli-fixtures.sh scenarios brain-stop-seeded and brain-stop-corrupt fail on main ('brain summary did not lead with ask and draft on stop 1: OPEN WORK (1)'); the bed is in the full battery, so every full-width landing on the fleet is red until this lands. The morning gate ran the hook suite, not the goal-cli bed, and the goal-cli bed ran only on the pre-landing tree
- Origin: main
- Next step: In internal/goal/verdictrender.go, renderTurnVerdict puts brainLines into the remainder after the actionable lines (line 61); on a brain seat the summary is the head of the display, as the two scenarios and the brain design expect: emit brainLines first, then the verdict line, the actionable lines, the run summaries, the greens and the file line, still within the bound. Update the renderer's own tests for the brain case, run scripts/agents/goal-cli-fixtures.sh seat-side (the bed that catches it), then supervision-hook-fixtures.sh. One Sol round, one Opus review, land. Owner: stop-refusal-fits-on-one-screen is parked behind m1c's hook chain; this fix touches no hook file, so it does not wait.
- OpenedAt: 2026-09-09T15:48:13Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-09T15:48:13Z ACJ0Z9NWDK6W56BR51PPH3BD6F-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=brain-summary-leads-the-stop-display
Integrity: sha256=0fa2a2c5faae061af90421bfe6ae48c2dfbc4a0a91cf06314567f80d4dc51f3b
