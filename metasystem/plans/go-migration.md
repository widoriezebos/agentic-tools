# Go migration: replacing Python and most of bash with the binary

- Goal and current status: the human's ruling of 2026-08-10, given
  after the overnight run exposed that the Go work had never
  executed as a program: the goal is the Go binary RUNNING the
  metasystem — all Python retired, the decision-making bash
  retired, benchmarks executing under the new system. The domain
  packages exist and are unit-tested (internal/registry, lock,
  identity, supervise, janitor — race detector, 80-100% coverage);
  no binary exists yet. This plan is the map and the safety
  contract for the switch.
- Next step: none
- In flight right now: nothing.
- Waiting on the human: nothing — the plan itself was requested and
  is hereby recorded; Phase 0 is unblocked.

## The two standing rulings this plan implements

1. Two-language end state (backlog item 13): a decision lives in
   Go; an invocation lives in shell; no Python. One multi-verb
   binary, git-style. Wrappers keep today's names and verbs.
2. The safety ruling (2026-08-10): NEVER switch on faith. The Go
   code was written against the design, not against production, so
   equivalence must be DEMONSTRATED before any component's shell or
   python original stops running. "There is danger in you having
   written something in Go that is not completely equivalent" — the
   human, verbatim, and correct.

## Equivalence: two different bars, on purpose

PORTS get strict conformance. A port is a component whose behavior
must NOT change: the lease (worktree-lease.py), the census
(process-census.py), fences and contracts (mission-fence.py,
mission-contract.py), the flight-recorder emitter, dispatch's
records/CAS, the json helpers. For these, "the Go version" is
correct exactly when it is indistinguishable through every
interface the rest of the system touches: verbs, exit codes,
refusal messages, file bytes written, state transitions.

REPLACEMENTS get the Proof list. Supervision (the owner, gate,
janitor) implements the CLOSED supervision-lifecycle design, which
DELIBERATELY diverges from the shipped shell owner — the shell
owner is immortal (KI-32); the Go owner must die. Chasing
byte-equivalence there would be chasing the defect. The bar is the
design's ~70-row Proof list executed against the RUNNING binary,
plus the D-7 invariants (behaviors the design says must NOT change:
live-checkout self-heal, census observes-never-kills, death-only
takeover) verified by the EXISTING S4 fixtures running unchanged.

PORT-THEN-FIX resolves the defect contradiction (GO-MIG-R1-002: a
port with a KNOWN defect — the lease's KI-33, for one — cannot be
both strictly equivalent and corrected). The rule: a PORT
reproduces the original EXACTLY, recorded defects included, each
reproduced defect named in the port's checklist against its KI row;
the FIX lands as its own change AFTER the cutover, through the
loop, with its own fixture — so equivalence stays checkable and the
fix stays reviewable, and neither masks the other. A defect too
dangerous to reproduce would be a recorded per-defect exception
with the human's sign-off; none is currently claimed.

## The three equivalence instruments (use what we own)

1. ENGINE-SWITCHED FIXTURE SUITE — through the WRAPPERS ONLY
   (GO-MIG-R1-001: many fixtures bind DIRECTLY to the python files
   today — `python3 .../worktree-lease.py ...` — so a switch in the
   wrappers would reroute nothing and "green under both engines"
   would silently keep testing the original). Step zero of every
   port is therefore a NORMALIZATION PASS: inventory every direct
   binding to the component (fixtures, scripts, hooks), rewrite
   each to the stable wrapper, and land that as its own reviewed
   no-op commit — fixtures still green against shell — BEFORE the
   engine switch exists. THE PASS IS GUARDED, NOT TRUSTED
   (GO-MIG-R3-009: one missed binding recreates the false green):
   once a component's normalization lands, the suite greps the
   tree for direct invocations of that component's original (its
   filename after python3 or as an executable path) outside the
   wrapper and the original itself — any hit is permanently red.
   The switch itself lives in the wrapper
   shim and is read from METASYSTEM.CONF, never from an environment
   variable (GO-MIG-R1-006: a process-local flag cannot enforce the
   single-writer claim — two processes with different environments
   would write one file class with two engines. The conf file is
   one source per checkout, the census fingerprint already covers
   config, and a mid-flight flip therefore forces re-arm through
   the EXISTING staleness mechanism; the flip procedure is
   shutdown → edit conf → arm).
