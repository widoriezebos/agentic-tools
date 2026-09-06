# hook-root-installation-fix

- State: done
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: fails closed today - refuses arming and reports a bootstrap world rather than granting anything - but while it does, supervision is absent and every health line the seat sees is false; novelty 1: two lines pass an already-computed installation root to an existing flag, plus one fixture; exposure 3: every machine whose installation sits below the Git root, the fleet's standard layout; accumulation 2: one root cause recurring across m3, m0b and m1 over five days"
- Tier: 3
- Intent: The supervision hook reads its runtime adapters and writes its evidence trail from the installation it belongs to, never from the Git toplevel. Where the installation sits below the toplevel, find-ancestor was handed a root with no scripts/agents/adapters, identity came back empty, every session start refused arming with 'could not identify the immediate claude agent process', and the evidence trail split into the wrapper repository so hook-freshness read dead against the installation while the steward stayed blind to turn-end supervision. DONE means proc find-ancestor and the evidence trail use the installation root, a nested-layout fixture runs the real ancestor walk under a real signature-matching parent and fails without the change, and hook-freshness reads alive on a nested checkout. Successor A of supervision-hook-wrong-root, decomposed on Wido's word 2026-09-06
- Origin: main
- Next step: Build through a chain from the verified reference on branch m1-2026-09-05-verified (two hook lines: find-ancestor --repo and supervision_dir take $harness_root; plus the nested-installation scenario in supervision-hook-fixtures.sh - the template scenario beside it stubs find-ancestor, which is why the suite never saw this); code review; land. Prefer the tier-1 lane if the diff stays within three files and forty lines. Needs approval at the enrolled terminal
- Concluded: The session-start refusal 'could not identify the immediate claude agent process' on nested checkouts is fixed at the source. NOT fixed here: hook-freshness reading dead against the installation (the steward component record is still written under the Git toplevel) and the vendored-operator layout's missing post-Stop assertion; both are recorded on hook-root-resolver-design, which owns the consumer sweep.
- OpenedAt: 2026-09-06T06:39:46Z
- Revision: 5
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:human:Wido at=2026-09-06T06:46:33Z revision=2 opid=D029HG6CHYMKBR1H95VR1YYJYB-m1-a4f8999f authority=proven digest=4ab8ba10266b58da1f528701ce3ad44516e5c44744feeba01d0d80ececf19767
- Sliced: machine=m1 lineage=main-1788594343-3833-fb64b9 revision=3 at=2026-09-06T06:48:12Z

History:
- 2026-09-06T06:39:46Z JZMAPNGRTSSQ63G2N2ABE591AM-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=hook-root-installation-fix
- 2026-09-06T06:46:33Z D029HG6CHYMKBR1H95VR1YYJYB-m1-a4f8999f approve actor=human:human:Wido targets=hook-root-installation-fix
- 2026-09-06T06:48:07Z GFBDA052GRVMWT2TNKPD14TT7F-m1-a4f8999f claim actor=m1+main-1788594343-3833-fb64b9 targets=hook-root-installation-fix
- 2026-09-06T06:48:12Z R1BNPC1SQETNZCFXF0BCJA1VYC-m1-a4f8999f slice-start actor=m1+main-1788594343-3833-fb64b9 targets=hook-root-installation-fix
- 2026-09-06T07:32:45Z MPECMZBXDPXYV3A9NM7FSWBBF5-m1-a4f8999f done actor=m1+main-1788594343-3833-fb64b9 targets=hook-root-installation-fix
Integrity: sha256=50677fe51948360849d7fe7a8ede4f822309b4e42f39ea3f409bd6a38cb230bf
