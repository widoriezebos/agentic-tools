# rounds-return-before-the-cap

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: two hours of work lost per occurrence; novelty 1: the cap exists, the return deadline is a prompt line and a follow-up rule; exposure 2: every long build round; accumulation 1: nothing built on it"
- Tier: 2
- Intent: A delegate round reaped at its budget cap (two hours today) leaves no return: on 2026-09-10 gawr-build2 round 2 was killed mid-scenario with twelve files built and green in its worktree, its interim messages are not a return, and delegate --follow-up refuses a timed-out round ('use a fresh dispatch after pending, running, timeout, or process-lost'), so the work had to be carried by hand as a patch into a fresh chain (120 job-minutes plus a coordinator hour). budget-death-on-return covers the money cap at return time; this is the time cap. DONE means: the dispatcher tells the delegate its wall-clock cap and demands a return no later than a fixed margin before it (the composed prompt carries 'return by <minutes>'), a round that reaches the margin writes its return with what is left named instead of running on, a reaped round keeps its worktree and a follow-up may continue it (the refusal above is lifted for timeout with an intact worktree), and a fixture proves a round that overruns returns partial-and-named rather than nothing.
- Origin: main
- Next step: Tier 2, MECHANICAL: the composed prompt line and the follow-up rule in dispatch.sh, a fixture in dispatch-fixtures.sh. Interim practice on m1b since 2026-09-10 04:45: every brief carries 'write your return no later than N minutes in'.
- OpenedAt: 2026-09-10T08:44:22Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T08:44:22Z XAJSF4GM5445XN2NVJRSZHRXFC-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=rounds-return-before-the-cap
Integrity: sha256=a282efd1e7fb53a933e73aa86f7c2d431bcc3e5da4305f1fa30a67dfb20da563
