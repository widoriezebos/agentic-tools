# seat-mutual-awareness — design: seats see each other and ask each other (revision 2)

Goal: plans/goals/seat-mutual-awareness.md. Wido's order (2026-08-31,
verbatim on the goal record): seats must be aware of each other and
ask each other questions directly, without the human as relay; DONE
means a seat can discover what other seats have in flight and put a
question to them as the normal, mechanized path. His binding word for
the inbound loop (2026-09-01, on the same record) is not touched here:
anything inbound on the external channel carries a single-use TOTP
code, verified against a secret provisioned once at his agent-free
terminal, and a code authorizes only the message it accompanies.
Author m1d (design delegate, job sma-design1-20260906), 2026-09-06.

Revision 2 answers critique round 1 (job sma-crit1-20260906, ten
material findings SMA-C-01..10 at commit 6948392b, register at
records/misc/seat-mutual-awareness-critique-r1.md); the dispositions
are at the end. Every cite in this revision was re-read at branch
agent/sma-design1-20260906 on main 412ddd95. What changed, in one
breath: no seat-authored words ever reach the digest, the health line,
the status post or the Stop message, only names, ages and the verb
that shows the words; the first writer is fenced behind a human act at
the terminal after every machine has rebuilt, and an upgraded
validator refuses a tip an old engine spoiled; presence publish
failure leaves the health role and goes to the tick's component
record; every seat verb's journal intent carries the whole record so
recovery rebuilds it under the same operation id, with presence
abandoned rather than replayed; a membership record written at session
start separates an unknown machine from an unreachable one; every ask
carries a deadline, an answer after it is recorded as late and binds
nothing, and the asker's wait has one named exit per outcome; the
refusal table is total over states and fields; the load bound, the
growth policy and the legacy transport are stated; the arming lineage
is snapshotted by the runner without a package cycle; and the proof
matrix and the estimate cover the risks the critic named.

The shape in one paragraph. Every machine already shares one git
branch that the goal verbs commit to and push, with a transaction
engine that fetches the tip, rebuilds, pushes by compare-and-swap and
lets the push race decide (internal/goal/txn.go:466-516). Every
steward tick already fetches that branch, validates the new tip and
advances a local "accepted" ref (internal/steward/ledgerattention.go:518-544).
This design adds one directory on that branch, `plans/seats/`, holding
four record kinds: an enable marker the human writes once at the
terminal; a membership record per machine, written at session start;
a presence record per machine, written by the steward tick, saying
what that machine has in flight; and an ask record per seat-to-seat
question, written by the asking seat and answered by the addressed
seat, each under its own machine's operation id. Nothing here posts to
a provider, nothing here reads the TOTP secret, nothing here writes a
goal file, and nothing here puts a seat's words on a surface the human
reads. The seat learns of a question it owes, and of an answer it
waited for, from the health line and the narrator digest the Stop hook
prints at every turn end, and reads the words with one verb.

## 1. What exists and binds (traced)

1. The channel ledger directory `plans/channel/` and its validator are
   landed: record types, the refusal table, the transition matrix and
   the opid helper (internal/goal/channel.go:13-108, 139-215, 775-880,
   979-985); the tree reader and the inbound matcher
   (internal/goal/channel_inbox.go:20-70, 74-121); inbound verification
   and the atomic answer publish (internal/channel/inbox.go:20-73).
   ValidateCommit calls the channel validator on every commit
   (internal/goal/validate.go:485). NOT landed: the posting protocol and
   open-work pass (gateway design step 3b), the verb cut-over (3c) and
   the resident listener with its heartbeat (step 4). `channel ask`
   still writes under the machine's own artifacts directory
   (internal/channel/question.go:78-81, 192), invisible to other
   machines, and `channel wait` loops on that local file
   (cmd/metasystem/channel_verbs.go:179-223).
2. Where the new records go. The channel validator lists
   `plans/channel/` and refuses any path it does not know with
   `channel-unknown-path` (channel.go:140, 168-171), and every record
   decoder refuses unknown keys (channel.go:370-374,
   DisallowUnknownFields). The channel tree reader errors on an unknown
   path too (channel_inbox.go:44-47). So a new record kind or key UNDER
   `plans/channel/` makes every engine that predates it refuse the
   shared tip. `plans/seats/` is a sibling that ReadCommitGoals
   (validate.go:405-407 lists only the goals and records prefixes) and
   ValidateChannelTree never list, so an old engine's READ path ignores
   it. Its WRITE path does not (SMA-C-02): to an old engine the
   directory resolves to the generic plan-record class
   (scripts/agents/path-classes.txt:30 `install:plans/ record`, longest
   prefix wins), the pre-commit guard's regexp covers only
   `plans/(goals|channel)/`, and a landing may create a previously
   absent plan record (internal/landing/observe.go:687-693: an existing
   record is frozen, a new one passes). So an old checkout's landing
   can create a file under `plans/seats/` that its own validator never
   reads and an upgraded validator refuses. Section 8 gives the fleet
   order and the fence that closes this.
3. The human answer object cannot honestly carry a seat's answer: its
   `step`, `userId` and provider `ref` are required and non-null
   (channel.go:439-455), and phase `approved` demands a receipt
   (channel.go:543-546). A seat answer in that shape would have to fake
   a verified TOTP step. So seat asks get their own record and never
   touch the question schema (section 4).
4. An operation id is `<26-char ULID>-<machine>-<8 hex of the lineage
   hash>` (internal/goal/file.go:1494-1497). Recovery rebuilds a dead
   owner's request from the journal entry alone: it takes the ULID
   from the opid, the machine and lineage from the entry, and refuses
   to proceed when the rebuilt request's opid differs
   (internal/goal/recover.go:209-229); the Intent must therefore be
   "the COMPLETE NORMALIZED COMMAND INTENT ... enough to rebuild the
   operation without the original process" (internal/goal/journal.go:57-66).
   A verb without a rebuild case is terminalised rejected with "re-runs
   from its own entry point" (recover.go:403, 185-194). Section 7 gives
   every seat verb its case.
5. Health: the tick evaluates a fixed role list in a fixed order and
   prints every role with its reason on one line
   (internal/steward/health.go:44-80, 176-193, 272-300); a role is
   alive, dead or unknown (health.go:35-39). The breaker alerts on two
   consecutive unknown observations of one role (health.go:544-549) and
   at once on a dead role with no lawful automatic remedy
   (health.go:556-568); an unhealthy aggregate opens an alert episode
   that reaches the human (internal/steward/alert_episode.go:241-281).
   The default tick is ten minutes (internal/steward/runner.go:52-60,
   TickSeconds). The Stop hook renders health with PreviewHealthAt,
   which evaluates the roles without advancing the breaker
   (health.go:242-270). The narrator digest appends entries
   deduplicated by source type and id (internal/narratordigest/digest.go:105-129);
   the tick writes them in NarrateDigest (internal/steward/narrate.go:47-95);
   the Stop hook reads the pending digest through `steward
   digest-pending` and carries it into the message the seat's runtime
   shows (scripts/agents/supervision-hook.sh:534-548, 605-607), and
   docs/seat-communication.md:1-9 names digest lines and stop messages
   as channels that reach the human (SMA-C-01).
