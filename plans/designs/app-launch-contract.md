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
   checkout on another port is already one command.
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
   process dead" (`cmd/metasystem/main.go:432`). A critique runs in its
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
  optional `stop` command; a `ready` probe (an http URL or a tcp
  address, both with the address substituted); the `address` the app
  listens on for the standing run and a port range for candidates; a
  `log` path; `readyMs` and `stopMs`; and an optional `build` command a
  candidate runs first. The engine hands the command three facts as
  environment: the address it must listen on, the state root it may
  write under, and the log path. A project with no contract has no
  `app` verbs, and the refusal says which file to write. Not a section
  of `metasystem.conf`, because the testing contract is a file and a
  merge driver already exists for that shape.
- D2. **Four verbs, one object.** `metasystem app start|stop|restart|
  status`, audience both, forms mirroring `ui`. Start runs the start
  command detached under the bounded supervisor, writes the run record
  (name, pid tree, started at, commit of the tree it runs, address, log,
  contract digest, goal if any) under `artifacts/agents/app/`, waits for
  the ready probe up to `readyMs`, and prints the address; a record
  whose process is alive makes a second start rejoin it. Stop runs the
  stop command where the contract has one, otherwise TERM then KILL at
  `stopMs`, and in either case proves every recorded process dead before
  it clears the record. Restart is stop then start with what is on disk.
  Status reads the record and probes, and says one of three things:
  running this commit at this address since then and answering; not
  running; or the record says running and the process is gone, which is
  named as stale rather than hidden.
- D3. **`--goal G` runs the candidate.** The engine takes a worktree of
  `goal/<id>` at its tip under `artifacts/`, runs the contract's build
  command there if it has one, allocates an address from the candidate
  range and a state root of its own, and starts the app from that tree.
  The record names the goal and the tip; status names both; a moved tip
  makes the next start replace the run. One candidate per goal at a
  time. The standing app's record and address are never touched by a
  candidate's start or stop.
- D4. **Who may press it.** A person at the enrolled terminal; a
  signed-in browser session through the act layer, as the eleventh
  browser act, with the four verbs joining the Partner's proposal
  grammar and the verb-table test; a delegate under launch permissions
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
  committed. When a run named a goal, stop copies the log tail to the
  evidence root under the goal, so a review's "how it behaved" survives
  the process. Nothing else is retained.
- D7. **Not here.** Traffic splitting and the releaser's bounds;
  production observation; remote hosts; more than one application per
  contract; Windows; the browser act and the Partner grammar (D4's
  second half); the Application section's running line (the
  interface's slice, reading the record this one writes).

## 4. Step 1, the smallest thing that works

D1, D2, D3, D5 and D6, as one engine slice: the contract and its check,
the four verbs on the standing app, the candidate run, and this
repository's own contract proving it on itself. D4's terminal path comes
free with the verbs; its browser half waits for the interface's slice.

## 5. Verification and box

Contract: validation refuses a missing start, a probe without an
address, a candidate range that overlaps the standing address, and
names each fault; `settings check` reports it beside the testing
contract. Verbs, against a fixture application (a small Go server in
testdata that answers its health URL after a delay and can be told to
ignore TERM): start waits for ready and records; a second start rejoins;
status is truthful in all three states, including a record whose
process was killed behind the engine's back; stop proves death for a
child that ignores TERM; restart replaces the process and the record.
Candidate: `--goal G` builds in a worktree at the tip, runs on an
address from the range with its own state root, leaves the standing
record untouched, and a moved tip replaces the run. Self-hosting: this
repository's contract starts a second interface on another port, its
health answers, and `ui status` still names the standing one. The
verify skill drives it once for real. Box: one build lane (Claude on
Opus), one code read (Codex on Sol) with one fix round under R-124,
after Astra's read of this page; two attempts, 240 to 360 job-minutes.

## 6. Self-grade

High on D2 and D3: the verbs copy a lifecycle the interface already has,
and the worktree and the proven stop copy what launch already does.
Medium on D1: a second contract file is a second thing for an adopter to
write, and the schema is deliberately small so that it is one screen.
Medium on D5: the candidate build needs the committed bundle to be
fresh, which the bundle test already enforces on the tree. Weakest: a
port range is a convention, not a guarantee, on a developer host; the
start refuses and names the address when it is taken, which is honest
and enough for now.
