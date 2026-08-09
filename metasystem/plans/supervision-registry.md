# Supervision registry contract

- Goal and current status: the single machine-wide custody view for
  supervision owners. Extracted from `plans/supervision-lifecycle.md`
  after critique round 3; revised against round 4 (locking, reduction,
  custody, corruption), round 5 (reservation slots, guarded appends,
  framing, kill proof, growth), round 6 (slot accounting until
  closure, gate-resolved orphans, the owner invocation shape, gating
  writes, run-tolerant torn repair, terminal-path reuse), round 7
  (guarded custodied arming, lock crash recovery, the newline edge,
  generation pairing, unseen-tag uniqueness, the terminated reason —
  SLC-R7-002..004, SLC-R7-007..009 folded here), round 8 (rename-born
  lock acquisition, `retiredThrough`, the triple kill proof —
  SLC-R8-001/002/006 folded here), and round 9 (the contiguous
  retirement watermark, the `shutdown-escalated` reason, binding
  records surviving compaction — SLC-R9-003/004/006 folded here),
  round 10 (compaction retains a bound custody's claim SKELETON
  including its terminal — SLC-R10-001), rounds 11-12 (the
  no-claim-append scope, arming-failure closure, custody release
  duties — SLC-R11-004..006, SLC-R12-003/004 folded here), and
  round 13 (armed before the lock replacement, custodyId
  uniqueness — SLC-R13-003/004 folded here). The chain is CLOSED at
  the cap. Being implemented in GO: internal/registry (REG-1..3),
  internal/lock (REG-4).
- Next step: none
  (the chain is CLOSED at the cap — see the lifecycle plan's close
  record; implementation is live: internal/registry implements
  REG-1..3 with 91.8% coverage, internal/lock implements REG-4.)
- In flight right now: nothing in this checkout — implementation
  proceeds in the orchestrator session.
- Waiting on the human: nothing.

## REG-1. Location, framing, and canonical paths

`~/.metasystem/armed-checkouts.jsonl` — one file per user per machine,
outside every checkout, so an owner whose checkout vanished still has
somewhere to speak.

FRAMING IS THIS FILE'S OWN, not the flight recorder's (SLC-R5-014 —
the recorder's leading-newline framing would turn every tolerated torn
tail into fatal mid-file corruption at the next append): one JSON
object per line, TRAILING newline. Every append happens under the
registry lock (REG-4) and first inspects the tail. The repair
condition is TWO-PART (SLC-R7-004): if the file does not END WITH A
NEWLINE BYTE and its final line IS valid JSON, the record was fully
written and only its newline was lost — the writer appends the
newline alone, completing it. If the final line is NOT valid JSON,
the writer ensures newline termination, appends a `torn` marker
record, then its payload. Testing only for valid JSON would let the
next append concatenate two objects into fatal mid-file corruption.
THE REPAIR IS ITSELF CRASH-RECOVERABLE (SLC-R6-009), so the tolerance
rule is stated over runs, not single lines: a non-JSON line is
TOLERATED iff every valid record after it is separated from it by a
`torn` marker — which makes all TRAILING garbage tolerated, however
the repair was interrupted, and makes the repair idempotent (a re-run
just adds another marker). Garbage followed by a valid record with no
intervening marker is CORRUPTION (REG-5).

CANONICAL PATHS (SLC-R4-014): every `checkoutPath` is the PHYSICAL
path (`realpath`) of the git top-level directory — the same
resolution the armer already performs — canonicalized AT
RECORD-CREATION TIME by the writers that open claims and custody.
TERMINAL WRITERS NEVER RE-RESOLVE (SLC-R6-010): `exited`, `reaped`,
and `swept` reuse the claim's recorded `checkoutPath` verbatim,
because the checkout they speak for may no longer exist to resolve.
Exact string equality on `checkoutPath` is a join key, and two
spellings of one checkout would silently split its state.

## REG-2. Events and schema (SLC-R5-017)

Common fields on EVERY record: `{schemaVersion: 1, event,
checkoutPath, at}`. CLAIM events additionally carry `ownerTag`;
CUSTODY events additionally carry `custodyId`; a custodied claim's
`arming` and `armed` carry BOTH (custody binds at arming, D-3 —
SLC-R5-005 removed the separate binding event). The validator is
per-event-type; there is no field that is sometimes present under one
meaning and sometimes another.

