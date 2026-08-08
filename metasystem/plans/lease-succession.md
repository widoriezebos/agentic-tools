# A mission's own succession is a renewal, not a takeover

- Goal and current status: stop the single-writer checkout lease from
  cancelling a mission's in-flight work every time the mission's own next
  process takes the lease. Design in critique round 2; not yet implemented.
- Next step: critique round 2 with sol, then implement
- In flight right now: design critique round 2 with sol
- Waiting on the human: nothing

## Human rulings for the unattended run of 2026-08-08

The human is away from keyboard for ten hours from 2026-08-08 and recorded
three standing decisions for that window. They are written here because an
authorization that exists only in a conversation cannot be audited later.

- SIGNING. The human pre-authorized the orchestrator to write the ordinary
  approval lines for bm-2 cohort contracts on their behalf during this
  absence, including every repetition and any cohort re-provisioned after a
  failure. Scope is bm-2 cohorts ONLY; nothing else is signed for them. Each
  signature records that it is human-authorized and that the uncontained-Devin
  residual (containment is the operator's VM) was accepted on 2026-08-08.
  This is a one-off, time-boxed decision by the human for one benchmark, NOT a
  mechanism and NOT the standing-authorization concept human-surface-design.md
  retired. That design's answer to signing many repetitions is unchanged and
  still the target: F-3's batch signing, where ONE typed human confirmation
  appends each contract's own ordinary approval line. F-1/F-3 are not built
  yet, which is exactly why the human had to delegate the transcription by
  hand.
- RETRY BUDGET. At most THREE full cohort attempts. If the harness still is
  not behaving after the third, stop, write the diagnosis, and wait for the
  human rather than continuing to spend. Each bm-2 repetition declares EUR:40
  exposure; swe-1-7 delegates are free under the roster ruling, so the real
  spend is the Opus host.
- FIX BAR. A mechanical defect (a wrong argument, a missing field, a bad path)
  is fixed directly, proven, and gated. Anything that changes a contract or an
  invariant goes through design -> sol critique -> implement, as usual.

## What went wrong, observed

The first real benchmark cohort with live delegates (bm-2, Devin `swe-1-7`) ran,
but two of its three delegate jobs died with `error: stale-claim-epoch` in
`phase: claim-sweep`. The target's lease record showed why: three
`holder-death` takeovers in one cohort's life, walking the claim epoch 1 → 2 →
3 → 4, at 18:51, 18:52, and 19:20. Each takeover swept the jobs created under
the previous epoch. One Devin delegate happened to finish between two takeovers
and survived; the others were swept mid-flight, through no fault of the delegate
or the adapter.

O-1. `claim_for_announcement` (scripts/agents/worktree-lease.py) treats a claim
whose `mainId` differs from the current `holderMainId` as a takeover WHENEVER
the recorded holder's process is dead: it sets `takeover = True`, bumps
`claimEpoch`, and runs `cleanup_stale_jobs`, which fails every job whose
recorded `claimEpoch` is below the new lease epoch.

O-2. A `mainId` is `main-<pidStart>-<pid>-<hex>` — inherently one per process.
The mission runner announces a fresh one per pid, but ONLY on the branch where
`classify` finds no live holder (arming_identity, mission-runner.sh); beneath a
live holder it reuses that holder's identity instead. The benchmark target is
always the first branch.

O-3. A mission is driven by a CHAIN of short-lived processes, not one. The kit
deliberately splits staging from the resume at the human sign boundary, so
staging arms and exits, the human signs, and the resume arms again — a second
process, a second `mainId`, over a now-dead holder.

O-3a. The DOMINANT chain is host turns, which the first version of this design
missed. `launch_host` starts a fresh host process per TURN
(`start_new_session=True`), and that process arms in the target and becomes the
lease holder under its own per-process `mainId`. So every turn boundary is a
succession, and a turn that ends with a delegate still running has that delegate
swept by the next turn's host. Threading a lineage through the mission runner's
OWN arming was therefore not enough: the first retry after the fix still showed
`ownerLineage` defaulted to a `mainId` and two holder-death takeovers, because
the holder was a host session, not the runner. Each succession is O-1's takeover
against the mission's own predecessor.

O-4. So a mission takes the lease from ITSELF, repeatedly, and each self-takeover
cancels the delegate work the previous process had in flight. Nothing foreign
is involved; the safety mechanism for a foreign takeover is firing on a
legitimate continuation.

## Root cause

Ownership is keyed on an identity that cannot outlive one process. `claimEpoch`
and `revision` are ALREADY separate and correctly so — `revision` advances on
every write for compare-and-swap, `claimEpoch` advances on ownership claims and
invalidates old jobs. This is not the old overloaded-generation finding (MV-2-4)
and this design does not claim to close it. The defect is narrower and different:
because the holder is identified by a per-process `mainId`, a legitimate
continuation of the same logical writer is indistinguishable from a foreign
actor seizing a dead writer's checkout, so the epoch bumps when it should not.

The missing capability is the one MM-2-7 named: a legitimate successor should
keep the predecessor's in-flight jobs rather than sweep them.

The distinction the mechanism cannot currently draw: "the same logical writer
continued in a new process" versus "a different actor seized a dead writer's
checkout." The first must preserve in-flight work; only the second must sweep
it.

## D-1. An ownership lineage, separate from the per-process mainId

The lease and the announcement gain an `ownerLineage`: a stable string
identifying the logical writer across the processes that carry it. The
per-process `mainId` is unchanged — it still identifies THIS process for
liveness and custody. The lineage is the coarser identity that survives a
process exit.

The DEFAULT is what preserves today's behaviour: an announcement with no
`ownerLineage` has lineage == its own `mainId`. One process, one lineage, so an
ad-hoc agent session is its own lineage and any succession by a different
session is still a foreign takeover, exactly as now.

### D-1a. The schema transition, stated exactly

Three readers enforce a field set, and all three must be named because a
producer-only change breaks arming:

- `load_lease` (worktree-lease.py) requires an EXACT field set
  (`set(value) != required_fields` fails). `ownerLineage` is added to
  `required_fields`, and a lease read WITHOUT it is accepted by defaulting
  in memory to `value["holderMainId"]` — never by rejecting. So a checkout
  already claimed before this change still arms and still authorises its
  holder. The default is persisted on the next lease write, so migration is
  automatic and needs no separate step.
- The lease's own announcement reader (`ANNOUNCEMENT_BASE_FIELDS` /
  `ANNOUNCEMENT_IDENTITY_FIELDS`) treats `ownerLineage` as OPTIONAL: a
  pre-change announcement is read with the default, not refused.
