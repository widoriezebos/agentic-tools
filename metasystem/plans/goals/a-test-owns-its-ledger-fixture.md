# a-test-owns-its-ledger-fixture

- State: queued
- Risk: severity=3 novelty=1 exposure=2 accumulation=1 basis="a red on main holds every hand-in on every host; the change is test code only; the lane proof gates it"
- Tier: 3
- Intent: Main red: TestLandingMainMoveRealContractPlanGitAdapter (cmd/metasystem/landing_main_move_test.go:34) opens the live ledger file plans/goals/lane-first-run-fixes.md as its fixture; the goal was concluded on 2026-11-07 and the file moved, so the test fails on main (incident red:metasystem/cmd/metasystem:TestLandingMainMoveRealContractPlanGitAdapter, opened 19:48). A test owns its fixture: it writes the goal file it needs into its own bed (or picks any live goal file by listing), never a named live ledger file; the same audit finds every other test that opens a plans/goals/<name>.md by name
- Origin: human
- Next step: Build without a design, one unit: the fixture in the test bed; an audit test that greps cmd/ and internal/ tests for plans/goals/<slug>.md literals and fails on any; fix the others it finds the same way; about 40 production lines
- OpenedAt: 2026-11-07T18:56:13Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T18:56:13Z A8MV8WQR0Z47BJM8NTRM80TXHA-m1e-746c9523 open actor=human:Wido targets=a-test-owns-its-ledger-fixture
Integrity: sha256=adacc01fb7defcb4de1d564a5474f552d989db983ebd3126c8d951e1530bd374
