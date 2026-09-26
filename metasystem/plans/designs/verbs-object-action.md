# Object-action verbs, and Go instead of orchestration Bash

- Kind: design
- Id: 01M3FS4JWK13Z7W87G1SAEJZ06
- Status: draft (revision 6)
- Goals: verbs-match-intent
- Supersedes: `verb-cleanup.md` rule "Do not migrate thousands of private
  process-protocol calls" and its retention of the family dispatcher;
  `agent-help.md` output-preservation rules wherever the grammar below changes
  them; `records/kill-shell/kill-shell.md` ruling of 2026-08-12 ("core in Go,
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
| | land | `land G ...`, `land job J` |
| | wait | `wait G`, `wait goal G`, `wait job/run/proof/resume/review ID`, `wait G --for`, and `wait file PATH --until present\|absent` as `work wait --path PATH --until present\|absent` (caller-relative path, same observation and durable resumption, `intent_work.go:193,1219`) |
| | stop | `stop job J`, `stop review REF` |
| | status | `status job J`, `status run RUN`, `status work`, `show review REF` |
| | close | `done job J` |
| | watch | `internal job watch`, `internal run watch` (turn facts) |
| | report | `internal launch report` (retro skill) |
| | check | `internal validate conformance`, `internal validate critique-closed` (critique skills) |
| **test** | run | `test` |
| | plan, list, check, verify, report | `internal test plan/list/check/verify/report` |
| | wait | `wait proof REF` |
| **question** | ask, retry, withdraw | `ask`, `ask --retry`, `ask --withdraw` |
| | answer, show, list, wait | `answer`, `show question`, (new list over the same owner), `wait question` |
| **mission** | start, status, resume, repair | `start/status/resume/repair mission M` |
| **incident** | list, claim, close | `incidents ...` |
| **grant** | add, revoke, list | `grant`, `revoke`, (list over the same owner) |
| **decision** | list, show | `show decisions`, `show record ID` |
| **session** | start, stop, report | `start session`, `stop session`, `internal report stop-status` (the Stop hook's instruction) |
| | handoff, context, verify | `internal context handoff/status/verify` (steward continuation role) |
| **system** | start, stop, restart, status, check, repair | `start/stop/restart/status [checkout]`, `check`, `repair waits`, `internal up --recover-only --if-down` (as `system start --if-down`, the cron line), `internal steward status` |
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
| `delegate-supervisor` (new, U6a) | the dispatch driver (U6b) | persistent owner of one delegate launch through completion and result publication, replacing the detached adapter supervisor (`dispatch.sh:1001`, `adapters/claude.sh:165`) (VOA-04) |

Entries that become function calls, because nothing outside the engine needs a
process for them: `steward revive`, `delegate --revive`, `up --recover-only`
called from landing (`landing_batch_owner.go:137`), `util hold` (becomes a test
helper binary built by the test). `proc setsid` goes when `launch/process.go`
sets `Setsid` directly and its argv recognizers (6.6) move with it. The list may
shrink during the build; adding an entry needs a revision of this page.

`metasystem internal` prints the entries and their launchers. `internal ENTRY`
is accepted as today for launchers that already prefix.

### 3.3 Plumbing Bash that remains, and the Go bootstrap

A script may remain only if it (a) makes no metasystem decision, (b) calls the
installed binary at most by `exec`-ing one entrypoint, and (c) exists because
something outside Go must start it by path.

- `scripts/agents/supervision-hook.sh`: kept at its path because installed
  runtime settings in every checkout name it. Body: locate the binary; `exec`
  `internal hook "$@"`. If the binary is absent, not executable, or refuses the
  entry (an engine older than the stub, VOA-11), it runs the bootstrap build
  (below) once under its fence and retries; if that fails it prints the degraded
  Stop JSON that `stop-degraded-forms.sh` holds today (moved inline) and exits 0.
- `scripts/agents/pre-commit-guard.sh`: same shape, `internal pre-commit`;
  `internal/ledgerfence/fence.go` checks the new stub shape.
- The Go bootstrap (VOA-08, VOA-09): a small program `metasystem/cmd/devgate`,
  run with `go run ./cmd/devgate ACTION` from the tree it builds, so it needs no
  installed engine and cannot recurse into the test scheduler. It imports the
  owners directly: `internal/gaterun` for the fence (live-owner exclusion,
  self-exemption, live/dead/unreadable markers, `gaterun/fence.go:16`,
  `gaterun/gaterun.go:100`), `internal/behaviorsurface` for the stamp selection
  (today `behavior-surface select` at `go-build.sh:65`), `internal/enginebuild`
  for the stamp. Actions: `build` (replaces `go-build.sh`, including `--out` and
  `--trimpath`), `static` (the fast static/build leaf that `fast-static-build`
  runs; replaces `go-gate.sh --fast`), `gate` (the full Go gate; replaces
  `go-gate.sh`). `go-build.sh` becomes a stub for as long as anything outside Go names
  it, then is deleted. The stub resolves its own installation directory as the
  script does today (`go-build.sh:10`) and runs `go -C "$installation" run
  ./cmd/devgate build "$@"`, so a caller in another directory (`adopt.sh:216`,
  which resolves the source root in a subshell, `adopt.sh:94`) still works
  (VOA-08-R2). Proof cases: clean, dirty, engine absent, stale marker, own gate,
  foreign live gate, and the stub invoked from the repository top and from an
  unrelated directory.
- Scripts that do not call the binary stay: `benchmark/attest.sh`, `grade.sh`,
  `compare.sh`, `extract.sh`, case gates, `environment/vms/*`,
  `plans/first-headless-run/*.sh`, `optional-skills/debug-java/scripts/preflight.sh`.

Everything else under `metasystem/scripts/` is deleted after its port, including
`adopt.sh` (becomes the public `system adopt`, run from a binary the adopter
builds with `go run ./cmd/devgate build`). The three benchmark scripts that call
internal verbs (`provision.sh`, `validate-kit.sh`, `run-cohort.sh`) switch to
public commands where one fits and otherwise move to a Go program under
`benchmark/`.

`validate-metasystem.sh` is not ported as a script: its sections become Go tests
or `testing.json` groups; the section selector, `enumerate-suite.sh`,
`witness-gate.sh` and `oldest-bash-gate.sh` go with it under the provider
transition protocol (6.4).

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
- U9 Close. Transitional family fallthrough and the family registry deleted; R1,
  R3, R4 at their final values; `help agent` and docs final pass; a fresh-eyes
  usability run by an agent that has never seen the CLI (discover, plan, build,
  review, land, recover from a refused land) recorded in the goal's verification
  page.

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
- Retained executable plans (VOA-16). A unit run stores its proof command
  verbatim (`intent_work.go:624`) and resume replays it
  (`launch/unit_run.go:155,369`); its reservation digest forbids editing it
  (`launch/unit_named.go:295`). U1 adds a check that scans resumable unit runs
  for a stored argv that invokes the engine with a spelling the landing removes.
  There is no unit-run abandonment owner (`internal/launch` has none), so the
  recovery is an owner-controlled migration (VOA-16-R4):
  - U1 carries the mapping from every removed spelling that has a public
    successor to that successor (3.1's table and 6.5's pairs), as data in the
    command table.
  - A unit-run owner operation migrates a stored argv through that mapping. It
    records a history entry on the run with the original argv, the new argv, the
    original and new reservation digests and the mapping revision, then stores
    the new digest; the digest check (`launch/unit_named.go:295`) accepts a plan
    whose digest equals the one the migration recorded. Rounds and run lineage
    are unchanged.
  - The migration also covers argv already materialized for a proof step that
    has not launched: the runner writes `round-N/proof-NAME.json` once and
    `PlainExec` executes it (`launch/unit_run.go:369`, `launch/plain.go:19`), so
    for every such unlaunched step the owner rewrites that file through the same
    mapping and records it in the same history entry (VOA-16-R5). A step that
    already launched keeps its file and evidence untouched.
  - Resume applies the migration automatically before launching when every
    removed spelling in the stored argv has a successor. A stored argv with a
    removed spelling that has no successor is refused before launching, naming
    the run.
  - The hook bootstrap (above) and the landing precondition run the same scan on
    the checkout's machine: a run with an unmigratable spelling keeps the old
    engine enrolled (no rearm) and blocks the landing, so it can finish on the
    engine that understands it; once it finishes, the next session start rearms.
  - Witness: a suspended run whose proof command is `metasystem test --goal G`
    resumes after U1 as `metasystem test run --goal G` with the migration in its
    history; a run interrupted after its proof file was written but before launch
    executes the migrated argv (asserted on the command actually executed); a run with a removed dead-verb spelling is refused at resume, holds
    the rearm, and rearm proceeds after it completes.
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
| VOA-16-R5 materialized proof argv | accepted | 7: migration covers unlaunched proof files |
