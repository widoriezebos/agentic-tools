# the-size-hold-stops-at-the-production-ceiling

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, root cause read (findings 554, 555; the week review of 2026-11-07); the unit check and the lane proof gate it"
- Tier: 2
- Intent: The size hold stops a unit only above the production ceiling of 250 production lines (decision 56); the production estimate of the units row is a signal that is reported, and the total allocation with tests is reported, never a stop (finding 554: today the hold stops above twice the declared changed lines with tests, internal/launch/unit_build_outcome.go:59-75, so an underestimate is a person act)
- Origin: human
- DesignGate: off
- Next step: Build without a design from finding 554 and the week review F7: measure production lines (tests excluded) against 250; report the estimate and the allocation; a unit at 1.8x its estimate below 250 continues, a unit at 260 stops; about 30 production lines
- OpenedAt: 2026-11-07T16:14:07Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-07T16:30:37Z revision=3 opid=CE39KBVP139FD5J94YCK483XPX-m1e-7d79a016 authority=proven digest=87495934a22f1ee196aa577c37e5169d357e512d88242406bfc4b6be928909c6 episode=3

History:
- 2026-11-07T16:14:07Z CCEPB2R5ZHS9SSEY90B0A8QTRM-m1e-7d79a016 open actor=human:Wido targets=the-size-hold-stops-at-the-production-ceiling
- 2026-11-07T16:29:04Z A8BD5CNCRN083VFV0KYNMWK42G-m1e-7d79a016 edit actor=human:Wido targets=the-size-hold-stops-at-the-production-ceiling reason=Allowed: build-without-design why=Allowed: build-without-design. One unit aligning the hold with decision 56; root read impact=This lets goal the-size-hold-stops-at-the-production-ceiling be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of the-size-hold-stops-at-the-production-ceiling until withdrawn. To undo: metasystem goal disallow the-size-hold-stops-at-the-production-ceiling build-without-design
- 2026-11-07T16:30:37Z CE39KBVP139FD5J94YCK483XPX-m1e-7d79a016 approve actor=human:Wido targets=the-size-hold-stops-at-the-production-ceiling
Integrity: sha256=225b9829d1118e83c80c6db60a36c19796bd7d316e019015e9a8863fb6b149ac
