# seat-mutual-awareness — design: seats see each other (revision 7, presence first)

- Kind: design
- Id: 01M3A3TATJ3NB3JC43W342M8GC
- Status: accepted
- Goals: seat-mutual-awareness

Goal: plans/goals/seat-mutual-awareness.md. Wido's order (2026-08-31, verbatim
on the goal record): seats must be aware of each other and ask each other
questions directly, without the human as relay; DONE means a seat can
discover what other seats have in flight and put a question to them as the
normal, mechanized path.

Revision 7, 2026-09-24, author Fable (interface seat), on Wido's decisions in
chat the same day, each restated in section 2. Every cite below was re-read
at main 220afab65. Revisions 1 to 5 (m1d, 2026-09-06 and 07) and their four
critique rounds are history: their registers are
records/misc/seat-mutual-awareness-critique-r1.md to -r4.md, and revision 5
is kept whole in git at commit 89d3674aa for anyone who needs the ask
machinery it designed. Revision 6 (the rollout confirmation rule) was written
in the m1d scratchpad and lost with it; what it folded is restated in section
7 from the goal record's own words.

What changed in one breath: presence comes first and alone; it rides one git
ref per machine instead of commits on the ledger branch; membership records
and seat-to-seat asks are deferred until a case needs them, with the ask's
future shape noted; presence exists to feed liveness, the fleet panel and
the flagging of claims whose holder has gone silent; the interface is the
human's surface for it; the test plan is rewritten under the testing
contract that landed with the test overhaul; and the rollout confirmation
collapses to reading the presence record itself.

## 1. What exists and binds (traced at 220afab65)

1. The ledger endpoint. `goal.ResolveEndpoint` reads `goal.sync-remote`
   (default `origin`; `local` is single-machine mode, `LocalMode`,
   internal/goal/txn.go:58-76) and the ledger branch `e.Branch`
   (`goal.sync-branch`, default `refs/heads/main`, and it must be fully
   qualified, txn.go:64-72). A transaction fetches with
   `git fetch --no-tags --refmap= <remote> +<branch>:<per-op ref>` into a
   per-operation ref, precisely so that concurrent operations never contend
   on one shared ref's lock (txn.go:132-146), and pushes by compare-and-swap,
   `push <remote> --force-with-lease=<branch>:<tip> <commit>:<branch>`
   (txn.go:383-385). The accepted ref is `refs/metasystem/goals/accepted`
   (txn.go:51) and `goal.AcceptedLedgerTip(root)` resolves it
   (internal/goal/attention.go:550).
2. The steward tick. `RunTick` (internal/steward/tick.go:124) runs named
   component attempts through `beginComponentAttempt(repoRoot, name,
   generation, ref, now)`: `steward-tick` (tick.go:146), `ledger-attention`
   (tick.go:194, running `RunLedgerAttention`, ledgerattention.go:444, which
   fetches the branch, validates the tip and advances the accepted ref) and
   `narrator` (tick.go:305). A component's outcome is recorded under
   artifacts/agents/steward/components and never degrades the tick. The
   tick queues human notifications with `QueueNotification` (tick.go:259).
   The tick runs inside the enrolled runner, ten minutes apart by default.
3. Health. `healthRoleOrder` is a closed list (internal/steward/health.go:68-89,
   `KnownHealthRole`), evaluated by `evaluateHealthRoles` (health.go:410);
   a verdict is `RoleVerdict{Role, Status, Reason, Remedy, ...,
   ConsecutiveFailures, NoAutomaticRemedy}` (health.go:103-113). `metasystem
   health --repo` prints it (`runStewardHealth`, cmd/metasystem/steward_verbs.go:95;
   registered at cmd/metasystem/main.go:640).
4. Identity. `InstallIdentity{Generation, InstallPath, InstallDigest,
   MintedAt, ...}` (internal/steward/identity.go:44-58);
   `EnrollmentProvenance` names the generation and the engine build of the
   enrolled runner (internal/steward/runner.go:864-880).
   `supervise.BuildStamp` (internal/supervise/disk.go:144) is the build stamp
   of the COMMAND process, so a record that must name the running reader's
   engine is composed inside the tick, never by a command.
5. Claims. A goal's `Claimed:` line parses `machine`, `lineage`, `at` and
   the revision fields (internal/goal/file.go:1356-1360). A claim holder's
   liveness is judged today from its process, `recordedProcessAlive` on pid
   and start time (internal/goal/project.go:455-457), which only a process
   on the same host can answer. The health role `RoleClaimedGoalDelivery`
   and the landed goal arming-dead-owner-takeover rest on that local
   evidence.
6. Jobs and landings. Delegate job records live under
   artifacts/agents/jobs (internal/dispatch/budget.go:369, close.go:16;
   chain membership `chainMembers`, claim.go:817). Landing receipts are
   lines of memory/receipts.log, parsed by internal/landing/receiptline.go
   (`ObserveReceiptLine`, :63; `locateReceiptLedger`, :172). Neither is read
   by this slice: the record carries no landing.
