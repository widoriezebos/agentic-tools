# census-lifecycle-scenario-holds-under-load

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: a false red costs each seat a classification per proof and may hide a real census defect; novelty 1: one scenario; exposure 3: every seat's selected plan includes this section; accumulation 2: it recurs on loaded runs until fixed"
- Tier: 2
- Intent: Scenario census-lifecycle in section supervision-and-census-fixtures (scripts/agents/supervision-fixtures.sh, the three-class inventory leg near line 3030) failed once on m1b's proof of tree f0f592bf (trunk 8d28185c, 2026-09-15 about 14:25 CEST, proof run artifacts/agents/proof-runs/proof-mu2n0ivo-7f8b96c629c669c7 in the m1b checkout) with 'supervision fixture scenario census-lifecycle failed while serving leg census-lifecycle with status 1'. The same section passed eight minutes later on a control of the same candidate (tree 02dc6c44, trunk 603532f8). The candidate changed only dispatch scripts, which this leg never runs. The output tail held only census inventory JSON, so the report does not say which check failed. DONE: the failing check identified from preserved evidence and code; the leg made independent of timing and load with an injectable clock or a recorded signal, never a longer wait or a retry; its failure names what it expected and what it observed; proven by the scenario passing under a loaded sweep and the section green on trunk apart from reds other goals own.
- Origin: human
- Next step: Diagnose first: preserve the m1b proof run's supervision-and-census-fixtures.log, find which census-lifecycle check failed (the report's output tail cut it off), and decide whether the leg or the census is at fault.
- OpenedAt: 2026-09-15T12:40:29Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-15T12:40:29Z 8X6HAV0YCYRDHNPWGFYERFHEJD-m1e-c6925449 open actor=human:Wido targets=census-lifecycle-scenario-holds-under-load
Integrity: sha256=b847ab7bbb5f33f2975dc84c42cf57b375b69c5ed8bdb4a030399406689e1ad9
