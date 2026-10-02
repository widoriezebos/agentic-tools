# ui-shows-true-facts

- State: approved
- Priority: 1
- Sequence: 4
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Wrong facts mislead the person's decisions (severity 2); fixes in existing readers (novelty 1); every page (exposure 2); read daily (accumulation 2)"
- Tier: 2
- Intent: Every count and state the web UI shows matches the engine (UI audit 2026-10-02, agentic-tools-evidence/ui-audit-20261002/report.md, section WRONG FACT and BROKEN): Needs you counts only what truly needs the person (priority 1 / actionable), one total shared by Overview and Decisions, open questions counted from the channel's open questions, In Progress cards show the engine's phase, a claimed goal shows claimed, the fleet health and expanded-row facts agree with the engine, duplicate known-issue rows gone, unknown tabs and goals refused, ruling rows titled by their sentence.
- Origin: main
- Next step: Fix each WRONG FACT and BROKEN item in report.md with a test that reads the same fact from the engine; check with Playwright (node script in internal/ui/web/_app) before and after
- OpenedAt: 2026-10-02T21:54:47Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-02T21:54:53Z revision=2 opid=BT9F3JXESQ027E6QZ70NTQVVDY-m1e-9c612d71 authority=proven digest=233bf29282f18fd29e586a8a4feb66d36f0e4ca80e81d9933b7f7cc573f20165 episode=2

History:
- 2026-10-02T21:54:47Z X6S7W8A5D6VQ483BBBRX5VSDGR-m1e-9c612d71 open actor=human:Wido targets=ui-shows-true-facts
- 2026-10-02T21:54:53Z BT9F3JXESQ027E6QZ70NTQVVDY-m1e-9c612d71 approve actor=human:Wido targets=ui-shows-true-facts
- 2026-10-02T21:54:59Z M0TSVAQNR16NW151930292CY3V-m1e-9c612d71 set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,critique-stops-on-convergence,cross-cutting-change-inventories-its-readers,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,fleet-forgets-an-unreachable-seat,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,rosters-are-configuration-items,round-proof-feeds-the-next-brief,seat-path-lands-without-help,seat-works-without-a-person,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,ui-connects-to-a-running-agent,ui-shows-true-facts,work-review-starts-its-critic reason=priority-order subject=ui-shows-true-facts from=unranked to=1:4 requested-sequence=4
Integrity: sha256=031d638861ff2ef3516ba0b76b8701e98b063201c21f11cceacf428a8b790d3a
