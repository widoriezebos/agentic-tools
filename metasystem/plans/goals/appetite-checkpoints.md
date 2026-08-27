# appetite-checkpoints

- State: queued
- Intent: Appetite becomes a structured, machine-checked line instead of prose: the ledger carries the token, elapsed effort is tracked against it, and crossing it forces a recorded stop-decision (slice / annotate-release / extend-with-reason) instead of relying on the claimant noticing (incident: suite-dispatch-exclusion ~4h appetite, ~1 day spent, no checkpoint ever fired)
- Origin: main
- Next step: Appetite: 3h, single slice. RULING R-13 (Wido 2026-08-27) is the design's escalation law: crossing appetite ALWAYS escalates to Wido for his decision — continue with adjusted appetite, or abandon; pending his response work continues only while estimated overrun stays within a configured grace band of the original appetite (appetite.overrun-grace-percent, default 25, in metasystem.conf — a repo knob, never hard-coded); past the band, stop and wait. NOTE the goal-list APPETITE BREACH banner already exists — this item's work is carrying appetite as a structured field and surfacing the breach at the WORKING surfaces (claim, dispatch, commit.sh preflight, steward tick when it lands) where it raises to Wido per R-13, pairing self-reported estimates with measured elapsed-past-appetite so the band cannot be argued with. The local slice/annotate/release option remains available only BELOW the line. Feeds actionable-metrics (same fields) and continuous-self-improvement (appetite breaches as draft-queue trigger). Incident-derived, single-slice: direct to backlog per the draft-first small-item lane.
- OpenedAt: 2026-08-27T05:43:56Z
- Revision: 3

History:
- 2026-08-27T05:43:56Z 1BYZ72DH3RP0NM3DSWJQZZ0RGZ-m2-bc1be9cb open actor=m2+mac-coordinator targets=appetite-checkpoints
- 2026-08-27T05:49:38Z 9KKG8A1DHXZPP6NY3B0Z54M8PF-m2-bc1be9cb edit actor=m2+mac-coordinator targets=appetite-checkpoints
- 2026-08-27T05:53:30Z N2RVN58AWBGT21D0J2RK0FFRZB-m2-bc1be9cb edit actor=m2+mac-coordinator targets=appetite-checkpoints
Integrity: sha256=2a1a3212b2007f396459db631560578cc8689618a89c97f665797535e1ab29dc
