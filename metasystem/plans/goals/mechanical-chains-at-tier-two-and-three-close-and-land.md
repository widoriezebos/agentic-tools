# mechanical-chains-at-tier-two-and-three-close-and-land

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a critic dispatched at the wrong class is refused at close by name and re-dispatched, nothing lands wrongly; novelty 1: the effective-obligations resolver and the landing lanes exist; exposure 2: every critic dispatch and every landing of a MECHANICAL chain on a tier-2 or tier-3 goal; accumulation 1: one resolver and one landing precondition"
- Tier: 1
- Intent: Since critique-always (894a269e) a MECHANICAL chain on a tier-2 or tier-3 goal needs an independent critique, and the critic proves the critique's effort only through its own class's builder rows, so the dispatcher must choose --destructive-reach DESIGN-BEARING for that critic or the close refuses it for effort; and such a chain has no landing lane: land.sh --chain refuses chain-not-design-bearing (internal/landing/observe.go) and the tier-one lane is gated on tier 1. DONE means: (1) a critic role dispatched to review a chain whose effective obligations require a critique carries the critique's rows as its builder rows by construction, whatever --destructive-reach names, and a test proves a MECHANICAL critic on a tier-3 chain closes; (2) a closed MECHANICAL chain on a tier-2 or tier-3 goal lands through land.sh --chain on its critic's read, proven by a land-fixtures leg, or the refusal names the lane to take. Origin: the Opus read of the critique-always build (records/misc/critique-always-build-critique-r1.md, F-2 and N-11).
- Origin: human
- Next step: Small build, one Opus read: (1) in internal/dispatch, EffectiveObligations or the compose path raises a critic role's builder rows to the critique rows when the reviewed chain's tier requires a critique; (2) internal/landing/observe.go admits a MECHANICAL chain at tier 2 or 3 to the --chain lane when its critique closed, with a land-fixtures leg. Tierless until Wido ranks it; queued. || Umbrella chain-closure-follows-the-tier (2026-09-13 backlog consolidation): this goal leads the fix for the members below. || Absorbed 2026-09-13 from goal merge-stage-critic-close: Merge stage binds to the root's closing critic, ignoring earlier critic chains. First half landed 8ed97738 (hazard.go criticOnly closes a critic-only chain); conformance.go:1096 still scans critic chains for chainClosed with no independentCritiqueJobRef binding (read today); four closing chains refused on 2026-09-03 wh
- OpenedAt: 2026-09-12T22:50:52Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-12T22:50:52Z QCV0G24BJ1EC54FV7FJ134N15R-m1e-c6925449 open actor=human:Wido targets=mechanical-chains-at-tier-two-and-three-close-and-land
- 2026-09-13T08:18:04Z 9S5ZKQTWR5PTT0FM1EF71NB9Q2-m1-c6925449 edit actor=human:Wido targets=mechanical-chains-at-tier-two-and-three-close-and-land
Integrity: sha256=df8ab99ab9a0fdf7e561d760fe0463f1a5bc5cb27bcfe5d64fa298a0d0e357cc
