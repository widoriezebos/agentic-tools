# the-impact-check-runs-what-changed

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Fable's round-2 check read 2026-10-24 13:10 (outside-read-fable-round-2-check.md, root cause A, row 2; decisions 57-58): for a 17-file change the-second-half's frozen check selected 120 entries and ran 126 groups (2111 of 2332 cmd tests) in 61 minutes; impact.go:126 runs every transitive internal dependent whole and impact.go:209 turns three-rune literals such as goal/show/status into symbols. This is the rule Wido retired on 2026-09-19. A correct selection is about 12-15 minutes."
- Tier: 2
- Intent: The unit check's impact selection runs what a change touches: a dependent internal package runs only the tests that reference a changed exported symbol, never whole; a changed string literal selects a test file only when it is a message (contains a space); a fixture directory name is no longer a symbol. Changed packages keep today's rule. One unit, about 25 production lines, built with --last.
- Origin: human
- Next step: Cut the brief (opus-designer, BRIEF-CHECKLIST, Read each round: yes, --last, stop line 250), one Astra round, then the machine route on ui right after the-second-half-is-the-seats.
- OpenedAt: 2026-10-24T11:24:03Z
- Revision: 2
- Budget: elapsedLimit=5d attemptLimit=12 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=MM15Y0ND5F9XRQ7W15H25JA0ER-m1e-5ea50bdc minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-10-24T11:24:13Z revision=2 opid=MM15Y0ND5F9XRQ7W15H25JA0ER-m1e-5ea50bdc authority=proven digest=0a1efd8b9d723812ff00941a94586437c093b7f85969d319886dedf2b8479c75 episode=2

History:
- 2026-10-24T11:24:03Z H21PE0N779P0B29C684DDHZ3H9-m1e-5ea50bdc open actor=human:Wido targets=the-impact-check-runs-what-changed
- 2026-10-24T11:24:13Z MM15Y0ND5F9XRQ7W15H25JA0ER-m1e-5ea50bdc approve actor=human:Wido targets=the-impact-check-runs-what-changed
Integrity: sha256=74f43228662adbe622368f154d6a33c0988dd8fd738a2b517feee5ba5f49a09d
