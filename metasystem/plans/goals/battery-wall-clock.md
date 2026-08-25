# battery-wall-clock

- State: claimed
- Intent: The landing battery's wall clock drops from ~1h45 to ~45min (full) and ~5min (iteration tier) with every guarantee intact: proofs transferred via witness, never skipped (Wido agreed 2026-08-24 evening)
- Origin: human
- Next step: Appetite: half-day, four levers in value order. (1) WITNESS REUSE: route the landing battery through one witness-armed validate-metasystem run (or arm the witness for standalone adopt-fixtures) so every nested validate does --witness-check-only instead of re-running the race suite — D33 machinery exists, it is wiring. (2) DEDUPE THE TOP: go-gate's race+cover run subsumes the battery's plain 'go test ./...'; drop the duplicate. (3) EVIDENCE GATE EARLY + LATE: run the covenant evidence gate early in validate with whatever engine is present (fail-fast for red fixtures, refusals in seconds) and re-run after the rebuild so the proven-binary property holds; the two red adoption fixtures stop paying a full suite each. (4) CANARY TIER: the fix-until-AGREE loop runs the touched-surface tier (~5min: changed packages + the touched fixture leg); the FULL battery runs once on the final bytes before push — it stays the landing requirement, only its cadence changes. Out of scope: leg-level parallelism inside adopt-fixtures (only if 1-3 disappoint). Queue: immediately after counselor slice one lands, before fleet-pull — it pays for itself within two landings.
- OpenedAt: 2026-08-24T21:34:17Z
- Revision: 2
- Claimed: machine=m1 lineage=coordinator at=2026-08-25T01:59:41Z

History:
- 2026-08-24T21:34:17Z 06AFK2SV91PDQCCD0PY88V96DC-m1-bf243850 open actor=human:wido targets=battery-wall-clock
- 2026-08-25T01:59:41Z N47MR5EJ2W2M7ATK59PGAC1BVB-m1-bf243850 claim actor=m1+coordinator targets=battery-wall-clock
Integrity: sha256=b431c946c72bfb00732284662c82feec5f10bc2c2fb2e6b21f7b40be21e6761f
