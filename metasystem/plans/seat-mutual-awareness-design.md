# seat-mutual-awareness — design: seats see each other and ask each other (revision 3)

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

Revision 2 answered critique round 1 (job sma-crit1-20260906, ten
findings SMA-C-01..10 at commit 6948392b, register at
records/misc/seat-mutual-awareness-critique-r1.md). Revision 3, the
last the ladder allows before the build, answers critique round 2
(job sma-crit2-20260906, ten findings SMA-C-11..20 at commit 2e109816,
register at records/misc/seat-mutual-awareness-critique-r2.md); both
rounds' dispositions are at the end. Every cite in this revision was
re-read at branch agent/sma-design1-20260906 on main 7a41b564. What
changed in revision 3, in one breath: the design stops claiming a code
fence against old checkouts and states the three things that actually
keep them out (the fleet order under the engine re-arm law, the fact
that no old verb writes under the directory, and refusal of a spoiled
tip), while the enable marker becomes a record the validator can bind
to its own transaction commit so an old landing cannot mint one; a
human-only `seat repair` at the terminal is the executable way off a
spoiled tip, exempt from the seat fences by its intent; `seat enable`
and `seat repair` land in slice 1 and every human verb takes `--by`;
enable, retire and repair have complete journal intents, rebuild
cases and named outcomes, with human acts rejected toward the
terminal rather than replayed; expiration has two owners and every
post-deadline transition exactly one outcome; a retired membership is
immutable except by a human un-retire; the lineage rides an argument
into the detached runner with a named no-lease value; the presence
timestamp rule is equality; the proof matrix gains the seven named
cases; and the box is counted in reservations.

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
2. Where the new records go, and what an old engine does with them.
   The channel validator lists `plans/channel/` and refuses any path it
   does not know with `channel-unknown-path` (channel.go:140, 168-171),
   and every record decoder refuses unknown keys (channel.go:370-374,
   DisallowUnknownFields). So a new record kind or key UNDER
   `plans/channel/` makes every engine that predates it refuse the
   shared tip. `plans/seats/` is a sibling that ReadCommitGoals
   (validate.go:405-407 lists only the goals and records prefixes) and
   ValidateChannelTree never list, so an old engine's READ path ignores
   it. An old engine's WRITE path is not fenced by code and cannot be
   (SMA-C-11): to it the directory is the generic plan-record class
   (scripts/agents/path-classes.txt:30 `install:plans/ record`, longest
   prefix wins, internal/pathclass), the pre-commit guard's regexp
   covers only `plans/(goals|channel)/`
   (scripts/agents/pre-commit-guard.sh:70-85), and a landing may create
   a previously absent plan record (internal/landing/observe.go:687-693).
   No engine can change what an older engine does. Section 8 states
   what actually keeps old checkouts out and who accepts what remains.
3. The human answer object cannot honestly carry a seat's answer: its
   `step`, `userId` and provider `ref` are required and non-null
   (channel.go:439-455), and phase `approved` demands a receipt
   (channel.go:543-546). So seat asks get their own record and never
   touch the question schema (section 4).
4. An operation id is `<26-char ULID>-<machine>-<8 hex of the lineage
   hash>` (internal/goal/file.go:1494-1497). Every transaction commit
   carries it as a `Goal-Transaction` trailer (txn.go:283), and
   TrailerPresent walks the log of a tip for that trailer
   (txn.go:379-390). Recovery rebuilds a dead owner's request from the
   journal entry alone and refuses when the rebuilt opid differs
   (internal/goal/recover.go:209-229); the Intent must be complete
   (internal/goal/journal.go:57-66); a verb without a rebuild case is
   terminalised rejected (recover.go:185-194, 403); a human act that
   cannot be replayed from journal text is rejected with a message
   naming the terminal act (`resume`, `steal`, a human-ratified
   `split`: recover.go:146-169). Section 7 covers every seat writer.
5. Human verbs take the human's name as `--by` and the engine prefixes
   `human:` (`goal repair --accept-remote --by <human>`,
   cmd/metasystem/goalsync_verbs.go:416-449 and
   internal/goal/accepted.go:22-25; `set-pin` and `steal` carry Actor.Human
   from the same flag, recover.go:377-393). The terminal proof supplies
   only the CLASS: lease.ClassifyAt returns `Class` with process
   coordinates and no person (internal/lease/verbs.go:243-253), and
   requireHumanStewardEnrollment consumes only that class
   (cmd/metasystem/steward_verbs.go:656-672). An approval's
   `authority=proven` is validated at rest by binding the approval
   record to the goal's own History event with the same opid, actor
   and time (file.go:586-596); the tree itself is not authenticated
   (accepted.go:9). `goal repair --accept-remote` is the shape of a
   human-reserved repair: clone-local, journaled under `repair-<nonce>`
   with a named intent and the human's name, validated whole before it
   moves anything (accepted.go:22-80). Every goal transaction refuses
   an invalid captured tip before it mutates (txn.go:595-613), which is
   why a repair that must build ON a spoiled tip needs its own path
   (section 8, `seat repair`).
6. Health: the tick evaluates a fixed role list and prints every role
   with its reason (internal/steward/health.go:44-80, 176-193, 272-300);
   two consecutive unknowns alert (health.go:544-549); a dead role with
   no lawful remedy alerts at once (health.go:556-568); an unhealthy
   aggregate opens an alert episode to the human
   (internal/steward/alert_episode.go:241-281). The tick is ten minutes
   by default (internal/steward/runner.go:54). PreviewHealthAt renders
   without advancing the breaker (health.go:242-270). The narrator
   digest deduplicates by source (internal/narratordigest/digest.go:105-129);
   the Stop hook carries it into the runtime's message
   (scripts/agents/supervision-hook.sh:534-548, 605-607), and
   docs/seat-communication.md:1-9 names that surface as human-reaching.
7. The runner and the lineage. `steward arm` mints the enrollment and
   launches the runner detached with exactly `steward run --repo
   <root>` (runner.go:629-645, launchRunner); the runner process, not
   the arming process, writes runner.json in RunLoop
   (runner.go:88-95); `steward run` is the verb that calls RunLoop
   (cmd/metasystem/steward_verbs.go:509-520). The lease carries
   OwnerLineage (internal/lease/lease.go:29-44) behind the private
   loadLease, which errors on an absent lease only when the caller
   requires it (lease.go:76-84); lease imports steward
   (internal/lease/verbs.go:16). `metasystem up` re-arms a rebuilt
   engine at its enrolled path when the build commit is reachable
   from the configured landing ref (internal/up/up.go:427-456,
   ReArmRebuiltEngine at runner.go:232), landed on main today, so a
   pull followed by a rebuild and `up` replaces the running engine
   without a terminal act. Fresh joining arms the steward before `up`
   (plans/fleet-join-bootstrap-design.md:234-263), so at arm time on a
   fresh machine there may be no lease.
8. The ledger-attention pass projects every intervening commit
   (ledgerattention.go:226-268) and emits an event only on a change of
   claimable, pinned or queue sets. Publish pushes the configured
   remote only (txn.go, `push e.Remote --force-with-lease`); the legacy
   transport mirror is a landing script (scripts/agents/sync-transport.sh:31-35)
   under goal transport-sync-law (plans/goals/transport-sync-law.md:3-6).
9. Budget accounting (SMA-C-20): an attempt is one admitted job
   reservation and the tuple is complete or nothing
   (docs/backlog-mechanism.md:17-30); the dispatcher reserves 120
   job-minutes per job. The goal record at revision 19 reads
   `elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=720
   activeJobLimit=1 reviewRoundLimit=2` (plans/goals/seat-mutual-awareness.md:11),
   of which one attempt and 120 minutes are used by this design chain.
   Coverage floors: internal/goal 80.0 and internal/steward 74.0
   (scripts/agents/coverage-ratchet.json).

