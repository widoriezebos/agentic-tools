Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal stop-decisions-record-deadline-evidence)
Date: 2026-09-13

# Goal

Fold the first rostered read (job member2-design-read-1, findings SDE-01
to SDE-10) into the landed design page
metasystem/plans/stop-decisions-record-deadline-evidence-design.md
(commit fc86daa97; your worktree is at that tip and the page is a tracked
file there). The seat has decided every design question the read
raised; write the decisions below into the page in plain English, keep
every section, at most 240 lines. Where a decision removes something the
page says today, remove it; never leave both versions.

# Workspace

Your job worktree, branch agent/<job>. Modify exactly one file, the
design page above. Touch nothing else. Do not commit.

# Inputs: the seat's decisions, one per finding

SDE-01 (critical), the lost-refusal race. The worker spends (seen
markers, idle count, verdict state) ONLY inside the decision record's
lock, and the parent never gives up silently. Sequence for the worker's
verdict verb: compute the decision without spending; take the record
lock; if the episode is already completed by the deadline, replay it and
spend nothing; else check the parent's `deadline-emitted` marker (below):
if present, replay as the deadline's allowance and spend nothing; else
spend, save the verdict state (its own lock, bounded by two seconds, taken
inside the record lock: the lock order stays record first, verdict state
second), complete the episode and append the decision line, then release.
The parent on expiry waits for the record lock up to 2.5 of its 3
reserved seconds (longer than the worker's maximum hold); if it gets the
lock it completes as today; if it does not, it writes the marker file
`deadline-emitted` beside the episode file, logs `completion pending`,
and emits the allowance. A worker that reaches its spend after that finds
the marker and does not spend. So a refusal the seat never received is
never recorded as seen. Test: TestStopDeadlineCompletionIsSingleUse
gains the marker race.

SDE-02 (high), the episode identity. One key:
`<turn generation>/<attempt sequence>/<deadline end>`, the generation and
attempt sequence of `steward hook-attempt` and the deadline end as UTC
epoch seconds; the attempt sequence makes two retries in one second
distinct. The uncertain form, when `hook-attempt` failed, is
`0/0/<deadline end>/<parent pid>-<parent start>`; it never borrows a
neighbour. Checkout, holder session, lease epoch and enrollment
generation are FIELDS of the episode (the record file is already per
checkout and session), and the condition fingerprint belongs to each
incident, not to the episode. Say so in one paragraph and drop the
umbrella's longer key list from this page.

SDE-03 (high), the episode file. The parent passes its directory to the
worker as `METASYSTEM_STOP_DEADLINE_DIR` beside
`METASYSTEM_STOP_DEADLINE_STARTED`. The worker writes the episode file
(the identity, one line) as soon as `hook-attempt` returns, BEFORE it
calls prepare. The parent on expiry: no file means the worker died before
it had an identity: the parent prepares and completes the uncertain
episode; a file whose episode is not prepared means the worker died
between the file and prepare: the parent prepares and completes THAT
identity; a file whose episode is pending: the parent completes it. One
episode per stop in every case.

SDE-04 (high), durable publication. The record writer returns three
outcomes: written and durable, written with durability unknown, or
failed. The decision line's `recorded` field takes `true`, `unproven` or
`false`, and a `recordError` field carries the write error when false.
The hook-log append of the decision line happens inside the record lock
by whoever completes the episode, so appends never interleave; a failed
append is said in the notice as today.

SDE-05 (high), incidents from the parent. `complete --by deadline`
takes `--cause` and `--component` and records an incident on the episode
with them (stop-deadline-expired / stop-deadline, or
stop-hook-output-was-unreadable / stop-worker) in the same write; the
steward's drain (member 3) therefore finds them in the record.

SDE-06 (high), the verb interfaces. Write them out:
`report stop-decision prepare --record PATH --session S --generation G
--attempt A --deadline-end E --lease-epoch L --enrollment-generation N
[--uncertain]`; `report stop-decision complete --record PATH --episode
ID --by worker|deadline --class C --outcome block|allow --reason TEXT
--command TEXT --count-state spent|not-spent|none [--cause CODE
--component NAME] --arming FILE`, returning one JSON object
`{episode, decision, alreadyCompletedBy, recorded, recordError}`, with
the provider response composed by the hook from `decision` as today;
`report stop-decision incident --record PATH --episode ID --cause CODE
--component NAME [--kind KIND] [--identity ID]`, the fingerprint being
cause code, component and the optional kind and identity the caller has.
The arming file is `up`'s raw output; complete keeps its component lines
and aggregate as text, exactly as printed, and parses nothing else. The
reason and command fields are the verdict's display and, once member 5
lands clearing commands, its command; until then command is empty.

SDE-07 (medium), one decision line. The completer appends the decision
line inside the record lock, once; the parent relaying a worker's
committed decision appends a `stop-relay` line (episode, relayed by
deadline, the committed outcome), never a second decision line. Every
line is one JSON object with fixed keys, including `recorded` and
`recordError`.

SDE-08 (medium), retention. Prepare prunes, under the lock, episodes
older than thirty days (the verdict state's horizon) and delivered
episodes older than seven days; version-1 causes are kept thirty days
from the record's upgrade and then dropped. The seven-day count of
member 8 reads the hook log's lines, which are never pruned, so retention
never touches its evidence.

SDE-09 and SDE-10 (not material). Cite the parent's range as lines 38 to
397 with the two emission sites named; rewrite the two passages in plain
words (say what the lost refusal was in one sentence; name the members
by what they do).

# Constraints

- Plain English, short sentences, no bullet padding. At most 240 lines.
- Every path you cite must exist in the worktree under the metasystem/
  prefix. No globs. Do not write code. Do not run bin/metasystem test run,
  test plan or test verify.
- Wall clock: 30 minutes. Stop and report if the fold is not finished by
  then.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json):
jobId, round, runtime, sessionId, model, evidence, gaps, mode, riskiestPart,
diffBoundary, whatWasDone. evidence carries exactly two `{command, observed,
level}` items, replayable verbatim from the worktree's repository root: (1)
`git -C metasystem status --short` observing the one modified file; (2)
`( cd metasystem/plans && wc -l stop-decisions-record-deadline-evidence-design.md )`
observing the line count. whatWasDone lists the ten findings and where
each was folded.

Every path in your return (diffBoundary, files) is relative to the
repository root, so it starts with `metasystem/`.

# Acceptance Criteria

- The page carries the spend-under-the-lock rule with the marker, the
  one episode key and its uncertain form, the episode file before
  prepare, the three-valued `recorded`, the parent's incidents, the
  three verb interfaces, the single decision line with the relay line,
  and the retention rule.
- Every path it cites exists.

# Gap Rule

stop and report a gap; never fill it silently.
