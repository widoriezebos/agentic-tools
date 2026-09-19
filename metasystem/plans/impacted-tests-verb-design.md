# Impacted tests behind one verb

Revision: r2, final design fold before building. The independent r1 critique is `artifacts/reports/impacted-tests-verb-critique-r1.md`; dispositions below cover all 13 findings. The owner's scope ruling limits implementation to eight units, 3,200 changed lines. No further design round is planned.

## 1. Boundary and evidence

Agents call only `metasystem test impacted`. Its handler gathers changes, delegates to project configuration, validates and reports. `internal/testimpact` owns protocol/invocation; `internal/goimpact` owns Go selection. Neither the handler nor testimpact imports goimpact; only command registration wires both. No engine, enrollment, state root, network, process census, dependency, separate binary, or feature flag is introduced.

Read: dispatch/build.go:1165–1185, testpolicy/contract.go:23–60,185–218,308–315,370–425, select.go:15–60,95–130, main.go:40–60, landing_batch_prove.go:175–215, proofrun/test_build.go:1280–1306, the brief template and testing.json. The contract has 78 groups: 33 Go, 44 section, one command. Existing risk selection remains delivery authority; this verb supplies builder feedback.

## 2. Verb, configuration and fallback

**V1.** `metasystem test impacted [--base <commit-ish>] [--list] [--json]`. Find the Git root; read working-tree `metasystem/testing.json`. Base defaults to HEAD and resolves once to a commit ID. Builders never commit, so HEAD is their starting commit. An unborn repository requires an explicit valid base.

Collect `git diff --name-only -z --no-renames <base> --` plus `git ls-files --others --exclude-standard -z`; sort/deduplicate. Include staged/unstaged net changes, deletions, both rename paths and untracked nonignored files. Reject conflicts, escaping paths and non-UTF-8 names; preserve spaces. Bind evidence only to base and changed paths: SHA-256 of length-prefixed base, sorted paths, modes and content hashes, with deletion markers and symlink targets, never following outside links. No whole-worktree scan or before/after drift gate. Exclusive builder checkout custody is assumed.

**V2.** Optional top-level member:

```json
"impacted":{"cwd":"metasystem","argv":["metasystem","tool","go-impacted"]}
```

Require both fields; cwd must exist inside the repository (`.` allowed). Share command-adapter argv validation: nonempty executable, no blanket-success command; reject NULs. Execute directly, without shell interpolation. Unknown fields/null are invalid. Preserve the member through contract merge, pruning, adoption, audit and serialization; conflicting edits follow existing merge-conflict behavior. It is not a test group. Exact argv[0] `metasystem` resolves through `os.Executable`, including the still-running temporary binary from `go run ./cmd/metasystem`; other commands use cwd/PATH normally.

**V3.** This is the owner's “never fail because unconfigured” ruling: missing member, missing contract, or `tailoringRequired:true` means full fallback. Detect the template before `Contract.Validate`; do not weaken validation for other consumers. Decode syntax strictly; validate declared executable groups even in fallback.

With groups, run every group once, sequentially in declaration order, including cadence groups, with group cwd/environment and inherited caches. Export proofrun's existing argv construction unchanged: Go through its existing goArguments path, section as `bash scripts/agents/validate-section-selector.sh run <id>`, command as declared argv. Reuse argv only; judge exit status, without engine admission, JUnit validation, execution identities or retained proof. Continue independent failures. Unavailable prerequisites produce incomplete results. List mode constructs commands without executing tests.

With no groups, exit 0, `fallback:true`, `fallbackReason:"no-tests-declared"`, result `status:"not-run"`, empty selections, and loud `FULL TEST FALLBACK: NO TESTS DECLARED`. Never say passed. Any evidence consumer must refuse this as successful testing evidence; it must not block the orchestrator's proof. No scripts/test.sh convention, undeclared-provider refusal, or implicit Go provider. Other fallback reasons are `implementation-undeclared` and `tailoring-required`; no-tests-declared takes precedence. Malformed configuration remains an error.

**V4.** Stdout is a summary or one JSON envelope: `{schemaVersion:1,request,implementation,fallback,fallbackReason,result,error}`. Request/result may be null before construction; implementation identifies actual cwd/argv or full fallback. Error is null or `{code,message}`. Summary includes mode, base, binding, fallback reason, status, labels, names/whole-package scope, outcomes, seconds, reasons and uncertainties. Even with JSON, stderr loudly announces fallback, nothing-selected and incomplete outcomes, alongside progress.

