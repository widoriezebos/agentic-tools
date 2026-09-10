# receipt-beds-run-the-candidate-engine

- State: claimed
- Priority: 1
- Sequence: 1
- Risk: severity=3 novelty=1 exposure=3 accumulation=1 basis="severity 3: the group refuses every engine-changing candidate; novelty 1: the proof engine and the METASYSTEM_BIN convention already exist; exposure 3: every code landing; accumulation 1: nothing built on it yet"
- Tier: 3
- Intent: Inside a schema-2 receipt (metasystem landing test-receipt --mode auto) the fixture-bed groups (section/gate-fence-fixtures, section/dispatch-fixtures, section/goal-cli-fixtures, section/land-fixtures and the rest) run in an isolated worktree of the candidate but with the checkout's ENROLLED bin/metasystem, so they test the old engine against the new scripts and, for engine-changing candidates, refuse at dispatch's skew preflight. DONE means: the receipt builds one proof engine from the candidate tree the way commit.sh's static re-proof does (go-gate.sh --fast --proof-out), every bed group in the receipt runs with METASYSTEM_BIN set to that engine, the receipt records both digests (enrolled engine, proof engine) and the candidate tree, and a fixture proves an engine-changing candidate's section/gate-fence-fixtures group passes in a receipt. Evidence: proof run proof-mtv5baou-9cb72e840909376d on m1b (35 of 36 groups green, gate-fence refused 'engine commit 2c1bc7db is older than checkout commit 2be2ed91...'); the same on m1's dispatch-cap-necessity. Replaces the first half of gate-fence-fixtures-refuse-engine-changing-candidates.
- Origin: main
- Next step: BUILDING (rbce-build1, Sol, started 09:32). COORDINATION with m1c 2026-09-10: m1c claimed gate-fence-returns-to-per-landing-proof at 06:54 with the same candidate-engine intent, saw this chain on the remote at 07:50 and yielded the ENGINE implementation to m1b; m1c does the cmd fixture repairs, the dispatch-fixtures METASYSTEM_BIN selection and the gate-fence runtime-custody declarations in testing.json, then integrates this delivery. m1c's design critique of the receipt engine (evidence receipt-engine-20260910/design-critic-r1-return.json) raised four material concerns that this chain's closing read must judge against the built code: frontend/worker identity mismatch at completion, precommit base-key invalidated by postcommit verification, unbound GOENV build inputs, build-cache coverage. Then: closing read and the receipt IN PARALLEL on the staged candidate (new practice), land, and m1c integrates.
- OpenedAt: 2026-09-10T07:27:25Z
- Revision: 7
- Labels: bootstrap
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-10T07:29:14Z revision=2 opid=4ZK6CRTM3WB0139F8R1W7B9NNZ-m1b-c6925449 authority=proven digest=7791b53db49be871db3b8d576863dbc78b9bd85bf95c16442cc4679e296e3910
- Sliced: machine=m1b lineage=main-1788680071-18713-e76d5d revision=4 at=2026-09-10T07:32:46Z
- Claimed: machine=m1b lineage=main-1788680071-18713-e76d5d at=2026-09-10T07:32:21Z revision=4 accountingRevision=4
- StopCapability: generation=4 revision=4 machine=m1b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-10T07:27:25Z J23YGSQ7VB7AEY7BPVQTRVV2CT-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=receipt-beds-run-the-candidate-engine
- 2026-09-10T07:29:14Z 4ZK6CRTM3WB0139F8R1W7B9NNZ-m1b-c6925449 approve actor=human:Wido targets=carried-landing-debt-and-cap,enrollment-binds-by-the-skew-rule-not-the-tip,human-carried-landing-verb,receipt-beds-run-the-candidate-engine,skew-preflight-knows-a-receipt-worktree,testing-contract-owns-record-paths,the-metasystem-validates-itself-with-itself
- 2026-09-10T07:29:23Z 4YQ8846PHRPC35JCF0ETCW674J-m1b-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,breach-stop-wedges-seat,burn-without-delivery-tripwire,delegate-job-liveness,dispatch-cap-necessity,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,governed-exhaustion-reprojection,human-goal-verbs-forgiving,job-record-birth-token,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,receipt-beds-run-the-candidate-engine,review-round-limit-counts-per-chain,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=receipt-beds-run-the-candidate-engine from=unranked to=1:1 requested-sequence=1
- 2026-09-10T07:32:21Z Y70P5YVK9Q98CSWHE7A2ZG427W-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=receipt-beds-run-the-candidate-engine
- 2026-09-10T07:32:46Z SP212Q6CGP64T5HV4P5TTH0P62-m1b-c6925449 slice-start actor=m1b+main-1788680071-18713-e76d5d targets=receipt-beds-run-the-candidate-engine
- 2026-09-10T07:32:57Z 62S14K1DARRTSYHBCWJEA8GXJ9-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=receipt-beds-run-the-candidate-engine
- 2026-09-10T08:44:25Z SX9M1FHE0NWPTXZAEB8RP6AJCT-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=receipt-beds-run-the-candidate-engine
Integrity: sha256=0282a2e6efd533b0ec8ac0f4d9b275ef1562da49a935cf0733cd9e0647ed6f5d
