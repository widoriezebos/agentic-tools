# a-fixture-commit-exists-before-head-names-it

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="A test fixture change only: a wrong fix leaves one test red on one computer, as today (severity 1); the fixture pattern exists in the same file (novelty 1); it runs only in that test (exposure 1); nothing is left behind (accumulation 1)."
- Tier: 1
- Intent: TestWorkCommitRealLaneFixImpactPlanGitAdapter passes on every computer of the fleet: its fixture no longer points HEAD at an object the repository does not hold (update-ref HEAD with a nonexistent object, work_commit_impact_test.go:159), which git 2.50.1 on the Intel MacBook refuses and git 2.54.0 on the laptop let pass.
- Origin: human
- Next step: Diagnose on the MacBook (seat m1f; the failure reproduces only there: lane proofs of 2026-11-07 and 2026-11-15 08:02Z and their repeats, logs under the MacBook's agentic-tools-landing/metasystem/artifacts/agents/landing/proofs). Fix the fixture so the commit exists before HEAD names it, whatever the git version; test-only change, one unit (--last). The two tests that failed once and passed on the repeat in the same proof (TestAuthenticatedOuterProofKeepsItsNestedWorkloadOutOfOuterProgress, TestGitAdapterGoManifestInventoryMatchesNativeGoList) are noted in the report, not fixed here.
- OpenedAt: 2026-11-15T12:59:34Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-11-15T12:59:34Z HYVHQTQ6NYFB49F28QYZMG707N-m1e-666638bb open actor=human:Wido targets=a-fixture-commit-exists-before-head-names-it
Integrity: sha256=5711483cb286ae45534e41afe356be9e5b0890db6021b1706fd9cbe028494dbd
