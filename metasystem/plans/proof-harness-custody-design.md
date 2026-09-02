# Proof-harness process custody: a load group that owns itself, and a sweep a seat may run (proof-harness-process-custody)

Working Mode: design
Goal: `plans/goals/proof-harness-process-custody.md` (revision 4, claimed by m1)
Author: dispatch delegate custody-design-r1b, 2026-09-02, round 1
Builds on: `plans/recovery-analysis.md` section 4 (the leak sources) and
section 6.2 (the arc, slice S4). This design does not redo that tracing;
it cites it and adds only the file-and-line facts the two decisions
below need.

## 0. What this design owns and what it leaves alone

Two things, as the brief drew the line:

- (a) Custody of what a seat-run proof harness spawns: load generators,
  busy loops, fake adapters. A harness killed on any path leaves zero
  orphans.
- (b) A seat-runnable sweep of machinery that no live record owns.

The umbrella's slice S4 (`recovery-analysis.md:431`) keeps: runners and
owners recording their bed root and exiting when it vanishes, and
fixtures arming through one custodian. Neither (a) nor (b) needs S4's
bed-root rule, and this design takes no piece of it. The reason is a
fact of the tree: every runner, owner and component already carries its
bed in argv. The runner is launched as `steward run --repo <root>`
(`internal/steward/runner.go:449`), the owner as `supervise owner --repo
<root> ... --tag <tag>` (`internal/supervise/arming.go:599-600`), and a
component as `supervise component --component watcher --tag <tag> --repo
<root>` (`internal/supervise/arming_test.go:848`, the shape
`cmd/metasystem/supervise_component.go:61` parses). The sweep reads the
bed from argv; it never needs the process to have recorded it anywhere
else. S4's self-exit would shrink the sweep's victim list, which is
welcome and independent.

## 1. The facts the decisions rest on

