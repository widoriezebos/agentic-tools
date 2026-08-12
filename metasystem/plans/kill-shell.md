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
2. A shell file may contain only: argument relay — flag-to-argv
   mapping alone, with defaults and usage text coming from verbs;
   any flag whose value selects POLICY is a decision and moves
   (r5/KS-R5-003, r6/KS-R6-006) — environment guards,
   a consult of one or more Go verbs, and one of three legal shapes
   (r3/KS-R3-004, r4/KS-R4-002): the final `exec` of an external CLI
   (the default); launch-wait-consult custody where a protocol
   requires regaining control after a child exits (the hosts); or
   the SEQUENCER — a fixture driver running arrange-act-assert
   sections in order with every decision in verbs — zero decisions
   in all three. Guard-clause `if`s on a consult's exit code are fine;
   business branching is not.
3. No python3 anywhere in the METASYSTEM tree — production or
   fixture. The benchmark kit's own tooling is measurement equipment
   with its own contract, out of this program's jurisdiction by the
   standing boundary ruling (docs/concepts.md), and recorded as a
   separate candidate program for the human (r21/KS-R21-001).
4. Scripts that exist only because internal callers name their path are
   deleted; callers call the binary. Scripts named by external
   contracts (skills, docs, hooks, adopted targets) stay as shims —
   on-disk contracts are preserved (the port rule).
5. A mechanical fence keeps it this way: the suite gains a shell
   complexity budget that refuses regressions, the same way the
   word-budget audit fences prompt growth. Its checks are enumerated
   once, in Phase A, and include the no-python check and the
   per-file function-count bound (r3/KS-R3-010), and per-file
   counts of if, while, case, and for, ratcheted like every other
   number (r5/KS-R5-007). Honesty about limits (r4/KS-R4-004): the
   fence mechanically enforces ratchets and structural counts; the
   zero-decisions invariant itself is enforced by the registry's
   per-script thin-shim verdict under review discipline — syntax
   counts do not prove semantics, and the plan does not pretend they
   do. Scope (r4/KS-R4-003, r5/KS-R5-001): the budget's jurisdiction
   is exactly the scripts REGISTERED in shell-dispositions.json —
   metasystem-owned means registered; an adopted project's own files
   are never registered and never judged.

## Dead code dies first (human ruling, same day)

Hunt down dead code and kill it — across shell AND Go. This runs as
Phase 0 and as a standing rule inside every later phase:

