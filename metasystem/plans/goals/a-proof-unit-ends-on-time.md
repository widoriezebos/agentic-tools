# a-proof-unit-ends-on-time

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Lane test finding 374 (2026-10-17): the full proof of main hung 74 minutes on one deploy test whose wait had no producer check and no deadline under the lane's -timeout 0; the lane only reported drift; the steward ended it by hand. Findings 375, 379, 380 record the test's hold race, the unrecordable repeat of a dead unit and the proof folder's growth. Astra design rounds 1-5 (R-35-m3: no elapsed-time failure)."
- Tier: 3
- Intent: A proof unit never hangs the lane: a test wait fails when the thing it waits on is observed gone (never because time passed), and the lane names a unit that has finished no test for longer than its history, so a person can act.
- Origin: human
- Next step: U1 (D1): testenv AwaitFrom + ProcessGroupProducer adopted by the deploy bed's awaitHeld; brief FINAL by the stop rule; Codex build in -unitbound, Opus read, gate, hand landing. U2 (D2): the lane's quiet diagnostics after the-lane-says-what-it-does U1a.
- OpenedAt: 2026-10-17T05:48:03Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-17T05:48:03Z BSASQ4NEZB9R09BCZH7FR6Q3CN-m1e-c5a1e456 open actor=human:Wido targets=a-proof-unit-ends-on-time
Integrity: sha256=ea814d91f8fec48f53aeb3308795d323befd55054c841617e9d5c51865829d59
