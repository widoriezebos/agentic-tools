# seat-mutual-awareness — design: seats see each other and ask each other (revision 5)

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

Revision 4 answers critique round 3 (job sma-crit3-20260906, five
findings SMA-C-21..25 at commit 278d8b9d, register at
records/misc/seat-mutual-awareness-critique-r3.md) under Wido's ruling
of 2026-09-06, option A: the rollout is operational, not fenced. In one
breath: the enable marker and the human-only `seat repair` verb are
gone, with their fixtures, tests and reservations; a machine is a
member when its own record exists at the tip and validates, and
nothing enables the directory; section 8 states the rollout rule in
one paragraph (the re-arm law, the validator's refusal of a spoiled
tip, the existing human acts that repair one, and the residual Wido
accepted); the deadline second has one strict boundary stated once in
section 5 and referenced from the live predicate, the validator and
the fixture (SMA-C-24); and the slices are reordered so the reader and
notification path lands before the ask writer, with the box recounted
at fifteen reservations (SMA-C-25). Revision 3's paragraph above is
kept as history; where it names the marker or the repair verb, this
revision overrides it. Every cite this revision touched
(accepted.go, pre-commit-guard.sh, goalsync_verbs.go, up.go, txn.go,
ledgerattention.go, path-classes.txt, observe.go, lease/classify.go)
was re-read at branch agent/sma-design4-20260907 on main 968af5e4;
the rest stand from revision 3.

Revision 5 answers critique round 4 (job sma-crit4c-20260907, three
findings at commit 7f8c13a6, register at
records/misc/seat-mutual-awareness-critique-r4.md), each bounded, and
touches nothing else; ruling A stands. In one breath: the rollout
rule's confirmation admits only surfaces that read the RUNNING
reader's generation, the re-arm outcome of `metasystem up` and the
`steward-runner` line of `metasystem health`, and drops `supervise
status`, whose `engineBuild` is the stamp of the command process, with
a fixture proving that rebuilt bytes alone confirm nothing (SMA-C-25);
every fixture assertion in section 9 is assigned to the slice that
owns the behaviour it asserts, so SMA-F-SPOILED-TIP loses its
presence-republish assertion to a slice-2 fixture and four other
fixtures split the same way, and section 10's gates name exactly the
fixtures and tests each slice owns (SMA-C-26); and the box asks Wido
for exactly the fifteen reservations and 1800 job-minutes the work
needs, because `goal set-budget` starts a fresh accounting revision
that counts none of the earlier reservations (SMA-C-20). Every cite
this revision touched (supervise.go, up.go, health.go, identity.go,
runner.go, steward_verbs.go, supervision-fixtures.sh, verbs.go,
budget.go, the goal record) was re-read at branch
agent/sma-design5b-20260907 on main 7887b061; the rest stand from
revision 4.

