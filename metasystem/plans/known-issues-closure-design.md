# Known-Issues Closure Design: an empty register before the next run

- Goal and current status: every OPEN row in `plans/known-issues.md` gets a decision and a fix with proof, every ACCEPTED row gets re-judged, and the register reads empty-of-OPEN before any benchmark runs, at the human's direction. Status: DRAFT, awaiting critique.
- Next step: design critique by Codex until closed by join; then implement in the stated order.
- In flight right now: nothing
- Waiting on the human: ratification through accepting this design after critique

## Scope and standard

Eleven rows are open or accepted; per row this design states the root cause,
the DECISION (where one is owed), the change, and the proof. A row may close
three ways: FIXED with a fixture, ACCEPTED with a written reason and a reopen
trigger the register keeps, or RETIRED when reality already closed it. What
no row may do is stay OPEN with a shrug. The parked F-4 + load-immunity batch
pushes as part of this work, gated by the same final proof.

## KI-18 — the fingerprint instability (first; it blocks the gate itself)

Root cause hypothesis, to be CONFIRMED by a reproduction harness before the
fix lands: the census fingerprint hashes live supervisor instances (via
`state.json` components) alongside code, signatures, and configuration; under
fixture-fast heartbeat thresholds a component self-heal rewrites instances
mid-suite, the armed fingerprint stays frozen at arming, and every later
dispatch refuses — nondeterministically, because self-heals are.

DECISION: **the fingerprint covers what was armed, not who is currently
running it.** Identity = code + signatures + configuration. Component
instances are LIVENESS, already proven separately by the census verdict
itself (a census cannot report SUCCESS over dead or foreign components, and
custody checks catch impostors). A self-heal therefore never invalidates
dispatch. The alternative — atomically re-recording the armed fingerprint on
every heal — was rejected: it makes the arming record mutable by an
unattended process, which is the pattern this repository keeps burying.

The critic proved the replacement duty I cited does not exist (KC-1-1):
nothing at dispatch time ties a SUCCESS census to the CURRENT armed
generation. So the decision gains its missing half: `fingerprint()` drops
instance-derived inputs, AND the census output gains the armed generation it
served (the arming record's `startedAt` + owner identity, echoed verbatim
from `state.json` into `last-census.json`); dispatch compares that
generation against the arming record explicitly. Identity by fingerprint,
generation by declaration, liveness by verdict — three separable checks
instead of one hash conflating them.

Change: `fingerprint()` inputs; census emit; dispatch's freshness check
gains the generation comparison with its own message.

Proof, two-sided per KC-1-8: the reproduction harness (fixture-fast sandbox,
kill a component, self-heal, dispatch) runs TWENTY iterations on the old
code and must refuse in at least five — establishing it reproduces the
class — and twenty on the new code with zero refusals; then three
consecutive full-suite greens plus one under artificial CPU load. The
harness stays as a suite fixture at reduced iteration count.

## KI-9 — delegates cannot commit in their worktrees (decision REVERSED at round 1)

The draft granted three writeRoots; the critic showed `.git/objects` is an
isolation hole (deletion and overwrite of every loose object, alternates,
packs — KC-1-2), the set was incomplete anyway (reflogs — KC-1-3), and the
ref-glob is inexpressible in the shipped adapters. All true. No sandbox
widening.

DECISION: KI-9 stays ACCEPTED — delegates never run git writes — and the
checkpoint need is met on the orchestrator's side of the boundary, where
git already lives: a new `dispatch.sh checkpoint --job <id>` verb commits
the job's current worktree state on its own agent branch with per-job
authorship (shipped) and a checkpoint trailer. Delegates REQUEST a
checkpoint through their return (files as the only interface, as
everywhere); the orchestrator or the runner grants it with one command.
The escape surface stays exactly zero.

Change: the `checkpoint` verb; the implementer return schema gains an
optional `checkpointRequested` boolean; the register row stays ACCEPTED
with the verb named as its mitigation.

Proof: fixtures — the verb commits a dirty worktree on the right branch
with job authorship and touches no other ref; a second invocation with a
clean tree is a recorded no-op; the schema round-trips.

## KI-11 — dispatch refuses a census one second past its interval

Root cause: freshness is a point predicate against a live, ticking process.

DECISION (single rule, KC-1-5): dispatch computes one hard deadline —
census age must be under 2x the interval, additionally capped at 180
seconds absolute so an absurd configured interval cannot stall dispatch
indefinitely. If the current age is below the deadline, dispatch waits for
the next census only up to that same deadline; at or past it, refuse. One
number, both branches.

