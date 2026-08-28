# The mission runner's actual cycle sequence

> SUPERSEDED SECTIONS (kept for history; the map is pinned to its
> commit): the drain narrative and false-stall surfaces (j) and the S13
> drain step predate patience satellites 1 and 2 — the drain is now
> finite with mission-scoped reaping and a drain-stalled park
> (records/patience/patience-mission-reap-drain.md), and turn identity, faulted
> measurement, and the capped outcome changed per
> records/patience/patience-turn-identity.md. A satellite designing against this
> map must cross-check those two shipped designs first.


Status: MAP, not design. This document records what the runner does today,
step by step, with file:line anchors into the code as of the commit it was
written against. It exists because `records/stop-loss/stop-loss-satellites.md` names it
as the precondition for all four satellite designs: the parent design
(`records/stop-loss/stop-loss-last-defense.md`) died in round 3 of critique because it
specified mechanisms against an assumed sequence — it assumed the runner
dispatches critique roots (the host does), assumed the ledger tail carries
lines it does not carry, and assumed conclude-time hooks that sit at the
wrong point in the order. Every claim below is anchored; where behavior is
surprising the surprise is stated as a fact, not a proposal.

Paths are relative to the `metasystem/` root. Line numbers refer to:
`internal/missionrunner/{loop,launch,host,adjudicate,turnio,cycle,answer,jobs,stoploss,contract,engine,missionrunner,status}.go`,
`internal/mission/{ledger,fence,state,anchor,prompt,measure}.go`,
`scripts/agents/dispatch.sh`, `scripts/agents/hosts/claude.sh`,
`internal/dispatch/{build,handshake,mission,critique}.go`,
`internal/validate/turnprompt.go`.

## 0. Process topology

Five distinct processes touch a mission's artifacts:

1. **The launcher** — whoever runs `mission-runner start|resume`. Runs in the
   caller's process: stale-lease cleanup, supervision arming, contract
   preflight and pin, then spawns the detached runner and blocks on the
   start-signal handshake (launch.go:313-434).
2. **The runner** — the detached `run-loop` process. Holds the mission lease
   for its whole life, turns cycles, is the single writer of mission state,
   the ledger's cycle lines, turn records, and adjudication artifacts
   (loop.go:25-114).
3. **The host adapter + host CLI** — one per turn, spawned by the runner in
   its own session (`setsid`), e.g. `scripts/agents/hosts/claude.sh` wrapping
   `claude -p`. The host is the orchestrator model. It does the repository
   work and **it, not the runner, dispatches every delegate job** via
   `scripts/agents/dispatch.sh` inside its turn (host.go:167-342,
   roles/orchestrator.md:3).
4. **Delegate jobs** — dispatched by the host through dispatch.sh, running in
   their own sessions, surviving the host's exit. The runner only ever reaps
   and closes them; it never creates them.
5. **The answer CLI** — `mission-runner answer`, run by a human at any time,
   alive runner or not. It writes ask records, the state, and (for stop-loss
   resets) the ledger (answer.go:17-200). It takes no lease and does not
   check whether a runner is live.

Locks in play (all `flock` on a sibling `.lock` file unless noted):

| Lock | Path | Held by | During |
|---|---|---|---|
| Mission lease | `missions/<id>/lease.d/` (atomic mkdir) + `lease.json` | runner | entire runner life (loop.go:178-228) |
| Fence lock | `missions/<id>/mission-fence.lock` | any process | contract pin (launch.go:223-229), cycle reserve, job reserve/authorize, fence refuse, usage aggregate (fence.go:370-429, 434-534, 541-548) |
| Ledger lock | `missions/<id>/ledger.md.lock` | any process | InitLedger, AppendCycle, AppendReset (ledger.go:108-205, 287-301) |
| State lock | `missions/<id>/state.json.lock` | any process | InitState, WriteState CAS, Reconcile (state.go:630, 738; anchor.go:369) |
| Chain lock | `artifacts/agents/locks/<root>.d` | dispatch.sh | dispatch, follow-up, close (dispatch.sh:295-311) |
| Job lifecycle lock | per job | dispatch.sh | reap (dispatch.sh:851-872) |

## 1. Before the loop: launch and resume

Launcher process, in order (launch.go:321-434):

1. Mode checks: `start` refuses an existing `state.json`; `resume` refuses a
   non-running state (launch.go:322-337).
2. `cleanupStaleLease` (launch.go:98-164): refuses if the recorded lease pid
   is alive with the recorded instance tag; otherwise walks
   `turns/*/turn.json`, and for every open turn (`status` pending/running or
   `outcome` running, launch.go:69-75) **whose recorded pgid+tag still match a
   live group** terminates the group and patches the turn to
   `failed/turn-lost` (launch.go:118-142). An open turn with no recorded
   pgid (a crash before host start) is skipped and stays `pending` on disk
   forever — only `state.turnLog` matters to later cycles, so this is
   cosmetic. Then removes the lease marker and file.
3. `armAndPreflight` (launch.go:268-307): arms supervision as the mission's
   own identity when unattended (lineage = `MissionLineage(mission)`, a
   sha256-derived constant, missionrunner.go:115-118), preflights the authored
   contract, and **pins** the verified snapshot to
   `missions/<id>/mission-<id>.contract.md` with its raw sha256 recorded in
   `fences.json.approvedContractSha256`, under the fence lock
   (launch.go:218-264).
