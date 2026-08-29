# machine-concurrency-governor

- State: queued
- Intent: A properly designed parallelism mechanism: run as much in parallel as possible under a MACHINE-LEVEL configurable concurrency governor so the machine never overloads - today parallel passes are the coordinator's hand-scheduling with no load awareness, and last night's load flipped the missionrunner suite over its timeout and surfaced the acp flake (Wido 2026-08-29)
- Origin: human
- Next step: Appetite: 3h — design then build: machine-level slots configured at the repo/machine root (metasystem.concurrency.*: heavy-pass slots, suite slots, battery exclusivity), every run-launched job admitted against slots (queue, never refuse, when full), load-aware defaults derived from CPU count (the workflow precedent: min(16, cpus-2)), and the delegate verb's admission (L13) as the natural enforcement seam - design with L13 so it lands as one law, not a bolt-on. Until this lands the coordinator hand-governs by recorded delegation: cap concurrent heavy passes, never run a battery beside gauntlets or beds, stagger suites under load
- OpenedAt: 2026-08-29T07:38:20Z
- Revision: 1

History:
- 2026-08-29T07:38:20Z XAR0VVP0CKT8476P0AZ8K4KG68-m1-bf243850 open actor=human:wido targets=machine-concurrency-governor
Integrity: sha256=16cab60747acc39a63549a49d15a5f75558c4c3a112b71d17719b9afbdb66e63
