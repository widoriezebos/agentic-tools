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

## The three equivalence instruments (use what we own)

1. ENGINE-SWITCHED FIXTURE SUITE. The suite is black-box —
   subprocess in, files out — which makes it a conformance suite
   for free. A single switch (METASYSTEM_ENGINE=shell|go) routes
   the stable verbs to the original or the binary. The gate for
   every port: the UNCHANGED fixture files pass under BOTH engines.
   The suite runs both engines for a migrated component until its
   original is deleted.
2. DIFFERENTIAL REPLAY. A small harness (fixture-side, shell) that
   feeds IDENTICAL scripted inputs to both implementations and
   diffs normalized outputs: verb sequences against the lease
   (announce/classify/require-holder/run-held including the
   takeover and lineage edges), recorded process tables against the
   census classifier, recorded event streams against the reducer,
   real artifacts as corpora — the trials directories hold months
   of genuine job records, leases, census verdicts, and ledgers to
   replay. Divergence in bytes, exit code, or message class is a
   failure with both outputs preserved.
3. SHADOW WITNESS. Where a component is cheap and read-only, the Go
   version runs ALONGSIDE production in observe mode, writing its
   would-be verdict to a shadow file that nothing consumes (the
   flight recorder's own philosophy): the census first — every real
   scan during normal work doubles as a free differential test over
   live workloads. Divergences land in the flight recorder as
   witness events.

## Cutover discipline, per component

build → conformance green under BOTH engines → default flips to go
with METASYSTEM_ENGINE=shell as the recorded escape hatch → one
full benchmark cohort runs with the component on go → the original
is DELETED and the escape hatch for that component with it. The
flag is scaffolding, not architecture: a permanent dual path is
drift, so each component's flag dies with its original. Single
writer holds throughout: for state-writing components the engine
switch selects exactly one writer — both engines never write one
file class in one checkout.

## Phases

- PHASE 0 — the binary exists and supervises for real.
  cmd/metasystem/main.go (verbs: supervise owner|arm|shutdown|
  status, identity probe|started-at, event emit, json get);
  real-process adapters behind the tested supervise interfaces
  (setsid launches, heartbeats, fenced state publication, signal
  delivery, registry appends); suite gains go build + gofmt + vet +
  go test -race ahead of the fixtures, building into gitignored
  bin/; arm-supervision.sh execs the binary behind the engine
  switch; NEW fixture file runs the Proof list against the running
  program (purpose-gone exit, breaker at five, takeover races,
  shutdown attribution, held-fence reap). Declared transitional
  seam: the Go watcher shells to process-census.py for the scan
  until Phase 1 — one seam, visible, flagged in the plan.
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
- PHASE 5 — benchmarks on the new system: a bm-2 v0.2 cohort with
  supervision on go (minimum bar), then one with dispatch custody
  on go (full bar); scorecards annotated with the engine;
  mechanical metrics (census duration, refusal counts, leaked
  processes = zero) compared against the shell-era cohorts. This —
  not a green unit run — is what "the binary works" means here.

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

- Every port: unchanged fixtures green under both engines, and the
  differential replay reports zero divergences over the recorded
  corpora before the default flips.
- Supervision: the Proof-list fixture file green against the
  running binary; S4 fixtures green unchanged under the go engine.
- The census shadow phase records zero unexplained divergences
  across at least one full benchmark cohort before the flip.
- Phase 5's cohorts complete with zero leaked processes and the
  engine recorded in the scorecard.
