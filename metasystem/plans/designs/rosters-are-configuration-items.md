# Design for rosters-are-configuration-items

- Kind: design
- Id: 01M3ZZVADN03HRKBNN0Q52T9Y5
- Status: draft
- Goals: rosters-are-configuration-items

Revision 5, 2026-10-03. Author: Fable (`claude-fable-5-1`), seat m1i. Code read at main `b4a3bb4c3`; paths are relative to `metasystem/`. Revisions 1 to 4 live only on branch `ui-development` (commit 9faaf905a); none was accepted, so on main this is the goal's first design record. Revision 4 kept rosters per checkout, answered a missing roster from today's keys, and left the seat agent and the steward's continuation outside. Wido's requirements of 2026-10-02 contradict all three.

Wido, 2026-10-02, verbatim: "the running agent; is that defined in the roster which agent and which model? That should be the case". Taken as given: the roster covers every agent kind, is one key family shared by every checkout on the host, must "refuse an unknown key instead of silently falling back", has `roster show` and the Settings view, and includes effort. Standing instruction: "build the smallest thing possible that works".

## 1. What happens today

Checked at the cited lines.

- **Two key families, two readers.** A dispatched job reads `role.ROLE.runtime` and `role.ROLE.model.RUNTIME`, each falling to `role.default` (`internal/dispatch/roster.go:145-229`). A launch reads `launch.KIND.runtime`, `.model` and `.model.RUNTIME` (`internal/launch/settings.go:157-229`). They can name different agents for the same work.
- **Borrowed and silent values.** The seat and landing agents run the build lane's model (`internal/launch/settings.go:142-155`); critique, design and read run its effort (`:333-352`). Warden, behavior-judge and steward-continuation run on `role.default.*`. Every key has a compiled default (`internal/config/defaults.go:259-397`).
- **Effort has two owners and two holes.** A launch takes `launch.build.effort`. A dispatched job takes the hazard table's value (`internal/dispatch/hazard.go:37-74`), which the record must equal (`internal/dispatch/build.go:466-467`). Claude's dispatch command carries no effort (`internal/adapter/claude.go:419-445`). Devin's dispatch takes the model id unchanged; Devin's launch appends the effort (`internal/launch/devin.go:38-46`).
- **Per checkout.** Precedence ends in each checkout's own files (`internal/config/resolve.go:295-302`). The Partner is a third family, read once at interface start (`internal/config/ui.go:223-249`, `cmd/metasystem/ui.go:263-284`).
- **No unknown-key check.** Validation refuses key shape and duplicates only (`internal/config/validate.go:57-75`, `:437-451`).
- **Models chosen before the reader runs:** `unitReadModel` (`cmd/metasystem/intent_work.go:953-973`), the unit runner (`internal/launch/unit_run.go:294`, `:316`), the standalone read (`internal/launch/standalone_read.go:187`, `:481-482`, `:696`), the critic's export into another installation's dispatch (`cmd/metasystem/goal_branch.go:644-649`) and the steward's staged pair (`cmd/metasystem/goal.go:733-748`).
- **Host state** already lives in the registry home's `host/` directory, under a lock and atomic writes (`internal/board/card.go:158-176`, `internal/landing/lane/lane.go:75-82`, `:113-127`, `:182-189`).

Where the brief's facts were off:

1. `settings set` needs the person's proof for `metasystem.runtimes` (`cmd/metasystem/intent_work.go:2104-2108`, `:2227`); every roster key is still written by any caller, an agent included.
2. The launch-lane check is `internal/config/validate.go:556-577`; `:511-552` checks roles.
3. `internal/protocol/roles/` also holds `orchestrator.md`, which is not dispatchable (`internal/protocol/protocol.go:91-97`).
4. `EffectiveObligations` has six callers at main: `internal/dispatch/build.go:459`, `:823`, `:972`, `internal/dispatch/hazard.go:80`, `:241`, `internal/dispatch/claim.go:253`.
5. No benchmark configuration that sets today's keys was found in this checkout (section 12).

## 2. The roster item

One file per computer holds every roster. A roster has an identifier, a type and rows. A row is one kind of agent work with a runtime, a model and an effort, written `RUNTIME:MODEL:EFFORT`. The row names are the one vocabulary; one table in the engine maps today's dispatch roles and launch kinds to them:

