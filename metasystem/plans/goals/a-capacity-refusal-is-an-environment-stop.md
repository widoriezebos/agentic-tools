# a-capacity-refusal-is-an-environment-stop

- State: approved
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="classification of one failure kind on the unit step path every build uses; a wrong class costs one retry, never a landing"
- Tier: 1
- Intent: A build step that ends on a provider capacity refusal ("Selected model is at capacity", or the provider's equivalent words) is classified environment, not unclassified: the engine's one machine retry (decision 37, retryFailedStep in unit_step_failure.go:89) runs the round again after a counted wait, and only a second refusal stops the unit and asks the person, naming the provider's words. Evidence: lane-steward U6a round 2, 2026-10-26 07:35, cost a person's act for zero changed lines.
- Origin: main
- Next step: Steward cuts one brief (one unit, about 40 production lines): the step classifier reads the builder's exec log for the provider's capacity words, the retry waits a counted interval, the ask names the words; tests: a capacity refusal retries once, a second refusal asks, a real build failure still stops own. One Astra round, machine route on ui after the queue ahead of it, lane landing.
- OpenedAt: 2026-10-26T07:13:20Z
- Revision: 2
- Budget: elapsedLimit=5d attemptLimit=12 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=SBX1SDWT81JEWAVEMWM2973Q53-m1e-acb921fd minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-10-26T07:14:07Z revision=2 opid=SBX1SDWT81JEWAVEMWM2973Q53-m1e-acb921fd authority=proven digest=4e21d1be6b23a5d4ee53272711f67fa02969bfde45962f4c9e8e1830f74bce5e episode=2

History:
- 2026-10-26T07:13:20Z 8M336VG5R8ESS42SM586A929NW-m1e-acb921fd open actor=human:Wido targets=a-capacity-refusal-is-an-environment-stop
- 2026-10-26T07:14:07Z SBX1SDWT81JEWAVEMWM2973Q53-m1e-acb921fd approve actor=human:Wido targets=a-capacity-refusal-is-an-environment-stop,the-impact-check-selects-only-tests-this-host-can-run,the-ui-bundle-builds-again
Integrity: sha256=285a93c914ce1353c03f43f1b8173aa66c3017fab5bfb9ff96e4fec41c3aaa50
