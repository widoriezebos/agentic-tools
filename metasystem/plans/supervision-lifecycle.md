# Supervision must be able to die

- Goal and current status: give the supervision owner a lifetime. Today it
  is immortal by construction and self-heals against a purpose that may no
  longer exist, which turned leaked fixture sandboxes into an unbounded
  respawn loop that took this machine to load 134 with 2,126 processes.
  DESIGN DRAFT 2026-08-09, written from a live incident; not yet critiqued.
- Next step: critique this design with sol
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

EXIT CONDITIONS, each decidable from a fact the owner holds:

- E1 PURPOSE GONE: the supervised repository root does not exist, or is
  not a git repository.
- E2 SUPERSEDED: the supervision state file names a generation HIGHER
  than the one this owner published — a legitimate successor exists.
- E3 REVOKED: the state file existed and now does not (a shutdown or a
  deliberate teardown removed it).

EXIT IS A TEARDOWN, NOT A RETURN (SLC-R1-001). Before exiting, the owner
stops exactly the components IT launched, identified by the tags it holds
and verified live by (pid, pidStartedAt, tag) — never components named by
a state file it no longer owns, so a departing owner can never kill a
successor's watcher. Under E2 it stops ONLY its own tagged components and
leaves the successor's alone; under E1 the components are dead already
and the stop is a no-op that still runs for certainty.

INDETERMINACY PREFERS THE LEAK (SLC-R1-003): a check that cannot be
evaluated — permission error, I/O failure, unreadable state — is NOT a
proof of absence. The owner keeps running and records the failed check.
Killing live supervision on a transient filesystem error is worse than
carrying one leaked owner, which D-2 bounds anyway.

ESTABLISHMENT IS NOT REVOCATION (SLC-R1-004): E3 applies only after this
owner has PUBLISHED its state at least once. Before first publication a
missing state file is startup, and D-2's counter governs.

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

BLAST-RADIUS BOUND (SLC-R1-006): the bound that matters is machine-wide,
not per-owner. `launch_set` therefore refuses to start a component while
the checkout already has more than a fixed number of live processes
carrying this owner's tags — a leaked owner cannot multiply helpers even
if every other rule fails. The Proof asserts a NUMBER of processes across
a fixed window, not merely that the owner eventually stops.

## D-3. No dead-man's switch

The draft's intent lease is DROPPED. Round 1 showed it cannot be made
safe with the mechanisms available: nothing guarantees renewal (stop
hooks run only at turn end — SLC-R1-007), its schema, renewers,
generation binding, and malformed-record behaviour were undefined
(SLC-R1-008), and the idle-predicate alternative misreads completed
missions whose state files persist (SLC-R1-009). A mechanism that can
kill healthy supervision is a worse bug than the one it fixes.

D-1 (purpose) and D-2 (breaker) already terminate every owner whose
checkout is gone or whose components cannot stay alive; D-4 catches
whatever escapes both. The design accepts that an owner supervising a
LIVE, HEALTHY, but permanently unattended checkout runs forever — that is
supervision working as intended, and stopping it is the human's call.

## D-4. The janitor stops orphans directly

`scripts/agents/reap-orphans.sh`, run by hand and by the suite AT START
(the run that leaked is the one that never reached its cleanup):

- reads the append-only registry `~/.metasystem/armed-checkouts.jsonl`
  (one line per arming: checkout path, owner pid, pidStartedAt,
  owner_tag, armedAt);
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
  appended to the REGISTRY and to the user-level log outside the
  checkout (SLC-R1-012), because an owner exiting because its checkout
  vanished cannot log inside that checkout.

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
