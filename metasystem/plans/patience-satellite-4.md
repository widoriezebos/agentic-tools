# Patience itself: value observables and patience floors

Working Mode: design

Satellite 4 of the patience program (plans/stop-loss-satellites.md).
Regenerated whole after round 1 (plans/dispositions/
patience-satellite-4-r1.md, 15/15 accepted). Parent ruling, inherited
and not re-litigated: stop-loss is a last defense, never a pacing
target, recursively. Vocabulary per docs/patience.md and
docs/glossary.md: progress is mechanically proven value; patience is
tolerated observation without progress; slower progress is still
progress.

Facts: plans/patience-satellite-4-facts.md (cited as F Qn.m); round-1
corrections noted inline where the sheet was wrong (P4-006).

## The gap this satellite closes

Every existing bound is per-round wall time or per-mission cycle
accounting: the job cap bounds one round (F Q5.3, F Q5.4), the host
turn cap one turn (F Q5.10, F Q5.11), the fuse gainless cycles
(F Q1.5, F Q1.6), the fences totals (F Q5.8). Nothing bounds a
delegate CHAIN burning round after capped round — each schema-valid
(evidence may be `inferred`, F Q3.10) — without any of it becoming
value the orchestrator ever certifies. That drought is what patience
floors measure.

## What counts as value — one observable, not three

**A chain round has produced value exactly when a concluded turn's
durable log certifies that round's job with verdict `accepted`
(F Q3.22).** Nothing else counts:

- Return validation is NOT value: it proves protocol, not worth
  (internal/validate/returncomplete.go checks schema and identity
  only), and counting it would reset the exact drought this satellite
  exists to catch — a critic returning schema-valid `inferred`
  narrative every round would never look barren (P4-001).
- Certifications with verdict `rejected` are NOT value (the schema
  admits both verdicts and the runner copies them unadjudicated,
  F Q3.32 / P4-003).
- Chain close is NOT value. A closed chain leaves the evaluation set —
  the drought ends by ending the spend, which is itself a recorded
  decision — and its annotation history stays in the ledger. Unconsumed
  landed value on closed chains remains the Landed Returns section's
  jurisdiction (F Q3.31).
- Critique closure is out of scope: no mechanically discoverable
  dispositions artifact exists to join (P4-002), and with
  certification as the sole observable none is needed. F Q3.18's
  observability gap stays open and stays out of this satellite.

**The count.** A chain's patience count = the number of its terminal
rounds (job records with status completed, failed, or timeout,
F Q3.3) whose round number exceeds the chain's highest
accepted-certified round. This is a pure function of the turn log and
job records: no schema is consulted (P4-010 dissolved), late
certification retroactively heals the streak (a round certified two
turns later stops being barren), and faults cannot launder rounds
(P4-008): a round that ran under a turn whose envelope was later
rejected still exists, still cost money, and still counts until some
turn certifies it — the Landed Returns section keeps re-surfacing it
for exactly that purpose.

**Malformed evidence (P4-009).** An unreadable or malformed round
record counts barren — losing sight of a round never counts as value
(the TerminalJobStatuses principle inverted, F: missionrunner.go
comment). A record that cannot be joined to any chain forms a
single-round chain keyed by its own jobId. Chains never disappear
from evaluation by evidence damage.

## Floors: sealed mission-contract entries, nowhere else

