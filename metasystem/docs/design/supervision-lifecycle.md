# The supervision lifecycle: what owners, watchers, and reapers must do

The durable rules (D-1..D-7) of the supervision lifecycle, promoted
from `plans/supervision-lifecycle.md` (which keeps the incident
measurement, proof obligations, implementation order, and the critique
chain, CLOSED at its cap). `internal/supervise` implements them. A
change to any D rule is a contract change, not a cleanup.

## D-1. Purpose from the state file, currency from the lock; exit means teardown

Rounds 1-3 tried to DERIVE checkout identity from repository
properties — path, fingerprint, inode — and every candidate failed. The
path is reusable (SLC-R2-003). The fingerprint measures the wrong thing
twice over: `process-census.py fingerprint` hashes the supervision
SCRIPTS, runtime signatures, and config values, and the supervised repo
contributes only its resolved path string — so an ordinary code or
config edit in a live checkout would terminate healthy supervision
(SLC-R3-001), while a replacement repository at the same path hashes
IDENTICAL and contributes nothing to adoption detection (SLC-R3-002).
The fingerprint keeps its real job — `verify_armed` forcing re-arm when
supervision code goes stale — and leaves the identity business
entirely.

Round 4 then showed that state CONTENT cannot decide who is current
either: a superseded owner's already-running `launch_set` can republish
`state.json` after the successor's publication, evicting the healthy
successor (SLC-R4-001). So the two questions are answered by two
different files, each already owned by the code:

- PURPOSE: `state.json` — the token this owner itself published inside
  the checkout. Its reachability proves the supervised thing still
  exists.
- CURRENCY: `owner.json` in the lock directory — the file arming
  atomically writes when an owner is established or replaced. Whoever
  it names is the current owner; nobody else is.

THE CHECK, every base-interval cycle, independent of backoff
(SLC-R2-008 — backoff delays relaunch attempts only, never
observations):

- PURPOSE GONE, checked FIRST (SLC-R6-001): the checkout root is
  definitively ABSENT — which takes both files with it — or
  `owner.json` names this process and `state.json` is definitively
  ABSENT (ENOENT on the file or any parent). The supervised thing no
  longer exists, or was deliberately revoked — subsumes the draft's E1
  and E3 (SLC-R2-005). Tear down; exit with reason `purpose-gone`. A
  DELETED CHECKOUT always lands here, never in SUPERSEDED: the lock
  vanishing WITH the checkout is not a replacement.
- CURRENT: the checkout root exists and `owner.json` is readable and
  names this process (pid + pidStartedAt + instanceTag) → this owner
  is current.
- SUPERSEDED: the checkout root exists and `owner.json` names ANOTHER
  identity, or is itself definitively ABSENT while the checkout
  persists (the lock was replaced or revoked). Tear down held tags;
  exit with reason `superseded`. Supersession of a live owner can only
  follow a FALSE death observation (the census helper erring under
  load) because takeover requires proven death — this branch is the
  insurance for exactly that case (SLC-R3-003).
- UNKNOWN: any other failure on either file — permission denied, I/O
  error, malformed content. Log and CONTINUE; attempt no relaunch this
  cycle. Only a definite negative kills (SLC-R1-003, SLC-R2-006).

State content NEVER decides currency, and stale state is REPAIRED, not
tolerated (SLC-R4-001, SLC-R5-016): an owner whose `owner.json` names
it but whose `state.json` owner stanza names another REPUBLISHES its
state from held identities within that same cycle — a cheap atomic
write, not a relaunch — so a departing revenant's late write converges
to the current owner within one cycle as the Proof requires.

PUBLICATION IS FENCED (SLC-R4-001): `launch_set` re-reads `owner.json`
immediately before its atomic `state.json` publication; if it no
longer names this owner, the publication is ABORTED and the owner exits
via the SUPERSEDED branch. The residual race is one atomic rename wide
and the successor's republish rule above erases it.

ESTABLISHMENT IS NOT REVOCATION, BUT IT IS BOUNDED (SLC-R1-004,
SLC-R4-005): the purpose/currency check arms only after this owner has
published its state once — and an owner that cannot COMPLETE first
publication (state written, both components heartbeat) within N
consecutive observations gives up: teardown of anything it launched,
best-effort `exited` with reason `establishment-failed`, exit. An owner
may not idle forever on the strength of a publication it never made.

