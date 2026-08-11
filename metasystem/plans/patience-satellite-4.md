# Patience itself: witnessed consumption and patience floors

Working Mode: design

Satellite 4 of the patience program (plans/stop-loss-satellites.md).
Regenerated whole after rounds 1, 2, 9, and 10; amended in place
after rounds 3-8 and 11-14
(plans/dispositions/patience-satellite-4-r{1..14}.md; 79/79
accepted). Round 10 named the loop's generating cause — the
damage-tolerance rule surface — and superseded it: patience evaluates
clean evidence only; round 11 closed the one silence that left (the
aggregate excluded-records line, since the janitor is not yet
wired). Parent ruling, inherited and not re-litigated:
stop-loss is a last defense, never a pacing target, recursively.
Vocabulary per docs/patience.md and docs/glossary.md.

Facts: plans/patience-satellite-4-facts.md (cited as F Qn.m). Sheet
corrections on the record: local config is NOT cap-only (r1/P4-006);
the dispatch transition map is not the terminal-status authority
(r2/P4-022); certification evidence is a string, not an array
(r10/P4-063).

## The gap this satellite closes

Every existing bound is per-round wall time or per-mission cycle
accounting: the job cap bounds one round (F Q5.3, F Q5.4), the host
turn cap one turn (F Q5.10, F Q5.11), the fuse gainless cycles
(F Q1.5, F Q1.6), the fences totals (F Q5.8). Nothing bounds a
delegate CHAIN burning round after capped round — each schema-valid
(evidence may be `inferred`, F Q3.10) — without the orchestrator ever
recording that any of it was worth consuming. That drought is what
patience floors measure.

## The observable — witnessed consumption, said honestly

**A chain round is WITNESSED exactly when a concluded turn's durable
log certifies that round's job, by jobId, with verdict `accepted` and
a non-empty trimmed evidence STRING (F Q3.22, F Q3.13, r9/P4-056,
r10/P4-063).** The design claims no more than this is: the
orchestrator's durable, accountable decision to consume the round.
Patience bounds UNWITNESSED SPEND — rounds paid for that no concluded
turn has stood behind. Whether certified work was truly valuable is
the grader's and a future certification-hygiene satellite's business
(r2/P4-024); patience never adjudicates certification content, only
its existence, verdict, evidence non-emptiness, and join.

Nothing else counts:

- Return validation is NOT witness: it proves protocol, not
  consumption (internal/validate/returncomplete.go checks schema and
  identity only); counting it would reset the drought this satellite
  exists to catch (r1/P4-001).
- Certifications with verdict `rejected`, or with empty evidence, are
  NOT witness (r1/P4-003, r9/P4-056).
- Chain close is NOT witness: a closed chain leaves the evaluation
  set — ending the spend is itself a recorded decision — its
  annotation history remaining in the ledger. Unconsumed landed value
  stays the Landed Returns section's jurisdiction (F Q3.31).
- Critique closure is out of scope (r1/P4-002).

**Certification join (r2/P4-024, r9/P4-056, r11/P4-071).** A
certification counts only when its jobId resolves inside the
mission's own job set (the missionJobs ownership authority: mission
stamp plus fence reservation), the job is a member of the chain under
evaluation, the job is itself PARTICIPATING and STARTED (any terminal
status — certifying a husk or a never-started failure is ignored and
cannot reset a drought), the verdict is `accepted`, and the evidence
string is non-empty after trimming. Foreign or nonexistent jobIds are
ignored.

## Clean evidence only (r10, superseding the damage surface)