- The process census (process-census.py) validates announcements with
  `set(value) <= expected | {"mainId", "commandHash"}`. `ownerLineage` joins
  that optional set. This one is load-bearing: arming BLOCKS on a successful
  census (`wait_for_first_heartbeat`), so omitting it would make every
  lineage-bearing announcement schema-invalid, fail the census, and prevent
  the new mission arming path from ever completing.

A lineage is validated on the way in: non-empty, `[A-Za-z0-9._-]{1,128}`, so it
cannot smuggle whitespace or control characters into a record that other tools
parse.

### D-1b. A repeated announcement for the same process

`announce` is idempotent by (pid, pidStartedAt): when a record for that identity
already exists it claims using the STORED record and ignores newly supplied
arguments. That default would silently drop a supplied lineage and leave the
mission's next process foreign — the whole fix defeated without a visible
failure. The rule for a repeat, stated per case:

- No lineage supplied → keep what is stored. Unchanged.
- Stored lineage ABSENT, one supplied → ADOPT it, and if this process holds the
  lease, write the same lineage onto the lease in the same operation. Same
  holder, so no epoch change and no sweep. This is the migration path for a
  record announced before the option existed, and it is a ONE-WAY fill.
- Stored and supplied lineage EQUAL → no-op.
- Stored and supplied lineage DIFFER → REFUSE loudly. A process does not change
  its logical owner mid-life; a disagreement is a caller bug and must not be
  resolved by silently preferring either value.

So a lineage is immutable once set for a given process, and absent-to-present is
the only transition.

### D-1c. The migration touches two records, so it needs an order

The announcement and the lease are persisted independently, and a crash between
them must not leave a durable mismatch — a lease still carrying the old lineage
while the announcement carries the new one makes the mission's next process look
foreign, which is the sweep this design exists to prevent.

The order is LEASE FIRST, then the announcement record, because only that
direction is self-healing:

- Crash after the lease, before the announcement: the lease already carries the
  lineage, so any later process of the same mission claims with a matching
  lineage and RENEWS correctly. The stale announcement is repaired on the next
  announcement by the absent-to-present fill in D-1b. Nothing is swept.
