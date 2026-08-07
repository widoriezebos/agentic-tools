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

**V-2: mission completion as a crash-recoverable state machine.**

The durable order is measure, persist, close, publish — with the persist
BEFORE closure, so no crash point can lose or repeat a measurement
(BV-6-2):

1. The gate measurement completes. The runner immediately writes
   `measuredOutcome = {classification, gatePassed, measuredCandidateSha,
   measuredAt}` into the mission state record. This write is the atomicity
   anchor: from here, resume NEVER runs another host turn and NEVER
   re-measures.
2. Closure attempts every chain: collects failures, stamps what closes
   (closing an implementer chain mirrors its diff.patch as part of the
   close), never aborts on the first failure.
3. Every chain closed: the runner publishes `completed`. Any failure: the
   mission parks with reason `chain-closure-failure`, failures named in
   the runner record.
4. `resume` reads the state: `measuredOutcome` present means closure-only
   — re-attempt step 2, publish `completed` with the preserved outcome on
   success. A crash between any two steps lands in exactly one of these
   states; each has one defined continuation.

Parks never close chains — a park is a resumable suspension, its chains
held open under the mission lease. Chains close on completion (step 2) or
through the mission's explicit abandonment path, and nowhere else.

The mission state record carrying `measuredOutcome` bumps its schema to
version 2. Like every versioned evidence artifact, it has two owners by
part (BV-6-1): the PRODUCER stamp and fields are metasystem code riding
the loop and merge gate; the kit's pinned mission-state schema gains
version 2 alongside version 1 and the extractor dispatches by the declared
version, exactly as for the census — kit change, human-ratified, kit
version bump. One pattern for all versioned evidence.

**V-3: run identity — two files, one writer each, joined by copy.**

- `execution-identity.json` (mission artifacts): owned and written ONLY by
  the runner (BV-6-3). Append-only; one entry per execution attempt (start
  and every resume), written before any turn executes. Entry fields:
  `machineFingerprint`, `adoptedMetasystemSha` — the metasystem commit
  adopted into the target, read from the adoption stamp provision writes,
  never from any repository HEAD (BV-6-4) — and `timestamp`. Completion
  appends `measuredCandidateSha`, the product tree the gate measured.
- `benchmark-identity.json`: owned and written ONLY by the cohort driver
  (or the single-run extraction path), which COPIES the execution half
  from `execution-identity.json` and adds its own: `candidateSha` keeps
  its established meaning — the source metasystem commit that selects the
  scorecard — plus `measuringKitSha` (the measuring kit's own commit),
  cohort id, repetition index and count, and proposal. The copy is the
  join; neither party ever writes the other's file; a retry is an
  idempotent rewrite from the two durable sources.
- The extractor validates `benchmark-identity.json` against its schema AND
  cross-checks the copied half against `execution-identity.json`;
  disagreement or a missing half is invalid, fail-closed. Run validity
  also requires every attempt entry to agree on `machineFingerprint` and
  `adoptedMetasystemSha`: one machine, one adopted metasystem version per
  attributable run.
- Provision writes nothing of identity except the adoption stamp it
  already writes.

**V-3a: the cohort's repetition state machine, executable and terminal
(BV-6-5).** All transitions are written by the cohort driver into its own
cohort record; nothing else transitions a repetition.

- `planned → running`: the driver provisions and starts the repetition.
- `running → graded`: mission completed and extraction valid.
- `running → pending-recovery`: mission parked, any reason — every park is
  a resumable suspension, and closure-failure parks are not special here.
  A pending repetition does NOT block the cohort: the driver proceeds to
  the next repetition and tracks all pending ones as a list.
- `pending-recovery → graded`: the operator runs the driver's `recover`
  command for that repetition; the mission resumes, reaches completed, and
  re-extraction grades it.
- `pending-recovery → ungradeable (terminal)`: the driver's explicit
  `finalize` command, run once at cohort end, converts every
  still-pending repetition. As part of that transition the driver invokes
  the mission's explicit abandonment path, which closes the parked
  mission's chains legitimately — abandonment, not parking, is the chain-
  closing act, so "parks never close chains" holds while nothing stays
  open forever and every cohort terminates.

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
and merge gate: the claude adapter (V-1); the runner's measure-persist-
close-publish machine, parking, the mission-state v2 producer stamp, and
execution-identity.json (V-2, V-3); the census v2 producer stamp (V-4).
Kit, human-ratified with the kit version bump, in commits separate from
any metasystem commit: the cohort driver's repetition state machine,
recover and finalize commands, and benchmark-identity.json (V-3a); the
version-2 entries of BOTH pinned evidence schemas — census and mission
state — and the extractor's version dispatch and identity cross-check
(V-2, V-4). The human ratified this closure in session on 2026-08-07:
"a sound solid yes to fixing all issues that you found now."

## What is deliberately not changed

The validity verdict logic itself. It was right all four times tonight,
including refusing to certify a model claim nobody could prove.