6. The tick already reads this machine's running delegate jobs and its
   landing receipts for the delivery role
   (internal/steward/delivery.go:85-88); the job record carries
   `role`, `round`, `parentJob`, `goalId`, `startedAt`, `status`
   (internal/dispatch/record.go:60-77). The engine digest is
   steward.installDigest (internal/steward/identity.go:96). The runner
   record (internal/steward/runner.go:40-46, RunnerRecord) carries pid,
   start identity and StartedAt and no lineage. The lease carries
   OwnerLineage (internal/lease/lease.go:29-44, loaded by the private
   loadLease at 76-102), its public CurrentHolder view omits it
   (internal/lease/verbs.go, CurrentHolderView), and the lease package
   imports steward (internal/lease/verbs.go:16), so steward cannot
   import lease (SMA-C-09). Both cmd/metasystem and internal/up import
   lease and steward (internal/up/up.go and
   cmd/metasystem/dispatch_verbs.go among the importers).
7. The ledger-attention pass projects every intervening ledger commit
   between the last diffed tip and the new one
   (ledgerattention.go:226-268: LedgerChanges, then ProjectAt per
   change) and emits an event only when the claimable, pinned or queue
   sets changed (the `if` at the end of that loop). Publish pushes the
   configured remote only (txn.go, `push e.Remote --force-with-lease`);
   the legacy transport mirror is a separate script run by landings
   (scripts/agents/sync-transport.sh:31-35 fetches origin's branch and
   pushes it to `transport`), and its future is the open goal
   transport-sync-law (plans/goals/transport-sync-law.md:3-6:
   "every machine's origin is GitHub", the bare relay is legacy).
8. The status post is composed from open channel questions and the
   goal projection (internal/channel/report.go:36-112). Conduct for
   anything a human reads is docs/seat-communication.md; rule 2 (every
   choice carries its consequence, silence has a stated cost) is
   applied between seats here. The core never names a runtime
   (AGENTS.md); every decision below is Go, the verbs are thin.
9. Coverage floors: internal/goal 80.0 and internal/steward 74.0
   (scripts/agents/coverage-ratchet.json), enforced by
   scripts/agents/coverage-delta.sh. New code lands in those two
   packages and in cmd/metasystem so it stands under a floor.

## 2. The directory, the enable marker and membership

Location: `plans/seats/` on the ledger branch (goal.sync-branch,
txn.go:49-61), four sub-paths, canonical serialisation MarshalChannel
(channel.go:111-117), keys in table order, no unknown keys, every time
RFC 3339 UTC at second precision (parseChannelTime, channel.go:650-656,
reused).

Enable marker: `plans/seats/enabled.json`, written once by the human.

| key | type | required | meaning |
|---|---|---|---|
| enabledAt | time | yes | |
| by | string | yes | `human:<name>`, the actor of the enrolled terminal |
| opid | string | yes | the opid of the commit that wrote it |

`metasystem seat enable --repo <root>` runs only from the agent-free
terminal (the ancestry proof `steward arm` uses,
cmd/metasystem/steward_verbs.go, the enrollment path the fleet-join
design cites at its step 5) and refuses everywhere else. EVERY seat
writer below (membership, presence, ask, answer, close) refuses
`seat-not-enabled` while the marker is absent at the tip it rebuilds
on. This is the fail-closed deployment fence of SMA-C-02: no machine
writes a seat record until the human has said the fleet is ready, and
the human says it only after every machine has rebuilt (section 8).

Membership record: `plans/seats/members/<machine>.json`, `<machine>`
the value goal.ResolveMachine returns (internal/goal/actor.go:21).

| key | type | required | meaning |
|---|---|---|---|
| machine | string | yes | equals the basename |
| engine | string | yes | the engine digest of the writer |
| seatSchema | integer | yes | exactly 1 in this design |
| joinedAt | time | yes | the first write; preserved on overwrite |
| updatedAt | time | yes | |
| retiredAt | time or null | yes | set only by `seat retire` |
| opid | string | yes | opid of the writing commit; its machine segment equals `machine` |

Writer: `metasystem up` (internal/up/up.go), at every session start,
after its enrollment and lease steps succeed, under the session's own
opid (machine, session lineage), Intent verb `seat-member`, Targets the
path, Args {machine, engine, seatSchema, joinedAt}. Its Mutate writes
when the path is absent or the tip's engine differs or the tip's
updatedAt is older than 24 hours; else NothingToDo (txn.go:462). It
refuses `seat-transition` when the tip's updatedAt is younger than the
one it read. A membership publish failure is printed as one `up`
component outcome (`component=seat-member outcome=<detail>`) and never
changes `up`'s aggregate: a seat that cannot announce itself still
works. `metasystem seat retire --machine <m> --repo <root>` is the
human's act at the terminal, sets retiredAt, never deletes. Nothing
under `plans/seats/` is deleted by any Mutate.

Membership answers SMA-C-05 with one durable source. The three
standings a reader derives, by name:

- `unknown`: no membership record at the tip. The machine has never
  run `up` on an engine with this goal, or does not exist; the ledger
  cannot tell these apart and says so.
- `unreachable`: a membership record, not retired, and no presence
  record younger than the staleness window (section 3).
- `reachable`: a membership record and a fresh presence record.
- `retired`: retiredAt set.

The fleet-join design (plans/fleet-join-bootstrap-design.md, step 6)
already makes `up` the last step of joining, so a joining machine
becomes a member the moment it joins, and no second enrollment is
invented.

## 3. Discovery: the presence record and `seat fleet`

Decision: a durable per-machine record, because goal claims cannot see
a chain, a landing or an engine, and deriving from claims alone would
leave "what is m2 doing right now" where it is today. The record does
NOT carry the claimed goals: claims are on the same tip in every goal
file's `Claimed:` line (delivery.go:63 reads them) and a copy written
at tick time would only ever be equal or stale. The read verb joins
claims from the goal files at the tip it reads.

Record: `plans/seats/presence/<machine>.json`.

| key | type | required | meaning |
|---|---|---|---|
| machine | string | yes | equals the basename |
| armedLineage | string | yes | the lineage of the session that armed this checkout's steward runner, snapshotted at arm (section 3, "lineage"); "" when the runner record predates the field |
| engine | string | yes | the engine digest the tick runs |
| chain | object or null | yes | the newest non-terminal delegate job: `{root, job, role, round, goal, startedAt}`, all strings non-empty except `goal` which may be "" for a goal-free root, `round` an integer at or above 1; `root` is reached by following `parentJob` until absent; null when no job is running |
| lastLanding | object or null | yes | `{commit, goal, at}`, strings non-empty, from the newest landing receipt in memory/receipts.log (delivery.go:85 reads it); null when none |
| tickAt | time | yes | the tick that composed this version |
| updatedAt | time | yes | equals tickAt at write |
| opid | string | yes | opid of the writing commit; its machine segment equals `machine` |

Writer and cadence. The steward tick composes the record after its
ledger-attention pass (internal/steward/tick.go:178-186, which has just
fetched and validated the tip) and publishes it through goal.Publish
under the runner's opid (fresh ULID, this machine, armedLineage),
Intent verb `seat-presence`, Targets the record path, Args {machine,
engine, body} where `body` is the whole composed record as one JSON
string (SMA-C-04), message `seat presence <machine>`, Validate
ValidateCommit. Its Mutate reads the machine's own record at the
rebuilt tip and writes when the path is absent, when the body digest
(every key but tickAt, updatedAt, opid) differs from the tip's, or when
the tip's updatedAt is older than `seat.presence-min` (default 60
minutes); else NothingToDo. It refuses `seat-transition` when the tip's
updatedAt is younger than the one it read, and `seat-not-enabled` when
the marker is absent.