| Fact | Where |
| --- | --- |
| The specimen harnesses are not in the tree. The twelve m2 and m3 loops were killed from a shell job list (`kill $LOADPIDS`) that the background-execution wrapper had already detached | goal Intent, `plans/goals/proof-harness-process-custody.md:4` |
| Runner, owner and components are launched with `Setsid`, so no harness process group contains them | `recovery-analysis.md:267-271`; `runner.go:453`, `arming.go:613`, `internal/supervise/proc.go:72` |
| The runner loop stops only on its stop file or a failed lock; a tick failure is printed and the loop continues | `runner.go:95-108` |
| Cleanup traps cover INT and TERM only; nothing survives KILL, sleep, a tmux pane kill or a hang kill | `recovery-analysis.md:314-315`; `scripts/validate-metasystem.sh:1509-1510`, `scripts/agents/supervision-fixtures.sh:414-416` |
| The last-resort cleanups are pattern kills over the bed path (`pgrep -f "$tmp"`) | `supervision-fixtures.sh:389-393`, `validate-metasystem.sh:1481-1484` |
| Preserved failure beds keep their directory by design while their processes keep writing into it | `validate-metasystem.sh:1486-1497`, `supervision-fixtures.sh:398-402` |
| Temp beds: bare `mktemp -d` and `metasystem-*.XXXXXX` templates under `${TMPDIR:-/tmp}`, and Go `t.TempDir` | `supervision-fixtures.sh:103`, `scripts/agents/dispatch-fixtures.sh:202`, `scripts/agents/health-fixtures.sh:101`; `recovery-analysis.md:318-321` |
| The dispatch-fixture bed named `agent-fixture` (the argv of the 303 leaked m1 runners) | `dispatch-fixtures.sh:427-445` |
| The census classifies a process as CUSTODY only by a live record joined on pid, start and tag; a process in a live run record's process group is CUSTODY with a `RUN <id>` label; everything else in scope is UNTRACKED | `internal/census/run.go:306-333`, `run.go:545-598` |
| Scope is cwd or an argv path below the checkout; nothing outside every live checkout is seen at all | `internal/census/scope.go:9-12`, `run.go:216-244`; `recovery-analysis.md:322-329` |
| Production enumeration is native (no `ps`): pid list, kernel prober for start and argv, `getpgid`; cwd is resolved natively per pid | `internal/census/production.go:43-72`, `78-94`; `internal/identity/enumerate_darwin.go:23,58`, `enumerate_linux.go:15,38` |
| An unreadable argv is absence of evidence and never authorizes anything | `internal/identity/identity.go:39-46` |
| The kill proof already exists in one place: a positioned tag in a known argv shape, joined with pid and start (`Killable`); group ownership is tri-state and an empty scan is INDETERMINATE (dab1dbdd) | `internal/janitor/killproof.go:15-63`, `129-171`, `206-222` |
| The D-4 janitor's target selection is built and pure, but no verb executes it and `reap-orphans.sh` does not exist | `internal/janitor/targets.go:70-129`; `records/supervision/supervision-lifecycle.md:576-620`; `ls scripts/agents/reap-orphans.sh` fails |
| The janitor family today has one verb, `headroom` | `cmd/metasystem/main.go:339-344`, `cmd/metasystem/janitor_verbs.go:19` |
| The proc family: `exists`, `group-exists`, `group-owned` (three-state, backed by `janitor.GroupOwnership`), `group-members` (refuses an undercount), `census`, `alive`, `classify`, `find-ancestor`, `acknowledge` | `main.go:44-60`, `cmd/metasystem/identity_probes.go:58-99`, `106-140` |
| The delegate cure for the same disease: a supervisor's kill domain is its own process group minus itself, enumerated by `proc group-members`, TERM then KILL, a death proof or a refusal | `scripts/agents/adapters/runtime-common.sh:232-250`; `scripts/agents/dispatch.sh:339-354` |
| The proof-run launcher puts a suite in its own group (`Setpgid`) with a sibling watchdog that signals the group by authenticated identity, TERM then KILL | `internal/proofrun/launcher.go:68`, `watchdog.go:209-213` |
| `util hold --tag` is the precedent for a tagged engine process that exists to be owned | `main.go:361`, `cmd/metasystem/hold.go:19-26`; shape `tagged-hold` at `killproof.go:57` |
| `run launch` spawns a wrapped command detached as a setsid leader and records custody `wrapped`; the census then labels the group CUSTODY `RUN <id>`; the lease sweep owns stale runs | `main.go:449-450`, `internal/run/verbs.go:109`, `census/run.go:319-330`, `internal/lease/sweep.go:54` |
| Owners must die before components or they relaunch them (KI-32 open; D-4's rule) | `memory/known-issues.md:39`; `supervision-lifecycle.md:593-603` |
| The shared-machine rule: kill only what you can prove is yours, by exact pid; never pattern-kill; tag what you launch | `docs/orchestration.md:221`, `224` |
| The steward treats UNTRACKED as blocking a death proof and a failed census as unprovable | `internal/steward/census.go:53-76` |
| Caller classes HUMAN, DELEGATE, UNTRUSTED, STEWARD; holder-only verbs classify the real parent pid | `internal/lease/classify.go:66-71`, `internal/authority/authority.go:24-58`, `cmd/metasystem/census.go:107-135` |
| The armed-checkout registry lives at `~/.metasystem/armed-checkouts.jsonl` unless `METASYSTEM_SUPERVISION_REGISTRY_HOME` points a fixture run elsewhere | `internal/registry/append.go:18`; `supervision-fixtures.sh:109`, `dispatch-fixtures.sh:206` |
| Fixture waits go through named, scaled ceilings; process-owning fixture beds run as child scenarios of one parent with a per-signal trap | `scripts/agents/fixture-budget.sh:26-73`, `230-248`, `341-345`; `scripts/agents/fixture-bed-scenarios.sh:18-28`, `31-44` |

## 2. Decision 1: harness custody is an engine verb, not a shell contract

### 2.1 The choice

The verb. A shell contract (one process group, `trap ... EXIT`, `kill --
-$pgid`) covers normal exit, error exit, INT, TERM and HUP of the
harness, and nothing else. The specimen died on exactly the path a shell
trap cannot see: the wrapper detached the shell, the job table went away,
and the trap fired in a shell that owned nothing. KILL of the harness and
a tmux pane kill are the same class. A trap is custody by the victim's
good behaviour; the delegate machinery stopped relying on that in
`runtime-common.sh:232-250` and the goal asks for the same cure.

So the custodian is a process the engine controls, in a group the engine
controls, that watches its parent's exact identity and winds its group
down by itself. No shell job table is ever the custodian; a harness that
forgets its trap still leaks nothing.

### 2.2 The verbs, exactly

Two new verbs in the `proc` family (`main.go:44-60`), implemented in a
new file `cmd/metasystem/load_verbs.go` over a new package
`internal/loadgen`.

```
metasystem proc load-generate --seconds N --workers K --tag TAG --bed DIR
                              --group own|inherit [--parent-pid P]
metasystem proc load-worker   --seconds N --tag TAG --bed DIR
                              --leader-pid P --leader-started-at S (internal)
```

Flags, all required unless marked:

- `--seconds N`: the ceiling. 1 through 86400. The leader winds the
  group down when it passes, whatever the harness does.
- `--workers K`: 1 through 64 busy workers.
- `--tag TAG`: the instance tag. Must begin with `load-` and be at most
  64 characters of `[A-Za-z0-9._-]`. The tag is the kill proof
  (`killproof.go:206-222`), so it is mandatory and positioned; a tag
  merely mentioned elsewhere in argv never matches (`killproof.go:25-28`).
- `--bed DIR`: the harness's bed. Must be an absolute, existing directory.
  Carried by leader and workers so the sweep can place them (section 3).
- `--group own|inherit`: `own` calls `setpgid(0, 0)` so the leader heads
  a fresh group; `inherit` keeps the caller's group. No default: the two
  launch contracts in 2.3 pass it explicitly, and a missing value is a
  usage error (exit 2).
- `--parent-pid P` (optional): the process whose death ends the load.
  Default `getppid()` read at start. The leader probes it once
  (`identity.KernelProber{}.Probe`, `identity.go:34-47`) and keeps the
  exact reference; a parent that cannot be probed alive at start is a
  refusal (exit 3, "parent identity unreadable; nothing launched").

Behaviour of the leader:

1. Group per `--group`. Then it spawns K workers as the same executable
   (`os.Executable()`, the pattern `proofrun/launcher.go:201`) running
   `proc load-worker` with the leader's pid and start second, in the
   leader's group (no `Setsid`, no `Setpgid`).
2. It prints one line to stdout before anything else and flushes it:
   `load-generate pid=<pid> pgid=<pgid> tag=<tag> workers=<k> seconds=<n>`.
   A shell harness that wants a belt beside the braces can kill
   `-<pgid>` in its trap; it never has to.
3. It loops at 250 ms: parent dead by exact identity
   (`identity.AliveRef`, `identity.go:187`) or ceiling passed or a TERM,
   INT or HUP received, in any order, starts the wind-down.
4. Wind-down is the delegate's kill domain (`runtime-common.sh:232-250`)
   in Go: members of the leader's group except itself
   (`supervise.GroupMemberPids`, `proc.go:258-287`), TERM each by exact
   pid, up to 2 s of 50 ms polls, KILL the rest, up to 2 s more, then a
   death proof: an empty member list. An indeterminable enumeration
   (`proc.go:266-278`) is printed as `wind-down unproven` and exit 3; a
   proven-empty group exits 0; a ceiling-triggered wind-down exits 0 too,
   because reaching the ceiling is the contract, not an error.
5. `--group own` and `--group inherit` wind down identically; with
   `inherit` the leader signals only members it spawned (it keeps their
   pids and start seconds and re-proves each before signalling), never the
   whole inherited group, because that group is the caller's.

Behaviour of a worker: a tight loop that does arithmetic on a register,
checks the clock every 10,000 iterations against its own `--seconds`
deadline, and every 500 ms probes the leader by pid and start second
(`identity.AliveRef`). Leader dead or deadline passed: exit 0. Any TERM,
INT or HUP: exit 0. Workers never fork.

The worker's leader watch is what makes the custodian itself killable.
KILL the leader and the workers notice within 500 ms and leave.

Ownership proof for both: two new rows in `janitor.DefaultShapes`
(`killproof.go:41-63`): `{Name: "load-generate", Includes: {"metasystem",
"proc", "load-generate"}, TagFlag: "--tag"}` and the same for
`load-worker`. `proc group-owned` (`identity_probes.go:58-99`) then
proves a load group the way it proves an adapter group, unchanged.

### 2.3 The harness contract

A seat-run proof harness starts load in exactly one of two ways.

Contract A, recorded run (preferred when a checkout root is at hand):

```
metasystem run launch --root <checkout> --id <run-id> --kind custom \
  --display "load: <why>" --log <bed>/load.log --stale-after-min <N/60+5> \
  -- metasystem proc load-generate --seconds N --workers K \
     --tag load-<run-id> --bed <bed> --group inherit
```

`run wrap` is the setsid leader (`main.go:450`); `--group inherit` keeps
the workers in the run's group, so the census classifies every one of
them CUSTODY `RUN <id>` (`census/run.go:319-330`), the run watcher and the
lease's stale-run sweep own the group (`lease/sweep.go:54`), and the
harness never signals anything. The leader's parent is `run wrap`; the
wrap's death ends the load.

Contract B, bare launch (a harness outside any checkout):

```
metasystem proc load-generate --seconds N --workers K \
  --tag load-<harness-name>-<hex> --bed <bed> --group own &
```

The leader's parent is the harness shell. The harness may keep the
printed pgid and kill `-<pgid>` in its EXIT trap. It does not have to.

Either way the harness never runs `yes`, `while :`, or a shell loop, and
never keeps a job list. A harness that does is out of contract and its
loops are unprovable (section 3.4).

### 2.4 Exit paths

| Path | What happens | Covered by |
| --- | --- | --- |
| Harness ends normally | Shell exits; leader sees its parent identity dead within 250 ms; group wound down; death proven | parent watch (2.2 step 3) |
| Harness exits on error (`set -e`) | Same | parent watch |
| INT to the harness | Shell dies or traps; either way the shell's exit ends the parent identity | parent watch; optional trap kill |
| TERM to the harness | Same | parent watch |
| HUP to the harness | Same | parent watch |
| KILL of the harness | Shell gone at once, no trap ran; parent watch fires | parent watch |
| tmux pane death | tmux sends HUP to the pane's process group; the shell dies; parent watch fires. With `--group own` the leader is outside that group and is not signalled directly, which is why the parent watch, not the signal, is the mechanism | parent watch |
| Background-execution wrapper detaches then kills the shell | The specimen path. Shell dies; parent watch fires. The job table never mattered | parent watch |
| Ceiling reached | Leader winds down and exits 0 | ceiling |
| KILL of the leader | Workers see the leader identity dead within 500 ms and exit | worker leader watch |
| Contract A: run wrap dies on any path | Leader's parent is the wrap; parent watch fires; the run record additionally lets the lease sweep and run watcher act on the group | parent watch; run custody |

What no custody in this design covers, stated plainly:

- A worker that is STOPPED (SIGSTOP) cannot run its watch. It is not a
  CPU hog while stopped, but it is an orphan. The sweep catches it: its
  argv carries the tag, the bed and the engine path, and its start time
  ages past the bound.
- The fork window: a worker forked by the leader but killed-leader
  before entering its watch loop. Sub-second, and the sweep catches it
  the same way.
- Machinery launched before this verb exists, or outside the contract:
  `bash -c 'while :; do :; done'` carries no tag, no bed and no engine
  path. The sweep REFUSES it (section 3.4) because nothing proves it is
  ours, and the shared-machine rule (`orchestration.md:221`) forbids the
  kill. Those are hand kills by a seat holding the launch-time pids from
  its session records, as the goal Intent describes for m2 and m3. This
  design does not make them lawful for a verb; it makes them stop
  happening.

## 3. Decision 2: the sweep is `janitor orphans`, run by hand

### 3.1 The verb, exactly

One new verb in the `janitor` family (`main.go:339-344`), implemented in
`cmd/metasystem/janitor_verbs.go` over a new file
`internal/janitor/orphans.go`.

```
metasystem janitor orphans --root R --older-than-min N
    [--temp-root DIR ...] [--bed DIR] [--include-preserved] [--apply]
```

- `--root R`: the invoking checkout's metasystem root. Supplies the
  engine path (`bin/metasystem` under it, resolved), the runtime
  signatures for the ancestry proof, and the registry home.
- `--older-than-min N`: the bound, in minutes, on process start time and
  on bed age. Required; 0 is legal and is what fixtures pass.
- `--temp-root DIR` (repeatable): roots under which beds are candidates.
  Default: the realpath of `${TMPDIR:-/tmp}` and `<R>/artifacts/agents/
  suite-failures`. Each is resolved through `census.realpath`
  (`scope.go:20-38`) so macOS `/var` and `/private/var` agree.
- `--bed DIR`: restrict to one bed under a temp root.
- `--include-preserved`: also REMOVE preserved failure beds. Without it
  they are kept (their processes are still killed): a preserved bed is
  evidence by design (`validate-metasystem.sh:1486-1497`).
- `--apply`: kill and remove. Without it the verb prints exactly what it
  would do and touches nothing. Report-only is the default so the first
  run on a strange machine is always a look.

### 3.2 Candidate discovery: what proves a process is ours

Enumeration is the census's native path
(`census.EnumerateProcesses`, `production.go:43-72`), once, then cwd
resolution only for the argv-matched candidates (`ResolveCwds`,
`production.go:78-94`), the same cost discipline the census keeps.

A process is a CANDIDATE only when ALL of these hold:

1. Engine shape. Its argv matches one of the janitor shapes
   (`killproof.go:41-63` plus the two load rows from 2.2), OR its argv
   word 0 resolves to an engine binary path (`.../bin/metasystem`, any
   checkout) and word 1 and 2 are one of `steward run`, `supervise owner`,
   `supervise component`, `proc load-generate`, `proc load-worker`,
   `util hold`, `mission run-loop`, OR its argv word 0 or 1 base name is
   one of the shipped adapter or dispatcher scripts (`fake.sh`,
   `claude.sh`, `codex.sh`, `devin.sh`, `dispatch.sh`,
   `watch-background-jobs.sh`). The shape table is the one list, extended
   for the runner (`steward run --repo`) which today has no shape.
2. Bed under a temp root. A path in argv named by `--repo`, `--root`,
   `--bed` or any bare absolute token (`census.ArgvPaths`,
   `scope.go:69-106`), OR its resolved cwd, lies at or below one of the
   temp roots (`census.PathBelow`, `scope.go:52-62`). That path is the
   process's bed.
3. Age. Its kernel start time (`Exact.StartedAt`) is older than the bound.

Anything failing rule 1 is invisible to the sweep, exactly as the census
ignores non-agent processes. Anything passing rule 1 and failing rule 2
or 3 is printed as `refused` with the failing rule, never acted on. This
is the lookalike case: an engine-shaped argv whose bed is a real checkout
(or has no bed) is somebody's live machinery.

A bed path is a checkout path for the rest of the verb. On the m1
specimen, rule 2 matches the `agent-fixture` beds under the user's temp
directory (`dispatch-fixtures.sh:427-445`) and the `metasystem-health`
beds (`health-fixtures.sh:101`); a runner armed on a real checkout under
the user's home fails rule 2 and is refused.

### 3.3 What refuses a candidate: the live-record and ancestry rules

A candidate becomes a VICTIM only when ALL of these hold, each evaluated
immediately before acting:

1. No live record owns it. The sweep runs one production census with the
   candidate's bed as the repository
   (`census.RunProductionCensusAt`, `production.go:109-112`, state root
   the bed). The candidate must appear in that inventory as UNTRACKED
   (`census/run.go:332`). CUSTODY or ANNOUNCED (`run.go:306-330`) means a
   live job, run, mission or announced session owns it: refused, reason
   `live-record`. A census verdict other than SUCCESS, or the candidate
   absent from the inventory, is refused with reason `census-failed`
   (fail closed, the steward's own rule at `steward/census.go:40-58`).
2. Not a live seat's. `census.FindAncestorProduction`
   (`ancestor_production.go:57`, `ancestor.go:41-56`) from the candidate's
   parent finds no live agent-signature ancestor. One found: refused,
   reason `live-ancestry`, because a seat's own harness may be running
   its bed right now, old or not.
3. Not registered live. The bed is not the checkout path of a registry
   claim (`registry/append.go:18`, home from `METASYSTEM_SUPERVISION_
   REGISTRY_HOME` when set) whose owner is provably alive
   (`identity.Liveness`, three-way). An open claim with a live owner:
   refused, reason `registry-live`. A corrupt registry: nothing is
   killed in that run and the verb exits 3, D-4's rule
   (`supervision-lifecycle.md:620-622`).
4. Re-proven at the kill. `janitor.Killable` (`killproof.go:206-222`) is
   applied to a fresh probe: same pid, same start second, argv readable
   and still matching its shape with the bed path still in it. A pid that
   now names another process, or an argv that became unreadable, is
   refused with reason `identity-changed` or `argv-unreadable`.

What the sweep never does: pattern-kill by name or path (no `pgrep`, no
substring over argv outside the positioned shapes); signal a whole group
by negative pgid unless `janitor.GroupOwnership` (`killproof.go:129`)
returns OWNED for the candidate's tag (load groups and adapter groups
qualify; runner, owner and component groups are signalled by the exact
leader pid, which is also their pgid under `Setsid`); act on a process
whose cwd could not be resolved AND whose argv names no bed
(`UNRESOLVED-CWD`, `run.go:236`); or touch any process outside the temp
roots.

### 3.4 Kill order and bed removal

Within one bed, D-4's order (`supervision-lifecycle.md:593-603`,
`targets.go:125-129`): supervision OWNERS first, then their components,
then the steward runner, then load groups, holds and adapter loops. Each
victim: TERM by exact pid, up to 2 s of 50 ms polls of
`identity.AliveRef`, then KILL, up to 2 s more, then a death proof by the
same reference. A survivor after KILL is reported `survived` and the bed
is kept.