The shape in one paragraph. Every machine already shares one git
branch that the goal verbs commit to and push, with a transaction
engine that fetches the tip, rebuilds, pushes by compare-and-swap and
lets the push race decide (internal/goal/txn.go:466-516). Every
steward tick already fetches that branch, validates the new tip and
advances a local "accepted" ref (internal/steward/ledgerattention.go:518-544).
This design adds one directory on that branch, `plans/seats/`, holding
three record kinds: a membership record per machine, written at
session start; a presence record per machine, written by the steward
tick, saying what that machine has in flight; and an ask record per
seat-to-seat question, written by the asking seat and answered by the
addressed seat, each under its own machine's operation id. Nothing
enables the directory: a machine is a member because its record is
there and validates. Nothing here posts to
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
   (accepted.go:9). `goal repair --accept-remote --by <human> --root
   <checkout>` (goalsync_verbs.go:416-449) is the one human-reserved
   ledger repair that exists: clone-local, no push, journaled under
   `repair-<nonce>` with a named intent and the human's name; it
   accepts the CURRENT remote tip as the clone's accepted tree past a
   rewind refusal, and only after that tip validates whole
   (accepted.go:16-21, 57-59). So it can never accept a spoiled tip;
   it accepts a repaired one. Every goal transaction refuses an
   invalid captured tip before it mutates, with the message "the
   captured tip does not validate; repair the canonical branch
   deliberately" (txn.go:605-613), so no transaction of any kind can
   build on a spoiled tip; section 8 names the hand act that repairs
   one. The pre-commit guard classifies its caller through `lease
   classify` and treats the class `HUMAN` as sovereign for the
   wrapper-token rule (pre-commit-guard.sh:44-68; lease/classify.go:70);
   its ledger fence on `plans/(goals|channel)/` refuses every caller
   with no acknowledgement path (pre-commit-guard.sh:80-86), because
   goal hand edits go through `goal reconcile`; the only
   acknowledgement the guard has, `METASYSTEM_ALLOW_NEW_PLAN=1`, is
   for new `plans/*.md` files and runs after that fence
   (pre-commit-guard.sh:97-113). Revision 3 claimed an acknowledgement
   path for the ledger fence; there is none, and section 8 is written
   against what the guard actually does.
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
   fresh machine there may be no lease. What proves WHICH engine is
   reading (SMA-C-25, round 4): `metasystem supervise status --repo`
   fills `engineBuild` from `supervise.BuildStamp`, the build stamp of
   the command process itself, and reads only the supervision owner
   and state files (cmd/metasystem/supervise.go:109-120, 145-151), so
   rebuilt bytes at the enrolled path show the new stamp there while
   the old runner keeps ticking. The running reader's generation is
   proven by two surfaces. First, `up`'s re-arm: ordinaryBody
   (up.go:553-605) records the `accepted-engine` component with
   outcome `re-armed` and a detail naming generation, previous
   generation, engine stamp, landed commit and landing ref
   (acceptedEngine, up.go:521-533), prints it as `component=accepted-engine
   outcome=re-armed` (up.go:81) with the aggregate fact
   `re-armed="generation=<n> previous=<n-1> engine=<stamp> landed=<commit>"`
   (up.go:94-95, 502-512), and appends `engine-re-armed
   generation=<n> ...` to artifacts/agents/supervision/arming.log
   (up.go:325, 562-573); the outcome exists only after the old runner
   was stopped and the new identity minted (runner.go:576-617, Stage
   StageMinted). Second, the `steward-runner` health role
   (health.go:626-687, printed by `metasystem health --repo`,
   steward_verbs.go:44-62): it reads runner.json, proves the runner
   pid alive, reads the enrolled generation (installedEnrollment,
   health.go:1265-1282) and is alive only when the running tick's own
   evidence carries that generation with a fresh success, then appends
   the enrollment's provenance, which names the generation and the
   engine stamp (EnrollmentProvenance, runner.go:734-754). That stamp
   is read from the enrolled bytes at mint time, never from the
   process performing the enrollment (identity.go:71-73), and
   VerifyIdentity checks no bytes digest (identity.go:127-157), so a
   health line printed by rebuilt bytes still names the generation and
   stamp of the runner that is actually reading. The rearm fixture bed
   (scripts/agents/supervision-fixtures.sh:673-695 helpers; the
   `rearm-rebuild` scenario at 1000-1044) already builds a stamped
   engine, installs it at the enrolled path and asserts the re-arm
   output, identity.json, the health provenance and the arming log.
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
   activeJobLimit=1 reviewRoundLimit=2` (plans/goals/seat-mutual-awareness.md:11);
   its next-step line at revision 24 counts three reservations and 360
   of the 720 job-minutes used since re-approval, before this revision
   and its one review, which make five and 600. What a new box counts
   (SMA-C-20, round 4): the goal record at revision 26 reads
   `Claimed: ... revision=24 accountingRevision=24`
   (plans/goals/seat-mutual-awareness.md:15). `goal set-budget` on a
   claimed goal calls bindClaim with the new revision
   (internal/goal/verbs.go:753-760), and bindClaim writes a fresh
   claim record whose AccountingRevision IS that revision
   (verbs.go:252-268). The projection reads that accounting revision
   (internal/dispatch/budget.go:250-254) and skips every reservation
   whose goalRevision is older (budget.go:359-361). So after Wido's
   set-budget every attempt and minute of the new tuple is fresh
   authority, and nothing spent under revision 24 or earlier is
   counted against it.
   Coverage floors: internal/goal 80.0 and internal/steward 74.0
   (scripts/agents/coverage-ratchet.json).

## 2. The directory and membership

Location: `plans/seats/` on the ledger branch (goal.sync-branch,
default `refs/heads/main`, txn.go:45-61), three sub-paths, canonical
serialisation MarshalChannel (channel.go:111-117), keys in table
order, no unknown keys, every time RFC 3339 UTC at second precision
(parseChannelTime, channel.go:650-656, reused).

There is no enable marker and no enable act (ruling A, Wido
2026-09-06). Membership is the record itself: a machine is a member
the moment its membership record exists at the tip and validates, and
it stays one until a human retires it. Every seat record's `opid` must
be the `Goal-Transaction` trailer of a commit in the validated tip's
history (`TrailerPresent`, txn.go:375-390; section 8's `seat-writer`
row), so a record that no transaction wrote is never a legal record,
whatever wrote it.

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
seatSchema, joinedAt}. Its Mutate: refuses `seat-retired` when the
tip's record has retiredAt non-null, writing nothing (SMA-C-16: a
retired record is
immutable to every writer but `seat unretire`); writes when the path
is absent, or the tip's engine differs, or the tip's updatedAt is
older than 24 hours, carrying joinedAt from the tip; else NothingToDo
(txn.go:462); refuses `seat-transition` when the tip's updatedAt is
younger than the one it read. A publish failure is one `up` component
outcome (`component=seat-member outcome=<detail>`) and never changes
`up`'s aggregate. `metasystem seat retire --machine <m> --by <name>
--repo <root>` and `metasystem seat unretire --machine <m> --by <name>
--repo <root>` are the two human verbs of this design. They run only
from the agent-free terminal (requireHumanStewardEnrollment's
classifier, steward_verbs.go:656-672, supplies the class; `--by`
supplies the name and the engine prefixes `human:`, the way `goal
repair --accept-remote --by` does, goalsync_verbs.go:416-449) and
refuse every other caller with the same text enrollment uses; there is
no relayed-word form. Each is one transaction under `Opid(<fresh
ULID>, <machine>, "human-terminal")`, Intent verbs `seat-retire` and
`seat-unretire`, Args {machine, by, at}; retire's Mutate refuses
`seat-already-retired`, unretire's refuses `seat-not-retired`; both
preserve joinedAt and engine. Nothing under `plans/seats/` is deleted
by any Mutate; the only removal is the human hand act of section 8.

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
the one it read.

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
refuses by name before any write: `seat-self`,
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
it yet.

THE DEADLINE BOUNDARY, stated once (SMA-C-24). A time T is ON TIME
when T is not later than deadlineAt, so equality is on time; T is PAST
when T is strictly later than deadlineAt. Three places use this one
rule and none states its own: the live predicate below (every FROM row
compares the transaction's `now` this way, so `answer.at` is on time
and `expiredAt` and `late.at` are past); the validator's `seat-state`
row in section 8, which refuses `answer.at` past, `late.at` not past,
and `expiredAt` not past, so an expired record stamped at the exact
deadline second is malformed, because at that second an answer is
still lawful; and the fixture SMA-F-DEADLINE-SECOND in section 9,
which proves the answer at equality, the late answer one second after,
and the refusal of an expired record stamped at equality. Every
post-deadline transition has exactly one outcome:

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
deadline is the arbiter on every rebuild, under the one boundary
above. Verbs: `metasystem
seat answer --question <qid> --text <text>` or `--not-mine --because
<text>` (Intent verb `seat-answer`, Args {qid, outcome, text, at});
`metasystem seat close --question <qid> --because <text>` (Intent
verb `seat-close`, Args {qid, reason, at}).

Refusals by name: `seat-not-addressee`;
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
| seat-retire, seat-unretire | machine, by, at | none: human acts | rejected with `human authority cannot be recovered from journal text; rerun metasystem seat retire --machine <machine> --by <by> from the enrolled terminal` (or `seat unretire`; the resume/steal pattern, recover.go:146-161); nothing is written |
| seat-member | machine, engine, seatSchema, joinedAt | none needed | abandoned with detail `the next up republishes membership` (the slice-start pattern, recover.go:102-108) |
| seat-presence | machine, engine, body | none needed | abandoned with detail `the next tick republishes presence` |
| seat-ask | id, to, goal, facts (JSON array), question, ifSilent, askedAt, deadlineAt | `seatAskRequest(r, args)` | completed under the SAME opid (recover.go:213-229): confirmed, lost (the path carries another opid), or rejected by name |
| seat-answer | qid, outcome (answer or not-mine), text, at | `seatAnswerRequest(r, args)` | completed under the same opid: confirmed; lost; or rejected `seat-closed`, `seat-already-answered`, `seat-already-late`; when the rebuild's now is past the deadline (section 5's boundary) the act lands in `late` with its kind, never as answered or closed |
| seat-close | qid, reason, at | `seatCloseRequest(r, args)` | as seat-answer, kind withdrawn when late |
| seat-expire (the tick's and the wait's expiration) | qid, at | none needed | abandoned with detail `the next tick re-expires` |

When the commit IS present on the tip, every one of these, human acts
included, is confirmed by the existing PostconditionPresent rule
(recover.go:64-92) and nothing is repeated. The live verb and the
rebuild share one constructor, so the rebuilt opid equals the entry's
(recover.go:227). Nothing in seat text is a secret, so the journal may
hold it, unlike the channel's inbox Intent (FCG-SECRET-15).

## 8. The human channel stays what it is; the validator; the rollout rule

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
total: every record is one of the three kinds, every kind's key set is
exactly its table, and every state admits exactly one field
combination. Because ValidateCommit receives the commit, the validator
can bind a record to its transaction:

| code | condition |
|---|---|
| seat-unknown-path | a file under plans/seats/ that is not `members/<machine>.json`, `presence/<machine>.json` or `asks/<ulid>.json` (validChannelULID, channel.go:270-280) |
| seat-json | not a JSON object; an unknown key; a missing required key; a wrong type; null where non-null is required; a time not RFC 3339 UTC at second precision; `seatSchema` not the integer 1; `chain.round` below 1 |
| seat-id-mismatch | basename does not equal `machine` or `id` |
| seat-authority | a member whose `retiredBy` is non-null without the `human:` prefix, or whose retiredAt and retiredBy nullness differ |
| seat-writer | any record's `opid` that is not a `Goal-Transaction` trailer in the validated commit's history (TrailerPresent, txn.go:375-390); `opid`'s machine segment not equal to `machine` (presence; member on refresh) or `from` (ask); `answer.opid`'s or `late.opid`'s machine segment not equal to its `by`; `answer.by` not equal to `to`; `late.by` not equal to `to` for kinds answer and not-mine or `from` for kind withdrawn; closedBy not `to` for not-mine or `from` for withdrawn; `expiredBy` not `from` or `to` |
| seat-self | `from` equals `to` |
| seat-unknown-machine | `from` or `to` has no membership record at the tip |
| seat-goal-missing | `goal` has no goal file on the tip |
| seat-state | state outside {open, answered, expired, closed}; open with answer, late, expiredAt, expiredBy or any closed key present; answered with answer null or late, expiredAt, expiredBy or any closed key present; expired with expiredAt or expiredBy absent, answer non-null, or any closed key present; closed with answer or late non-null, expiredAt or expiredBy present, or any of the four closed keys absent or empty; closedBecause outside {not-mine, withdrawn}; late.kind outside {answer, not-mine, withdrawn}; deadlineAt not later than askedAt; under section 5's one boundary: answer.at past (later than deadlineAt), late.at not past (not later than deadlineAt), expiredAt not past (not later than deadlineAt, so equality is refused); facts empty or more than four or any entry empty; question, ifSilent, fromLineage, answer.text or late.text empty; chain or lastLanding present with an empty required string; presence updatedAt not EQUAL to tickAt |
| seat-secret | a six-digit field anywhere in facts, question, ifSilent, answer.text, late.text or closedReason |

Fences on upgraded engines: `install:plans/seats/ ledger` beside
path-classes.txt:33-35, so a landing that touches it refuses
`ledger-path-not-goal-verb` (observe.go:583-587); and the pre-commit
guard gains one line after its ledger fence (pre-commit-guard.sh:80-86)
that refuses a staged change under `plans/seats/` when the caller's
class is not `HUMAN`, with the message "seat records change only
through seat verbs; a human repairs one by hand from the enrolled
terminal". The existing ledger regexp is not widened: it refuses
every caller because goal hand edits have `goal reconcile` to go
through, and seat records have no reconcile, so the human hand commit
IS the repair and the guard must let it through. The class comes from
the guard's existing `lease classify` call (pre-commit-guard.sh:47-49;
`ClassHuman`, lease/classify.go:70), the same classification that
already makes a human commit sovereign for the wrapper-token rule.

THE ROLLOUT RULE (ruling A, Wido 2026-09-06, "Ok, option a"; it
replaces the enable marker and the repair verb of revisions 2 and 3
and closes SMA-C-21, C-22 and C-23 by removing what they found). The
rollout is operational, not fenced: (a) every machine pulls, rebuilds
and re-arms before any seat writer runs, and before each later slice
lands every machine already runs the one before it; under the re-arm
law a pull followed by a rebuild and `up` replaces the running engine
when the build commit is reachable from the configured landing ref
(up.go:427-456, ReArmRebuiltEngine at runner.go:240), so "every
machine pulled" and "every machine runs the new engine" are one step.
THE CONFIRMATION (SMA-C-25, round 4): the seat of record confirms a
machine only through a surface that reads the generation of the
RUNNING reader, the steward runner, and the machine is confirmed for
a slice when that surface names a generation minted from the slice's
landed commit or later. Exactly two forms are admitted, and each is
quoted into the goal record's next-step line with the machine's name
before the next slice is dispatched. Form one, the re-arm outcome of
`metasystem up`: the `component=accepted-engine outcome=re-armed`
line and its `re-armed="generation=<n> previous=<n-1> engine=<stamp>
landed=<commit>"` fact (up.go:81, 94-95, 521-533), or the same fact
as the `engine-re-armed generation=<n> ...` line of the machine's
artifacts/agents/supervision/arming.log (up.go:325, 562-573); that
outcome is written only after the previous runner was stopped and the
new identity minted (runner.go:576-617), so it proves the replacement
happened, not that bytes were built. Form two, the enrolled running
steward's generation as the health surface reads it from the enrolled
runner: the `steward-runner` line of `metasystem health --repo
<checkout>` (steward_verbs.go:44-62), alive at generation `<n>` with
the provenance `enrollment generation <n> ... (engine <stamp> ...)`
(health.go:626-687, runner.go:734-754); that role reads runner.json,
proves the pid alive, and is alive only when the running tick's own
evidence carries the enrolled generation, so it proves what is
ticking. NOT admitted: `metasystem supervise status --repo` and its
`engineBuild`, because that field is the build stamp of the command
process itself (supervise.go:109-120), so rebuilt bytes at the
enrolled path show the new stamp while an old runner keeps reading;
a machine's own word, unless it quotes one of the two forms; and any
reading of the binary on disk. The `engine` field of the presence
record (section 3) is form two seen from the ledger once slice 2 is
in, because the tick that composes it runs inside the enrolled
runner and stamps the runner's own build; it confirms slices 3 and 4
but never slice 2 itself, whose confirmation is forms one and two.
SMA-F-REBUILT-BYTES-NOT-REARMED (section 9) proves that rebuilt bytes
alone satisfy neither admitted form while satisfying the dropped
one; (b) the upgraded validator refuses a spoiled
tip, that is a tip carrying a record an upgraded engine cannot read:
ledger-attention refuses to advance (ledgerattention.go:540-542) and
every transaction refuses the captured tip before it mutates,
printing "the captured tip does not validate; repair the canonical
branch deliberately" (txn.go:605-613), so a spoil stops every seat
writer and every goal verb on every upgraded machine until a human
repairs it, exactly as revision 3 said; (c) a spoiled tip is repaired
by an existing human act at the enrolled terminal and by nothing new:
the operator fetches the ledger branch, REMOVES the offending file
with a plain hand commit and pushes it on top of the spoiled tip, as
below; the guard passes the commit because the caller classifies
`HUMAN`; the commit is a descendant, so every machine's next
ledger-attention pass validates it and advances, and the record's
owner republishes it by itself (membership at the next `up`, presence
at the next tick; an ask is gone and the asker asks again). Remove,
never rewrite: a rewritten record would need an `opid` that is a
transaction trailer, which a hand commit cannot mint, and the owner's
writer already knows the right bytes. If the operator instead
rewinds the branch to the commit before the spoil (a force-push),
each machine's pass refuses the rewind by the descent rule and the
human runs `metasystem goal repair --accept-remote --by <name> --root
<checkout>` on each machine (goalsync_verbs.go:416-449; accepted.go:16-83),
which validates the rewound tip whole and moves that clone's accepted
ref; that verb never accepts a spoiled tip itself, because it validates
before it moves. (d) The residual Wido accepted with the ruling: an
old checkout can only spoil the directory by a deliberate hand landing
of a new plan record, which no workflow performs and nobody does by
accident; and, as before, a process that hand-crafts a commit with a
forged `Goal-Transaction` trailer can mint any seat record exactly as
it could forge a goal approval today, because the tree is not
authenticated (accepted.go:9). SMA-F-SPOILED-TIP proves (b) and both
forms of (c) end to end at slice 1, with nothing but slice 1's
validator, fences and verbs; SMA-F-SPOILED-TIP-REPUBLISH proves the
owner's republication of a removed record at slice 2, when the
writers that republish exist (SMA-C-26).

