# Deep sections cannot stay red unseen

- Kind: design
- Id: 01M3A2YHDM2VCX7NQPGH7MXT95
- Status: done
- Goals: deep-sections-cannot-stay-red-unseen

This design is based on the tree at `ee5d0829c88f06c1388a304b3e39907f77f184b0`. Citations without a historical commit refer to that tree. Historical citations name the commit they describe.

## Why the miss happened

Cadence did run between September 9 and September 14, 2026. At least run 19 executed `section/witness-gate-fixtures` afresh and still reported green. The result depended on its launcher. A cadence section inherited a valid outer proof-worker context, so each nested `go-gate.sh` treated itself as a proof worker and skipped the new standalone proof-engine build. The same fixture started from a plain shell had no outer context, took the build branch, and failed because its fake `go` did not implement `build`. The cadence result was therefore a false green for the standalone behavior that the fixture was intended to exercise, rather than an unreported red.

### The defect and the masking path

Commit `1b12f534984d9c89595e4050800eacc53720df1a` added the standalone full-gate path. The script asks `proof-run worker-authorized` and runs `go build -o "$proof_engine" ./cmd/metasystem` only when authorization fails (`scripts/agents/go-gate.sh:69-85` at that commit).

At the same commit, the witness bed created a leg with a `go.mod`, but its fake `go` accepted only `run`, `version`, `env`, and `list`; every other command failed. It cleared witness variables but did not clear `METASYSTEM_PROOF_*` (`scripts/agents/witness-gate-fixtures.sh:28-36,54-70,73-91,155-168` at `1b12f534`). The defect and its three failing scenarios are also recorded in `records/goals/witness-bed-fakes-the-proof-engine-build.md:6-11` (opened by `0f8f1ef849861f1f76f472e4b9132720a749f011`, concluded by `6d38111d56a31fc173e3db48999659996b9ff256`) and `memory/receipts.log:349` (blame `28c701543406cd1093dce608fee95f331d28a652`).

A proof-run launcher exports the control root, record key, creation claim, attempt, and authorization binary to the suite (`internal/proofrun/launcher.go:193-207` at `1b12f534`). The section adapter also supplies the candidate engine as `METASYSTEM_PROOF_AUTH_BIN` (`internal/proofrun/test_build.go:326-344` at `1b12f534`).

`worker-authorized --root` canonicalized the supplied fixture-leg root, but it authenticated the caller against the inherited control root and process lineage. It did not require the supplied root to equal the admitted execution root (`cmd/metasystem/proof_run.go:192-218` and `internal/proofrun/record.go:184-215` at `1b12f534`). A descendant fixture therefore passed authorization for its unrelated temporary leg. That set `proof_worker=1` and bypassed `go-gate.sh:77-85`.

A plain shell had no authenticated outer proof context, so the build branch ran and the fake rejected it. Commit `28c701543406cd1093dce608fee95f331d28a652` fixed that direct path by adding the narrowly accepted build and the leg configuration. The record says that no assertion changed (`records/goals/witness-bed-fakes-the-proof-engine-build.md:11`, blame `6d38111d56a31fc173e3db48999659996b9ff256`). This causal explanation comes from the code history because the ignored raw logs for the September cadence attempts are no longer present to show the authorization result directly.

### Cadence runs did happen

The repository records cadence runs 15, 17, and 19 on September 12, with attempt identifiers and deep mode, and it records red runs 16 and 18 between them (`records/goals/deep-battery-under-ten-minutes.md:8-11`, intent from `6de92f63b4ded05ea9408ebed352604c2e0b434c`, result from `b53bf2b13217a1fe73d21534bb084afc2cf1d589`). Another record says that `section/go-engine-gate` passed with purpose cadence in those three attempts (`records/goals/command-package-tests-red-inside-the-race-gate.md:9`, `888f1a1a706d57bd604a808e061bdc4de8d609fc`). The rulings record cadence runs 1 through 10 and 12 through 19 (`memory/rulings.md:158-162`, `d08a0a7a079728a0ba9651d33d30b5590b498a5d` and `b1fefb261936c56f93af9c2a7b23905391f0ac23`).

Raw proof attempts, run records, and `validation-weight.json` live under ignored `artifacts/`; the ledger path is explicitly local (`internal/gaterun/weight.go:79-85`; `.gitignore:1`). No tracked commit contains that ledger. The retained evidence therefore cannot establish exact start and end times, the group rows for each historical attempt, whether any run used the display `weight-triggered direct validation`, or whether a September 9 through 14 weight discharge occurred.

### What started cadence