After the processes, the bed. A bed is REMOVED only when: `--apply`; the
post-kill census over the bed lists zero inventory and zero
`UNRESOLVED-CWD` diagnostics (nothing lives there, and nothing is
unaccounted for); the bed root directory's mtime is older than the bound
(a narration log deeper in the tree does not touch the root's mtime, so
a bed that is still being torn down by its own trap is not old by this
measure); and the bed is not under `suite-failures` unless
`--include-preserved`. Removal is `os.RemoveAll` on the bed root, never
on a temp root itself.

A bed whose processes are all gone before the sweep began (the 8,789
stale beds are mostly this) goes straight to the removal rule.

### 3.5 The report

One line per victim, one per refusal, one per bed, then one summary:

```
killed   pid=<p> start=<s> shape=<name> bed=<dir> age=<m>m signal=TERM|KILL
survived pid=<p> start=<s> shape=<name> bed=<dir>
refused  pid=<p> start=<s> shape=<name> bed=<dir>|none reason=<rule>
removed  bed=<dir> age=<m>m
kept     bed=<dir> reason=preserved-failure|live-process|young
summary  candidates=<n> killed=<n> survived=<n> refused=<n> removed=<n> kept=<n> mode=report|apply
```

Exit 0 when nothing was refused for indeterminacy and nothing survived
(including the clean-machine case, which prints only the summary line
with zeros). Exit 3 when any line is `survived` or any `refused` carries
`census-failed`, `identity-changed`, `argv-unreadable` or a corrupt
registry: the machine is not clean and the reason is on the line. Exit 2
on usage. Lookalike refusals (`live-record`, `live-ancestry`,
`registry-live`, rule 2 or 3 of 3.2) do not change the exit code: they
are the verb working.