Claim events:

- `arming` — appended by the ARMER inside the arming gate (REG-4),
  after minting the owner tag, BEFORE launching the owner process.
  This is the RESERVATION, and a reservation consumes a cap slot
  (SLC-R5-001). Optional `custodyId` — and a custodied `arming` is
  GUARDED (SLC-R7-002): refused unless the referenced custody still
  reduces OPEN, so a provisioner that outslept the grace window fails
  provisioning instead of arming an owner no custody governs.
- `armed` — appended by the ARMER once the owner's identity is
  captured, BEFORE the checkout lock's `owner.json` is replaced
  with that identity and before ARMED is printed (SLC-R13-003: the
  registry speaks before the lock, so a live joinable owner is
  never an unarmed reservation). Adds
  `{ownerPid, ownerPidStartedAt, generation}`; carries the same
  `custodyId` if custodied. GUARDED (REG-3).
- `relaunched` — appended by the OWNER before each `launch_set`'s
  first `launch_detached` (write-ahead, SLC-R3-013). Adds
  `{generation, watcherTag, reaperTag, retiredThrough}`.
  `retiredThrough` is a CONTIGUOUS WATERMARK (SLC-R8-002,
  SLC-R9-003): the highest generation G such that EVERY unretired
  generation up to and including G has ALL its recorded identities
  VERIFIED DEAD. At each stop-and-relaunch the owner re-verifies
  every unretired prior generation it still holds — dead pids stay
  dead, so a generation unverifiable at its own stop usually becomes
  verifiable at the next — and advances the watermark over the
  verified contiguous prefix ONLY: one unverified older generation
  PINS the watermark below it, however many newer generations were
  verified, because a scalar must never license dropping a
  generation it skipped. The owner is the one process positioned to
  prove death, the proof is what licenses compaction to forget, and
  the cost of a pinned watermark is retained records, never a lost
  survivor.
- `launched` — appended by the OWNER once per COMPONENT, as soon as
  that component's identity is captured (SLC-R4-011, SLC-R5-008).
  Adds `{generation, component: watcher | reaper, pid, pidStartedAt}`.
  Under `setsid` a component's pgid is its pid, so this record is also
  the process-group key for the janitor.
- `exited` — appended by the OWNER on any voluntary exit, AFTER its
  teardown attempt (SLC-R4-012). Adds `{reason: purpose-gone |
  superseded | giving-up | establishment-failed | shutdown |
  terminated, diagnosis, teardownComplete}`. `shutdown` requires the
  shutdown-intent channel (D-1, SLC-R7-009); a signal without intent
  is `terminated`.
- `reaped` — appended by the JANITOR (or the arming gate closing a
  dead-owner claim, or the `--shutdown` caller after escalation).
  Adds `{reason: checkout-gone | custodian-dead | owner-dead |
  establishment-orphan | shutdown-escalated, killed: [...],
  sweepPending}` (SLC-R5-015). `sweepPending` is true whenever the
  reap could not PROVE every recorded process of the claim gone —
  for ANY reason, not only owner-dead (SLC-R6-008) — and marks the
  claim sweepable. `shutdown-escalated` is appended by the
  `--shutdown` CALLER after escalating past the owner-stop wait to
  KILL (D-1, SLC-R9-004): an owner killed mid-teardown cannot speak,
  the caller holds both the intent and the identity it signalled,
  and a terminal record must never hide survivors.
- `swept` — appended by the JANITOR after verifying a previously
  closed, still-sweepable claim now has zero surviving members
  (SLC-R5-015). The ONE post-terminal update: it clears sweepable and
  reopens nothing.
- `torn` — the framing repair marker (REG-1); carries no claim state.

Custody events (same-lifetime provisioners only, D-3):

- `custody` — appended BEFORE arming. Adds
  `{custodyId, custodianPid, custodianPidStartedAt, note}`.
- `custody-released` — appended after a VERIFIED teardown, or by the
  janitor after a custodian-dead reap. Names its `custodyId`, so a
  late release of one custody can never hide another (SLC-R4-007).

