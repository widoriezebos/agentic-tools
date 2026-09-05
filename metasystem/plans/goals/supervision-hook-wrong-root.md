# supervision-hook-wrong-root

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: the hook fails closed - it refuses arming and reports a bootstrap world rather than granting anything wrongly - but while it does, supervision is absent and every health line the seat sees is false; novelty 1: the fix passes an already-computed installation root to an existing flag, and the remaining sweep is the same substitution at the other consumers; exposure 3: every machine whose installation sits below the Git root, which is the fleet's standard layout; accumulation 2: one root cause recurring across machines and subsystems - m3 hook-freshness dead since enrollment, m0b's misdirected wrapper-root writes on 2026-09-02, m1's refused arming and split evidence trail on 2026-09-05"
- Tier: 3
- Intent: The harness supervision hook resolves the wrong repository on nested checkouts: run from metasystem/ inside the agentic-tools-m3 clone it derives the git toplevel (the outer repo) as its metasystem root, reports a bootstrap world (no ledger, no steward), and its turn evidence never lands where health's hook-freshness role reads - m3 has hook-freshness=dead since enrollment with the hook firing every turn. DONE means the hook resolves the metasystem project root deterministically on nested checkouts, its turn evidence lands, and hook-freshness goes alive, proven by a fixture running the hook from a nested layout
- Origin: main
- Next step: Revision 5 of plans/supervision-hook-root-design.md folds all four round-4 findings by id (register records/misc/hook-root-critique-r4.md, landed 12ed490c3); chain hook-root-fold5-20260905. Closing critique: dispatch as a follow-up on critic root hook-root-critique4b-20260905 (round 2 of that root), brief must state the landed rev-5 commit and sha256. TWO ITEMS the fold reported and did not fold, for the closing critic and the build to disposition: (a) a traced consequence written into Decision 4's existing-fixture blast - the fixture file's runtime-membership firing (supervision-hook-fixtures.sh ~128-130) and its two deadline firings (~416-418, ~429-431) change behaviour under the new resolver; strike or keep by reading; (b) the shipped hook gained a shell-side template-marker state-root block (HEAD ~275-288) and a missing-template fixture (~444-462) after the commit the design traced, and no finding names them; under the design the resolved repository already is the state root, so the block is either redundant or a second authority. BUDGET: 5/10 attempts and 600/720 minutes after the fold; closing critique + build + code review = 360 more against 120 left. 720 is the tier norm; on exhaustion this goal PARKS with this note unless Wido gives an over-norm raise (1200 minutes covers it). Then: build from plans/hook-root-build-brief.md (re-aim it at revision 5), code review, close, land.
- OpenedAt: 2026-09-01T07:25:56Z
- Revision: 18
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:human:Wido at=2026-09-05T21:14:30Z revision=15 opid=1A2K8PZJJMPA59QZ158EGW2A08-m1-a4f8999f authority=relayed digest=a779a36a4824fc5c8d0de0ae5f2cce1a32224907b1bb4e5dc8475a8286d6aeca reviewBy=2026-09-06
- Sliced: machine=m0b lineage=main-1788250419-3170380-8a1fb3 revision=6 at=2026-09-01T22:28:54Z
- Claimed: machine=m1 lineage=main-1788594343-3833-fb64b9 at=2026-09-05T21:18:14Z revision=16 accountingRevision=16
- StopCapability: generation=16 revision=16 machine=m1 claimEpoch=5 fenceEpoch=0

History:
- 2026-09-01T07:25:56Z HJPEPF3NATCRT1F2FE5080H6S6-m3-a5da21ff open actor=m3+mac-m3 targets=supervision-hook-wrong-root
- 2026-09-01T08:36:18Z M4Y1ZAC9GBG995JWNQZX6MFE6Z-m3-a5da21ff edit actor=m3+mac-m3 targets=supervision-hook-wrong-root
- 2026-09-01T08:37:13Z DPEJA3AF5F4Y3TSBB5H4JWMPRE-m2-bc1be9cb edit actor=m2+mac-coordinator targets=supervision-hook-wrong-root
- 2026-09-01T20:25:08Z S5WF9QRVKT4BYY75R5KTYJQCT2-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
- 2026-09-01T20:27:36Z R4E2ZEDCD08WGMAMXPT868345Q-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
- 2026-09-01T22:27:35Z P90TZBA3Z73HR3ZBG5NV1QXBYY-m0b-6638932d claim actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
- 2026-09-01T22:28:54Z CB1V3T4WTDEN0P0KBMQKR4WHMV-m0b-6638932d slice-start actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
- 2026-09-01T23:34:07Z 4080GD1YPFG2RJ5Z4ZYD9WX56Z-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
- 2026-09-01T23:35:27Z CDJQKRR80476C6GJPVDJNRH7CP-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
- 2026-09-01T23:35:31Z GTZ8BA0T05NB9DV9KWXY7EJSZH-m0b-6638932d release actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
- 2026-09-02T06:53:09Z 504Q8HYVBQSC634T92TB9CZEMJ-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=supervision-hook-wrong-root
- 2026-09-05T10:02:47Z 3N1CNSERR0KTC4XW0G7TNWT6X3-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=supervision-hook-wrong-root
- 2026-09-05T10:08:49Z 15WFP7Q3D9XB05KQXH1TA05CPK-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=supervision-hook-wrong-root
- 2026-09-05T10:10:04Z RY1WK638DNQG5J5HQ3KWV99W6C-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=supervision-hook-wrong-root
- 2026-09-05T21:14:30Z 1A2K8PZJJMPA59QZ158EGW2A08-m1-a4f8999f approve actor=human:human:Wido targets=supervision-hook-wrong-root authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="I approve supervision-hook-wrong-root"
- 2026-09-05T21:18:14Z YDA3N2EY8BMAHPMJEPWWDXDH5Y-m1-a4f8999f claim actor=m1+main-1788594343-3833-fb64b9 targets=supervision-hook-wrong-root
- 2026-09-05T21:34:05Z NF86SD9JCY1ZRYKX6RTK1FGMWM-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=supervision-hook-wrong-root
- 2026-09-05T22:04:27Z QVV7R7CWX3AJTHMT7ST4KK593B-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=supervision-hook-wrong-root
Integrity: sha256=f9e653086833968ef27496555ec0e9bc69c62d6bdf5a6f2be498f520f72eb682