```
git -C <checkout> fetch origin
git -C <checkout> checkout -B main origin/main      # goal.sync-branch, default refs/heads/main
git -C <checkout> rm plans/seats/<offending path>
git -C <checkout> commit -m "seat: remove <offending path>, spoiled tip (<why>)"
git -C <checkout> push origin main
```

Paths I can see and close: a seat answer reaching an authority
consumer (no goal history row); a seat ask posted or matched as a
human question (items 1 to 3); a code in seat text (item 4); seat
words on a human surface (section 6); a seat record minted by an old
landing (`seat-writer` refuses it, the hand act removes it). Paths I
cannot close: a seat pasting `seat show`'s output into a chat turn, a
conduct matter; a seat acting on a peer's answer by running a goal
verb it was already entitled to run; and the residual of (d), which is
the ledger's own and Wido's by ruling.

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
removed except by the human hand act of section 8.

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
rendered message, or `steward digest-pending`. Every fixture below
names in brackets the slice of section 10 that gates it, and no
assertion is gated before the slice that owns the behaviour it
asserts (SMA-C-26): a fixture whose assertions belonged to two slices
is split into two named fixtures, and where a slice-1 fixture needs a
record on the tip before any writer exists, the fixture commits the
record plainly, carrying an opid that IS a `Goal-Transaction` trailer
of an earlier goal verb from the same machine, so the validator
reaches the row under test.

