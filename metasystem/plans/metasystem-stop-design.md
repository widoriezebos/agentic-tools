# One word stops the metasystem: `metasystem stop`, `status` and `arm` (goal metasystem-stop-verb)

- Status: design, awaiting one critique
- Goal: metasystem-stop-verb (member of the arc verbs-match-intent; its
  slice 0 absorbs this page as the human page's three process verbs)
- Next step: critique once, then build behind the fixtures in section 8

## What is true today (read in the tree)

There is no verb that stops the metasystem. `metasystem delegate --cancel`
stops one job. `steward restart` stops a runner only to start another;
`steward disarm` ends the runner alone and leaves supervision up. The one
whole shutdown is `metasystem up --shutdown`, a flag the code labels an
internal fixture compatibility option, reached through
`scripts/agents/arm-supervision.sh --shutdown`. It stops supervision and
nothing else: no job is cancelled, the steward runner keeps ticking, and
the next session start or Stop hook arms everything again. The human's
recipe for a headless handover today is that script, typed by hand.

What the metasystem runs for one checkout, and where each thing records
itself:

- Delegate jobs: one record per job under `artifacts/agents/jobs/`, with
  `status`, `pid`, `pgid`, `instanceTag`, `machineId` and the custody
  process list. Non-terminal means status `pending-setup`, `pending` or
  `running`; terminal means `completed`, `failed`, `cancelled` or
  `timeout` (`TerminalStatus` in `internal/dispatch/record.go`). The cancel
  path (`dispatch.sh cancel`, reached by `delegate --cancel`) marks the
  record `cancelling` before any signal, winds down the custody process
  groups after proving ownership by tag, TERM then KILL after two seconds,
  and concludes `cancelled`.
- The steward runner: `artifacts/agents/steward/runner.json` names its
  pid, start time and boot identity; `liveRunner` proves it by that pair.
  `Disarm` writes the stop file and waits five seconds before one TERM; it
  never KILLs. `stopRunnerForReplacement` does TERM, two seconds, KILL.
- The supervision owner, watcher and reaper: `lock.d/owner.json` names the
  owner by pid, start time and tag; `state.json` names the watcher and
  reaper of the current generation. `ShutdownAt` in
  `internal/supervise/arming.go` refuses a lock whose owner tag is outside
  this checkout's prefix, refuses a live owner the host registry records
  for another checkout, refuses an uninspectable owner, writes the
  shutdown intent, sends TERM to the owner's group, waits five seconds,
  KILLs, then stops every recorded and tag-discovered watcher and reaper
  the same way and releases the lock. The registry then reads `exited
  reason=shutdown` from the owner, or `reaped reason=shutdown-escalated`
  from the caller.
- The narrator is not a process. The steward runner records a `narrator`
  component attempt on every tick and the owner appends a cycle trace to
  its own log; health reads the runner's record as `narrator-freshness`.
  It stops when the runner stops.
- The host registry `~/.metasystem/armed-checkouts.jsonl` is an append-only
  event log; `registry.Reduce` folds it into open and closed owner
  publications and claims per checkout path. Only the reduction says what
  is live.

Two authority facts govern the verbs. `steward arm` and `steward restart`
admit only a caller that `lease.ClassifyAt` classifies `HUMAN`: a process
with a controlling terminal and no agent, delegate, supervision or steward
ancestor (`requireHumanStewardEnrollment` in
`cmd/metasystem/steward_verbs.go`). `up --shutdown` instead requires the
checkout holder, which a human always is. And `up` arms on every session
start and every Stop hook, so anything stopped without a standing record
comes back at the seat's next turn end.

## 1. The verbs and their grammar

Three verbs at the top level, beside `up`, `health` and `watch`, dispatched
before the family table in `cmd/metasystem/main.go` and listed in its usage
text:

```text
metasystem stop   [--repo <path>] [--all]
metasystem status [--repo <path>] [--all]
metasystem arm    [--repo <path>] [--temporary-human-word <word> --review-by <date>]
```

No `--by`: the gate proves the human, and the record names the terminal.
No other flags. The checkout is found from the working directory the way
`up` finds its scope: `git -C <repo> rev-parse --show-toplevel`, with
`--repo` defaulting to `.`. The installation is the toplevel when it
carries `bin/metasystem`, else `<toplevel>/metasystem` when that does; the
state root is `stateroot.RootForInstallation` of that installation, which
is what `up` arms and what the registry records. A toplevel with neither is
refused (section 6). The owner tag prefix is
`metasystem-supervision-owner-<slug(toplevel)>-`, exactly as `up` computes
it. `--repo` is accepted for a terminal outside the checkout and is never
needed inside one.

`arm` is the start-again act and the one name the stop verb prints. It is
`steward arm`'s transaction (enrol the invoking engine, start the runner;
`already armed` when a live runner stands), preceded by clearing the
stopped record of section 2 and followed by the recovery-only arming of
`up` (`RecoverOnly` with `IfDown`: start only the missing supervision
rings, no announcement, no lease), so one word brings back everything one
word took down. `steward arm` and `steward restart` stay as the long forms
the plumbing and the re-arm remedies call; their help lines gain "(long
form of metasystem arm)" and the printed start-again line names `arm`
only. `--temporary-human-word` and `--review-by` pass through to
`ArmTemporary` unchanged.

