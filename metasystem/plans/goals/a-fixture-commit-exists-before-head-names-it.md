# a-fixture-commit-exists-before-head-names-it

- State: approved
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="A test fixture change only: a wrong fix leaves one test red on one computer, as today (severity 1); the fixture pattern exists in the same file (novelty 1); it runs only in that test (exposure 1); nothing is left behind (accumulation 1)."
- Tier: 1
- Intent: TestWorkCommitRealLaneFixImpactPlanGitAdapter passes on every computer of the fleet: its fixture no longer points HEAD at an object the repository does not hold (update-ref HEAD with a nonexistent object, work_commit_impact_test.go:159), which git 2.50.1 on the Intel MacBook refuses and git 2.54.0 on the laptop let pass.
- Origin: human
- Next step: Diagnose on the MacBook (seat m1f; the failure reproduces only there: lane proofs of 2026-11-07 and 2026-11-15 08:02Z and their repeats, logs under the MacBook's agentic-tools-landing/metasystem/artifacts/agents/landing/proofs). Fix the fixture so the commit exists before HEAD names it, whatever the git version; test-only change, one unit (--last). The two tests that failed once and passed on the repeat in the same proof (TestAuthenticatedOuterProofKeepsItsNestedWorkloadOutOfOuterProgress, TestGitAdapterGoManifestInventoryMatchesNativeGoList) are noted in the report, not fixed here.
- OpenedAt: 2026-11-15T12:59:34Z
- Revision: 3
- Pinned: m1f
- Budget: elapsedLimit=2d attemptLimit=12 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=NE0NP5MWGQG17Z9A7753ZSHKAE-m1e-666638bb minutes=1200 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-11-15T12:59:51Z revision=2 opid=NE0NP5MWGQG17Z9A7753ZSHKAE-m1e-666638bb authority=proven digest=ab7119e34c0b6f99f20330edcc1354c35e459759dd6302bd301acfb5b09cdbb7 episode=2

History:
- 2026-11-15T12:59:34Z HYVHQTQ6NYFB49F28QYZMG707N-m1e-666638bb open actor=human:Wido targets=a-fixture-commit-exists-before-head-names-it
- 2026-11-15T12:59:51Z NE0NP5MWGQG17Z9A7753ZSHKAE-m1e-666638bb approve actor=human:Wido targets=a-fixture-commit-exists-before-head-names-it
- 2026-11-15T13:00:14Z XMG418XX6KXDW9H13Y64XF9QDW-m1e-666638bb set-pin actor=human:Wido targets=a-fixture-commit-exists-before-head-names-it
Integrity: sha256=9b7a42ab27e853aad7918783582948f21baae71a75d4e23133e4c7360adbd7b9
