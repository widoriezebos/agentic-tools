# Benchmark validity closure: the four findings of trial 006

- Goal and current status: every reason the extractor stamped trial 006 invalid is fixed at its owner — two in the metasystem, two in the measuring kit — so the next run's validity verdict reflects behavior, not instrumentation debt. Status: IN CRITIQUE, round 1 folded (9 findings, 8 material, BV-1-1..9).
- Next step: design critique until closed by join; then implement through the loop.
- In flight right now: nothing; round 2 dispatches next
- Waiting on: nothing

## The findings, from the run's own records

Trial 006 (Opus coordinating, luna building, Opus-requested critics) scored
0.981 from the held-out grader with genuine delegation and an honest
self-check — and was correctly stamped INVALID for four reasons:

1. `rosterPinned`: the critic jobs carry `effectiveModel=None`. The claude
   adapter launches the CLI with `--model` but never records which model
   actually answered; the stored event stream contains no model string
   either. The human's question — "how sure are we it was Opus and not
   Fable?" — has no evidence-backed answer, and must never be unanswerable
   again.
2. `everyChainClosed`: all three implementer chains were left open at
   mission COMPLETION. The D-M1 rule closes chains when a mission parks;
   the completed path evidently does not close what the coordinator opened.
   The critic chains closed; the implementer chains did not.
3. `evidenceSetComplete/benchmarkIdentity`: the extractor requires an
   identity file the provisioner never writes.
4. `evidenceSetComplete/lastCensus`: the kit's evidence schema rejects
   `generation` and `stateDigest` — fields the census has carried since the
   KI-18 fix. The ruler lags the thing it measures.

## Changes (consolidated at round 5: one specification, no layered amendments)

**V-1: the answering model is recorded as runtime telemetry.** The claude
adapter reads the CLI result's `modelUsage`: one key becomes
`effectiveModel`; absent or empty becomes the literal `unreported`; multiple
keys become `multi-model:<sorted,keys>`. The latter two fail the
rosterPinned check closed. The value flows through the existing return
normalization so job record and role return always agree, and the fixture
asserts both. Telemetry is candidate-side evidence under the declared trust
model, never attestation. Handshake holds the requested model only until
the result replaces it.

**V-2: chain closure at mission end, specified whole.**
- Closure attempts every chain, collects failures, stamps what closes, and
  reports failures together; no abort-on-first.
- Closing an implementer chain mirrors its diff.patch as part of the close.
- Chains close on completion only. A park never closes chains; parked
  chains stay open under the mission lease for resume.
- Completion publishes only after closure succeeds for every chain. If any
  closure fails, the mission parks with reason `chain-closure-failure`.
- That park persists the measurement it interrupted: the mission state
  record gains `measuredOutcome = {classification, gatePassed,
  measuredCandidateSha, measuredAt}` — gatePassed persisted explicitly so
  resume knows whether the original result was a gate success (BV-5-2);
  measuredCandidateSha matches the ledger's candidate-sha, the tree the
  gate measured.
- `resume` on that reason re-attempts closure only; on success it publishes
  completed carrying the preserved measuredOutcome. No re-run, no
  re-measurement.
- The mission state record's schema bumps to version 2 for these fields,
  with a versioned reader exactly like the census schema below; version-1
  state records remain readable (BV-5-3).

**V-3: run identity, stamped by the party that knows each half.**
- The runner owns `artifacts/agents/missions/<id>/execution-identity.json`:
  append-only, one entry per execution attempt (start and each resume),
  each entry carrying machine fingerprint, measuring metasystem commit, and
  timestamp; completion appends the measured candidate sha. Run validity
  requires every attempt to agree on machine fingerprint AND measuring
  commit — an attributable run spans one machine and one measuring code
  version (BV-5-4).
- The cohort driver completes the record with cohort id, repetition index
  and count, and proposal; a single-run extraction completes it with
  repetition 1 of 1 and a generated cohort id. A missing runner half makes
  the run invalid, fail-closed. Provision writes nothing.
- A chain-closure-failure park has a durable cohort state: the cohort
  record marks that repetition `ungradeable-pending-recovery`; a later
  resume-to-completed lets re-extraction grade it; the cohort's finalize
  step converts any still-pending entry to permanently ungradeable so a
  cohort always terminates (BV-5-5).

**V-4: versioned census evidence.**
- The census schema bumps to version 2; the shared kit version bumps.
- `generation` and `stateDigest` are required PROPERTIES on every v2
  census; they must be non-null when verdict is SUCCESS and may be null
  otherwise — presence unconditional, nullability verdict-scoped.
- The producer stamps schemaVersion 2 (metasystem change, rides the loop).
- The extractor selects the validator by declared schemaVersion, rejects
  unknown versions, and the kit suite carries per-version fixtures
  including malformed-v2 rejections: missing SUCCESS fields, wrong types,
  undeclared properties. Version-1 archives validate against version 1.

**Ownership matrix (by part).** Metasystem, through the collaboration loop
and merge gate: the claude adapter (V-1), runner closure, parking,
mission-state v2, and identity stamping (V-2, V-3 runner half), the census
producer stamp (V-4). Kit, human-ratified with the kit version bump, in
commits separate from any metasystem commit: the cohort driver's park
handling and identity completion (V-3), the census schema and extractor
dispatch (V-4). The human ratified this closure in session on 2026-08-07:
"a sound solid yes to fixing all issues that you found now."

## What is deliberately not changed

The validity verdict logic itself. It was right all four times tonight,
including refusing to certify a model claim nobody could prove.
