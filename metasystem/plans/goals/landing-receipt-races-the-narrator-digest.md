# landing-receipt-races-the-narrator-digest

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="Every full-width chain landing on a live seat hits this; the workaround is manual and each hit costs a twenty-minute rerun; the exposure is every machine that lands chains."
- Tier: 3
- Intent: The full-battery landing receipt (landing test-receipt, run by land.sh for tier-1 and demanded by a full-width chain) refuses with 'the real index or working tree moved while the command ran' whenever the narrator appends to records/narrator-digest.log during the command. The command takes about twenty minutes (fast gate, dispatch fixtures, goal-cli fixtures) and the narrator writes on every ledger or landing movement anywhere in the fleet, so on a live seat the receipt fails by design of the two together; seen 2026-09-06 08:54Z on m1c for chain rgr-build1 after every fixture scenario had passed. DONE means the receipt's posture check ignores the narrator digest (a live log, not code under test) or the receipt runs against an isolated copy of the candidate tree, a fixture pins a digest append during the command as still-green, and the workaround of running the receipt in a detached worktree is retired from the recipe.
- Origin: main
- Next step: Read internal/landing/receipt.go receiptPosture and internal/gittree Snapshot; decide between excluding the digest path from the posture snapshot and snapshotting into a temporary worktree; brief one round with a fixture.
- OpenedAt: 2026-09-06T08:56:44Z
- Revision: 4
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T11:02:38Z revision=2 opid=SB01VF9Q2A59X40GM66RJJP2JV-m1-a4f8999f authority=proven digest=f4a673219b1d974e137d17e405d4192358e9cdeae4c422d5c9638194e70d2e2e
- Sliced: machine=m1c lineage=main-1788680061-17829-64951c revision=3 at=2026-09-06T14:54:17Z
- Claimed: machine=m1c lineage=main-1788680061-17829-64951c at=2026-09-06T14:52:51Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m1c claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T08:56:44Z XNZBQNDJX3QFRAN4NR8D9YHA8K-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=landing-receipt-races-the-narrator-digest
- 2026-09-06T11:02:38Z SB01VF9Q2A59X40GM66RJJP2JV-m1-a4f8999f approve actor=human:Wido targets=code-critic-runtime-has-no-shell,human-acts-derive-their-lineage,land-sh-omits-the-full-width-chain-receipt,landing-receipt-races-the-narrator-digest
- 2026-09-06T14:52:51Z NWZEPTPR8NC8S3F5XM6EADQK9J-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=landing-receipt-races-the-narrator-digest
- 2026-09-06T14:54:17Z ECZ0WY1TNX87F8P62G581JD1YN-m1c-7cd0bd60 slice-start actor=m1c+main-1788680061-17829-64951c targets=landing-receipt-races-the-narrator-digest
Integrity: sha256=42057fffdcb09d0e4698ef8b956b10bc15551e75082526d088ce0e1078a473b1
