# delegate-wait-primitive

- State: queued
- Intent: A failed or finished delegate job must be waitable-on honestly: one primitive (wait-job) that blocks on the JOB RECORD reaching terminal state with a hard bound, always terminates, and always prints status plus failure reason - replacing every hand-rolled artifact-file wait. Two specimens in two days of seats waiting on the wrong condition (2026-08-31 afternoon, 2026-09-01 night, records/misc/) prove the class
- Origin: main
- Next step: Appetite: 1h. INTENT: scripts/agents or a metasystem verb wrapping poll-to-terminal on artifacts/agents/jobs/<id>.json with bounded patience per R-35 (progress-based, never bare wall-clock), exit code mirroring job outcome, status and failureReason printed. CONSTRAINT: replaces conduct, adds no new state. Prove with a fixture covering completed, failed, and never-started jobs
- OpenedAt: 2026-09-01T06:44:30Z
- Revision: 1

History:
- 2026-09-01T06:44:30Z BXSHZ6VDPVJVG3140AG5MWHDWB-m3-a5da21ff open actor=m3+mac-m3 targets=delegate-wait-primitive
Integrity: sha256=a931b07b8f6835f268a02e595686c0a15973f69981acbc41a8325ee39386d427
