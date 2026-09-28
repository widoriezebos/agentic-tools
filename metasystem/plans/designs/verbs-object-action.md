# Object-action verbs, and Go instead of orchestration Bash

- Kind: design
- Id: 01M3FS4JWK13Z7W87G1SAEJZ06
- Status: accepted
- Goals: verbs-match-intent

Revision 7 (Wido's rulings of 2026-09-27: no metasystem scripts, scripts only at extension points, VM outside, every public action an intent). Revision 6 closed design critique at round 6 with zero material findings.
Supersedes `verb-cleanup.md`'s rule "Do not migrate thousands of private
process-protocol calls" and its retention of the family dispatcher;
`agent-help.md`'s output-preservation rules wherever the grammar below changes
them; `records/kill-shell/kill-shell.md`'s ruling of 2026-08-12 ("core in Go,
plumbing in scripts", Phases B-F held) is resumed and extended.

Author: m1e root (Opus 5.5), under Wido's instruction of 26 September 2026, about
22:30 CEST. Design critic: Codex on `gpt-6-astra`, read-only, looping until no
material finding would change the implementation. Builders: Opus 5.5 in git
worktrees, at most three in parallel. Code reads: Fable 5.1 (a model other than
the builder's).

## 1. Why this exists

The goal's intent was fewer verbs, shaped by what humans and agents want to do,
not by the internal software structure. The September result made help friendlier
and left the structure in place. Measured on main at `1b8e047d6`:

| Surface | Count |
|---|---|
| Public commands (`help all`) | 44, action first (`status job J`, `start ui`, `review design F`) |
| Object words mixed into public forms | about 25 (job, run, mission, work, attempt, proof, resume id, record, checkout, machine, ui, ...) |
| Internal families under `metasystem internal` | 45 |
| Internal verbs | 465, plus 8 top-level internal forms (`up`, `stop`, `status`, `arm`, `health`, `watch`, `wait`, `delegate`) |
| Internal verbs reachable without the `internal` prefix | all (`main.go:813`, `dispatchInternal` fallthrough) |
| Production Bash scripts | 72 files, 22,753 lines; 44 under `metasystem/scripts` call the binary |
| Bash test fixtures | 42 files, 33,653 lines |
| Go tests that exec a script | about 60 files |

Why the verbs exist, from a caller census verified by sampling:

- About 20 are genuine process entrypoints (section 3.2).
- About 210 exist only because a Bash script calls them. The big scripts run the
  workflows and call the Go binary for single steps: `supervision-hook.sh` makes 83
  `json get` calls; `dispatch.sh` about 150 verb calls; `land.sh`, `commit.sh`,
  `go-gate.sh`, `validate-metasystem.sh`, the adapters and hosts the rest. They are
  function calls stretched across a process boundary so Bash can use Go as its
  standard library.
- Public commands themselves subprocess about 25 internal verbs
  (`intent_operations.go:23 engineVerb`, `intent_work.go:98 subprocess`, the
  landing batch owner) instead of calling the owner function.
- Agents and remedy texts are told to run about 30 internal verbs (skills, role
  packets, `AGENTS.md`, refusal and remedy strings, the UI).
- About 90 have no production caller at all. The grep census said 117; a sample
  showed 23% of those are reached through arrays, variables, pass-through
  wrappers (`receipt.sh`: `exec "$ms" receipt "$@"`) or text, which is why
  section 6 does not trust grep.

Renaming cannot remove structure-shaped verbs. Moving orchestration into Go turns
each call into a function call and the verb disappears. The grammar change and the
script port are therefore one project.

## 2. Wido's rulings (binding)

1. Grammar is object then action: `metasystem goal approve G`.
2. `metasystem status` stays as the one top-level exception. No other aliases.
3. Full scope: grammar; deletion of every internal verb that is not a genuine
   external process entrypoint; porting all orchestration Bash to Go.
4. Bash survives only as plumbing and never calls an internal verb.
5. The Bash fixtures are ported to Go tests.
6. Hard cutover. Old spellings are deleted and every caller (scripts, hooks,
   settings, skills, docs, role packets, `AGENTS.md`, `wow.md`, UI strings, remedy
   texts) is updated in the same slice. No compatibility shim, no transition window.
7. Each reviewed, green slice lands on main and is pushed to origin.
8. At most three Opus builders in parallel worktrees.

## 3. End state

### 3.1 Public grammar

```
metasystem                        the objects, grouped, one line each
metasystem OBJECT                 that object's actions, one line each
metasystem OBJECT ACTION --help   forms, options, examples
metasystem OBJECT ACTION [TARGET...] [OPTIONS]
metasystem status                 the one overview (see below)
metasystem help [OBJECT [ACTION]] same text as the three forms above; help agent stays
```

Rules of the grammar:

- G1. The first word is an object noun, singular (`goal`, not `goals`). The second
  word is an action verb. Nothing else is public in first position except
  `status` and `help`. The hidden process entrypoints of 3.2 keep their existing
  argv, `internal` lists them, and no person or agent is told to type them.
- G2. The command table is keyed by `(object, action)`. Each entry carries its
  own usage forms, options, examples, audience and group. Help, JSON help, the
  Partner catalogue and typo suggestions are all generated from it. The ad hoc
  target-word matching inside handlers (`args[0] == "job"`, `reviewSubjectWords`,
  `kindAndTarget`) is removed; the router resolves object and action before the
  handler runs.
- G3. Internal implementation layers are not objects. Job, run, attempt, round,
  unit, chain, examination, launch record and resume id collapse into **work**,
  addressed by a goal (`G`, `G --work NAME`) or by a **work reference**. Test
  runs (proof attempts) are addressed under `test`, questions under `question`.
  The reference contract (VOA-12):
  - Every stored record keeps its identity; a reference is a qualified form over
    it: `j1:ID` launch record (`internal/launch/record.go`), `j2:ID` dispatch job
    (`artifacts/agents/jobs/ID.json`), `run:ID` unit run
    (`internal/launch/unit_run.go:803` store), `read:REF` diagnostic read,
    `wait:ID` durable wait resumption, `proof:ID` proof attempt (accepted by
    `test wait`/`test status` only). `j1:`/`j2:` exist today
    (`intent_process.go:576`); the others are added.
  - A bare word resolves in this order: a goal name (as `status G` does today);
    otherwise an exact raw id searched in every store the action applies to.
    Exactly one match is used. More than one refuses, nothing done, listing each
    qualified reference. A qualified reference is never ambiguous.
  - Each action declares which kinds it accepts (for example `work land` accepts
    a goal or `j2:`; `work wait` accepts a goal, `j1:`, `j2:`, `run:`, `read:`,
    `wait:`). A reference of another kind refuses before any effect and names
    the action that accepts it.
  - `work wait wait:ID` resumes that durable wait; `work wait TARGET` starts a
    new wait on the target. They are different owners (`intent_work.go:1227`,
    `:1265`) and stay different.
  - Refusals, results and remedies print the qualified reference, so a copied
    reference always resolves.
  - Tests: collision of a raw id across launch and unit-run stores, goal name
    equal to a raw id, each kind through each accepting action, each kind through
    one refusing action, `wait:` resume versus a new wait.
- G4. `metasystem status` with no target prints the overview the old bare
  `status` printed (this checkout's MetaSystem, running work, open questions,
  attention items), and `metasystem status G` prints one goal's work, as today.
  These are the only two top-level forms. Every other status is `OBJECT status`.
- G5. Every action takes `--repo PATH` and `--json`, as today.
- G6. Unknown object or action refuses before any effect, names the nearest
  object or action, and points at `metasystem OBJECT`.

The objects and their actions. Old spelling on the right; the builder of unit U1
carries this table into the command table and deletes the left column's
predecessor in the same slice.

| Object | Action | Replaces |
|---|---|---|
| **goal** | list | `goals` |
| | show | `show G` |
| | open, edit, approve, unapprove, budget, pause, resume, done, abandon, reopen, prioritize, pin, block, unblock, split, group, ungroup, claim, release | the same verbs at top level (`resume mission M` moves to mission) |
| | notes | `notes` (add/close as options, as today) |
| | accept-risk | `accept-risk` |
| | check | `check goals` |
| | repair | `repair goals ...` |
| **design** | write | `design G --brief` |
| | show, list | `show design --goal G`, `show designs` |
| | review | `review design FILE` |
| | stop | `stop design G` |
| | find | `internal project design-of` (skills) |
| | check-moves | `internal validate moved-effects` (design-critic role) |
| **work** | brief | `brief` |
| | build | `build` (and `build --resume RUN` as `work build ID`) |
| | review | `review G`, `review goal G`, `review G --changes/--patch`, `review commit SHA --goal G`, `review changes`, `review diff`, `review job J`, `review run RUN`, `review G --finding F --test` |
| | revise | `revise G`, `revise job R`, `revise run RUN` |
| | land | `land G ...`, `land job J`, and `land [G] --message FILE (--staged \| --path P...)` for a hand-made change (`scripts/agents/land.sh`, U5) |
| | wait | `wait G`, `wait goal G`, `wait job/run/proof/resume/review ID`, `wait G --for`, and `wait file PATH --until present\|absent` as `work wait --path PATH --until present\|absent` (caller-relative path, same observation and durable resumption, `intent_work.go:193,1219`) |
| | stop | `stop job J`, `stop review REF` |
| | status | `status job J`, `status run RUN`, `status work`, `show review REF` |
| | close | `done job J` (as `work close j2:J`), `repair review G` (as `work close G`) |
| | watch | `internal job watch`, `internal run watch` (turn facts; `work watch --job J` or `--run RUN`) |
| | report | `internal launch report` (retro skill) |
| | check | `internal validate conformance` (`work check --stage ...`), `internal validate critique-closed` (`work check --findings ... --dispositions ...`) (critique skills) |
| **test** | run | `test` |
| | plan, list, check, verify, report | `internal test plan/list/check/verify/report` |
| | wait | `wait proof REF` (as `test wait proof:ID`; U1a adds no `test status`, so `proof:` is accepted by `test wait` only) |
| **question** | ask, retry, withdraw | `ask`, `ask --retry`, `ask --withdraw` |
| | answer, show, list, wait | `answer`, `show question`, (new list over the same owner), `wait question` |
| **mission** | start, status, resume, repair | `start/status/resume/repair mission M` |
| **incident** | list, claim, close | `incidents ...` |
| **grant** | add, revoke, list | `grant`, `revoke`, (list over the same owner) |
| **decision** | list, show | `show decisions`, `show record ID` |
| **session** | start, stop, report | `start session`, `stop session`, `internal report stop-status` (the Stop hook's instruction) |
| | handoff, context, verify | `internal context handoff/status/verify` (steward continuation role) |
| **system** | start, stop, restart, status, check, repair, setup | `start/stop/restart/status [checkout]`, `check`, `repair waits`, `internal up --recover-only --if-down` (as `system start --if-down`, the cron line), `internal steward status` (as `system status --steward`); `setup` (U9, refinement): connect this checkout's runtime hooks and pre-commit fence to its engine, the per-checkout activation of 3.3 |
| **machine** | list, start | `status --machines`, `start machine NAME` |
| **ui** | start, stop, restart, status | `start/stop/restart/status ui` |
| **settings** | show, keys, check, coordinator | `settings`, `settings --keys`, `check settings`, `settings coordinator` |
| **terminal** | enroll | `enroll` |
| **receipt** | add, check, stats, correct, retro | `scripts/receipt.sh` and `internal receipt ...` |
| **experiment** | record, challenge, status, check | `internal report frontier record/challenge/status`, `internal validate stop-loss` (improve, take-a-step-back) |
| **critique** | rebind-budget, close-register | `internal job critique-budget-rebind`, `internal job critique-register-close` (critique skills) |
| **covenant** | check | `internal covenant validate` (inception skill) |

`help` groups the objects: Plan (goal, design, decision, grant), Deliver (work,
test, question, incident), Run (status, session, mission, system, machine, ui,
settings, terminal), Practice (receipt, experiment, critique, covenant).

Where the table says "same owner", the action calls the existing owner function;
no new capability is added. An action listed here whose owner only exists as an
internal verb handler today is bound to the handler's Go function, not to a
subprocess.

The builder may refine an action word when the critique or the build shows a
clearer verb, under two constraints: the word is a verb, and the choice is
recorded in this table in the same slice.

### 3.2 Process entrypoints

Some launches cannot be function calls: something outside the engine starts them,
or the engine starts them precisely because they must outlive, bound, or run in a
different binary from the caller. These are **entrypoints**. They are hidden from
public help, never named in a result, remedy or document for people or agents,
and listed only by `metasystem internal`.

**An entrypoint keeps its existing argv exactly** (VOA-03, VOA-06, VOA-11).
Renaming a hidden launch buys a person nothing, and today's spellings are
recognized elsewhere by exact argv: steward classification requires argv word 1
`steward` (`internal/lease/classify.go:504`), the janitor's kill proof matches
`supervise`, `mission run-loop` and adapter shapes (`internal/janitor/killproof.go:40,77`),
rearm-on-landed calls the rebuilt binary with `up` argv (`rearm_on_landed.go:410`),
the proof runner calls a pinned older engine with `test worker` /
`worker-capabilities` (`test.go:117,2037`, `test_protection.go:233,374`), seat
launch calls the destination engine with `validate session-isolation`,
`config validate|get`, `goal fetch|next`, `steward arm`, `up`
(`internal/seat/launch/sequence.go:512-781`), and generated delegate settings
embed `adapter claude-session-signal` (`internal/adapter/claude.go:122`). Keeping
the argv keeps every one of those boundaries working across binary generations
without a bridge.

Where an entry's first word is also a public object (`goal`, `test`, `mission`,
`ui`, `session`), the entry is a hidden action of that object in the same table
(for example `ui serve`, `test worker`, `mission run-loop`, `goal fetch`). A
public action and an entry never share an `(object, action)` pair; where a
current family verb has the same name as a new public action (for example family
`goal approve --id G` versus public `goal approve G`), the public action takes the
pair and every caller of the family form moves in U1 (6.5).

One pair is both (VOA-13): `test plan` is a public action and the machine
protocol a retained or candidate engine answers (`test.go:909,932`,
`test_protection.go:233`). It is one public registration that keeps its existing
argv and its raw plan JSON as its `--json` output (not the ordinary public JSON
envelope, `intent.go:596`); machinery-only options are hidden from help. Its test
calls it with the argv the preceding engine generation uses and decodes the
result strictly, as `test.go:932` does. No other pair may be both.

| Entrypoint (existing argv) | Launched by | Why a process |
|---|---|---|
| `supervise owner`, `supervise component` | `internal/supervise/arming.go:963`, `supervise_owner.go:142` | daemon in its own session, enrolled binary |
| `steward run` | `internal/steward/runner.go:870` | resident runner, enrolled binary |
| `mission run-loop` | `internal/missionrunner/launch.go:677` | detached loop |
| `launch supervise` (under `proc setsid --`) | `internal/launch/process.go:143` | detached launch supervisor |
| `seat launch` | `ui_launch.go:165-194` | must outlive the browser request and the UI server (VOA-04) |
| seat destination steps: `validate session-isolation`, `config validate`, `config get`, `goal fetch`, `goal next`, `steward arm`, `up` | `internal/seat/launch/sequence.go:512-781` | run on the destination checkout's own engine |
| `ui serve` | `internal/ui/lifecycle/launch.go:19` | background server with a ready fd |
| `ui tools` | `internal/ui/partner/runtime.go:91` (ACP `mcpServers`) | stdio MCP server started by the runtime |
| `proof-run watchdog`, `proof-run custody-exec` | `internal/proofrun/launcher.go:928`, `resource_custody.go:641` | sibling watchdog; exec barrier |
| `proof-run preserve` | `internal/proofrun/watchdog.go:228` | bounded copier the watchdog can kill; a blocked copy must not stop cleanup (VOA-05) |
| `proof-run worker-authorized` | suite commands via `METASYSTEM_PROOF_AUTH_BIN` (`proofrun/launcher.go:260`) | called by the suite process under proof; removed when the last suite script caller is gone (U7) |
| `test worker`, `test worker-capabilities`, `test plan` (also public, see above) | `test.go:117,2037`, `test_protection.go:233,374` | runs the pinned or candidate engine, a different binary |
| `brain boot-inputs` | `brain_boot.go:121` | killable child on a deadline |
| `run wrap` | `run.go:146-154`, `gate_cadence.go:385` | detached wrapper with log and nonce |
| `testing merge-driver` | git via `.gitattributes` | git merge driver |
| `adapter claude-session-signal` | delegate Claude settings (`internal/adapter/claude.go:122-127`) | runtime hook inside the delegate sandbox |
| `adapter claude-tool-gate` | PreToolUse hook (today `supervision-hook.sh:20`) | runtime hook |
| `up` | cron line (`internal/up/up.go:936`), rearm (`rearm_on_landed.go:410`), seat step | also the `system start` owner; kept as argv for these launchers |
| `hook RUNTIME EVENT` (new) | the plumbing stub `supervision-hook.sh` (3.3) | runtime hook body, replaces the script |
| `pre-commit` (new) | the plumbing stub `pre-commit-guard.sh` (3.3) | git hook body |
| `delegate-supervisor` (new, U6a) | the dispatch driver (U6b); the mission runner for host turns (`delegate-supervisor RUNTIME start-turn`, `internal/missionrunner/host.go:403`) | persistent owner of one delegate launch through completion and result publication, replacing the detached adapter supervisor (`dispatch.sh:1001`, `adapters/claude.sh:165`) (VOA-04) |

Entries that become function calls, because nothing outside the engine needs a
process for them: `steward revive`. `util hold` stays an engine verb as the fake runtime's stand-in CLI child (U6a: the fake is compiled into the engine and driven by shell beds, so a test-built helper cannot serve them; its janitor shape is `tagged-hold`). Two stay processes (U1b, confirmed by its read):
`up --recover-only` from landing (`landing_batch_owner.go:122-137`) is a
fire-and-forget detached launch whose recovery must not hold the ensure lock or
the joining seat; `delegate --revive` keeps its own session so dispatch never
inherits an operator's controlling terminal (`lease/classify.go:463` would read
HUMAN). `proc setsid` goes when `launch/process.go`
sets `Setsid` directly and its argv recognizers (6.6) move with it. The list may
shrink during the build; adding an entry needs a revision of this page.

`metasystem internal` prints the entries and their launchers. `internal ENTRY`
is accepted as today for launchers that already prefix.

### 3.3 No metasystem scripts; scripts only where users extend (revision 7)

Wido, 2026-09-27: "things that somebody using the meta system would like to
change or extend, that is easier in a script. And things that are truly meta
system behavior that nobody ever should want to change, that should become go";
the VM "should be just another test script or benchmark script that chooses to
launch inside the VM", never part of the standard metasystem.

**Rule S1.** No committed file under `metasystem/` is a script, except files a
user of the metasystem is expected to change or extend. Metasystem behavior
lives in Go. **Rule S2.** Nothing under `metasystem/` names, starts or depends on
`environment/vms`; a VM run is a test or benchmark script that chooses to
launch inside a VM. Witness (R11, VOA-24): a static test over `git ls-files`
fails on any `*.sh`/`*.bash` under `metasystem/` not matched by the exact
allowlist `metasystem/optional-skills/*/scripts/**` (skill helper tools) and the
two historical instruments `metasystem/plans/first-headless-run/gate.sh` and
`metasystem/plans/first-headless-run/guard.sh`; and on any `environment/vms`
reference in executable or configuration files under `metasystem/`
(`*.go`, `*.sh`, `*.bash`, `*.json`, `*.toml`, `*.yaml`, `*.yml`,
`metasystem.conf*`), excluding `_test.go` files that hold the witness itself.
Prose (`*.md`) may name the VM. Positive and negative fixtures pin both.
`benchmark/` lies outside `metasystem/` and is governed by S1 only as an
extension point: its scripts may call the engine through public actions only.

The end state for what earlier revisions kept as plumbing:

- **Runtime hook wiring.** The runtime settings command is generated data, not
  a script: `internal/hooks/setup.go` writes one command line that selects the
  engine as the stub does today (the primary checkout's `bin/metasystem`,
  resolved through `git rev-parse --path-format=absolute --git-common-dir`, so a
  linked worktree without its own binary uses the primary's engine,
  `supervision-hook.sh:19`), runs `internal hook RUNTIME EVENT` on it, and ends in
  the inline degraded fallback the settings already carry (`|| printf
  '{...}'`); `runtime setup` regenerates it in every checkout (VOA-19). The Go
  Stop worker, which today launches the stub by path (`hook_entry.go:104`),
  launches the engine's `internal hook` entry directly, keeping the
  deadline-parent identity, payload, installation context and cleanup (VOA-18).
  During the cutover the U4 stub bridges an engine older than the `hook` entry
  (it rebuilds detached). U9's activation, per checkout (m1e, m1b, m1c and any
  linked worktree): build and validate the selected engine (`internal hook
  --accepts`), then regenerate the settings to the direct command, then delete
  the stub. After U9 a missing engine yields the fallback JSON and the notice to
  run `go run ./cmd/devgate build`; an engine behind its sources is rebuilt by
  the Go hook itself. Witnesses: an allowed and a blocked Stop end to end with
  the stub absent; a linked worktree without a local binary; a checkout whose
  source is ahead of its engine.
