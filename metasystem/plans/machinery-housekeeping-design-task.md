# Task: write the design for machinery-housekeeping (plan goal 7)

Working Mode: Design
Write plans/designs/machinery-housekeeping.md from the brief plans/machinery-housekeeping-design-brief.md (read it in full first), plans/engine-inputs-design-brief.md (the engine inputs and stamp, and the findings-store isolation note of 10-07), plans/machinery-open-findings-plan-2026-10-06.md (findings 14, 17, 18, 19, 21 and "Learned 2026-10-07"), and plans/designs/machinery-mechanisms.md (the flake kind of the register, the engine stamp).

SIZE (Wido 2026-10-07): each unit at most 250 production lines (tests excluded), the goal at most 5 units; if it does not fit, keep the brief's order (flakes at the source first, then the flake register, then the engine stamp and inputs) and split the rest into a follow-up brief BEFORE the units table. Production callers only. Each unit's check is only its new and changed tests and their mutations; the full suite runs once when the goal lands.

Unit 1 is the flakes at their source: TestWorkRebaseGitAdapterHoldsAfterHistory and TestLauncherDeathKillsTheTest (load-fragile: injectable clocks and owned cleanup, per Wido 09-12 "artificial clocks, never load-fragile tests") and the findings-store isolation under parallel tests (TestIntentReadVerdictFromRetainedFindings, TestIntentGeneratedUnitPlan, TestIntentBuild{RoundLimitAndReadBudget,RetainedRequest,ConcurrentRepeat}: "no store record of this owner names the path"; reproduce with -count=5 -parallel 64 and name the cause from the code before designing the fix).

Shape as the accepted designs: header (Kind: design, Id: a new ULID, Status: draft, Goals: machinery-housekeeping), Wido's binding words, what it delivers, Decisions with code sites read on the current tree (cite file:line you read), Units table (unit, intent, production lines, areas), Estimates, one public-verb (or, for a test-only fix, one reproduction) test per unit failing under its mutation, readers; the five questions for every new function, record and act; the four defect classes.

Do not touch code, memory/, records/ or the ledger. Never open any metasystem.conf.local. Return: page path, units with production-line estimates, anything split off.
