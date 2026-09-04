Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal fixture-suite-drift-carry)
Date: 2026-09-04

# Follow-up: the serving-goal approve and claim need the coordinator lineage

Seat-side after your round the channel suite is green and the dispatch scenario fails at the serving-goal leg with: "mutations carry their coordinator's identity: export METASYSTEM_OWNER_LINEAGE or pass --lineage" (exit 1). The earlier `goal open` in that leg already passes the lineage; give the new `goal approve` and `goal claim` calls the same `--lineage` value (or export the variable for the leg). Nothing else. Every path in your return is relative to the repository root (starting with `metasystem/`). The orchestrator reruns the suites seat-side.
