# a-final-prefix-red-is-repeated-before-own

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: The lane repeats each failing test alone on the merge tree before it returns a goal as own (final-prefix shortcut, internal/landing/plain/replay.go:398-405; journal 2026-11-07 06:42)
- Origin: human
- DesignGate: off
- Next step: Build without a design from N4 and section 7 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: per-identity repeat selection, a retained repeat identity beyond the one-repeat-per-tree guard, mixed-result publication; test one reproducing and one passing failure; about 120 lines, split seam selection then publication; after a-return-names-the-failing-test U1a
- OpenedAt: 2026-11-07T09:51:15Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T09:51:15Z QSC4WX8X6BKFCMA1YRRD530EJN-m1e-1dae276b open actor=human:Wido targets=a-final-prefix-red-is-repeated-before-own
- 2026-11-07T09:51:24Z R2Z25YEMGY9YSJ029V26756XW0-m1e-1dae276b edit actor=human:Wido targets=a-final-prefix-red-is-repeated-before-own reason=Allowed: build-without-design why=Allowed: build-without-design. One unit from a recorded finding whose root cause is read in the fixes analysis revision 3 and critiqued twice by Astra impact=This lets goal a-final-prefix-red-is-repeated-before-own be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-final-prefix-red-is-repeated-before-own until withdrawn. To undo: metasystem goal disallow a-final-prefix-red-is-repeated-before-own build-without-design
Integrity: sha256=65c581672f6f6bcc82afe6ec7f8a720a00928fe0937222dbf7c7984ad0aec41a
