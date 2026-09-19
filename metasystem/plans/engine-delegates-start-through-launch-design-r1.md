# Engine delegates start through launch

Status: DRAFT r1, not accepted and not scheduled. Written 2026-09-19 by codex gpt-6-astra as the design for item 5 of goal machinery-runs-unattended-on-codex. m1e narrowed item 5 the same day: the engine already builds on codex gpt-5.6-sol and designs on gpt-6-astra through scripts/agents/adapters/codex.sh, so moving engine delegates onto `metasystem launch start` is not needed to switch the machinery on. This page waits for Wido to pick the follow-up after the target. It was not critiqued. Its 25 units add up to about 9,500 lines, which is far above the size a follow-up should start with. A pick should begin by cutting it down.


Revision: first draft, 2026-09-19. Scope: DONE item 5 of
`machinery-runs-unattended-on-codex`, plus the evidence needed for item 6.

## 1. Decision and evidence

Keep dispatch as the owner of delegation policy and job chains. Make Go launch
the sole owner of delegate processes. Every engine delegate attempt enters
through `metasystem launch start`; dispatch never executes a runtime adapter.
Switching the engine and deleting the superseded launch path are one landing,
without a feature flag. This page proposes implementation; it does not switch
the machinery on or authorize the trial.

**Read:** the nine cited code ranges and the example's units section. Delegate
currently executes `dispatch.sh`; its `__launch` selects an adapter and a start
gate. Mission completion and drain still use dispatch. Launch already resolves
settings, accepts an explicit identity, and selects adapters by kind/model.
The cited selector also includes `proof`, beyond the four kinds in the brief.

**Inferred:** dispatch policy remains necessary independently of runtime
creation. **Proposed:** the linkage, lifecycle extensions, accounting projection,
and trial commands below. Their existence is not claimed as current behavior.
No implementation, tests, role inventory outside the pack, or live trial was
inspected or run. Names below describe the target interfaces, not verified
current command syntax. The restricted pack does not establish complete legacy
file sizes; unit estimates include explicit size checks before editing.

## 2. Rules and witnesses

### R1 — Keep the delegation contract; remove runtime ownership

Keep job records, admission, budget reservations, claim capabilities, chain
ownership compare-and-swap, critique bookkeeping, and dispatch/watch/status/
follow-up/cancel/close/reap verbs. The shell may remain their thin orchestration
entry point. The existing Go job-record commands remain authoritative writers.
Do not move these responsibilities into launch or rewrite unrelated dispatch
policy merely to remove the shell.

Remove shell runtime selection, adapter execution, start gates, handshake
polling/timeouts, adapter-written heartbeat/protocol updates, and direct process
signalling. Replace internal launch with an authenticated bridge to
`metasystem launch start`. Missionrunner may continue calling dispatch reap and
close; those verbs obtain process facts and request termination through launch.
Launch's own adapters remain: they implement the surviving process owner.

**Test:** dispatch an admitted job through a recording command executor. Assert
one `metasystem launch start`, correct claim context, and no runtime-script
execution. Exercise refusal before admission, reservation failure, and claim
loss; none may spawn. Run existing admission and chain-policy tests unchanged
where their observable contracts remain valid.

### R2 — One durable identity per executable attempt

Persist a launch identity in the job record before requesting start. Use a
collision-resistant identity derived from the canonical repository identity,
job identity, and attempt generation; job names alone are not globally unique.
A launch record stores the reverse link: repository, job, generation, chain,
mission, goal, and ownership epoch. Reserve the identity atomically and bind it
to a digest of the immutable request, including prompt and working directory.

Retrying start with that identity and digest returns the existing launch;
different content refuses without spawning. A terminal identity never spawns
again. A genuinely new attempt receives a new generation. A follow-up is a new
linked attempt, never an instruction to a shell-owned session. Carry forward
the chain's explicit context; exact-session continuation is used only when the
selected launch adapter supports it. No silent loss of required context.

Use the launch store's lock and durable states for preparation and supervisor
acceptance. Its supervisor must make acceptance idempotent by launch identity.
A crash after process creation but before record publication must be recoverable
from supervisor ownership, not answered by starting another process. Recovery
joins prepared jobs to launch records, reconciles incomplete acceptance, and
repairs the job projection using its ownership compare-and-swap.

**Test:** concurrent starts; lost CLI response; crashes before and after durable
preparation, supervisor acceptance, and job projection; conflicting replay;
identical job names in two repositories; and follow-up generation. Assert at
most one process per identity and an eventual report entry for every process.

