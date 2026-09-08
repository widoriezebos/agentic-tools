# chain-landing-after-base-move-recertifies

- State: claimed
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="Every chain that takes more than an hour on a busy fleet day meets it; today it left a landing carrying a would-refuse verdict; nothing is destroyed; the fix is a lane, not a patch."
- Tier: 3
- Intent: When main changes a file a reviewed chain also changes, between the chain's base and its landing, the landing evaluator (internal/landing/observe.go, bindCertifiedChange) finds the reviewed tree's entries differ from the candidate's and records would-refuse chain-output-mismatch, and the commit is admitted with that verdict. Seen 2026-09-06 on m1c landing chain shr-build1 at c1525b90: the engine-rearm landing ea8c3ead had touched supervision-hook.sh and both supervision fixture files after the chain's base; every hunk applied cleanly and the merged candidate passed the suite and the receipt seat-side, but the critic's certification named the pre-merge tree. The dispatcher already warns WORKTREE-BEHIND and suggests merging main into the chain worktree, yet after a merge the conformance diff and the critic's reviewed tree must be recomputed and the review budget may be spent. DONE means the landing lane has a named path for this case: merge main into the chain worktree, recompute conformance, and either a bounded re-review of the merge (one critic round that does not count against the goal's box, reviewing only the merge) or a mechanical proof that no hunk of the chain overlaps a hunk main added to the same file, after which the landing passes bar a instead of recording would-refuse; a fixture pins it.
- Origin: main
- Next step: Read bindCertifiedChange, the WORKTREE-BEHIND hint in the follow-up path, and the critique register's round accounting; decide between a merge-only review round outside the box and a no-overlap proof; brief one round with a fixture.
- OpenedAt: 2026-09-06T12:20:33Z
- Revision: 4
- Labels: headless-fleet, headless-process, robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-08T10:40:50Z revision=3 opid=JJ1SE0ZBSQDKGK5158E5VQ8AG3-m1b-c6925449 authority=proven digest=64be6fe4726bc5e9238a47cb7fed19867d6b78c5154e9a9b12e4253399596f0d
- Claimed: machine=m1b lineage=main-1788680071-18713-e76d5d at=2026-09-08T10:41:06Z revision=4 accountingRevision=4
- StopCapability: generation=4 revision=4 machine=m1b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T12:20:33Z A7NATSTMKZ7S25XXSMY10W6WYV-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=chain-landing-after-base-move-recertifies
- 2026-09-07T21:12:01Z RPKM7WBBCDYH5MC47RN3ZCNKX0-m1-76f67331 edit actor=human:Wido targets=chain-landing-after-base-move-recertifies
- 2026-09-08T10:40:50Z JJ1SE0ZBSQDKGK5158E5VQ8AG3-m1b-c6925449 approve actor=human:Wido targets=chain-landing-after-base-move-recertifies
- 2026-09-08T10:41:06Z 0Y4R9RJX78F88E9W3GQESP03NT-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=chain-landing-after-base-move-recertifies
Integrity: sha256=abca9beb3d4773e52299eedbdd8cfe337bec05aba8b38ec13592eefc496be819
