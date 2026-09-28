# Missionrunner suite speed investigation

- Base: `c860cdf488750e848417c4adbb2c2fccf82d7776`
- Contract: `go test ./internal/missionrunner/ -count=1` completes in less than 90 seconds without changing production behavior or weakening test assertions; package coverage stays at or above 74.9%.
- Baseline: `GOCACHE=/tmp/missionrunner-go-cache /usr/bin/time -p go test -json ./internal/missionrunner/ -count=1 -timeout=20m` completed green in 582.31 seconds wall time. The package reported 580.609 seconds.
- Stop budget: one full candidate comparison for a mechanism that does not improve the contract; no more clock variants after a no-progress result.

## Problems

- Anchor symptom: the package takes about ten minutes, not the required 90 seconds.
- Anchor symptom: representative `TestInternalRunFullCycle` takes about 16 to 17 seconds.
- Consequential symptom: every full gauntlet and battery pays that package cost.

## Clock experiment

- Mechanism: route runner deadlines, sleeps, timers, and fence arithmetic through an engine clock. Keep the wall clock as the production default. Permit a synthetic clock only through `fixtureauth.ClockProbe` on a fake-runtime checkout.
- Expected progress signal: the full package completes below 90 seconds, every existing assertion passes, and coverage remains at or above 74.9%.
- Candidate command: the baseline command against the modified worktree.
- Candidate result: 586.20 seconds wall time; the package reported 585.198 seconds. `TestStillbornInitCleansItsArtifacts` also rejected synthetic artifact timestamps, correctly proving publication timestamps must remain real.
- Classification: `no-progress`.
- Decision: revert the clock experiment. Do not retry clock-interface variants without evidence that runner sleeps dominate wall time.

## Decisive trace

One focused `TestInternalRunFullCycle` run split the 16.79-second test into 1.75 seconds of fixture construction and 15.00 seconds inside `internalRun`. The run completed three real mission cycles. Each cycle spent about 0.84 to 0.90 seconds reserving and opening the turn and 3.05 to 3.44 seconds concluding it. Prompt gating, host supervision, and adjudication were each below 0.44 seconds. The synthetic clock saw 2.8 seconds of logical waits across 28 wait calls, but those waits consumed only about 28 milliseconds of wall settling.

## Supported diagnosis

The suite is dominated by repeated Git-backed mission work: workspace snapshots, anchors, measurement, state and ledger conclusion, plus many independently constructed full-cycle beds in wall, scope, and resolution tests. Runner sleep and deadline loops are not the dominant cost on this checkout.

## Do not retry

- Do not add another runner clock abstraction as a speed fix without a new trace showing wall-clock blocking inside a runner wait.
- Do not compress the existing real-time scale; the standing timing goal already records its cross-test flakiness.
- Do not weaken assertions or replace Git/process-backed proofs with stubs merely to hit the package target.

## Next redesign question

Find the earliest test-fixture owner that can provide an assertion-equivalent reusable mission bed or reduce repeated Git snapshot/anchor work while preserving each test's real boundary. Measure that mechanism with one focused group before another full package run.

## Round two contract

- Primary metric: wall time for `/usr/bin/time -p go test ./internal/missionrunner/ -count=1 -timeout=20m`, measured from `metasystem/`; lower is better.
- Baseline: remeasure base `c860cdf488750e848417c4adbb2c2fccf82d7776` before editing. The round-one reference is 582.31 seconds.
- Target: less than 150 seconds. A change smaller than 10 seconds on the full package is within the noise floor.
- Guard metrics: all existing assertions pass, package statement coverage remains at or above 74.9%, `go test -race ./internal/missionrunner/ -count=1` passes once, and `gofmt` plus `go vet` are clean.
- Mechanism: construct each reusable mission-bed shape once per package run and give eligible tests isolated local copies. Compare Git local/shared clone against APFS copy-on-write copying before selecting the copy mechanism.
- Boundary: tests whose law is mission construction, conclusion behavior, or child-process lifecycle remain on real construction and conclusion paths and are named in the result.
- Budget: one base full-package run, focused mechanism measurements, one candidate full-package run, one race run, one coverage run, and static checks. Stop if the candidate does not materially beat the baseline or isolation/correctness cannot be preserved.
- No-gain budget: 3
- Non-goals: no production behavior changes, synthetic clocks, assertion weakening, dependency changes, commits, or receipts.

