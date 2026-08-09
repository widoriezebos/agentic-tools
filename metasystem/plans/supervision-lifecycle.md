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

## D-1. The owner requires continuous proof of purpose

The owner's loop gains a PRECONDITION evaluated every iteration, before
any self-heal decision. It exits — cleanly, logging why — when any of:

- the supervised repository root no longer exists, or is no longer a
  git repository (its identity is gone);
- the supervision state file it owns no longer exists, or names a
  DIFFERENT owner (a newer generation legitimately replaced it);
- its own announcement/lease record is absent.

These are all kernel-or-file facts, not claims, consistent with the rest
of the system. "My purpose is gone" is a fact the owner can verify; it
should not need anyone's permission to stop.

## D-2. A crash-loop breaker on self-heal

`launch_set` failures become a counted, backed-off sequence rather than
an unbounded retry:

- consecutive failed relaunch attempts back off exponentially from the
  census interval to a ceiling (e.g. 1×, 2×, 4×, … up to 10 minutes);
- after N consecutive failures (proposal: 5) the owner STOPS
  self-healing, records `SUPERVISION-GIVING-UP` with the last error, and
  exits. A supervisor that cannot start its components five times in a
  row is not healing anything; it is a fork bomb with good intentions.

This alone bounds the blast radius of every future variant of this bug,
including ones this design has not imagined.

## D-3. A dead-man's switch for the arming intent

Cleanup paths cannot be the primary defence (RC-3), so supervision gains
an INTENT LEASE independent of any cleanup path: the armed checkout
carries `artifacts/agents/supervision/intent.json` with an expiry, and
the owner exits when it lapses. Renewal comes from the things that
legitimately want supervision — an armed session's stop hook, a running
mission, an active cohort driver — each of which already touches the
checkout regularly.

The honest trade-off to settle in critique: the renewal period must
exceed the longest legitimate quiet interval (a long unattended mission
turn), or the mechanism kills live supervision. A conservative first
value (proposal: 6 hours, renewed hourly) leaks at most one owner for a
bounded time instead of forever, and never fires during a working
session. Alternative for the critique to weigh: make expiry
CONDITIONAL on the checkout also being idle (no live jobs, no mission
state), which is safer but more complex.

## D-4. Fixture and cohort sandboxes stop being special

Every sandbox that arms supervision registers itself in a
checkout-independent registry (`~/.metasystem/armed-checkouts.jsonl`,
append-only, one line per arming), and `scripts/agents/reap-orphans.sh`
— runnable by hand, and run by the suite at START — stops any owner
whose checkout is gone. This is the janitor that exists because D-1 might
be bypassed by a future path nobody anticipated, and because today there
is NO way to answer "what supervision is running on this machine?"
without grepping `ps`.

The suite runs it at start rather than only at end: a suite that was
killed last time is exactly the case that leaked, and the next run is the
first moment anyone can clean it.

## D-5. What must not change

- Detachment (`setsid`) and self-healing for a LIVE checkout. Supervision
  outliving its launching shell, and restarting components that genuinely
  crash, are the properties that make it worth having.
- Observation stays separate from killing: the census still reports
  UNTRACKED processes without acting on them. The owner's new exits are
  about ITS OWN life, not about judging others — the flight recorder's
  witness rule and this are the same discipline.
- The death-only takeover semantics of the checkout lease.

## Proof

- Purpose gone: arm supervision in a temp checkout, delete the checkout,
  and the owner exits within one interval — with no relaunch attempt and
  a logged reason. (Today: it relaunches forever.)
- Generation replaced: a newer owner claims the state file; the older
  owner exits rather than competing.
- Crash-loop breaker: with components made unstartable (chmod 000 on the
  watcher script), the owner backs off and exits after N attempts,
  recording SUPERVISION-GIVING-UP; total spawned processes across the
  episode are bounded and asserted.
- Live supervision unharmed: a healthy checkout with a component killed
  by hand relaunches it exactly as today, and a long quiet mission turn
  does not trip the intent lease.
- Janitor: with three leaked owners on deleted checkouts, one
  `reap-orphans.sh` run stops exactly those three and leaves the live
  one; the suite's start-of-run invocation does the same automatically.
- Load regression: the incident's shape — N owners on deleted sandboxes —
  cannot produce more than a bounded number of processes in a fixed
  window, asserted as a number rather than a hope.
