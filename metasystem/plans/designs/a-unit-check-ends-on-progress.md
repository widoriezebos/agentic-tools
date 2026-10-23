# A unit check ends on progress

- Kind: design
- Id: 01M5M9CTKB2FKJP1CKCF72981Z
- Status: accepted
- Goals: a-unit-check-ends-on-progress
- Critique: accepted at the stop by the steward in Wido's name, 2026-10-23: Astra rounds 1-2 found 4 → 2 material; round 2's two folded as build obligations (U1 brief "Round 2 folds")

Tier 2, revision 2 (DRAFT, 2026-10-23): revision 1 plus decision 35 (Q1, Q2) and the Astra round 1 folds (section "Round 1 folds"). Revision 1 is kept beside this page (`…-revision-1.md`). Design lane (Opus 5.5) for Wido's steward. The Id was not handed to the designer; the steward confirms or replaces it. Code read at origin/main d91c6cb8ff in a scratch worktree of the m1e checkout (removed after); the line numbers of the analyses (read at 9c0e864adf) still hold at every anchor used here. Paths are relative to `metasystem/`. Run records read at `~/.metasystem/unit/20261023t070803-0e28b59ef2/`.

Starting material: the goal page (Intent, Next step); `first-machinery-run-20261023/partial-build-1042.md`; Fable's outside read `first-machinery-run-20261023/outside-read-fable-partial-build.md` (fixes table, section 6); the run journal beside them.

Rulings relied on: R-104-m1e (`memory/rulings.md:163`, artificial clocks, injected seams, a test that fails under load is a defect); R-121-m0 (`:180`, smallest thing that works first); R-142-m1e and R-143-m1e (`:201-202`, the machine never overrules a person; the person sees the impact); R-145-m1e (`:204`, all through the machinery, hand fix to unstick a broken machine with an Astra review). Wido's rules: artificial clocks (2026-09-12); a higher load must not make checks red (2026-10-23); not a nuclear reactor, COMMON or RARE (2026-10-22); summary by default; every message human-readable; a question reaches a person only when it is his, with a ready answer.

Line counts below are measured: all three units were sketched, compiled (`go build ./...`, `go vet` of the touched packages) and counted with `git diff --numstat` (additions plus deletions, production Go only). The U1 progress clock also ran in a throwaway test (FIFO-driven `/bin/sh`, injected timers): the reset held, the silent command stopped with outcome `deadline`, 0.12 s.

## Problem

