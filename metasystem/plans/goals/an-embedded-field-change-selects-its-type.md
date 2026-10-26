# an-embedded-field-change-selects-its-type

- State: approved
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="A small refinement of the unit check's test selection; a miss runs fewer tests, never more harm than today."
- Tier: 1
- Intent: The unit check's impact selection notices a change to a field of an embedded struct: when an embedded struct's field changes, the selection adds the embedding type's name as a changed symbol, so the tests that reference that type run. Today such a change adds no symbol and those tests are skipped (read note N-2 of the-impact-check-runs-what-changed).
- Origin: main
- Next step: Split from the-impact-check-runs-what-changed at its goal done (read note N-2). Small: one design paragraph and one unit under 125 lines; needs Wido's approval before a seat takes it.
- OpenedAt: 2026-10-26T04:00:28Z
- Revision: 2
- Budget: elapsedLimit=5d attemptLimit=12 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=DRDPT7FRZRX9XRYV781ZTG2JJ6-m1e-acb921fd minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-10-26T07:13:03Z revision=2 opid=DRDPT7FRZRX9XRYV781ZTG2JJ6-m1e-acb921fd authority=proven digest=a15445dead1c807b2d02fb6ae42e28dd030437659caa6c988eefe89e4ea38ba4 episode=2

History:
- 2026-10-26T04:00:28Z 7HN0FJDE4TGTJBA5D10309KNE8-m1e-386083e0 open actor=human:Wido targets=an-embedded-field-change-selects-its-type
- 2026-10-26T07:13:03Z DRDPT7FRZRX9XRYV781ZTG2JJ6-m1e-acb921fd approve actor=human:Wido targets=an-embedded-field-change-selects-its-type
Integrity: sha256=f4a24b69fdb417fbbac31fbbbfcb5b8860ffedccace6990524f23a628ba5f107
