# lane-reproves-only-what-a-change-can-affect

- State: approved
- Priority: 1
- Sequence: 2
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Decides what the lane proves before every push to main; a wrong selection lets an unproved change land, an over-broad one stalls landing"
- Tier: 3
- Intent: When main moves by changes that only some tests can see, the landing lane re-runs only the test groups that read the changed files and keeps its green for the rest, so proofs stay correct and the lane stays live (Wido 2026-10-03, on Astra night-review finding F1: 'yes, I follow your recommendation'). DECISION RECORDED: the shortcut of 703ecb830 (a tree that differs from a green tree only in goal-ledger paths inherits green) STAYS until this goal lands. Impact Wido accepted: a goal-ledger change that breaks one of the few audit groups reading ledger files (shipped-installation-standard, agent-protocol-standard, fast-static-build, verb-ratchet) could reach main unproved and show as red at the next full proof; code changes are always fully proved. Undo: land the strict fix (patch f1-ledger-inherit-needs-unchanged-inputs.patch, commit 4a16c79b0), which re-proves on every ledger move and brings back the 03:30-05:35 lane livelock while seats are busy.
- Origin: main
- Next step: Tier-2 design: per-group proof reuse in the plain lane (project-rules.md line 50: a proof holds while every selected group's execution identity is unchanged): which groups a changed path selects (existing testing.json inputs and pathpattern matcher, as in commit 4a16c79b0), run only those, record a combined green; narrow over-broad inputs such as verb-ratchet's metasystem/**; language-generic (groups, not Go packages); Astra critique; build starting from branch fix/ledger-inherit-needs-unchanged-inputs; then remove the path-only shortcut.
- OpenedAt: 2026-10-03T08:54:48Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-03T08:55:02Z revision=2 opid=D97PHF51W9HGHC3CPQRSJ1ZVWZ-m1e-718ba0eb authority=proven digest=d49831747d574b76d851f3f2d590e73b0f5ea1aaaa00772b0108ead25970ff84 episode=2

History:
- 2026-10-03T08:54:48Z 74YXV0DW10Q79JHYTRMM2DV8M3-m1e-718ba0eb open actor=human:Wido targets=lane-reproves-only-what-a-change-can-affect
- 2026-10-03T08:55:02Z D97PHF51W9HGHC3CPQRSJ1ZVWZ-m1e-718ba0eb approve actor=human:Wido targets=lane-reproves-only-what-a-change-can-affect
- 2026-10-03T08:55:13Z X4ERA2S34QV3PDK0J0T2TQ4H9K-m1e-718ba0eb set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,channel-questions-stand-alone,conflicts-resolve-unattended,critique-stops-on-convergence,cross-cutting-change-inventories-its-readers,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,fleet-forgets-an-unreachable-seat,fleet-page-redesign,fleet-panel-ux,lane-reproves-only-what-a-change-can-affect,machinery-blocks-of-2026-10-02,one-folder-deployed-and-evolved,one-steward-per-checkout-on-its-own-root,overrides-state-their-impact,partner-runtime-defaults-to-an-available-one,questions-carry-what-they-ask-about,receipt-writer-follows-the-worktree-it-runs-in,rosters-are-configuration-items,round-proof-feeds-the-next-brief,seat-path-lands-without-help,seat-works-without-a-person,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,ui-connects-to-a-running-agent reason=priority-order subject=lane-reproves-only-what-a-change-can-affect from=unranked to=1:2 requested-sequence=2
Integrity: sha256=fb939b91bf101aafd9383f6934707b5d1749c1e6ca90f7e2e1d42c6586ca6181
