# Patience itself: witnessed consumption and patience floors

Working Mode: design

Satellite 4 of the patience program (plans/stop-loss-satellites.md).
Regenerated whole after rounds 1, 2, and 9; amended in place after
rounds 3-8 (plans/dispositions/patience-satellite-4-r{1..9}.md; 62/62
accepted; convergence 15 → 13 → 4 → 4 → 4 → 6 → 4 → 5 → 7). Parent
ruling, inherited and not re-litigated: stop-loss is a last defense,
never a pacing target, recursively. Vocabulary per docs/patience.md
and docs/glossary.md: progress is mechanically proven value; patience
is tolerated observation without progress; slower progress is still
progress.

Facts: plans/patience-satellite-4-facts.md (cited as F Qn.m). Sheet
corrections on the record: local config is NOT cap-only (r1/P4-006);
the dispatch transition map is not the terminal-status authority
(r2/P4-022).

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
a NON-EMPTY evidence array (F Q3.22, F Q3.13, r9/P4-056).** The
design claims no more than this is: the orchestrator's durable,
accountable decision to consume the round. Patience bounds
UNWITNESSED SPEND — rounds paid for that no concluded turn has stood
behind. Whether certified work was truly valuable is the grader's and
a future certification-hygiene satellite's business (r2/P4-024);
patience never adjudicates certification content, only its existence,
verdict, evidence non-emptiness, and join.

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

**Certification join (r2/P4-024, r9/P4-056).** A certification counts
only when its jobId resolves inside the mission's own job set (the
missionJobs ownership authority: mission stamp plus fence
reservation), the job is a member of the chain under evaluation, the
verdict is `accepted`, and the evidence array is non-empty. Foreign
or nonexistent jobIds are ignored.

## The count — the current drought, not lifetime debt

**Patience count = the number of counted jobs strictly newer, in the
total job order, than the chain's newest witnessed job; when no job
is witnessed, all counted jobs (r9/P4-057).** Every certification
resets the streak, so steady alternating progress never breaches —
slower progress is still progress, enforced by arithmetic. Older
unwitnessed spend behind the newest witness stops counting toward
patience; it remains the Landed Returns section's nagging
jurisdiction.

**The chain set.** Records reachable from the root by the parentJob
walk, branch-tolerant: sibling follow-ups belong to the set and one
annotation per root stays well-defined (r3/P4-029). **The total job
order (r4/P4-034, r5/P4-038):** (endedAt, startedAt, jobId)
descending; timestamps parse as RFC3339, missing and unparseable
values sort oldest in one bucket, ties fall through to the next key,
jobId the final lexicographic tiebreak.

**A COUNTED job must have been real work (r9/P4-058).** Patience is
doctrine about who actually worked, and nobody worked in a setup
husk. A job counts when it is unwitnessed AND its record shows the
delegate started: terminal status completed, timeout, or cancelled
(missionrunner.TerminalJobStatuses, which includes cancelled —
r2/P4-022 — so cancellation cannot launder a STARTED round), or
failed with a recorded post-setup transition (handshake success or a
running phase). Pending-setup husks and launch/handshake failures
(abandoned-setup, handshake_timeout, launch_failed) are
dispatch-infrastructure noise with their own vocal channels — job
records, the watchdog — and leave the patience count entirely; this
supersedes the round-6 husk rows. The status rule stays three-way
(r3/P4-031, r6/P4-043): lawful nonterminal statuses (pending-setup,
pending, running) never count — that work is in flight; a status
missing or outside both vocabularies is damaged and counts IF the
record shows started work, fail-toward-vocal (fences act on money, so
losing sight must never finish a job; patience only speaks, so losing
sight must never hide a drought). Faults cannot launder rounds
(r1/P4-008): a round run under a rejected-envelope turn still counts
until witnessed; certifications join by jobId and round numbers play
no part (r2/P4-023).

