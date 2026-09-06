# One word stops the metasystem: `metasystem stop`, `status` and `arm` (goal metasystem-stop-verb)

- Status: design, revision 3 with the build addendum of section 13; the
  design ladder is closed and slice 1 is being built. This file supersedes `plans/metasystem-stop-design.md`
  (revisions 1 and 2, frozen where they landed); every later reference to
  the stop design means this file.
- Goal: metasystem-stop-verb (member of the arc verbs-match-intent; its
  slice 0 absorbs this page as the human page's three process verbs)
- Next step: one closing critique; the build proceeds behind the fixtures
  in section 10 whatever it says, any remaining finding carried as a build
  finding

Revision 2 folded the first critique's register in full. Revision 3
(2026-09-06) folds the second, `records/misc/metasystem-stop-critique-r2.md`,
eight material findings, all accepted. Wido's acceptance test stands: one
intuitive word, omnipotent over what the metasystem runs, honest about what
it could not stop. Revision 3 narrows the fleet promise in one place, and
says so where it does (section 8).

## Where each round-2 finding lands

| Finding | Folded in | One-line disposition |
|---|---|---|
| STOP-R2-001 | Section 2 (the generation handshake), 3, 4 step 8, 7, 10 | Every creator reads the fence generation before creating, publishes its record carrying that generation, and re-reads the fence after its process is up; stop takes a late-arrivals pass after acting; no creator holds the transition lock through startup. The page's own lock inconsistency is resolved: mission start opens the fence under the lock, releases it, then arms. |
| STOP-R2-002 | Sections 4 step 7 and 6 | The caller's orderly deadline for the owner is derived from the owner's teardown contract (held components times the component stop ceiling, plus the registry write allowance), never a fixed five seconds; a slow but cooperating owner is never force-killed. |
| STOP-R2-003 | Section 4 steps 3 and 4, section 10 | Proof-run suites are stopped before the monitored runs that host them, so the launcher writes its done marker, the wrapper writes its sidecar, and the run concludes on its own evidence; the fixture nests the launch. |
| STOP-R2-004 | Section 4 step 1, section 6 | `terminateGroup` returns the outcome that ended the group, the turn record carries the host termination result, and the stop reporter prints it from the record; the host-ignores-TERM fixture is added. |
| STOP-R2-005 | Section 2 (the transition) and 10 | After a crashed stop, re-inventory produces a new survivor list written to the fence record; `arm` refuses over any live survivor and prints the resumed-stop command; the aggregate crash fixture covers a live runner, job and run. |
| STOP-R2-006 | Section 4 step 2, section 6, section 9 | A non-terminal job owned by another machine is a survivor: `NOT STOPPED`, exit 1, owning machine named, nothing signalled; no remote cancellation is invented. |
| STOP-R2-007 | Section 8 | The directory-gone promise is withdrawn: a registry checkout whose directory is gone is reported, its recorded owner identity is probed and named, and nothing is signalled from a command-line shape. |
| STOP-R2-008 | Section 6 | The escalated registry row is appended only when the caller force-killed the owner; an unproven component death under an orderly owner is a `NOT STOPPED` line and a fence-record entry, never a registry event. |

## Where each round-1 finding lands

| Finding | Folded in | One-line disposition |
|---|---|---|
| STOP-R1-001 | What is true today; sections 4 and 5 | The inventory gains the mission runner with its host turn, monitored runs, and proof-run suites with their watchdog; each has a record terminal state and an identity-safe orderly-then-forceful stop. |
| STOP-R1-002 | Section 2 | The stopped record is a closed process-creation fence read by every creation path, the steward's direct verbs and the watcher's steward repair included; the order of stopping starts by closing it. |
| STOP-R1-003 | Section 2 | Stop and arm share one checkout-wide lock held for the whole transaction, a generation, a rollback rule and a failure-state mapping. |
| STOP-R1-004 | Section 6 | The shutdown transaction is extended, not copied, to return per-component outcomes and to append the escalated registry row; each printed line comes from one. |
| STOP-R1-005 | Section 7 | The handover is specified against the real launch flow: the transient launcher opens the fence and arms, the run loop takes the mission lease and publishes its record, the host session renews the checkout lease by lineage; resume takes the same path. |
| STOP-R1-006 | Sections 1 and 7 | `status` is open, read-only, no human gate. |
| STOP-R1-007 | Section 8 | Fleet discovery is a live-process inventory: registry reduction joined with one process enumeration by checkout path and with the mission, run and job records; a dead-owner checkout with a live runner or mission is stopped. |
| STOP-R1-008 | Section 6 | `status` is the current live inventory; `stop` prints what it acted on; the historical-identities fixture is dropped. |
| STOP-R1-009 | Section 10 | Fixtures for every process family and every fence reader, a positive seat-survives proof, escalation seams named honestly, and the survives-kill case moved to the injectable liveness and signal seams that exist. |
| STOP-R1-010 | Section 2 | The fence lives in a leaf package every reader imports without a cycle; the aggregate transition has a named Go owner in a second package; the command file stays thin. |
| STOP-R1-011 | Sections 1 and 8 | The single form takes `--installation` when the checkout does not carry one; the fleet form never refuses for a missing installation, it stops from records with the invoking engine's signatures and says so. |

## What is true today (read in the tree)

There is no verb that stops the metasystem. `metasystem delegate --cancel`
stops one job. `steward restart` stops a runner only to start another;
`steward disarm` ends the runner alone. The one whole shutdown is
`metasystem up --shutdown`, a flag the code labels an internal fixture
compatibility option, reached through `arm-supervision.sh --shutdown`. It
stops supervision and nothing else, and the next session start or Stop
hook arms everything again.

What the metasystem runs for one checkout, where each thing records
itself, and how its record reaches a terminal state:

- **Delegate jobs.** One record per job under `artifacts/agents/jobs/`
  with `status`, `pid`, `pgid`, `instanceTag`, `machineId` and the custody
  process list. Non-terminal is `pending-setup`, `pending` or `running`;
  terminal is `completed`, `failed`, `cancelled` or `timeout`
  (`TerminalStatus` in `internal/dispatch/record.go`). The cancel path
  (`dispatch.sh cancel`, reached by `delegate --cancel`) marks the record
  `cancelling` before any signal, winds down the custody groups after
  proving ownership by tag, TERM then KILL after two seconds, and concludes
  `cancelled`.
- **The steward runner.** `artifacts/agents/steward/runner.json` names
  pid, start second and the boot identity pair; `liveRunner` proves it by
  that pair. `Disarm` writes the stop file, waits five seconds, sends one
  TERM and never KILLs; `stopRunnerForReplacement` does TERM, two seconds,
  KILL. The runner's loop checks the stop file between ticks and sleeps up
  to the tick interval, so the signal is the real stop.
- **The supervision owner, watcher and reaper.** `lock.d/owner.json`
  names the owner by pid, start time and tag; `state.json` names the
  watcher and reaper of the current generation. `ShutdownAt` in
  `internal/supervise/arming.go` refuses a lock whose tag is outside this
  checkout's prefix, refuses a live owner the host registry records for
  another checkout, refuses an uninspectable owner, writes the shutdown
  intent, TERMs the owner's group, waits five seconds, KILLs, then stops
  every recorded and tag-discovered component the same way and releases
  the lock. It returns only an error: which signal ended what is lost, and
  the escalated registry row the lifecycle contract describes
  (`reaped reason=shutdown-escalated`) is written by nothing. The owner
  itself always installs TERM and INT handling (`supervise_owner.go`),
  latches the intent, tears down its held set by identity and appends
  `exited` with `teardownComplete`.
- **The narrator** is not a process: the runner records a `narrator`
  component attempt each tick and the owner appends a cycle trace to its
  log; health reads the runner's record as `narrator-freshness`.
- **The mission runner.** `mission start` and `mission resume` run in a
  transient launcher process that clears a stale mission lease, arms
  supervision through `arm-supervision.sh` as its own identity (the
  launcher's pid, or the holder's announcement when the launcher is the
  holder's child) with the mission lineage, preflights the contract, then
  spawns `mission run-loop` detached in its own session
  (`internal/missionrunner/launch.go`). The loop takes the mission lease
  (`artifacts/agents/missions/<id>/lease.d/owner.json`, one flock), writes
  its runner record `artifacts/agents/missions/runners/<id>.json` with
  pid, start pair, boot identity, pgid and `instanceTag`, and heartbeats.
  Each cycle spawns a host turn: an agent runtime leading its own process
  group, verified by tag within a grace, recorded in
  `artifacts/agents/missions/<id>/turns/<turn>/turn.json` with pid, pgid,
  tag, runtime and status; the runner winds a capped turn down through
  `terminateGroup`, which proves ownership by the positioned tag
  (`janitor.GroupOwnership`) before TERM and KILL. The runner installs no
  signal handling: a TERM kills it mid-cycle, its record keeps status
  `running` with a dead identity, its host turn survives in its own
  group, and only the next launcher's `cleanupStaleLease` proves the
  runner dead, terminates the stale turn group and patches the turn
  `failed / turn-lost`. `finishRunner` writes the record terminal only from
  the process that published it.
- **Monitored runs.** `run launch` writes `artifacts/agents/runs/<id>.json`
  (`launching`) and spawns `run wrap` detached; the wrapper binds pid,
  start pair, boot identity, pgid and the launch nonce that rides its argv
  (`running`, custody `wrapped`), runs the workload and writes the exit
  sidecar last. `run register` and `run adopt` bind an already running
  foreign leader (custody `adopted-verified` or `adopted-unverified`). The
  watcher's pass assesses every record: a dead leader freezes the
  provisional verdict and goes `draining` until the group empties, then
  terminal (`green`, `red`, `ended-unknown`, `launch-failed`). Nothing in
  the run family signals a process except the lease takeover sweep
  (`internal/lease/sweep.go`), which for wrapped custody proves the group
  by the nonce, TERMs it, drains five seconds and forces `ended-unknown`;
  adopted custody is never signalled, only surfaced.
- **Proof-run suites.** `proof-run launch` runs in the foreground of its
  caller (a `run wrap` workload when the validation script launches it),
  starts the suite in its own process group and a sibling `proof-run
  watchdog` in another, and writes a done file after the suite exits; the
  watchdog exits on the done file. No record carries these pids: the
  watchdog's argv carries the suite's exact identity (`--suite-pid`,
  `--suite-started-at`, `--suite-start-ticks`, `--suite-boot-id`) and the
  installation (`--root`). On a stall the watchdog preserves evidence,
  shuts supervision down through the internal flag, then CONT, TERM, term
  grace, KILL, kill grace against the suite identity, re-proved before
  each signal (`signalAuthenticated`).
- **The host registry** `~/.metasystem/armed-checkouts.jsonl` is an
  append-only event log; `registry.Reduce` folds it into open and closed
  owner publications and claims per checkout path. It records supervision
  claims only: a steward runner, a mission, a run or a suite never appears
  in it.

Every one of these has a creation path with no fence in front of it:
`up` in every mode, `steward arm`, `steward restart` and `steward run`
(`cmd/metasystem/steward_verbs.go`), the watcher's steward-repair pass
(`RepairEnrolledRunner` from `supervise_component.go`), `EnsureRunner`
from `up`, `dispatch.ClaimLaunch` (reached by `delegate`, by `dispatch.sh`
directly and by the steward's revival), `run launch`, `run register`, `run
adopt`, `proof-run launch`, `mission start`, `mission resume` and `mission
run-loop`. Two authority facts govern the verbs. `steward arm` and
`steward restart` admit only a caller the lease classifier calls `HUMAN`
(a controlling terminal and no agent, delegate, supervision or steward
ancestor: `requireHumanStewardEnrollment`). `up --shutdown` requires the
checkout holder, which a human always is.

## 1. The verbs and their grammar

Three verbs at the top level, beside `up`, `health` and `watch`, dispatched
before the family table in `cmd/metasystem/main.go` and listed in its usage
text:

```text
metasystem stop   [--repo <path>] [--installation <dir>] [--all]
metasystem status [--repo <path>] [--installation <dir>] [--all]
metasystem arm    [--repo <path>] [--temporary-human-word <word> --review-by <date>]
```

No `--by`: the gate proves the human. The checkout is found from the
working directory the way `up` finds its scope: `git -C <repo> rev-parse
--show-toplevel`, `--repo` defaulting to `.`. The installation is the
toplevel when it carries `bin/metasystem`, else `<toplevel>/metasystem`
when that does, else the directory `--installation` names, which must
carry `bin/metasystem`; the state root is `stateroot.RootForInstallation`
of the installation, which is what `up` arms and what the registry
records. The owner tag prefix is `metasystem-supervision-owner-<slug(toplevel)>-`,
exactly as `up` computes it. `--repo` and `--installation` are never
needed from inside a checkout that carries its installation.

`arm` is the start-again act and the one name `stop` prints. Under the
transition lock of section 2 it opens the fence, runs `steward arm`'s
transaction (enrol the invoking engine, start the runner; `already armed`
beside a live runner) and then the recovery-only arming of `up`
(`RecoverOnly` with `IfDown`: start only the missing supervision rings, no
announcement, no lease), so one word brings back everything one word took
down. `steward arm` and `steward restart` stay as the long forms the
plumbing and the re-arm remedies call; their help lines gain "(long form
of metasystem arm)". `--temporary-human-word` and `--review-by` pass
through to `ArmTemporary` unchanged.

`status` is open: no gate, read-only, usable by a seat, a script and the
fleet brain, like `supervise status` and `health` today. `stop` and `arm`
are human acts (section 7). Exit codes: `status` exits 0 whether or not
anything runs and 1 only when the inventory could not be read; `stop`
exits 0 when everything it listed is stopped or nothing was running, 1
when a line says `NOT STOPPED`.

## 2. The fence, its lock and its owner

### The record

`artifacts/agents/supervision/transition.json` under the state root, one
per checkout, never deleted once written:

```json
{
  "schemaVersion": 1,
  "state": "closed",
  "phase": "stopping",
  "generation": 7,
  "changedAt": "<RFC3339>",
  "by": {"verb": "stop", "pid": 4242, "pidStartedAt": 1788700000, "pidStartTicks": 0, "bootId": ""},
  "checkout": "<toplevel>",
  "notStopped": []
}
```

`state` is `closed` or `open`; an absent file is open. `phase` is
`stopping`, `stopped` or `stop-incomplete` while closed and `armed` while
open. `generation` advances by one on every stop and every arm.
`notStopped` lists, after a stop that could not end everything, each
surviving identity (`component`, `pid`, `pidStartedAt`, `tag`, `reason`).
`by` is the kernel identity of the process that last changed the record.

### The lock

`artifacts/agents/supervision/transition.lock.d`, taken through
`internal/lock.Acquire` with the caller's kernel identity and the kernel
probe, bounded wait of ten seconds scaled by the fixture scale. `stop`
holds it from before the record is closed until the last line is printed;
`arm` holds it from the record read until its last ring is up;
`mission start` and `mission resume` hold it only while they open the
fence, and release it before they arm (section 7): no creator holds this
lock through a process start. A holder that died mid-transaction is taken over by the next acquirer
(the lock's own dead-holder rule); a live holder makes a concurrent `stop`
or `arm` wait, then refuse with the holder named and the second line
`run: metasystem status --repo <toplevel>`.

### The transition, and who owns it

- **Stop:** under the lock, write `closed / stopping / generation+1`, take
  the inventory (section 3), act in order (section 4), then write the
  phase: `stopped` when every line ended in a stop, `stop-incomplete` with
  the `notStopped` list otherwise. The fence stays closed in both.
- **A crashed stop** leaves `stopping` with a dead `by` identity. The next
  `stop` or `arm` proves that identity dead and re-inventories (section
  3) under the lock. `stop` then acts on the inventory as an ordinary stop
  does. `arm` writes what it found: every live identity the re-inventory
  names becomes the new `notStopped` list and the phase becomes
  `stop-incomplete` with `by` set to the arming process; an empty
  re-inventory becomes `stopped`. Only then does `arm` apply its own rule
  below. Nothing left by a crashed stop is ever assumed stopped, and the
  stored list from before the crash is never consulted: the re-inventory
  replaces it.
- **Arm:** under the lock, refuse while the record is `stopping` with a
  live `by` identity (a stop is in progress). Under `stop-incomplete`,
  probe each listed identity by kernel identity: any still alive is a
  refusal naming it, with the second line `run: metasystem stop --repo
  <toplevel>` and the words "if it survives a second stop, end pid <pid>
  yourself; it is listed with its start time". A listed job owned by
  another machine cannot be probed here and counts as alive until its
  record is terminal; the refusal names the owning machine and the words
  "cancel it from that machine with `metasystem delegate --cancel <job>`".
  When none survives, or the phase is `stopped`, write `open / armed /
  generation+1` and arm. **Arm never re-closes the fence:** a failed arm leaves the fence open,
  leaves whatever came up running, prints the failing component with the
  remedy `steward arm` or `up` prints today, and exits 1; `status` then
  shows the partial state. That is the rollback rule, and it is the same
  world a failed `steward arm` leaves today.
- **Mission start and resume** open the fence exactly as `arm` does, with
  `by.verb` `mission-start` or `mission-resume`, and release the lock
  before arming (section 7).

### The generation handshake

The fence check before creation is not enough on its own: a creator can
read an open fence, pause, and publish or start after `stop` has closed
the fence and taken its inventory. Two rules close that window without
any creator holding the transition lock through a start:

1. **Creators carry the generation.** Every creation path reads the fence
   once before it creates anything and remembers the `generation` it
   read. The record it publishes carries that number in a new field
   `fenceGeneration` (job records at `BuildSetup`, run records at
   `Launch`, `Register` and `Adopt`, the mission runner record when the
   loop publishes it, the steward runner record when the runner starts,
   `lock.d/owner.json` when the owner is launched). A process without a
   record carries it in argv: `proof-run launch` passes
   `--fence-generation <n>` to itself and to its watchdog beside the
   suite identity. After its process is up and its record is published,
   the creator reads the fence a second time. If the fence is closed, or
   its generation differs from the one remembered, the creator ends what
   it started by the same identity proof it would use later (the run
   launcher fails its launch with the note `stopped by metasystem stop`,
   the dispatcher's setup refuses and its husk carries `refusalClass`
   `stopped`, the proof-run launcher runs its own ladder against the suite
   it started, the mission loop releases the lease and exits with the
   refusal in its log, the steward runner exits before its first tick) and
   prints the section 9 refusal. No creator holds the lock at any point.
2. **Stop takes a late-arrivals pass.** After acting on its inventory
   (section 4 steps 1 to 7), `stop` re-inventories once more. Anything
   live that carries a `fenceGeneration` at or below the generation
   `stop` closed with (or, for a process with no record, anything the
   enumeration finds in scope by the engine's own shapes) is a late
   arrival and is stopped through the same family step; the pass repeats
   until an inventory is empty, at most three times, and what is still
   live after that is `NOT STOPPED` with the reason `arrived during
   stop`.

Why this is complete: a creator that read the open fence either
publishes before `stop`'s last pass, in which case `stop` finds and stops
it, or publishes after it, in which case the creator's own second read,
which follows its publication, sees the closed fence and ends it. There
is no third case, and the proof is deterministic: the package tests of
section 10 drive a creator whose publication is injected between `stop`'s
first inventory and its last pass, and a creator whose second read sees
the fence close after it started.

Ownership: the record, its reader and the lock live in a new leaf package
`internal/stopfence`, importing only `internal/lock`, `internal/atomicfile`
and `internal/identity`, so `up`, `steward`, `dispatch`, `goal`, `run`,
`proofrun`, `missionrunner` and `supervise` all import it without a cycle
(`up` already imports `steward`, and `lease` imports `steward`, which is
why the record cannot live in `up`). The aggregate transition, the
inventory and the line rendering live in a second new package
`internal/stoptransition`, which imports every family it stops and is
imported by the command file alone; its named owner is the type
`Transition` with the methods `Stop`, `Status` and `Arm`, each returning a
`Report` of typed lines, and its package tests drive every ordering and
failure rule of this page against injected families. The command file
`cmd/metasystem/process_verbs.go` parses flags, applies the gate, calls the
transition and prints.

### The readers

The fence is closed when the record reads `closed`. Every creation path
reads it through `stopfence.Closed(stateRoot)` before it creates anything:

| Reader | What it does when closed |
|---|---|
| `up`, every mode (`internal/up`), after `host-preflight` and before the enrollment open | prints `component=stopped outcome=standing detail="since <changedAt>"` and `up outcome=stopped remedy="metasystem arm --repo <toplevel>"`, exits 0, does nothing else: no re-arm, no announcement, no lease, no supervision, no runner; the scheduler entry and the hooks take this path |
| `steward arm`, `steward restart` (cmd) | refuse (section 9) |
| `steward run` at start (the runner itself) | refuses to guard the repository; a launcher that raced the stop cannot leave a runner behind |
| `RepairEnrolledRunner` (the watcher pass) and `EnsureRunner` (`up`) | return status `FENCED`, launch nothing; the watcher pass records `steward fenced` as evidence and counts it as a completed pass |
| `dispatch.ClaimLaunch` (`delegate`, `dispatch.sh dispatch`, the steward's revival) | refuses with outcome `REFUSED-STOPPED` (section 9); `delegate --cancel` is not creation and works |
| `run.Store` `Launch`, `Register`, `Adopt` | refuse (section 9) |
| `proof-run launch` | refuses before starting the suite (section 9) |
| `mission start`, `mission resume` | a `HUMAN` caller opens the fence (section 7); any other caller is refused (section 9) |
| `mission run-loop` at start | refuses to take the lease (a stray loop under a closed fence exits with the refusal in its log) |
| `health` | prefixes its line `HEALTH STOPPED` and replaces every `metasystem up` remedy with `metasystem arm --repo <toplevel>`; roles render dead without advancing the failure counters, so a stop raises no alert episode |
| `goal.TurnVerdict` | treats a closed fence as the human-authorized quiet a consumed session stop grants, without consuming anything (section 5) |

Nothing else reads it. The goal ledger, announcements and the checkout
lease are untouched by `stop`.

## 3. The inventory

One function, `stoptransition.Inventory(checkout)`, is what `status`
prints and what `stop` acts on. It reads, in this order:

1. Job records under `artifacts/agents/jobs/`: every non-terminal record,
   with its `machineId`.
2. Mission runner records under `artifacts/agents/missions/runners/`: every
   record with status `running` whose identity (pid, start pair, boot
   identity) is alive and whose argv matches the `mission-run-loop` shape
   with the recorded tag; for each, the open turn records under the
   mission's `turns/` whose group is alive and owned by the turn's tag.
3. Run records under `artifacts/agents/runs/`: every `launching`,
   `running` or `draining` record with its custody and, for wrapped
   custody, the leader alive by identity.
4. The steward runner by `liveRunner`.
5. Supervision: the owner from `lock.d/owner.json` by the three-way rule,
   the watcher and reaper from `state.json` and from the tagged sweep
   `takeoverComponents` already performs.
6. Proof-run suites: from one process enumeration, every `proof-run
   watchdog` process whose `--root` canonicalizes to the checkout's
   installation, carrying its suite identity in argv; the suite by that
   identity; the `proof-run launch` process with the same `--root` and
   `--suite`.
7. Everything else in scope: the same enumeration classified through the
   configured adapter signatures and scoped by the census rule (a working
   directory at or below the toplevel, or an argv path below it), minus
   what the records above already own and minus announced sessions.

The enumeration is one `EnumerateConfiguredProcesses` pass with the
installation's signatures (`census`), and never writes `last-census.json`.
Records whose identity is dead are reported as `not running`; nothing is
inferred from a record alone. Every item carries the `fenceGeneration`
its record or argv names (absent for records written before this lands),
which the late-arrivals pass of section 2 reads. A job record whose
`machineId` names another machine is inventoried with that machine and
no identity probe: its liveness is unknown here, and section 4 treats it
as a survivor.

## 4. The order of stopping, and why

Per checkout, under the lock, after the record is closed:

1. **The mission runner, then its host turn.** Write
   `artifacts/agents/missions/<id>/stop-intent.json` naming the runner
   identity and the requester; TERM the runner's group. The run loop
   gains what the owner has: TERM and INT notification checked at its
   heartbeat points (the cycle boundary and the host supervision loop).
   On the signal it latches the intent, winds down an open host turn
   through its own `terminateGroup`, patches the turn `failed / turn-lost`
   with detail `stopped by metasystem stop` (the same words the stale-turn
   cleanup writes, so resume re-lists the turn), takes the failure ramp
   with runner status `stopped` (a fourth status beside `running`, `failed`
   and `completed`), releases the mission lease and exits. A TERM with no
   matching intent finalizes `failed / terminated` as an unexplained death.
   If the runner is still alive after the scaled ten-second wait, `stop`
   re-proves its identity and KILLs the group, then does what the runner
   could not: winds down the open turn's group after proving ownership by
   tag (the same proof and grant path `cleanupStaleLease` uses), patches
   the turn, finalizes the runner record `stopped` (allowed because the
   publisher is proven dead), and releases the lease marker through a
   function extracted from `cleanupStaleLease` so launch and stop share
   one dead-runner rule. The mission's `state.json` is untouched: a
   stopped mission is a running mission awaiting `resume`.
   The runner goes first because it is the one component that creates
      turns, and its turn's host is what dispatches jobs; with the fence
   closed neither can create, but the runner's own wind-down of its turn
   is the orderly path for the host group.
   The host result is carried, not guessed: `terminateGroup` returns an
   outcome (`already-gone`, `term`, `kill`) beside its error, the runner
   writes it into the turn record as `hostTermination` when it winds the
   turn down on the stop signal, and `stop` writes the same field when it
   winds the turn down itself. The printed host line comes from that
   field: `stopped (by the runner, TERM)`, `killed (by the runner, TERM
   ignored)`, `stopped (TERM)` or `killed (TERM ignored)`; the aggregate
   exit of the runner is never read as the host's result.
2. **Jobs, through the cancel path.** Every non-terminal job record whose
   `machineId` is this machine's (or absent) is cancelled by running the
   engine's own `delegate --cancel <job>` transaction, one job at a time,
   while the watcher and reaper are still up, so each conclusion is
   reaped, mirrored and censused as an ordinary cancel is. A record naming
      another machine is a survivor (below). `internal_cancel` gains one
   field on its concluding patch, `cancelEscalatedToKill: true`, when the
   wind-down reached KILL.
   A non-terminal record naming another machine is a survivor, not an
   exclusion: this checkout's stop cannot prove or end a process on
   another host, and reporting success over it would be false. Its line
   is `NOT STOPPED job <id> <status> machine <other>: owned by another
   machine; did: nothing, cancel it from <other> with metasystem delegate
   --cancel <id>`, it is listed in the fence record, and the exit code is
   1. No remote cancellation is invented here.
3. **Proof-run suites.** The watchdog's ladder against the suite identity
   from its argv: CONT, TERM, term grace, KILL, kill grace, each signal
   preceded by a kernel re-proof (`signalSuiteGroup` and
   `signalAuthenticated`, exported from `internal/proofrun`). The launcher
   then writes its done file and exits, and the watchdog exits on the done
   file; `stop` proves both gone by identity within the kill grace and
   otherwise TERMs and then KILLs each by identity. Suites go before runs
   because a suite is usually a run's workload: the launcher runs in the
   wrapper's process group, so stopping the run's group first would kill
   the launcher and the watchdog with the suite and lose the done marker
   and the sidecar. With the suite stopped first, the launcher exits on
   its own, the wrapper writes the exit sidecar, and the run concludes
   `red` from its own evidence before step 4 reaches it.
4. **Monitored runs.** A run whose workload was a suite stopped in step 3
   is given the scaled five seconds to conclude from its sidecar and is
   then reported from its record (`concluded red (suite stopped)`); only
   a run still non-terminal after that is acted on. `launching`:
   `FailLaunch` with the note `stopped by metasystem stop`. Wrapped
   `running` or `draining`: prove the group by the nonce (`groupOwnsTag`,
   the takeover sweep's proof), TERM it, wait the scaled five seconds,
   re-prove and KILL if it persists, then assess and, when the record is
   still not terminal, force `ended-unknown` with the note, exactly as
   `SweepStale` does. This is `SweepStale`'s body with the epoch test
   replaced by "every record", extracted into one store method both
   callers use. Adopted custody is reported, not signalled: the line
   names the leader and says the process is not the metasystem's.
5. **The steward runner.** `Disarm` becomes the one runner stop: write the
   stop file, TERM the recorded identity when it does not exit within the
   scaled five seconds, KILL after two more, prove death by `liveRunner`
   after each step, and return which signal ended it. `steward disarm`
   keeps calling it. The runner goes after the work because a live runner
   ticking over a set being torn down would queue an incident per missing
   component; it cannot recreate anything, because the fence is closed.
6. **Supervision: owner, then watcher and reaper.** `ShutdownAt`, extended
   as section 6 says: the owner's exit handler tears down its held set by
   identity, then the caller sweeps the recorded and tag-discovered
   components and releases the lock. The owner goes last because it is the
   one process that can prove its components dead and append the terminal
   registry row itself; the watcher must outlive the jobs and runs so
   their conclusions are censused and the reaper mirrors evidence.
7. **The owner's orderly deadline.** The caller's wait for the owner is
   not a fixed five seconds. The owner's teardown contract is sequential:
   each held component may consume one component stop ceiling
   (`StopCeiling`, five seconds, set in `supervise_owner.go`), and the
   owner then appends its terminal registry row. `ShutdownAt` therefore
   derives its deadline as the number of components the owner holds
   (read from `state.json`; two when the file is unreadable) times the
   ceiling, plus a registry write allowance of the scaled two seconds,
   all scaled by the fixture scale, and only after that deadline
   re-proves the owner's identity and KILLs. A slow but cooperating owner
   is `stopped (TERM, exited reason=shutdown)`; `killed (TERM ignored)` is
   printed only when the owner outlived its whole contract.
8. **The late-arrivals pass** of section 2: re-inventory, act on any late
   arrival through its family step, repeat until empty or three passes.
9. **The untracked sweep.** The enumeration of section 3 step 7, run
   after everything above, printed and never signalled.

Nothing in this order can recreate what a previous step stopped: every
creator reads the fence before and after creating, the fence was closed
before step 1, and step 8 catches what slipped between.

## 5. The human's own session

The seat session is never touched. Its process, announcement, lease and
delegate worktrees stay; the census lists it as `ANNOUNCED`. Its Stop hook
runs `up`, which returns `stopped` with exit 0, so no arming failure is
recorded and nothing blocks; the check-in tail carries the aggregate line
the way it carries a re-arm notice today. `goal.TurnVerdict` treats the
closed fence as the same human-authorized quiet a consumed session stop
grants, without consuming anything, because the same terminal authority
closed it: `ShouldBlock` is false for the idle-backlog rule while the
fence is closed. `metasystem session stop` stays what it is: one seat, one
turn end, the metasystem running; `stop` is every seat in the checkout and
nothing running. `stop` never mints a session stop.

## 6. Honesty: what is printed, and where each line comes from

### The extended shutdown transaction

`ShutdownAt` keeps its signature's inputs and returns
`(ShutdownReport, error)`; `up.Shutdown` and the internal flag print the
report's lines beside today's outcome. `ShutdownReport` holds one
`ComponentOutcome` per identity it touched: `Component` (`supervision-owner`,
`repo-watcher`, `job-reaper`), the identity and tag, the generation,
`Signal` (`none`, `term`, `kill`), `Result` (`stopped`, `already-gone`,
`not-stopped`) and `Reason`. `stopOwner` and `stopRecordedComponent`
return the outcome they produced instead of discarding it, and a component
found dead when the caller sweeps after the owner's own teardown reads
`already-gone` with the reason `by the owner`. The registry gets exactly
the event its contract allows. When the caller had to KILL the owner (the
owner never appended its own terminal row), `ShutdownAt` appends the row
the lifecycle contract already describes and no code writes today:
`reaped` with `reason=shutdown-escalated`, the `killed` list of identities
it signalled, and `sweepPending` true when a component's death is still
unproven, through `registry.LockedAppend` under the registry lock with the
caller's identity. When the owner exited orderly and appended `exited`,
that terminal event is absorbing (`registry.Reduce`) and nothing further
is appended, whatever the caller then had to do to a component: a
component the caller KILLed after an orderly owner exit, or whose death
it could not prove, is a stop line (`killed (TERM ignored)` or `NOT
STOPPED ... death unproven; did: left it listed in the fence record`) and
a fence-record entry, never a registry event. `Disarm`, the mission stop,
the run stop and the suite stop return the same outcome shape.

### The line grammar

One line per thing, stdout, in the order acted on, from the outcomes
above; a fixture parses them:

```text
checkout <toplevel>
mission <id> runner pid <pid> pgid <pgid> tag <tag>: stopped (TERM, runner concluded)
mission <id> runner pid <pid> pgid <pgid> tag <tag>: killed (TERM ignored; turn and lease closed by stop)
mission <id> turn <turn> host pid <pid> pgid <pgid>: stopped (by the runner, TERM) | killed (by the runner, TERM ignored) | stopped (TERM) | killed (TERM ignored)
job <id> <status> pid <pid> pgid <pgid> role <role>: cancelled (cancel path, TERM)
job <id> <status> pid <pid> pgid <pgid> role <role>: cancelled (cancel path, KILL after TERM was ignored)
NOT STOPPED job <id> <status> machine <other>: owned by another machine; did: nothing, cancel it from <other> with metasystem delegate --cancel <id>
run <id> <status> pid <pid> pgid <pgid> wrapped: concluded red (suite stopped)
run <id> <status> pid <pid> pgid <pgid> wrapped: concluded ended-unknown (TERM) | (KILL after TERM was ignored)
run <id> <status> pid <pid> adopted: not the metasystem's process, not signalled
run <id> launching: launch failed (stopped)
proof-run <suite> suite pid <pid> pgid <pgid>: stopped (TERM) | killed (TERM ignored)
proof-run <suite> watchdog pid <pid>: exited (suite ended) | stopped (TERM) | killed (TERM ignored)
proof-run <suite> launcher pid <pid>: exited (suite ended) | stopped (TERM) | killed (TERM ignored)
steward-runner pid <pid> started <epoch>: stopped (TERM) | stopped (KILL after TERM was ignored)
narrator: stopped with the steward runner
supervision-owner pid <pid> tag <tag> generation <n>: stopped (TERM, exited reason=shutdown) | killed (TERM ignored; reaped reason=shutdown-escalated)
repo-watcher pid <pid>: already gone (by the owner) | stopped (TERM) | killed (TERM ignored)
job-reaper pid <pid>: already gone (by the owner) | stopped (TERM) | killed (TERM ignored)
<any family line above>: <its outcome> (arrived during stop)
untracked pid <pid> <runtime> <argv>: not the metasystem's, not touched
NOT STOPPED <component> pid <pid> started <epoch>: <reason>; did: <what stop did about it>
stopped <toplevel>; start again: metasystem arm --repo <toplevel>
```

Every signal goes to an identity, never a bare pid: jobs by tag-proven
groups, runs by nonce-proven groups, the mission runner and its turn by
positioned tag, the steward runner by its recorded pair, the owner and
components by pid, start time and tag, the suite by the four-part
identity in the watchdog's argv. An identity that no longer matches is
`already gone` and is not signalled. KILL is sent only after the orderly
wait expired and the identity was re-proved beside it. A component that
survives KILL, an uninspectable identity, or a lock the tag prefix vetoes
is a `NOT STOPPED` line naming the reason and what `stop` did (left the
record, left the lock, appended the escalated row, listed it in the fence
record) and the exit code is 1; `stop` continues to the next step.

### The same-list contract

`status` prints the inventory of section 3 as it is now, one line per
thing with the state in the verdict position (`running`, or a record's
status), then either `nothing is running` or nothing more, then the fence
line `stopped since <changedAt>; start again: metasystem arm --repo
<toplevel>` when the fence is closed. `stop` prints the inventory it took
under the lock, one line per thing with what it did. The two agree by
construction on the identities they name at the moment each reads; neither
keeps history. A second `stop` prints `checkout <toplevel>`, `nothing is
running` and the start-again line, exits 0 and writes only the fence
record's `generation` and `changedAt`.

## 7. Authority and the headless handover

`stop` and `arm` are human acts at a terminal, gated by the function
`steward arm` and `steward restart` use, `requireHumanStewardEnrollment`,
renamed `requireHumanTerminal` and moved beside the new verbs so all four
share it. A fixture-granted `HUMAN` classification is admitted as it is for
`arm`. This is the classifier gate, not the enrolled-terminal proof of
`internal/humanauthority` the goal verbs and `session stop` use (its
refusal is `TERMINAL_NOT_REACHED`): a person at a terminal who never ran
`goal enroll-terminal` can still stop the metasystem, as they can arm it
today. `status` has no gate.

Machinery does not call `stop`. The supervision step it wraps stays
reachable as the internal long form `up --shutdown`, holder gated, writing
no fence, marked internal in the help text; the fixture beds and the
proof-run watchdog keep calling it through `arm-supervision.sh
--shutdown`, and nothing new may start calling it.

The headless handover, against the launch flow as it is:

1. The human ends the seat's session and runs `metasystem stop`. The
   fence closes; the seat's owner, watcher, reaper and runner are gone.
2. The human runs `metasystem mission start` (or `resume`). The launcher
   classifies its caller through the lease classifier it already calls in
   `resolveArmingIdentity`; a `HUMAN` caller takes the transition lock,
   opens the fence (`open / armed / generation+1`, `by.verb`
      `mission-start`), and releases the lock before `armAndPreflight`. This
   is the one rule for the lock (section 2): the launcher, like every
   creator, carries the generation it read and re-reads the fence after
   the loop is up, and never holds the lock while arming. Any other caller
   under a closed fence is refused (section 9). Nothing changes when the
   fence is open.
3. `armAndPreflight` arms through `arm-supervision.sh` as it does today:
   the launcher's own transient identity with the mission lineage. The
   owner, watcher, reaper and steward runner come up under that
   announcement.
4. The launcher spawns `mission run-loop` detached. The loop takes the
   mission lease and publishes its runner record: that is the ownership
   of the mission. The checkout lease follows the lineage: the host turn's
   session runs `up` from its hooks under the same lineage and renews the
   lease instead of taking it over, as the lineage comment in the launcher
   already states; the launcher's dead announcement is retired by the
   ordinary sweep.
5. `resume` takes the same path. After a `stop` during a mission, `resume`
   finds `state.json` running, the runner record `stopped`, the lease
   marker released and the interrupted turn `turn-lost`; it opens the
   fence, arms, and the new loop re-lists the lost turn in its next
   assembled prompt, exactly as it does after a runner death today.
   `mission status` maps a `stopped` runner record to
   `status=stopped reason=metasystem-stop` and exits 13, so a driver stops
   polling and names `resume`.

## 8. The fleet form

`--all` derives a live-process inventory for the whole host and acts on
nothing else. Its sources:

1. The host registry: `registry.DefaultPath()` (the run-scoped home when
   `METASYSTEM_SUPERVISION_REGISTRY_HOME` is set), folded by
   `registry.Reduce` into the set of checkout paths that ever carried a
   claim, open or closed, with the open owners' identities.
2. One process enumeration (`EnumerateProcesses`), classified twice: by
   the engine's own shapes (`supervise owner`, `supervise component`,
   `steward run`, `mission run-loop`, `run wrap`, `proof-run launch`,
   `proof-run watchdog`), each of which names its checkout or
   installation in argv (`--repo`, `--root`); and by the invoking
   installation's adapter signatures, scoped to a checkout by the census
   rule (working directory or argv path below it).
3. The records of every checkout path found by 1 or 2: jobs, mission
   runners, runs, the steward runner, the supervision lock.

A checkout is live when any source names a live identity for it, whatever
the registry says about its owner: a dead owner with a live runner, a live
mission or a live run is stopped. Each live checkout runs the per-checkout
transaction of sections 2 through 6 under its own header and lock, with
the invoking engine. The installation for signatures is the checkout's
own when it carries `bin/metasystem` (the path itself or its `metasystem`
subdirectory), else the invoking engine's, and the header says which:
`checkout <path> (signatures from <installation>)`. **A checkout whose directory no longer exists is reported, not stopped.**
Revision 2 promised to stop it from registry identities and argv shapes
alone; that promise is withdrawn. Without the state root there is no
lock, no fence and no job, run, mission or runner record, so the only
identities are the registry's owner claims and command-line shapes, and
this page's own rule forbids signalling on a shape: a shape cannot be
told from a human session. The fleet form prints `checkout <path>:
directory gone; not signalled (no records)` and then probes the
registry's open owner identity for that path by kernel identity: alive
gives `NOT STOPPED supervision-owner pid <pid> started <epoch>: checkout
directory gone; did: nothing, end pid <pid> yourself` and exit 1; dead or
absent gives `nothing live recorded`. The registry row is left to the
janitor's own checkout-gone reaping; `stop` appends nothing for such a
checkout. Wido may overturn this for the stronger fleet promise, at the
cost of a records-free identity design that does not exist today. A
registry that cannot be reduced is the one refusal of the fleet form
(section 9). `status --all` prints the same headers and lines without
acting.

## 9. Refusals

One function renders every refusal of the three verbs: `refuseProcessVerb`
in `cmd/metasystem/process_verbs.go`. It prints two lines to stderr and
returns the exit code the site names:

```text
metasystem stop: <one plain sentence>.
run: metasystem stop --repo <toplevel>
```

or, when the command cannot succeed from where it was typed:

```text
metasystem stop: <one plain sentence>.
at an agent-free terminal, run: metasystem stop --repo <toplevel>
```

| Refusal | Sentence | Second line |
|---|---|---|
| `stop` or `arm`, caller not `HUMAN` | stop is a human act at a terminal; this caller is `<class>` | the terminal form with the resolved `--repo` |
| classification failed | the caller's ancestry could not be read: `<error>` | the terminal form |
| `--repo` outside a git repository | `<path>` is not inside a git repository | `run:` with `--repo <a path inside the checkout>` in words |
| no installation at the toplevel and no `--installation` | `<toplevel>` carries no metasystem installation | `run: metasystem stop --repo <toplevel> --installation <dir>` with the words "where `<dir>` holds this checkout's `bin/metasystem`" |
| `--installation` without `bin/metasystem` | `<dir>` carries no engine | the same command with the words |
| `--all` with `--repo` or `--installation` | `--all` acts on every checkout on this host and takes no `--repo` | the `--all` command alone |
| `--all`, registry unreadable or corrupt | the host registry `<path>` could not be reduced: `<error>` | the per-checkout command for the working directory |
| `arm` with `--all` | arm takes one checkout | the `arm` command with the resolved `--repo` |
| `arm` while a stop is in progress | a stop by pid `<pid>` holds the checkout | `run: metasystem status --repo <toplevel>` |
| `arm` under `stop-incomplete` with a survivor | `<component>` pid `<pid>` started `<epoch>` survived the last stop | `run: metasystem stop --repo <toplevel>`, then the words of section 2 |
| `arm` under `stop-incomplete` listing a job owned by another machine | job `<id>` is owned by machine `<other>` and is not terminal | the words "cancel it from `<other>` with `metasystem delegate --cancel <id>`", then the `arm` command |
| a creator's second fence read (section 2) finds the fence closed | the metasystem was stopped for `<toplevel>` while `<thing>` started; it has been ended | `at an agent-free terminal, run: metasystem arm --repo <toplevel>` |
| `arm`, enrollment or launch failure | the sentence `steward arm` or `up` prints today | words: their remedy, then the `arm` command |
| `steward arm` or `steward restart` under a closed fence | the metasystem is stopped for `<toplevel>` since `<time>` | `run: metasystem arm --repo <toplevel>` |
| `dispatch.ClaimLaunch`, `run launch/register/adopt`, `proof-run launch`, `mission run-loop` under a closed fence | the metasystem is stopped for `<toplevel>` since `<time>` | `at an agent-free terminal, run: metasystem arm --repo <toplevel>` |
| `mission start` or `resume` under a closed fence, caller not `HUMAN` | the same sentence | `at an agent-free terminal, run: metasystem mission start --mission <id>` (or `resume`) |

`stop` never refuses because something is already stopped, dead or
absent; those are lines in the report, not refusals. The `REFUSED-STOPPED`
outcome of `delegate` carries the same two lines in its `detail`.

## 10. Fixtures

Every scenario runs under a run-scoped registry home and fixture human
authority in a scratch checkout with the fake runtime, and compares
stdout line for line against the grammar of section 6.

Escalation seams, named honestly. The owner installs orderly signal
handling always and the crash seam exits, so no shipped seam makes an
owner or component ignore TERM. The build adds four fixture-only seams,
each refused outside a fixture-mode root: `supervise owner --ignore-term`
and `supervise component --ignore-term` (the owner passes the component
flag when `METASYSTEM_GO_COMPONENT_IGNORE_TERM` is set, the shape of the
existing crash-on-start seam, and `launchOwner` passes the owner flag when
`METASYSTEM_GO_OWNER_IGNORE_TERM` is set); `steward run` honouring
`METASYSTEM_STEWARD_RUNNER_IGNORE_TERM`; `mission run-loop` honouring
`METASYSTEM_MISSION_RUNNER_IGNORE_TERM`. A run workload and a suite command
that ignore TERM need no seam: `bash -c 'trap "" TERM; sleep 600'`. The
fake runtime's host process traps TERM to complete (`fake.sh`), so the
host-ignores-TERM case uses one more fixture-only knob on the fake
runtime, `METASYSTEM_FAKE_HOST_IGNORE_TERM`, under which the fake host
ignores TERM and sleeps; a slow but cooperating component uses
`METASYSTEM_GO_COMPONENT_SLOW_STOP=<seconds>`, honoured only in a
fixture-mode root, which delays the component's orderly exit by that
long; a crashed stop uses `METASYSTEM_STOP_CRASH_AFTER=<step>`, honoured
only in a fixture-mode root, under which `stop` exits without cleanup
after the named section 4 step.
Survival after KILL cannot be produced with a real process, and liveness
is proved by kernel identity, never by the census process file; those
cases are package tests on the injectable seams that exist:
`armingOwnerLiveness` and `recordedComponentControl` in
`internal/supervise`, `runnerSignal` and `runnerStopWriter` in
`internal/steward`, the `Prober` and `GroupPresent` seams of `run.Store`,
`ProcComponents.signal`, and a `stopSignal` variable the build adds to
`internal/missionrunner` on the same pattern.

- **stop-everything** (new scenario in `scripts/agents/supervision-fixtures.sh`):
  arm a scratch checkout through `up` with a bed main identity, `steward
  arm` a runner, dispatch one fake job that holds its custody child
  (`custodial-critique`), `run launch` one wrapped run around a sleeping
  workload; `status` lists the job, the run, the runner, the owner, the
  watcher and the reaper as running; `stop`; assert the job record
  `cancelled`, the run record `ended-unknown` with the stop note, the
  runner identity dead, the owner, watcher and reaper identities dead,
  `lock.d` gone, the registry reducing to a closed owner with `exited
  reason=shutdown`, the fence record `closed / stopped`, the start-again
  line last, exit 0.
- **seat-survives** (same bed): after `stop`, the bed main identity (the
  `--pid` and start time given to `up`) is alive by kernel identity, its
  announcement file is byte-identical, and a Stop hook fired for that
  session the way the hook fixtures fire it returns `allow` with `up
  outcome=stopped` in the system message and no block on claimable work
  seeded in the bed's goal ledger.
- **status-is-live** (same bed): `status` before the stop names exactly
  the identities `stop` then prints; `status` after prints `nothing is
  running` and the fence line; a second `stop` prints the same and exits 0
  with no file under `artifacts/agents/` changed except the fence record.
- **stop-fence** (same bed, fence closed): each creation path is driven
  and refused, and a process table diff on the bed's tag prefix shows
  nothing created: `steward arm`, `steward restart`, `steward run`,
  `supervise watcher-pass` (which runs the steward repair; the pass
  records `steward fenced`), `run launch`, `run register`, `run adopt`,
  `proof-run launch`, `dispatch.sh dispatch` directly, `mission start` as
  the bed's agent-shaped identity, `mission run-loop` directly, `up`
  ordinary and recovery-only.
- **arm-again** (same bed): `arm`; assert the fence `open / armed`, a
  runner live, the owner, watcher and reaper live at a new generation; `up`
  from the bed main joins with `verified`.
- **arm-refuses-survivor** (same bed): a fence record `stop-incomplete`
  listing a live sleeping process by identity; `arm` refuses with the
  named identity and the `stop` command; end the process; `arm` succeeds.
- **mission-stop** (joins `scripts/agents/mission-fixtures.sh`, which
  already births fake-runtime missions with a held host turn): start a
  mission, wait for the host turn `running`; `stop`; assert the runner
  record `stopped`, the turn `failed / turn-lost` with the stop detail
  and `hostTermination` `term`, the host group gone by identity, the
  lease marker gone, `state.json` unchanged; `mission status` prints
  `stopped` and exits 13; `mission resume` under fixture human authority
  opens the fence, arms and reaches its next host turn; then the
  ignore-TERM variant: the runner seam set, `stop` prints the `killed`
  line for the runner and `stopped (TERM)` for the host turn; then the
  host-ignores-TERM variant: `METASYSTEM_FAKE_HOST_IGNORE_TERM` set and
  the runner cooperating, `stop` prints `stopped (TERM, runner
  concluded)` for the runner and `killed (by the runner, TERM ignored)`
  for the host, and the turn record carries `hostTermination` `kill`.
- **proof-run-stop** (joins `scripts/agents/suite-progress-fixtures.sh`,
  which drives `proof-run launch` with fixture commands): `run launch` a
  wrapped run whose workload is `proof-run launch` of a suite whose
  command sleeps, so the launcher and watchdog live in the run's group;
  `stop`; assert, in this order, the suite identity gone, the watchdog
  done marker written, the launcher and watchdog identities gone, the
  run's exit sidecar written by the wrapper, the run record `red` from
  the sidecar, and the printed lines: the proof-run lines before the run
  line `concluded red (suite stopped)`; the TERM-trapping variant prints
  `killed` for the suite and the same run line.
- **creator-race** (package tests, deterministic): `internal/stoptransition`
  with an injected family whose inventory is empty on the first pass and
  names one live item carrying the pre-close generation on the second,
  asserting the item is stopped in the late-arrivals pass and printed
  with `(arrived during stop)`; and a family that keeps producing items,
  asserting three passes then `NOT STOPPED ... arrived during stop` and
  exit 1. `internal/run` with an injectable fence reader that reads open
  before `Launch` and closed after the wrapper is spawned, asserting the
  launch is failed with the stop note and the group is gone; the same
  shape in `internal/dispatch` (`ClaimLaunch` refuses after start and the
  husk carries `refusalClass` `stopped`), `internal/proofrun` (the
  launcher runs its ladder against its own suite), `internal/missionrunner`
  (the loop releases the lease and exits) and `internal/steward` (the
  runner exits before its first tick).
- **slow-owner** (new scenario in `supervision-fixtures.sh`): owner with
  two components, `METASYSTEM_GO_COMPONENT_SLOW_STOP=4` so the orderly
  teardown takes about eight seconds; `stop` prints `stopped (TERM,
  exited reason=shutdown)` for the owner and `already gone (by the
  owner)` for both components, the registry carries `exited
  reason=shutdown` and no `reaped` row, and the owner's identity was
  never KILLed (the fixture reads the caller's report for `Signal`
  `term`).
- **crash-recovery** (new scenario in `supervision-fixtures.sh`): the
  stop-everything bed with `METASYSTEM_STOP_CRASH_AFTER=1`, so `stop`
  exits after the mission step with the fence `stopping`, a live steward
  runner, a live fake job and a live wrapped run; `arm` refuses naming
  all three with the `stop` command and the fence record reads
  `stop-incomplete` listing exactly those identities; `stop` again
  completes and prints the three; `arm` succeeds.
- **remote-job** (joins `scripts/agents/dispatch-fixtures.sh`): a
  non-terminal job record seeded with `machineId` `other-machine`; `stop`
  prints the `NOT STOPPED job ... owned by another machine` line naming
  it, exits 1, and the fence record lists the job; `arm` refuses naming
  the machine and the cancel words; the record patched terminal by the
  fixture, `arm` succeeds.
- **orderly-owner-unproven-component** (package test in
  `internal/supervise`): the owner exits orderly and appends `exited`; a
  `recordedComponentControl` whose signal is a no-op and whose group stays
  present; assert the report carries `not-stopped` for the component,
  the registry log gained no event after `exited`, and `sweepPending` is
  not set anywhere.
- **ignored-signal** (new scenario in `supervision-fixtures.sh`): owner,
  component and steward runner launched with their ignore seams; `stop`
  prints one `killed (TERM ignored)` line per process, the registry
  carries `reaped reason=shutdown-escalated` with the killed list, exit 0.
- **survives-kill** (package tests): `internal/supervise` with a
  `recordedComponentControl` whose signal is a no-op and whose group stays
    present, and an `armingOwnerLiveness` that stays alive (the owner never
  appended its own terminal row), asserting the `not-stopped` outcome and
  the escalated row with `sweepPending` true, which is the one case that
  row is allowed;
  `internal/steward` with `runnerSignal` a no-op against a real runner,
  asserting the outcome; `internal/run` with `GroupPresent` fixed true;
  `internal/missionrunner` with `stopSignal` a no-op; and
  `internal/stoptransition` asserting that a `not-stopped` outcome in any
  family renders `NOT STOPPED`, sets the fence `stop-incomplete` with the
  identity listed, continues to the next family and exits 1.
- **fleet** (new scenario in `supervision-fixtures.sh` with three scratch
  checkouts under one run-scoped registry home): arm two; on the third,
  arm then kill the owner by identity so the registry reads a dead owner
  while its steward runner and a wrapped run stay alive; `status --all`
  lists all three; `stop --all` stops all three and prints the third's
  runner and run lines; a fourth registry path whose directory was
  removed, with a sleeping process launched in the `supervise owner`
  shape naming that path and its identity recorded as the registry's open
  owner, prints `directory gone; not signalled (no records)` and the `NOT
  STOPPED` owner line, exits 1, and the process is alive afterwards by
  kernel identity; with that process ended the same run prints `nothing
  live recorded`; `--all --repo` prints the `--all` command.
- **wrong-terminal** (joins `scripts/agents/goal-cli-fixtures.sh`): `stop`
  from a shell the fixture table marks without a terminal refuses with the
  `at an agent-free terminal, run:` line; the line, run under fixture
  human authority, succeeds; `status` from the same shell succeeds.
- **seat-refused** (joins `scripts/agents/dispatch-fixtures.sh`): a fake
  delegate runs `stop` and is refused with class `DELEGATE` and the
  terminal line, and runs `status` and is answered; the bed's records are
  unchanged.
- **fence-readers** (package tests): `internal/up` asserts the `stopped`
  outcome, exit 0 and remedy for ordinary and recovery-only options;
  `internal/steward` asserts `FENCED` from `RepairEnrolledRunner` and
  `EnsureRunner` and the health prefix; `internal/dispatch` asserts
  `REFUSED-STOPPED` from `ClaimLaunch`; `internal/run` asserts the three
  refusals; `internal/goal` asserts the verdict rule with a seeded
  claimable goal; `internal/stopfence` asserts the lock's contention and
  dead-holder rules and the crashed-stop reading.

## 11. Agnosticism and boundaries

No runtime is named in core: the untracked and host lines print the
runtime the configured signature matched, which is adapter data; the
engine's own process shapes name the engine's verbs. Decisions live in Go:
`internal/stopfence` (record, lock, reader), `internal/stoptransition`
(inventory, order, lines, the transition owner), `internal/up` (the
`stopped` outcome), `internal/steward` (`Disarm` escalation and report,
`FENCED`, the health prefix), `internal/supervise` (the report and the
escalated row), `internal/dispatch` (`REFUSED-STOPPED`), `internal/run`
(the extracted stop and the three refusals), `internal/proofrun` (the
exported ladder), `internal/missionrunner` (the signal path, the `stopped`
status, the extracted dead-runner release, the fence at launch and loop
start), `internal/goal` (the verdict rule), and `cmd/metasystem/process_verbs.go`
for parsing, the gate and printing. `dispatch.sh` relays one field and
`supervision-hook.sh` one notice line. `ShutdownAt`, the cancel path, the
takeover sweep's proof, `terminateGroup` and the watchdog's ladder are
called or extended, never copied. The registry format, the owner, runner,
job, run and mission records keep their fields; the additions are the job
record's `cancelEscalatedToKill` and `fenceGeneration`, the run record's,
mission runner record's, steward runner record's and owner record's
`fenceGeneration`, the turn record's `hostTermination`, the mission runner
record's `stopped` status, the watchdog's `--fence-generation` argument,
and the fence record. No record family this touches decodes with unknown
fields refused (`DisallowUnknownFields` appears in none of `dispatch`,
`run`, `missionrunner`, `steward` or the owner record), so the additive
fields are safe for every existing reader. Nothing about the goal ledger
changes.

## 12. Scope of the build

Changes: the two new packages and `cmd/metasystem/process_verbs.go` with
the dispatch and usage lines in `main.go`; `internal/up` (fence read, the
`stopped` outcome, the report printed by `Shutdown`); `internal/supervise`
(`ShutdownReport`, outcome-returning stops, the escalated row, the owner
and component ignore-TERM seams); `internal/steward` (`Disarm`, `FENCED`,
the `arm` composition, the health prefix, the fence in `steward run`, the
runner seam); `internal/dispatch` (the fence in `ClaimLaunch`);
`internal/run` (the fence in `Launch`, `Register`, `Adopt`, the extracted
stop); `internal/proofrun` (the fence in `LaunchSuite`, the exported
ladder); `internal/missionrunner` (signal handling, the intent file, the
`stopped` status and its `Status` case, the extracted lease release, the
fence at launch and loop start, the human check that opens it, the seam);
`internal/goal` (the verdict rule); `dispatch.sh` (one field);
`supervision-hook.sh` (one notice line); the fake runtime's host
ignore-TERM knob in `scripts/agents/adapters/fake.sh`; the fixture
scripts and package tests of section 10.

Reject this design at the build if implementing it needs a creator to
hold the transition lock through a process start, a signal sent on a
command-line shape without a record identity, a registry event outside
the reducer's contract, or a change to any file family outside this
list.

Unchanged: `up --shutdown` and `arm-supervision.sh` as internal callers;
the cancel path's ladder and locks; the registry's events and reduction;
the job status graph; every announcement, lease and goal-ledger byte;
`steward arm`, `steward restart` and `steward disarm` as long forms;
`session stop`; the mission state machine and its ledger.

## 13. Build decisions after the closing critique (revision 3 addendum)

The closing critique (`records/misc/metasystem-stop-critique-r3.md`, six
material findings, all accepted) found that two round-2 folds moved their
problem, two inventories miss a live process, the fleet branch names an
identity the registry does not keep, and the scope needs slicing. The
design ladder is closed; these are the build's decisions, and where they
contradict an earlier section, they win.

**13.1 Creation claims, the completion barrier (STOP-R3-001).** The
generation handshake of section 2 stays, and gains a barrier. Before a
creator publishes or spawns anything it opens a claim:
`stopfence.Creating(stateRoot, verb, generation, identity)` writes
`artifacts/agents/supervision/creating/<verb>-<pid>-<nonce>.json` with
`verb`, `generation`, the creator's kernel identity and `openedAt`. The
creator closes the claim (`Claim.Close`, the file removed) only after its
second fence read and whatever that read demanded. Where the spawn
happens in a shell seam, the claim crosses the seam by path: `ClaimLaunch`
opens it and returns its path in the claim result, `dispatch.sh` closes it
through `metasystem stopfence creating-close --claim <path>` after
`launch_adapter` returned and the record was patched running; `run
launch` opens it before `store.Launch` and closes after the wrapper
started; `proof-run launch` opens before starting the suite and closes
after the watchdog started; the mission launcher opens before spawning
the loop and closes after the spawn, and the loop opens its own before
publishing its runner record and closes after the lease is held; `steward
run` opens before writing `runner.json` and closes after its first fence
re-read; `up` and `arm` open before launching the owner and close when
`lock.d/owner.json` names it. `stop`, after closing the fence and before
the inventory, reads every claim whose `generation` is at or below the
generation it closed: a claim whose creator identity is dead is stale and
is removed (what it left behind is the inventory's to find); a claim with
a live creator is waited on, re-read every scaled half second, up to the
scaled ten seconds; a claim still open after that is `NOT STOPPED creator
<verb> pid <pid> started <epoch>: still creating after <wait>; did:
nothing` and exit 1. Only then does the inventory run, and the
late-arrivals pass stays as the second net. `Register` and `Adopt` after
the race fail their record with the note `stopped by metasystem stop`
and leave the foreign process untouched, the adopted-custody rule.
Proof: package tests inject the fence and claim store around each Go
seam; the shell seam in `dispatch.sh` is proven in the dispatch fixture
bed by a fixture-only pause, `METASYSTEM_FIXTURE_PAUSE_BEFORE_LAUNCH=<s>`,
honoured only in a fixture-mode root, between the Go claim and
`launch_adapter`, during which the bed runs `stop` and asserts that it
waited (no `still creating` line; the job ends `cancelled` or refused
stopped) and that no adapter process survives.

**13.2 The owner publishes its own teardown ceiling (STOP-R3-002).** The
caller reconstructs nothing. Whenever the owner publishes `state.json`
(`PublishState`), it writes `teardownCeilingSeconds`: the number of
identities it currently holds, older generations included, times the
component stop ceiling plus the post-kill proof interval, plus the
registry append lock wait. `ShutdownAt` reads that number (falling back
to forty-one seconds when the file is unreadable: two identities at five
and a half seconds plus thirty), scales it by the fixture scale, and
waits for the owner's death by kernel identity or its `exited` registry
event up to that ceiling; only then does it re-prove the owner and KILL.
The `slow-owner` fixture stays as the live case, and a package test with
an injected clock proves an owner alive until just under its published
ceiling is never KILLed.

**13.3 The fleet form is slice 2, and the directory-gone branch is
honest about what it has (STOP-R3-003).** Slice 1 refuses `--all` with
the sentence "the fleet form is not built yet" and the per-checkout
command. In slice 2 a registry checkout whose directory is gone prints
`checkout <path>: directory gone; no identity recorded; not signalled`,
is not a survivor and does not change the exit code, because
`registry.Reduce` keeps no owner process identity and probing a shape is
forbidden; the promise to probe in section 8 is withdrawn. The fleet
fixture then uses production-shaped registry history (arm a checkout,
remove its directory) and a real announced session that must survive.

**13.4 Open turns are inventoried on their own (STOP-R3-004).** Section
3 step 2 becomes two reads: every running runner record whose identity is
alive, and, independently, every non-terminal turn record under every
mission's `turns/` whose group is alive and owned by the recorded tag,
whether or not a runner is alive. Section 4 step 1 ends an orphaned turn
through the same tag-proven group path `cleanupStaleLease` uses and
patches it `failed / turn-lost` with the stop detail. The `mission-stop`
fixture gains the case: a dead runner record beside a live owned turn
group; `stop` ends the group and prints its line.

**13.5 The proof-run launcher publishes a record (STOP-R3-005).**
Section 3 step 6 reads `artifacts/agents/proof-runs/<suite>.json`, which
`LaunchSuite` writes after starting the suite and the watchdog: the
suite name, root, `fenceGeneration`, and the launcher, suite and watchdog
kernel identities, status `running`; the launcher patches it `done` when
it touches the done file. The watchdog's argv still carries the suite
identity for the watchdog's own use, but the inventory and the stop act
on the record's identities, so a dead watchdog beside a live suite is
stopped by identity. The `proof-run-stop` fixture gains that case.

**13.6 Slices (STOP-R3-006).** Slice 1, this build: `stop`, `status` and
`arm` for one checkout; the fence, its lock, the claims and the
handshake; every local family (jobs, mission runner and turns, runs,
proof-run suites, the steward runner, supervision); the report and the
refusals; the seat's own session; every section 10 scenario except
`fleet`. Slice 2: the fleet form of section 8 as amended by 13.3, with
the `fleet` fixture. Nothing in slice 1 may make slice 2 harder: the
inventory and the transaction are per checkout from the start.
