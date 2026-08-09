# One adapter interface, with streaming hidden inside it

- Goal and current status: make every delegate turn observable while it runs —
  without changing what an adapter looks like from the outside. DESIGN DRAFT,
  consolidated from the 2026-08-09 discussion with the human; not yet
  critiqued. FOLDED 2026-08-09 into the flight-recorder stream
  (plans/flight-recorder.md) as its turn-interior leg — that file owns the
  sequencing now; this one keeps the design detail.
- Next step: none
- In flight right now: nothing
- Waiting on the human: nothing — parked by the sequencing rule below.

## Why this exists, with the measurements that forced it

Delegates are silent until their process exits. Four facts from the bm-2 runs
of 2026-08-08/09 show what that blindness costs:

- The one completed Devin delegate spent 319 of its 349 seconds — 91% — inside
  a SINGLE inference call. The job that hit the budget cap spent 98% of its
  wall clock in three such calls. From outside, five minutes of healthy token
  generation is indistinguishable from a hang.
- A delegate that ultimately returned an EMPTY reply burned 7 minutes before
  anyone could know nothing was coming. No partial evidence survived.
- When the mission runner died mid-cohort, the driver polled a dead mission
  for 4.5 hours. The watch sampled lease health and job states — all green —
  because no signal exists for "nothing has happened inside this turn lately".
- None of this is protocol overhead: context assembly measured ~1s, agent
  rounds 2–4s each, our invocation adds ~20–30s per job. The time is model
  inference. Streaming cannot make it faster; it makes it VISIBLE.

## The two axes, and the decision

"Streaming" conflates two independent choices:

1. HOW OUTPUT IS CONSUMED — batch-parse after exit (today) versus reading
   events as they arrive.
2. THE PROCESS LIFECYCLE — one process per turn (today) versus a persistent
   agent server.

Everything we need is on axis 1. Axis 2 is expensive and stays untouched: the
lease, census, reaper, and every custody proof are built on kernel facts —
a turn ends when its process exits, and (pid, pidStartedAt) is the identity.
A persistent server would decouple protocol-liveness from kernel-liveness and
force supervision to learn a second, weaker liveness signal. Rejected here;
revisit only if interactive permissions or session reuse become requirements
in their own right.

The original batch design is not being repudiated. It was correct when built:
verdicts, usage metering, and schema validation all need the COMPLETE
transcript; kernel facts beat protocol promises; one pattern covered three
runtimes. What changed is the workload — claude and codex turns are chatty
and their silence windows are seconds, so silence-blindness cost nothing
until a runtime with five-minute silent inferences arrived.

## D-1. The adapter interface, made explicit

Today's contract is implicit but real. Every adapter exposes the same verbs
(probe, dispatch, follow-up, cancel, selftest, identity, config-identity,
signature, local-config-paths) and every round produces the same artifacts
under its round directory: `prompt.md`, `raw.out`, the runtime transcript,
`return.json` (schema v2, normalized by `complete_from_cli`), the usage file,
and the session identity settled onto the job record. The common layer
(runtime-common.sh) already drives per-runtime behaviour through hook
functions (`runtime_repair_turn`, `runtime_usage_after_repair`,
`runtime_settle_after_repair`, `runtime_error`).

This design makes that contract THE interface, and adds exactly one member:

- Everything an adapter produces today, unchanged in name, shape, and timing.
- NEW, additive: a per-round sidecar heartbeat, `round_dir/liveness.json`,
  maintained while the turn runs (D-3).

Nothing else crosses the boundary. Dispatch, the lease, the census, the
verdict machinery, and the return schema are untouched. An adapter that never
writes the sidecar (or a round from before this change) behaves exactly as
today — absence of the file IS the legacy behaviour, so the rollout can be
one runtime at a time.

## D-2. Per-runtime transports, hidden behind the interface

How each adapter obtains its event stream is its own business:

- codex — `codex exec --json` ALREADY emits JSONL events as they happen; the
  round artifact `events.jsonl` exists today. The only change is WHEN the
  adapter reads: tail during the turn instead of parsing after exit. No CLI
  flag changes. This is the cheapest leg and goes first.
