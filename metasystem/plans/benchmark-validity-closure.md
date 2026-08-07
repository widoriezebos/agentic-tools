# Benchmark validity closure: the four findings of trial 006

- Goal and current status: every reason the extractor stamped trial 006 invalid is fixed at its owner — two in the metasystem, two in the measuring kit — so the next run's validity verdict reflects behavior, not instrumentation debt. Status: DRAFT, awaiting critique.
- Next step: design critique until closed by join; then implement through the loop.
- In flight right now: design critique round 1, dispatched to the design-critic role on gpt-5.6-sol
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

## Changes

**V-1 (metasystem, claude adapter): record the answering model, observed
not assumed.** The CLI's result JSON reports usage per model (`modelUsage`);
the adapter reads the answering model from there and writes it as the job
record's `effectiveModel`. When the result carries no model, the adapter
writes `effectiveModel="unreported"` — never silently equating requested
with effective, which is what the current fallback does at handshake. The
codex adapter already records effective models; this brings claude to
parity. Proof: a fixture drives the fake CLI shape with and without
`modelUsage` and asserts the record shows the observed model or the literal
"unreported"; the rosterPinned check fails on "unreported", fail-closed.

**V-2 (metasystem, runner): completion closes chains exactly like parking
does.** The runner's end-of-mission measurement closes every chain the
mission opened — one closure path shared by parked and completed, so the
two cannot drift again. Proof: a fixture completes a fixture mission with
an open implementer chain and asserts zero open chains and the
runner-closed stamp afterward.

**V-3 (kit, provisioner): write the identity file the extractor demands.**
Provision emits `benchmarkIdentity` (spec id, kit version, provision
timestamp injected by the caller, target path hash) at the location the
extractor reads. Proof: kit validation provisions a probe target and the
extractor's evidence-set check passes on it.

**V-4 (kit, schema): the evidence schema accepts the census as it is.**
`generation` (integer ≥ 1) and `stateDigest` (64 hex) become required
census fields — required, not optional, because their absence is exactly
the staleness KI-18 taught dispatch to refuse. Proof: the schema validates
a current census verbatim and rejects one missing either field.

## Ownership boundary

V-1 and V-2 change the metasystem and ride the collaboration loop:
design-critic on gpt-5.6-sol, implementer delegate, code-critic on a model
different from the implementer's, merge gated on the zero-material round.
V-3 and V-4 change only the kit, which lives outside the metasystem's merge
gate; they are still critiqued in this design and implemented by a
delegate, and kit validation is their gate.

## What is deliberately not changed

The validity verdict logic itself. It was right all four times tonight,
including refusing to certify a model claim nobody could prove.
