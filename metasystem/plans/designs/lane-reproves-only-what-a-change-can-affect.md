# Design for lane-reproves-only-what-a-change-can-affect

- Kind: design
- Id: 01M430Y3B2M47QXV8C3PRQDCQP
- Status: accepted
- Goals: lane-reproves-only-what-a-change-can-affect

## 1. Scope

- Step 1 changes one case: a tree holding a green tree's batch, resting on a full proof under an hour old, differing only in files no code unit owns. The lane runs the groups whose declared inputs cover a changed path; the rest keep their green.
- Every other proof stays full: a batch's first, a change in code, the contract or an undeclared path, a failed selection.
- A push resting on a scoped green owes a full proof within the hour; an idle lane is woken to pay it.
- Every proof records full or scoped, why, its base and the groups run.
- The ledger shortcut (703ecb830) is removed.
- Untouched: queue, push, `landing.prove.command`, `metasystem test run`, the full suite.

## 2. Threat model and rabbit holes

Too small a selection puts an unproved change on main; too large or slow, it stalls the lane.

- A second selector: one pure rule only, "a declared input covers a changed path" (`pathpattern.Covers`), the path form of the runner's input digest.
- A language rule in the lane: it calls only `adapter.Detect` and `Closure`.
- Wrong declared inputs: unchecked; a miss lasts until the owed full proof.
- A changed toolchain or VM image: the environment line must equal the base's.
- A scoped proof reported as full: `scope: full` only when the lane asked for full.
- Rabbit holes: `metasystem test run` in the VM, scoping code by the module graph (neither attempted); `fast-static-build`, which the VM suite never runs, needing Git there, or a ledger-only proof over five minutes (unit 2 measures both; either returns to design).

## 3. What is true today

At f67ff3725 the cited lines of facts 1-4, 6, 7, 10 and 11 hold. Corrections:

- 19 of the lane's 25 pushes put a tree on main for which nothing ran. Its last 25 run greens took 16 to 30 minutes, median 21.
- After a tip move the agent re-merges the same shas on the new main (`skills/landing-agent/SKILL.md:44-48`): HEAD does not descend from the green commit.
- The Go adapter's `Closure` widens a file no package owns to every package (`internal/gopackages/select.go:166-171`) and reads `go.mod` in the directory given (`:107-112`); this repository's is `metasystem/go.mod`.
- `metasystem test run` needs Git trees, a goal and a pinned policy engine (`internal/testrun/prepare.go:398-431`); the VM gets an archive.

## 4. Step 1

**Decision**, in `plain.Run`. The base is the newest green result with a `scope` whose commit holds HEAD's batch (`git rev-list --no-merges X ^origin/main` gives the same set). Scoped only when a base exists, its full proof (`fullAt`) ended at most 60 minutes ago, and the selection returns groups; otherwise full, reason recorded.

**Selection**, by the lane's engine (the VM has no Git objects, and a candidate must not choose its own proof), through neutral calls:

1. Changed paths: `git diff --name-only --no-renames BASE TREE`.
2. The contract, read from the tree. Unreadable or itself changed: full.
3. `adapter.Detect(root)`, then `Closure(root, base, tree)` at the repository root; the adapter finds its module root (here `metasystem/`) and returns repository-relative paths. No adapter, an error, or any unit in the closure: full. The Go adapter puts a path no unit owns in a new `Closure.Unowned` instead of widening; `go.mod` and `go.sum` still mean every unit.
4. `testpolicy.Affected(contract, paths)`, new and pure: each group with a declared input covering a path. A path covered by no group, or by a template group (`packageSelection`; `go-affected` declares `metasystem/docs/**`): full.

**To the command**: `LANDING_PROOF_SCOPE` (`full` or `scoped`), `LANDING_PROOF_BASE`, `LANDING_PROOF_GROUPS` (space-separated ids). On `scoped` it runs at least those groups, exiting 0 only when all passed. A command ignoring them runs everything: allowed.

**VM side**: on `scoped` the suite script builds the engine and runs `bin/metasystem test groups ID...`, a new verb running named contract groups in a plain directory under the runner's supervision and expected-test check (as `proofrun.RunGoGateTests`), without proof authority; a named test that did not run is red. It prints `landing group ID STATUS MS` per group and `landing environment TEXT` once.

**Record**. `results.jsonl` lines gain `scope`, `scopeReason`, `base`, `fullTree`, `fullAt`, `ran`, `environment`. `proofs/<attempt>.scope.json` holds the changed paths and `groups[]`: `id`, `state` (`ran` or `kept`), `why`, `from` (where it last ran), `durationMs`. A scoped green whose environment differs from its base's reruns in full, same attempt.

**The owed full proof**. `WakeReasons` gains `full-due`: the newest push rests on a scoped green whose `fullAt` is over an hour old, and no full green ended since. Woken with nothing waiting, the agent proves `origin/main` (a new skill case). `Settled` reuses an exact tree's scoped green only while its `fullAt` is under an hour old, so this tree, pushed on a scoped green, is proved again, in full as its base is too old. Any later full green pays, a batch's too: every lane tree contains main. A red is main's: the agent asks about the lane (case 8).

**Shortcut**. `ledgerPaths` and `ledgerOnlySinceGreen` are deleted. A ledger or plans move selects at most `verb-ratchet`, `shipped-installation-standard`, `agent-protocol-standard`, `fast-static-build` (targets 5, 20, 30, 120 s); `verb-ratchet`'s `metasystem/**` is accepted, sharing the others' test binary.

