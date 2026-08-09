# Supervision must be able to die

- Goal and current status: give the supervision owner a lifetime. Today it
  is immortal by construction and self-heals against a purpose that may no
  longer exist, which turned leaked fixture sandboxes into an unbounded
  respawn loop that took this machine to load 134 with 2,126 processes.
  Revised against critique rounds 1 and 2 (13 then 10 material, all
  folded).
  The reframe: the owner DOES have identity (its tag and generation), exit
  must be a TEARDOWN of its own components, the breaker must count
  component DEATHS (the draft's launch-failure counter would not have
  caught the real incident), and the dead-man's switch is dropped as
  unsafe.
- Next step: HANDED TO THE HUMAN. The chain is exhausted (13 -> 10 -> 13
  material; round 3 carries EIGHT criticals) and is NOT converging. The
  human takes the plan for an independent critique. Do not implement any
  part of this design in its current state.
- In flight right now: nothing
- Waiting on the human: nothing

## What happened, measured

Mid-session the machine reached **load average 134 with 2,126 processes**.
The cause was not the work in flight. A cleanup found:

- 16 supervision OWNERS alive, only one of which belonged to a real
  checkout; the rest supervised deleted fixture sandboxes under
  `/private/var/folders/.../tmp.*` and finished benchmark cohorts.
- 31 watchers, 54 orphaned `dispatch.sh __lock-owner` helpers reparented
  to launchd, and **149 concurrent `process-census.py` processes**, all
  0–1 seconds old: a continuous spawn, not a backlog.
- Killing the children did nothing — they came back within seconds.
  Only killing the OWNERS first stopped it. After that: load 31,
  1,641 processes, 0 census processes.

## The causal chain, verified in the code

1. `launch_detached` calls `os.setsid()`. The owner and its components
   are deliberately detached into their own session, so they survive the
   process that armed them. This is CORRECT and necessary: supervision
   must outlive the shell that started it.
2. `run_owner`'s loop is `while true; do ... done` with **no exit path**
   (verified: its three `break` statements all leave the inner
   `for component in watcher reaper` loop, never the outer loop) and
   **no check that `$repo` still exists** (verified: zero
   repo-existence tests in the function).
3. The only thing that stops an owner is an explicit
   `arm-supervision.sh --shutdown`.
4. When the supervised directory disappears — a fixture sandbox removed
   by its trap, a cancelled cohort target, a killed suite run, a
   trap stripped by hand during debugging — the components die, because
   their working directory and scripts are gone.
5. The owner's self-heal sees them stale and RELAUNCHES them. They die
   again immediately. The loop repeats at the census interval, forever,
   spawning helper processes on every attempt.

So the defect is not "processes leaked". It is: **a self-healing
mechanism with no liveness condition on its own purpose degenerates into
a respawn loop when that purpose is gone.** The stronger the self-heal,
the worse the failure — which is why the cleanup felt like fighting the
system: it was.

## Root causes, separated

- RC-1 NO SELF-TERMINATION. The owner has no condition under which it
  concludes it is no longer needed. Immortality is assumed rather than
  justified.
- RC-2 NO CRASH-LOOP BREAKER. Repeated failed relaunches are treated
  exactly like a single transient death: relaunch, at full rate, forever.
- RC-3 CLEANUP IS THE ONLY DEFENCE, AND IT IS THE WEAKEST LAYER. Every
  guarantee that supervision stops depends on a caller reaching its
  cleanup path: the suite's `validation_cleanup`, a cohort's shutdown, a
  `trap`. Those are exactly the paths that do not run when a run is
  killed, crashes, or is interrupted by a human — the situations where
  leaks matter most. `--shutdown` failures are also swallowed (`|| true`).

## D-1. The owner exits on facts it can prove, and exit means TEARDOWN

