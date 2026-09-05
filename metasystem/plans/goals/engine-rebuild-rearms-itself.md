# engine-rebuild-rearms-itself

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: the enrollment pin is the trust boundary for which engine becomes the steward and supervision owner, so a wrong relaxation lets unintended bytes supervise the repository; novelty 2: no auto re-enrollment existed, though the same act (mint a generation, replace the runner) is the shipped human arm path reused unchanged; exposure 3: every machine in the fleet arms through this path, and the scheduler's unattended recovery uses it too; accumulation 2: the wedge cost m1 a full day of refused turn ends on 2026-09-05 and cost m2 a night of re-prompts on 2026-09-04 (goal stop-hook-wedge-on-enrollment-drift), which treated the symptom"
- Tier: 3
- Intent: A rebuilt engine wedges every seat: metasystem up refuses ENROLLMENT_DRIFT whenever bin/metasystem's digest differs from the steward's enrolled pin, and rebuilding the engine is ordinary work here, so no session can arm until a human re-arms at an agent-free terminal. DONE means a rebuild of the enrolled engine at its own enrolled path, invoked by that engine, re-arms itself (fresh generation, runner replaced onto the new bytes, temporary human word and review date carried forward) and reports accepted-engine outcome=re-armed, while every drift cause that names a DIFFERENT installation still refuses to the human, proven by tests and a live rebuild-then-up on m1
- Origin: main
- Next step: Built and verified on m1, uncommitted, waiting on human approval to execute and land: internal/steward/identity.go types the rebuild cause (ErrEngineRebuilt), internal/steward/runner.go adds ReArmRebuiltEngine, internal/up/up.go routes only that cause to a re-arm and reports accepted-engine outcome=re-armed, with tests in both packages. Proven live three times including across a rebuild of 428-commits-newer code: up outcome=armed authority=writer. Landing needs a reviewed implementation chain (commit.sh refuses without --chain), so on approval: claim, dispatch a code-critic on codex for runtime independence from the claude implementer, fold findings, close the chain, land.
- OpenedAt: 2026-09-05T10:02:09Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-05T10:02:09Z CEK1GRKK6YHJN66D1A44EV2THJ-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=engine-rebuild-rearms-itself
Integrity: sha256=5511d8a4f0020d84f945a733b3dc4fdb6f650599a5fc2c29276834dd9bdc6b5d
