# conflicts-resolve-unattended

- State: approved
- Risk: severity=3 novelty=3 exposure=3 accumulation=2 basis="Decides how every seat's work reaches main when main moved; a wrong rule either stalls the fleet or lands unreviewed merges; it compounds across every future goal"
- Tier: 3
- Intent: When a goal's work conflicts with a newer main, the machinery resolves it without a person: in a seat with no landing lane, in the landing lane, and for a goal branch that must move onto a newer main (Wido 2026-10-03, on m1g's question 'resolve / replay / refuse': 'this is too simplistic. This needs a proper design and review round. The goal is unattended functionality'). Today: a goal branch cannot be brought onto a newer main (the branch reader refuses merge commits; work revise builds on the old base, so the same lines conflict again); a branch the lane returns for a conflict has no route back in (items 48, 57); the landing agent fixes small plain conflicts but its resolution is not reviewed, only proved. The design decides what is resolved by whom, how a resolution is reviewed and proved, when a conflict is genuinely the person's call (and then asked with its impact, R-143-m1e), and keeps one landing mechanism.
- Origin: main
- Next step: Tier-3 design: use cases from tonight's real conflicts (ui-shows-true-facts vs ui-reads-well-on-a-phone in ProjectPane.tsx and the bundle; fleet-panel-ux vs ui-reads-well; generated files such as testing.json and UI bundles), threat model, rabbit-hole risks with mitigations, options with pros and cons (resolve in place, replay onto main and re-review, regenerate generated files, refuse with paths); Astra critique tier 3; then build in slices.
- OpenedAt: 2026-10-03T07:50:15Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-03T07:50:25Z revision=2 opid=A92RG8C9M2R6RRWWS9X8XBW6Z5-m1e-718ba0eb authority=proven digest=d27063d82115b12fec0706a1cd9ddecdcba002bd493bc6f70f56c30f8d6606d3 episode=2

History:
- 2026-10-03T07:50:15Z VCKGVG2T8BVYFDGSX7F3AB9RN3-m1e-718ba0eb open actor=human:Wido targets=conflicts-resolve-unattended
- 2026-10-03T07:50:25Z A92RG8C9M2R6RRWWS9X8XBW6Z5-m1e-718ba0eb approve actor=human:Wido targets=conflicts-resolve-unattended
Integrity: sha256=9b329582f2d33caa38fd0318d4b7ede29848188ef8979df54f55ca3a42ae85e2