OWNER TAG UNIQUENESS IS ENFORCED, NOT ASSUMED (SLC-R5-013,
SLC-R7-008): the tag is the sanitized physical checkout path + epoch +
armer pid + 4 random hex characters, and the gate REFUSES a
reservation whose tag is SEEN AT ALL in the current reduction — open
OR closed — because a closed claim's terminal is absorbing and a
reused key could neither open nor be told apart. The armer regenerates
its suffix and retries; the check is free because the gate already
holds the lock and the reduction. CUSTODY IDS OBEY THE SAME LAW
(SLC-R13-004): a custodyId is the sanitized physical target path +
custodian pid + custodian start + 4 random hex characters, and a
`custody` append is REFUSED under the lock if its custodyId is SEEN
AT ALL in the current reduction, open or closed — reduction closes
custody absorbingly, so a reused id would read as already released
and one release could hide another. The provisioner regenerates and
retries, exactly like the armer.

## REG-3. Reduction and guarded appends (SLC-R3-009, SLC-R4-004, SLC-R5-003, SLC-R5-004)

Reduction is a FOLD over all records:

- Per `ownerTag`: a claim OPENS at `arming`, gains identity at
  `armed`, and CLOSES at `exited` or `reaped`. Terminal events are
  absorbing for claim state; `swept` is the one permitted
  post-terminal update. The reduced claim carries the owner identity,
  its `relaunched`/`launched` records PAIRED BY GENERATION
  (SLC-R7-007): the CURRENT set is the highest generation's tags with
  that same generation's identities — never append order, which a
  delayed retry can scramble — and EVERY recorded generation's
  identities remain available to teardown and sweeps, so a stale
  retry neither pairs old pids with new tags nor hides an old group.
  On close the claim carries the reason plus `teardownComplete`.
- GUARDED APPENDS: `armed` may be appended ONLY while its `arming`
  reservation still reduces as open, checked under the same lock as
  the append (SLC-R5-003). A refused armer tears down the owner it
  launched and fails. Because reopening is refused at the door,
  compaction may drop closed claims freely — no tombstone retention is
  needed for the absorbing property (SLC-R5-004).
- CAP SLOTS (SLC-R5-001, SLC-R5-002, SLC-R6-003, SLC-R6-007): a slot
  is consumed by a live claim whose owner is verified alive; by an
  OPEN reservation — open means NOT CLOSED, and grace expiry does not
  close it, it only makes it actionable; by a live claim whose owner
  liveness is UNKNOWN (indeterminacy counts toward the cap: the gate
  under-admits, never over-admits); and by a closed claim still
  SWEEPABLE — a possibly-surviving set holds its slot until `swept`.
  The GATE RESOLVES actionable claims it encounters (reap or compact
  under its lock, by REG-6) before granting slots, so admission never
  waits on a janitor that only runs at suite start (SLC-R6-003). Only
  provable owner death frees a slot, and the close stops the claim's
  recorded set (D-4, SLC-R5-012).
- STALE RULES: an `arming`-only claim past the grace window with no
  process matching its invocation signatures is compactable; with a
  live signature match — including the OWNER shape (REG-6,
  SLC-R6-004) — it is reaped as `establishment-orphan` (SLC-R4-005,
  SLC-R5-015). A custody with no bound claim past the grace window is
  stale: compactable and reported (SLC-R5-005).
- A claim is SWEEPABLE when closed by `exited` with
  `teardownComplete: false` or by `reaped` with `sweepPending: true`
  (SLC-R4-012, SLC-R5-012, SLC-R5-015, SLC-R6-008), until a `swept`
  record lands.
- Per `custodyId`: `custody` opens; the custodyId on a claim's
  `arming`/`armed` binds it; `custody-released` closes it (absorbing).
  Only a bound, unreleased custody triggers the dead-custodian rule,
  only against its bound claim, and only with no live announced
  session on the checkout (D-3, SLC-R4-007, SLC-R5-006).
- JOINS APPEND NOTHING: an ordinary arm that joins a live owner mints
  no owner and writes no claim.

