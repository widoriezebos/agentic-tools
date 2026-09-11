# testing-contract-owns-behaviour-paths

- State: done
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: every agent commit touching an unowned behaviour path is refused for missing shared-testing proof, so chains touching cmd/metasystem/goal.go or internal/channel land only by a human commit; novelty 1: two surfaces in testing.json plus a fixture that walks the tree; exposure 3: every seat; accumulation 2: each landing on those paths pays a sovereign commit until fixed"
- Tier: 3
- Intent: The shared testing contract (metasystem/testing.json) owns no surface for five behaviour paths that chain bsws-build1b-20260909 changed: cmd/metasystem/goal.go, cmd/metasystem/goal_priority_test.go, cmd/metasystem/goalsync_mutations_test.go, internal/channel/report.go, internal/channel/channel_test.go. With testing.contract set, test plan refuses (delivery impact is unresolved: no surface owns changed path ...), test verify refuses the same, and commit.sh refuses every agent commit (required shared testing proof is missing or insufficient), so on 2026-09-10 the fully proven chain (four reads, a live proof, a receipt) could only land as a human commit under R-90-m1d (2f764c6099d08a232fd76db888421c0ea818f9d7). Sibling of testing-contract-owns-record-paths (m1e), which covers records; this one is behaviour paths. DONE means every behaviour path under cmd/ and internal/ is owned by a surface, starting with a goal-ledger-commands surface (cmd/metasystem/goal*.go, goalsync*.go) and a channel surface (internal/channel/**), and a fixture walks the tree and refuses an unowned behaviour path, so the gap cannot reopen when a package is added.
- Origin: main
- Next step: Appetite: 2h. Add the two surfaces to testing.json under the testing-policy protection rule, with standard and deep groups that already exist (command-interface-smoke, the goal-cli bed section, the channel unit group); add the tree-walk fixture. Coordinate with m1e on testing-contract-owns-record-paths, which touches the same file.
- Concluded: Landed: testing.json now carries the 'leftover' fallback surface (line 25: paths [], every standard group; contract 'fallback': 'leftover') landed by testing-contract-owns-record-paths in 2779ffa4, and the human-authority-and-channel surface owns internal/channel/** and channel_verbs.go; test plan/verify no longer refuse an unowned behaviour path, so the tree-walk fixture's premise (an unowned path refuses) is gone and the b. Concluded 2026-09-11 in the backlog consolidation on Wido's word, no further work.
- OpenedAt: 2026-09-10T12:14:54Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T12:14:54Z VPECY6FWNRVXPPN9C59HM6DBFS-m1-c6925449 open actor=human:Wido targets=testing-contract-owns-behaviour-paths
- 2026-09-11T22:09:14Z 2VG1YJ93YRBA2BSHX9N8XJ7RC7-m1-c6925449 done actor=human:Wido targets=testing-contract-owns-behaviour-paths
Integrity: sha256=19f71eb86429a8975597c0b2402c11bed901fe830f0d94626967266e85e37086