### Cycle 2A

- Mechanism/question: Can a completed Git-backed mission-bed shape be copied into fresh isolated test directories substantially faster than rebuilding it, and is local/shared Git clone or APFS copy-on-write faster?
- Novel decision-relevant fact: The relative copy cost and whether copied beds preserve repository-local paths and writable Git state.
- Command/artifact/files: focused fixture timing and correctness checks under `internal/missionrunner`; disposable timing output under `/tmp`.
- Contract signal for progress: copied beds pass the same focused assertions and cost far less than the measured 0.85-second construction boundary.
- Budget and stop condition: one implementation comparison; stop this mechanism if either isolation fails or both copy choices approach construction cost.
- Recoverable checkpoint: base SHA plus the working-tree diff; commits are explicitly out of scope.
- Expected classification and next action: `contract-improved` permits the full-package candidate run; otherwise classify the failed mechanism and stop.
- Result: APFS `cp -cR` copied the complete 13–14 MiB bed in 0.06 and 0.08 seconds. A local/shared Git clone plus copy-on-write restoration of ignored runtime state and the bare origin took about 0.04 seconds, but it did not preserve local configuration, runner refs, or pseudorefs. The Git option therefore changes states that wall and recovery tests assert; APFS copying preserves the complete checkout namespace and was selected. Focused ordinary, nested, full-cycle, and cached parked-bed isolation tests passed.
- Classification: `contract-improved` at the named fixture-isolation boundary. Fresh writable copies now cost well below one real construction, with copied origins and fresh process facts.

### Cycle 2B

- Mechanism/question: Does package-wide shape reuse reduce the isolated full-package wall time below 150 seconds without changing assertions?
- Novel decision-relevant fact: The end-to-end speedup and any order-dependent state that focused copy checks cannot expose.
- Command/artifact/files: `GOCACHE=/tmp/missionrunner-round2-go-cache /usr/bin/time -p go test -json ./internal/missionrunner/ -count=1 -timeout=20m`, with disposable JSON and timing output under `/tmp`.
- Contract signal for progress: all tests pass below 150 seconds; a result at least 10 seconds faster than 582.31 seconds is material if the target remains unreachable.
- Budget and stop condition: one isolated candidate run; stop and diagnose rather than rerun if copied state changes an assertion.
- Recoverable checkpoint: base SHA plus the working-tree diff; commits are explicitly out of scope.
- Expected classification and next action: `contract-improved` proceeds to race, coverage, and static guards; an invalid or failing run gets one focused correctness repair only.
- First result: invalid for acceptance. The package reached 431.41 seconds and six restore assertions refused because copied `FETCH_HEAD` retained the template builder's transport commit. The common failure identifies checkout-local transport state as distinct from the reusable mission shape. Reset that pseudoref in each fresh copy, run the six failures together, then spend the single allowed full retry.
- Classification: `falsified-continue`; complete directory copying alone is insufficient without fresh checkout-local transport state.

### Cycle 2C

- Mechanism/question: Does removing the template builder's `FETCH_HEAD` from each copy restore fresh-checkout semantics while preserving explicit pseudoref tests?
- Novel decision-relevant fact: Whether all six order-dependent restore failures share only that copied transport state.
- Command/artifact/files: the six failed tests together, followed by the one full-package retry if green.
- Contract signal for progress: all six pass in one process; the full retry is green and materially faster than 582.31 seconds.
- Budget and stop condition: one focused repair and one full retry; any new copied-state failure ends this mechanism.
- Recoverable checkpoint: base SHA plus the working-tree diff; commits are explicitly out of scope.
- Expected classification and next action: `contract-improved` proceeds to the requested guards; otherwise stop at the measured floor.
- First retry result: invalid for acceptance at 429.93 seconds. Five of the six reproduced failures passed, but `TestResolveTaintMultiTaintDiscipline` exposed that the first test receiving a newly built cached shape kept the builder's `FETCH_HEAD`; only later copies were reset. Apply the same freshness rule to that first returned bed, verify builder-plus-copy ordering together, and require one final green full-package proof before keeping the mechanism.
- Final result: the first-builder plus copied-instance restore selection passed in 31.416 seconds. The isolated full package then passed in 379.20 seconds wall time, with the package reporting 379.004 seconds. This is 203.11 seconds and 34.9% below the 582.31-second baseline, but 229.20 seconds above the 150-second target.
- Guard results: race passed in 432.25 seconds wall time; coverage passed at 75.3%; gofmt, diff whitespace checking, and `go vet ./internal/missionrunner/...` passed.
- Classification: `contract-improved`. The primary metric beat the 10-second noise floor and every guard passed, but the target is unreachable under the boundary-preservation rule.