THE CHECKOUT LOCK IS BORN OWNING (SLC-R9-002): round 8 removed the
ownerless acquisition window from the machine REGISTRY lock and left
it in the checkout lock. The committed armer creates `lock.d` empty
and publishes `owner.json` only after the owner process launches, and
its join path refuses an ownerless lock forever ("no process identity
exists to prove dead") — so an armer killed inside that window wedges
a LIVE checkout permanently, against D-7. The checkout lock therefore
adopts REG-4's acquisition discipline verbatim: the armer builds a
PRIVATE directory containing `owner.json` naming ITSELF (pid,
pidStartedAt, its arming tag, role `armer`) and acquires by one
atomic RENAME to the lock path — a reservation, born owning. Once the
owner's identity is captured, the armer atomically REPLACES
`owner.json` with the owner's identity, the write D-1 already names.
Recovery is death-only and TOTAL: a lock naming a provably dead
identity — armer or owner, exact pid + pidStartedAt + tag — is
removed and re-acquired by the next armer; a live armer's reservation
is respected exactly like a live owner (joins wait bounded, re-arms
refuse); and a lock directory WITHOUT `owner.json`, which acquisition
can no longer produce, is garbage after the same bounded window
REG-4 grants — never a permanent refusal. THE REGISTRY SPEAKS
BEFORE THE LOCK (SLC-R13-003): the armer appends `armed` FIRST and
only then atomically replaces `owner.json` with the owner's
identity — so no interleaving leaves a live, joinable owner that
the registry still shows as an unarmed reservation, which the gate
would later reap as an establishment orphan out from under a
recordless joiner. An armer dying between the two writes converges
without a kill: the claim is ARMED (not orphaned), the lock still
names the dead armer, the next arm's death-only recovery replaces
the lock, and the running owner — whose currency check never saw
itself named — exits SUPERSEDED on its own. And the
establishment-orphan reap inherits the join guard (SLC-R5-006):
before reaping an `arming`-only claim with live tag-matched
processes, the gate and the janitor check for a live ANNOUNCED
session on that checkout, and report instead of reaping when one
exists.

ACCEPTED CONSEQUENCES, written down so nobody rediscovers them as
bugs: deleting `owner.json` or `state.json` by hand on a live checkout
stops supervision — that is revocation working, and the next arm
re-establishes. A wholesale copy of a dead sandbox including
`artifacts/` is adopted at the recorded path — rare, and arguably the
owner doing its job. An owner blinded forever by UNKNOWN (for example
chmod-000 state) idles — one quiet process, no relaunches, no helper
spawn — until its checkout dies or a human intervenes; that is
prefer-the-leak applied to the owner itself.

EXIT IS A TEARDOWN, NOT A RETURN (SLC-R1-001), BY HELD IDENTITY ONLY:
the departing owner stops exactly the components IT launched,
identified by the tags and identities it HOLDS IN MEMORY — never by
re-reading a state file it may no longer own. The shipped code violates
this today: the owner's EXIT trap (`cleanup_owner` →
`stop_recorded_components`) kills whatever the CURRENT state file
names, with no ownership check, so a superseded owner receiving a late
TERM kills its successor's components (SLC-F-001). REPLACING THAT TRAP
WITH HELD-IDENTITY TEARDOWN IS A NAMED IMPLEMENTATION ITEM — it does
not fall out of adding exit conditions, and the new exits fire through
that very trap.

TEARDOWN PRECEDES THE TERMINAL RECORD (SLC-R4-012): every voluntary
exit tears down FIRST, then appends `exited` carrying
`teardownComplete` true or false. A terminal record must never hide
survivors: the janitor treats `teardownComplete: false` terminals as
sweepable until a verified sweep clears them (D-4, SLC-R5-015), and
`custody-released` may only follow a VERIFIED teardown (D-3).

TEARDOWN SURVIVES THE CHECKOUT (SLC-R7-005): the PURPOSE GONE branch
runs when everything inside the checkout — including
`process-census.py`, which the committed `stop_identity` calls to
verify identities — is already gone. The owner therefore performs
teardown from MEMORY and SYSTEM BINARIES ONLY: identities held since
launch, verified with `ps` directly, signalled by process group;
and its terminal registry append is self-contained in the owner
process (the registry lives outside every checkout and its framing is
its own, REG-1). Reusing the checkout-resident helper in the exit
path fails D-1's none-survive requirement, so replacing that
dependence is a NAMED IMPLEMENTATION ITEM alongside the trap.