`stop` and `status` print the same list; `stop` acts on it. `status` exits
0 when nothing is running, 0 when things are running, 1 only when it could
not read what is running. `stop` exits 0 when every listed thing is
stopped or nothing was running, 1 when something could not be stopped.

## 2. The stopped record

The first act of `stop`, before any signal, is to write
`artifacts/agents/supervision/stopped.json` under the state root:

```json
{"schemaVersion":1,"stoppedAt":"<RFC3339>","terminal":{"pid":..,"pidStartedAt":..},"checkout":"<toplevel>"}
```

It is written atomically and durably (`atomicfile.WriteText`), and it is
the fence that makes a stop stay stopped:

- `up`, in every mode, reads it after `host-preflight` and before the
  enrollment open. When it stands, `up` prints `component=stopped
  outcome=standing detail="stopped at <time>"` and the aggregate `up
  outcome=stopped remedy="metasystem arm --repo <toplevel>"`, exits 0, and
  does nothing else: no re-arm of a rebuilt engine, no announcement, no
  lease, no supervision, no runner. `ExitCode` treats `stopped` as
  success. The recovery-only mode and the optional scheduler entry take
  the same path, so neither undoes a stop.
- `metasystem delegate` (dispatch, follow-up and revive) refuses under it
  with `REFUSED-STOPPED` and the human act line of section 6. The steward's
  revival goes through `delegate --revive`, so a runner that outlives a
  stop by seconds cannot start work.
- `mission start` and `mission resume` refuse under it unless their
  caller classifies `HUMAN`; a human's `mission start` clears it and arms
  as it does today. That is the headless handover (section 5).
- `arm` clears it first, under the steward arm lock, then arms.
- `health` prefixes its line with `HEALTH STOPPED` and every role whose
  remedy would say `metasystem up` says `metasystem arm --repo <toplevel>`
  instead; roles are rendered dead without the failure counters advancing,
  so a stop raises no alert episode.

Nothing else reads it. The goal ledger, the announcements and the lease
are untouched by `stop`.

## 3. The order of stopping, and why

Per checkout, after the record is written, in this order, each step
against the state read at that moment:

1. **Jobs, through the cancel path.** Every record under
   `artifacts/agents/jobs/` whose status is non-terminal and whose
   `machineId` is this machine's (or absent) is cancelled by running the
   engine's own `delegate --cancel <job>` transaction, one job at a time.
   The watcher and reaper are still up, so each conclusion is reaped,
   mirrored and censused exactly as an ordinary cancel is. A record naming
   another machine is reported, not touched. The cancel path's own TERM
   then KILL ladder is the orderly path for a job; `internal_cancel` gains
   one field on the concluding patch, `cancelEscalatedToKill: true`, when
   its wind-down reached KILL, so the line can say so.
2. **The steward runner.** `Disarm` becomes the one runner stop: write the
   stop file, TERM the recorded identity when it does not exit within the
   scaled five seconds, KILL after two more, prove death by `liveRunner`
   after each step, and return which signal ended it. `steward disarm`
   keeps calling it. The runner goes second because a live runner would
   tick against a set that is being torn down and report every missing
   component as an incident; with the stopped record standing it cannot
   start work anyway.
3. **Supervision: owner, then watcher and reaper.** `up.Shutdown`
   unchanged: `ShutdownAt` signals the owner, whose exit handler tears down
   its held watcher and reaper by identity, then sweeps the recorded and
   tag-discovered components and releases the lock. The owner goes last
   because it is the one process that can prove its components dead and
   append the terminal registry row itself; a caller that has to escalate
   appends `reaped reason=shutdown-escalated` in its place, as today.
   The narrator line follows the runner's and the owner's: it is reported
   as stopped with them.

