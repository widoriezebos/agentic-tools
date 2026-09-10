# receipt-beds-run-the-candidate-engine

- State: claimed
- Priority: 1
- Sequence: 1
- Risk: severity=3 novelty=1 exposure=3 accumulation=1 basis="severity 3: the group refuses every engine-changing candidate; novelty 1: the proof engine and the METASYSTEM_BIN convention already exist; exposure 3: every code landing; accumulation 1: nothing built on it yet"
- Tier: 3
- Intent: Inside a schema-2 receipt (metasystem landing test-receipt --mode auto) the fixture-bed groups (section/gate-fence-fixtures, section/dispatch-fixtures, section/goal-cli-fixtures, section/land-fixtures and the rest) run in an isolated worktree of the candidate but with the checkout's ENROLLED bin/metasystem, so they test the old engine against the new scripts and, for engine-changing candidates, refuse at dispatch's skew preflight. DONE means: the receipt builds one proof engine from the candidate tree the way commit.sh's static re-proof does (go-gate.sh --fast --proof-out), every bed group in the receipt runs with METASYSTEM_BIN set to that engine, the receipt records both digests (enrolled engine, proof engine) and the candidate tree, and a fixture proves an engine-changing candidate's section/gate-fence-fixtures group passes in a receipt. Evidence: proof run proof-mtv5baou-9cb72e840909376d on m1b (35 of 36 groups green, gate-fence refused 'engine commit 2c1bc7db is older than checkout commit 2be2ed91...'); the same on m1's dispatch-cap-necessity. Replaces the first half of gate-fence-fixtures-refuse-engine-changing-candidates.
- Origin: main
- Next step: CLAIMED by m1b 2026-09-10 10:30Z on Wido's word ('get this unblocked'). Build dispatched to Sol as rbce-build1: the receipt builds one proof engine from the candidate tree (go-build.sh --out, stamp = the candidate commit) and hands it to the beds as CandidateEngine, keeps the enrolled binary as the policy engine, records both digests on the result and the receipt, refuses rather than falling back when the candidate engine does not build; three fixtures. Seam: cmd/metasystem/test.go line ~511 os.Executable() handed to testingRunRequest; proofrun/test_build.go prepareSectionEngine. Then closing read (Fable lane), receipt (the first engine-changing candidate to take one, if the preflight passes with a candidate-stamped engine; else member 2 first), land. Then land review-round-limit-counts-per-chain and the abandoned-goal chains in order.
- OpenedAt: 2026-09-10T07:27:25Z
- Revision: 6
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
Integrity: sha256=403129eef6482b4c17fba9a6ed198a77ba6637c33b2bfefb15133d6aa81aaf6c