2. DIFFERENTIAL REPLAY, with an EXECUTABLE ORACLE (GO-MIG-R1-003:
   "semantic normalization" and "message classes" were weasel
   words). The oracle per comparison: exit codes EXACT; stdout and
   written files BYTE-EXACT after a NAMED normalization — each
   normalization is an enumerated regex in the harness (timestamps,
   pids, nonces, hostnames, temp paths), reviewed once, applied
   identically to both sides; stderr matched by the ANCHORED
   patterns the existing fixtures grep for, PROMOTED to a VERSIONED
   REFUSALS MANIFEST of COMPLETE LINES (GO-MIG-R2-004, sharpened by
   GO-MIG-R3-006: anchored substrings admit changed prefixes,
   suffixes, and extra lines): each entry is the FULL refusal line
   with named placeholders for variable fields; fixtures migrate
   their greps to manifest references as each component ports, and
   changing a refusal string fails the oracle until the manifest
   changes with review. IN THE REPLAY ORACLE the manifest is for
   ATTRIBUTION ONLY: the comparison itself is the WHOLE stderr
   stream, byte-equal after the enumerated normalizations —
   diagnostics, warnings, and their ordering included
   (GO-MIG-R4-011: an interface pinned only at refusal lines
   leaves the rest of the stream free to drift).
   Anything not normalized by an enumerated rule must match
   byte-for-byte, and NORMALIZATION PRESERVES VALIDITY,
   RELATIONS, AND ORDER (GO-MIG-R2-003, completed by
   GO-MIG-R3-007): normalized fields are schema-validated on both
   sides before erasure; equal source values map to equal
   placeholders on both sides (pid 41 is PID_A everywhere); and
   DERIVED CONSTRAINTS are asserted before the wash — timestamps
   mutually ordered as the records' semantics require, dual
   representations of one instant agreeing, durations equal to
   their endpoints' difference within recorded precision. The
   wash removes incidental values, never the propositions they
   participate in. REPLAY IS NECESSARY, NOT
   SUFFICIENT (GO-MIG-R1-004: corpora cannot reach live and
   clock-dependent branches): each port's checklist NAMES the
   branch classes replay covers and those it cannot, and the
   uncovered classes must be reached by the engine-switched
   fixtures — with fixture-injected clocks where the branch is
   clock-dependent (the existing env-override pattern). STATEFUL
   PORTS GET A STATE CONTRACT (GO-MIG-R3-008: lease and CAS verbs
   cannot share one sandbox sequentially): each stateful replay
   case declares its INITIAL STATE (a seeded sandbox, identical
   copies to both engines) and the port's COMPLETE EFFECTS SET
   (every file its original may write); the oracle compares the
   full effects set — including files the verb must NOT have
   touched, which must equal the seed on both sides.
