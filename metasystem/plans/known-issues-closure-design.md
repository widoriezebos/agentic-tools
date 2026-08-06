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

Change: remove instance-derived inputs from `fingerprint()` in
`process-census.py`; arming records and dispatch comparisons use the
identity-only fingerprint; the census verdict retains its component-health
duties unchanged.

Proof: a reproduction harness OUTSIDE the full suite (arm a sandbox repo with
fixture-fast intervals, kill a component, wait for self-heal, dispatch) that
fails on the old code and passes on the new, kept as a suite fixture; then
three consecutive full-suite greens plus one under artificial CPU load — the
final proof that also releases the parked batch.

## KI-9 — delegates cannot commit in their worktrees

Root cause: a worktree commit writes outside the sandbox's writeRoots — the
shared object store, the worktree's metadata directory, and its own branch
ref all live under the template's `.git`.

DECISION: grant the minimum that makes a delegate's OWN commits possible and
nothing else: `--worktree` dispatches add three precise writeRoots —
`.git/objects/` (content-addressed, append-only in practice),
`.git/worktrees/<name>/` (the worktree's own metadata), and
`.git/refs/heads/agent/<job-id>*` (its own branch and nothing beside it).
Orchestrator-owned merges and the conformance diff stay exactly as they are;
the settled workspace contract already anticipated delegate checkpoint
commits, and per-job authorship (shipped) stamps them honestly.

Change: `dispatch.sh` permission expansion for worktree jobs; the KI-9
register row closes; the WORKTREE-BEHIND warning stays (syncing remains the
orchestrator's job).

Proof: fixtures — a fake worktree job runs `git commit` successfully on its
own branch; an attempted write to another ref path is refused by the
sandbox; conformance still computes the identical boundary.

## KI-11 — dispatch refuses a census one second past its interval

Root cause: freshness is a point predicate against a live, ticking process.

DECISION: staleness within one interval of grace means WAIT, not refuse:
dispatch blocks up to one full interval for the next census; it refuses only
past 2x the interval, which genuinely indicates dead supervision. No
configuration knob — the interval itself is the unit.

Change: `require_fresh_census` waits bounded; refusal message states the age,
the bound, and the arming remedy (F-5's message rule).

Proof: fixtures — stale-but-live census: dispatch waits and proceeds;
stale-and-dead: refuses with the stated message.

## KI-8 — permission envelopes are asserted, not measured

Root cause: the recorded envelope is what dispatch REQUESTED; nothing ever
observes what the sandbox actually enforced.

DECISION: measure at the cadence where it is cheap and meaningful — the
adapter self-test, once per runtime version+config fingerprint, not per
dispatch. Each real adapter's `selftest` gains behavioural probes: attempt an
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

Proof: the flag exists, is green in a normal environment with ps removed
from PATH (simulating the sandbox), and the suite asserts the section list
it prints matches the sections actually skipped.

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
- **KI-2** (suite wall time growth): RETIRE as superseded — the load-immunity
  rework re-baselined the suite; the register keeps the new baseline number
  and the watch lives in the benchmark watches, where it now belongs.
- **KI-3** (BSD diff symlink diagnostics): FIX now — the adoption fixture
  compares with `diff -r --no-dereference` equivalents or filters the known
  benign lines EXPLICITLY BY PATTERN; exit codes already correct.
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