- claude — switch the internal invocation from `--output-format json` to
  `--output-format stream-json`; append events to the transcript as they
  arrive; assemble the SAME final result object the batch mode prints today
  (stream-json's terminal event carries it). Outside: byte-identical
  artifacts. Note the sequencing consequence: bm-2's HOST runs on this
  adapter, so even this invisible change waits for the cohort.
- devin — run `devin acp` PER TURN (the human's synthesis, 2026-08-09: ACP as
  a transport inside the turn, not as a persistent server). The adapter
  speaks JSON-RPC over the child's stdio: initialize, open or load the
  session, send one prompt, consume `session/update` notifications as the
  event stream, and wind the process down after the prompt's response. The
  adapter assembles `transcript.atif.json` ITSELF from the update stream —
  same filename, same downstream readers (`devin_record_effective_model`,
  `devin_settle_session_identity`, usage extraction). Custody is unchanged:
  the child is one process, one group, owned for exactly one turn.
- fake (fixtures) — emits scripted heartbeats, which is what makes the
  stall-visibility fixtures in Proof possible at all.

## D-3. The liveness sidecar, specified

`round_dir/liveness.json`, written atomically (tmp + rename), holds:

    {"lastEventAt": "<ISO-8601>", "events": <count>, "schemaVersion": 1}

  OPEN QUESTION for the critique: one optional `lastEventSummary` field — a
  single truncated line ("running mvn test", "inference"), no payloads. It is
  what turns the status surface (D-5) from "alive 11s ago" into "doing X, 11s
  ago". Recommendation: yes, bounded at 120 characters, best-effort.

- WRITER: the adapter's own supervision of the turn (the same code path that
  today waits for the CLI), updating on every event, throttled to at most one
  write per second. No new process.
- READERS, strictly OPT-IN: a watch script's "no event for N minutes" line;
  a human tailing a run. The REAPER DOES NOT ACT ON IT in this design. That
  is deliberate, twice over: first, observability must not become a kill
  signal — the reaper acting on event-age would recreate exactly the verdict
  race fixed on 2026-08-09 (budget-cap versus process-lost, KI-29), where two
  authorities judged one job by different clocks. Second, a slow inference IS
  silent; killing on silence would execute the innocent. If diagnostics ever
  want event-age, that is a separate design with its own critique.
- ABSENCE means legacy: no reader may treat a missing sidecar as a fault.
- POSSIBLY SUBSUMED: the human's 2026-08-09 observability requirement
  (plans/flight-recorder.md) proposes one append-only event stream for the
  whole metasystem; "last event for round X" is then a query over that
  stream and tailing one file replaces this sidecar. The two critiques
  should be read together; flight-recorder.md's draft position is stream-only.

## D-4. What ACP-per-turn additionally buys, and what it costs

Buys, beyond the event stream:

- Permission answering moves to OUR side of the wire. In ACP the agent asks
  the client to allow or deny each gated call; the adapter answers
  mechanically from the same policy it bakes into the config file today.
  Devin is the one runtime whose envelope is `notEnforced` across the board
  and runs under recorded per-role waivers — this is the first mechanism
  that would let the metasystem ENFORCE rather than configure-and-hope. The
  waiver story in plans/devin-support.md does not change until this is
  proven, but the design should be critiqued with that prize in view.
- Partial evidence by construction: the assembled transcript exists up to the
  last received update even when the turn dies or returns nothing — the
  empty-reply failure mode stops being evidence-free.

Costs, stated honestly (the probe questions for the critique):

- USAGE SOURCING. Today tokens and ACU come from ATIF `final_metrics` at
  exit. ACP has no standard usage reporting. The adapter must either extract
  usage from what `devin acp` emits (probe), or read the session store after
  the turn as `devin_usage` does now (fallback that keeps the contract), or
  declare usage unavailable for ACP turns (unacceptable for bm-2's fences —
  ACU metering is a mission-fence input). This MUST be settled before the
  devin leg lands; the codex and claude legs do not depend on it.
- TURN-END BOUNDARY. With `-p`, exit code + exported transcript end the turn.
  With ACP, the turn ends at the JSON-RPC response to the prompt request,
  and the process exits because WE close it. The adapter owns mapping
  response→artifacts→wind-down; the contract shift is adapter-internal but
  the wind-down path must be proven against the 2026-08-09 lesson: never die
  over a group whose ownership proof vanished (mission-runner
  terminate_group), and record verdicts before wind-down.