Exit 0: passed, listed, nothing-selected, or the no-tests-declared exception. Exit 1: valid failed or incomplete execution. Exit 2: usage, configuration/input/protocol errors, provider launch/nonzero exit or cancellation. Register `TEST_IMPACT_USAGE`, `TEST_IMPACT_CONFIG_INVALID`, `TEST_IMPACT_INPUT_INVALID`, `TEST_IMPACT_RESULT_INVALID`, `TEST_IMPACT_EXECUTION_FAILED`; each maps to exit 2. A provider nonzero takes precedence over its stdout.

Tests: `TestImpactedChanges` (V1 paths/base/binding); `TestImpactedConfig` (V2 validation/round-trips); `TestImpactedFallback` (V3 missing/template/groups, three adapters, argv parity, no implicit Go); `TestImpactedCLI` (V4 exit matrix, list, built/go-run self-resolution, no engine/state/census). Fixtures are temporary Git repositories with shell providers; these tests contain no Go analysis.

## 3. Protocol and tool family

**I1.** U1 publishes `docs/testing-impacted-protocol.md`, complete JSON Schemas and a shell provider example usable with Gradle/Maven. Every field below is required:

```json
{"schemaVersion":1,"mode":"run","base":"<commit-id>","repositoryRoot":"<absolute-path>","changedPaths":["src/a.java"],"binding":"<sha256>"}
```

```json
{"schemaVersion":1,"mode":"run","binding":"<echo>","status":"passed","selections":[{"label":"unit","cwd":".","argv":["./gradlew","test"],"scope":"named","tests":["ExampleTest"],"reasons":[{"test":"ExampleTest","because":"changed source"}],"result":"passed","seconds":1.2}],"uncertainty":[]}
```

Paths/cwd are repository-relative regardless of provider cwd. Request mode is run/list. Result statuses: passed, failed, incomplete, listed, nothing-selected; not-run is reserved for the wrapper's no-groups fallback. Selection scope is named/package/module/check; result is passed/failed/incomplete/not-run. Labels are unique/nonempty, argv nonempty, seconds finite/nonnegative. Named selections require names and a reason for each; broader/check selections require a `test:"*"` reason. Uncertainty entries are `{paths:[],because:"...",action:"..."}` with nonempty explanation/action.

Passed requires nonempty selections all passed. Failed requires a failed selection; incomplete requires incomplete selections and no failures. Blocked commands may be not-run with an explanation. Empty selections require nothing-selected (run) or listed (list), except wrapper not-run. List mode permits only listed, except the wrapper no-groups not-run outcome; its selections are not-run with zero seconds. Run mode forbids listed. Binding/mode/version must match. Nothing-selected is loudly reported, not fabricated passing evidence. Go always emits its guards.

Stdin contains exactly one UTF-8 JSON document, then closes. Stdout contains exactly one result plus optional whitespace; logs belong on stderr. Reject missing/unknown/duplicate fields, trailing text/documents, wrong types and inconsistent outcomes. Negotiation is exact: caller sends version 1; unsupported providers exit nonzero with diagnostics, without downgrade.

Provider exit 0 means a valid result, including failed/incomplete; every nonzero means no result and wrapper exit 2, `TEST_IMPACT_EXECUTION_FAILED`. Tests failing normally must therefore emit failed JSON and exit 0. Environment is inherited; no TTY. No implicit execution deadline: caller cancellation terminates the child's process group with SIGTERM, then SIGKILL after two seconds, without process-list reaping. Apply the same lifecycle to fallback commands. Go child commands preserve caches and set GOPROXY=off, GOSUMDB=off and GOTOOLCHAIN=local; missing dependencies are incomplete. External providers must document and satisfy the same offline sandbox contract. Inject the grace clock in tests.

`TestImpactProtocol` checks every status/exit/stream boundary and malformed schema; `TestExternalProvider` runs the documented example, cancellation and output separation.

**I2.** `metasystem tool go-impacted` is a thin handler in the existing binary. Names follow `<language-or-domain>-<operation>`. Tool verbs consume protocol stdin; there are no selection switches. CLI help and verb docs list “implementation verbs that project config calls, not agents.” Builder instruction audits forbid direct calls; maintenance replay below is an explicit diagnostic use. `TestToolBoundary` checks help and dependency direction.

