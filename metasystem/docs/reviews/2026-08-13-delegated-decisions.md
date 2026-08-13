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

## D7 — Rep 2 no-go; four engine fixes land first (fence-refusal diagnosis)

**Decision:** rep 2 of cohort bm-1-20260813t132947z stays unspent until four
fixes land, because rep 1 was not a fair measurement: half its signed job
budget was consumed by reservations of dispatches that never started a
process, and the headroom line lied to the host every turn.

The diagnosis (agent-verified, evidence in the cohort target's record-locks
mktemp residue): of the four "dispatch-refused" husks, only ONE was the
mission fence — and that refusal was CORRECT (two genuinely live delegates
at concurrency 2, one an orphan of the crashed round-1 host). Two died on
writable-without-worktree (host error, correct refusal), one on a
capability-snapshot miss after the codex CLI rewrote its own config
mid-run (KI-19's class). The capMin 7 and 12 the implementers timed out
under were the HOST's own `--cap-min` requests against a signed ceiling of
15 — the engine's arithmetic is exonerated. The host then wrote a false
ledger fact ("fence refused ... even though the contract states
fence.concurrency=2") because the prompt never showed concurrency headroom
or the live roster.

**Fixes landed:** (1) `mission fence-release-job` + `fail_setup_husk`
releases a husk's reservation — fence.jobs headroom becomes honest and the
reserve-before-setup concurrency race closes. (2) The turn prompt's fence
headroom now carries `concurrency=free/limit` plus the live-delegate
roster. (3) Every husked dispatch emits a `job-refused` event with a
reason class (fence/envelope/capability/worktree/setup) and stamps the
class into the record — rep 1's diagnosis needed mktemp file sizes as its
primary instrument. (4) A capability-snapshot miss self-heals with ONE
adapter probe and re-select; an unhealable miss still refuses, now naming
the failed probe. Plus the policy line in roles/orchestrator.md: dispatch
at the signed fence.job-cap-min unless there is a stated reason, and a
brief's budget must agree with --cap-min — the run's actual proximate
cause, which no engine fix touches.

**Alternative not taken:** rerunning as-is and averaging over host
variance. Rejected: the dominant failure mode (host-chosen 7-minute cap
for a 9-minute brief) is unconstrained by anything in the engine or
prompt, so it varies randomly rather than reproducing, and each sample
costs ~$10 while measuring a known-dishonest headroom display.

**Also noted for later:** an orphaned job's cap enforcement degrades to
standing-reaper granularity when its waiting dispatcher dies (f29a overran
its capDeadline by 75s); not fixed in this pass.

## D8 — Close attests evidence, not host workflow (amends D6); usage owns extraction

**Decision (CloseCheck):** rep 1 of cohort bm-1-20260813t155700z — the first
run with all D7 fixes — failed everyChainClosed on a NEW shape: the host
dispatched at the signed cap (the D7 prompt/policy worked), the implementer
COMPLETED, but the host parked on stop-loss without ever running the
conformance review — and diff.patch is CREATED by that review, never by the
reap. D6's completed-without-diff refusal therefore wedged the close of
every unreviewed chain at mission end. Amendment: an absent diff never
blocks the close (the unreviewed-implementer workflow gap is already
delegationFloorMet's verdict); a diff the MANIFEST knows but the disk lost
is still evidence loss and refuses ("vanished after mirroring"); an
existing diff must still be mirrored at current content.

**Decision (architecture-1):** internal/usage is the single owner of typed
usage extraction — DevinUsage (the host and adapter copies deleted),
CodexUsageValue, and RootJobID (usage attribution is per chain; W2's
walker consolidation may re-home it). mission/fence.go is off its adapter
import. The two former writers (host canonicalJSON, adapter encodeJSON)
were byte-identical wiredoc bodies, so the consolidation changes no
on-disk bytes; usage's writer carries the same body. host re-floors at
79.3 (well-covered code moved out, shifting the remaining ratio — the
deterministic-minimum rule, lock=84.0 precedent).

**Cohort consequence:** bm-1-20260813t155700z abandoned after rep 1 (one
SHA per cohort; rep 1 invalid on the now-amended close rule plus an honest
delegationFloorMet). Rep 2 unspent. Fresh cohort at the next validated pin.

## D9 — One directory-lock protocol; bindings keep their bytes (W2.2)

**Decision:** internal/lock is the single home of the rename-born
directory-lock protocol. It gained exactly two things: an OwnerCodec
option, so each binding's on-disk owner.json schema stays byte-compatible
(dispatch: {pid,instanceTag,acquiredAt}; census:
{function,pid,pidStartedAt,instanceTag,observedAtEpoch}); and a Cause on
HolderError so bindings can name a malformed owner file. NO staleness
extension: dispatch's tag-based stale-holder rule is its PROBE answering
Dead for a live pid whose successfully READ argv lacks the recorded tag —
the custodian stranger rule, which death-only takeover already expresses.
codex-2's row (Alive with unreadable argv keeps the lock) and
dispatch-supervise-3's row (Unknown refuses takeover) live in the
bindings' probes and are still pinned by their verdict tests, now driven
through the one implementation. ownerlock.go and censuslock.go are thin
bindings (~120 lines each of schema, probe, and refusal wording).

**Semantic change accepted:** dispatch's owner lock previously returned
Busy for an OWNERLESS husk; via lock.Acquire it now heals one — the
canonical package's documented garbage-by-construction rule, which the
review names as exactly what the copies lacked.

**Alternative not taken:** a fourth Liveness state for staleness.
Rejected: it would weaken the death-only takeover invariant every
consumer reasons about, when Dead already means "a successful read proved
the RECORDED identity absent".

## D10 — Rep 2 proceeds; the delegation floor's strictness is flagged for Wido

**Decision:** rep 1 of cohort bm-1-20260813t171239z (engine 2ef72cf, all
D7+D8 fixes) passed six of seven validity gates — everyChainClosed passes
with an unreviewed completed implementer, closing the whole close-protocol
arc. The one failure is delegationFloorMet, and its cause is now purely
the measured system: TWO implementers completed at the signed cap 15, the
host certified only design-critic rounds, and stop-loss parked the mission
before any implementer certification. Rep 2 runs as-is (~$10, within the
pre-approved envelope): the measurement is clean, so a second sample is
fair — the first cohort's rep 2 DID meet the floor once, so this is
mission-dynamics variance, not determinism.

**Flagged for Wido (kit measurement design, not decided under
delegation):** the floor requires a completed AND CERTIFIED implementer
per stream. Real delegation demonstrably happened (two healthy completed
implementer jobs); what failed is the host's adjudication step before a
stop-loss park. Whether that belongs in runValidity (as now) or in
productMetrics is a benchmark-design judgment that shapes the whole
series — if the strict floor stands and baseline reps keep failing it,
the Devin arms compare against no valid baseline. Not amended: re-scoping
a validity gate mid-series is referee tampering unless it is clearly a
kit defect, and this one is arguably intent.

## D11 — The drain-stall incident: my cadence change caused it; four fixes land

**What happened (rep 2 of bm-1-20260813t171239z, agent-diagnosed):** the
mission parked drain-stalled exactly 2.0s past a delegate's capDeadline,
because the drain's park window is capDeadline + 2s handshake grace while
my own 5f5db7f moved the kill-capable dispatch reap to a 5s cadence — a
~40% chance per expiry of parking without ever serving a reap inside the
window. The runner then exited 0 over the parked state and the cohort
graded FIVE SECONDS later while the delegate was still writing code (it
self-finished 60s past its cap; the score measured a bare checkout). My
earlier premise ("20+ minutes uncapped") was a timezone misread, and D7's
note that an orphan's cap "degrades to reaper granularity" was WRONG: the
Go standing reaper has no kill authority by design, so an orphaned job's
cap degrades to UNBOUNDED once its waiter and runner are gone.

**Landed:** F1 — the drain never parks while a kill-capable reap is owed:
at the deadline it runs the dispatch reap for every live record, re-reads,
and only parks what a fresh reap could not resolve. F2 — the drain
witnesses every reap's exit and stderr (this incident was undiagnosable by
artifact; the runner log was empty) and emits a job-refused event on
failure. F6 — the standing-mode shell reap sweep reports failures without
exiting (my accumulate change would have terminated a shell standing
reaper before its heartbeat — strictly worse than the starvation it
replaced; unbitten only because the standing reaper is the Go component).
F3 (kit) — grading a drain-stalled park waits out still-running survivors
(their caps bound them, ten-minute ceiling) and refuses rather than
scoring a bare checkout.

**Deferred, flagged for the review backlog:** F4 — the orphan window
itself (a job whose waiter and runner both exit has NO kill-capable
enforcer until mission end); needs a design decision between waiter
adoption and a kill-capable supervision sweep, which touches the
no-kill-authority rule and deserves its own pass. F5 — the standing
reaper logs nothing when it declines to act; emit the
running∧CapExpired∧custodian-alive state once per pass.