**Rule text**, appended to `docs/project-rules.md:50`: "In the landing lane a tip move after a batch's full green proof runs every test group whose declared inputs cover a changed path, and the other groups keep that green for at most one hour, after which main is proved in full; a change to code, the testing contract or an undeclared path is proved in full; the lane records which groups ran and why."

**Tests, each red without its rule.** Lane tests stub Git (`ProveSeams.Git`, `stubGit`), time (`Now`) and the adapter (new `ProveSeams.Closure`).

- `plain/scope_test.go`: `TestATipMoveRunsOnlyTheGroupsItsPathsReach` (a changed plans page), `TestABatchsFirstProofIsFull`, `TestAScopedChainOlderThanAnHourIsFull` (`Now` +61 min), `TestSelectionFailureIsAFullProof` (section 7), `TestAChangedEnvironmentProvesAgainInFull`.
- `plain/wake_test.go`: `TestAScopedPushOwesAFullProofWithinTheHour` (`full-due` at +61 min, nothing waiting; a full green clears it); `plain/scope_test.go`: `TestAStaleScopedGreenDoesNotSettleItsTree` (`Settled` at +61 min returns nothing; `Run` proves in full).
- `plain/plain_test.go`: `TestALedgerOnlyMoveRunsItsReaders` replaces the two inheritance tests.
- `testpolicy/affected_test.go`: `TestAffectedGroupsDeclareAChangedPath` (a three-group contract).
- `goadapter`: `TestClosureNamesUnownedPaths` and `TestClosureFindsANestedModule` (`go.mod` under `metasystem/`), on the snapshot seam.
- `proofrun/groups_test.go`: `TestANamedTestThatDidNotRunIsRed` (a one-test module).
- `cmd/metasystem`: `TestPlansAndLedgerMovesSelectAtMostFourGroups`, on the real `testing.json`.

## 5. A Java adopter, rule by rule

- Decision, hour, owed proof, variables, record, shortcut removal: Git and lane records only; they hold.
- No adapter, or two languages (`Detect` wants one): every proof is full, as today.
- With an adapter: `Closure` finds its build root (`pom.xml`, `settings.gradle`) and returns the modules owning a changed path, their reverse dependents, every module for a changed build file, and `Unowned` for the rest; rules 3 and 4 hold.
- `test groups` runs a `command` group by its `argv`; nothing in it is Go.
- A test running another unit's artifact (fact 8; a module's jar): any unit change is proved in full, so it cannot be missed.

## 6. How much this loosens

Today 19 of 25 pushes rest on a green for which nothing ran; under step 1 none does. A pushed tree is proved in full, or holds a batch proved in full under 60 minutes earlier plus the groups reading what main gained. Against the written whole-tree-at-every-push rule, what can slip is a test outside those groups reading an undeclared changed non-code file. The owed full proof starts at most an hour after the full proof the push rests on, so such a miss surfaces within an hour plus one full proof (16 to 30 minutes) of reaching main. Cost: at most one extra full proof per idle stretch.

## 7. When the selection fails

Every lane-side failure of section 4 gives a full proof with its reason. A `test groups` crash, unknown id or no id: red.

## 8. Moved effects

| Effect | From | To | Code |
|---|---|---|---|
| A ledger-only move's green | `Settled` | the detached `Run`; the agent pushes when woken | `metasystem/internal/landing/plain/prove.go:232-239` |
| Paths needing no proof | `ledgerPaths` | group inputs in `testing.json` | `metasystem/internal/landing/plain/prove.go:320-324` |

## 9. Deferred

- Scoped first proofs and their cadence: `scope`, `base`, `ran`, `full-due`; `Closure` units mapped to groups; a tested group field naming the units whose artifact it runs. Engine core is a `testing.json` surface, never a lane list.
- The ten-times hang rule: `groups[].durationMs` in `scope.json`.
- Narrowing `verb-ratchet`'s `metasystem/**` and `go-affected`'s `docs/**`: their `inputs`, judged by `durationMs`.
- Other languages: `adapter.Register`, `Closure.Unowned`.

## 10. Open questions for Wido

1. Step 1 proves any tip move touching code in full; proving changed packages and their dependents waits for step 2. Recommended, and assumed: yes; both recorded livelocks were non-code moves, code reaches main through the lane itself, and the known miss shows the import graph is not yet safe.
2. The VM suite script and the lane's proof command live outside the repository; both need a scoped branch. Recommended: commit the script under `development/`; m1e installs both before unit 3 deploys.
3. The ten-times hang rule (m1e, 2026-10-03) is deferred: no per-package duration is recorded yet (fact 5), the VM's 25-minute per-binary timeout bounds a hang (it fired on 2026-10-04), and `test groups` runs under the stall supervisor. Recommended: build it on `durationMs` in step 2.

## Units

| Unit | Content | Production | Test |
|---|---|---|---|
| 1 | `testpolicy.Affected`; `Closure.Unowned` and the nested module root in the Go adapter | 140 | 290 |
| 2 | `test groups` verb; the real-contract test; the VM script's scoped branch; both measurements | 260 | 380 |
| 3 | Lane decision, variables, records, environment check; shortcut removal; help, skill, rule text | 330 | 520 |
| 4 | Owed full proof: `full-due`, `Settled`'s age check, skill case | 80 | 180 |

Changed lines, estimated. Units 3 and 4 land last, in order: removing the shortcut before the VM honours `scoped` brings the livelock back.
