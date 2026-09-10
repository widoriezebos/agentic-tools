# skew-preflight-knows-a-receipt-worktree

- State: queued
- Risk: severity=3 novelty=1 exposure=3 accumulation=1 basis="severity 3: same refusal; novelty 1: one comparison reads a different stamp; exposure 3: every code landing; accumulation 1: nothing built on it"
- Tier: 3
- Intent: dispatch.sh's skew preflight (around line 249: engine stamp older than checkout HEAD along the ancestry path, and engine or agent scripts changed, then refuse 'run go-build.sh, then steward arm') is right on a seat's checkout and wrong inside a receipt worktree, where the checkout IS the candidate and the engine that will run is the proof engine built from it. DONE means: the preflight compares the engine that will actually run (METASYSTEM_BIN when set, else bin/metasystem) with the checkout it runs in, so a candidate-built engine in a candidate worktree shows no skew; a seat checkout with a stale engine still refuses exactly as today; fixtures for both. Second half of gate-fence-fixtures-refuse-engine-changing-candidates, which this and receipt-beds-run-the-candidate-engine together replace.
- Origin: main
- Next step: Small: dispatch.sh preflight reads the stamp of the engine it will launch; one fixture in dispatch-fixtures.sh. Second of the bootstrap members.
- OpenedAt: 2026-09-10T07:27:29Z
- Revision: 1
- Labels: bootstrap
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T07:27:29Z 50AGRJMEYNP0GD01N57BN4FMGP-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=skew-preflight-knows-a-receipt-worktree
Integrity: sha256=566eb29f7b376c86c1f54e113fa8649f52331ff9537b162dc3cd765f6d7310cd
