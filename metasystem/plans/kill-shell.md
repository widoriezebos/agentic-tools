# Kill-shell: everything of consequence moves to Go

Working Mode: design

Owner: main session (claude). Status: IN CRITIQUE — round 1 folded
(plans/dispositions/kill-shell-r1.md, 9/9 accepted). Facts:
plans/kill-shell-facts.md (cited as F Qn.m); each phase re-runs its
fact section before design (the moving-target rule, and r1/KS-R1-002
proved why: the sheet was already stale on lock ownership). Human ruling
(Wido, 2026-08-11): this must happen, thoroughly and well. The repo
should carry the very minimal amount of shell code — or even shell
scripts. Everything of complexity lives in the Go application where it
is unit-tested. Shell scripts are shims, if they exist at all. Plan
first, critique the plan, then implement.

## Why

The Go port moved every *decision* into the engine, but whole layers of
logic never moved: process-lifecycle choreography in dispatch.sh,
adapter protocol handling, production gates that still run python3, and
~170 python heredocs asserting fixture state. Shell logic is untestable
by unit test, invisible to the race detector and coverage floors, and
has produced this project's worst incidents (the four concurrent suite
runs; the cleanup grep that killed real supervision). The gate fence
(f3a8b78) is the target shape: the whole decision in one Go verb with
unit tests and live fixtures, consulted by a three-line shim.

## Inventory at kickoff (16,461 lines tracked shell)

Production logic, ~7.5k lines:

| file | lines | residue of consequence |
| --- | --- | --- |
| scripts/agents/dispatch.sh | 1574 | chain/lifecycle/cap-authority locks, pgid liveness, wind-down, record CAS choreography, cap resolution, census freshness |
| scripts/agents/adapters/runtime-common.sh | 587 | adapter protocol shared core |
| scripts/agents/adapters/{devin,fake,codex,claude}.sh | 1225 | probes, capability snapshots, session/event plumbing |
| scripts/agents/assert-conformance.sh | 630 | production gate; 5 live python3 calls |
| scripts/agents/arm-supervision.sh | 500 | arming order, census, owner/component launch |
| scripts/watch-background-jobs.sh | 402 | watchdog classification |
| scripts/adopt.sh | 333 | adoption transform |
| scripts/receipt.sh | 253 | receipt assembly |
| scripts/agents/supervision-hook.sh | 232 | harness hook: still-working report, watchdog nag suppression |
| scripts/assert-design-obligation-gate.sh | 232 | obligation-table gate |
| scripts/frontier.sh | 205 | frontier report |
| scripts/agents/hosts/{devin,codex,claude,fake}.sh | 467 | host turn assembly residue |
| scripts/audit-metasystem.sh | 110 | word-budget audit |
| ~20 smaller scripts | ~800 | mixed shims and residue; each needs a verdict |

Fixture harness, ~8.9k lines: validate-metasystem.sh (4329) plus 18
fixture scripts. Their logic of consequence is real: ~170 python3
heredocs asserting JSON state, plus orchestration (budgets, caps,
watchers, cleanup traps).

## Target end state

1. Every decision, transformation, gate, and report lives in a Go verb
   with unit tests under the coverage floor.
2. A shell file may contain only: argument relay, environment guards,
   a consult of one or more Go verbs, and the final `exec` of an
   external CLI. Guard-clause `if`s on a consult's exit code are fine;
   business branching is not.
3. No python3 anywhere in the repo — production or fixture.
4. Scripts that exist only because internal callers name their path are
   deleted; callers call the binary. Scripts named by external
   contracts (skills, docs, hooks, adopted targets) stay as shims —
   on-disk contracts are preserved (the port rule).
5. A mechanical fence keeps it this way: the suite gains a shell
   complexity budget (per-file line cap, no-python check, function
   count bound) that refuses regressions, the same way the word-budget
   audit fences prompt growth.

## Dead code dies first (human ruling, same day)

Hunt down dead code and kill it — across shell AND Go. This runs as
Phase 0 and as a standing rule inside every later phase:

- Phase 0 sweep: build the caller graph for every script (who invokes
  it: suite, hooks, skills, docs, adopted contracts, nobody) and every
  function within the big scripts; run the Go dead-code analyzer
  (golang.org/x/tools/cmd/deadcode) over cmd + internal. The sweep's
  product is a per-script DISPOSITION REGISTRY — port+shim,
  port+delete, keep — because adopted targets receive scripts/
  wholesale (F Q1.16-Q1.19, r1/KS-R1-001): every deletion pairs with
  an adopt.sh payload change in the same commit, and the registry is
  what the fence checks, not raw reachability.
- Standing rule: porting a file starts by proving which of its parts
  are alive. Dead logic is deleted, never ported — porting it would
  launder it into tested-looking Go.
- The complexity fence (below) also counts scripts: a script nothing
  references fails the budget.

## Disposition by phase

