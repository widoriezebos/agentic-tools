# Lane rehearsal, unit 1, correction 2

- Owner: this delegated implementation session in the existing goal worktree.
- Goal: lane-rehearsal-before-landing. Correction 2 is implemented, but U1 is NOT ready: the real scenarios exhausted their ten-minute budgets and the whole command-family run timed out at thirty minutes.
- Next step: coordinator reads this last correction and the failed verification; design follow-ups U1c and U1d carry the remaining proof work. No third correction round was started. No source commit, push, rebase, stash or reset was performed.
- Human decisions: keep this as one unit despite the size limit; do not split it after the fact. The design records the whole-unit Opus read and size exception.
- Separate work: receipt add reports that a retro is due; no retro or instruction change was undertaken here.

## Changed behavior and review entry

Read `cmd/metasystem/landing_rehearse.go` first: its scoped rehearsal effects are the authority boundary. The candidate's real `landing set` registers the private lane; its real `landing run` constructs and ticks the production keeper. Only the person, supervision readiness and model start are scripted. The scratch directory must be an accepted rehearsal store with its recorded directory identity, and the executable, installation, registry and target lane must all belong to it. The invoking controller contains no copied wake or selection logic. `Scratch.CheckIdentity` remains the candidate engine's single build-identity check.

The public command now has an example and appears in the public-pairs table. Private driver runs and test registries save their records inside scratch. Only the public command opts into checkout publication, and an isolated registry cannot opt in. This works when the CLI runs through `go run`, an installation pin or `bin/metasystem`; publication does not depend on one executable location.

Templates are read from the pinned candidate clone. Setup retry removes its first scratch once another scratch exists, merges both attempts' steps, and keeps the surviving scratch on failure.

The three named rehearsal records and the setup-deadline record were removed as directed. Previous handoff record paths are superseded; no test/private-clone evidence is copied into this repository's records.

## Size

Physical production lines: engine.go 88, green.go 256, proof.go 49, scratch.go 230, timing.go 55, command 88, fixture value.go 3: **769**. The two integration lines in existing intent files bring the inclusive count to **771**. Counts include blank lines and comments, and exclude tests and template data.

## Tests added or changed in this correction

| Test | Change and mutation caught |
| --- | --- |
| `TestRehearsalChildScopeRequiresItsAcceptedStoreAndPrivatePaths` (new, parallel) | Rejects another module, engine, registry, unregistered store and scratch alias; catches broadening the rehearsal authority beyond its accepted private store. |
| `TestRehearsalPrivateRecordNeverWritesTheInvokingRepository` (new, parallel) | A private driver and isolated caller keep records in scratch; catches default publication into the invoking repository. |
| `TestRehearsalRecordPublicationRequiresPublicCommandAndNormalRegistry` (new, parallel) | Public opt-in can publish; an isolated registry, private driver and different destination cannot; catches removing the opt-in, registry or destination guard. |
| `TestIntentPublicCoverage` (changed table) | Requires the command's example, public-pair membership, help, grammar and catalogue; removing the example or pair reproduces the reported red. |
| `TestLandingRehearsalRealRepositoryGreenAndIsolation` (changed) | Requires candidate `landing set` and `landing run` results, a queued wake and production keeper fingerprint, scratch-only record placement, and successful use of pinned templates despite an invalid working-copy template. Catches the copied keeper, omitted candidate tick, wrong record destination or reading working-tree templates. Retains private-origin movement, landed status, budget and unchanged host/hook assertions. |
| `TestLandingRehearsalRealCanaryCostRegression` (changed call signature) | Keeps the real candidate mutation counting canaries, requires full-depth refusal before push, and verifies unchanged private origin; removing the canary exemption must still fail this scenario. |
| `TestRehearsalIdentityAndRecordKeepFailedScratch` (changed record path) | Wrong engine identity records a failure in retained scratch; catches a false pass or writing the private failure in source records. |
| `TestRehearsalCancelledSetupDoesNotRetry` (changed call signature) | Cancelling before setup starts no child and does not retry; catches an extra attempt after cancellation. |

All newly added tests and subtests call `t.Parallel()`. Existing executable writes still use the testexec fork lock. Searches for `RunGreen`, `Prepare`, `HarnessSetup`, `recordScratch`, `createTempStore` and `CreateTempStore` identified their existing test consumers; the rehearsal package, `TestLandingRehearsal*`, `TestTempStore*` and `TestAUnitsFindingsStoreEndsWithItsUnit` were included. Real contract, actual ledger grammar and real impact-plan assertions remain in `TestLandingRehearsalRealContractLedgerAndPlan`.

## Verification exits collected so far

All own Go test invocations use `-count=1 -timeout 30m`. The source testing.json was not edited. No source local secrets file was opened.

