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

The generation identifier is a monotonic counter the ARMING side owns
(KC-2-1: startedAt plus owner identity is not unique across heals — whole
seconds, constant owner): `state.json` carries `generation`, incremented exactly when a new component
set is PUBLISHED — a fresh arming or a self-heal that replaces a component —
and never by an arm call that joins an already-live owner without writing
state (KC-4-5), and the census
echoes the value it observed. Liveness gets a real owner rather than an
assumed one (KC-2-2), and coherence is structural rather than hoped for
(KC-3-2): the census reads `state.json` ONCE into memory, and verifies owner,
watcher and reaper aliveness with matching tags against THAT snapshot,
echoing that same snapshot's generation. A concurrent rewrite therefore
yields a consistent older pair, never a mixed one, and the next census
carries the new generation. And the heal window follows KI-11's
simplification rather than the deleted wait (KC-6-1): a census carrying an
older generation is treated exactly like a stale census — refuse, naming the
age, the generations, and the same two remedies. Heals are brief by
construction, so the caller's next attempt succeeds; both the orchestrator
and the runner already retry.

The critic proved the replacement duty I cited does not exist (KC-1-1):
nothing at dispatch time ties a SUCCESS census to the CURRENT armed
generation. So the decision gains its missing half: `fingerprint()` drops
instance-derived inputs, AND the census output gains the armed generation it
served — the `generation` counter defined above, echoed verbatim from
`state.json` into `last-census.json`; dispatch compares that generation
against the arming record explicitly. `arm-supervision.sh` is its sole
writer, on arming and on every component-replacing heal. Identity by fingerprint,
generation by declaration, liveness by verdict — three separable checks
instead of one hash conflating them.

Change: `fingerprint()` inputs; census emit; dispatch's freshness check
gains the generation comparison with its own message.

Proof, two-sided AND state-machine-explicit (KC-1-8, KC-3-8): the harness
runs twenty iterations on the old code refusing at least five times, and
twenty on the new code refusing zero — but sampling alone cannot pass, so on the NEW
implementation only (the old one necessarily violates these, which is the
point — KC-4-6) it additionally ASSERTS, per iteration: generation strictly increases across
every heal; generation never repeats after an owner restart; the census's
echoed generation always equals the state snapshot it verified aliveness
against; and dispatch's comparison refuses when handed a deliberately
back-dated generation. The echo's provenance is observable (KC-5-6, KC-6-2):
the census emits `stateDigest` beside `generation` in `last-census.json` —
the sha256 of the exact state bytes it read — and the harness compares that
digest against the file it wrote. Then three consecutive full-suite greens plus one
under artificial CPU load. The harness stays as a suite fixture at reduced
iteration count with every assertion intact.

## KI-9 — delegates cannot commit in their worktrees (ACCEPTED; the round-1 invention DELETED at round 2)

Round 1 reversed a sandbox widening into an orchestrator-side `checkpoint`
verb. Round 2 killed that too, and correctly: KC-2-4 showed it is a direct
escape — committing in a delegate-controlled worktree fires the shared
pre-commit hook, which executes a guard script the delegate can rewrite,
outside any sandbox. KC-2-5 and KC-2-6 then showed the verb needed a whole
transaction and schema contract to be safe.

DECISION: build nothing. KI-9 is ACCEPTED as the boundary it always was —
delegates never run git writes; they report a file boundary and the
orchestrator integrates. That flow ran roughly fifteen times this session
without a single case where a checkpoint would have helped, so the need was
invented rather than observed. The register row states the boundary, its
mitigation (orchestrator integration, per-job authorship on the resulting
commits, the WORKTREE-BEHIND warning for staleness), and its reopen trigger:
a delegate task that demonstrably cannot proceed without intermediate
commits.

Proof: none needed; nothing is built. The register row's wording is the
deliverable, and the design's scope fence is upheld twice over.

## KI-11 — dispatch refuses a census one second past its interval (SIMPLIFIED at round 5)

Three rounds of critique have now found contradictions in a wait-then-decide
formula (KC-1-5, KC-2-9, KC-3-6, KC-4-4, KC-5-1). The fence applies: the
waiting is what generates the contradictions, and it buys almost nothing.

DECISION: no waiting. The freshness window widens to `2 x interval` with an
absolute ceiling of 180 seconds — one predicate, no branches: fresher than
the window proceeds, at or past it refuses. The refusal message states the
age, the window, and the two remedies (retry in a moment; re-arm if
supervision is dead). A caller who hits a genuinely-stale census retries one
command later, which is what the orchestrator has done all along and what
the runner already does on its own cadence.

Change: `require_fresh_census` computes the widened window; the message
follows F-5's rule.

Proof: fixtures — a census inside the window proceeds; one past it refuses
with the stated message and both remedies; the boundary case at exactly the
window refuses.

## KI-8 — permission envelopes are asserted, not measured (ACCEPTED at round 5, subsystem DELETED)

Rounds 1 through 5 grew a measurement subsystem: per-effective-envelope
keys, snapshot selection semantics, sticky failures, bootstrap exemptions,
recovery transactions, and a runtime-registration change — each fold
answering a real finding (KC-1-4, KC-2-7, KC-2-8, KC-3-3, KC-3-5, KC-4-1,
KC-4-2, KC-5-2, KC-5-3, KC-5-4, KC-5-5) and each introducing the next. Set
against that: KI-8 has caused zero observed harm in the entire life of this
repository. It is a rigor gap, not a defect. The fence is unambiguous here.

DECISION: ACCEPT, with the cheapest mitigation that carries real evidence
and no new machinery:

- The **fake adapter** gains behavioural probes in the suite — it is the one
  runtime whose sandbox the suite fully controls — proving the MECHANISM
  (that a denied write is refused and a denied network call fails) exactly
  where it can be proven without inventing a subsystem.
- **Real adapters declare rather than claim**, with the interface named
  (KC-6-3): the capability snapshot gains one field, `envelopeEnforcement`,
  an object mapping each of `writeRoots`, `readRoots` and `network` to
  `mapped` or `notEnforced`, populated by each adapter's existing `probe`
  from what that adapter actually maps (codex already records readRoots as a
  constraint); the job record's `permissions` block gains
  `enforcementSnapshot`, the id of the snapshot that declaration came from.
  Two fields, both in artifacts that already exist, so "requested" can never
  be read as "enforced".
- The register row states the residual honestly: enforcement is trusted
  runtime behaviour, verified by mechanism on the fake adapter and declared
  per-adapter elsewhere. Reopen trigger: any observed case of a delegate
  exceeding its envelope, which would make measurement worth its cost.

Devin's registration is unaffected (the round-4 change is withdrawn with the
subsystem it served), and no bootstrap, selection, or deadlock question
survives, because nothing gates on a measurement.

Proof: the fake-adapter mechanism fixtures; a fixture asserting the job
record carries the adapter declaration beside the requested envelope; the
register row's wording.

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
  closure implementation records, in the register row, the MEDIAN of three
  timed green runs together with the machine fingerprint and load average
  they were taken under (KC-2-10). The trigger is mechanical because the
  suite itself checks it: a green run whose wall time exceeds 1.5x the
  recorded median prints a WALL-TIME-REGRESSION notice naming both numbers.
  A notice, not a failure — timing must never turn the gate nondeterministic
  again.
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
