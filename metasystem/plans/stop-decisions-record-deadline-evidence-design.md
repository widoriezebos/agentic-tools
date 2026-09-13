# stop-decisions-record-deadline-evidence: design

This member of stop-hook-never-forces-an-empty-turn gives each stop one
episode and one completion. Its writer attempts durable publication before
emission and reports failed or unproven writes. The m1b seat wrote it on
2026-09-13. See `metasystem/plans/stop-hook-never-forces-an-empty-turn-design.md`,
sections 3 and 7.

## 1. What the record does not hold today

A Stop has two processes. The deadline parent
(`metasystem/scripts/agents/supervision-hook.sh`, lines 38 to 397) stages
the payload and starts the worker with `METASYSTEM_STOP_DEADLINE_STARTED`.
Its two allowance emission sites are the unreadable-worker-output branch
and the expired-deadline branch. The latter starts after fifty-seven
seconds, leaving three seconds of the sixty-second budget for the parent.
The worker gets the turn generation and attempt sequence from
`steward hook-attempt` (`metasystem/internal/steward/component_evidence.go`,
`BeginHookAttempt`). An emitted turn advances the generation; a failed
turn's retry advances the attempt sequence within that generation.
The worker arms supervision with `up`, reads health and the narrator digest,
and calls `report turn-verdict`. That verb spends seen markers and the
idle count and saves verdict state before answering
(`metasystem/internal/goal/turnverdict.go`, `saveVerdictState`).

The decision has no record naming its generation, deadline end, class and
arming result. Verdict state holds spent markers. The per-session verdict
file is replaced at each verdict and serves only as display.
The version-1 stop-refusal record (`metasystem/internal/report/stopblock.go`)
counts causes by text per session, without a generation or deadline.
A late worker can mark a refusal as seen after the parent has already
allowed the stop, so the seat never receives that refusal and the next
stop wrongly allows the same work.
Repeated observations inside one deadline and observations on later stops
also share a rolling count, instead of belonging to separate stop episodes.

## 2. The mechanism

The stop-refusal record becomes version 2 at its existing canonical
checkout state-root location. The record file is already per checkout and
session. Its `episodes` have one key:
`<turn generation>/<attempt sequence>/<deadline end>`. Generation and
attempt sequence come from `steward hook-attempt`; deadline end is the
parent's start plus sixty seconds, in UTC epoch seconds. The attempt
sequence distinguishes two retries in one second. When `hook-attempt`
fails, the uncertain key is `0/0/<deadline end>/<parent pid>-<parent start>`;
it never borrows a neighbour's episode. Checkout, holder session, lease
epoch and enrollment generation are episode fields, alongside `preparedAt`.
The condition fingerprint belongs to each incident, not to the episode.

An episode's `decision` starts as `pending`. Completion sets `class`
(infrastructure, seat-actionable or idle-with-backlog), `outcome` (block or
allow), `reason`, `command`, `countState` (spent, not-spent or none),
`completedBy` (worker or deadline) and `completedAt`.
Its `arming` has `complete` false until the worker attaches `up`'s result.
The arming file is `up`'s raw output. Completion keeps the component lines
and aggregate as text, exactly as printed, and parses nothing else.
The renderer in `metasystem/internal/up/up.go` owns that text. Incomplete
arming leaves `complete` false and an aggregate saying arming was incomplete.
Each incident has its fingerprint, `count`, `firstAt`, `lastAt` and
`delivery`. Delivery stays undelivered until the member that sends stop
incidents to the steward acknowledges it.

The record verbs live in `metasystem/internal/report/stopblock.go` and are
exposed by `metasystem/cmd/metasystem/report.go`. All writes take the
record's bounded lock (`stopRefusalLockWait`). Their interfaces are:

`report stop-decision prepare --record PATH --session S --generation G --attempt A --deadline-end E --lease-epoch L --enrollment-generation N [--uncertain]`

Prepare writes a pending episode. Repeating the same identity does not
reset it. Under the lock, prepare prunes episodes older than thirty days,
the verdict state's horizon, and delivered episodes older than seven days.
Version-1 causes stay as history for thirty days from the record's upgrade
and are then dropped. They never prove that a stop was processed.
The member that measures daily refusals over seven days reads the hook log,
whose lines are never pruned. Retention cannot remove its counting evidence.

