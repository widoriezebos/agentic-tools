# The application under the engine's hand: a launch contract and the app verb

- Kind: design
- Id: 01M3KA0C9CQZV1GAK720188VZ9
- Status: draft
- Goals: app-launch-contract

Wido, 2026-09-28: "we should probably have a verb for this that uses a
project specific launch script to control the app (stop, start, restart,
status etc)", after ruling that human review of a goal happens before it
reaches main and that the candidate must run locally so its behaviour can
be inspected. Author Fable. Every cite re-read at `f9385b1ca`.

## 1. What exists and binds

1. **The interface already has this verb set for itself.** `metasystem ui
   start|stop|restart|status` are public verbs for both audiences
   (`cmd/metasystem/intent_process.go:179-199`): start takes a loopback
   address, stop a wait, restart runs "with the executable on disk", and
   status says "whether the browser interface runs, and where". The
   lifecycle behind them was designed in g1-s1 (a readiness pipe the
   child writes to, state on disk, one loopback listener) and `ui serve`
   takes `--repo`, `--metasystem-root` and `--listen`
   (`cmd/metasystem/ui.go:44-52`). A second interface from another
   checkout on another port is already one command. The lifecycle
   behind those verbs is `internal/ui/lifecycle`, and it is the model
   this design copies: `launch.go` starts the child detached in its own
   session with a log and a readiness pipe; the serving process writes
   its own record, with its native identity ref, after it listens
   (`serve.go:159-176`) and removes it on exit; `state.go` reads six
   states, running, stopped, stale, uninspectable, unreadable and busy,
   by re-proving the ref (`Read`, `readInactive`, the lock authorizing
   removal of a stale record); and `stop.go` re-proves the identity
   immediately before every signal through `identity.SignalExact`
   (`internal/identity/ref.go:131-150`) and never signals a bare number.
2. **A project's commands have two homes, of two kinds.**
   `docs/project-rules.md` carries prose lines a person fills in, among
   them "Local run: `<command>`" (line 27), which nothing runs; the
   verify skill sends a worker there to choose an entrypoint
   (`skills/verify/SKILL.md`, "Choose the Surface"). `testing.json` is
   the machine-run kind: a per-project contract of groups with
   commands, tools, inputs and timeouts, selected by
   `testing.contract=testing.json` in `metasystem.conf`, validated by
   `settings check`, and executed under the bounded supervisor with
   evidence copied out (`internal/boundedexec`, `internal/proofrun`;
   `suite.*` keys at `metasystem.conf:113-116`). The launch contract
   belongs to the second kind.
3. **Process control the engine already owns.** `launch start` returns
   after its child is recorded, `launch status` reconciles one launch,
   and `launch cancel` "ends a launch group and proves every recorded
   process dead" (`cmd/metasystem/main.go:432`). Cancel is not the model
   for stop: it signals the recorded process group by its number before
   it proves anything and checks identities only afterwards
   (`internal/launch/launch.go:532-578`, `process.go:103-121`), so a
   reused group id would be signalled (Astra ALC-01). Nor is
   `boundedexec.Run` an owner for a serving app: it waits for the command
   to finish and kills its group at the deadline
   (`internal/boundedexec/boundedexec.go:84-121`). A critique runs in its
   own worktree (`internal/launch/codex.go:91`); a goal's built work
   lives on `goal/<id>` at origin (`cmd/metasystem/goal_branch.go:293,
   495`), and `work land` proves that branch and pushes it to main
   (`cmd/metasystem/intent_delivery.go:139-152`).
4. **The paper.** Chapter 15 has the machinery "request an isolated run
   through the governed tools"; chapter 14 has it "prepare the
   environment and carry out the experiment the engineer chose".
   Chapter 9 says why a script's exit code is not liveness: "a failed
   worker cannot report its own failure", identity is checked at the
   moment of use, and every wait has a deadline and an owner. The
   Application section of the interface is to show "the running engine
   build beside the source at HEAD, and say when they differ"
   (user-interface-design.md, Workspace identity).
