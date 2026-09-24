# seat-mutual-awareness — design: seats see each other (revision 7, presence first)

- Kind: design
- Id: 01M3A3TATJ3NB3JC43W342M8GC
- Status: draft
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
   (`goal.sync-branch`, default `main`). A transaction fetches with
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
   (`ObserveReceiptLine`, :63; `locateReceiptLedger`, :173).
7. The interface. Its snapshot loop runs a `Fetch func(goal.Endpoint)
   (goal.AdvanceResult, error)` (internal/ui/snapshot/loop.go:108) every
   five seconds while a browser reads and every thirty when none does
   (loop.go:19-20), then reads the ledger at the accepted tip
   (`goal.AcceptedLedgerTip`, `goal.ReadValidatedTree`, `goal.SyncModeGate`,
   snapshot.go). The fleet panel (g1-s42, its own design) reads through
   this loop.
8. Machine identity. A checkout's ledger nickname is `git config
   metasystem.goal.machine`; hostnames are never published, by the engine's
   own refusal text. A checkout without a nickname performs no ledger act.
9. The testing contract. Every package with tests has a `TestMain` using
   `testenv.Main` (`TestEveryPackageUsesSharedMain`); no test waits on the
   wall clock (`TestNoTestWaitsOnWallTime`, allowance file
   internal/testenv/testdata/walltime-allowance.tsv); every file operation
   in a test is classified in internal/testenv/testdata/isolation-inventory.tsv
   (fixture-data, host-readonly, fixture-time, process-private, ...);
   test counts per package are pinned in testing-parallel-ratchet.json
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
+<commit>:refs/metasystem/presence/<machine>`. One writer per ref, so a
force push is always safe and nothing ever merges. The previous commit
becomes unreachable and is collected by ordinary git maintenance on both
sides. In `LocalMode` the ref is updated locally with `git update-ref` and
nothing is pushed, so a single-machine seat still sees itself.

| key | type | required | meaning |
|---|---|---|---|
| presenceSchema | integer | yes | exactly 1 |
| machine | string | yes | equals the ref's last segment |
| generation | integer | yes | the enrollment generation of the running runner (InstallIdentity.Generation) |
| engine | string | yes | the running runner's build stamp, read from the runner's own identity, never from a command process |
| armedLineage | string | yes | the lineage the runner was armed under; the literal `no-lease` when there was none |
| holds | []string | yes | the goal ids whose `Claimed:` line names this machine at the accepted tip, sorted; may be empty. A convenience for readers: the ledger stays the truth |
| chain | object or null | yes | the newest non-terminal delegate job: `{root, job, role, round, goal, startedAt}`; null when idle |
| lastLanding | object or null | yes | `{commit, goal, at}` from the newest receipt line; null when none |
| tickAt | time | yes | RFC 3339 UTC at second precision; the tick that composed this record |

Every key is an identifier, a number, a hash or a time. There is no free
text field, so the record can carry neither a secret nor a seat's words. A
reader refuses unknown keys, missing keys, wrong types and malformed times;
a refused record reads as standing `unknown` with the reason `malformed
presence`, never as reachable.

Writer and cadence. The tick composes and publishes the record as component
attempt `seat-presence`, after `ledger-attention` (tick.go:194-264) so that
`holds` reads the tip that pass just accepted. It publishes EVERY tick,
changed or not: the timestamp is the liveness signal, and a record that
only moved on change would be indistinguishable from a dead machine. The
tick is the cadence; nothing else is configured. A checkout with no
nickname publishes nothing and the component records outcome `skipped`
with the reason `no machine nickname`. A failed publish records the git
error as the component's detail, leaves the previous ref in place, and
never degrades the tick; section 5 says how it surfaces.

Cost: one commit object of a few hundred bytes and one ref update per
machine per tick; one push per tick; no history anywhere.

## 4. Reading: the fetch, the standings and `seat fleet`

