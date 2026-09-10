# gate-fence-fixtures-refuse-engine-changing-candidates

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: no engine-changing candidate can land on any seat under the contract; novelty 2: the proof engine exists and the fast gate builds it, the receipt just does not hand it to the beds; exposure 3: every seat's every code landing; accumulation 2: four blocked chains on two seats already"
- Tier: 3
- Intent: Under the shared testing contract (1b12f534) the schema-2 receipt runs section/gate-fence-fixtures inside an isolated worktree of the CANDIDATE, but the fixtures dispatch with the checkout's ENROLLED engine, and dispatch's skew preflight refuses 'engine commit <enrolled> is older than checkout commit <candidate> and engine or agent scripts changed; run go-build.sh, then steward arm'. A candidate engine cannot be enrolled (ENROLLMENT_DRIFT), so every candidate that changes engine code or agent scripts fails that group and can never take a sufficient receipt. Seen 2026-09-10 on m1 (dispatch-cap-necessity, hp-terminal-grade-for-stopping-acts, both LAND-READY and blocked) and on m1b (review-round-limit-counts-per-chain, chain rrl-build1: 35 of 36 groups green, gate-fence refused, proof run proof-mtv5baou-9cb72e840909376d). DONE means the receipt's fixture beds run against an engine built from the candidate tree (the proof engine the fast gate already builds), the skew preflight inside a receipt worktree compares the candidate engine with the candidate checkout, and a fixture proves an engine-changing candidate takes a sufficient receipt.
- Origin: main
- Next step: Opened by m1b 2026-09-10 09:15Z with the group log as evidence. Approve and rank beside testing-contract-owns-record-paths; the two together decide whether anything lands this week.
- OpenedAt: 2026-09-10T07:15:05Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T07:15:05Z 1YMQDFF2RPPY0SME9G7MTSXCHW-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=gate-fence-fixtures-refuse-engine-changing-candidates
Integrity: sha256=d7a9e6c95ede6b1e7727ff576701d994ba0d7b596410203d9291253b58c3685e
