# hook-root-installation-fix

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: fails closed today - refuses arming and reports a bootstrap world rather than granting anything - but while it does, supervision is absent and every health line the seat sees is false; novelty 1: two lines pass an already-computed installation root to an existing flag, plus one fixture; exposure 3: every machine whose installation sits below the Git root, the fleet's standard layout; accumulation 2: one root cause recurring across m3, m0b and m1 over five days"
- Tier: 3
- Intent: The supervision hook reads its runtime adapters and writes its evidence trail from the installation it belongs to, never from the Git toplevel. Where the installation sits below the toplevel, find-ancestor was handed a root with no scripts/agents/adapters, identity came back empty, every session start refused arming with 'could not identify the immediate claude agent process', and the evidence trail split into the wrapper repository so hook-freshness read dead against the installation while the steward stayed blind to turn-end supervision. DONE means proc find-ancestor and the evidence trail use the installation root, a nested-layout fixture runs the real ancestor walk under a real signature-matching parent and fails without the change, and hook-freshness reads alive on a nested checkout. Successor A of supervision-hook-wrong-root, decomposed on Wido's word 2026-09-06
- Origin: main
- Next step: Build through a chain from the verified reference on branch m1-2026-09-05-verified (two hook lines: find-ancestor --repo and supervision_dir take $harness_root; plus the nested-installation scenario in supervision-hook-fixtures.sh - the template scenario beside it stubs find-ancestor, which is why the suite never saw this); code review; land. Prefer the tier-1 lane if the diff stays within three files and forty lines. Needs approval at the enrolled terminal
- OpenedAt: 2026-09-06T06:39:46Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-06T06:39:46Z JZMAPNGRTSSQ63G2N2ABE591AM-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=hook-root-installation-fix
Integrity: sha256=1d94a0f4beaca12462e1d751a4d178ddadb1fcf253387d155326a548bee81fec