**Evidence damage (r1/P4-009, r2/P4-020, r2/P4-021, r4/P4-035,
r5/P4-039).** The input set is the mission's own jobs whose recorded
jobId is valid under the job-id grammar AND equals the record's
filename stem — the on-disk identity certification resolution and
dispatch close actually address. Everything else (unreadable,
unattributable, identity-less, identity-mismatched) is outside
patience, in the janitor and usage jurisdictions: patience's remedies
are certify-by-jobId and close-by-chain, and neither can touch an
identity-less record. Within the input set, a record whose parent
walk cannot join a chain forms a single-round ORPHAN chain keyed by
its own jobId. **Orphans are floor-independent damage reports
(r7/P4-049):** a one-job chain can never exceed a positive floor, so
an unwitnessed STARTED orphan books its vocal line at every booking —
its broken lineage is itself the anomaly worth hearing about. Every
counted identifier is grammar-safe by construction.

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
sealed contract carries at least one patience entry. An unconfigured
mission's turn artifacts are byte-identical to today's structurally,
not by promise. Pre-feature contracts verify unchanged under a new
binary (no entries, none expected); the reverse direction is
unsupported by declaration (r2/P4-026): a pre-feature binary refuses
a patience-bearing contract through the existing unknown-key
preflight rule — loud refusal, never silent misbehavior. No conf
keys, no local keys, no environment override, no per-dispatch flag.
Non-mission chains have no runner evaluating them; the human at the
keyboard is their patience. Benchmark numbers ride kits and contracts
by construction (the 2026-08-11 boundary ruling).

**Floor selection (r2/P4-014, r2/P4-018, r2/P4-019, r3/P4-030,
r6/P4-041, r8/P4-054, r9/P4-060).** Matched on (role, runtime,
canonical model) — effective-model evidence, never the requested
model alone: patience is doctrine about who actually worked. MODEL
EVIDENCE means a value canonicalizing to a canonical model key FOR
THE RECORD'S RUNTIME (the cap-key test, F Q2.8); shipped sentinels
(`unobserved`, `multi-model:<names>`) are NOT model evidence, in
the table's own conditions, not only prose (r6/P4-041, r7/P4-048). **A QUALIFYING evidence record carries model evidence
AND a valid role AND a valid runtime, all on that one record
(r8/P4-054, r9/P4-060)** — the whole triple comes from one record,
no cross-record chimeras, and the newest qualifying record is used
before any fall-through. Floor selection applies to well-formed
chains only; orphans bypass floors (r7/P4-049). The table quantifies
over COUNTED jobs (r7/P4-050). Rows in order, first applicable wins (r4/P4-033):

| row | condition | floor |
| --- | --- | --- |
| 1 | some counted job qualifies as an evidence record: take the NEWEST; an exact (role, runtime, model) entry exists | that entry |
| 2 | same evidence; no entry for that triple | infinite — configured-nothing |
| 3 | no counted job qualifies; the chain root has valid role and runtime and its requestedModel IS model evidence; an exact entry exists | that entry |
| 4 | same as row 3; no entry for that triple | infinite — configured-nothing |
| 5 | neither source qualifies; the newest counted job with a valid runtime yields one, the root (else that job) a valid role; any (role, runtime, *) entries exist | the SMALLEST such floor — damage must never widen patience |
| 6 | same as row 5; no (role, runtime, *) entries | infinite |

Setup husks no longer reach selection at all (r9/P4-058 superseding
the round-6 husk rows of r6/P4-042): a chain whose only records are husks has no
counted jobs and no patience count.

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

**The crash and race contract (r2/P4-016, r2/P4-025).** The
pre-append read is the linearization point: booked counts are
counts-as-read. Two accepted windows, both one-booking lag in a
non-acting audit surface: a crash between AppendCycle and the state
write can leave one cycle's vocal line stale against the healed log,
and a reaper transition landing mid-write books one evaluation late.
A final booking cannot strand a stale line anywhere that matters — a
completed mission needs no nag; a parked mission's ask is the vocal
surface. No retry loop around the flocked append.

**Grammar — write AND read (r1/P4-013, r2/P4-028, r5/P4-037,
r7/P4-049).** Three annotation forms in BOTH annotationWriteRe and
the read-side grammar, with a parse round-trip test; the chain kind
lives in the durable bytes:

    - Patience: chain=<root> rounds=<n> floor=<m>
    - Patience: orphan=<id> rounds=<n>
    - Patience overflow: chains=<count>

**Bound (r2/P4-027, r8/P4-051).** The landed-returns bound covers ALL
Patience lines, breaches and orphans together: at most 20 lines per
booking — 20 detail, or 19 detail plus 1 overflow when the combined
set exceeds 20, the overflow count including both kinds. Ranking
(r5/P4-040, r7/P4-049): breach distance (count − floor) descending,
tiebreak count descending, then root ascending; orphans after all
breaches (distance zero), root ascending. Annotations remain audit
trail, never fuse input; the replay invariant (F Q1.4, F Q4.18) is
untouched.