- SMA-F-SPOILED-TIP [slice 1] (section 8's rollout rule, replacing
  revision 3's SPOILED-REPAIR): with `plans/seats/junk.json` committed
  plainly from `export-old`, both upgraded machines refuse the tip
  with `seat-unknown-path` in their ledger-attention pass, and a goal
  verb and `seat retire` on A both refuse with "the captured tip does
  not validate; repair the canonical branch deliberately". Form one:
  the hand act of section 8 on A under the fixture's terminal
  classification (the guard's `lease classify` returns `HUMAN`)
  removes the file and pushes on top; origin/main no longer holds it;
  A's and B's next pass advance and the seat tree validates again.
  The same staged removal from an agent-classified caller is refused
  by the guard with the seat message. Form two: the branch is
  force-pushed to the commit before the spoil; A's and B's pass refuse
  the rewind; `goal repair --accept-remote --by fixture-human --root`
  on each advances it; a run of the same verb against the still
  spoiled tip is refused by its whole-tree validation and moves
  nothing. Nothing here republishes: slice 1 has no writer that could.
- SMA-F-SPOILED-TIP-REPUBLISH [slice 2] (the owner republishes,
  section 8 (c)): run one, A's presence record on the tip is
  overwritten by a plain commit from `export-old` with an unknown
  key; both upgraded machines refuse `seat-json`; the hand act removes
  the file; A's next tick republishes presence with a fresh opid whose
  commit is on origin/main and the tree validates. Run two, the same
  with A's membership record; A's next `up` republishes it
  (`component=seat-member outcome=written`). Because the path was
  absent when `up` wrote, section 2's Mutate has no tip to carry
  joinedAt from, so the republished record's joinedAt is the
  republishing write, and the fixture asserts exactly that rather
  than the removed record's older value.
- SMA-F-REBUILT-BYTES-NOT-REARMED [slice 1] (SMA-C-25, round 4; the
  confirmation of section 8 (a)): a scenario `rearm-bytes-only` in
  scripts/agents/supervision-fixtures.sh on the rearm bed
  (prepare_rearm_repo, build_rearm_engine, install_rearm_engine,
  supervision-fixtures.sh:673-695; modelled on `rearm-rebuild` at
  1000-1044). Generation 1 is armed with engine A; engine B is built
  with the trunk stamp and installed at the enrolled path, and NO `up`
  runs. Then: `supervise status --repo` from the installed bytes
  prints `engineBuild` equal to B's stamp (the dropped form says
  "new"); artifacts/agents/steward/identity.json still reads
  generation 1 with engine A's stamp; `metasystem health --repo` from
  the same installed bytes prints the `steward-runner` role alive at
  generation 1 with A's stamp in its provenance; arming.log carries no
  `engine-re-armed generation=2` line; runner.json's pid is the
  generation-1 runner and alive. So neither admitted form names
  generation 2 while the dropped form does, and the fixture fails if
  either admitted surface ever reports B's stamp before `up`. Then
  `up` runs: `component=accepted-engine outcome=re-armed` with
  `re-armed="generation=2 previous=1 engine=<B stamp> ..."`, the
  arming log line, health at generation 2 with B's stamp, and a new
  runner pid. The fixture reads no seat record and needs nothing of
  slices 2 to 4; it gates slice 1 because the first fleet rebuild
  follows slice 1.
- SMA-F-MIXED-OLD-LANDING [slice 1]: a landing from `export-old` that
  creates a seat record is accepted by the old engine and refused by
  the upgraded ones by name (`seat-writer`, because a landing commit
  carries no transaction trailer), then repaired as in
  SMA-F-SPOILED-TIP form one.
- SMA-F-MIXED [slice 4]: `export-old` fetches a tip carrying every
  record kind, written by their real writers (membership from `up`,
  presence from the tick, an ask from `seat ask`), and lands a goal
  verb on it; the old engine's read path ignores the directory and
  the landing is accepted by every machine.
- SMA-F-MEMBER [slice 2]: `up` on A writes membership; B's `seat
  fleet` prints A `unreachable, no presence yet`; a claim-only machine
  prints under `unknown to the ledger`.
- SMA-F-RETIRE-THEN-UP [slice 2] (SMA-C-16, SMA-C-19): `seat retire
  --machine fixture-machine --by fixture-human`; A's next `up` records
  `component=seat-member outcome=seat-retired` and origin/main's
  record keeps retiredAt and retiredBy; `seat unretire` clears both
  and the next `up` refreshes normally. The refusal of an ask to a
  retired machine is asserted by SMA-F-UNKNOWN-VS-UNREACHABLE at
  slice 4, where the ask verb exists.
- SMA-F-RECOVER-RETIRE [slice 2] (SMA-C-14): FAIL_AT after the push
  with the outcome unknown, for retire and for unretire, against a
  membership record `up` wrote; `goal recover` reports the human-act
  rejection naming the verb when the commit is absent, and `confirmed
  on the canonical tip` when it landed; the record is never written
  twice. Gated at slice 2 because the record it retires is written by
  `up`, which slice 2 owns; the verbs and their recovery cases are
  slice 1's and are unit-tested there (TestSeatRequestsRebuildFromIntent).
- SMA-F-PRESENCE [slice 2]: A ticks; presence lands with updatedAt
  equal to tickAt; a hand-committed presence with updatedAt one second
  later is refused `seat-state` (SMA-C-18); a second tick within
  seat.presence-min with nothing changed adds no commit.
- SMA-F-LINEAGE [slice 2] (SMA-C-17): `steward arm` before any `up` on
  a fresh clone writes runner.json with `armedLineage: no-lease` and
  presence shows it; after `up` and a re-arm the value is the
  session's lineage; a lease succession without re-arm leaves it
  unchanged; a runner started without the flag shows `unknown`.
- SMA-F-WRONG-MACHINE-RECORD [slice 1]: a plain commit writing B's
  presence under an opid that is a trailer of A's earlier goal verb
  is refused `seat-writer` on the next pass (the machine segment
  clause), and the same record under an opid that is no trailer at
  all is refused `seat-writer` (the history clause).
- SMA-F-UNKNOWN-KEY [slice 1]: a plainly committed ask record with a
  `wants` key, otherwise valid, is refused `seat-json`; the same with
  a six-digit code in `question` is refused `seat-secret`.
- SMA-F-UNKNOWN-VS-UNREACHABLE [slice 4]: `--to nobody` is refused
  `seat-unknown-machine`; `--to <retired machine>` is refused
  `seat-retired`; an aged presence makes the ask proceed with the
  unreachable warning.
- SMA-F-ASK-ANSWER [slice 4]: A asks B; B's tick with seat.answer-min=0
  prints the owed reason with the qid and no words; B's digest names
  the ask once across two ticks; B answers; A's wait exits 0 with the
  text; A's digest names the answer once.
- SMA-F-DIGEST-NO-WORDS [slice 4]: every text field a sentinel; no
  sentinel in either machine's health line, digest, narration line,
  status post or the Stop hook's rendered message; `seat show` prints
  it.
- SMA-F-DEADLINE-SECOND [slice 4] (SMA-C-19, SMA-C-24; section 5's one
  boundary): with the fake clock the answer transaction's now equal to
  deadlineAt lands as answered; one second later it lands as late with
  kind answer and state expired; and a plainly committed expired
  record whose expiredAt equals deadlineAt is refused `seat-state` by
  A's and B's next pass, while the same record stamped one second
  later is accepted.
- SMA-F-WAITER-ABSENT-EXPIRY [slice 4] (SMA-C-15, SMA-C-19): A asks
  with `--deadline 1` and runs no wait; B's tick expires it with
  expiredBy fixture-b; a second run where B never ticks and A ticks
  expires it with expiredBy fixture-machine; with neither ticking,
  `seat fleet` and B's health treat it as expired while the record
  still reads open, and the first later tick writes it.
- SMA-F-POST-DEADLINE-NOT-MINE [slice 4] (SMA-C-15, SMA-C-19): after
  the deadline, B's `--not-mine` exits 3 and lands in `late` with kind
  not-mine; A's `seat close` after the deadline lands in `late` with
  kind withdrawn only when `late` is still null, else
  `seat-already-late`; the record never reaches closed.
- SMA-F-TIMEOUT-THEN-LATE [slice 4]: A's wait exits 4; B's answer
  exits 3 and lands in `late`; A's digest names it as answered late.
- SMA-F-NOT-MINE, SMA-F-WITHDRAW [slice 4] (on time): exit 2 on A's
  wait with the reason; the digest names the close.
- SMA-F-ALREADY [slice 4]: a second answer exits 3
  `seat-already-answered`.
- SMA-F-WRONG-MACHINE [slice 4]: `seat answer` on an ask addressed to
  B is refused `seat-not-addressee`; `seat close` from B is refused
  `seat-not-asker`.
- SMA-F-HUMAN-QUESTION-REFUSED [slice 4]: a channel question id is
  refused with "is a question to the human".
- SMA-F-NO-STEWARD and SMA-F-OLD-STEWARD [slice 4]: as revision 2,
  with the target's expiration now written by the asker's tick.
- SMA-F-RECOVER-PRESENCE, -MEMBER [slice 2]: FAIL_AT failure points;
  `goal recover` reports `abandoned` with the named detail.
- SMA-F-RECOVER-ASK, -ANSWER, -CLOSE, -EXPIRE [slice 4]: FAIL_AT
  failure points; `goal recover` reports `completed from the stored
  intent: confirmed` for ask, answer and close with the record's opid
  equal to the entry's, `abandoned` for expire; an answer entry
  recovered after the deadline lands in `late`.
- SMA-F-UNKNOWN-ALERT [slice 3]: an unreadable accepted ref sets
  ShouldAlert through `seat-questions=unknown` after two ticks; a
  failed presence push leaves the role alive and ShouldAlert false
  with the component error recorded.
- SMA-F-CHURN, SMA-F-TRANSPORT [slice 2]: as revision 2.
- SMA-F-STATUS-CLAUSE: as revision 2; its zero-owed half [slice 3]
  and its owed half [slice 4].

Go unit tests, under the floors of section 1 item 9, each gated by
the slice that lands the code it tests (SMA-C-26). Slice 1,
internal/goal/seats_test.go: TestValidateSeatTreeIsTotal (the
state-by-field grid, including the presence equality row and the
three deadline rows of section 5's boundary: answer.at at and past the
second, late.at at and past, expiredAt at and past),
TestSeatWriterRefusesUnboundOpid, TestSeatPresenceMutateWritesOnChangeOrAge,
TestSeatPresenceRefusesYoungerTip,
TestSeatMemberRefreshPreservesRetirement, TestSeatAskMutateRefusals,
TestSeatTransitionsAroundTheDeadline (answer at, before and after the
exact second; expire; late acts of every kind; not-mine and withdraw
on time; disjoint FROM rows), TestSeatExpireFirstCommitWins (two
clones expiring one ask), TestSeatRequestsRebuildFromIntent (ask,
answer, close rebuild to the entry's opid; member, presence, expire
abandoned; retire and unretire rejected toward the terminal),
TestSeatOpidMachineSegment, TestSeatSecretRefusesSixDigitsAnywhere;
cmd/metasystem: TestSeatHumanVerbsRefuseAgentCallerAndRequireBy;
scripts/agents (guard fixture): a staged removal under `plans/seats/`
passes for a `HUMAN` caller and is refused with the seat message for
an agent caller, while the `plans/(goals|channel)/` fence is unchanged.
Slice 2, internal/steward/seats_test.go:
TestPresenceComposesChainFromJobRecords,
TestPresencePublishFailureNeverDegradesTick,
TestStageBuilderSkipsSeatOnlyCommits, TestRunLoopWritesArmedLineageFromFlag,
TestLaunchRunnerPassesArmedLineage; internal/lease:
TestOwnerLineageReadSeamAbsentLease; internal/up:
TestUpPublishesMembership, TestUpRefusesRetiredMember. Slice 3,
internal/steward/seats_test.go: TestSeatQuestionsRoleThreeAges,
TestSeatQuestionsRoleNeverUnknownOnPresenceFailure,
TestSeatAttentionExpiresOverdueAsksBothSides,
TestSeatDigestEntriesFireOnce, TestNoSeatTextReachesHumanSurfaces;
internal/channel/report_test.go: TestStatusReportOwedClauseOnlyWhenOwed.
Slice 4, cmd/metasystem: TestChannelWaitDelegatesToSeatWait.

## 10. Build order and the box, counted in reservations

Four slices, each landing alone with its own gate, in an order that
makes every member able to hear before any machine can ask (SMA-C-25):
the reader and notification path (slice 3) lands before the ask writer
(slice 4), and under section 8's rollout rule slice 4 is not landed
until every machine runs slice 3. A machine can therefore never ask a
member that cannot hear: every member of the ledger at the moment the
first `seat ask` exists runs RunSeatAttention, the `seat-questions`
role, the digest lines and the status clause.

Each slice's gate names exactly the fixtures and tests tagged with
its number in section 9 (SMA-C-26); a gate never names an assertion
whose behaviour a later slice owns, and the slice-1 fixtures need no
seat writer, only the validator, the fences, the human verbs'
refusal paths and plain commits.

1. Records (three kinds), the validator with the trailer binding and
   the one deadline boundary, the two fences (path class and the
   guard's seat line), the request constructors and recovery cases,
   and the two human verbs `seat retire`/`seat unretire`. Gate: `go
   test ./internal/goal` under the coverage delta with the slice-1
   unit tests of section 9, TestSeatHumanVerbsRefuseAgentCallerAndRequireBy,
   the path-class and pre-commit-guard fixtures, SMA-F-SPOILED-TIP,
   SMA-F-REBUILT-BYTES-NOT-REARMED, SMA-F-MIXED-OLD-LANDING,
   SMA-F-WRONG-MACHINE-RECORD, SMA-F-UNKNOWN-KEY. Two attempts, 45 to
   75 minutes. The rollout rule's fleet rebuild follows, as after
   every slice, confirmed per machine by section 8 (a)'s two forms.
2. Membership in `up`, the lineage handoff, presence in the tick with
   its component record, the stage builder's skip, `seat fleet`. Gate:
   the slice-2 unit tests of section 9, SMA-F-SPOILED-TIP-REPUBLISH,
   MEMBER, RETIRE-THEN-UP, RECOVER-RETIRE, PRESENCE, LINEAGE, CHURN,
   TRANSPORT, RECOVER-PRESENCE, RECOVER-MEMBER. Two attempts, 75 to
   100 minutes.
3. The reader and notification path: RunSeatAttention with the two
   expiration owners' tick side, the `seat-questions` health role, the
   digest lines, the status clause and `seat show`. No ask verb yet;
   the unit tests publish asks through slice 1's request constructors
   in their fixture beds. Gate: the slice-3 unit tests of section 9
   (TestSeatQuestionsRoleThreeAges,
   TestSeatQuestionsRoleNeverUnknownOnPresenceFailure,
   TestSeatAttentionExpiresOverdueAsksBothSides,
   TestSeatDigestEntriesFireOnce, TestNoSeatTextReachesHumanSurfaces,
   TestStatusReportOwedClauseOnlyWhenOwed), SMA-F-UNKNOWN-ALERT and
   the zero-owed half of SMA-F-STATUS-CLAUSE. Two attempts, 45 to 75
   minutes.
4. `seat ask`, `seat answer`, `seat close`, `seat wait`, the wait's
   expiration, the `channel wait` delegation, and every end-to-end ask
   fixture: MIXED, UNKNOWN-VS-UNREACHABLE, ASK-ANSWER, DIGEST-NO-WORDS,
   DEADLINE-SECOND, WAITER-ABSENT-EXPIRY, POST-DEADLINE-NOT-MINE,
   TIMEOUT-THEN-LATE, NOT-MINE, WITHDRAW, ALREADY, WRONG-MACHINE,
   HUMAN-QUESTION-REFUSED, NO-STEWARD, OLD-STEWARD, RECOVER-ASK,
   -ANSWER, -CLOSE, -EXPIRE, and the owed half of STATUS-CLAUSE. Gate:
   those fixtures, TestChannelWaitDelegatesToSeatWait and the goal
   unit tests. Three attempts, 100 to 130 minutes.

Reservations (SMA-C-20, reopened in round 4 and answered here), every
one counted at the dispatcher's 120 job-minutes and every one mapped
to what it builds:

| reservations | what each builds or reviews |
|---|---|
| 2 | slice 1 build attempts (the second only if the first is refused or fails its gate) |
| 2 | slice 2 build attempts, likewise |
| 2 | slice 3 build attempts, likewise |
| 3 | slice 4 build attempts, likewise |
| 2 | the code-review chain after slice 2, one reservation per round at the record's two rounds |
| 2 | the code-review chain after slice 4, likewise |
| 1 | the conformance and verification job that closes the slice-2 review chain |
| 1 | the conformance and verification job that closes the slice-4 review chain |

Fifteen reservations and 1800 reserved job-minutes in total, and the
box asked for is exactly that. Revision 4 asked for twenty and 2400
by adding the five reservations spent on the design and its reviews
under the current claim; that was wrong in the engine's terms, because
`goal set-budget` on a claimed goal rebinds the claim with a fresh
accounting revision (verbs.go:753-760, bindClaim at 252-268) and the
projection skips every reservation recorded under an older revision
(budget.go:250-254, 359-361), so nothing spent before Wido's approval
counts against the new tuple and every attempt in it is fresh
authority (section 1 item 9). Five unnamed attempts would have been
600 job-minutes nobody had a use for. The raise of the review-round
member to three is not asked; a chain that needs a third round is a
recorded budget exception, not a plan. Elapsed: the slices are
sequential, each waits for its fleet rebuild and two for their
review, so three working days at the fleet's cadence. One active job:
the slices are sequential and each review chain runs alone. The
complete five-part tuple the seat asks Wido for, before slice 1 is
dispatched: `elapsedLimit=3d attemptLimit=15
reservedJobMinutesLimit=1800 activeJobLimit=1 reviewRoundLimit=2`.

## 11. Non-goals

No new provider and no second bot. No change to the TOTP rule, to
FCG-COMMIT-05 or to any channel record. No authority for seats. No
archive or rotation here (goal answer-archive gains
`plans/seats/asks/` by name). No central brain. No skip verb and no
repair verb: the human hand act of section 8 is the one escape. No
enable marker and no enable act: nothing enables the directory. No
mark on the goal file for a seat ask. No retirement of the transport
relay (goal transport-sync-law). No code fence against old engines:
none is possible, and the design says so; the rollout is operational
by Wido's ruling.

## 12. Self-grade

Confidence: high that the shape is right and buildable in the existing
engine, higher than revision 3 because the two pieces it graded
weakest, the validation exemption and the repair transaction, no
longer exist; revision 5 changed no mechanism, only what counts as
confirmation, which slice gates which proof, and what box is asked.
What revision 5 grades weakest in its own folds: the confirmation's
form two rests on two traced facts, that VerifyIdentity checks no
bytes digest (identity.go:127-157) so `metasystem health` invoked
from rebuilt bytes still reads the enrolled generation, and that the
running tick's evidence carries that generation (health.go:668-686);
if the build finds a pin or digest check on the health path that
this reading missed, `health` from rebuilt bytes reports the role
unknown instead of generation 1, and SMA-F-REBUILT-BYTES-NOT-REARMED
still holds because no admitted surface names generation 2, but the
fixture's exact assertion text changes and the builder must say so
rather than weaken it. Second, revision 4 admitted "the machine's own
word" as a confirmation and revision 5 drops it; the seat of record
now has to collect one of two quoted lines per machine per slice,
which is more operator work than before and is the cost of C-25. On
the slice assignment, the one judgment call: SMA-F-RECOVER-RETIRE
moved from slice 1 to slice 2 because the record it retires is
written by `up`, which slice 2 owns, even though the verb itself is
slice 1's; the verb's recovery case stays proven at slice 1 by
TestSeatRequestsRebuildFromIntent. On the box, the reservation table
counts every attempt at the ceiling; a slice that lands on its first
attempt leaves its second unspent, and the tuple is a ceiling, not a
forecast. Weakest claim carried from revision 4: the pre-commit
guard's seat line (section 8). Revision 3 said the human's hand
commit passes "under the
guard's existing acknowledgement path"; there is none for the ledger
fence (pre-commit-guard.sh:80-86 exits without one), so this revision
adds one guard line that lets the `HUMAN` class through and refuses
agents. That is the one mechanism this revision chose that ruling A
did not name; it is neither a verb nor a marker, it reuses the guard's
existing classification, and without it the hand act the ruling names
would need `git commit --no-verify`, which this design will not write
down as the procedure. If Wido prefers the hook bypass to a guard
change, the line is dropped and the procedure block says so. Second
weakest: whether the seat fixture bed can run a hand commit under the
terminal classification the human verbs already use (SMA-F-SPOILED-TIP
form one); `lease classify` grants `HUMAN` to fixtures
(lease/classify.go:372-383, FixtureGranted), which is what the bed
relies on. Third: the fake clock SMA-F-DEADLINE-SECOND needs; the seat
verbs take `--now` for fixtures the way goalCommandNow
(cmd/metasystem/goal.go:25, called at goalsync_verbs.go:493) supplies
a command clock, but I did not verify that every seat verb can reach
that seam. One reading this revision made and names: the brief's
words "membership is the presence record itself" are read as "the
record a machine writes for itself is its membership, nothing enables
it", keeping the membership record of section 2 (written by `up`,
retired by a human) distinct from the presence record of section 3
(written by the tick), because collapsing them would reopen SMA-C-05
and SMA-C-16, which the brief says are closed. Reject condition: Wido
wants a mechanical fence after all, which reverses ruling A and
returns the design to revision 3's open ladder; or he refuses the
tuple, in which case this design is not built inside the current box
in pieces.

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
| SMA-C-20 | accepted | An attempt is one admitted reservation and the tuple is complete or nothing (backlog-mechanism.md:17-30); the goal record reads ten attempts, 720 minutes, one active job, two review rounds (seat-mutual-awareness.md:11); revision 2 counted builds only | Section 10: seventeen reservations at the maximum (nine builds, six review rounds, two conformance jobs), 2040 job-minutes plus the 120 used, three working days; the complete tuple `elapsedLimit=3d attemptLimit=20 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=3`, with the review-round raise named rather than assumed (recounted to fifteen in revision 4) |

## Dispositions (round 3, job sma-crit3-20260906; Wido's ruling A of 2026-09-06)

| Finding id | Disposition | Reasoning and evidence | What changed in the design |
|---|---|---|---|
| SMA-C-21 | resolved by ruling A (removed) | The marker's trailer check was a content-shape check: TrailerPresent finds an opid anywhere in history (txn.go:375-390), so a reused or typed trailer could mint or alter a marker upgraded writers accept. Wido ruled the rollout operational; there is no marker to forge | Section 2: the enable marker, `seat enable` and `seat-not-enabled` are gone from every section; membership is the record itself. Section 8: `seat-authority` keeps only its member row; the rollout rule paragraph replaces the fence. Section 9: SMA-F-ENABLE, SMA-F-FORGED-MARKER, TestSeatMarkerBindsToTransactionTrailer, TestSeatWritersRefuseWhenNotEnabled removed; TestUpPublishesMembershipAfterEnable renamed |
| SMA-C-22 | resolved by ruling A (removed) | The captured-tip validation is hard-coded before Mutate (txn.go:605-613) and PublishRequest.Validate sees only the built commit, so the exemption could not travel and the fallback bypassed the engine's gates. There is no repair transaction to route | Section 8: `seat repair` is gone; a spoiled tip is repaired by an existing human act named in the rollout rule paragraph (a hand removal commit at the enrolled terminal, or `goal repair --accept-remote --by` after a rewind, goalsync_verbs.go:416-449, accepted.go:16-83). Section 7: the seat-repair row removed. Section 9: SMA-F-SPOILED-REPAIR replaced by SMA-F-SPOILED-TIP; TestSeatRepairValidatesBuiltTreeWithoutExemption removed. Section 1 item 5 corrects revision 3's claim of a guard acknowledgement path and section 8 adds the guard's `HUMAN`-class seat line |
| SMA-C-23 | resolved by ruling A (removed) | The replace form's opid named the terminal's machine while the validator requires the record owner's; no repair verb exists now, and the hand act removes and never rewrites | Section 8: "remove, never rewrite"; every record kind's owner republishes it (membership at the next `up`, presence at the next tick, an ask is asked again). Section 2: nothing under `plans/seats/` is deleted by any Mutate |
| SMA-C-24 | accepted | The transition table made equality on time while the validator refused only expiredAt earlier than deadlineAt, so an expired record stamped at the deadline second was accepted and blocked a lawful answer | Section 5: the deadline boundary stated once (on time is not later than deadlineAt; past is strictly later) and referenced from the live predicate, the validator's `seat-state` row (answer.at past, late.at not past, expiredAt not past are refused) and SMA-F-DEADLINE-SECOND, which now also rejects an expired record stamped at equality; TestValidateSeatTreeIsTotal gains the three deadline rows |
| SMA-C-25 | accepted | Revision 3 exposed `seat ask` in slice 3 and landed RunSeatAttention, the health role, the digest and the status clause in slice 4, so a slice-3 machine could ask a slice-2 member that could not hear | Section 10: the reader and notification path is slice 3 and the ask writer is slice 4, and the rollout rule (section 8) lands no slice until every machine runs the one before it, so every member can hear before the first ask exists; slice 3's unit tests publish asks through slice 1's request constructors; the box recounted at fifteen reservations and the tuple restated with the record's two review rounds |

## Dispositions (round 4, job sma-crit4c-20260907)

| Finding id | Disposition | Reasoning and evidence | What changed in the design |
|---|---|---|---|
| SMA-C-25 (reopened) | accepted | `supervise status` fills `engineBuild` from the command process's own build stamp and reads only the supervision owner and state files (cmd/metasystem/supervise.go:109-120, 145-151), so rebuilt bytes at the enrolled path show the new stamp while an old runner keeps reading; the state that proves the running reader is `up`'s re-arm outcome, written only after the previous runner was stopped and the new identity minted (up.go:521-533, 553-605; runner.go:576-617), and the `steward-runner` health role, alive only when the running tick's evidence carries the enrolled generation (health.go:626-687) | Section 8 (a): the confirmation admits exactly two forms, the re-arm outcome of `metasystem up` (its `component=accepted-engine outcome=re-armed` line, its `re-armed="generation=..."` fact or the arming-log line) and the `steward-runner` line of `metasystem health` with its enrollment provenance; `supervise status`, a machine's bare word and any reading of the bytes on disk are named as not admitted; the presence record's `engine` field is form two seen from the ledger and confirms only slices 3 and 4. Section 1 item 7 traces both surfaces. Section 9: SMA-F-REBUILT-BYTES-NOT-REARMED, a `rearm-bytes-only` scenario on the supervision rearm bed (supervision-fixtures.sh:673-695, 1000-1044), proves that installed rebuilt bytes without `up` satisfy the dropped form and neither admitted one, and that `up` then satisfies both; it gates slice 1 |
| SMA-C-26 | accepted | Section 10 gated all of SMA-F-SPOILED-TIP at slice 1 while its last assertion, the tick republishing presence, needs the presence writer slice 2 owns; the same audit found four more assertions gated before their owner slice: SMA-F-MIXED's "every record kind" (writers land through slice 4), SMA-F-RETIRE-THEN-UP's ask refusal (the ask verb is slice 4), SMA-F-RECOVER-RETIRE's record to retire (written by `up`, slice 2), SMA-F-WRONG-MACHINE's and SMA-F-HUMAN-QUESTION-REFUSED's plain-commit halves (validator only, slice 1, but listed under slice 4 where they also passed) | Section 9: every fixture carries its gating slice in brackets; SMA-F-SPOILED-TIP keeps the slice-1 assertions (refusal by name, the hand act, the tree validating again, the guard's refusal of an agent caller, the rewind form) and says plainly that nothing republishes at slice 1; SMA-F-SPOILED-TIP-REPUBLISH (slice 2) proves the tick and `up` republish a removed record; SMA-F-MIXED splits into MIXED-OLD-LANDING (slice 1) and MIXED (slice 4); the retired-target ask refusal moves into SMA-F-UNKNOWN-VS-UNREACHABLE (slice 4); SMA-F-RECOVER-RETIRE moves to slice 2; SMA-F-WRONG-MACHINE-RECORD and SMA-F-UNKNOWN-KEY are the plain-commit validator fixtures at slice 1; the unit tests are listed per slice. Section 10: each gate names exactly its slice's fixtures and tests |
| SMA-C-20 (reopened) | accepted | Revision 4 totalled fifteen reservations and 1800 job-minutes and asked for twenty and 2400 by adding five reservations already spent under the current claim; `goal set-budget` on a claimed goal calls bindClaim with the new revision (verbs.go:753-760), bindClaim sets AccountingRevision to that revision (verbs.go:252-268), and the projection skips every reservation under an older goalRevision (budget.go:250-254, 359-361), so the five would have been fresh, unnamed authority | Section 10: a table maps all fifteen reservations to the slice build attempt, review round or conformance job each buys; the tuple asked is exactly `elapsedLimit=3d attemptLimit=15 reservedJobMinutesLimit=1800 activeJobLimit=1 reviewRoundLimit=2`, with the elapsed and active-job members justified in words; section 1 item 9 traces the accounting-revision rule |