- `session/load` SUPPORT. Follow-up turns need the prior session in a fresh
  per-turn process. `devin -p -r <session>` does this today; the ACP leg
  needs a probe that `devin acp` can load a session created by a previous
  ACP turn (and one created by a `-p` turn, for migration). If it cannot,
  follow-ups keep the `-p` transport while dispatch turns use ACP — the
  interface permits mixed transports because nothing outside can tell.

## D-5. The status surface this makes possible

Raised by the human on 2026-08-09: can the whole execution be watched live?
Yes, and cheaply, because every input already lives in one artifact tree. A
read-only status command aggregates, per checkout: mission state and fences
(turn N of M, wall-clock remaining), the lease (epoch, lineage, takeovers),
every non-terminal job (runtime, model, elapsed versus cap), each live
round's sidecar (last event age, and the summary line if the open question
above lands), and usage so far. One page, human-readable, refreshed on
demand or under `watch`.

Constraints that keep it honest:

- READ-ONLY, strictly. It presents; it never acts. The same
  observation-is-not-a-kill-signal rule as D-3, for the same KI-29 reason.
- PULL, not push. The human (or the orchestrator between wakeups) asks; the
  ad-hoc watch scripts of 2026-08-08/09 were this tool's hand-rolled
  ancestors and are retired by it.
- It consumes ONLY the interface: job records, mission state, lease,
  sidecars. If it needs anything more, that is a sign the interface is
  missing a member, not a license to reach into adapter internals.

## D-6. What explicitly does not change

- One process per turn; dispatcher owns the group; (pid, pidStartedAt) is
  identity; turn end is process exit (for ACP: response then exit, still
  within the one owned process).
- The lease, census, reaper, handshake machinery, and verdict priorities.
- Return schema v2, the bounded same-session repair turn, usage contract
  fields, capability snapshots (each runtime's `transports` list already
  declares what it COULD speak; the adapter starts using a member it already
  declared — devin's snapshot lists "acp" today).
- The roster, the benchmark kit, and every artifact filename.

## Sequencing and classification

- AFTER the bm-2 cohort completes, without exception — changing the
  measuring instrument mid-cohort makes repetitions incomparable, and the
  claude adapter hosts bm-2's missions.
- Fix-bar classification: DESIGN LOOP. The sidecar is a (small) contract
  addition and the ACP leg moves a turn-end boundary; both are exactly the
  "contract or invariant" category of the two-bars rule. This document is
  the draft for that loop; sol critiques it before any implementation.
- Implementation order inside the stream: codex (tail what already streams),
  then claude (flag swap + assembly), then devin ACP (carries all the probe
  questions). Each leg lands with its fixtures and the full gates before the
  next starts; a leg that stalls does not block the ones already landed.

## Proof

- Interface conformance, per adapter: with streaming enabled, a turn produces
  artifacts byte-compatible with the batch path (same return.json fields,
  same transcript filename and shape, same usage file), proven by running the
  existing selftest unchanged — the selftest not needing to know is itself
  the proof that the outside cannot tell.
- Liveness sidecar: monotone lastEventAt and event count under a scripted
  fake-adapter turn; atomic (a reader never sees a partial file); throttled;
  ABSENT for a runtime still on the batch path, with all consumers content.
- Stall visibility: a fake turn that goes silent mid-stream yields a sidecar
  whose age grows while the job stays `running` and is NOT reaped for it —
  proving observability without a kill signal.
- ACP permission answering: a gated call inside a `devin acp` turn receives
  the adapter's policy answer, and the answer plus the call land in the
  assembled transcript.
- Usage parity (devin): the same brief run via `-p` and via ACP records the
  same usage fields (or the ACP turn's declared fallback path is exercised
  and its provenance recorded) — the fence must see no difference in kind.
- Empty-reply evidence: an ACP turn killed mid-stream leaves a transcript
  with every update received so far; the return normalizer treats it as the
  failure it is, with evidence attached.
- End to end: a bm-2-shaped mission on the streamed adapters completes with
  identical verdict machinery behaviour, and the watch can quote "last event
  age" for a live delegate.