SHUTDOWN HAS A CHANNEL, NOT A GUESS (SLC-R7-009), AND THE CHANNEL IS
SCOPED (SLC-R8-005): a TERM alone cannot tell the owner WHY it is
dying, and a bare marker file cannot either — it would outlive a
crashed caller and mislabel an unrelated TERM days later. The
`--shutdown` path writes a shutdown-intent file beside `owner.json`
BEFORE signalling (it runs with the checkout alive by construction),
carrying the TARGET OWNER's exact identity (pid, pidStartedAt, tag),
the requester, and a timestamp. The owner LATCHES the intent at
EXIT-INITIATION, not at append time (SLC-R9-004): the moment it
begins a signal-induced exit, BEFORE teardown, it reads the intent
once; if the intent names THIS owner and is younger than the
owner-stop wait (D-6) at that moment, the exit is attributed
`shutdown` however long the mandated teardown-before-terminal then
takes — the attempt was real even if the caller died mid-attempt.
Checking freshness at append time instead let a legitimate teardown
consume the window and relabel an honest shutdown `terminated`,
threading exactly between teardown-first (SLC-R4-012) and the
freshness rule. An intent outside its window, or naming another
identity, is IGNORED, REPORTED stale, and the reason is `terminated`.
The owner consumes the file on exit; a successor ignores and cleans
an intent naming its predecessor. THE CALLER'S ESCALATION RESPECTS
TEARDOWN (SLC-R9-004): `--shutdown` waits on the owner for the full
owner-stop wait — sized in D-6 to cover worst-case sequential
component teardown plus the terminal append — before any KILL; a
caller that does escalate appends the claim's terminal ITSELF,
`reaped` with reason `shutdown-escalated` and `sweepPending` per what
it can prove, because an owner killed mid-teardown cannot speak and a
terminal record must never hide survivors (SLC-R4-012).

SHUTDOWN IS CHECKOUT-WIDE, NOT SNAPSHOT-WIDE (SLC-R10-002): the
committed shutdown reads `owner.json` once and signals that identity
once, which leaves two honest gaps the amended lock makes worse. A
LOCK NAMING AN ARMER: the reservation identity is not an owner, so
the caller waits out the establishment transition (bounded by the
establishment deadline, D-6) for the lock to name an owner or
vanish; a PROVABLY DEAD armer's reservation is removed under D-1's
lock recovery and counts as stopped. A CONCURRENT REPLACEMENT: after
stopping its snapshotted owner, the caller RE-READS the lock; if it
now names a DIFFERENT live identity — a false-death successor whose
takeover D-1 explicitly permits, and who has cleaned the
predecessor's intent — the caller writes a FRESH intent for that
identity and repeats. The loop is bounded (D-6): more than 3
iterations fails LOUDLY per D-5, naming the identity that kept
appearing. `--shutdown` returns success ONLY on the exit condition
"the lock is absent, or its named identity is PROVEN dead — and the
condition still holds on a RE-READ after one settle window, with no
new identity having appeared — AND the claim's RECORDED SET is gone
too". THE LOCK SPEAKS FOR THE OWNER ONLY (SLC-R12-001): watcher and
reaper live in their own sessions and process groups and survive an
owner killed mid-teardown, so success additionally requires every
recorded component identity of the claim — every generation, REG-3
— DEFINITIVELY dead by the same three-way rule. AND THE SET
INCLUDES ITS WRITE-AHEAD SHADOW (SLC-R13-001): a component forked
after `relaunched` but before its own `launched` append exists only
as a TAG, and unlike the sub-second helper residue the design
accepts, a watcher is long-lived — so the success predicate also
requires NO live process matching the claim's component invocation
signatures with the claim's tags (the same REG-6 shapes the janitor
kills by); a signature-matched survivor fails the shutdown loudly,
named. An escalated
shutdown whose `reaped` recorded `sweepPending: true` therefore
CANNOT return success: the caller reports the surviving identities
and exits nonzero, and the janitor's sweep is the recovery — the
sweepable claim keeps holding its slot (D-4) until `swept` proves
the set gone. THE SETTLE WINDOW COVERS BOTH ARMS
(SLC-R11-001): an absent lock alone is not success, because a
death-only takeover removes the old lock BEFORE renaming its
reservation in — a shutdown observing that gap would report success
while an armer establishes the next owner; the re-read catches any
acquisition that lands within the window, and one that appears
counts as the next loop iteration. "PROVEN DEAD" IS D-2's
DEFINITIVE NEGATIVE, NEVER UNKNOWN (SLC-R11-002): the design admits
the census can falsely classify a live owner dead under load, and
the committed identity helpers collapse read errors into "dead" —
so this predicate demands a SUCCESSFUL read that shows absence;
an identity that cannot be read is UNKNOWN, UNKNOWN is never
success, and a loop that exhausts on UNKNOWN fails loudly rather
than reporting cleanup it cannot prove (making the shutdown path's
identity reads three-way is a NAMED IMPLEMENTATION ITEM — the
committed `identity_alive`/`stop_identity` pair cannot express it).
ACCEPTED CONSEQUENCE, stated so the Proof is honest: shutdown
guarantees no live supervision AT RETURN and catches acquisitions
in flight within the settle window; an INDEPENDENT arm that begins
after the window is new supervision outside any shutdown's scope —
preventing re-arming is revocation's job (delete the state file),
not shutdown's.

