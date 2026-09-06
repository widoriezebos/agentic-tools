# landing-receipt-races-the-narrator-digest

- State: done
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="Every full-width chain landing on a live seat hits this; the workaround is manual and each hit costs a twenty-minute rerun; the exposure is every machine that lands chains."
- Tier: 3
- Intent: The full-battery landing receipt (landing test-receipt, run by land.sh for tier-1 and demanded by a full-width chain) refuses with 'the real index or working tree moved while the command ran' whenever the narrator appends to records/narrator-digest.log during the command. The command takes about twenty minutes (fast gate, dispatch fixtures, goal-cli fixtures) and the narrator writes on every ledger or landing movement anywhere in the fleet, so on a live seat the receipt fails by design of the two together; seen 2026-09-06 08:54Z on m1c for chain rgr-build1 after every fixture scenario had passed. DONE means the receipt's posture check ignores the narrator digest (a live log, not code under test) or the receipt runs against an isolated copy of the candidate tree, a fixture pins a digest append during the command as still-green, and the workaround of running the receipt in a detached worktree is retired from the recipe.
- Origin: main
- Next step: Round one built (worktree lrr-build1, reviewed tree e5e7f029): the receipt verb grafts the exact staged candidate into a detached worktree under the system temp directory, runs the command there, checks the candidate did not change, writes the receipt to the live root and removes the worktree on every path; a new gittree helper (internal/gittree/detached.go) and seven Go test legs. Seat-side proof (15:24Z): the landing and gittree packages green; a live receipt with a six-second command stayed green while the narrator digest was appended to during it, the staged candidate stayed intact, the receipt landed in the live root, no temporary worktree remained. Critique lrr-critic1 is running; then close, records, and a landing whose own receipt the new verb makes in the live root.
- Concluded: Landed 5ddb7089 (chain lrr-build1, two rounds, critiques lrr-critic1 and lrr-critic2, the second with zero material findings; dispositions 6eabe574; provenance pass bar a). The landing receipt verb grafts the exact staged candidate into a detached worktree under the system temp directory, runs the battery there, checks the candidate did not change, writes the receipt to the live root and removes the worktree on every path without a repository-wide prune; seven Go legs pin it. Proven by this landing itself: the candidate's own verb made the receipt in the live root while the narrator appended a line to its digest during the run, and the landing went through.
- OpenedAt: 2026-09-06T08:56:44Z
- Revision: 7
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T11:02:38Z revision=2 opid=SB01VF9Q2A59X40GM66RJJP2JV-m1-a4f8999f authority=proven digest=f4a673219b1d974e137d17e405d4192358e9cdeae4c422d5c9638194e70d2e2e
- Sliced: machine=m1c lineage=main-1788680061-17829-64951c revision=3 at=2026-09-06T14:54:17Z

History:
- 2026-09-06T08:56:44Z XNZBQNDJX3QFRAN4NR8D9YHA8K-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=landing-receipt-races-the-narrator-digest
- 2026-09-06T11:02:38Z SB01VF9Q2A59X40GM66RJJP2JV-m1-a4f8999f approve actor=human:Wido targets=code-critic-runtime-has-no-shell,human-acts-derive-their-lineage,land-sh-omits-the-full-width-chain-receipt,landing-receipt-races-the-narrator-digest
- 2026-09-06T14:52:51Z NWZEPTPR8NC8S3F5XM6EADQK9J-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=landing-receipt-races-the-narrator-digest
- 2026-09-06T14:54:17Z ECZ0WY1TNX87F8P62G581JD1YN-m1c-7cd0bd60 slice-start actor=m1c+main-1788680061-17829-64951c targets=landing-receipt-races-the-narrator-digest
- 2026-09-06T14:54:40Z 2SFPWMYQ3GWVGMCNZ6569TQNAQ-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=landing-receipt-races-the-narrator-digest
- 2026-09-06T15:25:13Z 29H2PJV4423ZERBDY5HKY95W2P-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=landing-receipt-races-the-narrator-digest
- 2026-09-06T16:25:35Z 35NT6DA45VDGC1G92YC4SDA7PF-m1c-7cd0bd60 done actor=m1c+main-1788680061-17829-64951c targets=landing-receipt-races-the-narrator-digest
Integrity: sha256=275e22d162565689c2abbabdae548de61e4a2dbbc583e19cfc5661afc156b2db