Patience double-covered damage for seven rounds and the loop kept
finding the seams. Superseded (r10 dispositions, with reasons):
patience now evaluates ONLY clean records. A record PARTICIPATES when
it is readable, mission-owned (missionJobs authority), identity-sound
(jobId valid under the job-id grammar AND equal to the filename stem,
r5/P4-039), and its status is in a KNOWN vocabulary — terminal
(missionrunner.TerminalJobStatuses, cancelled included, r2/P4-022) or
lawful-nonterminal (pending-setup, pending, running). Everything else
— unreadable bytes, unknown or missing statuses, identity mismatches,
unattributable records — is OUTSIDE patience: patience's remedies
(certify-by-jobId, close-by-chain) cannot act on damaged records, and
a nag without a possible act is noise. Because the janitor is not yet
wired (U1, plans/go-production-grade.md), exclusion is not silence:
the aggregate excluded-count line (r11/P4-069, grammar below) books
the fact and hands it to the human; per-record reporting stays with
the jurisdictions that own damage.

**A COUNTED job is a participating job that is unwitnessed, terminal,
and provably STARTED (r9/P4-058, r10/P4-066, r11/P4-072).** Started
is defined over shipped record fields only: status `completed` or
`timeout` proves it structurally — the lifecycle CAS map reaches
those statuses only through `running` (F Q3.3) — and a `cancelled` or
`failed` record is started exactly when it carries RECORDED USAGE —
money provably spent is work, whatever the error is called
(r14/P4-078: the same handshake error names both a pre-running
rejection and a post-run session mismatch after usage capture) — or,
lacking usage, a non-empty effectiveModel AND an error outside the
NEVER-STARTED vocabulary: abandoned-setup, handshake_timeout,
launch_failed, and the handshake-mismatch error (HandshakeEval
patches effectiveModel before deciding failure, so the model field
alone cannot prove work — r12/P4-073, r13/P4-076;
sessionEstablishedSignal plays no part). The never-started vocabulary
is one table in patience.go, documented against dispatch's error
writers and enumerated by a verification test, so writer drift fails
loudly. A pending-cancelled husk
and launch/handshake failures (abandoned-setup, handshake_timeout,
launch_failed) satisfy none of these and never count: nobody worked,
so no one's patience is debited. Lawful-nonterminal jobs never
count — that work is in flight.

**The chain set and order.** Records reachable from the root by the
parentJob walk, branch-tolerant (r3/P4-029); a participating record
whose parent walk cannot join a chain forms a single-round ORPHAN
chain keyed by its own jobId. **Total order (r4/P4-034, r5/P4-038):**
(endedAt, startedAt, jobId) descending; RFC3339 parsing, missing and
unparseable timestamps sort oldest in one bucket, ties fall through,
jobId final tiebreak. (With only clean records participating, damaged
timestamps are rare; the rule remains for totality.)

**The count = the current drought (r9/P4-057).** The number of
counted jobs strictly newer, in the total order, than the chain's
newest witnessed job; with no witnessed job, all counted jobs. Every
certification resets the streak — steady alternating progress never
breaches; slower progress is still progress, by arithmetic. Older
unwitnessed spend behind the newest witness stops counting toward
patience and remains the Landed Returns section's nagging
jurisdiction. Faults cannot launder rounds (r1/P4-008): a round run
under a rejected-envelope turn still counts until witnessed; round
numbers play no part (r2/P4-023).

**Orphans are floor-independent damage reports (r7/P4-049):** their
records are clean, only their ancestry is broken. A one-job chain can
never exceed a positive floor, so an unwitnessed started orphan books
its vocal line at every booking in a patience-configured mission.

## Floors: sealed mission-contract entries, nowhere else

Round 1 killed the conf layer with evidence (r1/P4-006: the local
file resolves any key with precedence and environment outranks it;
r1/P4-007: an unsealed fallback lets a repository edit change a
signed mission). Floors exist only as mission-contract entries:

    patience.rounds.<role>.<runtime>.<model>=<positive integer>

validated by a `patience.` prefix in the contract allow-list
(F Q2.17): role and runtime in the identifier grammar, model a
CANONICAL model key in the cap-key encoding (config/model.go — e.g.
gpt-5.6-sol is keyed gpt-5-6-sol; r2/P4-018), value a positive
integer. Entries seal beside the cap entries in BOTH enumeration
surfaces — expectedSeal and the ordered emitter — with a
seal-then-preflight round-trip test (r1/P4-015, F Q2.22).