## 4. Go selection and execution

Stdlib only; resolution is syntactic, with no go/types engine.

**G1.** Inventory using offline `go list -e -json ./...`; include external tests and excluded/tagged source. Parse base and candidate once each, using batched Git reads. Compare declarations by receiver/name and normalized source; retain directives and every comment inside Example functions, including Output assertions. Added/removed declarations and changed test helpers are seeds; grouped const/iota changes seed the entire group. Removed tests are not executable; report lost coverage and follow their old references. Compare both reference graphs. An import binding change seeds declarations using that binding even if their source is unchanged.

**G2.** Reverse syntactic references cross production helpers, test helpers and imports to Test/Fuzz/Example roots. Traverse cycles once; include same-package, external-package and transitive importer tests. Resolve import aliases; dot imports and name collisions conservatively add all possible edges. Any method change also changes its receiver's base type; embedding a changed type changes the embedding type. Every `.Name` is a possible changed-method reference, including unexported methods within their package. Receiver-type references carry implicit String/Error/MarshalJSON/interface calls through factories and helpers.

TestMain, init and package-variable initializer expressions are package roots. Reaching TestMain selects the whole package, including external tests. Reaching init/initializer also selects every package whose test binary links that package. Determine links from production and test imports in both graphs. Reverse-import-closure broadening is otherwise forbidden. An import-list edit alone is not a broadening trigger; added imports with init/variable initializers broaden only the importing package.

Unparseable/unlistable files, unresolved dynamic references, reflection, cgo/assembly or unsupported constraints broaden the affected package and report uncertainty. Unknown Go ownership broadens the module. Package-clause/build-directive changes broaden affected packages. These are explicit over-approximations; never use uncertain name binding to prove absence.

**G3.** A non-Go path, basename or containing-directory literal/embed match seeds all declarations in its matching file, then G2. Collisions overselect. Without matches, files within a Go package directory, including testdata descendants, select that package whole. Without package ownership (docs/plans), select only guards and report `no Go reference`. Only go.mod/go.sum/go.work among non-Go changes force the module. testing.json follows its literal references.

Always select `internal/testenv.TestEveryPackageUsesSharedMain` and `cmd/metasystem.TestStandardStreamPipesUseSharedCapture`; a missing guard is a failed check.

**G4.** Run plain, then one run per custom tag found in selected packages' production/test files, lexically ordered. A constraint combining multiple custom tags broadens its package with uncertainty; combinations not exercised remain incomplete, never reported covered. Host-incompatible configurations are likewise incomplete. No boolean configuration enumeration is built.

**G5.** Sequential order: changed existing Go files through `gofmt -l` (output fails), `go build ./...`, `go vet ./...`, then sorted package/tag commands. Named selection uses `go test -json -count=1 -run '^(A|B)$' ./pkg` with escaped names; whole selections omit -run. Merge duplicate guards/selections. Fuzz seeds and runnable Examples use the normal runner. Require selected names in Go JSON events; missing/skipped names are incomplete. Continue independent failures; blocked checks are not-run with reasons. A whole cmd/metasystem run prevented by sandbox facilities, including its socket panic, is incomplete, never a test failure or pass; preserve observed assertion failures separately. No process scan or sandbox probing is needed: classify runner diagnostics.

Fixtures: `TestGoDeclarations` (G1 changes/deletions, Example Output-only); `TestGoClosure` (G2 production/test-helper cycles, aliases/importers, unrelated neighbour excluded, unexported method/wrapper, String via fmt.Sprint helper, embedding); `TestGoPackageRoots` (testenv.Main through TestMain, PlainExec through init registration, freshBaseDiagnosis through initializer/registry); `TestGoFiles` (G3 literals/embed/testdata/docs/module); `TestGoExecution` (G4–G5 tags/conjunction, guards, order/events, sandbox incomplete). Each scenario must fail when its rule is removed.

**G6.** Critic's host measurements: 92 packages, 90 with tests; warm `go list -e ./...` took 0.51 seconds, JSON inventory 0.25 seconds. These are cited measurements, not rerun here. Analysis caches each file version and visits each reverse-reference edge once for the union of seeds; root linking adds at most P² package edges (P=92). No per-test subprocess runs during list; tag runs grow linearly, without configuration enumeration.