Round 1 killed the conf layer (P4-006: the sheet's "local is cap-only"
fact was false — the file is metasystem.conf.local, its non-cap keys
are skipped by validation yet still resolved with precedence, and
environment values outrank everything; P4-007: conf fallback lets a
repository edit change a signed mission's behavior). The corrected
design:

**Patience floors exist only as mission-contract entries:**

    patience.rounds.<role>.<runtime>.<model>=<positive integer>

validated by extending the contract allow-list (F Q2.17) with the
`patience.` prefix — role required, runtime/model canonical
(capability.IsCanonicalModel, the F Q2.18 pattern widened by one
segment), value a positive integer — and folded into the seal beside
the cap entries. There are NO metasystem.conf patience keys, no local
keys, no environment override, no per-dispatch flag: a mission's
patience behavior is exactly its sealed contract, verifiable at
preflight from the contract body alone, so pre-feature contracts
verify unchanged (no entries → none expected) and byte-identical
degenerate behavior is structural rather than promised. Non-mission
dispatch chains have no runner evaluating them; the human at the
keyboard is their patience.

Benchmark and mission numbers therefore ride kits and contracts by
construction — the 2026-08-11 boundary ruling (docs/concepts.md)
needs no separate enforcement here.

**Selection (P4-014).** A chain's floor is selected by role plus the
EFFECTIVE model of its most recent terminal round (F Q3.1 records
both; fallbacks can make them differ): patience is doctrine about who
actually worked. No entry matching (role, runtime, effective model) =
infinite patience for that chain.

**Threshold (P4-011).** A floor of F tolerates exactly F barren
rounds silently. The breach books when the count strictly exceeds F.

**No wall-time claim (P4-012).** Rounds-floors bound spend-shaped
drought; wall fences bound time (F Q5.8); neither implies the other,
and the design claims no composition.

## Booking: atomic with the cycle, bounded, and actually vocal

**When.** Patience evaluates at EVERY cycle booking — ordinary,
faulted, and heal (F Q4.7, F Q4.10, F Q4.17) — no exemptions
(P4-008). Heal cycles evaluate the rounds drain ran to terminal; a
booking with no new terminal rounds advances no count by
construction, which is all the drain-stalled/turn-lost "exemption"
ever honestly was.

**How (P4-004).** The derivation takes the in-flight TurnConclusion
as an explicit input beside the durable turn log — the current turn's
accepted certifications count before anything is written — and its
annotations are passed to the SAME AppendCycle call as the cycle line
(the annotations parameter, F Q1.16, exactly the faulted-path
pattern, F Q4.10). One atomic ledger write; no ordering choice, no
crash window between cycle and annotation.

**Grammar (extending the closed write set, F Q1.11), two forms:**

    - Patience: chain=<root> rounds=<n> floor=<m>
    - Patience overflow: chains=<count>

**Bound (P4-013).** At most the 20 most-breached chains (by count
descending, chain root ascending) book per cycle; the remainder books
as the single overflow line — the landed-returns bound pattern
(F Q3.31). Annotations remain audit trail, never fuse input: the
replay invariant (F Q1.4, F Q4.18) is untouched.

**Vocal to whom (P4-005).** The Ledger Tail prompt projection drops
annotations, so the prompt surface is the `## This Turn` section:
runner-authored free text (not a validated data section) gains one
line per booked breach, `Patience: chain <root> has <n> uncertified
rounds (floor <m>) — certify landed value or close the chain.` The
ledger annotation is the human audit trail. The ask-candidate route is
dropped: candidates belong to the host's own return (F Q3.13).

**What expiry does not do.** Floors never kill, never park, never
feed the breaker (F Q5.2), never write fuse-visible lines. The
existing fuses remain the only actors (F Q1.23, F Q5.8). Escalation
from vocal to acting is a future human ruling taken with trial
evidence.

## Non-goals

- No conf/local/env/per-dispatch patience surface of any kind.
- No wall-clock patience; rounds only.
- No new validated prompt section and no Ledger Tail grammar change.
- No change to fuse semantics or ledgerSemantics versions (F Q1.7),
  the breaker (F Q5.2), drain (F Q5.19, F Q5.20), adapters, or hosts.
- No critique-closure machinery (P4-002); no return revalidation
  (P4-010).
- Deferred unless trial evidence demands: loop-advanced credits (the
  parent split's routing) — an accepted certification of a closed
  critique round already lands through the one observable.

## Implementation sketch

internal/mission/ledger.go: two new annotationWriteRe alternatives
(F Q1.11). internal/mission/contract.go: `patience.` prefix in the
allow-list (F Q2.17); entry validation (role/runtime/model + positive
integer); sealing in BOTH enumeration surfaces — expectedSeal and the
ordered emitter (P4-015) — with a seal-then-preflight round-trip unit
test proving field-count equality (F Q2.22). New
internal/missionrunner/patience.go: pure derivation
(contract entries, turn log, in-flight conclusion, job records) →
(annotations, prompt lines); called from the shared cycle-booking
path so ordinary, faulted, and heal conclusions all pass through it.
internal/mission/prompt.go: This Turn assembly accepts the breach
lines. No dispatch, adapter, or host changes.

## Verification

Race-detector unit tests: the count function (certification resets,
late certification heals, rejected certifications ignored, malformed
records barren, orphan records isolate, threshold strictly-exceeds,
20+overflow bound, effective-model selection); contract validation
and the seal round-trip (P4-015); ledger writer round-trips both
annotation forms. Mission fixtures: a breached chain books annotation
+ This Turn line and nothing else; an unconfigured mission's turn
artifacts are byte-identical to today's; heal bookings with no new
terminal rounds advance nothing. Suite green via the standing launch
recipe.