Under `--apply`, the same lines are also appended to
`<R>/artifacts/agents/supervision/janitor-orphans.log`, so a seat that
ran it has a record beside the census log.

### 3.6 Who runs it, and why not on a cadence

By hand: a seat or a human, in report mode first, then `--apply`. Not
the steward, not the watcher, not a scheduler entry. Reasons:

- The shared-machine rule (`orchestration.md:221`) is written for an
  actor who looks first. A cadence kill is a standing pattern kill by
  another name the day any proof rule regresses; dab1dbdd was exactly
  such a regression, caught because a person read the wind-down output.
- The steward's runner is one of the classes swept. A runner sweeping
  runners under temp roots while a suite is live on the same machine
  must guess the bound; the 2026-08-03 hang precedent
  (`orchestration.md:202`) ran 112 minutes with zero progress inside a
  live bed, so no bound short of hours is safe for a cadence, and a
  bound of hours is what a human already does by hand.
- D-4 already chose by hand and at suite start
  (`supervision-lifecycle.md:576-578`). This design keeps the by-hand
  half. The suite-start half is report-only and is not in this slice:
  the suite has its own custody design (`plans/suite-custody-design.md`)
  and a report line at start belongs to it.
- Authority: no holder gate. The m1 specimen showed a seat with nothing
  lawful to run; the beds are outside every checkout lease, and the verb's
  proof rules, not the caller's class, are what make a kill lawful. The
  caller's class is recorded on the summary line (`lease.ClassifyVerb`
  over the real parent pid, the `proc acknowledge` pattern at
  `census.go:112-113`) so the log says who swept.

