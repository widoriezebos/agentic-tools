# custody-launch-machine — satellite 1 design (2026-08-27, v3 after round 2)

Working Mode: design

Satellite of the delegate-job-liveness custody arc (parent map:
plans/delegate-job-liveness-design.md; facts:
plans/delegate-job-liveness-facts.md as amended). SCOPE: Lane A
only. Inherited: RULINGS 1-6 and the parent's converged ground —
with ONE recorded refinement: the parent's D-B phrase "reservation,
caps, and launch run under the cap-authority lock" is imprecise
against the shipped implementation (dispatch releases the lock
before launch, dispatch.sh:1187); the satellite refines it to
"single cap AUTHORITY, bounded lock hold" (S1R2-05) and the parent
map carries this correction.

Routed findings: R4-01/02/07/08/12. ROUND 1: 11 material, folded.
ROUND 2: 11 material — flat; roughly half arose from round-1
folds' own edges (the loop-critiquing-itself signal is noted).
All folded into this v3. ROUND 3 IS THE DECLARED FAILSAFE: it
closes on all-fixture-expressible, and if material findings remain
past it, the first exhaustion is recorded and the
fixtures-as-arbiter exit is weighed against the trajectory.

Round budget: 3 on chain `custody-launch-machine`; failsafe 3.

Threat model: accidents, crashes, drift; recycled pids AND pgids;
clock steps; two trusted operators; hostile processes out of scope.

## Resolutions

### R-A Presence is not progress; the qualifying stream is the child's own (R4-01, S1R1-09, S1R2-11)

- PRESENCE (supervisor heartbeats, supervisor log appends): never
  advances freshness.
- PROGRESS: growth of the child's own stdout — the adapter's events
  file (`<prompt >events 2>>log`); product mtimes only under R-D.
The artificial log-touch is removed. WATERMARK LAW (S1R2-11): the
high-water mark NEVER resets — after size 100 → 0 → 10, there is
no progress until size exceeds 100; truncation is an anomaly
detail. The sweep (S2) owns watermark state; this satellite records
the qualifying stream path (`outputStream`, immutable at launch).

### R-B claim-launch is total and fingerprinted (R4-02, S1R1-05/10/11, S1R2-05/06/07/09/10)

THE FINGERPRINT (versioned: `fingerprintVersion: 1` stored beside
the digest — an encoder change bumps the version and never
retro-compares across versions, S1R2-10) covers the whole
process-creating request: sha256 over the v1 canonical encoding of
(session key; DISPATCH MODE fresh|follow-up; the resumed runtime
session id when follow-up (S1R2-07); runtime; canonical model key;
role; launch mode worktree|shared-checkout; permission envelope
digest; declared product roots — resolved via the missing-tail-
aware resolver (fspath.go:18 precedent), relative roots based at
the GIT ROOT, sorted, deduplicated; cap request with defaults
explicit; input hash). A root resolving into the exclusion set
refuses.

Resolution order: SAME-OPID first (fingerprint equality or
REFUSED-OPID-MISMATCH; legacy no-fingerprint →
REFUSED-UNPROVABLE-LEGACY; live+verified → BOUND; terminal →
REPLAYED-<status>; dead-proven nonterminal → RECONCILING;
identityless reservation → IN-PROGRESS), then the BUSY GATE
(process-creating ops only), then WON.

WAIT BOUNDS WITHOUT WALL CLOCKS (S1R2-09): the IN-PROGRESS wait is
bounded by the WAITER'S OWN loop (N retries × interval — counted
iterations, immune to clock steps); the record's
`reservationDeadline` (createdAt + AbandonedSetupGrace) stays as
the ADVISORY cross-process signal, and reconciliation may also be
entered by any waiter whose own loop expires. A backward clock
step can delay the advisory signal, never the waiter's bound.

LOCKS AND BOUNDS (S1R1-03/10, S1R2-05/06): one order for every
process-creating reservation — chain lock (where chained) first,
then the cap-authority lock; INSIDE the cap lock: the same-opid
read, the busy check, and husk creation only — no kernel reads, no
waits, no adapter work; launch happens AFTER release (the
refinement above; single cap authority is preserved because every
cap decision still happens under the lock). THE BUSY CHECK IS
O(SESSION), NOT O(REGISTRY) (S1R2-06): a per-session OCCUPANCY
INDEX (`artifacts/agents/sessions/<key>.json`) is maintained
transactionally with reservation creation and terminal stamping
(same flock discipline as the record lock); the busy gate reads
the index; a missing or stale index entry falls back to the full
scan ONCE and rebuilds the entry (self-healing, labeled in the
launch record). Both existing reservation call sites migrate.

### R-C Identity: sandwich with a total, ordered table; tag-POSITION proof (R4-12, S1R1-06/07, S1R2-03/04)