Phase A — production gates and reports (mechanical, kills production
python), with the family table decided (r1/KS-R1-004):
assert-conformance.sh and assert-design-obligation-gate.sh → the
validate family (whole-artifact validators, F Q5.8); frontier.sh →
report; receipt.sh → a receipt family (it owns durable state and
several commands); audit-metasystem.sh → an audit family whose first
verb it becomes. Plus a residue audit of every existing assert-*.sh
shim. The complexity fence lands here as `audit shell-budget`
(r1/KS-R1-008): a Go verb over a checked-in budget file that only
ratchets down — total tracked shell lines, per-file caps, per-file
control-flow construct counts, refusal of here-doc-generated shell,
and the disposition-registry check — numbers set from measured
values at landing.

Phase B — dispatch.sh lifecycle layer (the riskiest seam, its own
design round inside the loop). Scope corrected by r1/KS-R1-002: the
lock PRIMITIVE already lives in Go (internal/dispatch/ownerlock.go);
what ports is the remaining choreography — when each lock is taken,
held, and released; liveness sequencing; wind-down ramps; CAS
choreography; cap resolution. The phase STARTS with characterization
fixtures pinning the branches no test reaches today (r1/KS-R1-003):
rename-born lock publication, the six holder classifications,
non-owner release, lifecycle-lock timeout scaling, cap-authority lock
disappearance, and the wind-down refusal ramps — behavior-preserving
is only meaningful against pinned behavior. Prerequisite pulled
forward (r1/KS-R1-009): the coverage ratchet of
plans/go-production-grade.md Phase 0c (mechanical, measured-value
ratchet) lands before Phase B begins, so 'unit-tested under the
floor' is enforced, not asserted. End state: dispatch.sh parses flags
and consults.

Phase C — adapters and hosts: runtime-common.sh and the four adapters
move into internal/adapter drivers under an added constraint
(r1/KS-R1-005): record authority and lifecycle serialization stay
with dispatch — drivers get a narrow Go interface whose record
mutations route through the same authority matrix the shell router
enforces today, and this phase's design gate must show the
classification equivalence. The host boundary is launch-wait-
parse-write, NOT final exec (r1/KS-R1-006): hosts must regain control
after the runtime exits, so the shim keeps process custody while the
post-exit DECISIONS — outcome classification, session and usage
parsing, atomic result writing — become Go verbs the shim calls. The
fake adapter becomes a Go test double behind the same driver
interface.

Phase D — supervision arming and watchdog: arm-supervision.sh into the
supervise family; watch-background-jobs.sh classification into census;
supervision-hook.sh stays a hook entry point but every sentence it
prints comes from a report verb.

Phase E — adopt.sh becomes `metasystem adopt run`; go-gate.sh stays a
script by necessity (it builds the binary) and is already near-minimal.

Phase F — fixtures, DECIDED by evidence (r1/KS-R1-007): bash stays
the end-to-end driver — arrange via verbs, act by calling the CLI
exactly as a user would, assert via Go assert verbs; every python
heredoc dies. The Go-integration-test alternative is recorded as
INVALID for the adopted contract, not merely unchosen: adopted
targets run validate-metasystem.sh without a Go module by design
(F Q6.1-Q6.8, go-gate's no-go.mod skip), so fixtures that only exist
as Go tests would vanish from the very repos the suite protects. The
suite keeps driving the real CLI — the heading-order bug on
2026-08-11 was caught only because fixtures drive the shipped
surface, and that property is not negotiable.

## Ordering

0 → A → B → C → D → E → F. The dead-code sweep first so no later
phase spends a design round on code that should not exist. A next
because it is mechanical, deletes the
last production python, and builds the verb-family muscle the later
phases reuse. B before C because dispatch owns the semantics the
adapters plug into. F last and incrementally: each earlier phase
already converts the fixtures that drive what it ports.

## Verification

Per phase: unit tests for every ported decision (race detector,
coverage floor), the full suite green via the standing launch recipe
(gate fence → supervise launch-detached → identity), and a line-count
delta recorded in the phase's commit message. The complexity fence
lands with Phase A and ratchets downward as phases complete — the
budget only ever shrinks.

## Non-goals

- No behavior changes while porting: same refusal messages, exit
  codes, and artifact shapes unless a defect is found (then it is
  fixed and named, per the port rule).
- No new shell features in the meantime: anything new lands as a Go
  verb from day one (the gate fence precedent).
- go-gate.sh bootstrap and hook entry points are not forced to zero
  lines; they are forced to zero decisions.

## Loop plan

Facts pass first (standing rule, skills/design-critique/SKILL.md):
a Codex fact sheet anchoring every mechanism claim above — dispatch.sh
section map with line ranges, adapter call graph, every python3 call
site, every script's callers (internal vs external contract), existing
Go family surfaces. Then the critique loop on this plan: sol (codex
gpt-5.6-sol) at xhigh via dispatch.sh --role design-critic, mechanical
closure joins, diminishing-returns stop rule. Phase B gets a second,
focused round on the lock/liveness seam before its implementation
starts.
