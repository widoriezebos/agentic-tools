# session-start-relays-the-durable-wait-line

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: a revived seat whose SessionStart drops the durable wait line loses the pending-wait context it needs to continue, and the supervision bed is red on trunk; novelty 2: an interaction between the new start finaliser and a runtime without a declared start context; exposure 3: every seat restart or compaction in that environment; accumulation 2: a red supervision bed hides new failures in the same scenario"
- Tier: 2
- Intent: Since 157fac92 (hook-root-resolver-design, landed by m1c) the supervision bed's wait-restart scenario is red on trunk. TestWaitSessionStartPrintsPendingRows (cmd/metasystem/wait_verb_test.go:505) reports 'session-start hook did not relay the durable wait line', and the output names a missing fake signature adapter, 'no start context declared for fake' and an arming refusal (internal/census/ancestor_production.go:136). m1b reported it on 2026-09-14: it reproduces on trunk ded41d16 through the bed child path, passes when the Go test runs directly, and passed inside the bed at 80059012. DONE: the cause and the environment difference are recorded; the SessionStart path never drops the durable wait line a seat needs, whether arming is refused or the runtime declares no start context, or the design records why the test expectation was wrong; a regression proof fails before the fix in the bed's environment and passes after; wait-restart passes inside the supervision bed on trunk; metasystem audit hook-start-exits still passes; no Stop decision changes.
- Origin: human
- Next step: Codex Sol diagnosis and fix dispatched by m1c in scratchpad g12/wt20; m1c claims it once go-tests-never-inherit-a-candidate-engine lands.
- OpenedAt: 2026-09-14T22:00:35Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T22:00:35Z FS0N9XQYE5XQCFCTT74S49ADAN-m1e-c6925449 open actor=human:Wido targets=session-start-relays-the-durable-wait-line reason=TierOverride: derived=3 set=2 why=Opened by m1c in Wido's name under R-110-m1e: a trunk regression from m1c's own landing 157fac92, reported by m1b.
Integrity: sha256=a18c230f80aafe0a09153a329ab081363986a792f5c3492cd47daa0a128cf8df
