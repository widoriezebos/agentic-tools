# the-steward-proves-main-on-the-live-clock

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="One wiring change in the lane steward step (internal/landing/plain/steward.go:80, :420, :494-511): the proof path reads the clock the step was given, not the frozen one; a wrong clock could only accept a green that the frozen clock refuses as written in its future, and a FullAt in the future of the given clock stays refused; found by the proof-by-proof trace of 2026-11-09 (proof 28, results rows 1224 and 1225), cause inferred, the unit reproduces it first or the goal is withdrawn"
- Tier: 2
- Intent: The steward step decides a proof of main with the time at which it decides: a full green of main written after the step froze its clock is seen by the proof decision, so no full proof of main starts for a records move that the green already covers (design the-lane-proves-only-what-is-new, D2; 60.8 minutes on 2026-11-09 02:55Z).
- Origin: human
- Next step: Build U1 (the only unit, --last) from D2 of plans/designs/the-lane-proves-only-what-is-new.md: keep the clock the step was given before steward.go:80 freezes it, hand a copy of the proof seams carrying it to the Trunk call (steward.go:420); test through StewardStep with distinct frozen and given times and a changed goal-record tree, reaching the real proof path and failing against the present wiring; if the test cannot reproduce the decision of rows 1224 and 1225, withdraw the goal. About 8 production lines.
- OpenedAt: 2026-11-09T09:30:08Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-09T09:30:34Z revision=2 opid=912J2CTAGJWF2EY1Q9WEG7QYYK-m1e-83ea64f2 authority=proven digest=7720fa47dfd429f2b475b428253baaa337a53e0f01bf0bd5795f47c22bec4750 episode=2

History:
- 2026-11-09T09:30:08Z HBY02019HC7DRJP9DQYJV2H89Z-m1e-83ea64f2 open actor=human:Wido targets=the-steward-proves-main-on-the-live-clock
- 2026-11-09T09:30:34Z 912J2CTAGJWF2EY1Q9WEG7QYYK-m1e-83ea64f2 approve actor=human:Wido targets=the-steward-proves-main-on-the-live-clock
Integrity: sha256=cfe36eb372ed2f4a967f1535cb23a82e403a6e60b95565d5a6d64155e74e4988