## D-2. The breaker counts deaths on one clock; the ceiling and single-flight bound the population

OBSERVATION IS THREE-WAY, matching D-1's indeterminacy rule
(SLC-R3-004): DEAD — the component's exact identity is proven dead;
STALE — the identity is alive but its heartbeat is stale or missing;
UNKNOWN — state or identity cannot be read at all. DEAD and STALE
increment the consecutive counter and permit a relaunch; a full base
interval with both components healthy resets the counter; UNKNOWN
neither increments nor resets — it is logged, and no relaunch is
attempted on it.

ONE TIMELINE (SLC-R3-005): the counter counts BREAKER OBSERVATIONS,
taken every base interval, and nothing else. Backoff gates RELAUNCH
ATTEMPTS only: after k consecutive incrementing observations, the next
relaunch is not attempted before interval × 2^(k-1), capped (D-6).
Observations and D-1's checks never slow down. After N consecutive
incrementing observations the owner gives up: teardown per D-1, then
`exited` with reason `giving-up`, the last diagnosis, and the teardown
outcome (SLC-R4-012).

THE POPULATION BOUND HAS TWO PARTS, honestly separated (SLC-R4-010,
SLC-R5-009). The ceiling observation is a DURATION property: every
breaker observation counts the members of this owner's component
process groups, a count above the ceiling is itself an incrementing
observation, and the owner STOPS THE SET immediately — so a group
above the ceiling survives at most one base interval, and `launch_set`
additionally refuses to start a component at or above the ceiling
(SLC-R3-006, SLC-R3-007). The NUMBER property comes from single-flight:
COMPONENTS ARE SINGLE-FLIGHT SPAWNERS — a component may have at most
one helper of each kind in flight (the committed watcher already
enforces exactly this for the census through the census-writer lock,
and the duration monitor is per-census). A component that cannot spawn
its next helper before the previous one is accounted for cannot storm
within an interval, whatever the observation cadence. The machine
bound is then K owners × (ceiling + the single-flight transient), and
the Proof asserts both halves separately.

WRITE-AHEAD LAUNCH, THEN IDENTITY PER COMPONENT (SLC-R3-013,
SLC-R4-011, SLC-R5-008): `launch_set` appends its `relaunched`
registry record — generation and freshly minted component tags —
BEFORE the first `launch_detached`, and appends a `launched` record
for EACH component as soon as THAT component's identity is captured,
not batched after both. The unrecorded window is thus the sub-second
between a component's first fork and its own `launched` append.
ACCEPTED CONSEQUENCE, stated so the Proof can be honest: a helper
orphaned inside that window is invisible to the janitor's records —
and is bounded-lifetime by construction (single-flight helpers run one
pass and exit), so the residue self-terminates. The design accepts
that residue rather than pretending a record can precede the pid it
records.

THE WRITE-AHEAD APPENDS GATE THE LAUNCH (SLC-R6-006): write-ahead
that may be skipped is not write-ahead. If the `relaunched` append
fails, `launch_set` does NOT launch — an owner that cannot record
intent must not create processes; the cycle counts as a failed
attempt and the breaker timeline proceeds. A failed `launched` append
is retried at every observation, and persistent failure is itself an
incrementing observation — an unrecordable set is stopped by the
breaker, not tolerated outside custody. Only terminal appends stay
best-effort (REG-5): an owner that cannot speak must still die.

## D-3. No dead-man's switch; custody only where a custodian actually lives

The draft's intent lease stays DROPPED (SLC-R1-007, SLC-R1-008,
SLC-R1-009): a mechanism that can kill healthy supervision on a missed
renewal is a worse bug than the one it fixes.

CUSTODY RECORDS ARE SCOPED TO SAME-LIFETIME PROVISIONERS
(SLC-R4-008): a `custody` record may be written only by a process
whose OWN LIFETIME spans the supervised sandbox — the fixture pattern,
where the provisioning process holds the sandbox and its trap is the
teardown. Such a provisioner appends `custody` BEFORE arming and
`custody-released` after a VERIFIED teardown (SLC-R4-012).

