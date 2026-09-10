# live-proof-needs-a-dispatchable-verifier

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: a DESTRUCTIVE-REACH chain cannot close on the shipped roster, so every such landing needs a seat-local roster edit first; novelty 1: one roster line or one main-run verifier path; exposure 3: every adopting checkout at that hazard class; accumulation 2: each seat rediscovers it at its first destructive-reach landing"
- Tier: 3
- Intent: Chain closure at DESTRUCTIVE-REACH requires a live-proof evidence reference: a completed job with role verifier that reviews the final work round (internal/dispatch/hazard.go, validateLiveProofReference). The shipped roster assigns role.verifier.runtime=main, and the delegate door refuses a main role (role verifier is assigned to main and cannot be dispatched); no other path creates a verifier job record. So on the shipped configuration no DESTRUCTIVE-REACH chain can ever close. Found by m1d on 2026-09-10 closing chain bsws-build1b-20260909 (refusal REFUSED-R22-M1-RULING-O-LIVE-PROOF), worked around by a seat-local roster line role.verifier.runtime=claude with model claude-opus-5, recorded under R-90-m1d. DONE means either the shipped roster assigns the verifier to a dispatchable lane with the reason written beside it, or a main-run verifier path exists that records the coordinator's own live drive as a verifier job with the same evidence shape; and a fixture proves a destructive-reach chain closes on the shipped configuration.
- Origin: main
- Next step: Appetite: 2h. Decide which of the two shapes in DONE; the roster line is the smaller change, the main-run path is the more honest one for seats whose coordinator does drive the surface. Fixture: the dispatch bed closes a destructive-reach chain end to end on the shipped metasystem.conf.
- OpenedAt: 2026-09-10T10:43:49Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T10:43:49Z SGDGTM7VP5TNF48MVFFZB91QDS-m1-c6925449 open actor=human:Wido targets=live-proof-needs-a-dispatchable-verifier
Integrity: sha256=7895a65cdf46a0bea17e60aac188f3e1fbe46ffb52c8b19c7856863388e10a4e