- The reverse order is recoverable too, once every announcement reconciles the
  held lease, but it is still wrong: it leaves a WINDOW in which the lease
  carries the old lineage while the mission believes it has migrated. The lease
  is what a claim reads, so a mission process arriving inside that window is
  judged foreign and sweeps — the failure this design exists to prevent, merely
  made rarer instead of impossible. Lease-first has no such window: the record
  that decides the claim is correct from the first write.

The no-op in D-1b is therefore scoped to the ANNOUNCEMENT record only. Every
announcement re-checks the lease it holds and repairs a lineage mismatch, so an
interrupted migration heals on the next arming instead of persisting. The lease
repair is an ordinary compare-and-swap write by the same holder: `claimEpoch`
unchanged, `revision` advances, no takeover appended, nothing swept.

## D-2. The claim rule, restated by lineage

`claim_for_announcement`, when the current holder's `mainId` differs from the
claimant's:

- Holder LIVE → REFUSE, whatever the lineage. A live holder never loses the
  lease. (Revised: an earlier draft allowed a same-lineage live claimant to
  take over, which would let an accidental duplicate launcher displace a live
  holder and let siblings alternate custody. That contradicted the invariant
  the design promises to keep, so it is gone. A same-PROCESS re-announcement
  is a different branch and is unaffected.)
- Holder DEAD, SAME lineage → RENEWAL. Preserve the claim epoch, adopt the
  predecessor's in-flight jobs, do not sweep. This is the mission's own
  succession, and it is the broken case.
- Holder DEAD, DIFFERENT lineage → TAKEOVER, unchanged: bump the epoch and
  sweep, because a genuinely different actor has taken a dead writer's
  checkout.

### D-2a. What the renewal writes

A renewal replaces the WHOLE holder-identity tuple, not a subset:
`holderMainId`, `pid`, `pidStartedAt`, and `commandHash` all become the
successor's. Liveness is the PAIR (pid, pidStartedAt); writing a new pid beside
the predecessor's start time would make the live successor test as dead, invite
a foreign takeover, and sweep the very jobs this design exists to protect.
`claimEpoch` is preserved (that is the point); `revision` advances as it does on
any write; `renewedAt` is stamped; `takeovers` is NOT appended (this is not a
takeover, and the takeover history stays a record of foreign seizures).

### D-2b. Finishing a predecessor's interrupted sweep

A foreign takeover writes the new epoch BEFORE sweeping, so a crash between the
two leaves a lease whose `reaped-after-claim` stamp still names the prior epoch.
Today only the same `holderMainId` announcing again resumes that cleanup.

A same-lineage successor must therefore, BEFORE renewing, complete any
outstanding cleanup for the lease's CURRENT epoch: if the stamp does not match
(`holderMainId`, `claimEpoch`) of the lease as it stands, run
`cleanup_stale_jobs` for that epoch and write the stamp. Two things this gets
right: it never certifies foreign jobs as reaped without actually reaping them
(which would silently break the foreign-takeover guarantee), and it never
sweeps jobs of the current epoch — `cleanup_stale_jobs` fails only records
BELOW the lease epoch, and a renewal does not raise it. The stamp is rebound to
the successor's `holderMainId` with the SAME epoch, so the next arming sees a
complete stamp and does not re-sweep.

## D-3. Threading the lineage through a mission

The mission runner already scopes its arming session to the mission
(`mission-runner-<mission>-<pid>`); only the `-<pid>` makes each process a
stranger. Every process that arms for the same mission — staging, resume, any
re-arm — must derive the SAME lineage from the mission id alone, so no token is
stored or shared.

Host turns inherit it rather than deriving it: `launch_host` exports
`METASYSTEM_OWNER_LINEAGE`, which `arm-supervision.sh` takes as the default for
`--owner-lineage`. The host's own arming (a session hook, not a call this code
makes directly) therefore carries the mission's lineage without the host adapter
knowing anything about lineages. That is what makes every turn of one mission
the same logical writer.

The lineage is DERIVED, not concatenated: `mission-<first 32 hex of
sha256(missionId)>`, a fixed 40 characters. Concatenating the mission id would
break on length — the mission contract puts no bound on a mission id, so a
114-character id would exceed the 128-character lineage bound and the mission
could not arm at all. Truncating it instead would be worse: two missions sharing
a prefix would share a lineage, and a foreign takeover between them would be
misread as a renewal, suppressing exactly the epoch bump and sweep that keeps an
abandoned mission's jobs from mutating a checkout. A hash is fixed-length for
any id, and it stays debuggable because it is recomputable from any mission id. `arm-supervision.sh` gains an
`--owner-lineage` option; the mission runner passes it; `announce` records it on
the announcement and the lease.

