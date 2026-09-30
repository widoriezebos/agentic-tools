# correction-cost-machinery

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Severity 3: admission and landing authority; a wrong widening lets an unproven seat land. Novelty 2: new grants and a retry envelope on existing owners. Exposure 3: every goal on every machine. Accumulation 2: admission, budget and the landing route."
- Tier: 3
- Intent: What: A second seat on the same machine may run the proof (the full check run) for a goal another seat has claimed, when the claim matches or the person has recorded that this is allowed. The other two parts of the old goal are finished or obsolete: retries inside an approved budget were solved by another goal, and the old landing route was replaced by the batch lane. Why: On 2026-09-09 a seat had fixed five defects but was not allowed to run the proof, so the only options were the claiming seat or the person at the terminal. Pros: A small correction costs one proof run instead of a human visit. Cons: Loosening who may run a proof must not weaken the claim and identity checks, so the admission rule has to be exact.
- Origin: main
- Next step: Next: Design and build the admission rule in the proof command: admit a seat on the same machine whose goal claim matches, or one the person has marked as allowed, and keep refusing everyone else. Done when: A test shows a same-machine second seat with a matching claim can start the proof, while a seat on another machine, or one without a matching claim, is still refused.
- OpenedAt: 2026-09-09T20:15:56Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-09T20:15:56Z HCBXY60AEQVBHTNPZJHP1RVJV9-m1c-8d678ae8 open actor=m1c+main-1788963308-60248-b019cb targets=correction-cost-machinery
- 2026-09-11T22:07:54Z XTJV3NY2DVZB41FF0Z6KD5JNXQ-m1-c6925449 edit actor=human:Wido targets=correction-cost-machinery
- 2026-09-30T18:46:53Z JGE07AMVQXG4DTCX4E7R4WXT1Z-ui-bc2fda53 edit actor=human:Wido targets=correction-cost-machinery
Integrity: sha256=96b6df5c864e429c5a70c4b7f3a1e80c281852be510e022fb6c4396ab99a992b
