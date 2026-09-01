# machine-concurrency-governor

- State: queued
- Intent: A properly designed parallelism mechanism: run as much in parallel as possible under a MACHINE-LEVEL configurable concurrency governor so the machine never overloads - today parallel passes are the dispatch delegate's hand-scheduling with no load awareness, and last night's load flipped the missionrunner suite over its timeout and surfaced the acp flake (Wido 2026-08-29)
- Origin: human
- Next step: Appetite: 3h — design then build: machine-level slots configured at the repo/machine root (metasystem.concurrency.*: heavy-pass slots, suite slots, battery exclusivity), every run-launched job admitted against slots (queue, never refuse, when full), load-aware defaults derived from CPU count (the workflow precedent: min(16, cpus-2)), and the delegate verb's admission (L13) as the natural enforcement seam - design with L13 so it lands as one law, not a bolt-on. Until this lands the dispatch delegate hand-governs by recorded delegation: cap concurrent heavy passes, never run a battery beside gauntlets or beds, stagger suites under load SECOND DRIVING INCIDENT (2026-08-31): the first governed weight-discharge validation stalled in go-engine-gate past the 45m section cap while still producing output, during m3's attested heavy window on the shared Mac (both seats' timelines on record; cross-seat hold coordinated by session message). Wido's leniency word R-32-m2 covers the interim (machine-local cap 90m, coordinated quiet windows); THIS goal is the permanent fix and gains priority from the incident.
- OpenedAt: 2026-08-29T07:38:20Z
- Revision: 5
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-29T07:38:20Z XAR0VVP0CKT8476P0AZ8K4KG68-m1-bf243850 open actor=human:wido targets=machine-concurrency-governor
- 2026-08-31T10:15:44Z AQ5VCH7NJG1MC2EZJ3R3Q7SZ3D-m2-bc1be9cb edit actor=m2+mac-coordinator targets=machine-concurrency-governor
- 2026-08-31T10:15:59Z B4JFV1YQA7MNHK8KPM09SGA62M-m2-bc1be9cb edit actor=m2+mac-coordinator targets=machine-concurrency-governor
- 2026-08-31T11:44:41Z Q6WV24WKKFF8FRQJXDEGMXYXVQ-m2-bc1be9cb edit actor=m2+mac-coordinator targets=machine-concurrency-governor
- 2026-09-01T20:28:38Z HTA6Y0BRQ44FQ3MSZN8BQJ0152-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=machine-concurrency-governor
Integrity: sha256=54bc65e56c6bf86c5cea2b8f10f3fb960195ebcc9f67bb5d6389bea37154d089