- Reported red reproduced: `go test -count=1 -timeout 30m ./cmd/metasystem -run '^TestIntentPublicCoverage$'`: exit **1**, missing example and public pair.
- `go vet ./...`: all three invocations exit **0**, including the final source.
- `go test -count=1 -timeout 30m ./internal/landing/rehearsal/... ./internal/landing/plain ./internal/layering`: all three runs exit **0**, including the final source.
- `go test -count=1 -timeout 30m ./internal/layering`: exit **0**.
- `go test -count=1 -timeout 30m ./cmd/metasystem -run TestAudit`: exit **0**.
- `go test -count=1 -timeout 30m ./internal/audit ./internal/testenv ./internal/hostsetup`: exit **0**.
- `go test -count=1 -timeout 30m ./internal/diskstore -run 'TestTempStore|TestTempStoreAfterAnInterruptedCreation|TestOwnerTempStore'`: exit **0**.
- `go test -count=1 -timeout 30m ./internal/diskstore -run '^TestAUnitsFindingsStoreEndsWithItsUnit$'`: exit **0**.
- `go run ./cmd/devgate static`: exit **1**, only the brief-permitted missing goal for the accepted design; the acceptance repeat also exits **1** for that same complaint.
- Whole `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestIntent|TestAudit|TestLanding'` family check: superseded initial run **143** (cancelled); final verbose run **1**, `panic: test timed out after 30m0s`, package elapsed 1818.460 s. It timed out during existing serial integration tests, before starting the parallel rehearsal cases. No ordinary failing assertion was reported before timeout.
- Exact public coverage check on final source: `go test -count=1 -timeout 30m ./cmd/metasystem -run '^TestIntentPublicCoverage$'`: **0**, package 7.324 s; the reported missing-example/pair red is fixed.
- Dedicated real rehearsal family: `go test -count=1 -timeout 30m ./cmd/metasystem -run '^TestLandingRehearsal' -v`: **1**, package 1268.352 s. Grammar and the real contract/ledger/plan test passed; green and canary scenarios failed on their ten-minute limits. Details below.
- Unnecessary HEAD-only carried-test control: **143** (cancelled after the current full-family log confirmed its refusal is expected and that test passes); no verification credit claimed. The private clone used only tracked HEAD and no local settings.
- `go run ./cmd/metasystem experiment check --file plans/handoff-lane-rehearsal-before-landing-u1.md`: **0**, one invalid-run investigation cycle, no no-progress cycles. No production changes were made for recovery.
- `go run ./cmd/metasystem goal list --ready`: **0**; read the existing claim's continuation, made no claim or ledger changes.
- The public receipt action: exit **0**, appended this correction's receipt and reported a separate retro due.
- `git diff --check`: exit **0**.

Raw logs and the failed green JSON record are mirrored with verified content hashes outside the repository; see the evidence location below. No rehearsal records were copied into source records.

## Design obligations

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| candidate registration and keeper | HIGH | D1, correction 2 | All lane decisions execute in the candidate | candidate verbs and production keeper | `Scratch.Green`, `rehearsalLaneEffects` | real green and scope tests | setup failed before keeper; private candidate registration was reached in the canary attempt | PARTIAL | U1c |
| host and record isolation | CRITICAL | D1, correction 2 | No host changes or private records in source | ChildScope and Finish/public opt-in | accepted private store and target checks | scope, publication and real isolation tests | setup failed before keeper; private candidate registration was reached in the canary attempt | PARTIAL | U1c |
| real contract, ledger and plan | HIGH | D3, step 1 | Tier-one impact; canary counting refuses before push | candidate impact/proof verbs | fixture harness, CheckGreenProof | real contract/ledger/plan and canary mutation | actual plan assertion passed; scenarios did not reach their intended outcomes | PARTIAL | U1c |
| failure evidence and budget | HIGH | D5 | Failures retained; ten-minute limit and timings | Scratch | Finish, bounded children and stages | cancellation, failed identity, expiry, cleanup and timing tests | green setup consumed 600007 ms; canary bootstrap timed out | CONTRADICTED | U1c |

## Default completion check

1. Requested code changes exist, and the reported public-help failure is fixed. The requested green and canary runtime outcomes and all-green command families are NOT proved; U1 remains unfinished.
2. Registration and wake/selection belong to the candidate's real verbs/keeper; Scratch owns identity, setup, deadlines and recording; only the public command opts into checkout publication.
3. Focused rehearsal/plain/layering tests, exact public coverage, repository/host audits and module vet passed. The real contract and ledger shapes produced the actual plan; counting its real canaries classified full depth. The real end-to-end scenarios did not finish, and the combined command-family check timed out.
4. The scenario cancels at ten minutes and never publishes a pass on failure; green failure evidence and scratch were retained. Setup retry preserves steps and removes the earlier scratch when another exists. An invalid child scope falls back to ordinary authority rather than granting rehearsal authority.
5. End-to-end keeper/push success, intended canary-depth refusal and full named-family completion remain unverified. The whole-module Go test suite belongs to the landing lane and was not invoked. Actual model judgement and a live supervisor/enrolled human terminal are deliberately scripted rehearsal boundaries. No third correction round or timeout increase was attempted.