## Round two group result

Top-level test elapsed time sums to 373.61 seconds; package setup, teardown, and reporting account for the remainder of the 379.20-second wall result.

| Group | Tests | Summed seconds | Share of test time | Boundary choice |
| --- | ---: | ---: | ---: | --- |
| Preflight and real internal-run boundary | 30 | 185.73 | 49.7% | Real |
| Outage internal-run boundary | 2 | 29.45 | 7.9% | Real conclusion; copied setup |
| Wall and resolution | 39 | 76.22 | 20.4% | Copied setup; one parked conclusion per scale shape |
| Wall scope and recovery | 51 | 57.08 | 15.3% | Copied ordinary, nested, full-cycle, and parked setup |
| Other package tests | 167 | 25.13 | 6.7% | Existing direct fixtures |

The named-real groups consume 215.18 seconds by themselves. Even if every copied-bed test took zero time, the package could not reach 150 seconds without changing which construction, lifecycle, and conclusion laws remain real.

## Named real boundary tests

Construction, admission, and child lifecycle remain real in `TestArmAndPreflightFullPass`, `TestWallPreflightPreconditions`, `TestWallPreflightContractIdentity`, `TestAdmissionBindsTheApprovedContract`, `TestBirthRefusesAReplacedPin`, `TestBirthUsesTheBytesItAuthenticated`, `TestNonRegularStatePathFreezesTheMission`, `TestLostStateFreezesTheBornMission`, `TestLaunchLockSerializesStartDecisions`, `TestSealedBaselineBirthsAndRuns`, `TestBirthRecordSelfHealsAtResume`, `TestUnprovableEmptinessRefusesRebirth`, `TestStillbornInitCleansItsArtifacts`, `TestNestedCheckoutMissionBirth`, and `TestResumeChildRechecksFileMode`.

Conclusion and run-loop laws remain real in `TestInternalRunFullCycle`, `TestInternalRunHostFailureCycle`, `TestInternalRunMalformedReturnCycle`, `TestInternalRunNoReturnCycle`, `TestInternalRunParkRequestCycle`, `TestInternalRunCloseStreamCycle`, `TestInternalRunResumeVerdicts`, `TestInternalRunDispatchTerminalCycle`, `TestInternalRunAnswerAndResumeChain`, `TestInternalRunSoloBuildRecoversThenRepeatParks`, `TestInternalRunOverloadedHostStaysOffTheBreaker`, and `TestInternalRunCleanExitOverloadDocumentStaysOffTheBreaker`.

## Round two design obligations

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| MR2-1 | CRITICAL | Round two contract | Every copied bed has an isolated writable checkout and bare origin | `missionBedTemplate` | `testbed_cache_test.go` copies the complete template and rewrites `origin` | `TestScopeSiblingEditViolates` and the restore selection | `/tmp/missionrunner-round2-final.json` records the green package run | DONE | — |
| MR2-2 | HIGH | Process identity boundary | Every copied full bed receives live process identities and copy-local heartbeat paths | `writeFreshSupervision` | `preflight_fixture_test.go` overwrites copied supervision state | `TestResumeInspectsOpenTurnFirst` and the full-cycle selection | `/tmp/missionrunner-round2-race.out` records the green race run | DONE | — |
| MR2-3 | HIGH | Fresh checkout state | Template transport pseudorefs never enter a test's wall carrier set | `resetMissionBedTransportState` | `testbed_cache_test.go` removes `FETCH_HEAD` from first and copied parked beds | `TestResolveTaintMultiTaintDiscipline` and `TestScopeRestoreVerifiesCarriers` | `/tmp/missionrunner-round2-final.json` records the green package run | DONE | — |
| MR2-4 | HIGH | Boundary preservation | Construction, admission, child lifecycle, and conclusion laws continue to execute their real boundaries | `buildFullCycleRoot` | `preflight_fixture_test.go` retains the real builders and `internalRun` calls | `TestNestedCheckoutMissionBirth` and the named real tests above | `/tmp/missionrunner-round2-coverage.out` records the green coverage run | DONE | — |