4. Spawns `metasystem mission run-loop` detached (`setsid` unless
   foreground), then polls up to 15s (scaled) for the start-signal file the
   runner writes on its first verified host start; on timeout it kills the
   runner group (launch.go:344-433).

Runner process (loop.go:33-114), in order: write the runner record
(`missions/runners/<id>.json`), heartbeat, **acquire the mission lease**
(atomic `mkdir lease.d`, loop.go:178-219), then:

- `start`: `initializeState` (loop.go:308-341) — `InitLedger` (budgets from
  the pinned contract, under ledger lock), `InitState` (streams from
  `stream.*` contract keys, `ledgerSemantics: 2` pinned at state.go:627),
  anchor commit, verify.
- `resume`: `resumeState` (loop.go:348-387) — `mission.Reconcile` under the
  state lock (anchor.go:368-421), which:
  - parks `state-integrity` when the state is corrupt or the ledger is
    *behind* the state;
  - when the ledger is *ahead* of the state (crash between ledger append and
    state write): verifies the current ledger extends the anchored bytes,
    then **adopts** the ledger's cycle count into `state.ledger.cycles` and
    bumps `fences` if needed (anchor.go:391-407) — the interrupted turn's
    turn-log entry (session id, measurement) is *not* recovered;
  - when counts agree but the latest anchor disagrees with the state:
    parks `state-integrity`, with exactly one tolerance — an unanchored
    ledger suffix consisting solely of `Stop-loss reset:` lines naming
    stagnation stop-loss asks on disk (anchor.go:409-450).
  - Then `applyPendingReset` (loop.go:394-430): a stop-loss-parked state
    whose last ledger event is a reset line naming an answered `reset:` ask
    is unparked here — the crash-recovery half of the vocal reset.
  - Anything not `running` after that refuses resume (loop.go:379-381).

The loop itself is three lines: while `status == "running"`, heartbeat, run
`oneCycle` (loop.go:93-100). When the loop exits, `closeTerminalChains`
reaps and closes every fully-terminal delegation chain (loop.go:101,
625-635), the start signal is answered if it never was (loop.go:104-109),
the lease is released, the runner record finalized.

## 2. THE SEQUENCE: one cycle

All of `oneCycle` (loop.go:665-858). Steps in execution order. "On crash"
describes a runner death between this step and the next; the host and
delegates keep running regardless (own sessions).

**S1. Reserve the cycle.** `mission.ReserveCycle` (loop.go:666,
fence.go:404-429), under the fence lock: re-reads the *live* authored
contract from `plans/`, verifies its raw sha256 equals the pinned
`approvedContractSha256` (fence.go:140-154), checks wall-clock and cycle
fences, then increments `fences.json.cycles`. A fence refusal writes/extends
the batched `fence-bound*` ask under the same lock (fence.go:274-357) and the
runner parks the mission with reason `fence` (loop.go:667). The cycle number
is spent **before anything else exists**.

- Writes: `fences.json`, possibly `asks/fence-bound*.json`. Lock: fence.
- On crash anywhere between S1 and S13 (the ledger append): `fences.cycles`
  is now permanently one ahead of the ledger's cycle count. Every later
  cycle passes the fence counter as the ledger cycle number (loop.go:673,
  825) and `AppendCycle` refuses a non-contiguous number (ledger.go:139-141).
  That refusal is a returned error, i.e. the runner's fail exit ramp
  (loop.go:97-99) — not a park. Nothing in `Reconcile` repairs a
  fences-ahead-of-ledger gap (it only repairs ledger-ahead-of-state). A
  runner crash mid-turn therefore wedges the mission: every resume fails at
  its first ledger append until a human intervenes. See §6(g).

**S2. Read the reserved cycle number** back from `fences.json`
(loop.go:669-675).

**S3. Prior context from the state's turn log.** `PriorContext`
(loop.go:681, contract.go:103-137): from the *last* turn-log entry —
`hostSession` (its recorded `sessionId`), `reconciliation` (last outcome not
`completed`/`return-ok`), and `priorFailures` counted backwards to the last
completed turn, **skipping `unresumable` entries** and dropping the session
when the last outcome was `unresumable`. Only concluded turns exist in the
turn log; a turn that crashed mid-flight is invisible here.

**S4. Parse the pinned contract** (loop.go:682, contract.go:67-96): the
approved snapshot, sha-checked against the fence pin. Yields
`host.runtime`, `host.model`, `host.turn-cap-min`.

**S5. Allocate the turn.** `allocateTurn` (loop.go:690, 434-447): turn id
`<mission>-t<cycle>-<4hex>`; `mkdir turns/<turnId>` is the collision check.

**S6. Write `turn.json`** (loop.go:695-718) with, at this moment:
`missionId`, `turnId`, `cycle`, `runtime`, `model`, **`hostSession`** (from
S3 — fixed *now*, before the host exists), `reconciliation`, `startedAt`,
`turnCapMin`, null process identity (`pid`, `pidStartedAt`, `pgid`,
`instanceTag`), `status: pending`, null outcome/error/detail, and the three
result paths (`result.json`, `return.json`, `raw.out`) inside the turn dir.

