# Metasystem full-system review

Date: 2026-08-12. Scope: everything under `metasystem/` — the Go engine (29 internal
packages plus `cmd/metasystem`, ~26k source lines, ~20k test lines) and the shell layer
(38 scripts, ~15k lines, plus 43 data files under `scripts/`). Generated artifacts under
`artifacts/` were excluded; the code that generates them was not.

The review answered two questions:

1. **Would a senior Go engineer read this engine cold and call it well-designed, solid
   code?** Judged against a nine-point rubric: single responsibility per package, correct
   layering, minimal API surface, error discipline, concurrency ownership, idiom,
   complexity, test quality, documentation.
2. **Do the scripts contain only plumbing?** Every script was classified against a hard
   rule: a line is *plumbing* if it is environment setup, path resolution, argument
   forwarding to `metasystem <family> <verb>`, exit-code propagation, or trivial glue.
   Anything that branches on domain state, parses or produces domain JSON, implements
   retry/timeout/selection policy, or computes a value the Go side also understands is
   *logic* and must move to Go.

## Verdict

**The scripts are the gap.** All five script clusters fail the bar. Of 38 shell scripts,
7 are pure plumbing, 23 are plumbing with leaks, and 8 contain substantial logic that
must move to Go: `validate-metasystem.sh` (4,409 lines, three shadow implementations of
Go-owned computations), `dispatch.sh` (roster/tier/escalation policy in shell),
`watch-background-jobs.sh` (the watchdog's whole classification engine),
`runtime-common.sh` (the terminal-outcome state machine for every adapter turn),
`refactor-baseline.sh` (the last policy gate still deciding in shell),
`return-schema-fixtures.sh`, `validate-skill.sh`, and `fixture-budget.sh` (the last one
stays until the fixtures it serves are retired). On top of the boundary leaks, roughly
4.5k lines of shell fixtures now duplicate coverage the Go test suite already owns —
migration debt from the port, not new rot.

**The Go engine is genuinely strong, with specific debts.** Six of eight clusters pass
the senior-engineer bar outright. The reviewers' consistent note: package docs are
exceptional, tests are behavior-driven with real fault injection, the dependency graph is
acyclic and coherently layered, and `go vet`, `gofmt`, and `staticcheck` are all clean.
The two failing clusters fail for concrete, fixable reasons: the adapter/host pair was
ported twice (the Devin spend-metering algorithm exists as two verbatim ~100-line
copies), and `cmd/metasystem` carries real supervision policy in the one package exempt
from the coverage ratchet. Beyond those, the review found a small set of high-severity
correctness defects — most of them places where the code fails open in a tree whose
doctrine is fail-closed — listed as workstream W1 below.

**No criticals in the original sweep — one critical-shaped amendment from critique.**
Round 1 of the Codex critique surfaced the closest thing to a critical this review
holds: evidence GC can permanently discard a chain's closure state, on a path that runs
at every turn end (codex-1). It leads W1 now. The other high-severity items are latent:
they fire under crash, permission failure, pid reuse, concurrent landing, or
multi-checkout setups. For adopted repositories one is not latent at all — the shipped
CI workflow is red on day one (script-misc-1) — so the adopted-engine-delivery ruling
is the first decision to make, ahead of any W4 work.

## The numbers

The review ran as 14 scoped reviewers (8 Go clusters, 5 script groups, 1 whole-system
architecture pass), each followed by an adversarial verifier that re-read the cited code
and tried to kill every finding. 187 findings went in; 4 were killed (Appendix A), 15
were downgraded with reasons preserved inline, none were upgraded.

The drafted report then went through the Codex critique loop (`gpt-5.6-sol` at xhigh;
every round recorded verbatim, with adjudications, in `2026-08-12-critique-trail.md`).
Round 1 added six findings (codex-1 … codex-6, stated in full at the end of Part 1),
expanded four existing ones, revised two fix targets, and downgraded the
triple-reported usage duplication to medium. Every accepted point was verified against
the tree before acceptance. Round 2 tightened four fix specifications (the GC hash
guard is mandatory, classification aborts on any walk uncertainty, ceiling-count
uncertainty maps to Indeterminable, gate/guard worktrees stay independent), expanded
the external-call inventory to ten sites across three packages round 1 never swept,
folded every adjudicated target into the canonical finding text, and moved the test
seams into the fixes they prove — no new findings, no severity changes. The table
below includes all of it. The loop converged in round 3: Codex's verdict on the
revised report was AGREE, with no remaining material points on either side.

| | high | medium | low | total |
|---|---|---|---|---|
| Go application (Part 1) | 13 | 44 | 48 | 105 |
| Scripts (Part 2) | 8 | 41 | 25 | 74 |
| Architecture and docs (Part 3) | 2 | 5 | 3 | 10 |
| **Total after critique round 1** | **23** | **90** | **76** | **189** |

A handful of findings were reported independently by more than one reviewer (the Devin
usage duplication three times — a good sign the verification net works). They are
cross-referenced in place and counted once in the backlog; roughly 184 distinct issues.

Sixteen findings would touch an on-disk format, a shipped file set, or the CLI verb
surface. None of those may be executed without sign-off; they are collected in **Decisions
needed** below, together with the ~14 new verbs the script migrations propose.

Baseline tooling: `go vet`, `gofmt -l`, and `staticcheck` pass with zero findings across
all packages. Neither `shellcheck` nor `golangci-lint` is part of the toolchain; two
toolchain recommendations are included in the backlog (W5.9, W3.7) on that basis — those
two are from the baseline pass, not from a reviewer.

## How to read the findings

Every finding states **Current** (what the code does, with file:line), **Target** (the
end state), and **Why** (the failure it prevents), plus severity, effort, and a sign-off
flag. Severity: *high* = clear violation of the target standard, must change; *medium* =
should change, materially improves correctness or maintainability; *low* = polish.
Effort: S = under an hour, M = part of a day, L = a day or more. Where the adversarial
verifier downgraded a finding, its note is preserved as a quoted block — those notes
often contain the sharpest analysis in this report and several correct the original
finding's facts. Read them before executing an item.

## Execution backlog

Ordered for execution. W1 is pure correctness and contract-free: it can start
immediately. W4–W6 mostly need verbs signed off first. W7 can run in parallel with
anything. Within a workstream, items are in intended order.

### W1 — Correctness: fail-closed and proof-discipline fixes (Go first, then shell)

The theme: the tree's doctrine is "refuse loudly, never skip silently, indeterminacy
authorizes nothing" — and these are the places the code contradicts it. Items marked
[R1.n] were added or expanded by round 1 of the Codex critique; the added findings are
stated in full at the end of Part 1.

1. codex-1 (high/M) — evidence GC can permanently discard a chain's closure state: the sourceStateHash equality guard in pruneMirroredRecords is mandatory; a post-close re-mirror is a durability supplement, never the alternative (a failed re-mirror leaves the stale manifest and GC still prunes). [R1.1, R2.1]
2. lease-census-1, expanded (high/M) — classification fail-open: distinguish absence from failure for supervision state and job records, treat corrupt records like corrupt state, and abort classification on any identity/ancestry uncertainty — a skipped unreadable ancestor escalates to MAIN, not just HUMAN (codex-3). Seam and refusal tests land in the same change. [R1.3, R2.2, R2.6]
3. lease-census-2, expanded (high/M) — the sweep: only IsNotExist means vanished; unparseable records are hard errors; schema-invalid records (missing or noninteger claimEpoch, unknown status) are hard errors; in groupOwnsTag only ESRCH counts as member absence — any other inspection error yields unprovable. Seam and refusal tests land in the same change. [R1.4, R2.6]
4. codex-2 (high/S) — owner-lock takeover on unreadable argv: holderState treats Alive+ArgvKnown=false as busy, ideally by delegating to identity.Custodian. Prerequisite for W4.1. [R1.2]
5. codex-4 (high/S) — the arming blocker scans fail open in cmd: tolerate only ENOENT; any other read, parse, or schema failure refuses arming. Fix in place first; W3.1 relocates the fixed code. [R1.6]
6. dispatch-supervise-3 (high/S) — census-lock takeover on Unknown liveness; with codex-2, the two remaining three-way violations in the tree.
7. mission-contract-1 + cli-4 (high/S) — the Dir^3 root-discovery off-by-one, both copies.
8. mission-contract-3, expanded (high/M-L) — bound every production external call; the verified inventory is ten sites: the two original, both gitTry copies, census.SignatureText, the gate/guard CommandContext execs (group-kill + WaitDelay), dispatch's gitOutput inside the locked build-record path (a hung git blocks dispatch and arming), and the bare git helpers in validate/conformance.go and report/frontier.go. Sweep the remaining production exec.Command sites and classify each. [R1.7, R2.4]
9. mission-contract-2, target revised (high/M) — pin candidateSHA and gateRef once; run gate and guards in independent clean worktrees at the pinned SHAs (a shared worktree would let the gate pollute what guards read). [R1.5]
10. foundations-1 (high/M) — evidence GC must not trust other checkouts' manifests.
11. mission-contract-6 (med/S) — surface batched-ask write failures in fence refusals.
12. missionrunner-4 (med/S) — fence answer must leave the ask open while fences are still reached.
13. adapter-host-registry-4 (med/S) — refuse non-object payloads at the registry door.
14. lease-census-6 (med/S) — protect the event envelope from payload clobber.
15. dispatch-supervise-5 (med/S) — gate the `.mirror-fail-once` fault hook to the fake runtime.
16. lease-census-7 (med/M) — gate the identity fixture env var the way the process-table fixture is gated.
17. validate-report-3, -4, -5 (med/S each) — frontier fail-open, frontier durability, plan-consistency fail-open.
18. foundations-2 (med/S) — fix `chain()`'s non-ancestor behavior to match its doc.
19. missionrunner-5 (med/S) — decouple drain reap cadence from the 100ms heartbeat.
20. lease-census-9 + codex-5 (med/M) — bound BOTH record-lock implementations: the sweep's acquireRecordLock and dispatch's withRecordLock (a wedged holder currently stalls every lease claim and succession while RunHeld holds the lease lock). [R1.8]
21. dispatch-supervise-6 (med/M) — make the group-ceiling verdict real: enumerate via AllPids + Getpgid; only ESRCH is an absent member, and any other failure maps to Indeterminable in Owner.Cycle instead of being ignored (today the ceiling check runs only when err == nil). [R1.13, R2.3]
22. foundations-11 (med/M) — structural self-hooks check instead of substring matching.
23. foundations-5, -9 (med/S each) — bound-kind in timeout errors; validate numeric knobs.
24. Shell error-handling batch (all S): script-validate-11 (unguarded kill), script-orchestration-07 (reap sweep aborts on first failure), -11 (silent arming death), -15 (swallowed mirror CAS failure), script-adapters-11 (unbounded start-gate wait), script-fixtures-016 (machine-wide pkill), script-misc-10 (state-file key collision), script-misc-4 (non-JSON records misclassified).
25. lease-census-10 (med/M) — residual sweep coverage beyond the seams and refusal tests that now land inside W1.2 and W1.3 themselves (R2.6): additional rows, not the proof of those fixes.
26. codex-6 (low/S) — segment the durable events root by checkout hash like mirrors; consistency hardening, the collision is practically foreclosed by PID+timestamp naming. [R1.9]

### W2 — One rule, one home: Go consolidations

1. architecture-1 (med/M — downgraded in critique round 1 to match the cli-3 adjudication: both copies are tested and identical today, so this is preventive consolidation, not a live defect) — single owner for typed usage extraction (kills adapter-host-registry-1 and cli-3 too). Design decision recorded: a small leaf package, not host-imports-adapter, because mission/fence.go must also come off its adapter import. [R1.11]
2. dispatch-supervise-4 (med/L) — one directory-lock protocol in internal/lock; do after W1.6 and codex-2 (W1.4).
3. dispatch-supervise-1 + -2 (med/M) — reverse the dispatch→supervise edge; one home for the grace constant.
4. missionrunner-1 (med/M) — one `concludeCycle` helper; pairs with the loop.go file split (missionrunner-6).
5. foundations-4 (med/S) — one publication sequence inside atomicfile.
6. foundations-3 (med/L) — finish or retire the atomicfile durability contract; decide, don't carry.
7. foundations-7 (med/S) — one conf line parser.
8. mission-contract-4 (med/S) — one authored-block parser.
9. mission-contract-5 (med/S) — one cap-key validator.
10. lease-census-4 (med/M) — one home for the announcement schema.
11. lease-census-3 (med/M, sign-off) — one fixture-identity reader; retiring the "started" key spelling needs the sign-off.
12. dispatch-supervise-7 (med/M) — one chain-ancestry walker.
13. adapter-host-registry-2 (med/M) — wiredoc RenderValue plus one home for the shared helpers.
14. adapter-host-registry-3 (med/S) — newest-snapshot by capturedAt, not filename order.
15. cli-5 (med/S) — one watcher-config constructor with one precedence rule.
16. lease-census-8 (med/M) — typed structs across the lease/census boundary.
17. Small dedups (all S/M): validate-report-9, -10, -13, lease-census-5, adapter-host-registry-9, missionrunner-9.

### W3 — Evacuate cmd, make the CLI uniform

1. cli-1 (high/M) — reserved-cap blocking policy into internal/supervise, one terminal-status predicate exported from dispatch; relocation happens after the fail-closed fix (codex-4, W1.5) so what moves is tested code. [split per R1.6]
2. cli-2 (high/M) — watchdog health policy into internal/supervise, table-tested.
3. architecture-2 remainder (med) — slug rule and json helpers out of cmd; re-seed the coverage ratchet in the same commit.
4. cli-6 (med/S, sign-off) — usage errors exit 2 uniformly.
5. cli-7 (med/M, sign-off) — one boolean-flag dialect.
6. cli-8 (low/S), cli-9 (low/M), cli-10 (low/S) — error surfacing, one parsing dialect, file organization.
7. Toolchain: wire `staticcheck` into go-gate.sh so the clean state is enforced, not incidental (baseline recommendation).

### W4 — Script boundary: move the decisions into the engine

Every item here keeps on-disk formats and existing behavior; most propose a new verb
(sign-off list below). Grouped by subsystem.

Dispatch (`dispatch.sh`):
1. script-orchestration-01 (high/M, sign-off) — cap-authority lock through the owner-lock verb; kills the unhealable mkdir spinlock. Prerequisite: codex-2 (W1.4) — the owner lock must refuse takeover on unreadable argv before it guards cap authority. [R1.2]
2. script-orchestration-02 (high/L) — roster/tier/escalation policy behind `job resolve-roster`.
3. script-orchestration-03 (high/M) — config-origin and the cap chain behind `config origin` / `job resolve-cap`.
4. script-orchestration-12 (med/S) — fold the fingerprint match into `job census-fresh`.
5. script-orchestration-09 (med/M) — three-way liveness probe for the shell ladder.
6. script-orchestration-13 (med/M) — one authorize-and-launch tail for dispatch_job and follow_up.
7. script-orchestration-08 (med/M, sign-off) — delete the dead standing-reaper mode.

Supervision and watching:
8. script-orchestration-04 (med/M, sign-off) — `supervise derive-ceiling`.
9. script-orchestration-10 (med/M, sign-off) — `supervise verify-armed`.
10. script-orchestration-05 (med/M, sign-off) — `report running-work` for the hook.
11. script-orchestration-06 = script-misc-3 (med→high/L, sign-off) — the watch-scan verb; family placement (report vs supervise) must be reconciled with the KS-R3-009 ruling at sign-off.

Adapters and hosts:
12. script-adapters-01 (high/L) — `adapter adjudicate-turn`: the terminal-outcome state machine into Go.
13. script-adapters-02 (high/M) — `adapter claude-command`, consumed by adapter and host both (the codex pattern).
14. script-adapters-06 (med/S) — permission→sandbox mapping into the existing codex/devin builders.
15. script-adapters-10 (med/M) — `host finish`: one outcome adjudication instead of three.
16. script-adapters-07 (med/M) — `adapter devin-settle`.
17. script-adapters-04 (med/S) — repair eligibility and prompt into Go.
18. script-adapters-05 (med/L) — `adapter selftest-run`: the conformance suite into Go.
19. script-adapters-13 (med/S) — host-common.sh for the quadruplicated host boilerplate.
20. Small dedups: script-adapters-08, -09, -12, -15 (all low).

Standalone gates and adoption:
21. script-misc-2 (med/M, sign-off) — the refactor gate into the validate family.
22. script-misc-1 (high/L, sign-off, decide first) — resolve adopted-engine delivery so the shipped CI workflow can pass; this is the open r10 ruling. Per critique round 1 this decision moves ahead of the whole W4 stream: adopted CI is red on day one until it lands. [R1.10]
23. script-misc-8 (low/S) — stop producing settings.json with sed.
24. script-misc-9 (low/S) — delete the dead emit-event wiring.
25. script-misc-5 (low/L, sign-off) and script-misc-7 (low/M, sign-off) — adoption-plan and skill-validation verbs; both downgraded by the verifier as not currently carrying their weight — treat as optional, decide at sign-off.

### W5 — validate-metasystem.sh: decompose and de-shadow

1. script-validate-4 (med/L) — extract the two giant blocks into sub-suite scripts, the pattern the file already uses.
2. script-validate-1 (med/S) — delete the awk contract-hash shadow (fix shape per the verifier's note: hash-only verb or reshape the fixture).
3. script-validate-2 (med/M) — poll the existing `dispatch census-fresh` verb instead of inline python, twice.
4. script-validate-3 (med/M) — a typed exit code for the arming-window transient; collapse three retry loops.
5. script-validate-8 (med/S) — read `metasystem.runtimes` through the config engine.
6. script-validate-9 (med/S) — replace the Go-source grep pin with a Go unit test.
7. script-validate-10 (med/L, sign-off) — triage the ~60 python3 heredocs; adopted repos should need only the binary.
8. script-validate-5 (low/M, sign-off) and -7 (low/M, sign-off) — protocol-shape catalog and conf-writing verbs; verifier-downgraded, decide at sign-off.
9. Toolchain: add `shellcheck` to the suite for `scripts/` (baseline recommendation; the adapters cluster found quoting bugs shellcheck would have flagged).
10. Smalls: script-validate-11 (in W1.24), -12 (low/S).

### W6 — Fixture retirement: port to Go tests, then delete

The discipline for every item: verify parity first (several verifier notes document
shell-only legs that Go does NOT yet cover — porting those is the bulk of the work),
port the missing legs, then retire the shell. Never delete first. And one more gate
from critique round 1: adoption ships the scripts/ tree, so retiring any shipped file
changes the shipped surface — every whole-file retirement is itself a sign-off item, or
the filename survives as a thin compatibility wrapper. [R1.12]

1. script-fixtures-002 (high/S) — the schema linter becomes an internal/returnschema test.
2. script-fixtures-003, -004, -005 (med) — mission/contract/runner legs; -005 notes what must stay shell.
3. script-fixtures-012, -014, -021, -006 (med) — record-protocol, config-identity, exhaustion, fence legs.
4. script-fixtures-008 (low, verifier-corrected) — port the five missing open-work cases to Go first; the 1:1 parity claim was false.
5. script-fixtures-001 (med/M) — the G-5 lint; scope decision needed (Go test drops adopted-repo enforcement — see the verifier note).
6. script-fixtures-007 (med/M) — replace the ps shim and updater daemon with the engine's fixture seam.
7. script-fixtures-013 (med/S) — point the wiring test at the real `__record-cas`.
8. script-fixtures-020 (med/M, sign-off) — extend `config tailor` for the fake runtime; kill three perl rewrites.
9. Hygiene batch: -010 (vacuous leg), -016 (in W1.24), -017 (fixture-budget caps), -019 (source grep), -022, -009, -011, -015, -018 (all S).

### W7 — Documentation: make the map match the territory

Can run in parallel with everything; the first item should happen this week.

1. architecture-4 (high/S) — the glossary points at five deleted Python scripts; every agent session inherits the misdirection.
2. architecture-3 (med/M) — one architecture document: package map, layering, family-to-package table. The 29 package docs already exist; this is assembly.
3. architecture-5 (med/M) — promote the standing contracts out of plans/ into docs/design, per the tree's own wire-documents pattern.
4. architecture-7 (med/M) — the dispatch-sequence ground-truth document.
5. architecture-8 (med/S) — fix wow.md's dead route.
6. Package-doc batch (all S): mission-contract-11/architecture-6 (contract's invisible doc comment; the rename is optional), dispatch-supervise-8 (false launcher-shell narrative), -10, -12, adapter-host-registry-7, validate-report-14/architecture-10, -16, foundations-13/architecture-9, -15, missionrunner-8 fragment, script-orchestration-14 (stale TODOs).
7. Idiom lows as encountered: missionrunner-3, -7, -8, -9, mission-contract-7, -9, -10, -12, -13, dispatch-supervise-9, -11, -13, -14, adapter-host-registry-5, -8, validate-report-6, -11, -12, -15, -2, -1, lease-census-11, -12, foundations-8, -10, -12, -14, -16, -6, cli-8, -9, -10, script-fixtures and script-misc lows not listed above.

## Decisions needed (nothing here executes without sign-off)

**A. Findings that touch an on-disk format, shipped file set, or CLI surface (16):**
mission-contract-8 (ask JSON gains a `reasons` array), lease-census-3 (retire the
"started" key spelling), cli-6 (exit-code normalization is observable by callers), cli-7
(boolean-flag values), script-validate-2 (--wait form, if added), -5, -7, -10 (new util
verbs / python removal), script-orchestration-01 (cap-authority through owner-lock), -08
(delete standing-reaper surface), script-fixtures-020 (config tailor --fixture),
script-misc-1 (adopted-engine delivery / CI), -2 (refactor-baseline verb), -3
(watch-scan verb), -5 (adopt plan verb), -7 (validate skill verb). Added in critique
round 1: every W6 whole-file script retirement (the adoption payload ships scripts/),
taken as one blanket ruling or per file.

**B. Proposed new verbs, for naming and existence sign-off:**
`job resolve-roster`, `job resolve-cap`, `config origin` (or --origin on get),
`supervise derive-ceiling`, `supervise verify-armed`, `report running-work`,
`report watch-scan` (family TBD vs KS-R3-009), `adapter adjudicate-turn`,
`adapter claude-command`, `adapter repair-prompt`, `adapter devin-settle`,
`adapter selftest-run`, `host finish`, `validate refactor-baseline`, plus the optional
`validate skill`, `adopt plan`, `util json-get`/`config set` family from W5.7-8.
Some of these will collapse into fewer verbs at design time; the list is the surface
requested, not a final design.

**C. Open rulings this report intersects:** adopted-engine delivery (r10) — decide
FIRST, it blocks the whole W4 stream and adopted CI is red until it lands;
watch-scan family placement (KS-R3-009 said report, script-orchestration-06 proposed
supervise); the atomicfile durability contract (finish or retire, W2.6).

## Method

Baseline: `go vet ./...`, `gofmt -l`, `staticcheck ./...` (all clean), script inventory
and dependency-graph extraction. Review: 14 scoped reviewers, each reading its full
assignment end to end against the rubric above, with the house doctrine (decisions in
Go; scripts are plumbing; on-disk contracts preserved; CLI changes need sign-off; linter
findings excluded) embedded in every brief. Verification: one adversarial verifier per
cluster, instructed to re-read every cited location and kill findings that were
factually wrong, doctrine-violating, linter-duplicating, or not clearly better than
current. 28 agents, ~1,200 tool calls. Findings whose verifier notes corrected facts
carry those notes inline.

---
## Part 1 — The Go application

### internal/missionrunner

*Verdict: passes the senior-engineer bar. 9 findings (4 medium, 5 low).*

**Reviewer's summary.** internal/missionrunner is the mission runner: the Engine that launches and drives a mission's unattended host turns (launch/lease/run-loop/host supervision/drain/status/answer) plus the decision surfaces it rests on (adjudication of orchestrator returns, conclusion proposals, stop-loss replay, patience floors, fence projection, job selection). 18 non-test files, ~5.3k source lines; 5.3k test lines across 24 test files. Overall state: genuinely strong. The package doc states the two-layer responsibility precisely; naming matches contents; every exported identifier is documented, usually with the why; errors are labeled refusals with mapped exit codes; the one goroutine (hostProcess's single Wait) has a clean owner/shutdown story; kill decisions require ownership proof; tests are behavior-driven, table-driven where natural, use real processes and real git checkouts, inject seams (anchorFn, custodianFn, fixture cap scaling) instead of sleeping, and cover the crash-window heals (reserve/append gap, drain-stall ask re-raise, pending reset) that matter most. On the god-package question: this is not a grab-bag — everything here is one runner and the name is honest — and I do not recommend splitting it into multiple packages now, because the decision surfaces and the engine share vocabulary (jobRecord, ask helpers, the wire-doc readers) and a split would force exporting a dozen internals for no consumer. The two seams that DO deserve attention are internal: (1) the pure-proposal layer the package doc promises is violated in one spot — the proposal functions write the usage aggregate as a side effect — and fixing that restores the layer boundary the doc claims; (2) loop.go at 1,267 lines concentrates the process lifecycle, state CAS, heals, and two near-duplicate conclusion sequences, and wants a file-level split plus one shared conclude helper. If a package seam is ever wanted later, the right cut is the one the doc already names: pure decisions (adjudicate/cycle/stoploss/patience/contract-context) versus the writing Engine, enforced by forbidding os/exec and dispatch imports on the decision side. Findings: no critical, no high; 4 medium (duplicated conclusion sequence, side-effecting proposals, cross-package error matched by string, fence-answer consuming its only ask without unparking) and 5 low polish items. cmd/metasystem stays thin over this package.

#### missionrunner-1 — concludeFaultedTurn and cycleConclude duplicate the ~60-line cycle-conclusion sequence

`internal/missionrunner/loop.go:838` · duplication · **medium / effort M**

**Current:** concludeFaultedTurn (loop.go:838-905) and cycleConclude (loop.go:1179-1243) each independently implement: drainJobs -> parked short-circuit -> measure -> candidateSHA fallback via gitRevParse -> appendLedger -> write measurement.json -> build proposal -> deliverLandedUnconsumed on completed -> writeState -> anchor -> host-failure park -> continueOrParkStopLoss. The two copies differ only in which proposal builder runs (ConcludeFaultedTurn vs ConcludeFiles) and the ledger annotations passed.

**Target:** One unexported concludeCycle helper owning the shared sequence, parameterized by a proposal-builder closure and the ledger annotations; both call sites shrink to their genuinely different lines. This is the single home for the binding order the comments describe twice.

**Why:** The binding order (drain before measure, ledger before state, anchor after state) is the package's central correctness argument; two hand-maintained copies of it will drift the next time the order changes, and the drift would be a crash-recovery bug, not a style problem.

#### missionrunner-2 — Proposal functions documented as pure write the usage aggregate as a side effect

`internal/missionrunner/cycle.go:114` · design · **medium / effort M**

**Current:** cycle.go's header says 'this package never writes the state itself' and the package doc says the decision surfaces 'stay pure and compute proposals; the Engine is the single writer'. But ConcludeTurn (line 114), ConcludeFaultedTurn (197), RecordFailureProposal (239), and parkOutcome (310) all call aggregateUsageForProjection, which invokes mission.AggregateUsage — a disk write to usage.json — plus a flight-recorder emit on failure. Consequently the CLI verbs 'mission turn-conclude', 'turn-record-failure', and 'turn-park', documented in cmd/metasystem/missionrunner_verbs.go as verbs that 'judge and print a JSON proposal', mutate mission artifacts when run.

**Target:** Hoist the aggregateUsageForProjection call to the writers: the Engine call sites (cycleConclude, concludeFaultedTurn, recordFailedTurn, parkState/parkDrainStalled) and the cmd layer of the proposal verbs, invoked immediately before the proposal is computed so ProjectFences reads the same fresh aggregate. The proposal functions become read-only, matching their documentation.

**Why:** The single-writer discipline is this system's stated invariant, and the package doc's purity claim is currently false; a proposal verb that silently writes is exactly the class of surprise the doc exists to prevent, and it makes the pure functions untestable without observing disk writes.

#### missionrunner-4 — Fence answer consumes the only fence ask even when the amended fences are still reached

`internal/missionrunner/answer.go:102` · error-handling · **medium / effort S**

**Current:** In Answer's fence branch (answer.go:83-104), a preflight-failing contract is refused with the ask left open, but a preflight-passing amendment whose fences are STILL reached (e.g. the human raised fence.cycles while the wall-clock fence — which keeps growing while parked — is the one tripped) falls through: the mission stays parked, yet the ask is marked answered and pruned from the waiting list. ReserveCycle raises fence-bound asks only from a running mission's cycle, and resume refuses any non-running mission, so the sanctioned surface (mission-runner.sh answer/resume) offers no way forward; recovery needs the internal 'mission fence-refuse' verb or hand-written ask files.

**Target:** When reached is still true, refuse the answer and leave the ask open — mirroring the preflight refusal a few lines above — with a message naming which fence is still reached so the human amends the right limit and answers again.

**Why:** An unattended-mission park that the documented human surface cannot unpark is the exact failure class this runner exists to prevent; the fix is a few lines and turns a wedge requiring tribal knowledge into a self-explaining retry.

#### missionrunner-5 — Drain reap cadence is coupled to the 100ms heartbeat: thousands of subprocess spawns per drain

`internal/missionrunner/drain.go:71` · design · **medium / effort S**

**Current:** drainJobs polls at Interval("METASYSTEM_HEARTBEAT_INTERVAL_MS", 100). Every 100ms pass re-globs and JSON-decodes the entire shared artifacts/agents/jobs directory three times (activeJobRecords twice plus reapReservedRecords), then spawns scripts/agents/dispatch.sh reap — a bash script that runs git rev-parse and the metasystem binary — once per active job. A lawful drain waits out a job's capMin (fence.job-cap-min is 30 in the fixtures), so one 30-minute drain of one job is ~18,000 passes and ~18,000 subprocess spawns, more per extra job.

**Target:** Decouple the two cadences: heartbeat every heartbeat interval, but run the reap pass (runner reap + dispatch.sh reap + directory rescans) on a coarser reap interval — seconds, not the heartbeat's milliseconds — with its own env override for fixtures. The deadline recomputation logic stays as is.

**Why:** The load is pure waste (a job cannot become provably dead more often than the reap facts change), it burns a core for the length of every cap-bound drain, and this repository's own receipts say timing fixtures flake under machine load — the drain is manufacturing exactly that load.

#### missionrunner-3 — Cross-package error contract held by a string comparison on wiredoc's message

`internal/missionrunner/missionrunner.go:141` · error-handling · **low / effort S**

> downgraded by the verifier: The mechanism is as claimed: missionrunner.go:140 matches err.Error() against the literal fmt.Errorf at wiredoc.go:49, and the sentinel + errors.Is target is strictly better Go. But the severity rationale is factually wrong: wiredoc_equivalence_test.go:49 asserts decodeJSONDoc([]byte(`["array"]`)) returns errNotJSONObject, so any rewording in wiredoc fails that test loudly on the next full-suite run — the E1 distinction is NOT silently collapsible and the 'nothing would fail loudly' claim does not survive. What remains is idiomatic hardening of a tested coupling: worth the small change, but low, not medium.

**Current:** decodeJSONDoc re-maps wiredoc's not-an-object refusal by matching err.Error() == "not a JSON object" against the literal fmt.Errorf in internal/wiredoc/wiredoc.go:49, then substitutes the package's own errNotJSONObject sentinel so readDocLabeled can word the refusal differently from unreadable bytes (the E1 distinction).

**Target:** Export a sentinel in wiredoc (var ErrNotObject = errors.New("not a JSON object")), return it (or a wrap of it) from Decode, and match here with errors.Is. The local errNotJSONObject can alias or wrap it.

**Why:** Any rewording in wiredoc silently collapses the two refusal classes this package deliberately distinguishes — the labeled 'must be a JSON object' message and its tests would quietly become dead — and nothing in either package would fail loudly at the moment of the change.

#### missionrunner-6 — loop.go at 1,267 lines mixes four separable concerns

`internal/missionrunner/loop.go:1` · complexity · **low / effort S**

**Current:** One file holds the detached process lifecycle (RunLoop/internalRun/lease/heartbeat/records), the state CAS and anchor plumbing (verifyState/writeState/anchorState), the crash-window heals (healReservedCycle/applyPendingReset), and the per-cycle machinery (five cycle steps, recordFailedTurn, concludeFaultedTurn, parks). It is 2.5x the next-largest file and its own section comments already name the seams.

**Target:** Split along the existing comment seams into runloop.go (process life + lease), stateio.go (verify/write/anchor), heal.go (resume heals), and conclude.go (cycle steps + conclusions). Pure file moves, no API change; pairs naturally with missionrunner-1's shared helper.

**Why:** The file is where every future runner change lands, and the four concerns have different risk profiles (crash-recovery code should not scroll past process plumbing); the split is mechanical now and gets more expensive every satellite.

#### missionrunner-7 — exitFor branches on a bare type assertion instead of errors.As

`internal/missionrunner/engine.go:42` · idiom · **low / effort S**

**Current:** exitFor uses err.(*RunnerError) while the same package correctly uses errors.As for *SessionFault (loop.go:1145) and errors.Is for errNotJSONObject. RunnerError has no Unwrap and failf flattens causes with %v, so today nothing wraps a refusal — but the first future fmt.Errorf("...: %w", failf(...)) silently downgrades a deliberate exit code (7, unreadable-state) to the generic 3. The raw codes 3/7/10/11/13 are also magic numbers repeated across five files.

**Target:** errors.As in exitFor, and named constants for the exit codes (exitFailure=3, exitUnreadable=7, ...) beside RunnerError so the code-to-meaning table exists once.

**Why:** Cheap insurance on the package's one error-branching contract, and the exit codes ARE the CLI contract drivers branch on — they deserve names where they are minted.

#### missionrunner-8 — Dead fields: cycleContext.values, hostProcess.err, and a stray lint pragma on a used local

`internal/missionrunner/loop.go:982` · dead-code · **low / effort S**

**Current:** cycleContext.values is assigned (loop.go:982) but never read by any later step — turnCapMin uses the local. hostProcess.err is written by the Wait goroutine (host.go:36) but never read anywhere; exit status flows through ProcessState. patience.go's staged fields (floor/breach/witness) carry documented lint pragmas, but the pragma at patience.go:302 sits on the local floor := selectPatienceFloor(...) which IS used two lines later — pragma noise that no longer suppresses anything.

**Target:** Drop cycleContext.values and hostProcess.err (or read err where a Wait I/O failure would matter, with a comment); delete the stray pragma at patience.go:302. The staged patienceChain fields stay — they are documented plan work, not dead code.

**Why:** Dead context fields imply cross-step data flow that does not exist, which is exactly what a reader of a five-step state machine has to rule out; the unused err field suggests error capture that never happens.

#### missionrunner-9 — pathBase hand-reimplements filepath.Base

`internal/missionrunner/patience.go:509` · idiom · **low / effort S**

**Current:** patience.go:507-514 defines pathBase over strings.LastIndexByte(path, '/') with a rationale comment about not 'importing path/filepath twice in this package's mental model'; the sibling files (jobs.go, contract.go) already call filepath.Base for the identical job-record-stem operation via strings.TrimSuffix(filepath.Base(...)).

**Target:** Use filepath.Base here like the rest of the package (or share one jobRecordStem helper with contract.go:235 and jobs.go:83, which is the third copy of the same stem derivation).

**Why:** A private re-spelling of a stdlib function makes a reader hunt for the behavioral difference, and there is none worth having; three copies of the stem derivation is one home short.

### internal/mission and internal/contract

*Verdict: passes the senior-engineer bar. 13 findings (3 high, 3 medium, 7 low).*

**Reviewer's summary.** Core domain pair: internal/contract owns the signed mission contract document (grammar, validation, sealing with a measured baseline, launch preflight, per-cycle gate/guard measurement, dispatch-envelope queries); internal/mission owns mission runtime truth (hash-chained state.json with legal-transition enforcement, the markdown stop-loss ledger and its annotation grammar, fence counters/reservations, usage aggregation with death-proof gating, landed-returns derivation, deterministic prompt assembly, and git anchor/reconcile). The split is a REAL boundary, not a historical one: contract answers "what was signed and does the candidate still measure", mission answers "what is the mission doing now", and mission imports contract only through the deliberately narrow grammar surface (AuthoredBlocks/SealBlock in grammar.go, created for exactly this per its own comment). missionrunner composes both; cmd/ is thin flag-parse-call-print throughout. Two genuine leaks blur the edge: fence.go re-implements cap-key canonicality policy that contract owns (finding 5), and the authored key=value parse loop is copied three times inside mission (finding 4). The deliberate-duplication doctrine (helpers.go C-3) is documented and mostly holds, but it has already produced drift (a contract-path error that says "mission ledger", finding 9). Code quality is high: decisions are documented with plan references, locking is deliberate flock+atomic-write with no goroutines, and the tests are excellent — behavior-driven, fixed-clock injected (no timing flakiness), covering refusal paths, tamper detection, crash-tolerance predicates, and byte-stability. The material defects are specific, not systemic: an off-by-one in executable-relative root discovery that kills the documented nested-checkout case (finding 1), gate/guard measurement duplicated into two worktrees that can read different commits (finding 2), and two external calls that escape the tree's own B4 every-external-call-is-bounded rule (finding 3).

#### mission-contract-1 — contractMetasystemRoot strips one directory too many, so nested-root discovery never works

`internal/contract/contract.go:1476` · design · **high / effort S**

**Current:** contractMetasystemRoot derives the checkout as filepath.Dir(filepath.Dir(filepath.Dir(exe))). The binary ships at <root>/bin/metasystem (scripts/agents/arm-supervision.sh:38 wires $harness_root/bin/metasystem), so Dir^3 lands on the checkout's PARENT; the confirmation check (metasystem.conf or scripts/agents at that path) then fails and the function returns "" everywhere — verified against this very repo: exe=agentic-tools/metasystem/bin/metasystem yields agentic-tools, which has neither. The shell originals derive root as script_dir/../.. from scripts/agents/<script> (three components deep); the port kept three Dir() calls on a binary only two deep. cmd/metasystem/census.go:30 carries the identical latent off-by-one.

**Target:** Dir(Dir(exe)) so the derived root is the checkout containing bin/, keeping the existing metasystem.conf/scripts-agents confirmation as the guard; add a test that lays out <tmp>/bin/<fake-exe> beside metasystem.conf and pins the resolution. Fix census.go's copy in the same change.

**Why:** contractProjectRoot's documented case — a metasystem checkout nested under the repository, i.e. this template repo itself — silently falls back to the outer git repo, so validate/seal/preflight on a nested contract read the wrong docs/project-rules.md and the wrong artifacts/ tree and fail closed ("cannot read envelope policy"). In flat adopted repos the fallback happens to be correct, which is why every fixture test passes while the feature is dead; worse, a metasystem nested one level under an adopted repo would pass the parent's confirmation check and resolve a plausible-looking WRONG root.

#### mission-contract-2 — runGate re-implements candidateWorktree+measureCommand inline, and Measure materializes the candidate twice

`internal/contract/contract.go:845` · duplication · **high / effort M**

**Current:** runGate (contract.go:845-936) contains a near line-for-line copy of measure.go's candidateWorktree (gate-ref resolve, branch resolve, candidate rev-parse, restoredPaths, MkdirTemp, worktree add, gate-ref checkout, project-rel compute) and of measureCommand (bash -lc under the job-cap timeout, metric-line scraping). measure() then calls runGate and runGuards back to back, and runGuards builds a SECOND throwaway worktree via candidateWorktree (measure.go:147), re-resolving the branch tip; runGuards even discards candidateWorktree's returned SHA.

**Target:** measure() resolves candidateSHA and gateRef exactly once and passes the pinned SHAs to runGate and runGuards, each materializing an independent clean worktree at the pinned SHA (or one worktree with reset+clean+re-restore between gate and guards); runGate takes its worktree recipe from candidateWorktree and its execution from measureCommand, so each recipe keeps one home. A shared live worktree is NOT the fix: gate and guards are arbitrary bash with cwd inside the worktree, so the gate could mutate what guards read and defeat the frozen-instruments restore. Add a test where the gate mutates a guarded path. [revised R1.5]

**Why:** Beyond the maintenance cost of ~80 duplicated lines in one package, the double materialization is a correctness hole: with fence.concurrency jobs allowed to land commits on the mission branch while a cycle measures, the gate metrics and the guard readings can be taken at DIFFERENT commits, yet MeasureResult reports one CandidateSHA and one GatePassed verdict over the mixed readings — the ledger then records evidence that never existed on any single commit, in a system whose stated point is measured evidence.

#### mission-contract-3 — landedRoundValid and the anchor commit run external commands with no bound, violating the tree's own B4 rule

`internal/mission/landed.go:148` · boundary · **high / effort M-L**

**Current:** landedRoundValid execs scripts/assert-return-complete.sh with a bare cmd.Run() — no boundedexec, no timeout — and runs once per emitted Landed Returns row (up to 19 spawns) inside every prompt assembly; the script itself re-enters the metasystem binary (Go→bash→Go). Anchor's commit exec (anchor.go:227-233, git commit or the scripts/agents/commit.sh wrapper) is likewise unbounded, while every neighboring git call in both packages routes through boundedexec with an explicit "Bounded like every other external call (B4)" comment.

**Target:** Route every production external call through boundedexec or give it group-kill semantics; a timed-out checker is "unproven" (the invalid marker landedRoundValid already documents). The verified inventory is ten sites: these two; both gitTry copies (contract/helpers.go:94, mission/anchor.go:41, each with multiple production call sites); census.SignatureText (fingerprint.go:47 — fully unbounded, and a hung adapter script hangs watcher passes and lease classification); the gate/guard CommandContext execs (contract.go:888, measure.go:252 — bounded in name only: no process group, so at the deadline Go kills only bash, grandchildren holding the output pipe block Run() past the ceiling, and the deferred worktree cleanup waits with it — fix with Setpgid + group-kill Cancel + WaitDelay or boundedexec at the job-cap bound); dispatch's gitOutput (gitcmd.go:13, called from build.go:127/131 inside the locked build-record path, where a hung git blocks dispatch and arming indefinitely); and the bare git helpers in validate/conformance.go:78/87 and report/frontier.go:154. Sweep the remaining production exec.Command sites and classify each as deliberately-bounded or defect; add a hanging-git test for the locked dispatch path. [expanded R1.7, R2.4]

**Why:** A wedged checker or commit wrapper (stdin-blocking child, NFS stall, lease-wrapper hang) freezes unattended prompt assembly or the anchor step forever with no fence — the exact failure mode the repository's B4 doctrine exists to prevent, applied everywhere else in these two files. The Go→shell→Go re-entry also deserves a look: return-validity is decision logic that internal/validate already owns natively.

#### mission-contract-4 — The authored-block key=value parse loop exists four times, three of them inside mission

`internal/mission/state.go:572` · duplication · **medium / effort S**

**Current:** authoredContractValues (state.go:572-596), contractValuesFromBytes (fence.go:84-105), and promptAuthoredValues (prompt.go:70-90) are line-identical parse loops (split lines, Cut on '=', reject missing '=' and duplicate keys) over the same contract.AuthoredBlocks output, beside contract's own contractParseKeyValues which additionally rejects whitespace-padded keys. fence.go's copy also carries an unused repo parameter.

**Target:** One unexported mission helper — or better, an exported parser in contract/grammar.go beside AuthoredBlocks, which every copy already calls — used by all three mission call sites; drop the dead repo parameter.

**Why:** The helpers.go C-3 doctrine defends duplicating tiny pure helpers ACROSS the package boundary; it does not cover three identical in-package copies of domain parsing that have already diverged from the contract owner's stricter grammar (padded keys pass in mission, fail in contract). Any grammar change now has four homes, three of them in the same package.

#### mission-contract-5 — Per-pair cap-key canonicality policy is implemented in both packages

`internal/mission/fence.go:123` · design · **medium / effort S**

**Current:** fence.go:123-139 re-implements the cap.min.<runtime>.<model> canonicality check — SplitN, idRe on the runtime, config.CanonicalModel on the model, expected-key reconstruction, positive-int value — that contract.contractValidatePairCap (contract.go:414-428) owns, with independently worded errors. Patience keys, by contrast, validate only in contract, showing the asymmetry.

**Target:** contract exports one small validator (e.g. contract.ValidatePairCap(key, value) error) and fence.go calls it; the two packages then cannot disagree about what a canonical cap key is.

**Why:** Cap-key canonicality is signed-exposure policy, not a stdlib-adjacent pure helper, so the C-3 deliberate-duplication rationale does not apply. If the cap grammar evolves (a new model-encoding rule in config.CanonicalModel's surroundings, a new segment), the seal-time check and the runtime fence check can drift apart — precisely the two checks that must agree for a signed cap to mean anything.

#### mission-contract-6 — writeBatchedAsk failures are swallowed at every fence-refusal site

`internal/mission/fence.go:394` · error-handling · **medium / effort S**

**Current:** CheckOrReserve (fence.go:394), ReserveCycle (:428), and AuthorizeCap (:478) all do `ask, _ := writeBatchedAsk(...)` and embed the possibly-empty path in the refusal message. If the ask write fails (permissions, disk full, unmarshalable existing ask), the job is still refused but the human ask — the designed recovery channel for a tripped fence — silently never exists, and the error message ends with "batched ask written: ".

**Target:** Propagate the write error into the returned refusal ("fence refused (...); FAILED to write batched ask: ...") so the operator learns the recovery channel is broken instead of waiting on an ask that was never raised.

**Why:** The whole fence design routes recovery through the batched ask; a mission parked on a fence whose ask write failed is indistinguishable from one whose ask is pending a human, and stays parked forever. The rubric's "nothing swallowed" is violated at exactly the three places where it matters most.

#### mission-contract-10 — ContractError and StateError are exported with zero consumers, and stateErr flattens all causes

`internal/contract/helpers.go:31` · idiom · **low / effort S**

**Current:** No code outside the defining files references either type (verified by grep: no errors.As/Is against them anywhere, cmd/ just prints), and stateErr formats underlying errors with %v, so os.ErrNotExist/permission causes are unrecoverable through the chain. helpers.go justifies ContractError's existence (C-2) as decoupling from mission's type, but neither is consumed.

**Target:** Either unexport the types (plain fmt.Errorf semantics, honest surface) or make them earn the export: wrap causes with %w and have at least one caller branch on them.

**Why:** Exported speculative API is exactly what the rubric's minimal-surface rule targets: today the types promise a distinction no caller uses while actively destroying the cause chain callers might someday want. Small, but it is the package's public face.

#### mission-contract-11 — Neither package has a go-doc-visible package comment in the conventional place

`internal/contract/contract.go:29` · docs · **low / effort S**

*Overlaps architecture-6, which adds the package-rename question.*

**Current:** contract's excellent overview (contract.go:29-38) sits AFTER the import block, where go doc never surfaces it, so `go doc internal/contract` shows nothing; mission's package doc opens ledger.go and immediately narrows to "This file is the atomic owner of the ledger", conflating package role with file role.

**Target:** Move contract's block above `package contract` (or into a doc.go); give mission a package-level sentence separated from ledger.go's file comment.

**Why:** Both packages are the domain core and the prose already exists and is good — it is just invisible to the toolchain and to the first reader running go doc. Rubric 1 and 9 ask exactly for this.

#### mission-contract-12 — latestAnchor walks the entire git history on every verify/reconcile

`internal/mission/anchor.go:140` · complexity · **low / effort S**

**Current:** latestAnchor runs `git log --format=%H%x1f%B%x1e` with no limit and scans every commit message in Go until it finds the mission's newest anchor; verifyAnchor, verifyStateAnchor, and Reconcile each pay this, holding the full history's messages as one string.

**Target:** Let git do the filtering: `git log -1 --grep='^Mission-Id: <id>$'` (anchored, fixed mission id) or at least a --max-count bound, keeping the trailer parse for the single matched commit.

**Why:** On a long-lived repository this is O(history) time and memory per reconcile call in the unattended loop; the anchor is by construction recent, so the full walk buys nothing. Cheap to fix, measurable on real repos.

#### mission-contract-13 — InitState's authored candidate.branch fallback can never fire on a validated contract

`internal/mission/state.go:620` · dead-code · **low / effort S**

**Current:** InitState resolves the branch as branchArg → authored values["candidate.branch"] → seal-block candidate.branch. But contract validation rejects candidate.branch as an unknown AUTHORED key (contract.go:356), so on any contract that passed validate/seal/preflight the middle lookup is unreachable; it is live only for test fixtures (state_test.go writes it into the authored block) that bypass validation.

**Target:** Drop the authored-values lookup and move the fixtures to seal blocks (or branchArg), so the production resolution order is the code's only order.

**Why:** A fallback that only fixtures can reach is a small honesty problem: the code documents a resolution path production forbids, and a future reader will preserve or extend it believing it is real. Removing it also tightens the invariant that the branch comes from the seal the human signed.

#### mission-contract-7 — mission.Anchor prints its result to stdout from library code

`internal/mission/anchor.go:248` · boundary · **low / effort S**

> downgraded by the verifier: Facts confirmed: anchor.go:248 fmt.Println and :71 stderr are the only library stdout/stderr writes in either package, and cmd's runMissionStateAnchor prints nothing. But the composability harm is speculative today: the only caller is cmd/, and missionrunner deliberately anchors through the binary (engine.go:54-57 anchorState) with tests injecting anchorFn — no in-process caller ever receives the stdout noise. The fix is sound and byte-identical at the CLI, making this a consistency/testability polish rather than an operational defect; that is a tier below the medium findings here (broken recovery channel, policy drift), so low.

**Current:** Anchor ends with fmt.Println(strings.TrimSpace(head)) and clearIndexLock writes advisory text to os.Stderr (anchor.go:71); cmd/metasystem/mission.go's runMissionStateAnchor consequently prints nothing itself. Every sibling API (VerifyStateShape, AuthorizeCap, Refuse, Preflight) returns values and lets cmd/ own the output.

**Target:** Anchor returns the commit sha (and clearIndexLock's notice becomes part of the returned/logged flow or a callback), with cmd/ printing — byte-identical CLI output, library silent.

**Why:** It is the one place in the two packages where the cmd/internal boundary inverts: the library owns CLI bytes, so Anchor cannot be composed (missionrunner or a future caller gets stdout noise it cannot capture) and its output is untestable except by pipe capture. Consistency with the rest of the surface is the standard the tree itself sets.

#### mission-contract-8 — Batched-ask reason accumulation round-trips through the question prose

`internal/mission/fence.go:327` · design · **low / effort S / needs sign-off**

**Current:** writeBatchedAsk recovers the already-raised reasons by regexing backticked tokens out of the previously written human-readable question (askReasonRe over value["question"]), merges the new reasons in, and re-renders the sentence. The machine state (which fences tripped) has no field of its own; its only carrier is presentation text.

**Target:** Store a `reasons` array in the ask JSON and render the question from it; the prose becomes a projection instead of the store.

**Why:** Deriving state from display text is fragile in a known way: rewording the question template (or a human editing an open ask's question) silently drops accumulated reasons on the next merge, with no error. The fix adds a field to the on-disk ask shape, so it needs the owner's sign-off.

#### mission-contract-9 — contract's relUnderRepo reports "mission ledger is outside the repository" for contract paths

`internal/contract/helpers.go:126` · error-handling · **low / effort S**

**Current:** The contract package's copy of relUnderRepo kept the mission twin's error text verbatim, but its only caller is verifyOrigin (contract.go:1266) checking the CONTRACT file's location — so a contract outside the repository is refused with a diagnostic naming a ledger that was never involved.

**Target:** Error text that names the artifact actually checked ("mission contract is outside the repository"), or a label parameter if the helper is ever shared.

**Why:** A misleading refusal on the launch-gate path costs real diagnosis time, and it is concrete evidence of the C-3 duplication doctrine's drift cost: the copy was taken wholesale, message included, and nobody noticed because no test exercises this refusal.

### internal/dispatch, internal/supervise, internal/janitor

*Verdict: passes the senior-engineer bar. 14 findings (1 high, 7 medium, 6 low).*

**Reviewer's summary.** Cluster: agent dispatch and supervision — internal/dispatch (job-record lifecycle: locked create/setup/CAS, record templates, attestation gates, permissions expansion, mission-lease proof, evidence mirroring, critique-exhaustion policy, owner lock), internal/supervise (owner loop with pure decision core behind small consumer-defined interfaces, disk/process/ledger adapters, watcher census pass, no-kill reaper sweep, census-writer lock), internal/janitor (pure D-4 target selection plus the REG-6 triple kill proof). Overall state: strong. The decision cores are pure and behavior-tested against named design rules (three-way liveness discipline, CAS races, watermark, breaker), adapters are thin, cmd stays wiring-only, and the janitor package is exemplary. The real problems are architectural seams: the dispatch→supervise import points mechanism-toward-policy and exists only to share the budget verdict, which in turn forces the reaper to hardcode a grace constant dispatch exports as the single source; the census-writer lock re-implements the directory-lock protocol that internal/lock already owns and gets the takeover rule wrong (Unknown authorizes takeover — the one three-way violation in an otherwise disciplined tree); and a couple of test hooks and stale doc comments sit in production paths. On the brief's explicit question: the dispatch→supervise import direction is wrong — supervise is the policy layer and dispatch owns record semantics, so the shared verdicts (CapExpired, AbandonedSetupGrace) belong in dispatch with the reaper receiving them by injection exactly as it already receives the CAS Apply.

#### dispatch-supervise-3 — Census-writer lock takeover on Unknown liveness violates the proven-death rule

`internal/supervise/censuslock.go:86` · concurrency · **high / effort S**

**Current:** Claim() refuses only when AliveRef(...) == identity.Alive; any other verdict — including identity.Unknown (probe failed, e.g. transient sysctl error) — falls through to the takeover path under the comment "The owner is provably dead". identity.Liveness is three-way and its own doc says Unknown never authorizes; the package comment above says "Takeover requires proven death"; internal/lock's doc says "Uninspectable is alive"; dispatch's holderState (ownerlock.go:104) correctly maps unknown → Busy. censuslock_test.go never exercises Unknown (its fake prober only returns Alive/Dead), so the gap is untested.

**Target:** Refuse takeover unless AliveRef returns exactly identity.Dead; add the Unknown row to censuslock_test.go beside the live/dead rows.

**Why:** A transient probe failure against a live census writer steals its lock and produces two concurrent publishers of last-census.json — the split-brain the lock exists to prevent, flapping the generation/stateDigest attestation dispatch gates on. This is the one place in the reviewed tree where indeterminacy authorizes an action.

#### dispatch-supervise-1 — dispatch→supervise edge is inverted; record-budget semantics live in the wrong package

`internal/dispatch/reapfacts.go:84` · design · **medium / effort M**

> downgraded by the verifier: Facts all verified: reapfacts.go:84 is the only dispatch->supervise use (grep confirms no other non-test import), CapExpired interprets fields dispatch's own record.go declares immutable (capMin, capDeadline, startedAt), and both reaper.go:50-53 and cmd/metasystem/supervise_component.go:235-236 explicitly name the cycle as the reason for the injected CAS. The inversion is real and its costs are concrete (finding 2's duplicated constant is a direct consequence). Downgraded from high: the current arrangement is behaviorally correct and the injection seam doubles as a legitimate test seam; the demonstrable harm today is one duplicated constant and a documented workaround, which is medium-grade structural debt, not a high-severity defect.

**Current:** The only dispatch→supervise import is reapfacts.go:84 calling supervise.CapExpired. The budget verdict — pure interpretation of record fields dispatch defines (capDeadline, capMin, startedAt) — lives in supervise/reaper.go, and because supervise cannot import dispatch (it would cycle), the reaper takes the record CAS by injection (ReaperConfig.Apply) and hardcodes the abandoned-setup grace.

**Target:** Move CapExpired (and the abandoned-setup grace, see finding 2) into dispatch, the package whose doc claims ownership of the job-record lifecycle. Give ReaperConfig an injected budget/grace verdict alongside Apply, wired at cmd — or, once the cycle is gone, let supervise import dispatch directly (policy→mechanism, the correct direction) and drop the Apply seam too.

**Why:** Layering: supervise is the policy loop, dispatch is the record mechanism; mechanism importing policy to fetch a record-interpretation function is backwards, and the workaround costs are visible today (injected CAS, duplicated constant). The comment in reaper.go:50-52 already acknowledges the cycle as the reason for the injection.

#### dispatch-supervise-2 — Abandoned-setup grace duplicated despite an exported single-source constant

`internal/supervise/reaper.go:96` · duplication · **medium / effort S**

> downgraded by the verifier: Confirmed factually: reaper.go:96 hardcodes now.Sub(at) > 10*time.Minute; dispatch.AbandonedSetupGrace (reapfacts.go:36) is exported with a doc comment promising it is THE setup grace binding the standing reaper and missionrunner/drain.go:111 — and the standing reaper (supervise.ReaperPass) is exactly the consumer that cannot import it due to the cycle. Downgraded from high: both values agree today, so there is no behavioral defect, only a latent desync trap for whoever tunes the constant. A booby trap with an S-effort fix is medium, not high; high should be reserved for live correctness risk like finding 3.

**Current:** reaper.go:96 hardcodes `now.Sub(at) > 10*time.Minute`. dispatch.AbandonedSetupGrace (reapfacts.go:36) is exported with the doc comment "Exported because it is THE setup grace: the standing reaper's verdict and the mission runner's drain clock must measure the same window" — but the standing reaper, the primary consumer that comment names, cannot import it because of the dispatch→supervise cycle (finding 1).

**Target:** One constant, one home (dispatch, the record owner), consumed by the reaper via the same injection seam as Apply or via a direct import once finding 1 reverses the edge.

**Why:** Anyone tuning AbandonedSetupGrace will silently desynchronize the standing reaper from the mission runner's drain clock — the exact disagreement the constant's own documentation promises cannot happen. The duplication is invisible at the call site.

#### dispatch-supervise-4 — Rename-born directory-lock protocol implemented three times

`internal/dispatch/ownerlock.go:74` · duplication · **medium / effort L**

**Current:** The staged-directory claim (MkdirTemp + owner.json + rename), dead-owner husk takeover, and identity-checked retiring release exist in internal/lock (the documented canonical owner, per its package comment), again in dispatch/ownerlock.go (OwnerLockClaim/Release, with a stale-holder classification), and again in supervise/censuslock.go (CensusWriterLock). The three already disagree: censuslock takes over on Unknown (finding 3), ownerlock refuses, internal/lock documents refusal plus a bounded-wait ownerless-husk rule the other two lack.

**Target:** internal/lock as the single home: extend it with the tag-based stale-holder classification dispatch's owner lock needs, then reduce ownerlock.go and censuslock.go to thin bindings (owner-file schema plus paths).

**Why:** Three hand-rolled copies of a subtle crash-safe locking protocol guarantee semantic drift — finding 3 is the drift already shipped. The correct discipline is written once, in internal/lock's package doc, and then not used by two of its three implementations.

#### dispatch-supervise-5 — Unguarded fault-injection hook in the production mirror path

`internal/dispatch/mirror.go:45` · boundary · **medium / effort S**

**Current:** Mirror() unconditionally checks for a `.mirror-fail-once` file in the chain payload and fails the mirror once when present (writing `.mirror-failed`). The hook is written by scripts/agents/adapters/fake.sh for fixtures, but the Go check runs in every checkout, real runtimes included. Contrast mission.go's fixtureCommand, which refuses its fixture unless metasystem.runtimes=fake.

**Target:** Gate the hook the way fixtureCommand gates its fixture: honor `.mirror-fail-once` only when the checkout's sole configured runtime is fake (or behind an explicit env var), so production Mirror has no scripted-failure branch.

**Why:** The payload directory is delegate-writable territory; a stray or malicious `.mirror-fail-once` makes evidence mirroring fail in a real checkout, and production behavior diverges from the documented contract because of a test-only branch. The package already established the right guard pattern and this path skips it.

#### dispatch-supervise-6 — GroupCount stub makes the production ceiling verdict unreachable

`internal/supervise/proc.go:229` · design · **medium / effort M**

**Current:** processGroupMembers counts only the group leader (kill(pgid,0) → 1 or 0), so ProcComponents.GroupCount can never exceed len(held) ≤ 2, while cmd wires Ceiling: 12. CeilingVerdict — the SLC-R4-010 "duration property" proven in owner_test via an injected count — can therefore never fire against real processes; a forking component set grows unbounded as far as this bound is concerned. The inline comment acknowledges the stub and defers to census enumeration. Additionally, Owner.Cycle runs the ceiling check only when GroupCount returns err == nil, so once enumeration is real, a count error would be silently ignored and supervision would continue normally.

**Target:** Real enumeration: AllPids() + Getpgid per pid inside processGroupMembers — about ten lines, no new platform code, no import cycle; precedent at internal/census/production.go:41, but do NOT copy its error handling, which collapses Getpgid failures to PGID 0. Only ESRCH counts as an absent member; an AllPids or any other Getpgid failure makes the count indeterminable, and Owner.Cycle maps that error to Indeterminable instead of ignoring it — a process-table denial must not undercount while the breaker resets. Test both error paths. The documentation-only option is withdrawn. [resolved R1.13, tightened R2.3]

**Why:** A safety bound that exists only under test injection is the metasystem's own "prose mistaken for enforcement" failure class: the Proof row passes while the production property it names is vacuous.

#### dispatch-supervise-7 — Two chain-ancestry walkers with divergent cycle verdicts

`internal/dispatch/chain.go:24` · duplication · **medium / effort M**

**Current:** chainMembers (chain.go) resolves each record's root by re-reading parent records from disk per step, and on a parent cycle breaks mid-walk — so a member of a cycle can still be attributed to whatever record the walk stopped on. critiqueState.chainRoot (critique.go:47) walks an in-memory table and returns "" for any cycle or malformed parent. Same concept, two homes, different verdicts on corrupt lineage, and the disk version is O(members × depth) file reads.

**Target:** One lineage walker over a loaded record table (the critiqueState shape) in chain.go, used by chainMembers, LatestChainRecord, ChainMemberStatuses, CloseCheck, and the critique state — with one documented verdict for cycles and malformed parents.

**Why:** Chain membership feeds CloseCheck (evidence durability) and critique-budget enforcement; two walkers that can classify the same corrupt chain differently means those two gates can disagree about which records a chain contains.

#### dispatch-supervise-8 — Launch doc comment describes a launcher-shell architecture the code does not implement

`internal/supervise/proc.go:63` · docs · **medium / effort S**

**Current:** The comment block above Launch says the component is "backgrounded inside a launcher shell that immediately exits", reparents to launchd, "prints the component's pid on stdout", and "the owner Waits on the (short-lived) launcher". The code does none of this: it execs argv directly with Setsid, treats launcher.Process as the component itself, never reads stdout, and keeps the component as a direct child reaped via reap() — as the later inline comment ("The launched process IS the component here") admits.

**Target:** Rewrite the Launch doc to describe the actual mechanism: direct setsid child, tracked in children, zombies cleared by reap() so death is provable. Keep the zombie rationale, drop the launcher-shell narrative.

**Why:** This is process-lifecycle code where the doc's claimed reparenting story would send a maintainer debugging zombies or teardown in exactly the wrong direction; the two halves of the same function currently contradict each other.

#### dispatch-supervise-10 — dispatch package doc describes one of the package's ten responsibilities

`internal/dispatch/record.go:1` · docs · **low / effort S**

> downgraded by the verifier: Confirmed factually: record.go:1-5 describes only the record lifecycle, and the package verifiably also carries attestation gates (attest.go), permissions expansion (envelope.go), mission-lease proof (mission.go), mirroring/close proof (mirror.go, close.go), critique-exhaustion policy (critique.go), the owner lock (ownerlock.go), brief/cap parsing (brief.go), and chain usage (usage.go). Downgraded from medium: the doc is under-scoped rather than wrong — what it says about the record lifecycle is accurate, and file names still route a reader correctly — which misleads far less than finding 8's affirmative false mechanism. The S doc rewrite is clearly worthwhile; the optional split half of the target should be treated as needing its own justification, not ridden in on a doc fix.

**Current:** The package comment says dispatch "owns the job-record lifecycle: the single writer of a job's control-plane record" — accurate for record.go/build.go/custody.go, but the package also carries the census/watcher attestation gates (attest.go), permissions expansion (envelope.go), mission-lease identity proof (mission.go), evidence mirroring and close proof (mirror.go, close.go), critique-exhaustion policy (critique.go), the supervision owner lock (ownerlock.go), cap resolution and brief parsing (brief.go), and chain usage aggregation (usage.go).

**Target:** State the package's real purpose — the decision core behind every `metasystem dispatch/job` verb: record lifecycle plus the launch gates, evidence duties, and chain policies dispatch.sh delegates — or split the non-record halves (attestation, mirroring, critique policy) into named packages if the grab-bag keeps growing.

**Why:** Rubric 1: the doc is the contract a reader trusts; here it actively misleads about where launch-gating and evidence policy live. The cohesion story ("what dispatch decides") is defensible but is nowhere written down.

#### dispatch-supervise-11 — RetireWatermark gap branch is dead: both arms return the same value

`internal/supervise/decide.go:223` · dead-code · **low / effort S**

**Current:** Inside `if !contains(recorded, next)`, the nested `if next > highest(recorded) { return watermark }` is followed by an unconditional `return watermark` — the conditional and the comment's claimed distinction ("absence past the highest recorded number ends the walk") change nothing.

**Target:** Collapse to a single `return watermark` with a comment saying a gap and walk-past-the-end are the same verdict, or make the distinction real if one was intended (tests pin the current behavior either way).

**Why:** Dead conditional in a correctness-critical watermark function invites the next reader to hunt for a semantic difference that does not exist.

#### dispatch-supervise-12 — Backoff formula in doc comments is off by one against the pinned behavior

`internal/supervise/decide.go:143` · docs · **low / effort S**

**Current:** Breaker's doc and backoff()'s doc both say "relaunch k waits interval × 2^(k-1), capped", but the implementation (pinned by TestBackoffSchedule) gives 0 for k=1 and interval × 2^(k-2) for k≥2 (k=2→interval, k=3→2×interval, …).

**Target:** State the actual schedule: first failing observation relaunches immediately; the k-th consecutive failing observation waits interval × 2^(k-2), capped.

**Why:** D-6's numbers are load-bearing for anyone tuning supervision; a formula that disagrees with the tested behavior will be trusted over reading the loop.

#### dispatch-supervise-13 — ValidateMission hand-rolls liveness with EPERM-as-dead, inconsistent with the identity owner it calls two lines later

`internal/dispatch/mission.go:57` · boundary · **low / effort S**

**Current:** ValidateMission pre-checks liveness with raw unix.Kill(pid, 0), where any error — including EPERM (alive, other uid) — reads "mission lease holder is not alive"; then processCommand probes the same pid through identity.KernelProber anyway. ownerlock.go's holderState in the same package maps EPERM to live, and the identity package's whole point is owning three-way liveness.

**Target:** Drop the raw Kill pre-check and derive liveness from the single KernelProber probe processCommand already performs (Dead → "not alive", Unknown → the fixture fallback), keeping unix.Getpgid for the group check.

**Why:** Duplicated liveness logic with different EPERM semantics inside one package is exactly the drift the identity owner exists to prevent; the redundant syscall also makes the refusal ladder harder to reason about.

#### dispatch-supervise-14 — ExpandPermissions silently drops non-string root entries

`internal/dispatch/envelope.go:55` · error-handling · **low / effort S**

**Current:** expandAll keeps only entries that type-assert to string, so a malformed envelope (e.g. a number or object in readRoots/writeRoots) passes the strict five-key shape check yet loses roots with no signal — while every other malformation in this function is a named refusal.

**Target:** Refuse a root entry that is not a string, matching the function's own "must contain exactly …" strictness (stringMembers in handshake.go already implements the right check).

**Why:** This is the permission surface: a typo'd envelope should fail loudly at expansion, not launch a delegate with silently fewer roots than the envelope's author believes were requested, then fail obscurely at handshake or runtime.

#### dispatch-supervise-9 — JobRecord typed lens is exported but has no production caller

`internal/dispatch/jobrecord.go:14` · dead-code · **low / effort S**

> downgraded by the verifier: Confirmed factually: grep shows JobRecordOf's only callers are jobrecord.go itself and jobrecord_test.go; chain.go/critique.go/close.go still do raw asString casts, and missionrunner/turnrecord.go is a parallel lens whose own comment cites dispatch's JobRecord as its model. Not tool-duplicative (staticcheck U1000 does not flag exported identifiers). Downgraded from medium: an exported-but-unused internal API has zero runtime impact and the two-access-styles confusion is mild; this sits in the same tier as the other no-behavior findings (11, 12) rather than beside the unguarded fault hook or the vacuous ceiling. The S-effort adopt-or-remove target is right.

**Current:** JobRecord/JobRecordOf (the Phase 5.1 typed read lens) is used only by its own test. The decision paths it was designed for still do raw asString(record["status"]) casts (chain.go, critique.go, close.go), and missionrunner grew its own parallel lens (turnrecord.go) rather than sharing this one.

**Target:** Either adopt the lens in this package's own readers so the per-field cast rules have the one home its comment promises, or unexport/remove it until the migration step that uses it lands.

**Why:** An exported, tested, unused API is speculative surface: it advertises a migration that has not happened, and every reader must now check which of the two access styles is authoritative. Adopting it would also delete a dozen scattered ill-typed-field decisions.

### internal/adapter, internal/host, internal/wiredoc, internal/registry

*Verdict: does not yet pass the senior-engineer bar. 9 findings (1 high, 4 medium, 4 low).*

**Reviewer's summary.** Runtime adapters and host abstraction: internal/adapter (delegate-job plumbing behind the "adapter" CLI family: permission envelopes, capability snapshots, return normalization, per-runtime usage/config builders for claude/codex/devin/fake), internal/host (the mission-turn twin behind the "host" family: result envelope, per-runtime return/usage extraction, fake fixtures), internal/wiredoc (the frozen-grammar decoder and two-dialect canonical encoder for every persisted JSON document), and internal/registry (the machine-wide supervision custody ledger: JSONL framing with torn-tail repair, per-event schema validation, fold/reduction, compaction, slot accounting).

Overall state: strong, with one structural debt. registry is exemplary — contract-as-code with clause citations, fail-closed corruption handling, fault-injected durability tests, crash-storm replay of the repair rule; a senior engineer would sign off on it nearly untouched. wiredoc clearly earns its place as the shared dependency of adapter, dispatch, host, and missionrunner: it is a tiny single-owner mechanism (frozen accepted-input grammar plus the two on-disk dialects), corpus-pinned in each consumer, and exactly the kind of package that prevents four families from drifting byte-wise; its one gap is that adapter and host can only reach the canonical encoder through a marshal-decode-render detour, which invites the private helper duplication found here. The adapter/host overlap is the real issue: the two-package split is justified at the surface (two shell families with different permission policies — job-scoped envelopes vs workspace-write turns), but the implementation layer was ported twice. Devin's cumulative-delta usage metering exists as two near-identical ~90-line copies, Claude's typed-usage extraction twice, plus six smaller helper families (canonical render, atomic JSON write, asInt/asFloat/isNumber, sorted keys, XDG devin-config resolution, symlink-free path resolve). A fix to fence metering in one package will silently miss the other. The consolidation is internal only — both CLI verb families and all on-disk bytes stay as they are — so no finding here touches a contract. Tests across all four packages are genuinely good: behavior-asserting, table-driven where natural, error paths covered, corpus goldens for wire bytes, no timing-flaky patterns.

#### adapter-host-registry-1 — Devin usage metering and Claude usage extraction duplicated wholesale between adapter and host

`internal/host/devin_usage.go:23` · duplication · **medium / effort M**

*Same defect as cli-3 and architecture-1; one backlog item (W2.1).*

**Current:** host.DevinUsage (internal/host/devin_usage.go:23-108, plus devinUsageFields and providerUnit) is a near-verbatim copy of adapter.DevinUsage (internal/adapter/devin.go:206-291, devinUsageFields, devinProviderUnit): same signature, same cumulative-delta algorithm, same ACU handling, same unavailable-on-missing-predecessor rule. host.ClaudeResult (internal/host/claude.go:23-39) likewise re-implements adapter.ClaudeUsage's typed-usage map (internal/adapter/claude.go:78-97) field for field. Both copies are live: cmd wires adapter.DevinUsage to 'adapter devin-usage' and host.DevinUsage to 'host devin-usage'.

**Target:** One implementation of each runtime's usage extraction, with both CLI families calling it, in a small leaf package — see architecture-1 for the adjudicated shape (the leaf, not host-imports-adapter, because mission/fence.go must also come off its adapter import); the differing lenient-read wrappers stay in each caller. Output bytes and both verb surfaces are unchanged. Same issue and severity as architecture-1 (R1.11); one backlog item, W2.1.

**Why:** This is fence/spend metering — the code the mission budget trusts. The two copies have already started to drift trivially (isNumber vs asFloat acceptance in the ACU probe); the next real fix (a new Devin metric spelling, a double-count bug) applied to one copy will silently leave the other wrong, and nothing will fail loudly because each package's tests only see its own copy.

#### adapter-host-registry-2 — Private JSON plumbing duplicated between adapter and host; wiredoc should expose a value renderer

`internal/host/host.go:21` · duplication · **medium / effort M**

**Current:** host.canonicalJSON/atomicWriteJSON (internal/host/host.go:21-46) are byte-identical to adapter.encodeJSON/atomicWriteJSON (internal/adapter/adapter.go:60-85), both doing the json.Marshal -> wiredoc.Decode -> Render detour because wiredoc only renders from a decoded Doc. Also duplicated: asInt/asFloat/isNumber/sortedKeys (host/host.go:96-132 vs adapter/runtime.go:126-180), the XDG devin-config resolution (host/devin.go:37-48 vs adapter/devin.go:79-87), and resolvePath vs resolve (host/devin.go:81-89 vs adapter/adapter.go:96-104). Even the bytecheck tests are copy-pasted (host/bytecheck_test.go and adapter/bytecheck_test.go are identical files).

**Target:** wiredoc grows a RenderValue(any) ([]byte, error) (marshal-seed-decode-render in one owned place, corpus-pinned once), and the atomic canonical write plus the json.Number coercion helpers get one home both packages import — either alongside RenderValue in wiredoc or with the finding-1 consolidation. The devin config-path and path-resolve helpers move with the finding-1 runtime core.

**Why:** Six helper families maintained twice between sibling packages that cannot see each other is how dialect drift starts; the wire-documents doctrine ('structs never marshal onto the wire') is currently enforced by two private copies of the same detour instead of by the package that owns the canon.

#### adapter-host-registry-3 — SelftestEnvelopeDeclaration re-implements 'newest snapshot' with filename-lexical order, diverging from capability's rule

`internal/adapter/selftest.go:66` · duplication · **medium / effort S**

**Current:** SelftestEnvelopeDeclaration globs runtime+'-*.json' and takes the last parseable match in filepath.Glob's lexical order as 'newest'. Snapshot names are runtime-version-confighash-date-seq, so lexical order sorts by CLI version and config hash before date. internal/capability/select.go:219 already owns the correct rule: newest by (capturedAt, filename).

**Target:** Select the newest snapshot by capturedAt (reuse or mirror capability's newest()), not by filename order. Also note the glob runtime+'-*.json' would match a different runtime whose name extends this one's prefix; capability's version-pinned glob avoids that too.

**Why:** After a CLI upgrade like 1.9 -> 1.10, the old version's snapshots sort lexically last, so the self-test reads a stale envelope declaration: a stale 'mapped' makes the self-test demand a denial the current runtime cannot produce (spurious failure), and a stale 'notEnforced' skips behavioral proof the current runtime could have given. The tests only cover a fixed version/hash prefix, where lexical happens to equal chronological.

#### adapter-host-registry-4 — AppendFrame accepts non-object JSON payloads that the reader classifies as corruption

`internal/registry/framing.go:41` · error-handling · **medium / effort S**

**Current:** AppendFrame guards with json.Valid plus a newline check, so a payload like [1,2] or "x" or 123 appends successfully. ReadFrames (framing.go:196-203) additionally requires the line to unmarshal as an object: a valid non-object line becomes a tolerated fragment that sets garbageLine, and the next valid record then trips CorruptionError. inspectTail also treats such a tail as 'fully written' (json.Valid at framing.go:62), so no later append ever fences it with a torn marker.

**Target:** The append guard matches the reader's acceptance: refuse any payload that is not a JSON object (unmarshal into map[string]any or equivalent). One-line tightening of an internal API; the on-disk format is unchanged.

**Why:** One buggy internal caller marshaling the wrong value turns the machine-wide registry permanently fail-closed (arming and reaping refuse, REG-5) with no self-repair path, when the append could have refused loudly at the door — which is exactly this codebase's own doctrine. Torn writes of object payloads can never produce this shape (a strict prefix of an object is invalid JSON), so today the writer guard is strictly weaker than the reader for no benefit.

#### adapter-host-registry-6 — WriteCapabilitySnapshot takes nine consecutive string parameters

`internal/adapter/snapshot.go:29` · complexity · **medium / effort S**

**Current:** func WriteCapabilitySnapshot(dir, runtime, version, configHash, transports, capabilities, permissions, envelope, keyHashes string) — nine positional same-typed parameters, five of which are JSON blobs. A transposition of any two blob arguments compiles and, since parseBlob accepts any valid JSON, several transpositions (e.g. transports/permissions) would validate and write a structurally wrong snapshot. WriteSelftestRecord (selftest.go:88) has the same shape at seven parameters including a behavior-forking bool (devinChecks).

**Target:** A SnapshotInput struct (identity fields plus named blob fields), and for WriteSelftestRecord a small options struct that names devinChecks. Internal API only; the CLI flags in cmd already carry the names.

**Why:** Snapshots are what later dispatches trust to decide whether a runtime can hold a restrictive permission; a silently field-swapped snapshot is exactly the class of error a parameter struct makes impossible, and the happy-path test pins only the correct ordering.

#### adapter-host-registry-5 — Return normalization's per-brace decode plus recursive re-walk is quadratic on large transcripts

`internal/adapter/return.go:147` · complexity · **low / effort M**

> downgraded by the verifier: The code reading is accurate (per-brace decode at return.go:147-154, re-walk plus embedded-string re-parse at nestedValues 113-124), but the exposure is materially overstated. Claude and Codex adapters pass NO transcript to normalize-return (claude.sh:187 and codex.sh:185 hand complete_from_cli only the candidate — claude-result.json and Codex's raw.out); the 'tens of MB Claude/Codex transcripts' and the 32MB scanner buffer belong to jsonlObjects on the Codex event path, not this function. Only devin.sh:411 passes a transcript, and it is transcript.atif.json — a single JSON document, so the fix's central detail ('parse per line, it is JSONL') misreads the one source that actually flows in. What remains is a real superlinear (depth-times-size) pattern on Devin's growing session transcript, worth bounding, but it is not the mission-loop-minutes defect claimed. Low.

**Current:** parseEmbeddedJSON attempts a fresh json.Decoder parse from every '{' byte in the text (return.go:147-154), and bestReturnCandidate then runs nestedValues over every one of those parses — which itself re-runs parseEmbeddedJSON on every string member containing '{' (return.go:113-124). For a JSONL transcript, every nested object is parsed once per enclosing brace and then re-walked, so work grows roughly with depth times size.

**Target:** Bound the search: try the whole-text parse and fenced blocks first and stop when a full-score candidate (all eight required fields) is found; for the transcript source, parse per line (it is JSONL) instead of brace-scanning the concatenated text; skip brace offsets that fall inside an already-parsed value.

**Why:** NormalizeReturn runs on every delegate round and is handed the full transcript; real Claude/Codex transcripts reach tens of MB (jsonlObjects already provisions a 32MB scanner buffer). A multi-MB transcript with hundreds of thousands of braces turns a per-round bookkeeping step into minutes of CPU and large allocation churn inside the mission loop.

#### adapter-host-registry-7 — adapter's package doc describes a boundary the package no longer keeps

`internal/adapter/adapter.go:1` · docs · **low / effort S**

> downgraded by the verifier: Factually correct: the package comment (adapter.go:7) claims command lines, event parsing, and identity 'stay in each adapter; only the reusable core lives here', while BuildCodexCommand (codex.go:75), CodexEventField (codex.go:13), and DevinSessionCorrelate (devin.go:96) live in this very package — a pre-port leftover that contradicts the decisions-in-Go doctrine. The stacked stale comment on encodeJSON (adapter.go:54-59) is also confirmed. But the whole fix is rewriting two comments with zero behavior impact, and the misdirection is dispelled by the package's own file listing; medium overweights it relative to the behavioral findings in this batch. Low.

**Current:** The package comment says 'The runtime command lines, event parsing, and identity stay in each adapter; only the reusable core lives here' — but claude.go, codex.go, and devin.go in this same package carry exactly those: BuildCodexCommand assembles argv, CodexEventField parses the event stream, DevinSessionCorrelate resolves identity. Additionally encodeJSON (adapter.go:54-59) carries two stacked doc comments, the first describing the pre-Phase-5.3 encoder.

**Target:** Rewrite the package comment to state what the package actually owns now (the Go core of every runtime adapter: lifecycle plumbing plus each runtime's command construction, event parsing, and usage extraction, with the shell adapters as thin drivers), and drop the stale half of encodeJSON's comment.

**Why:** The package doc is the responsibility anchor a reviewer and the next contributor navigate by; this one sends them to the shell scripts for logic that lives here, in a codebase whose explicit doctrine is that decisions live in Go.

#### adapter-host-registry-8 — LockedAppend double-wraps AppendFrame's already-prefixed errors

`internal/registry/append.go:30` · error-handling · **low / effort S**

**Current:** LockedAppend wraps with 'registry append: %w' while every error AppendFrame returns is already prefixed 'registry append:' (or 'registry open:'/'registry tail inspection:'), producing 'registry append: registry append: not durably written: ...'.

**Target:** Return AppendFrame's error unwrapped from LockedAppend (its prefixes already name the operation), keeping the distinct 'registry directory:' and 'registry lock:' wraps.

**Why:** Stuttered prefixes make the ledger's carefully worded refusals read as noise precisely where operators are meant to trust the message (D-5: an error message is part of how the registry explains itself).

#### adapter-host-registry-9 — The dated-artifact timestamp format is re-spelled instead of using the existing helper

`internal/adapter/selftest.go:111` · duplication · **low / effort S**

**Current:** WriteSelftestRecord (selftest.go:111) and WriteCapabilitySnapshot (snapshot.go:72) each inline now().UTC().Format("2006-01-02T15:04:05")+"Z" while fake.go:22 already defines timestampUTC as the one spelling of 'how every dated artifact in this system is stamped'.

**Target:** Both call timestampUTC, and the helper moves out of the fake-runtime file (it is not fake-specific) into adapter.go beside now.

**Why:** Three spellings of a wire-visible timestamp format is how one of them eventually gains a fractional second or loses the Z; the codebase already named the rule in a helper, so the other two writers should be unable to drift from it.

### internal/lease, internal/census, internal/lock, internal/events, internal/identity

*Verdict: passes the senior-engineer bar. 12 findings (2 high, 7 medium, 3 low).*

**Reviewer's summary.** Coordination-primitives cluster of the metasystem Go CLI: internal/lease (checkout write-authority: announcements, claim/succession/takeover state machine, caller classification, stale-job sweep), internal/census (process classification: signature matching, scope resolution, fixture and production census runs, supervision fingerprint), internal/lock (born-owning directory lock with death-only takeover), internal/events (flight-recorder emitter, capped append-only JSONL), internal/identity (kernel-fact process identity: three-way liveness probers for darwin/linux, custodian verdict, procfs restriction detection). Overall state: strong. The design doctrine (proof over assertion, three-way liveness where Unknown authorizes nothing, fuse proof and action) is genuinely enforced in code, not just prose — lock's fenced takeover, identity's B1/B2 rows, and lease's KI-32/KI-33 fixes all have focused tests, including race-replay tests for the two-winners takeover and the ownerless-window property. Documentation is exemplary; error messages name themselves. The findings are where the tree contradicts its own doctrine: two fail-open I/O paths in lease (unreadable supervision/job state silently upgrades a caller to HUMAN; an unreadable job record lets the sweep certify an epoch it did not clear), and a family of drift-prone duplications — the fake-identity fixture file is parsed in five places with two different key spellings (shell fixtures already write both keys to cope), the mains-directory announcement schema is parsed independently and with divergent strictness in lease and census, and the fixture/production census runners duplicate their orchestration skeleton. Remaining items are hardening (events envelope clobber, unguarded fixture env var in production liveness verdicts, unbounded record-lock inside the deliberately-bounded lease lock) and polish. All tests pass; gofmt/vet/staticcheck-clean was verified by the caller and nothing here restates linter output.

#### lease-census-1 — Unreadable supervision state or job records fail open to HUMAN, which bypasses the holder gate

`internal/lease/classify.go:203` · error-handling · **high / effort M**

**Current:** custodyIdentities ignores os.ReadFile failures: `if data, readErr := os.ReadFile(statePath); readErr == nil { ... }` has no else branch, and job-record read errors at line 225 `continue`. A parse failure refuses classification loudly ("caller classification refused"), but a read failure (EACCES, EIO) silently yields empty custody maps. Classify then finds no recognised ancestor and returns HUMAN — and RequireHolder (verbs.go:193) returns {"holder": true} for HUMAN, so the caller passes the write gate ungated.

**Target:** Distinguish absence from failure across all of classification's inputs: os.IsNotExist on state.json (supervision not armed) keeps the current skip; any other read error refuses classification exactly like the existing unmarshal path does. Job records: ENOENT (glob race) skips; any other read error refuses; and corrupt records — unmarshal failure or empty jobId (classify.go:234) — refuse like corrupt state instead of silently dropping custody, since a fix covering only ReadFile errors still fails open on corruption. The identity/ancestry half of the same pattern is codex-3. The refusal tests land in the same change as the fix (R2.6). [expanded R1.3]

**Why:** This inverts the tree's own doctrine ("fail toward the fuse, refuse loudly, never skip silently") at the single most security-relevant decision: a permissions mishap on state.json turns a SUPERVISION-descended helper into a HUMAN with full write authority. The neighbouring parse-failure branch already fails closed, so the asymmetry is clearly unintended.

#### lease-census-2 — Sweep certifies an epoch complete despite unreadable or unparseable job records

`internal/lease/sweep.go:66` · error-handling · **high / effort M**

**Current:** sweepOne treats every os.ReadFile error as "vanished under us; nothing to sweep" (return false, nil) and silently skips records that fail json.Unmarshal (line 69). cleanupStaleJobs then emits sweep-completed and the caller stamps the epoch. Yet the function's own contract comment (lines 30-32) says "A job it cannot prove ownership of is a hard error: the sweep must not certify a generation it did not actually clear."

**Target:** Only os.IsNotExist means vanished; any other read error, an unparseable record, and a schema-invalid record that parses as JSON — missing or noninteger claimEpoch, missing or unknown status (sweep.go:72-75) — is a hard error that aborts the sweep before the stamp is written, the same refusal shape stopStaleGroup already uses for unprovable ownership. In groupOwnsTag (sweep.go:131-140), only ESRCH counts as member absence: any other Getpgid or identity error yields provable=false instead of "provably not owned". The process-table seam and these refusal tests land in the same change as the fix (R2.6). [expanded R1.4]

**Why:** An EACCES-unreadable or corrupted stale job record means its process group is never TERMed and its record never failed, but the reaped-after-claim stamp still certifies the new epoch — so RequireHolder admits the new generation while the predecessor's job may still be writing. That is precisely the duplicate-writer scenario the lease exists to prevent, and the code contradicts its own stated contract.

#### lease-census-10 — The sweep's safety refusals — its most safety-critical branches — are untestable and untested

`internal/lease/sweep.go:107` · testing · **medium / effort M**

**Current:** The refusal paths that prevent killing innocents are not covered: groupOwnsTag's provable=false branch (requires identity.AllPids to fail), stopStaleGroup's EPERM refusal (line 116), and the cannot-prove-ownership hard error (line 107). They are unreachable from tests because the process-table access is hardwired to the real kernel; sweep_test.go covers only the skip and happy paths.

**Target:** A narrow seam for the sweep's process-table reads (package-level var or small interface at point of use, matching how identity's fakeProber tests the three-way rows), then one test per refusal row, in the style of custodian_test.go's verdict table.

**Why:** These branches are the fuse: they are what stands between a takeover sweep and SIGTERM-ing a recycled process group. Every other refusal in this cluster (lock takeover, custodian rows, classification tampering) has a focused test; the sweep's do not, so a regression that flips a refusal into a kill would land green today.

#### lease-census-3 — The fake-identity fixture file is parsed in five places with two different key spellings

`internal/census/verbs.go:29` · duplication · **medium / effort M / needs sign-off**

**Current:** The METASYSTEM_FAKE_PROCESS_IDENTITY_FILE table is parsed by census.AuthIdentity (key "started", verbs.go:29), census.identityAlive (key "pidStartedAt", run.go:351), identity.Custodian (key "pidStartedAt", custodian.go:32), missionrunner/proc.go and dispatch/mission.go — each with its own inline anonymous struct. Shell fixture writers already pay the drift cost: delegate-caps-fixtures.sh:213 and supervision-fixtures.sh:546 write BOTH "started" and "pidStartedAt" into every entry so that all readers see it.

**Target:** One exported fixture-table reader in internal/identity (the leaf every consumer already imports) returning a typed struct, accepting both spellings during transition; retire the "started" spelling with owner sign-off so fixtures write one key.

**Why:** Five independent parsers of one cross-process file format is the textbook two-fixes-diverge shape the atomicfile package doc warns about in this same tree. A fixture written for one reader silently misses in another (falls through to the kernel), producing test verdicts that differ by which reader ran — a bug class that is miserable to diagnose because everything "works" until the key spelling matters.

#### lease-census-4 — The mains-directory announcement schema is parsed independently in census and lease, with divergent strictness

`internal/census/run.go:439` · duplication · **medium / effort M**

**Current:** census.announcementsList (run.go:436-489) maintains its own skip list of non-announcement files, expected/optional key sets, and mainId/commandHash regexes; lease.readAnnouncements (classify.go:45-128) maintains a parallel nonAnnouncementFiles map, field list, and the same two regexes (classify.go:39-40 vs run.go:76-77). They disagree on strictness: census rejects any UNKNOWN key (run.go:467-471) while lease tolerates unknown keys.

**Target:** One home for the mains-directory record contract — a leaf package (lease imports census, so census cannot import lease) owning the file classification, schema validation, and identity regexes; both consumers use it.

**Why:** The optional-key list in census had to learn ownerLineage when lease grew it; the next Announcement field that misses that second edit turns every census run CENSUS-FAILED with announcement-schema errors and alarms supervision — a production incident caused purely by double-maintained schema knowledge. The duplicated skip lists (worktree-lease.json etc.) have the same failure mode for the next protocol file added to the directory.

#### lease-census-6 — A payload field named like an envelope field silently clobbers the event envelope

`internal/events/emit.go:98` · error-handling · **medium / effort S**

**Current:** emitWithSeq builds the required envelope into record, then copies every remaining caller field over it: `for name, value := range args { record[name] = clip(value, caps["payload"]) }`. Only "level" (deleted at line 87) and "summary" (reassigned at line 101) are protected; fields{"pid": "x"}, "seq", "ts", "component", "event", "schemaVersion" or "pidStartedAt" would replace the emitter's kernel-fact identity with a caller string.

**Target:** Skip (or prefix-rename) payload keys that collide with requiredEnvelope and id-field names in the copy loop — shrink() already treats those keys as inviolable, so the framing layer should too.

**Why:** The whole package contract is that the stream is a trustworthy witness whose envelope carries kernel facts; one careless Emit call site turns pid into an attacker-of-its-own-record string and breaks every replay that joins on (pid, pidStartedAt, seq). The guard is three lines and the protected-set already exists for shrink().

#### lease-census-7 — The identity fixture env var is honored unconditionally in production liveness verdicts, unlike the process-table fixture

`internal/identity/custodian.go:29` · boundary · **medium / effort M**

**Current:** The census process-table fixture is guarded — enumerateFixture refuses unless metasystem.runtimes=fake (run.go:247-249). But METASYSTEM_FAKE_PROCESS_IDENTITY_FILE is honored with no guard in identity.Custodian (custodian.go:29), census.identityAlive (run.go:348) and census.AuthIdentity (verbs.go:26): kernel death still vetoes, but for a live pid the fixture's start time overrides the kernel's.

**Target:** Gate the fixture override on the fake runtime the way enumerateFixture does (the check can live at the CLI/census layer that can read metasystem.conf, or refuse at arming time when the env var is set outside fake runtimes), and say so in the package doc.

**Why:** Pid-reuse detection is the load-bearing proof in every reap/sweep/takeover decision. If the env var leaks into an armed supervision component (supervision-fixtures.sh exports it; components inherit the arming shell's environment), a recycled pid whose fixture entry matches the recorded start reads Alive for a stranger — a dead holder's lease or custodian kept alive indefinitely by a stale test file, with no refusal anywhere. The tree already recognised this class of risk for the process-table fixture and guarded it; the identity fixture deserves the same fence.

#### lease-census-8 — Verb results and cross-package identity travel as map[string]any instead of typed structs

`internal/lease/verbs.go:148` · idiom · **medium / effort M**

**Current:** ClassifyVerb, RequireHolder, Renew, ProtocolGrowth return map[string]any; census.AuthIdentity returns map[string]any that lease.ProcessIdentity (identity.go:44-46) unpacks with bare type assertions on "pidStartedAt" and "command"; census.FindAncestorProduction does the same. ClassifyVerb's doc comment also still reads "Classify resolves...", not naming its identifier.

**Target:** Typed result structs with json tags at each boundary (cmd marshals them identically — JSON object key order is not part of the contract); census.AuthIdentity returns a small struct so lease's ok=false path can only mean "no such process", never "someone changed the map's value type".

**Why:** The lease's own package doc records that a split identity source once made a main unable to recognise itself; the map-based hand-off recreates that fragility in the type system — a value-type change in census (int64 -> float64 via any JSON round-trip) makes every ProcessIdentity call return ok=false, which reads as "everything is dead" with zero compiler or test signal. Typed structs turn that failure into a compile error.

#### lease-census-9 — The sweep takes unbounded blocking record locks while holding the deliberately bounded lease lock

`internal/lease/sweep.go:150` · concurrency · **medium / effort S**

**Current:** acquireRecordLock flocks LOCK_EX with no timeout, and sweepOne runs it per job while the claimer holds worktree-lease.lock. lease/lock.go:13-16 documents exactly why the lease lock is bounded: "a blocking acquire would deadlock ... into an unexplained hang, so acquisition is bounded and turns that into a plain refusal."

**Target:** Bounded acquisition for record locks too (reuse acquireBounded with a refusal that names the job record), so a wedged record-lock holder produces a loud claim refusal instead of an indefinite hang under the lease lock.

**Why:** A live-but-stuck dispatch process holding one record flock stalls every claim, succession and takeover in the checkout forever — the exact unexplained-hang failure mode the package already identified and engineered away one layer up. Death releases flocks, but wedged-alive does not, and wedged-alive is the documented failure class (stale jobs) this sweep exists to handle.

#### lease-census-11 — Crash leftovers of acquisition and release are never cleaned

`internal/lock/lock.go:289` · design · **low / effort S**

**Current:** populatePrivate creates <path>.acquire-<pid>-<hex> (removed only by a defer that a crash skips) and removeLock renames to <path>.release-<hex> before RemoveAll (a crash between the two strands the trash directory). Nothing in this package or the janitor sweeps either pattern, so crashed acquirers/releasers accumulate directories beside every lock indefinitely.

**Target:** Opportunistic cleanup of stale *.acquire-*/*.release-* siblings (age-bounded, same fenced discipline) during Acquire, or an explicit janitor duty for the two suffixes.

**Why:** Unbounded garbage in artifacts/agents directories that operators and the census read; each crashed holder leaves a plausible-looking directory next to the real lock, and after months of unattended missions the noise obscures the one directory that matters during an incident.

#### lease-census-12 — The lease's atomic writes still discard the durability outcome the atomicfile contract carries

`internal/lease/lease.go:151` · design · **low / effort S**

**Current:** atomicJSON calls atomicfile.WriteText with an empty anchor and drops the durable bool: `_, writeErr := atomicfile.WriteText(path, string(encoded), "")`, with a comment acknowledging this is transitional "until its caller is converted to the two-outcome contract". Every lease-protocol record — the lease itself, the reaped-after-claim stamp, swept job records, protocol cursors — is written this way.

**Target:** Finish the B5 conversion for the lease writers: pass the checkout root as anchor and surface committed-but-durability-unknown (at minimum as an emitted event), since the stamp and lease record are exactly the files whose post-crash survival the recovery paths (completeInterruptedSweep) reason about.

**Why:** The recovery machinery assumes the stamp's presence/absence after a crash is meaningful, but without the directory-chain sync that meaning is weaker than the code comments claim; the tree built a two-outcome contract precisely for these records and its most safety-critical writer is the one not yet using it.

#### lease-census-5 — RunProductionCensus duplicates RunFixtureCensus's orchestration skeleton

`internal/census/production.go:82` · duplication · **low / effort S**

> downgraded by the verifier: Partially misreads the code: the claim "the logic is copied, not shared" overstates — classifyProcess, assembleVerdict, readSupervisionSnapshot, verifySupervisionSnapshot, configuredSignatures, liveCustody, and announcementsList are all shared, so a verdict-shape change is made ONCE, not twice. What is duplicated is ~30 lines of orchestration wiring (root realpaths, error accumulation, enumerate/signature error labels, count loop). That duplication is real and the enumerator-parameter refactor is feasible, but the production path's matched-pids-only cwd resolution is a deliberate structural difference the unified runner must absorb, and the demonstrated harm is thin. Downgraded to low: worthwhile tidy-up, not a medium defect.

**Current:** Both runners (production.go:82-140, run.go:85-135) repeat the same sequence: realpath the roots, read/verify the supervision snapshot, enumerate + configuredSignatures with identical error-string prefixes, Classify argvs, loop classifyProcess accumulating counts/inventory, assembleVerdict. Only the enumeration source and the cwd-resolution step differ.

**Target:** One unexported census runner taking an enumerator (and optional cwd resolver); RunFixtureCensus and RunProductionCensus become thin bindings, which is what the file comments already claim they are.

**Why:** The comment says "the production path substitutes live enumeration ... over the same logic", but the logic is copied, not shared: the error-label conventions and count bookkeeping have already had to be kept in sync by hand twice (the errors-slice wiring differs subtly between the two). The next verdict-shape change must be made twice or the two paths silently diverge.

### internal/validate, internal/returnschema, internal/turn, internal/receipt, internal/report, internal/gaterun, internal/audit

*Verdict: passes the senior-engineer bar. 16 findings (5 medium, 11 low).*

**Reviewer's summary.** Validation and reporting cluster: seven packages that port the metasystem's shell/awk gates and ledgers to Go while preserving each gate's exact message and exit-code contract. Per-package state: internal/validate (~2,900 LOC) holds the whole-artifact validators (conformance gate, turn-prompt, critique join, obligation matrices, stop-loss, quotes, wrapper-token proof) plus two admitted non-validators (TailorConf conf rewriting, SessionIsolation worktree provisioning) and two receipt-claim helpers consumed only by internal/receipt — the one genuine grab-bag in the cluster. internal/returnschema (90 LOC), internal/turn (58 LOC), and internal/gaterun (182 LOC) are small but each earns separate-package status with an explicit dependency-direction rationale in its package doc (shared vocabulary, versioned schema materializer, kernel-identity-backed gate markers). internal/receipt (430 LOC) is a clean append-only ledger with an injectable durability barrier and a fault test. internal/report mixes the turn-end stop-hook/open-work decisions with the frontier ledger (its largest file, unmentioned by the package doc, and the one writer in the cluster still using bare os.WriteFile with a fail-open unreadable-file path). internal/audit pairs the ported instruction audit with the Go coverage ratchet under a package doc that describes only the latter; its fail-closed read discipline (B7) is exemplary and is exactly the standard PlanConsistency and FrontierRecord currently miss. Test quality across the cluster is high: behavior-driven, real git fixtures, error paths and fail-open holes proven (unreadable policy file, injected sync failure, pid recycling), t.Helper used consistently, no timing-based flakiness patterns. Findings are one boundary/grab-bag design issue, a handful of fail-open error paths that contradict the house "refuse loudly, never skip silently" doctrine, one in-place-mutation API, one oversized function, and duplication/docs polish. No criticals, nothing contract-touching.

#### validate-report-3 — FrontierRecord treats an unreadable frontier file as no frontier, bypassing the regression guard

`internal/report/frontier.go:179` · error-handling · **medium / effort S**

**Current:** `if read, readErr := frontierReadFields(opts.File); readErr == nil { fields, fileExists = read, true ... }` — any read error other than not-exist (permission denial, I/O error) silently sets fileExists=false, so the direction check, the expiry check, and the frontierBeats regression refusal are all skipped and line 233 overwrites the recorded frontier without --force. FrontierChallenge (line 250) at least fails closed on the same conflation, but with the misleading message "no frontier recorded".

**Target:** Distinguish os.IsNotExist from other errors in both Record and Challenge: not-exist keeps today's first-record / "record the baseline first" behavior; any other error refuses with the real cause (exit 2), matching the B7 pattern.

**Why:** README sells this exact guard ("record refuses frontier regressions") and the house doctrine says an unreadable measurement is never progress; audit/metasystem.go:181-196 (B7) already implements the required distinction — a scan root that exists but cannot be read refuses rather than reporting clean. Record is the one writer in the cluster that still fails open.

#### validate-report-4 — Frontier ledger written with bare os.WriteFile while the tree's durable-write program exists

`internal/report/frontier.go:233` · error-handling · **medium / effort S**

**Current:** FrontierRecord persists the best-known-state file via os.WriteFile — no temp-and-rename, no sync — so a crash mid-write can leave a truncated file whose malformed score then makes every later challenge fail with exit 2 until a human re-baselines. Meanwhile conftailor.go:149-152 documents the migration of exactly this pattern to atomicfile.WriteText ("the old path here was WriteFile plus rename with no sync at all", B5) and receipt.appendLine got the B6 sync barrier.

**Target:** Write the frontier through internal/atomicfile like TailorConf does; bytes and path stay identical, so the on-disk contract is unchanged.

**Why:** The frontier is the single durable record improvement mode compares against; "nothing paid is silently discarded" is house doctrine, and this file is the one state writer in the reviewed cluster the B5/B6 program missed.

#### validate-report-5 — PlanConsistency silently skips unreadable plan files and reports clean

`internal/validate/planconsistency.go:44` · error-handling · **medium / effort S**

**Current:** In the collection loop, `data, err := os.ReadFile(...); if err != nil { continue }` — a plan that exists but cannot be read contributes neither its RETIRED declarations nor its violating lines, and the gate exits 0 saying "none prescribed". The error is fully swallowed.

**Target:** Return the read error (the function already has an error return the CLI maps to exit 2), or at minimum report the file as unscannable, following the B7 rule audit/metasystem.go:225-233 states in a comment: gone is fine, unreadable is not.

**Why:** This is a gate whose whole job is catching drift; failing open on I/O errors is the precise fail-open hole the audit package fixed and wrote a test for (TestAuditEdgeBranches), and "never skip silently" is the doctrine this contradicts.

#### validate-report-7 — mergeCritique is a 255-line function nesting five distinct validations

`internal/validate/conformance.go:706` · complexity · **medium / effort M**

**Current:** One function performs chain discovery, final-round selection, return parsing, material-finding extraction, the 75-line critiqueExhaustions shape/successor/prompt-enumeration validation (lines 843-917, four levels deep inside the per-critic loop), the reviewed-tree staleness check, and the model-independence ruling, accumulating failures across nested loops with local closures (successorText, finalScore) defined mid-flight.

**Target:** Extract per-concern helpers on conformanceRun — e.g. criticChainMembers, validateExhaustion(exhaustion, index) []string, independenceFailure(final) — keeping the exact message strings; the top level becomes a readable list of checks per critic chain. TurnPrompt at similar length is fine because it is strictly linear; this one is not.

**Why:** The exhaustion block alone carries enough branch depth that the fixture (TestConformanceReviewAndCritiqueMerge) is the only practical way to understand it; the next rule added here will be inserted at the wrong depth, and the message contract makes such mistakes expensive to notice.

#### validate-report-8 — unprefixMetasystem hardcodes the template's own directory name while installPrefix is computed

`internal/validate/conformance.go:134` · boundary · **medium / effort S**

**Current:** The review stage uses two normalization dialects: plans/ and control-plane checks go through installationPath(), which strips the prefix git reports for this checkout (line 260), but the diffBoundary comparison strips the literal string "metasystem/" from both declared and changed paths (lines 388, 393). Installation-relative boundary declarations therefore only normalize correctly when the installation directory happens to be named "metasystem" — true in this template checkout, false in any adopted repository with a different subdirectory name.

**Target:** Use r.installPrefix (via installationPath) for boundary normalization too, so the accepted declaration dialect is "repo-root-relative or installation-relative" uniformly; verify the conformance fixtures still pass since they assert the message contract, not the normalization.

**Why:** A gate that behaves differently in the template than in the repositories it ships to violates the template's own one-builder-per-system claim, and the asymmetry is invisible until an adopted repo's first boundary refusal names paths the implementer did declare.

#### validate-report-1 — Package validate mixes validators with transforms it names as exceptions

`internal/validate/validate.go:1` · design · **low / effort M**

> downgraded by the verifier: Facts confirmed: validate.go:1-7 enumerates its own exceptions, TailorConf is wired under the config family (main.go:53, runConfigTailor in validate_verbs.go:18), and SessionIsolation is provisioning-plus-audit, not artifact checking. But the harm is purely organizational — no test impediment, no bug class, no behavior change — so under the 'must carry its weight' bar this is a low cleanup, not medium. One correction: internal/adapter's package doc says it owns runtime-adapter lifecycle plumbing (job chains, permissions, capability snapshots), NOT adoption-time mechanics; internal/config or a new package is the defensible home for TailorConf, and the adapter option should be dropped.

**Current:** The package doc itself concedes the split: each function "checks one artifact shape end to end ... or performs one entangled transform (the metasystem.conf runtime tailoring, the second-session isolation copy)". TailorConf (conftailor.go) is a config rewriter and SessionIsolation (sessionisolation.go) is worktree provisioning — file copying, symlink auditing, harness-root resolution — neither validates anything, and the CLI even wires TailorConf under the `config tailor` verb (cmd/metasystem/validate_verbs.go:18), not a validate verb.

**Target:** Move TailorConf beside the config family (internal/config or internal/adapter, which already owns adoption-time mechanics) and SessionIsolation to internal/adapter or its own small package; validate then contains only artifact checkers and its name matches its contents. CLI verbs stay untouched — only internal package homes move.

**Why:** A package whose doc comment must enumerate its own exceptions is the definition of a grab-bag; the next transform will land here too because precedent beats the name, and the rubric's responsibility test (one clear purpose, name matches contents) fails today.

#### validate-report-10 — Boundary-violation loop and Mission Stream parsing each implemented twice

`internal/validate/conformance.go:500` · duplication · **low / effort S**

**Current:** The plans/ and control-plane violation scan appears verbatim in reviewStage (lines 303-314) and mergeWaiver (lines 500-511), and the "Mission Stream:" brief parsing with the "standalone" default exists both as conformanceRun.streamFor (lines 617-634, SplitN) and inside validate.WaiverFacts (receipts.go:78-91, strings.Cut).

**Target:** A boundaryViolations(paths []string) []string method serving both stages, and one missionStream(root, rootJob string) helper shared by streamFor and WaiverFacts (which also serves finding validate-report-2's move).

**Why:** These encode gate policy (what counts as tampering, what the default stream is); two copies of policy invite the classic drift where one stage is fixed and the other silently is not.

#### validate-report-11 — Add reports 'retro is due' for Check's error exits too

`internal/receipt/receipt.go:188` · error-handling · **low / effort S**

**Current:** After a successful append, `if after := Check(recheck); after.Code != 0` prints the retro-due note — but Check returns code 2 for a malformed receipts file or an invalid cadence value (e.g. retro.max-receipts=many), so a configuration error masquerades as "a metasystem retro is due" on every add, and the real cause is never surfaced.

**Target:** Branch on Code == 1 for the due note and forward Check's own stderr lines (or a "cadence could not be resolved" note) on Code == 2.

**Why:** The note exists to trigger a human workflow; making it fire on config errors trains the reader to ignore it and hides the misconfiguration it should have exposed.

#### validate-report-12 — StopLoss reports every read failure as 'missing --file ledger'

`internal/validate/stoploss.go:45` · error-handling · **low / effort S**

**Current:** `if err != nil { return nil, []string{"missing --file ledger"}, 2 }` discards err entirely — a permission error or I/O failure on an existing ledger is reported as a missing file. Exit 2 is correct (the gate fails closed), but the message misdiagnoses the cause.

**Target:** Keep exit 2 and append the real error, or word missing and unreadable separately the way readFileIfExists (validate.go:65-76) was written to enable elsewhere in this package.

**Why:** The stop-loss gate refuses new investigation cycles; when it refuses for the wrong stated reason, the human debugging it starts from a false premise.

#### validate-report-13 — Fence duplicates Running's marker scan-and-prune loop

`internal/gaterun/fence.go:42` · duplication · **low / effort S**

**Current:** Fence's glob/read/unmarshal/prune loop (fence.go:42-62) repeats Running's (gaterun.go:89-111) almost line for line — the doc comment even says "pruning dead or unparsable markers exactly like Running" — with only the self-chain exemption and Holder collection differing.

**Target:** One liveMarkers(root) []marker helper owning the scan and prune policy; Running reduces to len>0 over it, Fence filters by chain and maps to Holder.

**Why:** The prune policy (when a marker may be deleted) is a correctness rule for the fence; a promise of "exactly like" enforced only by hand-synchronized copies eventually breaks.

#### validate-report-14 — Package report's doc comment omits frontier, its largest and least report-like member

`internal/report/stopblock.go:1` · docs · **low / effort S**

*Same fix as architecture-10.*

**Current:** The package comment says report "holds the turn-end report decisions: the stop-hook block ... and the open-work check" — frontier.go (310 lines, the improvement-mode ledger that runs git, enforces the noise floor, and rewrites plans/frontier) is not a turn-end report decision and is not mentioned at all.

**Target:** Either extend the package comment to own all three members honestly (the CLI's `report` family already groups them, so co-location has a rationale), or move frontier to its own internal/frontier package with the CLI verb unchanged; the first is a two-line fix.

**Why:** The rubric's responsibility test is judged by the package comment; today it describes two-thirds of the package, and a reader looking for the frontier ledger has no reason to open internal/report.

#### validate-report-15 — readJobRecords smuggles a synthetic __path key into decoded job records

`internal/report/openwork.go:84` · idiom · **low / effort S**

**Current:** `record["__path"] = path` injects a bookkeeping field into the decoded JSON map, which stalePlans later reads back with a type assertion (line 128); the map type now lies about holding only job-record fields, and a record legitimately containing __path would be silently clobbered.

**Target:** Return a small struct { Path string; Record map[string]any } (or a parallel paths slice); two call sites change.

**Why:** Mixing transport metadata into domain data is the kind of shortcut that survives refactors and eventually collides with real data; the struct costs four lines.

#### validate-report-16 — Package audit's doc describes only the kill-shell fences, not the shipped instruction audit

`internal/audit/coverage.go:1` · docs · **low / effort S**

**Current:** The package comment (on coverage.go) frames the package as "the kill-shell program's mechanical fences ... decisions the gate consults between steps" — which fits the coverage ratchet, but metasystem.go (302 lines, the ported audit-metasystem.sh behind the shipped audit verb) is a product feature adopted repositories run, not a between-steps fence of the porting program.

**Target:** Rewrite the package comment to state both concerns (the shipped metasystem audit and the development-time coverage ratchet), or split the ratchet into its own package if the kill-shell program is expected to grow more fences.

**Why:** These two files share only the word audit; the doc comment is where that tension should be either justified or resolved, and today it describes the smaller half.

#### validate-report-2 — Receipt-claim policy lives in validate, forcing receipt to import validate

`internal/validate/receipts.go:15` · boundary · **low / effort S**

> downgraded by the verifier: Facts confirmed: CodeCritiqueClaim and WaiverFacts are called only from receipt.Add (receipt.go:167,172), creating the receipt=>validate edge, and moving them needs a copy of readJobRecord (also used by conformance.go:331,575,651). But the cost today is a compile-time edge in a single-binary tree — no cycle, no runtime cost, no test impediment — and the helpers are findable in a file named receipts.go. Legitimate S-effort layering cleanup, but medium overstates present harm.

**Current:** CodeCritiqueClaim and WaiverFacts (receipts.go) exist solely for internal/receipt's Add path (receipt.go:167,172) — they resolve a receipt's code-critique claim and waiver facts from job records. Their placement here creates the receipt => validate edge in the dependency graph, making the receipt ledger depend on the whole validator package (which itself execs git in conformance.go) to append a line.

**Target:** Move both functions (with a small local copy of readJobRecord, or a shared job-record reader) into internal/receipt, where the receipt-refusal policy they implement belongs; validate keeps readJobRecord for conformance.go.

**Why:** Dependencies should point from policy toward mechanism; here the ledger mechanism imports a validator package to get two policy helpers only it calls, coupling receipt appends to everything validate drags in and hiding where receipt-claim rules actually live.

#### validate-report-6 — VersionTwo mutates its input schema in place, partially even on error paths

`internal/returnschema/returnschema.go:26` · design · **low / effort S**

> downgraded by the verifier: Facts confirmed: returnschema.go:26-32 aliases the argument, writes $comment and title before the required/properties checks can fail, and returns the same map it mutated; the doc reads as pure. But both callers are verified safe — Materialize reads from disk per call (returnschema.go:72-81) and checkReturn loads the schema fresh (returncomplete.go:148,158) — so the harm is entirely latent. A landmine worth defusing for 4 lines, but with zero current misbehavior it is low, not medium.

**Current:** `value := schema` merely aliases the argument, then the function sets $comment and rewrites title before the required/properties checks can fail — so on error the caller's map is left half-upgraded, and on success the "returned" v2 schema is the same object as the v1 the caller passed in. The doc comment ("returns the v2 form of a v1 schema ... without touching the v1 files") reads as pure. checkReturn (validate/returncomplete.go:158-164) is safe today only because it reloads the schema from disk per call.

**Target:** Deep-copy first (a marshal/unmarshal round-trip is 4 lines and this is not a hot path), or make the mutation the documented contract and drop the misleading alias; either way no observable output changes.

**Why:** An API that silently corrupts its input on failure is a landmine for the next caller that caches a schema; the pointless `value := schema` alias actively disguises the mutation from a reader.

#### validate-report-9 — Three near-identical Python-repr formatters in one package

`internal/validate/conformance.go:29` · duplication · **low / effort S**

**Current:** quoted (conformance.go:29), scalarText (conformance.go:199), and jsonRepr (returncomplete.go:459) all render decoded JSON in the ported message dialect ('single-quoted' strings, None, True/False, integral floats without decimal point) with subtly different rules each — jsonRepr falls back to json.Marshal, quoted to fmt.Sprintf, scalarText leaves strings bare.

**Target:** One small file (e.g. repr.go) owning the dialect with two functions (bare vs quoted rendering), each current name kept as a thin alias or replaced at call sites; the fixture-asserted messages do not change.

**Why:** The message dialect is a tested contract; three private implementations is three places for it to drift when the next port touches one of them.

### internal/atomicfile, internal/boundedexec, internal/config, internal/capability, internal/authority, internal/hooks, internal/evidence

*Verdict: passes the senior-engineer bar. 16 findings (1 high, 7 medium, 8 low).*

**Reviewer's summary.** Seven leaf/utility packages of the metasystem Go CLI. Overall state: strong. Package docs are unusually good (they explain outcome models and carry scar-tissue rationale), tests are behavioral with real fault injection (atomicfile's syncDir injection, boundedexec's process-group kill proof, evidence's frozen-clock GC passes), and cmd/ wrappers stay thin. Per package: atomicfile — well-designed durability owner, but its two-outcome/anchor contract has zero production adopters (all 12 call sites pass anchor "" and ignore `durable`), its chain() helper contradicts its own doc for non-ancestor anchors, and WriteText/CopyFile duplicate the publication sequence the package exists to de-duplicate. boundedexec — correct and tested; its error text infers the config key from the bound's magnitude and misnames it once an operator tunes values. config — the one grab-bag: metasystem.conf reading/resolution/validation plus runtime-CLI config-identity hashing (confidentity.go) plus CanonicalModel under one roof, with four private copies of the conf line-parsing rule and a stale package doc; confidentity.go also prints warnings to stderr from library code. capability — sound policy, but Select is a 110-line monolith that mixes parsing, policy, and file output behind weakly-typed maps. authority — small, cohesive, correctly a standalone decision-matrix package (not a stub; merging it away would blur the doctrine boundary); its doc overclaims "typed refusal". hooks — small and legitimately standalone (audit owns a different concern), but its two key checks are substring matches over the whole settings file, which can false-pass the exact inert-hooks condition it exists to catch. evidence — careful, well-annotated GC with excellent tests; one real defect: it resolves mirror manifests by globbing across all per-checkout segments, so with a shared evidence root and a chain-name collision (the exact scenario dispatch/mirror.go's segmenting defends against), one checkout's GC can delete another checkout's never-mirrored job records. No package is an orphan; none should merge elsewhere.

#### foundations-1 — GC trusts mirror manifests from other checkouts' segments

`internal/evidence/gc.go:381` · design · **high / effort M**

**Current:** manifestCandidates globs evidenceRoot/agents/*/<chain>/manifest.json across every per-checkout segment, and pruneMirroredRecords calls chainManifestPath(evidenceRoot, rootChain, "") — deliberately ignoring the record's own mirror stamp — then deletes a terminal job record if any segment's manifest merely lists jobs/<name>.json (presence-only, no digest). collectChains has the same first-existing-candidate fallback when a record lacks a mirror stamp. It also re-implements manifestFiles inline at lines 385-392.

**Target:** Derive this checkout's segment the same way dispatch/mirror.go:53-56 does (sha256 of the resolved checkout root, first 12 hex — extract that into one shared helper), restrict manifest candidates to that segment plus the legacy unsegmented layout, and honor the record's mirror stamp in pruneMirroredRecords as collectChains already does. Reuse manifestFiles instead of the inline copy.

**Why:** mirror.go segments mirrors precisely because "two checkouts' chains may collide" under a shared evidence root. The GC undoes that defense: checkout B, holding an unmirrored terminal chain that shares a name with checkout A's mirrored chain, finds A's manifest via the glob, sees the record listed by name alone, and past the grace window deletes B's only copy of its job records — silent evidence loss in the multi-checkout setup the system explicitly supports.

#### foundations-11 — Self-hooks check uses substring matches over the whole settings file and can false-pass

`internal/hooks/hooks.go:47` · design · **medium / effort M**

**Current:** After comparing only top-level hook names, CheckOwnHooks asserts strings.Contains(liveData, "supervision-hook.sh") and strings.Contains(liveData, "$CLAUDE_PROJECT_DIR/metasystem") over the raw bytes of the entire settings file. A supervision hook attached to the wrong lifecycle event, or the path string appearing in any unrelated hook's args, satisfies both checks.

**Target:** Parse the live hooks structure (the file is already unmarshalled) and verify the supervision command and vendored-directory entry appear inside the specific event arrays the shipped configuration puts them in.

**Why:** This package exists to catch the template repository silently not running under itself; a check that a hook merely mentioned somewhere passes exactly the inert-hook states it was built to detect — e.g. the supervision command moved to an event that never fires at session start still reads as compliant.

#### foundations-2 — chain() contradicts its doc for non-ancestor anchors and carries an identical-branch if

`internal/atomicfile/atomicfile.go:128` · dead-code · **medium / effort S**

**Current:** The doc on chain says "An empty or unrelated anchor yields just dir", but the loop appends every ancestor up to and including the filesystem root when the anchor never matches, so WriteText fsyncs the entire path to "/". The if !strings.HasPrefix(...) at line 128 returns dirs in both branches — a vestigial condition that does nothing.

**Target:** Detect a non-ancestor anchor up front and return []string{dir} as documented (or fix the doc to state the walk-to-root behavior deliberately), and delete the identical-branch if. TestChainBounds currently only asserts the walk terminates, so tighten it to pin the chosen behavior.

**Why:** A caller passing a wrong or relative anchor silently pays fsyncs on every ancestor including "/", and any failure there (a permission-restricted ancestor, an fs that rejects directory fsync) fails the write pre-publication for a target that was perfectly writable — behavior the doc explicitly denies. Dead branches in the one durability-critical helper invite wrong fixes later.

#### foundations-3 — The durability contract has zero production adopters: every caller passes anchor "" and discards durable

`internal/atomicfile/atomicfile.go:66` · design · **medium / effort L**

**Current:** All 12 production WriteText/CopyFile call sites (dispatch, lease, adapter, host, mission, missionrunner, registry, validate, cmd) pass anchor "" — the documented "pre-B5 behavior, for callers not yet converted" — and every one discards the durable flag (dispatch/record.go:349 propagates it only to be discarded one frame up). The chain-sync machinery and the (false, nil) committed-with-doubt outcome are exercised only by this package's own tests; dispatch/record.go:338-348 marks the migration as not done.

**Target:** Finish the marked migration for the writers of durable state (thread the repository root as anchor through dispatch custody/record writes, lease, registry) and have at least those callers act on durable=false (log or stamp doubt); or, if the migration is abandoned, collapse the API back to error-only and record the decision. Either way, stop carrying an unclaimed guarantee.

**Why:** The package's reason to exist (the B5 durability contract) is currently inert: after a crash, job-record renames can vanish exactly as before, while the elaborate doc invites readers to believe durable state is crash-safe. A half-adopted two-outcome API is worse than either endpoint — callers can neither rely on durability nor see doubt.

#### foundations-4 — WriteText and CopyFile duplicate the publication sequence the package exists to de-duplicate

`internal/atomicfile/atomicfile.go:142` · duplication · **medium / effort S**

**Current:** WriteText (lines 69-107) and CopyFile (lines 142-181) each carry the same ~35-line sequence — MkdirAll, chain sync, CreateTemp, fill, Sync, Close, Rename, post-publication dir sync with the (false, nil) rule — differing only in how the temp file is filled. WriteVolatile repeats the temp-write plumbing a third time. The package doc itself argues "two copies become two fixes that silently diverge".

**Target:** One unexported publish(path, anchor string, fill func(*os.File) error) (bool, error) that owns the whole sequence; WriteText and CopyFile become one-line fills. WriteVolatile can share the temp-write half while keeping its no-fsync semantics explicit.

**Why:** A future hardening fix (e.g. handling EINTR on rename, tightening temp permissions) must now be applied twice in the one package whose charter is that durability fixes happen once; the doc's own B5 argument applies to its internals.

#### foundations-5 — boundKeyFor guesses the config key from the bound's magnitude and misnames it once tuned

`internal/boundedexec/boundedexec.go:97` · error-handling · **medium / effort S**

**Current:** The timeout error tells the operator which metasystem.conf key to raise by comparing the limit against the network default: limit <= 120s → exec.network-timeout-sec, else exec.local-timeout-sec. The comment admits "the bound itself does not carry its kind".

**Target:** Carry the kind: have Timeout return a small Bound{Limit time.Duration; Key string} (or pass Kind to Run) so the error names the key that actually produced the bound. Callers already call Timeout and Run as a pair.

**Why:** The inference is wrong exactly when it matters — an operator who sets exec.local-timeout-sec=100 gets a timeout message telling them to raise exec.network-timeout-sec, and raising it does nothing; the self-help text the design prides itself on then actively misdirects.

#### foundations-7 — Four private copies of the conf line-parsing rule with divergent duplicate-key semantics

`internal/config/resolve.go:117` · duplication · **medium / effort S**

**Current:** The rule "a setting is a non-blank, non-comment line containing '='; key left of the first '=', both sides trimmed" is implemented four times in one package: ConfValue (conf.go:23-33, last duplicate wins), ConfLookup (resolve.go:117-133, duplicate is an error), Keys (resolve.go:157-166), and Validate's parse loop (validate.go:40-66, duplicate is a problem). The last-wins vs strict split is real and deliberate (hot-path readers must not fail) but is stated nowhere at the divergence point.

**Target:** One unexported iterator — parseSettings(content string) yielding (line, key, value) — that all four consume; ConfValue keeps last-wins and ConfLookup keeps strict-duplicate on top of it, each with one sentence saying why they differ.

**Why:** Any future change to the line rule (say, trailing-comment stripping or CRLF handling) must land identically in four places or the same file resolves differently through `config get`, boundedexec.Timeout, Keys enumeration, and validate — a divergence nobody would notice until a conf file behaves differently per verb.

#### foundations-9 — Numeric operational knobs are soft-defaulted at read time but never checked by Validate

`internal/config/validate.go:19` · error-handling · **medium / effort S**

**Current:** boundedexec.Timeout (boundedexec.go:49-53) deliberately swallows malformed exec.local-timeout-sec / exec.network-timeout-sec values and falls back to defaults ("a malformed bound must not disable bounding"); cmd/supervise_component.go:170 does the same for its interval key. config.Validate checks cap floors, tiers, roles, and evidence.root but never these keys, so a typo like exec.local-timeout-sec=300s is invisible everywhere, forever.

**Target:** Add the known numeric knobs (exec.*-timeout-sec and the supervision interval keys) to Validate's checks as positive integers. The read-time soft fallback stays — it is correct — but the typo then surfaces at `metasystem config validate` time.

**Why:** Soft-fallback-on-read is only safe when something else reports the malformation; today an operator who fat-fingers a timeout silently runs on defaults and discovers it from a 300-second hang or a killed healthy job, the exact failure classes the bounds exist to prevent.

#### foundations-10 — Select is a 110-line monolith mixing parsing, policy, and file output behind untyped maps

`internal/capability/select.go:44` · complexity · **low / effort M**

> downgraded by the verifier: The description is accurate (Select spans parse/glob/freshness/policy/serialize over map[string]any, os.WriteFile at line 154, Result.Fallbacks is []map[string]any), but the harm is overstated: the policy leaves — isRestrictive, waived, validEnforcement, intCapability — are already small pure functions, and select_test.go's ~40-line newEnv helper makes the fixture cost per branch a few lines, with all five policy paths tested today. 'Currently untestable' is wrong. The function reads as linear guarded stages. Returning Result and typing Fallbacks are nice-to-haves; a medium restructuring must show harm the current shape causes, and it does not.

**Current:** Select parses the identity, globs and filters snapshots, checks freshness, loads requirements and envelope, evaluates required/optional/waiver policy, and serializes the Result to outputPath via os.WriteFile — all in one function over map[string]any. The exported Result carries Fallbacks []map[string]any, and every test must build a full directory fixture to exercise any policy branch.

**Target:** Split the stages (find snapshot → check freshness → evaluate(requirements, envelope, snapshot) Result) with the policy core operating on small typed structs (Snapshot, Requirements, Fallback{Capability string; Fallback any}); Select keeps the current signature as a thin composition, or better, returns Result and lets cmd write the file.

**Why:** The policy decisions here (what counts as restrictive, when a waiver applies) are exactly the decision logic house doctrine wants in Go and under test — but they are currently untestable without filesystem fixtures and unreadable as a unit; the weakly-typed Fallbacks also leaks map-shape guessing to the shell-facing JSON consumer.

#### foundations-12 — Run hand-rolls timeout/group-kill that exec.Cmd's Cancel/WaitDelay now owns, and cannot be canceled

`internal/boundedexec/boundedexec.go:72` · concurrency · **low / effort M**

**Current:** Run spawns a wait goroutine, selects against time.After, kills the process group with syscall.Kill(-pid), and bounds the reap with a hardcoded 5s killGraceWindow. There is no context: a caller shutting down (missionrunner draining a mission) cannot interrupt a 300s local bound early, and the timeout error is not errors.Is-branchable.

**Target:** Accept a context and build on exec.CommandContext with cmd.Cancel set to the group-SIGKILL and cmd.WaitDelay as the grace window (Go ≥1.20; the module is on 1.26) — same semantics, less bespoke machinery — and wrap the expiry error around context.DeadlineExceeded so callers can branch.

**Why:** The current code is correct for today's callers, but it re-implements a since-standardized mechanism and bakes in non-cancelability; the first long-running caller that needs orderly shutdown will have to fork or wrap this.

#### foundations-13 — Package doc says config is the CONF-ONLY reader with no .local precedence; resolve.go implements exactly that precedence

`internal/config/conf.go:2` · docs · **low / effort S**

*Same fix as architecture-9.*

**Current:** The package comment states "This is the CONF-ONLY reader (no metasystem.conf.local precedence — that belongs to dispatch's config_get)", but Get in resolve.go of the same package resolves flag → env → .local → mode-scoped → committed → default, and cmd's config verbs run it.

**Target:** Rewrite the package doc to describe the package's actual span (per foundations-6's split): ConfValue as the never-fails hot-path reader, Get as the full layered resolution, Validate as the domain check — and drop the reference to a shell function that has been ported.

**Why:** The first sentence a maintainer reads about the package actively misdirects them about where precedence lives — the kind of drift the metasystem's own one-rule-one-home doctrine treats as a defect.

#### foundations-14 — parseIdentity collapses four distinct failures into one undiagnosable message

`internal/capability/select.go:160` · error-handling · **low / effort S**

**Current:** Malformed JSON, wrong runtime, an invalid version token, an invalid configHash token, and bad key hashes all return the identical "<runtime> adapter returned a malformed configuration identity" with no detail about which field failed or what was received.

**Target:** Name the failing part in each branch ("identity is not JSON: %v", "identity names runtime %q, want %q", "cliVersion %q is not a valid token", ...), keeping the shared prefix if the shell matches on it.

**Why:** This error fires when an adapter probe misbehaves — precisely when an operator needs to know whether the probe emitted garbage JSON, the wrong runtime, or a bad hash; today they must add print statements to the adapter to find out.

#### foundations-15 — Package doc promises a "typed refusal" but Authorize returns bare fmt.Errorf strings

`internal/authority/authority.go:4` · docs · **low / effort S**

**Current:** The doc says the package decides "returning a typed refusal naming why", yet every refusal is an untyped fmt.Errorf; callers cannot errors.Is/As on refusal categories. The exported Modes is also a mutable package-level map used only as a membership check by cmd/metasystem/authority.go:22.

**Target:** Either introduce a small typed refusal (e.g. RefusalError{Mode, Class string} or per-category sentinels) if any caller will ever branch, or reword the doc to "a refusal error naming why". Replace the exported Modes map with a ValidMode(string) bool function so the set cannot be mutated by importers.

**Why:** The only consumer today prints the error, so the fix is cheap either way — but a doc that claims a stronger contract than the code provides is exactly what a future caller will build against.

#### foundations-16 — pyRepr emulates Python's repr for error messages in a Go codebase

`internal/config/validate.go:437` · idiom · **low / effort S**

**Current:** A 29-line hand-rolled quoter reproduces "the way the reference tooling quoted it" — single quotes, Python escape rules — solely so validator messages match the retired Python implementation. Nothing in scripts/ parses these messages (verified by grep); only the package's own tests assert the quoting.

**Target:** Use %q/strconv.Quote and update the handful of test expectations, deleting pyRepr.

**Why:** This is conformance-over-clean in exactly the sense the port doctrine rejects: 29 lines of bug surface maintained to imitate another language's repr, with no consumer that depends on it.

#### foundations-6 — internal/config is a grab-bag: conf reading, runtime-CLI identity hashing, and model canonicalization under one name

`internal/config/confidentity.go:26` · design · **low / effort M**

> downgraded by the verifier: Facts check out: confidentity.go (324 lines) shares zero code with the conf-reading half, parses JSON/TOML of external runtime CLIs, and hard-errors where the conf readers soft-default; the package doc describes only conf reading. But the demonstrable harm is navigational plus a stale doc, and the doc is fixed by foundations-13 at S cost — no bug class is hidden, no testing is impeded, no stated goal blocked. By the materiality bar a package split that only improves naming is low, not medium. The move itself is sound if taken (no on-disk contract involved).

**Current:** The package holds three concerns: metasystem.conf reading/resolution/validation (conf.go, resolve.go, validate.go), BuildConfigIdentity — canonical JSON/TOML flattening, sha256 fingerprinting, and version-gated filters over external runtime CLIs' config files (confidentity.go, 324 lines), and CanonicalModel (model.go). The package doc describes only the first.

**Target:** Move confidentity.go into its own package (e.g. internal/configidentity) whose doc states its role — fingerprinting a runtime adapter's behavior-relevant configuration; census and cmd import it directly. CanonicalModel can stay (it is genuinely shared conf vocabulary) with a line in the package doc.

**Why:** The two domains share nothing — different file formats, different consumers (adapters/census vs every conf reader), different failure philosophies (hard error on unparsable source vs silent default) — and the shared name "config" already misled the package doc. Rubric point 1 directly: the name no longer matches the contents.

#### foundations-8 — BuildConfigIdentity prints warnings to os.Stderr from library code

`internal/config/confidentity.go:45` · boundary · **low / effort S**

> downgraded by the verifier: The stderr write at confidentity.go:45 is real, but the finding's main harm scenario is factually wrong: census never calls BuildConfigIdentity — it uses only config.ConfValue (verified by grep). The sole production caller is cmd/metasystem/config.go:36, a CLI verb whose stderr is exactly where the warning belongs today, so nothing spams a supervision cycle. What survives is the boundary point: the caller cannot test or branch on 'filter ignored', which does change what configHash means. Worth fixing, but with one caller already printing to the right stream, it is low.

**Current:** When the version-gated filter is malformed or out of range, the library writes "warning: ..." directly to os.Stderr via fmt.Fprintf and proceeds hashing all keys. The caller (cmd/metasystem/config.go, and census through the graph) has no programmatic access to the fact a filter was ignored.

**Target:** Return the warning (e.g. (identity map[string]any, warning string, err error) or a typed FilterIgnoredError the caller inspects) and let cmd print it to stderr, preserving today's text.

**Why:** A leaf package writing to process streams takes a presentation decision away from every caller: census hashing identities inside a supervision cycle spams the cycle's stderr, and no caller can test or branch on "the filter was ignored" — which changes what the configHash means for that run.

### cmd/metasystem

*Verdict: does not yet pass the senior-engineer bar. 10 findings (2 high, 5 medium, 3 low).*

**Reviewer's summary.** cmd/metasystem: the single CLI binary (38 non-test Go files, ~5,900 lines) exposing 17 git-style families and ~150 verbs that the shell wrappers exec into. Overall state: strong. main.go is a clean data-table dispatcher (family/verb structs, exit 2 for routing errors); the overwhelming majority of verbs are exemplary thin relays — flag.FlagSet with ContinueOnError, explicit required-flag checks printing usage to stderr with exit 2, one call into an internal package, documented non-standard exit codes (3/4/7/77) where shell callers branch. Doc comments on verbs are genuinely informative and exit-code contracts are stated. The supervise owner/component verbs are model composition roots: they assemble disk/process/ledger adapters and inject them into the tested internal/supervise core. Tests that exist are behavioral (full record lifecycle through the verbs, signal-driven hold, procfs refusal wiring) and use t.Helper; the package is deliberately exempt from the coverage ratchet with a recorded rationale premised on cmd staying thin. The findings concentrate in three places: two supervise verbs that carry real supervision policy inside cmd (where the coverage exemption means it is unmeasured), a wholesale duplicated Devin spend-accounting algorithm surfaced by cmd registering it under two families, and consistency drift (usage-error exit codes in the mission family, three boolean-flag dialects, four argument-parsing styles). internal/janitor is the one internal package not reachable from the binary; the supervision-lifecycle plan tracks that wiring (D-4) as open work, so it is not reported as dead code.

#### cli-1 — Reserved-cap blocking policy lives in cmd, with a duplicated terminal-status set

`cmd/metasystem/supervise_arming.go:31` · boundary · **high / effort M**

**Current:** runSuperviseBlockingReservedCap scans artifacts/agents/jobs/*.json and missions/*/fences.json, decides which reservations block a proposed watcher ceiling, and ranks the blockers — all inside cmd. Its armTerminalStatuses set (line 23) re-declares the terminal-status vocabulary that internal/dispatch/record.go:37 owns unexported. Because cmd/metasystem is coverage-ratchet-exempt on the premise that it is thin wiring, this decision logic has no Go test and no coverage floor.

**Target:** Move the scan/rank decision into internal/supervise (it already owns the arming lifecycle) or internal/dispatch, export one terminal-status predicate from internal/dispatch as the single home, and leave the verb as a flag-parse-and-print relay. The logic then falls under a package coverage floor and gets table tests.

**Why:** House doctrine and the package's own coverage exemption both assume decisions live in internal packages; a wrong blocker judgment arms a watcher ceiling under a live reservation, and a status added to dispatch's transition table (or renamed) silently desynchronizes this copy.

#### cli-2 — The watchdog health policy is implemented entirely in cmd

`cmd/metasystem/supervise_arming.go:254` · boundary · **high / effort M**

**Current:** runSuperviseWatchdogReport contains the supervision health judgment: the staleness window formula (2×interval capped at 180s), the SUCCESS-verdict rule, untracked-inventory extraction, fingerprint mismatch detection, per-identity kernel liveness checks, and the 20-line report truncation — about 90 lines of policy in cmd, exercised only via supervision-hook.sh (which calls it with 2>/dev/null || true, discarding any error path).

**Target:** Extract a WatchdogReport(lastCensus, state, now, alive) []string decision function into internal/supervise beside decide.go, table-test the window/verdict/liveness rules there, and keep the verb as JSON-read plus print.

**Why:** This is exactly the class of logic the coverage exemption assumes is not in cmd; the staleness window and re-arm advice decide whether a human is told supervision is broken, and today no test pins any of it.

#### cli-3 — The Devin spend-delta algorithm is duplicated wholesale between internal/adapter and internal/host

`internal/host/devin_usage.go:23` · design · **medium / effort M**

*Same defect as adapter-host-registry-1 and architecture-1; one backlog item (W2.1).*

> downgraded by the verifier: Duplication verified wholesale: internal/adapter/devin.go:207 and internal/host/devin_usage.go:23 are line-for-line the same ~100-line algorithm (delta closure, ACU providerUnits rule, predecessor-missing unavailability, identical devinUsageFields), differing only in helper names, and both verbs are live (adapters/devin.sh:137 and hosts/devin.sh:139). Target is feasible without import cycles. Downgraded from high: both copies sit inside floored, tested packages (adapter 86.5, host 80.6) and are semantically identical today, so unlike cli-1/cli-2 there is no untested policy — the harm requires a future fix landing in only one copy. Important preventive consolidation, not a present defect.

**Current:** Cross-package observation forced by cmd registering both verbs: adapter devin-usage (adapter_runtime_verbs.go:287) and host devin-usage (host_verbs.go:124) expose two near-identical ~100-line implementations — internal/adapter/devin.go:206 and internal/host/devin_usage.go:23 — of the cumulative-totals delta, the ACU providerUnits rule, the predecessor-missing unavailability rule, and the same devinUsageFields list, differing only in helper names (readObject/loadObject, devinProviderUnit/providerUnit).

**Target:** One implementation in a single home both packages import (a small internal/devinusage package, or host importing adapter's), with both verbs delegating to it. The two CLI verbs can stay as-is, so no verb surface changes.

**Why:** Spend measurement is a core failure class the README names (runaway spend); a fix to the double-counting or ACU rules must currently land twice, and the copies have already drifted cosmetically — semantic drift is one patch away.

#### cli-4 — supervise fingerprint's --root fallback resolves the parent of the checkout, not the binary's checkout

`cmd/metasystem/census.go:30` · design · **medium / effort S**

*The same Dir^3 arithmetic bug as mission-contract-1; fix both call sites in one change (W1.7).*

**Current:** When --root is omitted, runCensusFingerprint derives the metasystem root as filepath.Dir(filepath.Dir(filepath.Dir(exe))). With the binary at <root>/bin/metasystem that is the PARENT of the checkout — verified empirically: the built binary fails with "fingerprint input is unavailable: scripts/agents/arm-supervision.sh" when --root is omitted. The doc comment claims it "defaults to the binary's checkout". All production callers (arm-supervision.sh) pass --root, so the fallback is broken, untested, and dead in practice — but in a nested-adoption layout the parent can itself be a metasystem root, and the verb would then silently fingerprint the wrong checkout's conf and adapters.

**Target:** Either fix the arithmetic to filepath.Dir(filepath.Dir(exe)) with a test, or drop the fallback and require --root (the latter changes the flag contract and would need sign-off).

**Why:** A fallback that computes a provably wrong path is worse than no fallback: loud failure in the common layout, silently wrong fingerprint in the nested layout the adopt tooling explicitly supports.

#### cli-5 — Watcher wiring duplicated between component and watcher-pass, with divergent config resolution

`cmd/metasystem/supervise_component.go:170` · duplication · **medium / effort S**

**Current:** setupWatcher (supervise_component.go:163-195) and runSuperviseWatcherPass (supervise_watcherpass.go:58-88) duplicate ~25 lines assembling supervise.WatcherConfig: the METASYSTEM_CENSUS_INTERVAL_MS override, the budget-percent read, the fixture/production census switch, Now and Warn. Worse, the same key census.max-interval-share-percent resolves via config.ConfValue (committed conf only) in the component but via config.Get (full flag/env/local/mode/conf precedence) in watcher-pass — a .local or env override is honored by one census writer and ignored by the other.

**Target:** One constructor (e.g. a watcherConfig helper in cmd, or a supervise.NewWatcherConfig accepting the resolved knobs) used by both verbs, with one deliberate choice of config-resolution mechanism.

**Why:** Two census writers that read the same tuning key through different precedence rules produce operationally confusing drift, and every future watcher knob must be added twice.

#### cli-6 — Usage errors exit 1 instead of 2 in the mission-fence verbs, breaking the package's own exit-code convention

`cmd/metasystem/mission.go:161` · error-handling · **medium / effort S / needs sign-off**

**Current:** The package convention is exit 2 for usage errors (flag parse failures, missing required flags) and 1 for operation failures. The fence verbs break it: invalid mission id / job / cap returns 1 (mission.go:161-167, 184-186, 213-221, 240-242, 259-261), and runMissionStateVerify's --repo/--ledger pairing error returns 1 (mission.go:94-97), while the runner verbs' missionRunnerScope returns 2 for the identical invalid-mission-id condition (missionrunner_verbs.go:27-37). Separately, adapter devin-session overloads exit 2 to also mean ambiguous correlation (adapter_runtime_verbs.go:278-281), so a shell caller cannot distinguish bad flags from ambiguity.

**Target:** Usage/validation errors exit 2 uniformly; devin-session's ambiguous outcome gets its own documented code (as owner-lock's 3/4 and chain-usage's 7 already do).

**Why:** Shell plumbing branches on these codes; the same malformed input yields 1 or 2 depending on which mission verb receives it, so callers cannot reliably separate misuse from operational failure.

#### cli-7 — Three boolean-flag dialects across the verb surface

`cmd/metasystem/dispatch_verbs.go:136` · idiom · **medium / effort M / needs sign-off**

**Current:** Booleans arrive as string flags compared to "true" (--overridden, --signal at dispatch_verbs.go:136/142/175/284, --network in codex-command), as string flags compared to "1" (--worktree at dispatch_verbs.go:358, --devin-checks at adapter_selftest_verbs.go:89), and as real flags.Bool switches elsewhere (--terminal-only, --expect-previous, --force, --allow-placeholders, --foreground). Any other value for the string variants (e.g. --signal yes) silently means false.

**Target:** One convention. Where shell callers already pass explicit values, flags.Func with strict true|false / 0|1 validation that rejects anything else; new verbs use flags.Bool.

**Why:** The silent-false behavior on a typo is a real trap in a system whose fences depend on these values (a mistyped --signal value quietly disables the session-handshake deadline), and three dialects make every new verb a coin toss.

#### cli-10 — Verb-file organization drift: strays and shared helpers in family-named files

`cmd/metasystem/validate_verbs.go:18` · naming · **low / effort S**

**Current:** The convention (one file per internal package/family) has drifted: runConfigTailor lives in validate_verbs.go but registers under the config family (validate_verbs.go:18); runCensusFingerprint lives in census.go but registers under supervise (main.go:297); the util family is scattered across slug.go and hold.go; and the package-wide helpers writeIdentityJSON/readJSONObject/jsonIntField live in supervise_arming.go (lines 134-248) yet are used by json.go and lease.go, while printJSON lives in lease.go and is used by missionrunner_verbs.go.

**Target:** A helpers.go (or jsonutil.go) for the cross-family helpers, tailor moved beside the config verbs, fingerprint beside the supervise verbs, and a one-line file-naming note in the package comment.

**Why:** With ~150 verbs, findability is the file layout's only job; today grep is the only reliable way to locate a verb's implementation from its family.

#### cli-8 — Three verbs swallow or replace real errors

`cmd/metasystem/adapter_runtime_verbs.go:161` · error-handling · **low / effort S**

> downgraded by the verifier: All three legs are factually accurate (claude-result-field returns 1 with no output at adapter_runtime_verbs.go:161-163; component-identity has no flag validation and four silent exit-1 paths at supervise_arming.go:149-173; plan-consistency prints 'no such plans directory' for any error including EACCES at validate_verbs.go:91-94). But the claimed harm is overstated: every current caller of the two silent verbs deliberately wraps them in 2>/dev/null || true probe semantics (claude.sh:177-178, arm-supervision.sh:264,281), so printed errors would be discarded anyway — silence here matches the callers' design, like json get's documented behavior. The plan-consistency mislabeled error is the one leg with user-visible harm (assert-plan-consistency.sh surfaces stderr). Worth the cheap fix, but low, not medium.

**Current:** runAdapterClaudeResultField returns 1 on an unreadable result document without printing anything (adapter_runtime_verbs.go:160-163), unlike every sibling adapter verb; runSuperviseComponentIdentity does no required-flag validation and exits 1 silently for missing --state/--component, unreadable state, and absent fields alike (supervise_arming.go:149-173); runValidatePlanConsistency discards the real ReadDir error and prints "no such plans directory" even for EACCES or not-a-directory (validate_verbs.go:91-94).

**Target:** Print the underlying error to stderr before the non-zero exit (json get is the one documented silent-by-design exception and says so); component-identity validates its flags with the standard usage message and exit 2.

**Why:** These verbs run inside detached supervision and hook plumbing where stderr is the only diagnostic channel; a silent exit 1 turns a permissions problem into an unexplainable no-op.

#### cli-9 — Four argument-parsing dialects in one package

`cmd/metasystem/receipt_verbs.go:40` · complexity · **low / effort M**

**Current:** Most verbs use flag.FlagSet; validate design-obligations/conformance/stop-loss hand-roll index loops to relay shell conventions (validate_verbs.go:273-303, 337-368, 417-433); runReceipt is a ~75-line manual switch with a need() closure duplicating flag semantics (receipt_verbs.go:40-113); missionrunner has its own parseRunnerArgs strict grammar (missionrunner_verbs.go:217-232). flags.Func already covers repeated flags (json.go and supervise_arming.go use it), so most of the hand-rolling re-implements what the stdlib provides.

**Target:** Collapse the receipt and validate relays onto flag.FlagSet with flags.Func for repeatable flags, keeping the shipped usage texts; keep parseRunnerArgs only if the strict no-equals grammar is genuinely load-bearing, and say why in its comment.

**Why:** Every dialect is a separate place for the missing-value and unknown-flag edge cases to diverge, and the receipt switch is the package's least readable function for zero behavioral gain.

### Findings added in critique round 1

These six findings originate from the Codex critique (`gpt-5.6-sol` at xhigh, round 1)
and were verified against the tree before acceptance; the verification evidence is in
the critique trail. They carry `codex-` ids to keep provenance visible.

#### codex-1 — Evidence GC deletes the only record carrying a chain's closure state

`internal/evidence/gc.go:393` · design · **high / effort M**

**Current:** The mirror manifest entry for a job record stores sourceStateHash, the record's semantic hash at mirror time (internal/dispatch/mirror.go:200-231). Mirroring happens only on the transition to terminal (the reap/cancel paths in dispatch.sh); closing a chain then CAS-patches chainClosed/runnerClosed into the root record (dispatch.sh:1316-1326) and nothing re-mirrors — close-check validates only, and the standing reaper explicitly skips terminal jobs. pruneMirroredRecords checks the manifest for presence only (gc.go:393-395) and deletes the local record once the manifest's pre-close updatedAt exceeds the grace window; the stop hook runs GC at every turn end.

**Target:** Make pruneMirroredRecords require the manifest entry's sourceStateHash to equal the current record's semantic hash before deleting — this guard is mandatory, not one of two alternatives: a post-close re-mirror can itself fail before the manifest updates (mirror failures are an existing, logged occurrence), leaving the stale manifest in place, and GC would still prune on presence and age. Re-mirroring after the close CAS is a worthwhile supplement for prompt durability. Test: close, force a mirror failure, GC must retain. Preserves all on-disk formats. [tightened R2.1]

**Why:** The surviving mirror predates closure, so chainClosed/runnerClosed are lost permanently once GC prunes — silent loss of exactly the state the close protocol exists to prove, on a path that runs every turn end. The closest thing to a critical in this review.

#### codex-2 — Owner-lock takeover is legal against a live holder whose argv cannot be read

`internal/dispatch/ownerlock.go:65` · error-handling · **high / effort S**

**Current:** Both kernel probers return Alive with ArgvKnown=false when a live process's argv is unreadable (identity_darwin.go:59-63, identity_linux.go:63-74). holderState never consults ArgvKnown: it joins the nil argv to an empty string, fails the tag match, classifies the holder stale (ownerlock.go:65-69), and OwnerLockClaim takes the lock over (ownerlock.go:104-112) — contradicting the file's own header ("a live or unreadable holder keeps the lock") and the identity doctrine ("UNKNOWN never authorizes anything"). The repo fixed this exact class in identity.Custodian (B1 verdict-table row 5, human-approved 2026-08-12); holderState is the one consumer still carrying the pre-correction behavior, and ownerlock_direct_test.go has no Alive+ArgvKnown=false row.

**Target:** holderState returns busy when the probe is Alive with ArgvKnown=false and a tag is expected — ideally by delegating the verdict to identity.Custodian instead of re-implementing the table — with the missing test row added. Prerequisite for W4.1 (reusing this lock for cap authority).

**Why:** A transient argv-read denial against a live owner steals the supervision owner lock — the split-brain the lock exists to prevent, via the one verdict table that missed the correction.

#### codex-3 — Classification's identity and ancestry errors collapse to false, landing on HUMAN

`internal/lease/classify.go:277` · error-handling · **high / effort M**

**Current:** ProcessIdentity collapses any census.AuthIdentity error to ok=false (lease/identity.go:41-44); upstream, psIdentity conflates prober failure with non-Alive into one "no such process" error (census/verbs.go:44-47), so ESRCH and EPERM are indistinguishable to every caller; ParentPid collapses read errors to (0,false) (identity/enumerate_linux.go:53-56, darwin twin). In Classify, a false from ParentPid ends the ancestry walk and falls through to ClassHuman (classify.go:277, 294-296); a false from StartedAt silently skips the custody check for that ancestor while the walk continues. An error-truncated walk is indistinguishable from "no recognised ancestor" — and RequireHolder returns holder=true for HUMAN, RunHeld runs HUMAN argv with no lock and no gate.

**Target:** Distinguish not-exist from unreadable through the prober chain (census/verbs.go:44-47 is the choke point), and abort classification on ANY identity, command, start-time, or parent-read uncertainty encountered before classification completes. Refusing only HUMAN is not enough: the walk that skips an unreadable adapter or supervision ancestor continues upward, can recognize the authenticated main above it, and returns ClassMain — which RequireHolder authorizes as the checkout holder, and which can even claim an absent lease. Add a child → unreadable-adapter-ancestor → authenticated-main test. Extends lease-census-1, which covers the file-read legs. [tightened R2.2]

**Why:** Indeterminacy currently lands on a MORE privileged class — HUMAN when the walk terminates on an unreadable parent, MAIN when it skips past an unreadable restricted ancestor to the authenticated main above. The reachable window is narrow (transient prober failure, proc-restriction denials), but when it lands it is a privilege escalation at the single most security-relevant decision in the tree.

#### codex-4 — The arming blocker scans fail open on unreadable or malformed records

`cmd/metasystem/supervise_arming.go:47` · error-handling · **high / effort S**

**Current:** runSuperviseBlockingReservedCap skips job records on ANY readJSONObject error — ENOENT, EACCES, EIO, and malformed JSON are one continue (supervise_arming.go:47-49); missing or noninteger capMin is silently non-blocking (:51, :57); fence files get the same all-error skip (:63-66). With every blocker skipped it exits 0 with no output, and all three arm-supervision.sh gates (re-arm, establishment, takeover) read empty output as safe to arm. One leg is already correct and must be preserved: the fence path's job-status lookup is fail-closed — an unreadable record leaves status empty, which counts as a blocker.

**Target:** Tolerate only ENOENT (the glob race); any other read, parse, or schema failure exits nonzero so arming refuses. Fix in place first; the W3.1 relocation into internal/supervise then moves tested fail-closed code.

**Why:** A permissions error or corrupt record on a job holding a live capMin reservation arms a watcher ceiling under that reservation — the exact unsafe-arming decision the verb exists to prevent, reachable through an I/O error.

#### codex-5 — dispatch's record lock is a second unbounded flock, held under the global lease lock

`internal/dispatch/record.go:109` · concurrency · **medium / effort M**

**Current:** withRecordLock takes a plain blocking LOCK_EX flock with no deadline (record.go:109). Production paths run record create/setup/CAS through lease run-held (dispatch.sh:171-179, :999, :1069, :1324 → RunHeld, lease/verbs.go:270-288), which holds the global lease lock across the child process. A record-lock holder wedged mid-hold (SIGSTOP, fsync stall, an inherited flock fd) blocks the child forever; RunHeld never releases; every subsequent claim, renew, and succession then refuses at its 10-second bound for as long as the wedge lasts. Lock ordering is consistent (lease → record), so this is wedge propagation causing persistent unavailability, not deadlock. lease-census-9 covers only the sweep's separate acquireRecordLock implementation — two flock implementations, one bounded fix so far.

**Target:** Bound withRecordLock the way lease's acquireBounded does (LOCK_NB + deadline + a refusal naming the job record), or restructure so record operations never run while RunHeld holds the lease lock. Same fix discipline as lease-census-9, second site.

**Why:** The failure class this repo engineered away one layer up — "a blocking acquire turns a wedge into an unexplained hang" is the lease lock's own doc rationale — is still live at the record layer, and it stalls the lease machinery checkout-wide while it lasts.

#### codex-6 — Durable event archives share a basename-keyed directory across checkouts

`internal/evidence/gc.go:632` · design · **low / effort S**

**Current:** The durable events destination is evidenceRoot/events/basename(checkoutRoot) (gc.go:632), unlike mirrors, which segment by sha256 of the resolved checkout path (mirror.go:54-56). durableCopyVerified replaces a divergent same-name durable file with local content before the local delete decision (gc.go:661-676). In principle two same-basename checkouts share one durable directory; in practice the archive naming — events-UTCsecond-PID[-n].jsonl — makes a divergent same-name collision require sub-second PID reuse, so this is a consistency gap, not a reachable data-loss bug. (The critique's data-loss framing was rebutted on that basis; the namespace inconsistency stands.)

**Target:** Segment the durable events root by the same checkout hash mirrors use, with an explicit legacy-ownership rule for existing archives.

**Why:** Two namespace schemes for durable evidence under one root is a design inconsistency that will cost a future multi-checkout debugging session; the fix is small and mechanical.

## Part 2 — The boundary audit: scripts

### scripts/validate-metasystem.sh

*Verdict: does not yet pass the senior-engineer bar. 11 findings (7 medium, 4 low).*

**Reviewer's summary.** scripts/validate-metasystem.sh — LOGIC-MUST-MOVE — a 4,409-line end-to-end self-check suite, not a runner: it is dense with domain JSON production/parsing, retry/timeout policy, and three outright shadow implementations of Go-owned computations; the process-driving harness skeleton legitimately stays shell, but the domain knowledge embedded in it must move behind the binary.

WHAT IT IS. The metasystem's full self-check (README "Scripts" table): audit, skill validation, routed-asset checks, and positive/negative fixture tests for every gate script and for the dispatcher/mission-runner engine, designed to run in both the template and adopted repositories (where no Go toolchain exists — only bin/metasystem). It is a test suite, so producing fixture JSON and asserting on-disk records is largely its legitimate job; the findings separate that from genuine boundary leaks.

SECTION CHART (line ranges):
1–28 arg parsing + delegate-scope bookkeeping (plumbing). 30–107 root/sentinel, gate-fence consult and marker (gate fence, gate register — decision correctly in Go), go gate, live gate-fence fixtures (one unguarded kill, L103). 109–144 fixture-budget init, audit run, gofmt-shim negative fixture. 146–258 skill validation, template-vs-adopted mode detection (L170), two large routed-asset existence lists. 260–331 sub-suite dispatch (bash -n + scripts/agents/*-fixtures.sh — the existing decomposition pattern), conf template-demotion greps. 332–398 hooks-JSON python asserts, adapter usage/declaration greps, a grep-pin of Go source text (L385). 400–489 template skill assets, registration-drift diff/cmp, adopted-mode registration checks (sed-parses metasystem.runtimes, L459). 491–583 fake envelope probe, cleanup trap with failure-evidence preservation (excellent), no-rg audit fixture. 585–743 fill_harness_conf (conf grammar in python) and a 90-line python catalog of all role schemas/permission presets/requirements shapes (L653). 747–1462 preamble-quote drift, return fixtures + assert-return-complete, turn-prompt fixtures (deliberately hand-authored — good independence), critique-closure, plan consistency, hooks check (binary), job-mode return fixtures. 1464–2827 dispatcher E2E in a synthetic adopted repo: config precedence, conf-validate negatives, arming, ~50 dispatch/reap/cancel/follow-up/mission-fence fixtures; leaks: census-freshness policy in python (L1557), transient-dispatch retry policy triplicated (L1682, L1739, L2747), contract hash reimplemented in awk (L2596), conf mutation via 600-char perl regexes (L1483). 2829–3429 mission-runner E2E: ARM wrapper shim, runner_git index.lock stale-deletion policy (L1516, used here), fake and codex-shim hosts, patience, prompt refusals, fences, resume, adapter selftest. 3437–3899 stop-hook, debug-java preflight, obligation-gate, refactor-baseline, frontier, stop-loss, conf-knob, and receipt fixtures (harness driving the gate scripts; fine). 3901–4294 nested adopted-copy validation + adopt.sh self-test (~350 lines, extractable). 4296–4395 watch-background-jobs fixtures. 4397–4409 delegate accounting + verdict.

BINARY VERBS CALLED DIRECTLY: gate fence, gate register, hooks check, proc started-at, job snapshot-select, mission fence-aggregate-usage, mission prompt-assemble, report open-work, util hold — plus indirectly via every wrapper script under test (dispatch.sh, metasystem-config.sh, receipt.sh, assert-*.sh, watch-background-jobs.sh).

WHAT BELONGS IN GO: (a) the three shadow computations — contract canonical hash (use assert-mission.sh --seal, which the same file already uses 350 lines later), census freshness (propose supervise census-fresh), and metasystem.runtimes parsing (use config get); (b) the transient-dispatch retry policy, into internal/dispatch; (c) the protocol-shape catalog, into internal/returnschema tests plus a proposed protocol verify verb for adopted repos; (d) the Go-source grep-pin, into an internal/adapter unit test; (e) over time, the inline-python JSON assertions, continuing the kill-python direction (the repo is already moving missionrunner fixtures into in-process Go tests).

WHAT LEGITIMATELY REMAINS: the E2E skeleton — process lifecycle driving, PATH shims (gofmt, codex), TTY fixtures, PATH-restricted environments, nested adoption runs, and black-box assertions on the on-disk records (asserting the persisted format from outside the engine is exactly what preserves the contract). The end state is a thin orchestrator that sequences per-area fixture scripts (the pattern lines 265–323 already establish) with every domain answer obtained from the binary.

HYGIENE: genuinely strong. set -euo pipefail, disciplined quoting including ${arr[@]+...} guards, documented set +e/set -e toggles with a correct errexit-scoping comment (L1674), a cleanup trap that preserves failure evidence instead of deleting it, bounded waits everywhere with scaled caps and rich timeout diagnostics. Defects found are small: one unguarded kill under errexit (L103, contrast the guarded one at L2804), a re-derived template-mode test (L1287), and a reused fixture output name (agent_fails pending-follow-up at L2023 and L2334). The dominant hygiene problem is size: a 4,409-line single file whose two biggest blocks (~1,965 and ~350 lines) ignore the sub-suite decomposition pattern the file itself uses elsewhere.

#### script-validate-1 — Mission contract hash reimplemented in awk, shadowing internal/mission

`scripts/validate-metasystem.sh:2596` · boundary · **medium / effort S**

> downgraded by the verifier: Facts verified: fixture_contract_hash (2596-2600) reimplements contractCanonicalSignedBytes in awk+shasum, and make_runner_contract (2954) seals via assert-mission.sh --seal. Two corrections cap the severity. First, the canonical function now lives at internal/contract/contract.go:640, not the comment's internal/mission/contract.go — but its algorithm is frozen by every on-disk signed approval (changing it invalidates existing contracts), so the drift the finding fears is the least likely in the tree, and the awk comment names the Go function so grep finds it. Second, the 'S' fix is not the seal path: mission contract-seal validates the full authored grammar and runs the frozen gate (assert-mission.sh usage text, cmd/metasystem/mission_contract.go), which the deliberately minimal envelope-only mission-alpha contract cannot satisfy without reshaping the fixture around gate instruments it doesn't need. The realistic fix is the hash-only verb, a new CLI verb needing owner sign-off. Real shadow implementation, bounded blast radius (deterministic suite-red, discoverable), low trigger probability: medium.

**Current:** fixture_contract_hash() reimplements contractCanonicalSignedBytes (strip Approval lines, trim trailing whitespace, drop trailing blanks, sha256) in awk+shasum to seal the mission-alpha dispatch-envelope contract. The comment itself names internal/mission/contract.go as the canonical source. 350 lines later, make_runner_contract (line 2954) seals its contracts through scripts/assert-mission.sh --seal — the sanctioned path — so the awk copy is not a deliberate independence pattern, just a divergent second implementation.

**Target:** Seal the mission-alpha fixture contract through assert-mission.sh --seal like every other fixture contract (shape the minimal contract so the checker accepts it), or if the checker cannot accept a headerless contract, add a hash-only mode to the mission family rather than keeping the algorithm in awk. Delete fixture_contract_hash.

**Why:** If canonicalization ever changes in Go, the awk silently keeps the old algorithm and the suite fails with a baffling signature mismatch deep in the dispatch fixtures — a red-herring failure in exactly the suite people trust to localize regressions. Shadowing a named Go function is the textbook boundary violation this audit exists to find.

#### script-validate-10 — python3 is still a hard validation dependency in adopted repositories

`scripts/validate-metasystem.sh:3447` · design · **medium / effort L / needs sign-off**

**Current:** The suite guards for python3 exactly once (line 3447, the stop-hook fixture) but invokes it unconditionally in roughly sixty other heredocs — fixture generation, JSON field extraction, census parsing, TTY driving — most of which run in adopted-mode validation, where the product itself no longer needs python after the Go migration.

**Target:** Triage the heredocs: JSON field reads become a small 'util json-get --file --path' verb (or existing status/report verbs grown a --field); fixture-record fabrication moves behind the __record-* dispatch verbs that already exist for exactly this; genuinely harness-side python (the pty driver, the atomic-result watcher) stays but gets an explicit up-front python3 requirement check with a clear message.

**Why:** The migration's promise to adopted repositories was 'the binary plus plumbing scripts'; this file quietly re-adds a python3 runtime requirement to every adopting project, and the single command -v guard makes the dependency look optional when it is load-bearing. It is also the largest remaining kill-python surface in the repo.

#### script-validate-2 — Census freshness policy computed in inline python, twice

`scripts/validate-metasystem.sh:1557` · boundary · **medium / effort M / needs sign-off**

> downgraded by the verifier: Both copies verified (wait_for_agent_census_fresh 1557-1568 with the verdict/fingerprint/generation/interval//2 policy; the ARM shim's completedAtEpoch loop 2848-2855). Two corrections. The 'passes against a stale census' harm is wrong: internal/dispatch.CensusFresh re-gates every dispatch (attest.go), so drift in the shell copies can only produce a loud fixture timeout ('timed out waiting for a fresh census') or a retried refusal — a flake/hang class, never false-green validation. And the proposed verb largely exists already: 'metasystem dispatch census-fresh' (main.go:89) exposes the engine's own freshness ruling, so both call sites can poll it today; contract_touching should be false unless a --wait convenience form is added. Genuine boundary duplication with the engine's policy encoded twice in shell, and suite flakes are a documented first-order cost here, but the failure mode is loud and bounded: medium, not high.

**Current:** wait_for_agent_census_fresh (lines 1557-1568) parses last-census.json and state.json and decides freshness with domain policy: verdict==SUCCESS, fingerprint match, generation==state.generation, and 0 <= age <= max(1, intervalSec//2). The ARM wrapper shim (lines 2848-2855) re-parses the same census file for completedAtEpoch with its own wait loop. Both encode what a fresh census means outside the supervise package that writes those records.

**Target:** A supervise census-fresh (or supervise status --fresh --wait --timeout-sec N) verb in the binary: exit 0 when the latest census is fresh by the engine's own definition against the current fingerprint/generation. Both shell call sites become a one-line poll or a single blocking call.

**Why:** The freshness rule (notably the interval/2 heuristic and the generation join) is supervision policy the Go side owns. When internal/supervise changes the census record or the staleness semantics, these two python fragments diverge silently and the suite either hangs to its cap or passes against a stale census. Two copies in one file is already drift.

#### script-validate-3 — Arming-window dispatch retry policy triplicated in shell, keyed on message text

`scripts/validate-metasystem.sh:1682` · boundary · **medium / effort M**

> downgraded by the verifier: All three retry loops verified (1670-1692, 1732-1741, 2747-2759), each grepping 'censusGeneration=' and each re-implementing the job-record-exists boundary. One material correction: the grep key is not prose — 'censusGeneration=' is the structured diagnostic field of the generation-mismatch refusal (attest.go:84). Rewording the surrounding message leaves retries intact; only renaming the field breaks them, and a repo grep for that token lands on all three shell sites, so 'reword the diagnostic and the retries silently stop' overstates the trigger. The age-stale refusal (attest.go:93) carries no such token and is correctly not retried. What survives: triplicated retry policy for an engine-owned transient, matched on message content because no typed exit code exists, and dispatch.sh callers outside the suite get only 'retry in a moment' prose. The target (engine-owned retry or a distinct exit code) is sound and doctrine-aligned. Medium.

**Current:** The bounded retry for the self-heal transient (dispatch refused between arming publication and confirming census) is implemented three times: run_agent_fixture_captured (1663-1693), run_tty_agent_fixture (1725-1743), and inline in the mission-timeout subshell (2747-2759). Each detects the transient by grepping stdout for the literal token 'censusGeneration=' and each re-implements the 'safe only before the job record exists' boundary condition.

**Target:** internal/dispatch owns the bounded retry for its own arming-window transient (safe by construction: it knows whether a job record was created). If the caller must stay in charge, give the transient a distinct exit code so callers branch on a contract, not on grep of prose; the three shell loops collapse to one or zero.

**Why:** Retry policy is decision logic, and this one is coupled to the refusal's message wording — reword the engine's diagnostic and the retries silently stop, resurfacing as suite flakes. Worse, any production caller of dispatch faces the same transient and must reinvent the same loop; that is the signature of policy living on the wrong side of the boundary.

#### script-validate-4 — Two giant fixture blocks ignore the file's own sub-suite decomposition pattern

`scripts/validate-metasystem.sh:1464` · complexity · **medium / effort L**

**Current:** The file already delegates fixture areas to dedicated scripts (supervision-fixtures.sh, mission-fixtures.sh, lease-succession-fixtures.sh, etc., lines 265-323), yet the dispatcher/mission-runner/selftest E2E block (1464-3429, ~1,965 lines) and the adopt.sh self-test (3945-4294, ~350 lines) live inline, pushing the file to 4,409 lines with helper functions defined mid-flow (copy_tree_without_artifacts appears at line 3907 inside an if-block).

**Target:** Extract scripts/agents/dispatch-fixtures.sh, scripts/agents/mission-runner-host-fixtures.sh, and scripts/adopt-fixtures.sh, invoked from validate-metasystem.sh exactly like the existing sub-suites (with the same delegate_process_section gating and shared fixture-budget sourcing); validate-metasystem.sh becomes the orchestrator the rest of the file already is.

**Why:** The monolith defeats the delegate-scope machinery's own purpose (skippable, individually re-runnable sections), makes every fixture change a diff against a 4.4k-line file, and hides the boundary leaks this audit found inside 2,000 lines of scroll. The repo already proved the decomposed pattern works — this block just predates or ignores it.

#### script-validate-8 — metasystem.runtimes read with sed, bypassing the config engine's precedence

`scripts/validate-metasystem.sh:459` · boundary · **medium / effort S**

**Current:** Adopted-mode registration checks read the runtime selection as configured_runtimes=$(sed -n 's/^metasystem\.runtimes=//p' metasystem.conf), shadowing the resolver the binary owns — while lines 1830-1851 of this same file prove that resolution must honor flag > environment > metasystem.conf.local > conf precedence.

**Target:** configured_runtimes=$(scripts/metasystem-config.sh get --key metasystem.runtimes) — the wrapper and verb already exist; one line changes.

**Why:** A repo that overrides metasystem.runtimes via metasystem.conf.local or METASYSTEM_METASYSTEM_RUNTIMES gets its registrations validated against the wrong runtime set: skills required for a runtime that is not actually selected, or missing registrations passed silently. The suite enforces precedence rules it does not itself obey, three hundred lines apart.

#### script-validate-9 — Snapshot naming contract pinned by grepping Go source text

`scripts/validate-metasystem.sh:385` · testing · **medium / effort S**

**Current:** Lines 384-390 grep internal/adapter/snapshot.go for two exact fmt.Sprintf source lines to assert the capability-snapshot naming contract. The same contract is already exercised behaviorally twice: the fake-probe sequence fixture (lines 1914-1917 asserts *-001.json/*-002.json) and internal/adapter's unit tests under the go gate.

**Target:** Delete the grep block; if the naming grammar deserves its own pin, add a TestSnapshotNameGrammar unit test in internal/adapter asserting generated names match ^<runtime>-<version>-<hash>-<date>-\d{3}\.json$ — behavioral, and it runs under the go gate this suite already invokes.

**Why:** This tests source bytes, not behavior: renaming a variable, extracting a helper, or reflowing the Sprintf fails the whole validation suite with zero behavior change, training people to update the grep without thinking — while an actual naming regression would already be caught by the behavioral fixtures. Text-pinning Go internals from shell inverts the ownership the migration established.

#### script-validate-11 — Unguarded kill under errexit can abort the suite if the fence fixture outlives its sleep

`scripts/validate-metasystem.sh:103` · error-handling · **low / effort S**

**Current:** kill "$gate_fence_foreign" 2>/dev/null has no || true (line 103), unlike the equivalent guarded kill "$mission_pid" 2>/dev/null || true at line 2804. The foreign gate process is sleep 60; if the intervening fence checks and the fenced go-gate refusal ever take longer than 60 seconds — a loaded machine, exactly the condition the suite's own history documents as its flake source — the sleep is already reaped, kill returns 1, and set -e aborts the entire validation at a line that prints nothing.

**Target:** kill "$gate_fence_foreign" 2>/dev/null || true, matching line 2804.

**Why:** A silent hard abort with no diagnostic, in the one place the file forgot the guard it applies everywhere else, and triggered only under load — the most expensive kind of flake to diagnose. One-token fix.

#### script-validate-12 — Template-mode test re-derived inline instead of using template_mode

`scripts/validate-metasystem.sh:1287` · duplication · **low / effort S**

**Current:** Line 1287 re-states the full detection expression ([[ "${metasystem_here##*/}" == metasystem && -f "${metasystem_here%/*}/development/metasystem-design.md" ]]) for the own-hooks check, although template_mode was computed from the identical expression at line 171 and is used everywhere else as (( template_mode )).

**Target:** Replace the inline test at 1287 with if (( template_mode )); then, keeping line 171 as the single derivation.

**Why:** A mode decision stated twice will eventually be edited once; the own-hooks check would then run (or not run) under a different definition of 'template repository' than the rest of the suite, which is precisely the plan-inconsistency class this file tests other documents for.

#### script-validate-5 — Full protocol-shape catalog restated in 90 lines of inline python

`scripts/validate-metasystem.sh:653` · boundary · **low / effort M / needs sign-off**

> downgraded by the verifier: The heredoc (653-743) exists as described and is not template-gated. But the finding's premise is substantially wrong: internal/returnschema does NOT encode the role property catalogs — it loads the shipped schema JSON as data and augments it (returnschema.go:34-54), and internal/validate knows only a handful of fields semantically (verdictMaterialCount, dimensions, diffBoundary). So this heredoc is not 'a second encoding of what the engine already understands'; it is the ONLY pin protecting the shipped protocol files from drift, and a drift pin needs an independent copy by definition. Moving it into a Go unit test plus a new protocol-verify verb relocates the copy and types it, but the claimed lockstep (JSON + pin) remains exactly two edits either way, and the adopted-mode verb is new CLI surface. The adopted-mode python3 dependency is real but is finding 10's item. Residual merit — a Go-owned home under the go gate would be sturdier — justifies low, not medium.

**Current:** The heredoc at 653-743 hardcodes the complete property sets of all seven role return schemas, the closed-schema rule for the orchestrator, both permission presets byte-for-byte, and the requirements-file grammar (required empty, waiver shapes, the implementer resume special case). This is a second, test-side encoding of the protocol that internal/returnschema and internal/validate already understand — and it runs in adopted repositories too, since the block is not template-gated.

**Target:** Template mode: an internal/returnschema unit test (under the existing go gate) asserting shipped scripts/agents/schemas/*.json, permissions/*.json, and roles/*.requirements.json match the engine's canonical shapes. Adopted mode: a proposed 'job protocol-verify --root' verb doing the same from the binary, called here as one line.

**Why:** Drift-pinning the shipped JSON files is legitimate, but the pin's home is wrong: protocol knowledge maintained as an inline python literal in a shell file rots invisibly, and every schema evolution now requires editing Go, the shipped JSON, and this heredoc in lockstep. The Go engine can state its own expectation once.

#### script-validate-7 — Fixture conf rewriting encodes the metasystem.conf grammar in perl/python regexes

`scripts/validate-metasystem.sh:1483` · boundary · **low / effort M / needs sign-off**

> downgraded by the verifier: Every cited site verified (the 1483 one-liner, fill_harness_conf 585-618, the 1866-1893 series, 2531, 4179). Two things deflate the harm. First, most of the 1866-1893 perl edits deliberately fabricate INVALID confs for negative validation fixtures (model.tier.one, runtimes=ghost, deleted model lines) — raw byte surgery is the correct tool there, and a config-set verb that understands the grammar is the wrong one. Second, the valid-conf builders are self-checking, not silent: the rewritten conf goes straight through the engine ('$agent_config' validate at 1853) and is then driven by fixtures whose roster depends on the rewrite, so a regex that stops matching fails loudly (e.g. role.default.runtime=fake lands 'outside metasystem.runtimes'); the author even added grep fallbacks at 1485-1486 for the two keys with no downstream check. The proposed set/unset (plus implied list) verb family is real new CLI surface requiring sign-off, bought mostly for the two valid-conf builders. Direction has merit; weight is low.

**Current:** Fixture repositories get their conf built by regex surgery: a ~600-character perl -0pi one-liner at 1483 rewrites runtimes, evidence root, watch intervals, and the whole role roster; fill_harness_conf (585-618) re-parses role/mode model keys with its own compiled regex and rebuilds model tiers; a dozen smaller perl -0pi edits follow (1866-1893, 2531, 4179). Each fragment restates the conf key grammar the Go config engine owns.

**Target:** A 'config set/unset --file <conf> key=value' verb (internal/config already parses the format), turning each perl blob into explicit key operations; fill_harness_conf becomes a loop of config get/set calls with the model-tier assembly done by the engine that defines tier syntax.

**Why:** Every conf-grammar change (new key family, tier syntax) now needs to be mirrored in write-side regexes scattered through the suite, and a regex that silently stops matching produces a fixture conf that tests something other than what the comment claims. The read side already went through the binary; the write side should too.

### Dispatch and supervision plumbing

*Verdict: does not yet pass the senior-engineer bar. 18 findings (3 high, 10 medium, 5 low).*

**Reviewer's summary.** Dispatch and supervision plumbing for the metasystem: dispatch.sh is the delegate-job lifecycle driver (dispatch/follow-up/status/cancel/close/reap plus internal adapter callbacks), arm-supervision.sh establishes the per-repository supervisor set, supervision-hook.sh is the runtime hook adapter, go-gate.sh is the Go toolchain gate, watch-background-jobs.sh is the generic background-job watcher, preflight-commands.sh is the command inventory check. Overall state: unusually disciplined shell (set -euo pipefail everywhere, careful quoting, deliberate error propagation, scar-tissue comments citing incidents), pervasively consulting the Go binary for decisions — but three policy subsystems have not finished migrating, one lock primitive has a real robustness bug, and one whole mode of dispatch.sh is production-dead. A recorded human ruling (plans/go-surface-consolidation.md, Wido 2026-08-12) explicitly accepts the shell reap ladders, the two-phase reservation ordering, and shell-owned custody orderings as plumbing; findings below respect that severance and target only logic outside it. Classifications: scripts/agents/dispatch.sh — LOGIC-MUST-MOVE — the reap ladder and reservation ordering are accepted plumbing by ruling, but roster/model-tier/escalation policy (~150 lines), the cap-resolution fallback chain plus a shell reimplementation of config-resolver precedence, and the census fingerprint verdict are substantial domain policy in shell; section map: L1-77 preamble/paths (plumbing), L79-91 wait-cap helpers (plumbing, duplicated), L93-114 drift+census gate (plumbing with fingerprint-verdict leak), L116-193 record/lease/authority wrappers (pure forwarding), L195-299 process/group liveness and wind-down (kill-capable ladder, accepted by ruling; two-way liveness and stale TODOs), L302-367 chain/lifecycle/cap-authority locks (plumbing; cap-authority lock is the bug), L369-524 config/cap/tier/escalation helpers (LOGIC), L526-591 mission/permissions/snapshot/prompt (forwarding), L593-686 launch/handshake/wait (plumbing with hand-built proof JSON), L688-746 usage/mirror (forwarding; one subshell-return defect), L748-879 reap ladder (accepted by ruling; standing-only branches dead), L881-1080 dispatch_job (orchestration + escalation leak), L1098-1261 follow_up (orchestration, duplicates dispatch_job; design-critic worktree sync is custody and stays), L1263-1328 status/cancel/close (plumbing), L1330-1382 reap entry + dead standing mode, L1384-1544 internal adapter verbs (forwarding + accepted handshake-timeout ladder), L1546-1602 re-exec and routing (plumbing). scripts/agents/arm-supervision.sh — PLUMBING-WITH-LEAKS — the fixed arming order (announce, lock, launch, verify) is accepted custody ordering and forwards identity/lock/launch work to Go, but the watcher-ceiling derivation and the armed-verification verdict arithmetic are policy fragments Go should own. scripts/agents/supervision-hook.sh — PLUMBING-WITH-LEAKS — hook glue that correctly forwards watchdog/open-work/lease decisions to Go, but computes the running-work answer by grepping job-record JSON and sed-parsing process argv. scripts/agents/go-gate.sh — PURE-PLUMBING — toolchain orchestration whose only branching (module identity, seed mode, platform baseline) is bootstrap-inherent because it builds the very binary decisions live in; fence consult and coverage ratchet already go through metasystem verbs; hygiene clean. scripts/watch-background-jobs.sh — LOGIC-MUST-MOVE — the census half already moved to Go (supervise watcher-pass), but scan_once (~75 lines) is a verdict engine: terminal-status taxonomy, sidecar-mtime liveness, DONE/CAPPED/NEVER-STARTED/STALE precedence, auto-baseline policy. scripts/agents/preflight-commands.sh — PURE-PLUMBING — flat command inventory check, clean, nothing to move. Verified empirically during review: BSD grep BRE alternation used by the hook works on this platform; errexit does kill a for-loop when the final command of an && list fails (finding 07's mechanism); the standing shell reaper has no production caller (supervise owner launches Go components); internal/dispatch/ownerlock.go and supervise launch-detached already implement what two dispatch.sh TODOs claim is pending.

#### script-orchestration-01 — Cap-authority lock is an unhealable mkdir spinlock; a SIGKILLed holder bricks all dispatch and arming

`scripts/agents/dispatch.sh:352` · concurrency · **high / effort M / needs sign-off**

**Current:** acquire_cap_authority_lock (dispatch.sh:346-360, duplicated verbatim in arm-supervision.sh:86-100) spins on `mkdir cap-authority.lock.d` with a 10s deadline and releases via rmdir in an EXIT trap. The directory carries no owner identity and nothing anywhere heals it (verified: no janitor or Go path touches cap-authority outside fixtures). The held window spans cap resolution, git worktree add, and adapter config-identity subprocess calls.

**Target:** Route it through the existing rename-born owner lock: internal/dispatch/ownerlock.go already implements claim-with-identity, holder-liveness classification, and dead-husk healing behind `metasystem job owner-lock`. Both scripts should claim/release the cap-authority lock through that verb (or a thin `job cap-authority-lock` wrapper) exactly as chain and lifecycle locks already do three functions above.

**Why:** kill -9, OOM-kill, or machine crash while a dispatcher or armer holds the lock (traps do not fire) leaves the directory forever; every subsequent dispatch AND arming dies with 'timed out acquiring repository cap-authority lock' until a human runs rmdir. Total dispatch outage from a crash the rest of the lock family already survives — the same husk-healing lesson the owner_lock comment block documents was learned once already.

#### script-orchestration-02 — Roster resolution, model-tier ranking, and escalation policy live in shell

`scripts/agents/dispatch.sh:948` · boundary · **high / effort L**

**Current:** dispatch_job lines 925-979 plus helpers model_tier (462-475), configured_tier_indices (482-485), assert_tiers_contiguous (494-503), registered_runtime (438-447): two-level role/default fallback for runtime and model, comma-list parsing with whitespace trimming, tier rank lookup with ambiguity->999999, contiguity re-validation (which internal/config/validate.go contiguousFromOne already owns), cost-direction wording, and the three-way escalation refusal (tiers absent / unranked / higher tier) with signed-envelope bypass.

**Target:** Proposed verb: `metasystem job resolve-roster --role R --mode M [--runtime X] [--model Y] [--mission ID]` (internal/config + internal/mission) printing JSON {runtime, model, rosterPair, requestedPair, escalationRequired, costDirection, reason}. Shell keeps only the interactive confirm_escalation prompt and the TTY check. New verb, so it needs the owner's CLI sign-off per house rule.

**Why:** This is selection policy — exactly 'computing values the Go side also understands': Go validates tiers and the mission fence evaluates envelope.dispatch-allow, while shell independently re-derives ranks and contiguity, so the two sides can drift (the contiguity check is already implemented twice). Escalation refusal is a spend-control decision, the class the README says must be enforced, not prose-in-bash.

#### script-orchestration-03 — config_key_origin reimplements the config resolver's precedence; the non-mission cap chain is shell policy

`scripts/agents/dispatch.sh:373` · boundary · **high / effort M**

**Current:** config_key_origin (373-393) re-derives the env-name mangling (upper-case, tr '.-' '__') and probes env -> conf-local -> conf -> default by calling `config conf-value` per file — a shadow copy of the precedence order the Go resolver behind `config get` already owns. resolve_nonmission_cap (395-422) implements the cap fallback chain (cap.min.role.runtime.model -> cap.min.runtime.model -> dispatch.cap-min -> built-in 120) and then calls `job cap-resolution` merely to serialize the result. refuse_unsigned_mission_cap_override (424-432) decides mission-cap legality from the shell-computed origin.

**Target:** internal/config grows an origin answer (`metasystem config origin --key K` or an --origin flag on config get), and the existing `job cap-resolution` verb grows into `job resolve-cap --role --runtime --model [--requested]` owning the chain and the unsigned-override refusal; mission fence authorize-cap (already Go) stays the mission side. Shell then just forwards.

**Why:** Two implementations of resolution precedence is the classic drift bug this repo already documents in the model_tier comment ('reading metasystem.conf directly made every tier in metasystem.conf.local invisible'); the origin probe also ignores mode-scoped keys that `config get --mode` honors. The unsigned-cap refusal is a fence-authority decision, not glue.

#### script-orchestration-04 — derive_watcher_ceiling computes the watcher cap authority in shell

`scripts/agents/arm-supervision.sh:47` · boundary · **medium / effort M**

> downgraded by the verifier: Facts verified (lines 47-73; blocking-reserved-cap is Go; dispatch reads the ceiling via Go `job watcher-ceiling`). But the severity case overstates: there is no second implementation to drift against — the ceiling is derived once in shell, attested into state.json, and every consumer reads the attested number, so 'a drift between this derivation and what the Go fence believes' has no mechanism today. The miss directions also fail safe: a ceiling derived too low produces a refused dispatch with explicit re-arm guidance, not a breach. 'Both refusals are already Go' is loose — the final comparisons are shell, only the fragments are Go. Real Phase-D-aligned boundary defect, medium harm profile.

**Current:** Lines 47-73 scan config (dispatch.cap-min, fence.job-cap-min, every cap.min.* key) and the whole environment for METASYSTEM_CAP_MIN_*, take the maximum, and add 30 — the number the watcher's ceiling attestation, the re-arm refusal, and dispatch's `cap < watch_cap` check all hang off.

**Target:** Proposed verb: `metasystem supervise derive-ceiling [--max-cap N]` in internal/supervise, sibling of the already-Go `supervise blocking-reserved-cap` that consumes its output; arm-supervision.sh forwards. Needs CLI sign-off as a new verb.

**Why:** The ceiling is the supervision contract's core number: dispatch refuses caps at or above it and re-arm refuses ceilings below reserved caps — both refusals are already Go, but the input they check is computed by shell arithmetic over config and env. A drift between this derivation and what the Go fence believes silently changes which dispatches are legal.

#### script-orchestration-05 — count_running_work greps job-record JSON and sed-parses process argv to answer 'is anything running'

`scripts/agents/supervision-hook.sh:60` · boundary · **medium / effort M**

> downgraded by the verifier: All facts verified: the raw grep at supervision-hook.sh:60 sits four lines above proper `$ms json get` calls on the same records; mission detection at 80-84 is pgrep plus sed over `--root` argv shape (mission-runner.sh does exec with --root, so the coupling is real); work_sentence composes the turn-end verdict. Direction matches kill-shell Phase D, which names this file's POLICY as the port surface, and `report running-work` is a natural sibling of the existing internal/report/openwork.go. Downgraded because this is an advisory human-facing surface: watcher, reaper, budget caps, and the stop-block enforce independently, so a silent break here misinforms the human rather than skipping an enforcement — a real cost (false 'nothing running' while a mission burns), but not the outage/bypass class the other highs carry.

**Current:** Lines 54-106: `grep -q '"status": *"\(pending\|running\)"'` against raw record files (four lines above proper `$ms json get` calls on the same records), mission detection via pgrep + sed extraction of --root from mission-runner command lines, gate detection via pgrep on script names, and work_sentence composing the verdict shown to the human every turn end.

**Target:** internal/report, beside the existing open-work: proposed verb `metasystem report running-work --repo <root>` returning the jobs/missions/gates inventory (or one `report turn-end` that also folds in open-work); the hook keeps only payload parsing, identity lookup, and surface_json. Needs CLI sign-off as a new verb.

**Why:** This is the answer to 'is it still working?' — the README's unsupervised-runs failure class — computed by format-coupled grep (any nested "status":"running" in an error field or mirror result false-positives) and argv-shape-coupled sed that silently breaks the moment mission-runner's flags change. Parsing domain JSON and shadowing domain state in shell is exactly what the doctrine forbids, and the Go binary is already in hand in the same function.

#### script-orchestration-06 — The watcher's job-classification engine (DONE/STALE/CAPPED/NEVER-STARTED/VANISHED) is verdict policy in shell

`scripts/watch-background-jobs.sh:302` · boundary · **medium / effort L**

*Same migration as script-misc-3; one backlog item (W4.6).*

> downgraded by the verifier: Facts verified: classification half at 302-375 in shell while the census half already runs through Go `supervise watcher-pass` (line 219); the concatenated digit check at 161 does pass an empty --stale-min (Go's resolver returns an empty flag verbatim — 'an empty flag still wins'), after which `[ age -ge "" ]` fails and STALE silently never fires; the unquoted glob loop is deliberate but space-fragile. Two calibration corrections: (1) kill-shell.md r3/KS-R3-009 already rules this exact migration, but assigns the job-file classification to the REPORT family, not internal/supervise as this finding proposes — the target conflicts with the recorded plan and needs reconciling at sign-off; (2) no live defect beyond edge-case nits exists — the verdict logic is stable and fixture-covered by validate-metasystem — so this is scheduled migration debt, not an active high. Confirmed as a real boundary finding at medium.

**Current:** scan_once (302-375) plus is_terminal (265-270), in_scope (252-258), sidecar-primary selection, newest-sibling-mtime liveness, verdict precedence (terminal > cap > never-started > stale), the running-set VANISHED tracking, and the auto-baseline policy (384-392). The census half of this script already moved to Go (`supervise watcher-pass`, line 219); the classification half did not.

**Target:** Proposed verb: `metasystem supervise scan-jobs --dir <glob>... --state <file> [--scope P --scope-field F --stale-min N --cap-min N --start-verify-min N --baseline]` in internal/supervise, emitting the exact same greppable report lines and state-file format; the script keeps argument parsing, the ARMED banner, and the sleep loop. Needs CLI sign-off as a new verb; formats preserved so no on-disk contract changes.

**Why:** These verdicts are the enforcement for the README's 'unsupervised runs' failure class — status taxonomy, staleness thresholds, and precedence are decision logic the doctrine assigns to Go, and half of this file's decisions already migrated there. The remaining shell also carries avoidable fragility: unquoted glob iteration splits on spaces, and the concatenated digit-validation at line 161 accepts an empty --stale-min that later breaks comparisons silently.

#### script-orchestration-07 — One failing reap_one aborts the whole all-jobs reap sweep under set -e

`scripts/agents/dispatch.sh:1373` · error-handling · **medium / effort S**

**Current:** `for record in "$jobs"/*.json; do [[ -f "$record" ]] && reap_one "$(basename ...)"; done` — reap_one is the final command of the && list, so its nonzero return (reachable via reap-facts failure on a malformed record, or wind_down_group refusing an unowned/surviving group) trips errexit and kills the process mid-sweep. Verified the mechanism empirically with a minimal set -e loop.

**Target:** Accumulate instead of die: `reap_one ... || sweep_failed=1` per iteration, exit nonzero after visiting every record — the exact contract internal/supervise/reaper.go ReaperPass already documents ('a single unreadable record does not abort the sweep; the first such error is returned after all records are visited').

**Why:** A single bad record starves every job after it in the sort order: `dispatch.sh reap` (all-jobs) stops at the first failure and later jobs are never budget-checked or reaped that pass. The Go sibling explicitly guards against this; the shell ladder — which the ruling keeps — should hold the same invariant.

#### script-orchestration-08 — The standing-reaper mode of dispatch.sh has no production caller; Go owns the standing sweep

`scripts/agents/dispatch.sh:1354` · dead-code · **medium / effort M / needs sign-off**

**Current:** reap --interval/--heartbeat/--instance-tag/--start-gate (1330-1382), standing_reaper=1 with supervision-only authority, emit_component's reaper attribution (51-53), and the standing-gated verdict branches (stale-claim-epoch sweep 766-775, abandoned-setup 788-793). Searched the whole tree: nothing launches `dispatch.sh reap --interval`; supervise owner launches `metasystem supervise component --component reaper`, stale-claim-epoch lives in internal/lease/sweep.go, abandoned-setup in internal/supervise/reaper.go and missionrunner/drain.go.

**Target:** Remove the standing mode from dispatch.sh (flags, gate wait, loop, standing-gated branches), leaving the lease-held single-shot reap path that wait_for_job and missionrunner drain actually use. authority-regression-fixtures.sh asserts the `internal_authority supervision-only` line and must move with it.

**Why:** ~80 lines of a kill-capable daemon mode that production never starts, shadowing verdicts (stale-claim-epoch, abandoned-setup) whose live owners are now Go — a future re-activation would run a second, divergent standing reaper with kill authority the human ruling explicitly denied standing reapers. Needs owner confirmation precisely because it removes script verb surface and touches a fixture.

#### script-orchestration-09 — Shell liveness probes are two-way where the Go discipline is three-way; an unreadable ps turns live jobs into failures

`scripts/agents/dispatch.sh:199` · concurrency · **medium / effort M**

**Current:** process_matches (195-201) and lock_owner_state's callers treat 'cannot read the command line' as no-match: job_supervisor_matches then reports a live supervisor as gone, reap_one_locked heads for process-lost, wind_down_group refuses (unowned), and wait_for_job maps the failed __reap-held into return 3 — dispatch --wait reports 'failed' for a job that is still running. The file's own comment at 255-258 acknowledges sandboxes that deny process-table reads, but only carves out the fake runtime; internal/identity's Liveness and the Go reaper's 'indeterminacy never acts' rule already solve this three-way.

**Target:** A Go probe the ladder consults (consistent with the severance ruling's 'each already consulting Go decision fragments'): `metasystem proc classify --pid P --start S --tag T` returning live|dead|unknown from internal/identity, with the shell deferring on unknown exactly as arm-supervision's identity_alive already does ('inability to inspect a tag is never proof').

**Why:** Two liveness semantics live in one control plane: arm-supervision treats unreadable argv as live, dispatch treats it as dead-enough-to-reap. The divergent copy sits on the kill-capable path, where a transient ps failure during the wait loop becomes a false terminal verdict surfaced to the orchestrator.

#### script-orchestration-10 — verify_armed re-derives census freshness and fingerprint/generation agreement in shell

`scripts/agents/arm-supervision.sh:271` · boundary · **medium / effort M**

**Current:** Lines 270-312 poll state.json, heartbeats, and last-census.json, doing the staleness arithmetic (observedAtEpoch vs interval*2+2, completedAtEpoch vs interval), the derivedWatcherCapMin/loadedCapMin equality, and the fingerprint+generation match — the same freshness judgment `metasystem job census-fresh` already renders for dispatch.

**Target:** Proposed verb: `metasystem supervise verify-armed --root R --repo P --owner-pid N --owner-start S --owner-tag T [--deadline-sec N]` (internal/supervise) returning armed/not-with-reason; the script keeps at most the retry loop. Needs CLI sign-off as a new verb.

**Why:** Arming's success criterion is a domain verdict computed twice: once here in shell for arming, once in Go for dispatch's census gate. When the freshness rule changes (as the generation field's addition shows it does), two implementations must move in lockstep or arming certifies a set dispatch will refuse.

#### script-orchestration-11 — stop_recorded_components dies silently under set -e when component identity is unreadable

`scripts/agents/arm-supervision.sh:264` · error-handling · **medium / effort S**

**Current:** `read -r pid start tag < <(read_component_identity ... || true)` is an unguarded simple command: when state.json exists but `supervise component-identity` yields nothing (corrupt/partial state, missing component entry), read returns 1 on EOF and errexit kills the whole arming takeover with no message. The `|| true` inside the substitution and the `[[ -n "${pid:-}" ]]` on the next line show tolerance was intended. The twin read at line 281 is safe only by accident (verify_armed is called under `|| exit 1`, which disables errexit inside it).

**Target:** Guard the read itself: `read -r pid start tag < <(...) || true` (or `if ! read ...; then continue; fi`) in stop_recorded_components, and the same for symmetry at line 281 so the safety stops depending on caller context.

**Why:** The failure mode lands exactly where recovery matters: a takeover from a dead owner with damaged state aborts arming with exit 1 and zero diagnostics, leaving the repository unarmed and the operator with nothing to act on — the silent-exit class this repo's own hook comments call out as the worst kind.

#### script-orchestration-12 — The census fingerprint match — a dispatch refusal — is decided by shell string equality

`scripts/agents/dispatch.sh:112` · boundary · **medium / effort S**

**Current:** require_fresh_census calls `job census-fresh` (Go) for freshness, then separately computes the expected fingerprint via arm-supervision and compares it to the verdict's fingerprint field in bash (lines 110-113), issuing its own die message.

**Target:** Extend the existing `job census-fresh` verb to take/compute the expected fingerprint (it already receives --repo and --arm for its remediation text) and render the complete fresh-and-matching verdict in one place.

**Why:** Half the census gate's verdict is Go and half is shell: the two halves can disagree about what 'fresh' means versus what 'armed code' means, and the refusal text diverges from the verb's own remediation guidance. One verb, one verdict.

#### script-orchestration-13 — follow_up duplicates dispatch_job's launch tail and has already drifted

`scripts/agents/dispatch.sh:1237` · duplication · **medium / effort M**

**Current:** Cap resolution + unsigned-override refusal + watcher-ceiling check + snapshot selection + inline-size check + prompt write + record-setup + launch + handshake (~70 lines) appear once in dispatch_job (1009-1078) and again in follow_up (1200-1259). The drift is already visible: dispatch_job validates dispatch.max-inline-input-kb as a positive integer (1048-1049); follow_up (1237) feeds it straight into arithmetic, so a malformed config value dies with a bash arithmetic error instead of the intended message.

**Target:** One shared shell function for the authorize-and-launch tail (the pieces are already function-shaped: resolve cap, check ceiling, select snapshot, size-check, write, launch), or fold the cap+ceiling+size checks into the `job build-record`/`build-follow-record` verbs that already receive most of these inputs.

**Why:** This is the security- and budget-relevant tail of every dispatch; two copies means every fix lands twice or — as the max_kb validation shows — once.

#### script-orchestration-14 — Two TODO(go-wiring) comments describe migrations that already shipped

`scripts/agents/dispatch.sh:294` · docs · **low / effort S**

**Current:** Lines 294-296 say the owner-lock claim/release protocol 'moves to Go whole or not at all' — it did: internal/dispatch/ownerlock.go implements staged rename, holder liveness, and husk healing behind the `job owner-lock` verb the very next function calls. Lines 597-599 say daemonization 'stays python until a dedicated launcher exists' — the next line calls `supervise launch-detached`, native Go with Setsid. Relatedly, lock_owner_state (212-221) still shadow-classifies holder liveness with ps for acquire_chain_lock's error message, duplicating ownerlock.go's holderState.

**Target:** Delete both stale TODOs; have `job owner-lock` print the busy holder's classification on rc=3 so acquire_chain_lock relays it and lock_owner_state disappears.

**Why:** Stale migration TODOs actively misdirect the next engineer (they assert python still exists on a path that is pure Go), and the leftover shadow classifier is the only remaining consumer of the pre-migration liveness idiom.

#### script-orchestration-15 — mirror_record's mid-chain CAS failure is swallowed by a pipeline subshell

`scripts/agents/dispatch.sh:743` · error-handling · **low / effort S**

**Current:** `"$ms" job chain-members ... | while ...; do ... record_cas ... || return 1; done` — the `return 1` exits only the pipeline subshell; because every caller invokes mirror_record with `|| true` (errexit suppressed inside the function), execution continues to the rm and the function returns 0. Unlike every other mirror failure path, no mirror_fail line lands in mirror-failures.log.

**Target:** Feed the loop with process substitution (`while ...; done < <("$ms" job chain-members ...)`) so return propagates, and route the failure through mirror_fail like the copy/verification failures above it.

**Why:** A partially-stamped mirror across a chain is exactly the state the durable mirror-failures.log exists to witness, and this path is the one that leaves no trace.

#### script-orchestration-16 — Ownership proof JSON is hand-printf'd and later re-parsed by substring match

`scripts/agents/dispatch.sh:626` · idiom · **low / effort S**

**Current:** launch_adapter builds ownershipProof with printf (626-631); job_supervisor_matches (233) and group_owned (261) then test it with `[[ "$proof" == *"\"pid\":$pid"* ]]` — substring matching against a re-serialized JSON object, coupled to compact formatting, when the same functions already use `$ms json get` with dotted paths (e.g. capResolution.truncatedBy at 852).

**Target:** Compare extracted fields (`json_field "$record" ownershipProof.pid` etc.), and let Go own the patch: extend `supervise launch-detached` or add a `job build-launch-patch` so the proof object is produced by the same code that later judges it.

**Why:** The substring idiom silently depends on the JSON re-serializer never adding a space; the values it guards feed group_owned, which authorizes signalling a process group — a place where a formatting change should fail loudly, not change the verdict.

#### script-orchestration-17 — Per-session hook state files accumulate without any garbage collection

`scripts/agents/supervision-hook.sh:141` · design · **low / effort S**

**Current:** watchdog-surfaced-$session.state (141-149) and stop-block-$session.state (178-189) are created per session under artifacts/agents/supervision/; the stop-block file is removed only when the same session later ends a turn with no open work, and nothing else cleans either (verified: evidence-gc and the Go tree never reference them).

**Target:** Age them out in the same turn-end pass that already runs evidence-gc (delete state files older than a few days), or key both by session inside one small JSON the Go stop-block verb owns.

**Why:** Every session that ends while blocked or warned leaves a permanent file in the supervision directory; over months of multi-session use this becomes hundreds of stale .state files interleaved with the supervision contract files operators actually need to read — the same residue class the record-locks 142k-file incident comment in dispatch.sh warns about.

#### script-orchestration-18 — Fixture wait-cap scaling and millisecond-sleep helpers are triplicated across the cluster

`scripts/agents/arm-supervision.sh:79` · duplication · **low / effort S**

**Current:** supervision_wait_cap (arm-supervision.sh:79-84) and dispatch_fixture_wait_cap (dispatch.sh:79-84) are the same function under two names; milliseconds_to_sleep exists in both scripts (dispatch.sh:86-91, arm-supervision.sh:109-114) and inline as printf -v in watch-background-jobs.sh:165-166; acquire/release_cap_authority_lock is copied whole between dispatch.sh and arm-supervision.sh (covered by finding 01).

**Target:** One sourced helper file beside emit-event.sh (the sourcing pattern already exists in all three scripts), or fold the scaling into the Go verbs that consume the deadlines.

**Why:** These are timing-critical fixture-scaling knobs (METASYSTEM_FIXTURE_CAP_SCALE_MILLI): three copies means a scaling fix or a rounding change lands unevenly, and the suite's known sensitivity to timing (fixtures flake under load) makes uneven scaling expensive to diagnose.

### Runtime adapters and hosts

*Verdict: does not yet pass the senior-engineer bar. 13 findings (2 high, 7 medium, 4 low).*

**Reviewer's summary.** Runtime adapter scripts (scripts/agents/adapters/) supervise one delegate CLI turn each — launch the runtime CLI, hold custody/heartbeat sidecars, drive the dispatch record through pending→running→terminal via dispatch.sh's Go-backed __handshake/__record-cas/__protocol-error verbs — and host scripts (scripts/agents/hosts/) run one mission host turn and write a result envelope via `metasystem host result-write`. The layer is unusually disciplined for shell: all JSON parsing/production is pushed into `metasystem adapter *`, `host *`, `json get`, `config identity`, `util slug|hold|json-validate`, records mutate only through CAS verbs, and every entry script sets `set -euo pipefail`. The remaining leaks cluster in four places: (a) the shared terminal-outcome/repair state machine in runtime-common.sh (error/phase taxonomy decided in shell), (b) Claude argv+permission+budget policy in shell and duplicated between adapter and host — while Codex already proved the intended pattern with the Go `adapter codex-command` builder reused by hosts/codex.sh via sourcing, (c) the devin host's per-session usage store, which publishes only on success and double-counts a failed resumed turn's spend, and (d) systematic adapter/host duplication (schema-prompt augmentation, gate waits, adjudication tails). Per-file classification: scripts/agents/adapters/runtime-common.sh — LOGIC-MUST-MOVE — legitimate process-supervision loops, but carries the terminal-outcome state machine (complete_from_cli), the one-repair policy plus repair-prompt authoring, and a ~200-line selftest conformance suite, all domain decisions. scripts/agents/adapters/claude.sh — PLUMBING-WITH-LEAKS — launch plumbing forwarding to `adapter claude-settings/claude-usage/claude-read-roots`, but permission-mode/tool-list/budget policy and full argv are built in shell, and post-CLI session reconciliation is decided inline. scripts/agents/adapters/codex.sh — PLUMBING-WITH-LEAKS — cleanest real adapter (argv built by Go `adapter codex-command`), but the permissions→sandbox/network mapping and post-CLI reconciliation stay in shell. scripts/agents/adapters/devin.sh — PLUMBING-WITH-LEAKS — parsing/correlation forwarded to `adapter devin-session/devin-usage/devin-config` and `config canonical-model`, but schema-into-prompt authoring, transcript settle/model-fallback decisions, empty-reply classification, and permission-mode mapping are shell decisions. scripts/agents/adapters/fake.sh — PLUMBING-WITH-LEAKS — a fault-injection simulator whose FAKE:<marker> branching is its purpose, but it reimplements runtime-common's lifecycle plumbing instead of sourcing it and hand-writes record-patch/event JSON that Go verbs own elsewhere. scripts/agents/adapters/claude-config-filter.v1.json — PURE-PLUMBING — data consumed by `config identity --filter`. scripts/agents/adapters/codex-config-filter.v1.json — PURE-PLUMBING — same. scripts/agents/adapters/devin-config-filter.v1.json — PURE-PLUMBING — same. scripts/agents/hosts/claude.sh — PLUMBING-WITH-LEAKS — rebuilds the Claude argv/budget policy independently of the adapter and adjudicates the outcome/exit taxonomy in shell. scripts/agents/hosts/codex.sh — PLUMBING-WITH-LEAKS — the reuse model to copy (sources adapters/codex.sh for the Go-backed builders); only the triplicated adjudication tail remains shell decision logic. scripts/agents/hosts/devin.sh — PLUMBING-WITH-LEAKS — forwards return/usage/config to `host devin-return/devin-usage/devin-config`, but manages the .session-usage on-disk store in shell (with the double-count defect) and duplicates the schema-prompt authoring. scripts/agents/hosts/fake.sh — PURE-PLUMBING — FAKEHOST marker parse plus forwarding to `host fake-return/fake-result`; the behavior allow-list is trivial fixture glue re-validated in Go.

#### script-adapters-01 — Terminal-outcome state machine for every adapter turn is decided in shell

`scripts/agents/adapters/runtime-common.sh:238` · boundary · **high / effort L**

**Current:** complete_from_cli (lines 236-286) maps cli exit status, handshake state, validation outcome, and repair outcome to the record's terminal transitions, choosing the protocol error codes (runtime_error, handshake_missing_session_id, session_identity_disagreement) and phase names (handshake/runtime/delivery/completed) in bash; fail_pending/finish_running (lines 129-149) additionally encode CAS tolerance policy ([[ $status -eq 0 || $status -eq 3 ]] treats an expect-state mismatch as success), and devin.sh lines 396-410 extend the same machine with the empty_reply classification. The error/phase vocabulary is exactly what internal/dispatch and internal/missionrunner adjudicate on the Go side.

**Target:** A Go verb in internal/adapter, e.g. `adapter adjudicate-turn --job --cli-status --handshake-done --candidate --transcript --usage`: it already owns normalize-return, so it validates the candidate, decides target status/error/phase, performs the CAS through internal/dispatch, and prints a 'repair' decision when a bounded repair turn is warranted; shell keeps only the CLI process launch for the repair.

**Why:** This is the core protocol decision of the whole adapter layer — branching on domain state to compute values Go also understands — living in a sourced shell library where it cannot be unit-tested and where every taxonomy change must be mirrored against internal/dispatch by hand.

#### script-adapters-02 — Claude argv, permission-mode/tool-list mapping, and budget policy are built in shell, twice

`scripts/agents/adapters/claude.sh:115` · boundary · **high / effort M**

**Current:** claude.sh lines 115-147 decide permission_mode (dontAsk vs acceptEdits), the tool allow-lists, budget defaults/validation (METASYSTEM_CLAUDE_MAX_BUDGET_USD default 5.00, max-turns 50, regex checks), and assemble the full `claude -p ...` argv in bash; hosts/claude.sh lines 65-79 rebuild the same argv and budget policy independently. Codex already has the intended shape: Go `adapter codex-command` emits NUL-separated argv and hosts/codex.sh reuses it by sourcing the adapter.

**Target:** A `metasystem adapter claude-command` verb in internal/adapter/claude.go (which already owns claude-settings and claude-read-roots), taking the job record or a --host permissions source and emitting NUL-separated argv exactly like codex-command; both adapters/claude.sh and hosts/claude.sh consume it, hosts/claude.sh reusing via sourcing the way hosts/codex.sh does.

**Why:** The permission-envelope→runtime-flag mapping is decision logic the Go permission-check verb must agree with, and the duplicated copy in hosts/claude.sh has already forked policy surface (host hardcodes acceptEdits); the codex precedent proves the migration is intended and cheap.

#### script-adapters-04 — Return-repair policy and the repair prompt are authored in shell

`scripts/agents/adapters/runtime-common.sh:212` · boundary · **medium / effort S**

**Current:** attempt_return_repair (lines 212-234) decides repair eligibility (runtime_repair_turn defined, return_repairs==0, session present), enforces the one-attempt bound, and composes the repair prompt's domain text (violation + schema + reply instructions) with printf/cat.

**Target:** `metasystem adapter repair-prompt --violation --schema --output` in internal/adapter (return.go already owns normalization), with the eligibility/bound decision folded into the adjudicate-turn verb from script-adapters-01; shell keeps only the runtime_repair_turn process launch.

**Why:** The repair bound and eligibility are retry policy, and the prompt is domain content the delegate's compliance is judged against; both belong beside the schema/normalize logic Go already owns rather than as printf lines nobody tests.

#### script-adapters-05 — The full-contract selftest is a ~200-line protocol conformance suite in shell

`scripts/agents/adapters/runtime-common.sh:431` · boundary · **medium / effort L**

**Current:** run_full_contract_selftest (lines 431-587) plus selftest_attempt_matches_declaration (393-429) implement the conformance policy in bash: model placeholder validation (line 437), git fixture construction, nonce probes, the mapped/notEnforced denial taxonomy (which error codes count as a denial, lines 408-412), session-equality assertions, and evidence checks done by grep -Fq on return.json (line 565) instead of a typed read; make_selftest_brief (326-359) authors the domain brief as a heredoc.

**Target:** `metasystem adapter selftest-run --runtime <r>` in internal/adapter/selftest.go (which already owns selftest-usage/selftest-envelope/selftest-record/selftest-listener), orchestrating jobs through dispatch and asserting on parsed returns; adapters keep only their per-runtime knobs (ceiling, denial-ends-turn) and the selftest entrypoint.

**Why:** The selftest is the acceptance evidence for every runtime's protocol claims; its judgments (what counts as a denial, what proves a read) are domain policy that shell grep can silently get wrong, and half its helpers already live as Go verbs — the suite is split across the boundary mid-decision.

#### script-adapters-06 — Permission envelope to sandbox/network mapping decided in shell before Go builds the argv

`scripts/agents/adapters/codex.sh:95` · boundary · **medium / effort S**

**Current:** codex_permission_settings (lines 90-105) maps writeRoots==[] → read-only vs workspace-write and network==allow → true/false in bash, then hands the mapped values to Go `adapter codex-command`; hosts/codex.sh line 69 reuses the same shell mapping, and devin.sh lines 314-315 make the analogous auto vs accept-edits decision in shell.

**Target:** Extend `adapter codex-command` (internal/adapter/codex.go) to accept --permissions/--record and derive sandbox/network itself; have `adapter devin-config` (internal/adapter/devin.go) emit the permission-mode alongside the config it already assembles.

**Why:** The envelope→flag mapping is the security-relevant half of command construction (KI-12 in the code's own comment was exactly this going wrong); splitting it so shell decides and Go formats leaves the decision untested and the Go verb's inputs pre-chewed.

#### script-adapters-07 — Devin transcript settle and effective-model fallback decisions live in shell

`scripts/agents/adapters/devin.sh:174` · boundary · **medium / effort M**

**Current:** devin_settle_session_identity (lines 174-199) decides in bash whether the exported transcript session certifies or contradicts the correlated session and writes session-disagreement.txt; devin_record_effective_model (153-172) implements the unobserved-fallback policy around Go's canonical-model; runtime_settle_after_repair (224-236) repeats the no-transcript refusal policy.

**Target:** A `metasystem adapter devin-settle --transcript --session --round-dir` verb in internal/adapter/devin.go returning the verdict (certified / disagreement / unconfirmable) and writing the disagreement artifact, with the model canonicalization+fallback folded in.

**Why:** These are certification decisions (what the record may claim about session and model identity) expressed as bash string comparisons; the same package already owns the correlation half (devin-session), so the adjudication half sitting in shell splits one domain judgment across the boundary.

#### script-adapters-10 — Host outcome adjudication and exit taxonomy triplicated in shell

`scripts/agents/hosts/claude.sh:91` · boundary · **medium / effort M**

**Current:** hosts/claude.sh lines 91-102, hosts/codex.sh lines 91-102, and hosts/devin.sh lines 144-169 each decide failed vs unresumable vs completed from cli_status and the extracted session, and each encodes the exit-code contract (3 = failure, 6 = missing session) that missionrunner interprets.

**Target:** Extend `metasystem host result-write` (internal/host/result.go) — or add a `host finish` verb — to take --cli-status and --raw, decide the outcome, write the envelope, and return the exit code; host scripts just propagate it.

**Why:** The outcome vocabulary and exit taxonomy are the contract missionrunner adjudicates on; three shell copies of that decision (one of which, devin, already needed an extra empty-reply branch) is exactly the shadowing of a Go-understood value the doctrine forbids.

#### script-adapters-11 — Adapter start-gate wait is unbounded, unlike every host's capped wait

`scripts/agents/adapters/runtime-common.sh:50` · error-handling · **medium / effort S**

**Current:** prepare_supervision line 50 (`while [[ ! -e "$gate" ]]; do sleep "$gate_poll"; done`) and fake.sh line 131 poll for the start gate forever. Every host script caps the same wait via METASYSTEM_HOST_START_GATE_TIMEOUT_SEC (default 10s). dispatch.sh only touches the gate after launch-detached, a proc-identity poll, and a record CAS (dispatch.sh lines 601-633), so a dispatcher that dies or fails the CAS in that window leaves a detached supervisor sleeping forever with no heartbeat and no handshake deadline stamped for the reaper backstop.

**Target:** Apply the hosts' pattern: a capped gate wait (env-tunable like the host cap) that exits with a named error, in prepare_supervision and fake.sh's supervise.

**Why:** A silent immortal supervisor process per crashed dispatch is precisely the unsupervised-run failure class this repo's README says the metasystem exists to prevent, and the fix is the ten lines the hosts already have.

#### script-adapters-13 — Host boilerplate quadruplicated across the four host scripts

`scripts/agents/hosts/claude.sh:20` · duplication · **medium / effort S**

**Current:** wait_for_start_gate, the start-turn argument parser, the mission/turn-id validation regexes, and atomic_result are copy-pasted nearly verbatim in hosts/claude.sh (15-51), hosts/codex.sh (19-55), hosts/devin.sh (16-52), and hosts/fake.sh (19-53).

**Target:** A scripts/agents/hosts/host-common.sh sourced by all four (this is plumbing, so a shared shell library is the doctrine-conformant home), leaving each host with only its runtime-specific launch.

**Why:** Four copies of identical plumbing means every fix lands four times or fewer; hosts/fake.sh's gate wait already drifted (it error-messages differently and validates separately), which is the standard first symptom.

#### script-adapters-08 — Schema-into-prompt augmentation duplicated between devin adapter and devin host, already drifted

`scripts/agents/adapters/devin.sh:281` · duplication · **low / effort S**

> downgraded by the verifier: Duplication is real (adapters/devin.sh:280-288 vs hosts/devin.sh:84-92, both hand-maintained), but the cited drift evidence is factually wrong: both copies read 'no property THIS schema does not name' — the 'the schema' variant lives in runtime-common.sh's repair prompt, not in this pair — and the only difference is line-break placement, which is semantically inert for a prompt. With the demonstrated-harm evidence collapsing to zero observed divergence in ~6 lines of static text, this is a could-be-deduped, not a defect; low.

**Current:** devin.sh lines 280-288 and hosts/devin.sh lines 84-92 both append the return schema plus reply instructions to the prompt via printf/cat; the instruction wording has already drifted between the copies ('no property the schema does not name' vs 'no property this schema does not name' with different line breaks).

**Target:** `metasystem adapter devin-prompt --prompt --schema --output` in internal/adapter/devin.go, consumed by both scripts.

**Why:** This text is the only schema channel the runtime has — its exact content decides whether returns validate — and two hand-maintained copies of load-bearing domain prose is how the two paths quietly diverge in behavior.

#### script-adapters-09 — Post-CLI session reconciliation duplicated between claude and codex adapters

`scripts/agents/adapters/claude.sh:179` · duplication · **low / effort S**

> downgraded by the verifier: The branch ladders exist as claimed (claude.sh:177-186, codex.sh:177-184), but the cited drift is not drift: codex_event_field exposes only session|turn — codex has no result model to record, so the difference is a runtime capability fact, not divergence. What remains shared is ~8 lines whose natural home is the adjudicate-turn verb from finding 01; as a standalone finding it shows no demonstrable harm. Low, and effectively subsumed by 01.

**Current:** claude.sh lines 177-186 and codex.sh lines 177-184 implement the same three-way decision after the CLI exits: late handshake from the result/events, resume_collision when the result session differs from the handshaken one, and effective-model recording — each in its own copy of the branch ladder.

**Target:** One shared implementation, ideally inside the adjudicate-turn verb proposed in script-adapters-01 (a --result-session/--result-model input), or minimally a settle_result_identity function in runtime-common.sh.

**Why:** It is protocol state-machine logic (the resume_collision verdict) maintained twice; the copies already differ subtly (claude records the result model, codex does not) and nothing forces them to stay consistent.

#### script-adapters-12 — fake.sh reimplements the shared lifecycle plumbing and hand-writes record/event JSON

`scripts/agents/adapters/fake.sh:129` · duplication · **low / effort M**

> downgraded by the verifier: The duplication is factual (field, parse_supervisor_args, ms-conversion, gate wait, layout math, cas_terminal; inline patch at 157; hand-written event JSONL). But the target's premise is wrong: runtime-common's optional hooks cover only the repair path (runtime_repair_turn/usage_after_repair/settle_after_repair), while fake.sh must inject failures before and inside prepare_supervision's fixed sequence (pending-process-loss, handshake-failure, missing-session-id), so wholesale sourcing requires restructuring the library first. The fixture also exercises the dispatch/Go surface, not runtime-common, and its fixtures run constantly so layout drift breaks loudly, not silently. The defensible remainder — sharing the small helpers and routing the one inline patch through result-patch — is low-value.

**Current:** fake.sh duplicates runtime-common.sh's field(), parse_supervisor_args, millisecond conversion, gate wait, round-dir/heartbeat layout, and finish_running (as cas_terminal) across lines 36-143, so the rounds/<n> and hb/<job> on-disk layout is computed in two shell places plus Go; it also printf's a record patch inline ('{"error":"authentication_failed","phase":"handshake"}', line 157) where every other patch goes through `adapter result-patch`, and fabricates event JSONL lines by hand (178, 219-222).

**Target:** Source runtime-common.sh for the shared lifecycle (the library already isolates runtime specifics behind optional hooks), and route the handshake-failure patch through `adapter result-patch`; event fabrication can move behind a small `adapter fake-event` if drift bites.

**Why:** The fixture exists to exercise the same dispatch surface as the real adapters, so a diverged copy of the lifecycle is a fixture that can silently stop testing what production does — and the duplicated directory-layout math is a contract computed in multiple places.

#### script-adapters-15 — Unquoted $dispatch/$root command expansions in the selftest path

`scripts/agents/adapters/runtime-common.sh:374` · idiom · **low / effort S**

**Current:** Lines 374 and 401/504 invoke `$dispatch status ...` unquoted, and line 436 invokes `$root/scripts/metasystem-config.sh` unquoted, while every other call site quotes them ("$dispatch" reap, etc.); a checkout path containing a space breaks only these selftest calls.

**Target:** Quote the expansions like the rest of the file: "$dispatch" status ..., "$root/scripts/metasystem-config.sh" get ....

**Why:** shellcheck is not in this repo's toolchain so nothing catches the inconsistency, and a break-on-spaces that only manifests inside the acceptance selftest is the worst place to discover it.

### Shell test fixtures

*Verdict: does not yet pass the senior-engineer bar. 22 findings (1 high, 14 medium, 7 low).*

**Reviewer's summary.** Shell test-fixture cluster: 17 files, ~4.5k lines, all invoked from scripts/validate-metasystem.sh or scripts/canary.sh. The craft is unusually high for shell fixtures — strict mode everywhere, ceiling-bounded waits, careful process ownership, and explicit retirement notes documenting what already moved to Go. The dominant issue is not hygiene but migration debt: the Go test suite (internal/contract, internal/mission, internal/missionrunner preflight_fixture_test.go, internal/dispatch/record_test.go, internal/config/confidentity_test.go, internal/report/openwork_test.go, internal/adapter/runtime_test.go, internal/events/emit_test.go, internal/capability/select_test.go, internal/validate/sessionisolation_test.go) now covers, often leg-for-leg, large sections these fixtures still re-prove through the CLI pipe. A second, smaller issue is genuine policy logic embedded as Python-in-shell (a schema linter, an instruction-owner derivation lint, a reimplemented record-CAS fake, a fake `ps`).

Per-file classification:
- scripts/agents/pre-commit-guard-fixtures.sh — PURE-PLUMBING — sandbox setup, stub engine, exit-code and message assertions on a shell guard; the guard is shell, the test must be shell; keep as-is.
- scripts/agents/authority-regression-fixtures.sh — PLUMBING-WITH-LEAKS — WC-1 is a thin CLI probe; WC-3/WC-9 dissect dispatch.sh source with awk/grep (fragile but fails closed, and the subject is shell source, so no Go home fits); keep as-is.
- scripts/agents/evidence-segment-fixtures.sh — PURE-PLUMBING — drives dispatch.sh __reap-held and evidence-gc.sh cross-script, asserts manifests; the legacy-manifest fabrication is legitimate (frozen legacy format); keep as-is.
- scripts/agents/second-session-fixtures.sh — PLUMBING-WITH-LEAKS — the symlink/copy isolation legs duplicate internal/validate/sessionisolation_test.go; the adapter local-config-paths manifest and the WC-8 second-session.sh bootstrap are shell-owned and stay; trim to those.
- scripts/agents/record-protocol-fixtures.sh — PLUMBING-WITH-LEAKS — create/setup/epoch/protocol-error legs duplicate internal/dispatch/record_test.go leg-for-leg; the one unique property (reader never sees failed-without-protocolError) belongs in a Go test; retire after moving that.
- scripts/agents/telemetry-census-fixtures.sh — PLUMBING-WITH-LEAKS — reimplements __record-cas in Python and re-proves modelUsage derivation that internal/adapter/runtime_test.go owns; keep only the claude.sh/runtime-common.sh wiring path, routed through the real dispatcher.
- scripts/agents/lease-succession-fixtures.sh — PLUMBING-WITH-LEAKS — well-scoped (its header documents exactly what moved to internal/lease and TestMissionLineage); real-process announce/renew legs stay shell; the grep-pin of host.go source should become a Go unit assertion.
- scripts/agents/return-schema-fixtures.sh — LOGIC-MUST-MOVE — a ~45-line structured-output schema linter (required lists, additionalProperties, typed nodes) is product validation policy living in Python; Go home: internal/returnschema test over `schema materialize` output for every role. The normalize_return legs are thin and stay.
- scripts/agents/supervision-go-fixtures.sh — PLUMBING-WITH-LEAKS — deliberately complements internal/supervise/owner_test.go (fakes) with the real running binary; keep as-is, but fix the machine-wide pkill and the caps hardcoded outside fixture-budget.sh.
- scripts/agents/fixture-budget.sh — LOGIC-MUST-MOVE by the letter of the rubric (it is pure timing/calibration policy), but the right disposition is keep-until-migration-completes: it is the declared single owner of harness caps for the shell fixtures and dies naturally as they migrate; a standalone Go port would have no consumer.
- scripts/agents/config-identity-fixtures.sh — PLUMBING-WITH-LEAKS — lines 13–108 duplicate internal/config/confidentity_test.go test-for-test, and the snapshot-select refusal duplicates internal/capability/select_test.go; keep only the executable-appendix data pin plus one CLI smoke.
- scripts/agents/flight-recorder-fixtures.sh — PLUMBING-WITH-LEAKS — caller-harmlessness, cross-process concurrency, torn-fragment, and witness-not-authority legs only real processes can prove: keep. The registry-door, hard-cap, and stream-sweep legs duplicate internal/events/emit_test.go, and FRCC-011 is vacuous.
- scripts/agents/fingerprint-harness.sh — PLUMBING-WITH-LEAKS — an operator diagnostic harness (KI-18); domain predicates read state via the engine's json verbs, which is the right idiom; keep as-is, replacing its perl conf rewrite with the engine's config tailor once extended.
- scripts/agents/delegate-caps-fixtures.sh — PLUMBING-WITH-LEAKS — AUTH-R2-001 duplicates internal/mission/fence_test.go; 002/003 (pin drift) should become Go tests then retire; the real-supervision legs 005–007 stay shell but the fake `ps` + background identity-updater daemon reimplement a process-identity seam the engine should own.
- scripts/agents/conformance-fixtures.sh — PLUMBING-WITH-LEAKS — the merge-gate matrix substantially overlaps internal/validate/conformance_test.go and the exhaustion section overlaps internal/dispatch tests; the G-5 leg is a live-repo instruction-owner lint in Python that must move to Go; keep a thin stage-level E2E through assert-conformance.sh.
- scripts/agents/mission-fixtures.sh — PLUMBING-WITH-LEAKS (largest duplication) — the 27-key grammar mutation matrix and seal/preflight legs duplicate internal/contract/contract_test.go; state/ledger/anchor legs duplicate internal/mission tests; end-state runner legs are now duplicated by internal/missionrunner's preflight fixture tests. Keep: mission-runner.sh process-level start/status, the concurrency-fence race, and the hosts/claude.sh adapter legs (shell subjects).
- scripts/agents/supervision-fixtures.sh — PLUMBING-WITH-LEAKS — the arming/heal/takeover/hook/foreign-owner legs are true process-level integration and stay; the open-work/stale-plan/gate-marker section duplicates internal/report and internal/gaterun 1:1, and the shell-side ERE pre-check validates signatures with the wrong regex engine.

#### script-fixtures-002 — Structured-output schema linter lives as Python in a shell fixture

`scripts/agents/return-schema-fixtures.sh:43` · boundary · **high / effort S**

**Current:** A ~45-line Python walker enforces the structured-output rules (every object typed, complete required list, additionalProperties:false, every property declaring a type) over `schema materialize` output for six roles. The comment notes two shipped defects motivated it.

**Target:** A Go test in internal/returnschema (or cmd/metasystem's schema tests) that materializes every role's version-2 schema and walks it with the same invariants; the shell file keeps only its thin normalize_return and assert-return-complete legs.

**Why:** These are generator invariants of Go-owned code guarding a dispatch-critical surface; asserting them in the generator's own package makes them run under the go gate and survive fixture retirement.

#### script-fixtures-001 — G-5 instruction-owner derivation lint is domain policy in embedded Python

`scripts/agents/conformance-fixtures.sh:198` · boundary · **medium / effort M**

> downgraded by the verifier: Facts verified: 37-line Python block at conformance-fixtures.sh:198-234 runs against $source_root (live repo), and internal/validate/preamblequotes.go:21 parses the same quote-source markers; conformance_test.go already reads repo files via ../../ as precedent. Downgraded because the check works today and the proposed 'no new verb, go gate runs it' target silently drops adopted-repo enforcement: validate-metasystem.sh:306 runs conformance-fixtures.sh unconditionally (adopted repos included), while a Go test runs only where Go source exists. The heuristic fragility ('owns'/'only routing index' line matching) is also unchanged by the move. Boundary claim valid; fix needs either an engine verb or an explicit owner-accepted scope reduction.

**Current:** A 37-line Python block derives rule-owning document references from AGENTS.md (heuristics: lines containing 'owns', 'only routing index', 'lists the project'), role preamble quote-source attributes, and the host-turn template, then gates on coverage by instruction-bearing-paths.txt. It runs against the live repo ($source_root), not the sandbox — it is a repo conformance rule, not a fixture assertion.

**Target:** A Go test in internal/validate (beside preamblequotes.go, which already parses the same quote-source markers) that derives the owner set and asserts coverage; delete the Python block. No new verb needed — the go gate runs it.

**Why:** House doctrine puts decision logic in Go; this is a heuristic policy parser whose false negatives silently un-protect canonical instruction files. internal/validate already owns the adjacent parsing.

#### script-fixtures-003 — Contract grammar matrix and seal/preflight legs duplicate internal/contract tests

`scripts/agents/mission-fixtures.sh:152` · duplication · **medium / effort M**

> downgraded by the verifier: The preflight legs (mission-fixtures.sh:219-243) are true 1:1 duplicates of TestContractPreflightUnsealed/Unsigned/ApprovalHashMismatch/StaleExposure. But the 'covering the same surface' claim is overstated: TestContractValidateRejects (contract_test.go:144-187) has 7 rows against the shell matrix's ~26 keys x 2 variants, so most of the per-key grammar coverage currently exists ONLY in shell and the bulk of the work is porting, not deleting. The finding's port-then-retire sequencing is right and the retained seal-sign-preflight smoke preserves script-forwarding proof, but as mostly-port-needed rather than mostly-duplicated it is medium, not high.

**Current:** Lines 146-243: a Python mutation table generates missing/malformed variants for all 27 contract keys (~54 assert-mission.sh subprocesses per run), then unsealed/unsigned/mismatched-hash/stale-exposure preflight legs. internal/contract/contract_test.go has TestContractValidateRejects, TestContractPreflightUnsealed/Unsigned/ApprovalHashMismatch/StaleExposure covering the same surface in-process.

**Target:** Verify per-key parity of the malformed-value table inside TestContractValidateRejects (port any missing rows), then delete the shell matrix and preflight legs, keeping one seal-sign-preflight smoke through assert-mission.sh to prove the script forwards correctly.

**Why:** Re-proving table-tested Go code through a subprocess pipe ~60 times per suite run is exactly the retirement criterion these files' own comments state; the bad-value table is domain knowledge that belongs beside the parser.

#### script-fixtures-004 — Mission state/ledger/anchor legs duplicate internal/mission unit tests

`scripts/agents/mission-fixtures.sh:315` · duplication · **medium / effort S**

**Current:** Lines 306-396 drive ledger-init/append, state-init/write/verify, chain-fork detection, reconcile exit 3, anchor round trip and rewritten-anchor refusal via the CLI with Python JSON assertions. internal/mission's state_test.go (TestChainDetectsTamper, TestWriteRefusesIllegalTransition), ledger_test.go, and anchor_test.go (TestVerifyAnchorDetectsLedgerTamper, TestReconcile*) cover the same behavior in-process.

**Target:** Delete these legs after confirming the reconcile-fork exit-3 park and self-park-stop-loss refusal have Go equivalents (add them to anchor_test/state_test if absent); the CLI arg-forwarding is already exercised by the retained runner legs.

**Why:** Same behavior proven twice, the shell copy through Python asserts that drift silently from the Go schema; the Go copy is the one that runs on every gate.

#### script-fixtures-005 — Mission end-state legs now duplicated by internal/missionrunner fixture tests

`scripts/agents/mission-fixtures.sh:574` · duplication · **medium / effort M**

**Current:** The gate-and-close and runner-closes-chain end-state legs (lines 574-633) assert completed state, landed-orphan prompt/ledger/usage annotations, and runner-closed chains. internal/missionrunner's preflight_fixture_test.go (TestInternalRunCloseStreamCycle, TestInternalRunDispatchTerminalCycle, TestArmAndPreflightFullPass) and orphanusage_test.go (TestDeliverLandedUnconsumedWritesFinalBlock) now cover these cycles in-process, including the real fake-host adapter.

**Target:** Keep only what the Go fixture cannot reach: mission-runner.sh's process-level start --foreground path, the status exit-code contract, and the armed/unarmed/lease preflight probes that exercise assert-mission.sh --preflight against real held processes. Retire the duplicated end-state JSON assertions.

**Why:** The repo's declared direction is fixture coverage migrating into Go tests; this is the freshest duplication (commits 5c6c62e..5e20328) and the shell copy is the slowest, flakiest leg of the suite.

#### script-fixtures-006 — AUTH-R2-001..003 duplicate or belong in internal/mission fence tests

`scripts/agents/delegate-caps-fixtures.sh:66` · duplication · **medium / effort S**

**Current:** AUTH-R2-001 (pair cap selected, lower argument narrows, above-signed refused) duplicates TestAuthorizeCapUsesPairCap and TestAuthorizeCapRefusesAboveSigned in internal/mission/fence_test.go. AUTH-R2-002/003 (pinned-bytes drift and whitespace-only drift refused against approvedContractSha256) have no named Go test.

**Target:** Add pin-drift and whitespace-drift cases to internal/mission/fence_test.go, then retire 001-003 from the shell file, leaving the registry check (AUTH-R2-009) over the remaining supervision legs.

**Why:** 001 is already double-covered; 002/003 are pure file-in/refusal-out logic that needs no processes, and moving them closes the fixture's own retirement note (AUTH-R2-004 already retired this way).

#### script-fixtures-007 — Fake ps on PATH plus a background identity-updater daemon shadow the engine's process-identity seam

`scripts/agents/delegate-caps-fixtures.sh:216` · boundary · **medium / effort M**

**Current:** Lines 179-241 install a Python `ps` shim ahead of PATH and run a 20ms-poll background loop that fcntl-locks and rewrites METASYSTEM_FAKE_PROCESS_IDENTITY_FILE to mirror supervision state.json components, so ownership proofs pass under the fake runtime.

**Target:** Extend the engine's existing fixture seam (METASYSTEM_FAKE_PROCESS_IDENTITY_FILE, already honored per internal/census verbs_test.go TestAuthIdentityFixtureFile) so every ownership-proof path reads it directly, letting the fixture pre-register component identities once instead of intercepting ps and polling; alternatively migrate AUTH-R2-005..007 to a missionrunner-style Go fixture test with an injected process table.

**Why:** A PATH-level ps shim changes behavior of every subprocess in the fixture, and the poll loop is a race generator on loaded machines (the known suite-flake mode); the engine owning one injectable process source removes both.

#### script-fixtures-010 — FRCC-011 is vacuous: both its command and its assertion end in || true

`scripts/agents/flight-recorder-fixtures.sh:192` · testing · **medium / effort S**

**Current:** The lease-refused witness leg runs `lease announce ... || true` then `grep -q '"event":"lease-refused"' ... || true`, so the named proof row can never fail; the comment defers to 'the unit case below', which is not in this file (it is internal/lease/refusals_test.go).

**Target:** Delete the leg and point the FRCC-011 label at internal/lease/refusals_test.go, or make it real by announcing from a second process identity so the refusal path actually fires and the grep asserts.

**Why:** A proof row that cannot fail is worse than absent — it reads as coverage in the fixture's own PASSED banner while proving nothing.

#### script-fixtures-012 — Record create/setup/epoch/protocol-error legs duplicate internal/dispatch/record_test.go

`scripts/agents/record-protocol-fixtures.sh:89` · duplication · **medium / effort M**

**Current:** The fixture proves pending-setup persistence, epoch-mismatch setup refusal, protocol-error stamping and idempotency — all covered by TestRecordCreateWritesReservation, TestRecordSetupCompletesReservation, TestRecordSetupRefusesEpochMismatch, TestRecordProtocolErrorStampsAndIsIdempotent. The only unique property is the concurrent Python reader asserting no reader ever sees status=failed without its protocolError object.

**Target:** Move the reader-visibility property to a Go test (a goroutine re-reading the record during Protocol-error application; atomicfile's rename already guarantees it) plus the existing .tmp-residue check, then retire this file or shrink it to a single __record-create CLI smoke.

**Why:** Everything but one assertion is double-cover through a slower pipe; the one novel property is stronger and less racy as an in-process test.

#### script-fixtures-013 — fixture_record_cas reimplements the dispatcher's __record-cas in Python

`scripts/agents/telemetry-census-fixtures.sh:12` · boundary · **medium / effort S**

**Current:** The fixture substitutes a Python function for `dispatch` that re-implements CAS semantics (expect-status check, patch merge, status write) to unit-test claude.sh's record_result_effective_model/normalize_return wiring; the model-derivation cases (one-key/zero-keys/two-keys) additionally duplicate internal/adapter/runtime_test.go's ClaudeResultField coverage.

**Target:** Point the wiring test at the real dispatch.sh __record-cas against a scratch job record (the adapters already resolve $dispatch), and drop the three model-derivation expectations in favor of the Go tests; the CENSUS-FAILED schema leg at the CLI boundary can stay.

**Why:** A hand-rolled CAS fake drifts from the real transition rules (record_test.go shows CAS also refuses status-in-patch and immutable fields), so the wiring is being proven against semantics production does not have.

#### script-fixtures-014 — Config-identity legs duplicate internal/config/confidentity_test.go test-for-test

`scripts/agents/config-identity-fixtures.sh:47` · duplication · **medium / effort S**

**Current:** Bookkeeping-churn invariance, change sensitivity, JSON canonical equivalence, and malformed/out-of-range filter fallback (lines 13-108) map one-to-one onto TestConfigIdentityIgnoresBookkeepingChurn, TestConfigIdentityChangeSensitive, TestConfigIdentityStableAcrossEquivalentJSON, TestConfigIdentityMalformedAndOutOfRangeFilterHashesFullMap; the snapshot-select refusal naming changed keys duplicates internal/capability/select_test.go TestSelectNoMatchNamesChangedKeys.

**Target:** Shrink the file to the executable-appendix pin (lines 152-168, real data worth pinning at the shipped JSON files) plus at most one `config identity` CLI smoke proving flag forwarding.

**Why:** Five near-identical proofs already run under the go gate; the shell copies add wall time and a second Python assertion layer that must track the JSON output shape.

#### script-fixtures-016 — Cleanup pkill -f "gofix-" can kill unrelated processes machine-wide

`scripts/agents/supervision-go-fixtures.sh:20` · error-handling · **medium / effort S**

**Current:** The EXIT trap runs `pkill -f "gofix-"`, matching any process on the host whose command line contains the substring — an editor with gofix-notes.md open, another checkout's run, or a coincidental match — not just the four owners this fixture launched (whose pids it already captures in owner1..owner4).

**Target:** Kill the tracked owner pids (and pgrep by the full instance tags rooted in $tmp) instead of a bare substring pkill; keep pkill only as a last-resort fallback scoped to patterns containing $tmp.

**Why:** The suite runs on developer machines and under peer agents; a substring pkill from a test's exit trap is a real collateral-damage vector.

#### script-fixtures-017 — Hardcoded wait ceilings bypass fixture-budget.sh, the declared single owner of caps

`scripts/agents/supervision-go-fixtures.sh:43` · testing · **medium / effort S**

**Current:** wait_until deadlines are fixed literals (8s, 30s) with sleep 0.1 polls and a bare `sleep 3` stability window; the file neither sources scripts/agents/fixture-budget.sh nor scales with METASYSTEM_FIXTURE_CAP_SCALE, although fixture-budget.sh's header declares itself the only owner of harness cap values.

**Target:** Source fixture-budget.sh, register named caps for these waits (e.g. supervision-wait), and poll at METASYSTEM_FIXTURE_POLL_INTERVAL_SEC so loaded machines get the same headroom scaling every other fixture gets.

**Why:** Timing fixtures flaking under load is this suite's documented failure mode; an unscaled 8-second ceiling on a real-process owner establish is the first thing to trip on a busy host, and the policy violation makes cap audits incomplete.

#### script-fixtures-020 — Three divergent perl one-liners re-implement conf tailoring the engine owns

`scripts/agents/supervision-fixtures.sh:110` · duplication · **medium / effort M / needs sign-off**

**Current:** supervision-fixtures.sh:110, fingerprint-harness.sh:125, and mission-fixtures.sh:463 each carry a multi-clause perl -0pi regex rewriting metasystem.conf to the fake runtime (runtimes, evidence.root, role/model/tier keys), encoding conf-key grammar in regex three slightly different ways. The engine already owns this rewrite: validate.TailorConf behind `metasystem config tailor` (used by adopt.sh), which currently accepts only claude/devin/codex/none.

**Target:** Extend `config tailor` to accept the fake runtime (or a --fixture profile that also sets evidence.root and watch.interval-sec overrides), then replace all three perl rewrites with one engine call.

**Why:** Conf-grammar knowledge in fixture regex drifts silently when keys change (the fingerprint-harness variant already diverges from supervision-fixtures'); the Go rewriter is tested and atomic. Verb-surface extension needs owner sign-off.

#### script-fixtures-021 — Dispatcher exhaustion section overlaps internal/dispatch critique-exhaustion tests

`scripts/agents/conformance-fixtures.sh:462` · duplication · **medium / effort M**

**Current:** The __critique-exhaustion legs (wrong-party enumeration refusal, critic-cannot-own-successor, budget reopen via recorded implementer successor, protocol-error recovery, second-budget human-only remedy, record-round-beats-return-round) run through dispatch.sh; internal/dispatch's decisions_test.go (TestCritiqueExhaustionDesignCritic, TestCritiqueExhaustionRoundOffBudget) and exhaustion_direct_test.go (TestCritiqueExhaustionGuards) cover the same owner in-process.

**Target:** Diff the shell legs against the Go tests, port the cases the Go side lacks (protocol-error recovery with absent return, and the lying-return round-3 case if missing), then drop this section, keeping the assert-conformance.sh stage-level E2E above it.

**Why:** The exhaustion owner is pure Go decision logic; the shell copy fabricates six job records in Python per run to reach it, and every future rule change must be mirrored twice.

#### script-fixtures-008 — Open-work, stale-plan, and gate-marker legs duplicate internal/report and internal/gaterun 1:1

`scripts/agents/supervision-fixtures.sh:882` · duplication · **low / effort S**

> downgraded by the verifier: The 1:1 parity claim is false. openwork_test.go's five tests cover only the basic unblocked/settled/waiting/in-flight/stale cases; the shell section also proves chain-root round matching (plan names 'design-critic-...-aaaa' while '...-aaaa-r3' runs, lines 948-951), per-stream staleness isolation (other.md, 952-958), plans/README exclusion (960-963), and the OpenWork-to-gate-marker integration with live-marker silencing and dead-marker pruning through report open-work (906-935) — gaterun_test.go proves Running()/prune only at package level, not the reporter integration. Deleting the section as proposed loses real coverage; the genuinely duplicated five legs are a small fraction of the 90 lines. Port the missing cases into Go first, then retire.

**Current:** Lines 875-963 drive `report open-work` through unblocked/settled/waiting/in-flight/stale-plan/chain-root cases and gate register/dead-marker pruning. internal/report/openwork_test.go (five tests matching these legs by name) and internal/gaterun/gaterun_test.go (TestRegisterThenRunningIsTrueForLiveGate, TestRunningPrunesDeadMarker) cover every case in-process.

**Target:** Delete the section; keep the supervision-hook.sh legs (S4-14, S4-15, idle wording) that exercise the shell hook itself, which is the only part with no Go home.

**Why:** Ninety lines of exact double-cover in the suite's largest and slowest file; the Go copies run under the gate on every platform.

#### script-fixtures-009 — S4-7 shell-side ERE validity check uses grep's engine, not the engine that consumes signatures

`scripts/agents/supervision-fixtures.sh:341` · boundary · **low / effort S**

> downgraded by the verifier: Wrong-engine claim verified: supervision-fixtures.sh:341 validates patterns with grep's POSIX ERE while the consumer compiles with Go regexp.Compile (RE2, census/signature.go:27), and the authoritative `proc signature-check` runs immediately after (line 350) with invalid-ERE fail-closed proven at 353-367. Downgraded because the redundant pre-check cannot mask a production defect — signature-check gates every pattern anyway — so the only failure mode is a spurious fixture failure on an engine-disagreement pattern. Removal is correct cleanup, but it is hygiene, not a property the fixture wrongly asserts reaching production.

**Current:** Each adapter signature line is validated with `printf '' | grep -Eq -- "${line#* }"` — grep's POSIX ERE — while the consumer of those patterns is Go's regexp inside `proc signature-check`/the census engine. The two engines disagree on accepted patterns, and the authoritative check (`proc signature-check`, whose invalid-ere fail-closed behavior the very next loop proves) runs immediately after.

**Target:** Drop the shell-side grammar/ERE pre-check loop and rely on `proc signature-check` for both positive/lookalike and validity; if line-grammar (match/exclude prefix) needs an explicit proof, assert it inside signature-check's Go tests (TestSignatureCheckContract).

**Why:** A wrong-engine validator can pass a pattern Go rejects (or vice versa), making the fixture assert a property production does not have.

#### script-fixtures-011 — Registry-door, hard-cap, and stream-sweep legs duplicate internal/events/emit_test.go

`scripts/agents/flight-recorder-fixtures.sh:111` · duplication · **low / effort S**

**Current:** Section 5 (Python sweep re-validating every emitted event against event-registry.json), section 4 and FRCC-002 (4096-byte cap), and FRCC-001 (unregistered event / wrong emitter dropped) re-prove TestEmitDropsUnregisteredEventAndWrongEmitter, TestEmitHonorsHardCap, TestEmitShrinksOptionalFieldsUnderCap, and TestEmitWritesRegisteredEvent.

**Target:** Keep sections 1-3 and 6-7 (caller harmlessness under set -e, cross-process concurrent writers, torn fragment, witness-not-authority under chmod 000) — those need real processes — and drop the duplicated door/cap legs and the section-5 sweep.

**Why:** Cheap but real: the Python registry sweep is a second registry-conformance implementation that can drift from the Go door it mirrors.

#### script-fixtures-015 — Isolation copy/symlink legs duplicate internal/validate/sessionisolation_test.go

`scripts/agents/second-session-fixtures.sh:44` · duplication · **low / effort S**

**Current:** The copy-verification and symlink-into-primary refusal legs re-prove TestSessionIsolationCopiesAndResolvesHarness and TestSessionIsolationRejectsSymlinkIntoPrimary through the CLI.

**Target:** Keep the adapter local-config-paths manifest aggregation (shell adapters are the subject) and the WC-8 human-shell bootstrap of second-session.sh; drop the duplicated validate session-isolation legs.

**Why:** Small, but the remaining shell legs are exactly the cross-boundary ones worth the subprocess cost; trimming keeps the file honest about what it proves.

#### script-fixtures-018 — make_repo's first cp deposits stray script copies at the fixture repo root

`scripts/agents/supervision-fixtures.sh:103` · dead-code · **low / effort S**

**Current:** `cp watch-background-jobs.sh metasystem-config.sh metasystem.conf "$repo/scripts/../"` puts all three at $repo root; the next three lines then copy each to its correct shipped path, and the comment admits the first copy is misplaced. The stray root-level watch-background-jobs.sh and metasystem-config.sh are committed into every fixture repo by `git add .`.

**Target:** Delete the first cp line; the three explicit copies below it already place every asset at its shipped path.

**Why:** Committed stray copies of path-sensitive scripts inside the sandbox can mask path-resolution bugs (a wrong resolver finding the root-level copy would pass) and confuse anyone reading the fixture repo.

#### script-fixtures-019 — Lineage-export wiring pinned by grepping Go source text from shell

`scripts/agents/lease-succession-fixtures.sh:119` · testing · **low / effort S**

**Current:** The fixture greps internal/missionrunner/host.go for the literal '"METASYSTEM_OWNER_LINEAGE="+MissionLineage(e.Mission)' (guarded by a go.mod check for template checkouts) to prove the host launcher still exports the lineage.

**Target:** A Go unit test in internal/missionrunner asserting the constructed host command's environment contains METASYSTEM_OWNER_LINEAGE=<lineage> (host_process_test.go is the natural home); delete the source grep.

**Why:** A byte-level source pin breaks on any refactor of the expression (variable extraction, helper rename) while the behavior it guards survives; the env-content assertion is refactor-proof and runs in adopted-repo CI too.

#### script-fixtures-022 — Whitespace-sensitive JSON grep and a private Python json_field where the engine ships json get

`scripts/agents/mission-fixtures.sh:561` · idiom · **low / effort S**

**Current:** wait_end_state detects runner failure with `grep -Fq '"status": "failed"'` — dependent on the Go writer's exact indentation; if the record format compacts, the fast-fail branch never fires and every failure degrades into a full timeout with no diagnostic. supervision-fixtures.sh:122 likewise re-implements json_field in Python while fingerprint-harness.sh already uses `metasystem json get` for the same job.

**Target:** Use `"$repo/bin/metasystem" json get --file ... --field status` in wait_end_state, and replace supervision-fixtures' Python json_field with the engine verb.

**Why:** Format-coupled greps fail silent-slow instead of loud-fast, and two private JSON readers exist beside an engine verb built for exactly this.

### Remaining scripts and data files

*Verdict: does not yet pass the senior-engineer bar. 10 findings (2 high, 3 medium, 5 low).*

**Reviewer's summary.** Catch-all cluster: the six shell scripts under scripts/ not owned by the gate-shim or agents clusters, plus every .json/.md/.yml/.txt data file under scripts/ (43 files). Overall state: the pure-forwarder standard set by kill-shell Phase A (receipt.sh, frontier.sh, assert-stop-loss.sh, assert-design-obligation-gate.sh, audit-metasystem.sh are all 10-line exec shims) is NOT yet met by three scripts in this cluster — refactor-baseline.sh, validate-skill.sh, and watch-background-jobs.sh still make domain decisions in bash/awk — and adopt.sh keeps adoption's recognition/refusal decisions in shell (its Phase E port was superseded, not resolved, by the 2026-08-12 core-vs-plumbing ruling). Hygiene of the remaining shell is genuinely good: set -euo pipefail everywhere, careful quoting, refusal-first error handling, scar-tissue comments with dated incidents. The shipped CI workflow, however, cannot pass in an adopted repository (engine binary gitignored, no Go source shipped), which intersects the tracked adopted-engine-delivery severance.

Per-file classification:
- scripts/adopt.sh — PLUMBING-WITH-LEAKS — staging/copy/symlink/hook installation is legitimate bootstrap plumbing and conf tailoring plus permission logic already delegate to Go (`config tailor`), but target-state recognition/refusal (lines 116-135), the payload allowlist policy, and sed-produced settings.json are decisions in shell.
- scripts/canary.sh — PURE-PLUMBING — a static change-class-to-fixture-driver dispatch table that refuses unknown classes; parses no domain state, computes nothing Go understands.
- scripts/metasystem-config.sh — PURE-PLUMBING — flag translation and exec into `metasystem config get/keys/validate`; exemplary shim.
- scripts/refactor-baseline.sh — LOGIC-MUST-MOVE — the entire refactor-gate decision (baseline record parse, dirt classification via awk over porcelain, ancestry, cadence backstop) lives in shell; Go home: validate family (see finding 2).
- scripts/validate-skill.sh — LOGIC-MUST-MOVE — skill frontmatter contract parsed and judged in sed/awk; Go home: `metasystem validate skill` (finding 8).
- scripts/watch-background-jobs.sh — LOGIC-MUST-MOVE — the arm/poll/sleep loop and state file are legitimate plumbing under the 2026-08-12 ruling, but the five-state watchdog classification, terminal-status vocabulary, sidecar/primary selection, scope containment, and census-log rotation are decision engines in shell; Go home: report/supervise family per the plan's own r3/KS-R3-009 (finding 3).
- scripts/agents/adapters/claude-config-filter.v1.json — PURE-PLUMBING (data) — runtime config-churn filter; shell only passes the path (runtime-common.sh:300 --filter), Go parses; Go owns the schema.
- scripts/agents/adapters/codex-config-filter.v1.json — PURE-PLUMBING (data) — same contract, Go-owned; carries dated churn provenance per key.
- scripts/agents/adapters/devin-config-filter.v1.json — PURE-PLUMBING (data) — same contract, Go-owned.
- scripts/agents/coverage-ratchet.json — PURE-PLUMBING (data) — darwin coverage floors judged by the Go verb `audit coverage-ratchet` (cmd/metasystem/audit.go); Go owns the schema; shell (go-gate.sh, other cluster) only selects darwin vs linux file.
- scripts/agents/coverage-ratchet-linux.json — PURE-PLUMBING (data) — linux floors, same Go owner.
- scripts/agents/event-registry.json — PURE-PLUMBING (data) — flight-recorder envelope and event contract; Go owns parsing and enforcement (internal/events/emit.go:162); shell emits only through emit-event.sh which forwards to the binary.
- scripts/agents/permissions/none.json — PURE-PLUMBING (data) — permission envelope; shell resolves preset name to path (dispatch.sh:551), Go expands and clamps (`job expand-permissions`, internal/adapter/permissions.go); Go owns the schema.
- scripts/agents/permissions/workspace.json — PURE-PLUMBING (data) — same; the <worktree> placeholder is rewritten by Go (RewriteWriteScope), never by shell.
- scripts/agents/roles/behavior-judge.requirements.json — PURE-PLUMBING (data) — capability requirements, Go-owned (internal/capability/select.go:81).
- scripts/agents/roles/code-critic.requirements.json — PURE-PLUMBING (data) — same Go owner.
- scripts/agents/roles/design-critic.requirements.json — PURE-PLUMBING (data) — same Go owner.
- scripts/agents/roles/implementer.requirements.json — PURE-PLUMBING (data) — same Go owner; only file with a non-empty optional block (resume fallback).
- scripts/agents/roles/investigator.requirements.json — PURE-PLUMBING (data) — same Go owner.
- scripts/agents/roles/verifier.requirements.json — PURE-PLUMBING (data) — same Go owner.
- scripts/agents/schemas/behavior-judge.schema.json — PURE-PLUMBING (data) — role return contract; Go owns enforcement (internal/returnschema/returnschema.go:71, internal/validate/returncomplete.go:148).
- scripts/agents/schemas/code-critic.schema.json — PURE-PLUMBING (data) — same Go owner.
- scripts/agents/schemas/design-critic.schema.json — PURE-PLUMBING (data) — same Go owner.
- scripts/agents/schemas/implementer.schema.json — PURE-PLUMBING (data) — same Go owner.
- scripts/agents/schemas/investigator.schema.json — PURE-PLUMBING (data) — same Go owner.
- scripts/agents/schemas/mission-state.schema.json — PURE-PLUMBING (data) — but an ORPHAN: no Go or shell code validates against it (the suite only existence-checks it at validate-metasystem.sh:244); the real contract lives in internal/mission/state.go and has already drifted (finding 6).
- scripts/agents/schemas/orchestrator.schema.json — PURE-PLUMBING (data) — Go-owned like the other role schemas.
- scripts/agents/schemas/verifier.schema.json — PURE-PLUMBING (data) — same Go owner.
- scripts/agents/roles/behavior-judge.md — PURE-PLUMBING (data) — role preamble; quote provenance enforced by check-preamble-quotes/Go `validate preamble-quotes`.
- scripts/agents/roles/code-critic.md — PURE-PLUMBING (data) — same.
- scripts/agents/roles/design-critic.md — PURE-PLUMBING (data) — same.
- scripts/agents/roles/implementer.md — PURE-PLUMBING (data) — same.
- scripts/agents/roles/investigator.md — PURE-PLUMBING (data) — same.
- scripts/agents/roles/orchestrator.md — PURE-PLUMBING (data) — byte-compared into assembled prompts by Go (internal/validate/turnprompt.go:137).
- scripts/agents/roles/verifier.md — PURE-PLUMBING (data) — same family.
- scripts/agents/templates/brief.md — PURE-PLUMBING (data) — dispatch brief skeleton.
- scripts/agents/templates/follow-up.md — PURE-PLUMBING (data) — follow-up round skeleton.
- scripts/agents/templates/host-turn-instruction.md — PURE-PLUMBING (data) — read by Go (internal/mission/prompt.go:433).
- scripts/agents/instruction-bearing-paths.txt — PURE-PLUMBING (data) — waiver-ineligible path list, Go-owned consumer (internal/validate/conformance.go:452).
- scripts/enforcement/claude-code-hooks.json — PURE-PLUMBING (data) — hook wiring for an external runtime; the inline Stop command only translates receipt.sh exit codes into systemMessages (decision stays in Go); schema is owned by the external runtime, installed by adopt.sh (with a sed munge, finding 5).
- scripts/enforcement/codex-hooks.json — PURE-PLUMBING (data) — same shape, copied verbatim.
- scripts/enforcement/devin-hooks.json — PURE-PLUMBING (data) — same shape, copied verbatim.
- scripts/enforcement/github-actions-metasystem.yml — PURE-PLUMBING (data) — 15-line CI wiring, but as shipped it cannot pass in an adopted repository (finding 1).

JSON ownership answer in one line: of the 25 .json files, 22 encode domain contracts that Go alone parses and enforces today (role schemas, requirements, permissions, config filters, event registry, coverage ratchets); the 3 enforcement hook files belong to external runtimes' schemas; the single exception is mission-state.schema.json, which nothing consumes and which shadows the Go-owned contract.

#### script-misc-1 — Shipped CI workflow cannot pass in an adopted repository: no engine, no source, no build step

`scripts/enforcement/github-actions-metasystem.yml:15` · boundary · **high / effort L / needs sign-off**

**Current:** The workflow's whole job is `bash scripts/validate-metasystem.sh` on a fresh checkout of the ADOPTED repo. But adoption delivers the engine only as an untracked binary (adopt.sh:251 copies bin/metasystem; the merged .gitignore ships 'bin/' from the template's line 2) and ships no Go source (the payload allowlist at adopt.sh:162 has no cmd/, internal/, or go.mod). In CI the suite skips the go-gate build (metasystem_go_source=0 at validate-metasystem.sh:75) and then unconditionally runs `scripts/audit-metasystem.sh .` (validate-metasystem.sh:126), whose `exec "$ms" audit metasystem` hits a nonexistent bin/metasystem and dies with 127.

**Target:** Resolve engine delivery for adopted targets so the shipped workflow is honest: either the workflow rebuilds/downloads the engine before the suite, or adoption commits the binary (drops bin/ from the shipped .gitignore), or the payload ships Go source. This is exactly the SEVERED adopted-engine-delivery decision in plans/kill-shell.md (r10) — the finding here is that the shipped CI enforcement is red-on-day-one in every adopted repo until that ruling lands, which the severance text does not state.

**Why:** The README sells this file as the enforcement that runs 'without anyone remembering'; in the one environment it is shipped for, it fails on the first push with exit 127 rather than a diagnosable refusal.

#### script-misc-3 — Watchdog job-file classification is a 200-line decision engine in shell

`scripts/watch-background-jobs.sh:265` · boundary · **high / effort L / needs sign-off**

*Same migration as script-orchestration-06; one backlog item (W4.6).*

**Current:** The five-state classification is all shell: the terminal-status vocabulary (is_terminal, lines 265-270), the DONE/CAPPED/NEVER-STARTED/STALE threshold ladder (lines 341-359), sidecar-vs-primary record selection (lines 314-323), scope containment (in_scope, lines 252-258), VANISHED bookkeeping (lines 364-374), and census-log size rotation policy (append_census_log, lines 200-210). The engine is used only as a JSON field reader (`$ms json get`) and for the census pass (`supervise watcher-pass`). The plan's own ruling r3/KS-R3-009 (plans/kill-shell.md Phase D) already assigned JOB-FILE classification to the report family before Phases B-F were superseded.

**Target:** Move the single-pass scan into a Go verb (report or supervise family, e.g. `report watch-scan --dir ... --scope ... --state ...`) that emits the existing stable report lines and updates the state file; the script keeps what the 2026-08-12 ruling calls plumbing — arming, the poll loop, sleep, signal lifetime. Report-line format and state-file behavior stay byte-compatible; the new verb needs owner sign-off.

**Why:** This is the supervision failure-detection brain — the exact class of logic (thresholds, status vocabulary, selection policy) the house doctrine says must be unit-testable Go, and the values it computes (job status, workspace scope) are ones the Go side already understands from the same records.

#### script-misc-2 — The refactor gate is the last policy gate still deciding in shell

`scripts/refactor-baseline.sh:100` · boundary · **medium / effort M / needs sign-off**

> downgraded by the verifier: Facts all check out: parsing at lines 100-102, awk dirt classification at 72-79, ancestry at 108, cadence at 109-117; the named Phase A siblings (receipt, frontier, stop-loss, obligation gate, audit, critique-closed) are verified exec shims; F Q1.12 groups refactor-baseline.sh in the same policy-facing class; the port target preserves the on-disk format and exit codes and flags sign-off, consistent with the 2026-08-12 core-vs-plumbing ruling. Downgraded because high implies demonstrable harm and there is none: no known defect, thorough positive/negative fixtures in the suite (lines 3564-3612 plus the knob fixtures, including space and unicode baseline paths), and the decisions lean heavily on git primitives (status --porcelain, merge-base, rev-list) rather than on shell-implemented rules. The gap is real doctrine inconsistency, not active harm.

**Current:** All decisions live in bash/awk: parsing the baseline record (sed over sha=/recorded_epoch= at lines 100-102), dirt-beyond-baseline classification (awk over NUL-converted porcelain, lines 72-79), ancestry (line 108), and the cadence backstop arithmetic (lines 109-117). Its named siblings from kill-shell Phase A — receipt.sh, frontier.sh, assert-stop-loss.sh, assert-design-obligation-gate.sh, audit-metasystem.sh — are all 10-line exec shims into engine verbs; kill-shell-facts.md F Q1.12 lists refactor-baseline.sh in the same policy-facing class.

**Target:** Port the decision to the validate family — e.g. `metasystem validate refactor-baseline record|check` — preserving the plans/refactor-baseline on-disk format (sha=/recorded_epoch=/gate= lines) and exit codes 0/1/2, with the script reduced to the Phase A shim shape. New verb naming needs owner sign-off.

**Why:** This is a completion-gate decision engine (the refactor skill's blocking check per skills/refactor) that is untestable by unit test and invisible to the coverage ratchet, in a repo whose ruling is that core decisions live in Go; it is the clearest remaining Phase A-shaped gap in this cluster.

#### script-misc-4 — Status-less records are classified NEVER-STARTED, contradicting the documented mtime-only contract for non-JSON records

`scripts/watch-background-jobs.sh:347` · design · **medium / effort S**

**Current:** The header (lines 26-28) promises that when a record does not parse as JSON, 'the file's mtime alone drives STALE/CAPPED and terminal status is unknown'. But the NEVER-STARTED branch (lines 347-353) fires on `[ -z "$status" ]` — which is true for every non-JSON record — after only --start-verify-min (default 5) quiet minutes, well before STALE's 20. Because a report marks the id seen forever (line 353), the job's real later STALE/CAPPED/DONE can never be reported.

**Target:** Restrict the empty-status arm of the NEVER-STARTED test to records that parsed as JSON but lack a status field (distinguishable via a `$ms json get`-style parse check), or amend the header and add a fixture pinning the intended behavior; today the suite has no fixture for a standalone non-JSON record older than start-verify-min.

**Why:** A runner that keeps plain-text records — explicitly supported by the header — gets a falsely named NEVER-STARTED at 5 minutes and then permanent silence on that job, which is precisely the silent supervision miss this watcher exists to prevent.

#### script-misc-6 — mission-state.schema.json is an unconsumed shadow of the Go contract and has already drifted

`scripts/agents/schemas/mission-state.schema.json:6` · duplication · **medium / effort S**

**Current:** No code validates anything against this schema — internal/mission/state.go's ValidateState is the real contract and the suite only existence-checks the file (validate-metasystem.sh:244). Drift is already present: Go allows the optional key `lastDrainStall` (state.go:183 `allowed := map[string]bool{"ledgerSemantics": true, "lastDrainStall": true}`), while the schema's `additionalProperties: false` (line 6) lists no such property — a Go-valid mission state fails the shipped schema.

**Target:** Either add a Go test that validates representative ValidateState-accepted/rejected fixtures against this schema so the two can never drift silently, or retire the file in favor of a pointer to the Go owner (retirement changes the shipped file set and the suite's asset list, so it needs sign-off). The parity test is the no-contract-change fix.

**Why:** One rule, one home is this repo's own doctrine; a published schema that disagrees with the enforcing implementation misleads adopters and future maintainers with the authority of a spec.

#### script-misc-10 — Derived state-file key concatenates cksum fields without a separator

`scripts/watch-background-jobs.sh:179` · idiom · **low / effort S**

**Current:** `key=$(printf '%s\n' "${dirs[@]}" "scope=$scope" | cksum | tr -d ' \t')` glues the 32-bit checksum and the byte length into one digit string, so distinct --dir/--scope sets can collide on the same state path (sum=123/len=4567 and sum=1234/len=567 both yield 1234567).

**Target:** Keep a separator (e.g. `cksum | tr ' ' '.'` or translate to '-'), so the (sum, length) pair stays injective in the filename.

**Why:** The header itself declares shared state 'the exact silent miss this watcher exists to prevent' — two scopes silently suppressing each other's reports is the watcher's worst failure mode, and the fix is one character.

#### script-misc-5 — Adoption's target-state recognition and refusal decisions live in shell

`scripts/adopt.sh:117` · boundary · **low / effort L / needs sign-off**

> downgraded by the verifier: Facts verified (target recognition 117-135, allowlist loop 162-170, collision loop 211-221, Phase E superseded-not-resolved). Downgraded on the restructuring-must-carry-its-weight standard: the refusal quadrant is thoroughly exercised by the adopt selftests (foreign asset, collision-with-name, unknown/duplicate/none-mixed runtimes, partial-installation reruns, dirty template — all in validate-metasystem.sh's adoption section), every scar-tissue incident in the script's comments is already fixed and pinned, adoption runs once per target from the template checkout, and the just-completed human-approved consolidation program explicitly affirmed adopt.sh's allowlist as 'the export contract and stays the single one' (r1/GSC-R1-006). No demonstrable current harm; the right home for this proposal is the still-open r10 adopted-engine ruling, where an 'adopt plan' verb can be weighed with engine delivery.

**Current:** The fresh/current-SHA/incomplete/other-SHA/foreign quadrant is decided by grep and string matching over docs/project-rules.md's adoption marker (lines 116-135), the ship-set is the shell allowlist loop (lines 162-176), and collision policy is a shell find/cmp loop (lines 211-221). The genuinely delegated parts (conf tailoring via `config tailor` line 206, engine build via go-build.sh) show the intended shape. plans/kill-shell.md Phase E had already concluded 'adoption's decisions become `metasystem adopt run`, adopt.sh stays a thin BOOTSTRAP shim'; that phase was superseded by the consolidation redesign, not resolved.

**Target:** Under the consolidation redesign, give adoption a Go verb that owns classify-target, allowlist, and collision verdicts (e.g. `metasystem adopt plan --target ... --sha ...` returning the decision and refusal reason), leaving adopt.sh the bootstrap plumbing: build engine, stage archive, copy, link, install hooks. Needs owner sign-off (new family/verb).

**Why:** These refusals guard against destroying a target repo's existing instruction assets — the highest-consequence decision this script makes — and today they are string-matching heuristics with no unit tests, only end-to-end fixtures.

#### script-misc-7 — Skill frontmatter contract is parsed and judged in sed/awk instead of the validate family

`scripts/validate-skill.sh:15` · boundary · **low / effort M / needs sign-off**

> downgraded by the verifier: Facts verified: frontmatter extraction at lines 9-16, rules at 18-22, the validate family's neighboring verbs at cmd/metasystem/main.go:60-69, and internal/audit listing scripts/validate-skill.sh as a required file only. Downgraded because the port does not carry its weight: 26 lines of stable, trivial rules with direct suite coverage over every skill, no recorded incident, and the owner's own fresh precedent from the consolidation loop (r4/GSC-R1-024: a few lines of shell carrying a documented rule beat an eleventh validate verb) cuts against exactly this class of extraction. The YAML-parsing fragility is real but fails loud (empty name/description rejects), not silent. The stated M effort exceeds the payoff.

**Current:** Frontmatter extraction (sed/awk lines 9-16) and the naming/structure rules (lowercase-hyphen pattern, 64-char cap, folder-name match, lines 18-22) are decided in shell. The engine already owns the neighboring validation surface (validate turn-prompt, preamble-quotes, return-complete, design-obligations, stop-loss per cmd/metasystem/main.go:60-69), and internal/audit only checks that this script exists (internal/audit/metasystem.go:35).

**Target:** A `metasystem validate skill --dir <skill>` verb owning the frontmatter parse and rules, with this script reduced to the Phase A shim shape; behavior and exit codes preserved. New verb needs owner sign-off.

**Why:** It is a small but real decision engine (a suite gate over a shipped contract) whose YAML-ish parsing is exactly the fragile-in-shell, trivial-in-Go work the core-vs-plumbing ruling assigns to the engine.

#### script-misc-8 — settings.json produced by sed-deleting a line from JSON

`scripts/adopt.sh:284` · idiom · **low / effort S**

**Current:** `sed '/"_comment"/d' "$target/scripts/enforcement/claude-code-hooks.json" >"$target/.claude/settings.json"` — the adopted runtime config is manufactured by text-deleting any line containing "_comment". It works only while the comment stays a single self-contained line whose removal leaves valid JSON, and while no hook command ever contains that substring.

**Target:** Strip the key structurally via the engine (a small `json` family helper beside the existing `json get`), or ship the comment-free settings file as its own enforcement asset and keep the annotated one for humans.

**Why:** Producing domain JSON by line-oriented text munging is the canonical shell leak; when it breaks it breaks the adopted runtime's hook config silently, at the adopter's site rather than in this repo's fixtures.

#### script-misc-9 — emit-event.sh is sourced but emit_event is never called

`scripts/watch-background-jobs.sh:194` · dead-code · **low / effort S**

**Current:** Lines 194-198 source scripts/agents/emit-event.sh (with a no-op fallback definition), but no line in this script invokes emit_event; the only event emission happens inside the Go `supervise watcher-pass` call.

**Target:** Delete the source block and the fallback, or actually emit the watcher's reportable states into the flight recorder if that was the intent.

**Why:** A reader (or incident responder) skimming the watcher reasonably concludes its DONE/STALE/CAPPED reports reach the flight recorder; they do not, and the dead wiring is what creates that false impression.

## Part 3 — Architecture, naming, and documentation

### The system as a whole

*Verdict: passes the senior-engineer bar. 10 findings (3 high, 4 medium, 3 low).*

**Reviewer's summary.** Whole-system architecture review of the metasystem Go engine (29 internal packages, ~32k source lines under internal/, 5.9k under cmd/metasystem, 19 CLI families with 167 verbs) plus its shell plumbing (~15k lines, dominated by fixtures) and documentation tree.

Overall state: strong. The dependency graph is acyclic with a coherent three-tier direction (infrastructure leaves: atomicfile, identity, config, wiredoc, events, lock, turn, boundedexec, returnschema; domain middle: census, lease, registry, supervise, dispatch, contract, mission, evidence, receipt; engine top: missionrunner). Package doc comments are exceptional by industry standards; nearly every package states what it owns, why it exists as one owner, and which incident or clause it implements. Granularity verdict: 24 of 29 packages clearly earn their existence. Five micro-packages (authority 56 lines, hooks 54, returnschema 90, gaterun 182, capability 377) are marginal, but each is coherent and documented, and the repo's own consolidation program correctly withdrew merges whose only product was renaming; the same logic says leave them. No package needs splitting. The one lateral import (dispatch -> supervise for the budget-expiry verdict) is justified inline as single-ownership of the verdict. The real structural leaks are: (1) typed usage extraction has no owner (DevinUsage duplicated verbatim between internal/host and internal/adapter; mission/fence.go reaches into adapter for Codex parsing), and (2) genuine decisions live in package main, which is the one package exempt from the coverage ratchet.

Naming: the vocabulary (lease, census, reaper, fence, wiredoc, janitor) is learnable and the glossary teaches it. Two flaws: internal/contract is the only package with no doc comment and carries the repo's most overloaded word for what is specifically the mission-contract engine; internal/report's doc claims turn-end decisions while it also houses the improve-mode frontier ledger. The family-to-package drift (family "job" -> package dispatch, "proc" -> identity+census) is an accepted consequence of the adjudicated surface consolidation and is tolerable, but only if a package map documents it, which today none does.

Go/shell boundary: the kill-shell ruling (core in Go, plumbing in scripts) is settled and the surface consolidation program completed 2026-08-12. The structural work remaining is not more porting; it is (a) deduplicating the implementations behind the adapter/host verb pairs (the family-level merge was adjudicated and withdrawn; the implementation-level copy was not), (b) evacuating decisions from cmd into floored internal packages, and (c) giving dispatch.sh, the permanent shell owner of the riskiest call sequence, a ground-truth sequence document like the mission cycle already has.

Documentation: the conceptual docs (concepts.md, glossary.md, wire-documents.md, mission-cycle-sequence.md) are professional and unusually honest. But the tree carries visible drift from the two fast 2026-08 migrations: the glossary names five deleted Python scripts as owning implementations, wow.md routes maintenance to a nonexistent meta/ directory, the README layout omits cmd/, internal/, and bin/ entirely, and there is no architecture overview or package map anywhere (the only doc titled architecture, development/metasystem-architecture.md at the repo toplevel, predates the Go port). Several packages cite plans/ documents as their standing contract authority, which contradicts wow.md's own rule that plans/ is task-local evidence, never policy; wire-documents.md shows the correct promotion pattern.

Evolvability: adding a runtime adapter or a mission fence today is well-supported. The places an incident would hurt: reconstructing the job-record lifecycle from 1,602 lines of dispatch.sh with no sequence map, and onboarding anyone cold without a package map or an accurate glossary.

#### architecture-1 — Typed usage extraction has no single owner; DevinUsage is copied verbatim across packages

`internal/host/devin_usage.go:23` · duplication · **medium / effort M**

*Canonical statement of the defect also reported as adapter-host-registry-1 and cli-3.*

**Current:** internal/host.DevinUsage (devin_usage.go:23) and internal/adapter.DevinUsage (devin.go:206) have the identical signature and near-identical bodies implementing the same correctness-sensitive rule: per-turn deltas against Devin's cumulative session counters, with publish-unavailable-on-missing-predecessor to avoid double counting. Meanwhile internal/mission/fence.go:748-761 imports adapter to call RootJobID and CodexUsageValue for fence usage aggregation, pulling runtime event-stream parsing into the mission domain's dependency set.

**Target:** One owner for typed usage extraction and aggregation: a small leaf package (e.g. internal/usage) consumed by adapter, host, and mission — the host-imports-adapter alternative is rejected because mission/fence.go must also come off its adapter import, which that shape leaves in place. The r1/GSC-R1-003 ruling that keeps adapter and host as separate CLI families adjudicated the verb surface, not the implementation; both verbs front one function. A Devin metric change or a delta bugfix must land once. Severity is medium per adjudication (R1.11): both copies are floored, tested, and identical today — preventive consolidation, not a live defect.

**Why:** This is exactly the failure the tree's own atomicfile doc names: two copies become two fixes that silently diverge. The duplicated rule guards against double-counting spend, and runaway spend is one of the nine failure classes the README says the system exists to prevent. The copies have already drifted cosmetically (loadObject vs readObject, comment wording), which is how semantic drift starts.

#### architecture-2 — Real decisions live in package main, the one package exempt from the coverage ratchet

`cmd/metasystem/supervise_arming.go:31` · boundary · **high / effort M**

*System-level umbrella for cli-1 and cli-2; the backlog tracks them as one workstream (W3).*

**Current:** runSuperviseBlockingReservedCap (supervise_arming.go:31-90+) scans job records and mission fence reservations and decides whether a proposed watcher ceiling may be armed; its only caller is scripts/agents/arm-supervision.sh:76 and no test references it. cmd also owns json set/get/object (json.go) and the slug sanitize rule that must match across arming and hook paths (slug.go:17-35). scripts/agents/coverage-ratchet.json:3 exempts cmd/metasystem as 'thin verb wiring exercised end to end by the shell fixtures', and main.go:14 claims 'main only routes'. cmd is 5.9k lines with four test files of its own.

**Target:** Verb functions in cmd parse flags, call an internal package, and print. The blocking-reserved-cap decision belongs in internal/supervise (floor 85.4), the slug rule wherever the arming/hook contract lives, and the json edit helpers in or beside atomicfile/wiredoc. Re-seed the ratchet in the same commit per the recorded procedure.

**Why:** An arming-safety decision ('a ceiling that does not strictly clear every live reservation must not be armed') sits in the only package with no coverage floor and no unit test, in a system whose doctrine is that decisions are unit-tested Go. The exemption's stated reason is false for these files, so the ratchet is being quietly bypassed by placement.

#### architecture-4 — The glossary names five deleted Python scripts as the owning implementations

`docs/glossary.md:27` · docs · **high / effort S**

**Current:** docs/glossary.md tells readers 'the named scripts are where to look when a definition and reality seem to disagree' (lines 4-5), then names scripts/agents/worktree-lease.py (line 27), process-census.py (line 77), emit-event.py (line 119), mission-fence.py (line 156), and return-schema.py (line 192). None exist; the kill-python port moved all five into the Go binary. AGENTS.md:14 makes this glossary the terms-of-art authority loaded on effectively every task.

**Target:** Each glossary owner points at the current implementation: the metasystem verb (e.g. 'metasystem lease', internal/lease) or the surviving script. One editing pass, checked by the existing outside-reference audit if it can be taught to verify referenced paths exist.

**Why:** The always-loaded vocabulary document of a system whose pitch is evidence over assertion currently directs both agents and humans to five dead paths, during exactly the disagreement-resolution moments it was written for. Every agent session inherits this misdirection.

#### architecture-3 — No architecture overview or package map exists for the Go engine

`README.md:173` · docs · **medium / effort M**

> downgraded by the verifier: Facts confirmed: README layout (173-192) omits cmd/, internal/, bin/, go.mod and the README never mentions the Go binary at all (grep verified); ../development/metasystem-architecture.md predates the port (describes meta/ and scripts/audit-harness.sh, zero internal/ references); docs/ mention internal/ only in three mechanism-specific files. The gap is real and the target is cheap assembly of existing package docs. Downgraded to medium: this is onboarding friction, not active misdirection on a safety or correctness path — the 29 in-package doc comments substantially mitigate, and the stale architecture doc sits in low-traffic template-maintenance land behind a route that is itself dead (finding 8). It does not rank with the spend-rule duplication or the untested arming decision.

**Current:** The README's layout block (lines 173-192) omits cmd/, internal/, bin/, and go.mod entirely; the README never mentions that a Go binary exists. The only document titled architecture (development/metasystem-architecture.md, repo toplevel) predates the port: it still describes meta/ and scripts/audit-harness.sh and contains zero references to internal/. The layering direction, the family-to-package mapping (job -> dispatch, proc -> identity+census, gate -> gaterun), and the core-vs-plumbing boundary ruling live only in main.go's usage text and in superseded plans (plans/kill-shell.md:5).

**Target:** One architecture document in docs/ (or a rewritten development/metasystem-architecture.md): what the binary is, the package map with one line per package, the three-tier layering direction, the family-to-package table, and a pointer to the core-vs-plumbing ruling as standing doctrine. The 29 package docs are already written; the map is mostly assembly.

**Why:** A senior engineer onboarding cold onto a 32k-line, 29-package engine currently has to reverse-engineer the system's shape from go list and main.go. The system's own rule is one rule, one home; the architecture has no home. This is also where the family/package vocabulary drift stops being acceptable and starts being a trap.

#### architecture-5 — Standing contracts are cited as authority from plans/, which the routing index defines as non-policy evidence

`internal/registry/framing.go:1` · docs · **medium / effort M**

**Current:** internal/registry/framing.go:1 declares plans/supervision-registry.md 'the authority' for the machine-wide custody contract; internal/supervise/decide.go:1 cites plans/supervision-lifecycle.md, internal/events/emit.go:1 cites plans/flight-recorder.md, internal/audit/coverage.go:1 cites plans/kill-shell.md. wow.md:29 rules that plans/ content 'must not become global policy unless deliberately promoted'. The tree already shows the correct end state: docs/design/wire-documents.md:3-5 is the promoted durable contract with its plan deleted at close-out, and mission-cycle-sequence.md lives in docs/design too.

**Target:** Promote the standing contracts (supervision registry, supervision lifecycle, flight recorder, stop-loss core) into docs/design/ following the wire-documents pattern, leave plans/ entries as historical designs, and update the package docs to cite the promoted homes.

**Why:** The system's own doctrine says durable rules get one canonical owner outside task-local evidence. Today a production incident in supervision sends the responder into a plans/ directory that adoption does not ship and that the routing index says is not policy, and the authority documents can be pruned or go stale with no gate noticing.

#### architecture-7 — dispatch.sh permanently owns the riskiest call sequence with no ground-truth sequence document

`scripts/agents/dispatch.sh:31` · docs · **medium / effort M**

**Current:** Under the settled core-vs-plumbing ruling, dispatch.sh (1,602 lines, 131 branch points, orchestrating the 26 job-family verbs) remains the owner of the delegate-job lifecycle choreography: record CAS ordering, handshake windows, custody, wind-down. plans/kill-shell.md:247 called this 'the riskiest seam'. The mission path has docs/design/mission-cycle-sequence.md precisely because a reference design died of being written against an assumed sequence (docs/concepts.md:175-177). No equivalent exists for the dispatch path.

**Target:** A dispatch-sequence ground-truth map in docs/design/, same genre as mission-cycle-sequence.md: the ordered steps, which verb decides at each step, which shell lines merely plumb, and the failure/reap joins. It documents what the ruling decided will stay shell.

**Why:** This is where debugging a production incident hurts most today: a job-lifecycle incident (double-reap, handshake race, stuck wind-down) is reconstructed by reading 1,602 lines of bash, and any future design touching dispatch repeats the assumed-sequence failure mode the repo has already paid for once.

#### architecture-8 — The single routing index routes maintenance to a directory that does not exist

`wow.md:27` · docs · **medium / effort S**

**Current:** wow.md:27 routes 'Metasystem maintenance and rationale' to meta/ (template repository only); no meta/ directory exists in this repo or the toplevel. The content lives at the repository toplevel in development/. README.md:191 compounds it by drawing development/ inside the metasystem layout tree, though it is a sibling one level up.

**Target:** wow.md routes to the real location (../development/ from this directory, with the template-only caveat kept), and the README layout block either shows development/ at its true level or drops it from the tree with a sentence pointing up.

**Why:** wow.md declares itself 'the single routing layer'; a routing index with a dead route undermines the exact mechanism (progressive disclosure with one trusted index) the system is built on, and every agent that follows the route burns a lookup and starts distrusting the index.

#### architecture-10 — report's package doc omits the frontier ledger it houses

`internal/report/stopblock.go:1` · docs · **low / effort S**

*Same fix as validate-report-14.*

**Current:** The doc says the package holds 'the turn-end report decisions: the stop-hook block ... and the open-work check', but internal/report/frontier.go is the improve-mode best-known-state ledger (surfaced as 'metasystem report frontier'), which is not a turn-end decision. The package is a small grab-bag of three loosely related features shaped by the CLI family rather than by a domain.

**Target:** Either the doc names all three residents honestly, or frontier moves to its own small package (it is the improve skill's ledger, closer kin to receipt than to the stop hook). The doc fix alone is acceptable.

**Why:** A misdescribed package boundary invites the next unrelated report-ish feature to land here too; naming what the grab-bag actually holds is the cheap way to stop it growing.

#### architecture-6 — internal/contract is the only package without a doc comment, and its name collides with the repo's most overloaded word

`internal/contract/contract.go:29` · naming · **low / effort M**

*Overlaps mission-contract-11 (the doc-comment half); the rename question is this finding's own.*

> downgraded by the verifier: Facts confirmed: a scan of internal/*/ shows contract is the only package without a '// Package' doc comment, and its de-facto description floats at contract.go:29-38 below the import block where godoc drops it; the name genuinely collides with the repo's other 'contract' senses (AGENTS.md contract, wire contracts). Downgraded to low: the load-bearing defect is an S-effort doc-comment move; the rename is an unforced design opinion the finding itself half-retracts ('if the rename churn is not worth it'). By the reviewer's own calibration in finding 9 — an affirmatively wrong doc costs more than a missing one, rated low — a missing doc plus a naming quibble cannot sit at medium.

**Current:** The package parses, seals, preflights, and measures MISSION contracts, but is named bare 'contract' in a repository where the word also means the repository agent contract (AGENTS.md), the work contract, and on-disk wire contracts. Its de-facto package description is a floating comment below the import block (contract.go:29-38), which godoc will not attach to the package. All 28 other packages carry deliberate, high-quality doc comments.

**Target:** Minimum: move the existing comment above the package clause as a proper doc comment that names the mission scope in its first sentence. Better: rename to a domain-precise name (missioncontract, or fold the mission scoping into the doc if the rename churn is not worth it). Internal rename only; no CLI verb or on-disk artifact changes.

**Why:** The dependency graph shows contract imported by mission, missionrunner, and cmd; a newcomer reading the graph will guess it is a general contract mechanism and be wrong. In a tree whose package docs are otherwise the primary onboarding surface, the one silent package is also the one with the most ambiguous name.

#### architecture-9 — config package doc attributes the precedence resolver to a shell function that has since moved into this package

`internal/config/conf.go:2` · docs · **low / effort S**

*Same fix as foundations-13.*

**Current:** The doc comment says local precedence 'belongs to dispatch's config_get', but the full flag/env/local/mode/conf/default resolver now lives in this same package (internal/config/resolve.go:49, surfaced as 'metasystem config get' via cmd/metasystem/config_verbs.go:15).

**Target:** One sentence: the package holds both the conf-only reader and the full precedence resolver, with the conf-only entry point documented as the subset.

**Why:** The doc actively sends a reader away from the file that sits beside it; in a tree where package docs are the primary map, a affirmatively wrong one costs more than a missing one.

## Appendix A — Findings rejected by adversarial verification

Four findings were killed in verification. They are recorded here so the same ground is not re-litigated later.

- **missionrunner-10 — Two-thirds of the exported surface has no consumer outside the package** (proposed low). The usage survey is accurate — cmd/metasystem references exactly ReadDoc, ParkProposal, RecordFailureProposal, TurnFromDoc, AdjudicateFiles, ConcludeFiles, NewEngine plus Engine methods, no other package imports missionrunner, and lease-succession-fixtures.sh:118-119 does pin MissionLineage's source spelling. Rejected on materiality, per the bar for design opinions: the current shape does no demonstrable harm (tests are in-package, cmd is the sole consumer, internal/ already voids any stability promise beyond this module), and the finding itself concedes it is 'residual polish'. The reducible set is also smaller than 'two-thirds': Verdict (returned by AdjudicateFiles), ParkOutcome (returned by ParkProposal), and Turn (returned by TurnFromDoc, parameter of RecordFailureProposal) are load-bearing types of the kept entry points that the finding's own target says to keep, while ConcludeTurn/ConcludeFaultedTurn/Adjudicate/ProjectFences are the advertised decision surface the package doc narrates — exports functioning as godoc for the layer the CLI wraps via the *Files variants. What remains freely unexportable (ScaledSeconds, Interval, CloseableChains, PriorContext, PreviousMetrics, SessionFault, RunnerError) is churn without a bug class, plus a live trap in the MissionLineage fixture pin. If the team splits loop.go (missionrunner-6), tightening exports can ride along; as a standalone finding it does not carry its weight.

- **script-validate-6 — Git index.lock stale-detection and deletion policy invented in the fixture** (proposed medium). The code as cited exists (runner_git 1516-1533 with the mtime-based deletion), but the finding's central claim — 'the runner evidently can still exit or die with git operations in flight, so the fixture papered over a real ordering gap' — misreads the engine. In internal/missionrunner/loop.go every anchor runs synchronously inside the cycle (anchor → anchorState → runCaptured, loop.go:301-312) and releaseLease strictly follows the last cycle (loop.go:118; fail ramp 73-81), so a runner cannot exit with a git op in flight and lease.d removal already IS the true quiesce signal. The suite knows this: wait_lease_released (2961-2971) documents the ordering and is used at 3087/3122/3259/3386 before exactly the git writes that race trailing anchors. Target prong 1 is therefore already implemented — the proposed engine work is a no-op. The mtime branch guards only a KILLED runner's orphaned lock, which green runs never produce (the suite TERM/KILLs fixture driver processes at 1620-1638, not detached runners), and prong 2 — the engine's reap deleting .git/index.lock — is risky new policy for a fixture-manufactured case. What remains is minor fixture hygiene (lean on wait_lease_released, narrow runner_git), not the claimed boundary defect.

- **script-adapters-03 — Session-usage store publishes only on completion, double-counting a failed resumed turn's spend** (proposed high). The mechanism misreads missionrunner. Both failed paths in hosts/devin.sh (lines 144-154) exit 3; cycleRunHost (loop.go:1110-1123) routes nonzero exits to recordFailedTurn -> RecordFailureProposal, which writes sessionId:null into the turn log (cycle.go:230). PriorContext (contract.go:115) reads only the turn log, so hostSession is nil and the next turn launches WITHOUT --resume-session (host.go:227) — a fresh session whose usage is computed with expect_previous=0. The failed-then-resumed scenario the finding needs cannot occur. The turns that ARE resumed after a fault (rejected return, capped — ConcludeFaultedTurn keeps sessionId) either already ran the host's completed path and published the store (hosts/devin.sh:165-169) or were killed before devin-usage wrote any delta. No double count exists.

- **script-adapters-14 — Devin runtime facts declared as shell globals instead of capability-snapshot fields** (proposed low). Factually accurate (devin.sh:27,32 vs the snapshot's sessionEstablishedTimeoutSec at line 78), but the target is not clearly better. Both globals are selftest-local tuning consumed only by this file's own selftest path; they overlap no snapshot field, so the predicted disagreement between the two homes cannot arise. sessionEstablishedTimeoutSec earns its snapshot slot by being consumed cross-component (dispatch's handshake deadline); these two have no consumer outside the adapter. Widening the Go-validated snapshot contract (correctly flagged as contract-touching, so it also costs owner sign-off) to carry selftest knobs trades a locality virtue for hypothetical auditability — a plausible-sounding restructuring that does not carry its weight.
