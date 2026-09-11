Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 2: the three gaps are decided; finish the supervisor

Chain phd-build1-20260910. Round 1 stopped at three specification gaps
and preserved a partial supervisor (internal/proofrun/supervisor.go), the
`-timeout 0` argument, the nullable `cpuBudgetSeconds` field with its
protection and test. Nothing of that is to be deleted. The design is
metasystem/plans/proof-groups-detect-hangs-by-progress-not-the-clock-design.md
(revision 3); the build brief is
metasystem/plans/proof-groups-detect-hangs-by-progress-not-the-clock-build1-brief.md.
Report `round` as 2. Your worktree was fast-forwarded to the trunk tip
by the dispatcher; do not fetch or rebase it yourself.

## Decisions (the designer's answers; they amend revision 3 and will be
## recorded on the design page)

D-R2-1 Linux clock ticks. Read AT_CLKTCK from /proc/self/auxv once per
process (cgo-free, no new dependency). If the auxv is unreadable or the
entry is absent, use 100, the USER_HZ constant the kernel exposes to
userspace, and say so on the group record's progressRule (append
`ticks/assumed-100`). Darwin keeps `ps -S -o cputime=`.

D-R2-2 Stopped and host-waiting trees. `stopped` and `waiting on the
host` are not terminal verdicts. They are the supervisor's current
reading, named on the group record and on the TEST-GROUP progress line,
and the supervisor keeps waiting without bound: the group and the
attempt stay under supervision until the process tree exits on its own
(then the ordinary result applies) or an authorized cancellation of the
attempt takes custody through the existing cancellation path, which
kills the tree as it does today. No custody-transfer record; no
detached-worktree cleanup while the tree lives. The zero-consumption
window restarts when the tree leaves the stopped or waiting state, so a
tree that resumes and then stays idle is judged from its resumption.
Only `dead` (zero consumption, no output, no member stopped or waiting
for the whole window), `runaway` and `invalid` end a group.

D-R2-3 The budget's domain. `cpuBudgetSeconds` is a nullable integer,
strictly positive when present, refused by contract validation
otherwise; absent means no budget and the group runs under the dead
rule only. Zero is never written and never read as a budget. This is the
representation the round-1 edit already carries; keep it.

## Mandate for this round

1. Finish `superviseCommand` and the process-tree readers for both
   platforms per the build brief's mandate item 3, with D-R2-1 and
   D-R2-2 above: tree by parent links rebuilt each sample; CPU counter
   = self plus reaped-children per live member, high-water, never
   decreasing; partial samples keep the counter; three outright reader
   failures in a row end the group `invalid`; the stop ladder (SIGQUIT,
   wait, kill at once if the counter still rises at the next sample,
   `dump: not produced, the process kept computing`; a quiet tree falls
   to the zero-consumption rule then SIGKILL; `dump: complete` or
   `dump: killed while writing`).
2. `internal/proofrun/test_build.go`: the group child runs under the
   supervisor in place of `command.Run()`; same bytes to the same buffer
   and log the parsers read today; a section's stage-results file growth
   counts as output. Native discovery in test_go.go runs under the
   supervisor with a sixty-CPU-second budget and no window, replacing
   the thirty-minute context. No `-timeout` literal other than `0` in
   internal/proofrun.
3. The group record carries `progressRule`, `cpuSeconds`,
   `longestSilentSeconds`, `longestZeroCpuSeconds`; `runaway` and `dead`
   are failed deliveries with their reason on the TEST-GROUP line;
   `invalid` names the reader failure.
4. Tests per build-brief item 6: rows 1 to 9 and 12 to 14 as amended,
   the measured scheduling-share precondition for the busy-loop rows
   (skip by name where the share cannot be pushed below one half), the
   `stopped` row (a SIGSTOPped child is reported stopped and left alive
   past a shortened window; SIGCONT then ends it normally), the
   SIGQUIT-ignoring busy child killed at once, the protected-field test
   (kept), the production-pair test for `supervisorLimitsForTest`, the
   AT_CLKTCK reader test (Linux build tag) and the Darwin cputime parser
   test with the [[dd-]hh:]mm:ss shapes.
5. No test in this round may depend on the host's load: every outcome is
   decided by consumption, task state or bytes. If a test needs a wall
   duration it is the shortened test window from the one hook, and only
   to bound how long an idle child is watched.

## Proof

go build ./..., go vet ./..., gofmt -l; go test -count=1 on
internal/proofrun and internal/testpolicy, green. Run them; report the
commands and their tails as evidence. Report the round as your own.

## Constraints

Wall-clock budget: 40 minutes. Return per the implementer schema. Stop at
a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
