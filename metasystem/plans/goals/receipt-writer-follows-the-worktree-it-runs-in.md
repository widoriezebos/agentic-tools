# receipt-writer-follows-the-worktree-it-runs-in

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: a receipt in the wrong checkout is lost or double-recorded; novelty 1: the writer exists, its root resolution changes; exposure 3: every landing from a worktree; accumulation 1: one writer and one root resolver"
- Tier: 2
- Intent: scripts/receipt.sh add writes the MAIN checkout ledger even when run from a linked worktree (internal/stateroot resolves to the main checkout), so a landing lane row lands in R0 while its commit is built elsewhere, and every ff-merge and rebase on R0 then collides with an uncommitted append. On 2026-09-15/16 this led two seats to discard ledger lines by hand and cost the fleet dozens of steward and narrator appends, seven receipt rows of which were restored from a stash. DONE: a receipt added from a linked worktree is written to that worktree ledger, or the writer refuses naming the worktree and the one command that writes it in the right place; a fixture runs the writer from a worktree and shows the main checkout ledger unchanged; the mechanism lives in the engine and the shell contract, never one runtime, and is proven on two runtimes.
- Origin: human
- Next step: Locate the root resolution (internal/stateroot) and the receipt writer; decide worktree-local write versus refusal-with-remedy; one unit with the fixture; Opus read; land.
- OpenedAt: 2026-09-16T06:43:05Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-16T06:43:05Z 391XJEQMBX338BR0VH5DZF192W-m1e-c6925449 open actor=human:Wido targets=receipt-writer-follows-the-worktree-it-runs-in
Integrity: sha256=41370d99a9e7e3907d457e84b4eeed537a4e7380e7e5bc5937c1002d5480a921
