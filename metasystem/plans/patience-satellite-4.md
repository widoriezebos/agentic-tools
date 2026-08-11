# Patience itself: value observables and patience floors

Working Mode: design

Satellite 4 of the patience program (plans/stop-loss-satellites.md).
Regenerated whole after rounds 1 and 2, amended in place after
rounds 3-5 (plans/dispositions/patience-satellite-4-r{1..5}.md;
40/40 accepted; convergence 15 → 13 → 4 → 4 → 4, severity falling).
Parent ruling,
inherited and not re-litigated: stop-loss is a last defense, never a
pacing target, recursively. Vocabulary per docs/patience.md and
docs/glossary.md: progress is mechanically proven value; patience is
tolerated observation without progress; slower progress is still
progress.

Facts: plans/patience-satellite-4-facts.md (cited as F Qn.m). Two
sheet corrections stand on the record: local config is NOT cap-only
(r1/P4-006) and the dispatch transition map is not the terminal-status
authority (r2/P4-022).

## The gap this satellite closes

Every existing bound is per-round wall time or per-mission cycle
accounting: the job cap bounds one round (F Q5.3, F Q5.4), the host
turn cap one turn (F Q5.10, F Q5.11), the fuse gainless cycles
(F Q1.5, F Q1.6), the fences totals (F Q5.8). Nothing bounds a
delegate CHAIN burning round after capped round — each schema-valid
(evidence may be `inferred`, F Q3.10) — without any of it becoming
value the orchestrator ever certifies. That drought is what patience
floors measure.

## What counts as value — one observable

**A chain round has produced value exactly when a concluded turn's
durable log certifies that round's JOB, by jobId, with verdict
`accepted` (F Q3.22, F Q3.13).** Nothing else counts:

- Return validation is NOT value: it proves protocol, not worth
  (internal/validate/returncomplete.go checks schema and identity
  only); counting it would reset the drought this satellite exists to
  catch (r1/P4-001).
- Certifications with verdict `rejected` are NOT value (r1/P4-003).
- Chain close is NOT value: a closed chain leaves the evaluation set —
  ending the spend is itself a recorded decision — its annotation
  history remaining in the ledger. Unconsumed landed value stays the
  Landed Returns section's jurisdiction (F Q3.31).
- Critique closure is out of scope (r1/P4-002): no discoverable
  dispositions join exists, and none is needed.

**Certification join (r2/P4-024).** A certification counts only when
its jobId resolves to a job inside the mission's own job set (the
missionJobs ownership authority: mission stamp plus fence
reservation) AND that job is a lineage member of the chain under
evaluation. Foreign or nonexistent jobIds are ignored by patience;
adjudicating certification hygiene is a possible future satellite,
not this one.

## The count — a pure function over the chain set

**Patience count = the number of terminal, jobId-uncertified jobs in
the chain's job set** — the set of records reachable from the root by
the parentJob walk, which tolerates branches: sibling follow-ups
simply belong to the set, and one annotation per root stays
well-defined (r3/P4-029). A round is terminal when its status is in
missionrunner.TerminalJobStatuses — completed, failed, timeout, AND
cancelled (r2/P4-022) — so cancellation cannot launder a spent round;
an attributable record whose status is missing or outside that
vocabulary ALSO counts, deliberately diverging from the fence
direction (r3/P4-031): fence projection acts on money, so losing
sight must never finish a job; patience only speaks, so losing sight
must never hide a drought. Certifications join by jobId; round
numbers play no part (r2/P4-023). A certified job leaves the count
wherever it sits — healing is inherent — and faults cannot launder
rounds (r1/P4-008): a round run under a rejected-envelope turn still
exists, still cost money, and still counts until certified; Landed
Returns keeps re-surfacing it for exactly that purpose. Barren early
rounds under a later certified sibling keep counting until certified
or the chain closes: their spend never landed witnessed value.

**Evidence damage (r1/P4-009, r2/P4-020, r2/P4-021, r4/P4-035).**
The input set is the mission's own jobs (missionJobs ownership
authority) whose recorded jobId is valid under the job-id grammar
AND equals the record's filename stem — the on-disk identity that
certification resolution and dispatch close actually address
(r5/P4-039). Everything else — fully unreadable records,
unattributable records, and attributable records without that usable
identity — is outside patience and belongs to
the janitor and usage jurisdictions (satellite 3): patience's only
remedies are certify-by-jobId and close-by-chain, and neither can
touch an identity-less record, so counting one would be a nag with no
possible act. Within the input set: a damaged record counts (see the
status rule above); a record whose parent walk cannot join a chain
forms a single-round ORPHAN chain keyed by its own jobId. Every
counted identifier is grammar-safe by construction, so annotations
and prompt lines interpolate only job-id-grammar tokens.