A landing added scaled weight and received only a Boolean `due` result (`internal/gaterun/weight.go:288-328`, principally `a584e5c8cd3e5da6fbad0877daf2d1d5475abe9b` and `fe65edd635d9a7b0dffce04f12e8fb4cbf94a8b0`). The command printed “run the governed direct validator”; it did not launch the validator (`cmd/metasystem/gate_weight.go:33-58`, principally `9cd5befb9740c362b5b5e132c8d7487fd8f456bb` and `fe65edd635d9a7b0dffce04f12e8fb4cbf94a8b0`). `commit.sh` called that bookkeeping after push, described the message as a “NUDGE,” and treated the bookkeeping as non-fatal (`scripts/agents/commit.sh:972-980`, principally `fe65edd635d9a7b0dffce04f12e8fb4cbf94a8b0`).

The operating contract required the standing validator's custodian to run, watch, and discharge the command manually (`docs/collaboration.md:35-68`, `fe65edd635d9a7b0dffce04f12e8fb4cbf94a8b0`, amended by `1b12f534984d9c89595e4050800eacc53720df1a` and `a584e5c8cd3e5da6fbad0877daf2d1d5475abe9b`). R-3 described roughly six accumulated hours, not a scheduler (`memory/rulings.md:28`, amended by `a584e5c8cd3e5da6fbad0877daf2d1d5475abe9b`). The trigger was a threshold that somebody had to notice. There was no scheduled start and no bounded wall-clock guarantee.

The September 12 runs belong to the explicitly staffed deep-battery goal (`records/goals/deep-battery-under-ten-minutes.md:8-11`). The retained records do not prove that those runs were weight-triggered or discharged.

### Whether cadence executed and recorded the witness group

Cadence selected the witness group. At the incident commit, the group was in `cadence`, and cadence purpose selected that list exactly (`testing.json:50,86` and `internal/testpolicy/select.go:205-208` at `1b12f534`). It remains in the list (`testing.json:73,111`; `internal/testpolicy/select.go:227-230`).

Before `db58ad9318c92a24196a20e2eb35b3a07da74ce1`, cadence could reuse exact retained components (`cmd/metasystem/test.go:548-603` at `1b12f534`), although the `go-gate.sh` change was inside this group's declared `metasystem/scripts/**` input and therefore changed its execution identity. From `db58ad931` onward, cadence set `ExecuteAfresh`, and its composer emitted `cadence-executes-afresh` instead of a reused group (`cmd/metasystem/test.go:789-797` and `internal/proofrun/test_result.go:405-455` at `db58ad931`). Runs 15 and 17 proved the pre-change trees `18b5effc0a77e76987fc48bf107ddbb716f7e5e4` and `58b469e28437ff76285fa123f89f66fdd4cd96ed`, so their witness rows may have been retained. Those rows are not available. Run 19 proved `d42308910b2005898c72e81643540952a3409f11`, which descends from `db58ad931`, so run 19 necessarily executed every cadence group, including witness (`records/goals/deep-battery-under-ten-minutes.md:10-11`, `b53bf2b13217a1fe73d21534bb084afc2cf1d589`). The current rule is the same (`cmd/metasystem/test.go:152-154`; `internal/proofrun/test_result.go:423-464`).

Execution under the outer proof context skipped the defective build path, so those executions could be green. That explains how the recorded green cadence attempts coexist with the later plain-shell red. The goal page names the remaining class as “a bed's verdict depends on its launcher” (`plans/goals/deep-sections-cannot-stay-red-unseen.md:10`).

Proof and run results were local ignored artifacts. The steward watched only the first two governed runs whose display was exactly `weight-triggered direct validation`, compared six catch groups including witness, and linked a failure to the run's already claimed goal (`internal/steward/validation_window.go:89-169,172-210`, introduced or amended by `fe65edd635d9a7b0dffce04f12e8fb4cbf94a8b0` and `1b12f534984d9c89595e4050800eacc53720df1a`). It did not publish every cadence result for every seat or the next landing. The shared trunk-red register and the `goal next` projection did not land until September 17 (`fae71a23de78b3f007daa54f78632e02bc5b5b89`; `internal/goal/trunkred.go:359-428`, `cmd/metasystem/goal.go:493-604`).

### Deep-only sections

The inventory loads `testing.json`, unions canary and standard selection over every surface, and includes fallback selection, consumer closure, critical-obligation providers, and maximum accumulation. This follows the affected-surface and consumer rules at `internal/testpolicy/select.go:117-160`, standard selection and critical providers at `:180-217`, and cadence selection at `:227-243`. A section-adapter group outside that union is deep-only. All 21 current results are in the cadence list (`testing.json:111`):

