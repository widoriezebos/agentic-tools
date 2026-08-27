# appetite-checkpoints

- State: done
- Intent: Appetite becomes a structured, machine-checked line instead of prose: the ledger carries the token, elapsed effort is tracked against it, and crossing it forces a recorded stop-decision (slice / annotate-release / extend-with-reason) instead of relying on the claimant noticing (incident: suite-dispatch-exclusion ~4h appetite, ~1 day spent, no checkpoint ever fired)
- Origin: main
- Next step: Appetite: 3h, single slice. RULING R-13 (Wido 2026-08-27) is the design's escalation law: crossing appetite ALWAYS escalates to Wido for his decision — continue with adjusted appetite, or abandon; pending his response work continues only while estimated overrun stays within a configured grace band of the original appetite (appetite.overrun-grace-percent, default 25, in metasystem.conf — a repo knob, never hard-coded); past the band, stop and wait. NOTE the goal-list APPETITE BREACH banner already exists — this item's work is carrying appetite as a structured field and surfacing the breach at the WORKING surfaces (claim, dispatch, commit.sh preflight, steward tick when it lands) where it raises to Wido per R-13, pairing self-reported estimates with measured elapsed-past-appetite so the band cannot be argued with. The local slice/annotate/release option remains available only BELOW the line. Feeds actionable-metrics (same fields) and continuous-self-improvement (appetite breaches as draft-queue trigger). Incident-derived, single-slice: direct to backlog per the draft-first small-item lane.
- Concluded: Landed ea51eae: R-13 mechanical — appetite-at-claim stamped in the ledger, cumulative meter across re-claims, price frozen after first breach (human-actor edits only), fail-closed BREACH-STOP dispatch refusal, banners at claim/dispatch/commit/watch/turn-verdict, grace band in conf. Certified 1 round, 3 findings fixed and suite-proven. Journey chapter this landing.
- OpenedAt: 2026-08-27T05:43:56Z
- Revision: 5

History:
- 2026-08-27T05:43:56Z 1BYZ72DH3RP0NM3DSWJQZZ0RGZ-m2-bc1be9cb open actor=m2+mac-coordinator targets=appetite-checkpoints
- 2026-08-27T05:49:38Z 9KKG8A1DHXZPP6NY3B0Z54M8PF-m2-bc1be9cb edit actor=m2+mac-coordinator targets=appetite-checkpoints
- 2026-08-27T05:53:30Z N2RVN58AWBGT21D0J2RK0FFRZB-m2-bc1be9cb edit actor=m2+mac-coordinator targets=appetite-checkpoints
- 2026-08-27T11:09:03Z ETJB2KTYPAQW9ENBHDVBBVCV9H-m2-bc1be9cb claim actor=m2+mac-coordinator targets=appetite-checkpoints
- 2026-08-27T12:28:07Z SCWNEXHQ3CA4M2WMHM4BNCHQPD-m2-bc1be9cb done actor=m2+mac-coordinator targets=appetite-checkpoints
Integrity: sha256=97a0dacdf5c35f568a53c974fb5498488bef9ab08131dfa68486d8797390ef2a