The fetch. Every reader fetches `+refs/metasystem/presence/*:refs/metasystem/presence/*`
from the ledger remote, as its OWN fetch call and never inside a goal
transaction's per-operation fetch (section 1.1: that fetch exists to avoid
shared-ref lock contention, and a shared presence refspec inside it would
put the contention back). Three readers:

- the tick, once per tick, at the start of `ledger-attention`'s pass, so the
  transition check of section 5 and the `holds` of other machines are fresh;
- the interface's snapshot loop, in its `Fetch`, so the panel is at most one
  interval behind;
- `metasystem seat fleet` itself, unless `--no-fetch`.

A fetch that fails leaves the previously fetched refs and the reader
reports the fetch's age beside its verdicts. An old engine never fetches
these refs, because the refspec is explicit, and is never disturbed by
them.

The standings, derived at read time and never stored:

| standing | when |
|---|---|
| reachable | the record is well formed and `tickAt` is younger than `seat.presence-stale-min` (default 30 minutes, three ticks) |
| unreachable | the record is well formed and `tickAt` is older than that |
| unknown | no ref exists for a machine the ledger names (a claim or a job root), or the record is malformed |

Retired is not a standing of this slice. An operator removes a machine that
is gone for good with one git act, `git push <remote>
:refs/metasystem/presence/<machine>`, and the machine falls to `unknown`
until membership records (section 9) make retirement a recorded human act.

The machine set a reader reports is the union of the presence refs it
fetched and the machines named by `Claimed:` lines at the accepted tip, so a
claiming machine that never published is visible as unknown rather than
invisible.

Clocks. A reader compares `tickAt` with its own clock, as the ledger already
does for every `at`. A record from the future is reachable, with the reason
`clock ahead by <d>` appended, and a skew above `seat.presence-stale-min`
is reported as such rather than hidden.

The verb: `metasystem seat fleet [--root <checkout>] [--no-fetch] [--json]`.
One line per machine, this machine first:

```
m1e   reachable    2 min ago   generation 4  engine 3f9c1e2  holds finish-test-repairs-and-integrate  running implementer round 2 on finish-test-repairs-and-integrate  landed 5b9d9588e 41 min ago
m1c   unreachable  6 h ago     generation 3  engine 8a01d77  holds none  idle  landed 2ef5d8dc8 2 d ago
m0b   unknown      no presence; named by the claim on fleet-channel-gateway
presence of this machine: published 2 min ago
fetched 2 min ago from origin
```

`--json` prints the same as one object per machine with the standing, the
age in seconds, the raw record and the reasons, for the interface. A
checkout with no nickname prints `this checkout has no machine nickname
and publishes no presence` as its own line and still reports the others.
Timestamps and ages come from an injected clock, so the verb's output is
testable without the wall.

## 5. Presence feeds liveness

Own publish, the health role `seat-presence`, placed after
`RoleLedgerAttention` in `healthRoleOrder` (health.go:68-89) and added to
the closed schema:

- alive, reason `presence published <age> ago`, while the last successful
  publish is younger than `seat.presence-stale-min`;
- dead, `NoAutomaticRemedy` false, `ConsecutiveFailures` counted, reason
  `presence not published since <last>: <git detail>`, remedy `check the
  ledger remote; the next tick republishes`, once three consecutive
  publishes failed or the last success is older than the stale window;
- never unknown: a failure to publish is a fact about this machine, not a
  gap in its knowledge. This is the retro-debt pattern in shape
  (health.go:381-396) with an automatic remedy, because the next tick
  retries by itself.

Others' silence, the transition check in the tick: after fetching, the tick
compares every machine's standing with the standing it recorded at its
previous tick (artifacts/agents/steward/seat-fleet.json, local) and queues
one notification per change of standing, deduplicated by `(machine,
standing, since)`: `m1c has been unreachable since 09:40 and holds two
goals: <ids>` and `m1c is reachable again`. Names only, which is all a
presence record has. This is the human's early warning, delivered through
the notification journal the interface already shows.

