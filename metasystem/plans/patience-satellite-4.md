# Patience itself: per-activity observables and patience floors

Working Mode: design

Satellite 4 of the patience program (plans/stop-loss-satellites.md).
Parent ruling, inherited and not re-litigated: stop-loss is a last
defense, never a pacing target, and the rule applies recursively —
patience about patience is also a last defense. Vocabulary per
docs/patience.md and docs/glossary.md: progress is mechanically proven
value; patience is tolerated observation without progress; a stall is
a vocal parked verdict; slower progress is still progress.

Facts: plans/patience-satellite-4-facts.md (cited as F Qn.m). Every
shipped-mechanism claim below carries an anchor; unanchored mechanism
claims are review defects.

## The gap this satellite closes

Every existing bound is either per-round wall time or per-mission
cycle accounting. The job cap bounds one round (F Q5.3, F Q5.4); the
host turn cap bounds one turn (F Q5.10, F Q5.11); the fuse bounds
gainless mission cycles (F Q1.5, F Q1.6); the fences bound totals
(F Q5.8). Nothing bounds a delegate CHAIN that burns round after
round — each individually inside its cap, each returning schema-valid
narrative (evidence may be `inferred`, F Q3.10) — without any of it
ever becoming certified value. The runner today cannot even see
critique closure mechanically (F Q3.18). That drought is what patience
floors measure, and what the bm-2s forensics showed being tolerated
silently until the mission-level fuse fired for the wrong reason.

## Deliverable 1 — per-activity progress observables

**An observable is a checkable artifact the orchestrator or runner
already records; never a narrative claim.** Per activity:

| activity | value observables (all derivable today) |
| --- | --- |
| delegate chain round | the round's return validates for its role (F Q3.7); the round's job is certified in a concluded turn's log (F Q3.22); the chain closes |
| host turn | the turn concludes with at least one accepted dispatch, certification, or answered-ask application (F Q3.22) |
| mission cycle | classification and best marker — already the fuse's food (F Q1.5), unchanged |

Derivation, not registration: the runner computes observables at turn
conclusion by joining the tree and the durable turn log, exactly the
landed-returns pattern (F Q3.31). No new job-record fields, no new
writers into dispatch state (its mutation surface stays F Q3.3), no
change to adapters or hosts. The critique-closure gap (F Q3.18) is
closed the same way: closure is visible in the final critic return
plus the dispositions artifact, joined mechanically (F Q3.14), not by
a new record field.

**Grammar.** One new ledger annotation write form (extending the
closed set in F Q1.11), booked on the current cycle at turn
conclusion, one line per breached chain:

    - Patience: chain=<root> rounds=<n> floor=<m>

Annotations are audit trail, never fuse input — the replay invariant
(F Q1.4, F Q4.18) is untouched. The fuse stays a pure replay of
classification, best, and reset lines.

## Deliverable 2 — patience floors

**Unit: consecutive value-barren rounds per chain, not minutes.** Wall
time punishes slow runtimes; caps already bound each round's wall time
and are sealed per mission (F Q2.19, F Q2.25), so a floor in rounds
composes with the sealed cap into an implicit wall bound while staying
speed-neutral. Slower progress is still progress.

A chain's patience count at any turn conclusion = the number of its
most recent consecutive rounds that landed no observable from the
table above. The floor is the largest tolerated count.

**Configuration.** Roster keys in metasystem.conf, mirroring the cap
grammar (F Q2.8):

    patience.rounds.<role>.<runtime>.<model>=<positive integer>
    patience.rounds.<runtime>.<model>=<positive integer>

Resolution: role-pair, then runtime-pair — same shape as cap
resolution minus the explicit flag (F Q2.12); there is no per-dispatch
patience override, because patience is doctrine, not a launch
parameter. Validation extends the committed-config validator
(F Q2.8-Q2.11): positive integers, canonical models. NOT accepted in
metasystem.local.conf — that file stays cap-only (F Q2.2): a machine's
speed changes how long rounds take (cap territory), never how many
value-barren rounds are tolerable (doctrine).

