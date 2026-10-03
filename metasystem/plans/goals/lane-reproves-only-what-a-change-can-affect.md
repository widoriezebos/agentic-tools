# lane-reproves-only-what-a-change-can-affect

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Decides what the lane proves before every push to main; a wrong selection lets an unproved change land, an over-broad one stalls landing"
- Tier: 3
- Intent: When main moves by changes that only some tests can see, the landing lane re-runs only the test groups that read the changed files and keeps its green for the rest, so proofs stay correct and the lane stays live (Wido 2026-10-03, on Astra night-review finding F1: 'yes, I follow your recommendation'). DECISION RECORDED: the shortcut of 703ecb830 (a tree that differs from a green tree only in goal-ledger paths inherits green) STAYS until this goal lands. Impact Wido accepted: a goal-ledger change that breaks one of the few audit groups reading ledger files (shipped-installation-standard, agent-protocol-standard, fast-static-build, verb-ratchet) could reach main unproved and show as red at the next full proof; code changes are always fully proved. Undo: land the strict fix (patch f1-ledger-inherit-needs-unchanged-inputs.patch, commit 4a16c79b0), which re-proves on every ledger move and brings back the 03:30-05:35 lane livelock while seats are busy.
- Origin: main
- Next step: Tier-2 design: per-group proof reuse in the plain lane (project-rules.md line 50: a proof holds while every selected group's execution identity is unchanged): which groups a changed path selects (existing testing.json inputs and pathpattern matcher, as in commit 4a16c79b0), run only those, record a combined green; narrow over-broad inputs such as verb-ratchet's metasystem/**; language-generic (groups, not Go packages); Astra critique; build starting from branch fix/ledger-inherit-needs-unchanged-inputs; then remove the path-only shortcut.
- OpenedAt: 2026-10-03T08:54:48Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-03T08:54:48Z 74YXV0DW10Q79JHYTRMM2DV8M3-m1e-718ba0eb open actor=human:Wido targets=lane-reproves-only-what-a-change-can-affect
Integrity: sha256=d7b426e0edb5cdd042f76fe7a1906f73bbb23ce20eb7f58759a3672f737dc4f7
