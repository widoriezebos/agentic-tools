# benchmark-evidence-schema-drift

- State: queued
- Intent: The measuring kit's evidence schemas evolve WITH the engine: schema-4 mission states validate, and a battery fixture makes engine/kit schema drift impossible to land
- Origin: human
- Next step: Appetite: 2h, agreed with Wido 2026-08-23 — HIGHEST PRIORITY, reserved for m2 (Wido's word; execution deliberately HELD until he finishes the current conversation). Two parts, one landing: (1) update benchmark/schemas/evidence/ (mission-state, orchestrator, census surfaces) to admit the landed WSS schema 4 — admissionOrigins, extended openTurn, schemaVersion 4, census scanSeq — verified by re-running extraction against rep 1's real evidence in the VM (benchmark/results/89b2509.../1.json currently fails evidenceSetComplete with sourceOwner=kit); (2) the structural fix: a battery-wired fixture that creates a mission state with the CURRENT engine and validates it against the kit's evidence schema, so this drift class (three sightings this weekend: o8 case edit, il-28 provisioning collisions, this) can never land green again. Unblocks a meaningful bm-2d rep 2.
- OpenedAt: 2026-08-23T18:25:29Z
- Revision: 1

History:
- 2026-08-23T18:25:29Z T5VHM5ZEY3HBK0061JDGP7Z3SN-m2-bc1be9cb open actor=m2+mac-coordinator targets=benchmark-evidence-schema-drift
Integrity: sha256=50a4e527b24bf2aed1fd4075240b6a08d2d08a45f5357b4429b8e938d607a237
