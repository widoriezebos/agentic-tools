# a-batch-gate-is-accepted-for-its-tree

- State: approved
- Priority: 1
- Sequence: 124
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, root cause read in a recorded finding (556, 557); the unit check and the lane proof gate it"
- Tier: 2
- Intent: A recorded batch gate is accepted again for the same tree (Skipped result, Green static check, same BaseCommit and PlanHash) instead of only for the same commit, so a re-merge after an abandoned resolution does not run the gate twice (finding 556; internal/landing/plain/impact.go:103-105, prove.go:467, scope.go:498)
- Origin: human
- DesignGate: off
- Next step: Build without a design from finding 556: key the gate acceptance on the tree; one test with two merge commits of one tree; about 20 lines
- OpenedAt: 2026-11-07T15:02:25Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-07T15:02:45Z revision=3 opid=SAN72GZ3YQHDSAB5NHA6VDBDJE-m1e-5a3e856b authority=proven digest=683b3409040a95f18d0aac7553fcf0354c09a3989707b20a726cde3fe0b399ba episode=3

History:
- 2026-11-07T15:02:25Z FYXYRR9049G82FDFAZ9HG6HHCX-m1e-5a3e856b open actor=human:Wido targets=a-batch-gate-is-accepted-for-its-tree
- 2026-11-07T15:02:35Z 0V3N0FWSB9QKPD5RPF85Z81KWM-m1e-5a3e856b edit actor=human:Wido targets=a-batch-gate-is-accepted-for-its-tree reason=Allowed: build-without-design why=Allowed: build-without-design. One unit from a recorded finding (556) whose root cause is read impact=This lets goal a-batch-gate-is-accepted-for-its-tree be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-batch-gate-is-accepted-for-its-tree until withdrawn. To undo: metasystem goal disallow a-batch-gate-is-accepted-for-its-tree build-without-design
- 2026-11-07T15:02:45Z SAN72GZ3YQHDSAB5NHA6VDBDJE-m1e-5a3e856b approve actor=human:Wido targets=a-batch-gate-is-accepted-for-its-tree
- 2026-11-07T15:02:55Z DY25DR7E2WZXFZ3KWVMMV5F280-m1e-5a3e856b set-priority actor=human:Wido targets=a-batch-gate-is-accepted-for-its-tree reason=priority-order subject=a-batch-gate-is-accepted-for-its-tree from=unranked to=1:124 requested-sequence=append
Integrity: sha256=29b76f5071e1f93690fd693342a9f12231cc77ab35f5893503b98230a11ecc58
