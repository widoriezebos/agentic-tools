# Glossary

The metasystem's working vocabulary. Each term names a mechanism, not a
metaphor: the definition here is what the code enforces, and the named
scripts are where to look when a definition and reality seem to disagree.
How these concepts compose into one system — the philosophy and the
working model — is `docs/concepts.md`.

NAMESPACING RULE (2026-08-09): the metasystem builds OTHER systems, and a
project may legitimately own the same generic nouns — a task runner has
jobs, an emitter library has events. So: INSIDE A PROJECT WORKSPACE, THE
PROJECT OWNS THE BARE WORDS, and metasystem prose — role briefs, prompt
templates, reports, these docs — speaks the QUALIFIED canonical forms for
the collision-prone subset: **delegate job**, **recorder event**,
**mission gate**, **mission runner**, **host turn**. The already
distinctive names (lease, census, reaper, arming, sweep, fence, flight
recorder) need no qualification — which is the pattern to imitate when
naming anything new. Code identifiers (`jobId`, `artifacts/agents/jobs/`)
are unaffected: paths are already namespaced under `scripts/agents/` and
`artifacts/agents/`, and the confusion this rule prevents lives in prose,
not paths.

## Custody: who may write to a checkout

- **Checkout lease** — the single-writer claim on one repository checkout,
  stored at `artifacts/agents/mains/worktree-lease.json`. One holder at a
  time; everything else reads. Owned by the engine's `metasystem lease`
  family (`internal/lease`).
- **Holder / main** — the process (a "main" agent session) currently
  holding the lease. Identified by kernel facts — pid plus its start time —
  never by claims. A **delegate** is a worker dispatched *by* a holder; it
  never holds or claims a lease itself.
- **Epoch** (`claimEpoch`) — the ownership generation of a lease. It
  increments only on a **takeover**, and every job record is stamped with
  the epoch it was created under; a job whose epoch is below the lease's is
  stale by definition. Distinct from **revision**, which increments on
  every lease write and exists only for compare-and-swap.
- **Lineage** (`ownerLineage`) — the identity of the *logical* writer,
  which can outlive any single process. A mission's staging, resume, and
  every host turn all derive the same lineage (`mission-<hash>`), so when
  one of its processes succeeds a dead predecessor the lease is **renewed**
  — same epoch, in-flight work preserved. A claim from a *different*
  lineage over a dead holder is a **takeover**: epoch bump, stale jobs
  swept. A live holder is never displaced by anyone.
- **Satellite** — a design unit severed from a critique-exhausted parent
  design: evidence-born (accepted findings route to it), inheriting the
  parent's ruling without re-litigation, converging through its own
  critique loop, ordered by dependency on truth. A process mechanism; the
  full definition lives in `skills/design-critique/SKILL.md`.
- **Progress** — value produced and proven mechanically, never an
  assertion. Mission level: the gate metric beating its best. Chain
  level: an accepted certification in the durable turn log — the one
  observable satellite 4 settled after rejecting per-activity proxies
  (`docs/patience.md`, `plans/patience-satellite-4.md`).