3. RECORDED-INPUT REPLAY IN PRODUCTION (GO-MIG-R1-005: a live
   "shadow scan" at a different instant sees a different process
   table, so divergence would be ambient noise and "unexplained
   divergences" an invitation to explain defects away). The
   production watcher RECORDS its raw decision inputs AS ONE
   BUNDLE whose CONTENTS ARE DEFINED BY THE CLASSIFIER'S INPUT
   SEAM, NOT A PROSE LIST (GO-MIG-R2-005, completed by
   GO-MIG-R3-003: any enumerated list rots — announcements, job
   records, cwd resolutions are consumed too): the python
   classifier is instrumented ONCE to log every input a scan
   consumes — file reads, process queries, CLOCK READINGS,
   environment lookups, timezone, working-directory and path
   resolutions (GO-MIG-R4-006: reads-and-queries alone leave
   nondeterminism outside the bundle); that logged closure IS the
   bundle schema; replay INJECTS every recorded input class
   (clocks included, via the existing injection pattern) and FAILS
   any comparison in which the go side consumes anything outside
   the bundle — "consumes nothing live" enforced, not promised. Same input,
   deterministic comparison, and therefore ZERO divergences of any
   kind is the bar: there is no "explained" category. Every real
   scan still becomes a free test case; it just becomes a
   deterministic one.

## Cutover discipline, per component

normalization pass (wrapper-only bindings, no-op, committed) →
build → conformance green under BOTH engines → default flips to go
in metasystem.conf (the fingerprint forces re-arm, so the flip
cannot straddle a live armed generation) → one full benchmark
cohort runs with the component on go → SOAK: the shell original
stays present but unselected for at least one further week of suite
runs → the original is DELETED in a TAGGED cutover commit
(engine-cutover/<component>) and the conf key's shell value becomes
a refusal. POST-DELETION ROLLBACK IS A CONTRACT, NOT AN
ABSENCE (GO-MIG-R1-010): the originals live in git history; each
cutover commit's message carries the exact revert-and-re-arm
recipe, and the cutover checklist rehearses that recipe in a
sandbox ON TOP OF CURRENT HEAD immediately before the deletion
lands. THE ROLLBACK WINDOW IS BOUNDED AND SAID SO (GO-MIG-R2-011:
a rehearsal cannot vouch for a future tree): the recipe is
warranted until the next change touching that component's
contract; after that, roll-FORWARD (fix through the loop) is the
contract, which is exactly the standing rule for every other
component in the system. DEPLOYED TARGETS ROLL BACK BY
RE-ADOPTION (GO-MIG-R3-012: a fresh target has neither the
template's git history nor the cutover commit): every adoption
records the template SOURCE COMMIT in the target's provenance;
during a soak window the payload still carries the shell
originals so mid-window targets can flip locally; after deletion
the recipe for a deployed target is RE-PROVISION FROM THE TAGGED
PRE-CUTOVER COMMIT — replace the disposable target wholesale
(GO-MIG-R4-010: in-place re-adoption from an older commit would
be refused by adoption's own identity checks, so the recipe that
works is the one targets are built for: replacement; in-flight
mission state, if any matters, is parked by the ledger the same
way any re-provision parks it). Single writer holds throughout: the conf file is
the one engine selector per checkout, read at arm/dispatch entry,
recorded into the state the fingerprint covers.

## The complete inventory (GO-MIG-R2-002: the taxonomy must cover
## everything Python-removal touches, or the long tail is invented)

Every Python file and decision-bash script, classified. PORT =
strict conformance; REPLACEMENT = new-design bar; RETIRE = deleted
with its consumer, conformance not applicable; TEST-SCAFFOLD =
rewritten alongside the fixtures it serves.

- PORT: worktree-lease.py; process-census.py (classifier and
  identity verbs); mission-fence.py; mission-contract.py;
  mission-state.py; mission-ledger.py; emit-event.py;
  canonical-model.py; config-identity.py; return-schema.py;
  gate-run.py; open-work.py (its KI-34 fix is post-cutover,
  port-then-fix); select-capability-snapshot.py;
  control-plane-authority.py; second-session-isolation.py;
  dispatch.sh's record/CAS/reap logic; the json_field heredocs
  (ported once as `metasystem json get`).
- REPLACEMENT: arm-supervision.sh's run_owner loop and trap
  (KI-32 is the defect being designed away); its ARMING-GATE logic
  too (GO-MIG-R4-001: the closed design REPLACES arming — born-
  owning checkout lock, registry reservation, armed-before-lock
  ordering — so strict conformance would preserve the ownerless
  window being designed away); the registry, gate, and janitor
  (new components — no original exists).
- RETIRE: worktree-lease-fixtures.py and
  authority-regression-fixtures.py (python test scaffolds) are
  REWRITTEN as engine-agnostic fixtures when their subject ports —
  their assertions carry over one-for-one, recorded in the port's
  checklist.
- STAYS SHELL: runtime adapters, hosts, the fixture suite,
  hooks, commit wrappers (thin), benchmark drivers (for now,
  recorded scope ruling).
- ADOPT.SH IS A LATE PORT, NOT A RESIDENT (GO-MIG-R3-004: it
  DECIDES — source cleanliness, payload membership, collisions,
  configuration transformation, refusals — and the settled end
  state retires decision-making bash): its decision core ports in
  Phase 4 as `metasystem adopt`; the thin copy layer may stay
  shell; until then it is declared seam 3 in engine-seams.json.

INVENTORY COMPLETENESS IS ENFORCED AND GENERATED, NOT CURATED
(GO-MIG-R3-005, completed by GO-MIG-R4-004 — four live python
components were already missing from the hand list — and
GO-MIG-R4-005): engine-inventory.json is checked against a
FILESYSTEM SWEEP of every *.py AND every *.sh under scripts/ and
benchmark/; every file carries a classification (PORT /
REPLACEMENT / RETIRE / STAYS-SHELL-COMPOSES), and for shell files
the classification asserts DECIDES vs COMPOSES — a shell file
that decides (hooks included) must name its port phase; category
waivers are not classifications. Any unclassified file is red.
The prose lists above are illustration; the json under the sweep
is the contract.

## Phases

- PHASE 0 — the binary exists and supervises for real.
  cmd/metasystem/main.go (verbs: supervise owner|status, identity
  probe|started-at, json get — DONE 2026-08-10, first conformance
  point green: started-at byte-identical to the python original);
  real-process adapters behind the tested supervise interfaces
  (setsid launches, heartbeats, fenced state publication, signal
  delivery, registry appends); suite gains go build + gofmt + vet +
  go test -race ahead of the fixtures, building into gitignored
  bin/; arm-supervision.sh execs the binary's owner loop behind the
  conf-file engine switch. PHASE 0 ACCEPTANCE IS AN ENUMERATED
  FIXTURE SET, not "the Proof list" (GO-MIG-R1-008): the
  OWNER-ALONE rows — purpose-gone exit with none-survive teardown,
  code-edit survival, superseded-via-lock, blind idle on
  indeterminacy, breaker at five with giving-up, one-clock backoff,
  establishment bound, ceiling stop-the-set, write-ahead gating,
  launched-append retry, shutdown attribution including the latch
  and escalation, held-identity teardown with the checkout gone.
  Gate, janitor, custody, cohort, and registry-lock rows are each
  deferred TO A NAMED PHASE (gate+janitor: Phase 0b, its own
  fixture file; custody + cohort ledger: Phase 3; they are listed
  in the fixture file as EXPLICIT deferrals so the set is closed).
  PHASE 0 DOES NOT FLIP PRODUCTION (GO-MIG-R2-008: cutting real
  supervision over before the machine-wide admission gate and
  janitor exist would run the new owner without the pieces the
  design says make it safe): Phase 0 ends with the binary
  fixture-proven in sandboxes; the PRODUCTION FLIP is its own
  step, THE FLIP PROTOCOL below. THE FLIP'S PREREQUISITE BAR IS
  ENUMERATED (GO-MIG-R3-001): green runs of exactly (a) the Phase
  0 owner-alone fixture file, (b) the Phase 0b gate+janitor
  fixture file including the syscall-seam negative proof below,
  and (c) the S4 fixtures under the go engine. The Proof rows
  those files DEFER (custody, cohort ledger, registry-lock crash
  cases beyond the gate's own) gate the LATER cutovers that touch
  them — each later cutover names its required fixture files the
  same way — and Phase 0b is a DEFINED phase with its own entry
  below, not an aside. The observes-never-kills invariant gets its
  OWN fixture with the assertion AT THE SYSCALL SEAM
  (GO-MIG-R2-006, first fold lost, re-applied and sharpened per
  GO-MIG-R3-011: survivor checks miss delivered-but-ignored,
  raced-with-exit, and mistargeted signals): the census under test
  runs with its signal function behind an interface whose test
  implementation RECORDS every (pid, signal) requested and
  delivers nothing; the fixture asserts the recorded list is EMPTY
  over a table full of kill-shaped candidates — zero requested
  signals, not zero observed casualties.
  TWO transitional seams, both declared with RETIREMENT TRIPWIRES
  (GO-MIG-R1-007, GO-MIG-R1-009): (seam 1) the Go watcher shells to
  process-census.py for the scan; (seam 2) arming's lease
  operations (announce, require-holder, classify) remain
  worktree-lease.py calls in the shell arm wrapper — the Go OWNER
  itself touches no lease by design (D-1/REG-7), which is what
  makes the seam legal. The tripwire is EXECUTABLE (GO-MIG-R2-009,
  re-applied after GO-MIG-R3-010 caught the first fold silently
  failing to land): seams live in scripts/agents/engine-seams.json,
  each entry {seam, retireWhenExists: <path>} where the path is
  the ARTIFACT whose existence proves the retiring work landed
  (the census conformance fixture for seam 1; the lease
  conformance fixture for seam 2; the adopt port fixture for seam
  3). The suite check: for each unretired seam whose
  retireWhenExists path exists, fail red. No prose parsing.
- PHASE 0b — the machine-wide safety pieces, a real phase
  (GO-MIG-R4-002: a fixture filename is not an implementation
  map): the registry bootstrap (~/.metasystem location, first
  append), the ARMING GATE wired at arm entry (slot accounting
  under the registry lock, gate-resolves-actionable), the JANITOR
  as `metasystem janitor sweep` invoked at suite start and on
  demand, the held-flock reap fence, and the syscall-seam
  negative-proof fixture — all with their own fixture file, which
  is item (b) of the flip bar. PLUS the COHORT TEARDOWN LEDGER in
  the bash driver (GO-MIG-R4-003: the flip's matched pair demands
  zero leaked processes, and the closed design says today's driver
  has NO teardown — deferring it to Phase 3 would make the pair
  unable to pass; it is driver logic, stays bash per the scope
  ruling, and moves HERE because the flip depends on it).
- PHASE 1 — custody core native: census in Go (sysctl enumeration,
  exact start times; recorded-input replay first, then engine
  flip), THEN the lease — the lease port takes a design note and
  ONE sol round first (it is THE authority boundary), and it is a
  PORT under the port-then-fix rule WITHOUT EXCEPTION
  (GO-MIG-R2-001: the draft said the port would fold the KI-33 fix
  in, contradicting its own taxonomy): the Go lease reproduces
  KI-33 exactly, the conformance suite proves it does, and the
  KI-33 fix lands as the first post-cutover change through the
  loop with its own fixture. emit-event rides along as a port.
- PHASE 2 — dispatch control plane: records, CAS, lifecycle locks,
  reaper verdicts, handshake deadlines, envelopes as `metasystem
  job ...`; dispatch.sh thins to adapter orchestration, then shim.
- PHASE 3 — missions: fence (including Codex's cap authority,
  ported as-is — freshly critiqued and fixtured), contract, state,
  ledger; mission-runner.sh thins.
- PHASE 4 — the long tail: open-work (fixing KI-34's worktree
  blindness natively), config-identity, return-schema,
  canonical-model, gate-run, and the heredoc purge via `metasystem
  json`. Exit criterion: `git grep python3` in the metasystem finds
  nothing outside historical documents.
- THE FLIP PROTOCOL (replaces the old terminal "Phase 5", fixing
  GO-MIG-R2-012 — a shell control cannot run after the shell is
  deleted — and GO-MIG-R1-011): for EACH major cutover
  (supervision first, dispatch custody later), the matched pair
  runs INSIDE the cutover window while both engines exist:
  (1) QUIESCE (GO-MIG-R2-007): supervision shutdown, dispatch
  quiesced — no pending/pending-setup/running jobs, lifecycle
  locks free — verified by the flip script, not assumed;
  (2) CONTROL cohort on the shell engine — which BREAKS quiescence
  by design (GO-MIG-R3-002: the control runs real jobs), so
  quiescence is never claimed to span the window;
  (2b) RE-QUIESCE UNDER THE ADMISSION LOCK (GO-MIG-R4-007: a
  check-to-use race lets a dispatch read the old selector, pause,
  and create work after re-quiescence): the flip script takes the
  cap-authority lock — the lock arming and dispatch admission
  already serialize on — and holds it across re-quiesce, the conf
  edit, and the re-arm; dispatch reads the engine selector under
  that same lock at record creation, so no admission can straddle
  the flip instant;
  (3) flip the conf key, re-arm (the fingerprint forces the new
  generation);
  (4) CANDIDATE cohort on the go engine — same spec version,
  fences, roster, machine;
  (5) the comparison is the proof and lives in the results:
  census durations, refusal counts, handshake failures, leaked
  processes (go must be ZERO), AND the mission outcomes —
  acceptance, requirement coverage, every validity gate — equal
  or better under go (GO-MIG-R2-013). THE PAIR SUPPLEMENTS PORT
  CONFORMANCE, NEVER SUBSTITUTES FOR IT (GO-MIG-R4-008: aggregates
  can mask a port that silently stopped refusing something —
  "fewer refusals" can be a defect): port equivalence is proven
  ONLY by the conformance instruments; in the pair, mission
  outcomes must be equal-or-better while refusal and mechanical
  COUNTS must be equal within the recorded run-to-run variance,
  and ANY unexplained difference — in either direction — blocks
  deletion until attributed. EACH SCORECARD BINDS ITS
  ENGINE (GO-MIG-R3-013), BY EXECUTION ATTESTATION, NOT
  INSTALLATION RECORDS (GO-MIG-R4-009: installed-and-selected does
  not prove executed — a faulty wrapper could route to shell while
  the paperwork says go): the go binary STAMPS its build hash and
  engine name into the artifacts it actually writes (census
  verdicts, registry appends, state publications carry
  engine+hash fields), and extraction verifies the WORKLOAD'S OWN
  artifacts carry the expected stamps — a pair whose artifacts
  cannot prove which engine executed fails the protocol;
  (6) soak, then deletion per the cutover discipline. This — not
  a green unit run — is what "the binary works" means here.

## Deployment: adoption ships the engine (GO-MIG-R2-010)

A checkout-local binary that adoption never delivers would leave
benchmark targets unable to run the Go metasystem at all — the
matched pair would be impossible. Rule: `adopt.sh` ships the built
binary to the target (same machine, same architecture — the
existing reality of every target to date) TOGETHER WITH ITS
PROVENANCE: source commit sha and binary sha256, recorded in the
target's artifacts. The suite gains a gate asserting the adopted
binary's hash equals the hash of the binary built from the same
source tree, and a target's conf engine keys select it exactly as
the template's do. Cross-architecture targets are out of scope
until one exists; the provenance record is what makes that a loud
failure instead of a silent one.

## Scope rulings and open edges

- The benchmark kit's own drivers (provision.sh, run-cohort.sh,
  graders) STAY shell for now: composition, not decisions. Revisit
  after Phase 3.
- The fixture suite and adapters stay shell permanently
  (language-neutral arbiter; CLI invocation respectively).
- darwin is the only identity backend today; a linux build tag is
  Phase 4 work, recorded not implied.
- Risks named: dual-writer accidents (mitigated by the single-flag
  writer selection), silent contract drift in refusal MESSAGES that
  fixtures grep (the differential replay catches exactly this),
  and the temptation to "improve" ports mid-port — a port changes
  NOTHING; improvements come after deletion of the original,
  through the loop.

## The 2026-08-10 sequencing rulings: port first; benchmarks as
## the port's test instrument, sol roster only

The human, in three rulings the same day: (1) "postpone the whole
dev and benchmarking thing until we have everything ported and
running perfectly fine on go" — benchmark EXPERIMENTS (cap
discovery, agent comparison, anything Devin) are postponed;
(2) "You can use the benchmark to test the work you're doing on
the Go port, but the benchmark should be Opus and Sol on Claude
and Codex. So leave Devin out of the picture for now" — the flip
protocol's matched pairs are IN, as migration verification, using
the bm-2s spec (host claude-opus-5, delegates gpt-5.6-sol)
EXCLUSIVELY; no Devin cohorts until the port is done and the
human reopens them; (3) NOTHING IS DELETED during the porting
run: originals stay in the tree, rollback stays a conf-flip away;
the deletion sweep waits for the end-state acceptance pair over
the fully ported system. bm-2 v0.2's Devin fences stay committed,
dormant.

## Observability: a first-class requirement (the human, 2026-08-10)

"From the get-go focus on extreme observability so that it will be
very easy to diagnose and then fix issues with the meta-system."
Folded into the engineering standard for every Go component:
- EVERY DECISION NARRATES ITS INPUTS: the owner logs each cycle as
  one structured line — the three D-1 reads and their three-way
  results, the classification, the breaker observation and
  counter, the ceiling count, and every action taken — so a single
  log reconstructs WHY without a debugger. Same rule for the gate
  (every slot's class), the janitor (every target and every
  not-killed reason), and every refusal (what was read, what it
  meant, what would have satisfied it).
- THE FLIGHT RECORDER IS THE SPINE: the binary emits witness
  events for every lifecycle transition through the existing
  stream and registry (never-fail, never-authority), so one
  `tail -F` and one registry read tell the whole story across
  engines during the migration.
- SELF-DESCRIBING ARTIFACTS: everything the binary writes carries
  engine and build-stamp fields (already required by
  GO-MIG-R4-009) — observability and attestation are one
  mechanism.
- INTROSPECTION VERBS: every family gets its `status`-class verb
  (supervise status exists; registry reduce, janitor plan —
  print-what-would-happen without acting — follow with their
  phases).
- DIAGNOSIS-FIRST ERRORS: a refusal names the file, the value
  found, the value required, and the remedy — the census gate's
  refusal style, made law for the binary.

## Review model from round 4 on (IL-23 applied)

Four rounds ran 11, 13, 13, 11 material — the count stopped
falling, and rounds 3-4 cut mostly at the precision edges of
earlier folds. IL-23 says split or rescope at exactly this
signature, and the ratified fixtures-as-arbiter discipline names
the exit: THIS DOCUMENT is the strategy and freezes except for
structural change (a structural change reopens a master round);
each phase's IMPLEMENTATION PRECISION — the gate wiring, the
bundle instrumentation, the flip script's lock choreography —
gets its own SHORT design note and ONE just-in-time sol round
immediately before that phase implements, with the phase's
fixtures adjudicating everything below that altitude. The human
may order further master rounds at any time; none is scheduled.

## Proof (the migration's own)

- Every port: the normalization pass landed first as a reviewed
  no-op; wrapper-routed fixtures green under both engines; the
  differential replay's enumerated-oracle comparison reports zero
  divergences over the recorded corpora; the port's checklist names
  replay's uncovered branch classes and the fixture that reaches
  each.
- Ports reproduce their recorded defects (KI-named) and the fixes
  land separately after cutover, each with its own fixture.
- Supervision Phase 0: the enumerated owner-alone fixture rows
  green against the running binary; S4 fixtures green unchanged
  under the go engine; the deferral list in the fixture file is
  closed (every non-owner Proof row names its phase).
- The census recorded-input replay shows ZERO divergences — no
  explained category exists — across at least one full benchmark
  cohort before the flip.
- The seam-registry check is green (no seam outlived its phase) in
  every suite run.
- One rollback rehearsal per component executed in a sandbox before
  its original's deletion; the cutover commit is tagged and carries
  the recipe.
- Phase 5: the matched pair ran; the go cohort is no worse on every
  validity gate and mechanical metric, with zero leaked processes.