**Patience runs only where it is configured (r9/P4-059).** The whole
evaluation — floor breaches AND orphan reports — runs only when the
sealed contract carries at least one patience entry; the unconfigured
case is byte-identical structurally. Pre-feature contracts verify
unchanged under a new binary; the reverse direction is unsupported by
declaration (r2/P4-026): a pre-feature binary refuses a
patience-bearing contract through the existing unknown-key preflight
rule — loud refusal, never silent misbehavior. No conf keys, no local
keys, no environment override, no per-dispatch flag. Non-mission
chains have no runner evaluating them; the human at the keyboard is
their patience. Benchmark numbers ride kits and contracts by
construction (the 2026-08-11 boundary ruling).

**Floor selection (r2/P4-014, r2/P4-018, r2/P4-019, r3/P4-030,
r6/P4-041, r8/P4-054, r9/P4-060, r10/P4-065, r10/P4-067).** Matched
on (role, runtime, canonical model) — effective-model evidence, never
the requested model alone: patience is doctrine about who actually
worked. MODEL EVIDENCE means a value canonicalizing to a canonical
model key FOR THE RECORD'S RUNTIME (the cap-key test, F Q2.8);
shipped sentinels (`unobserved`, `multi-model:<names>`) are NOT model
evidence (r6/P4-041, r7/P4-048). **A QUALIFYING evidence record
carries model evidence AND a valid role AND a valid runtime, all on
that one record** — one record, one triple, no cross-record chimeras
(r8/P4-054); the newest qualifying record wins (r9/P4-060). Selection
quantifies over the STREAK set only — the counted jobs newer than the
newest witness — never pre-witness history (r10/P4-065). Orphans
bypass floors (r7/P4-049). Rows in order, first applicable wins
(r4/P4-033):

| row | condition | floor |
| --- | --- | --- |
| 1 | some streak job qualifies as an evidence record (effectiveModel evidence, valid role and runtime, one record): take the NEWEST | the exact (role, runtime, model) entry, if present |
| 2 | same evidence; no entry for that triple | infinite — configured-nothing |
| 3 | no streak job QUALIFIES as an effective-model evidence record (the r8/P4-054 absence rule, r12/P4-074); the newest STREAK job whose requestedModel IS model evidence, with valid role and runtime on that same record (r11/P4-070 — the pre-witness root is never consulted) | the exact entry, if present |
| 4 | same as row 3; no entry for that triple | infinite — configured-nothing |
| 5 | nothing in the streak qualifies | infinite — the streak carries no model identity, and configured-nothing is the honest verdict; the spend stays visible through Landed Returns and the usage ledger |

The round-6 husk rows and the round-8 damage-fallback rows are
deleted with the damage surface (r10 dispositions): a chain whose
only records are husks has no counted jobs and no patience count.

**Threshold (r1/P4-011).** A floor of F tolerates exactly F
unwitnessed counted jobs silently; the breach books when the count
strictly exceeds F.

**No wall-time claim (r1/P4-012).** Rounds-floors bound spend-shaped
drought; wall fences bound time; neither implies the other.

## Booking: one write, bounded, with an honest crash contract

**When.** In a patience-configured mission, evaluation runs at EVERY
cycle booking — ordinary, faulted, and heal (F Q4.7, F Q4.10,
F Q4.17); no exemptions (r1/P4-008). A booking with no new counted
jobs advances no count by construction.

**How.** The derivation reads the mission's job records, the durable
turn log, and the in-flight TurnConclusion (its qualifying
certifications reset streaks before anything is written), and its
annotations ride the SAME AppendCycle call as the cycle line
(F Q1.16, the faulted-path pattern F Q4.10).