7. The interface. Its snapshot loop runs a `Fetch func(goal.Endpoint)
   (goal.AdvanceResult, error)` (internal/ui/snapshot/loop.go:108) every
   five seconds while a browser reads and every thirty when none does
   (loop.go:19-20); any error from it blanks the tip and backs the loop
   off toward five minutes (loop.go:27, 205-211). It then reads the ledger
   at the accepted tip
   (`goal.AcceptedLedgerTip`, `goal.ReadValidatedTree`, `goal.SyncModeGate`,
   snapshot.go). The fleet panel (g1-s42, its own design) reads presence
   beside this loop, never through it (section 4).
8. Machine identity. A checkout's ledger nickname is `git config
   metasystem.goal.machine`; hostnames are never published, by the engine's
   own refusal text. A checkout without a nickname performs no ledger act.
9. The testing contract. Every package with tests has a `TestMain` using
   `testenv.Main` (`TestEveryPackageUsesSharedMain`); no test waits on the
   wall clock (`TestNoTestWaitsOnWallTime`, allowance file
   internal/testenv/testdata/walltime-allowance.tsv); every file operation
   in a test is classified in internal/testenv/testdata/isolation-inventory.tsv
   (fixture-data, host-readonly, fixture-time, process-private, ...);
   each package's SERIAL test count is pinned in testing-parallel-ratchet.json
   (`metasystem audit parallel-ratchet -update`); the direction of goal
   finish-test-repairs-and-integrate is that behaviour tests use stubs and
   real git appears only in narrow, explained adapter integration tests.
   Unit groups in testing.json name packages and tests (the `events-standard`
   row is the shape); surfaces name paths and the groups they select.
10. What is not there. No `seat` verb family, no `plans/seats/`, no presence
    of any kind, no cross-host liveness. The channel gateway's ledger inbox
    pieces landed without callers and the goal is parked; nothing here
    depends on them.

## 2. Decisions (Wido, 2026-09-24, in chat with the interface seat)

- D1. Presence rides git, on one ref per machine, never as commits on the
  ledger branch. One mechanism for seats on the same host and across hosts;
  no sockets, no second transport. If same-host latency ever hurts, a
  nudge that only says "fetch now" may be added; it never carries content.
- D2. Presence comes first and alone. Membership records (join, retire) wait
  until a case needs them. Seat-to-seat asks wait likewise; when they come,
  they are a mailbox on the presence record (section 9), not records on
  the branch.
- D3. Presence exists to feed liveness: the fleet panel's presence column,
  and the flagging of claims whose holder has gone silent. Presence never
  changes who holds what; steal and resume stay human acts.
- D4. The interface is the human's surface for the fleet. The health line
  stays the seat's own surface. No seat-authored free text exists in this
  slice, so nothing can reach a human surface.
- D5. Human verbs of this design, when membership brings them, accept the
  signed-in browser session as human proof (`humanauthority.SignedInSessionProof`,
  internal/humanauthority/authority.go:280-310) exactly as the other human
  acts do, beside the enrolled terminal.
- D6. Where records live: with presence on refs, nothing of this slice is
  written under `plans/`. Whether later membership records sit at
  `plans/seats/` beside `plans/channel/` is decided when they come.

## 3. The presence record

Ref: `refs/metasystem/presence/<machine>` at the ledger remote. Object: a
parentless commit whose tree holds exactly one file, `presence.json`, with
the message `seat presence <machine> <tickAt>` and the machine's git
identity as author and committer. Each publish creates a fresh parentless
commit and force-updates the ref to it: `git push <remote>
+<commit>:refs/metasystem/presence/<machine>`. Nothing ever merges. The
previous commit becomes unreachable and is collected by ordinary git
maintenance on both sides: each tick leaves three unreachable objects per
machine at the remote and in every fetching clone until `gc --auto` runs,
bounded churn rather than history. In `LocalMode` the ref is updated
locally with `git update-ref` and nothing is pushed, so a single-machine
seat still sees itself.

| key | type | required | meaning |
|---|---|---|---|
| presenceSchema | integer | yes | 1 for this slice |
| machine | string | yes | equals the ref's last segment |
| repoIdentity | string | yes | the enrolled installation's repo identity (InstallIdentity.RepoIdentity, identity.go:46), so two checkouts that share a nickname are told apart |
| generation | integer | yes | the enrollment generation of the running runner (InstallIdentity.Generation) |
| engine | string | yes | the running runner's build stamp, read from the runner's own identity, never from a command process |
| armedLineage | string | yes | the lineage the runner was armed under; the literal `no-lease` when there was none |
| tickSeconds | integer | yes | the writer's own cadence, `metasystem.steward.tick-seconds` as the runner read it (runner.go:98), so a reader judges staleness against the writer's clock and not its own configuration |
| chain | object or null | yes | the newest non-terminal delegate job: `{root, job, role, round, goal, startedAt}`; null when idle. Only the machine knows this, and it is the panel's "running" column |
| tickAt | time | yes | RFC 3339 UTC at second precision; the tick that composed this record |

What the record does NOT carry, on purpose: the goals the machine holds,
because the claims are on the accepted tip every reader already reads
(`Claimed:` lines, file.go:1356) and a copy on the record would be the
staler one; and the last landing, because receipts are on the branch too.
Every key is an identifier, a number, a hash or a time. There is no free
text field, so the record can carry neither a secret nor a seat's words.

The reader's rule, so that a schema addition never reads as a dead machine:
a reader accepts any `presenceSchema` of 1 or more, validates the keys it
knows, and ignores keys it does not know. It refuses only a missing or
lower schema, a missing required key, a wrong type, or a malformed time,
and a refused record reads as standing `unknown` with the reason
`malformed presence`, never as reachable.

The machine name. A nickname that publishes presence matches
`[A-Za-z0-9._-]+`; the engine's own `ValidateMachineNickname` refuses only
whitespace (internal/goal/actor.go:34-44), and a slash or a dot pair would
break "equals the ref's last segment" and the notification file names of
section 5, so the publisher refuses any other name with
`SEAT_MACHINE_NICKNAME_INVALID` and publishes nothing.

The runner context, so that only the resident runner can publish. `RunLoop`
(runner.go:171) calls `RunTick` with a `TickConfig`; the manual verb
`metasystem steward tick` calls the same `RunTick` with `TickConfig{Now}`
(steward_verbs.go:356-376), and `RunTick` reads the installed generation
from disk (tick.go:140), so publishing "inside the tick" alone would let a
command process publish the rollout confirmation of section 7. Therefore
`TickConfig` gains `Runner *RunnerContext{RepoIdentity, Generation, Engine,
ArmedLineage, TickSeconds}`, filled only by `RunLoop` from the identity it
was enrolled with and from the lineage `steward arm` hands to `steward run
--repo <root> --lineage <lineage or no-lease>` (revision 5's handoff,
restored; the runner keeps it in memory, and neither `RunnerRecord` nor
`InstallIdentity` gains a field). A tick with no runner context publishes
nothing: component outcome `skipped`, reason `manual tick publishes no
presence`.

Writer and cadence. The tick composes and publishes the record as component
attempt `seat-presence`. It publishes EVERY tick, changed or not: the
timestamp is the liveness signal, and a record that only moved on change
would be indistinguishable from a dead machine. The tick is the cadence;
nothing else is configured. Three refusals to publish, each a component
outcome `skipped` with its reason: no machine nickname; an unarmed tick,
whose generation is zero (tick.go:140-145) and which must never overwrite
the armed runner's record; and a conflict, when the record fetched for this
machine names a different `repoIdentity` with a newer `tickAt`, which means
another checkout is publishing under this nickname
(`SEAT_PRESENCE_CONFLICT`, also a health reason in section 5). A failed
publish records the git error as the component's detail, leaves the
previous ref in place, and never degrades the tick.

Transport is bounded. The fetch and the push each run on an owned process
group under a deadline, the way the ledger fetch does (`CaptureTipBounded`,
internal/goal/attention.go:630, whose budget they reuse), so a hung remote
ends as a failed attempt and never as a hung tick, which holds the
repository's tick arbitration for its whole pass (tick.go:127-136). Two
failure classes are kept apart: a transport failure is the component's
outcome and the tick continues; a failure to write the component's own
evidence follows the tick's rule for every component (tick.go:194-204).

The chain field is composed by dispatch's own rules: a job qualifies while
its status is non-terminal in the canonical vocabulary (`pending-setup`,
`pending`, `running`; internal/dispatch/record.go:56-60); the newest is by
`createdAt`, ties by job id; `startedAt` is null for a record that has none
yet (a pending-setup reservation, claim.go:718) and `round` is 0 when the
record carries none; an unreadable or corrupt record makes `chain` null and
names itself in the component's detail rather than failing the publish.

The component keeps its own publication state, local, at
artifacts/agents/steward/seat-presence.json: `lastAttemptAt`,
`lastSuccessAt`, `lastOutcome` (`published`, `skipped: <reason>` or
`failed: <detail>`), `consecutiveFailures` and `detail`. Health reads it
(section 5); the component evidence record stores a digest, not this
(component_evidence.go:38), and health's own counters count dead
observations, not attempts.

Refusal codes, rowed in the register because they are emitted as errors:
`SEAT_MACHINE_NICKNAME_INVALID` (the publisher), `SEAT_PRESENCE_MALFORMED`
(the reader) and `SEAT_PRESENCE_CONFLICT` (the publisher). No nickname and
manual tick are skipped outcomes, not refusals, and a failed push is the
component's detail.

Where in the tick. The component owns both its fetch (section 4) and its
publish, and runs immediately after the ledger-attention completion record
(tick.go:194-204) and before the evidence-store reads whose failure ends
the tick degraded (tick.go:206-215, `degradedTick`), so a torn evidence
store on this machine does not make it read dead to its peers; and it
completes before `completeTickHealth` (tick.go:276), so the role of section
5 reads this tick's outcome rather than the previous one. Its fetch never
runs inside `RunLedgerAttention`, whose every error is a ledger-attention
failure (ledgerattention.go:444-450); a presence problem is reported as a
presence problem.

Cost: one commit object of a few hundred bytes and one ref update per
machine per tick; one push per tick.

## 4. Reading: the fetch, the standings and `seat fleet`

The fetch, and why every reader has a namespace of its own. The ledger's
per-operation fetch refs exist because two concurrent fetches into ONE
shared ref collide on that ref's lock (txn.go:126-133). Three readers in
one clone fetching into the same `refs/metasystem/presence/*` would collide
the same way, so:

- the tick's `seat-presence` component fetches both remote namespaces into
  the clone's canonical local copy, `refs/metasystem/presence-copy/metasystem/*`
  and `refs/metasystem/presence-copy/heads/*`, once per tick; the copy lives
  beside the publish namespace rather than in it, because in LocalMode the
  publish namespace holds this machine's own live ref;
- every presence fetch is `git fetch --no-tags --refmap= --atomic --prune
  <remote> +refs/metasystem/presence/*:<namespace>/*`: `--refmap=` keeps
  it out of the remote-tracking refs, `--atomic` makes a multi-ref update
  all or nothing so a failed fetch leaves no half-updated namespace, and
  `--prune` with that one refspec removes only namespace refs whose remote
  ref is gone, so an operator's deletion (below) reaches every reader;
- `metasystem seat fleet` reads that copy by default and reports its age;
  with `--fetch` it fetches into `refs/metasystem/presence-fetch/<ulid>/*`,
  reads, and deletes the namespace before it exits;
- the interface, in slice 2, fetches into `refs/metasystem/presence-ui/*`
  at most once a minute whatever its loop's cadence, from its own call and
  never through the ledger `Fetch` of its snapshot loop: that function
  returns one error, and a failure there blanks the ledger tip and backs
  the whole loop off toward five minutes (loop.go:27, 205-211). A presence
  fetch failure is recorded beside the panel as the presence copy's own
  age and outcome and is never returned as a ledger failure.

A presence fetch never runs inside a goal transaction's per-operation
fetch. In `LocalMode` a reader reads the local refs and neither fetches nor
prunes. A fetch that fails leaves the previously fetched refs. An old engine
never fetches these refs, because the refspec is explicit, and is never
disturbed by them.

The standings, derived at read time and never stored. Let `age` be the
reader's clock minus `tickAt`, and let the threshold be
`max(seat.presence-stale-min, 3 × tickSeconds)`, with
`seat.presence-stale-min` defaulting to 30 minutes, so a machine that ticks
every forty minutes is not read dead by a reader that expected ten. The one
inequality, stated once:

| standing | when |
|---|---|
| reachable | the record is well formed and `0 <= age <= threshold`; a record from the future by less than the threshold is reachable with `clock ahead by <d>` appended to its reason |
| unreachable | the record is well formed and `age > threshold` |
| unknown | no ref exists for a machine the ledger names in a `Claimed:` line; the record is malformed; or `tickAt` is ahead of the reader's clock by more than the threshold, reason `clock ahead by <d>`, the house pattern for clock disorder (`CLOCK_REGRESSED`, health.go:570, 678, 822) |

Retired is not a standing of this slice. An operator removes a machine that
is gone for good with one git act, `git push <remote>
:refs/metasystem/presence/<machine>`, and the machine falls to `unknown`
until membership records (section 9) make retirement a recorded human act.

The machine set a reader reports is the union of the presence refs it read
and the machines named by `Claimed:` lines at one captured accepted tip,
so a claiming machine that never published is visible as unknown rather
than invisible, and a machine whose ref was deleted and that nothing claims
is simply gone. The flag of section 5 is computed by joining those
`Claimed:` lines to each machine's standing, never from the record. When
the accepted tip cannot be read, the reader reports standings from the refs
alone, prints `claims unavailable: <reason>`, and raises no flag.

`since`, wherever a standing carries one, is the reader's first observation
of that standing, frozen in the local standings file of section 5, never
recomputed; an unknown machine with no record has no `since`. Residual, and
said so in the reason text: an old `tickAt` alone cannot tell a slow clock
from a silent machine; the reader reports what it observed.

The verb: `metasystem seat fleet [--root <checkout>] [--fetch] [--json]`.
One line per machine, this machine first:

```
m1e   reachable    2 min ago   generation 4  engine 3f9c1e2  running implementer round 2 on finish-test-repairs-and-integrate  holds finish-test-repairs-and-integrate
m1c   unreachable  6 h ago     generation 3  engine 8a01d77  idle  holds tests-parallel-and-deterministic (held by a machine unreachable since 09:40)
m0b   unknown      no presence; named by the claim on fleet-channel-gateway
presence of this machine: published 2 min ago
presence copy fetched 2 min ago by the tick
```

`--json` prints the same as one object per machine with the standing, the
age in seconds, the raw record, the joined holds and the reasons, for the
interface. A checkout with no nickname prints `this checkout has no machine
nickname and publishes no presence` as its own line and still reports the
others. Timestamps and ages come from an injected clock, so the verb's
output is testable without the wall.

## 5. Presence feeds liveness

Own publish, the health role `seat-presence`, placed after
`RoleLedgerAttention` in `healthRoleOrder` (health.go:68-89) and added to
the closed schema. The check itself decides only one thing:

- alive, reason `presence published <age> ago`, while the last successful
  publish is younger than the threshold of section 4;
- dead, reason `presence not published since <last>: <detail>`, remedy
  `check the ledger remote; the next tick republishes`, once the last
  success is older than that; a `SEAT_PRESENCE_CONFLICT` skip is dead with
  its own reason and the remedy `one checkout per nickname`;
- unknown only when the publication state of section 3 cannot be read, the
  house rule for unreadable evidence; a known publish failure is never
  unknown.

Precedence, evaluated in this order from the publication state: unreadable
state, unknown; no attempt yet since arming, alive with `first publish
pending`; last outcome skipped for no nickname, alive with `publishes no
presence: no machine nickname` (a configuration fact, not a fault); last
outcome a conflict, dead; last success older than the threshold, dead with
the last detail; otherwise alive, with the last failure's detail appended
when the newest attempt failed while an earlier success is still fresh. A
manual tick writes no publication state and changes no verdict.

The engine owns the rest, and the design bends to it rather than the other
way round: `ConsecutiveFailures` is counted by the health evaluator over
dead observations (health.go:728) and is not the check's to set; and
`hasLawfulAutomaticRemedy` is a switch on the role's name with a default of
false (health.go:745-774), so without a case a first dead observation
escalates as `NoLawfulRemedy` and opens an alert episode. The build adds
`case RoleSeatPresence: return true`, so the role follows the retry pattern
of `checkRetroDebt` (health.go:531-547) and alerts only through the
ordinary consecutive-failure path. A dead role does mark the aggregate
unhealthy (health.go:707), which is intended: a push that fails while the
ledger fetch works is a remote permission or namespace fault a human must
fix, and the ledger-attention role already says when the remote itself is
gone.

Others' silence, the transition check inside the `seat-presence` component:
after its fetch, the component compares every machine's standing with the
standing it recorded at the previous tick (artifacts/agents/steward/seat-fleet.json,
local) and queues one notification per change of standing, deduplicated by
`(machine, standing, since)`. Order matters, and it is ledger attention's
order (tick.go:259-264): every transition's notification is queued first,
then the new standings are persisted with their frozen `since`; a crash
between the two repeats a notification, which delivery already permits
(notify.go:242), and never loses one. The first read after arming persists
the standings as a baseline and notifies nothing. `QueueNotification` uses the nonce as a file
name (internal/steward/intervene.go:345), so the nonce is
`seat-presence-<machine>-<standing>-<since as unix seconds>`, which with
the nickname charset of section 3 is always a plain file name. The words:
`m1c has been unreachable since 09:40 and holds two goals: <ids>` and `m1c
is reachable again`. Names only, which is all a presence record has. This
is the human's early warning, delivered through the notification journal
the interface already shows.

Claims held by a silent machine. `seat fleet`, and the interface through it,
mark every goal whose `Claimed:` line names an unreachable or unknown
machine as `held by a machine unreachable since <t>`. That is a flag, never
an act: the goal stays claimed, `goal next` still refuses it to other
machines, and the human decides with steal or resume as today. Wiring the
flag into the backlog's own rows and into `goal next`'s printed reasons is
slice 2 (section 10); the existing local liveness of claims (section 1.5)
is not touched.

## 6. Failure, recovery and what an old engine sees

Presence writes are outside the transaction journal: no Intent, no rebuild
case, no recovery classification. A publish that fails or dies half way
leaves the previous ref; the next tick publishes again; there is nothing to
recover because the next record supersedes every earlier one. A runner that
dies stops publishing and the machine becomes unreachable after the stale
window, which is exactly the signal wanted. A malformed record, however it
arose, reads as unknown and spoils nothing: the refs are never seen by
`ValidateCommit`, never touch the ledger branch and never enter a
transaction's captured tip. An engine that predates this slice neither
fetches nor writes the refs and is not affected by their presence at the
remote.

## 7. Rollout and its confirmation

Each machine pulls, rebuilds and re-arms with `metasystem up`, whose re-arm
outcome names the new generation and engine stamp (the `accepted-engine`
component, internal/up/up.go). From then on that machine's presence record
carries the same generation and stamp, composed by the running runner
itself (section 1.4), so the seat of record confirms every machine by
reading `seat fleet`: a machine is on this slice when its record exists and
its `engine` is the slice's stamp or later. This is what revision 6 folded
as its confirmation rule, seen from the refs instead of collected by hand,
and it is the whole rollout ceremony of this slice: rebuild, re-arm, watch
the fleet line appear. A machine whose line does not appear within three
ticks has not re-armed, and its health line says why.

## 8. Authority, security, cost

A presence record authorizes nothing and no consumer acts on one except to
display and to flag. Whoever can push to the ledger remote can forge a
machine's presence, exactly as they could forge its claims today; the
remote's push permission is the boundary, unchanged. The record carries no
token, no hostname and no free text. The push uses the credentials the
ledger push already uses and one round trip per tick. Two configuration keys, read through internal/config like the steward's
other keys: `seat.presence-stale-min`, default 30; and
`seat.presence-namespace`, default `refs/metasystem/presence`, the ref
prefix presence is published under and fetched from. The default is
ordinary git: any ref under `refs/` is legal to the protocol, every
self-hosted server (SSH, a bare repository, git daemon, Gitea, Forgejo)
serves it, and it was proven on this fleet's own remote, github.com, on
2026-09-24 with the exact operations of sections 3 and 4: push of a
parentless commit, listing, fetch with `--no-tags --refmap= --atomic
--prune`, force-update without history, deletion and pruning, all nine
accepted. GitHub Enterprise Server runs the same server code. A host that
admits only branches and tags (some hosted services do) is served by
setting the key to `refs/heads/presence`: the same design under a branch
name per machine, still one writer each, with the one caveat that a
branch ruleset forbidding force pushes to all branches must exempt
`presence/*`.

The fallback is automatic, a ladder the publisher climbs by itself, so a
remote that refuses the first rung never leaves a fleet without presence
(Wido, 2026-09-24: presence must not fail on a corporate GitHub
Enterprise). The rungs, tried in order on every publish until one is
accepted, and remembered in the publication state so later ticks start
from the rung that worked:

| rung | ref | push | cost |
|---|---|---|---|
| 1 | `refs/metasystem/presence/<machine>` | force, parentless | none; proven on github.com, standard git elsewhere; already the kind of ref this remote holds (`refs/metasystem/goals/accepted` and `refs/metasystem/machines/*` are at origin today) |
| 2 | `refs/heads/presence/<machine>` | force, parentless | a branch per machine appears in the host's branch list; blocked only by a ruleset that forbids force pushes to every branch |
| 3 | `refs/heads/presence/<machine>` | fast-forward, each record a child of the last | works under any branch policy that allows pushes at all; history grows by one small commit per tick, and the publisher deletes and recreates the branch once a week where deletion is allowed, otherwise the history simply grows |

A refusal that names the ref (git's `funny refname`, a pre-receive hook, a
ruleset message) moves the publisher one rung down for that publish and
the next; a transport failure that names no ref (a timeout, a credential
fault) does not, because the rung was not the fault. `seat.presence-namespace`
pins the ladder to one NAMESPACE for an operator who knows the host, and
is otherwise unset: pinned to `refs/metasystem/presence` the publisher
never falls; pinned to `refs/heads/presence` it may still fall from the
force push of rung 2 to the fast-forward push of rung 3, because a ban on
force pushes is not a namespace question. Rung 3's weekly branch reset is
deferred: slice 1 lets rung 3's history grow, and the reset comes when a
host actually lands a fleet on rung 3. The rung in use is a fact on the health line (`presence
published 2 min ago on rung 2, a branch per machine`) and in the
publication state, and readers do not need to know it: every presence
fetch carries both remote namespaces, `refs/metasystem/presence/*` and
`refs/heads/presence/*`, into two local sub-namespaces of the reader's
own, and the reader joins them by machine, the newest `tickAt` winning, so
a machine that moved rungs is read from its live rung and its stale ref on
the other rung is ignored and, once deleted, pruned. The remote's answer to
the first publish is therefore the preflight, and an operator learns on
the first tick, never at rollout, which rung a host allows; a host that
refuses all three refuses git pushes altogether, and then the ledger does
not work either.

## 9. Deferred, with their shape fixed so they stay small

- Membership records, when a joined-at history, a human-attested retirement
  or a member-at-tip check is wanted: one record per machine on the ledger
  branch, written by `up` and retired by a human through `seat retire --by`,
  accepting the enrolled terminal or the signed-in session (D5). Revision 5
  section 2 is the specification, minus its enable marker.
- Seat-to-seat asks, on the first real case: a `asks` list on the asking
  machine's presence record (to, goal, question, ifSilent, deadlineAt, id)
  and an `answers` list on the addressee's (id, text, at), kept until the
  ask's deadline has passed. Single writer per record, no transaction, no
  expiry machinery: after the deadline the asker does what `ifSilent` says
  and records any outcome on the goal, where decisions live. The moment
  free text enters the record, revision 5's rule returns: no seat text on
  any human surface, and `seat show` is the one reader of it.
- The same-host nudge, if minutes ever hurt: a file in the host's temporary
  directory or a socket that carries only "fetch now".
- A `seat-fleet` health role alerting on a peer's long silence, if the
  notification of section 5 proves too quiet.

## 10. Test plan under the contract, build order and box

Package `internal/seat`: the record and its reader, the composer, the
publisher and fetcher behind interfaces (`Publisher`, `Fetcher`, `Clock`),
the standings, the fleet report and its JSON. Behaviour tests run on fakes
with no git and no wall clock, `t.Parallel`, under the shared `TestMain`:
compose from given identity and jobs; refuse each malformed shape and
accept a higher schema with unknown keys; the three publish refusals
(no nickname, unarmed, conflict); the three standings at `age` equal to,
one second below and one second above the threshold, with the threshold
taken from the writer's `tickSeconds`; clock-ahead within and beyond the
threshold; the machine-set union; the clock-ahead reason; the health verdict
after one, two and three failures and after recovery; the transition check
firing once per change; the text and JSON of `seat fleet` from an injected
clock.

Behaviour tests also cover: the ladder, with a fake remote refusing rung 1
by name, refusing rungs 1 and 2, and failing with a timeout that moves no
rung; the reader joining both namespaces with the newest `tickAt` winning;
the pinned key; a pending-setup reservation composing a chain
with a null `startedAt`; a corrupt job record yielding a null chain with a
named detail; a tick without runner context skipping; the verdict
precedence of section 5 case by case; and the transition order, queue then
persist, with the baseline read notifying nothing.

Git, narrowly and explained: one adapter integration test,
`TestPresenceRefRoundTripsThroughABareRemote`, marked `t.Parallel()` so it
stays outside the serial count the ratchet limits, with a bare repository
in `t.TempDir()` as the remote and two clones: publish, fetch into the
other clone, read the file back, publish again and prove the ref moved
without history; two readers fetching concurrently into their own
namespaces without blocking each other; a ref deleted at the remote pruned
from the namespace; a fetch that fails part way leaving the namespace as
it was; and the `LocalMode` update of the local ref only. Its operations
are classified in the isolation inventory as fixture-data and
host-readonly following the existing git fixture rows. The tick integration in slice 1 is a Go test that drives `RunTick` against
a bare repository in `t.TempDir()` with an injected runner context and
asserts the component record, the ref at the remote, the published record,
the publication state and the health line, then that a tick without runner
context moves neither the ref nor the state. The arm-to-runner handoff of
`--lineage` through a resident runner is verified at rollout on the first
armed seat, by reading its presence record; a scenario on the supervision
fixture bed is owed when that bed is next touched. The scenario, when it
comes, is `seat-presence`, added to the supervision fixture bed
(scripts/agents/supervision-fixtures.sh, the kit's existing git-driven
integration layer) in both its invocation and its acceptance lists
(supervision-fixtures.sh:46), with the assertions in Go where the bed
allows: arm against a local bare remote, tick once, assert the component
record, the ref at the remote, the publication state and the health line;
then run `metasystem steward tick` by hand and assert the ref did not
move. Isolation inventory rows carry an owner sentence and evidence beside
their classification, as the existing rows do.

testing.json: a unit group `seat-standard` (packages `internal/seat`, tests
all) and a surface `seat-presence` over `metasystem/internal/seat/**`,
`metasystem/cmd/metasystem/seat_verbs.go` and the steward files touched,
standard `seat-standard`, `fast-static-build`, `refusal-register-standard`,
deep `section/supervision-and-census-fixtures`. Every new refusal code
(`SEAT_MACHINE_NICKNAME_INVALID`, `SEAT_PRESENCE_MALFORMED`,
`SEAT_PRESENCE_CONFLICT`) gets its register row with a Shape and a
Site naming the real emission line within the register test's two-line
window (internal/refusal/register_test.go:55). The parallel ratchet
is refreshed with `audit parallel-ratchet -update`, and the shared test
main lands with the package.

Build order, smallest first:

1. Slice 1, presence: `internal/seat`; the tick component and the fetch at
   the start of ledger-attention; the runner context and the `--lineage` handoff from `steward arm` to
   `steward run`; the health role and its `hasLawfulAutomaticRemedy` case;
   the transition check and its notification; `metasystem seat fleet`; the
   namespace ladder, its state and the two-namespace fetch; the
   configuration keys; refusal rows; the tests above. One build
   lane (Claude on Opus), one code read (Codex on Sol), after one design
   critique of this page (Codex on Astra, Wido's named extra read). Box: two
   attempts, 90 to 150 job-minutes.
2. Slice 2, liveness consumers: the interface's own presence fetch into
   its namespace, `seat fleet --json` served to the interface and the fleet
   panel g1-s42 built on it; the `held by a machine
   unreachable since` flag in the backlog rows and in `goal next`'s printed
   reasons. Box: with g1-s42.

## 11. Non-goals

No sockets and no second transport. No authority from presence and no
automatic takeover. No writes to the ledger branch by this slice. No seat
free text anywhere. No change to the channel, the TOTP rule or the
enrollment. No membership and no asks until a case asks for them.

## 12. Self-grade

Confidence: high that slice 1 is small, buildable and useful on its own,
because every seam it touches is traced above and none is new: a component
in the tick, a role in the health list, a fetch, a push, one verb. Weakest
claims, in order. First, custom refs at the remote: proven on github.com on 2026-09-24
(section 8) and standard git everywhere self-hosted; a hosted service that
admits only branches is served by the namespace key and the preflight,
without a rebuild. Second, the per-reader
namespaces of section 4 are the answer to ref-lock contention within one
clone; the build proves with two concurrent readers that neither blocks
the other, and `seat fleet --fetch`'s cleanup of its namespace must
survive an interrupted read (a stale `presence-fetch/<ulid>` namespace is
harmless and collected by the next run). Third, clocks: a machine whose clock is off by more
than the stale window reads wrong in one direction or the other; the
reason text names it, and the ledger already carries the same exposure.
One alternative declined: a reversible encoding of arbitrary nicknames into
the ref suffix, so that a name with a slash could publish. Every nickname
in this fleet fits the charset, the encoding would be one more thing to
get right on both sides, and a refused name says exactly how to fix
itself. Reject condition: Wido wants membership or asks in the first slice
after all, in which case revision 5's sections 2 and 4 to 8 return and the box
is the one it counted.

## Dispositions (Opus read, 2026-09-24, on revision 7 as first written)

Ten material findings, all verified against the tree before folding.

| id | finding | fold |
|---|---|---|
| F1 | three readers fetching into one `refs/metasystem/presence/*` collide on ref locks, and a presence fetch inside the interface's ledger `Fetch` blanks the tip and backs the loop off | each reader has its own namespace; the interface fetches beside its loop and never through it (section 4) |
| F2 | a reader refusing unknown keys turns every schema addition into fleet-wide `unknown` | accept schema >= 1, validate known keys, ignore unknown (section 3) |
| F3 | the stale window was the reader's configuration while the cadence is the writer's; the boundary was unassigned | the record carries `tickSeconds`; threshold `max(window, 3 × tickSeconds)`; one inequality (section 4) |
| F4 | a clock-ahead record read reachable forever | ahead by more than the threshold is `unknown`, the `CLOCK_REGRESSED` pattern (section 4) |
| F5 | `hasLawfulAutomaticRemedy` defaults false, so the role would alert on the first dead tick; `ConsecutiveFailures` is engine-owned | a `RoleSeatPresence` case; the check decides only alive or dead by age (section 5) |
| F6 | the component's place in `RunTick` was unspecified; a fetch inside `RunLedgerAttention` reports as a ledger failure | after the ledger-attention completion, before the degraded exits and before `completeTickHealth`; the component owns its fetch (section 3) |
| F7 | one writer per ref was asserted, not enforced: shared nicknames, unarmed ticks | `repoIdentity` on the record, a conflict refusal, no publish from an unarmed tick (section 3) |
| F8 | notification nonces are file names and nicknames admit `/` and `..` | a nickname charset and a plain-file nonce (sections 3 and 5) |
| F9 | `holds` and `lastLanding` duplicated the tip and were the staler copy | dropped; the flag joins the tip's claims to standings; `chain` stays as the panel's running column (section 3) |
| F10 | the interface refspec in slice 1 had no consumer | moved to slice 2 (section 10) |

Non-material corrections folded: the sync-branch default and its
qualification; the receipt-line cite; the ratchet pins serial test counts;
refusal rows need Shape and a real Site; the retro-debt cite; the churn
of unreachable objects named in section 3.

## Dispositions (Astra read, 2026-09-24, on revision 7 as first written)

Ten material findings on the same text Opus read; six were already folded
by the Opus round (per-reader namespaces, the total standing predicate,
clock-ahead as unknown, the health remedy switch, `holds` and `lastLanding`
dropped, the nickname rule). What this round added, each verified against
the tree:

| id | finding | fold |
|---|---|---|
| A1 | the fetch contract lacked pruning and atomic updates; deleted remote refs would linger | `--atomic --prune` with the one refspec; LocalMode reads without fetching (section 4) |
| A2 | presence transport was unbounded inside a tick that holds repository arbitration; transport failures and evidence-write failures were not told apart | an owned process group under the ledger fetch's deadline; the two failure classes named (section 3) |
| A3 | `since` was undefined and an old timestamp cannot tell a slow clock from silence | `since` is the reader's frozen first observation; the residual is said in the reason (section 4) |
| A4 | a manual `steward tick` calls the same `RunTick`, so a command process could publish the rollout confirmation; `armedLineage` had no seam | a runner context on `TickConfig` filled only by `RunLoop`, the `--lineage` handoff restored, manual ticks skip (section 3) |
| A5 | the role's counters and details had no owner; initial, skipped and unreadable cases were unspecified | a publication state file owned by the component; verdict precedence stated case by case (sections 3 and 5) |
| A6 | job roots in the machine universe; an unreadable accepted tip unspecified | universe is refs and claims; an unreadable tip reports standings only, no flag (section 4) |
| A7 | once-per-transition needed durable identity and queue-before-persist order | ledger attention's order, frozen `since`, a silent baseline (section 5) |
| A8 | nicknames with a slash cannot be a ref suffix | the charset refusal stands; the encoding alternative is declined in section 12 |
| A9 | receipts carry no landing commit | already dropped with F9 |
| A10 | `pending-setup` reservations lack `startedAt`; corrupt ancestry | qualifying states, ordering and null fields defined by dispatch's vocabulary (section 3) |

Builder clarifications folded into section 10: the ratchet limits serial
tests, so the git adapter test is parallel; isolation rows carry owner and
evidence; the supervision scenario goes in both lists; no-nickname stays a
skipped outcome and only emitted errors get register rows. What neither read could establish, the remote's acceptance of the
namespace, was then proven on the real remote and made configuration
(section 8).