5. **What the browser can do with an act.** Ten goal acts run through
   the act layer under the human's sign-in, and the Partner proposes
   them in the terminal's own grammar, held against the verb table by a
   test (g1-s58, g1-s62). A new verb the browser should offer joins that
   grammar, not a second one. g1-s65, the review sitting, needs one
   press: run this candidate and tell me where it is.

## 2. What a person wants from it

- **H1. One word for the app.** Start, stop, restart and status of this
  project's application, from the terminal and from the browser, without
  remembering the project's script.
- **H2. A candidate beside the standing app.** Run a goal's built work
  from its branch, on its own port, while the standing app keeps running
  and the review room links to the candidate.
- **H3. A truthful status.** Which commit runs, where, since when, and
  whether it is alive: read from a record and a probe, never from what
  the last command printed.
- **H4. A stop that is a stop.** A review must not be looking at a stale
  process; stop proves death, the way launch cancel does.
- **H5. The Partner can offer it.** "Run the candidate for G" is a
  proposal card like any other act, performed under the human's sign-in.
- **H6. The project's script stays the project's.** The contract names
  the commands; it does not replace them with a new script language.

## 3. Decisions

- D1. **The launch contract.** One file beside `testing.json`,
  `launch.json`, selected by `launch.contract=launch.json` in
  `metasystem.conf` and validated by `settings check`. Schema 1 holds
  one application: its name; a `start` command (argv and cwd); an
  optional `stop` command; a `ready` probe; the `address` the app
  listens on for the standing run and a port range for other runs; an
  optional `log` path; `readyMs` and `stopMs`; an optional `build`
  command a run at another commit runs first; an optional `prepare`
  command that gives a run its own data (a database, a queue, a
  directory), given the run's state root, address and log in its
  environment, run before a run's first start and on reset; an
  optional `check`, the id of a group in the testing contract to run
  against a run's address; and `tools`, declared as the testing
  contract declares them (executable and version arguments), so
  `settings check` names a missing JDK, cargo or Go before a build
  fails. **Everything but `start` is optional, and leaving a thing out
  leaves a working contract** (Wido, 2026-09-28: "the smallest thing
  that works" applies here too, "if it is decided we don't need certain
  aspects for the app we are building then we can just leave them out
  and all still works"): no `stop` means TERM then KILL; no `ready`
  means alive is ready; no `log` means the engine's capture; no `build`
  means the tree is run as it is; no `prepare` means a run at another
  commit shares the standing run's data, and the record, the status and
  the room say so in those words; no `check` means `app check` answers
  that none is declared; no `tools` means no preflight. The one
  refusal in the contract is a contradiction: `data: own` declared with
  no `prepare` to make it. The contract is language-neutral by construction: the engine
  interprets no language, it starts a process, probes, tracks a tree,
  signals and copies a log, so `mvn spring-boot:run`, `cargo run`, `go
  run`, `npm start` and `docker compose up` are all one thing to it.
  Four rules make that hold for any application:
  - **Placeholders in argv and probes.** `${address}`, `${host}` and
    `${port}` are substituted in the command's argv and in the probe,
    so an app that takes its port as an argument gets it there; the
    same three facts, with the state root and the log path, reach the
    command as environment.
  - **Four forms of readiness.** An http URL that must answer 2xx, a
    tcp address that must accept, a log line matching a pattern, or
    none, where ready means the process tree is alive. A worker, a
    batch job or a desktop application has nothing to probe, and the
    contract says so rather than pretending. The http and tcp forms are
    probes: they can go dark and are asked again by status. The log and
    none forms are startup observations scoped to this run: the pattern
    is sought in the log from the offset at which this run's supervisor
    opened it, and once seen it is ready for the run's life; for these
    two forms stopping is proven by the death of every recorded process
    alone, never by a log that cannot unmatch (Astra ALC-04).
  - **The start command runs the app in the foreground**, and the
    engine detaches it, so the tree it records is the app. A start
    script that backgrounds a process and exits leaves nothing to stop,
    and validation says so where it can (the command exits before the
    probe answers). Where the app must be stopped its own way (a
    compose stack, a service manager), the `stop` command does it, and
    the proof of stopping is the recorded processes dead and, for the
    http and tcp forms, the probe dark.
  - **Stdout and stderr are captured to the log** unless the contract
    names the application's own log file, in which case that file is
    the log.
  A project with no contract has no `app` verbs, and the refusal says
  which file to write. Not a section of `metasystem.conf`, because the
  testing contract is a file and `settings check` already validates a
  project-owned command contract beside the settings. The testing
  contract's merge driver is specific to its schema
  (`internal/testpolicy/contractmerge`), so `launch.json` merges as a
  plain file in step 1 (Astra ALC-05). Declared tools are an
  availability check, the executable found and its version line
  printed into the record; a version constraint is later (ALC-06).
- D2. **Seven verbs, one object, and a supervisor that owns the run.**
  `metasystem app start|stop|restart|status|log|reset|check`, audience
  both, forms mirroring `ui`; every one takes `--at REF` to name a run
  at a commit (D3) and means the standing run without it. `log` prints
  the run's log tail and follows it with `--follow`, from the file the
  engine captured or the file the contract named. `reset` is stop, then
  `prepare` where the contract has one, then start, and says "no
  prepare declared: reset is a restart" where it has none. `check` runs
  the contract's named testing group through the testing contract's own
  runner and nothing else, over the bridge that runner lacks today
  (Astra ALC-09): the runner's diagnostic form for one named group
  (`--mode canary --groups <id>`, which is the only form that runs a
  group by name), executed fresh (`--no-reuse`), with one new declared
  input, the run's address, which the runner overlays into the group's
  environment as `METASYSTEM_APP_ADDRESS` in both environment modes, the
  way it overlays its worker count and execution root today, and which
  is part of the group's execution identity so a result for one address
  is never reused for another. The check runs against the recorded
  run: the run must be live and answering, or the check is refused in
  words. Its outcome and time are written on the run record; `check`
  answers that no check is declared where the contract names none.
  `log` on an ended run reads the retained log. The owner is an internal `app serve`, the shape of
  `ui serve`: `app start` launches it detached the way `ui start`
  launches the interface (its own session, the log, a readiness pipe),
  and returns when the pipe reports ready or failed, or at `readyMs`.
  The supervisor writes the run record first, before it spawns
  anything: its own native identity ref, the group it leads, the
  contract digest, the commit of the tree, the address, the log and the
  goal if any, under `artifacts/agents/app/`. Then it spawns the
  contract's start command as its child, in the supervisor's own group,
  and writes the child's native ref into the record as the very next
  act, before any readiness wait. Only then does it wait for readiness
  and report ready, and it waits on the child for the run's life. When
  the child ends, the supervisor does not leave while any process of
  its own group remains: it reads as "child ended, descendants alive"
  and stays the owner, so ownership is never removed while something
  it started still runs (Astra ALC-08). When the group is empty the
  supervisor writes the record as **ended**, with the exit status, the
  time, the log path and the last check, and exits; it never deletes
  the record. A record is removed only by `stop`, `reset` or the next
  `start` for that ref, and, where the run named a goal, only after the
  evidence copy of D6 has completed (ALC-10). That ordering is the ownership
  handoff (Astra ALC-02, held at round 2): an engine that dies before
  the supervisor started has left no run; a supervisor that dies leaves
  a record whose supervisor ref is dead, and status reads it as
  "orphaned: the application may still run" and stop ends the child by
  its recorded ref, re-proven; a readiness timeout ends the child by its
  ref, never only the supervisor. The one window left is the instant
  between the spawn and the child's ref write; a supervisor killed
  inside it leaves a record naming its group and no child, and status
  says so, "supervisor gone, group G, child not recorded", listing what
  the inspection finds in that group by identity, never signalling by
  number. That window is a fixture obligation of the build, not a
  mechanism of this design. Status reads liveness
  from the record's refs, the six states the interface's lifecycle
  reads, and readiness from the probe, and says them separately: running
  and answering; running and not answering since a time; starting;
  child ended, descendants alive; ended, with its exit status and time,
  which is a record and not a live process; stopped, no record at all;
  stale, a record whose supervisor is gone without having written
  ended, said as such and kept, since only `stop`, `reset` or the next
  `start` removes a record, after the evidence copy, which is where
  this design departs from the copied lifecycle's reader that removes a
  stale record under its lock; uninspectable (ALC-03, ALC-10). A second start with a live supervisor rejoins it and still
  waits for readiness before it says started. Stop runs the contract's
  stop command where there is one; otherwise it ends the owned tree,
  not one process: TERM to the child, re-proven by identity immediately
  before the signal, then, while any process of the supervisor's own
  group remains, TERM then KILL to that group, sent by the supervisor
  itself after re-proving its own identity as the group's leader, since
  a group whose living leader is provably ours cannot be a reused id;
  then KILL to the child at `stopMs`; then the supervisor. No signal is
  ever sent to a bare number by the engine, and a recorded process whose
  identity cannot be proven is refused by name rather than signalled
  (ALC-01, ALC-08). "The group is empty" means no member but the
  supervisor itself, which leads it and is in it; a KILL to the group
  ends the supervisor too, so after that the bookkeeping, the ended
  record and the evidence copy, is the stop caller's, reading the
  record the supervisor wrote before it spawned. Where the supervisor
  is already gone, stop ends the child by its recorded ref and says
  what an inspection of the recorded group finds by identity, and
  signals none of it. Stopping is proven when every recorded ref is
  dead, the supervisor's among them, and the group has no member, and,
  for the http and tcp forms, the probe is dark; the record is kept
  until then. Restart is stop then start with what is on disk.
- D3. **`--at REF` runs the application at any commit; `--goal G` is
  sugar for the goal branch's tip.** The engine takes a worktree at the
  commit REF names under `artifacts/`, runs the contract's build
  command there if it has one, allocates an address from the range and
  a state root of its own, runs `prepare` there before the first start
  where the contract has one, and starts the app from that tree. So
  main and a candidate run side by side, `app start --at main` and `app
  start --goal G`, and a review compares the same press on two ports.
  The record names the ref and the commit; status names both and, where
  no `prepare` exists, says "data: shared with the standing run"; a
  moved ref makes the next start replace the run. One run per ref at a
  time. The standing app's record, address and data are never touched
  by another run's start, stop or reset. `app reset --at REF` re-runs
  `prepare`, which is what a learning sitting needs to repeat an
  experiment from a known state (docs/paper/14-how-engineers-learn.md).
- D4. **Who may press it.** A person at the enrolled terminal; a
  signed-in browser session through the act layer, as the next browser
  acts, with the verbs joining the Partner's proposal grammar and the
  verb-table test; a delegate under launch permissions
  may `app start --goal G` for the goal it holds, in its own worktree.
  The browser act and the grammar are the interface's slice, designed
  and built after this one lands.
- D5. **This repository is the first contract.** Its application is the
  interface: start is `bin/metasystem ui serve --repo . --listen
  <address>`, ready is `/-/health`, build is the Go build of the
  candidate tree with its committed bundle. `ui start|stop|restart|
  status` stay what they are, the seat's own window; `app --goal G`
  is how a candidate interface runs beside it.
- D6. **What the record is for.** The run record is per seat and never
  committed; it carries the last `check`'s verdict and time and the
  data word, and it outlives the process as an ended record (D2). When
  a run named a goal, the evidence copy, the log tail and the last
  check to the evidence root under the goal, runs at `stop` and at
  `reset`, and at the next `start` of that ref where the run had ended
  by itself, always before the record is removed and before the run's
  worktree is reclaimed, as the proof runner preserves evidence before
  it disposes of a candidate (Astra ALC-10). A worktree is reclaimed
  only by the next `start` of its ref or by `stop --clean`, and never
  while a record, live or ended, still names it. Nothing else is
  retained.
- D7. **Not here.** Traffic splitting and the releaser's bounds;
  production observation; remote hosts; more than one application per
  contract; Windows; the browser act and the Partner grammar (D4's
  second half); the Application section's running line (the
  interface's slice, reading the record this one writes).

## 4. Step 1, the smallest thing that works

D1, D2, D3, D5 and D6, as one engine slice: the contract and its check,
the seven verbs on the standing app, the run at any commit with its
own data where `prepare` is declared, and this repository's own
contract proving it on itself. D4's terminal path comes free with the
verbs; its browser half waits for the interface's slice. The four
capabilities Wido added on 2026-09-28 (own data and reset, any commit,
the log, the check) are in step 1 because each is one field or one verb
over the same owner, and because the first adopted application with a
database would trip over the data one on its first review.

## 5. Verification and box

Contract: validation refuses a missing start, an http or tcp probe
without an address, a candidate range that overlaps the standing
address, an unknown placeholder, and a declared tool that is absent,
and names each fault; `settings check` reports
it beside the testing contract. Each readiness form is proven with the
fixture: http, tcp, a log line, and none; a start command that exits
before readiness is reported as such and not as running. Verbs, against a fixture application (a small Go server in
testdata that answers its health URL after a delay and can be told to
ignore TERM): start waits for ready and records; a second start rejoins
and still waits for readiness; status says liveness and readiness
separately in every state, including running and not answering, a
supervisor alive before readiness, and a record whose process was
killed behind the engine's back; a recorded pid reused by an unrelated
process is refused by name and never signalled; an engine killed
between the launch and the record leaves no run and the next status
says stopped; stop proves death for a child that ignores TERM; a log
readiness form stops on death alone; restart replaces the process and
the record.
Runs at a commit: `--at main` and `--goal G` build in two worktrees,
run on two addresses from the range with their own state roots, leave
the standing record untouched, and a moved tip replaces the goal's run;
`prepare` runs once before the first start and again on `reset`, with
the run's state root and address in its environment; without
`prepare` the record and status carry "data: shared with the standing
run"; `data: own` without `prepare` is refused by validation. `log`
prints the captured tail and follows it; `check` runs the named group
with the address in its environment and records the verdict, and
answers "no check declared" where the contract names none; a group run
through `check` receives the address in both environment modes, runs
fresh, and its result is not reused for a run at another address; a
check against a run that is not answering is refused in words. Stop
against a foreground wrapper whose descendant survives TERM to its
parent ends the descendant through the supervisor's own group and keeps
the record until the group is empty; a child that ends leaving
descendants reads as such and the supervisor stays; an application
that exits by itself leaves an ended record with its exit status, `log`
still reads its log, and the next `start` copies its evidence before it
removes the record and the worktree. Every optional field left out
leaves a contract that validates and verbs that work, proven with a
contract of `start` alone. Self-hosting: this
repository's contract starts a second interface on another port, its
health answers, and `ui status` still names the standing one. The
verify skill drives it once for real. Box: one build lane (Claude on
Opus), one code read (Codex on Sol) with one fix round under R-124,
after Astra's read of this page; two attempts, 240 to 360 job-minutes.

## 6. Self-grade

High on D2 and D3: the verbs and the supervisor copy the lifecycle the
interface already has, identity re-proven before every signal, and the
worktree copies what a critique already gets. Medium on D1: a second
contract file is a second thing for an adopter to write, and the schema
is deliberately small so that it is one screen. Medium on D5: a Go
build and a green health answer do not prove the bundle fresh; the
candidate's freshness is established by running the existing bundle
test against the candidate tree before its build, and step 1 does that
(Astra ALC-07). Weakest: a port range is a convention, not a guarantee,
on a developer host; the start refuses and names the address when it is
taken, which is honest and enough for now.

## Dispositions (Astra round 1, 2026-09-28, under R-121 and R-124)

Read at `647234149`, verbatim in `app-launch-contract-astra-critique.md`.
Four material findings, all folded; three deferred. Every cited line was
re-read at whole-function depth before folding, and the fold names the
owner the design had missed, `internal/ui/lifecycle`.

| id | finding | fold |
|---|---|---|
| ALC-01 | stop modelled on `launch cancel` signals a recorded group number before proving identity; a reused id would be signalled | D2: identity refs in the record, `identity.SignalExact` before every signal including escalation, an unprovable identity refused by name; §1 names cancel as the wrong model |
| ALC-02 | no durable owner: `boundedexec.Run` waits and kills at its deadline; an engine dying before the record leaves an uncontrollable app and a false "not running" | D2: an internal `app serve` supervisor in the shape of `ui serve`, launched detached with a readiness pipe, writes the record before ready and owns the child for the run's life; a run without a record is no run |
| ALC-03 | three status answers cannot say "alive but not answering" or "starting" | D2: liveness (the lifecycle's six states) and readiness reported separately; rejoin waits for readiness |
| ALC-04 | a log-pattern readiness cannot go dark, so stop could never be proven for that form | D1: log and none forms are startup observations scoped to the run's log offset; stop for them is proven by death alone; probe darkness only for http and tcp |
| ALC-05 | the testing merge driver decodes the testing schema and cannot merge a launch contract | deferred: D1 says plain git merge in step 1 |
| ALC-06 | tool declarations check availability, not version policy | deferred: D1 says availability and a printed version line; constraints later |
| ALC-07 | a build and a health answer do not run the bundle test | deferred as non-material: §6 says freshness is the bundle test run against the candidate tree before its build |

**Round 2, the declared failsafe (2026-09-28, same chain, at
`9eb7cf433`):** ALC-01, ALC-03 and ALC-04 confirmed answered; ALC-02
held, because the supervisor still spawned the application before
publishing its identity, so a supervisor killed in that interval (the
copied launcher's readiness timeout kills only its immediate child)
left an untracked application and a false "stopped". Folded in D2: the
record is written before the spawn with the supervisor's ref and its
group; the child's ref is written as the very next act after the spawn;
a supervisor gone with a child recorded reads as orphaned and stop ends
the child by its re-proven ref; a readiness timeout ends the child by
its ref, never only the supervisor. The one instant left, between the
spawn and the child's write, is the round's fixture obligation, as
Astra named it: interrupt the supervisor there and assert the
application is dead or discoverable and stoppable by identity through
the recorded group, never signalled by number. Two wording residues
fixed (an unconditional "probe going dark", a "wrong version" in
validation). The loop is closed at round 2 on one fixture obligation.

**Fold after the close, on Wido's word (2026-09-28):** asked whether the
verb misses "a powerful capability along the lines of this intent", the
intent being to control the runtime state of the application under
construction, four things were named and Wido said "Yes, indeed. Add
all to the design": a run's own data (`prepare`, `data`, `reset`), a
run at any commit (`--at REF`, with `--goal G` as sugar), the log verb,
and the check against a testing group. Folded into D1, D2, D3, D6, §4
and §5. Wido added two rules for this fold: every aspect an application
does not need can be left out and all still works (D1's optional
fields), and this design is reviewed by Astra "until you both agree
that nothing material that would change the implementation remains",
which for this design overrides the two-round cap: rounds continue on
one chain while a material finding stands, each under R-124, and the
loop closes at the first round the critic and the author both read as
holding nothing that would change what step 1 builds.

**Round 3 (2026-09-28, at `265aa0118`):** three material findings on the
seams the additions opened, every cited line re-read, all three folded.
ALC-08: the default stop signalled one process, so a foreground wrapper
whose descendant survives TERM left the application running and a
false stopped; now stop ends the owned tree through the supervisor's
own group, re-proven as ours by its leader's identity, and the record
stays until the group is empty. ALC-09: `check` through the testing
runner had no bridge: the runner's environment allowlist drops an
address variable, explicit groups build theirs from the contract alone,
a named group needs the diagnostic canary form, and results are reused;
now the bridge is named, one declared runner input overlaid in both
modes and part of the group's identity, canary form, fresh execution,
against a live run only. ALC-10: the copied lifecycle deletes its
record on exit, so an application that ended by itself lost its last
check before the evidence copy; now a run ends into an ended record
that only `stop`, `reset` or the next `start` removes, after the
evidence copy and before the worktree is reclaimed. Round 4 asks
whether anything material remains.

**Round 4 (2026-09-28, at `f70683e2b`): closed.** "VERDICT: 0 material
findings"; "I agree that nothing material requiring an additional
change to what Step 1 builds remains." The author agrees. Two wording
notes folded without mechanism: a stale record is kept and said, not
removed under a lock as the copied reader does; "the group is empty"
excludes the supervisor itself, and after a group KILL the ended record
and the evidence copy are the stop caller's. One fixture obligation
stands for the build, from round 2 and retained by every round since:
interrupt the supervisor between the spawn and the child's write and
prove the application dead, or discoverable and stoppable by proven
ownership. Built next, on Wido's "ok, now build it".

## Built (2026-09-28)

Landed on ui-development and main. Built by Claude on Opus 5.5 in the
worktree `agentic-tools-alc` on branch `app-launch-contract` from
`149c3b7a2`: ten build commits (the first four on Opus 5 through the
subagent tool's `opus` alias before the roster's model was pinned by
id, the rest on `claude-opus-5-5` headless), two fix rounds of four and
two commits, merged with main at `05254901b` (six mechanical conflicts:
the `helm` and `app` objects side by side in the verb tables, the
ratchet ceilings summed to 294 and 128, one refusal site re-anchored,
both floors kept). Read by Codex on Sol under R-124 (R-133-ui roster):
five material findings, three fixed in round 1 (a missing evidence root
refuses closure and keeps the record; a goal's candidate is origin's
tip; a taken standing address is refused before launch and readiness is
accepted only while the child lives) and two held and fixed in round 2
(no ended record over a group that still has a member or cannot be
read, and status checks the group before trusting an ended marker;
`--at goal/G` records its goal as `--goal G` does); Sol's last re-read:
"0 material findings". Every fix was held by a test that failed
first. The reads are verbatim in `app-launch-contract-sol-read.md`;
the builder's report and the briefs are under
`~/LocalStorage/agentic-tools-evidence/review-room-20260928/`.

What exists: `launch.json` schema 1 with every field but `start`
optional and its defaults, validated by `settings check` beside the
testing contract; `metasystem app start|stop|restart|status|log|reset|
check`, each with `--at REF` or `--goal G`; the internal `app serve`
supervisor writing its record before the spawn and the child's
identity as the next act, staying while its group has members, ending
into an ended record; stop through the supervisor's own group with
identity re-proven before every signal; liveness and readiness reported
apart; runs at any commit in their own worktree, address, state root
and data directory, `prepare` before the first start and on `reset`;
`log` with follow; `check` through the testing runner's canary form
with `--app-address` overlaid in both environment modes and part of the
group's identity; declared tools as a preflight written on the record;
the evidence copy for a goal run at stop, reset and the next start,
before the record is removed and the worktree reclaimed; this
repository's own `launch.json`, proven for real: a candidate interface
from the branch's tree served on 7980 beside the standing one on 7878,
health answering with the tree's digest, a proven stop, the tree
reclaimed. The package `internal/applaunch` has a floor of 82.0 in
both ratchet files and measures 83.8 to 84.2.

Departures the reads accepted: `LaunchSupervisor` takes a settle bound
so `app start` does not wait two minutes for a failed supervisor that
rightly stays for a live descendant (start passes stopMs plus five
seconds; Reap keeps its default). The builder found and fixed four
defects of its own earlier commits with failing tests first: the
readiness descriptor leaked to the application, a moved tip rejoined a
live run, the standing run's `prepare` was handed the installation as
its state root, and evidence copies within one second collided.

Later, when it hurts: SOL-06, the launcher kills its direct child by
handle rather than by re-proven reference (harmless while the child is
unreaped); the refusal hint "before it stopped" on a failed start whose
supervisor still owns a descendant; a run at another commit uses the
invoking checkout's `launch.json`, not the candidate tree's, which the
design left unsaid; no end-to-end `check` ran against a real testing
group, since no contract here declares one, which the Java test bed
will supply; in the window between the spawn and the child's write,
stop lists the orphaned application by identity and signals none of
it, as designed, and a person ends it; `settings check` on a bare
worktree refuses before the launch line for reasons outside this slice
(`evidence.root` and runtime registrations); the browser half of D4
and D7 are the interface's slice. The goal `app-launch-contract` is
concluded by Wido's own act.
