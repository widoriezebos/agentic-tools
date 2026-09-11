# Dispositions: code critique round 1 of slice 1 (phd-build1-crit2-20260910)

Goal proof-groups-detect-hangs-by-progress-not-the-clock. Reviewed: chain
phd-build1-20260910 round 6, tree 0df8a6d06c51259b0a84d2bb51792df9fe5f85c0.
Critic: Opus 5 (claude runtime). Five material findings, two notes. An
earlier critic job (phd-build1-crit1-20260910) was cancelled against
round 2 after the seat gate exposed a specification defect; it made no
findings. Decisions recorded on the design page under "Build decisions".

| Finding | Severity | Disposition |
| --- | --- | --- |
| PHD-01 seven fixtures race a re-executed helper against a shortened wall window | critical | Folded (D-R7-1): every helper hand-shakes `ready` after its setup; the scripted reader reports rising CPU until the handshake is read; fixtures wait on reader calls or the process, never on a sleep; the stop row's five-second wait removed. |
| PHD-02 the counter is the high-water of a sum; Darwin `ps -S` does not fold reaped children | high | Folded (D-R7-2): per-member high-water summed and retained after exit; the seat verified `ps -S` adds nothing on the fleet's Macs; Darwin's sub-sample-interval children recorded as residual (f); no libproc (cgo-free engine). |
| PHD-03 rows 3, 5, 6, 14 never touch the real reader | medium | Folded (D-R7-3): rows 5, 6, 14 run with the real reader and hand-shaken Setsid helpers; row 3 asserts the goroutine dump in the log. |
| PHD-04 row 12's test passes on the old code | low | Folded (D-R7-4): ValidateTestResult accepts runaway and dead, refuses unknown; the TEST-GROUP line carries the reason. |
| PHD-05 competitor helpers can leak and burn every processor | medium | Folded (D-R7-5): every helper exits when its stdin reaches end of file; the fixture holds the write end; a test closes it and observes the exit. |
| PHD-06 the quiet run's share is not measured before the wall comparison | note | Accepted as a note: the ratio bound (two to nine times) and the measured loaded share make a reversal need a quiet run slower than the loaded one; the comment names the precondition. |
| PHD-07 a windowless group waits forever after SIGQUIT; Linux ESRCH counts as an outright failure | note | Folded (D-R7-6): the dump wait of a windowless group ends at the first sample with no rise; ENOENT and ESRCH are partial samples. |

Round 7 carries the fold; the second code critique reviews round 7.