Build acceptance: warm list analysis under 30 seconds, ordinary targeted execution under five minutes. Full fallback/explicit uncertainty is reported separately. Measure:

```sh
time go run ./cmd/metasystem test impacted --base HEAD --list --json
impact_bin=$(mktemp)
go build -o "$impact_bin" ./cmd/metasystem
bash scripts/agents/impacted-replay.sh --binary "$impact_bin" --tip main --count 50 --max-broad-share 0.20
```

U8 supplies the replay script: select the newest 50 landed `Goal-Unit` trailer commits reachable from main, excluding merge/plan/read commits; retain their IDs. Materialize each candidate and first parent in temporary checkouts; send v1 list requests directly to the current binary's Go tool, with that parent as base and exact changed paths/content binding. Do not use historical provider config. Record every result, timing and broadening reason. Count commits with any package/module selection: at most 10/50 (20%). Require all 50; no cherry-picking or silent omissions. Errors fail acceptance. `TestImpactReplay` tests counting and `TestGoAnalysisWork` bounds parse/visit counters. Actual replay and timing remain unverified until build; exceeding either bound blocks build acceptance, not routine execution.

## 5. Builders, proof and cleanup

**B1.** Coexist with testpolicy.Select, surfaces/groups, test plan/run, orchestrator and batch proof. Reuse only argv construction/validation. No engine path calls impacted now. `TestImpactImportBoundary` asserts proofrun/testpolicy do not import testimpact and public selection does not import goimpact.

**B2.** Replace ImpactedTestsRule verbatim in generated briefs/template:

> Before you return, run `metasystem test impacted --json` from the workspace and return its JSON envelope and summary. Project configuration owns selection; agents call only this verb. The orchestrator's proof decides delivery.

TestingRequirement and brief.md append this fixed line:

> In this repository run it as `go run ./cmd/metasystem test impacted --json`, and also run `scripts/agents/go-gate.sh --fast`.

Extend `TestBuildBriefsCarryTheImpactedTestsRule` to assert both unchanged literal strings. Builder evidence `observed` carries the envelope under existing `{command,observed,level}`; a static return-schema check requires its presence/schema, accepting failed/incomplete/not-run outcomes as information. It never certifies testing success or gates prove-round. `TestBuilderImpactReturn` covers this distinction. No pre-proof evidence gate is built.

**B3.** Replace obsolete manual grep/reference/tag/count recipes in exactly `internal/dispatch/build.go`, `scripts/agents/templates/brief.md`, `docs/orchestration.md`. Keep go-gate and risk testing. `TestNoManualImpactRecipe` checks these sources. Activate testing.json only after engine and seat binaries are rebuilt from main containing V2; verify old-host decoding is avoided before landing activation.

## Later

- Pre-proof evidence gate (r1 U18): host proof already decides; any future check validates attribution, never status.
- Whole-worktree snapshot: exclusive builder custody makes changed-path binding sufficient for feedback.
- Full build-configuration enumeration: single tags cover current custom-tag use; complex cases remain explicitly incomplete.
- Additional repetition policy: count=1 plus host proof suffices for this bounded build.
- Engine consumption: no current delivery consumer needs the feedback protocol.

## Critique record

1. FOLDED — §4 G2/G3/G5/G6: narrowed triggers, sandbox incomplete, 50-commit/20% acceptance.
2. FOLDED — §2 V3: template/missing/no-groups fallback, exit 0 without passing evidence.
3. FOLDED — §5 B2, Later: static return check only; U18 deferred.
4. FOLDED — §4 G2: all three package roots and named fixtures.
5. FOLDED — §4 G1/G2: syntactic methods/types/embedding and Example comments.
6. FOLDED — §5 B2: literal rule, repository launcher/gate, extended pinned test.
7. FOLDED — §3 I1: provider exits, empty selections, environment/cancellation contract.
8. FOLDED — §§2–4, Later, units: argv reuse, reduced hashing/tags; eight units overrides critic's approximate ten per seat ruling.
9. FOLDED — §5 B3, U8: rebuild host binaries before config activation.
10. FOLDED — §4 G6: measured inventory; exponential enumeration removed.
11. FOLDED — §5 B1: import assertion replaces unsupported negative test.
12. FOLDED — units: fallback independent; inventory depends only on protocol. Landing order remains seat-mandated.
13. FOLDED — §5 B3: three exact deletion targets.

