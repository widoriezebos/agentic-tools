# appetite-checkpoints

- State: queued
- Intent: Appetite becomes a structured, machine-checked line instead of prose: the ledger carries the token, elapsed effort is tracked against it, and crossing it forces a recorded stop-decision (slice / annotate-release / extend-with-reason) instead of relying on the claimant noticing (incident: suite-dispatch-exclusion ~4h appetite, ~1 day spent, no checkpoint ever fired)
- Origin: main
- Next step: Appetite: 3h, single slice. RULING R-12 (Wido 2026-08-27) is the design's escalation law: crossing appetite ALWAYS escalates to Wido for his decision — continue with adjusted appetite, or abandon; pending his response work continues only while estimated overrun stays within a configured grace band of the original appetite (appetite.overrun-grace-percent, default 25, in metasystem.conf — a repo knob, never hard-coded); past the band, stop and wait. Build: an Appetite field on goal records (open/edit/claim carry it; prose appetites migrate on touch); claim stamps claimedAt; the breach check at the coordinator's working surfaces (goal show, commit.sh preflight, steward tick when it lands) compares elapsed-since-claim to the token, and past the line RAISES TO WIDO (stop message now; steward incident when steward lands) while printing the grace-band state so the continue/stop default is mechanical. The local slice/annotate/release option remains available to the claimant only BELOW the line; at the line the decision is Wido's. Feeds actionable-metrics (same fields) and continuous-self-improvement (appetite breaches as draft-queue trigger). Incident-derived, single-slice: direct to backlog per the draft-first small-item lane.
- OpenedAt: 2026-08-27T05:43:56Z
- Revision: 2

History:
- 2026-08-27T05:43:56Z 1BYZ72DH3RP0NM3DSWJQZZ0RGZ-m2-bc1be9cb open actor=m2+mac-coordinator targets=appetite-checkpoints
- 2026-08-27T05:49:38Z 9KKG8A1DHXZPP6NY3B0Z54M8PF-m2-bc1be9cb edit actor=m2+mac-coordinator targets=appetite-checkpoints
Integrity: sha256=510bd0b378bd9d42c2c074f03c71b48a02007574045967d9135aaed1d35ee19b