`report stop-decision complete --record PATH --episode ID --by worker|deadline --class C --outcome block|allow --reason TEXT --command TEXT --count-state spent|not-spent|none [--cause CODE --component NAME] --arming FILE`

Complete changes a pending decision once. It returns one JSON object:
`{episode, decision, alreadyCompletedBy, recorded, recordError}`.
A repeated completion returns the committed decision and its completer
in `alreadyCompletedBy`, without changing that decision. The hook composes
the provider response from `decision` as today. `reason` is the verdict's
display. `command` is empty until the member that adds executable clearing
commands lands; it then holds the verdict's clearing command.
The writer returns three outcomes: written and durable, written with
durability unknown, or failed. They set `recorded` to `true`, `unproven`
or `false`, respectively. `recordError` carries the write error when false.
An unproven write is described as such; a failed write says `unrecorded`.

`report stop-decision incident --record PATH --episode ID --cause CODE --component NAME [--kind KIND] [--identity ID]`

Incident adds or counts a condition in that episode. Its fingerprint is
the cause code, component, and the optional kind and identity the caller
has; it excludes elapsed time and rendered wording.
`report stop-block --class infrastructure` gains `--episode <identity>`
and records through this path. Its notice says, for example, `occurrence
2 in this stop`, instead of reporting a rolling per-session number.

The parent passes its directory as `METASYSTEM_STOP_DEADLINE_DIR` beside
`METASYSTEM_STOP_DEADLINE_STARTED`. As soon as `hook-attempt` returns, the
worker writes the identity on one line in that directory's `episode` file,
before prepare. It prepares before optional collection through `up`, health,
narrator or watchdog. A failed attempt uses the uncertain identity above.

The verdict verb owns worker completion. `report turn-verdict` gains
`--episode <record path>:<identity>` and `--arming <file>`.
It first computes the decision without spending. It then takes the record
lock and reads the episode. If the deadline already completed it, the
worker replays that allowance and spends nothing. Otherwise it checks
the `deadline-emitted` marker beside the episode file. If present, it
replays the deadline's allowance and spends nothing. A replay says
`decided by the deadline at <end>`.
Only after both checks may it spend seen markers and the idle count,
save verdict state, complete the episode and append the decision line.
All of those actions happen inside the record lock, before release.
Saving verdict state takes its own lock, bounded by two seconds, inside
the record lock. The lock order stays record first, verdict state second.
The parent takes only the record lock. If verdict state cannot be written,
completion still records the observed decision with `countState` not-spent,
as required by the member that classifies infrastructure failures.

On expiry the parent reads the episode file. No file means the worker
died before it had an identity: the parent prepares and completes the
uncertain episode. A file naming an unprepared episode means the worker
died between publication and prepare: the parent prepares and completes
that identity. A file naming a pending episode lets the parent complete
it. Every case keeps one episode per stop.
The parent waits for the record lock for up to 2.5 of its three reserved
seconds, longer than the worker's maximum hold. With the lock, it completes
the pending episode as an infrastructure allowance. If the worker already
committed a decision, the parent relays it; a block stays a block, with
the deadline noted in the system message.
If the parent cannot take the lock, it writes `deadline-emitted` beside
the episode file, logs `completion pending`, and emits the allowance.
It never gives up silently. A worker that reaches its spend after that
finds the marker and spends nothing. A refusal the seat never received
is therefore never recorded as seen.

The parent's `complete --by deadline` takes `--cause` and `--component`.
Expiry uses `stop-deadline-expired` and `stop-deadline`; unreadable output
uses `stop-hook-output-was-unreadable` and `stop-worker`. Completion records
the incident in the same write as the decision. The member that drains
incidents to the steward can find both parent conditions in the record.

Whoever completes the episode appends its one `stop-decision` line to the
hook log inside the record lock, before emission. Appends cannot interleave.
The line is one JSON object with fixed keys: `episode`, `checkout`,
`session`, `leaseEpoch`, `generation`, `attempt`, `deadlineEnd`, `class`,
`outcome`, `reason`, `command`, `countState`, `completedBy`, `aggregate`,
`recorded` and `recordError`. A parent relaying the worker's committed
decision appends a `stop-relay` line, never another decision line.
That line is also one JSON object with fixed keys. It identifies the
episode, that the deadline relayed it, and the committed outcome, and
includes `recorded` and `recordError`. The append's exit is checked;
a failed append is said in the emitted notice. Existing `stop-condition`
lines from the member that classifies infrastructure failures stay as they are.