COMPACTION rewrites the file as the fold's output: live claims —
their `arming`/`armed` plus every UNRETIRED generation's
`relaunched`/`launched` records (SLC-R8-002: a generation may be
dropped ONLY when a later `relaunched` covers it via
`retiredThrough` — a CONTIGUOUS watermark, SLC-R9-003, so every
dropped generation belongs to a fully verified prefix and an
unverified older generation pins everything above it into retention;
retaining all generations forever would grow without bound, and
keeping only the latest would let an unverified old group vanish
behind a clean sweep — the retirement proof is what reconciles the
two) — plus sweepable closed claims until swept; bound unreleased
custody TOGETHER WITH ITS BOUND CLAIM'S FULL REDUCED SKELETON —
`arming`, `armed`, AND the claim's terminal (`exited` or `reaped`,
plus `swept` if landed) — even when that claim is closed clean
(SLC-R9-006, completed by SLC-R10-001: the binding lives only on
the claim's opening records, and CLOSURE lives only on its
terminal. Round 9's fold retained the first and dropped the second,
so re-reduction reopened a PHANTOM claim — consuming a slot,
eligible for re-reaping — out of a cleanly closed one. The claim's
records ride along solely to keep the custody bound AND the claim
closed until `custody-released` lands); and unbound custody still
inside its grace window (SLC-R6-002). Everything else may be
dropped. Healthy operation
retires each generation at the next relaunch, so the compacted state
stays near one generation per claim. Compaction runs under the registry lock, inside
the janitor and ALSO by any writer that observes the file past the
size threshold (D-6) — growth is bounded by the trigger, not by a
per-claim event count, because separated relaunch cycles are unbounded
(SLC-R5-018).

## REG-4. One lock for every mutation (SLC-F-003, SLC-R4-002, SLC-R4-003)