The human's seat session is never touched. The session process, its
announcement, its lease and its delegate worktrees stay; the census will
list it as `ANNOUNCED`. What its Stop hook does once supervision is gone:
the hook runs `up`, which returns `stopped` with exit 0, so no arming
failure is recorded and nothing blocks; the check-in tail carries the
aggregate line's remedy the way it carries a re-arm notice today. The turn
verdict (`goal.TurnVerdict`) treats a standing stopped record as the same
human-authorized quiet that a consumed session stop grants, without
consuming anything, because the same terminal authority wrote it: the seat
may end every turn while the metasystem is stopped, and `ShouldBlock` is
false for the idle-backlog rule. `metasystem session stop` stays what it
is: one seat, one turn end, the metasystem running; `stop` is every seat
in the checkout and nothing running. `stop` never mints a session stop.

## 4. Omnipotence with honesty

One printed line per thing, on stdout, in the order acted on. The line
grammar is fixed so a fixture can parse it:

```text
checkout <toplevel>
job <id> <status> pid <pid> pgid <pgid> role <role>: cancelled (cancel path, TERM)
job <id> <status> pid <pid> pgid <pgid> role <role>: cancelled (cancel path, KILL after TERM was ignored)
job <id> <status> machine <other>: not ours, not touched
steward-runner pid <pid> started <epoch>: stopped (TERM)
steward-runner pid <pid> started <epoch>: stopped (KILL after TERM was ignored)
narrator: stopped with the steward runner
supervision-owner pid <pid> tag <tag> generation <n>: stopped (TERM, exited reason=shutdown)
supervision-owner pid <pid> tag <tag> generation <n>: killed (TERM ignored; reaped reason=shutdown-escalated)
repo-watcher pid <pid>: stopped (by the owner)   |   stopped (TERM)   |   killed (TERM ignored)
job-reaper pid <pid>: stopped (by the owner)     |   stopped (TERM)   |   killed (TERM ignored)
untracked pid <pid> <runtime> <argv>: not the metasystem's, not touched
NOT STOPPED <component> pid <pid>: <reason>; did: <what the verb did about it>
stopped <toplevel>; start again: metasystem arm --repo <toplevel>
```

`status` prints the same lines with the verdict part replaced by the
state: `running`, `not running`, or for a job its status. When nothing is
running, `stop` prints `checkout <toplevel>`, `nothing is running` and the
start-again line, and exits 0; a second `stop` is that.

Every signal is sent to an identity, never a bare pid: the job path proves
its groups by tag before each signal; the runner is proved by pid, start
time and boot identity; the owner and its components by pid, start time
and tag through the three-way liveness rule. A pid that no longer carries
its recorded identity is reported `already gone` and is not signalled.
KILL is sent only after the orderly signal's wait expired and the identity
was re-proved beside it, and the line says so. A component that survives
KILL, an uninspectable identity, or a lock the owner tag prefix vetoes is a
`NOT STOPPED` line naming the reason and what the verb did (left the
record, left the lock, appended `reaped` with `sweepPending` true where
the registry contract allows) and the exit code is 1; the verb continues
to the next step rather than stopping at the first refusal, because a
human reading one report should learn everything at once.

`untracked` lines come from one read-only enumeration through the census
package (`EnumerateConfiguredProcesses` and the configured signatures,
scoped to the toplevel the way the watcher scopes them), run after
supervision is down so the seat can see what survived and whose it is.
The verb never signals an untracked process.

## 5. Authority

`stop`, `status` and `arm` are human acts at the enrolled terminal, gated
by the same function `steward arm` and `steward restart` use,
`requireHumanStewardEnrollment`, renamed `requireHumanTerminal` and moved
beside the new verbs so the four share one line of code. A fixture-granted
`HUMAN` classification is admitted exactly as it is for `arm`. `status`
shares the gate because it reads other processes' records and the audit's
human page lists it as a human verb; a seat that wants the same facts has
`supervise status` and `health`. This is the classifier gate, not the
enrolled-terminal proof of `internal/humanauthority` that the goal verbs
and `session stop` use (its refusal is `TERMINAL_NOT_REACHED`): a person
at a terminal who never ran `goal enroll-terminal` can still stop the
metasystem, exactly as they can arm it today.