## 3. What does not change

The member that classifies infrastructure failures and the umbrella own
which stops block or allow, seen-marker limits, and the idle counter's
handoff after three refusals. This member records those decisions and
prevents late workers from spending a refusal the seat did not receive.
`steward hook-attempt` and `hook-complete` still own generation and attempt
sequence. The per-session verdict file stays display.
The steward does not read this record yet. The member that delivers
incidents will drain and acknowledge them under the retention rule above.
The member that enforces the common gate owns the Codex adapter boundary
and `host stop-gate`.

## 4. Risks

The record lock covers spending, verdict-state saving, completion and
the decision append. The worker's bounded hold and the parent's longer
2.5-second wait must leave time to emit within the sixty-second budget.
The marker covers a parent that cannot acquire the lock. A late worker
checks it before any spend, leaving open work eligible to refuse again.
The episode file is the identity channel. Publishing it before prepare
lets the parent recover the same identity after a crash; absence uses
the isolated uncertain identity. Same-second retries remain separate
because their attempt sequences differ.
A successful write with unknown durability stays `unproven`. An append
failure stays visible in the notice even if the record write succeeded.

## 5. Fixtures

`TestInfrastructureDeadlineIdentity` in
`metasystem/internal/report/stopblock_test.go` uses a fake clock. Repeated
conditions in one episode make one incident with count two; concurrent
writers keep one episode; a changed fingerprint makes another incident.
Different generations, attempts in the same second, or deadline ends
make distinct episodes. Uncertain identities borrow nothing. Upgrade
keeps version-1 history for thirty days. Prepare prunes thirty-day episodes
and seven-day delivered episodes without changing the hook log.

`TestStopDecisionPersistsBeforeEmission` is a new command test for
`metasystem/cmd/metasystem/report.go`. Completion writes before answering
and keeps the full raw arming text and generation. A faulted arming file
leaves arming incomplete. A faulted incident write leaves the decision
intact. Durable, durability-unknown and failed record writes return the
three `recorded` values; failure includes `recordError`. Parent completion
records its cause and component incident with the decision.

`TestStopDeadlineCompletionIsSingleUse` in
`metasystem/internal/goal/turnverdict_test.go` covers deadline completion
before the worker: the replay spends no seen marker or idle count, and
the next episode blocks on the work. In reverse order the parent relays
the committed worker block with `alreadyCompletedBy` worker. The marker
race forces the parent's lock wait to expire and publish `deadline-emitted`
before the worker's spend; the worker replays the allowance, spends
nothing, and leaves that work eligible to refuse the next stop.

`TestArmingDetailSurvivesStop` in `metasystem/cmd/metasystem/up_test.go`
extends the test from the member that classifies infrastructure failures.
Arming text, notice and `up` rendering agree byte for byte on component
outcomes, details, remedies and aggregate.

Hook cases in `metasystem/scripts/agents/supervision-hook-fixtures.sh`
cover a missing episode file, a published identity before prepare, and
a pending episode: the parent leaves one episode in each case. A delayed
worker spends nothing after deadline completion or the marker. A worker
block committed before expiry is relayed with one decision line and one
relay line. Repeated narrator conditions across two stops leave separate
incidents and two decision lines. Append failures appear in the notice.
The idle and session-stop tests in
`metasystem/internal/goal/turnverdict_idle_test.go` keep passing unchanged.

## 6. Landing

Implementation changes `metasystem/internal/report/stopblock.go` and
`metasystem/cmd/metasystem/report.go` for the record verbs,
`metasystem/internal/goal/turnverdict.go` for spending and completion under
the record lock, and `metasystem/scripts/agents/supervision-hook.sh` for
the episode file, marker, parent completion and relay. It adds the tests
above. No seat rebuild is needed beyond the landing's own re-arm.
Later members deliver incidents to the steward, join ready work to its owner
and revision, add clearing commands, surface each unchanged fence once,
enforce the gate at both runtime boundaries, and measure seven days of refusals.