- Phase 0 sweep: build the caller graph for every script (who invokes
  it: suite, hooks, skills, docs, adopted contracts, nobody) and every
  function within the big scripts; run the Go dead-code analyzer
  (golang.org/x/tools/cmd/deadcode) over cmd + internal. The sweep's
  product is a DISPOSITION REGISTRY checked in at
  scripts/agents/shell-dispositions.json beside the budget, one file
  with three schema'd sections (r5/KS-R5-006, r6/KS-R6-003,
  r6/KS-R6-004): scripts (path, verdict of port+shim / port+delete /
  keep, SHAPE of exec/custody/sequencer, VERIFIED date, debt
  deadline) for live files only; tombstones, where a deletion's
  entry moves; and go-packages (import path, governing plan file).
  The registry is a CLOSED WORLD and the EXPORT MANIFEST in one
  (r6/KS-R6-002, r7/KS-R7-001): adoption ships exactly the
  registry's live scripts, adopted repos validate exactly the
  registered files, and anything an adopted project adds is
  unregistered by construction — one list, no globs. Every tracked
  template shell file must carry an entry; an unregistered tracked
  script fails the fence outright. The audit verb validates all
  sections — for go-packages, TEMPLATE-ONLY, gated on the
  metasystem's OWN module path in go.mod, exact match, because
  adopted targets may be ordinary Go repositories with modules of
  their own (r6/KS-R6-001 as corrected by r8/KS-R8-001) — the same
  identity rule now governs go-gate.sh, the suite's Go sections, and
  the lease-succession fixture, fixed in shipped code during rounds
  12-13; the discriminator is THREE-STATE — metasystem module runs,
  absent source skips, source without the module line fails loudly
  as a damaged template — where "source" means the PRESENCE of ANY
  engine sentinel (internal/missionrunner/stoploss.go or
  internal/mission/ledger.go; any-of, so a merge deleting one cannot
  dress damage as adoption, r20/KS-R20-001, superseding the
  import-scanner wording of rounds 14-18, r21/KS-R21-002) — no
  lexing at all, ending the rounds 12-19 seam where an awk Go-lexer
  grew a new edge every round; adopted targets receive no Go source,
  and a collision on a sentinel path fails loudly rather than
  silently — so a broken
  module declaration can never validate green with zero Go checks
  and a sentinel collision in an adopted tree fails LOUDLY rather
  than silently (the round-20 rule, r22/KS-R22-004)
  (r12/KS-R12-001, r13/KS-R13-001, r13/KS-R13-002, r14/KS-R14-001, r15/KS-R15-001, r16/KS-R16-001, r17/KS-R17-001, r18/KS-R18-001, r18/KS-R18-002, r19/KS-R19-001): the named
  governing plan must exist, the package must exist, and an
  unreachable package without an entry fails (r7/KS-R7-002) —
  package grain is the enforcement boundary, while function-grain
  sweep findings are recorded as registry debt — go-packages entries
  carry an optional symbols list, each symbol with its own deadline,
  and the definition of done includes zero expired Go debt,
  template-only like all Go-section enforcement (r8/KS-R8-004,
  r9/KS-R9-004).
  Script entries carry an EXPORT CONDITION (always, or
  with-skill:<name>) so optional-skill scripts are registered
  without being unconditionally shipped (r8/KS-R8-003); conditions
  project through a SOURCE-TO-INSTALL PATH PAIR on the entry —
  representing the optional-skill relocation — installed when the
  condition holds, judged only when present (r9/KS-R9-005,
  r10/KS-R10-005). For scripts
  (r3/KS-R3-008): `keep` is lawful only for scripts already
  satisfying the thin-shim contract or carrying a dated port entry
  the fence treats as debt with a deadline. Adopted targets receive
  exactly the registry's live scripts — the registry IS the export
  manifest (r7/KS-R7-001, superseding the allowlist wording,
  r22/KS-R22-003) — so a deletion propagates to the payload
  automatically (r3/KS-R3-011); what a deletion REQUIRES is
  its registry verdict plus an entry in docs/migrations.md — shipped
  in the payload, one entry per deleted script naming path,
  replacement verb, and date; port+delete entries stay in the
  registry forever as TOMBSTONES, and the fence cross-checks
  tombstones against migrations.md — two durable records that must
  agree, so a vanished script cannot vanish quietly (r4/KS-R4-010,
  r5/KS-R5-005). The registry is what the fence
  checks, not raw reachability. Keep-deadline expiry is
  TEMPLATE-ONLY enforcement; adopted targets get warnings — time
  passing never fails an installed target (r4/KS-R4-007).
- Standing rule: porting a file starts by proving which of its parts
  are alive. Dead logic is deleted, never ported — porting it would
  launder it into tested-looking Go. The Go sweep uses the same
  disposition mechanism as shell (r4/KS-R4-009): staged work carries
  a registry entry naming its governing plan (internal/janitor names
  plans/supervision-lifecycle.md) and is exempt; deletion requires
  no-caller AND no-governing-plan, so dead-code-dies-first cannot
  eat parked-by-design work.
- The complexity fence (below) also counts scripts, with a NAMED
  evidence source (r22/KS-R22-002): registry entries carry a CALLERS
  manifest recorded by this sweep — EVIDENCE, never a closed graph
  (r23/KS-R23-001): reference sites with their kind (exec, source,
  hook, skill, doc), each re-verified for existence by the audit,
  informing human-reviewed dispositions; the fence checks recorded
  callers only and claims no closure. An entry whose callers list is
  empty must be tombstoned or carried as debt.

