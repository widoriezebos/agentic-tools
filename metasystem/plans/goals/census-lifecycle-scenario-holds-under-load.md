# census-lifecycle-scenario-holds-under-load

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: a false red costs each seat a classification per proof and may hide a real census defect; novelty 1: one scenario; exposure 3: every seat's selected plan includes this section; accumulation 2: it recurs on loaded runs until fixed"
- Tier: 2
- Intent: Scenario census-lifecycle in section supervision-and-census-fixtures (scripts/agents/supervision-fixtures.sh, the three-class inventory leg near line 3030) failed once on m1b's proof of tree f0f592bf (trunk 8d28185c, 2026-09-15 about 14:25 CEST, proof run artifacts/agents/proof-runs/proof-mu2n0ivo-7f8b96c629c669c7 in the m1b checkout) with 'supervision fixture scenario census-lifecycle failed while serving leg census-lifecycle with status 1'. The same section passed eight minutes later on a control of the same candidate (tree 02dc6c44, trunk 603532f8). The candidate changed only dispatch scripts, which this leg never runs. The output tail held only census inventory JSON, so the report does not say which check failed. DONE: the failing check identified from preserved evidence and code; the leg made independent of timing and load with an injectable clock or a recorded signal, never a longer wait or a retry; its failure names what it expected and what it observed; proven by the scenario passing under a loaded sweep and the section green on trunk apart from reds other goals own.
- Origin: human
- Next step: Diagnose first: the m1b proof's supervision-and-census-fixtures.log is preserved as artifacts/agents/suite-failures/census-lifecycle-red-20260915T1225Z-supervision-and-census-fixtures.log in the m1b checkout; find which census-lifecycle check failed (the report's output tail cut it off) and decide whether the leg or the census is at fault. Sibling sighting (m1c): nested-worktree-deadlines' leg nested-wt-parent-steered-timeout hit the 60 s provider deadline once on tree 58a55197 at 13:47 and passed on a rerun at 14:05 (memory/backlog-notes.md in fb6839a44). Both look like wall-time waits inside a bed, the class tests-never-wait-on-wall-time covers.
- OpenedAt: 2026-09-15T12:40:29Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-15T12:41:56Z revision=3 opid=22AGFWVMZRT2RJG4SVXEYBK8FJ-m1e-c6925449 authority=proven digest=4aa84a3197fe596d19bf766fc70a56487e2e9cc8dd8d11e55f2bc167175e2a0e

History:
- 2026-09-15T12:40:29Z 8X6HAV0YCYRDHNPWGFYERFHEJD-m1e-c6925449 open actor=human:Wido targets=census-lifecycle-scenario-holds-under-load
- 2026-09-15T12:41:50Z M13C7X9YG872MDZKRDM27MPTSF-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=census-lifecycle-scenario-holds-under-load
- 2026-09-15T12:41:56Z 22AGFWVMZRT2RJG4SVXEYBK8FJ-m1e-c6925449 approve actor=human:Wido targets=census-lifecycle-scenario-holds-under-load
Integrity: sha256=4f63e17dd66d5f27c4ee9a27dafc1c36c77dfc2406d1c5f952ef70a69f24619c
