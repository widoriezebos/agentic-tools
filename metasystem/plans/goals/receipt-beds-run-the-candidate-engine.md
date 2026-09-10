# receipt-beds-run-the-candidate-engine

- State: approved
- Risk: severity=3 novelty=1 exposure=3 accumulation=1 basis="severity 3: the group refuses every engine-changing candidate; novelty 1: the proof engine and the METASYSTEM_BIN convention already exist; exposure 3: every code landing; accumulation 1: nothing built on it yet"
- Tier: 3
- Intent: Inside a schema-2 receipt (metasystem landing test-receipt --mode auto) the fixture-bed groups (section/gate-fence-fixtures, section/dispatch-fixtures, section/goal-cli-fixtures, section/land-fixtures and the rest) run in an isolated worktree of the candidate but with the checkout's ENROLLED bin/metasystem, so they test the old engine against the new scripts and, for engine-changing candidates, refuse at dispatch's skew preflight. DONE means: the receipt builds one proof engine from the candidate tree the way commit.sh's static re-proof does (go-gate.sh --fast --proof-out), every bed group in the receipt runs with METASYSTEM_BIN set to that engine, the receipt records both digests (enrolled engine, proof engine) and the candidate tree, and a fixture proves an engine-changing candidate's section/gate-fence-fixtures group passes in a receipt. Evidence: proof run proof-mtv5baou-9cb72e840909376d on m1b (35 of 36 groups green, gate-fence refused 'engine commit 2c1bc7db is older than checkout commit 2be2ed91...'); the same on m1's dispatch-cap-necessity. Replaces the first half of gate-fence-fixtures-refuse-engine-changing-candidates.
- Origin: main
- Next step: Small: one change in the receipt's bed runner (internal/landing/testing.go or where test-receipt launches groups) plus the two digests on the receipt. First of the bootstrap members; see the-metasystem-validates-itself-with-itself.
- OpenedAt: 2026-09-10T07:27:25Z
- Revision: 2
- Labels: bootstrap
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-10T07:29:14Z revision=2 opid=4ZK6CRTM3WB0139F8R1W7B9NNZ-m1b-c6925449 authority=proven digest=7791b53db49be871db3b8d576863dbc78b9bd85bf95c16442cc4679e296e3910

History:
- 2026-09-10T07:27:25Z J23YGSQ7VB7AEY7BPVQTRVV2CT-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=receipt-beds-run-the-candidate-engine
- 2026-09-10T07:29:14Z 4ZK6CRTM3WB0139F8R1W7B9NNZ-m1b-c6925449 approve actor=human:Wido targets=carried-landing-debt-and-cap,enrollment-binds-by-the-skew-rule-not-the-tip,human-carried-landing-verb,receipt-beds-run-the-candidate-engine,skew-preflight-knows-a-receipt-worktree,testing-contract-owns-record-paths,the-metasystem-validates-itself-with-itself
Integrity: sha256=faed3e6c9c4c41316a5ec15c3ac19000336ca250a0e2c34eb9ce67a647009309