Claims held by a silent machine. `seat fleet`, and the interface through it,
mark every goal in an unreachable or unknown machine's `holds` as `held by
a machine unreachable since <t>`. That is a flag, never an act: the goal
stays claimed, `goal next` still refuses it to other machines, and the
human decides with steal or resume as today. Wiring the flag into the
backlog's own rows and into `goal next`'s printed reasons is slice 2
(section 10); the existing local liveness of claims (section 1.5) is not
touched.

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
ledger push already uses and one round trip per tick. `seat.presence-stale-min`
is the one new configuration key, read through internal/config like the
steward's other keys, default 30.

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
compose from given identity, jobs, claims and receipts; refuse each
malformed shape; the three standings across the stale boundary at exact
seconds; the machine-set union; the clock-ahead reason; the health verdict
after one, two and three failures and after recovery; the transition check
firing once per change; the text and JSON of `seat fleet` from an injected
clock.

Git, narrowly and explained: one adapter integration test,
`TestPresenceRefRoundTripsThroughABareRemote`, with a bare repository in
`t.TempDir()` as the remote and two clones: publish, fetch into the other
clone, read the file back, publish again and prove the ref moved without
history, and the `LocalMode` update of the local ref only. Its operations
are classified in the isolation inventory as fixture-data and
host-readonly following the existing git fixture rows. The tick integration
is one scenario, `seat-presence`, added to the supervision fixture bed
(scripts/agents/supervision-fixtures.sh, the kit's existing git-driven
integration layer): arm against a local bare remote, tick once, assert the
component record, the ref at the remote and the health line.

testing.json: a unit group `seat-standard` (packages `internal/seat`, tests
all) and a surface `seat-presence` over `metasystem/internal/seat/**`,
`metasystem/cmd/metasystem/seat_verbs.go` and the steward files touched,
standard `seat-standard`, `fast-static-build`, `refusal-register-standard`,
deep `section/supervision-and-census-fixtures`. Every new refusal code
(`SEAT_NO_MACHINE_NICKNAME`, `SEAT_PRESENCE_MALFORMED`,
`SEAT_PRESENCE_PUBLISH_FAILED`) gets its register row. The parallel ratchet
is refreshed with `audit parallel-ratchet -update`, and the shared test
main lands with the package.

Build order, smallest first:

1. Slice 1, presence: `internal/seat`; the tick component and the fetch at
   the start of ledger-attention; the health role; the transition check and
   its notification; `metasystem seat fleet`; the interface loop's fetch
   refspec; the configuration key; refusal rows; the tests above. One build
   lane (Claude on Opus), one code read (Codex on Sol), after one design
   critique of this page (Codex on Astra, Wido's named extra read). Box: two
   attempts, 90 to 150 job-minutes.
2. Slice 2, liveness consumers: `seat fleet --json` served to the interface
   and the fleet panel g1-s42 built on it; the `held by a machine
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
claims, in order. First, custom refs at the remote: git pushes and fetches
`refs/metasystem/*` like any ref and GitHub stores them, but a remote
policy could refuse ref namespaces outside heads and tags; the build proves
the round trip against the real remote before the tick ships it, and the
fallback is `refs/heads/presence/<machine>`, the same design in a different
namespace, still one writer per branch. Second, the interface loop's
`Fetch` today is the ledger's `AdvanceResult` path; adding the presence
refspec means one more fetch call in that function, and the loop's cadence
of five seconds while a browser reads must not turn into a push storm at
the remote: the presence fetch in the loop runs at most once a minute
whatever the cadence. Third, clocks: a machine whose clock is off by more
than the stale window reads wrong in one direction or the other; the
reason text names it, and the ledger already carries the same exposure.
Reject condition: Wido wants membership or asks in the first slice after
all, in which case revision 5's sections 2 and 4 to 8 return and the box
is the one it counted.