The end-of-turn hook already surfaces UNTRACKED (`orchestration.md:195`).
This design adds no new nag; a seat that sees leaked beds runs the verb.

## 4. Fixtures

New file `scripts/agents/custody-fixtures.sh`, in the shape of
`supervision-fixtures.sh:1-93`: a parent that runs child scenarios
through `run_fixture_bed_scenarios` (`fixture-bed-scenarios.sh:31-44`),
each child in its own `mktemp -d` bed with `METASYSTEM_SUPERVISION_
REGISTRY_HOME` under the bed (`supervision-fixtures.sh:109`), waits
through `harness_fixture_cap custody-wait` (`fixture-budget.sh:341`), and
the per-signal trap of `supervision-fixtures.sh:407-416`. No unbounded
waits; every wait names itself. The census assertions use the fixture
process table only where the fixture must be hermetic; the harness
scenarios use the live table because the claim is about real processes.

- C-1 `harness-signals`. For each of INT, TERM, HUP, KILL: start a
  harness (a bash script that launches Contract B with `--seconds 300
  --workers 2 --tag load-c1-<sig>-<hex> --bed $bed`), read the printed
  pgid, wait bounded until `proc group-members --pgid <pgid>` lists 2
  workers plus the leader, send the signal to the harness pid, then wait
  bounded until `proc group-exists --pgid <pgid>` fails. Assert: no live
  process carries the tag (`proc group-members` empty, and a scan of the
  live table by `proc classify` for the recorded leader and worker pids
  and starts returns dead). Then run `proc census` over `$bed` and assert
  `UNTRACKED` count 0. tmux pane death is asserted as HUP; a real tmux
  server is not started (gap G3).
