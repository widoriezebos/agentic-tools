# a-publication-refuses-only-what-it-adds

- State: approved
- Priority: 1
- Sequence: 125
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: A ledger publication is refused only for a dangling design reference it adds, not for ones its parent already had (finding 540; internal/project/publication.go:42-55)
- Origin: human
- DesignGate: off
- Next step: Build without a design from N6 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: on the refusal path read the parent commit problems and refuse only new ones; test the three-page sequence of finding 540; about 25 lines. Decided in Wido's name; rejected: atomic multi-goal open
- OpenedAt: 2026-11-07T09:52:30Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-07T09:52:49Z revision=3 opid=94CV1YAXB01FT4016HTMTES5M8-m1e-1dae276b authority=proven digest=cb572171f896227e37214c00138fd94765593ba3caeaa85d450584ddf770d494 episode=3

History:
- 2026-11-07T09:52:30Z Y34TYSP6876J4M98A72JAVGQ57-m1e-1dae276b open actor=human:Wido targets=a-publication-refuses-only-what-it-adds
- 2026-11-07T09:52:39Z K38GACFKG5G61YDD1T5TZWCRPH-m1e-1dae276b edit actor=human:Wido targets=a-publication-refuses-only-what-it-adds reason=Allowed: build-without-design why=Allowed: build-without-design. One unit from a recorded finding whose root cause is read in the fixes analysis revision 3 and critiqued twice by Astra impact=This lets goal a-publication-refuses-only-what-it-adds be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-publication-refuses-only-what-it-adds until withdrawn. To undo: metasystem goal disallow a-publication-refuses-only-what-it-adds build-without-design
- 2026-11-07T09:52:49Z 94CV1YAXB01FT4016HTMTES5M8-m1e-1dae276b approve actor=human:Wido targets=a-publication-refuses-only-what-it-adds
- 2026-11-07T09:52:59Z HESWH5NWSWWEMX5XN7SYKWBYCY-m1e-1dae276b set-priority actor=human:Wido targets=a-publication-refuses-only-what-it-adds reason=priority-order subject=a-publication-refuses-only-what-it-adds from=unranked to=1:125 requested-sequence=append
Integrity: sha256=f6d5f5ec7f0e9dea656e4a1b809f7f4f9efd3cf48330dfcf3ddf67f182dbce79
