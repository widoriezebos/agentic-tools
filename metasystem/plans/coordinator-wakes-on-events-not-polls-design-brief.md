Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal coordinator-wakes-on-events-not-polls)
Date: 2026-09-13

# Goal

Author the design document coordinator-wakes-on-events-not-polls-design.md,
a NEW file you create in the metasystem plans directory (the directory that
holds delivery-efficiency-plan.md), for goal
coordinator-wakes-on-events-not-polls (goal 14 of
metasystem/plans/delivery-efficiency-plan.md, tier 3, origin human, the
goal record metasystem/plans/goals/coordinator-wakes-on-events-not-polls.md
carries the full intent). Coordinator seats spent 302 of 360 session hours
waiting: 25 percent in 2,139 blocking polls (90 hours, issued by turns
carrying 982 million prompt tokens), 28 percent idle awaiting a background
notification, 30 percent idle awaiting a human. The waiting is intrinsic;
the cost is the model waking to ask. DONE: (1) a seat waiting on a delegate
job, a proof attempt, a landing or a human act issues one wait verb
(metasystem wait --job, --attempt or --goal) that returns on the event or a
bounded deadline, with no polling by the model; (2) a runtime's native wake
(a Claude Code background-task notification, a Codex or Devin session
event) accelerates the return through its adapter, and a runtime without
one gets the same verb with bounded blocking and a records-based resume;
(3) the seat's stop gate on every runtime does not force a turn while a
registered wait is pending; (4) a missed event or a seat restart recovers
by re-registering the wait; (5) measured by model wake-ups per pending hour
(under 4), prompt tokens spent while pending (under 5 percent of the
seat's day) and event-to-resume latency (under 60 seconds), proven on one
coordinator session carrying a whole goal on each of two runtimes. The
mechanism lives in the Go engine, the ledger verbs and the adapter
contract, never in one runtime's hook, harness, CLI or transcript format.

# Workspace

Your job worktree (the dispatcher names it), branch agent/<job>. Create
exactly one file, coordinator-wakes-on-events-not-polls-design.md, in the
metasystem plans directory. Touch nothing else. Do not commit: the
dispatcher and the proof read the worktree as you leave it.

# Inputs

- The waiting a seat does today, each a blocking poll by the model or a
  wake to ask: metasystem/cmd/metasystem/run.go (`job watch`, function
  runJobWatchVerb, and `run watch`, runRunWatch, both polling records on
  `--poll-ms`); metasystem/scripts/agents/dispatch.sh (`watch --job`, the
  handshake and status polls near lines 988 to 1083); the proof attempt
  record metasystem/internal/proofrun/attempt.go (AttemptTerminal, EndedAt,
  Terminal: a nil terminal is nonterminal work) and its launcher
  metasystem/internal/proofrun/launcher.go; the goal ledger record and its
  human acts (metasystem/internal/goal/txn.go publishes every verb as a
  commit on the ledger ref, so a human act is a ledger event); the
  steward's durable notification queue metasystem/internal/steward/notify.go.
- The stop gate's live-work reading: metasystem/internal/goal/turnverdict.go
  (the idle-with-backlog and open-work decisions; WORK IN FLIGHT exempts a
  seat with a live job) and the hook
  metasystem/scripts/agents/supervision-hook.sh; a registered wait must be
  visible to that decision on every runtime.
- The adapter contract: metasystem/scripts/agents/adapters/runtime-common.sh,
  metasystem/scripts/agents/adapters/claude.sh,
  metasystem/scripts/agents/adapters/codex.sh,
  metasystem/scripts/agents/adapters/devin.sh, and the doctrine in
  metasystem/docs/orchestration.md and
  metasystem/docs/design/turn-verdict-delivery-contract.md (the engine
  decides; adapters own provider flags and delivery; an accelerator never
  carries correctness).
- The seat-side patterns the goal names (watch, TaskOutput, tmux capture
  loops, Monitor) are harness facilities of one runtime; treat them as
  accelerators a Claude adapter may use, never as the mechanism.

# Constraints

- The page is one design page in plain English (short sentences, no
  LLM-style prose, no bullet padding), at most 240 lines, with these
  sections: 1. The wait sites today (every place a seat blocks or wakes to
  poll, with file:line evidence, which record each waits on, and what
  event ends it); 2. The mechanism (the wait verb's arguments and exit
  codes; the registered-wait record: where it lives, its identity, who
  writes and clears it; how the verb returns on a job terminal, a proof
  attempt terminal, a landing on the ledger ref and a human act on a goal,
  with a bounded deadline; how a runtime's native wake reaches the verb
  through its adapter and what a runtime without one does; how the stop
  gate reads a pending registered wait and on which runtimes; how a missed
  event or a seat restart re-registers the wait from the records alone);
  3. What does not change; 4. Risks (a wait that never returns, a stale
  registered wait exempting an idle seat, an accelerator that lies, lock
  or clock drift); 5. Fixtures (name each Go test and bed leg, what it
  sets up and what it asserts, including the restart-recovery replay, the
  stop gate with a pending wait under the fake adapter and one real
  runtime, and how the three measurements of DONE clause 5 are taken on
  two runtimes); 6. Landing (which files, whether every seat must rebuild
  and re-arm, and the order of the slices if the build is more than one
  landing).
- Every path you cite must exist in the worktree under the metasystem/
  prefix. No globs. Do not write code. Do not run bin/metasystem test run,
  test plan or test verify.
- Non-goals: any change to what the stop gate refuses for open work or
  idle backlog beyond honouring a registered wait; any runtime-specific
  mechanism as the carrier of correctness.
- Wall clock: 40 minutes. Stop and report if the page is not finished by
  then.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json):
jobId, round, runtime, sessionId, model, evidence, gaps, mode, riskiestPart,
diffBoundary, whatWasDone. evidence carries exactly two `{command, observed,
level}` items, replayable verbatim from the worktree's repository root: (1)
`git -C metasystem status --short` observing the one new file; (2)
`( cd metasystem/plans && wc -l coordinator-wakes-on-events-not-polls-design.md )`
observing the line count. whatWasDone names the wait sites counted in
section 1 and the record the registered wait lives in.

Every path in your return (diffBoundary, files) is relative to the
repository root, so it starts with `metasystem/`.

# Acceptance Criteria

- The one new file exists in the metasystem plans directory under the name
  above, with the six sections, and every path it cites exists.
- Section 1 lists every wait site with file:line evidence; section 2
  defines the verb, the record and the restart recovery; section 5 names a
  fixture for each of DONE's five clauses.

# Gap Rule

stop and report a gap; never fill it silently.
