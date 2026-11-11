# unit-check-deadline-matches-current-runtime

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="A mistaken limit can stall or kill checks on every seat, but the change is one committed value and reversible; it is a proof input, so it lands through the lane."
- Tier: 2
- Intent: proof.deadline is 30 minutes so a green check whose cmd/metasystem shards print only at their end (752-958 s normally, longer under load) is not killed for inactivity (finding 598).
- Origin: main
- Next step: One unit: metasystem/metasystem.conf proof.deadline=15 -> 30. Consequences recorded: the inactivity limit doubles and the elapsed ceiling (eight times the limit) rises from two to four hours per command; frozen runs keep their old declaration. Inverse: 30 -> 15 once retained timings justify it. Heartbeat output deferred (an unconditional heartbeat would hide a hang until the elapsed ceiling).
- OpenedAt: 2026-11-11T10:38:21Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-11T10:38:21Z TWSJCS7RG8A9X4M19Z6WM9AETE-m1e-b89f1157 open actor=human:Wido targets=unit-check-deadline-matches-current-runtime
Integrity: sha256=8ec3e02df37d2e3e59069aa5a4e08646e45cac19b9a4041848694e2a4a421b80