- C-2 `harness-ceiling`. Contract B with `--seconds 2`; the harness
  stays alive; assert the group is gone within the bounded wait and the
  leader exited 0.
- C-3 `leader-killed`. Contract B; KILL the leader by pid; assert every
  recorded worker pid and start is dead within the bounded wait.
- C-4 `run-launch-custody`. Contract A in a fixture checkout; assert the
  census over that checkout lists the leader and both workers as CUSTODY
  with a `RUN <id>` label (`inventory_has` style, `supervision-fixtures.sh:497`);
  `run watch` after KILL of the wrap ends terminal and the group is gone.
- S-1 `sweep-synthetic-bed`. Build `$tmp/agent-fixture/repo` the way
  `dispatch-fixtures.sh:427-445` does. Launch, each detached with
  `setsid` and the record deliberately abandoned: a real `steward run
  --repo <bed>` (the runner ticks against a bed without goals and keeps
  looping, `runner.go:95-108`); an owner through the arming path
  `health-fixtures.sh:330` uses, with its shutdown skipped; a load group
  by Contract B with `--parent-pid` of a `util hold` sleeper the fixture
  keeps alive. Run `janitor orphans --root $src --temp-root $tmp
  --older-than-min 0` without `--apply`: assert the report names every
  launched pid with its shape and the bed and that no `refused` line
  appears. The fixture's own ancestry carries no runtime signature (the
  suite shell is not an agent), so the live-ancestry rule finds nothing,
  which is the orphan case the verb exists for. Assert every process is
  still alive after the report run. Run again with `--apply`: assert one
  `killed` line per process in the order owner, component, runner, load;
  the post-kill census over the bed shows zero inventory; the bed is
  `removed`; exit 0.