- **Git pre-commit wiring.** The existing enrollment and composition contract
  stays (`internal/ledgerfence/fence.go`): the effective hooks directory
  including `core.hooksPath` (`:66`), preservation of foreign hooks (`:162`),
  `pre-commit.local` after the guard (`:210`), and the nonce/exit-status
  execution probe (`:118`). Only the guard invocation changes: the generated
  hook calls `bin/metasystem internal pre-commit` instead of the committed
  `pre-commit-guard.sh`, which U5 deletes (VOA-20). Witnesses: a rejecting
  `pre-commit.local` still rejects; a repository-local `core.hooksPath` gets the
  fence.
- **Engine build.** `go run ./cmd/devgate build` is the bootstrap; every caller
  (Go, docs, remedies, adopt) names it, and the `go-build.sh` stub is deleted in
  U9 once no caller names the path.

Declared extension points, where scripts are fine because users change them:
benchmark cases, their gates and graders (under `benchmark/`, outside
`metasystem/`), skill helper tools (`metasystem/optional-skills/*/scripts/**`),
and commands an adopting project writes in its own repository and testing
contract (never committed under `metasystem/`). Extension-point scripts may call
the engine only through public actions (R4 still applies). The benchmark kit's
engine-driving scripts (`provision.sh`, `validate-kit.sh`, `run-cohort.sh`) call
public actions only (U8). `environment/vms/*` stays outside `metasystem/` and
untouched. `plans/first-headless-run/*.sh` are a historical mission's contract
instruments: history, left as they are.