1. `section/covenant-evidence-pre-rebuild`, `section/go-engine-gate`, `section/covenant-evidence-post-rebuild`, and `section/suite-host-prerequisites` (`testing.json:65-66,69-70`).
2. `section/gate-fail-open-tripwire`, `section/supervisor-fingerprint-heal-harness`, `section/telemetry-census-fixtures`, and `section/authority-regression-fixtures` (`testing.json:72,79,85,88`).
3. `section/pre-commit-guard-fixtures`, `section/static-reproof-fixtures`, `section/project-extra-suites`, `section/record-protocol-fixtures`, and `section/evidence-segment-fixtures` (`testing.json:89-93`).
4. `section/second-session-fixtures`, `section/lease-succession-fixtures`, `section/flight-recorder-fixtures`, `section/acp-fixtures`, and `section/delegate-caps-fixtures` (`testing.json:94-98`).
5. `section/enumeration-mode-fixtures`, `section/agent-protocol-fixtures`, and `section/watch-background-jobs-fixtures` (`testing.json:100,102,107`).

`section/witness-gate-fixtures` is explicitly present in the proof surface's `deep` list, but it is not deep-only under computed selection. The same surface marks `test-execution-integrity` critical, the group provides that obligation, and critical providers join standard (`testing.json:7,73`; `internal/testpolicy/select.go:192-197`). The same relationship existed at `1b12f534` (`testing.json:6,50`). The gate-plumbing landing that introduced the defect did not select witness, and unrelated standard surfaces need not select it. The evidence does not support a stronger claim that no standard selection could ever run it.

## Design

### One owner and one bounded trigger

The design adds no role, daemon, shell script, or terminal multiplexer path. It extends the dedicated landing-owner process with a global cadence tick implemented as the one-shot Go verb `metasystem gate cadence-tick`. The owner already ticks each minute and is the only lease holder allowed to advance landing work (`cmd/metasystem/landing_batch_owner.go:28,92-144,426-466`; `internal/landing/batch/owner.go:69-96`).

Each tick fetches `origin/main` and considers three triggers. A run is due when a deep-only group's revalidated execution identity is changed, missing, or newest-non-green; when validation weight is due; or when six hours have passed since the last forced sweep, matching R-3. A relevant trunk change surfaces within one owner tick plus the bounded run. An unmodelled dependency surfaces within six hours plus one tick and the run. An unrelated new trunk tip for which every exact identity remains green starts no group and publishes a tip-bound revalidation observation. Tests inject the clock; production alone supplies the real clock.

The existing `standing-validation` goal and obligation remain the governed accounting owner. A person approves that standing authority once, while individual runs need no manual reminder. The tick claims that existing goal, launches the run, discharges an authorized green weight generation, and releases the claim. It never opens a goal. If the standing authority is absent or refuses, the cadence status becomes non-green and visible instead of silently skipping (`internal/gaterun/weight.go:331-417`; `plans/goals/standing-validation.md:6-8`).

Ordinary seats do not run cadence; they route to the configured landing owner. The local landing-owner lease excludes a second process. A shared compare-and-swap claim excludes another machine. The claim is keyed by the trunk tree, weight generation, and forced-window start and lives in the same goal-ledger file as cadence status. Concurrent ticks join a live claim for that key or read its terminal result. A claim can be recovered after its injected-clock lease expires. A live owner is not duplicated merely because its work is slow.

### What runs and what may be reused

The cadence inventory is derived from the contract rather than from the six-name `CadenceCatchGroupIDs` list (`internal/testpolicy/metasystem.go:5-14`). The runner launches the existing full cadence plan on the exact fetched trunk tree with `test run --mode deep --purpose cadence --tree <tree>`. Keeping the full cadence plan preserves weight-discharge evidence while allowing execution to be pruned.

The current `ExecuteAfresh` setting is split into `ForceAttempt` and `ForceGroups`. Every trigger produces a recorded attempt. An input or weight trigger may reuse an unchanged group's newest complete green result only when its exact `groupExecutionIdentity` matches. Missing, live, failed, incomplete, and changed evidence executes. A six-hour trigger sets `ForceGroups` and executes every cadence group. Existing reuse already rejects the newest failed or live observation (`internal/proofrun/test_result.go:316-366,449-464`).

The cheap no-change probe uses `RevalidateRetainedGroupExecutionIdentities`. It rehashes declared and retained implicit inputs and executable bytes without version helpers, `go list`, tests, builds, or downloads (`internal/proofrun/test_build.go:1048-1124`, `1b12f534984d9c89595e4050800eacc53720df1a`). Exact identity covers the group definition, inputs, environment, tools, discovery, platform, contract, judge, behavior policy, and section engine (`internal/proofrun/test_build.go:1221-1245,1409-1455`).

