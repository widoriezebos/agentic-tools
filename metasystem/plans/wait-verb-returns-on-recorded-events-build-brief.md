Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Goal

Build member 1 of coordinator-wakes-on-events-not-polls: the wait verb
over the existing records, as the design page
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md decides
in sections 2, 5 and 6 (the page is the record; where this brief and the
page differ, the page wins and you report the difference as a gap). DONE
for this member: the installed commands `metasystem wait --job ID`,
`--run ID`, `--attempt ID` and `--goal ID --event landing|human-act
--after COMMIT [--verb V] [--question ID] [--chain ROOT]` each return on
the recorded event or the bounded deadline with the typed exits of
section 2 (0 terminal or matching ledger event; 1 failed or red; 2 target
timeout or unknown terminal; 3 cancelled or launch-failed; 4 missing,
replaced or invalid source record; 6 actionable work or ownership change;
64 live waiter or ineligible registration; 65 storage or transport
failure; 66 uncertain identity; 67 invalid arguments; 124 this wait's
deadline; 130 interrupted); every adapter's wait-delivery operation
answers `blocking` and the row records it; the version-2 waiter row and
its by-id pointer exist; `--resume WAIT-ID` recovers from the rows alone;
session start prints one WAITING line per pending row with the resume
command where a start hook exists, and `goal next` and `report
turn-verdict` print the same lines.

# Workspace

Your job worktree (the dispatcher names it), branch agent/<job>. You may
change: metasystem/internal/run/waiter.go and its tests,
metasystem/internal/dispatch/watch.go and its tests,
metasystem/internal/proofrun/attempt.go and metasystem/internal/proofrun/launcher.go
tests, metasystem/internal/goal/attention.go and its tests,
metasystem/cmd/metasystem/main.go, metasystem/cmd/metasystem/run.go,
metasystem/cmd/metasystem/goal.go, metasystem/cmd/metasystem/channel_verbs.go
(the cursor `channel ask` saves and `channel wait` reads), a NEW file
named wait_verb.go beside run.go in the cmd/metasystem directory (the
verb) with its test file wait_verb_test.go,
metasystem/internal/report (the WAITING lines), metasystem/internal/host,
metasystem/internal/identity/identity_linux.go and
metasystem/internal/identity/identity_darwin.go (boot-clock samples),
metasystem/scripts/agents/adapters/runtime-common.sh and the four adapters
beside it (the wait-delivery operation answering `blocking`), and
metasystem/scripts/agents/supervision-hook.sh only where the `start`
event prints the WAITING lines. Do not touch the stop gate
(metasystem/internal/goal/turnverdict.go beyond reading), the publication
owners (metasystem/internal/goal/txn.go, the launcher's terminal commit,
the job transition writer) or the watch wrappers: those are members two
and three. Do not commit.

The build cache is provided: the adapter sets GOCACHE, GOTMPDIR and
STATICCHECK_CACHE to the chain's cache in the worktree's git dir, shared by
every round; never set, unset or strip them. The proof of your round is
made by the orchestrator's engine on the worktree as you leave it
(metasystem job prove-round); run the focused tests and the fast gate for
what you change, and leave everything you return in the worktree,
uncommitted.

# Inputs

- The design page, sections 2 (the verb, the record, the readers, the
  adapter operation, the recovery), 4 (the bounds) and 5 (the fixtures
  named for this member). Build exactly what section 2 says; the page's
  wording of each exit and each refusal is the contract.
- metasystem/internal/run/waiter.go: the existing Waiter record, WaiterPath
  and WaitersDir, the bounded waiter lock; version 2 extends it, legacy
  rows (no schemaVersion) stay readable for legacy watch liveness.
- metasystem/internal/dispatch/watch.go: the job observation the verb
  reads through its owner; metasystem/internal/proofrun/attempt.go: a
  valid Terminal with EndedAt equal to Terminal.At is the attempt event;
  metasystem/internal/goal/attention.go (CaptureTipBounded): the ledger
  observation from the saved cursor with the existing acceptance gates;
  metasystem/scripts/agents/commit.sh writes the Goal-Item and
  Landing-Provenance trailers a landing wait matches.
- metasystem/scripts/agents/adapters/runtime-common.sh: the executable
  adapter contract; add the wait-delivery operation (waitId, nonce,
  deadline, session reference in; one word `blocking` out, exit 0; exit 2
  declines) and answer `blocking` in claude.sh, codex.sh, devin.sh and
  fake.sh.
- metasystem/cmd/metasystem/channel_verbs.go: `channel ask` saves the
  accepted ledger tip it read before publishing the question in the
  question record; `channel wait` becomes the human-act wait with
  `--verb answer --question ID` and `--after` from that record; a legacy
  question without a cursor is refused unless `--after` is given.

# Constraints

- Go owns the logic and the tests; shell only launches. Bed legs
  wait-job-run, wait-proof, wait-ledger, wait-restart and wait-bounds are
  Go tests or existing bed scenarios extended, never new shell assertions.
- Injected clocks, source readers and hint delivery in tests; no wall
  time in Go tests; the hint pipe of member two is not built here, but the
  waiter's ten-second reread and its bounded reads are.
- Plain-English error texts; source comments say what and why, never
  which round or finding.
- Non-goals: the stop gate's PendingWait rows, the publication hints, the
  compatibility wrappers, the measurements.
- Wall clock: 90 minutes for this round. Stop and report a gap if the
  member cannot be finished in it; a partial round returns with its tests
  green for what exists.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json):
jobId, round, runtime, sessionId, model, evidence, gaps, mode, riskiestPart,
diffBoundary, whatWasDone. evidence carries `{command, observed, level}`
items replayable verbatim from the worktree's repository root: (1)
`git -C metasystem status --short`; (2) the focused Go tests you ran, one
command per package, with their pass lines; (3)
`( cd metasystem && scripts/agents/go-gate.sh --fast )` with its last
line. whatWasDone names each fixture of section 5 this member owns and
says whether it exists and passes.

Every path in your return (diffBoundary, files) is relative to the
repository root, so it starts with `metasystem/`.

# Acceptance Criteria

- `metasystem wait` exists with the four selectors, `--resume`, the
  common arguments and the typed exits; a wait returns on each recorded
  event and on the deadline in the tests.
- The version-2 row, its pointer, the legacy rule and the same-key refusal
  are tested.
- Every adapter answers `blocking`; TestWaitAdapterBlocking proves it.
- `channel ask` saves its cursor; TestWaitChannelAnswer covers the two
  questions and the answer landing between ask and wait.
- The recovery replay and the failure bounds tests pass; the WAITING
  lines print from session start, goal next and report turn-verdict.
- The fast gate passes.

# Gap Rule

stop and report a gap; never fill it silently.