- S-2 `sweep-lookalike`. Launch `util hold --tag load-fake-<hex>` with
  cwd outside every temp root and no bed in argv; launch `bash -c "sleep
  600 # --repo $bed"` (a bed path in argv, no engine shape). Run the
  sweep with `--apply`. Assert: the hold is printed `refused reason=
  bed-not-under-temp-root` and is still alive; the sleep appears nowhere;
  exit 0. Then stop both by recorded pid and start.
- S-3 `sweep-live-record`. A real `run launch` under a fixture checkout
  whose root is under `$tmp`; the sweep must print `refused reason=
  live-record` for the wrap and its workload and remove nothing.
- S-4 `sweep-clean`. `--temp-root $tmp/empty` on an empty directory:
  exactly one summary line with all zeros, exit 0, no log file written.
- S-5 `sweep-preserved`. A dead bed under `$tmp/suite-failures/<stamp>`
  with no processes: `kept reason=preserved-failure` without the flag,
  `removed` with `--include-preserved`.

Go unit tests, table-driven, for the pure parts: shape matching for the
new rows and the runner shape; candidate rules 1 to 3 of 3.2 over an
injected process table; victim rules 1 to 4 of 3.3 over an injected
census verdict, ancestor tree and registry; kill order; report lines.
The process-touching fixtures above are the shell layer, as
`targets.go:11-14` already separates law from execution.

