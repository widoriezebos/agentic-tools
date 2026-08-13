# Delegated decisions — 2026-08-13 (AFK window)

Wido delegated, in writing before leaving: all review sign-off items and
in-flight judgment calls, decided by the agent and documented here for
after-the-fact review. Revert on disagreement. Entries are appended as
decisions are made; each names the alternative that was NOT taken and why.

## D1 — lease-census-7: identity fixture fenced at arming, not root-threaded

**Decision:** `supervise blocking-reserved-cap` — the verb every arming gate
(re-arm, establishment, takeover) already consults — refuses to arm when
`METASYSTEM_FAKE_PROCESS_IDENTITY_FILE` is set in a checkout whose
`metasystem.runtimes` is not `fake`. The identity package doc names the
fence.

**Alternative not taken:** threading a checkout root through
`identity.Custodian`, `census.identityAlive`, and `census.AuthIdentity` so
each read is conf-gated like `enumerateFixture`. Rejected because the
signature ripples through every custodian call site and the missionrunner's
`custodianFn` seam, while the finding's named attack vector is exactly one:
the env var leaking into an ARMED component's inherited environment. The
arming gate kills that vector at its choke point; one-shot CLI reads with
the var set remain sanctioned test usage (Go test seams depend on them).
Follow-up if Wido prefers depth: do the threading as part of W2's
lease-census-8 typed-structs work, where those signatures move anyway.

## D2 — Benchmark: cohort bm-1-20260813t113617z-83558 abandoned after rep 1

**Decision:** stop the cohort (rep 2 sealed but never resumed — no further
spend), fix the three close-protocol defects rep 1 surfaced (KI-6 round 3,
commit ef9d142) plus the kit's capped-turn gap (4b4f7d1), and start a fresh
two-rep cohort at the fixed engine.

**Why not run rep 2:** a cohort is one candidate SHA; rep 1 was already
invalid on everyChainClosed (frozen consequence of the now-fixed engine
bug), so the cohort could never yield two valid reps. Rep 2's provisioned
engine predates the fixes and would have reproduced the failure for ~$10.
Series spend so far ≈ $30 of the EUR 240 ceiling.

## D3 — fencesEnforced carries a 30-second bookkeeping allowance (kit)

**Decision:** the extractor tolerates turn wall-clock overshoot up to 30s
past the contract cap, because the runner stamps `endedAt` after return
validation, ledger delivery, and the state write (rep 1 of the first rerun:
1203s against 1200s, outcome `completed`).

**Alternative not taken (yet):** engine-side measurement of the host
interval at the process boundary, which would make the reading exact and
the allowance unnecessary. Cleaner, but touches the turn record's field
semantics — queued as a possible follow-up rather than blocking the series.

## D4 — Kit role schemas pinned as the engine-materialized v2 envelope

**Decision:** the five delegate-role schemas in `benchmark/schemas/evidence/`
are the byte output of `metasystem schema materialize --version 2` (stored
returns carry the `schemaVersion`/`claimed` envelope); mission-state and
orchestrator schemas are byte copies of the engine's shipped files.
`validate-kit.sh` enforces both rules (drift guard), plus the rule that
every engine-owned evidence filename the extractor requires still appears
in the engine's sources.

**Alternative not taken:** the extractor materializing schemas from the
TARGET's binary at scoring time. Rejected for referee independence: a
candidate must not define its own scoring contract at run time. The pin
updates deliberately, via the guard, at kit-update time.

## D5 — mirror stamps are per-job (on-disk record semantics change)

**Decision:** `mirror_record` stamps only the job actually mirrored;
sibling chain records no longer receive a copy of another job's mirror
result. This changes what a record's `mirror` field MEANS: it is now a
claim about that record's own evidence, which is what every consumer
(CloseCheck, evidence GC after codex-1/foundations-1) actually needs.
The happy-path suite fixture was updated from asserting stamp equality
across the chain to asserting the shared manifest COVERS both records —
a strictly stronger durability property.

**Alternative not taken:** keeping chain-wide stamping and teaching close
to re-mirror instead. Rejected: the chain-wide stamp was a false
durability claim (rep 1: a record stamped mirrored while its artifacts
were absent from the manifest), and stamps that can lie defeat the point
of stamping.

## D6 — CloseCheck tolerates undelivered implementer rounds

**Decision:** an implementer round with no `diff.patch` on disk blocks the
close only when its record says `completed`; timeout/failed/cancelled
rounds without a diff pass (nothing was delivered, nothing to attest). An
EXISTING diff must still be mirrored at current content, whatever the
status.

**Alternative not taken:** waiving the diff requirement for the whole
chain when any round is non-completed — too weak, would skip attestation
of diffs that do exist.