## Real-run wall times and breakdowns

- Green scenario: **600007 ms (10m0.007s)**, all setup; bootstrap, hand-in, gate, proof and push **0 ms**. Test elapsed **736.61 s**, including candidate freezing and cleanup. Its candidate engine built successfully, then checkout hit the scenario deadline before registration. Candidate tree `c5264ef22f697434bcc9d75bbeea60510875bf2f`; JSON retained in the raw log and evidence directory.
- Canary-mutation test elapsed **999.84 s**. Its scenario reached `landing prove --trunk --wait` but exhausted the ten-minute scenario limit before the intended depth refusal. The test's early unexpected-error assertion did not emit its per-stage record, so the exact scenario wall milliseconds and breakdown are **unavailable**, not guessed. This diagnostic gap is included in U1c. It was not a successful negative proof.
- The real contract/ledger/plan test passed in **1199.48 s**, including two real preparations/build reuse and cleanup. Actual plan: cheap with canaries excluded; full when they are counted. This proves the policy on real inputs, but cannot substitute for a completed green/canary scenario.


## Setup investigation (bounded, no production edits)

### Cycle 1
- Mechanism/question: do the real green and canary scenarios reach the candidate's registration and keeper under their ten-minute limit?
- Novel decision-relevant fact: the green run spent all 600007 ms in setup: archive 7337 ms, extract 142522 ms, candidate build 338336 ms, clone 42138 ms, checkout interrupted after 68866 ms. The candidate engine was built successfully, but setup expired before registration. The concurrently running real-contract test reached the actual plan and still classified canary counting as full.
- Command/artifact/files: `/tmp/rehearsal-u1-correction2-real-runs.log`; candidate tree `c5264ef22f697434bcc9d75bbeea60510875bf2f`.
- Contract signal for progress: green lands at impact depth; canary mutation refuses at full depth before push; both finish under ten minutes.
- Budget and stop condition: one environment retry only, already present in the required whole command-family run; no timeout change, fixture weakening or third correction round. If that run cannot prove the scenarios, retain the red as a named follow-up unit.
- Recoverable checkpoint: uncommitted source and failed accepted scratch recorded in the log; source history and index preserved.
- Expected classification and next action: `invalid-run` for the scenario claim, because setup timed out before any lane action. Do not patch lane behavior. Read the existing whole-family run, which builds the latest candidate with the now-populated build cache.

Supported diagnosis: time was consumed by real archive extraction, compilation and checkout, rather than a refusal by the lane. A copied keeper cannot explain a failure before registration. Warm-cache operational recovery uses the existing owner and identity checks; changing the scenario limit or prebuilding under a false identity is excluded. No new abstraction or dependency is proposed.


Cycle result: the required whole-family run returned exit 1 after its thirty-minute timeout, before dispatching the rehearsal cases. It supplied no successful environment retry. The remaining acceptance work is named in design units U1c (setup budget and runtime proof, including records for unexpected negative failures) and U1d (complete the requested test families within their existing timeouts). The implementation is preserved uncommitted for review; no further recovery run was launched.


## Final evidence and completion-gate exits

Evidence directory: `/Users/wido/metasystem-evidence/agentic-tools-m1e-lrbl/lane-rehearsal-u1-correction2-20261011-0341`. Every copied raw log was checked by SHA-256 against its original; `sha256.json` records those hashes. `failed-green-record.json` extracts the actual CLI record. The canary attempt's exact per-stage record was not emitted by its early unexpected-error assertion.

Both `go run ./cmd/metasystem design check plans/handoff-lane-rehearsal-before-landing-u1.md` and the same command with `--complete` exit **1**, naming the partial candidate/isolation/real-input obligations and contradicted timing obligation. The gate correctly refuses completion.

Final `git diff --stat` (tracked paths only, including changes already present at intake):

```text
 metasystem/cmd/metasystem/audit_modes_test.go             | 1 +
 metasystem/cmd/metasystem/audit_output_layout_g1b_test.go | 1 +
 metasystem/cmd/metasystem/audit_structured_output_test.go | 1 +
 metasystem/cmd/metasystem/intent.go                       | 2 +-
 metasystem/cmd/metasystem/intent_coverage_test.go         | 2 +-
 metasystem/cmd/metasystem/intent_landing.go               | 2 ++
 metasystem/internal/diskstore/tempstore.go                | 5 +++++
 metasystem/memory/receipts.log                            | 3 +++
 8 files changed, 15 insertions(+), 2 deletions(-)
```

This stat omits the untracked unit implementation, tests, templates, design and handoff. All remain uncommitted. The named stray records are gone; no new source rehearsal records exist.
