# a-capacity-refusal-is-an-environment-stop

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="classification of one failure kind on the unit step path every build uses; a wrong class costs one retry, never a landing"
- Tier: 1
- Intent: A build step that ends on a provider capacity refusal ("Selected model is at capacity", or the provider's equivalent words) is classified environment, not unclassified: the engine's one machine retry (decision 37, retryFailedStep in unit_step_failure.go:89) runs the round again after a counted wait, and only a second refusal stops the unit and asks the person, naming the provider's words. Evidence: lane-steward U6a round 2, 2026-10-26 07:35, cost a person's act for zero changed lines.
- Origin: main
- Next step: Steward cuts one brief (one unit, about 40 production lines): the step classifier reads the builder's exec log for the provider's capacity words, the retry waits a counted interval, the ask names the words; tests: a capacity refusal retries once, a second refusal asks, a real build failure still stops own. One Astra round, machine route on ui after the queue ahead of it, lane landing.
- OpenedAt: 2026-10-26T07:13:20Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-10-26T07:13:20Z 8M336VG5R8ESS42SM586A929NW-m1e-acb921fd open actor=human:Wido targets=a-capacity-refusal-is-an-environment-stop
Integrity: sha256=402925891c543bfd7ca26ceef2c5735e3e228c6738fd285d0ef3de7d8a20e5c2