Where a presence failure goes (SMA-C-03): the publish is recorded as a
tick component attempt named `seat-presence` (beginComponentAttempt,
tick.go:126, completeComponentAttempt with ComponentError and the
Publish detail on failure). A failure never degrades the tick, never
enters the seat-questions health role (section 6) and never alerts.
It is visible in two places: `seat fleet` prints, for this machine,
`presence not published since <last confirmed>: <detail>` when the
newest `seat-presence` component attempt on this checkout is an error;
and the tick's best-effort narration line (narrate.go:135-189) gains
the note `presence not published: <detail>`. A pushed-blocking refusal
(txn.go:512-516) is the detail `journal blocked by <opid>; run goal
recover`; recovery classifies the blocking entry as it does every goal
verb's (section 7).

Staleness. A record whose updatedAt is older than twice
`seat.presence-min` (120 minutes by default) makes its machine
`unreachable` (section 2); the verb prints `silent since <updatedAt>`.
A member with no presence record at all is `unreachable` with `no
presence yet`.

Lineage (SMA-C-09). The owner of the read seam is the steward runner
record: RunnerRecord (runner.go:40-46) gains `armedLineage string`,
written by steward.Arm from a new parameter. The two callers that arm,
`steward arm` (cmd/metasystem/steward_verbs.go) and `metasystem up`
(internal/up/up.go), already import both lease and steward, so they
read the lease's OwnerLineage through a new public
`lease.OwnerLineage(root) (string, error)` (a one-line wrapper over
loadLease, lease.go:76-102) and pass it in; steward imports nothing
new and the cycle the critic found (lease imports steward,
lease/verbs.go:16) is never entered. Presence reports the ARMING
lineage, not the current lease holder's, because the record describes
the runner process that writes it, that process was started by the
arming session, and a later lease succession that does not re-arm
leaves the runner exactly as it was; the session that succeeded is
visible on the ledger through its own claims' lineage. A runner record
written before the field exists yields "" and the verb prints `armed
by session unknown`.

The read verb: `metasystem seat fleet [--root <checkout>] [--no-fetch]`.
It captures the tip with the bounded fetch the projection uses
(internal/goal/project.go:44, 117-123; `--no-fetch` reads the accepted
ref), reads the seat tree and the goal tree at that tip, and prints one
block per MEMBER (every membership record, retired ones last), sorted
by name, plus one line `unknown to the ledger: <names>` for any machine
named in a claim or an ask that has no membership record:

```
m2   reachable (presence 4 min ago, engine 9a1c0f2e), armed by session main-1788683763-71870-f7f607
     claimed: channel-poll-refuses-legacy-budget-questions — next: <first sentence of the goal's next step>
     chain: implementer round 2 under root fcg-build3 for fleet-channel-gateway, started 41 min ago
     last landing: 8c86a874 for seat-mutual-awareness, 2 h ago
     owes 1 question (from m1d about seat-mutual-awareness, 12 min, deadline in 3 h 48 min); waiting on 0
m3   unreachable, silent since 2026-09-06T08:10:00Z (engine 7b2d11aa)
     claimed: none
paper unreachable, no presence yet (joined 2026-09-07T09:00:00Z, engine 7b2d11aa)
m1b  retired 2026-09-05T10:00:00Z
```

"owes" counts open asks addressed to the machine whose deadline has
not passed, "waiting on" open asks it sent. Question ids and goal names
appear; question text never does (section 6). This verb is what a seat
runs before it asks; it is read-only and needs no lineage.

## 4. Asking a seat: the ask record and `seat ask`

A seat-to-seat question is its own record kind, not a channel question
with a machine destination (section 1, items 2 and 3). Record:
`plans/seats/asks/<qid>.json`, `<qid>` a fresh 26-character ULID from
goal.NewOperationULID (question.go:184 mints one the same way).

| key | type | required | meaning |
|---|---|---|---|
| id | string | yes | equals the basename |
| from | string | yes | the asking machine; equals the opid's machine segment; a member at the tip |
| fromLineage | string | yes | the asking lineage, non-empty |
| to | string | yes | the addressed machine; never equals `from`; a member at the tip |
| goal | string | yes | the goal the question concerns; a goal file must exist on the tip (channelGoalIDs, channel.go:221-241, reused) |
| facts | []string | yes, one to four non-empty entries | what the asker knows |
| question | string | yes, non-empty | the question in plain words |
| ifSilent | string | yes, non-empty | what the asker will do at the deadline if nobody answers (seat-communication rule 2) |
| askedAt | time | yes | |
| deadlineAt | time | yes | later than askedAt; the one durable boundary (SMA-C-06) |
| opid | string | yes | the opid of the ask commit |
| state | string | yes | open, answered, expired, closed |
| answer | object or null | yes | `{text, by, lineage, at, opid}`, text non-empty, `by` equals `to`, `at` not later than deadlineAt, the answer opid's machine segment equals `by`; non-null exactly when state is answered |
| late | object or null | yes | the same shape with `at` later than deadlineAt; non-null only when state is expired and a late answer was recorded |
| expiredAt | time, expired only | | when the record was first written past its deadline |
| closedAt, closedBy, closedBecause, closedReason | strings, closed only | | closedBecause is `not-mine` (closedBy equals `to`) or `withdrawn` (closedBy equals `from`); closedReason non-empty |

There is no `kind`, `wants`, `budget`, `options`, `destination`,
`thread`, `step`, `userId` or `ref` key, and an unknown key is refused
(`seat-json`). A seat cannot express a stop, a budget or any
token-bearing question to another seat, because there is nothing a
seat could approve. The ask touches no goal file (goal.Asked,
verbs.go:43-65, is for human questions only), so no other machine's
claimed goal is edited from outside.

