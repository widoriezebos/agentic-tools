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

## Changes

**V-1 (metasystem, claude adapter): record the answering model as the
runtime's own telemetry — better than assumption, never claimed as
attestation (BV-1-9).** The CLI result's `modelUsage` is read with a defined
contract for every shape (BV-1-5): exactly one key — that key becomes
`effectiveModel`; empty or absent — the literal `unreported`, which the
rosterPinned check fails closed; more than one key — the literal
`multi-model:<sorted,keys>`, which the check also fails closed, because the
roster gate admits one scalar and certifying either key could certify the
wrong roster. Dictionary-order selection dies. The same value flows into
the canonical role return through the existing normalization point, so the
job record and the return cannot disagree and a fail-closed roster result
cannot mutate into an evidence-set failure (BV-1-7); the fixture asserts
both copies. Handshake keeps its provisional requested-model value only
until the result arrives; result telemetry always replaces it.

**V-2 (metasystem, runner): rebuilt on the trial's actual evidence.** The
original V-2 was a no-op (BV-1-1): one shared closure path already exists.
Trial 006's runner record holds the real defect — "runner could not close
terminal job chain implementer-…a593: implementer diff.patch is not
mirrored" — reproduced live by hand. Three changes:

- **V-2a: closure never aborts the closable.** close_terminal_chains
  attempts every chain, collects failures, stamps what it can, and reports
  the failures together; one unmirrorable chain no longer strands the
  chains sorted behind it.
- **V-2b: the close path satisfies the custody it enforces.** Closing an
  implementer chain mirrors the chain's diff.patch as part of the close —
  the same mirror call the collaboration loop uses — instead of demanding
  someone else already did.
- **V-2c: completion is published after closure, and parking does not
  close chains at all (BV-1-1, BV-1-6).** The completed status becomes
  externally visible only once closure has run, so a cohort driver that
  reacts to "completed" reads closed chains. A parked mission is a
  suspension: its chains stay open so a resumed coordinator can follow up
  through existing sessions; the mission lease already fences them from
  everyone else. Chains close on completion, and on parking only via the
  existing explicit abandonment path.

Proof: a fixture with one unmirrorable and one healthy chain — the healthy
chain closes, the failure is reported, completion publishes after; a parked
fixture mission resumes and successfully follows up a pre-park chain.

**V-3 (kit, cohort driver): identity is written by the party that knows it
(BV-1-2).** The provisioner cannot honestly write the eleven-field identity
record — it does not know cohort, repetition, candidate commit, proposal,
or measuring machine. run-cohort.sh owns all of them and already stamps
extractor-visible identity per the kit's own rule; the fix completes that
stamp to the full schema and adds the single-run path: an extraction
invoked outside a cohort writes the same record with repetition 1 of 1 and
a generated cohort id, so ad-hoc trials like tonight's are attributable
too. Provision writes nothing. Proof: kit validation runs the extractor on
a cohort-stamped and a single-run target; both pass the identity check.

**V-4 (kit, schema): a versioned schema that describes every artifact the
producer can emit (BV-1-3, BV-1-8).** The census schema bumps to version 2
and the shared kit version bumps with it, per the kit's own measuring-stick
rule. Version 2 requires `generation` and `stateDigest` for SUCCESS
censuses and requires them null for the failure shapes the producer
actually emits (state unreadable, fingerprint acquisition failed) — a
diagnostic record must never be hidden behind a format error. The census
producer stamps schemaVersion 2. The extractor dispatches by the
artifact's declared version: version-1 archives validate against the
version-1 schema, so re-extraction of archived trials keeps working.
Proof: the schema suite validates a v2 success census, a v2 failure
census, and an archived v1 census, and rejects a v2 success census missing
either field.

## Ownership boundary

V-1 and V-2 change the metasystem and ride the collaboration loop:
design-critic on gpt-5.6-sol, implementer delegate, code-critic on a model
different from the implementer's, merge gated on the zero-material round.

V-3 and V-4 change the ruler itself, and the kit's rules make that a
human-approved, version-bumped change — kit validation alone is not an
acceptance path (BV-1-4). The human ratified exactly this closure in
session on 2026-08-07: "a sound solid yes to fixing all issues that you
found now", given in response to the report naming these four findings.
That ratification is recorded here, the kit version bumps in the same
change, and the kit commit is separate from any metasystem commit so the
candidate and the measuring stick never change in one batch.

## What is deliberately not changed

The validity verdict logic itself. It was right all four times tonight,
including refusing to certify a model claim nobody could prove.
