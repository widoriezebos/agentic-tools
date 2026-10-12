# lane-steward U1 correction 1 (Astra round 4, fixture-expressible): four reporting defects

Working Mode: Implement. Correction to the unit you just built from `plans/lane-steward-u1-health-facts-and-the-rows-that-fire.md` (re-read it: these four changes are now in it). Worktree uncommitted; change only what this brief names; never commit, rebase or push; never touch memory/receipts.log, plans/, testing.json, go.mod or any metasystem.conf*.

1. `silent-agent` also fires for an EMPTY batch: alive, no step for > 20 min, no explaining record — with or without waiting work (finding 134's stall was an empty batch). The row text says "with N goals waiting" or "with nothing to do". Reuse `agentIdleFinding` (status.go) as the seed. Fixture (c): the empty-queue case fires.
2. `EngineHealth` reads the LANE INSTALL's engine identity from its binary stamp through `internal/enginebuild`'s stamp reader (record.go), never the observer's `debug.ReadBuildInfo`. Fixture (i): a stamped fixture binary, not the test process.
3. `abandoned-resolution` is observed where `readStatus` detects the abandoned fix, BEFORE it sets `RunningFix = nil`; hand that observation to the health builder. Fixture (e2) drives the case through `readStatus`.
4. `batch-of-one` names its prerequisite: `sealed_at` is stamped by the lane at the full proof start (lane-batching, main 9f869cbcc); a batch record without the stamp → fact unknown, no row.

## Checks
as the unit's brief; report the four changes, their tests and every check's exit.

## Added from Astra round 5 (also folded into the unit brief)
5. `batch-of-one`: a different proof depth is NOT an exclusion (depth.go admits every entry; the batch takes the highest depth). The predicate is: eligible, cap above one, automatic policy, no person's seal. Fixture: a newcomer of another depth handed in before the seal → fires.
6. `proof-drift` compares COMPARABLE executions only (same scope as the running proof; `Result` records it) and counts each attempt once; reuse the full-proof estimator's selection in restart.go. Fixture: five 4-min impact proofs before a 40-min full proof → no drift; duplicate result lines for one attempt → counted once.
7. Reading health writes nothing: `readStatus` persists an abandoned resolution through `RefreshMerge`/`WriteFix`; give the health path a read-only observation (split `refreshMergeLocked` into observe + persist; the status path keeps persisting as today) and add the test that every lane record is byte-identical after `landing health`.