## Floors: sealed mission-contract entries, nowhere else

Round 1 killed the conf layer with evidence (r1/P4-006: the local
file resolves any key with precedence and environment outranks it;
r1/P4-007: an unsealed fallback lets a repository edit change a
signed mission). Floors exist only as mission-contract entries:

    patience.rounds.<role>.<runtime>.<model>=<positive integer>

validated by a `patience.` prefix in the contract allow-list
(F Q2.17): role and runtime in the identifier grammar, model a
CANONICAL model key in the same encoding cap keys use (config/
model.go's canonical mapping — e.g. gpt-5.6-sol is keyed
gpt-5-6-sol; r2/P4-018), value a positive integer. Entries are folded
into the seal beside the cap entries in BOTH enumeration surfaces —
expectedSeal and the ordered emitter — with a seal-then-preflight
round-trip test (r1/P4-015, F Q2.22).

No conf keys, no local keys, no environment override, no per-dispatch
flag. A mission's patience behavior is exactly its sealed contract;
pre-feature contracts verify unchanged under a new binary (no entries,
none expected). **The reverse direction is unsupported by
declaration (r2/P4-026):** a pre-feature binary refuses a
patience-bearing contract through the existing unknown-key preflight
rule — loud refusal, never silent misbehavior; upgrade the engine or
strip the entries. Non-mission chains have no runner evaluating them;
the human at the keyboard is their patience. Benchmark numbers ride
kits and contracts by construction (the 2026-08-11 boundary ruling).

**Floor selection (r2/P4-018, r2/P4-019, r3/P4-030).** A chain's
floor is matched on (role, runtime, canonical model). The fallback
rows exist for MISSING OR NON-CANONICALIZABLE model evidence only; a
model that canonicalizes cleanly but matches no entry is
configured-nothing for that pair. **Job order is total
(r4/P4-034, r5/P4-038):** jobs sort by (endedAt, startedAt, jobId)
descending; timestamps parse as RFC3339 and a missing OR unparseable
value sorts oldest (one bucket), ties falling through to the next
key, jobId the final lexicographic tiebreak — so "newest" is
deterministic across sibling branches and damaged records alike. The
resolution table, rows tried in order, first applicable row wins
(r4/P4-033):

| row | condition | floor |
| --- | --- | --- |
| 1 | some terminal job in the chain set has a canonicalizable effectiveModel: take the NEWEST such job's model; an exact (role, runtime, model) entry exists | that entry |
| 2 | same model evidence as row 1; no entry for that triple | infinite — configured-nothing |
| 3 | no terminal job canonicalizes; the chain root's requestedModel canonicalizes; an exact entry exists | that entry |
| 4 | same as row 3; no entry for that triple | infinite — configured-nothing |
| 5 | no canonicalizable model evidence anywhere; any (role, runtime, *) entries exist | the SMALLEST such floor — damage must never widen patience |
| 6 | no canonicalizable model evidence anywhere; no (role, runtime, *) entries | infinite |

**Threshold (r1/P4-011).** A floor of F tolerates exactly F barren
rounds silently; the breach books when the count strictly exceeds F.

**No wall-time claim (r1/P4-012).** Rounds-floors bound spend-shaped
drought; wall fences bound time; neither implies the other.

## Booking: one write, bounded, with an honest crash contract

**When.** Patience evaluates at EVERY cycle booking — ordinary,
faulted, and heal (F Q4.7, F Q4.10, F Q4.17); no exemptions
(r1/P4-008). A booking with no new terminal rounds advances no count
by construction.

**How.** The derivation reads the mission's job records, the durable
turn log, and the in-flight TurnConclusion (its accepted
certifications suppress breaches before anything is written), and its
annotations ride the SAME AppendCycle call as the cycle line
(F Q1.16, the faulted-path pattern F Q4.10).

**The crash and race contract (r2/P4-016, r2/P4-025) — stated, not
wished away.** The pre-append read is the linearization point: booked
counts are counts-as-read. Two windows exist and are accepted because
the surface is non-acting audit, self-repairing at the next booking:
a crash between AppendCycle and the state write can leave one cycle's
vocal line stale against the healed log, and a reaper transition
landing mid-write books one evaluation late. A "final booking" cannot
strand a stale line anywhere that matters: a completed mission needs
no nag and a parked mission's ask is the vocal surface. No retry loop
is added around the flocked append.

