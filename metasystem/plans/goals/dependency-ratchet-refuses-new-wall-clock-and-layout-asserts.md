# dependency-ratchet-refuses-new-wall-clock-and-layout-asserts

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=3 basis="The audit changes fixture rules repository-wide; misses repeatedly leave main red, while false positives block tests rather than production behavior."
- Tier: 2
- Intent: The dependency ratchet refuses new wall-clock and layout assertions. DONE: (1) new time.Now and time.Sleep uses in Go tests outside declared clock seams are refused from baselines of 128 and 33 files; (2) shell fixture waits on date or sleep are refused; (3) adopted-copy fixtures cannot assert template-only paths without a template-only declaration; (4) next retro has zero clock or adopted-copy trunk-red repairs and the time.Now test-file count is at most 128 and falling.
- Origin: human
- Next step: Evidence: at least five clock or load reds and three layout reds included bd257b010, 9e6c87030, 986d58624 and the lane-6 headless-launchers ejection. No design first: brief the baselines, seams and adopted-copy declaration, build, read.
- OpenedAt: 2026-09-16T21:02:59Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-16T21:02:59Z 30SKF3XM8D5E9482JHBHYCKREP-m1e-c6925449 open actor=human:Wido targets=dependency-ratchet-refuses-new-wall-clock-and-layout-asserts
Integrity: sha256=8ee35e5a538d73d346aeddc2aa830e2d2c862e3f9dcc4b1bc12634aa5987d7c1