A single exclusive lock (a directory lock beside the file, the
codebase's existing idiom) covers EVERY mutation:

- an APPEND holds the lock for its tail inspection plus one framed
  write (REG-1);
- a GUARDED append holds it across its reduce-check-write (REG-3);
- the ARMING GATE holds it across reduce-count-reserve, so two armers
  racing the last slot cannot both see it free (SLC-R4-002);
- COMPACTION holds it across read-reduce-replace, so a concurrent
  append can never be discarded by the rewrite (SLC-R4-003).

Appends are rare, so contention is negligible; lock acquisition uses
the same bounded-wait discipline as the supervision locks. Read-only
reduction (status, reporting) needs no lock.

THE LOCK HAS ITS OWN CRASH RECOVERY (SLC-R7-003, SLC-R8-001) — a lock
protecting crash repair must itself survive crashes, and the
acquisition must have NO ownerless window for a paused acquirer to
resume into: the acquirer builds a PRIVATE directory containing its
`owner.json` (pid, pidStartedAt) and acquires by an atomic RENAME of
that directory to the lock path — the lock is born already owning,
so an ownerless lock directory can never result from acquisition. A
waiter that exhausts its bounded wait may take the lock over ONLY
after proving the recorded holder DEAD by exact identity; a lock
directory with no owner file is garbage by construction and takeable
after the same bounded window, because no live acquirer can ever be
mid-publication. Uninspectable is alive: an unreadable owner file
never authorizes takeover.

## REG-5. Failure and corruption contract (SLC-R3-010, SLC-R4-013)

- ARMING: if `arming` or `armed` cannot be appended — or `armed` is
  REFUSED by the guard — ARMING FAILS, loudly, and the armer STOPS any
  owner process it already launched before exiting nonzero
  (SLC-R4-005, SLC-R5-003); if that stop fails, the stale-`arming`
  rule is the recovery.
- PROVISIONING: if `custody` cannot be appended, provisioning fails —
  a driver must not create supervision it cannot bequeath.
- OWNER, split by direction (SLC-R6-006): appends that CREATE process
  custody are GATING — a failed `relaunched` append means
  `launch_set` does not launch this cycle, and a failed `launched`
  append is retried at every observation with persistent failure
  counting as an incrementing breaker observation (D-2). Appends on
  the way OUT (`exited`) are best-effort: on failure the owner logs
  to `supervisor.log` and proceeds, because an owner that cannot
  speak must still die.
- JANITOR: kills are verified first; then the janitor appends the
  terminal ITS POST-KILL RE-REDUCTION CALLS FOR (SLC-R11-004,
  aligning with D-4's owner-terminal-wins rule): `reaped` for a
  claim still open, `swept` for a self-closed claim that was
  sweepable and is now verified clear — and a claim the owner
  closed CLEANLY (`teardownComplete: true`, survivors verified
  none) calls for NO CLAIM append at all, because its terminal is
  absorbing and a sweep of a non-sweepable claim is not a legal
  record; that absence is the janitor's success, not its failure.
  THE NO-APPEND RULE COVERS CLAIM TERMINALS ONLY (SLC-R12-003):
  custody transitions the run owes are appended regardless — a
  CUSTODIAN-DEAD reap appends `custody-released` whether the bound
  claim was reaped, self-closed clean, or already closed, because a
  bound unreleased custody would otherwise stay actionable and
  compaction-retained forever. Where an append IS called for and
  fails, the run reports it and exits nonzero. The kills stand —
  reaping is idempotent.
- ARMING FAILURE CLOSES WHAT IT OPENED (SLC-R12-004): the armer's
  failure path (armed unappendable or refused) stops the owner it
  launched AND appends the claim terminal itself — `reaped` with
  reason `establishment-orphan` and `sweepPending` per what its
  teardown could prove; the failing armer is a legal `reaped`
  writer for ITS OWN reservation only. A CUSTODIED arming that
  fails this way leaves custody bound to a CLOSED claim, and the
  responsibility ladder is explicit: the PROVISIONER (alive by
  definition — same lifetime) fails provisioning loudly and
  appends `custody-released` after the verified teardown; a
  provisioner that died anyway is the janitor's custodian-dead
  case, whose release rule above applies. D-3's grace-window expiry
  covers only custody whose `arming` was NEVER appended.
- CORRUPTION: a torn tail under REG-1's repair rule is tolerated and
  reported. ANY other unparseable or schema-invalid record marks the
  registry CORRUPT, and both safety-critical readers FAIL CLOSED
  (SLC-R4-013): the arming gate REFUSES to arm and the janitor KILLS
  NOTHING — both report loudly. Repair is a human decision, performed
  with the compaction tool against the readable records; fail-closed
  trades availability for never killing healthy supervision on a
  corrupt view.

## REG-6. Kill proof (SLC-R4-011, SLC-R5-010, SLC-R5-011)

One rule, shared with D-4: A PROCESS IS KILLABLE ONLY IF PROVEN, and
proof is the TRIPLE, never identity alone (SLC-R8-006): pid,
pidStartedAt, AND argv consistent with the claim — a known invocation
shape with the claim's tag in its tag position, or for helpers,
membership of a proven group. The committed identity source records
start times to WHOLE SECONDS, so pid + start alone is satisfiable by
a stranger after same-second pid reuse; argv consistency is the third
factor that makes that practically impossible. The known shapes are
the component invocations AND the owner invocation
(`arm-supervision.sh __owner ... --tag <tag>` — SLC-R6-004: without
the owner shape, an establishment orphan's live owner is unkillable
and respawns its components exactly as the incident did). The triple
is captured in ONE observation immediately before signalling.
ACCEPTED RESIDUAL, stated because POSIX kill offers no
compare-and-signal: between that observation and the signal a proven
process can exit and its pid be reused; the window is milliseconds,
and a misfire additionally requires same-second reuse WITH matching
argv. The design accepts that residual explicitly rather than
claiming an atomicity the kernel does not provide. A process
that merely mentions a tag (a shell, a grep, an editor) matches no
invocation shape and is never signalled. A recorded process group is
killed THROUGH its proven members or live proven leader, never by
group number alone: an empty group's number is recyclable, and
membership without provenance is not proof. The unprovable residue is
reported and accepted — D-2's single-flight rule makes it
bounded-lifetime.

## REG-7. What this file is and is not

- It is the janitor's and the arming gate's ONLY input for
  machine-wide questions: consumed cap slots and orphan discovery.
- It is NOT a lease, NOT a lock on anything but itself, and NOT
  consulted by the owner's own exit logic — the owner reasons from
  its lock file, state file, and held identities only (D-1).
- Growth is bounded by the compaction trigger (D-6), not by a
  per-claim event count: a long-lived checkout's separated relaunch
  cycles append without limit, and any writer past the threshold
  compacts under the lock (SLC-R5-018).
