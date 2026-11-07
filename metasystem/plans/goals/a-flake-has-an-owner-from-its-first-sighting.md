# a-flake-has-an-owner-from-its-first-sighting

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: A flake gets an owner at its first sighting, and an owner whose repair failed again is reopened (steward_flake.go:42, flakefix.go:43-49)
- Origin: human
- Next step: Build without a design from N7 and section 7 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: three states (repair pending, landed awaiting closure passes, recurring after repair); only the last reopens; abandoned owners keep the person route. Approve only after the incident dispositions of row 33 are done
- OpenedAt: 2026-11-07T09:53:08Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T09:53:08Z 5C28T70NWK63AYYY33RFSA32T1-m1e-1dae276b open actor=human:Wido targets=a-flake-has-an-owner-from-its-first-sighting
Integrity: sha256=382576afdf1456ae891abb15e148d1b904e3c87faf9046e4a435b86ecdfb40fe