`cmd/devgate` is the Go bootstrap (VOA-08, VOA-09): run with `go run
./cmd/devgate ACTION` from the tree it builds, so it needs no installed engine
and cannot recurse into the test scheduler; actions `build`, `static`, `gate`
(U0b, U7a). It is a development tool for this repository, not a second public
surface.

`validate-metasystem.sh` is not ported as a script: its sections become Go tests
or `testing.json` groups; the section selector, `enumerate-suite.sh` and
`oldest-bash-gate.sh` go with it under the provider transition protocol (6.4).
`adopt.sh` becomes the public `system adopt`.

### 3.4 Every public action states an intent (revision 7, U1d)

Wido, 2026-09-27: size is not the problem; an action is wrong when it is a
technical verb that is not clearly, easily and intuitively an intent a person or
agent has. Rule G7: an action names what its caller wants done or wants to know,
in the caller's words; it never names a mechanism (register, record store,
published base, continuation, retained proof, job record, conformance stage).
Mechanics a public action needs run inside it. Unit U1d applies this table to
the U1a surface (it changes the public table, routing and the texts that name
these actions; owners are unchanged):

| U1a action | Problem | Revision 7 |
|---|---|---|
| `critique rebind-budget`, `critique close-register` | critic-register plumbing | no public action: the review flows (`work review`, `design review`) perform them; the `critique` object goes |
| `work check` | conformance and disposition plumbing | `work review SUBJECT --check-only` checks a subject's boundary and a round's dispositions without launching a critic |
| `work watch` | duplicates `work wait` with a pinned exit code | `work wait ... --exit-code` |
| `work close` | "complete a job's records" | `work finish REF`: record a finished job with nothing to review or land (an investigation, a read) as complete, through the existing close owner with its authority and durability checks (VOA-23); finished reviews complete inside `work review`, landed work inside `work land` |
| `work report` | "launch outcomes and refusals" | `work status --history [--since T]` |
| `design find` | duplicates `design list` | `design list --goal G` |
| `design check-moves` | critic tooling | performed by `design review`; a critic reading outside the command uses `design review FILE --check-only` |
| `goal check`, `goal repair` | ledger internals | `goal sync` with four modes: preview (default, changes nothing); `--recover` completes an interrupted goal change (binds `recoverGoalJournal`, the old default, `intent_operations.go:177`; VOA-22); `--refresh` completes an interrupted refresh; `--publish --goal G... --by NAME` publishes reviewed edits of exactly the named goals. The scope is enforced inside the reconciliation owner (`reconcilepub.go:38`): its request carries the allowed goal ids and it refuses, before recording any pending publication, when the captured deltas it would publish include another goal, naming them (VOA-21; the 175-file stale-base case of 2026-09-27). This is the one owner change U1d makes |
| `system repair` | lists wait continuations | `work wait --list` |
| `session context`, `session verify` | context-budget internals | inside `session handoff` (`--status`, `--verify`) |
| `session report` | named after the Stop mechanism | `session status [--id ID]`: why this session may or may not stop |
| `session stop` | wording | kept; summary "stop this session's work quietly" |
| `receipt check`, `receipt correct` | retro plumbing | `receipt status` (is a retro due, and the period's numbers; replaces `check` and `stats`); `receipt add --corrects LINE` |
| `test check`, `test verify`, `test report` | proof internals | `test status [--tree T]` (is this tree already proven, with cost); contract validation joins `settings check` |
| `terminal enroll` | a one-action object | `system enroll` |
| `covenant check` | a one-action object, shape validation | part of `system check` |

After U1d the objects are goal, design, decision, grant, work, test, question,
incident, session, mission, system, machine, ui, settings, receipt, experiment,
and the top-level `status`. `help agent` and skills name only these.

U1d build notes (the forms as built, recorded here per section 3.1's refinement
rule): `goal sync` keeps the two administrative modes the old `goal repair`
held, `--accept-remote-history --by NAME` and `--upgrade ...`, beside the four
above; `--publish` requires at least one `--goal`, and the owner's refusal code
is `RECONCILE_OUTSIDE_SCOPE`. `work finish` accepts `j2:` only and keeps the
close owner's `--evidence R` (the `REDUNDANT_READ` remedy) and `--dispositions
FILE`. `work wait --exit-code` takes `j2:J` (or a bare job id), and a tracked
run (the `internal run` store, not a unit run) as `--run ID`; `--list` takes
`--session S`. `work review --check-only` takes `j2:J --stage
review|recertify|merge [--test-command C] [--recertification R]`, or
`[j2:ROOT] --findings RETURN --dispositions FILE`. `session status` keeps
`--id ID` required (no newest-report lookup exists to default it).
`receipt add --corrects EPOCH:SHA1` names the corrected line by the epoch and
SHA-1 the receipt owner already keys it by. `test status` takes `--tree T` for
retained proof and `--result FILE --expensive-ms N` for a recorded result's
cost (the old `test report`). Every follow-up and every close
of a chain first rebinds the round budget of each open critic root it
continues (`CritiqueChainBudgetRebind`: the root itself when it is a critic's,
else every code-critic or warden root reviewing the implementation chain, the
selection `CritiqueExhaustionAdvance` inspects); a root whose goal is no longer
claimed keeps its stored limit, and any other rebind failure refuses the
follow-up or close. Register close already ran inside the close owner. `settings check` validates the
testing contract and its declared tools in process; it does not run native
discovery, which waits for the host's heavy proof lease (a test run resolves
native tests when it runs). `system check` reports the
covenant's shape when a `covenant.json` exists at the installation or
repository root.

**Rule C1 (clean code, Wido 2026-09-27: "There's nothing in the wild out
there. There's only us. So we can have clean code.").** There are no external
installations, so the build carries no compatibility code for old spellings: no
table from removed spellings to successors, no old-spelling refusal messages,
no tests pinning old spellings. An unknown command refuses before any effect and says
"did you mean" with the current command that was probably meant, computed only
from the current table (Wido: "it should not point to legacy code. It should
point to the [command] that was probably meant"): a first word that is a current
action of one or more objects suggests `OBJECT ACTION` with the caller's
remaining arguments (`approve x` → `goal approve x`; several objects list
each); otherwise the nearest current object or action by spelling. U1d deletes
U1a's removed-spelling table and its tests and adds tests for these two
suggestion rules.

### 3.5 Agent adapters: built-ins in Go, an external contract for new agents (revision 8)

Wido, 2026-09-27: moving the adapters to Go is fine, "but we do not want to lose
extensibility": supporting an agent the metasystem does not ship must stay
possible without changing Go, and "if people want to change the behavior of the
built-in adapter somehow, that is possible". Today both work by adding or
editing `scripts/agents/adapters/<name>.sh` and `hosts/<name>.sh`.

**One runtime interface, two implementations.** Every runtime, built-in or
external, implements the same Go interface; the shared layer (Go, identical for
every runtime: custody, deadlines and kill domains, record CAS and patches,
handshake bookkeeping, return normalization and turn adjudication, permission
comparison and refusal, the janitor's kill decision) only calls it. The
operations (VOA-25, VOA-26, VOA-27, VOA-30):

| Operation | Input | Output |
|---|---|---|
| `describe` | none | runtime name, schema version, capabilities (resume, follow-up, repair, wait delivery, usage), classification signatures with their exclusions, the claim-bound invocation shape (where a claim tag sits in argv, accepted prefix and basename variants), config paths |
| `probe` | none | installed, version, and the runtime's own permission-denial check |
| `selftest` | the shared selftest parameters | the runtime-specific evidence the shared selftest judges |
| `prepare` | turn context: role (`delegate` or mission `host`), job or mission and turn, resume identity, requested permissions, workspace, prior usage | argv, environment, stdin, settings files and callback channels to write, artifact paths to observe, and the **effective permission envelope** the settings and command produce |
| `observe` | the CLI's stdout stream and the declared channels and artifact paths | events: session and turn handshake, effective model, delivery, protocol violation |
| `finalize` | CLI exit status, artifacts, prior usage | result candidate, usage (including repair spend), observed session identity |
| `repair` | only when `describe` declares it: the violation and the same session | a repaired result, through the same custody |
| `cancel` | the owned process group | runtime-specific cancellation beyond the shared group kill, if any |

The shared layer compares the effective envelope with the request and refuses
a wider grant before starting the CLI; mapping a request to settings is the
runtime's (the "working directory is the write boundary" rewrite of
`internal/adapter/permissions.go:20` becomes the built-ins' choice). The
mission runner uses the same interface with role `host` (today
`missionrunner/host.go:337` and `hosts/*.sh`), so a new runtime can serve both
delegate jobs and mission host turns.

**Built-ins** (claude, codex, devin, fake) implement it in Go (U6a).
Built (U6a): process recognition (census, lease classification, human
authority) used the built-ins' signatures only until U6c. Built (U6c): the
registry (`internal/runtimes/external`, `Load`) holds the built-ins and the
named, trusted externals as one effective declaration per name with the
cross-check and the VOA-31 merge; every recognizer, the janitor's shapes,
configuration validation, wait delivery, dispatch, probes, self-test and the
mission runner read it; the executable implementation of the operations is
`internal/adapter/supervisor/external.go`; `docs/agent-adapters.md` is the
contract. A refused override leaves the built-in's declaration to the
recognizers and refuses running the runtime until fixed; a launch an
override's own prepare made must be observed and finalized by it (a built-in
cannot continue a launch it did not prepare). The Fable read's F1-F3 and
F5(ii) are folded: a refused override keeps protecting its built-in in the
cross-check; an override that leaves describe to its built-in keeps the
built-in's whole description (repair included); a named but unsafe override
refuses running the runtime with the fix; the shared self-test runs against
an external runtime. Later (U6c read), one line each:

- F4: adapter trust checks the file only: not the `adapters/` directory's
  owner and mode, not a symlink's target (os.Stat follows it), and the check
  and the exec are separate (a swap between them is not refused).
- F6: an override may declare its own positive vector and stop claiming the
  built-in's real CLI while its prepare falls back to the built-in, which then
  launches that CLI unrecognized; require an override to claim the built-in's
  positive vector unless it also answers prepare.
- F7: the cross-check tests declared vectors only; a signature matching a flag
  real claude or codex processes carry is admitted and, sorting first by name,
  labels them (both are DELEGATE, so only the runtime label is wrong).
- F8: the host path skips LookPath for an argv[0] containing '/', so a missing
  absolute program fails at start instead of with 127.
- F9 (fixed at U6c): a relative --root no longer hides the installation's
  external adapters; the entry answers with its usage.
- F10: every named adapter's describe runs in every recognizing process (hook,
  census, janitor, settings show, system check), memoized per process only;
  a cost and exposure to weigh when adapters multiply.
- F11: still on the compiled runtime set: `internal runtime` list and lookup,
  the hook's start context, steward context sampling, config tailor, the seat
  manifest, LocalConfigManifest and runtimes.Lookup itself; route them through
  the registry when a real external agent is adopted.
**External runtimes** implement it as an executable at
`<installation>/adapters/<name>` (any language; an extension point under rule
S1): `<executable> OPERATION`, JSON request on stdin, JSON response on stdout
(`observe` writes one JSON event per line), exit 0 on success (U6c).

**Trust boundary (VOA-28).** An external adapter is trusted code, installed by
a person: it runs only when the installation's configuration names it
(`adapters.<name>.use=external`), the file is owned by the installation's user
and not group- or world-writable, and discovery never executes an unnamed or
unsafe file (it reports it). **Overriding a built-in** is the same setting on a
built-in's name; an override may implement only some operations and exit with
the reserved code 64 to fall back to the built-in for the rest. `settings show`
and `system check` report every external adapter and override. The registry
refuses a declaration whose classification signatures match another runtime's
declared positive vectors or its reserved exclusions (each `describe` carries
positive and lookalike vectors; the registry cross-checks all of them), so one
runtime cannot claim another's processes (for example Devin's `devin acp`
intermediary).

**One effective declaration per runtime name (VOA-31).** An override of a
built-in yields a single declaration under that name: its signatures are the
override's, but the built-in's reserved exclusions and lookalike vectors (the
helpers its retained fallback operations depend on, such as `devin acp` at
`internal/runtimes/runtimes.go:206`) are always part of it. The registry refuses
an override whose signature matches any of the built-in's reserved lookalike
vectors, while matches of that runtime's own positive vectors are not
conflicts.

**Recognizers.** Census and lease classification take signatures from the
registry. The janitor keeps its kill proof in Go (kernel identity plus
claim-consistent argv, `janitor/killproof.go:19,206`) and takes each runtime's
claim-bound invocation shape declaratively from `describe`; a runtime's
assertion that a process is its own never authorizes a signal (VOA-29).

**Every consumer uses the registry (VOA-30):** configuration validation
(`config/validate.go:170`), `internal/runtimes` resolution (`runtimes.go:298`),
probes (`adapter/probe.go:47`), selftest, dispatch and the mission runner.

The operation schemas are versioned and documented in `docs/agent-adapters.md`.

### 3.6 A human is never denied a verb (rule H1, 2026-09-27)

Wido, 2026-09-27: "if a human wants to use a verb; that should never be denied.
Ever. Unless it damages the system somehow so then it is more like protecting
the human against himself and advising the human on how to do it properly (but
that is like detecting the wrong way, pointing to the right way: not
blocking)".

**Rule H1.** An authenticated human (the HUMAN classification, with the human
proof a verb already requires) is authorized for every public action and every
control-plane mode. A refusal a human can meet is allowed only when the act
would damage the system; it then states what would go wrong and names the
command that achieves the person's intent the right way, so no refusal is a
dead end. Machinery separations (custodial bookkeeping, holder-only writes) are
not damage: the human's act is admitted and recorded as human-ordered. First
case: `stop-custodian` (`internal/authority/authority.go:27-32`) admits HUMAN;
a human-ordered breach stop records the person as its actor.

Unit U-H1: audit every refusal an authenticated human can reach (authority
modes, owner refusals in internal/goal, landing, dispatch, delegation, steward,
proofrun, and the command layer), classify each as admit or guide, fix both
kinds, and add witness R14.

## 4. Rules and their witnesses

Witness tests land in U0 with today's counts as ceilings and tighten in every
later unit (a ratchet: numbers, not lists, except where a list is the rule).

| Rule | Statement | Witness |
|---|---|---|
| R1 | First-position words are the objects, `status`, `help`, `internal`, and the first words of the 3.2 entries. Old public spellings refuse. | Router test: every table pair routes; every removed spelling refuses before effect with a suggestion; transitional family fallthrough count ratchets to zero by U9. |
| R2 | Every public form is `OBJECT ACTION ...` (or one of the two `status` forms); help, JSON help and the Partner catalogue come from the one table; hidden entries never appear. | Test renders every help surface and checks each usage line. |
| R3 | Entries are exactly 3.2's list, each with a launcher reference that exists in non-test code. | Test: entry set equals the table's hidden set; each launcher string found. |
| R4 | No committed shell file invokes the installed binary except the stubs of 3.3. | Static test over `git ls-files '*.sh' '*.bash'` for binary invocation patterns (`$ms`, `${ms}`, `$engine`, `$deadline_engine`, `$gate_build_scratch`, `bin/metasystem`, `METASYSTEM_BIN`, `METASYSTEM_PROOF_AUTH_BIN`, `go run ./cmd/metasystem`, pass-through `"$@"` into those); ceiling ratchets to the stub count. A second ceiling on total shell lines under `metasystem/scripts`. |
| R5 | Public actions never subprocess the engine for a non-entry, except a proof launched by a resident owner (6.2). | Static test for exec of `os.Executable()`, `os.Args[0]` or a resolved engine path whose argv is neither an entry nor the resident proof launch; ceiling ratchets to zero. |
| R6 | Text shown to people and agents names only public forms. | Test extracts `metasystem W W` and `bin/metasystem W W` from Go string literals, live docs, skills, role packets, `AGENTS.md`, `wow.md` and UI source (excluding `records/`, historical `plans/`, `memory/`) and checks each against the public table. |
| R7 | Authority parity for every subprocess replaced by an in-process call (6.2). | Per replaced call: authorization and refusal parity, holder classification and claim epoch, human-proof rejection, hand-back and release, compared against the subprocess era on the same fixture. |
| R8 | Process recognizers follow their process (6.6). | For each entry and each ported long-lived process: classification and authorized versus refused signalling against the process's actual argv. |
| R9 | Orchestration sits above owners (6.3). | `go list -deps` test: no package under `internal/` that is an owner imports an orchestration package. |
| R10 | Hard cutover per slice. | R1-R9 green at every landing. |
| R11 | No metasystem scripts outside declared extension points; nothing in `metasystem/` depends on `environment/vms` (3.3). | Static test over `git ls-files` under `metasystem/`: extension-point allowlist by directory, zero other `*.sh`/`*.bash`, zero references to `environment/vms`. |
| R13 | A new agent needs no Go change, and a built-in can be overridden (3.5). | With an engine built before the helper exists: a uniquely named external runtime passes configuration validation, dispatches, resumes, cancels, serves a mission host turn and runs its selftest with a custom probe; the fake runtime as built-in and as external executable produce identical records; a partial override changes one operation and falls back (64) for the rest; an unnamed executable, an unsafe file and a signature overlapping another runtime's vectors are refused; the janitor kills a correctly tagged orphaned external CLI and leaves wrongly tagged or reused-pid processes alone; an override of Devin's `describe` that falls back for host preparation is refused when it drops the `devin acp` exclusion, and with the exclusion kept a host turn's tool call still classifies through its announced main. |
| R14 | A human is never dead-ended (3.6). | For each refusal reachable with a HUMAN classification: either it no longer refuses, or its message names a runnable public command; a static test over the refusal register and the authority modes fails on a new human-reachable refusal without a way forward. |
| R12 | Every public action states an intent (3.4, G7). | Router test pins the revision 7 table; an unknown command's "did you mean" comes from the current table only (C1). |

## 5. Units

Waves run in order; units inside a wave run in parallel (at most three builders).
**Every unit is at least one landing, and every landing carries delivery proof
for its own tree** (6.4). Units that replace a proof provider land in two steps
(6.4). A unit larger than about 1,500 lines of new Go splits at a seam the
builder names.

**Wave 0 (serial)**

- U0 Witnesses. R1-R9 tests with today's counts as ceilings. No behavior change.
- U0b Bootstrap build. `cmd/devgate build` and the self-resolving `go-build.sh`
  stub (3.3), with the Go callers (`test.go:1506`, `rearm_on_landed.go:73,399`,
  `seat/launch/sequence.go:450`) moved to it. Lands before any unit whose cutover
  needs a rebuild (VOA-11-R2).
- U1 Grammar. The `(object, action)` table with hidden entry pairs, router,
  generated help, the reference contract (G3), all public forms of 3.1 including
  the public homes of agent-facing internal verbs. Deletes every old public
  spelling and `intentYieldsToLegacy`/`command.legacy`; the only fallthrough left
  is to family verbs whose `(word, verb)` pair is neither a public action nor an
  entry (transitional, R1 ratchet). The top-level machinery branches of
  `dispatchInternal` that are not entries (`wait`, `delegate`, `watch`, `health`,
  `arm`, and the flag forms of `stop` and `status`, `main.go:822-850`) are not
  families: U1 moves every caller of them (for example `dispatch.sh:1131`
  `wait --job`, `stoptransition/families.go:241` `delegate --cancel`,
  `goal_branch.go:216` `delegate --follow-up`, `intent_delivery.go:884`) either to
  an owner call under 6.2 or, where the caller is a script awaiting its port, to
  the explicit `internal` form, which stays routed until that port lands
  (VOA-03-R2). Cancellation, wait and follow-up tests run in U1. Moves every caller of a family form whose
  pair a public action now takes (6.5), in shell and in Go argv. Updates skills,
  role packets, `AGENTS.md`, `wow.md`, docs, UI strings (including the fleet
  card's `stop --repo`, `httpd/walkthrough/fleet.go:231-288`) and remedy texts.
  Replaces the public layer's subprocess calls (`engineVerb`, `subprocess`,
  landing batch children) with owner calls under 6.2.

U1 is built as three parts that land together: U1a the table, router, help,
references, public forms, deleted spellings and every caller and text (colliding
Go callers may use the explicit `internal` form until U1b); U1b the subprocess
replacements under 6.2; U1c the hook-side bootstrap and the retained-plan scan,
migration and resume refusal of section 7.

U1d (revision 7) applies 3.4's table after U1a: public table, routing, removed
spellings with suggestions, and every text naming them; owners unchanged except the
reconciliation owner's scoped publication (3.4, VOA-21).

**Wave 1 (parallel)**

- U2 Dead verbs, under 6.1.
- U3 Shims and leaves: Go callers off `assert-return-complete.sh`, `receipt.sh`,
  `arm-supervision.sh`, `audit-metasystem.sh`, `dependency-ratchet.sh`,
  `refactor-baseline.sh`, `metasystem-config.sh`, `emit-event.sh`;
  `sync-transport.sh`, `coverage-delta.sh`, `preflight-commands.sh`,
  `validate-skill*.sh`, `evidence-gc.sh`, `second-session.sh`,
  `watch-background-jobs.sh`; `fake.sh` and `fingerprint-harness.sh` cut loose
  from `fixture-budget.sh`. Each with its recognizers (6.6).
- U4 Hook. `supervision-hook.sh` and `stop-degraded-forms.sh` become `internal
  hook` under `internal/hooks`; the stub of 3.3; hook fixtures port; the reader of
  the fixture file (`internal/audit/hookstartexits.go:44`) moves to the Go tests.
- U7a Gate. `devgate static` and `devgate gate` replace
  `go-gate.sh` and `witness-gate.sh` under the two-step transition (6.4): step A
  lands `devgate` and switches `fast-static-build` and the gate groups to it
  while the scripts still exist; step B deletes the scripts.

**Wave 2 (parallel)**

- U5 Landing. `commit.sh`, `land.sh`, `pre-commit-guard.sh` (stub), the wrapper
  token and `sync-transport.sh` into `internal/landing` behind the public `work
  land` and a Go commit path, in two steps (6.4): step A lands the Go path
  alongside the scripts, minting the same wrapper token format the base guard
  verifies, and is itself landed through `commit.sh`; step B is landed through
  the Go path, switches the guard stub to `internal pre-commit`, and deletes the
  scripts. `landing/batch/transport.go:301` and `intent_exception.go:227` call Go.
  Fixtures port.

  U5 build notes (step A, as built): the composition package is
  `internal/landing/landpath` (`Commit` is the commit boundary, `Land` the
  driver with its carried transaction, `Guard` the pre-commit guard body); every
  owner and Git are reached through injected `Owners`, which
  `cmd/metasystem/landing_path.go` wires to the owner functions in the
  landing's own process (6.2: the current process is the supplied identity,
  the lineage is named on the request). The public form for a hand-made change
  is `work land [G] --message FILE (--staged | --path P...)` with land.sh's
  declarations as options (`--chain`, `--recertification`, `--test-receipt`,
  `--direct-fix`, `--revert-of`, `--root-job`, `--tests`, `--allow-new-plan`,
  `--skip-transport`). The wrapper token keeps the shell format
  (`wrapperPid`, `wrapperPidStartedAt`, a 32-hex nonce, `createdAt`), minted by
  the landing process. By rule C1 the contract-off branch of commit.sh is not
  ported: the boundary refuses without a testing contract (coverage delta,
  `go-gate.sh --fast --proof-out` static re-proof, `internal audit metasystem`
  and `--ratchet` are gone). The in-process engine always has `landing drift`,
  `advance` and `receipt-line`, so the older-engine fallbacks are not ported.
  Carried asks name the public forms (`work land G --using-exception X`,
  `work land G --exception PAST --replace-exception X`). Step B adds the
  `pre-commit` entry, switches the enrolled hook composer to it, and deletes
  the scripts.

  U5 build notes (step B, as built): the composer
  (`ledgerfence.composerFor`) is the shebang, one `prefix='...'` line and a
  fixed body that runs `bin/metasystem internal pre-commit --root
  <installation>` (a linked worktree without its own engine runs its primary
  checkout's), then `pre-commit.local`; a retired script-guard composer is
  recognized and upgraded in place by the next `Ensure` (any goal mutation), and
  `adopt.sh` writes the same bytes. `work land --message FILE --staged --local`
  is the former `commit.sh` without `--push` (benchmark provisioning uses it).
  The internal verbs whose only callers were the deleted scripts are deleted
  under 6.1 (`landing advance`, `drift`, `carry-status`, `held`,
  `receipt-line`, `sync-transport`, `gate weight-add`, `output spill`,
  `validate wrapper-token`); their owners stay and the landing path calls them.
  `landing park` stays: the recertified-landing bed drives the park owner's
  failure path through it.
- U6c External runtimes (revision 8). The external-executable implementation of the runtime interface, the registry and its trust rules, every consumer on the registry, `docs/agent-adapters.md`, R13. After U6a.
- U6a Runtimes. `adapters/runtime-common.sh`, the four adapters,
  `host-common.sh` and the four hosts into Go; the `delegate-supervisor` entry;
  adapter signature discovery from a Go runtime registry instead of enumerating
  `adapters/*.sh` (`lease/classify.go:257`) (6.6). dispatch.sh launches
  `delegate-supervisor` until U6b. Fixtures port.
- U6-seam (serial before U6b, may run alongside U5/U6a). The dispatch
  composition package (6.3) with its injected lease, steward, adapter and goal
  operations; no behavior yet.

**Wave 3 (parallel)**

- U6b Dispatch. `dispatch.sh` with `checkout-execution-guard.sh` and
  `emit-event.sh` into the composition package; the `__` callbacks become calls;
  the `job` family goes. At least three sub-units at the lifecycle seams
  (launch, record/CAS, close/reap). Fixtures port.
- U7b Suite. `validate-metasystem.sh` sections become Go tests or groups under
  the two-step transition; selector, enumerate, oldest-bash gate, canary,
  fingerprint harness and fixture libraries go with the last fixture that needs
  them. Remaining fixtures port here or with the unit that touches their subject.

**Wave 4 (serial)**

- U8 Adopt and benchmark: `system adopt`; benchmark scripts per 3.3; their
  fixtures port.
- U9 Close. Runtime settings regenerated to the direct `internal hook` command and the `supervision-hook.sh` stub deleted; the `go-build.sh` stub deleted; R11 at zero scripts outside extension points. Transitional family fallthrough and the family registry deleted; R1,
  R3, R4 at their final values; `help agent` and docs final pass; a fresh-eyes
  usability run by an agent that has never seen the CLI (discover, plan, build,
  review, land, recover from a refused land) recorded in the goal's verification
  page.

  U9 build notes (hook part, as built): the settings renderer
  (`internal/hooks/setup.go`) writes the direct command: Git discovery, `cd`
  into the installation, the engine selected as the stub selected it (the
  primary checkout's `bin/metasystem` through `--git-common-dir` for a linked
  worktree; Claude's tool gate prefers a local executable engine;
  `METASYSTEM_BIN` only beside an installed engine), `exec ENGINE internal
  hook RUNTIME EVENT`, and the inline degraded answer naming `go run
  ./cmd/devgate build` when no engine is installed. The shipped templates name
  `metasystem internal hook RUNTIME EVENT`; the stub-era rendered commands are
  recognized and replaced. The entry takes its installation from its working
  directory when no stub names itself, and the Stop deadline parent launches
  its worker as the engine's own entry in that directory
  (`hooks.LaunchEngineWorker`). The engine-behind-its-sources rebuild was
  already the Go start's (`bootstrapEngine`). The per-checkout activation is
  the public `system setup` (refinement, 3.1): it validates `internal hook
  --accepts` on the selected engine, renders the hook settings of every
  runtime the checkout already registers (hooks only), and runs the fence
  enrollment, which upgrades a composer that still runs the deleted
  `pre-commit-guard.sh`; a repeat writes nothing. Until a checkout switches, a
  commit through the Go commit path that such a composer refuses names
  `metasystem system setup` (H1). The template repository's own tracked
  settings are regenerated in the same landing, so a seat that pulls runs the
  direct command from its next session; its `system setup` then validates the
  engine and re-enrolls its fence. The stub is deleted in a following commit
  once every seat has switched; with it go the entry's stub path
  (`METASYSTEM_HOOK_SCRIPT`, `Invocation.Script`) and its tests, while setup
  keeps recognizing stub-era settings so a late checkout still switches.

## 6. Protocols

### 6.1 Deleting a verb

Deleted only when all hold, recorded in the unit's return:

1. Its registration is removed and the module builds.
2. A full-text search of the whole repository for the verb word near its family
   word finds no invocation: shell arrays (`args=(family verb)`), variables
   (`"$engine"`, `job "$verb"`), pass-through wrappers (`receipt.sh`), Go string
   slices, hook text generated in Go, argv recognizers, remedy and hint strings.
   Matches in `records/`, historical `plans/` and `memory/` do not count.
3. Its handler function goes if nothing else calls it; the owner function it
   wrapped stays if it has another caller.
4. Targeted tests of its package and `cmd/metasystem` pass.

### 6.2 Replacing a subprocess with a call (VOA-02)

Today a child process classifies its parent (`goalsync_mutations.go:738`), proves
human ancestry from it (`:631`) and derives claim-epoch authority from that
classification (`:706`); the landing owner announces its own pid and gives its
children explicit lineage (`landing_batch_owner.go:179,365`). A function call
inside the parent would classify the parent's parent instead.

So the owner functions take an explicit **invocation context**: the supplied
process identity (pid, start time) that classification starts from, its
authenticated classification and claim epoch, the human-proof context, the
selected roots, and the lineage. The supplied identity is fixed per call edge
(VOA-02-R2):

- Where a child is replaced, the supplied identity is the **current process**,
  because that is the parent the child supplied. The classifier starts
  runtime-signature checks at the supplied process's parent
  (`lease/classify.go:426`, `lease/directinvoker.go:11`), so starting from the
  current process's caller would skip the invoking runtime and could classify an
  unannounced, terminal-bearing agent as HUMAN (`classify.go:463`).
- Where an owner is already called directly, it keeps its existing supplied
  identity.
- The landing owner supplies itself, as its children did.

Witness beyond R7: an unannounced agent runtime with a controlling terminal runs
`settings coordinator --declare --by NAME` directly and is refused by the human
gate (`brain.go:28-32`), as today.

A proof keeps its own process when the caller is a resident owner (VOA-15).
Admission records the current process as the proof's launcher
(`proof_run.go:1130`, `proofrun/launcher.go:125`) and goal-stop cancellation
signals that launcher (`dispatch/stop.go:707`, `proofrun/stop.go:64`). Run
in-process inside the landing batch owner, cancelling one proof would signal the
owner serving every other batch. So the landing batch owner's proofs
(`landing_batch_prove.go:322`, `landing_batch_red.go:91`,
`landing_batch_land.go:553`) stay child processes running `test run`, with the
invocation context passed explicitly rather than through inherited environment.
Witness: cancel one proof while the owner is proving another batch; the owner
survives and completes the other.

Per-invocation execution state moves with the call (VOA-14). A replaced child
discarded its environment on exit; an in-process call inside a resident owner
does not. Every environment variable the reached code reads or writes as
invocation state (for example `METASYSTEM_PREPARATION_RESTARTED`,
`test.go:485-498`) becomes a field of the request; the builder lists them by
searching `os.Getenv`, `os.Setenv` and `os.LookupEnv` in the reached code and
records the list in the return. Witness: two consecutive preparations in one
resident owner, each with one base movement, both admitted; two movements within
one invocation still refused. Hidden reads of `os.Getppid()` and of
`METASYSTEM_OWNER_LINEAGE` inside these owner paths are removed; no code sets
process-global environment to imitate a child. R7 witnesses each replacement.

### 6.3 Where orchestration lives (VOA-07)

`internal/lease` and `internal/steward` import `internal/dispatch`
(`lease/sweep.go:14`, `steward/tick.go:12`), so dispatch orchestration cannot sit
in `internal/dispatch`. The ported dispatch lifecycle goes in a new composition
package `internal/delegation` that imports dispatch, lease, steward, adapter and
goal; those packages never import it (R9). The same rule places the hook
(`internal/hooks` composing report, steward, brain, lease) and the landing path:
a builder that finds a cycle creates the composition package above the owners
rather than moving a decision into a lower package or duplicating it.

### 6.4 Proof for every landing, and provider transitions (VOA-01, VOA-10)

Every landing runs delivery selection and verification for its own tree through
the existing machinery, reusing eligible unchanged group results. There is no
"proof once per wave"; an additional aggregate run per wave is optional.

Base-controlled groups keep the base's command for a candidate's own proof
(`testpolicy/protection.go:55,187`), and `commit.sh`, `land.sh` and
`validate-section-selector.sh` are protected paths (`protection.go:40-45`). So a
unit that replaces a proof provider or the landing path lands in two steps:

- Step A adds the Go provider and switches the group command (or the landing
  path) while the old script still exists. Its proof runs the base's old command
  against a tree where that script still works.
- Step B, after A is on main and is the base, deletes the script. Its proof runs
  the new command.

The scenario map (6.7) for a provider names the replacement group, selector and
assertions, and the unit's return shows step B's proof actually discovering and
executing them.

### 6.5 Callers of a pair a public action takes

Where a public action takes a `(word, verb)` pair a family already uses with a
different grammar (`goal approve --id G`, `goal done`, `goal show`, `goal list`,
`test run`, `test plan` as a public action versus entry, `mission start`, and
the rest of `intent_coverage_test.go:80-134`), U1 lists every caller of the family
form in shell, Go argv and text, moves each to the public form or to an owner
call, and adds each pair to the R1 router test. A family form that is also an
entry (3.2) keeps its argv and does not become public.

### 6.6 Process recognizers follow their process (VOA-06)

Any code that recognizes a process by argv or by a script path is part of that
process's cutover: `internal/lease/classify.go:257,504`,
`internal/janitor/killproof.go:40-77`, `internal/census/run.go:285,732`,
`internal/census/ancestor_production.go:133`, `internal/census/fingerprint.go:91`,
`internal/humanauthority/authority.go:619`, `internal/dispatch/attest.go:47`,
`cmd/metasystem/wait_verb.go:82`, `intent_worktree.go:308`,
`internal/adapter/toolgate.go:60,248`, and any found by 6.1 step 2. The unit that
replaces a script or changes a long-lived process's argv updates these to the
new process shape, sourced from one Go definition shared by launcher and
recognizer, and adds R8 tests.

### 6.7 Porting fixtures

The unit's return carries a scenario map: every scenario (named case, `assert_*`
block or section) of each retired fixture file, with one of `ported to TEST`,
`covered by TEST (existing)`, or `retired: the behavior it tested is deleted
(which)`. A scenario without a row blocks the landing. Ported tests follow the
local rules: Git stubbed per test instance, injectable clocks, no retries, no
global fake state.

### 6.8 Landing

Static gate first, then impacted tests (changed packages, reverse dependents,
the `cmd/metasystem` audit tests, R1-R10 witnesses, race for changed packages),
then delivery proof for the tree (6.4), then a Fable code read with no breaking
finding. Each landing is a commit in Wido's name with a `Landed-By: m1e` trailer
and is pushed to origin. A red on main stops new landings until fixed.

## 7. Cutover effects (VOA-11)

Because entrypoints keep their argv (3.2), rearm, seat steps, pinned test
engines and generated delegate settings cross engine generations unchanged. What
changes per landing is the public grammar, deleted internal verbs, and the two
new entries (`hook`, `pre-commit`) with their stubs.

- The stubs tolerate an older engine by rebuilding once (3.3). A unit that
  changes the hook stub lands only with the engine that serves it in the same
  commit, so a checkout that pulls and rebuilds is consistent.
- The cutover executor is the existing authorized rearm-on-landed decision and
  rebuild (`rearm_on_landed.go:73-83,462`). Today only test preparation calls it
  (`test.go:560`, `rearm_on_landed.go:548`), so a checkout that pulls and runs
  no test keeps its old engine (VOA-11-R3). The trigger must run before the new
  engine exists, so it lives on the hook side, whose script comes from the pulled
  tree (VOA-11-R4). U1 adds to `supervision-hook.sh`'s start event, before its
  `session start` call (`supervision-hook.sh:1101`), a bootstrap step: when the
  installed engine's stamp is behind the tree's landed engine inputs, run the
  existing authorized rearm command, the rebuild followed by `up` re-enrolling
  the rebuilt engine (`rearm_on_landed.go:73`, `up.go:566`); the hook already
  calls `up`. The step first runs the retained-plan check below and does not
  rearm while it reports an unmigratable run. From U4 the Go hook and its stub
  carry the same step. Witness: a checkout with a preceding-generation engine
  installed and only its source advanced to U1 starts a session; the engine is
  rebuilt and re-enrolled, and `work build` then routes. `system stop`
  and `system start` stay human-only (`process_verbs.go:172,479`) and are not
  part of the automatic cutover (VOA-11-R2). U0b moves that rebuild to
  `devgate build` before any later cutover depends on it; until then it uses
  today's `go-build.sh`. Peers (m1b, m1c) cut over when their checkout's tip
  includes the landing, through the same path; if a peer's steward is not armed,
  its human restarts it.
- A unit that removes a verb a live delegate might call lands only when the
  checkout's job registry shows no running delegate launched before it.
- Retained executable plans (VOA-16; revised by C1). Measured 2026-09-27: 151
  retained unit runs on m1e (148 awaiting judgement, 3 running), none storing an
  engine command in its proof argv, and no other installation exists. So there
  is no migration code: the landing precondition is only that no retained run
  stores a removed spelling, checked once by hand before the grammar lands on
  main. Unit U1c is withdrawn.
- The browser UI: acts call Go functions; displayed command strings change in U1
  (`ui/decisions/decisions.go:817`, `ui/act/act.go:20,45,220,665`,
  `lifecycle/*`, `httpd/walkthrough/fleet.go:231-288`,
  `web/_app/src/fleet/launching.ts:330`, `session/session.go:385`,
  `httpd/walkthrough/smoke.go:51`). Browser machine creation keeps its detached
  `seat launch` entry (3.2). The UI lane's g1-s58 public-grammar unit waits for
  U1 and consumes its table.
- Records, receipts and history keep old spellings; R6 excludes them.

## 8. Non-goals

- No change to authority, ledger, lease, landing or proof semantics. A latent
  defect a port finds is recorded as a finding for a separate goal unless
  preserving it would require keeping Bash.
- No new capability beyond public homes for things agents are already told to do.
- No renaming of entrypoints (3.2).
- No compatibility aliases, deprecation warnings or old-spelling hints on the
  public surface.
- No restructuring of `internal/` beyond composition packages (6.3), moving
  script logic in, and deleting verb handlers.

## 9. Critique record

Round 1, Codex `gpt-6-astra`, read-only, against revision 1: twelve material
findings, all accepted. Verbatim critique: `verbs-object-action-astra.md`.

| Finding | Disposition | Where folded |
|---|---|---|
| VOA-01 protected proof providers | accepted | 6.4 two-step provider transition; U7a, U7b, U5 |
| VOA-02 subprocess authority context | accepted | 6.2 invocation context; R7 |
| VOA-03 Go launchers lose routing | accepted | 3.2 entries keep argv; U1 fallthrough scope; 6.5 |
| VOA-04 controllers that outlive callers | accepted | 3.2 `seat launch`, `delegate-supervisor`; 7 |
| VOA-05 `proof-run preserve` bound | accepted | 3.2 kept as entry |
| VOA-06 argv recognizers | accepted | 3.2 argv kept; 6.6; R8 |
| VOA-07 dispatch import cycles | accepted | 6.3 `internal/delegation`; U6-seam; R9 |
| VOA-08 bootstrap fence and stamp | accepted | 3.3 `cmd/devgate build`; U7a |
| VOA-09 recursive static group | accepted | 3.3 `devgate static` leaf |
| VOA-10 proof per landing | accepted | 6.4; 6.8 |
| VOA-11 generation cutover | accepted | 3.2 argv kept; 3.3 stub rebuild; 7 |
| VOA-12 work reference contract | accepted | G3 |

Round 2, Codex `gpt-6-astra`, against revision 2: eight folds verified, six
material findings, all accepted.

| Finding | Disposition | Where folded |
|---|---|---|
| VOA-02-R2 supplied identity per call edge | accepted | 6.2 |
| VOA-03-R2 top-level machinery branches | accepted | U1 |
| VOA-08-R2 stub location independence | accepted | 3.3; U0b |
| VOA-11-R2 cutover executor and bootstrap order | accepted | 7; U0b |
| VOA-13 `test plan` public and entry | accepted | 3.2 |
| VOA-14 invocation-local state | accepted | 6.2 |

Round 3, Codex `gpt-6-astra`, against revision 3: five of six round-2 folds
verified, four material findings, all accepted.

| Finding | Disposition | Where folded |
|---|---|---|
| VOA-11-R3 cutover trigger | accepted | 7: session start runs the rearm decision |
| VOA-15 in-process proof cancellation | accepted | 6.2: resident owners keep proof children; R5 |
| VOA-16 retained executable plans | accepted | 7: scan precondition and resume refusal |
| VOA-17 file waits | accepted | 3.1: `work wait --path` |

Round 4, Codex `gpt-6-astra`, against revision 4: two of four round-3 folds
verified, no new findings, two failed folds, both accepted.

| Finding | Disposition | Where folded |
|---|---|---|
| VOA-11-R4 first cutover runs on the old engine | accepted | 7: hook-side bootstrap in U1 |
| VOA-16-R4 no abandonment owner | accepted | 7: owner-controlled argv migration |

Round 5, Codex `gpt-6-astra`, confirmation read of revision 5: VOA-11-R4
verified; one failed fold, accepted; no new findings.

| Finding | Disposition | Where folded |
|---|---|---|
| VOA-16-R5 materialized proof argv | accepted | 7: migration covers unlaunched proof files (withdrawn in revision 7 by rule C1) |

Round 6, Codex `gpt-6-astra`, confirmation read of revision 6: VOA-16-R5
verified; zero material findings. The loop is closed.

Revision 7 read, Codex `gpt-6-astra`, against b5354fcc5: seven material
findings, all accepted.

| Finding | Disposition | Where folded |
|---|---|---|
| VOA-18 Stop worker launches the stub | accepted | 3.3 hook wiring; U9 |
| VOA-19 engine discovery and bootstrap in the generated launcher | accepted | 3.3 hook wiring; U9 activation |
| VOA-20 pre-commit composition | accepted | 3.3 pre-commit wiring |
| VOA-21 scoped publication in the owner | accepted | 3.4 goal sync |
| VOA-22 journal recovery mode | accepted | 3.4 goal sync --recover |
| VOA-23 completion of non-review jobs | accepted | 3.4 work finish |
| VOA-24 exact R11 predicates | accepted | 3.3 S2 |

Revision 7 confirmation, Codex `gpt-6-astra`, against 464fd20d0: seven of seven folds
verified; C1 and the section 7 withdrawal checked; zero material findings. The
loop for revision 7 is closed.

Revision 8 read, Codex `gpt-6-astra`, against 26708ddce: six material findings,
all accepted and folded into 3.5, R13 and unit U6c.

| Finding | Disposition | Where folded |
|---|---|---|
| VOA-25 mission host turns | accepted | 3.5 role `host`; mission runner on the interface |
| VOA-26 permission mapping is runtime-specific | accepted | 3.5 effective envelope from `prepare`; shared comparison |
| VOA-27 preparation, observation, finalization, repair | accepted | 3.5 operation table |
| VOA-28 trust and signature interference | accepted | 3.5 trust boundary; registry cross-check |
| VOA-29 janitor claim shapes | accepted | 3.5 recognizers |
| VOA-30 end-to-end with an unknown runtime | accepted | 3.5 consumers; R13 |

Revision 8 confirmation, Codex `gpt-6-astra`, against 233caaa2b: six of six
folds verified; one new material finding, VOA-31 (an override could drop the
built-in's reserved exclusions its fallback depends on), accepted and folded
into 3.5 ("One effective declaration per runtime name") and R13.

VOA-31 confirmation against 36bffdd8f: verified, zero material findings.
Revision 8 is closed.

U6a read (Fable, 2026-09-27): R5 rose to 124 with genuine process boundaries
now recorded in 3.2 (host turns via `delegate-supervisor ... start-turn`; the
fake runtime's `util hold` child); the fake self-test's `internal delegate`
children are transitional until U6b removes dispatch.sh.