CUSTODY BINDS AT ARMING, NOT AFTER IT (SLC-R4-007, SLC-R5-005): the
provisioner passes its custodyId to the arm, and the claim's own
`arming`/`armed` records carry it. There is no separate binding event
and therefore no post-arm window in which a killed provisioner leaves
an owner that no custody governs: if the arm succeeded, the claim is
bound. A FAILED custodied arm does NOT leave unbound custody
(SLC-R12-004 — the binding rides the `arming` append, which precedes
every failure the armed guard can produce): it leaves custody bound
to the claim the failing armer itself closes (REG-5), released by
the provisioner's own failure path, or by the janitor's
custodian-dead rule if the provisioner died too. Only a custody
whose `arming` was never appended expires unbound through the grace
window (REG-3). This also removes the schema split the binding event created
(SLC-R5-017). A CUSTODIED ARM NEVER JOINS (SLC-R8-003): a join
appends nothing, so a joined arm would "succeed" with no record for
the guard to check and no claim carrying the custodyId. A provisioner
provisions FRESH supervision by definition — a live owner already on
its target means the sandbox is not fresh — so an arm carrying a
custodyId must ESTABLISH (or take over proven death); if it would
join, it FAILS provisioning. The janitor's rule for LIVE checkouts is then safe: an
unreleased custody bound to a claim whose custodian is PROVABLY DEAD →
reap that bound claim and report it. Acting on proven death of a
recorded identity, never on a missed renewal. A later human arm of the
same checkout mints a NEW claim carrying no custodyId and can never be
reaped by a stale one.

A CUSTODIED OWNER CAN BE JOINED LATER (SLC-R5-006): joining mints no
owner and writes no claim, so a human session can be attached to the
very claim a dead custodian would condemn. Before a custodian-dead
reap, the janitor therefore verifies there is NO OTHER LIVE ANNOUNCED
SESSION on that checkout — announcements carry exact identities, and
the checkout is live and readable in this branch by definition. If one
exists, it reports instead of reaping: supervision someone is actively
using is never killed out from under them.

THE CHECK AND THE JOIN ARE FENCED, NOT ASSUMED ATOMIC (SLC-R8-004): a
join is recordless, so an announcement landing between the janitor's
check and its kill would be invisible to any registry re-reduction.
The fence lives in the checkout both parties already touch: the
janitor writes a REAP-INTENT marker in the supervision directory,
waits one bounded grace, RE-CHECKS announcements, and only then
kills; the ARM path refuses to join while a fresh reap-intent is
present ("being reaped; re-arm to establish"). Arming order writes
the announcement BEFORE joining, so a join that beat the marker shows
at the re-check and aborts the reap, and a join that saw the marker
never happened. A stale marker (older than its bounded validity) is
ignored and cleaned by the next arm. THE MARKER LICENSES THE KILL AT
FIRE TIME (SLC-R11-003), AND IT IS HELD, NOT MERELY PRESENT
(SLC-R12-002): a freshness READ still leaves the check-to-signal
gap exposed to an arbitrary scheduler pause, so the final form of
the fence is kernel-mediated exclusion. The janitor takes an
EXCLUSIVE OS advisory lock (flock) on the marker file spanning its
final check — marker its own, fresh, re-read in the same
observation that captures the kill triple (REG-6) — through its
last signal; cleaning or replacing ANY marker, fresh or stale,
requires that same lock; the kernel releases it the moment a
paused-forever janitor dies. Eligibility can no longer change
between the janitor's last look and its kill, because whoever holds
the lock owns the interval. A joiner that cannot acquire the lock
within its bounded wait (D-6) REFUSES the join and reports — fail
closed toward not-joining. ACCEPTED CONSEQUENCE: a janitor paused
indefinitely while holding the lock blocks joins on that checkout
until it dies or a human intervenes; that is prefer-the-leak
applied to the fence, and the refusal names the holder.

COHORTS ARE NOT CUSTODIED — THEY GET REAL TEARDOWN WITH A DURABLE
MARKER (SLC-R4-008, SLC-R4-009, SLC-R5-007): the cohort lifecycle is
intentionally multi-invocation — `provision.sh` arms and exits,
`run-cohort.sh` records awaiting-approval and exits, a human resumes
later. There is no process whose lifetime spans a repetition, so
process custody would false-reap supervision that is healthily
WAITING. And `run-cohort.sh` contains no shutdown invocation at all —
the previous revision's claim that it "still tears down" was false —
so the driver GAINS teardown as NEW behavior, anchored to a durable
marker because the committed phase and scorecard states do not encode
teardown and must not be guessed from:

