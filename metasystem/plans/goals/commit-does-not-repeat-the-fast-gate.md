# commit-does-not-repeat-the-fast-gate

- State: approved
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: each item removes measured waste inside one test or fixture and changes no law; novelty 1: known shapes; exposure 1: one group's duration; accumulation 1: none"
- Tier: 1
- Intent: scripts/agents/commit.sh lines 311 to 317 re-run go-gate.sh --fast after the proof run already executed fast-static-build; the testing-contract design says commit owns a verify-only boundary. DONE means commit consumes the retained fast-static-build proof instead of re-running the gate, saving about 9 s per landing, while the proof engine the landing boundary needs is still built.
- Origin: human
- Next step: Slice 4 item 7 of plans/suite-speed-plan.md. The proof-out build is still required for the landing boundary; only the duplicate gate run goes. Code critique only.
- OpenedAt: 2026-09-10T12:03:01Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T14:32:23Z revision=2 opid=1H40VRZVQWFMM6KWZC40XMR85J-m1-f47a9d40 authority=proven digest=faaa049b0f32d8cc3a0326303088aa1b70294eb874ee383be545e5ccee1b1cd5

History:
- 2026-09-10T12:03:01Z M9B003DZFK243SP0NK8P9KQSX6-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=commit-does-not-repeat-the-fast-gate
- 2026-09-11T14:32:23Z 1H40VRZVQWFMM6KWZC40XMR85J-m1-f47a9d40 approve actor=human:Wido targets=commit-does-not-repeat-the-fast-gate
Integrity: sha256=7345f855a7cf0276a643e9c53ca6abd5104b7090e14868680fb9f6d75a4a4603
