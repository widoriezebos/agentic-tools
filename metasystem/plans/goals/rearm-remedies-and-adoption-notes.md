# rearm-remedies-and-adoption-notes

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: wording and two adoption edge cases, nothing unsafe; novelty 1: small edits in files that just landed; exposure 2: operators reading a refusal, and any re-adoption; accumulation 1: nothing compounds"
- Tier: 2
- Intent: Four notes the third code critique (err-review3-20260906) left on the engine re-arm landing ea8c3ead7, none material: adoption refuses a detached target outright where it should continue and report that automatic re-arm stays off; adoption overwrites an existing metasystem.steward.landing-ref where the human verbs preserve one; the not-landed refusal remedy names only the terminal though a merely stale remote-tracking ref is repaired by one fetch of the configured remote, so the remedy names both; and a machine re-arm that fails before any stage transition with a non-drift error (missing notification channel, missing runner directory, lock error) gets the remedy 'repair the named enrollment publication failure' regardless of cause - the remedy names the actual error class.
- Origin: main
- Next step: MECHANICAL, tier-1 lane or one small chain: scripts/adopt.sh (detached target continues and reports; existing key preserved), internal/steward/rearm_resolver.go and internal/up/up.go (remedy texts), with a test per remedy. FIFTH ITEM, from the field: on m1b and m1d the human steward verb that seeded metasystem.steward.landing-ref (run by Wido in an agent-free terminal at 11:20Z and 11:23Z, with the new engine at the enrolled path) was followed at once by a generation minted as mintedBy=machine-rebuild with humanWitnessedGeneration absent (arming.log: 'engine-re-armed generation=2 previous=1 engine=999e2c07 landed=999e2c0 ref=refs/remotes/origin/main'). The bytes were landed, so nothing unsafe re-armed, but a human's own arm or restart must leave a human-witnessed generation, not a machine-labelled one: decide whether the human verbs mint before the machine rule runs, or record the human act on the machine-minted generation; a fixture proves a human restart on a legacy identity yields a witnessed generation. Receipt: the cmd, steward and up packages; land-fixtures for adoption; the bed's rearm scenarios once bed-child-death-reported-as-pass lands. Any free seat.
- OpenedAt: 2026-09-06T11:17:34Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T11:17:34Z QPGB1R343FQPHD5FHKC2G58XTB-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=rearm-remedies-and-adoption-notes
- 2026-09-06T11:42:28Z BTY2B0FT8G2RNNPTVS2A2C1NW7-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=rearm-remedies-and-adoption-notes
Integrity: sha256=71f7c5eeaa38296d319c089b9d17f9313239c4b1f3b82bcfa02283ea5e1e343d