- the driver keeps a per-repetition TEARDOWN LEDGER in the cohort's
  state directory: it appends `teardown-due <target>` BEFORE advancing
  any completion state (write-ahead), and `teardown-done <target>`
  only after a verified teardown. THE SCORECARD IS COMPLETION STATE
  (SLC-R6-005): the committed driver creates the scorecard while the
  phase still reads grading and REFUSES an existing scorecard on
  re-entry, so `teardown-due` must precede scorecard creation, not
  merely the phase transition — otherwise the exact
  scorecard-then-crash interval wedges again. Grading does not
  require the target's supervision, so a recovery teardown between
  the due record and a re-graded scorecard is safe;
- at repetition and cohort completion it performs the teardown it just
  recorded as due, loudly (D-5);
- at EVERY driver entry, before other work, it recovers: every
  due-without-done target that still exists is torn down then. A
  driver killed between the scorecard write and the phase transition
  is therefore repaired by the next invocation — the ledger, not the
  phase or the scorecard, is the authority;
- RECOVERY CONTINUES COMPLETION, NOT JUST TEARDOWN (SLC-R10-004):
  the committed grading path REFUSES an existing scorecard, so
  ledger recovery alone leaves the exact scorecard-then-crash shape
  wedged in `grading`. The rule: a driver entering `grading` and
  finding a scorecard VALIDATES IT BY FULL REPETITION IDENTITY
  (SLC-R11-005) — schema-valid AND equal on every identity field
  the committed driver already compares: cohort id, repetition
  index, repetition count, and reviewed commit. The commit alone
  cannot identify a repetition (every repetition in a cohort shares
  it); AND the reuse demands the crash's own fingerprint — a
  `teardown-due` record for THIS repetition in the ledger
  (SLC-R13-002: full identity alone also matches a scorecard from a
  PRE-LEDGER deployment or foreign copy, and reusing those would
  grade unverified state). Full identity plus the ledger trace is
  the crash residue and is REUSED, advancing the phase as if just
  written; full identity WITHOUT any ledger trace parks the
  repetition loudly for a human (D-5) — re-provisioning is the
  remedy, per the KI-30 precedent. One failing
  identity is FOREIGN, not residue: the scorecard-exists refusal
  stands for it, loudly. Only a scorecard with the RIGHT identity
  but invalid schema is archived aside and re-extracted — and THE
  ARCHIVE LIVES OUTSIDE THE ACTIVE SCORECARD DIRECTORY
  (SLC-R11-006): `archived-scorecards/<repetition>-<timestamp>.json`
  in the cohort's state directory, never beside the live ones,
  because the comparer requires the active directory to hold
  EXACTLY the numbered scorecards 1..N and any extra file there
  makes the whole cohort permanently incomparable;
- the ledger is a DURABLE FILE WITH THE REGISTRY'S OWN RULES
  (SLC-R7-006): REG-1's framing, torn-run tolerance, and repair apply
  to it verbatim. A partial `teardown-due` line is a torn fragment
  and therefore NOT a due record — the completion path re-runs and
  writes it again — so a driver killed mid-append neither wedges the
  cohort nor triggers an early teardown;
- a cohort abandoned mid-wait keeps its supervision armed on a live
  target: bounded (one owner within cap K, idling), visible in the
  registry, and the human's call — approval-wait is human territory by
  definition.

## D-4. The registry is the machine-wide view; the arming gate and the janitor enforce it

The registry contract — location, canonical paths, framing, schema,
guarded appends, reduction, locking, failure and corruption rules —
is its own document: `docs/design/supervision-registry.md`.

THE ARMING GATE (SLC-R3-011, SLC-R4-002, SLC-R4-006), under the
REGISTRY LOCK so the count and the reservation are one atomic step. A
SLOT IS CONSUMED BY (SLC-R5-001, SLC-R5-002, SLC-R6-003, SLC-R6-007):
a live-VERIFIED claim; an OPEN RESERVATION — a reservation holds its
slot until its claim is CLOSED, not merely until grace expires, or
two armers race seven claims to nine owners; a claim whose owner
liveness is UNKNOWN — indeterminacy counts TOWARD the cap, so the
gate can only under-admit, never over-admit; and a CLOSED CLAIM STILL
MARKED SWEEPABLE — a possibly-surviving set holds its slot until
`swept` proves it gone, or repeated SIGKILLed owners accumulate
component leaders outside the cap. Grace expiry makes a stalled
establishment ACTIONABLE, and the GATE ITSELF RESOLVES actionable
claims it encounters — reap or compact under its lock, by the kill
proof below — before granting slots, so admission never outruns
cleanup (SLC-R6-003). Only a PROVABLY DEAD owner frees its slot, and
closing one is not bookkeeping alone (SLC-R5-012): the close also
STOPS the claim's recorded components and groups under the kill
proof — death-only, identical in legality to takeover cleanup —
because a closed claim must not hide surviving sets. At or above K
consumed slots machine-wide, REFUSE, printing the claims and pointing
at `reap-orphans.sh`.

