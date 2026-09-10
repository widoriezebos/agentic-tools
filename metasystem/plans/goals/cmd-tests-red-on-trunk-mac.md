# cmd-tests-red-on-trunk-mac

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="severity 2: two command-package tests are red on bare trunk on the m1 Mac, so every landing receipt that runs the command package on this host is red for reasons unrelated to the change; novelty 1: diagnosis of two existing tests; exposure 2: the Mac seats; accumulation 2: every chain landing here pays it until it is fixed or registered"
- Tier: 2
- Intent: Two tests in metasystem/cmd/metasystem fail on bare trunk d1a47c354 on the m1 Mac (Darwin arm64, Go 1.27.1), outside any chain: TestProcessClassifierDataFailureRepairsThenRetriesTheRequestedVerb, subtest arm, at process_verbs_test.go:235 ('ungated status = code 1 stdout "checkout /private/var/folders/.../T/..."'), and TestFrozenPublicVersionOneCorpusRunsAllSixCasesThroughFirstTransitionWorker at test_test.go:281 ('authenticated first-transition worker did not complete all six frozen cases: exit status 78'). Found 2026-09-10 by m1d while proving chain bsws-build1b-20260909 outside the delegate sandbox: the chain's own tree fails them identically to trunk, so they are not the chain's. Neither is in memory/flake-registry.md. DONE means each either passes on this host, or is registered as a known flake with its cause and the host it is bound to, and the landing receipt's verdict on this host is no longer red for them.
- Origin: main
- Next step: Appetite: 1h diagnosis, on the Mac. Run each test alone with -v on bare trunk; read what the arm subtest expects from an ungated status call and why the frozen-corpus worker exits 78 (EX_CONFIG) here; check whether the VM passes them, which would make this a host binding. Fix forward if the cause is in the test; register if it is in the host.
- OpenedAt: 2026-09-10T07:23:43Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T07:23:43Z JPV51QPP1TSD72E1HSTG387S7Q-m1-c6925449 open actor=human:Wido targets=cmd-tests-red-on-trunk-mac
Integrity: sha256=c2672269d6f2e8a9271af57863079c72d776e05c2485b9d11061593390452077