- **Rosters `tier-1`, `tier-2`, `tier-3` (type tier):** `design` (the implementer in mode design; the design launch), `design-critique` (the design-critic; the critique launch), `build` (the implementer in any other mode; the build launch), `code-critique` (the code-critic; the read launch), `verify` (the verifier), `investigate` (the investigator), `warden`, `behavior-judge`.
- **Roster `seat` (type seat):** `seat` (the seat launch) and `steward` (the steward-continuation role).
- **Roster `landing` (type landing):** `landing` (the landing launch).
- **Roster `partner` (type partner):** `partner` (the Project Partner).

The `seat` type is one Wido did not name (question 1). The launch kind `proof` runs no agent and has no row; neither has a session a person starts by hand. A test walks the embedded role files and the launch kinds and fails when one has no row.

**Rules of a row.**

- All three fields are required. Nothing is borrowed, and `auto` is gone: the file is this computer's, so it names this computer's agent.
- Effort is `low`, `medium`, `high`, `xhigh` or `max`.
- `main` in place of the three fields means the calling session does the work and no agent is started. It is allowed only in the four rows no launch reads (`verify`, `investigate`, `warden`, `behavior-judge`); a dispatch of such a row is refused in today's words (`internal/dispatch/roster.go:164-166`).
- The `partner` row is `RUNTIME:MODEL`: the Partner's session takes a model and no effort (`internal/ui/partner/host.go:537-556`), so a third field is refused instead of shown.
- A template placeholder as model is refused at write and at read (R-106-m1e).
- Within a tier roster, `build` and `code-critique` name different models, and so do `design` and `design-critique` (R-133-ui). Only the write path checks this; a dispatch's `--model` override is not checked.

**Effort has one owner: the row.** The hazard table stops supplying a value and becomes a floor. `EffectiveObligations` takes the row's effort as a third argument, answers it as the job's effort, and refuses a row below what the job's class requires: "roster tier-1 gives build effort medium, and a design-bearing job needs xhigh; a person raises it with metasystem roster set tier-1 build RUNTIME:MODEL:xhigh; nothing was dispatched". The record check (`internal/dispatch/build.go:466-467`) then compares the record with the row. The table's other rules stand. A mechanical build runs at what its row says, never at the table's medium, which closes R-90-m1 without sweeping the table.

**What carries each value into the running agent.** A launch: `--model` and `--effort` on Claude (`internal/launch/claude.go:41`, `:45`), `-m` and `model_reasoning_effort` on Codex (`internal/launch/codex.go:88`), the model id with the effort as its suffix on Devin (`internal/launch/devin.go:38-46`). A dispatched job: the record's model on every runtime (`--model` on Claude, `internal/adapter/claude.go:420`); the record's effort through `model_reasoning_effort` on Codex (`internal/adapter/codex.go:78-79`) and, new, through `--effort` on Claude and the launch's suffix rule on Devin. The Partner: the session's model option, and no effort.

## 3. How work finds its row

- **Goal work** reads `tier-N`, N being the goal's risk tier. The dispatcher moves its goal binding read (`internal/delegation/dispatch_phase.go:333-338`) above the roster read (`:245-253`) and passes the tier. The launch manager gains one port, `Tier func(goal string) uint8`, filled from the ledger the work verbs read.
- **No goal, or an unreadable tier,** reads `tier-3`: the engine's precedent (`internal/dispatch/admission.go:408-409`) and the careful direction. It is a selection rule, not a fallback: a missing `tier-3` row is refused like any other.
- **The warden** reads the tier roster of the job it reviews. **The seat agent** reads `seat` in roster `seat`; **the steward's continuation** reads `steward` there, whatever goal it continues. **The landing agent** reads `landing`; **the Partner** reads `partner`.

One function answers every reader, `config.RosterRow(home, roster, row)`. A refusal names the roster, the row and the one command, and nothing else is tried: no other roster, no old key, no built-in default.

- Missing roster or row: "No agent is set for build in roster tier-2 on this computer. A person sets it with: metasystem roster set tier-2 build RUNTIME:MODEL:EFFORT. Nothing was started."
- A runtime this installation does not list: "Roster tier-2 runs build on gemini, and this installation runs claude and codex", then the same command and ending.
- An unreadable file, or a roster or row the engine does not know: "The rosters file ~/.metasystem/host/rosters.json can't be used: REASON. metasystem roster list shows what is wrong. Nothing was started."

**Every caller, traced to the adapter.**