The verification table is ORDERED — first matching row wins
(S1R2-04):
1. Either start read UNKNOWN (EPERM etc.) → INDETERMINATE.
2. Second read finds the pid GONE (definitive ESRCH) → DEAD
   candidate.
3. start2 ≠ start1 → INDETERMINATE (recycled mid-read).
4. argv unreadable (start reads consistent) → INDETERMINATE for
   binding; presence-only for liveness.
5. argv readable, tag NOT in tag position → NOT-OURS.
6. All consistent + tag in position → VERIFIED.
TAG-POSITION PROOF (S1R2-03): the mechanism is
`janitor.MatchShape`; the build ADDS the adapter invocation shapes
(supervisor and CLI wrapper) to the shipped shape set, and
MIGRATES the kill path's substring proof (`wind_down_group`,
dispatch.sh:273) to the same matcher — the substring arm is
retired. `rg <tag>` never satisfies any custody, adoption, or kill
predicate.

### R-D Product signals: isolated AND contained (R4-07, S1R1-04, S1R2-08)

Product freshness is liveness evidence ONLY when the roots are
job-exclusive by construction: worktree mode with every declared
root CONTAINED in the worktree (containment checked after
resolution; an outside root demotes to attribution-only and the
launch record says so). Shared-checkout mode: never liveness;
declared roots are attribution reporting only. The exclusion set
binds every scan; scans read file mtimes recursively, max-reduced,
event time.

### R-E Generations live in the TAG; evidence is written in advance (R4-08, S1R1-01/02/08, S1R2-01/02)

The instance tag carries a per-reservation nonce:
`metasystem-job-<job>-<opid-suffix>`. Two mechanisms complete it:

- THE PRE-FORK MARKER (S1R2-01): before forking the CLI child, the
  supervisor writes `artifacts/agents/prefork/<tag>` naming the
  intended pgid; custody-add removes it. An untagged, unregistered
  in-group survivor is OURS-SUSPECT (indeterminacy, defer) ONLY
  while the marker exists; without a marker, an untagged in-group
  survivor is FOREIGN (the recycled-pgid case) — death and
  occupancy resolve. The indistinguishable-evidence hole closes by
  evidence written before the ambiguity can arise; the marker file
  is machine-local operational state with the same lifecycle
  discipline as heartbeats.
- NONCE-GLOBAL ADOPTION (S1R2-02): because the tag is
  reservation-unique, ANY process bearing it in tag position is
  ours by construction. Adoption for an IDENTITYLESS reservation
  scans globally by tag (a complete census), adopts the
  leader-most nonce-tagged process, and custody-adds every other
  nonce-tagged process REGARDLESS of group (the round-1
  cross-PGID-is-incident rule is retired — it was wrong under
  nonce uniqueness). Once a pgid is recorded, in-group scoping
  applies for death; nonce-tagged processes outside it are still
  custody-added on sight.
- DEATH: proven when the recorded pgid has no live members, or
  every live member is proven foreign (identity says not-ours AND
  no pre-fork marker stands). Unproven in-group members defer.

## Obligations (fixture-expressible)

- S1-O1 Presence and supervisor appends never advance freshness;
  events growth does; high-water never resets (100→0→10 = no
  progress); the artificial log-touch is gone.
- S1-O2 claim-launch totality: one fixture per outcome incl.
  fingerprint mismatch, legacy arm, dead-nonterminal,
  follow-up-vs-fresh mode digest difference, and resumed-session
  digest difference.
- S1-O3 Atomicity under the named lock order at BOTH call sites;
  two distinct-opid claimants → one WON; two same-opid → one
  process, loser BOUND/IN-PROGRESS (never `chain is busy`).
- S1-O4 An identityless husk reconciles via nonce-global adoption
  (adopt + custody-add across groups), fails only on
  complete-census absence, defers on incompleteness; the waiter's
  own loop bound triggers reconciliation under a backward clock
  step (advisory deadline frozen).
- S1-O5 The ordered R-C table: one fixture per row on both
  platforms; `rg <tag>` (even as group leader) fails adoption,
  custody, AND the migrated kill predicate.
- S1-O6 Marker semantics: supervisor death in the fork window
  (marker standing, child untagged) → defer, never death, session
  busy; recycled pgid (no marker) with untagged foreign survivors
  → death resolves and the session frees; marker removed at
  custody-add; a stale marker past its job's terminal state is
  swept with the record's lifecycle.
- S1-O7 Containment: a worktree root outside the worktree demotes
  to attribution-only (record says so); shared-checkout jobs never
  read product freshness; exclusion-set refusal after symlink
  resolution.