## 5. Size

Two slices; one does not fit 240 reserved minutes.

| Slice | Content | Reserved minutes | Basis |
| --- | --- | --- | --- |
| 1 Load custody | `proc load-generate`, `proc load-worker`, two janitor shapes, `custody-fixtures.sh` scenarios C-1 to C-4, Go tests for the leader's state machine over an injected prober | 180 | Unsupported by a recorded per-verb precedent. Bounded above by the goal's own 240 (`proof-harness-process-custody.md:9`) and by suite-custody, a comparable signal-and-teardown slice that landed inside 240 (`plans/goals/suite-custody.md:10`) |
| 2 The sweep | `janitor orphans`, the runner shape, rules 3.2 to 3.5, scenarios S-1 to S-5, the log, one paragraph in `docs/orchestration.md` under Shared Machines naming the verb as the lawful sweep | 240 | The umbrella's S4 estimate for a larger scope (`recovery-analysis.md:431`); unsupported beyond that convention |

Slice 1 lands first: it is what stops the next specimen. Slice 2 has no
dependency on slice 1 except the two shape rows, which slice 2 can add
itself if it lands alone.

## 6. Obligations on the implementer

- Never weaken a proof rule to make a fixture green. A fixture that
  cannot prove death names itself and fails.
- The verbs print plain English on refusal, naming the rule and the
  seat-runnable next command, per the recovery umbrella's S1 rule.
- No `ps`, `pgrep`, `pkill` or `lsof` anywhere in the new code; the
  native enumeration is the only process source.
- The load worker's loop is the only busy code; it must check its clock
  and its leader in every case, with no path that skips the check.
- Both new verbs get a row in `metasystem help` with the same one-line
  style as `main.go:47-58`.

## 7. Self-grade

Confidence: high on the harness verb (every mechanism it uses exists and
is cited: parent identity watch on `identity.AliveRef`, group enumeration
on `GroupMemberPids`, shapes on `DefaultShapes`, the delegate's kill
domain); medium-high on the sweep's rules (each rule maps to an existing
function, but their composition is new and the per-bed census cost on a
machine with hundreds of beds is not measured); medium on the fixture
list (S-1 depends on a real `steward run` and a real owner staying alive
against a bare bed, inferred from `runner.go:95-108`, not run).

Weakest claim: that the bed root directory's mtime is a usable age for
bed removal. A fixture that creates or removes an entry directly in the
bed root late in its life would look young; the rule errs toward keeping,
so the failure mode is a bed that is swept a run later, not a live bed
removed.

Reject condition: if `run wrap` does not keep its workload in the wrap's
own process group (so Contract A's `--group inherit` does not land the
workers in the run's pgid), Contract A falls back to Contract B under a
run record and the census label claim in C-4 is withdrawn. If
`FindAncestorProduction` cannot walk from a reparented process on macOS
(a parent of 1 stops the walk at `ancestor_production.go:31-34`), the
live-ancestry rule refuses nothing for true orphans, which is the
intended outcome, but the S-3 refusal must then come from the
live-record rule alone.

## 8. Gaps (reported, not filled)

- G1: the m2 and m3 harness scripts and the m1 seat's kill attempts are
  in session records, not the tree; this design describes their shape
  from the goal Intent only. Whether any of those loops carried a path or
  tag the sweep could have proven is unknown; the design assumes not.
- G2: no recorded minutes exist for a comparable engine verb with
  process fixtures; both estimates are marked unsupported.
- G3: tmux pane death is asserted as HUP; the design has not verified
  that the seat's tmux configuration sends HUP to the pane's group
  rather than killing the pane's process directly. Both end the harness
  shell, which is all the parent watch needs, but the fixture proves only
  HUP.
- G4: the report-only run at suite start that D-4 named is left to the
  suite-custody stream and is not scheduled here.