- `Manager.Start` (`internal/launch/launch.go:105-131`) takes runtime, model and effort from the row for the spec's kind and goal. `work build --model` and `--effort` (`cmd/metasystem/intent_work.go:781`) stay explicit, recorded overrides.
- `unitReadModel`, the unit runner's two choices, and the standalone read's frozen model and re-verification ask the function `Start` asks, so nothing is chosen ahead of the roster (RO-02).
- `ResolveRoster` reads the row in place of `internal/dispatch/roster.go:148-229`. Aliases and the escalation ladder run on the row's pair. A `--runtime` override now needs `--model`: a row names one runtime.
- `criticDelegateEnvironment` stops exporting role keys; both installations read the same file.
- The steward's callers (`cmd/metasystem/goal.go:735`, `cmd/metasystem/steward_verbs.go:207`, `internal/steward/handoff_capture.go:1096`) keep calling `ResolveRoster`, which answers the `steward` row.
- `config.UIPartner` reads the `partner` row; `ui.partner.command.RUNTIME` stays. No row means no Partner; its routes answer with the refusal line, as an unadmitted Partner's do today.

**Revision 4's settled points.** RULING-RO-01 and RO-02 are kept, above. RO-03 is moot for the roster's place: the landing agent and every seat read one file. What remains is that the `landing` row's runtime is checked against the lane installation's list; `ResolveSettings` gains that list, which the landing launcher already resolves for the lane (`cmd/metasystem/landing_agent.go:114-141`), so that file does not change. RO-04 is kept: a Partner change takes effect on `metasystem ui restart`. The two-row display (RO-05, RO-06) disappears: one row feeds both readers and effort is never optional. Only the import preview shows two values (section 7).

## 4. Where rosters live

- **Place:** `~/.metasystem/host/rosters.json`, in the registry home's `host/` directory beside the landing lane's record. Readers take the home as a parameter, as `lane.Read` does.
- **Form:** JSON: a version and, per roster, its type and rows of three fields.
- **Writer:** only the engine, through the verbs and the Settings page. A write reads, changes and replaces the file under an exclusive lock on `host/rosters.lock`, then renames atomically, as the lane record does. Two checkouts changing rows at once both keep their change. Readers take no lock.
- **Tests** point `METASYSTEM_SUPERVISION_REGISTRY_HOME` at a temporary home, or pass one to the reader.
- **A second computer** has its own file; nothing is copied.
- **Several projects on one computer** share the file, as they share the landing lane. A row whose runtime a project does not list is refused in that project.
- **A brand-new computer** has no file and nothing is seeded: each kind's first start is refused with the command that sets its row (question 6).
- **Proof digest:** the retired keys were proof inputs. A roster is machine-specific, which by the table's own rule (`internal/config/defaults.go:22-26`) is not one (question 5).

## 5. The verbs

A new object `roster` in the Run group and the administration objects (`cmd/metasystem/intent.go:1313-1349`). Every verb takes `--json`.

- `metasystem roster list`: every roster and its rows; an unset row reads "not set".
- `metasystem roster show ROSTER`: one roster, a row as "build  claude  claude-opus-5-5  xhigh". The `partner` row adds, while it differs: "running now: claude claude-opus-5-5, until metasystem ui restart".
- `metasystem roster set ROSTER ROW RUNTIME:MODEL:EFFORT`: writes one row, creating the roster on its first. Success: "tier-1 build is now codex gpt-6-sol xhigh on this computer; the next build of a tier-1 goal uses it." A repeat: "tier-1 build already is codex gpt-6-sol xhigh; nothing was changed." For `partner`: "it takes effect when the interface restarts", next step `metasystem ui restart`.
- `metasystem roster import`: section 7.

Refusals of `set`, each ending "nothing was changed":

- "tier-9 is not a roster; the rosters are tier-1, tier-2, tier-3, seat, landing and partner"
- "tester is not a row of tier-1; its rows are design, design-critique, build, code-critique, verify, investigate, warden and behavior-judge"
- "a row is RUNTIME:MODEL:EFFORT, and effort is low, medium, high, xhigh or max"
- "gemini is not one of this installation's runtimes (claude, codex)"
- "the Partner takes no effort; write RUNTIME:MODEL"
- "tier-3 build and code-critique would both run claude-opus-5-5; the author and the reviewer of a piece of work are different models"
- "main is only for verify, investigate, warden and behavior-judge"