**Contract-sealed overrides.** Mission contracts may carry
`patience.rounds.<runtime>.<model>` entries — runtime/model only,
exactly the signed cap-key shape (F Q2.18) — validated by extending
the contract allow-list (F Q2.17) and folded into the seal beside the
cap entries (F Q2.19), so a benchmark's numbers ride its kit and
sealed contract, never the engine's defaults (the 2026-08-11 boundary
ruling in docs/concepts.md). A sealed entry replaces the conf value
for that mission entirely.

**No built-in default.** Unconfigured means infinite patience —
exactly today's behavior, bounded only by the fuse and fences. The
shipped core is the degenerate case: floor absent, window =
ledger.no-gain-budget cycles (docs/patience.md, F Q2.27). Configured
defaults ship generous, sized in the order of the cycle fence, per
the last-defense rule applied recursively.

## What expiry does — and deliberately does not do

At each ordinary turn conclusion the runner evaluates every open
chain. For each chain at or past its floor it books the vocal
annotation (above) — repeated every concluding turn while the drought
persists, the landed-unconsumed nagging pattern — and includes the
breach in the turn's ask-candidate surface as an observation, not a
park.

Patience floors never kill, never park, never feed the breaker, and
never write fuse-visible lines. The existing fuses remain the only
actors (F Q1.23, F Q5.2, F Q5.8): a mission whose chains sit vocally
breached still parks only through no-gain, cycle budget, fences, or
the human reading the noise. Escalation from vocal to acting is a
future human ruling, taken with trial evidence, not designed now.

## Exemptions (the drain-stalled / turn-lost decision)

Cycles healed as `unmeasurable:drain-stalled` or `unmeasurable:
turn-lost` (F Q4.17) conclude no orchestrator turn and dispatch no
rounds: patience evaluation simply does not run during heal, and no
chain's count advances — nothing happened that could have produced or
withheld value. The same holds for faulted conclusions
(ConcludeFaultedTurn, F Q4.10): the turn's fault is the breaker's
jurisdiction (F Q5.2); a chain must not be debited patience for a
turn the host lost. Patience counts advance only at ordinary,
accepted turn conclusions — the only moments a certification could
have landed (F Q3.22).

## Non-goals

- No wall-clock patience anywhere; rounds only.
- No new prompt section; the orchestrator sees breaches through the
  existing ledger tail (annotations ride cycle blocks, F Q1.16).
- No per-dispatch patience flag, no local-conf patience keys.
- No change to fuse semantics, ledgerSemantics versions (F Q1.7), the
  breaker (F Q5.2), drain (F Q5.19, F Q5.20), or any adapter or host.
- No benchmark-specific numbers in conf; kits seal their own.
- Deferred unless trial evidence demands it: loop-advanced credits
  (routed here by the parent split) stay unbuilt; a closed critique
  round already lands certification observables through the normal
  table, which covers the honest-design-phase case mechanically.

## Implementation sketch

internal/mission/ledger.go: extend annotationWriteRe with the
Patience form (F Q1.11 pattern). internal/config/validate.go: accept
and validate patience.rounds.* keys (beside F Q2.8). internal/mission/
contract.go: allow-list `patience.` prefix, validate entries like
signed caps (F Q2.18), seal them (F Q2.19). New internal/missionrunner/
patience.go: pure derivation (chains → consecutive barren rounds →
breaches) over the turn log and job records, called from the ordinary
conclusion path only (F Q4.7); returns annotations plus ask-candidate
observations. Unit tests per decision; mission fixtures: a breached
chain books the annotation and nothing else; certification resets the
count; heal paths advance nothing; unconfigured missions byte-identical
to today.

## Verification

Race-detector unit tests for the derivation and each config/contract
validation; the ledger writer round-trips the new annotation; suite
green via the standing launch recipe; the degenerate case proven by
running an existing mission fixture unchanged against the new binary.