**Vocal to whom (r1/P4-005, r2/P4-017, r9/P4-061).** Two surfaces,
one durable source. The ledger annotation is the human audit trail.
The orchestrator sees breaches in its NEXT prompt: at assembly time
the runner projects the final cycle block's Patience annotations
(readable because the read grammar round-trips) into `## This Turn`
lines — runner-authored free text, validator-neutral — FILTERED
against current chain-closed flags: a line whose chain root is now
closed is dropped from projection (park-then-close cannot strand a
stale warning; the ledger keeps its history; the prompt stays a pure
function of ledger plus chain-closed flags). Wording per kind
(r4/P4-036): `Patience: chain <root> has <n> unwitnessed rounds
(floor <m>) — certify landed value or close the chain.`; `Patience:
orphan job <id> has unwitnessed spend — certify landed value or flag
it to the human.` (no close offer: dispatch close cannot resolve a
broken lineage and this design refuses dispatch changes); overflow:
`Patience: <count> more chains need attention (see ledger).`
(r3/P4-032, r8/P4-051). The ask-candidate route stays dropped
(F Q3.13).

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
- No critique-closure machinery; no return revalidation (r2/P4-010
  dissolved with r1/P4-001: no schema is ever consulted).
- No certification-content adjudication (r2/P4-024, r9/P4-056):
  patience checks existence, verdict, non-empty evidence, and join —
  never whether the evidence is good. Certification hygiene is a
  candidate future satellite.
- Deferred unless trial evidence demands: loop-advanced credits; a
  witnessed certification of a closed critique round already lands
  through the one observable.

## Implementation sketch

internal/mission/ledger.go: all THREE Patience forms (chain, orphan,
overflow) in annotationWriteRe AND the read-side grammar (r6/P4-044).
internal/mission/contract.go: `patience.` allow-list prefix; entry
validation (identifier grammar, canonical model key, positive
integer); sealing in expectedSeal AND the ordered emitter with the
round-trip test (r1/P4-015). New internal/missionrunner/patience.go:
the pure derivation — configured gate (r9/P4-059); mission job set
bounded to valid jobId EQUAL to filename stem (r5/P4-039, r6/P4-045);
chain sets via the branch-tolerant parent walk; started-work counted
set (r9/P4-058); streak count against the newest witnessed job
(r9/P4-057); qualifying-evidence-record selection over the six-row
table (r9/P4-060); bounded ranked annotations — called from the
shared cycle-booking path so ordinary, faulted, and heal conclusions
all pass through it. internal/mission/prompt.go: This Turn assembly
projects the final cycle's Patience annotations filtered by
chain-closed flags (r9/P4-061). No dispatch, adapter, or host
changes.

## Verification

Race-detector unit tests: streak counting (certification resets the
streak — alternating witness/barren never breaches, r9/P4-057; no
witness counts all counted jobs; late certification of the newest
spend heals; rejected and EMPTY-EVIDENCE certifications ignored,
r9/P4-056; cancelled started-rounds count while setup husks and
handshake failures do not, r9/P4-058; lawful nonterminal statuses
never count while damaged-status started records do; duplicate round
numbers harmless; foreign-jobId certifications ignored; jobId-vs-
filename mismatches excluded; orphans isolate and emit despite
positive floors in configured missions only, r8/P4-052, r9/P4-059;
sibling ordering deterministic over damaged timestamps; threshold
strictly-exceeds; 19+1 overflow counting breaches AND orphans,
r8/P4-051; mixed ranking at the cutoff, r8/P4-052; breach-distance
ranking with unequal floors defeating a count-descending comparator,
r6/P4-046; the six-row table with sentinel models, damaged-status
jobs driving rows one and two, r8/P4-053, and invalid-role/runtime
records not qualifying, r9/P4-060); contract validation and the seal
round-trip; ledger write→parse round-trip of all THREE forms; prompt
projection from a ledger fixture including the chain-closed filter
(r9/P4-061). Mission fixtures: a breached chain books the annotation
and the NEXT prompt carries the line; an UNCONFIGURED mission's turn
artifacts are byte-identical to today's including in the presence of
orphans (r9/P4-059); a heal booking with no new counted jobs advances
nothing. Suite green via the standing launch recipe.
