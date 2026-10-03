# units-are-sized-from-a-measured-inventory

- State: approved
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="Changes how large goals are planned; wrong sizing wastes attempts and hides scope growth"
- Tier: 2
- Intent: A multi-unit build plan sizes its units from a measured inventory, not estimates (Wido 2026-10-03, lessons from m1f's one-folder: roots planned at 900 lines needed ~2,330 and was split; root-structs planned at 1,000 needed 1,650; the measure that worked was attempt 2 of roots retyping the declarations in a scratch copy and counting what stops compiling, 1,167 sites in 234 files). Before the first build unit, the seat runs a measuring step (compiler-driven count, static audit or the class check) and the plan's units and line budgets come from its numbers; a unit that outgrows its measured size by a set margin re-plans instead of trimming to fit.
- Origin: main
- Next step: Short design: the measuring step as a standard first unit for tier-3 multi-unit goals (what it counts per language, behind the language adapter), how the plan records measured vs planned, the re-plan trigger; Astra critique tier 2; build.
- OpenedAt: 2026-10-03T07:56:05Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-03T07:56:36Z revision=2 opid=7WWZMS74NJ89M0GCKV8BP3MXT7-m1e-718ba0eb authority=proven digest=5ab6788305686e4e7fbf63e16ddd9f1cc262713f353727d98c904ee1765c0d98 episode=2

History:
- 2026-10-03T07:56:05Z DX0BN9TG9QEPPJGZT605X6C1J2-m1e-718ba0eb open actor=human:Wido targets=units-are-sized-from-a-measured-inventory
- 2026-10-03T07:56:36Z 7WWZMS74NJ89M0GCKV8BP3MXT7-m1e-718ba0eb approve actor=human:Wido targets=units-are-sized-from-a-measured-inventory
Integrity: sha256=24fd2d6a5e303d20330e10fe51bcae3fceee9db68ab285d0ff3e7aed5ff21b08