Reuse is not sound for a launcher or ancestry dependency, an undeclared external service, wall time, randomness, or mutable state absent from the environment, input, or tool identity. This incident demonstrates that boundary. Such a dependency must become an input, tool, or environment identity, or the group must be marked non-retainable. The six-hour forced sweep remains the backstop. A red result is never reused as permission.

### Durable result, visibility, and ownership

The existing `plans/goals/trunk-red.json` transaction gains one compact latest-cadence status. It records the trunk commit and tree, trigger, run and attempt identifiers, start and end times, weight generation, forced-window start, and each deep-only group's execution identity, status, evidence digest, and reuse source. The transaction publishes that status and any red entries atomically. Green and red observations therefore live in the shared ledger instead of ignored per-seat artifacts. Publication retains the trunk-red publisher's identity-based idempotence (`internal/goal/trunkred.go:359-451`, `fae71a23de78b3f007daa54f78632e02bc5b5b89`).

Every non-green group follows the batch adapter's existing result-to-`RedGroup` and `RedGroup`-to-`TrunkRedRecordGroup` mappings (`cmd/metasystem/landing_batch_red.go:95-129`; `cmd/metasystem/landing_batch_trunkred.go:47-66`). If a person has already assigned an existing live fix goal, cadence preserves that assignment. Otherwise it publishes an ownerless entry. `goal next` already shows owned entries first and every unowned or differently owned entry as owned by “nobody.” Taking an entry requires an existing live goal; opening that goal remains a person's act (`cmd/metasystem/goal.go:519-581,586-604`; `internal/goal/trunkred.go:518-580`, `d66e323c27b03703422de7a445c0ab836cd78d22`). Steward health already treats an unowned entry as dead and names the ownership command (`internal/steward/trunkred.go:30-66`).

Batch status reads the latest cadence status. A missing cadence run and an overdue run are printed in batch status and reported as not green by health, but neither holds a landing. A cadence red affects landing only through the existing trunk-red entry path and its existing `held-trunk-red` behavior. There is no cadence-specific hold state and no cadence run is started by this read.

An existing `plans/goals/trunk-red.json` without cadence fields remains valid. Reading it yields no recorded cadence run. A no-op write retains its exact bytes and digest.

### End-to-end proof

`TestCadenceTickSurfacesPlantedDeepOnlyRedWithoutDeepLanding` creates a temporary shared origin, goal ledger, landing-owner root, and two seat roots. Its minimal contract contains one section absent from every standard selection but present in cadence. The fixture records green, pushes a commit that makes only that section fail, and advances an injected clock by one owner tick without a deep landing. Two simultaneous ticks must produce one native execution, a tip-bound non-green cadence status, an ownerless trunk-red entry visible in `goal next` from both seats, and a held next batch. The test uses no sleeps, retry loop, or wall clock.

### Delivery units

The contract inventory belongs in `internal/testpolicy/deep_only.go` and its focused test. It computes standard reachability rather than repeating the result as a list, and the real contract must cover every computed deep-only section in cadence.

Shared cadence state belongs beside the trunk-red register. It adds strict status and claim schemas, lease-based compare-and-swap behavior, atomic status and red publication, and existing-goal-only ownership.

Reuse policy belongs in `internal/proofrun/attempt.go` and `internal/proofrun/test_result.go`. It separates forced attempts from forced group execution and reuses only exact newest green identities.

Trigger choice belongs in `internal/gaterun/cadence.go`, with a small integration in `weight.go`. It owns the three triggers, artificial time, live and dead claim handling, and authorized discharge.

The owner and command wiring belong in `cmd/metasystem/gate_cadence.go` and `landing_batch_owner.go`. They add the one-shot Go verb, minute-tick wiring, batch status, and the existing trunk-red hold response.

The final fixture belongs in `cmd/metasystem/cadence_trigger_batchtest_test.go` and uses the existing `batchtest` package tag.

## Rulings

A cadence red is published to the trunk-red register through the same result-to-`RedGroup`-to-`TrunkRedRecordGroup` mapping as a batch red. It stays ownerless until a person names an existing live goal. This design opens no goal and adds no ownership rule.

An overdue or missing cadence status never holds a landing. Batch status prints it and health raises it. A cadence red holds landings only through the unchanged trunk-red entry path, with no new hold state.

A `plans/goals/trunk-red.json` file without cadence state loads unchanged, and a no-op write leaves its bytes and digest unchanged. An absent cadence status means that no cadence run has been recorded yet, which health reports as not green.

All time comes from an injected clock. Tests use no sleep, wall time, or retry loop.

## Open class

`proof-run worker-authorized --root` authenticates the caller's lineage but never compares the supplied root with the attempt's admitted execution root, so a fixture leg under an outer proof is treated as a proof worker and skips the standalone build branch of `go-gate.sh`; this is fixed by a separate job.
