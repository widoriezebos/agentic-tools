# gate-fence-fixtures-refuse-engine-changing-candidates

- State: done
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: no engine-changing candidate can land on any seat under the contract; novelty 2: the proof engine exists and the fast gate builds it, the receipt just does not hand it to the beds; exposure 3: every seat's every code landing; accumulation 2: four blocked chains on two seats already"
- Tier: 3
- Intent: Under the shared testing contract (1b12f534) the schema-2 receipt runs section/gate-fence-fixtures inside an isolated worktree of the CANDIDATE, but the fixtures dispatch with the checkout's ENROLLED engine, and dispatch's skew preflight refuses 'engine commit <enrolled> is older than checkout commit <candidate> and engine or agent scripts changed; run go-build.sh, then steward arm'. A candidate engine cannot be enrolled (ENROLLMENT_DRIFT), so every candidate that changes engine code or agent scripts fails that group and can never take a sufficient receipt. Seen 2026-09-10 on m1 (dispatch-cap-necessity, hp-terminal-grade-for-stopping-acts, both LAND-READY and blocked) and on m1b (review-round-limit-counts-per-chain, chain rrl-build1: 35 of 36 groups green, gate-fence refused, proof run proof-mtv5baou-9cb72e840909376d). DONE means the receipt's fixture beds run against an engine built from the candidate tree (the proof engine the fast gate already builds), the skew preflight inside a receipt worktree compares the candidate engine with the candidate checkout, and a fixture proves an engine-changing candidate takes a sufficient receipt.
- Origin: main
- Next step: SPLIT 2026-09-10 10:10Z at Wido's word (no large goals): the work is now two small members of the umbrella the-metasystem-validates-itself-with-itself: receipt-beds-run-the-candidate-engine (the receipt hands its proof engine to the beds and records both digests) and skew-preflight-knows-a-receipt-worktree (the preflight compares the engine that will run with the checkout it runs in). This record keeps the evidence (proof run proof-mtv5baou-9cb72e840909376d, group log section/gate-fence-fixtures.log) and closes when both members land; approve and rank the members, not this one.
- Concluded: Landed: The record closes when its two split members land: receipt-beds-run-the-candidate-engine and skew-preflight-knows-a-receipt-worktree are both concluded, landed 016b83e2 (2026-09-10, Build receipt section engines from the candidate tree); 92b31f77 binds the enrollment by the skew rule. Concluded 2026-09-11 in the backlog consolidation on Wido's word, no further work.
- OpenedAt: 2026-09-10T07:15:05Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T07:15:05Z 1YMQDFF2RPPY0SME9G7MTSXCHW-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=gate-fence-fixtures-refuse-engine-changing-candidates
- 2026-09-10T07:28:02Z VADVDHYJ27ETC8JVMW8MAAD7XP-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=gate-fence-fixtures-refuse-engine-changing-candidates
- 2026-09-11T22:02:54Z 7MA8XCQZ2XC7H8YTP36BDY1DYZ-m1-c6925449 done actor=human:Wido targets=gate-fence-fixtures-refuse-engine-changing-candidates
Integrity: sha256=2f311e8d5081cf92ca3aa6035e61d47eacb9acfbe3446c88477b274883136e49