### R3 — Fence starts and terminate through the process owner

Treat dispatch authorization and launch acceptance as one ordered decision.
Acceptance validates the current epoch, pending job, reservation, and an open
mission start fence under the same per-mission coordination boundary used by
stop/cancel. Never hold that boundary while waiting for a child to finish.
If stop wins, start refuses. If start wins, stop sees the registered launch and
cancels it. A late worker or stale epoch cannot recreate a canceled job.

Launch owns a supervisor-managed process tree and a stable process identity;
dispatch never kills a numeric PID from an old job record. Proposed
`metasystem launch cancel --id ID` is idempotent, requests graceful termination,
and escalates after a configured deadline. The supervisor handles descendants
and reaps children without tmux or cron. A detached descendant still belongs to
the launch's containment mechanism. If containment cannot be established,
start fails before executing the runtime.

Launch publishes process liveness and heartbeat timestamps. Dispatch projects
those facts into job status; the mission runner retains its own heartbeat.
Heartbeat silence means suspect, not proof of death. Reap reconciles through
the supervisor, including after runner restart. Close releases reservation and
chain ownership only after all attempts are terminal and their process trees
are empty. A lost supervisor produces an explicit unresolved state; neither
the reservation nor the stop fence is cleared by guessing.

**Test:** interleave stop and start at every acceptance boundary; cancel before
and after acceptance; reuse a numeric PID with a different process identity;
kill the runner; lose a heartbeat; spawn a descendant; and lose supervisor
contact. Use an injected clock and controllable supervisor. Assert no start
beyond the fence, no unrelated kill, and no successful close with descendants.

### R4 — Resolve roles and configuration once

The engine supplies a role; one Go mapping translates it to a launch kind.
Shell adapters, roster runtime choices, environment variables, and prompt text
must not override engine model or effort. Launch snapshots the resolved values
in its record; an admitted attempt does not change when configuration changes.
Explicit model overrides may remain available for independent manual launches.

| Engine role or operation | Launch kind | Engine configuration |
|---|---|---|
| implementer | build | `launch.build.model`, `launch.build.effort` |
| designer | design | `launch.design.model`, proposed `launch.design.effort` |
| critic | critique | build model and effort, explicitly shared |
| reader | read | `launch.read.model`, proposed `launch.read.effort` |
| executable proof, if launched here | proof | plain executor; no model or effort |

Set engine build to `gpt-5.6-sol` and design to `gpt-6-astra`. Reader defaults to
`gpt-5.6-sol`; extend read selection to use codex-exec for a Codex model instead
of unconditionally choosing claude-headless. Efforts are explicit validated
configuration values, not newly invented constants. This design does not
choose a more expensive effort tier. Missing/invalid effort blocks admission
with the offending key before reserving or spawning.

Enumerate actual dispatchable roles during the mapping unit. Add aliases only
when their existing responsibilities match a row; do not infer them from names.
An unmapped executable role refuses because selecting an arbitrary execution
contract could violate authority or budget. Coordinator/warden control logic
is not itself a delegate role. Proof routing is not a requirement to launch
additional proof runs.

**Test:** a table covering every dispatchable role and alias, checked against
the producer vocabulary; config changes affect only subsequent attempts;
engine overrides cannot evade configured policy; design/read select Codex;
unknown roles and invalid settings create neither reservation nor process.

### R5 — Charge launch measurements exactly once

Launch publishes cumulative measurement snapshots keyed by identity and
monotonic measurement revision: execution start/end, process-active duration,
input/output/cached token categories, and whether usage is final or incomplete.
Persist available usage on cancellation and failure as well as success.

Dispatch's accounting owner projects those snapshots into the existing mission
and goal budget ledgers. Store the last applied cumulative values and revision
atomically with the charge, so duplicate or out-of-order delivery cannot double
charge or subtract spend. A restart replays safely. Concurrent attempts sum job
minutes; they do not replace them with elapsed mission time. Keep precision
until the existing budget owner's rounding boundary.

Apply the existing budget weighting policy once, recording its version with
the charge. Cached-token categories must have an explicit weight. Do not add a
second token formula to dispatch. Charge all attempts, retries, critics, reads,
and canceled work to their originating mission/goal. Direct and delegated
views of the same launch must not create duplicate charges.

