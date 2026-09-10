# proof-groups-detect-hangs-by-progress-not-the-clock, design critique round two: dispositions

Design revision 2 (landed b0c9ccb37) was critiqued by
phd-design-crit2c-20260910 (codex gpt-5.6-sol): nine material findings,
carried here from the job's return record (its sandbox could not write
the declared output). Revision 3 folds each as a decision and, under
the implementation-first ruling, the build proceeds behind fixtures.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| PHD-ZERO-SCHEDULER-STATE | accepted | Exactly zero CPU over thirty minutes also describes a stopped tree (SIGSTOP), an uninterruptible wait under swap or storage pressure, or a frozen quota; revision 2 would kill those. | Revision 3 reads task state: stopped and uninterruptible members make the tree `stopped` or `waiting on the host`, named on the record and left alone; quota freezing is residual on this fleet. |
| PHD-CPU-BUDGET-NONINVARIANT | accepted | CPU seconds vary with cache and bandwidth contention, spin waits and GC paths; equal consumption across loads cannot be asserted. | The budget is a ceiling at eight times measured consumption with the inflation assumption named as residual and checked by the records; row 4 asserts the same order of magnitude. |
| PHD-COUNTER-UNIT-RACE | accepted | Ticks, formatted seconds, vanished members and reader failures were unspecified. | Revision 3 names the units, the one-second rise, the partial-sample rule and the three-outright-failures rule. |
| PHD-SEED-MULTICORE | accepted | Four times a wall target as CPU seconds is exhausted by a healthy parallel group on an eighteen-processor host. | No wall-derived seed: the budget is optional, absent groups run under the dead rule only, and the fleet's measurements set budgets later. |
| PHD-DUMP-NONTERMINATING | accepted | A process that ignores SIGQUIT and keeps computing never reaches zero consumption; a dumping process blocked on a full pipe reaches zero and is killed with a partial dump. | The tee drains continuously; a tree that keeps consuming after SIGQUIT is killed at once with the reason recorded; the zero rule is the only fallback for a quiet one. |
| PHD-FIXTURE-WAIT-SEMANTICS | accepted | A generic consumption rule cannot tell a slow producer from one that will never satisfy a fixture's waited-for state. | Slice 3 leaves the goal; fixture waits become owned-producer contracts under goal fixture-waits-name-their-producer; this goal's DONE is narrowed to the receipt's groups and attempt. |
| PHD-CLOCK-INVENTORY-GAPS | accepted | The policy-engine context, section caps, evidence preservation bound, mutation-lock ceiling and the three-samples rule were missing from the inventory. | Revision 3 adds them to slice 2; the three-samples rule counts failures, not time. |
| PHD-BUDGET-PROTECTION | accepted | A candidate could raise its own runaway bound through the contract it ships. | The field joins the protected contract: the base's value wins for existing groups, a new group takes the candidate's, with a test. |
| PHD-FOUR-LOOPS-NOT-STARVATION | accepted | Four busy loops leave fourteen of eighteen processors free; the rows would prove ordinary co-scheduling. | Rows 1 and 4 start two loops per processor, measure the test's scheduling share and assert it below one half first, or skip by name. |
