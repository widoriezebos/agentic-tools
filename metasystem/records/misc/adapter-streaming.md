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

This design makes that contract THE interface, and adds exactly two
members, both settled by critique round 1 (FR-012, FR-014):

- Everything an adapter produces today, unchanged in name, shape, and timing
  — with the ONE declared exception in D-2's claude leg (FR-018).
- NEW, additive: throttled activity events emitted into the flight
  recorder's stream (plans/flight-recorder.md D-1), at most one per round
  per 5 seconds. The earlier liveness sidecar is REMOVED from this design —
  the stream does its job with one mechanism.
- NEW, additive: the round records WHICH TRANSPORT actually carried the turn
  (`"transport": "print" | "stream-json" | "jsonl" | "acp"`), so a mixed
  ACP-and-fallback history is provenance in the record, not an inference
  from the witness stream.

Nothing else crosses the boundary. Dispatch, the lease, the census, the
verdict machinery, and the return schema are untouched. An adapter that never
emits activity events (or a round from before this change) behaves exactly
as today — absence IS the legacy behaviour, so the rollout can be one
runtime at a time.

## D-2. Per-runtime transports, hidden behind the interface

How each adapter obtains its event stream is its own business:

- codex — `codex exec --json` ALREADY emits JSONL events as they happen; the
  round artifact `events.jsonl` exists today. The only change is WHEN the
  adapter reads: tail during the turn instead of parsing after exit. No CLI
  flag changes. This is the cheapest leg and goes first.
- claude — switch the internal invocation from `--output-format json` to
  `--output-format stream-json`; assemble the SAME final result object the
  batch mode prints today (stream-json's terminal event carries it). ONE
  DECLARED ARTIFACT DELTA (FR-018): `round_dir/events.jsonl` becomes a true
  event-by-event stream instead of one post-exit batch object. The
  contracts the rest of the system reads — return.json, usage, transcript,
  session identity — are byte-compatible, and the selftest asserts THOSE,
  which is what "outside cannot tell" means precisely.
- devin — run `devin acp` PER TURN (the human's synthesis, 2026-08-09: ACP as
  a transport inside the turn, not as a persistent server). The adapter
  speaks JSON-RPC over the child's stdio: initialize, open or load the
  session, send one prompt, consume `session/update` notifications as the
  event stream, and wind the process down after the prompt's response. The
  adapter assembles `transcript.atif.json` ITSELF from the update stream —
  same filename, same downstream readers (`devin_record_effective_model`,
  `devin_settle_session_identity`, usage extraction). Custody is unchanged:
  the child is one process, one group, owned for exactly one turn.
- mission HOST adapters (scripts/agents/hosts/claude.sh, codex.sh,
  devin.sh) — the same transports apply to the benchmark's most expensive
  turns; the first draft covered only delegates (FR-011). The host leg lands
  with the same conformance proof: the mission fixtures unchanged.
- fake (fixtures) — emits scripted activity events, which is what makes the
  stall-visibility fixtures in Proof possible at all.

## D-3. Liveness through the flight recorder's stream

The earlier draft specified a per-round `liveness.json` sidecar here. It is
GONE (critique FR-012): the flight recorder's event stream carries the same
information with one mechanism — each adapter emits an `activity` event at
most once per 5 seconds per round while its turn runs, and "last event for
round X" is a filtered tail. What this section retains is the RULES, which
outlive the mechanism:

- Observation is not a kill signal. The reaper does not act on event age —
  the KI-29 lesson: two authorities judging one job by different clocks is
  how verdict races are made, and a healthy inference IS silent.
- Absence means legacy. A runtime not yet emitting activity events behaves
  exactly as today, and no reader may treat the absence as a fault.
- The status surface (D-5) reads the stream plus the state files; usage
  shown is COMPLETED ROUNDS ONLY (FR-013) — adapters meter usage at turn
  end, and this design does not invent live metering.

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
- `session/load` SUPPORT, BOTH DIRECTIONS (FR-015). Follow-up turns need
  the prior session in a fresh per-turn process. Probes required: ACP loads
  an ACP-created session; ACP loads a `-p`-created session (migration); and
  the REVERSE — `devin -p -r` resumes an ACP-created session — because the
  declared fallback (follow-ups on `-p` when ACP loading fails) and the
  unchanged same-session repair turn both depend on that bridge. A
  direction that fails closes the corresponding fallback, and the design
  must then say so rather than assume it.
- PERMISSION PARITY, the fourth probe (FR-016). Client-side permission
  answering is a PRIZE, not an assumption. Before any enforcement claim:
  prove every gated operation class actually produces a client permission
  request under `devin acp`; prove a denial is honored (the operation does
  not happen); and define what replaces the generated config file and
  permission mode while ACP carries the turn. Until all three hold, the
  capability snapshot's `notEnforced` declarations and the role waivers
  STAND UNCHANGED — certifying enforcement the transport never supplied
  would be worse than the status quo.
- SNAPSHOT HONESTY (FR-014). ACP changes what the runtime observably offers
  (protocol session identity, native events, possibly enforcement). The
  adapter probe therefore captures a DISTINCT snapshot for the ACP
  transport rather than reusing `-p`'s, and the round's recorded
  `transport` field (D-1) says which snapshot governed each turn.
- OUTCOME MAPPING AND CRASH-SAFE ASSEMBLY (FR-017). The design maps every
  ACP failure shape onto the EXISTING verdict phases: JSON-RPC error
  response → runtime_error; EOF before the prompt response → the same
  failure the current empty-reply path records; malformed notification →
  protocol violation; close timeout → wind-down with the mission-runner's
  never-die rule. Transcript assembly is crash-surviving by construction:
  updates append to a journal file as they arrive (the journal IS the
  partial evidence), and the ATIF document is assembled from the journal at
  turn end — never rewritten in place mid-turn, so a kill leaves a valid
  journal instead of a torn JSON object.

## D-5. The status surface this makes possible

Raised by the human on 2026-08-09: can the whole execution be watched live?
Yes, and cheaply, because every input already lives in one artifact tree. A
read-only status command aggregates, per checkout: mission state and fences
(turn N of M, wall-clock remaining), the lease (epoch, lineage, takeovers),
every non-terminal job (runtime, model, elapsed versus cap), each live
round's latest activity event (age, and the summary line if the open
question above lands), and usage for COMPLETED rounds (FR-013 — live
in-turn metering does not exist and is not invented here). One page,
human-readable, refreshed on demand or under `watch`.