**The crash and race contract (r2/P4-016, r2/P4-025, r10/P4-068).**
The pre-append read is the linearization point: booked counts are
counts-as-read. Accepted one-booking-lag windows in a non-acting
audit surface: a crash between AppendCycle and the state write can
leave one cycle's vocal line stale against the healed log; a reaper
transition landing mid-write books one evaluation late; and the
overflow line's count can go stale against chains closed after
booking. A final booking cannot strand a stale line anywhere that goes
UNSEEN — a completed mission needs no nag; a host-failure or
stop-loss park carries its ask; and the all-streams-inactive park,
which shipped code parks WITHOUT an ask, is itself visible through
the standing open-work reporting surface, which is where its
Patience lines wait for the human too (r14/P4-079, superseding
r13/P4-077's invented park ask — no park-path change of any kind,
which also keeps that path byte-identical). No retry loop around the
flocked append.

**Grammar — write AND read (r1/P4-013, r2/P4-028, r5/P4-037,
r7/P4-049).** Four annotation forms in BOTH annotationWriteRe and
the read-side grammar, with a parse round-trip test; the chain kind
lives in the durable bytes:

    - Patience: chain=<root> rounds=<n> floor=<m>
    - Patience: orphan=<id> rounds=<n>
    - Patience: excluded=<count>
    - Patience overflow: chains=<count>

The excluded form (r11/P4-069, scoped by r12/P4-075) is the
aggregate voice for the records patience can honestly attribute:
mission-owned READABLE records that missionJobs yields and the
participation boundary rejects (identity mismatch, unknown status).
Terminal identity damage in that set is voiced by no wired
jurisdiction today (the janitor is unwired — U1 in
plans/go-production-grade.md), so a patience-configured mission books
the count — no identities, no floors, no taxonomy — and hands the
matter to the human. Fully unreadable or unattributable records
cannot be charged to any mission and stay outside entirely:
machine-global damage, the janitor's when it is wired.

**Bound (r2/P4-027, r8/P4-051).** The landed-returns bound covers ALL
Patience lines, breaches and orphans together: at most 20 lines per
booking — 20 detail, or 19 detail plus 1 overflow when the combined
set exceeds 20, the overflow count including both kinds. Ranking
(r5/P4-040, r7/P4-049): breach distance (count − floor) descending,
tiebreak count descending, then root ascending; orphans after all
breaches (distance zero), root ascending. Annotations remain audit
trail, never fuse input; the replay invariant (F Q1.4, F Q4.18) is
untouched.

**Vocal to whom (r1/P4-005, r2/P4-017, r9/P4-061, r10/P4-068).** Two
surfaces, one durable source. The ledger annotation is the human
audit trail. The orchestrator sees breaches in its NEXT prompt: at
assembly time the runner projects the final cycle block's Patience
annotations (readable because the read grammar round-trips) into
`## This Turn` lines — runner-authored free text, validator-neutral —
FILTERED against current chain-closed flags for DETAIL lines (their
identities are in the durable bytes; a closed chain's line is dropped
from projection, so park-then-close cannot strand a stale warning).
The overflow line is exempt from the filter by design: it names no
chains, only a count pointing at the ledger, and its staleness joins
the crash-contract lags. Wording per kind (r4/P4-036): `Patience:
chain <root> has <n> unwitnessed rounds (floor <m>) — certify landed
value or close the chain.`; `Patience: orphan job <id> has
unwitnessed spend — certify landed value or flag it to the human.`
(no close offer: dispatch close cannot resolve a broken lineage and
this design refuses dispatch changes); `Patience: <count> record(s)
excluded from patience — flag it to the human.` (r11/P4-069, exempt
from the chain-closed filter like the overflow: it names no chains);
`Patience: <count> more chains need attention (see ledger).`
(r3/P4-032, r8/P4-051). The ask-candidate route stays dropped
(F Q3.13). The excluded line sits outside the 20-line detail bound:
it is at most one line per booking.

**What expiry does not do.** Floors never kill, never park, never
feed the breaker (F Q5.2), never write fuse-visible lines. The fuses
remain the only actors (F Q1.23, F Q5.8). Escalation from vocal to
acting is a future human ruling taken with trial evidence.