Machinery does not call `stop`. The supervision step it wraps stays
reachable as the internal long form `up --shutdown`, unchanged, holder
gated, writing no stopped record, marked internal in the help text; the
fixture beds and the proof-run watchdog keep calling it through
`arm-supervision.sh --shutdown`, and nothing new may start calling it. The
headless handover is: the human ends the seat's session, runs `metasystem
stop`, runs `metasystem mission start`. `mission start` from a `HUMAN`
caller clears the stopped record inside its launch lock and arms as the
mission-runner identity; from any other caller it refuses under the record
(section 2). Nothing here changes what `mission start` does when no record
stands.

## 6. Refusals

One function renders every refusal of the three verbs: `refuseProcessVerb`
in a new file `cmd/metasystem/process_verbs.go`, beside the verbs
themselves. It prints two lines to stderr and returns the exit code the
site names:

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
| caller not `HUMAN` | stop is a human act at a terminal; this caller is `<class>` | the terminal form with the resolved `--repo` |
| classification failed | the caller's ancestry could not be read: `<error>` | the terminal form |
| `--repo` outside a git repository | `<path>` is not inside a git repository | `run:` with `--repo <a path inside the checkout>` written in words |
| no installation at the toplevel | `<toplevel>` carries no metasystem installation (`bin/metasystem` is absent there and in `metasystem/`) | `run:` with `--repo <path>` in words |
| `--all` with `--repo` | `--all` acts on every enrolled checkout and takes no `--repo` | the `--all` command alone |
| `--all`, registry unreadable or corrupt | the host registry `<path>` could not be reduced: `<error>` | the per-checkout command for the working directory |
| `--all`, a live checkout whose installation cannot be found | `<checkout>` is live in the registry but carries no installation | `run:` with `--repo <checkout>`, so the human can name the installation by standing in it |
| `arm` with `--all` | arm takes one checkout | the `arm` command with the resolved `--repo` |
| `arm`, enrollment or launch failure | the sentence `steward arm` prints today | words: the remedy `steward arm` prints today, then the `arm` command |
| `delegate` under the stopped record | the metasystem is stopped for `<toplevel>` since `<time>` | `at an agent-free terminal, run: metasystem arm --repo <toplevel>` |
| `mission start` or `resume` under the stopped record, caller not `HUMAN` | the metasystem is stopped for `<toplevel>` since `<time>` | the same human act line |

`stop` never refuses because something is already stopped, dead or
absent; those are lines in the report, not refusals.

## 7. The fleet form

`--all` derives the live set from the host registry and acts on nothing
else. It reads `registry.DefaultPath()` (the run-scoped home when
`METASYSTEM_SUPERVISION_REGISTRY_HOME` is set, as every fixture does),
folds the frames with `registry.Reduce`, and takes the set of checkout
paths that have an open published owner or an open claim. A path whose
recorded owner is provably dead by the three-way rule is not live: it is
the janitor's and is printed as `<checkout>: owner dead, left to the
janitor`. For each live path the verb resolves the installation as section
1 does for a toplevel (the path itself, else `<path>/metasystem`) and
requires `stateroot.RootForInstallation` of it to equal the recorded path;
a mismatch is a refusal line for that checkout, not a stop. Then it runs
the per-checkout transaction of section 3 with the invoking engine, each
checkout under its own `checkout <path>` header, and prints one
start-again line per checkout. `status --all` prints the same headers and
lines without acting. The registry's rows are never rewritten by the verb;
the owner and the escalating caller append to it exactly as today.

## 8. Agnosticism and boundaries

No runtime is named: the untracked lines print the runtime the configured
signature matched, which is data from the adapters, and the cancel path
reaches each runtime through its adapter as today. Decisions live in Go
(`internal/up` for the stopped record and the `stopped` outcome,
`internal/steward` for the runner stop and `arm`, `internal/dispatch` for
the refusal under the record, `internal/goal` for the verdict rule, the
new `cmd/metasystem/process_verbs.go` for the three verbs and their
refusal renderer); `dispatch.sh` relays one field. `up.Shutdown` and
`ShutdownAt` are called, not copied. The job cancel transaction is called
through the engine's own `delegate --cancel` path, not re-implemented.
The registry format, the owner and runner records and the job record's
status graph are unchanged; the one job-record addition is the boolean
`cancelEscalatedToKill`, which no reader is required to know. Nothing
about the goal ledger changes.

## 9. Fixtures

Every scenario runs under a run-scoped registry home and fixture human
authority, in a scratch checkout with the fake runtime, and captures
stdout to compare line for line against the grammar of section 4.

- **stop-everything** (new scenario in `scripts/agents/supervision-fixtures.sh`,
  where the armed-steward beds and the `--shutdown` teardown already live):
  arm a scratch checkout through `up` with a bed main identity, `steward
  arm` a runner, dispatch one fake job that holds its custody child
  (`custodial-critique` release file) so it is `running` with a pgid;
  `status` shows the job, runner, owner, watcher and reaper as running;
  `stop`; assert every job record terminal with status `cancelled`, the
  runner identity dead and `runner.json` no longer proving a live runner,
  the owner, watcher and reaper identities dead, `lock.d` gone, the
  registry reducing to a closed owner with `exited reason=shutdown`, the
  stopped record present, the start-again line printed last, exit 0.
- **stop-twice** (same bed, continues): a second `stop` prints
  `nothing is running` and the start-again line, exits 0, and writes
  nothing: every file under `artifacts/agents/supervision/` and
  `artifacts/agents/jobs/` keeps its modification time, the stopped
  record included.
- **status-matches** (same bed): `status` before the stop lists exactly
  the identities `stop` then reports; `status` after lists each as `not
  running`; the two outputs differ only in the verdict part of each line.
- **stop-then-hook** (same bed): with the stopped record standing, fire
  `supervision-hook.sh` Stop for the bed's session the way the hook
  fixtures do; assert `decision` is `allow`, the system message carries
  `up outcome=stopped`, no owner, watcher, reaper or runner process
  appears, and `report turn-verdict` does not block on claimable work
  seeded in the bed's goal ledger.
- **arm-again** (same bed): `arm`; assert the stopped record is gone, a
  runner is live, the owner, watcher and reaper are live at a new
  generation, and `status` lists them; then `up` from the bed main joins
  with `verified`.
- **stop-refuses-work** (same bed, before `arm`): `delegate` under the
  record prints `REFUSED-STOPPED` and the human act line; `mission start`
  from a non-human caller (the bed's agent-shaped identity) refuses with
  the same line.
- **ignored-signal** (new scenario in `supervision-fixtures.sh`, beside the
  existing dead-owner and false-death beds): launch a runner replacement
  and an owner whose signal handling is disabled through the existing
  fixture crash and ignore hooks (the component `--crash-on-start` seam
  for the owner, a fake runner wrapper that traps TERM for the runner);
  `stop`; assert one `KILL after TERM was ignored` line for each, the
  registry's `reaped reason=shutdown-escalated` where the owner could not
  speak, exit 0 because everything ended.
- **survives-kill** (same scenario): a fixture process table
  (`METASYSTEM_CENSUS_PROCESS_FILE`) that keeps a recorded component
  identity alive after KILL; `stop` prints `NOT STOPPED` for it with what
  it did, exits 1, and still prints every later line.
- **fleet** (new scenario in `supervision-fixtures.sh` using two scratch
  checkouts under one run-scoped registry home): arm both; `status --all`
  lists both under their headers; `stop --all` stops both; a third
  registry path whose owner identity is dead prints the janitor line and
  is not signalled; the refusal for `--all --repo` prints the `--all`
  command.
- **wrong-terminal** (joins `scripts/agents/goal-cli-fixtures.sh`, whose
  refusal scenarios already capture the second line and run it): `stop`
  from a shell the fixture table marks without a terminal refuses with the
  `at an agent-free terminal, run:` line; the line, run under fixture
  human authority, succeeds.
- **seat-refused** (joins `scripts/agents/dispatch-fixtures.sh` behind the
  armed fake-runtime set): a fake delegate runs `stop` and `status` and is
  refused with class `DELEGATE` and the terminal line; the bed's records
  are unchanged.
- **health-and-up-under-record**: a package test in `internal/up` asserts
  the `stopped` outcome, exit code 0 and remedy for ordinary and
  recovery-only options with the record present; a package test in
  `internal/steward` asserts the health line prefix and remedy
  substitution; a package test in `internal/goal` asserts the verdict
  rule of section 3 with the record present and a seeded claimable goal.

## 10. Scope of the build

Changes: `cmd/metasystem/process_verbs.go` (new: `stop`, `status`, `arm`,
the refusal renderer, the shared human gate) and the dispatch and usage
lines in `main.go`; `internal/up` (stopped record read and write, the
`stopped` outcome); `internal/steward` (`Disarm` escalation and signal
report, the `arm` composition, the health prefix); `internal/dispatch`
(`REFUSED-STOPPED`); `internal/goal` (the verdict rule); the mission
launch (`HUMAN` check and clear under the record); `dispatch.sh` (one
field); `supervision-hook.sh` (one notice line keyed on `up
outcome=stopped`, the shape of the re-arm notice); the fixture scripts and
package tests named above.

Unchanged: `ShutdownAt`, `up --shutdown` and `arm-supervision.sh`; the
cancel path's ladder and locks; the registry's events and reduction; the
job status graph; every announcement, lease and goal-ledger byte; `steward
arm`, `steward restart` and `steward disarm` as long forms; `session stop`.
