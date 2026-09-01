# delegate-wait-primitive

- State: queued
- Intent: A failed or finished delegate job must be waitable-on honestly: one primitive (wait-job) that blocks on the JOB RECORD reaching terminal state with a hard bound, always terminates, and always prints status plus failure reason - replacing every hand-rolled artifact-file wait. Two specimens in two days of seats waiting on the wrong condition (2026-08-31 afternoon, 2026-09-01 night, records/misc/) prove the class
- Origin: main
- Next step: Appetite: 1h. Wido's word 2026-09-01 (verbatim: 'Anything you design/fix needs to be hard deterministic machinery. This is Go territory enforcing your behaviour'): the primitive is an ENGINE VERB, metasystem job wait, in Go with Go tests - not a shell script. INTENT: block until the job record reaches a terminal status; always terminate; always print status and failureReason; deterministic exit codes (0 completed, 1 failed, 2 other-terminal, 3 stalled-record, 4 hard-bound, 5 no-record). Patience per R-35: progress-based on record change, never bare wall-clock; bounds injectable for tests. Terminal statuses come from the engine's own terminalStatuses map - one source of truth, no mirror. A thin scripts/agents/wait-job.sh shim may relay per the kill-shell pattern. Prove with Go tests covering all five exits
- OpenedAt: 2026-09-01T06:44:30Z
- Revision: 2

History:
- 2026-09-01T06:44:30Z BXSHZ6VDPVJVG3140AG5MWHDWB-m3-a5da21ff open actor=m3+mac-m3 targets=delegate-wait-primitive
- 2026-09-01T06:55:55Z 6FWMV8TM5K8Q0C4ZRBZ7CP7ESY-m3-a5da21ff edit actor=m3+mac-m3 targets=delegate-wait-primitive
Integrity: sha256=e12e617571c973ab3879846dbc27009fe6ff8550f8e108b5d429ddacdcec73bd
