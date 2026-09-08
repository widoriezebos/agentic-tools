# norm-approval-reads-a-file-the-ledger-does-not-carry

- State: queued
- Priority: 3
- Sequence: 6
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="severity 2: a human act is refused for a reason that has nothing to do with the act, and the refusal names the wrong cause, so the human retries or gives up; novelty 2: the fix has to decide where a strict approval token lives when the thing it guards is a shared ledger; exposure 2: every over-norm raise on a machine with more than one checkout, which is every machine in this fleet; accumulation 2: twice in one day for one goal, and it recurs on every raise"
- Tier: 2
- Intent: An over-norm budget raise is refused when the human's terminal sits in a checkout whose working tree lacks the rulings row, even though the goal ledger the raise applies to is shared across checkouts. The engine reads the strict approval token from memory/rulings.md ON DISK (internal/goal/norm.go RecordedNormApproval reads repoRoot/memory/rulings.md), while the goal it guards syncs through refs/metasystem/goals. Seen twice on 2026-09-07 with goal metasystem-stop-verb: Wido ran goal set-budget --approved-ref R-83-m1b and then R-84-m1b from the checkout he was standing in, and both were refused with GOAL_NORM_REFUSED does not name a rulings-register row, because the seat that minted the row had landed it in a different checkout. The refusal names the ref as unknown, which reads as a bad reference rather than a stale tree, so the obvious retry is to doubt the ref. DONE means an over-norm raise resolves its token from the same shared source as the ledger, or the refusal distinguishes an unknown ref from a ref this checkout has not fetched and says which, with a fixture for both
- Origin: main
- Next step: read internal/goal/norm.go RecordedNormApproval, which reads memory/rulings.md at the repo root and also scans goal history operations. Candidate shapes: resolve the row from the ledger tip rather than the working tree; or, cheaper and honest, keep the read where it is and split the refusal so a ref that matches the row syntax but is absent from THIS tree says so and names the fetch. The second is a message and one comparison. Adjacent: goal enrollment is per checkout (goal hook-enrollment-per-checkout), so the human's terminal is often not in the checkout that minted the row
- OpenedAt: 2026-09-07T17:23:32Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-07T17:23:32Z ME8EJKSF7R2FJEWYZ13W5JVC9C-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=norm-approval-reads-a-file-the-ledger-does-not-carry
- 2026-09-08T15:59:25Z 4Z0F1RFSCTG2VY4NFTWQ28AX46-m1-7cd0bd60 set-priority actor=human:Wido targets=norm-approval-reads-a-file-the-ledger-does-not-carry reason=priority-order subject=norm-approval-reads-a-file-the-ledger-does-not-carry from=unranked to=3:6 requested-sequence=6
Integrity: sha256=8cd0db0a30de3fe80201b923213b8fc48ef73d97a354404babf795357f7008de