## Non-goals

- No conf/local/env/per-dispatch patience surface of any kind.
- No wall-clock patience; rounds only.
- No new validated prompt section; no Ledger Tail grammar change.
- No change to fuse semantics or ledgerSemantics versions (F Q1.7),
  the breaker (F Q5.2), drain (F Q5.19, F Q5.20), adapters, hosts, or
  dispatch.
- No damage coverage: damaged records belong to the janitor,
  watchdog, and usage jurisdictions (r10, superseding r3/P4-031's
  fail-toward-vocal rule with reasons on the r10 record).
- No critique-closure machinery; no return revalidation (r2/P4-010
  dissolved with r1/P4-001).
- No certification-content adjudication (r2/P4-024, r9/P4-056):
  existence, verdict, non-empty evidence, join — never quality.
  Certification hygiene is a candidate future satellite.
- Deferred unless trial evidence demands: loop-advanced credits.

## Implementation sketch

internal/mission/ledger.go: all FOUR Patience forms in
annotationWriteRe AND the read-side grammar (r6/P4-044).
internal/mission/contract.go: `patience.` allow-list prefix; entry
validation; sealing in expectedSeal AND the ordered emitter with the
round-trip test (r1/P4-015). New internal/missionrunner/patience.go:
the pure derivation — configured gate (r9/P4-059); participation
boundary (clean records only: readable, mission-owned, jobId equal to
filename stem, known status vocabulary — r10); chain sets via the
branch-tolerant parent walk; counted set (unwitnessed, terminal,
provably started over shipped fields — structural for
completed/timeout, witnessed fields for cancelled/failed,
r11/P4-072); streak count
against the newest witnessed job (r9/P4-057); streak-scoped
qualifying-evidence selection over the five-row table (r10/P4-065,
r10/P4-067); bounded ranked annotations — called from the shared
cycle-booking path. internal/mission/prompt.go: This Turn assembly
projects the final cycle's Patience annotations, detail lines
filtered by chain-closed flags, overflow exempt (r9/P4-061,
r10/P4-068). No dispatch, adapter, or host changes.

## Verification

Race-detector unit tests: participation boundary (damaged statuses,
identity mismatches, and unreadable records EXCLUDED — jurisdiction,
not tolerance, r10; husks and pending-cancelled never count,
r10/P4-066); streak counting (certification resets — alternating
witness/barren never breaches, r9/P4-057; no witness counts all
counted jobs; rejected and empty-evidence certifications ignored with
evidence as a STRING, r10/P4-063; cancelled started-rounds count;
foreign-jobId certifications ignored; orphans isolate and emit
despite positive floors in configured missions only, r8/P4-052,
r9/P4-059; ordering deterministic; threshold strictly-exceeds); the
five-row selection table (sentinel models excluded; invalid-role/
runtime records not qualifying, r9/P4-060; streak-scoped
quantification — pre-witness history selects nothing, r10/P4-065);
bound and ranking (19+1 overflow counting breaches AND orphans,
r8/P4-051; mixed ranking at the cutoff, r8/P4-052; breach-distance
ranking with unequal floors defeating a count-descending comparator,
r6/P4-046); contract validation and the seal round-trip; ledger
write→parse round-trip of all FOUR forms; prompt projection from a
ledger fixture including the chain-closed detail filter and the
exempt overflow (r9/P4-061, r10/P4-068). Mission fixtures: a breached
chain books the annotation and the NEXT prompt carries the line; an
UNCONFIGURED mission's turn artifacts are byte-identical to today's
including in the presence of orphans (r9/P4-059); a mission-owned
readable record with an identity mismatch books the excluded line
while an unreadable foreign record books nothing (r12/P4-075); a
pending-cancelled Codex job with sessionEstablishedSignal true never
counts and its certification never resets a drought (r12/P4-073); a
heal booking with no new counted jobs advances nothing. Suite green via the standing
launch recipe.