CLAIM-OPENING APPENDS ARE GUARDED (SLC-R5-003, SLC-R5-004): `armed`
may only be appended while its `arming` reservation still reduces as
open — checked under the same lock as the append. A paused armer whose
reservation was reaped or compacted away has its `armed` REFUSED; it
tears down the owner it launched and fails. This retires the
tombstone problem: a late `armed` cannot reopen anything because it is
refused at the door, not absorbed at reduction — so compaction may
drop closed claims freely.

THE JANITOR: `scripts/agents/reap-orphans.sh`, run by hand and by the
suite AT START (the run that leaked is the one that never reached its
cleanup):

- reduces the registry under its lock, then tears down, in order: live
  claims whose CHECKOUT NO LONGER EXISTS (it cannot call `--shutdown`,
  which resolves scripts inside the vanished checkout — SLC-R1-011);
  OPEN claims on LIVE checkouts whose owner is PROVABLY DEAD (reason
  `owner-dead` — SLC-R9-005: D-7 and REG-2 always named this case and
  this list omitted it, leaving two implementations — the arming gate
  resolves such claims only when an arm happens to run, and a dead
  owner's detached components must not run on until one does);
  `arming`-only claims past grace with live tag-matched processes
  (reason `establishment-orphan` — SLC-R4-005, SLC-R5-015); closed
  claims still marked sweepable (SLC-R4-012); and, on LIVE checkouts
  only, bound custody claims whose custodian is provably dead and
  which no live announced session is using (D-3);
- WITHIN A CLAIM, THE OWNER DIES FIRST (SLC-R7-001) — the incident's
  own remediation, now a rule: killing components while a live owner
  watches only makes it respawn them, and D-2's gating lets it append
  a fresh `relaunched` and launch during the sweep. So: kill the
  owner (recorded identity or owner-signature match), THEN the
  components and groups, then RE-REDUCE the registry and verify
  against the POST-KILL reduction — never a pre-kill snapshot — so a
  relaunch that raced the sweep is seen; any residue sets
  `sweepPending` rather than closing clean;
- THE OWNER'S OWN TERMINAL WINS (SLC-R10-003): the stop discipline
  stays TERM-then-KILL — graceful teardown is worth having — so a
  healthy owner can complete held-identity teardown and append
  `exited terminated` between the janitor's TERM and its `reaped`.
  That is not a race to resolve but an outcome to accept: the
  post-kill RE-REDUCTION decides what the janitor appends. A claim
  still OPEN gets `reaped` with the janitor's causal reason; a claim
  the owner closed itself gets NO post-terminal append except the
  one REG-3 already permits — the janitor verifies survivors and
  appends `swept` if the self-close was sweepable, and treats a
  clean self-close (`teardownComplete: true`, survivors verified
  none) as the sweep outcome it wanted, reported, not an append
  failure. Terminal absorption (REG-3) is thereby never violated
  and the janitor never reports failure because the owner did its
  job politely;
- a completed sweep is recorded with a `swept` append, the one
  post-terminal update, so a swept claim stops being re-swept
  (SLC-R5-015);
- on a CORRUPT registry (REG-5) it kills NOTHING and reports loudly —
  fail closed both ways: no arming, no reaping, until a human repairs
  (SLC-R4-013).

THE JANITOR KILLS ONLY WHAT IT CAN PROVE (SLC-R4-011, SLC-R5-010,
SLC-R5-011), and the proof rule lives in ONE PLACE: REG-6
(SLC-R9-001 — this section previously restated the rule
disjunctively, "recorded identity OR invocation signature", which
authorized a kill on pid + start alone: exactly the same-second
pid-reuse hole SLC-R8-006 closed. Two normative statements of one
rule is how the hole opened, so REG-6 is now the only one).
Summarized here with no independent force: proof is the TRIPLE —
pid, pidStartedAt, AND claim-consistent argv (a known invocation
shape, component or owner — SLC-R6-004 — with the claim's tag in its
tag position; for helpers, membership of a proven group) — captured
in one observation immediately before signalling. Recorded identity
alone never suffices; a shell, grep, or editor that merely mentions
a tag matches no shape. A recorded process group is killed THROUGH
its proven members or live proven leader, never by group number
alone. Whatever cannot be proven is REPORTED, not killed; the design
accepts that residue because D-2's single-flight rule makes it
bounded-lifetime.

## D-5. Failures stop being swallowed, and the record survives the checkout

- RC-3 gets a changed outcome (SLC-R1-013): `--shutdown` failures in the
  suite's cleanup and in cohort teardown are REPORTED — a failed teardown
  prints the surviving owner's identity and the run's exit reflects it.
  Silent `|| true` on a custody teardown is how leaks became invisible.
- Terminal owner events (exit reason, GIVING-UP diagnosis, janitor
  kills) are appended to the REGISTRY ITSELF as `exited`/`reaped`
  records with a `reason` field and the teardown outcome (SLC-R2-010,
  SLC-R3-008, SLC-R4-012) — one file, one schema, outside every
  checkout, so an owner whose checkout vanished still has somewhere to
  speak. No second log format is invented.
- EVERY teardown path that RUNS reports (SLC-R2-009): validation
  cleanup that skips a deleted checkout, a cohort teardown whose
  shutdown fails, and a shutdown that returns nonzero all print the
  surviving owner's identity and mark the run. The teardown path that
  DOES NOT run — a killed driver — is covered by the teardown ledger's
  entry recovery and, for fixtures, custody (D-3), not by a report
  nobody survives to print (SLC-R3-012, SLC-R4-009, SLC-R5-007).
- The `--shutdown` path appends `exited` with reason `shutdown`
  (best-effort), so a normal stop is distinguishable from a death in
  the reduced registry (SLC-R4-006).

## D-6. Numbers are decisions, not examples (SLC-R1-010)

Fixed here so no implementer chooses: breaker N = 5 consecutive
incrementing observations at base interval; establishment deadline =
5 observations (SLC-R4-005); relaunch backoff = interval × 2^(k-1)
capped at 10 minutes, gating relaunches only; per-checkout
process-group ceiling = 12, refused at the ceiling at launch and
enforced by stop-the-set on the observation after any overshoot
(SLC-R4-010); machine-wide cap K = 8 consumed slots, counting
live-verified claims + open reservations + unknown-liveness claims +
closed sweepable claims (SLC-R5-001, SLC-R5-002, SLC-R7-N001 — one
statement of the four classes, per SLC-R9-N001); registry grace
window for `arming`-only and unbound-custody staleness = 10 minutes;
component stop ceiling = 5 seconds scaled (the committed supervision
wait cap); shutdown owner-stop wait = 20 seconds scaled — two
component ceilings plus terminal-append slack — and the
shutdown-intent latch window equals that wait (SLC-R9-004);
shutdown's checkout-wide loop retries at most 3 identities and its
post-stop settle window = 5 seconds scaled (SLC-R10-002);
reap-intent grace before the re-check = 10 seconds scaled, a
reap-intent marker is stale past 10 minutes — the registry's
standing grace number (SLC-R10-005) — and a joiner waits at most
10 seconds scaled for the marker's held flock before refusing the
join (SLC-R12-002); owner
tags carry a 4-hex-char random suffix and a reservation is REFUSED
if its tag is SEEN AT ALL in the current reduction, open or closed —
the armer regenerates its suffix and retries (SLC-R5-013,
SLC-R7-008); registry compaction triggers at 1 MiB or 10,000
records, performed by any writer under the lock (SLC-R5-018);
janitor runs at suite start and on demand. Revisable by a recorded
ruling, but not open questions in the implementation.

## D-7. What must not change

- Detachment (`setsid`) and self-healing for a LIVE checkout.
- Observation stays separate from killing OTHER processes: the census
  still reports UNTRACKED without acting. D-1's check governs the
  owner's own life; the janitor kills only under its proof rule (D-4),
  and only for: vanished checkouts, orphaned establishments, admitted
  incomplete teardowns, provably dead owners' surviving sets, and
  provably dead custodians with no live user.
- Death-only takeover of the checkout lease. The SUPERSEDED branch is
  not a takeover: the superseded owner leaves voluntarily.
- The fingerprint's re-arm job (`verify_armed` comparing census
  fingerprint against state) stays exactly as is. It is a
  code-staleness detector, not an identity.