1. **The check's bound is a wall clock.** The frozen unit check runs `proof.cheap` then `proof.audits`, each under `boundedexec.FixedBound(minutes, "proof.deadline")` (`internal/launch/unit_check.go:97`). `boundedexec.Run` waits on `time.After(bound.Limit)` once (`internal/boundedexec/boundedexec.go:88`, select `:121-126`); expiry kills the process group (`:130-132`). The step records outcome `deadline` (`unit_check.go:103-105`). `proof.deadline=15` is committed only (`metasystem.conf:19`; `internal/config/defaults.go:95`).
2. **The check was slow, not stuck.** Round 1 (`round-1/builder-check-2609937135/result.json`): cheap ran 15.0026 min and ended "cheap timed out after 15m0s (raise it with proof.deadline)"; audits passed in 43 s. The output holds one `landing package <unit> <shard> ok <ms>` line per finished unit: 7 testpolicy, 2 proofrun, 18 cmd/metasystem shards, 1 testrun, then `landing planned 18` and the kill. No line says fail. Round 2 stopped after the same 28 of 46 units at 15m0s with nothing else of ours running (Fable, section 1). This selection does not fit 15 wall-clock minutes on this Mac at any load.
3. **The runner already reports each finished unit.** `runNamedTestGroups` prints `landing package %s %d %s %d` as each shard ends (`cmd/metasystem/test.go:126-140`). The lane reads the same line as its record of a finished unit (`internal/landing/plain/scope.go:423-439`) and counts `UnitsDone` from it (`internal/landing/plain/status.go:621-634`). Inside each shard the proof supervisor already ends a real hang by progress, not by clock: dead after 30 minutes with zero CPU, no output and no stopped member (`internal/proofrun/supervisor.go:18`; shards run under `superviseCommand`, `internal/proofrun/test_build.go:1608`; the rule is `plans/proof-groups-detect-hangs-by-progress-not-the-clock-design.md`). The outer wall clock is a second, load-fragile bound over a runner that has its own hang rule.
4. **A deadline stop is called "unclassified".** `judgeBuildResult` sets every PARTIAL or FAILED build to `cause=unclassified` (`internal/launch/unit_run.go:950-952`). By then the builder's check observation is already in the round directory: `driver.save` collects launches (`unit_run.go:772-776`), and `CollectLaunch` copies `artifacts/unit-checks/<run>/<round>/check-*/{observation,result}.json` to `<round>/builder-check-*/` (`cmd/metasystem/intent_work.go:543-563`). `run.json` today: rounds 1 and 2 `build-partial`, cause `unclassified`, stop "stopped unclassified"; the observation says `Outcome: deadline`. `ReadCheckExecutions` reads such observations already (`internal/launch/check_evidence.go:46-77`).
5. **The retry route exists but is never reached.** `holdFailedStep` writes `Handoff: "stopped " + cause` and `Class: step.Name + ": " + step.Reason` (`internal/launch/unit_step_failure.go:100-114`). With round cause `environment` (or `deadline`) the question's act is `work-review-retry` (`cmd/metasystem/intent_unit_questions.go:175`), the continuation is `work review --reason TEXT --by NAME` (`cmd/metasystem/intent_selection.go:465-466`), and `reviewUnit` retries the retained step (`cmd/metasystem/intent_unit_review.go:42-61`, `RetryFailedStep` `unit_step_failure.go:118-179`). Premise corrected: `deadline` is not a valid stop cause (`internal/loopstop/stop.go:21-22`); `roundCause` turns it into `unclassified` (`unit_run.go:1026-1035`). The engine's convention for a launch deadline is cause `environment` with `step.Deadline = true` (`failedStepCause`, `unit_step_failure.go:32-40`; `endStep`, `internal/launch/read_sequence.go:174-179`). D3 follows it.
6. **The check runs twice, the first time inside the builder's turn.** The brief gets "Before returning, run: metasystem test run --unit-run RUN" (`cmd/metasystem/intent_unit_check.go:74`), and the same check is also the round's engine proof step `proof:unit-check` (`intent_unit_check.go:72`; run after a passed build, `unit_run.go:582-626`). In round 1 the check shared the host with the builder's own `go test ./cmd/metasystem` (633 s), `devgate static` (1042 s), coverage and the lane's scoped proof (Fable, section 1).
7. **Only builds wait for host load.** `createAdmitted` returns at once for any kind but `build` (`internal/launch/hostcapacity.go:35-37`); the load gate is `:89-91`. A refusal `LAUNCH_BUILD_CAPACITY` leaves the step to start again on the next continuation (`read_sequence.go:111-113`) and the verb answers "no build was launched" (`intent_work.go:1534-1536`).
8. **No machine retry for a deadline.** The automatic retry covers `environment` once and excludes a deadline (`unit_step_failure.go:89`; `launch.go:906-908`). Every deadline needs a person.
9. **The ask is wrong on this path.** The decision is "a build step failed" for every failure (`intent_unit_questions.go:197`). Its facts read `BuildLines`, `DeclaredLines` and `GapMessage` (`:204`), set only in `freezeBuildOutcome` (`internal/launch/unit_build_outcome.go:40-56`, `:76-91`), which a failed build never reaches (`unit_run.go:541-557` returns first). So "0 changed lines against 0 declared" and an empty message; the real change was 474 lines (Fable, section 2). The command carries `TEXT` and `NAME` placeholders (`intent_selection.go:466`); Telegram thread 1229 showed them.