Round 1 corrected two assumptions. First, the owner is not identity-less:
it holds `owner_tag` and the `generation` it published, and it knows the
watcher and reaper tags IT launched. Those are the facts it may reason
from — not "its own announcement", which is undefined (one owner is
shared by every joined session, and a session end deletes only that
session's record).

EXIT CONDITIONS. The owner records, at arming, the checkout's IDENTITY —
not its path: the fingerprint `arm-supervision.sh` already computes plus
the `.git` inode. A path is not an identity (SLC-R2-003): a deleted
sandbox can be replaced by an unrelated repository at the same path, and
an owner must not adopt a stranger.

- E1 PURPOSE GONE: the recorded checkout identity is no longer present at
  the recorded path — the path is gone, is not a git repository, or is a
  git repository with a DIFFERENT fingerprint/inode. This single
  condition subsumes the draft's E3: the state file lives inside the
  checkout, so its disappearance is either this same fact or a deliberate
  teardown, which already stops the owner directly (SLC-R2-005).
- E2 REVIVED-AFTER-REPLACEMENT: the state file names a generation HIGHER
  than the one this owner published. Round 2 was right that a live owner
  cannot normally be superseded (the live-owner lock, D-7) — this
  condition exists for exactly one real case: an owner that was PRESUMED
  DEAD (hung, stopped, unschedulable), had its checkout legitimately
  re-armed by a successor under the death-only rule, and then resumed.
  Such a revenant must leave (SLC-R2-007).

THE EXIT CHECKS RUN EVERY CYCLE, INDEPENDENT OF BACKOFF (SLC-R2-008):
D-2's backoff delays RELAUNCH ATTEMPTS only. An owner in a ten-minute
backoff still evaluates E1/E2 at the base interval, so "checkout deleted
→ owner exits within one interval" holds regardless of breaker state.

INDETERMINACY PREFERS THE LEAK (SLC-R1-003, refined by SLC-R2-006): the
identity check has three outcomes, not two — PRESENT (continue), ABSENT
(a definite negative: the path does not exist, or the fingerprint helper
returns a valid different fingerprint → exit), and UNKNOWN (the helper
errors, permission denied, I/O failure → CONTINUE and record). Only a
definite negative kills.

EXIT IS A TEARDOWN, NOT A RETURN (SLC-R1-001). Before exiting, the owner
stops exactly the components IT launched, identified by the tags it holds
and verified live by (pid, pidStartedAt, tag) — never components named by
a state file it no longer owns, so a departing owner can never kill a
successor's watcher.

ESTABLISHMENT IS NOT REVOCATION (SLC-R1-004): the exit conditions apply
only after this owner has PUBLISHED its state at least once.

## D-2. The breaker counts what actually happened last time

The draft counted failed `launch_set` calls. The real incident had
components that STARTED (each emitted its first heartbeat, so launch_set
"succeeded") and then died immediately because their checkout was gone —
so the draft's counter would never have incremented (SLC-R1-005). The
breaker therefore counts COMPONENT DEATHS, not launch failures:

- every self-heal cycle that finds a component stale increments a
  consecutive counter, whether the previous launch reported success or
  not; a full interval with both components healthy resets it;
- consecutive cycles back off exponentially from the census interval to
  a ceiling;
- after N consecutive stale-cycles the owner records
  `SUPERVISION-GIVING-UP` with the last diagnosis, TEARS DOWN per D-1,
  and exits.

BLAST-RADIUS BOUND, honestly scoped (SLC-R2-001): `launch_set` refuses to
start a component while this CHECKOUT already carries more than 12 live
processes tagged by this owner. That is a PER-CHECKOUT bound and the
design no longer calls it machine-wide — the machine-wide answer is D-4's
janitor plus the registry, which is the only component with a view across
checkouts. The Proof asserts the per-checkout number AND that the janitor
bounds the machine.

## D-3. No dead-man's switch

The draft's intent lease is DROPPED. Round 1 showed it cannot be made
safe with the mechanisms available: nothing guarantees renewal (stop
hooks run only at turn end — SLC-R1-007), its schema, renewers,
generation binding, and malformed-record behaviour were undefined
(SLC-R1-008), and the idle-predicate alternative misreads completed
missions whose state files persist (SLC-R1-009). A mechanism that can
kill healthy supervision is a worse bug than the one it fixes.

COMPLETED BENCHMARK REPETITIONS (SLC-R2-004): a finished repetition's
target still exists, so no exit condition fires — and that is correct.
The COHORT DRIVER tears down supervision when a repetition ends, exactly
as it already provisions it; a driver that fails to do so is a leak the
janitor cannot see (the checkout exists) and is therefore reported by
D-5, not silently tolerated.

D-1 (purpose) and D-2 (breaker) already terminate every owner whose
checkout is gone or whose components cannot stay alive; D-4 catches
whatever escapes both. The design accepts that an owner supervising a
LIVE, HEALTHY, but permanently unattended checkout runs forever — that is
supervision working as intended, and stopping it is the human's call.

## D-4. The janitor stops orphans directly

`scripts/agents/reap-orphans.sh`, run by hand and by the suite AT START
(the run that leaked is the one that never reached its cleanup):

- reads the append-only registry `~/.metasystem/armed-checkouts.jsonl`,
  whose record carries EVERYTHING a direct teardown needs (SLC-R2-002):
  `{schemaVersion, event: "armed"|"exited"|"reaped", checkoutPath,
  repoFingerprint, gitInode, ownerPid, ownerPidStartedAt, ownerTag,
  watcherTag, reaperTag, generation, at}`. One framed line per write,
  same append discipline as the flight recorder;
- for each entry whose CHECKOUT NO LONGER EXISTS, stops the recorded
  owner and its recorded component tags DIRECTLY by (pid, pidStartedAt,
  tag) — it cannot call `--shutdown`, which resolves scripts and state
  inside the vanished checkout (SLC-R1-011);
- verifies each kill by identity before signalling, so a recycled pid is
  never a casualty; entries whose checkout still exists are left
  untouched, whatever their state.

## D-5. Failures stop being swallowed, and the record survives the checkout

- RC-3 gets a changed outcome (SLC-R1-013): `--shutdown` failures in the
  suite's cleanup and in cohort teardown are REPORTED — a failed teardown
  prints the surviving owner's identity and the run's exit reflects it.
  Silent `|| true` on a custody teardown is how leaks became invisible.
- Terminal owner events (exit reason, GIVING-UP, janitor kills) are
  appended to the REGISTRY ITSELF as `exited`/`reaped` records
  (SLC-R2-010) — one file, one schema, outside every checkout, so an
  owner whose checkout vanished still has somewhere to speak. No second
  log format is invented.
- EVERY teardown path reports (SLC-R2-009): validation cleanup that skips
  a deleted checkout, a cohort teardown whose shutdown fails, and a
  shutdown that returns nonzero all print the surviving owner's identity
  and mark the run. "Skipped because the path is gone" is precisely the
  leak case and must be loud, not silent.

## D-6. Numbers are decisions, not examples (SLC-R1-010)

Fixed here so no implementer chooses: breaker N = 5 consecutive stale
cycles; backoff = interval × 2^(k-1) capped at 10 minutes; per-owner live
process ceiling = 12; janitor runs at suite start and on demand. These
are revisable by a recorded ruling, but they are not open questions in
the implementation.

## D-7. What must not change

- Detachment (`setsid`) and self-healing for a LIVE checkout.
- Observation stays separate from killing OTHER processes: the census
  still reports UNTRACKED without acting. D-1's exits govern the owner's
  own life; D-4 acts only on owners it has a registry record for and
  whose checkout is provably gone.
- Death-only takeover of the checkout lease. E2 is not a takeover: the
  superseded owner leaves voluntarily.

## Proof

- Purpose gone: arm supervision in a temp checkout, delete the checkout,
  and the owner exits within one interval, TEARS DOWN its own components
  (none survive), and records the reason OUTSIDE the checkout.
- Generation replaced: a newer owner claims the state file; the older
  owner exits rather than competing.
- Breaker on the REAL shape: components that start, heartbeat once, then
  die every cycle (not merely unstartable) trip the counter and the owner
  gives up after 5 — the incident's own shape, which the draft's counter
  would not have caught.
- Blast radius: across the whole episode the checkout never carries more
  than 12 live tagged processes, asserted as a number.
- Superseded: a higher-generation owner appears; the old owner stops ONLY
  its own tagged components and exits, and the successor's watcher and
  reaper are still alive afterwards.
- Indeterminacy: with the state file unreadable (chmod 000), the owner
  keeps running and logs the failed check — it does not exit.
- Live supervision unharmed: a healthy checkout with a component killed
  by hand relaunches it exactly as today, and a long quiet mission turn
  does not trip the intent lease.
- Janitor: with three leaked owners on deleted checkouts, one
  `reap-orphans.sh` run stops exactly those three and leaves the live
  one; the suite's start-of-run invocation does the same automatically.
- Load regression: the incident's shape — N owners on deleted sandboxes —
  cannot produce more than a bounded number of processes in a fixed
  window, asserted as a number rather than a hope.

## Round 3 findings — NOT FOLDED (13 material, 8 critical)

The chain spent its three rounds and the count rose. These are recorded
verbatim-in-substance so the next reader starts from the real state, not
from an optimistic summary. NOTE R3-001 especially: it is a defect the
round-2 FOLD introduced — using the supervision fingerprint (a hash of
code and configuration) as checkout identity would make any ordinary
code edit terminate healthy supervision, which is the exact failure this
design exists to prevent.

- SLC-R3-001 (critical): Design section D-1 mistakes a mutable supervision configuration hash for checkout identity, so an ordinary code or configuration edit would terminate healthy supervision in a live checkout.
- SLC-R3-002 (high): The proposed fingerprint-plus-inode pair still cannot guarantee that a replacement repository at the same path is never adopted.
- SLC-R3-003 (high): Exit condition E2 has no legitimate production path under the design's death-only takeover rule.
- SLC-R3-004 (critical): The breaker does not actually define component death and conflicts with the indeterminacy proof by treating any stale observation as a death.
- SLC-R3-005 (medium): The breaker counter and exponential relaunch backoff do not define one timeline, so implementations can give up after materially different durations and numbers of relaunches.
- SLC-R3-006 (critical): The twelve-process ceiling excludes the untagged helper processes that created the incident, so it does not bound the relevant blast radius.
- SLC-R3-007 (medium): The ceiling comparison permits a thirteenth tagged process even though the Proof requires that the count never exceed twelve.
- SLC-R3-008 (critical): The registry schema still lacks data required for both direct component teardown and the promised terminal log.
- SLC-R3-009 (critical): The append-only registry has no lifecycle reduction rule, so the janitor cannot determine which armed record is current and which records are already terminal.
- SLC-R3-010 (critical): The registry has no mandatory durability and failure contract even though it is the only machine-wide custody view.
- SLC-R3-011 (critical): The revised design still provides no machine-wide process bound, contrary to D-2 and the load-regression proof.
- SLC-R3-012 (critical): A cohort driver that is killed before teardown cannot produce the report promised by D-3 and D-5, and the design supplies no durable teardown phase for recovery.
- SLC-R3-013 (high): The teardown contract does not cover components spawned by a partially failed launch before their complete identity set is published.

HONEST ASSESSMENT. Three structural problems keep regenerating:
(a) IDENTITY — nothing available cheaply identifies 'the checkout I was
armed for' across deletion and recreation; every candidate (path,
fingerprint, inode) is either mutable, reusable, or both.
(b) MACHINE-WIDE BOUNDS — every bound proposed so far is per-owner or
per-checkout, while the incident was machine-wide; a real bound needs a
shared, durable, reduced registry, which drags in its own lifecycle,
durability, and failure contract (R3-008..011).
(c) DURABLE TEARDOWN — every teardown promise still assumes some process
survives to perform it, which is false exactly when it matters (R3-012).

What is NOT in doubt: the DIAGNOSIS (KI-32) is verified fact, and the
remediation worked in practice — killing owners before components took
the machine from load 134 / 2,126 processes to load 31 / 1,641 and zero
census processes. The operational workaround stands even though the
permanent design does not.
