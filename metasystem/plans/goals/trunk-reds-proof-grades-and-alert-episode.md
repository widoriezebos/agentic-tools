# trunk-reds-proof-grades-and-alert-episode

- State: queued
- Risk: severity=3 novelty=1 exposure=3 accumulation=2 basis="severity 3: two groups red on main make every deep proof insufficient, so nothing lands until they are fixed; novelty 1: fixes inside existing fixtures and the episode code; exposure 3: every seat landing through the deep proof; accumulation 2: held units go stale while main moves"
- Tier: 3
- Intent: Two fixture scenarios are red on main (origin/main 0e4e8327f, base tree b10999c6, attempt proof-mu4ebrii-7205c539f4e61b18, 2026-09-16 18:06Z) with no candidate unit in the tree: (1) section/goal-cli-fixtures scenario proof-grades, goal-cli-fixtures.sh:743 "headless human-word caller did not receive the terminal-not-reached refusal" (a setsid tty-less `goal release --by Wido` was CONFIRMED); (2) section/supervision-and-census-fixtures health scenario alert-episode "failure five must open one digest-keyed episode, found 2". Both were green at about 17:00Z in proof-mu4ctkn6 under engine generation 152 and red at 17:45Z and 18:00Z under generation 153, after the four units 423be4ebf, f045749ea, 32f920eb8 and 47a5d5d5e landed. Lane 5 stopped on the red-on-base rule and held drop-reasons and headless-launchers-live-in-the-repo. Wido 2026-09-16 20:30 CEST: highest priority to fix. DONE: (1) the root cause of each red is recorded here with file:line and the input or shared state that differed between the green and the red runs; (2) each fix lands with a witness (Go test or fixture assertion) that goes red under the mutation that reverts it, never a skip, retry, weaker floor or raised bound, artificial clocks only; (3) a deep proof at the fixed tip executes section/goal-cli-fixtures and section/supervision-and-census-fixtures green with no reuse and no rerun, recorded here with the attempt id.
- Origin: human
- Next step: Opened in the name of Wido by seat m1e from the enrolled pane on his word 2026-09-16 20:30 CEST. Codex gpt-5.6-sol diagnosis task-mu4f5qih-3hx4fg runs in worktree .claude/worktrees/trunk-reds (task S5/trunk-reds-task.txt; report metasystem/artifacts/reports/trunk-reds-diagnosis.md). The seat verifies any fix by hand (each scenario standalone twice, the package tests, go-gate --fast) and lands it under the fix-forward rule; the next deep proof (the held drop-reasons plus headless-launchers stack) is the DONE (3) evidence. Lead for units-land-in-batches-under-one-proof: a red on main gets a goal with an owner on the ledger and holds only the batches that select the failing groups.
- OpenedAt: 2026-09-16T18:29:13Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-16T18:29:13Z M7V381FZ4M5PVVNNH1VY659RSQ-m1e-c6925449 open actor=human:Wido targets=trunk-reds-proof-grades-and-alert-episode
Integrity: sha256=3cc61c83be9e5c71471fba54a6d1b4ccc2efccca3e2e834d6be6a6b7fec3c58d
