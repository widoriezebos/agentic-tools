# Design brief: machinery-housekeeping (plan goal 7)

Working Mode: Design
Opened 2026-10-07 in Wido's word. Read first: `plans/machinery-open-findings-plan-2026-10-06.md` findings 14, 17, 18, 19, 21, the "Learned 2026-10-07" section, and `plans/engine-inputs-design-brief.md` (the engine inputs and stamp, dropped from lane-drain-and-fresh-claims on 10-07, and the findings-store isolation note recorded there on 10-07). SIZE: each unit at most 250 production lines, the goal at most 5 units.

## Intent

Small, known fixes bundled: flakes fixed at their source, the flake kind of the register, the engine stamp and inputs, and the remaining small findings.

## Must deliver, in this order

1. **Flakes at the source first (10-07):** `TestWorkRebaseGitAdapterHoldsAfterHistory` and `TestLauncherDeathKillsTheTest` (load-fragile: wall-clock waits and TempDir cleanup; convert to injectable clocks and owned cleanup per Wido 09-12), and the findings-store isolation under parallel tests (`TestIntentReadVerdictFromRetainedFindings`, `TestIntentGeneratedUnitPlan`, `TestIntentBuild{RoundLimitAndReadBudget,RetainedRequest,ConcurrentRepeat}`: "no store record of this owner names the path", reproduced on main-equivalent trees with `-count=5 -parallel 64`; likely a shared store registry and TMPDIR swept by a steward in the test registry home). A flaky full suite doubles every gate; this unit lands before the register.
2. The flake kind of the register (finding 14): a known flake gets its one repeat, recorded, with the test's own output as evidence.
3. The engine stamp and inputs (plans/engine-inputs-design-brief.md).
4. Findings 17, 18, 19, 21 as the plan states them, smallest form each.

## Shape

As the accepted designs (see plans/fleet-survives-its-providers-design-brief.md, "Shape").