10. **No launch cap bounds an ordinary check.** `LaunchCap` returns a cap only for the lane's fix unit (`cmd/metasystem/intent_work.go:472-481`); every other launch gets 0, so no `deadline` is set (`internal/launch/launch.go:207-214`) and `waitChild` waits without bound (`:608-612`). The carry check calls `UnitCheck.Run` with no launch at all (`cmd/metasystem/intent_carry_check.go:144`). Decision 35's Q1 says a command that keeps printing is ended by "the existing launch cap": that premise is false (Astra U1-01). D1 brings its own ceiling.
11. **Both commands run after a failure, and observations pile up.** `UnitCheck.Run` runs cheap and audits in turn whatever the first one did (`unit_check.go:80-121`). Builder observations of a round accumulate in the round directory (`intent_work.go:550-562`); a retry keeps the round and gives the step a new launch id (`read_sequence.go:59-68`, `unit_step_failure.go:167-175`). Each observation step records its `Parent`, the launch it ran under (`unit_check.go:89`; round 1: `Parent: 20261023t070803-0e28b59ef2-r1-s1`, the build step's launch id in `run.json`).
12. **The retry tells the person the wrong thing for a build step.** The impact text is "rerun the retained failed step with the same inputs and fresh output, without another build" (`cmd/metasystem/intent_unit_review.go:54`). For a failed build step the retry resets the build step and starts another builder turn (`unit_step_failure.go:167-175`). R-143-m1e: the person understands the impact first (Astra R-143 finding).

### What is not the fix

Raising `proof.deadline` (`metasystem.conf:19`). It is the load-fragile knob the 2026-09-12 rule forbids, and the next selection would need another number. Fable's fix 1 (estimate the selection, go static-only when it cannot fit) is not in this goal: it changes what the check proves.

## Decisions

**D0. The generic rule.** A check outcome this page does not name keeps today's handling: the step stays `unclassified`, the stop goes to a person with the existing route. A rare case gets no code of its own. Every case below is COMMON (handled) or RARE (D0, or an existing bound).

**D1. proof.deadline means minutes without output; a ceiling ends the rest.** Decision 35, Q1: progress is any new output of the check's command, bytes on its stdout or stderr, in any language. Each write of one or more bytes restarts the bound. A command that writes nothing for `proof.deadline` minutes stops, outcome `deadline`. A command that keeps writing stops at a fixed ceiling of 8 × `proof.deadline` after its start (2 h at 15), outcome `deadline`; the ceiling never restarts (Astra U1-01; Problem 10). The factor is a hang fail-safe an order of magnitude above a normal check (R-104 allows such bounds for real-process wiring), not a measure of the check; it is a constant, not a key. Mechanism:
- `boundedexec.Bound` gains `Progress <-chan struct{}` and `Ceiling time.Duration`. In `runContext` the single select (`boundedexec.go:121-126`) becomes a loop: a receive on `Progress` takes a new `deadline(bound.Limit)` and drops the old one; a `Ceiling` above 0 takes one `deadline(bound.Ceiling)` at the start, never renewed. A nil `Progress` and a zero `Ceiling` never fire, so every other caller is unchanged; `TotalDeadline` (`:106-111`) is not involved. The error still wraps `ErrTimedOut` (so `unit_check.go:103` keeps outcome `deadline`) and says which bound ended it: "`<what>` timed out: no output for 15m0s (proof.deadline)" or "… still running after 2h0m0s (proof.deadline)".
- `unit_check.go:86-97`: the output buffer becomes a writer that copies everything (`CheckExit.Output` unchanged) and does a non-blocking send on a one-slot channel for each non-empty write; one writer takes stdout and stderr. The bound gets `Progress` and `Ceiling = 8 × Limit`; the call becomes `boundedexec.RunWithDeadline(process, bound, name, context.After)`. Both production callers (`intent_unit_check.go:127`, `intent_carry_check.go:144`) get both bounds.
- `CheckContext` gains `After func(time.Duration) <-chan time.Time`, nil meaning `time.After` (beside `Now`, `unit_check.go:50-54`). It is the test seam (R-104).
A hang that writes nothing ends at 15 minutes. A hang inside a shard ends by the proof supervisor's dead rule or by D1. A command that writes forever ends at the ceiling (RARE).

**D2. The frozen value is unchanged.** `check.Minutes` stays the committed `proof.deadline` (frozen at `intent_unit_check.go:279`); only its meaning changes, in the engine. No conf change, no new key. The admission text "this admission uses these committed checks, audits and deadline" (`intent_unit_check.go:304`) stays true.

**D3. A check deadline is the step's cause.** One helper `checkCause(step, roundDirectory)` in `check_evidence.go`. It sets `step.Cause = "environment"` and `step.Deadline = true` only when all of these hold; otherwise it changes nothing (D0):
1. `ReadCheckExecutions(roundDirectory)` (`check_evidence.go:46-77`) reports no unreadable observation (Astra U1-03, second part);
2. among the executions whose first step's `Parent` equals `step.LaunchID` (the launch that ran it, `unit_check.go:89`; a retry has a new launch id, `read_sequence.go:59-68`), the newest by parsed start time is taken (Astra U1-03);
3. in that execution at least one step is `deadline` and every other step is `passed` (Astra U1-02: `cheap=failed, audits=deadline` or the reverse stays `unclassified`, so a real failure is never called environment).
It is called:
- in `judgeBuildResult` after the PARTIAL/FAILED line (`unit_run.go:952`). `step.Reason` stays `build-partial`/`build-failed`, so the round outcome and `BuildResultSummary` (`unit_step_failure.go:14-27`) are unchanged;
- at the entry of `attributeProof` (`unit_attribution.go:30-34`), for a failed `unclassified` step, before any comparison: it sets the reason "the frozen check stopped at proof.deadline" and saves. `attributeProof` has two callers, the proof loop (`unit_run.go:616`) and the resumed failure (`unit_run.go:334`), so both classify (Astra U1-05). A deadline starts no base comparison.
`holdFailedStep` sets `Handoff: "stopped deadline"` when the failed step has `Deadline` (`unit_step_failure.go:106-111`). Every reader of the handoff tests the prefix `stopped ` (`intent_unit_stop.go:63`, `intent_unit_review.go:87`, `:96`, `unit_stop.go:343`) and still matches; the round cause stays `environment`, a valid kind, so `work status`, the question and the continuation offer `work review --reason … --by …` (Problem 5). A launch deadline gets the same handoff; that is correct.

**D3a. The retry says what it runs.** The impact text in `reviewUnit` (`intent_unit_review.go:54`) names the failed step's kind: for a check step (kind `attest`) "rerun the retained check with the same inputs and fresh output, without another build"; for a build step "start another builder turn with the same brief and inputs; it finds its earlier change in the worktree". The rest of the text is unchanged (R-143-m1e; Problem 12).

**D4. The check runs after the builder's turn, as the engine's step.** The prepended line (`intent_unit_check.go:74`) becomes "The engine runs the frozen unit check after your turn; do not run it yourself." The lane-fix line "Stage the repair before running its check." (`:68`) stays: the engine's check still reads the staged tree. The engine step `proof:unit-check` (`intent_unit_check.go:72`, `unit_run.go:582-626`) is unchanged. A builder who runs it anyway still leaves records that `CollectLaunch` copies; nothing reads them for a passed build.

**D5. The check waits for host load like a build.** `StartSpec` gains `HostGated bool`; the round's proof steps set it (`unit_run.go:612`). `createAdmitted` gates `build` and `HostGated` launches (`hostcapacity.go:35`); for a check it skips `host.builds` (a check is no build) and keeps the load limit (`:72-91`) and the person's bypass (`:55-57`). A refusal is the existing `LAUNCH_BUILD_CAPACITY`: the step stays starting and starts on the next continuation (`read_sequence.go:111-113`). The verb's summary for a check says "the unit check waits for the host" instead of "no build was launched" (`intent_work.go:1535`). Waiting is not red (Wido 2026-10-23). The comparison proof (`unit_attribution.go:162`) and review proofs (`round_result.go:63`) are not gated: a refusal there would turn load into a failed attribution.

**D6. One machine retry for a check deadline.** In `retryFailedStep` (`unit_step_failure.go:89`): a failed step named `proof:unit-check` with `Deadline` and cause `environment` and fewer than two launches is retried once by the machine; `RetryBy` is `machine` and `RetryReason` "one retry after the check reached proof.deadline". Its relaunch is `HostGated` (D5, from the retained spec), and a `-retry` launch also waits while the lane's proof runs: in the `BeforeModelLaunch` hook (`intent_work.go:523-541`) through `inv.helmLane` and `plain.ReadRunningProof(...).State == "running"` (`internal/landing/plain/status.go:843-846`), refused as `LAUNCH_BUILD_CAPACITY`, which the hook's wrapper passes through (`unit_run.go:781`). A second deadline has two launches: no retry, D3's stop, a person. A build step with `Deadline` is not retried by the machine: that would be a whole builder turn.

**D7. The ask says what happened.** In `syncBuildHolds` (`intent_unit_questions.go:197-204`):
- the decision names the step and its class: "Unit U of goal G stopped after build round N: `<Stop.Class>`";
- the line counts are real: a failed build counts its changed and declared lines with the code `freezeBuildOutcome` uses (`unit_build_outcome.go:40-56`, moved into `countBuildLines`), called in the failed-build branch (`unit_run.go:553-557`). After D4 a check stop happens after `freezeBuildOutcome`, so its counts were already right;
- the builder's message is quoted, not named: the failed branch keeps the builder's last-message path (as `:86-90`), and the fact is "The builder's last words: `<last paragraph, at most 600 bytes>`";
- the command is runnable: `TEXT` becomes "the machinery stopped: `<Stop.Class>`" and `NAME` the human of this checkout's enrolment (`humanauthority.ReadEnrollment`, `internal/humanauthority/authority.go:672`, field `Human` `:64`), through the existing `replaceWord` (`cmd/metasystem/intent_delivery.go:2698-2709`). With no enrolment `NAME` stays (RARE).
The question still goes to the person only after D6's retry failed: by then it is his.

### Cases

| Case | Class | Where |
|---|---|---|
| A big selection, units finish every few minutes (this run) | COMMON | D1 |
| A non-Go check that prints as it goes | COMMON | D1 |
| A person retries a builder deadline | COMMON | D3a |
| The check reaches proof.deadline inside the builder's turn | COMMON | D3 builder role |
| The check reaches proof.deadline as the engine's step | COMMON | D3 attestation role |
| The check overlaps the builder's own suites | COMMON | D4 |
| The host is loaded when the check would start | COMMON | D5 waits |
| First deadline stop | COMMON | D6 machine retry |
| Second deadline stop | COMMON | D3, D7, a person |
| The ask's facts | COMMON | D7 |
| A command that prints nothing until it ends (audits) | RARE | D1: 15 min, as today |
| Stuck while still printing | RARE | D1 ceiling: 8 × proof.deadline |
| A check that legitimately runs past the ceiling | RARE | D3 stop, retry or person |
| One shard runs longer than 15 min with nothing else running | RARE | D1 stops; D6 retries; a person |
| Observation missing or unreadable | RARE | D0: `unclassified` |
| A deadline beside a failed command in one check | RARE | D3.3: `unclassified` |
| An older attempt's deadline, the new attempt failed otherwise | RARE | D3.2: `unclassified` |
| The carry check (no launch) | RARE | D1 ceiling bounds it |
| A check cancelled by a signal (outcome `cancelled`) | RARE | D0 |
| The lane proves when the retry would start | RARE | D6 waits |
| No enrolment for the ready command | RARE | D7 keeps `NAME` |

## Units

Inventory read at d91c6cb8ff. Counts are from the compiled sketch (production Go, additions plus deletions, new code included).

| Owner | file:line | Change | Unit | Lines |
|---|---|---|---|---|
| `Bound`, `runContext` | `internal/boundedexec/boundedexec.go:49-54`, `:121-126`, `:146-150` | Progress and Ceiling, loop, two messages | U1 | 33 |
| `CheckContext`, `UnitCheck.Run`, output writer | `internal/launch/unit_check.go:50-54`, `:67-69`, `:86-99` | seam, ceiling factor, writer, call | U1 | 35 |
| `checkCause` | `internal/launch/check_evidence.go` (new function) | D3 helper with the three guards | U1 | 31 |
| `judgeBuildResult` | `internal/launch/unit_run.go:952` | one call | U1 | 1 |
| `attributeProof` | `internal/launch/unit_attribution.go:30-34` | classify at entry | U1 | 4 |
| `holdFailedStep` | `internal/launch/unit_step_failure.go:106-111` | `stopped deadline` | U1 | 3 |
| `reviewUnit` impact | `cmd/metasystem/intent_unit_review.go:54` | D3a | U1 | 9 |
| brief line | `cmd/metasystem/intent_unit_check.go:74` | D4 | U2 | 2 |
| `StartSpec` | `internal/launch/launch.go:53-71` | `HostGated` (placed last: no realignment) | U2 | 2 |
| `createAdmitted` | `internal/launch/hostcapacity.go:35-37`, `:61-69` | gate a check, skip `host.builds` | U2 | 6 |
| proof spec | `internal/launch/unit_run.go:612` | `HostGated: true` | U2 | 2 |
| refusal text | `cmd/metasystem/intent_work.go:1534-1536` | "the unit check waits for the host" | U2 | 6 |
| `retryFailedStep` | `internal/launch/unit_step_failure.go:89` | D6 | U3 | 9 |
| hook wrapper, failed-build branch | `internal/launch/unit_run.go:781`, `:553-557` | pass-through, count, message | U3 | 6 |
| `countBuildLines`, `builderMessage` | `internal/launch/unit_build_outcome.go:40-56`, `:86-90` | extract and reuse | U3 | 60 |
| lane wait for a retry | `cmd/metasystem/intent_work.go:523-541` | D6 | U3 | 7 |
| `syncBuildHolds`, `lastParagraph` | `cmd/metasystem/intent_unit_questions.go:197-204` | D7 | U3 | 24 |

U1 was sketched again for revision 2 at d91c6cb8ff (compiled, vetted, `git diff --numstat`): 116 lines, 4 under the stop line. A throwaway test of the new clock passed in 0.17 s: twenty writes with the old timer fired after each, no stop; a silent command stopped "no output for 15m0s"; an endless writer stopped "still running after 2h0m0s", outcome `deadline`. If the builder measures over 120, the split is D1 (U1a, about 70: the clock alone unblocks round 4) and D3/D3a (U1b, about 48).

Call sites: `UnitCheck.Run` has two production callers, the unit check (`intent_unit_check.go:127`) and the carry check (`cmd/metasystem/intent_carry_check.go:144`); both get D1, which is right for both. `boundedexec.Run`/`RunWithDeadline` serve 17 packages; none sets `Progress`. `judgeBuildResult` has one caller (`unit_run.go:910`). `retryFailedStep` is reached from `waitStep` (`read_sequence.go:135`, `:163`). `syncBuildHolds`' ask is the only unit-build ask.

| Unit | Decisions | Production lines (measured) | Stop | Usable alone |
|---|---|---|---|---|
| U1 progress clock and stop cause | D0-D3a | 116 (revision 1: 97) | 120 | yes: this run's selection finishes; a deadline says "stopped deadline" and offers the retry |
| U2 check after the builder's turn | D4, D5 | 18 | 60 | yes: one check per round, never beside the builder's suites, never started on a loaded host |
| U3 honest asks and one machine retry | D6, D7 | 106 | 150 | yes: a deadline is retried once; a person gets a true question with a command to paste |

U3's 60 lines in `unit_build_outcome.go` include the sketch's second diff call in `freezeBuildOutcome`; the builder should pass the one diff into `countBuildLines` (hypothesis: about 45). Order of value is U1, U2, U3. U3's retry rule names the engine step, so it has its full effect after U2.

### How round 4 picks up U1

- Round 3 (`round-3`, build launched 11:53 CEST) runs on today's engine. If its check reaches 15 minutes, its stop is written "stopped unclassified" by the engine that judges it; that judgement is not redone.
- The run freezes command text, minutes and environment, not engine bytes. The frozen `PATH` and the proof argv start with `/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/bin` (`round-3/plan.json`, `check.environment` and `proof[0].argv`). Whatever binary sits there runs `metasystem test run --unit-run` and so `UnitCheck.Run`.
- So no fresh run: a fresh round. After U1 is on main, the ui engine is rebuilt from it while the run awaits judgement (not during a check), the pane re-enrols the ui checkout's engine (`system start` from the pane, as at 10:50; the watcher refuses a rebuilt engine otherwise, journal 09:07 row), and Wido starts round 4 as for rounds 2 and 3: `metasystem work revise a-done-goals-branch-is-swept --work landed-proof --after 3 --brief <rerun brief> --reason "rerun the frozen check on the progress-clock engine; no code change" --by Wido`. A person's revise skips the correction budget and round limits (`internal/launch/unit_revise.go:178-195`); the run has used both corrections (`correctionBudget 2`).
- With U1 alone round 4 runs the check twice: in the builder's turn and as `proof:unit-check`. Each ends on its result. The full time of this selection was never seen: hypothesis 25-35 minutes each (28 units took 15 minutes; 18 were left).
- If round 3 is still open when the engine is replaced, the new engine may judge it (hypothesis: the collecting process is a later `work` invocation): then its stop reads "stopped deadline" and `work review run:… --reason … --by Wido` retries the build step instead of a revise.

## Tests

Through the production owners (R-104): `UnitCheck.Run` with a real `/bin/sh` child and an injected `After`; the unit runner through `newUnitFixture` (`internal/launch/unit_test.go:240`) with the judge table of `TestUnitBuildResultJudgement` (`internal/launch/unit_result_test.go:13`) and the proof failure path of `TestUnitContinueFinishesRecordedProofFailure` (`:254`); the cmd path of `TestIntentDeclaredCheckFrozenAcrossBuilderAndProof` (`cmd/metasystem/intent_unit_check_test.go:125`). The child waits on a FIFO the test writes; the test waits on calls of its fake timer; no sleep, no `time.Now`, no wall-clock bound.

U1:
- **T1 (D1, reset without a race).** The cheap command loops: read a line from the FIFO, write one byte, until it reads `end`. Twenty times: the test writes the FIFO, then selects on {a new idle timer, the check's end}. On the new timer it fires the old one and goes on; the check's end before the new timer fails the test (a retained old timer kills the child, the end arrives, no new timer comes). The test writes the FIFO with `O_NONBLOCK`, so a dead reader gives `ENXIO` at once instead of a blocked open. Then `end`: exit 0, outcome `passed`. This is the twenty-minute case in virtual time.
- **T2 (D1).** A silent cheap command: firing its idle timer kills it; outcome `deadline`; the error contains "no output for 15m0s (proof.deadline)".
- **T3 (D1, table).** Output that counts: a write to stdout; a write to stderr; bytes with no newline. Each requests a new idle timer. Then silence: firing the newest timer stops it, outcome `deadline`.
- **T4 (D1, ceiling).** A command that writes forever: the ceiling timer is requested once with 2h0m0s at the start and never again however often it writes; firing it stops the command, outcome `deadline`, error "still running after 2h0m0s (proof.deadline)", `observation.json` retained.
- **T5 (D1).** `boundedexec`: with `Progress` nil and `Ceiling` 0, the existing `TestRunKillsAHangingCommand`, `TestTimeoutErrorNamesItsOwnKey` and `TestTotalDeadlineIncludesProcessReaping` stay green unchanged.
- **T6 (D3, table, judge).** A PARTIAL build whose step launch is L, with, in the round directory: (a) an observation of L, cheap `deadline`, audits `passed`: cause `environment`, `Deadline`, reason `build-partial`, outcome `build-partial`, handoff `stopped deadline`, class `build: build-partial`; (b) of L, an older `deadline` and a newer all-`passed`: `unclassified`; (c) of L, cheap `failed`, audits `deadline`; (d) of L, cheap `deadline`, audits `failed`; (e) only an observation of another launch (the earlier attempt) with `deadline`; (f) a readable `deadline` of L beside an unreadable `check-*` directory. (b) to (f): `unclassified`.
- **T7 (D3, proof step, both callers).** `proof:unit-check` fails with an observation of its own launch, `deadline`: cause `environment`, reason "the frozen check stopped at proof.deadline", no comparison launch, handoff `stopped deadline`. The same through the resumed-failure continuation (`unit_run.go:334`, the path `TestUnitContinueFinishesRecordedProofFailure` drives).
- **T8 (D3, D3a, cmd).** From T6a: the question's act is `work-review-retry`; the retry by a proved person records the impact "start another builder turn …" and launches a step of kind build. From T7: the impact says "rerun the retained check … without another build" and launches a step of kind proof.

U2:
- **T9 (D4).** The checked brief starts with the "engine runs the frozen unit check" line and has no "Before returning, run:". `intent_work_test.go:919` and `intent_unit_check_test.go:53` change with it.
- **T10 (D5, table).** With an injected capacity reading: load at or above `host.load-max`: the proof start is refused `LAUNCH_BUILD_CAPACITY`, the step stays starting, the summary says "the unit check waits for the host"; below: it starts; a `host.builds` cap already full of other goals' builds does not hold a check; `Actor: person` passes at any load; a comparison proof is never refused.

U3:
- **T11 (D6).** A first `proof:unit-check` deadline: one relaunch `…-retry`, `RetryBy` `machine`; its second deadline: no third launch, stop `stopped deadline`, one question. `TestUnitContinueRecordedDeadlineAllowsRetry` (`unit_result_test.go:337`) stays green.
- **T12 (D6).** An injected lane status with a running proof: the retry is refused `LAUNCH_BUILD_CAPACITY`; after the status says idle it starts.
- **T13 (D6).** A build step with `Deadline` is not retried by the machine.
- **T14 (D7).** A PARTIAL build with a 474-line diff and a declared 750: the question says "474 changed lines against 750 declared", quotes the builder's last paragraph, names the class, and its command has no `TEXT` or `NAME` when an enrolment names a human.

Mutations, each failing the named test: drop the reset in the `runContext` loop (T1); keep the old timer selected after a reset (T1); drop the `Progress` assignment in `UnitCheck.Run` (T1); signal only on stdout (T3, stderr row); signal only on a newline (T3, no-newline row); renew the ceiling on output (T4: a second ceiling request); drop the ceiling (T4: no ceiling request); drop the `checkCause` call in `judgeBuildResult` (T6a); take the oldest execution (T6b); drop the "every other step passed" guard (T6c, T6d); drop the parent match (T6e); ignore unreadable evidence (T6f); drop the call in `attributeProof` (T7: a comparison starts); classify in the proof loop instead of `attributeProof` (T7 resumed row); drop the `stopped deadline` handoff (T6a); keep the old impact text (T8 build row); keep the prepended line (T9); leave `HostGated` off the proof spec (T10); apply `host.builds` to a check (T10); allow a second machine retry (T11); drop the lane wait (T12); retry a build deadline (T13); drop `countBuildLines` in the failed branch (T14); keep the placeholders (T14).

## Notes for later goals (not units)

- **Fix 7.** Long `go test` lines in briefs carry `-timeout 30m`, as the frozen audits already do. Go's default 10-minute package timeout killed the builder's cmd run at 618 s (`cmd.log:1568`). The brief checklist names it; the brief generator should too.
- **Fix 8.** Housekeeping: no `capDeadline is missing` alert for a pending job that has `capMin`; narration says "running", not "is starting" every ten minutes; a seat that rebuilds its engine re-enrols or is refused.
- **Fix 1.** A selection that cannot fit on a quiet host should be known before it runs (`cmd/metasystem/test_impact.go:240-252`). Separate goal: it changes what a unit check proves.
- Seats wait through the engine, not in private load loops (journal 09:27 and 10:42 rows). D5 keeps that shape; it does not fix the loop.

## Questions only Wido can answer

Answered by decision 35 (section "Owner decisions" below): Q1 any new output; Q2 yes. Q1's "existing launch cap" premise was false (Problem 10); D1's ceiling replaces it. No open question for Wido.

## Critique record

- Round 1, 2026-10-23, Astra (Codex), log `/Users/wido/agentic-tools-evidence/lane-test-20261009/designs/a-unit-check-ends-on-progress-critique-1.log`: 4 material (U1-01, U1-02, U1-03 RARE; R-143 COMMON), 2 minor (U1-05 RARE, U1-06 COMMON). Folded as D1 ceiling (U1-01), D3.3 (U1-02), D3.1-2 (U1-03), D3a (R-143), D3 at the `attributeProof` entry (U1-05), T1-T4 and the mutation list (U1-06). Q1 (decision 35) folded into D1. U1 re-measured: 97 to 116.

## Round 1 folds

| Finding | Class | Verified at | Fold |
|---|---|---|---|
| U1-01 a writing command is not bounded: no launch cap for ordinary launches | MATERIAL, RARE | `intent_work.go:472-481` (cap only for the lane fix), `launch.go:207-214`, `:608-612`; carry check `intent_carry_check.go:144` | D1: a fixed ceiling, 8 × proof.deadline from the start, never renewed, inside `boundedexec` for both callers; T4. Problem 10 added; decision 35's "existing launch cap" corrected. |
| U1-02 a deadline masks a real failure in the same check | MATERIAL, RARE | `unit_check.go:80-121` (both commands always run); `unit_attribution.go:32` | D3.3: classify only when every other step passed; T6c, T6d. |
| U1-03 an older attempt's or unreadable evidence classifies the current failure | MATERIAL, RARE | `intent_work.go:550-562`; `read_sequence.go:59-68`; `unit_step_failure.go:167-175`; `check_evidence.go:64-76`; `Parent` at `unit_check.go:89` | D3.1-2: match the step's launch id; any unreadable observation leaves it unclassified; T6e, T6f. Problem 11 added. |
| R-143 the build-step retry says "without another build" | MATERIAL, COMMON | `intent_unit_review.go:54`; `unit_step_failure.go:167-175`; `memory/rulings.md:202` | D3a: the impact names what runs; T8. Problem 12 added. |
| U1-05 the resumed failure skips the classification | MINOR, RARE | `unit_run.go:334` calls `attributeProof` directly | D3: classify at the `attributeProof` entry; T7 resumed row. |
| U1-06 tests and mutations describe the line parser; T1 races | MINOR, COMMON | design revision 1 T3, T4, mutations; sketch test | T1 selects on the check's end before each write, `O_NONBLOCK` FIFO; T3 stdout/stderr/no-newline; T4 ceiling; mutation list rewritten. |

Round 2: needed only for U1's new mechanism, the ceiling, and the three guards of D3; the folds introduce no new seam. The steward decides under the stop rule.

## Owner decisions (steward in Wido's name, decision 35, 2026-10-23)

- Q1: progress is any new output from the check's command (bytes on its stdout or stderr), language-generic; the metasystem runner's `landing package` line is not required. A command that prints but never finishes is RARE and is bounded by the existing launch cap (D0 covers it: stop, keep the record, one line). The brief's U1 counts output instead of the unit line.
- Q2: yes; the steward rebuilds and re-enrols the ui engine and runs round 4's revise from the pane in Wido's name, recorded as CARRIED in the run journal.