Change: `require_fresh_census` waits bounded; refusal message states the age,
the bound, and the arming remedy (F-5's message rule).

Proof: fixtures — stale-but-live census: dispatch waits and proceeds;
stale-and-dead: refuses with the stated message.

## KI-8 — permission envelopes are asserted, not measured

Root cause: the recorded envelope is what dispatch REQUESTED; nothing ever
observes what the sandbox actually enforced.

DECISION: measure at the cadence where it is cheap and meaningful — the
adapter self-test, once per runtime version+config fingerprint AND PER
SHIPPED PRESET (KC-1-4: `none` and `workspace` produce materially different
sandboxes, so each is probed; a custom envelope file is recorded as
`measured: false` in the job record, honestly unmeasured rather than
falsely covered). Each real adapter's `selftest` gains behavioural probes: attempt an
out-of-root write, attempt network under deny, attempt a read outside
readRoots; record observed outcomes in the capability snapshot as
`measuredEnvelope`. Dispatch records then cite the snapshot that measured
the preset they requested. Per-dispatch behavioural probing was rejected as
pure overhead: the sandbox implementation does not vary between dispatches
of the same runtime version and preset.

Change: `runtime-common.sh` probe helpers; fake adapter implements them
honestly (its sandbox is the suite's); real adapters implement in their
selftest paths; snapshot schema gains the field.

Proof: suite fixture over the fake adapter's measured envelope (deny paths
observed as denied); codex selftest run once for real by the orchestrator;
devin deferred to its first selftest (already on the board).

## KI-15 — delegates cannot run the gate of record

Root cause: census fixtures need real process visibility; sandboxes deny ps.
This is a permissions fact, not a bug.

DECISION: ACCEPT the boundary, but give delegates a runnable surface so
"cannot run the suite" never again means "cannot verify":
`validate-metasystem.sh --delegate-scope` runs everything that needs no
process visibility (audit, static checks, schema and role validation,
contract and state fixtures, grammar checks) and prints plainly which
sections the orchestrator still owes. Briefs cite the flag; the KI row moves
to ACCEPTED with the flag as its mitigation and "sandboxes gain process
visibility" as the reopen trigger.

Proof: the flag exists and is green under an honest simulation (KC-1-6): a
PATH-shimmed `ps` that EXISTS and fails with `Operation not permitted` on
stderr and a nonzero exit, mimicking the observed seatbelt behaviour rather
than command absence; the suite asserts the printed section list matches
the sections actually skipped.

## KI-7 — the launch window race

Root cause: a sweep can classify a job as lost between record creation and
handshake.

DECISION: records carry their handshake budget from birth; the sweep never
touches a pending record younger than that budget. Simple age arithmetic,
no new state.

Change: reaper/sweep guard in `dispatch.sh`.

Proof: fixture — a pending record inside its window survives a sweep; one
past its window is reaped with the existing classification.

## KI-1 through KI-5 and KI-10 — the aged residue, re-judged

- **KI-1** (durationMs vs durationSeconds naming): FIX now — rename the doc
  reference to match the artifact; one line, fixture greps both.
- **KI-2** (suite wall time growth): ACCEPT with a measured trigger
  (KC-1-7: retirement was unearned — nothing showed the growth ceased). The
  closure implementation RECORDS the post-rework baseline from three timed
  runs in the register row, and the reopen trigger is mechanical: a green
  suite exceeding 1.5x that baseline.
- **KI-3** (BSD diff symlink diagnostics): FIX now, without the unsafe
  choices the draft allowed (KC-1-9: BSD diff lacks --no-dereference, and
  filtering diagnostics can hide skipped content): the fixture compares by
  an explicit walk — `find` both trees, compare the sorted path sets, `cmp`
  regular files pairwise, and compare symlink TARGETS with `readlink`,
  never traversing them. Deterministic on every platform the suite runs on.
- **KI-4** (census cost proportional to machine processes): ACCEPT with the
  existing watch and a reopen trigger (census duration exceeding its own
  interval); measuring on other machines belongs to real adoption.
- **KI-5** (one S4-8 timeout under load): RETIRE — subsumed by the
  load-immunity rework; if it recurs post-KI-18 it reopens as its own row.
- **KI-10** (fixtures need real process access): keep ACCEPTED, unchanged
  reason; it is the same boundary as KI-15 seen from the suite's side.

## Implementation order and the final proof

KI-18 first and alone (it unblocks the gate; its harness precedes its fix).
Then one brief for KI-9 + KI-11 + KI-7 (all dispatch.sh), one for KI-8 (adapters),
one for KI-15 + the KI-1/2/3/4/5/10 register pass (suite and docs). Every
brief cites this design; the loop applies unchanged.

The final proof, releasing everything at once including the parked batch:
three consecutive full-suite greens, one additional green under artificial
CPU load, the kit gate green, and a register containing zero OPEN rows.

## Completion

Complete when the final proof holds and the register's every row reads
FIXED, ACCEPTED-with-trigger, or RETIRED-with-reason. Then this file closes
into `development/` as a finished report, and the benchmark the human is
waiting to run gets a machine with an empty conscience.
