# testing-contract-owns-behaviour-paths

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: every agent commit touching an unowned behaviour path is refused for missing shared-testing proof, so chains touching cmd/metasystem/goal.go or internal/channel land only by a human commit; novelty 1: two surfaces in testing.json plus a fixture that walks the tree; exposure 3: every seat; accumulation 2: each landing on those paths pays a sovereign commit until fixed"
- Tier: 3
- Intent: The shared testing contract (metasystem/testing.json) owns no surface for five behaviour paths that chain bsws-build1b-20260909 changed: cmd/metasystem/goal.go, cmd/metasystem/goal_priority_test.go, cmd/metasystem/goalsync_mutations_test.go, internal/channel/report.go, internal/channel/channel_test.go. With testing.contract set, test plan refuses (delivery impact is unresolved: no surface owns changed path ...), test verify refuses the same, and commit.sh refuses every agent commit (required shared testing proof is missing or insufficient), so on 2026-09-10 the fully proven chain (four reads, a live proof, a receipt) could only land as a human commit under R-90-m1d (2f764c6099d08a232fd76db888421c0ea818f9d7). Sibling of testing-contract-owns-record-paths (m1e), which covers records; this one is behaviour paths. DONE means every behaviour path under cmd/ and internal/ is owned by a surface, starting with a goal-ledger-commands surface (cmd/metasystem/goal*.go, goalsync*.go) and a channel surface (internal/channel/**), and a fixture walks the tree and refuses an unowned behaviour path, so the gap cannot reopen when a package is added.
- Origin: main
- Next step: Appetite: 2h. Add the two surfaces to testing.json under the testing-policy protection rule, with standard and deep groups that already exist (command-interface-smoke, the goal-cli bed section, the channel unit group); add the tree-walk fixture. Coordinate with m1e on testing-contract-owns-record-paths, which touches the same file.
- OpenedAt: 2026-09-10T12:14:54Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T12:14:54Z VPECY6FWNRVXPPN9C59HM6DBFS-m1-c6925449 open actor=human:Wido targets=testing-contract-owns-behaviour-paths
Integrity: sha256=a66a6c13b1063ff3c439b4dbe5ba7efcee6b3a7732cef6c0a19ce054c252a7ec
