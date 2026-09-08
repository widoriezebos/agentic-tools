# cross-repo-guardrails

- State: approved
- Priority: 3
- Sequence: 109
- Intent: The net can include guardrails that live outside the repository at a pinned commit — a corpora repo's frozen gold, a consumer's pin of its upstream — so an app's most valuable guardrails are enforced, not merely described (Wido 2026-08-25, morpheus/yoda fit)
- Origin: human
- Next step: Appetite: 1d design-first (custody arc discipline). The driving cases: morpheus's gold lives in a separate corpora repository at a pinned commit under a freeze contract; yoda consumes morpheus through pins needing deliberate refresh. Design questions the chain must settle: the reference grammar (repo identity + commit + paths within it), how the wall proves pinned bytes at mission time without network trust, pin MOVEMENT as a guardrail change through the warden's lane, the counselor's stale-pin signal (drift signal 3's cross-repo sibling), and what adoption/validation do when the pinned repo is absent on a machine. Out of scope: fetching/mirroring machinery beyond what proof requires. Queue: after metasystem-way-patterns.
- OpenedAt: 2026-08-25T06:41:14Z
- Revision: 4
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=62efaa95b24aa5365508da3985295d7d19b2789da3a2d6c0d22b8b239f0e7593

History:
- 2026-08-25T06:41:14Z T0EJERXXCQJN9VY3TJ59AY10CA-m1-bf243850 open actor=human:wido targets=cross-repo-guardrails
- 2026-09-01T20:28:19Z YJQYZR9TP5T27FDSRDFDVXJTVR-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=cross-repo-guardrails
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=cross-repo-guardrails reason=sweep
- 2026-09-08T16:05:31Z VDB4FAM6MS58PPVA7XA2791B4Z-m1-7cd0bd60 set-priority actor=human:Wido targets=cross-repo-guardrails reason=priority-order subject=cross-repo-guardrails from=unranked to=3:109 requested-sequence=109
Integrity: sha256=13f00a3246f1cb9d6715fdd34604e5af5aab8199e7bef7dcfa939a13ecf0d47f