## 2. The directory, the enable marker, membership

Location: `plans/seats/` on the ledger branch (goal.sync-branch,
txn.go:49-61), four sub-paths, canonical serialisation MarshalChannel
(channel.go:111-117), keys in table order, no unknown keys, every time
RFC 3339 UTC at second precision (parseChannelTime, channel.go:650-656,
reused).

Enable marker: `plans/seats/enabled.json`, written once by the human,
and bound to its own transaction so that it cannot be minted by a
landing (SMA-C-11).

| key | type | required | meaning |
|---|---|---|---|
| enabledAt | time | yes | |
| by | string | yes | `human:<name>` from `--by` |
| authority | string | yes | exactly `proven` |
| machine | string | yes | the enrolled machine the terminal act ran on |
| opid | string | yes | the opid of the transaction that wrote this record; its machine segment equals `machine` |

`metasystem seat enable --by <name> --repo <root>` runs only from the
agent-free terminal (requireHumanStewardEnrollment's classifier,
steward_verbs.go:656-672, supplies the class; `--by` supplies the name,
the way `goal repair --accept-remote --by` does) and refuses every
other caller with the same text enrollment uses. It is one ledger
transaction under `Opid(<fresh ULID>, <machine>, "human-terminal")`,
Intent verb `seat-enable`, Targets the path, Args {by, machine,
enabledAt}, message `seat enable by <by>`. What the validator checks
at rest that a landing cannot fake without forging a transaction:
`authority` is `proven`; `by` starts with `human:`; the opid's machine
segment equals `machine`; and `TrailerPresent(commit, opid)` holds,
that is, the opid appears as the `Goal-Transaction` trailer of a
commit in the validated tip's history (txn.go:379-390). A landing from
an old checkout is a plain commit with `Goal-Item` trailers and no
`Goal-Transaction` trailer for a ULID it never minted, so a marker it
writes fails `seat-authority` and every upgraded writer stays refused.
The same trailer binding is applied to every seat record's `opid`
(section 8's `seat-writer` row), so no seat record an old landing
creates is ever a legal record. There is no relayed-word form of
enable: it is a terminal-only act like enrollment.

Membership record: `plans/seats/members/<machine>.json`.

| key | type | required | meaning |
|---|---|---|---|
| machine | string | yes | equals the basename |
| engine | string | yes | the engine digest of the writer |
| seatSchema | integer | yes | exactly 1 |
| joinedAt | time | yes | the first write; preserved on every later write |
| updatedAt | time | yes | |
| retiredAt | time or null | yes | set by `seat retire`, cleared only by `seat unretire` |
| retiredBy | string or null | yes | `human:<name>`; non-null exactly when retiredAt is non-null |
| opid | string | yes | opid of the writing commit; machine segment equals `machine` for a refresh, equals the terminal's machine for retire and unretire |

Writer: `metasystem up` (internal/up/up.go), at every session start,
after enrollment and lease steps succeed, under the session's opid,
Intent verb `seat-member`, Targets the path, Args {machine, engine,
seatSchema, joinedAt}. Its Mutate: refuses `seat-not-enabled` when the
marker is absent; refuses `seat-retired` when the tip's record has
retiredAt non-null, writing nothing (SMA-C-16: a retired record is
immutable to every writer but `seat unretire`); writes when the path
is absent, or the tip's engine differs, or the tip's updatedAt is
older than 24 hours, carrying joinedAt from the tip; else NothingToDo
(txn.go:462); refuses `seat-transition` when the tip's updatedAt is
younger than the one it read. A publish failure is one `up` component
outcome (`component=seat-member outcome=<detail>`) and never changes
`up`'s aggregate. `metasystem seat retire --machine <m> --by <name>
--repo <root>` and `metasystem seat unretire --machine <m> --by <name>
--repo <root>` are terminal-only like enable, each one transaction
under the terminal's opid, Intent verbs `seat-retire` and
`seat-unretire`, Args {machine, by, at}; retire's Mutate refuses
`seat-already-retired`, unretire's refuses `seat-not-retired`; both
preserve joinedAt and engine. Nothing under `plans/seats/` is deleted
by any Mutate except `seat repair` (section 8).

Standings a reader derives, by name: `unknown` (no membership
record); `unreachable` (a member, not retired, no presence younger
than the staleness window); `reachable` (a member with fresh
presence); `retired` (retiredAt set). The fleet-join design already
makes `up` the last step of joining, so a joining machine becomes a
member the moment it joins.

## 3. Discovery: the presence record and `seat fleet`

Decision: a durable per-machine record, because goal claims cannot see
a chain, a landing or an engine. The record does NOT carry the claimed
goals: claims are on the same tip in every goal file's `Claimed:` line
(delivery.go:63 reads them); the read verb joins them at the tip it
reads.

Record: `plans/seats/presence/<machine>.json`.

| key | type | required | meaning |
|---|---|---|---|
| machine | string | yes | equals the basename |
| armedLineage | string | yes | the lineage the arming caller handed to the runner (below); the literal `no-lease` when there was no lease at arm time; the literal `unknown` when the runner was started by an engine that predates the field |
| engine | string | yes | the engine digest the tick runs |
| chain | object or null | yes | the newest non-terminal delegate job: `{root, job, role, round, goal, startedAt}`, strings non-empty except `goal` which may be "", `round` an integer at or above 1; `root` by following `parentJob` until absent; null when none |
| lastLanding | object or null | yes | `{commit, goal, at}`, strings non-empty, from the newest landing receipt in memory/receipts.log (delivery.go:85); null when none |
| tickAt | time | yes | the tick that composed this version |
| updatedAt | time | yes | EQUALS tickAt; the validator refuses any other value (SMA-C-18) |
| opid | string | yes | opid of the writing commit; machine segment equals `machine` |

Writer and cadence. The steward tick composes the record after its
ledger-attention pass (internal/steward/tick.go:178-186) and publishes
it under `Opid(<fresh ULID>, <machine>, armedLineage)`, Intent verb
`seat-presence`, Targets the path, Args {machine, engine, body} where
`body` is the whole composed record as one JSON string, message `seat
presence <machine>`, Validate ValidateCommit. Its Mutate writes when
the path is absent, when the body digest (every key but tickAt,
updatedAt, opid) differs from the tip's, or when the tip's updatedAt is
older than `seat.presence-min` (default 60 minutes); else NothingToDo.
It refuses `seat-transition` when the tip's updatedAt is younger than
the one it read, and `seat-not-enabled` when the marker is absent.

Where a presence failure goes: a tick component attempt named
`seat-presence` (beginComponentAttempt, tick.go:126), the local line
of `seat fleet` (`presence not published since <last confirmed>:
<detail>`) and the best-effort narration note (narrate.go:135-189). It
never degrades the tick, never enters the seat-questions role and
never alerts. A pushed-blocking refusal (txn.go:512-516) is the detail
`journal blocked by <opid>; run goal recover`.

Staleness. updatedAt older than twice `seat.presence-min` (120
minutes) makes the machine `unreachable`, printed `silent since
<updatedAt>`; a member with no presence record is `unreachable` with
`no presence yet`. Because updatedAt must equal tickAt and tickAt is
the composing tick's own clock, a hand-written future timestamp is
refused at rest rather than extending reachability.

Lineage handoff (SMA-C-17), end to end. (1) The arming caller reads the
lineage: `steward arm` (steward_verbs.go) and `metasystem up`
(up.go) both import lease and steward, so they call a new public
`lease.OwnerLineage(root) (string, bool, error)`, a wrapper over
loadLease with `required=false` (lease.go:76-84) that returns
`("", false, nil)` when no lease exists; the caller maps that to the
literal `no-lease`. (2) steward.Arm and ArmTemporary (runner.go:154,
163) gain a `lineage string` parameter and pass it to launchRunner,
which appends `--armed-lineage <value>` to the child's argv
(runner.go:635, today `steward run --repo <root>`). (3) `steward run`
(steward_verbs.go:509-520) parses the flag and passes it to RunLoop,
which writes it into RunnerRecord as `ArmedLineage` (runner.go:88-95);
a RunLoop started without the flag writes `unknown`. (4) The tick
reads RunnerRecord (the runner's own file, same package) to compose
presence and to mint its opid. Presence reports the ARMING lineage,
not the current lease holder's: the record describes the runner
process, which the arming session started; a lease succession that
does not re-arm leaves the runner as it was, and the succeeding
session is visible on the ledger through its own claims' lineage. The
literal values are named, never guessed: `no-lease` (a fresh machine
armed before its first `up`) and `unknown` (a runner from before the
field). Under the re-arm law (item 7) the next `up` after a rebuild
replaces such a runner and the value becomes the session's.

The read verb: `metasystem seat fleet [--root <checkout>] [--no-fetch]`
captures the tip with the bounded fetch (project.go:44, 117-123;
`--no-fetch` reads the accepted ref), reads the seat tree and the goal
tree, and prints one block per member, retired ones last, plus
`unknown to the ledger: <names>` for machines named only in claims or
asks:

```
m2   reachable (presence 4 min ago, engine 9a1c0f2e), armed by session main-1788683763-71870-f7f607
     claimed: channel-poll-refuses-legacy-budget-questions — next: <first sentence of the goal's next step>
     chain: implementer round 2 under root fcg-build3 for fleet-channel-gateway, started 41 min ago
     last landing: 8c86a874 for seat-mutual-awareness, 2 h ago
     owes 1 question (from m1d about seat-mutual-awareness, 12 min, deadline in 3 h 48 min); waiting on 0
m3   unreachable, silent since 2026-09-06T08:10:00Z (engine 7b2d11aa)
paper unreachable, no presence yet (joined 2026-09-07T09:00:00Z, engine 7b2d11aa), armed by no-lease
m1b  retired 2026-09-05T10:00:00Z by human:Wido
```

Question ids and goal names appear; question text never does.

## 4. Asking a seat: the ask record and `seat ask`

Record: `plans/seats/asks/<qid>.json`, `<qid>` a fresh ULID
(question.go:184 mints one the same way).

| key | type | required | meaning |
|---|---|---|---|
| id | string | yes | equals the basename |
| from | string | yes | the asking machine; equals the opid's machine segment; a member at the tip |
| fromLineage | string | yes | non-empty |
| to | string | yes | never equals `from`; a member at the tip |
| goal | string | yes | a goal file must exist on the tip (channelGoalIDs, channel.go:221-241) |
| facts | []string | yes, one to four non-empty entries | |
| question | string | yes, non-empty | |
| ifSilent | string | yes, non-empty | what the asker does at the deadline if nobody answers |
| askedAt | time | yes | |
| deadlineAt | time | yes | later than askedAt; the one durable boundary |
| opid | string | yes | opid of the ask commit |
| state | string | yes | open, answered, expired, closed |
| answer | object or null | yes | `{text, by, lineage, at, opid}`, text non-empty, `by` equals `to`, `at` not later than deadlineAt (equal is on time); non-null exactly when state is answered |
| late | object or null | yes | `{kind, text, by, lineage, at, opid}` with `kind` one of answer, not-mine, withdrawn and `at` later than deadlineAt; non-null only when state is expired and one late act was recorded |
| expiredAt | time, expired only | | when the record was first written past its deadline |
| expiredBy | string, expired only | | the machine whose transaction expired it |
| closedAt, closedBy, closedBecause, closedReason | strings, closed only | | closedBecause `not-mine` (closedBy equals `to`) or `withdrawn` (closedBy equals `from`); closedReason non-empty |

No `kind`, `wants`, `budget`, `options`, `destination`, `thread`,
`step`, `userId` or `ref` key exists; an unknown key is refused. The
ask touches no goal file.

Verb: `metasystem seat ask --to <machine> --goal <id> --fact <text>
[--fact ...] --question <text> --if-silent <text> [--deadline
<minutes>] [--root <checkout>]`, printing the qid; `--deadline`
defaults to `seat.ask-deadline-min` (default 240). Identity is
channelIdentity (channel_verbs.go:21-31). One transaction under the
asker's opid, Intent verb `seat-ask`, Args {id, to, goal, facts (JSON
array as one string), question, ifSilent, askedAt, deadlineAt}. Mutate
refuses by name before any write: `seat-not-enabled`, `seat-self`,
`seat-unknown-machine`, `seat-retired`, `seat-goal-missing`,
`seat-duplicate` (an open ask from me to that machine on that goal
with the same facts digest; prints the existing qid). An `unreachable`
target is a warning on stderr, not a refusal. A path already present
is the ordinary AlreadyApplied or LostToCompetitor classification
(channel.go:905-927 is the model).

## 5. Answering, expiring, closing, waiting, showing

Two owners expire an overdue ask (SMA-C-15), first commit wins: the
TARGET's steward tick, in RunSeatAttention, publishes the `expire`
transition for every open ask addressed to it whose deadlineAt has
passed; and the ASKER's steward tick does the same for every open ask
it sent whose deadlineAt has passed. A running `seat wait` on the
asker's side also publishes it at the moment it passes. Whichever
lands first is the record's expiration; the others classify as
LostToCompetitor and read the winner. A target that never ticks is
expired by the asker's tick; an asker that never ticks is expired by
the target's; both silent means nobody writes and readers still treat
(open, now past deadlineAt) as expired for display and health, so the
one durable boundary is deadlineAt whether or not a tick has written
it yet. Every post-deadline transition has exactly one outcome:

| transition (owner) | FROM | TO |
|---|---|---|
| answer (the machine named in `to`) | state open, now not later than deadlineAt | state answered; answer {text, by: me, lineage, at: now, opid: this commit} |
| expire (the target's tick, the asker's tick, the asker's wait) | state open, now later than deadlineAt | state expired; expiredAt now; expiredBy me; late null |
| late act (the machine named in `to` for answer or not-mine; the machine named in `from` for withdrawn) | state open, now later than deadlineAt; or state expired with late null | state expired; expiredAt set if absent, expiredBy me if absent; late {kind, text, by: me, lineage, at: now, opid: this commit}; the verb prints `late: recorded after the deadline <deadlineAt>; it binds nothing` and exits 3 |
| not-mine (the machine named in `to`) | state open, now not later than deadlineAt | state closed; closedBecause not-mine; closedBy me; closedReason the text |
| withdraw (`seat close`, the machine named in `from`) | state open, now not later than deadlineAt | state closed; closedBecause withdrawn; closedReason the text |

So not-mine and withdraw after the deadline are late acts recorded in
`late` with their kind, never a close; a close and an expiration can
never both win because their FROM rows are disjoint on `now` against
deadlineAt, and the transaction's own clock against the record's
deadline is the arbiter on every rebuild. An answer whose `at` equals
deadlineAt is on time; one second later is late. Verbs: `metasystem
seat answer --question <qid> --text <text>` or `--not-mine --because
<text>` (Intent verb `seat-answer`, Args {qid, outcome, text, at});
`metasystem seat close --question <qid> --because <text>` (Intent
verb `seat-close`, Args {qid, reason, at}).

Refusals by name: `seat-not-enabled`; `seat-not-addressee`;
`seat-not-asker`; `seat-already-answered` (exit 3, the standing
answer's time printed); `seat-already-late` (state expired with late
non-null, exit 3); `seat-closed` (exit 3 printing closedBecause and
closedReason); `seat-not-a-seat-question`, with the message "<id> is
a question to the human; seats do not answer those" when the id names
a channel question at the tip. AlreadyApplied and LostToCompetitor are
classified as in ClassifyChannelTransition (channel.go:905-927) over
the tuple `(state, answer.opid or null, late.opid or null, expiredBy
or null)`.

Exactly one outcome per ask for the asker: answered on time; closed
not-mine on time; closed withdrawn on time; or expired at deadlineAt,
whether or not a late act is ever recorded. A seat answer carries no
human authority: it is written only into `plans/seats/` and into no
goal file, and every authority consumer reads goal history
(RecordedNormApproval; AuthenticatedChannelApproval, verbs.go:67-93),
where a seat opid never appears. The residual, plainly: any process
that can push to the ledger branch as machine X can answer as X and
can forge an opid naming X; the ledger already trusts that for claims,
and a seat answer authorizes nothing.

`metasystem seat show --question <qid> [--root]` prints the record to
the caller's stdout and is the ONE reader of seat-authored words
besides `seat wait`. `metasystem seat wait --question <qid> [--timeout
<minutes>] [--interval <seconds>, default 30]` fetches every interval,
reads the ask, and exits: 0 printing `answer.text` when answered; 2
printing `not-mine: <reason>` or `withdrawn: <reason>` when closed; 4
printing `expired: no answer by <deadlineAt>; <ifSilent>` when the
deadline has passed (publishing the `expire` transition, and exiting
4 whether or not that write wins); 1 `seat wait timed out` when
`--timeout` is shorter than the deadline and passes first. `--timeout`
defaults to the deadline. `channel wait --question <id>`
(channel_verbs.go:179-223) gains one lookup: when no local question
file exists and the id names a seat ask at the accepted ref, it runs
`seat wait` with the same flags.

## 6. Surfacing without polling, and no seat words on a human surface

The tick gains one read after its ledger-attention pass: RunSeatAttention
(new, internal/steward/seats.go) reads the seat tree at the accepted
ref (goal.AcceptedLedgerTip, internal/goal/attention.go:32-45),
publishes the `expire` transitions of section 5, and returns {owed:
open asks addressed to me inside their deadline, oldest first;
resolved: asks from me whose state is answered, expired or closed}.
TickResult carries it; NarrateDigest consumes it; the health role reads
the same tree.

Health role `seat-questions` (RoleSeatQuestions), after
`ledger-attention` in healthRoleOrder (health.go:64-80), evaluated in
evaluateHealthRoles (health.go:272-300) by reading the accepted ref
itself:

- alive, reason `owes no seat question`, when no open ask addressed to
  this machine is inside its deadline, or every such ask is younger
  than `seat.answer-min` (default 30 minutes);
- alive, reason `owes <n> seat question(s): <from> asks about <goal in
  words>, <age> min, deadline in <m> min, question <qid>; show it with
  metasystem seat show --question <qid>` while the oldest is between
  `seat.answer-min` and `seat.answer-dead-min` (default 180 minutes);
- dead, NoAutomaticRemedy true (the retro-debt pattern,
  health.go:381-396), the same reason prefixed `UNANSWERED SEAT
  QUESTION`, once the oldest passes `seat.answer-dead-min` and is still
  inside its deadline; it alerts at once (health.go:566-568), the
  intended fault report;
- unknown ONLY when the accepted ref or the seat tree cannot be read,
  the same fault ledger-attention raises; its two-tick alert is the
  existing law and is accepted. Presence publish failure never
  produces unknown.

Why two ages: a dead role alerts the human; the 30-minute key governs
what the seat sees at every turn end, the 180-minute key when the
fault becomes the human's business as a fault report. Keys are read
through internal/config (delivery.go:90 shows the idiom).

No seat-authored words on any human-visible surface. The digest, the
health line, the status post and the stop message carry NAMES only:
the asking or answering machine, the goal in words, the age, the
deadline, the question id and the verb that shows the words. The text
fields (facts, question, ifSilent, answer.text, late.text,
closedReason) are read by exactly two consumers, `seat show` and `seat
wait`, both to the caller's stdout; `seat fleet`, RunSeatAttention,
the health role, NarrateDigest, the narration line,
ComposeStatusReport and the Stop hook never read a text field, and
TestNoSeatTextReachesHumanSurfaces plus SMA-F-DIGEST-NO-WORDS prove it.
Digest lines (deduplicated by source marker, each fires once):

- new question owed: lowlight, source `seat-ask`/<qid>: `Seat <from>
  asks this machine about <goal in words> (deadline in <m> min). Show
  it: metasystem seat show --question <qid>. Answer it: metasystem
  seat answer --question <qid> --text "...".`
- answer received: highlight, source `seat-answer`/<qid>: `Seat <to>
  answered this machine's question about <goal in words>. Show it:
  metasystem seat show --question <qid>.`
- late act, not-mine or withdrawn: lowlight, source `seat-answer`/<qid>:
  `Seat <by> <answered late | closed as not theirs | withdrew | acted
  late> on this machine's question about <goal in words>. Show it:
  metasystem seat show --question <qid>.`
- expired unanswered (asker's side): lowlight, source `seat-answer`/<qid>:
  `This machine's question to <to> about <goal in words> expired
  unanswered at <deadlineAt>.`

The status post (report.go:36-112) gains one line, only when non-zero,
after the "Needs you" lines: `Owes <n> question(s) from other seats,
oldest <m> min`.

## 7. Recovery: every seat writer rebuilds or is sent back to the terminal

Every seat Publish is an ordinary goal-ledger transaction with a
complete Intent (journal.go:57-66), the pushed-blocking rule
(txn.go:512-516) and the recovery classification (recover.go:58-135).
What this design adds, total over the writers (SMA-C-14):

| verb | Intent.Args (complete) | rebuild case in requestForEntry (recover.go:236) | outcome when the owner died with the commit absent |
|---|---|---|---|
| seat-enable | by, machine, enabledAt | none: a human act | rejected with `human authority cannot be recovered from journal text; rerun metasystem seat enable --by <by> from the enrolled terminal` (the resume/steal pattern, recover.go:146-161); nothing is minted |
| seat-retire, seat-unretire | machine, by, at | none: human acts | rejected with the same shape naming `seat retire` / `seat unretire` |
| seat-repair | by, path, action (remove or replace), body (when replace), reason | none: a human act | rejected naming `seat repair` at the terminal |
| seat-member | machine, engine, seatSchema, joinedAt | none needed | abandoned with detail `the next up republishes membership` (the slice-start pattern, recover.go:102-108) |
| seat-presence | machine, engine, body | none needed | abandoned with detail `the next tick republishes presence` |
| seat-ask | id, to, goal, facts (JSON array), question, ifSilent, askedAt, deadlineAt | `seatAskRequest(r, args)` | completed under the SAME opid (recover.go:213-229): confirmed, lost (the path carries another opid), or rejected by name |
| seat-answer | qid, outcome (answer or not-mine), text, at | `seatAnswerRequest(r, args)` | completed under the same opid: confirmed; lost; or rejected `seat-closed`, `seat-already-answered`, `seat-already-late`; when the rebuild's now is past the deadline the act lands in `late` with its kind, never as answered or closed |
| seat-close | qid, reason, at | `seatCloseRequest(r, args)` | as seat-answer, kind withdrawn when late |
| seat-expire (the tick's and the wait's expiration) | qid, at | none needed | abandoned with detail `the next tick re-expires` |

When the commit IS present on the tip, every one of these, human acts
included, is confirmed by the existing PostconditionPresent rule
(recover.go:64-92) and nothing is repeated. The live verb and the
rebuild share one constructor, so the rebuilt opid equals the entry's
(recover.go:227). Nothing in seat text is a secret, so the journal may
hold it, unlike the channel's inbox Intent (FCG-SECRET-15).

## 8. The human channel stays what it is; the validator; the fleet order; repair

Stated, then enforced:

1. A seat record is never a channel record: ValidateChannelTree lists
   only `plans/channel/` (channel.go:140), ReadChannelTree likewise
   (channel_inbox.go:26); the inbound matcher iterates tree.Questions
   only (channel_inbox.go:75-77) and rule (b) needs a posted thread
   (channel_inbox.go:104).
2. No provider ever posts a seat record: today's poster reads local
   channel questions (question.go:195-209); the future open-work pass
   reads `plans/channel/questions/` (FCG-POST-08); no seat verb calls a
   Provider method.
3. The provider poll never confirms or matches anything a seat wrote:
   Verify (inbox.go:20-43) is reached only through InboundRecord
   (inbox.go:45); no seat verb constructs an Inbound, reads
   `channel.human.totp-secret` or calls VerifyTOTP.
4. A seat's words never carry a code and never reach a human surface:
   `seat-secret` and section 6.

ValidateSeatTree (new, internal/goal/seats.go), called from
ValidateCommit next to the channel validator (validate.go:485), is
total: every record is one of the four kinds, every kind's key set is
exactly its table, and every state admits exactly one field
combination. Because ValidateCommit receives the commit, the validator
can bind a record to its transaction:

| code | condition |
|---|---|
| seat-unknown-path | a file under plans/seats/ that is not `enabled.json`, `members/<machine>.json`, `presence/<machine>.json` or `asks/<ulid>.json` (validChannelULID, channel.go:270-280) |
| seat-json | not a JSON object; an unknown key; a missing required key; a wrong type; null where non-null is required; a time not RFC 3339 UTC at second precision; `seatSchema` not the integer 1; `chain.round` below 1 |
| seat-id-mismatch | basename does not equal `machine` or `id` |
| seat-authority | `enabled.json` whose `authority` is not `proven`, whose `by` does not start with `human:`, or whose `opid` is not a `Goal-Transaction` trailer in the validated commit's history (TrailerPresent, txn.go:379-390); a member whose `retiredBy` is non-null without `human:`, or whose retiredAt and retiredBy nullness differ |
| seat-writer | any record's `opid` that is not a `Goal-Transaction` trailer in the commit's history; `opid`'s machine segment not equal to `machine` (presence; member on refresh) or `from` (ask); `answer.opid`'s or `late.opid`'s machine segment not equal to its `by`; `answer.by` not equal to `to`; `late.by` not equal to `to` for kinds answer and not-mine or `from` for kind withdrawn; closedBy not `to` for not-mine or `from` for withdrawn; `expiredBy` not `from` or `to` |
| seat-self | `from` equals `to` |
| seat-unknown-machine | `from` or `to` has no membership record at the tip |
| seat-goal-missing | `goal` has no goal file on the tip |
| seat-state | state outside {open, answered, expired, closed}; open with answer, late, expiredAt, expiredBy or any closed key present; answered with answer null or late, expiredAt, expiredBy or any closed key present; expired with expiredAt or expiredBy absent, answer non-null, or any closed key present; closed with answer or late non-null, expiredAt or expiredBy present, or any of the four closed keys absent or empty; closedBecause outside {not-mine, withdrawn}; late.kind outside {answer, not-mine, withdrawn}; deadlineAt not later than askedAt; answer.at later than deadlineAt; late.at not later than deadlineAt; expiredAt earlier than deadlineAt; facts empty or more than four or any entry empty; question, ifSilent, fromLineage, answer.text or late.text empty; chain or lastLanding present with an empty required string; presence updatedAt not EQUAL to tickAt |
| seat-secret | a six-digit field anywhere in facts, question, ifSilent, answer.text, late.text or closedReason |

Fences on upgraded engines: `install:plans/seats/ ledger` beside
path-classes.txt:35, so a landing that touches it refuses
`ledger-path-not-goal-verb` (observe.go:583-587); the pre-commit
guard's regexp widens to `plans/(goals|channel|seats)/`; a staged
change under it is a deliberate acknowledged act under the guard's
existing acknowledgement path, never a silent one.

What keeps old checkouts out (SMA-C-11), stated as truth and not as a
fence: (a) the FLEET ORDER: slice 1 lands with no writer; every
machine pulls, and under the re-arm law landed today a pull followed
by a rebuild and `up` replaces the running engine
(up.go:427-456), so "every machine pulled" and "every machine runs the
new engine" are one step; the seat of record confirms each machine
through `metasystem supervise status --repo` (engineBuild) or the
machine's own word; only then does Wido run `seat enable`; (b) NO OLD
VERB WRITES UNDER `plans/seats/`: the only old path that can create a
file there is an agent landing a new plan record by hand, which the
guard already makes a deliberate acknowledged act and which no
workflow performs; (c) the UPGRADED VALIDATOR REFUSES a spoiled tip:
a record without a bound transaction fails `seat-writer` or
`seat-authority`, and Wido's first-commit-wins law implies refusal
rather than tolerance, because the push race decides which commit is
the ledger and two engines must read one tip the same way. What
remains a residual: a process that hand-crafts a commit with a forged
`Goal-Transaction` trailer under a ULID and machine of its choosing
can mint any seat record, including the marker, exactly as it could
forge a goal approval today (the tree is not authenticated,
accepted.go:9). Wido accepts that residual at approval of the build,
and this design names it so that he does so with eyes open.

The executable escape (SMA-C-12): `metasystem seat repair --path
<plans/seats/...> (--remove | --replace <file>) --by <name> --reason
<text> --repo <root>`, terminal-only like enable. It is one ledger
transaction under the terminal's opid, Intent verb `seat-repair`, Args
{by, path, action, body, reason}, message `seat repair <path> by
<by>`. It is exempt from the three refusals that would otherwise stop
it, by that intent: (1) its Publish passes a `Validate` that runs
ValidateCommit with a `repairing=<path>` exemption so the validator
skips that one path's rows on the CAPTURED tip and validates the
BUILT commit whole, the way the captured-tip check at txn.go:601-613
would otherwise abandon the transaction before Mutate; the exemption
is a parameter of ValidateSeatTree, never a global switch, and the
built tree must validate with no exemption or the transaction is
refused; (2) the pre-commit guard does not run, because Publish
builds with commit-tree (txn.go:236-290), not `git commit`; (3) the
landing fence does not run, because this is a goal-verb transaction
and not a landing. Its Mutate removes the path or writes the given
body; a replaced record's `opid` is this transaction's, so it binds.
`goal repair --accept-remote` (accepted.go:22-80) is the shape it
follows: human-attributed, journaled with a named intent, whole-tree
validation before anything moves; unlike that verb it pushes,
because the spoil is on the shared branch. Recovery: a human act,
rejected toward the terminal (section 7). The fleet after a repair:
every machine's next ledger-attention pass validates the repaired
tip and advances; nothing else is needed. The poison case of a plain
hand commit without a trailer is therefore no longer "a hand commit
at the terminal removes it" but `seat repair --remove`, and
SMA-F-SPOILED-REPAIR proves the path end to end.

Paths I can see and close: a seat answer reaching an authority
consumer (no goal history row); a seat ask posted or matched as a
human question (items 1 to 3); a code in seat text (item 4); seat
words on a human surface (section 6); a seat record or marker minted
by an old landing (`seat-writer`, `seat-authority`); a spoiled tip
with no way out (`seat repair`). Paths I cannot close: a seat pasting
`seat show`'s output into a chat turn, a conduct matter; a seat acting
on a peer's answer by running a goal verb it was already entitled to
run; and the forged-trailer residual above, which is the ledger's own.

## 9. Load, growth, transport; fixtures and tests

Load bound. Nine members, presence at most once per `seat.presence-min`
(60) when idle plus one write per change of chain or landing, plus the
expire transitions (one per overdue ask): accepted bound 400 seat
commits per day of about one kilobyte each. Slice 2 teaches
buildLedgerAttentionStage to skip the projection of a change whose
tree difference is confined to `plans/seats/` (one `git diff-tree
--name-only` per change instead of a ProjectAt; such a change cannot
move Ready, Pinned or Queue); SMA-F-CHURN tests the bound. Files are
overwritten in place except asks, which grow by one file each; the
accepted growth is under one megabyte per week. Rotation of
`plans/seats/asks/` belongs to goal answer-archive; the seat of record
adds it to that goal's scope by name; until it lands nothing is
removed except by `seat repair`.

Legacy transport. No seat verb calls sync-transport.sh; the transport
relay receives seat commits with the next landing's sync
(sync-transport.sh:31-35) and lags by one landing; no machine reads
from it today (transport-sync-law.md:3). SMA-F-TRANSPORT proves the
lag and the catch-up.

Fixtures: `scripts/agents/seat-fixtures.sh`, on the channel fixture
bed (bare origin, export clone, fake provider; channel-fixtures.sh:21-54)
plus `export-b` enrolled as `fixture-b` with its own lineage and
`export-old` built from the commit before slice 1; machine A is
`fixture-machine`. Human verbs run under the fixture's terminal
classification. Every assertion reads origin/main, a verb's stdout
and exit code, a component record, the health line, the hook's
rendered message, or `steward digest-pending`.

- SMA-F-ENABLE: before `seat enable`, A's tick records component
  `seat-presence` as `seat-not-enabled` and `seat ask` is refused;
  after `seat enable --by fixture-human`, origin/main holds the marker
  with `authority: proven`, `by: human:fixture-human` and an opid that
  is a Goal-Transaction trailer; both proceed.
- SMA-F-FORGED-MARKER (SMA-C-19): a plain git commit (no trailer)
  writing a well-formed `enabled.json` with `by: human:forged` and
  `authority: proven` is refused by A's and B's next ledger-attention
  pass with `seat-authority`; the writers stay refused
  `seat-not-enabled` because ValidateCommit fails before any Mutate.
- SMA-F-SPOILED-REPAIR (SMA-C-12, SMA-C-19): with `plans/seats/junk.json`
  committed plainly from `export-old`, both upgraded machines refuse
  the tip with `seat-unknown-path`; `seat repair --path
  plans/seats/junk.json --remove --by fixture-human --reason ...` on A
  publishes; origin/main no longer holds the file; A's and B's next
  pass advance; `goal recover` on A after a FAIL_AT during the repair
  reports the human-act rejection naming `seat repair`.
- SMA-F-MIXED: `export-old` fetches a tip carrying every record kind
  and lands a goal verb on it; a landing from `export-old` that
  creates a seat record is accepted by the old engine and refused by
  the upgraded ones by name (then repaired as above).
- SMA-F-MEMBER: `up` on A writes membership; B's `seat fleet` prints A
  `unreachable, no presence yet`; a claim-only machine prints under
  `unknown to the ledger`.
- SMA-F-RETIRE-THEN-UP (SMA-C-16, SMA-C-19): `seat retire --machine
  fixture-machine --by fixture-human`; A's next `up` records
  `component=seat-member outcome=seat-retired` and origin/main's
  record keeps retiredAt and retiredBy; `seat ask --to fixture-machine`
  is refused `seat-retired`; `seat unretire` clears both and the next
  `up` refreshes normally.
- SMA-F-RECOVER-ENABLE and -RETIRE (SMA-C-14): FAIL_AT after the push
  with the outcome unknown; `goal recover` reports the human-act
  rejection naming the verb when the commit is absent, and `confirmed
  on the canonical tip` when it landed; the record is never minted
  twice.
- SMA-F-PRESENCE: A ticks; presence lands with updatedAt equal to
  tickAt; a hand-committed presence with updatedAt one second later is
  refused `seat-state` (SMA-C-18); a second tick within
  seat.presence-min with nothing changed adds no commit.
- SMA-F-LINEAGE (SMA-C-17): `steward arm` before any `up` on a fresh
  clone writes runner.json with `armedLineage: no-lease` and presence
  shows it; after `up` and a re-arm the value is the session's
  lineage; a lease succession without re-arm leaves it unchanged; a
  runner started without the flag shows `unknown`.
- SMA-F-UNKNOWN-VS-UNREACHABLE: `--to nobody` is refused
  `seat-unknown-machine`; an aged presence makes the ask proceed with
  the unreachable warning.
- SMA-F-ASK-ANSWER: A asks B; B's tick with seat.answer-min=0 prints
  the owed reason with the qid and no words; B's digest names the ask
  once across two ticks; B answers; A's wait exits 0 with the text; A's
  digest names the answer once.
- SMA-F-DIGEST-NO-WORDS: every text field a sentinel; no sentinel in
  either machine's health line, digest, narration line, status post or
  the Stop hook's rendered message; `seat show` prints it.
- SMA-F-DEADLINE-SECOND (SMA-C-19): with the fake clock the answer
  transaction's now equal to deadlineAt lands as answered; one second
  later it lands as late with kind answer and state expired.
- SMA-F-WAITER-ABSENT-EXPIRY (SMA-C-15, SMA-C-19): A asks with
  `--deadline 1` and runs no wait; B's tick expires it with expiredBy
  fixture-b; a second run where B never ticks and A ticks expires it
  with expiredBy fixture-machine; with neither ticking, `seat fleet`
  and B's health treat it as expired while the record still reads
  open, and the first later tick writes it.
- SMA-F-POST-DEADLINE-NOT-MINE (SMA-C-15, SMA-C-19): after the
  deadline, B's `--not-mine` exits 3 and lands in `late` with kind
  not-mine; A's `seat close` after the deadline lands in `late` with
  kind withdrawn only when `late` is still null, else
  `seat-already-late`; the record never reaches closed.
- SMA-F-TIMEOUT-THEN-LATE: A's wait exits 4; B's answer exits 3 and
  lands in `late`; A's digest names it as answered late.
- SMA-F-NOT-MINE, SMA-F-WITHDRAW (on time): exit 2 on A's wait with the
  reason; the digest names the close.
- SMA-F-ALREADY: a second answer exits 3 `seat-already-answered`.
- SMA-F-WRONG-MACHINE: `seat answer` on an ask addressed to B is
  refused `seat-not-addressee`; a plain commit writing B's presence
  under A's opid is refused `seat-writer` on the next pass.
- SMA-F-HUMAN-QUESTION-REFUSED: a channel question id is refused with
  "is a question to the human"; a seat ask with a `wants` key is
  refused `seat-json`.
- SMA-F-NO-STEWARD and SMA-F-OLD-STEWARD: as revision 2, with the
  target's expiration now written by the asker's tick.
- SMA-F-RECOVER-ASK, -ANSWER, -CLOSE, -PRESENCE, -MEMBER, -EXPIRE:
  FAIL_AT failure points; `goal recover` reports `completed from the
  stored intent: confirmed` for ask, answer and close with the record's
  opid equal to the entry's, `abandoned` for presence, member and
  expire; an answer entry recovered after the deadline lands in `late`.
- SMA-F-UNKNOWN-ALERT: an unreadable accepted ref sets ShouldAlert
  through `seat-questions=unknown` after two ticks; a failed push
  leaves the role alive and ShouldAlert false with the component error
  recorded.
- SMA-F-CHURN, SMA-F-TRANSPORT, SMA-F-STATUS-CLAUSE: as revision 2.

Go unit tests, under the floors of section 1 item 9:
internal/goal/seats_test.go: TestValidateSeatTreeIsTotal (the
state-by-field grid, including the presence equality row and the
marker's authority row), TestSeatMarkerBindsToTransactionTrailer,
TestSeatWriterRefusesUnboundOpid, TestSeatPresenceMutateWritesOnChangeOrAge,
TestSeatPresenceRefusesYoungerTip, TestSeatWritersRefuseWhenNotEnabled,
TestSeatMemberRefreshPreservesRetirement, TestSeatAskMutateRefusals,
TestSeatTransitionsAroundTheDeadline (answer at, before and after the
exact second; expire; late acts of every kind; not-mine and withdraw
on time; disjoint FROM rows), TestSeatExpireFirstCommitWins (two
clones expiring one ask), TestSeatRequestsRebuildFromIntent (ask,
answer, close rebuild to the entry's opid; member, presence, expire
abandoned; enable, retire, unretire, repair rejected toward the
terminal), TestSeatRepairValidatesBuiltTreeWithoutExemption,
TestSeatOpidMachineSegment, TestSeatSecretRefusesSixDigitsAnywhere.
internal/steward/seats_test.go: TestSeatQuestionsRoleThreeAges,
TestSeatQuestionsRoleNeverUnknownOnPresenceFailure,
TestSeatAttentionExpiresOverdueAsksBothSides, TestSeatDigestEntriesFireOnce,
TestNoSeatTextReachesHumanSurfaces, TestPresenceComposesChainFromJobRecords,
TestPresencePublishFailureNeverDegradesTick, TestStageBuilderSkipsSeatOnlyCommits,
TestRunLoopWritesArmedLineageFromFlag, TestLaunchRunnerPassesArmedLineage.
internal/lease: TestOwnerLineageReadSeamAbsentLease. internal/up:
TestUpPublishesMembershipAfterEnable, TestUpRefusesRetiredMember.
internal/channel/report_test.go: TestStatusReportOwedClauseOnlyWhenOwed.
cmd/metasystem: TestChannelWaitDelegatesToSeatWait,
TestSeatHumanVerbsRefuseAgentCallerAndRequireBy.

## 10. Build order and the box, counted in reservations

Four slices, each landing alone with its own gate.

1. Records, validator with the trailer binding, fences, reader,
   request constructors and recovery cases, AND the three human verbs
   `seat enable`, `seat retire`/`seat unretire` and `seat repair`
   (the fences land with their human escape, SMA-C-13). Gate: `go test
   ./internal/goal` under the coverage delta, path-class and
   pre-commit-guard fixtures, SMA-F-ENABLE, FORGED-MARKER,
   SPOILED-REPAIR, RECOVER-ENABLE, RECOVER-RETIRE. Two attempts, 60 to
   90 minutes. The fleet order's rebuild and enable steps follow.
2. Membership in `up`, the lineage handoff, presence in the tick with
   its component record, the stage builder's skip, `seat fleet`. Gate:
   MEMBER, RETIRE-THEN-UP, PRESENCE, LINEAGE, CHURN, TRANSPORT,
   RECOVER-PRESENCE, RECOVER-MEMBER and the steward unit tests. Two
   attempts, 75 to 100 minutes.
3. `seat ask`, `seat answer`, `seat close`, `seat wait`, `seat show`,
   the expiration owners, the `channel wait` delegation, and the
   deadline, race, recovery and refusal fixtures. Gate: those fixtures
   and the goal unit tests. Three attempts, 100 to 130 minutes.
4. The health role, the digest lines, the status clause,
   DIGEST-NO-WORDS, UNKNOWN-ALERT, STATUS-CLAUSE. Gate: health
   fixtures and the seat fixtures. Two attempts, 45 to 60 minutes.

Reservations (SMA-C-20), every one counted at the dispatcher's 120
job-minutes: build attempts, nine at the maximum above; two closing
code-review chains (after slice 2 and after slice 4), each up to
three rounds at one reservation per round, six; a conformance or
verification job per review chain, two. Maximum seventeen
reservations and 2040 reserved job-minutes, plus the one attempt and
120 minutes this design chain has used, against a goal record that
today reads one day, ten attempts, 720 job-minutes, one active job
and two review rounds (item 9). Elapsed: the slices are sequential
and each waits for its review, so three working days at the fleet's
cadence. The complete five-part tuple the seat asks Wido for, before
slice 1 is dispatched: `elapsedLimit=3d attemptLimit=20
reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=3`.
The review-round member is a raise from the record's 2 to the
configured ceiling of 3 (`metasystem.budget.review-round-max`), so
that each code-review chain has the rounds section 9's fixture list
may need; it is stated as part of the tuple, not assumed.

## 11. Non-goals

No new provider and no second bot. No change to the TOTP rule, to
FCG-COMMIT-05 or to any channel record. No authority for seats. No
archive or rotation here (goal answer-archive gains
`plans/seats/asks/` by name). No central brain. No skip verb; `seat
repair` is the one human escape. No mark on the goal file for a seat
ask. No retirement of the transport relay (goal transport-sync-law).
No code fence against old engines: none is possible, and the design
says so.

## 12. Self-grade

Confidence: medium-high that the shape is right and buildable in the
existing engine. Weakest claim: the `seat repair` exemption inside
ValidateCommit on the captured tip (section 8): the transaction
engine validates the captured tip before Mutate (txn.go:601-613) and
the design threads a one-path exemption through ValidateSeatTree for
that call only; if the implementer finds that ValidateCommit's
signature cannot carry it without touching goal validation, the
fallback is a dedicated `seat repair` transaction path that captures,
validates with the exemption, builds and pushes through BuildCommit
and the CAS push directly (txn.go:236-290, the push at 343-372), which
is more code but the same contract. Second weakest: the fake clock
SMA-F-DEADLINE-SECOND needs; the channel fixtures already stamp
synthetic times on the fake provider, and the seat verbs take
`--now` for fixtures the way goalCommandNow (cmd/metasystem/goal.go:25,
called at goalsync_verbs.go:493) supplies a command clock, but I did not verify that every seat verb
can reach that seam. Reject condition: Wido wants no human act in the
rollout, which removes enable and reopens SMA-C-02 and SMA-C-11; or
he refuses the raise, in which case this design is not built inside
the current box in pieces.

## Dispositions (round 1, job sma-crit1-20260906)

| Finding id | Disposition | Reasoning and evidence | What changed in the design |
|---|---|---|---|
| SMA-C-01 | accepted | The Stop hook carries the pending digest into the runtime's message (supervision-hook.sh:534-548, 605-607) and seat-communication.md:1-9 names digest and stop messages as human-reaching; revision 1 copied question, ifSilent, answer and close text into digest lines | Section 6: digest, health line, status post and stop message carry names, ages, deadlines, the qid and the verb only; `seat show` is the one reader of the words, `seat wait` the second; TestNoSeatTextReachesHumanSurfaces and SMA-F-DIGEST-NO-WORDS |
| SMA-C-02 | accepted | To an old engine `plans/seats/` is class `record` (path-classes.txt:30), the guard covers only goals and channel, and a landing may create a new plan record (observe.go:687-693) | Revision 2 added the enable marker and the fleet order; revision 3 (SMA-C-11) corrects the claim that this is a code fence |
| SMA-C-03 | accepted | Two consecutive unknowns set ShouldAlert (health.go:544-549); the tick is ten minutes (runner.go:54) | Presence publish failure goes to the tick's component record, never the role; the role's only unknown is an unreadable ledger |
| SMA-C-04 | accepted | The Intent must be complete (journal.go:57-66); requestForEntry rebuilds only known verbs (recover.go:209-229, 236-403) | Section 7: complete Args, shared constructors, named outcomes; extended in revision 3 (SMA-C-14) |
| SMA-C-05 | accepted | Revision 1 could not tell an idle machine from a nonexistent one | Section 2: membership written by `up`; unknown, unreachable, reachable, retired |
| SMA-C-06 | accepted | No default timeout, no durable deadline, a late answer could still bind | Section 4 and 5: deadlineAt, states, late, exits; tightened in revision 3 (SMA-C-15) |
| SMA-C-07 | accepted | The seat-state row was not total | Section 8: the total refusal table; corrected in revision 3 (SMA-C-18) |
| SMA-C-08 | accepted | ledger-attention projects every commit (ledgerattention.go:243-268); Publish pushes only the configured remote | Section 9: bound, skip, growth, rotation owner, transport lag |
| SMA-C-09 | accepted | OwnerLineage on the lease behind a private loader; lease imports steward | Section 3: public read seam and runner snapshot; completed in revision 3 (SMA-C-17) |
| SMA-C-10 | accepted | The proof matrix and estimate covered none of the named cases | Section 9 and 10: scenarios and tests named; recounted in revision 3 (SMA-C-20) |

## Dispositions (round 2, job sma-crit2-20260906)

| Finding id | Disposition | Reasoning and evidence | What changed in the design |
|---|---|---|---|
| SMA-C-11 | accepted | No engine can fence an older engine: the old guard (pre-commit-guard.sh:70-85), the old path class (path-classes.txt:30) and old landing creation (observe.go:687-693) are what they are; revision 2 called the marker a fail-closed fence and validated only shape and a `human:` prefix | Section 8 states the three things that keep old checkouts out (the fleet order under the re-arm law, up.go:427-456; no old verb writes there; the upgraded validator refuses a spoiled tip) and the forged-trailer residual Wido accepts at build approval; section 2 binds the marker to its transaction (`authority: proven`, `machine`, `opid` as a Goal-Transaction trailer, TrailerPresent txn.go:379-390), and `seat-writer` applies the same binding to every seat record; SMA-F-FORGED-MARKER and TestSeatMarkerBindsToTransactionTrailer |
| SMA-C-12 | accepted | The proposed guard refuses staged seat changes, the ledger path class refuses them in a landing (observe.go:583-587), and every transaction refuses an invalid captured tip before Mutate (txn.go:595-613); revision 2 named a hand commit that could not be made | Section 8: `seat repair --path --remove|--replace --by --reason`, terminal-only, a ledger transaction with Intent `seat-repair`, exempt by that intent from the three refusals (a one-path validation exemption on the captured tip, no guard because Publish uses commit-tree, no landing fence because it is a goal verb), modelled on `goal repair --accept-remote` (accepted.go:22-80); its recovery case in section 7; SMA-F-SPOILED-REPAIR; the fallback path named in the self-grade |
| SMA-C-13 | accepted | The classifier returns a class and no name (lease/verbs.go:243-253; steward_verbs.go:656-672); revision 2 put the enable verb in slice 2 after a fleet order that needed it after slice 1 | Section 10: `seat enable`, `seat retire`/`unretire` and `seat repair` land in slice 1; section 2: every human verb takes `--by <name>` and the engine prefixes `human:`, as `goal repair --accept-remote --by` does (goalsync_verbs.go:416-449) |
| SMA-C-14 | accepted | Revision 2's recovery table omitted enable and retire; an unrecognized verb terminalises rejected (recover.go:185-199) | Section 7: enable, retire, unretire and repair each have a complete Intent, no rebuild case, and the human-act rejection naming the terminal verb (the resume/steal pattern, recover.go:146-161); confirmed when the commit landed; SMA-F-RECOVER-ENABLE and -RETIRE; TestSeatRequestsRebuildFromIntent covers them |
| SMA-C-15 | accepted | Only a running wait wrote expiration, and not-mine or withdraw could win after the deadline | Section 5: two expiration owners (the target's tick and the asker's tick, plus the wait), first commit wins; not-mine and withdraw are on-time only, and after the deadline they are late acts recorded in `late` with a kind; FROM rows disjoint on the deadline; equality is on time; SMA-F-WAITER-ABSENT-EXPIRY, SMA-F-POST-DEADLINE-NOT-MINE, SMA-F-DEADLINE-SECOND, TestSeatTransitionsAroundTheDeadline, TestSeatExpireFirstCommitWins |
| SMA-C-16 | accepted | The refresh intent omitted retiredAt and only joinedAt was preserved | Section 2: a retired record is immutable to every writer but `seat unretire` (`up` refuses `seat-retired` and writes nothing); retiredBy recorded; SMA-F-RETIRE-THEN-UP, TestSeatMemberRefreshPreservesRetirement, TestUpRefusesRetiredMember |
| SMA-C-17 | accepted | launchRunner passes only `--repo` (runner.go:629-645) and RunLoop writes the record (runner.go:88-95); the lease may be absent at arm time on a fresh machine (fleet-join design step 5 before step 6; lease.go:76-84) | Section 3: the handoff is an argument, `--armed-lineage`, from Arm through launchRunner to `steward run` to RunLoop; the literals `no-lease` and `unknown` are named values; presence reports the arming lineage with the reason; SMA-F-LINEAGE, TestLaunchRunnerPassesArmedLineage, TestRunLoopWritesArmedLineageFromFlag, TestOwnerLineageReadSeamAbsentLease |
| SMA-C-18 | accepted | The schema said equality and the table refused only earlier-than | Section 8: `presence updatedAt not EQUAL to tickAt` is refused; section 3 says why (a future value cannot extend reachability); SMA-F-PRESENCE's refused case |
| SMA-C-19 | accepted | The matrix lacked the forged marker, guarded repair, enable and retire recovery, retire then up, waiter-absent expiry, post-deadline not-mine, and the deadline second | Section 9: SMA-F-FORGED-MARKER, SPOILED-REPAIR, RECOVER-ENABLE/-RETIRE, RETIRE-THEN-UP, WAITER-ABSENT-EXPIRY, POST-DEADLINE-NOT-MINE, DEADLINE-SECOND, each with the guard or transaction path it exercises |
| SMA-C-20 | accepted | An attempt is one admitted reservation and the tuple is complete or nothing (backlog-mechanism.md:17-30); the goal record reads ten attempts, 720 minutes, one active job, two review rounds (seat-mutual-awareness.md:11); revision 2 counted builds only | Section 10: seventeen reservations at the maximum (nine builds, six review rounds, two conformance jobs), 2040 job-minutes plus the 120 used, three working days; the complete tuple `elapsedLimit=3d attemptLimit=20 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=3`, with the review-round raise named rather than assumed |
