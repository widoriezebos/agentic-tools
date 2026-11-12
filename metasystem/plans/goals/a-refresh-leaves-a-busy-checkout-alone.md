# a-refresh-leaves-a-busy-checkout-alone

- State: queued
- Risk: severity=3 novelty=1 exposure=2 accumulation=1 basis="The m1e checkout's index and worktree were left at a pre-merge tree under a moved HEAD after the refresh raced another writer (607)."
- Tier: 3
- Intent: The seat steward's engine refresh never restores an index it did not change: when it cannot take the index lock it stops before changing anything and retries; on a later failure it undoes only its own steps (finding 607, analysis cause K1).
- Origin: main
- Next step: Build one unit (45, --last) from a brief cut and Astra-checked before the build; program design /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/the-route-holds-when-something-deviates.md decision D12.
- OpenedAt: 2026-11-12T16:59:02Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-12T16:59:02Z V73J5CJC9DQDGWPD9B8349E03D-m1e-5a087708 open actor=human:Wido targets=a-refresh-leaves-a-busy-checkout-alone
Integrity: sha256=e479a1030e3a6b522c27dc938f7d8e54bee2c6e9638eaa8d21eac4129202edde