Reservation remains with admission and is reconciled against actual spend;
launch admission may enforce its own limits but does not debit the same goal
again. Missing usage is unknown, never zero: retain a conservative outstanding
reservation and mark the goal's spend incomplete until recovered or explicitly
reconciled. A trial cannot pass with unknown spend. Budget exhaustion fences new
starts and applies the existing running-job budget policy.

**Test:** replay snapshots, reordered revisions, partial usage followed by final
usage, concurrent jobs, restart between measurement and projection, cancellation,
and missing usage. Assert mission totals equal linked launch totals and goal
totals equal unique attributable charges, including after repeated reaping.

### R6 — Delete the replaced path in the cutover landing

Deletion is determined by ownership, not file extensions. Delete these targets:

- `scripts/agents/adapters/{claude,codex,devin,fake}.sh` and helpers exclusively
  used by them; retain Go launch adapters and deterministic supervisor fakes.
- `launch_adapter`, the old runtime/verb form of `internal_launch`, its callers'
  runtime selection, adapter capability transport, start-gate creation, and
  shell background-process launch wrappers.
- `__handshake`, `__handshake-timeout`, their implementations, handshake-only
  ownership transitions, polling settings, timeout records, and gate cleanup.
- Adapter-only `__protocol-error` authority and reporting paths; retain generic
  job error reporting if a surviving caller requires it.
- Adapter-written heartbeat paths and dispatch kill logic superseded by R3.
- Fixtures/tests whose only assertion concerns old adapters, gates, handshakes,
  fake runtime scripts, or old runtime-selection environment variables; remove
  their registration and documentation too.

Preserve policy tests by moving their process seam to the launch supervisor.
Preserve any helper with a surviving consumer. An implementation inventory must
record each deletion target, remaining callers, and changed-line count before
cutover work; the final repository reference check must show no executable
caller of the removed path. Historical prose may describe it as history.

Drain before deployment: stop admitting jobs through the old engine and prove
all old processes and chains closed using its current owner. Then install the
whole landing and restart. Existing terminal records remain readable as history.
Do not add an active legacy-record fallback. If an old process remains, deployment
waits for its existing owner to terminate it. Rollback likewise requires draining
the new owner before reverting the whole landing.

**Test:** migrated policy fixtures pass; source-reference and fixture-registration
checks reject executable legacy dependencies; drained historical records remain
readable; startup refuses active legacy ownership. No compatibility switch.

### R7 — Make the one-goal trial auditable

Extend `launch report` with machine-readable goal filtering, reverse job links,
effective settings, lifecycle facts, cumulative usage, and containment results.
Add proposed command `metasystem goal trial-report --goal GOAL --json`, a
read-only join of launch, mission, proof, intervention, and accepted-diff records.
It must refuse a passing verdict when attribution or history is incomplete.

| Item 6 requirement | Command and passing evidence |
|---|---|
| Every started delegate is visible | `metasystem launch report --goal GOAL --json`: one record for every job attempt, no unmatched process ownership, resolved model/effort included |
| One proof | `metasystem goal trial-report --goal GOAL --json`: exactly one goal-scoped proof execution, successful and covering the accepted final revision; failed attempts count too |
| No human rescue | Same trial report: complete intervention history from activation to completion, zero manual starts/retries/cancels/repairs or externally supplied progress; initial authorization is excluded |
| No leaked process | `metasystem launch report --goal GOAL --verify-processes --json`: all attempts terminal, supervisor reconciled, zero live owned descendants and zero attributable strays |
| Under eight hours | Trial report: completion minus durable activation timestamp, including queue/drain/proof time, strictly below 28,800 seconds |
| Under 2,000 weighted tokens per changed line | Trial report: final attributable weighted spend divided by additions plus deletions in the accepted goal diff, strictly below 2,000; positive denominator and complete usage required |

Seal the trial's baseline, final accepted revision, goal/mission membership,
weighting policy, and intervention audit at activation/completion. Calculate
changed lines once from the final baseline-to-accepted diff; do not sum unit
diffs, count reverted churn, or inflate the denominator with unrelated changes.
A zero-line result is not a passing efficiency trial. Count all trial-attributable
model usage, including orchestration outside delegate launches; missing such
records blocks the efficiency verdict.

The no-rescue claim requires audited entry points and complete event history.
An unaudited external mutation makes it unprovable. A quiet log alone cannot
prove absence of human help. Process verification is fresh at trial closure,
after closing the start fence, not merely a reading of terminal record flags.