## Build obligations

All are HIGH and PARTIAL: behavior is specified, implementation proof remains. The units below own completion; no runtime proof is claimed by this design fold.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
|---|---|---|---|---|---|---|---|---|---|
| IMP-V | HIGH | §2 V1–V4 | neutral invocation, honest fallback | testimpact | cmd/metasystem/test_impacted.go; contract; proofrun argv export | TestImpactedChanges/Config/Fallback/CLI | shell-provider and no-groups sandbox CLI | PARTIAL | U2/U3 |
| IMP-I | HIGH | §3 I1–I2 | external protocol and lifecycle | testimpact | protocol types; tool registration | TestImpactProtocol, TestExternalProvider, TestToolBoundary | documented provider, cancellation | PARTIAL | U1/U3/U4 |
| IMP-G | HIGH | §4 G1–G3 | conservative selection | goimpact | inventory and reference graph | TestGoDeclarations/Closure/PackageRoots/Files | tiny module, three package-root shapes | PARTIAL | U4–U6 |
| IMP-E | HIGH | §4 G4–G6 | observable execution and bounded cost | goimpact | runner; replay script | TestGoExecution/AnalysisWork, TestImpactReplay | tags, sandbox incomplete, 50-commit replay | PARTIAL | U7/U8 |
| IMP-B | HIGH | §5 B1–B3 | adoption without proof gate | dispatch | return schema; brief/orchestration docs | TestBuilderImpactReturn, TestImpactImportBoundary, pinned brief test | builder return, rebuilt-host activation | PARTIAL | U8 |

## 6. Units and completion

Every unit includes its tests/docs/registration within the estimate; total 3,200 lines, additions plus deletions. Landing order is row order. U1's documented protocol is usable by external implementers; U2's fallback is callable without an engine and returns raw command outcomes; U3 maps them into I1 and exposes the complete public verb. Thus U2 needs neither the protocol nor the impacted config member. U4 registers a runnable coarse tool: changed packages whole, package-root propagation, literal matches and guards; it conservatively uses every declaration in changed files as a seed. Importer references propagate through production/test helpers from this first cut. U5/U6 narrow only after fixtures prove coverage. Until precision lands, U4 uses package-wide possible-reference edges, including receiver/interface uses, and broadens reachable package roots; every such over-selection is explained as coarse analysis. Until U7, unexercised tags and unchecked execution events yield incomplete, not passed. Final performance acceptance belongs to U8. No feature flags or dormant public verbs.

New tests start with t.Parallel(); tested packages use testenv.Main. Builders list/register new tests, use injected clocks, and never sleep. Completion requires focused fixtures, real tiny-module and external-provider sandbox runs, replay bounds, code critique, and the design-obligation check. Fold verification ran: all 13 critique IDs join their dispositions; eight allocations total 3,200 lines; word ceiling, final-table shape and whitespace checks pass. The repository design validator could not run because bin/metasystem is absent; goal next is also unavailable because metasystem is not installed on PATH. Only this page changed. No implementation or runtime acceptance was run; the proof rows remain PARTIAL for builders.

| Unit | Lines | Rules | Depends on |
|---|---:|---|---|
| U1 protocol types, validation and external docs/example | 400 | I1; protocol fixtures | none |
| U2 export unchanged argv construction; fallback executor | 400 | V3; adapter/no-groups parity fixtures | none |
| U3 contract member, paths/binding, public verb and lifecycle | 400 | V1/V2/V4, I1; CLI/config/sandbox fixtures | U1, U2 |
| U4 runnable coarse Go tool, inventory/reference roots/literals | 400 | I2, G1–G3 coarse, G5 basic runner; root/file fixtures | U1; lands after U3 |
| U5 declaration diff, same-package/helper precision | 400 | G1/G2; declaration/cycle/Example fixtures | U4 |
| U6 importer/method/type precision and uncertainty | 400 | G2; wrapper/interface/embedding fixtures | U5 |
| U7 tags, ordered checks and observed execution outcomes | 400 | G4/G5; tags/events/incomplete fixtures | U6 |
| U8 replay acceptance, builder returns/docs, config activation after engine/seat rebuild from U3 main | 400 | G6, B1–B3; replay/return/import/brief checks | U3, U7 |