Verb: `metasystem seat ask --to <machine> --goal <id> --fact <text>
[--fact ...] --question <text> --if-silent <text> [--deadline
<minutes>] [--root <checkout>]`, printing the qid. `--deadline`
defaults to `seat.ask-deadline-min` (default 240, four hours);
deadlineAt is askedAt plus that. Identity is channelIdentity
(channel_verbs.go:21-31). One ledger transaction under the asker's own
opid: Intent verb `seat-ask`, Targets the record path, Args {id, to,
goal, facts (JSON array as one string), question, ifSilent, askedAt,
deadlineAt}, so recovery rebuilds the whole record (section 7);
message `seat ask <to> about <goal>`; Validate ValidateCommit. Mutate
refuses, by name and before any write: `seat-not-enabled`;
`seat-self` when `--to` is this machine; `seat-unknown-machine` when
`--to` has no membership record ("seat fleet lists the machines the
ledger knows; a machine joins by running metasystem up on this
engine"); `seat-retired`; `seat-goal-missing`; `seat-duplicate` when an
open ask from this machine to that machine on that goal with the same
facts digest is at the tip (the dedup rule of Ask, question.go:174-183,
printing the existing qid). An `unreachable` target is NOT a refusal:
the verb prints `warning: <to> is unreachable (silent since <t>); the
deadline is the only outcome you can count on` and proceeds, because
the machine may return before the deadline and the deadline already
bounds the wait. A path already present is the ordinary AlreadyApplied
or LostToCompetitor classification (channel.go:905-927 is the model).
No provider is posted; a machine without a bot token asks exactly as
one with.

## 5. Answering: `seat answer`, `seat close`, `seat wait`, `seat show`

`metasystem seat answer --question <qid> --text <text>` or
`--not-mine --because <text>`. One transaction under the answering
machine's own opid, Intent verb `seat-answer`, Targets the record
path, Args {qid, outcome (answer or not-mine), text, at}. `metasystem
seat close --question <qid> --because <text>` by the asker, Intent
verb `seat-close`, Args {qid, reason, at}. Mutate at every rebuilt tip,
`now` being the transaction's own clock and deadlineAt the record's:

| transition (owner) | FROM | TO |
|---|---|---|
| answer (the machine named in `to`) | state open, now not later than deadlineAt | state answered; answer {text, by: me, lineage: mine, at: now, opid: this commit} |
| late answer (the machine named in `to`) | state open, now later than deadlineAt; or state expired with late null | state expired; expiredAt (set if absent); late {text, by: me, lineage, at: now, opid: this commit}; the verb prints `late: recorded after the deadline <deadlineAt>; it binds nothing` and exits 3 |
| not-mine (the machine named in `to`) | state open (any time) | state closed; closedBecause not-mine; closedBy me; closedReason the text |
| withdraw (`seat close`, the machine named in `from`) | state open (any time) | state closed; closedBecause withdrawn; closedReason the text |
| expire (any reader that writes: `seat wait` of the asker, see below) | state open, now later than deadlineAt | state expired; expiredAt now; late null |

Refusals by name: `seat-not-enabled`; `seat-not-addressee` when `to`
is not this machine (answer, late answer, not-mine); `seat-not-asker`
when `from` is not this machine (close); `seat-already-answered` when
state is answered (a second answer from the same machine, or from the
same machine after a crash whose first answer landed: the first
stands, an amendment is a new ask in the other direction; exit 3
printing the standing answer's time); `seat-already-late` when state
is expired with late non-null (exit 3); `seat-closed` when state is
closed (exit 3 printing closedBecause and closedReason);
`seat-not-a-seat-question` when the id names no ask record; when the
id names a channel question at the tip (plans/channel/questions/<id>.json)
the message says "<id> is a question to the human; seats do not answer
those". AlreadyApplied and LostToCompetitor are classified as in
ClassifyChannelTransition (channel.go:905-927) over the tuple
`(state, answer.opid or null, late.opid or null)`.

Exactly one outcome per ask for the asker (SMA-C-06): answered before
the deadline; closed not-mine; closed withdrawn by itself; or expired
at deadlineAt, whether or not a late answer is ever recorded. A target
that never runs a steward, never ticks, or sits on an old engine
produces `expired` at deadlineAt and nothing else; the asker's
ifSilent is its own plan for that case and the deadline is when it
executes it.

The one sentence the validator enforces: a seat answer carries no
human authority, because it is written only into `plans/seats/` and
into no goal file, and every authority consumer reads goal history
(RecordedNormApproval for `goal claim --approved-ref`;
AuthenticatedChannelApproval, verbs.go:67-93, for resume and
set-obligation), where a seat opid never appears. `goal approve`,
`set-budget`, `resume` and enrollment stay at the enrolled terminal or
behind a verified channel answer, untouched. Authentication of a seat
answer is the committing machine's identity in the opid, exactly as a
goal claim is authenticated today. The residual, plainly: any process
that can push to the ledger branch as machine X can answer as X and
can forge an opid naming X; the ledger already trusts that for claims,
and nothing here widens what such a process could do, since a seat
answer authorizes nothing.

`metasystem seat show --question <qid> [--root]` prints the record
(facts, question, ifSilent, the answer or the late answer or the close
reason) to the caller's stdout. It is the ONE reader of seat-authored
words (section 6) and is what the digest line names.

`metasystem seat wait --question <qid> [--timeout <minutes>]
[--interval <seconds>, default 30]` fetches the tip every interval
(the bounded capture of project.go:117-123), reads the ask, and exits:
0 printing `answer.text` when answered; 2 printing `not-mine:
<reason>` or `withdrawn: <reason>` when closed; 4 printing `expired:
no answer by <deadlineAt>; <ifSilent>` when the deadline has passed
(the wait writes the `expire` transition under the asker's opid so
the record says what the asker acted on, and prints the same exit
whether or not that write wins the race); 1 `seat wait timed out`
when `--timeout` is given and shorter than the deadline and passes
first. `--timeout` defaults to the record's deadline. A late answer
recorded after the asker's exit 4 changes nothing the asker acted on:
the record is expired, the late object is information for `seat show`,
and the asker's digest names it as late (section 6). `channel wait
--question <id>` (channel_verbs.go:179-223) gains one lookup: when no
local question file exists and the id names a seat ask at the accepted
ref, it runs `seat wait` with the same flags.

## 6. Surfacing without polling, and no seat words on a human surface

The tick gains one read after its ledger-attention pass: RunSeatAttention
(new, internal/steward/seats.go) reads the seat tree at the accepted
ref (goal.AcceptedLedgerTip, internal/goal/attention.go:32-45) and
returns {owed: open asks addressed to me whose deadline has not
passed, oldest first; resolved: asks from me whose state is answered,
expired-with-late or closed}. TickResult carries it; NarrateDigest
consumes it; the health role reads the same tree.

Health role `seat-questions` (RoleSeatQuestions), placed after
`ledger-attention` in healthRoleOrder (health.go:64-80), evaluated in
evaluateHealthRoles (health.go:272-300) by reading the accepted ref
itself, so PreviewHealthAt from the Stop hook sees the last tick's
view without a fetch:

- alive, reason `owes no seat question`, when no open ask addressed to
  this machine is inside its deadline, or every such ask is younger
  than `seat.answer-min` (default 30 minutes);
- alive, reason `owes <n> seat question(s): <from> asks about <goal in
  words>, <age> min, deadline in <m> min, question <qid>; show it with
  metasystem seat show --question <qid>` (the oldest named, the rest
  counted), while the oldest is between `seat.answer-min` and
  `seat.answer-dead-min` (default 180 minutes);
- dead, NoAutomaticRemedy true (the retro-debt pattern,
  health.go:381-396), the same reason prefixed `UNANSWERED SEAT
  QUESTION`, once the oldest passes `seat.answer-dead-min` and is still
  inside its deadline; a dead role with no lawful remedy alerts at once
  (health.go:566-568), which is the intended fault report to the human
  and the only path by which this role reaches him;
- unknown ONLY when the accepted ref or the seat tree cannot be read,
  the same fault the ledger-attention role raises for the same cause;
  two consecutive unknowns alert (health.go:544-549), which is the
  existing health law and is accepted here because a machine that
  cannot read the ledger is the human's business. Presence publish
  failure never produces unknown (section 3, SMA-C-03).

Why two ages and not one: a dead role alerts the human, which would
make him the relay for every slow seat, the thing the goal exists to
end. The 30-minute key governs what the seat sees at every turn end;
the 180-minute key governs when the fault becomes the human's business
as a fault report, not as a question. Wido may collapse them by
setting both keys equal. Keys are read through internal/config like
the steward's other keys (delivery.go:90 shows the idiom), from
metasystem.conf with metasystem.conf.local overriding.

No seat-authored words on any human-visible surface (SMA-C-01). The
Stop hook carries the pending digest and the health line into the
message the runtime shows, and seat-communication names both as
surfaces that reach the human. So the digest, the health line, the
status post and the stop message carry NAMES only: the asking or
answering machine, the goal in words, the age, the deadline, the
question id and the verb that shows the words. The text fields of a
seat record (facts, question, ifSilent, answer.text, late.text,
closedReason) are read by exactly two consumers: `seat show` (to the
caller's stdout) and `seat wait` (answer.text or the close reason to
the caller's stdout). `seat fleet`, RunSeatAttention, the health role,
NarrateDigest, the narration line, ComposeStatusReport and the Stop
hook never read a text field; the unit test
TestNoSeatTextReachesHumanSurfaces (section 9) builds a tree whose
every text field is a sentinel and asserts the sentinel appears in
none of those outputs. Digest lines (narrate.go:47-95 pattern,
deduplicated by source marker so each fires once):

- new question owed: kind lowlight, source type `seat-ask`, source id
  the qid: `Seat <from> asks this machine about <goal in words>
  (deadline in <m> min). Show it: metasystem seat show --question
  <qid>. Answer it: metasystem seat answer --question <qid> --text
  "...".`
- answer received: kind highlight, source type `seat-answer`, source
  id the qid: `Seat <to> answered this machine's question about <goal
  in words>. Show it: metasystem seat show --question <qid>.`
- late answer, not-mine or withdrawn: kind lowlight, source type
  `seat-answer`, source id the qid: `Seat <by> <answered late | closed
  as not theirs | withdrew> this machine's question about <goal in
  words>. Show it: metasystem seat show --question <qid>.`
- expired unanswered (from the asker's side): kind lowlight, source
  type `seat-answer`, source id the qid: `This machine's question to
  <to> about <goal in words> expired unanswered at <deadlineAt>.`

The status post (report.go:36-112) gains one line, only when the count
is non-zero, after the "Needs you" lines: `Owes <n> question(s) from
other seats, oldest <m> min`. No id and no text reaches the post.

## 7. Recovery: every seat verb rebuilds from its journal entry

Every seat Publish is an ordinary goal-ledger transaction: a journal
entry with the complete Intent before anything is pushed
(journal.go:57-66), the same pushed-blocking rule (txn.go:512-516) and
the same recovery classification (recover.go:58-135). What this design
adds (SMA-C-04):

| verb | Intent.Args (complete) | rebuild case in requestForEntry (recover.go:236) | outcome when the owner died with the commit absent |
|---|---|---|---|
| seat-member | machine, engine, seatSchema, joinedAt | `seatMemberRequest(r, args)` | abandoned with detail `the next up republishes membership` (the slice-start pattern, recover.go:102-108): a stale body is never replayed |
| seat-presence | machine, engine, body | none needed | abandoned with detail `the next tick republishes presence` |
| seat-ask | id, to, goal, facts (JSON array), question, ifSilent, askedAt, deadlineAt | `seatAskRequest(r, args)` | completed from the stored intent under the SAME opid (the ULID from the entry, the pair from the entry, recover.go:213-229); outcome confirmed, or lost when the path already carries another opid (a crash after push), or rejected by name (`seat-not-enabled`, `seat-unknown-machine` if the member was retired meanwhile) |
| seat-answer | qid, outcome, text, at | `seatAnswerRequest(r, args)` | completed under the same opid; confirmed; or lost when the record's answer or late field names another opid; or rejected `seat-closed`, `seat-already-answered`, `seat-already-late`; an answer whose stored `at` is inside the deadline but is rebuilt after it is written as LATE with `late.at` = the rebuild's now, never as answered, because the deadline is the record's boundary and the rebuild is what actually landed |
| seat-close | qid, reason, at | `seatCloseRequest(r, args)` | completed under the same opid; confirmed, lost or rejected `seat-closed` / `seat-already-answered` |

Every constructor is the one the live verb runs (the live verb builds
the same VerbRequest and calls the same function), so the rebuilt
request's opid equals the entry's opid and recover.go:227's check
holds. The commit message and the record body carry the same text the
Intent carries; nothing in seat text is a secret (section 6's
`seat-secret` row refuses codes), so the journal may hold it, unlike
the channel's inbox Intent which deliberately carries none
(FCG-SECRET-15). The recovery outcomes are printed by `goal recover`
exactly as for goal verbs: `completed from the stored intent:
<outcome>`, `abandoned ...`, `confirmed on the canonical tip`.

## 8. The human channel stays what it is; the validator; the fleet order

Stated, then enforced:

1. A seat record is never a channel record. ValidateChannelTree lists
   only `plans/channel/` (channel.go:140) and ReadChannelTree likewise
   (channel_inbox.go:26); `plans/seats/` is never decoded as a
   ChannelQuestion, ChannelInbound or ChannelListener. The inbound
   matcher iterates tree.Questions only (channel_inbox.go:75-77), so no
   inbound record can name a seat ask, and rule (b) needs a posted
   thread (channel_inbox.go:104), which a seat ask has no key for.
2. No provider ever posts a seat record. Today's poster reads local
   channel questions (question.go:195-209); the future open-work pass
   reads `plans/channel/questions/` (FCG-POST-08). Neither has a code
   path into `plans/seats/`, and the seat verbs call no Provider
   method.
3. The provider poll never confirms or matches anything a seat wrote.
   Verify (inbox.go:20-43) is reached only through InboundRecord
   (inbox.go:45) on a provider Inbound; no seat verb constructs an
   Inbound, reads `channel.human.totp-secret`, or calls VerifyTOTP.
   Wido's inbound rule is the gateway design's FCG-COMMIT-05 and is not
   edited here.
4. A seat's words never carry a code and never reach a human surface.
   `seat-secret` refuses ANY field of six ASCII digits in any seat text
   field (channelCodeField, channel.go:671-682, reused); section 6 keeps
   the text off the digest, the health line, the status post and the
   stop message.

ValidateSeatTree (new, internal/goal/seats.go), called from
ValidateCommit next to the channel validator (validate.go:485), is
total (SMA-C-07): every record is one of the four kinds, every kind's
key set is exactly its table, and every state admits exactly one field
combination. It refuses by name; a refusal abandons the publishing
transaction as a goal refusal does:

| code | condition |
|---|---|
| seat-unknown-path | a file under plans/seats/ that is not `enabled.json`, `members/<machine>.json`, `presence/<machine>.json` or `asks/<ulid>.json` (ULID: 26 Crockford characters, validChannelULID, channel.go:270-280) |
| seat-json | not a JSON object; an unknown key; a missing required key; a wrong type; null where non-null is required; a time not RFC 3339 UTC at second precision; `seatSchema` not the integer 1; `chain.round` below 1 |
| seat-id-mismatch | basename does not equal `machine` or `id` |
| seat-writer | `opid`'s machine segment does not equal `machine` (member, presence) or `from` (ask); `answer.opid`'s or `late.opid`'s machine segment does not equal its `by`; `answer.by` or `late.by` does not equal `to`; closedBy is not `to` for not-mine or `from` for withdrawn; `enabled.json`'s `by` does not start with `human:` |
| seat-self | `from` equals `to` |
| seat-unknown-machine | `from` or `to` has no membership record at the tip |
| seat-goal-missing | `goal` has no goal file on the tip |
| seat-state | state outside {open, answered, expired, closed}; open with answer, late, expiredAt or any closed key present; answered with answer null, or with late, expiredAt or any closed key present; expired with expiredAt absent, answer non-null, or any closed key present; closed with answer or late non-null, expiredAt present, or any of closedAt, closedBy, closedBecause, closedReason absent or empty; closedBecause outside {not-mine, withdrawn}; deadlineAt not later than askedAt; answer.at later than deadlineAt; late.at not later than deadlineAt; expiredAt earlier than deadlineAt; facts empty or more than four or any entry empty; question, ifSilent, fromLineage, answer.text or late.text empty; chain or lastLanding present with an empty required string; presence updatedAt earlier than tickAt |
| seat-secret | a six-digit field anywhere in facts, question, ifSilent, answer.text, late.text or closedReason |

A record that violates none of these rows is legal; there is no other
state. Unit test TestValidateSeatTreeIsTotal (section 9) enumerates
the state-by-field grid and asserts each cell is either the one legal
shape or a named refusal.

Fences: `install:plans/seats/ ledger` beside path-classes.txt:35, so a
landing on an upgraded engine that touches it refuses with
`ledger-path-not-goal-verb`; the pre-commit guard's regexp widens to
`plans/(goals|channel|seats)/`.

The fleet order (SMA-C-02), written into the slice landing messages
like FCG-MIGRATE-10's cut-over: (1) slice 1 lands: the validator, the
reader, the request constructors and recovery cases, the two fences,
and NO writer; (2) every enrolled machine pulls and rebuilds under
R-37's standing word for re-arming after an engine rebuild, and the
seat of record confirms each machine's engine through `metasystem
supervise status --repo <checkout>` (engineBuild) or the machine's own
word; (3) Wido runs `seat enable` once at the terminal; (4) writers
start: `up` writes membership at each machine's next session start,
the tick writes presence, seats ask. Between (1) and (3) nothing is
written, so the window in which an old checkout could spoil the
directory is the same window that exists today for any plan record,
and only a landing that deliberately creates a file under
`plans/seats/` from an old checkout can use it. What an upgraded
validator does with such a record: it REFUSES the tip
(`seat-unknown-path` or `seat-json`), exactly as a malformed goal file
is refused (ledgerattention.go:537-543), and the accepted ref stops
advancing on every upgraded machine until a hand commit at the
terminal removes the file. Wido's law of first-commit-wins implies
refusal, not tolerance: the push race decides which commit IS the
ledger, and a validator that tolerated-and-named would give two engines
two readings of one tip, which is the one thing the race exists to
rule out; the committed record stands as the ledger's state and the
ledger says, loudly and identically on every machine, that it is
unreadable. The fixture SMA-F-MIXED proves both halves: an old engine
reads a seat-bearing tip and keeps working, and an old checkout's
spoiled record is refused by name and not silently absorbed.

Paths I can see and close: a seat answer reaching an authority consumer
(closed: no goal history row, section 5); a seat ask being posted or
matched as a human question (closed: items 1 to 3); a code smuggled in
seat text (closed: item 4); seat words on the digest, the health line,
the status post or the stop message (closed: section 6 and its test);
a record forged under another machine's name by a well-formed commit
(closed at the writer, and at rest by `seat-writer` for the opid
field; NOT closed against a process that forges the opid itself, which
is the claims residual of section 5). Paths I cannot close: a seat
pasting `seat show`'s output into a chat turn or a human question's
facts, which then stands as that seat's own words under
docs/seat-communication.md, a conduct matter; a seat acting on a
peer's answer by running a goal verb it was already entitled to run,
which is the seat's own act; and a hand-written malformed seat record
committed without a trailer, which poisons the accepted ref on every
upgraded machine as described above, with the same fallback and no
skip verb.

## 9. Load, growth, transport; fixtures and tests

Load bound (SMA-C-08). Nine members, presence at most once per
`seat.presence-min` (60) when idle plus one write per change of chain
or landing: idle 216 commits per day, change-driven under 200 at the
fleet's observed cadence, accepted bound 400 seat commits per day of
about one kilobyte each. Every other machine's ledger-attention pass
must handle those commits, and today it projects each intervening
commit (ledgerattention.go:243-268). Slice 2 therefore teaches
buildLedgerAttentionStage to skip the projection of a change whose
tree difference is confined to `plans/seats/` (one `git diff-tree
--name-only` per change instead of a ProjectAt); such a change cannot
move Ready, Pinned or Queue, so no event is lost. The bound is TESTED,
not asserted: SMA-F-CHURN stages 400 seat-only commits and asserts one
ledger-attention pass completes inside the fixture cap and emits no
event. Membership writes are one per machine per session start or per
day. Git history grows by the commit objects only; the presence and
membership files are overwritten in place, and the ask directory grows
by one file per ask, under one kilobyte, at the fleet's ask rate
(tens per day at most): the accepted growth is under one megabyte per
week. Rotation of `plans/seats/asks/` belongs to goal answer-archive,
which owns harvest and rotation for the channel ledger; the seat of
record adds `plans/seats/asks/` to that goal's scope by name when this
design is approved, and until it lands nothing is removed.

Legacy transport. Publish pushes the configured remote only (item 7);
no seat verb calls sync-transport.sh. The transport bare repository
receives seat commits when the next landing's sync mirrors origin's
branch head (sync-transport.sh:31-35), so it lags by one landing. No
machine reads from it today (transport-sync-law.md:3), and its
retirement is that goal's question; SMA-F-TRANSPORT proves the lag and
the catch-up.

Fixtures: `scripts/agents/seat-fixtures.sh`, on the channel fixture
bed (bare origin, export clone, fake provider serving;
channel-fixtures.sh:21-54) plus a second clone `export-b` enrolled as
`fixture-b` with its own lineage; machine A is `fixture-machine`. A
third clone `export-old` is built from the commit before slice 1 (the
fixture builds that engine once into its bed). Every assertion reads
origin/main, a verb's stdout and exit code, a component record, the
health line, the hook's rendered message, or `steward digest-pending`.

- SMA-F-ENABLE: before `seat enable`, A's tick records component
  `seat-presence` as `seat-not-enabled` and writes nothing; `seat ask`
  is refused `seat-not-enabled`; after `seat enable` (run with the
  fixture's terminal classification) both proceed.
- SMA-F-MEMBER: A runs `up`; origin/main holds A's membership record
  with A's engine and seatSchema 1; B's `seat fleet` prints A
  `unreachable, no presence yet` and lists no unknowns; a machine
  named only in a claim prints under `unknown to the ledger`.
- SMA-F-PRESENCE: A ticks; presence lands with A's engine, chain null,
  the tick's opid; B's `seat fleet` prints A `reachable` with A's
  claimed goal's next step's first sentence; a second tick within
  seat.presence-min with nothing changed adds no commit.
- SMA-F-UNKNOWN-VS-UNREACHABLE: B `seat ask --to nobody` is refused
  `seat-unknown-machine`; with A's presence aged past the window
  (fixture sets seat.presence-min=0), `seat ask --to fixture-machine`
  proceeds with the unreachable warning on stderr.
- SMA-F-ASK-ANSWER: A asks B; B's tick with seat.answer-min=0 prints
  `seat-questions=alive (owes 1 seat question: fixture-machine asks
  about <goal>, ..., question <qid>; show it with ...)` and B's
  pending digest names the ask once across two ticks; B `seat answer`;
  A's `seat wait` prints the text and exits 0; A's tick digest names
  the answer once.
- SMA-F-DIGEST-NO-WORDS: with every text field of the ask and answer
  set to the sentinel `ZQXJ-SENTINEL-<n>`, neither machine's health
  line, pending digest, narration line, status post nor the Stop
  hook's rendered message (the hook run against the fixture bed)
  contains the sentinel; `seat show` does.
- SMA-F-TIMEOUT-THEN-LATE: A asks with `--deadline 1`; A's `seat wait`
  exits 4 with `expired:` and origin/main shows state expired; B `seat
  answer` exits 3 with `late:`, the record carries `late` and state
  stays expired; A's digest names it as answered late; B's health role
  no longer owes it.
- SMA-F-NOT-MINE and SMA-F-WITHDRAW: exit 2 on A's wait with the reason;
  the digest names the close.
- SMA-F-ALREADY: a second `seat answer` on an answered ask exits 3
  `seat-already-answered`; the answer opid is unchanged.
- SMA-F-WRONG-MACHINE: A `seat answer` on an ask addressed to B is
  refused `seat-not-addressee`; a plain git commit on the ledger
  branch (no trailer) writing a presence record for `fixture-b` under
  an opid naming `fixture-machine` makes B's next ledger-attention
  report failed naming `seat-writer`, and B's accepted ref does not
  advance.
- SMA-F-HUMAN-QUESTION-REFUSED: a valid channel question record is
  hand-committed on the ledger (the shape channel_test.go:151 stages);
  `seat answer --question <that id>` is refused with "is a question to
  the human"; a seat ask hand-committed with a `wants` key is refused
  `seat-json`.
- SMA-F-MIXED (SMA-C-02): `export-old` fetches a tip carrying every
  seat record kind, `goal fetch` advances, and a goal verb from it
  lands; then a landing from `export-old` that creates
  `plans/seats/junk.json` is accepted by the old engine and refused by
  A's and B's next ledger-attention pass with `seat-unknown-path`; a
  hand commit removing it restores both.
- SMA-F-NO-STEWARD: A asks B with `--deadline 1` while B never ticks;
  A's wait exits 4; B's later tick shows nothing owed and no dead
  role.
- SMA-F-OLD-STEWARD: `export-old` has no membership record and is
  `unknown`; after it is rebuilt on the new engine and runs `up`, it
  is a member and can be asked.
- SMA-F-RECOVER-ASK, -ANSWER, -CLOSE, -PRESENCE, -MEMBER:
  METASYSTEM_CHANNEL_FAIL_AT-style failure points (`after-push:seat-ask`
  and the like, read into the seat verbs as the channel verbs read
  theirs) leave a pushed entry with the outcome unknown; `goal recover`
  on that clone reports `completed from the stored intent: confirmed`
  for ask, answer and close with the record's opid equal to the
  entry's, and `abandoned` for presence and membership; a second run
  with the commit already landed reports `confirmed on the canonical
  tip`; an answer entry recovered after the deadline lands as late.
- SMA-F-UNKNOWN-ALERT (SMA-C-03): with A's accepted ref made unreadable,
  two ticks set ShouldAlert through `seat-questions=unknown` alongside
  ledger-attention; with the ref restored and A's push made to fail
  (the fixture points goal.sync-remote at an unwritable path for one
  tick), the role stays alive, the `seat-presence` component records
  the error, `seat fleet` prints `presence not published since ...`,
  and ShouldAlert is false.
- SMA-F-CHURN: 400 seat-only commits staged on origin; one
  ledger-attention pass on B completes inside the scaled fixture cap
  and reports no event; B's digest gains no "ledger moved" entry.
- SMA-F-TRANSPORT: with a `transport` remote configured on A, presence
  commits do not appear on it until A runs a landing; after
  sync-transport.sh they do.
- SMA-F-LINEAGE: A's runner.json carries armedLineage after `steward
  arm`; A's presence shows it; a lease succession on A without re-arm
  leaves it unchanged and `seat fleet` prints the same `armed by`.
- SMA-F-STATUS-CLAUSE: B's `channel status` shows `Owes 1 question(s)
  from other seats` only while the ask is open and inside its
  deadline.

Go unit tests, respecting the floors of section 1 item 9:
internal/goal/seats_test.go: TestValidateSeatTreeIsTotal (the
state-by-field grid, one cell per row of the refusal table and one
legal shape per state), TestSeatPresenceMutateWritesOnChangeOrAge,
TestSeatPresenceRefusesYoungerTip, TestSeatWritersRefuseWhenNotEnabled,
TestSeatAskMutateRefusals (self, unknown, retired, goal missing,
duplicate), TestSeatAnswerTransitions (answer, late answer, not-mine,
withdraw, expire, already-answered, already-late, closed,
not-addressee, not-asker, already-applied, lost-to-competitor),
TestSeatRequestsRebuildFromIntent (every verb's Intent through
requestForEntry yields the entry's opid; presence and membership are
abandoned), TestSeatOpidMachineSegment, TestSeatSecretRefusesSixDigitsAnywhere.
internal/steward/seats_test.go: TestSeatQuestionsRoleThreeAges,
TestSeatQuestionsRoleNeverUnknownOnPresenceFailure,
TestSeatDigestEntriesFireOnce, TestNoSeatTextReachesHumanSurfaces
(sentinel text in every field; health line, digest entries, narration
line and status clause contain none of it),
TestPresenceComposesChainFromJobRecords (root through parentJob, null
when none), TestPresencePublishFailureNeverDegradesTick,
TestStageBuilderSkipsSeatOnlyCommits, TestArmSnapshotsLineage.
internal/lease: TestOwnerLineageReadSeam. internal/up:
TestUpPublishesMembershipAfterEnable. internal/channel/report_test.go:
TestStatusReportOwedClauseOnlyWhenOwed. cmd/metasystem:
TestChannelWaitDelegatesToSeatWait, TestSeatEnableRefusesAgentCaller.

## 10. Build order and the box

Four slices, each landing alone with its own gate.

1. Records, validator, fences, reader, request constructors and
   recovery cases, no writer and no verb: internal/goal/seats.go, the
   ValidateCommit call, the requestForEntry cases, the path-classes
   row, the guard regexp. Gate: `go test ./internal/goal` under the
   coverage delta, path-class and pre-commit-guard fixtures green.
   One to two attempts, 45 to 60 minutes. The fleet order's steps (2)
   and (3) follow this landing.
2. The enable and retire verbs, membership in `up`, the lineage seam
   and snapshot at arm, presence in the tick with its component
   record, the stage builder's seat-only skip, `seat fleet`. Gate:
   SMA-F-ENABLE, MEMBER, PRESENCE, LINEAGE, CHURN, TRANSPORT,
   RECOVER-PRESENCE, RECOVER-MEMBER and the steward unit tests. Two
   attempts, 75 to 100 minutes.
3. `seat ask`, `seat answer`, `seat close`, `seat wait`, `seat show`,
   the `channel wait` delegation, and the fixtures SMA-F-ASK-ANSWER,
   UNKNOWN-VS-UNREACHABLE, TIMEOUT-THEN-LATE, NOT-MINE, WITHDRAW,
   ALREADY, WRONG-MACHINE, HUMAN-QUESTION-REFUSED, MIXED, NO-STEWARD,
   OLD-STEWARD, RECOVER-ASK/ANSWER/CLOSE. Gate: those fixtures and the
   goal unit tests. Two to three attempts, 90 to 120 minutes.
4. The health role, the digest lines, the status clause, SMA-F-DIGEST-
   NO-WORDS, UNKNOWN-ALERT, STATUS-CLAUSE and the role assertions inside
   ASK-ANSWER. Gate: health fixtures and the seat fixtures green. One
   to two attempts, 45 to 60 minutes.

One closing code review after slice 2 and one after slice 4, each its
own critique chain with the box's three rounds; slice 3 is reviewed
with slice 4 because they share the fixture script. Estimate: six to
nine build attempts, 255 to 340 build minutes, plus two reviews at 30
to 45 minutes each, 315 to 430 job-minutes in all, over two to three
days. The box (one day, six attempts, 240 job-minutes, three review
rounds) does not hold this build. The number the seat asks Wido for,
before slice 1 is dispatched: elapsed three days, ten attempts, 480
job-minutes, review rounds unchanged at three. This design is not
built inside the current box in pieces; the raise is the precondition.

## 11. Non-goals

No new provider and no second bot. No change to the TOTP rule, to
FCG-COMMIT-05 or to any channel record. No authority for seats: a seat
answer approves, budgets, resumes and enrolls nothing. No archive or
rotation here (goal answer-archive owns harvest and rotation, and
gains `plans/seats/asks/` by name). No central brain: every machine
reads the same branch and decides for itself. No skip verb for a
poisoned seat record. No mark on the goal file for a seat ask. No
retirement of the transport relay (goal transport-sync-law).

## 12. Self-grade

Confidence: medium-high that the shape is right and buildable in the
existing engine, because every write is a goal.Publish with a Mutate
and a recovery case of the kinds already landed for the goal verbs and
the channel, and every read is a tip the tick already fetches.
Weakest claim: that the seat-only skip in the ledger-attention stage
builder keeps a nine-machine fleet's ticks cheap at 400 seat commits a
day; SMA-F-CHURN measures it, and if it fails the presence cadence
doubles to 120 minutes before anything else changes. Second weakest:
the `up` package as the membership writer: it imports lease and
steward (up.go:20-21) and not goal, so slice 2 adds a goal import
there; that is cycle-free because no file under internal/goal imports
steward or up, but `up` has never published a ledger transaction and
its endpoint, lineage and error handling for one are new code the
implementer writes against section 2's rules. Reject condition: Wido wants the records
under `plans/channel/`, which makes slice 1 a fleet-wide
rebuild-before-writer action without the enable marker's protection;
or he wants no human act in the rollout, which removes the enable
marker and reopens SMA-C-02; or he wants the human alerted at 30
minutes, which collapses the two ages to one.

## Dispositions (round 1, job sma-crit1-20260906)

| Finding id | Disposition | Reasoning and evidence | What changed in the design |
|---|---|---|---|
| SMA-C-01 | accepted | The Stop hook carries the pending digest into the runtime's message (supervision-hook.sh:534-548, 605-607) and seat-communication.md:1-9 names digest and stop messages as human-reaching; revision 1 copied question, ifSilent, answer and close text into digest lines | Section 6: digest, health line, status post and stop message carry names, ages, deadlines, the qid and the verb only; `seat show` is the one reader of the words, `seat wait` the second (stdout to a script); the consumer list is stated as a structural fact; TestNoSeatTextReachesHumanSurfaces and SMA-F-DIGEST-NO-WORDS prove it; section 8 adds the surfaces to the closed-path list and the conduct residual |
| SMA-C-02 | accepted | To an old engine `plans/seats/` is class `record` (path-classes.txt:30, longest prefix), the guard regexp covers only goals and channel, and a landing may create a new plan record (observe.go:687-693); revision 1 called the directory harmless | Section 2: the enable marker written once by the human at the terminal, refused by every writer while absent (`seat-not-enabled`); section 8: the fleet order (slice 1 with no writer, every machine rebuilds, `seat enable`, then writers), refusal not tolerance for a spoiled record with Wido's first-commit-wins law as the reason, and SMA-F-MIXED proving both halves |
| SMA-C-03 | accepted | Two consecutive unknowns set ShouldAlert (health.go:544-549) and the tick is ten minutes by default (runner.go:52-60); revision 1 marked the role unknown on a refused presence publish | Section 3: presence publish failure goes to the tick's `seat-presence` component record, `seat fleet`'s local line and the best-effort narration note, never to the role; section 6: the role's only unknown is an unreadable ledger, the same fault ledger-attention raises, whose two-tick alert is accepted; dead alerts at once by the no-lawful-remedy rule (health.go:566-568) at 180 minutes; SMA-F-UNKNOWN-ALERT and TestSeatQuestionsRoleNeverUnknownOnPresenceFailure |
| SMA-C-04 | accepted | The Intent must be complete (journal.go:57-66); requestForEntry rebuilds only known verbs and derives the opid from the entry (recover.go:209-229, 236-403); a pushed entry with unknown outcome blocks the clone (txn.go:512-516) | Section 7: every seat verb's complete Args, a rebuild constructor shared with the live verb, named outcomes (confirmed, lost, rejected by name; presence and membership abandoned, never replayed); a recovered answer past the deadline lands as late; SMA-F-RECOVER-* and TestSeatRequestsRebuildFromIntent |
| SMA-C-05 | accepted | Revision 1 enumerated presence, claims and asks only, so an idle real machine and a nonexistent one looked alike, and the `paper never seen` example could not be produced | Section 2: the membership record written by `up` at every session start (the fleet-join design's last step), `seat retire` at the terminal; the standings unknown, unreachable, reachable, retired; `seat ask` refuses unknown and retired and warns on unreachable; `seat fleet` lists members and names unknowns; SMA-F-MEMBER, UNKNOWN-VS-UNREACHABLE, OLD-STEWARD |
| SMA-C-06 | accepted | Revision 1 had no default timeout, no durable deadline, and let an answer after the asker's timeout still become the answer | Section 4: deadlineAt on the record, `--deadline` with default `seat.ask-deadline-min` 240; section 5: states open, answered, expired, closed; an answer after deadlineAt is recorded in `late`, state expired, exit 3, never binding; `seat wait` exits 0, 2, 4 or 1 by outcome and writes the expire transition; a target that never runs a steward yields exactly `expired`; SMA-F-TIMEOUT-THEN-LATE and SMA-F-NO-STEWARD |
| SMA-C-07 | accepted | Revision 1's seat-state row left state vocabulary, per-state field sets, empty payloads and target existence undefined at rest | Section 8: the refusal table is total: state vocabulary, exact field set per state, non-empty payloads, time ordering, `seat-unknown-machine` at rest for `from` and `to`, `seat-writer` for every by/opid pair; TestValidateSeatTreeIsTotal enumerates the grid |
| SMA-C-08 | accepted | ledger-attention projects every intervening commit (ledgerattention.go:243-268); Publish pushes only the configured remote and the transport mirror is a landing-run script (sync-transport.sh:31-35); the relay's status is goal transport-sync-law's | Section 9: presence default 60 minutes; the stated bound of 400 seat commits per day; the stage builder skips projection for seat-only changes; SMA-F-CHURN tests the bound; growth stated and rotation assigned to goal answer-archive by name; transport lags by one landing, proven by SMA-F-TRANSPORT |
| SMA-C-09 | accepted | OwnerLineage lives on the lease (lease.go:29-44) behind a private loader (76-102); CurrentHolderView omits it; lease imports steward (lease/verbs.go:16); RunnerRecord has no lineage (runner.go:40-46) | Section 3: `lease.OwnerLineage(root)` as the public read seam, read by the two arming callers (steward_verbs.go, up.go) that already import both packages, passed into steward.Arm and snapshotted as RunnerRecord.armedLineage; presence reports the arming lineage, with the reason; SMA-F-LINEAGE, TestArmSnapshotsLineage, TestOwnerLineageReadSeam |
| SMA-C-10 | accepted | Revision 1's eight fixtures and thirteen tests covered none of the mixed-version, recovery, absent-steward, target-standing, late-answer, alert, digest, churn, transport or lineage cases, and the estimate deferred the raise | Section 9: eleven new fixture scenarios and eight new unit tests named; section 10: four slices, 315 to 430 job-minutes over two to three days, the box does not hold it, and the raise (three days, ten attempts, 480 job-minutes) is asked for before slice 1 |