## Disposition by phase

Phase A — production gates and reports (mechanical, kills production
python), with the family table decided (r1/KS-R1-004):
assert-conformance.sh and assert-design-obligation-gate.sh → the
validate family (whole-artifact validators, F Q5.8); frontier.sh →
report; receipt.sh → a receipt family (it owns durable state and
several commands); audit-metasystem.sh → an audit family whose first
verb it becomes. Plus a residue audit of every existing assert-*.sh
shim. Prerequisite (r3/KS-R3-002): the coverage ratchet of
plans/go-production-grade.md Phase 0c lands BEFORE Phase A — the
first production ports are exactly what it protects — implemented as
a Go verb (audit coverage-ratchet) consulted by one go-gate shim
line, never as gate shell logic (r5/KS-R5-004). Recorded
supersession (r6/KS-R6-005): go-production-grade Phase 0c's wording
'add the check to go-gate.sh' is superseded on OWNERSHIP by this
verb; flagged for the human rather than edited into his plan, which
is under his own live critique. The complexity
fence lands here as `audit shell-budget`
(r1/KS-R1-008): a Go verb over a checked-in budget file that only
ratchets down — total tracked shell lines, per-file caps, per-file
control-flow construct counts, the no-python check STAGED like the
here-doc rule (r4/KS-R4-001: zero for production scripts at Phase A,
a measured ratchet for fixture scripts reaching zero at Phase F — a
fence that reds the suite on landing day would be a different
contract), the per-file function-count bound (r3/KS-R3-010), a
RATCHET over here-docs whose sink
is a shell interpreter (a syntactic pattern: piped to bash/sh or
written to a path later executed; prompt and payload here-docs are
untouched — r2/KS-R2-003), and the disposition-registry check —
numbers set from measured values at landing. The here-doc ratchet
reaches zero in Phase F, when the fixture conversion removes the
legitimate generated scripts; no refusal lands before the code it
would refuse can be ported.

