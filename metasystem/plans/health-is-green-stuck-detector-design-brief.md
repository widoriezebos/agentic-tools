# Design brief: health-is-green-when-the-seat-is-healthy, unit 2 (the stuck detector)

## Revision

Revision: 2

Reason: Astra's one critique round (chain design-critic-0416d2dda92071e470e82ccc) returned two material findings on revision 1 (`plans/designs/health-is-green-stuck-detector.md`, attempt 2); the seat accepted both. Revise that page in place: keep everything the findings do not touch, fold each finding, mark the folded passages "(revision 2)", and stay within 1,000 words. Unit 1's page is not touched.

Folds required (finding, then the seat's amendment):

1. STUCK-SUPPRESSION-RESET. A person's `metasystem alert clear` closes the episode and marks the work suppressed (`internal/steward/pattern_episode.go:386-395`); a suppressed work refuses later openings (`internal/steward/pattern_episode.go:211-213`) until a Clear observation releases it (`internal/steward/pattern_episode.go:335-340`). Revision 1 emits Clear only for open episodes, so a dismissed unit never alerts again. Emit the Clear observation for every stuck-unit work that has an open or a suppressed episode (read both through the alert store) whenever its run is readable and not stuck, so a dismissal is released by the first clean reading and a later stall alerts again. Add the red test: open, dismiss, a clean reading, a new stall: a new alert.
2. STUCK-PARALLEL-READ. A round's reads may run in parallel when the plan declares no shared read outputs (`internal/launch/unit_run.go:432-436`, `internal/launch/read_sequence.go:234-240`, `internal/launch/read_sequence.go:268-273`), so the last step with a launch is not always the running one. The reader evaluates every step of the newest round whose launch record is running, applies each one's kind limit, and the standing is the running launch furthest over its limit (the one the stop command names), or none over the limit. Add the red test: two parallel reads, the earlier one running 40 minutes, the later one completed: stuck, naming the earlier launch.

## Context pack

Read this pack first. Open another file only to check one of the cited lines below; do not widen the read beyond that check. Paths are relative to the installation `metasystem/` of `/Users/wido/LocalStorage/GitHub/agentic-tools-m1l` (main at the time of writing: e277d7da8).

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

The goal's words for unit 2 (`metasystem goal show health-is-green-when-the-seat-is-healthy --json` carries the goal whole): "THE STUCK DETECTOR. Today the coordinator runs a script (scratchpad/seat-watch.py) every three minutes: for every running unit run it reads the newest launch's kind and start time and the round count, and raises WRESTLING when a build step runs over 45 minutes, a check over 30, a read over 35, or the round count reaches 3. The steward's health pass does the same for its seat from the same records, raises one alert line naming seat, goal, unit, step, age and the remedy (stop the step, cut the unit, land with notes), closes it when the condition clears (item AK), and the fleet page shows it on the card. Limits are settings with these defaults. Generic: it reads unit runs and launches, not any adopter's code."

The coordinator's script, summarized (it lives outside the repository): for each `~/.metasystem/unit/RUN/run.json` whose `state` is `running`, rounds = the count of `round-*` directories; the current step is the newest `~/.metasystem/launch/RUN-rN-sM/record.json`, its `kind` and `state`, and its age from `startedAt` while `state` is `running`; it prints `WRESTLING SEAT GOAL/UNIT: rounds=N` and/or `STEP running N min`, and finds the seat from the worktree path.

Facts traced on 2026-10-04 (file:line at e277d7da8; verify only what you rely on):

1. Unit runs. `launch.UnitRunRecord` (`internal/launch/unit_run.go:38-60`): Unit, Goal, Worktree, State, `Rounds []UnitRound`, MaxRounds. `UnitRound` (`internal/launch/unit_run.go:62-70`) holds its Steps; `UnitStep` (`internal/launch/unit_run.go:72-88`) holds Name, LaunchID, State, StartedAt, FinishedAt. Run states: `running` and `awaiting-judgement` (set at `internal/launch/unit_run.go:571`); `NamedWork.Running()` (`internal/launch/unit_named.go:466-468`). The round count is `len(record.Rounds)` (`internal/launch/unit_run.go:191`). The store is `~/.metasystem/unit` (`internal/launch/unit_run.go:837-843`), written by `save` (`internal/launch/unit_run.go:845-857`). A step's state in run.json changes only while a command advances the run (`internal/launch/read_sequence.go:39-95`), so the launch record is the reliable source for whether a step still runs. No reader lists every running run today.
2. Launches. `launch.Record` (`internal/launch/record.go:71-100`): Kind, Goal, Tag (the unit), WorkingDirectory, State, StartedAt, FinishedAt, Round. States at `internal/launch/record.go:24-32`; unit runs launch kinds `build`, `proof` and `read` (`internal/launch/unit_run.go:339`, `internal/launch/unit_run.go:384`, `internal/launch/unit_run.go:435`). `Store.Read` and `Store.List` (`internal/launch/record.go:175-241`). A launch id is `RUN-rROUND-sPOSITION` (`internal/launch/read_sequence.go:332-334`); the position is not the kind. A launch record names no seat.
3. The steward's health pass. Roles are declared at `internal/steward/health.go:47-96` and evaluated each tick by `evaluateHealthRolesWithLedger` (`internal/steward/health.go:474-510`), from `completeTickHealthWithDependencies` (`internal/steward/tick.go:539-581`). A role returns `RoleVerdict` (`internal/steward/health.go:110-124`). Any dead role makes the seat "unhealthy" (`internal/steward/health.go:775-776`). A small setting-backed role: `checkStopHookDuration` (`internal/steward/health.go:664-719`). A role sees only the repository root; the seat nickname comes from `goal.ResolveMachine` (`internal/steward/health.go:700`).
4. Alerts. Health alerts are one episode per seat, keyed by all non-alive roles together, and clear only when the whole seat reads healthy (`internal/steward/alert_episode.go:372-492`); every seat has standing reds today (the goal's intent), so a stuck unit raised as a health role would never close by itself. The per-work alert: `OpenAlert` (`internal/steward/pattern_episode.go:166-233`) with `AlertOpening{Owner, Work, Since, Evidence, Message, Now, Deliver}` (`internal/steward/pattern_episode.go:86-95`), deduplicated on Owner and Work; `UpdatePatterns` with `PatternCycle{Now, ClearTicks, Deliver, Step}` (`internal/steward/pattern_episode.go:69-75`, `internal/steward/pattern_episode.go:238-368`) clears an episode after ClearTicks clean observations. Its only caller is the lane steward's pattern pass (`internal/pattern/pass.go:46-111`). Alerts live under `artifacts/agents/steward/alerts/` of the seat's own checkout and reach the browser as notifications (`internal/steward/alert_episode.go:514-516`); `metasystem alert list|acknowledge|clear` (`cmd/metasystem/intent_alert.go:57-131`).
5. Settings. Declared in the `config.Setting` table (`internal/config/defaults.go:15-35`) with a compiled default and a Meaning; example `steward.stop-slow-sec` (`internal/config/defaults.go:149-150`, validated at `internal/config/validate.go:594`, read at `internal/steward/health.go:668`). Tests require a Meaning (`internal/config/defaults_test.go:144-159`) and forbid repeating a compiled default in `metasystem.conf` (`internal/config/defaults_test.go:166-181`).
6. The Fleet page. `/api/fleet` (`internal/ui/httpd/fleet.go:27-53`) carries health for this seat only; alerts are not in it. The board is host-wide (`/api/board`, `internal/ui/httpd/board.go:31`); `stuckSeats` (`internal/ui/web/_app/src/fleet/panel.ts:472-494`) already shows stalled board cards; board cards carry `Round{N, Max}` (`internal/board/card.go:97-100`), written by the unit runner (`internal/launch/unit_run.go:583-599`).
7. What exists for stuck work: the board's stall reason (`internal/board/board.go:16-18`, `internal/board/bridge.go:274-286`) and the unit round limit, a refusal at MaxRounds (`internal/launch/unit_run.go:191-193`). No step-age or round-count alert exists.

## Decisions the seat has taken (build on them; argue only with evidence)

- D1 The owner is the seat's own steward, on its tick, for its own seat's unit runs only. You decide the seat filter and say why it is exact: a run's Worktree under the seat's checkout or one of its goal worktrees, the goals the ledger shows this machine claiming, or both.
- D2 The signal is the coordinator's rule: a running unit run whose current step's launch is running longer than its kind's limit (build 45 minutes, proof 30, read 35), or whose round count reaches 3. The launch record decides whether the step runs and since when (fact 1).
- D3 One alert per stuck unit, through the per-work episode (`OpenAlert` / `UpdatePatterns`, fact 4), so it closes by itself when the unit is no longer stuck; not a health role (fact 4 says why). Its one line names seat, goal, unit, step, age and the remedy (stop the step with `metasystem work stop`, cut the unit, or land with notes), as one plain sentence. You decide ClearTicks.
- D4 The limits are four settings with these compiled defaults; you name the keys.
- D5 The Fleet page shows the condition on the seat for every seat of this host, not only this one. Say where it travels (the board is the host-wide surface; alerts and `/api/fleet` health are per-checkout) and keep cards to identifiers, numbers and times.
- D6 Behaviour tests stub Git (project rule); the unit and launch stores are read from a temporary home through their existing seams.

## What the page must answer

Write the page as a design record (`docs/design/design-obligation-gate.md`, "A design is a record": head with `Kind: design`, a new `Id`, `Status: draft`, `Goals: health-is-green-when-the-seat-is-healthy`) at `plans/designs/health-is-green-stuck-detector.md`, at most 1,000 words, scope first.

1. Scope: what unit 2 changes and what it leaves alone, in five lines.
2. What is true today: confirm the facts you rely on, correct any that are wrong.
3. The design: the reader (which runs, which launch, how the step kind and age are found), the rule, the alert (owner and work keys, message, ClearTicks, what closes it, what happens when the run ends or the goal is released), the settings, the Fleet page path, and the failure path (an unreadable store or record never raises or clears an alert on a guess). For each behaviour: the test that is red without it (file, test name, fixture).
4. Moved effects: in the form `| Effect | From | To | Code |` with existing backticked repository paths (prefixed `metasystem/`), or the line `No owner moves.`
5. Deferred, each with the field or home it builds on.
6. Open questions for Wido, each with your recommended answer; keep them to real choices.

Estimate the build's changed lines honestly (production and test separately; a Fleet page change includes rebuilding the committed bundle), and propose the unit in a table with a `Unit` column first. One unit, under 1,000 changed lines, is expected.

## Tool-call budget

Maximum reader tool calls: 60
