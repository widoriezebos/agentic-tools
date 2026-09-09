Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal backlog-ordered-by-priority)
Date: 2026-09-09

# Build brief: the conclusion-vs-compaction repair and the Refused frontier category

## The specification, and how to read it

`metasystem/plans/backlog-ordered-by-priority-conclusion-vs-compaction-design.md`,
revision 2, landed at 0260aef6, is the whole contract. Its critique
ladder is closed (registers `metasystem/records/misc/backlog-ordered-by-priority-conclusion-vs-compaction-critique-r1.md`
and `-r2.md`). Where this brief and the page disagree, the page wins. The
parent specification `metasystem/plans/backlog-ordered-by-priority-design.md`
revision 2 is not yours to change.

You build two things the page fully specifies:

1. **Section 1.4**: replace `historyTime` in `metasystem/internal/metrics/compute.go`
   with `archiveVerbs` and `concludedAt` exactly as the page prints them;
   route `concludingEpoch`, `goalBounds`, `selectedGoals` and
   `ConcludedInWindow` (`metasystem/internal/metrics/report.go`) through the
   helper; delete `concludingEpoch`'s own backward scan. After this, no
   function in `internal/metrics` scans history for `done` or `split` except
   `concludedAt`. Report text does not change.
2. **Section 2.3**: `AdmissionRefusal` and the fifth field `Refused` on
   `NextVerdict` in `metasystem/internal/goal/project.go`, populated from the
   discarded else-branch of the gate call the frontier already makes;
   `Refused` copied onto `ClaimableBudgetedWork`; the one non-blocking
   `CLAIM WOULD REFUSE:` line in `enforceIdleBacklog`
   (`metasystem/internal/goal/turnverdict.go`), which must not change the
   digest, `ShouldBlock`, `IdleRefusal` or `IdleBlocks`; the `goal next` none
   line's new first case in `metasystem/cmd/metasystem/goal.go`; the
   `; skipped <name>: <cause head>` clause on the channel `Next for` line in
   `metasystem/internal/channel/report.go`; and one sentence in
   `metasystem/docs/backlog-mechanism.md`. `goal list` is unchanged on
   purpose. `metasystem/AGENTS.md` is not touched.

## Order of work — this is the page's section 3 and it is not optional

1. Write the question 1 canaries first, in this new file: `metasystem/internal/metrics/conclusion_test.go`
   — test `TestConclusionIsTheArchiveAct` with the eight subtests of section
   1.5, records and timestamps exactly as the table gives them. Then run, on
   the UNTOUCHED tree:

   ```sh
   cd metasystem && go test ./internal/metrics -run '^TestConclusionIsTheArchiveAct$/^split-parent$' -count=1 -timeout=2m
   cd metasystem && go test ./internal/metrics -run '^TestConclusionIsTheArchiveAct$/^waiting-row$' -count=1 -timeout=2m
   ```

   Each must FAIL, and fail for the reason its table row names: the `Value`
   line naming `proving_hours=12.000 waiting_share=0.500`. Record the
   observed failure text in your return as evidence at level `ran`. A canary
   that passes before the repair, or fails for any other reason, is a defect
   in the canary: stop, report it, do not touch `compute.go`.
2. Add the three premise pins to `TestPriorityLifecycle` in
   `metasystem/internal/goal/order_test.go` exactly as section 1.5 lists them
   (extend `done-reopen`, extend `split`, add `done-then-split`). Run:

   ```sh
   cd metasystem && go test ./internal/goal -run '^TestPriorityLifecycle$' -count=1 -timeout=2m
   ```

   All must PASS on the untouched tree. A pin that fails there means the page
   misread a writer: stop and report it. Do not adjust a pin to make it pass.
3. Replace `historyTime`; repair the four readers. Run the canaries green,
   then the package: `cd metasystem && go test ./internal/metrics -count=1 -timeout=2m`.
   `TestO10LifecycleEdgesStayIncompleteOrNameEpochs`,
   `TestWaitingZeroDurationLifecycleIsLabelledWithoutJudgment` and the
   attribution tests must not move.
4. Question 2: the frontier field and its readers, then the command and
   channel canaries of section 2.5 red then green (`refused` in
   `TestGoalPrioritySelection`; `refused-head` and `refused-only` in
   `TestReportPriority`), recording the observed before-text (`no matching
   eligible work`; `Next for` lines without the `skipped` clause) as `ran`
   evidence; then `TestNextPriority/over-norm` and `refused-only`; then the
   seat test `TestRefusedBacklogIsReportedWithoutBlocking` in
   `metasystem/internal/goal/turnverdict_idle_test.go`. The unit fixtures for
   question 2 compile only after the field exists; the page says so, and the
   command and channel canaries carry the fail-before proof.
5. The fast gate: `cd metasystem && scripts/agents/go-gate.sh --fast`.

The process-owning beds (`scripts/agents/dispatch-fixtures.sh`,
`scripts/agents/goal-cli-fixtures.sh`) cannot run in your sandbox; the
orchestrator runs them. Do not treat that as evidence about your change, and
do not weaken anything to make them runnable. The page requires
`goal-cli-fixtures.sh:800-813` to keep their exact `none` text; your change to
the none line must not alter the cases those steps exercise.

## Boundary — the page's untouched list is a hard wall

Do not change `metasystem/internal/goal/file.go`, `validate.go`, `verbs.go`,
`split.go`, `reconcilepub.go`, `reconcilemap.go`, `migrate.go`, `order.go`,
`approval.go` or `norm.go`; any record under `plans/goals/` or
`records/goals/`; `metasystem/AGENTS.md`; or the steward's continuation,
revival and attention code. No rank is set, inferred or moved. `goal next`
remains a read. Admission remains one predicate with one more branch
recording its answer; do not add a second traversal. If a step cannot be done
without crossing this wall, stop and say which one.

Your `diffBoundary` must be exactly the section 3 change list plus the new
test file, and nothing else.

## Two standing rules that apply to the code you write

- No round, slice, or finding references in source comments (no `BOC-01`,
  `BCC-01`, `revision 2`). Comments describe the application; the page and
  the registers carry the history. The comment text the page prints for
  `archiveVerbs` and `concludedAt` is fine as printed.
- Keep the page's exact strings. The fixtures assert on them, the shell bed
  pins some of them, and a paraphrase costs a fold.

## Return

Per the implementer schema. Evidence rows at level `ran` for: each canary's
observed fail-before text on the untouched tree; each premise pin green on
the untouched tree; each canary green after; the package runs; the fast
gate. `riskiestPart` names the one thing you are least sure of. Gap rule:
stop and report a gap; never fill it silently. Wall-clock budget: 90
minutes.