Phase B — dispatch.sh lifecycle layer (the riskiest seam, its own
design round inside the loop). Scope corrected by r1/KS-R1-002: the
lock PRIMITIVE for chain and lifecycle locks already lives in Go
(internal/dispatch/ownerlock.go) — but the cap-authority lock is
still shell mkdir/rmdir polling in BOTH dispatch and arm-supervision
(r4/KS-R4-005, correcting round 1's overcorrection): Phase B ports
that primitive to the Go owner-lock discipline under the shared-lock
interoperability contract. What else ports is the remaining
choreography — when each lock is taken, held, and released; liveness
sequencing; wind-down ramps; CAS choreography; cap resolution. The phase STARTS with characterization
fixtures pinning the branches no test reaches today (r1/KS-R1-003):
rename-born lock publication, the six holder classifications,
non-owner release, lifecycle-lock timeout scaling, the
standing-vs-explicit reaper contention rule (a standing reaper skips
a busy lifecycle lock while an explicit reap waits and fails on
timeout), cap-authority acquisition timeout AND lock disappearance,
and the wind-down refusal ramps (r2/KS-R2-001) — behavior-preserving
is only meaningful against pinned behavior. SHARED-LOCK REGISTRY
(r3/KS-R3-005): locks touched by more than one phase — the
cap-authority lock is coordinated by both dispatch and
arm-supervision — carry an interoperability contract: the Go
choreography keeps the on-disk protocol bit-compatible until the
last shell participant ports, with a fixture proving mixed-era
contention. (The coverage-ratchet prerequisite of r1/KS-R1-009 moved
to Phase A per r3/KS-R3-002.) End state: dispatch.sh parses flags
and consults.

Phase C — adapters and hosts: runtime-common.sh and the four adapters
move into internal/adapter drivers under an added constraint
(r1/KS-R1-005): record authority and lifecycle serialization stay
with dispatch — drivers get a narrow Go interface whose record
mutations route through the same authority matrix the shell router
enforces today. The design gate is three-dimensional (r2/KS-R2-002):
a table mapping every driver operation to (caller classification,
authority mode, exact job scope), proven equal to the shell router's
current mapping by a test that walks the table against
internal/authority's matrix — any dimension differing fails the
gate — plus the FOURTH dimension, classification provenance
(r3/KS-R3-006): the driver's caller classification must derive from
the same live lease and process inspection the shell router
performs, never from caller-supplied claims; the matrix trusts its
input, so the derivation is the security boundary. The host boundary is launch-wait-
parse-write, NOT final exec (r1/KS-R1-006): hosts must regain control
after the runtime exits, so the shim keeps process custody while the
post-exit DECISIONS — outcome classification, session and usage
parsing, atomic result writing — become Go verbs the shim calls. The
fake adapter becomes a Go test double behind the same driver
interface.

Phase D — supervision arming and watchdog: arm-supervision.sh into
the supervise family; watch-background-jobs.sh splits along the
glossary's own boundary (r3/KS-R3-009) — its JOB-FILE classification
(done, stale, capped, never-started, vanished) goes to the report
family while process-side checks stay with supervise/census;
supervision-hook.sh stays a hook entry point but its POLICY is the
port surface (r3/KS-R3-003): suppression windows, once-per-session
stop blocking, protocol advancement, lease renewal, and evidence
collection become report/supervise verbs, the file keeping only the
harness entry contract. Receipts are NOT on this list
(r4/KS-R4-006): the hook performs no receipt operation today, and
inventing one would be new behavior — receipt state stays with
Phase A's receipt family alone.

Phase E — adoption's decisions become `metasystem adopt run`, while
adopt.sh stays a thin BOOTSTRAP shim by necessity, exactly like
go-gate (r2/KS-R2-004): it builds or locates the binary on a fresh
checkout, then execs the verb — zero decisions in shell, and the
README's fresh-checkout path stays valid. Bootstraps ARE the
custody shape (r9/KS-R9-003): they launch the toolchain, wait,
consult verbs, and finish — their registry verdicts say custody, and
no fourth shape exists.

**SEVERED — engine delivery to adopted targets (r10, three
criticals; a HUMAN decision under the reserved-decisions rule).**
Rounds 8-10 proved every mechanical answer wrong: adoption copies
the binary into gitignored space (nothing tracks it,
r10/KS-R10-001); one binary cannot serve multi-platform hosts and CI
(r10/KS-R10-002); and a binary cannot embed the HEAD that commits it
(r10/KS-R10-003). The decision — Go source in the adopted payload,
per-platform release artifacts, a rebuild allowance, or something
else — changes the adoption contract, interacts with
go-production-grade's Linux phase, and is Wido's to make; the
options and constraints are recorded here and Phase E proceeds
TEMPLATE-ONLY until ruled. Containment is COHERENCE BY PAIRING
(r11/KS-R11-001): exported scripts and the engine travel only
together, through an adoption run — the program never ships them
separately — so every adopted target holds a coherent shim-engine
pair from its adoption date, whatever the template does meanwhile.

In the template: the bootstrap proves binary FRESHNESS causally
(r3/KS-R3-007, r10/KS-R10-003) — the stamp matches the tracked
source tree at build time, and the template never commits the
binary, so no self-referential commit exists. Every bootstrap build
is NON-PUBLISHING (r7/KS-R7-003), compiling to a temporary path. The
publication protocol is ORDERED and KIND-SCOPED (r8/KS-R8-002,
r9/KS-R9-002, r10/KS-R10-004, r25/KS-R25-001): register the run's
own gate marker FIRST under the publish-bootstrap kind — the marker's
gate-name field already carries kind — then consult the fence, which
exempts the registrant's chain, REFUSES on foreign VALIDATION
markers (a live suite must never have its binary swapped), and
treats foreign PUBLICATION markers as contention; then claim
internal/dispatch/ownerlock.go's publication lock, then REVALIDATE
freshness under the lock. VALIDATION ADMISSION completes the family
(r26/KS-R26-001): on a first build the suite's admission rides its
child builder's publish-bootstrap marker; the moment a binary
exists, the suite registers its own validation marker BEFORE any
fixture runs; and suite-versus-suite contention resolves by TWO-PHASE
admission (r27/KS-R27-001): register, wait one settle grace covering
registration skew, then check — a foreign ADMITTED validation marker
always refuses the newcomer regardless of rank, and among
not-yet-admitted contenders the elder rule orders marker-CREATION
facts (the atomic rename's timestamp, pid tiebreak), a total order
both compute identically, so exactly one marks itself admitted at
grace end and the loser exits with the standing refusal — re-derive the tracked-source state and
abort unless it still equals the stamp (r11/KS-R11-002) — then
staged atomic rename. Register-then-check makes admission and replacement one protocol;
for two racing first-builds the LOCK alone adjudicates
(r23/KS-R23-002): both register markers for visibility, the lock
picks one winner who publishes, and the loser waits bounded for the
publish, re-derives freshness against the published stamp, and
proceeds as a CONSUMER of the published binary. If the bounded wait
expires with no usable binary — the winner died or published
nothing — the loser re-enters from registration and the owner-lock's
dead-holder takeover makes it the new publisher; a contender still
binary-less after one full re-entry aborts loudly (r24/KS-R24-001).
Never two publishers at once, never mutual refusal. Two implementation
requirements ride this protocol (r11/KS-R11-003, r11/KS-R11-004):
gate markers move to temp-then-rename writes — today's direct write
is a latent defect where a pruner can observe partial JSON and eat a
live registration — and the claiming process carries its generated
publication tag in its own argv for the whole critical section, the
owner-lock's ownership condition by construction. go-gate.sh's own POLICY
joins this phase too, split against the native-only rule AND the
pre-binary boundary (r6/KS-R6-007, r9/KS-R9-006, r22/KS-R22-001):
Go never launches the toolchain, and no trustworthy binary exists
when the run/skip/fail discriminator and the toolchain-presence
check execute — those PRE-BINARY guards stay in the bootstrap's
custody shape by necessity, while post-binary policy — check
ordering, failure classification, the coverage ratchet — is Go
verbs the bootstrap consults between steps. Near-minimal was a grade, not an
exemption.

Phase F — fixtures, DECIDED by evidence (r1/KS-R1-007): bash stays
the end-to-end driver — arrange via verbs, act by calling the CLI
exactly as a user would, assert via Go assert verbs; every python
heredoc dies, replaced by ONE owner (r4/KS-R4-008): a fixture-only
family (construct, mutate, corrupt, assert, poll, probe verbs)
shipped like any other family, with domain validators used where
they already exist. Fixture DECISIONS move too (r3/KS-R3-001):
budget computation, cap scaling, watcher verdicts, and cleanup
selection become Go verbs, bash keeping only section sequencing —
the drivers are lawful as SEQUENCERS (r4/KS-R4-002) and the end
state's logic-of-consequence claim carries no fixture exemption. The Go-integration-test alternative is recorded as
INVALID for the adopted contract, not merely unchosen: adopted
targets run validate-metasystem.sh without the METASYSTEM module by
design (F Q6.1-Q6.8; the module-identity rule of rounds 12-20,
r23/KS-R23-003), so fixtures that only exist
as Go tests would vanish from the very repos the suite protects. The
suite keeps driving the real CLI — the heading-order bug on
2026-08-11 was caught only because fixtures drive the shipped
surface, and that property is not negotiable.

## Definition of done (r5/KS-R5-002)

The program closes only when the registry's scripts section carries
zero debt entries, every live entry holds a VERIFIED date with one
of the three legal shapes (r6/KS-R6-004), and the adopted-engine
ruling is either MADE or the adopted payload explicitly remains the
last coherent pair, recorded in the migration notes
(r11/KS-R11-001); tombstones are outside the quantifier
(r6/KS-R6-003). Phases finishing is not the program
finishing; `keep` debt outliving the last phase is the program still
open.

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
