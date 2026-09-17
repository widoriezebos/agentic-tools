# reads-keep-the-whole-diff-in-context

- State: approved
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="A launcher choice per read; a wrong choice costs one rerun."
- Tier: 2
- Intent: An independent read judges the whole diff without compacting. DONE: (1) a read over 1,200 diff lines runs per package, or headless in its own window (400K), chosen by the launcher from the diff size; (2) the read's return records its compaction count, and a read that compacted is rerun split before its verdict counts; (3) the retro reports zero compacted reads.
- Origin: human
- Next step: Evidence 2026-09-17: the reads of blb Build B (2,219 lines) and rom R2 compacted twice each under the 200K delegate window, so the reader judged the last files against a summary of the first; the C+D read (3,187 lines) ran headless at 400K as the workaround (scripts/agents/headless-design-launch.sh with CLAUDE_CODE_AUTO_COMPACT_WINDOW). No design first: brief the size rule, the window and the return field, build, read. Lands on the launcher of delegate-launchers-become-go-verbs; the window keys come from efficiency-settings-ship-in-the-repository.
- OpenedAt: 2026-09-17T13:39:34Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-17T13:39:53Z revision=2 opid=ZSTP02XSE2GK11Q40DRD4REDM1-m1e-c6925449 authority=proven digest=55fb7d3f4724ba8b78f2b9460d111ccc6164c2c0b85ab8c4d55ba8f91d871fdb

History:
- 2026-09-17T13:39:34Z JKXEBQE08X0WT13KJBAYGDS4TZ-m1e-c6925449 open actor=human:Wido targets=reads-keep-the-whole-diff-in-context
- 2026-09-17T13:39:53Z ZSTP02XSE2GK11Q40DRD4REDM1-m1e-c6925449 approve actor=human:Wido targets=reads-keep-the-whole-diff-in-context
Integrity: sha256=2ee081a74a5130726543c793eca5bab7d2f38a2785d32f5a397447de1a1df8e1