### D-3a. The live-holder branch is explicitly out of scope

`arming_identity` has two branches (O-2). This design changes ONLY the
unattended branch, where the runner is itself the main — which is every
benchmark target, and where bm-2 failed.

Beneath a LIVE holder the runner reuses that holder's identity and announces
nothing, so the lease keeps the holder's own lineage (its `mainId` by default).
The consequence, stated rather than hidden: if that interactive holder dies
mid-mission, a fresh mission process is a different lineage, so it takes over
and sweeps — today's behaviour, unchanged. Fixing that means giving an
interactive session a lineage that survives its own restart, which is a
separate question about session identity, not about missions. It is not
required to close the observed defect, and no fixture here claims it.

## D-4. What "adopt" means

Nothing is written to job records. Adoption is achieved ENTIRELY by preserving
the epoch: surviving jobs still carry the lease's current `claimEpoch`, so
`cleanup_stale_jobs` (which fails only records below it) leaves them alone.

Surviving records keep the predecessor's `mainId`, and that is safe because no
reader compares a job's `mainId` to the lease. The only comparison is
record-to-record inside the `setup` transition (dispatch.sh: `record.get("mainId")
!= current.get("mainId")`, comparing the incoming record with the stored one for
the same job), and `mainId` is in the immutable set, so a self-consistent record
stays valid across the renewal. This is deliberately NOT the per-job rewriting
MM-2-7 once proposed: rewriting an immutable field under per-job locks would
reintroduce the stateful adoption scan that was later deleted, for no benefit
the epoch does not already provide.

## D-5. What stays exactly as it is

- The one-writer guarantee: a live holder is never displaced; a second live
  writer is still refused with OWNED-ELSEWHERE.
- The foreign-takeover sweep: a different lineage claiming a dead holder still
  bumps the epoch and sweeps, so an abandoned foreign session's dispatch
  children cannot keep mutating after a new owner starts.
- Job stamping: jobs still record the claim epoch and mainId at creation;
  delegates are still children of the holder and never claim the lease.
- Non-mission arming: no lineage supplied → lineage is the mainId → unchanged.

## Proof

- Same-lineage succession: arm as lineage L, create a pending job, kill the
  holder process, arm again as the SAME lineage L. The epoch is unchanged, the
  lease's holder tuple (mainId, pid, pidStartedAt, commandHash) is entirely the
  successor's, `takeovers` did not grow, and the pending job is still pending.
- Foreign takeover still sweeps: the same fixture with a DIFFERENT lineage on
  the second arming bumps the epoch, appends a takeover, and fails the pending
  job with `stale-claim-epoch`.
- A LIVE holder is refused by a different lineage AND by the same lineage —
  both OWNED-ELSEWHERE (proving D-2's revision).
- Interrupted foreign sweep: with a lease whose epoch was bumped but whose
  stamp names the prior epoch, a same-lineage successor completes the sweep
  (the below-epoch job is failed) and does not raise the epoch, then renews.
- Lineage derivation: a 114-character mission id (which would overflow a
  concatenated lineage) produces a valid 40-character lineage, and two mission
  ids sharing a long prefix produce DIFFERENT lineages — so the foreign sweep
  between them still fires.
- Interrupted migration: with the lease already carrying the lineage and the
  announcement not yet updated, re-arming the same mission reconciles both, the
  epoch is unchanged and no job is swept. The completed-transition case cannot
  expose this, so it is a separate fixture.
- Repeated announcement: for one process identity, supplying a lineage where
  none was stored fills it in and updates the lease without an epoch change;
  supplying the same one again is a no-op; supplying a DIFFERENT one is refused.
- Schema compatibility: a lease written before this change (no `ownerLineage`)
  still loads, still authorises its holder, and gains the defaulted field on
  the next write; an announcement without the field is accepted by the census
  and by the lease reader; a lineage-bearing announcement passes the census.
- An arming with no lineage behaves exactly as today (lineage == mainId).
- End to end: a staging→exit→resume sequence for one mission preserves the
  first process's in-flight work across the human sign boundary, and a bm-2
  cohort completes without a `stale-claim-epoch` sweep of its own delegates.