Constraints that keep it honest:

- READ-ONLY, strictly. It presents; it never acts. The same
  observation-is-not-a-kill-signal rule as D-3, for the same KI-29 reason.
- PULL, not push. The human (or the orchestrator between wakeups) asks; the
  ad-hoc watch scripts of 2026-08-08/09 were this tool's hand-rolled
  ancestors and are retired by it.
- It consumes ONLY the interface: job records, mission state, lease,
  the flight recorder's stream. If it needs anything more, that is a sign
  the interface is missing a member, not a license to reach into adapter
  internals.

## Carried critique (round 2 of the flight-recorder chain)

These five findings belong to THIS leg and BLOCK its implementation; the
core-scoped chain did not resolve them, by the recorded rescope. This leg's
own chain must:

- FR2-010: define the activity contract as a FLOOR, not just a ceiling —
  what an adapter must emit during provider silence (a periodic heartbeat
  event vs notification-driven only), since that difference decides
  visibility during the five-minute inferences that motivated everything.
- FR2-011: the DELEGATE compatibility obligation stands on its own — the
  design must define byte-compatibility versus shape-compatibility per
  artifact, and the proof requires a BATCH BASELINE the current selftest
  does not have (it exercises only the installed transport): capture the
  batch-path artifacts first, cut over, compare per the defined contract.
  Filename equality is not artifact compatibility.
- FR2-012: real conformance proofs per host — the suite's host fixture
  drives only codex today; claude and devin hosts need live-path fixtures
  before their transports can claim the mission fixtures as proof.
- FR2-013: snapshot SELECTION must key on transport, not just record it —
  two Devin snapshots (print, acp) under one config hash otherwise compete
  by capture time, and a post-launch fallback can leave the job governed by
  the wrong one. Attempted-then-fell-back provenance needs representing.
- FR2-014: the ACP-to-ATIF mapping (fields, ordering, tool records, session
  identity, effective model, final_metrics) must be specified per protocol
  shape, not left as "assemble from the journal".

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

- Sequenced by the stream head (plans/flight-recorder.md): this leg lands
  AFTER the recorder core and after benchmark attempt 3 runs under that
  core. Never mid-cohort — changing the measuring instrument between
  repetitions makes them incomparable, and the claude adapter hosts bm-2's
  missions.
- Fix-bar classification: DESIGN LOOP. The activity events and transport
  field are (small) contract additions and the ACP leg moves a turn-end
  boundary; both are exactly the "contract or invariant" category of the
  two-bars rule. This document is part of the flight-recorder design loop.
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
- Activity events: monotone timestamps and per-round throttling (max one
  per 5 seconds) under a scripted fake-adapter turn; ABSENT for a runtime
  still on the batch path, with all consumers content.
- Stall visibility: a fake turn that goes silent mid-stream leaves a last
  activity event whose age grows while the job stays `running` and is NOT
  reaped for it — proving observability without a kill signal.
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
