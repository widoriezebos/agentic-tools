# benchmark-evidence-schema-drift

- State: done
- Intent: The measuring kit's evidence schemas evolve WITH the engine: schema-4 mission states validate, and a battery fixture makes engine/kit schema drift impossible to land
- Origin: human
- Next step: Appetite: 2h, agreed with Wido 2026-08-23 — HIGHEST PRIORITY, reserved for m2 (Wido's word; execution deliberately HELD until he finishes the current conversation). Two parts, one landing: (1) update benchmark/schemas/evidence/ (mission-state, orchestrator, census surfaces) to admit the landed WSS schema 4 — admissionOrigins, extended openTurn, schemaVersion 4, census scanSeq — verified by re-running extraction against rep 1's real evidence in the VM (benchmark/results/89b2509.../1.json currently fails evidenceSetComplete with sourceOwner=kit); (2) the structural fix: a battery-wired fixture that creates a mission state with the CURRENT engine and validates it against the kit's evidence schema, so this drift class (three sightings this weekend: o8 case edit, il-28 provisioning collisions, this) can never land green again. Unblocks a meaningful bm-2d rep 2.
- Concluded: Landed 8dcb90a, battery green after four runs that each caught a real defect (the template's outside-reference law, the adopted-conf leak, the fixture prune lag). Evidence schemas admit the landed schema-4 state and census; wall-parked turns owe no return; dot-dirs are bookkeeping; model leniency is the declared benchmark/model-equivalence.json through the existing acceptableEffective hook; delegationFloorMet reports as a metric per Wido's measurement-bar ruling; the drift fixture validates a fresh CURRENT-engine state against the kit's ruler in validate-kit AND every battery via the new project-declared validate.extra-suites seam, with adoption's tailor provably stripping the promise. Rep 1 rescored VALID with every gate green under the corrected ruler (local verification; official VM rescore next). Concluded by Wido's standing word on landed-and-verified reports.
- OpenedAt: 2026-08-23T18:25:29Z
- Revision: 3

History:
- 2026-08-23T18:25:29Z T5VHM5ZEY3HBK0061JDGP7Z3SN-m2-bc1be9cb open actor=m2+mac-coordinator targets=benchmark-evidence-schema-drift
- 2026-08-23T18:25:38Z RNHTBJHHRD8KG6DAT7HBF54MPM-m2-bc1be9cb claim actor=m2+mac-coordinator targets=benchmark-evidence-schema-drift
- 2026-08-24T00:10:07Z A2HDV0WQ7ZD74T0KSDBD55VCJK-m2-bc1be9cb done actor=human:wido targets=benchmark-evidence-schema-drift
Integrity: sha256=2c83e8c5d6242abb0356ab59e7d9e1b8318cdc362bf161c66e9b6ba6cddaf4bf