- **Patience** — how much observation without progress is tolerated
  before a verdict, set per role and (runtime, model) pair: slower
  progress is still progress. A last defense, never a pacing target
  (`docs/patience.md`; the shipped degenerate case is the stop-loss
  core's no-gain budget).
- **Stall** — the verdict when patience is exhausted with nothing else
  to blame. Mission level: the fuse parks vocally and a human resets
  via a ledger-recorded answer. Chain level: vocal only — annotation
  and prompt line, never a park (`docs/patience.md`,
  `plans/patience-satellite-4.md`, `docs/design/stop-loss-core.md`).
- **Sweep** — the takeover's cleanup: every non-terminal job stamped with
  an older epoch is failed with `stale-claim-epoch`, so an abandoned
  session's children cannot keep mutating a checkout that changed hands.
- **Handshake** — the window between launching a delegate and its runtime
  reporting a live session. A job that never completes the handshake is
  failed as `handshake_timeout` by whoever holds the stamped deadline.

## Supervision: who watches the processes

- **Arming** — starting supervision for a checkout
  (`scripts/agents/arm-supervision.sh`): announce the session, claim or
  join the lease, launch the watcher and reaper, and wait for a first
  healthy census. "Armed" means the checkout is being watched.
- **Census** — the periodic scan (the engine's census, run by
  `scripts/watch-background-jobs.sh --census`; `internal/census`) that classifies every process
  touching the checkout: **ANNOUNCED** (a registered main), **CUSTODY**
  (owned by a tracked job), or **UNTRACKED** (nobody can account for it —
  surfaced, never killed). Each scan ends in a **verdict**: SUCCESS or
  CENSUS-FAILED.
- **Watcher** — the long-lived supervision component that runs the census
  on an interval.
- **Reaper** — the supervision component that applies *verdicts* to jobs
  whose processes are gone or whose budgets are spent: `process-lost`,
  `budget-cap` (timeout), `abandoned-setup`. Budget expiry outranks
  process-lost for a job that actually ran, so the verdict is
  deterministic rather than a race.
- **Heartbeat** — a small file each long-lived component rewrites on every
  cycle (`*.heartbeat.json`), carrying its pid and start time. Staleness
  plus a dead pid is how a component is *proven* dead rather than assumed.
- **Wind-down** — terminating a job's or turn's process group, permitted
  only while ownership of that group can be proven (the tag is visible on
  a live member). Lost proof means: stop signaling, let the census surface
  the leftovers. Never kill what you cannot prove is yours.
- **Outage mark** — the shared record of a model-provider outage
  (`artifacts/agents/outage.json`; `internal/outage`), written when a
  provider call comes back overloaded (529/overloaded/5xx) and cleared by
  any provider success. While it stands, the steward's patience clocks
  pause, revival is held, and overloaded host exits stay off the
  mission runner's host-failure breaker — the provider's weather is
  nobody's failure. A mark nobody feeds lapses on its own horizon, so it
  can never permanently blind the steward to a real stall.

## The flight recorder: how a run explains itself

- **Flight recorder** — the append-only event stream
  (`artifacts/agents/events.jsonl`, one per checkout) in which every
  component narrates its decisions: lease claims and renewals, census
  verdicts, job verdicts, turns, phases. One `tail -F` is the live view; a
  collected bundle is the post-mortem. Contract: `docs/design/flight-recorder.md`.
- **Witness, never an authority** — the recorder's one law. No machinery
  decision reads the stream; verdicts come from records, liveness from the
  kernel, custody from the lease. The log may be lossy or absent without
  making the system wrong — which is also why writers need no locks.
- **Event (recorder event)** — one line of the stream: a single decision
  or observation, narrated by exactly one component, named from the
  registry, attributed to its writer by self-reported pid and start time.
  A diary entry, not a message: nothing consumes events, nothing waits on
  them, and losing one loses detail, never correctness. A bare "event" in
  this repository always means this.
- **Event registry** — `scripts/agents/event-registry.json`, the closed
  catalogue of event names, allowed emitters, required ids, and typed
  payloads. An event not in the registry is a bug, not a feature.
- **Emitter** — the never-fail append helper
  (`scripts/agents/emit-event.sh`, a thin wrapper over `metasystem event
  emit`; `internal/events`). An emit may silently lose its own
  event; it may never fail its caller.
- **executionId** — the cohort id, exported by the benchmark driver to
  everything it spawns so one run's events can be joined across the
  harness and its targets. Supervision components never carry it.

## Missions and benchmarks

- **Mission** — a contract-governed unit of autonomous work: a sealed and
  human-signed `mission-*.contract.md`, executed as a series of **turns**
  by a **host** (the orchestrating agent session), which dispatches
  **delegates** for the actual work. Run by
  `scripts/agents/mission-runner.sh`.
- **Cycle** — one plan-act-measure iteration of a mission: the host takes
  a turn, the runner measures the gate, the ledger gains a line. Bounded
  by the cycle fence and the no-gain budget (consecutive cycles without
  measured progress).
- **Mission ledger** — the append-only, per-cycle record of what a mission
  measured (`artifacts/agents/missions/<id>/ledger.md`): classification
  (progress or no-progress), candidate sha, observed gate value. THE
  LEDGER IS TRUTH: state that disagrees with it parks the mission, and
  each anchor commit binds the ledger's bytes to git history. State files
  can be rebuilt; the ledger is the mission's memory.
- **Gate (mission gate)** — the mission's own success measurement, named
  in its contract (`gate.command`, e.g. bm-2's `self-assessment`): the
  runner runs it every cycle and the completion threshold decides when the
  mission is done. Distinct from the verification **gates** below, and
  from the held-out grader, which the mission never sees.
- **Seal / sign / preflight** — the human boundary: sealing freezes the
  contract and prints its hash; the human signs by adding the Approval
  line; preflight verifies the signed bytes are on origin before anything
  runs. The kit is built to stop here, on purpose.
- **Anchor** — a commit the runner makes in the target after each cycle,
  binding the mission ledger's bytes to git history so progress claims are
  auditable.
- **Fence** — a hard resource boundary a mission may not cross: wall-clock
  hours, cycle count, job count, concurrency, per-job minutes, and spend.
  Enforced by the runner and the engine's `mission fence-*` verbs
  (`internal/mission`).
- **Park / ask / answer** — a mission that cannot safely continue parks
  with a reason and an **ask**; a human answers; `resume` continues it.
  "Running with no live runner but a cleanly concluded record" is the
  legitimate awaiting-resume state, distinct from a crashed runner.
- **Benchmark case** — WHAT a benchmark builds and how it is judged: the
  task specification, seed repository, held-out grader and probes,
  instruments (gate, guards), metrics and noise floors, and what the task
  itself needs of any environment. Immutable per version:
  `benchmark/cases/<caseId>/<caseVersion>/`; identity `taskrun@0.1`. Any
  change to spec, seed, grader, instruments or `case.json` is a new
  version directory (`benchmark/README.md`).
- **Benchmark configuration** — WHO builds a case and under WHAT limits:
  the roster, the fences, host caps, network allowance, machine
  constraint, exposure and **purpose** (`capability` measures;
  `orchestration-health` probes and is never verdict-eligible). Immutable
  per version: `benchmark/configurations/<configId>/<configVersion>.json`;
  identity `cheap@1`. Reusable across cases.
- **Benchmark run / trial** — one case version under one configuration
  version ("run CASE under CONFIG"), repetition n, on one machine, at one
  metasystem sha; pinned by the git object ids of both (`caseTree`,
  `configTree`). "The benchmark taskrun@0.1 under cheap@1" is the pair
  named in one breath; a bare id is never a benchmark.
- **Cohort / repetition / target** — N **repetitions** of one pair, each in
  its own freshly provisioned **target** repository, graded by a held-out
  grader the mission must not read. Driven by `benchmark/run-cohort.sh`.
- **Alias (benchmark)** — a retired spec id (`bm-1` … `bm-2d-og`) that
  resolves, read-only, to a pair (`benchmark/aliases.json`); alias mode
  keeps the legacy naming so pre-migration cohorts stay uniform.
- **Roster** — the pinned assignment of runtimes and models in a
  configuration: which model hosts, which model delegates (per role when
  they differ), and whether the code critic's independence is
  `session-only`. Changing it is a new configuration version and a human
  ruling.

## Delegation plumbing

- **Job (delegate job)** — the unit of delegation: one piece of work
  dispatched to one delegate runtime session. Its record —
  `artifacts/agents/jobs/<jobId>.json` — is the authority on its life:
  `pending-setup → pending → running → completed | failed | timeout |
  cancelled`, transitions made only by compare-and-swap, stamped at
  creation with the epoch and main that own it, and carrying its budget
  (`capMin`). Dispatch creates it, the adapter runs it, the reaper and the
  sweep judge it. A bare "job" in this repository always means this; a
  mission's host TURNS are not jobs.
- **Adapter** — the per-runtime driver (`scripts/agents/adapters/*.sh`)
  that turns one dispatched job into one runtime session — one per
  registered runtime (`bin/metasystem runtime list`; today claude,
  codex, devin, and the fixture-only `fake`). A **host adapter**
  (`scripts/agents/hosts/*.sh`) does the same for mission turns.
- **Capability snapshot** — the probed record of what a runtime CLI can
  actually do and enforce, captured by `<adapter> probe`. Its
  **envelopeEnforcement** declares, per boundary, `mapped` (the runtime
  enforces it) or `notEnforced` (it cannot).
- **Envelope / waiver** — the permission bounds a job requests (write
  roots, read roots, network). Where a runtime cannot enforce a requested
  bound, dispatch refuses — unless the role carries a **waiver**: a
  recorded human acceptance of that named residual for that runtime.
- **Return schema** — the JSON contract a delegate's final answer must
  satisfy (materialized by `metasystem schema`, validated by `validate
  return-complete`; `internal/returnschema`); one bounded same-session
  **repair turn** may fix a malformed return, recorded, never invented.
- **ATIF transcript** — the exported trajectory of a runtime session
  (agent trajectory interchange format); the source of settled session
  identity, effective model, and usage.
- **ACU** — Devin's enterprise metering unit, carried as a provider unit
  and fenced like money; never converted into tokens or cost.

## Verification

- **Gates** — the checks that must pass, chained with the push in one
  command: the metasystem **suite** (`scripts/validate-metasystem.sh`) and
  the benchmark **kit gate** (`benchmark/validate-kit.sh`). A verdict is
  read from the verifying command's own exit code, captured — never
  from a log tail, and never from the exit of a composite or
  background invocation that wraps the command and reports its own
  success instead. A landing gates on that captured code.
- **Fixture** — a self-contained proof of one behavior inside the suite;
  the design loop's findings land as fixtures so they cannot regress
  silently.
- **Design loop** — design → critique by an independent model to zero
  material findings (or a recorded close rule) → implement → code-critique
  → gates. `docs/collaboration.md` owns the details.
- **Guardrails** — the files that ARE an app's net: the specs, golden
  data, gate scripts, and budgets a mission contract declares under
  `wall.guardrails` (exact files, or directory prefixes with a
  trailing slash). A change touching them never rides an ordinary
  implementation authorization: authorization issuance refuses the
  merge without the warden's review, and the wall re-refuses any
  unmarked record at consumption — the net is never changed by the
  work it judges.
- **Warden** — the reviewer of the net itself, with no pen: it judges
  whether a guardrail change strengthens the net or quietly weakens
  it, and its review is what admits the change into an authorization
  (the lane fact rides inside the record's own digest). The warden
  edits nothing — product or guardrail; what it refuses stays refused
  and what it finds becomes work.

## The backlog: what the fleet works on

The working rules live in `docs/backlog-mechanism.md`; the machinery is
the engine's `metasystem goal` family (`internal/goal`).

- **Goal** — one item of intent that survives every turn, a file under
  `plans/goals/`. A goal is opened, claimed, worked, and concluded
  through the goal verbs, never by hand-editing state (hand edits go
  through `goal reconcile`).
- **Backlog** — all live goals plus the root record
  (`plans/goals/backlog.md`, the ledger's minted identity). On a
  converted checkout every goal verb publishes directly to the shared
  remote, so all machines read the same truth; a checkout catches up
  with an ordinary pull.
- **Claim** — one machine's exclusive hold on one goal (one claim per
  machine at a time). An **arc** is a set of goals that claim and move
  as a whole.
- **Appetite** — the worth-sizing agreed between Wido and the
  coordinator before a goal is ready: a duration token (`4h`, `1d`)
  opening the goal's next step. It scopes the design, is re-checked as
  effort accumulates, and a blown appetite stops the work and raises
  the human. Breaches print as banners on every read and reach the
  operator through the steward.
- **Slicing law** — large work is never embarked on in one piece; it is
  split into iterative, independently deployable slices. Composes with
  appetite: appetite sizes what a feature is worth, slicing governs how
  anything big gets delivered.
- **Draft** — a backlog item still being shaped, living in
  `plans/goals-drafts/`, invisible to the fleet until the coordinator
  promotes it through the intake checklist.
- **Steward** — the always-running idle watchdog (`metasystem steward`):
  open delegated work is never silently idle, and what the steward
  tells an agent is covenant, not advice.
- **Review brief** — the contract a critique chain starts from
  (`scripts/agents/templates/review-brief.md`): round budget, threat
  model, appetite, and scope, fixed before round one.
- **Flake registry** — the shared table of known-flaky suite legs and
  the rerun protocol (`docs/flake-registry.md`): a listed leg earns one
  solo rerun, an unlisted failure is diagnosed first, three sightings
  in thirty days force a fix goal.
- **Journey** — the plain-English story of the program
  (`docs/journey.md`). Concluding a goal appends its paragraph in the
  same landing.
