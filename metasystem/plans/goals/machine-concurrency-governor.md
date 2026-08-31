# machine-concurrency-governor

- State: queued
- Intent: A properly designed parallelism mechanism: run as much in parallel as possible under a MACHINE-LEVEL configurable concurrency governor so the machine never overloads - today parallel passes are the dispatch delegate's hand-scheduling with no load awareness, and last night's load flipped the missionrunner suite over its timeout and surfaced the acp flake (Wido 2026-08-29)
- Origin: human
- Next step: Appetite: 3h — design then build: machine-level slots configured at the repo/machine root (metasystem.concurrency.*: heavy-pass slots, suite slots, battery exclusivity), every run-launched job admitted against slots (queue, never refuse, when full), load-aware defaults derived from CPU count (the workflow precedent: min(16, cpus-2)), and the delegate verb's admission (L13) as the natural enforcement seam - design with L13 so it lands as one law, not a bolt-on. Until this lands the dispatch delegate hand-governs by recorded delegation: cap concurrent heavy passes, never run a battery beside gauntlets or beds, stagger suites under load SECOND DRIVING INCIDENT (2026-08-31): the first governed weight-discharge validation stalled in go-engine-gate past the 45m section cap while still producing output, during m3's attested heavy window on the shared Mac (both seats' timelines on record; cross-seat hold coordinated by session message). Wido's leniency word R-32-m2 covers the interim (machine-local cap 90m, coordinated quiet windows); THIS goal is the permanent fix and gains priority from the incident.
- OpenedAt: 2026-08-29T07:38:20Z
- Revision: 4

History:
- 2026-08-29T07:38:20Z XAR0VVP0CKT8476P0AZ8K4KG68-m1-bf243850 open actor=human:wido targets=machine-concurrency-governor
- 2026-08-31T10:15:44Z AQ5VCH7NJG1MC2EZJ3R3Q7SZ3D-m2-bc1be9cb edit actor=m2+mac-coordinator targets=machine-concurrency-governor
- 2026-08-31T10:15:59Z B4JFV1YQA7MNHK8KPM09SGA62M-m2-bc1be9cb edit actor=m2+mac-coordinator targets=machine-concurrency-governor
- 2026-08-31T11:44:41Z Q6WV24WKKFF8FRQJXDEGMXYXVQ-m2-bc1be9cb edit actor=m2+mac-coordinator targets=machine-concurrency-governor
Integrity: sha256=01b98cbc00fe61e097764990ebd42e95774f00b06a9033e98512afe3bd8ba7fb
