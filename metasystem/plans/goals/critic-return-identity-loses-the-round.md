# critic-return-identity-loses-the-round

- State: approved
- Priority: 3
- Sequence: 40
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="severity 1: nothing breaks, a review round's accounting and its register are lost while the findings survive in return.json; novelty 1: the validator and the return schema exist; exposure 2: every critic dispatch; accumulation 2: each lost round costs a seat a manual register and a re-dispatch"
- Tier: 2
- Intent: Sighting 2026-09-06 (m1, job stopverb-crit1-20260906): a Sol design critic returned eleven complete material findings but wrote jobId as a descriptive name ('metasystem-stop-verb-design-critique-round-1') instead of the job id; validation recorded protocol_error, the job failed, criticRoundsConsumed stayed 0 and no register was written, so the coordinator carried the findings by hand. DONE means: the return's identity is never something the critic types - the adapter or the return contract stamps the job id (or the validator accepts a missing/descriptive jobId when the session and turn identity already prove the return belongs to the job), a failed-validation return with findings is still projected into the critique register with a protocol note, and a fixture drives a return with a wrong jobId through the path.
- Origin: main
- Next step: Read internal/dispatch return validation for the jobId identity rule and the v4 critic schema; decide stamp-vs-accept; small build with the fixture; land.
- OpenedAt: 2026-09-06T19:48:27Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T21:08:31Z revision=2 opid=0KQJ8DGZWD471Z6F441NFFA97T-m1-7cd0bd60 authority=proven digest=0c4fb79680330fa4fba850947aeb19304870afa6a2d3b4e1a98ffae8101463c9

History:
- 2026-09-06T19:48:27Z WRCW8Z8SG6WE2BPNQ38BPDT2B0-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=critic-return-identity-loses-the-round
- 2026-09-06T21:08:31Z 0KQJ8DGZWD471Z6F441NFFA97T-m1-7cd0bd60 approve actor=human:Wido targets=critic-return-identity-loses-the-round
- 2026-09-08T16:01:28Z JVC1WSBJT0BCTEJEQVTZXV0D9E-m1-7cd0bd60 set-priority actor=human:Wido targets=critic-return-identity-loses-the-round reason=priority-order subject=critic-return-identity-loses-the-round from=unranked to=3:40 requested-sequence=40
Integrity: sha256=886f717b445e3edab7fb23fde152f8660b7381593c6f0f99e296821271fd746a
