# lane-steward U2 correction 3 (Opus read of commit 9501b6feb): the barren stop is kept while the hold stands

Working Mode: Implement. Correction to the top commit of this branch (build/lst6, "goal lane-steward units U2 correction 2"); re-read plans/lane-steward-u2-after-2.md item 2. Leave the changes uncommitted; change only what this brief names; never commit, rebase or push; never touch memory/receipts.log, plans/, testing.json, go.mod or any metasystem.conf*.

1. (F-1) `closeGoneLoopStops` (stop.go ~88-89) compares a barren stop's Attempt with `status.Owner.Barren`, which is always 0 because `StewardStep` builds its view (steward.go ~69) without a `Fingerprint` source (view.go ~163 needs it); so the lane's barren-hold stop (lane-return, subject "lane") is closed as "subject gone" on every steward step while the keeper still holds, the person-facing remedy vanishes between ticks, and the keeper appends a new stop each tick (stops.jsonl grows two lines per tick). Give `StewardStep`'s `BuildView` the `Fingerprint: KeeperFingerprint` source (so `Owner.Barren` carries the real count), or compare against `lane.ReadAgentState(home).Barren` and keep the stop when the read fails. Fixture: an agent state with Barren 2 and an open lane-return stop with Attempt 2 → the step keeps it and appends nothing; Barren 0 and the same stop → closed as "subject gone".
2. (N-1, small) `TestLandingHealthCLIRealEntrypoint` stamps its fixture stop with time.Now(); use the bed's clock.

## Checks
as correction 2's brief. Report the change, its fixture, the mutation (drop the fingerprint source → fixture red) and every check's exit.
