# closing-read-and-receipt-in-parallel

- State: approved
- Priority: 1
- Sequence: 34
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: time, not correctness; novelty 1: the receipt already binds the tree; exposure 2: every chain landing; accumulation 1: nothing built on it"
- Tier: 2
- Intent: A build chain's closing read and its landing receipt are run one after the other by every coordinator today, though nothing binds them: the read judges a frozen round tree, the receipt binds the same tree staged on main, and neither changes the tree. On 2026-09-10 a MECHANICAL chain waited 54 minutes for its receipt after its round, then would have waited for a read; run together they cost the longer of the two. DONE means: docs/orchestration.md states the practice (stage the candidate, dispatch the closing read, take the receipt in the same turn; a material finding voids the receipt and the fold restarts both), land.sh accepts a receipt taken before the read closed as long as the receipt's tree equals the closed round's reviewed tree, and a fixture proves a receipt taken before the read and a clean read land together.
- Origin: main
- Next step: Tier 1, MECHANICAL: one doc paragraph, one check in land.sh, one fixture. Interim practice on m1b from 2026-09-10 12:00: the coordinator runs both in parallel. | FIRST ATTEMPT 2026-09-10 12:55Z (m1b, chain rbce-build1): the receipt's proof reservation goes through EvaluateGoalRevisionAdmission with the goal's activeJobLimit, so with the closing read live the receipt is refused 'proof reservation refused for goal ... revision N' (cmd/metasystem/proof_run.go:533). Worked around with set-budget --active-job-limit 2 in Wido's name. DONE additionally means: a proof reservation for a candidate whose critic read is live is admitted beside it (the read and the proof are the two halves of one certification), without the human raising the active-job limit.
- OpenedAt: 2026-09-10T08:45:30Z
- Revision: 5
- Pinned: m1c
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-13T06:41:00Z revision=4 opid=AV13C99R3FA0783F9AMD28MMQ8-m1e-c6925449 authority=proven digest=348e8a3f878c93f8796fcada0b23b66f78fdb3c1a5930c580da070d774d1d23f

History:
- 2026-09-10T08:45:30Z 291A0WATNK558JY5QZKZ0FK1GR-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=closing-read-and-receipt-in-parallel
- 2026-09-10T09:12:09Z RVMTAWZFVHY5ZEFP1ZBRVYT3BR-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=closing-read-and-receipt-in-parallel
- 2026-09-13T06:40:54Z KT84MZ5D0GTTRZF5Q0K1R9TVPM-m1e-c6925449 set-pin actor=human:Wido targets=closing-read-and-receipt-in-parallel
- 2026-09-13T06:41:00Z AV13C99R3FA0783F9AMD28MMQ8-m1e-c6925449 approve actor=human:Wido targets=closing-read-and-receipt-in-parallel
- 2026-09-13T06:44:48Z P20NT4ZWGZRA8M5CH7PZQGJE0Q-m1e-c6925449 set-priority actor=human:Wido targets=closing-read-and-receipt-in-parallel reason=priority-order subject=closing-read-and-receipt-in-parallel from=unranked to=1:34 requested-sequence=34
Integrity: sha256=4ecefe112f8ec9b605247992912c039dd0c16e82ff6a64c09a22d3dbbbb83399
