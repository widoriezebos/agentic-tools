# brain-summary-leads-the-stop-display

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: a red bed in the full battery refuses every full-width landing, no work is lost; novelty 1: a line order in a renderer landed today; exposure 3: every seat's full-width landing and every brain seat's Stop; accumulation 1: nothing compounds"
- Tier: 3
- Intent: The bounded Stop display landed in d533caf17 (stop-refusal-fits-on-one-screen) moved the brain seat's summary from the head of the display to after the actionable lines, so goal-cli-fixtures.sh scenarios brain-stop-seeded and brain-stop-corrupt fail on main ('brain summary did not lead with ask and draft on stop 1: OPEN WORK (1)'); the bed is in the full battery, so every full-width landing on the fleet is red until this lands. The morning gate ran the hook suite, not the goal-cli bed, and the goal-cli bed ran only on the pre-landing tree
- Origin: main
- Next step: In internal/goal/verdictrender.go, renderTurnVerdict puts brainLines into the remainder after the actionable lines (line 61); on a brain seat the summary is the head of the display, as the two scenarios and the brain design expect: emit brainLines first, then the verdict line, the actionable lines, the run summaries, the greens and the file line, still within the bound. Update the renderer's own tests for the brain case, run scripts/agents/goal-cli-fixtures.sh seat-side (the bed that catches it), then supervision-hook-fixtures.sh. One Sol round, one Opus review, land. Owner: stop-refusal-fits-on-one-screen is parked behind m1c's hook chain; this fix touches no hook file, so it does not wait. CONFIRMED on main HEAD 17e916d05 with engine d533caf17 (2026-09-09 15:55Z): goal-cli-fixtures.sh 13 passed, 2 failed, exactly brain-stop-seeded and brain-stop-corrupt. Build brief written: plans/brain-summary-leads-the-stop-display-build-brief.md. Any seat with a free slot may take it; m1 takes it after hp-terminal-grade-for-stopping-acts lands if nobody has.
- OpenedAt: 2026-09-09T15:48:13Z
- Revision: 4
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-09T16:25:12Z revision=3 opid=V3W1VFRK2XCC5FBN74P68AEA5P-m1-c6925449 authority=proven digest=fd95d0fa581d41dbac32776a6465c4a88e595226a98f21fc49e5584592edd8ef
- Claimed: machine=m1 lineage=main-1788940932-18533-7fa6c2 at=2026-09-09T16:38:02Z revision=4 accountingRevision=4
- StopCapability: generation=4 revision=4 machine=m1 claimEpoch=6 fenceEpoch=0

History:
- 2026-09-09T15:48:13Z ACJ0Z9NWDK6W56BR51PPH3BD6F-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=brain-summary-leads-the-stop-display
- 2026-09-09T15:49:37Z SV7EA5Q4PVQCSKM2VFBK6NG16E-m1-1701c13c edit actor=m1+main-1788940932-18533-7fa6c2 targets=brain-summary-leads-the-stop-display
- 2026-09-09T16:25:12Z V3W1VFRK2XCC5FBN74P68AEA5P-m1-c6925449 approve actor=human:Wido targets=brain-summary-leads-the-stop-display
- 2026-09-09T16:38:02Z QDSB8ZCT4ZD76HS9SYNF7WJ152-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=brain-summary-leads-the-stop-display
Integrity: sha256=9e8a6f70c02f7dd79e20012f7516335cd6266102f0aaf020d01735449f8d84d2