**Test:** synthetic joined histories independently violate each condition, omit
a launch, duplicate a charge, omit control-seat usage, change the final revision,
and introduce a stray descendant. Test strict boundaries at eight hours and
2,000 tokens per line. Use fake timestamps and counters, never elapsed waiting.

## 3. Verification and landing

All tests use injected clocks, event barriers, and controllable supervisors;
no sleep calls, polling delays, tmux, cron, or model calls. Exercise the changed
surface end to end through delegate, real CLI parsing and record stores, and a
deterministic launch executor that returns usage and completion events. Include
start-response loss, runner restart, cancel, reap, close, and trial reporting.
Run focused launch/dispatch/missionrunner tests and migrated policy fixtures.
The real one-goal trial remains a separate item 6 activity, not a build test.

Before cutting units, measure additions plus deletions including tests. Estimates
below are ceilings for planning, not measured diffs. If a deletion group exceeds
400 lines, subdivide its review commits and update this table before implementing
that group; never hide deletions or tests from the count. No unit changes the
contracts above without a revised design.

U1–U14 are independently green preparation commits: introduce and test the new
owner and consumer APIs without changing the active engine route. U15–U25 form
one cutover landing assembled in order in an isolated integration worktree.
Each commit must pass its focused checks: after U15 the engine uses launch;
subsequent commits delete unreachable legacy code and its obsolete fixtures.
Each deletion unit also migrates or deletes every fixture that calls its removed
symbols; fixture cleanup cannot be postponed if it would leave that commit red.
None of that group is deployed or landed separately. The final combined tree
must pass the default design-obligation completion check and end-to-end witness.
This distinction permits small green review units while switching and deleting
in the same landing; it introduces no runtime flag.

## 4. Units

Line budgets include implementation, tests, and deletions. Dependencies are unit
identifiers. R1–R7 refer to the rules and mandatory witnesses above.

| Unit | Lines | Rules | Depends on |
|---|---|---|---|
| U1 Job/launch identity fields, immutable request binding, collision tests | 340 | R2 | none |
| U2 Durable idempotent start acceptance and replay tests | 390 | R2 | U1 |
| U3 Supervisor adoption after crashes, recovery witnesses | 380 | R2 | U2 |
| U4 Mission fence ordering and epoch validation | 360 | R3 | U2 |
| U5 Launch cancellation and process-tree containment witnesses | 390 | R3 | U3, U4 |
| U6 Launch liveness projection, reaper and close consumer APIs | 380 | R1, R3 | U5 |
| U7 Role vocabulary, kind mapping, settings and Codex read selection | 370 | R4 | U1 |
| U8 Follow-up attempt linkage and context tests | 300 | R2, R4 | U3, U7 |
| U9 Cumulative launch measurements and incomplete-usage persistence | 360 | R5 | U2 |
| U10 Idempotent mission/goal accounting projection and reservations | 390 | R5 | U9 |
| U11 Goal-filtered launch report and fresh process verification | 360 | R2, R3, R7 | U6, U9 |
| U12 Trial joins, accepted-diff denominator and budget verdicts | 380 | R5, R7 | U10, U11 |
| U13 Proof/intervention attribution and trial command boundary tests | 380 | R7 | U12 |
| U14 CLI bridge, deterministic executor and end-to-end contract harness | 390 | R1–R5 | U6, U8, U10 |
| U15 Switch engine start and lifecycle calls, migrate entry-point tests | 390 | R1–R4, R6 | U13, U14 |
| U16 Delete shell launch/runtime wrappers and their direct fixtures | 390 | R1, R6 | U15 |
| U17 Delete handshake/gate implementation and its direct fixtures | 390 | R3, R6 | U16 |
| U18 Delete timeout/capability/protocol remnants and direct fixtures | 390 | R3, R6 | U17 |
| U19 Delete shell Codex adapter and exclusive helpers | 390 | R6 | U18 |
| U20 Delete shell Claude adapter and exclusive helpers | 390 | R6 | U19 |
| U21 Delete shell Devin/fake adapters and exclusive helpers | 390 | R6 | U20 |
| U22 Delete remaining adapter fixture support and registrations | 390 | R1, R6 | U21 |
| U23 Delete remaining handshake fixture support and registrations | 390 | R3, R6 | U22 |
| U24 Remove old heartbeat/kill paths and remaining legacy references | 390 | R3, R6 | U23 |
| U25 Drain/startup checks, operator commands, final integration witness | 390 | R1–R7 | U24 |