**Who may change a roster.** Changing a roster is a human ruling. `roster set` and `roster import --write` require the person's own proof at the enrolled terminal, `directPersonProof` (`cmd/metasystem/intent_work.go:2229-2263`), never the helm or a grant: the rule `metasystem.runtimes` has today. An agent is refused in that function's words. The other verbs are reads for anyone.

## 6. The Settings page

A Rosters card between the landing gate and Appearance (`internal/ui/web/_app/src/panes/Settings.tsx:29-40`). The empty state stops promising a page for runtimes and models (`internal/ui/web/_app/src/panes/empties.ts:47-55`).

- **What it shows.** Six groups under plain names: Tier 1 work, Tier 2 work, Tier 3 work (and work with no goal), Seat, Landing lane, Project Partner. A row is a stacked fact, as on the landing gate card: the name ("Build", "Code critique", "Seat agent") above the value ("Claude · claude-opus-5-5 · effort xhigh"). Nothing sits side by side, so it fits 390 pixels. Identifiers (`tier-3`, `code-critique`) stand behind Details, as keys do today (`:151-154`).
- **Changing a row.** Change opens a sheet with three controls: the agent (this installation's runtimes), the model (text), the effort (the five). Save posts to `POST /api/settings/rosters/ROSTER/ROW`, which calls the store function `roster set` calls, under the acts' policy and human proof (`internal/ui/httpd/acts.go:50-57`). A refusal shows in the sheet as the engine wrote it.
- **Missing row:** "Not set. Work of this kind is refused until a person sets it.", with Set.
- **Unlisted runtime:** the row, then "This workspace does not run gemini, so this work is refused here."
- **Broken file:** one line in place of the groups: "The rosters on this computer can't be read: REASON. Nothing can start until this is repaired."
- **The Partner before the restart:** "Set to: Codex · gpt-6-sol" and "Running now: Claude · claude-opus-5-5, until the interface restarts", from `partner.Service.Runtime()` (`internal/ui/partner/service.go:573`).

The card reads `Workspace.Rosters`, filled beside the landing gate's facts (`internal/ui/httpd/httpd.go:634`) through a port beside `LandingGate` (`:204`). The browser's proof is admitted today for goal acts only (R-125-m1u, R-128-ui), so Save waits on question 2; the read card does not.

## 7. The path from today's keys

**Retired keys**, replaced by the rosters:

- the whole `role.` prefix: every `role.ROLE.runtime` and `role.ROLE.model.RUNTIME`, `role.default` included;
- every `mode.MODE.role.ROLE.runtime` and `.model.RUNTIME`;
- `launch.build`, `launch.critique`, `launch.design` and `launch.read`: `.runtime`, `.model`, `.model.RUNTIME`; and `launch.build.effort`;
- `launch.seat` and `launch.landing`: `.runtime`, `.model`, `.effort`;
- `ui.partner.runtime` and `ui.partner.model`.

**One command moves a checkout.** `metasystem roster import`, run in a checkout, calls today's resolvers for every row and prints what the checkout resolves today beside what the computer's roster holds: "new", "same", "differs (the roster keeps its value)", or "the two readers disagree: dispatched jobs use A, launches use B; choose with metasystem roster set". It writes nothing. `--write` writes the "new" rows only and never overwrites, so the six checkouts on this host are imported one after another and their differences are shown, not settled. Today's keys carry no tier, so the values go to all three tier rosters. A row with no effort key today is proposed at `xhigh` and marked (question 7). `--write` also removes from `metasystem.conf.local` each line no reader reads any more and whose value the roster holds (a removal `SetConfKeys` lacks today, `internal/validate/confset.go:21`); a line in a committed `metasystem.conf` is listed for a person to remove.

**No launch reads both.** Each reader moves in one unit, from keys only to the roster only. From then on `settings show` answers its keys with "no longer read; see metasystem roster show ROSTER" and `settings set` refuses them. The four rows with two readers move both in one unit (unit 11).

**Afterwards.** The last unit deletes the keys' compiled rows and refuses a retired key found in either file or the environment: `settings check` reports it, and every launch and dispatch in that checkout is refused: "launch.build.model is retired; build is a roster row now: metasystem roster show tier-3. Remove the line from metasystem.conf.local." The whole `role.` prefix is retired, so a misspelt role key is refused too. The unit lands only after `roster import` reports nothing left on each of the six checkouts; the importer's key reading is deleted with the keys.

**The seat's switch.** `launch.seat.runtime` is also the per-checkout switch for steward-started seats (`cmd/metasystem/steward_seat.go:61-63`). From unit 4 it is read as off or not off only; its final form is question 3.

**Tests.** 64 test files set or assert these keys: 19 in `cmd/metasystem`, 8 each in `internal/delegation` and `internal/config`, the rest over eleven packages. A helper writes rows into the test's own temporary home or hands the launch manager an in-memory port: one instance per test, no global state. Each unit moves its reader's tests. Tests that lean on compiled defaults would be refused once their reader moves, so fixtures get a roster ahead, in a test-only unit (unit 9).

**Benchmarks.** A benchmark runs under its own run-scoped home and writes its cohort as that home's roster file, which the reader checks on every read. A historical configuration stays a record.

**Other users of the keys.** Adoption tailoring stops rewriting them (`internal/validate/conftailor.go:126-268`). The runtime self-test (`internal/adapter/supervisor/codex.go:193-200`, `:237`, `internal/adapter/supervisor/external.go:311`) takes its model from the first row naming the runtime under test, says which, and is refused when none does.

## 8. Step 1 and the units

**Step 1 is units 1 to 4.** After it, `metasystem roster show seat` in any checkout tells Wido which agent, model and effort the seat agent runs, one `roster set` changes it for all five seat checkouts, and the seat and landing agents start from their rows with no fallback. Before unit 4 lands, a person sets the two rows and `roster show` prints them; if the lane is refused anyway, recovery is one `roster set landing landing` from any terminal.

Each unit is at most 300 changed lines, in landing order.

| # | Unit | Witness test | Mutation that turns it red |
|---|---|---|---|
| 1 | The store, `internal/config/roster.go`: kind table, read, locked write, row rules | `TestRosterRowRefusesAMissingRowNamingTheCommand`; `TestSetRosterRowKeepsARowAnotherWriterAdded`; `TestEveryRoleAndLaunchKindHasARow` | answer an empty row; read before the lock; drop `warden` from the table |
| 2 | `roster list`, `roster show` | `TestRosterShowPrintsRowsAndNotSet` | print a built-in value for a missing row |
| 3 | `roster set`, a person's act | `TestRosterSetNeedsThePersonAndWritesOneRow` | remove the proof call |
| 4 | The seat and landing agents read their rows | `TestSeatAndLandingStartFromTheirRowsOnly` (old keys say otherwise; no row: refused) | read `launch.seat.model` when the row is missing |
| 5 | `roster import`, preview and `--write` | `TestRosterImportShowsAndNeverOverwrites` | overwrite a differing row |
| 6 | A dispatched job runs at its record's effort: Claude's `--effort`, Devin's suffix | `TestClaudeDispatchCarriesTheRecordEffort`; `TestDevinDispatchIdCarriesTheEffort` | drop the flag; pass the id unchanged |
| 7 | `EffectiveObligations` takes a job effort and checks the floor; an empty effort keeps the table's value until unit 10 | `TestRowEffortBelowTheClassFloorIsRefused` | skip the floor |
| 8 | One function answers a launch's agent; `Start`, the unit runner, `unitReadModel` and the standalone read ask it. No behaviour change | `TestWorkBuildReadAsksWhatStartAsks` | let `unitReadModel` read the key itself |
| 9 | Test fixtures carry a standard roster. Test-only | `TestStandardRosterFixtureHasEveryRow` | drop a row from the fixture |
| 10 | The dispatch-only rows in force (`verify`, `investigate`, `warden`, `behavior-judge`, `steward`); effort always from the row | `TestDispatchOnlyRolesReadTheirRows` | fall to `role.default.model` |
| 11 | The four shared rows in force for both readers; the tier reaches both; `--runtime` needs `--model`; the critic's export goes | `TestTierRowDecidesLaunchAndDispatch` (tier-1 goal: tier-1's row; no goal: tier-3's; through `work build`, `work review` and a dispatch) | read `launch.read.model`; always pass tier 0 |
| 12 | The Partner reads its row; set and running values | `TestPartnerReadsItsRowAndShowsRunningUntilRestart` | read `ui.partner.model` |
| 13 | Settings page: the Rosters card, read only | `TestWorkspaceCarriesRosters`; the Settings pane's test of "Not set", the broken file and the Partner's two values | omit the missing-row line |
| 14 | Settings page: Change (waits on question 2) | `TestRosterActWritesThroughTheStoreAndNeedsAHuman` | skip the proof |
| 15 | Retirement: compiled rows deleted; a retired key refused; tailoring and self-test moved (waits on question 3) | `TestARetiredKeyIsRefusedNamingItsRow` | ignore a retired key in the local file |

Verb units keep the help audits green (`cmd/metasystem/intent_coverage_test.go:58`, `cmd/metasystem/intent_help_test.go:83`, `:142`); units 13 and 14 rebuild the committed bundle.

## 9. Later, when it hurts

`roster copy`, or one row set in several rosters at once. Named rosters bound to tiers; rosters per project, or shared across the fleet. An unknown-key check for the rest of `metasystem.conf`. The Partner re-admitted without a restart, and an effort for it. The author-and-reviewer rule checked on overrides. An importer that outlives the keys.

## 10. Not in it

Which models the rosters hold (R-133-ui stands). Prices and spend keys. The adapters' own flags and `ui.partner.command.RUNTIME`. Aliases and `model.tier.N`. The hazard table's rules other than its effort. Sharing rosters between computers. Every other `launch.` key.

## 11. Open questions for Wido

1. **A fourth type, `seat`.** Recommendation: yes. The seat agent and the steward's continuation serve no single goal, so they fit none of your three types; the alternative is rows only `tier-3` would ever read.
2. **May a signed-in browser session change a roster?** Recommendation: yes, for `roster set` alone: you named the Settings page as a place to change rosters, and today's rule admits the session's proof for goal acts only. Only unit 14 waits.
3. **The seat's per-checkout switch.** Recommendation: keep one that names no agent (`launch.seat.start`, on or off, default off), because today each checkout turns steward-started seats on by itself. The alternative: a `seat` row turns them on in every checkout except the lane. For the `partner` row the recommendation is no switch: one Partner setting for the computer. Unit 15 waits.
4. **Fixed identifiers** (`tier-1` to `partner`) rather than names you give. Recommendation: fixed; "create" is the first `roster set` on a roster.
5. **Rosters outside the proof digest.** Recommendation: yes; who works does not change what a proof proves. The digest changes once, at unit 15.
6. **A brand-new computer gets no starter roster.** Recommendation: nothing seeded (R-106-m1e and your "refuse"). Say so if a one-command starter set is wanted.
7. **Rows with no effort today are imported at `xhigh`.** Recommendation: yes (R-90-m1); the preview marks them, and you can set another before `--write`.

Questions 1 and 4 shape unit 1's table; a different answer moves table rows, not units.

## 12. What the budget did not allow to check

- The benchmark configurations: no conf, json, toml, yaml, env or sh file in this checkout sets today's keys, so where they live is unread.
- How the steward's staged pair reaches its dispatch, and whether one staged before a roster change is refused or run.
- Devin's dispatch adapter, beyond the absence of effort handling in `internal/adapter/devin.go`.
- The landing agent's Claude-only refusal, which revision 4 cited.
- The verb that runs the runtime self-test.
- How many tests lean on compiled defaults; unit 9 measures it.
- Where the critique lane and the seat are started.

## Threat model and rabbit-hole risks

Our own agents and operators make mistakes and crash; nobody attacks. The lock and the atomic write cover two honest writers and a crash mid-write; the person's proof covers an agent that changes a roster by mistake.

What could make this goal grow without end, and where the page cuts it:

- **The test migration.** 64 files are known; tests leaning on compiled defaults are not counted. Cut: one helper, fixtures prepared ahead, each reader's tests move with it. A unit past 300 lines splits by package; it does not grow.
- **Adopted projects.** Cut: adoption stops writing the keys. A project that jumps past unit 15 with old keys gets one refusal per key naming its row and sets rows by hand; no migration tool for other people's checkouts.
- **The fleet.** Cut: each computer has its own file; nothing syncs.
- **The lane stopping itself.** Unit 4 moves the landing agent, which lands the units. Cut: its row is set and shown first, and recovery is one command from any terminal.
- **A second vocabulary creeping back.** Cut: one kind table, one reader function, and the test that every role and launch kind has a row.
- **Per-project or per-checkout rosters.** Cut: one file per computer; the only per-checkout value left is the seat's switch (question 3).