**Grammar — write AND read (r1/P4-013, r2/P4-027, r2/P4-028,
r5/P4-037).** Three new annotation forms in BOTH annotationWriteRe
and the read-side annotation grammar, with a parse round-trip test —
the chain KIND lives in the durable bytes so the prompt projection
needs no second source:

    - Patience: chain=<root> rounds=<n> floor=<m>
    - Patience: orphan=<id> rounds=<n> floor=<m>
    - Patience overflow: chains=<count>

Bound copied exactly from the landed-returns implementation
(F Q3.31): at most 20 lines total per booking — 20 detail lines, or
19 detail plus 1 overflow when breaches exceed 20. Ranking
(r5/P4-040): breach distance (count − floor) descending, tiebreak
count descending, then root ascending. Annotations remain audit trail,
never fuse input; the replay invariant (F Q1.4, F Q4.18) is
untouched.

**Vocal to whom (r1/P4-005, r2/P4-017).** Two surfaces, one durable
source. The ledger annotation is the human audit trail. The
orchestrator sees breaches in its NEXT prompt: at assembly time the
runner projects the final cycle block's Patience annotations
(readable because the read grammar round-trips) into `## This Turn`
lines — runner-authored free text, validator-neutral. Wording is
per chain kind (r4/P4-036): a well-formed chain projects `Patience:
chain <root> has <n> uncertified rounds (floor <m>) — certify landed
value or close the chain.`; an orphan singleton projects `Patience:
orphan job <id> has uncertified spend — certify landed value or flag
it to the human.` — the close offer is omitted because dispatch close
cannot resolve a broken lineage and this design refuses dispatch
changes; the persistent nag over an uncertifiable orphan is vocal
noise pointing at real damage, which is the system working. An
overflow annotation projects as `Patience: <count> more chains are
past their floors (see ledger).` after the detail lines (r3/P4-032).
Restart-deterministic: the prompt derives from the ledger, not from
runner memory. The
ask-candidate route stays dropped; candidates belong to the host's
return (F Q3.13).

**What expiry does not do.** Floors never kill, never park, never
feed the breaker (F Q5.2), never write fuse-visible lines. The fuses
remain the only actors (F Q1.23, F Q5.8). Escalation from vocal to
acting is a future human ruling taken with trial evidence.

## Non-goals

- No conf/local/env/per-dispatch patience surface of any kind.
- No wall-clock patience; rounds only.
- No new validated prompt section; no Ledger Tail grammar change.
- No change to fuse semantics or ledgerSemantics versions (F Q1.7),
  the breaker (F Q5.2), drain (F Q5.19, F Q5.20), adapters, or hosts.
- No critique-closure machinery; no return revalidation.
- No certification-hygiene adjudication (r2/P4-024) — recorded as a
  candidate future satellite.
- Deferred unless trial evidence demands: loop-advanced credits; an
  accepted certification of a closed critique round already lands
  through the one observable.

## Implementation sketch

internal/mission/ledger.go: the two Patience forms in
annotationWriteRe AND the read-side grammar (r2/P4-028).
internal/mission/contract.go: `patience.` allow-list prefix; entry
validation (identifier grammar, canonical model key, positive
integer); sealing in expectedSeal AND the ordered emitter with the
round-trip test (r1/P4-015). New internal/missionrunner/patience.go:
the pure count function (mission job set → chain sets via the
branch-tolerant parent walk vs jobId-joined accepted certifications;
terminal set = TerminalJobStatuses plus damaged-status records;
selection table; valid-jobId input boundary) →
(bounded annotations); called from the shared cycle-booking path.
internal/mission/prompt.go: This Turn assembly projects the final
cycle's Patience annotations into breach lines. No dispatch, adapter,
or host changes.

## Verification

Race-detector unit tests: chain-set counting (branches; late certification
heals; rejected ignored; cancelled counts; duplicate round numbers
harmless; foreign-jobId certifications ignored; damaged records
barren; orphans isolate; identity-less records excluded; sibling
ordering deterministic; threshold
strictly-exceeds; 19+1 overflow; selection fallback chain including
the null-effectiveModel and smallest-floor branches); contract
validation and the seal round-trip; ledger write→parse round-trip of
both forms; prompt projection from a ledger fixture. Mission
fixtures: a breached chain books the annotation and the NEXT prompt
carries the line; an unconfigured mission's turn artifacts are
byte-identical to today's; a heal booking with no new terminal rounds
advances nothing. Suite green via the standing launch recipe.
