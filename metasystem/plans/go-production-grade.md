# Go Production Grade

Review findings and staged implementation plan, 2026-08-11. Scope: the whole Go module (`internal/`, `cmd/metasystem`, 33,012 non-test lines plus 16,177 test lines, ran it) reviewed against Go best practices, the repository's own design standard (`docs/design/design-principles.md`: responsibility-driven and domain-driven design), and production-grade expectations — package and file sizes, naming, patterns, test coverage, tooling, and mac/Linux portability. The Linux port itself is already specified in `plans/linux-portability.md`; this plan incorporates it as Phase 1 and does not repeat its detail.

This file is written to be read on its own. Evidence for every claim is marked **ran it**, **read it**, or **inferred it**, per `docs/collaboration.md`. Every refactor phase below is behavior-preserving under `skills/refactor/SKILL.md` discipline unless explicitly marked otherwise.

## Verdict

The codebase is closer to production grade than the request implies. Formatting and vet are clean, staticcheck finds only 9 issues in 33k lines, the full suite passes under the race detector, the import graph is a strict layered DAG with no dumping-ground packages, and the command layer is a clean table-driven verb router (all ran it). The real distance to "fully adhering" is five things:

1. **It does not build on Linux.** One package (`internal/identity`) is Darwin-only and sits under everything. The port surface is small and fully specified in `plans/linux-portability.md`.
2. **A handful of oversized units.** One 1,472-line file whose sixty declarations all carry a disambiguating prefix (the language's signal that it wants to be its own package), and about twenty functions over 100 lines.
3. **Domain state rides in raw maps.** Over 1,100 `map[string]any` sites (ran it); the design standard's own smell list names this. Fixing it is the largest and riskiest item, because the maps are also the on-disk JSON contract.
4. **Claims without enforcement.** The gate's comment claims a coverage floor that no code checks; coverage actually spans 54–95% across domain packages.
5. **Small naming and idiom debt.** Redundant file prefixes, two `util` files, two wrongly-named error variables, one grab-bag helper file, and 401 unwrapped `fmt.Errorf` calls.

Nothing found contradicts the shell layer's contracts. The dangerous work is Phase 4 (typed documents); everything else is mechanical or additive.

## What already holds

Recorded so the plan does not "fix" what is not broken (all ran it, 2026-08-11):

- `gofmt -l` clean; `go vet ./...` clean; staticcheck 2025.1 reports 9 findings total.
- Full suite green: `go test -count=1 ./...` passes; the gate (`scripts/agents/go-gate.sh`) already runs gofmt, vet, race-detector tests, and the stamped build.
- Import graph: 22 `internal` packages in a five-layer DAG, no cycles, no `util`/`common`/`misc` package, domain-language names throughout (`lease`, `census`, `custody`, `fence`, `mission` — glossary-backed).
- `cmd/metasystem` routes git-style family/verb via one declarative table (`main.go:27`); verbs own their flags and exit codes; main only routes.
- Eight interfaces, all small and consumer-side (`identity.Prober`, `census.ProcTree`, `janitor.World`, `validate.ProcessTree`, four in `supervise/owner.go`) — idiomatic Go.
- No branching on error strings anywhere; no `init()`; no `os.Exit` outside main; exactly one `panic`, justified (CSPRNG failure, `internal/missionrunner/engine.go:114`).
- Every package has a package doc; comments are dense and contract-focused; `t.Helper()` appears in 115 test helpers; receiver names are short and consistent; no `GetX` getters.
- Only one background goroutine outside tests (`missionrunner/host.go:35`), the standard start-then-wait process pattern, correctly closed over a done channel.

## Findings

Numbered so phases can reference them. File:line evidence is as of 2026-08-11.

### Portability (P)

- **P1 — Darwin-only identity.** `internal/identity` implements `KernelProber` only under `//go:build darwin`; `CGO_ENABLED=0 GOOS=linux go build ./...` fails there and 13 packages fail behind it (ran it). Port surface, semantics tables, and traps: `plans/linux-portability.md`.
- **P2 — CGO not pinned.** `scripts/adopt.sh:240` and the `go build` in `scripts/agents/go-gate.sh` use the environment default (read it). Static-binary portability across Linux distributions is currently accidental.
- **P3 — No Linux signal in the gate.** Nothing cross-compiles for Linux, so a Darwin-only regression is invisible until someone tries (ran it — the current failure proves it).
- **P4 — Over-narrow build tag.** `internal/census/ancestor_production.go` is Darwin-gated but contains nothing Darwin-specific (read it; detailed in the portability ledger).
- **P5 — Two syscall dialects.** Eight files import legacy `syscall` while the rest of the module uses `golang.org/x/sys/unix` (ran it). Every symbol used (`SIGTERM`, `Kill`, `Flock`, `SysProcAttr`, `Wait4`/`WNOHANG`, `ENOTEMPTY`, `EEXIST`) exists on both platforms, so this is consistency debt, not a port blocker.
- **P6 — `ps` bypasses the identity owner.** `internal/dispatch/mission.go:81` and `internal/mission/contract.go:1339` exec `ps -p <pid> -o command=` — the identical line duplicated in two packages (read it). The flags are POSIX and work on Linux, but the module built `internal/identity` precisely so process facts have one kernel-backed owner; these two sites re-derive the same fact through an external tool that fixtures cannot fake the same way.
- **P7 — Filename convention hazard.** `identity_darwin.go` is deliberate, but Go silently build-gates *any* file ending in a GOOS/GOARCH name. The `adapter_<vendor>.go` pattern would silently drop an adapter for a vendor named `js`, `wasm`, or `plan9`. No current file is affected (ran it — full scan). Fixed as a side effect of S5.

### Structure and size (S)

- **S1 — `mission/contract.go` is a package trapped in a file.** 1,472 lines; roughly sixty top-level declarations, nearly all prefixed `contract` (`contractRead`, `contractGlobRegex`, `contractPathWithin`, `contractDirExists`, …) purely to avoid collisions inside `package mission` (read it). In Go, that prefix pattern is the namespace asking to become a package: as `internal/contract` (or `mission/contract`), the prefixes drop and the seal/verify/gate machinery gets its own boundary and doc. `mission/envelope.go` (`DispatchEnvelopeAllows`) calls `contractRead` and the verify helpers directly, so it moves with the extraction or calls the new package's exported surface.
- **S2 — Functions over ~100 lines** (ran it — scan; `main.go`'s `families()` at 329 lines is exempt, it is a declarative table):
  `internal/config/validate.go:19 Validate` (322), `internal/validate/turnprompt.go:61 TurnPrompt` (254), `internal/missionrunner/loop.go:887 (*Engine).oneCycle` (239, in a 951-line file), `internal/dispatch/mirror.go:20 Mirror` (179), `internal/missionrunner/host.go:167 launchHost` (175), `internal/mission/prompt.go:371 AssemblePrompt` (171), `internal/dispatch/critique.go:186 CritiqueExhaustionAction` (155), `cmd/metasystem/supervise_owner.go:28` (130), `internal/missionrunner/launch.go:321` (123), `internal/validate/conftailor.go:33 TailorConf` (120), `internal/registry/records.go:96 ParseRecord` (117), `internal/dispatch/handshake.go:16 HandshakeEval` (117), `internal/missionrunner/answer.go:17 Answer` (115), `internal/missionrunner/adjudicate.go:238 Adjudicate` (114), `internal/capability/select.go:44 Select` (111), `internal/host/fake.go:12 FakeReturn` (110), `internal/supervise/owner.go:211 (*Owner).Cycle` (105), `cmd/metasystem/supervise_component.go:33` (103), `internal/mission/landed.go:40 LandedReturns` (100), `internal/mission/contract.go:303 validate` (100). Length alone is a heuristic, not a violation — but most of these mix two or three abstraction levels, which is the design standard's actual rule ("the main path reads top to bottom at one level of abstraction").
- **S3 — `util` files in cmd.** `cmd/metasystem/util.go` and `util_hold.go` (read it) — the design standard names `util` a dumping ground for code whose owner was never decided (`design-principles.md:60`). The content has obvious owners: slug derivation and hold-loop verbs.
- **S4 — `dispatch/jsonutil.go` grab-bag.** Self-described as "shared JSON, number, path, and digest helpers" (read it) — four responsibilities in one file inside one package. Package-internal, so mild; dissolve into owners or split by concept.
- **S5 — Redundant file prefixes in `adapter`.** `adapter_claude.go`, `adapter_codex.go`, … repeat the directory name (17 files); `internal/host` already does it right (`claude.go`, `devin.go`, `fake.go`). Renaming also retires the P7 hazard pattern.
- **S6 — One-symbol cross-package imports.** `internal/validate` imports all of `missionrunner` for one vocabulary table (`PromptAskReasons`, ran it); `internal/dispatch` imports `supervise` for one value (`CapExpired`, ran it). Both are ownership questions: the ask-reason vocabulary is part of the turn contract, not the runner engine; the cap-expiry value's owner should be whichever package defines the cap lifecycle.

### Responsibility- and domain-driven design (D)

- **D1 — Domain state as raw maps.** 1,100+ `map[string]any` occurrences in non-test code; the heaviest are exactly the domain cores: missionrunner 354, mission 164, dispatch 151, adapter 132, registry 93 (ran it). Job records, turns, and mission state flow untyped through the very functions in S2 — `oneCycle(statePath, ledger string, state map[string]any, …)`. The design standard's smell list names this twice ("important behavior rides in raw maps, untyped payloads"; "prefer rich domain types"). `internal/registry` already shows the house pattern — a typed `Record` with `ParseRecord` validation — and the rest of the module never adopted it. **The constraint that makes this dangerous:** these maps are also the on-disk JSON contract shared with the shell layer, some records are content-hashed (sha256 in dispatch, ran it), and record-cas patches arbitrary keys — so any typed representation must preserve unknown keys and must not perturb canonical bytes. That is why this is its own late phase with a design gate, not a rename.
- **D2 — One word, two meanings.** "Envelope" is both the mission contract's signed pre-authorization (`envelope.<category>`, `docs/project-rules.md`) and the dispatch permissions document (`dispatch/envelope.go ExpandPermissions`, read it). Bounded-context collision in the glossary's own terms; one of them should be renamed (the permissions document is the newer, less entrenched usage).
- **D3 — Exported mutable vocabulary tables.** `missionrunner.KnownAskReasons`, `PromptAskReasons`, `TerminalJobStatuses`, `LegalStreamTransitions` are exported `map[string]bool` package variables (read it) — mutable shared state across package boundaries. Go cannot make maps const; the idiom is unexported maps behind lookup functions, or moving them to the vocabulary's owner (with S6).

### Errors (E)

- **E1 — Wrapping is the exception.** 401 of 523 non-test `fmt.Errorf` calls use `%v`/none, not `%w`; one sentinel error; 15 `errors.Is/As` sites (ran it). For a CLI whose verbs print and exit, opaque errors are mostly fine — and **error text is fixture-asserted contract**: the shell suite's `agent_fails` assertions match stderr substrings that originate in Go code — `illegal job transition`, `capability snapshot is stale`, `escapes the job worktree`, and more (ran it — cross-greped the suite's asserted patterns against `internal/`). So: adopt `%w` at boundaries where a caller actually inspects, and never blanket-rewrite error strings.
- **E2 — The nine staticcheck findings** (ran it, staticcheck 2025.1): SA4006 dead assignment `internal/config/validate.go:300`; SA1006 dynamic-format printf ×5 `internal/dispatch/critique.go:145,151,249,277,315`; ST1012 error vars `OwnerLockBusy`/`OwnerLockNotOwner` should be `ErrOwnerLockBusy`/`ErrOwnerLockNotOwner` `internal/dispatch/ownerlock.go:27-28`; ST1005 error string ends with newline `internal/lease/lock.go:47`. The ST1005 fix changes an error string — check the fixture suite for a matching assertion first (E1).

### Unwired code (U)

- **U1 — `internal/janitor` has no importer.** 505 non-test lines at 93.8% coverage, imported by nothing (ran it — grep and `deadcode ./cmd/...`). Behind it, most of `internal/registry`'s frame/reduce/compact machinery and `lock.Holder` are unreachable from the binary. This is the staged KI-32 work (`plans/supervision-lifecycle.md` names an orphan janitor; read it) — **not** deletion candidates. The gap is that nothing marks it staged: a reader (or a deadcode gate) cannot tell parked-by-design from forgotten. Wiring it belongs to the supervision-lifecycle stream, not this plan.

### Tests and gate (T)

- **T1 — The coverage floor is prose.** `go-gate.sh:53` says "the domain packages carry a coverage floor (the engineering standard)" but nothing enforces one (read it). Actual coverage (ran it): cmd 3.5% (thin wiring, exercised end-to-end by the shell fixtures), missionrunner 54.4%, validate 60.1%, dispatch 66.8%, identity 68.1%, mission 69.1%, census 74.2%, everything else 73–95%. The repo's own rule: machine-verifiable requirements live in scripts, never in repeated prose.
- **T2 — Linux enumeration would ship untested.** `enumerate_darwin_test.go` is Darwin-gated; the portability ledger specifies the untagged cross-platform contract test to add.
- **T3 — White-box only, serial only.** Every test is in-package (no `package x_test` anywhere) and `t.Parallel()` is never used (ran it). In-package testing is acceptable Go; the serial choice looks deliberate (fixtures communicate through `METASYSTEM_*` env vars, which are process-global) — leave it, and record why.
- **T4 — Clock seams are inconsistent.** `gaterun` injects `var clock = time.Now` (read it); everything else calls `time.Now` inline. Minor; align opportunistically during Phase 3 extractions, not as a sweep.
- **T5 — One snake_case function.** `strings_join`, `internal/config/conf_test.go:38` (ran it). Trivial rename.

### Tooling (G)

- **G1 — Staticcheck is not in the gate.** It caught real issues (E2) and is the community-standard analyzer beyond vet. Adding it means a pinned tool dependency (network at gate time, or a vendored tools module) — dependency additions are human-reserved (`docs/project-rules.md`).
- **G2 — No vulnerability scan.** `govulncheck` is the standard; same dependency/network caveat as G1.
- **G3 — `go.mod` pins a patch version** (`go 1.26.5`, read it). Legal and enforceable; just know it forces that exact minimum on every builder. No change proposed — recorded so nobody "fixes" it casually either way.

## Acceptance discipline

Every phase runs under `skills/refactor/SKILL.md`:

- **Acceptance gate:** `scripts/validate-metasystem.sh` (the full local suite, which sources `go-gate.sh` — gofmt, vet, race tests, build — ahead of the shell fixtures). Per KI-10, the local suite is the gate; no hosted runner is assumed.
- **Baseline:** before the first edit, `scripts/refactor-baseline.sh record --gate "scripts/validate-metasystem.sh"` on a green run. Re-record at each phase acceptance.
- **Checkpoints:** each lettered step below is one commit that can be replayed alone; a phase is one cluster accepted by the full gate.
- **Tests before restructuring:** any unit under Phase 3–4 whose material behavior lacks focused tests gets coverage hardening as its first checkpoint (this binds hardest in missionrunner, at 54.4%).
- **Error text is contract** (E1): any change that touches an error string greps `scripts/` for a matching fixture assertion first.
- **On-disk bytes are contract** (D1, and the port memory's standing rule): no phase may change the JSON written to or read from `artifacts/`, `plans/`, or the registry, except where a step explicitly proves byte-compatibility.

## Phases

Ordered so that cheap, standing guards land first and the riskiest change lands last, behind its own design gate. Phases 2 and 3 are independent of each other; Phase 4 wants 3 first (extractions shrink the surface the typing touches).

### Phase 0 — Pin the guarantees the code already has

Small, self-contained, do first.

- a. Pin `CGO_ENABLED=0` in `scripts/adopt.sh:240` and the gate build (P2).
- b. Fix the nine staticcheck findings (E2). The ST1012 rename is internal-only (grep confirms the names appear nowhere in `scripts/`, ran it); the ST1005 newline fix follows the E1 fixture-grep rule.
- c. Make the coverage floor real or delete the claim (T1): add a per-package floor check to `go-gate.sh` set at today's measured coverage rounded down (a ratchet — it can never regress; raising it is Phase 5's job), and exempt `cmd` with the stated reason (fixture-covered wiring).
- d. Add staticcheck and govulncheck to the gate, pinned to exact versions (G1/G2 — approved by the human 2026-08-11: both are standard quality tools). Order them after the fast rejectors (gofmt, vet) and before the race suite; a network-unreachable tool run fails the gate loudly rather than skipping silently.

Verification: full gate green; `staticcheck` clean; a deliberately-lowered coverage number fails the new floor check (prove the check can fail).

### Phase 1 — The Linux port

Execute `plans/linux-portability.md` steps 1–7 as written (its step 1 is Phase 0a here). Summary: `identity_linux.go` mirroring the three-way liveness table exactly, `enumerate_linux.go` (`AllPids`, `ProcessCwd`, `ParentPid`), widen the tag on `census/ancestor_production.go`, the untagged cross-platform contract test (T2), the two resolution-comment updates, and `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...` added to `go-gate.sh` as a standing guard (P3).

Not in this phase: P6 (the `ps` sites) — the port stays minimal; P6 moves in Phase 2.

Verification: cross-compile green in the gate on macOS; full suite run on a real Linux host from a copied tree (the ledger's case-sensitivity trap). The Linux-host run is the phase's acceptance evidence — cross-compiling proves it builds, only execution proves the `/proc` reads.

### Phase 2 — Names and owners (mechanical, one cluster)

- a. `git mv` the `adapter_*.go` files to drop the directory-redundant prefix (S5, retiring P7's pattern); same for `cmd/metasystem/util.go` → `slug.go` and `util_hold.go` → `hold.go` (S3). Pure renames, no code edits.
- b. Rename `strings_join` (T5).
- c. Dissolve `dispatch/jsonutil.go` (S4): JSON canonicalization and digest helpers keep a file named for that concept; path and number helpers move beside their callers.
- d. Route the two `ps` sites through the identity prober and delete the duplicated function (P6). This is the one step in this phase with behavior risk (fixtures fake identity through `METASYSTEM_FAKE_PROCESS_IDENTITY_FILE`, which `ps` never consulted — confirm each site's fixture expectations before switching).
- e. Move the ask-reason vocabulary to its owner (S6, D3): the turn-contract tables leave `missionrunner` for the package that owns turn vocabulary, unexported behind lookup functions where export is not needed. Target package chosen at implementation (candidates: with the turn documents in `mission`, or a small `internal/turn` if extraction order makes that cleaner); `validate`'s import of `missionrunner` disappears. Same treatment for `CapExpired` only if its owner is equally clear — otherwise leave and record why.

Verification: full gate; `go list` shows the validate→missionrunner edge gone; diff of the binary's verb output on a fixture run is empty.

### Phase 3 — Extract the oversized units

Tests-first per the acceptance discipline; behavior identical; one package per checkpoint.

- a. **`mission/contract.go` → its own package** (S1): move the contract document, seal, verify, and gate machinery to `internal/contract` (name settled at implementation), drop the sixty `contract` prefixes, move or re-export what `mission/envelope.go` needs. The seal computes canonical bytes from file content, not code layout (read it — `contractCanonicalSignedBytes`), so the move cannot shift hashes; the contract fixtures prove it.
- b. **`missionrunner` extractions**: `oneCycle` into named cycle steps, `launchHost`/`launch` into assemble-spawn-record steps. Coverage hardening first (54.4% is the module's weakest domain package).
- c. **The remaining S2 list**, largest first, applying the standard's actual test — one abstraction level per function — rather than a line-count quota. Stop where an extraction would only rename complexity without naming a concept (the standard forbids that too).

Verification: full gate per checkpoint cluster; coverage may only move up.

### Phase 4 — Typed domain documents (design-gated; the only phase that may touch bytes)

The D1 work. **Not started until its design passes `docs/design/design-obligation-gate.md`** — and, if the human wants it attacked first, the design-critique skill; the on-disk records are named API surfaces under the project's reserved-decision rules.

Constraints the design must prove, per document family:

- Byte-identical round-trip for every record the shell layer or a content hash reads (golden-file tests: parse → re-serialize → compare bytes).
- Unknown-key preservation wherever `record-cas` or future writers patch fields the type does not know.
- The typed boundary lives at IO (`ParseX`/`WriteX` like `registry.ParseRecord`); interior code works with the type; no half-converted flows where a map and a struct describe the same document in one call chain.

Staging, highest value first: dispatch job records (the CAS/lifecycle core), then missionrunner turn+state, then host results. Adapter/capability leaves stay maps where typing would add nothing — the standard's proportionality rule, stated per family in the design.

### Phase 5 — Raise coverage to a real floor

With the ratchet from Phase 0c in place, raise the weak packages toward the floor the human sets (proposal: 75% for domain packages): missionrunner 54.4 (partly done by Phase 3b hardening), validate 60.1, dispatch 66.8, identity 68.1 (the Linux files arrive tested from Phase 1), mission 69.1. Raise the ratchet as each lands. `cmd` keeps its stated exemption; record T3's serial-tests rationale in a package doc while in the area.

### Phase 6 — Close out

- Promote durable lessons per `wow.md`: the CGO pin and Linux cross-build live in the gate (self-documenting); the GOOS-filename hazard (P7) and the error-text-is-contract rule (E1) go to their canonical owners if judged durable.
- Delete this plan and `plans/linux-portability.md`; move any do-not-retry dead ends to `plans/known-issues.md`.
- U1 stays with the supervision-lifecycle stream; this plan only hands over the deadcode inventory.

## Decisions reserved for the human

1. **G1/G2** — ~~adding staticcheck (and govulncheck) to the gate~~ **Decided 2026-08-11: yes, add both, pinned** (the human confirmed after the review; recorded in Phase 0d).
2. **T1/Phase 5** — the coverage floor number (proposal: ratchet now at measured values, target 75% domain floor).
3. **Phase 4** — approve the typed-documents design before implementation; decide whether it gets a design-critique round first.
4. **D2** — renaming the dispatch permissions document away from "envelope" (touches shell-facing vocabulary; glossary edit).
5. **Phase 1 step 7** — which real Linux host/distribution runs the acceptance suite.

## Verification commands

```bash
# The acceptance gate for every phase.
scripts/validate-metasystem.sh

# Fast signals, in rejection-speed order.
gofmt -l internal cmd
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@2025.1 ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...
go test -count=1 -race -cover ./internal/...

# Unreachable-code inventory (U1 tracking; not a gate).
go run golang.org/x/tools/cmd/deadcode@latest ./cmd/...
```

## Assumptions, gaps and risk

- **Not run:** no refactor has been executed; every phase above is planned, not shipped. No Linux build has ever succeeded (the port is Phase 1). Staticcheck's nine findings are still present.
- **Reviewed statically:** the raw-map count and the long-function scan are mechanical (ran it); the judgment that most long functions mix abstraction levels is from reading the top dozen, not all twenty.
- **Fixture blast radius:** the two riskiest couplings this plan touches are error strings (asserted by the shell suite) and on-disk JSON (hashed and shell-consumed). Both have named rules in the acceptance discipline; a phase that cannot satisfy them stops and escalates rather than bending them.
- **Moving target:** the tree gained files during review (`missionrunner/drain.go`, `mission/landed.go` appeared between scans, ran it). Re-run the mechanical scans at each phase start; the finding numbers name concepts, not line counts.

## Proposed receipt

```
scripts/receipt.sh add --type other --outcome shipped --verify skipped \
  --note "review: plans/go-production-grade.md records the full Go-codebase critique (naming, structure, RDD/DDD, errors, tests, tooling) and a six-phase behavior-preserving plan to production grade plus mac/linux portability. Clean already: gofmt, vet, race suite, layered DAG, table-driven cmd router; 9 staticcheck findings in 33k lines. Real work: linux port (plans/linux-portability.md as phase 1), contract.go package extraction, ~20 functions >100 lines, 1100+ raw-map sites typed behind a design gate, coverage floor made executable (weakest: missionrunner 54.4%). No refactor executed; no Linux build has ever run"
```