**S7. Assemble the prompt.** `mission.AssemblePrompt` (loop.go:721,
prompt.go:370-535) writes `turns/<turnId>/prompt.md` deterministically from:
turn.json + state.json + `ledger.md` + `asks/` + **the authored contract in
`plans/`** (prompt.go:422 — not the pinned snapshot; S1 verified live==pinned
at reserve time, but an edit between S1 and S7 would be assembled unnoticed
and only refused at the *next* cycle's reserve). The prompt is, in order
(prompt.go:503-515):

1. Machine header: `Mission-Id`, `Turn-Id`, `Cycle`, **`Host-Session`**
   (the announced session, `none` when null), `Runtime`, `Model`,
   `Reconciliation` (prompt.go:487-495). The Host-Session value was fixed
   at S6 from the *previous concluded turn's* result envelope.
2. The shipped orchestrator preamble, byte-exact
   (`scripts/agents/roles/orchestrator.md`).
3. `## Mission Contract` — the contract text.
4. `## Ledger Tail` — see §5 for exactly what this carries.
5. `## Human Answers` — the standing human rulings (issue #11): every ask
   named by a stream's `answeredAsk`, as
   `[askId, streamId, answeredAt, question, answer]` rows; a stream
   naming a missing or unanswered ask fails assembly loudly.
6. `## Open Asks` — every unanswered, un-superseded ask on disk as
   `[askId, streamId, reasonClass, question]` rows.
7. `## Streams` — `[id, state, goal, reason, answeredAsk]` rows.
8. `## Reconciliation` — when `reconciliation=yes`, the single most recent
   non-completed prior turn as `[turnId, outcome, detail]`.
9. `## Landed Returns` — the delegate rounds whose return landed but which
   no concluded turn certified or superseded, one row per owned chain as
   `[chain-root, round-or-marker, return-path-or-none]`, capped at 20 rows
   with an overflow summary (records/patience/patience-orphan-usage.md, landed.go).
10. `## This Turn` — the instruction template with cycle number and fence
   headroom (`cycles=<n>,jobs=<n>` from contract limits minus fence
   counters).

Sections 4-9 are framed between `<<<DATA>>>`/`<<<END>>>` markers with
marker-defanging, one row per line, tab-joined (prompt.go:231-248). Total
size is capped by `mission.max-prompt-kb` (default 256; over-size is a
refusal naming the widest block, prompt.go:522-533). Assembly failure →
`failTurnBeforeLaunch` (loop.go:722-726, 864-872): turn patched
`failed/prompt-refused`, then `recordFailedTurn` with `consecutiveFailures=2`
— an **immediate host-failure park** that still burns a ledger cycle (§6(h)).

**S8. Validate the prompt.** `scripts/assert-turn-prompt.sh` →
`validate turn-prompt` (loop.go:728-737, validate/turnprompt.go): framing,
header order, byte-exact preamble, section fencing, row grammar, and the
ask reason-class whitelist `PromptAskReasons` (orchestrator-raisable classes
plus the runner's own `fence` and `stop-loss`; missionrunner.go:42-48 records
the cohort that died when this whitelist was narrower). Refusal → same
immediate-park path as S7.

**S9. Launch the host.** `launchHost` (loop.go:742, host.go:167-342):

- Adapter = `scripts/agents/hosts/<runtime>.sh`; missing/non-executable is a
  **runner defect** (fails the runner, host.go:169-171).
- Instance tag `metasystem-host-<turnId>`. `--resume-session <hostSession>`
  is passed only when S6 recorded a session (host.go:186-188).
- Environment (host.go:194-208): git author pinned to the turn id,
  `METASYSTEM_MISSION_ID`, `METASYSTEM_OWNER_LINEAGE` (the shared mission
  lineage, so each turn's host renews rather than seizes the target lease),
  `METASYSTEM_MISSION_LEASE` (the runner's lease.json path),
  `METASYSTEM_MISSION_TURN` (the turn id — this is how dispatched job records
  get stamped), `METASYSTEM_HOST_START_GATE` (a gate file in the turn dir)
  and its timeout. These four are what make the host's dispatches
  mission-stamped (§3).
- Spawned `setsid`, stdout+stderr to `turns/<turnId>/host.log`.
- **Start proof** (host.go:237-259), up to 5s scaled: the child must lead its
  own process group (`pgid == pid`) and carry the minted tag on its command
  line (host.go:74-76). The runner heartbeats (with the turn id) each poll.
  Failure: wind down, patch turn `failed/start-unverified`
  (host.go:261-277), return launch detail `start-unverified` → S10.
- On proof: patch turn with `pid`, `pidStartedAt`, `pgid`, `instanceTag`,
  `status/outcome: running` (host.go:278-283); **release the start gate**
  (write `host.start`, host.go:284) — the adapter blocks on this file before
  invoking the CLI (hosts/claude.sh:20-31,53); then answer the launcher's
  start-signal exactly once per runner life (host.go:287, 153-162).
- **Cap loop** (host.go:299-313): heartbeat every
  `METASYSTEM_HEARTBEAT_INTERVAL_MS` (default 100ms) while the host runs;
  at `turnCapMin` minutes, wind the group down (SIGTERM, grace, SIGKILL —
  never signaling a group whose tag proof is gone, host.go:91-128), patch
  turn `failed/capped/turn-cap`, return detail `capped`.
- On natural exit: read `result.json` if present and parseable, else
  `result=nil` with detail "host exited without a usable result"
  (host.go:331-341).

The adapter's own contract (hosts/claude.sh): wait for the gate; run the CLI
with the prompt on stdin; copy the provider result to `raw.out`; extract
`return.json` and `usage.json`; read the provider `session_id` **after the
CLI exits** (there is no mid-turn session signal for host turns — session
discovery is post-hoc); exit 3 on CLI failure, **exit 6 when the session is
absent or differs from the resume session** (claude.sh:95-98), else write the
result envelope — exactly `{sessionId, outcome, usage, rawPath, returnPath}`
(host/result.go:9-25) — and exit 0.

- On runner crash during S9: the host (and its delegates) keep running and
  may finish, commit, and write `result.json`; nothing will ever adjudicate
  it (an orphaned return), and the next launch's stale-lease cleanup marks
  the turn `turn-lost` if the group is still alive. Plus the S1 ledger wedge.

**S10. Failure triage** (loop.go:746-781). Every branch here goes through
`recordFailedTurn` (loop.go:559-600), which is: resolve the branch head as
candidate sha → **append a ledger cycle line classified `no-progress`,
observed `unmeasurable:<detail>`** (loop.go:577-578; measurement never runs
on a failed turn) → `RecordFailureProposal` (turn-log entry with null
session/measurement, ledger cycle count advanced, park on
`consecutiveFailures >= 2`; cycle.go:139-164) → state write → anchor → if
parked `host-failure`, a second park write that adds the host-failure ask
(loop.go:596-598, cycle.go:178-187) → else stop-loss check (§S16).

| Branch | Condition | consecutiveFailures | Turn outcome recorded |
|---|---|---|---|
| start-unverified | launch proof failed (loop.go:758) | **2 — immediate park** | failed |
| unresumable | adapter exit 6 (loop.go:761-769) | `priorFailures` — **no increment** | unresumable (drops the session for the next turn, contract.go:132-137) |
| capped / non-zero exit / unusable result | loop.go:770-781 | priorFailures+1 | failed (the `capped` outcome written by S9 is overwritten to `failed`, error `host-failure`, detail `capped`) |
| protocol-error | adjudication refused the return (loop.go:785-794) | priorFailures+1 | failed |

**S11. Adjudication.** `AdjudicateFiles` (loop.go:784, turnio.go:32-60)
reads state, turn.json, result.json fresh from disk, then validates in this
exact order (`ValidateReturn`, adjudicate.go:68-119):

1. the envelope has exactly `{sessionId, outcome, usage, rawPath,
   returnPath}` (69-77);
2. `outcome == "completed"` (78-80);
3. `rawPath` and `returnPath` resolve inside the turn directory, symlinks
   resolved (81-87, 123-157);
4. `scripts/assert-return-complete.sh --role orchestrator` passes on the
   return file (89, turnio.go:16-26);
5. the return's `turnId`, `missionId`, `cycle` equal the turn record
   (96-107);
6. `identity.runtime` and `identity.model` equal the turn record (108-114);
7. **the session check sits last**: `identity.sessionId` must equal
   `turn.hostSession` — the session the *prompt announced*, nil on a first
   or post-unresumable turn — never the session the adapter discovered
   (115-118 and the comment at 63-67).

Any failure here is one `protocol-error` failed turn (S10). Then
`Adjudicate` (adjudicate.go:190-304) judges each claim against state and job
records:

- `dispatched[]`: accepted only if the job record exists, is stamped
  `mission == <this mission>` **and `turnId == <this turn>`**
  (220-237). A job dispatched in an earlier turn, or finishing later, can
  never be re-claimed.
- `streamUpdatesRequested[]`: stream must exist, transition legal for an
  orchestrator (`LegalStreamTransitions`, missionrunner.go:67-72 — parked
  streams can only be re-activated by an answered ask, never by the
  orchestrator), park requests need a reason. Accepted updates are applied
  to the verdict's stream map (239-265).
- `askCandidates[]`: stream must exist, reason class in `KnownAskReasons`
  (267-283). Accepted ones become asks `ask-<cycle>-<n>`.
- **Every rejection becomes a `host-failure` ask** `rejected-<cycle>-<n>` on
  the entry's stream or the fallback stream (285-299).

**S12. Persist the verdict and its asks** (loop.go:798-810): write
`turns/<turnId>/adjudication.json`, mkdir `asks/`, write every proposed ask
record exactly as proposed (loop.go:452-461). Ask records:
`{askId, streamId, reasonClass, question(one-line), createdAt, answeredAt:
null, answer: null}` (adjudicate.go:342-353).

- On crash after S12: the asks exist and will appear in every later prompt;
  the accepted stream updates do **not** exist (they only land at S15);
  plus the S1 wedge.

**S13. Drain the mission's jobs.** `drainJobs` (loop.go:812, 605-621): an
unbounded loop — list `ActiveJobs` (every readable job record stamped for
the mission, or reservation-keyed with an empty stamp, whose status is not
terminal; jobs.go:23-81), run `dispatch.sh reap --job <job>` on each
(output and failures discarded), sleep 100ms, repeat **until the list is
empty**. There is no deadline, no fence check, and **no heartbeat** inside
this loop. What reap does is dispatch.sh's judgment (reap_one_locked,
dispatch.sh:741-848): a running job with a live, matching supervisor and an
unexpired budget is left alone; budget expiry → `timeout` plus a mission
fence refusal ask; lost supervisor → `process-lost`. Two consequences:

- The drain normally lasts up to the largest remaining `capMin` among
  still-running delegates — the runner waits for delegates the host left
  running.
- A `pending-setup` husk is only failed by the **standing** reaper
  (`standing_reaper` gates the abandoned-setup verdict, dispatch.sh:774-790);
  the runner's one-shot `reap --job` returns without acting. If no standing
  reaper is armed, `drainJobs` spins forever on such a husk — the runner's
  drain is bounded only by an *external* process.

**S14. Measure.** `measure` (loop.go:815, 640-659): parses the **live
authored contract** (`parseContract(false)` — again `plans/`, not the pin),
computes `PreviousMetrics` from the most recent turn-log entry carrying every
declared metric (contract.go:142-168; sealed baseline otherwise), and runs
`mission.ContractMeasure`: gate command in a throwaway worktree of the
candidate branch with gate/truth paths restored to `gate.ref` bytes, guards
under `fence.job-cap-min` (measure.go:38-278). Classification vocabulary
produced here: `contract-improved` (some metric improved past noise, none
regressed), `unresolved` (all within noise), `no-progress` (any regression
or mixed) (measure.go:104-135). **Any failure to measure is itself a
measurement**: `no-progress, unmeasurable:<err>`, gate not passed
(loop.go:641-647).

**S15a. Append the ledger line.** `appendLedger` (loop.go:825, 528-537):
first compute the `best=yes|no` marker by replaying the whole ledger fold
against the pinned gate (stoploss.go:364-391 — replay-semantics missions
only), then `mission.AppendCycle` under the ledger lock: refuses unless the
cycle number is exactly one past the recorded count, appends
`### Cycle N` + `- Classification: <class>; candidate-sha=<sha>;
observed=<metric=value,...>; best=yes|no` (ledger.go:129-171).

**S15b. Write `measurement.json`** (loop.go:828-836):
`{measurement: {metrics, guards, candidateSha} | null, gatePassed}`.

**S15c. Conclude.** `ConcludeFiles` (loop.go:838, turnio.go:67-107) re-reads
all six artifacts (state, turn, verdict, return, result, measurement) from
disk, overwrites the fresh state's streams with the **adjudicated** stream
map, and `ConcludeTurn` (cycle.go:92-132) builds the proposal:

- turn-log entry: `{turnId, cycle, outcome: completed, detail, sessionId
  (from the result envelope — the discovered session, which becomes the next
  turn's Host-Session via S3), measurement, accepted, rejected, certified,
  factsForLedger, gaps}`;
- `state.ledger.cycles = turn.cycle` (the fence number);
- `waitingList` = open asks on disk right now;
- `ProjectFences` (cycle.go:19-69): copies `fences.json` counters, counts
  reservations and still-active reservations (a reservation with no readable
  record counts active — "losing a record must never relax a fence"), and
  copies `usage.json` units;
- status: `gatePassed` → `completed`; no active stream → parked
  `all-streams-parked`; else `running`.

**S15d. Apply the state.** `writeState` (loop.go:843, 251-287): re-reads the
current state from disk for the expected hash, then `mission.WriteState`
under the state lock — compare-and-write against that hash, transition
legality enforced (immutable identity, stream-set fixed, stream transitions,
non-decreasing counters; state.go:645-706, 729-762).

- Because the expected hash is read at conclude time, a human `answer`
  landing mid-turn is not clobbered at the hash level — but the conclude
  proposal's streams were adjudicated from the *turn-start* state, so an
  answer that re-activated a stream during the turn is either reverted or,
  because reverting `answeredAsk` outside a human-answer transition is
  illegal (state.go:667-673), refuses the write and **fails the runner**.

**S15e. Patch the turn to completed** (loop.go:847-853): status/outcome
`completed`, embedded result, canonical raw/return paths.

**S15f. Anchor.** (anchor.go): a plumbing-built commit on the RUNNER-OWNED
anchor ref `refs/metasystem/missions/<mission>/state-anchors` — never the
mission branch — whose tree carries exactly the ledger bytes, with
trailers binding `Mission-State-Hash`, `Mission-Ledger-SHA256`, ledger
path and cycle count; the ref advances with compare-and-swap. The mission
branch and the real index are untouched (the wall made the old on-branch
`git add -f` doctrine untenable: it force-tracked bookkeeping into every
delegate worktree and split the wall's tree identity from the branch's).
Refuses when: the checkout is not on the mission branch, or the ledger
count disagrees with the state.

- On crash between S15d and S15f: the next resume's `Reconcile` sees a
  state hash no anchor certifies → **`state-integrity` park**, human
  required (anchor.go:409-418).
- On crash between S15a and S15d: `Reconcile`'s ledger-ahead branch adopts
  the count, losing the turn-log entry (see §1); the next turn resumes the
  *older* session id and measures against the older baseline.

**S16. Stop-loss check.** `continueOrParkStopLoss` (loop.go:857, 539-554):
only on a still-running state, derive the verdict as a **pure replay** of
(pinned contract, ledger events) — never a cached counter
(stoploss.go:313-336). Replay semantics (ledgerSemantics=2): `stagnant`
counts every `no-progress`/`unresolved` cycle since the last `best=yes` line
or reset line; cycle budget dominates stagnation (stoploss.go:212-246).
Legacy missions replay the old shell rules (255-293). Tripped → park with
reason `stop-loss` and a kind-worded ask (`stagnation` names the `reset:`
answer; `cycle-budget` is amendment-only; loop.go:484-497, cycle.go:193-199,
stoploss.go:45-54). Note the check runs **only here** (and after failed
turns via recordFailedTurn) — a crash after S15f re-runs a full cycle before
the fuse is consulted again.

Park mechanics, all reasons (`applyPark`, loop.go:501-513): asks are written
*first* so the state never names an unanswerable ask, then the parked state
(waiting list merged from disk + new asks), then the anchor. Park reasons
that can appear in state: `fence`, `host-failure`, `all-streams-parked`,
`stop-loss`, plus reconcile-time `state-integrity` (and the schema also
admits `gate-integrity`, `contract-changed`; state.go:33-36).

## 3. Who dispatches what

**The runner never dispatches delegate work.** Its only child processes are
the per-turn host adapter (S9) and `dispatch.sh reap`/`close` invocations
(S13, mission end). Everything else is the host's doing, inside its turn:

- The host inherits `METASYSTEM_MISSION_ID`, `METASYSTEM_MISSION_LEASE`,
  `METASYSTEM_MISSION_TURN` from the adapter environment (host.go:194-208).
  When it runs `dispatch.sh dispatch --role <r> --brief <f>`,
  `resolve_mission` (dispatch.sh:523-539) picks those up and
  `ValidateMission` (internal/dispatch/mission.go:22-77) proves the lease:
  canonical path, exact field set, **live pid in the recorded pgid with the
  recorded instance tag on its command line** — i.e. a delegate can only
  join a mission whose runner process is alive at dispatch time.
- Fence reservation: mission dispatches are cap-authorized by
  `mission fence-authorize-cap` under the fence lock (dispatch.sh:1000-1006,
  fence.go:434-514): the (runtime, model) pair cap from the signed contract
  (`cap.min.<runtime>.<model>`) or `fence.job-cap-min`, deadline truncated by
  mission wall clock, and **the reservation is recorded in
  `fences.json.reservations[jobId]`** with cap and deadline. Reservations
  are never removed; "active" is judged from the job record's status
  (contract.go:210-214). Every round of a chain gets its own reservation
  (follow_up authorizes the child too, dispatch.sh:1184-1188).
- The job record (`artifacts/agents/jobs/<jobId>.json`,
  internal/dispatch/build.go:161-213) is stamped `mission` and
  `turnId` (the dispatching turn) at build; the handshake keeps an existing
  turn stamp (handshake.go:102-114). `round: 1`, `parentJob: null` on a
  root.
- **Chain roots and rounds**: the chain root id is simply the first
  dispatch's job id (`<role>-<utc>-<hex>` or `--job-id`). A follow-up
  resolves the root by walking `parentJob`, reads the latest chain record,
  and mints `round = latest+1`, child id `<root>-r<round>`
  (dispatch.sh:1099-1124). Round prompts live at
  `artifacts/agents/<root>/rounds/<n>/prompt.md`, round returns at
  `rounds/<n>/return.json` (write_prompt dispatch.sh:576-580;
  critique.go:106). Round identity is owned by the job record, never by the
  delegate's return (critique.go:90-99).
- Critique budget: enforced by dispatch.sh at follow-up time, not by the
  runner — a critic chain with open material findings at a round divisible
  by 3 requires the successor message to enumerate every open finding id,
  recorded once as `critiqueExhaustions` on the chain root; a second
  exhaustion is refused outright (critique.go:12-18, 186-341).

**What the runner can see of all this**: job records (role, mission, turnId,
round, parentJob, status, usage, critiqueExhaustions), `fences.json`
reservations, and chain shape via parentJob walks (jobs.go:88-135). At
adjudication it verifies only that a *claimed* dispatch's record exists with
this mission+turn stamp.

**What the runner cannot know today**: whether a critique round *closed*.
Closure — every material finding dispositioned
(`scripts/assert-critique-closed.sh` joining a round return's findings
against a Markdown dispositions table) — is a skill-level check over
artifacts the runner never reads (round `return.json` files and dispositions
documents in `plans/`). No job-record field records closure; the runner sees
only terminal statuses and exhaustion entries. The runner also cannot see
work a delegate produced after its dispatching turn concluded: `dispatched`
claims are only accepted for the current turn (adjudicate.go:229-231), and
nothing re-scans finished chains at later conclusions.

The runner's two dispatch.sh touchpoints: S13's `reap --job` per active job
each drain pass, and at mission end `reap` + `close --job <root>
--runner-closed` for every all-terminal, not-yet-closed chain
(loop.go:625-635, dispatch.sh:1274-1300 — close verifies chain closability
and CAS-patches `chainClosed`/`runnerClosed` onto the root record).

## 4. The ask/answer machinery

Ask files live at `missions/<id>/asks/<askId>.json`; the record carries
`askId, streamId, reasonClass, question, createdAt, answeredAt, answer,
supersedes, supersededBy` (plus `stopLossKind` on stop-loss asks and
`supersededAt` once closed). SUPERSESSION (issue #11): an orchestrator ask
candidate may name an open same-stream ask in `supersedes`; adjudication
validates it (grammar-checked id — also the traversal guard — open, not
already superseded in this pass or on disk, same stream), the successor
ask is written FIRST, then the predecessor closes with
`supersededBy`/`supersededAt` and the `ask-superseded` event. Superseded
asks leave Open Asks, the waiting list, and the park guarantee's open
predicate alike; answering one refuses by name toward its successor.
Writers:

| Creator | When | Ids |
|---|---|---|
| Runner, from adjudication | S12 | `ask-<cycle>-<n>` (accepted candidates), `rejected-<cycle>-<n>` (rejections, reasonClass host-failure) |
| Runner, park proposals | S10 park, S16 park | `host-failure*`, `stop-loss*` (nextAskID suffixing, adjudicate.go:357-371) |
| Mission fence, any process holding the fence lock | cycle reserve (runner, S1), job authorize (host's dispatch), reap timeout refusal | one batched `fence-bound*` ask, reasons merged into its question (fence.go:274-357) |

The prompt's Open Asks section carries every unanswered ask that no
successor supersedes — closure is DERIVED from a successor naming the ask
in `supersedes`, so a crash before the predecessor's marker write cannot
resurrect the stale question; the state's
`waitingList` is refreshed from disk at every conclude and park under the
same rule. The runner
never blocks on an ask mid-cycle; an unanswered ask only matters when a park
ends the loop.

**Answering** (`Engine.Answer`, answer.go:17-129) is human-initiated, in a
separate process, with no lease or liveness check. One-shot: an ask with
`answeredAt` set refuses re-answer (answer.go:34-37). Per reason class:
`reserved-decision`/`red-test`/`merge-conflict` require the named stream to
be `parked-reserved`, reactivate it with `answeredAsk`, unpark the mission;
`host-failure` requires the matching park reason and an active stream;
`fence` re-preflights the live contract and unparks only when the fences are
no longer reached (answer.go:79-99). The ask record and the state advance
together or not at all — the ask write is rolled back if the state write or
anchor refuses (answer.go:109-126).

**Stop-loss answers** take a dedicated path (answer.go:140-200) in binding
order: (1) `AppendReset` ledger line under the ledger lock — the
authoritative fact; (2) mark the ask answered; (3) unpark state write +
anchor. Crash between 1 and 2: re-answering is lawful, a second reset line
is harmless. Crash between 2 and 3: the next `resumeState` applies the
unpark (`applyPendingReset`, loop.go:394-430). Only a `stagnation` park
accepts `reset:`; a `cycle-budget` park is amendment-only.

**Parks and resume vs. cycle boundaries**: a park always happens at a cycle
boundary (S1 refusal, S10 failure conclusion, S15c/S16) — the loop condition
then exits the runner entirely (loop.go:93-100); parked missions have no
live runner by design (status.go:54-60 treats "running state, concluded
runner record" as awaiting-resume). Resume, after a human answer unparks the
state, is a **new runner process** re-entering at the top of the loop: the
next cycle is a fresh S1 with a fresh turn id. Nothing of an interrupted
turn is replayed. What a resume *does* replay: the pending stop-loss unpark,
and the ledger-ahead-of-state adoption (§1). What it silently drops: the
crashed turn's turn-log entry, so `PriorContext` reaches back to the last
*concluded* turn — including its (possibly stale) session id.

## 5. The ledger

File: `missions/<id>/ledger.md`. Every mutation takes
`ledger.md.lock` and writes atomically (ledger.go:287-340). The complete set
of line kinds that exist today:

| Line | Writer | Lock | Sequence point |
|---|---|---|---|
| `# Mission Ledger` header | runner, `InitLedger` | ledger | mission start (loop.go:327) |
| `- Cycle budget: N` | runner, `InitLedger` | ledger | mission start |
| `- No-gain budget: N` | runner, `InitLedger` | ledger | mission start |
| `### Cycle N` + `- Classification: <class>; candidate-sha=<sha>; observed=<pairs>[; best=yes\|no]` | runner, `AppendCycle` | ledger | S15a (accepted turn) or S10 (failed turn); exactly one line per cycle, contiguous from 1 |
| `Stop-loss reset: ask=<id>; reason=<text>` | **the answer CLI process**, `AppendReset` | ledger | whenever a human answers a stagnation stop-loss ask — between cycles by construction (parked mission has no live runner) |

Classification vocabulary accepted by the parser: `contract-improved`,
`falsified-continue`, `falsified-dead-end`, `no-progress`, `unresolved`,
`invalid-run` (ledger.go:22-29). The runner's own writes only ever produce
`contract-improved`/`unresolved`/`no-progress` (measure.go:127-134) and
failed-turn `no-progress` (loop.go:578). Replay reads the file as an ordered
event stream of cycle lines and reset lines (ledger.go:227-276).

**What the prompt's Ledger Tail actually carries** (prompt.go:93-128,
441-465): the last `mission.ledger-tail-cycles` (config chain: env override
→ `metasystem.conf.local` → `metasystem.conf` → default 5, max 50) **cycle
blocks only**, each reduced to the row
`[cycle, classification, candidate-sha, observed, best?]` extracted by a
regex that matches only `- Classification:` lines. Reset lines, budget
lines, and any future line kind are **not** carried into the prompt — this
is the fact the parent design tripped over when it assumed new ledger line
kinds (orphan entries, loop credits) would "land in the prior-context ledger
tail". A cycle block containing anything that makes its classification line
count ≠ 1 fails prompt assembly outright (prompt.go:108-111).

Consistency couplings: `AppendCycle` refuses a non-contiguous cycle number
(ledger.go:139-141); the anchor refuses when the ledger count disagrees with
`state.ledger.cycles` (anchor.go:165-172); `Reconcile` treats the ledger as
truth over the state (anchor.go:382-407).

## 6. False-stall surfaces

Every place in the sequence where a cycle is (or the whole mission becomes)
recorded as valueless or dead for reasons that are not the host's or
delegates' fault. The ledger effect in every per-cycle case is the same:
one `no-progress` line (usually `unmeasurable:*`), which the replay counts
toward `stagnant`, plus a spent cycle toward the cycle budget.

(a) **Identity mismatch — announced vs. observed session.** ValidateReturn
compares the return's `identity.sessionId` to what the prompt announced
(S11 step 7), and the adapter separately fails the turn when the provider's
discovered session differs from the resume session (exit 6,
claude.sh:95-98). A runtime that rotates session ids on every resume books a
`no-progress` cycle per turn while the host works honestly; an honest model
echoing the *observed* rather than the *announced* session (e.g. first turn,
where announced is null) is a `protocol-error` no-progress cycle. Every
protocol-shaped slip (an extra envelope field, a return path through a
symlink out of the turn dir, a schema miss) books a full cycle the same way.

(b) **Starved dispatch.** During the host's turn its dispatches can be
refused by the mission fence (jobs, concurrency, job-cap-min, wall clock)
— the refusal writes the batched fence ask and the host simply cannot act;
the resulting cycle measures `no-progress`/`unresolved`. Concurrency
counts every reservation whose job record is missing or unreadable as
active forever (contract.go:210-214, cycle.go:38-46), and reservations are
never released, so leaked records permanently shrink effective concurrency.
Dispatch preconditions outside the fence (fresh census, watcher cap
ceiling, lease-holder entry check; dispatch.sh:982-983, 1011-1012) starve
the same way. The mission burns cycles while structurally unable to act.

(c) **Orphaned returns.** Value that lands where no adjudication will ever
read it: a delegate chain finishing after its dispatching turn returned
(only same-turn `dispatched` claims are accepted, adjudicate.go:229-231); a
host that crashed or was capped after writing `result.json` (S10 branches
never adjudicate the return, even a complete one — a capped turn with a
finished return is booked `no-progress, unmeasurable:capped`); a runner
crash mid-S9 leaving a finished result no resume replays; chains still
running when the mission parks (mission-end close only reaps/closes, never
banks content, loop.go:625-635).

(d) **start-unverified.** The 5s-scaled start proof (host.go:237-259) failing
— slow spawn under load, adapter environment issues — books a cycle **and**
parks host-failure immediately (`consecutiveFailures=2` hardcoded,
loop.go:759), without the host ever running.

(e) **Failed turns never measure.** `recordFailedTurn` writes
`unmeasurable:<detail>` without running the gate (loop.go:577-578). A capped
or non-zero-exit turn whose repository work in fact improved the metrics is
still a `no-progress` line; the improvement is only visible if a *later*
cycle concludes and measures (and then that later cycle, not this one,
resets the stagnation count).

(f) **Measurement infrastructure failure.** Any gate/guard plumbing error —
worktree creation, `gate.ref` resolution, a guard command environment
problem, a guard hitting `fence.job-cap-min` — is classified
`no-progress, unmeasurable:*` by design (loop.go:640-647,
measure.go:141-183) and counts toward stagnation.

(g) **The reserve/append crash wedge.** A runner death (or kill) anywhere
between S1 and S15a leaves `fences.cycles` ahead of the ledger count. All
subsequent appends pass the fence number and are refused as non-contiguous
(ledger.go:139-141), which is the runner's *fail* exit, not a park — resume
succeeds, then dies at the next cycle's ledger append, every time. Nothing
in the resume path repairs this direction of desync (Reconcile only heals
ledger-ahead-of-state). Related, smaller windows: crash between state write
and anchor → `state-integrity` park (human required); crash between ledger
append and state write → healed, but the turn's session id and measurement
are dropped from the turn log, so the next turn resumes a stale session —
feeding surface (a).

(h) **Runner-side prompt refusal burns a cycle.** Prompt assembly or the
prompt validator refusing (S7/S8) — an oversized ledger tail block, a
foreign line in a cycle block, an ask with an unexpected reason class, a
preamble byte drift — is booked as a `no-progress` cycle plus an immediate
host-failure park (loop.go:864-872), although it is by the code's own
comment "a runner-side defect".

(i) **Mid-turn human answers.** An `answer` applied while a turn is in
flight either has its stream reactivation clobbered by the conclude
proposal's adjudicated streams or, when the `answeredAsk` change makes the
conclude transition illegal (state.go:667-673), refuses the conclude write
and fails the runner (§S15d).

(j) **Unbounded drain.** S13 has no deadline and depends on an external
standing reaper for pending-setup husks; a wedged drain does not heartbeat,
so the runner looks dead to `status`-style liveness probes while it is
lawfully waiting — and wall clock burns against the mission fence the whole
time.

(k) **Anchor refusals from checkout state.** The anchor refuses on a
checked-out branch other than the mission branch (anchor.go). Staged index
changes no longer matter: the anchor is plumbing-built onto the
runner-owned anchor ref through an isolated temporary index, so the real
index is never read or swept. A host switching branches still turns the
*runner's* conclude into a hard
failure after the ledger line already landed — surface (g)'s second window,
triggered without any crash.

## 7. Reference: artifact inventory per turn

`missions/<id>/turns/<turnId>/` after a fully concluded cycle: `turn.json`
(S6, patched S9/S10/S15e), `prompt.md` (S7), `host.start` (S9 gate),
`host.log` (S9), `raw.out`, `return.json`, `usage.json`,
`claude-result.json` (adapter), `result.json` (adapter envelope),
`adjudication.json` (S12), `measurement.json` (S15b). Mission-level:
`state.json` (+`.lock`), `ledger.md` (+`.lock`), `fences.json`,
`mission-fence.lock`, `usage.json` (fence aggregate), `asks/*.json`,
`lease.d/`, `lease.json`, `mission-<id>.contract.md` (pin);
`missions/runners/<id>.{json,heartbeat,log}`.
