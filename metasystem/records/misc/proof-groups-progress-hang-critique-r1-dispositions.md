# proof-groups-detect-hangs-by-progress-not-the-clock, design critique round one: dispositions

Design revision 1 (landed 19775023a) was critiqued by
phd-design-crit1b-20260910 (codex gpt-5.6-sol), which returned nine
material findings and could not write its record from its read-only
sandbox; the findings are carried here from the job's return record.
Revision 2 folds every one.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| PHD-PROGRESS-NOT-HANG | accepted | "No output and no CPU" is neither necessary nor sufficient: a test waiting on a socket or a disk consumes nothing and is alive; a runaway loop consumes forever and never finishes. | Revision 2 replaces the rule with a CPU budget for runaway compute and an exactly-zero-consumption window of thirty minutes for deadlock, each with its ground (Decision 1). |
| PHD-DEADLINE-REMAINS | accepted | The attempt deadline is enforced by the worker context in cmd/metasystem/test.go and by child admission, authentication and finalization in internal/proofrun/attempt.go, not only by the watchdog. | Revision 2 inventories all five sites and slice 2 removes each as a kill or refusal (Decision 3, rows 10 and 11). |
| PHD-REMAINING-CLOCKS | accepted | go-gate.sh runs go test with a sixty-minute timeout, coverage-delta.sh with thirty, and twenty-one scripts under scripts/agents carry SECONDS deadlines; revision 1 named none of them. | Revision 2 inventories every bound with its slice; slice 3 replaces the shell clocks through one engine verb, proc supervise. |
| PHD-PROCESS-GROUP-SELECTOR | accepted | Numeric ps -g selects a session on Linux procps, not the process group Setpgid creates. | Revision 2 reads the descendant tree by parent links and the counter from /proc/<pid>/stat on Linux and ps -S on Darwin, with a Linux fixture on the VM seat (Decision 2, row 7). |
| PHD-CPU-SNAPSHOT | accepted | Summing live members only loses exited children and short-lived ones; the one-second threshold had two readings. | Revision 2 counts self plus reaped-children CPU, keeps a high-water counter that never decreases, maps reader failures to invalid, and drops the threshold (Decision 2, rows 6 and 8). |
| PHD-SECTION-ESCAPE | accepted | Section launchers call Setsid, and in worker mode the section's output goes to its own stage log, not the injected progress log. | Revision 2 supervises the descendant tree and watches the section's stage results (Decision 2, row 14). |
| PHD-FIFTEEN-MINUTE-BASIS | accepted | The fleet has no measurement of its longest silent or zero-CPU interval; fifteen minutes was a guess. | Revision 2 removes the window from the runaway rule, grounds the deadlock window in what it must not misjudge, and records every group's longest silent and zero-consumption intervals so the fleet measures itself from the first landing (Decision 2 item 6). |
| PHD-DUMP-GRACE | accepted | A fixed ten-second grace before SIGKILL is a clock and cannot promise the dump completed under load. | Revision 2 waits for the dumping process to exit and falls back only to the zero-consumption rule; the record says whether the dump completed (Decision 2 item 5, row 3). |
| PHD-PROOF-MATRIX | accepted | The matrix omitted legitimate I/O wait, a nonterminating loop, exited and between-sample children, Linux selection, sampler errors, dump completion and the attempt lifecycle, and named no interface for the test hook. | Revision 2 rewrites the matrix around those paths with one named hook whose zero value is the production pair (Decision 4). |