- S1-O8 Fingerprint canonicalization: reordered/duplicate/relative/
  symlinked/missing-tail roots hash stably; version bump never
  retro-compares; model aliases canonicalize; explicit-vs-default
  caps encode identically.
- S1-O9 Bounded lock phase: a fixture holds a slow registry and
  proves the busy gate reads the occupancy index in O(session);
  index self-heals from a stale entry with the labeled fallback;
  dispatch/arming acquisition stays within its 10s bound.

## ROUND 3 (FAILSAFE): 11 material — FLAT AT 11-11-11; FIRST BUDGET EXHAUSTED; THE CRITIC DECLARED STOP-LOSS

The failsafe left seven shape-level findings (S1R3-01..07: cross-
group custody closure, the marker's own crash orders, index
transactionality, self-healing re-unbounding the lock, census
completeness shape, forward-clock-step deadline abuse, missing-tail
containment drift) and four fixture-expressible (S1R3-08..11).
The fixtures-as-arbiter exit is UNAVAILABLE by its own conditions
(trajectory not falling, findings not all mechanical). Open set
enumerated: S1R3-01 through S1R3-11. Per the stop-loss precedent
and the implementation-first ruling (D81: two prose budgets
exhausted across this arc's core — the parent chain's and this
satellite's — means BUILD BEHIND FIXTURES, never a third prose
budget), the coordinator recommended the implementation-first exit.

**WIDO RULED (2026-08-27 late evening, in-session): IMPLEMENTATION-
FIRST (RULING 7).** v3 plus all eleven open findings convert to the
build brief at plans/custody-launch-machine-build-brief.md — every
finding a named failing-first fixture obligation, shape choices
resolved in code, MANDATORY code-critique as the arbiter, ~10h of
codex passes timeboxed, starting the next morning. No further prose
round runs on this chain; the design record freezes as the build's
source of truth alongside the round-3 verdict.

## Round 1 dispositions (r1-output.md)

| Finding id | Disposition | Amendment |
| --- | --- | --- |
| S1R1-01 | accepted | superseded by v3 R-E marker law (S1R2-01) |
| S1R1-02 | accepted | floor deleted; nonce-tag generations |
| S1R1-03 | accepted | named lock order; both call sites migrate |
| S1R1-04 | accepted | products liveness only in isolated workspaces |
| S1R1-05 | accepted | full fingerprint tuple (extended again in v3) |
| S1R1-06 | accepted | tag-position proof (mechanism corrected in v3: MatchShape + shapes + kill-path migration) |
| S1R1-07 | accepted | R-C table (made ordered/total in v3) |
| S1R1-08 | accepted | adoption totality (re-scoped in v3: nonce-global) |
| S1R1-09 | accepted | events-file-only stream; watermark to S2 (truncation law pinned in v3) |
| S1R1-10 | accepted | bounded inside-lock phase (index-backed in v3) |
| S1R1-11 | accepted | canonicalization rules (versioned in v3) |

## Round 2 dispositions (r2-output.md)

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| S1R2-01 | accepted | Indistinguishable evidence demanded opposite verdicts. | R-E pre-fork marker: advance-written evidence distinguishes them; S1-O6 |
| S1R2-02 | accepted | PGID scoping cannot adopt an identityless reservation; the nonce makes global adoption sound. | R-E nonce-global adoption; cross-PGID incident rule retired; S1-O4 |
| S1R2-03 | accepted | signature-check has no tag arm; MatchShape lacks adapter shapes; wind_down_group is substring. | R-C names MatchShape + new shapes + kill-path migration; S1-O5 |
| S1R2-04 | accepted | Second-read UNKNOWN unassigned; two rows overlapped. | R-C table made ordered and total; S1-O5 |
| S1R2-05 | accepted | The parent's D-B phrase contradicted the bounded phase and the shipped release-before-launch. | Inheritance refinement recorded at the top; parent map carries the correction |
| S1R2-06 | accepted | "Non-terminal only" still reads the whole registry under the lock. | Per-session occupancy index, transactional, self-healing; S1-O9 |
| S1R2-07 | accepted | Mode and resumed session change the command under an equal digest. | Fingerprint gains dispatch mode + resumed session id; S1-O2 |
| S1R2-08 | accepted | A worktree job could declare an outside shared root. | R-D containment; outside roots demote to attribution-only; S1-O7 |
| S1R2-09 | accepted | Wall-clock deadlines step with the clock. | Waiter-loop bounds (counted iterations); advisory deadline retained; S1-O4 |
| S1R2-10 | accepted | Base for relatives, encoder stability, missing-tail behavior undefined. | Versioned v1 encoding; git-root base; missing-tail resolver; S1-O8 |
| S1R2-11 | accepted | Two watermark readings both honored the prose. | High-water never resets; S1-O1 |
