# critic-return-identity-loses-the-round

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="severity 1: nothing breaks, a review round's accounting and its register are lost while the findings survive in return.json; novelty 1: the validator and the return schema exist; exposure 2: every critic dispatch; accumulation 2: each lost round costs a seat a manual register and a re-dispatch"
- Tier: 2
- Intent: Sighting 2026-09-06 (m1, job stopverb-crit1-20260906): a Sol design critic returned eleven complete material findings but wrote jobId as a descriptive name ('metasystem-stop-verb-design-critique-round-1') instead of the job id; validation recorded protocol_error, the job failed, criticRoundsConsumed stayed 0 and no register was written, so the coordinator carried the findings by hand. DONE means: the return's identity is never something the critic types - the adapter or the return contract stamps the job id (or the validator accepts a missing/descriptive jobId when the session and turn identity already prove the return belongs to the job), a failed-validation return with findings is still projected into the critique register with a protocol note, and a fixture drives a return with a wrong jobId through the path.
- Origin: main
- Next step: Read internal/dispatch return validation for the jobId identity rule and the v4 critic schema; decide stamp-vs-accept; small build with the fixture; land.
- OpenedAt: 2026-09-06T19:48:27Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T19:48:27Z WRCW8Z8SG6WE2BPNQ38BPDT2B0-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=critic-return-identity-loses-the-round
Integrity: sha256=8ae35faa894a07a068cbfd8c82361beccc9368f671aa776e825cd77c9f4adb9a
