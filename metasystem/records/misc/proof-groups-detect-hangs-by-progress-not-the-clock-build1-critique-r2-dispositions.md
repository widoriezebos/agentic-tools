# Dispositions: code critique round 2 of slice 1 (phd-build1-crit3b-20260910)

Goal proof-groups-detect-hangs-by-progress-not-the-clock. Reviewed: chain
phd-build1-20260910 round 10, tree 8fd7fcedc21af38d3e02257d2f91a34c4fcf3027.
Critic: Opus 5 (claude runtime). Four material findings, three notes. The
critic could not run the real reader in its sandbox and emulated a
scheduler holding a helper off the CPU with SIGSTOP; plain load (72
spinners on 18 processors, 15 repeats) did not reproduce the failures,
which is why they are folded and not argued: a margin that only a starved
scheduler exposes is still a wall margin. The op phd-build1-crit3 was
refused at admission (its brief cited an unlanded record) and made no
findings. Decisions on the design page under "Build decisions".

| Finding | Severity | Disposition |
| --- | --- | --- |
| PHD-08 the dead-dump row passes only if the helper dumps and exits within four ticks of SIGQUIT | high | Folded (D-R11-1): after the verdict the gate marks output on every sample until the helper exits; the exit decides `dump: complete`. |
| PHD-09 pre-readiness synthetic CPU counts toward the budget, so `runaway` can fire before signal.Ignore is installed | high | Folded (D-R11-2): pre-readiness keep-alive marks output, never CPU. |
| PHD-10 the stopped row can send SIGCONT before the helper stops itself, then hang without bound | high | Folded (D-R11-3): the row reads the real task state and resumes only a stopped helper, repeating on every sample until it exits. |
| PHD-11 the Darwin retained-child row hangs if no real sample sees the nested child alive | medium | Folded (D-R11-4): nested children hold their work until the row releases them by closing their stdin after a sample has seen them. |
| PHD-12 the quiet run's share is not measured before the wall comparison (PHD-06 again) | note | Accepted as a note, unchanged. |
| PHD-13 Linux counts a reaped child twice under D-R7-2 read literally | note | Folded (D-R11-5): retention of vanished members is a Darwin rule; Linux uses live members plus cutime/cstime. |
| PHD-14 a real-reader row hangs when the reader has the defect it guards against, and slice 2 removes the attempt deadline that ends it today | note | Folded (D-R11-6): every real-reader row carries a CPU budget of ten times its expected consumption, so a reader defect ends the row `runaway` by name. |

Round 11 carries the fold; the third code critique reviews round 11.
