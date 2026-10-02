# ui-shows-true-facts

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Wrong facts mislead the person's decisions (severity 2); fixes in existing readers (novelty 1); every page (exposure 2); read daily (accumulation 2)"
- Tier: 2
- Intent: Every count and state the web UI shows matches the engine (UI audit 2026-10-02, agentic-tools-evidence/ui-audit-20261002/report.md, section WRONG FACT and BROKEN): Needs you counts only what truly needs the person (priority 1 / actionable), one total shared by Overview and Decisions, open questions counted from the channel's open questions, In Progress cards show the engine's phase, a claimed goal shows claimed, the fleet health and expanded-row facts agree with the engine, duplicate known-issue rows gone, unknown tabs and goals refused, ruling rows titled by their sentence.
- Origin: main
- Next step: Fix each WRONG FACT and BROKEN item in report.md with a test that reads the same fact from the engine; check with Playwright (node script in internal/ui/web/_app) before and after
- OpenedAt: 2026-10-02T21:54:47Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-02T21:54:53Z revision=2 opid=BT9F3JXESQ027E6QZ70NTQVVDY-m1e-9c612d71 authority=proven digest=233bf29282f18fd29e586a8a4feb66d36f0e4ca80e81d9933b7f7cc573f20165 episode=2

History:
- 2026-10-02T21:54:47Z X6S7W8A5D6VQ483BBBRX5VSDGR-m1e-9c612d71 open actor=human:Wido targets=ui-shows-true-facts
- 2026-10-02T21:54:53Z BT9F3JXESQ027E6QZ70NTQVVDY-m1e-9c612d71 approve actor=human:Wido targets=ui-shows-true-facts
Integrity: sha256=2de8029830cc3e8e1d97ed04ade09944ccb9e70eeba4afed3eff1550e13891d5
