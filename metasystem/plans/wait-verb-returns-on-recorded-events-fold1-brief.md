Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Goal

Round 2 of the build of member 1 (round 1 stopped on four gaps in the
brief, all answered here; the worktree is unchanged). The goal, the
design page and the return schema are the build brief's (round 1's brief,
which the dispatcher already gave you and which is not yet a repository
file); this brief widens the workspace and decides the four points, then
the build proceeds as that brief says.

# Workspace, widened

Everything the build brief allows, and also:

- metasystem/internal/channel/question.go and its tests: `channel ask`
  records the accepted ledger tip it read immediately before publishing
  the question, in the durable question record, as the field
  `ledgerCursor` (the commit id), written in the same transaction that
  writes the record and before provider and ledger publication.
- metasystem/internal/goal/verbs.go and metasystem/internal/goal/file.go
  and their tests: the accepted answer act's History row carries the
  question identifier it answers (a `question` field on the row, written
  by the answer verb from its request; an answer without one is refused
  by the verb). This is the record the answer wait's predicate matches;
  it is not a publication hint (member two) and not the stop gate
  (member three).
- metasystem/internal/adapter and its tests: the Go adapter port for the
  wait-delivery operation (TestWaitAdapterBlocking lives in
  metasystem/internal/adapter/runtime_test.go as the page names).
- metasystem/testing.json, metasystem/scripts/agents/supervision-fixtures.sh
  and metasystem/docs/orchestration.md: the bed legs wait-job-run,
  wait-proof, wait-ledger, wait-restart and wait-bounds are wired as
  scenarios of the existing supervision fixture bed that launch Go tests
  (no shell assertions), registered in testing.json under the groups the
  file already uses for that bed; docs/orchestration.md gains one
  paragraph naming the wait verb and the adapter's wait-delivery
  operation as part of the adapter contract.

# The four decisions

1. Adapter operation wire format. The executable operation is
   `wait-delivery --wait-id ID --nonce NONCE --deadline RFC3339-UTC
   --session SESSION-ID`, added to the operation table of
   metasystem/scripts/agents/adapters/runtime-common.sh and implemented in
   claude.sh, codex.sh, devin.sh and fake.sh. It prints exactly one line,
   `blocking`, and exits 0; exit 2 declines (no output required); any
   other exit is an adapter failure the registration reports as 65. The
   Go port calls it through the same adapter execution path the other
   operations use and records the one-word answer in the row.
2. The question cursor is `ledgerCursor` in the question record, as
   above; `channel wait` and `metasystem wait --goal G --event human-act
   --verb answer --question ID` read it; a record without the field is
   refused with a named reason unless `--after` is given.
3. The answer act names its question, as above; the ledger predicate
   matches an accepted answer History row on the goal whose `question`
   equals the selector's; older rows without the field never match.
4. The session-start WAITING lines: `session start` (the engine verb the
   hook's `start` event calls) prints them; `goal next` and
   `report turn-verdict` print the same lines through the report package.
   The lines are `WAITING <kind> <target> until <deadline>: metasystem
   wait --resume WAIT-ID`.

Everything else the build brief and the design page decide stands. Do not
touch metasystem/internal/goal/txn.go (the publication owner), the
launcher's terminal commit, the job transition writer, the stop gate's
decisions in metasystem/internal/goal/turnverdict.go, or the watch
wrappers. Do not commit.

# Constraints, Expected Return, Acceptance Criteria, Gap Rule

As in the build brief. Wall clock: 90 minutes for this round; a partial
round returns with its tests green for what exists and names what is
left.
