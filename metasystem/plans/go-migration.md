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
   engine switch exists. The switch itself lives in the wrapper
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
   patterns the existing fixtures already grep for, extracted into
   the harness verbatim. Anything not normalized by an enumerated
   rule must match byte-for-byte. REPLAY IS NECESSARY, NOT
   SUFFICIENT (GO-MIG-R1-004: corpora cannot reach live and
   clock-dependent branches): each port's checklist NAMES the
   branch classes replay covers and those it cannot, and the
   uncovered classes must be reached by the engine-switched
   fixtures — with fixture-injected clocks where the branch is
   clock-dependent (the existing env-override pattern).
3. RECORDED-INPUT REPLAY IN PRODUCTION (GO-MIG-R1-005: a live
   "shadow scan" at a different instant sees a different process
   table, so divergence would be ambient noise and "unexplained
   divergences" an invitation to explain defects away). The
   production watcher RECORDS its raw scan input — the enumerated
   process table snapshot it actually classified — and the Go
   classifier replays THAT RECORDED INPUT offline. Same input,
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
recipe, and the cutover checklist rehearses that recipe ONCE in a
sandbox before deletion — a rollback path nobody has executed is a
hope, not a path. Single writer holds throughout: the conf file is
the one engine selector per checkout, read at arm/dispatch entry,
recorded into the state the fingerprint covers.

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
  TWO transitional seams, both declared with RETIREMENT TRIPWIRES
  (GO-MIG-R1-007, GO-MIG-R1-009): (seam 1) the Go watcher shells to
  process-census.py for the scan; (seam 2) arming's lease
  operations (announce, require-holder, classify) remain
  worktree-lease.py calls in the shell arm wrapper — the Go OWNER
  itself touches no lease by design (D-1/REG-7), which is what
  makes the seam legal. The tripwire: the suite carries a
  seam-registry check listing each seam with its retiring phase;
  when a phase completes (its plan row flips), a seam it should
  have retired turns the check RED. A seam cannot silently outlive
  its phase.
- PHASE 1 — custody core native: census in Go (sysctl enumeration,
  exact start times; shadow-witness first, then engine flip), THEN
  the lease — the lease port is the one piece that takes a design
  note and ONE sol round first (it is THE authority boundary), and
  it folds the KI-33 fix (same-process re-announce reuses the
  mainId) in as designed behavior. emit-event rides along.
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
- PHASE 5 — benchmarks on the new system, as a MATCHED PAIR
  (GO-MIG-R1-011: an uncontrolled go cohort proves running, not
  equivalence): a shell-engine CONTROL cohort and a go-engine
  candidate cohort, same spec version, same fences, same roster,
  same machine, run back to back; scorecards annotated with the
  engine; the comparison IS the proof and lives in the results —
  census durations, refusal counts, handshake failures, leaked
  processes (go must be zero), and the mission-level outcomes,
  with the go cohort required to be no worse on every validity
  gate and mechanical metric. This — not a green unit run — is
  what "the binary works" means here.

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
